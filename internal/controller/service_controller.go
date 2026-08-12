/*
Copyright 2026 Fábio Matavelli.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"net"
	"slices"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
	"github.com/fabiomatavelli/netbox-herald/internal/config"
	"github.com/fabiomatavelli/netbox-herald/internal/netbox"
)

// ServiceReconciler syncs Kubernetes Services of type LoadBalancer into
// NetBox, recording each assigned external IP as an IPAM IP Address with no
// Device/VM parent. NetBox's ipam.Service model expects a single owning
// Device/VM, which Kubernetes Services don't naturally have — a bare IP
// Address is the only NetBox object with real external meaning to document.
// ClusterIP and NodePort Services, and LoadBalancer Services without an
// assigned external IP yet, are not synced.
//
// A Service can own more than one address (e.g. dual-stack), so — unlike
// Cluster or Node — each address's external ID is a namespace/name/address
// composite rather than the Service's UID: once a Service is deleted,
// Reconcile only has its namespace/name (from the reconcile request) to
// work with, not its UID.
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Store supplies the current HeraldConfig spec and authenticated NetBox
	// client, populated by HeraldConfigReconciler.
	Store *config.Store
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

// Reconcile creates, updates, or deletes the NetBox IP Address(es)
// representing a single Kubernetes Service's external IP(s), based on
// spec.resources.services.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	spec, netboxClient := r.Store.Get()
	if spec == nil || netboxClient == nil {
		// NetBox isn't reachable yet. This controller only otherwise wakes
		// on Service changes and real HeraldConfig spec changes (see the
		// GenerationChangedPredicate in SetupWithManager), so poll until the
		// Store is populated instead of waiting for one.
		return ctrl.Result{RequeueAfter: storeNotReadyPollInterval}, nil
	}

	managedTagSlug := resolveManagedTagSlug(spec)
	resyncInterval := resolveResyncInterval(spec)

	var svc corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			if delErr := netbox.DeleteServiceIPs(ctx, netboxClient, managedTagSlug, req.Namespace, req.Name); delErr != nil {
				log.Error(delErr, "failed to remove NetBox IP addresses for a deleted service", "service", req.NamespacedName)
				return ctrl.Result{}, r.syncServicesStatus(ctx, managedTagSlug, spec, delErr)
			}
			return ctrl.Result{}, r.syncServicesStatus(ctx, managedTagSlug, spec, nil)
		}
		return ctrl.Result{}, err
	}

	qualifies := spec.Resources.Services.Enabled && svc.Spec.Type == corev1.ServiceTypeLoadBalancer

	if !qualifies {
		// Either Services sync is off (honor retainOnDisable), or this
		// particular Service just isn't a LoadBalancer — always clean up in
		// that second case, since there's nothing to "pause" for an object
		// that no longer matches what Herald syncs at all.
		if spec.Resources.Services.Enabled || !spec.Resources.Services.RetainOnDisable {
			if err := netbox.DeleteServiceIPs(ctx, netboxClient, managedTagSlug, svc.Namespace, svc.Name); err != nil {
				log.Error(err, "failed to remove NetBox IP addresses for a service no longer synced", "service", req.NamespacedName)
				return ctrl.Result{}, r.syncServicesStatus(ctx, managedTagSlug, spec, err)
			}
		}
		if err := r.syncServicesStatus(ctx, managedTagSlug, spec, nil); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: resyncInterval}, nil
	}

	if err := r.syncServiceAddresses(ctx, netboxClient, managedTagSlug, &svc); err != nil {
		log.Error(err, "failed to sync service to NetBox", "service", req.NamespacedName)
		return ctrl.Result{}, r.syncServicesStatus(ctx, managedTagSlug, spec, err)
	}

	log.V(1).Info("synced service to NetBox", "service", req.NamespacedName)
	if err := r.syncServicesStatus(ctx, managedTagSlug, spec, nil); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

// syncServiceAddresses ensures every currently-assigned external IP on svc
// has a matching NetBox IP Address, then prunes any previously-synced
// address that's no longer among them (e.g. the LoadBalancer was
// re-provisioned with a different IP).
func (r *ServiceReconciler) syncServiceAddresses(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTagSlug string, svc *corev1.Service) error {
	managedTag, err := netbox.EnsureManagedTag(ctx, netboxClient, managedTagSlug)
	if err != nil {
		return err
	}

	description := fmt.Sprintf("Kubernetes Service %s/%s", svc.Namespace, svc.Name)

	var desiredExternalIDs []string
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.IP == "" {
			continue
		}
		address, err := cidrForIP(ingress.IP)
		if err != nil {
			return fmt.Errorf("service %s/%s: %w", svc.Namespace, svc.Name, err)
		}

		externalID := netbox.ExternalIDForServiceAddress(svc.Namespace, svc.Name, ingress.IP)
		desiredExternalIDs = append(desiredExternalIDs, externalID)

		if _, err := netbox.EnsureServiceIP(ctx, netboxClient, netbox.ServiceIPSpec{
			Address:     address,
			Description: description,
			ExternalID:  externalID,
			ManagedTag:  managedTag,
		}); err != nil {
			return err
		}
	}

	owned, err := netbox.ListServiceIPs(ctx, netboxClient, managedTagSlug, svc.Namespace, svc.Name)
	if err != nil {
		return err
	}
	for _, addr := range owned {
		id, _ := addr.CustomFields.(map[string]any)[netbox.ExternalIDFieldName].(string)
		if slices.Contains(desiredExternalIDs, id) {
			continue
		}
		if err := netbox.DeleteServiceIP(ctx, netboxClient, addr.ID); err != nil {
			return err
		}
	}

	return nil
}

// cidrForIP renders ip in CIDR notation with a host mask, since the exact
// prefix length of the containing subnet isn't knowable from a bare
// external IP.
func cidrForIP(ip string) (string, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("invalid IP address %q", ip)
	}
	if parsed.To4() != nil {
		return ip + "/32", nil
	}
	return ip + "/128", nil
}

// syncServicesStatus recomputes status.resources.services on the
// HeraldConfig singleton after this Service's sync attempt. lastError from
// the most recent attempt is recorded; a nil syncErr clears any previously
// reported error.
func (r *ServiceReconciler) syncServicesStatus(ctx context.Context, managedTagSlug string, spec *heraldv1alpha1.HeraldConfigSpec, syncErr error) error {
	var cr heraldv1alpha1.HeraldConfig
	if err := r.Get(ctx, types.NamespacedName{Name: singletonName}, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	_, netboxClient := r.Store.Get()
	if netboxClient == nil {
		return nil
	}

	status := heraldv1alpha1.ResourceSyncStatus{
		Enabled:            spec.Resources.Services.Enabled,
		ObservedGeneration: cr.Generation,
	}

	if count, err := netbox.CountManagedServiceIPs(ctx, netboxClient, managedTagSlug); err == nil {
		status.ObjectCount = int32(count)
	}

	if syncErr != nil {
		status.LastError = syncErr.Error()
	} else {
		now := metav1.Now()
		status.LastSyncTime = &now
	}

	cr.Status.Resources.Services = status
	return r.Status().Update(ctx, &cr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Watches(
			&heraldv1alpha1.HeraldConfig{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllServices),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Named("service").
		Complete(r)
}

// enqueueAllServices lists every Service and enqueues a reconcile request
// for each one, so that changes to HeraldConfig (enabling/disabling service
// sync, etc.) are applied to every known Service immediately rather than
// waiting for their next native event.
func (r *ServiceReconciler) enqueueAllServices(ctx context.Context, _ client.Object) []ctrl.Request {
	var services corev1.ServiceList
	if err := r.List(ctx, &services); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list services for HeraldConfig change")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(services.Items))
	for _, svc := range services.Items {
		requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}})
	}
	return requests
}
