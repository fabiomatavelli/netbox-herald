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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
	"github.com/fabiomatavelli/netbox-herald/internal/config"
	"github.com/fabiomatavelli/netbox-herald/internal/netbox"
)

// ClusterReconciler syncs the Kubernetes cluster itself into a single NetBox
// Virtualization Cluster object. Unlike Nodes or Services, there is exactly
// one of these per Herald deployment, so it reconciles on HeraldConfig
// directly rather than watching a native per-instance Kubernetes type.
//
// The synthetic cluster's stable external ID is the kube-system Namespace's
// UID: immutable, present in every cluster, and requires no extra config.
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Store supplies the current HeraldConfig spec and authenticated NetBox
	// client, populated by HeraldConfigReconciler.
	Store *config.Store
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile creates, updates, or deletes the NetBox Cluster representing
// this Kubernetes cluster, based on spec.resources.cluster.
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if req.Name != singletonName {
		return ctrl.Result{}, nil
	}

	spec, netboxClient := r.Store.Get()
	if spec == nil || netboxClient == nil {
		// NetBox isn't reachable yet. This controller only otherwise wakes
		// on real HeraldConfig spec changes (see the GenerationChangedPredicate
		// in SetupWithManager), so poll until the Store is populated instead
		// of waiting for one.
		return ctrl.Result{RequeueAfter: storeNotReadyPollInterval}, nil
	}

	var cr heraldv1alpha1.HeraldConfig
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	var kubeSystem corev1.Namespace
	if err := r.Get(ctx, types.NamespacedName{Name: "kube-system"}, &kubeSystem); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting kube-system namespace: %w", err)
	}
	externalID := string(kubeSystem.UID)

	managedTagSlug := resolveManagedTagSlug(spec)
	resyncInterval := resolveResyncInterval(spec)

	if !spec.Resources.Cluster.Enabled {
		if err := netbox.DeleteCluster(ctx, netboxClient, managedTagSlug, externalID); err != nil {
			return r.reportSyncError(ctx, &cr, err)
		}
		return ctrl.Result{RequeueAfter: resyncInterval}, r.updateStatus(ctx, &cr, heraldv1alpha1.ResourceSyncStatus{
			Enabled:            false,
			ObservedGeneration: cr.Generation,
		})
	}

	managedTag, err := netbox.EnsureManagedTag(ctx, netboxClient, managedTagSlug)
	if err != nil {
		return r.reportSyncError(ctx, &cr, err)
	}

	if _, err := netbox.EnsureCluster(ctx, netboxClient, netbox.ClusterSpec{
		Name:             spec.Resources.Cluster.Name,
		ClusterTypeName:  spec.Resources.Cluster.ClusterTypeName,
		ClusterGroupName: spec.Resources.Cluster.ClusterGroupName,
		SiteName:         spec.Resources.Cluster.SiteName,
		ExternalID:       externalID,
		ManagedTag:       managedTag,
	}); err != nil {
		return r.reportSyncError(ctx, &cr, err)
	}

	log.V(1).Info("synced cluster resource to NetBox")

	now := metav1.Now()
	return ctrl.Result{RequeueAfter: resyncInterval}, r.updateStatus(ctx, &cr, heraldv1alpha1.ResourceSyncStatus{
		Enabled:            true,
		LastSyncTime:       &now,
		ObservedGeneration: cr.Generation,
		ObjectCount:        1,
	})
}

// updateStatus writes status into cr.Status.Resources.Cluster and persists
// it. A zero-value LastError implicitly clears any previously reported
// failure.
func (r *ClusterReconciler) updateStatus(ctx context.Context, cr *heraldv1alpha1.HeraldConfig, status heraldv1alpha1.ResourceSyncStatus) error {
	cr.Status.Resources.Cluster = status
	return r.Status().Update(ctx, cr)
}

// reportSyncError records a failed sync attempt in status without disturbing
// the rest of HeraldConfig's status, and requeues with the standard
// controller-runtime error backoff.
func (r *ClusterReconciler) reportSyncError(ctx context.Context, cr *heraldv1alpha1.HeraldConfig, cause error) (ctrl.Result, error) {
	cr.Status.Resources.Cluster.LastError = cause.Error()
	cr.Status.Resources.Cluster.ObservedGeneration = cr.Generation
	if err := r.Status().Update(ctx, cr); err != nil {
		logf.FromContext(ctx).Error(err, "failed to update HeraldConfig status after a cluster sync failure")
	}
	return ctrl.Result{}, cause
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&heraldv1alpha1.HeraldConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("cluster").
		Complete(r)
}
