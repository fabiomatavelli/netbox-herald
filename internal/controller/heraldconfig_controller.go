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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
	"github.com/fabiomatavelli/netbox-herald/internal/config"
	"github.com/fabiomatavelli/netbox-herald/internal/netbox"
)

// singletonName is the only HeraldConfig name Herald reconciles. It's
// cluster-scoped and meant to exist as a single instance; enforced by
// convention here rather than a validating webhook for now.
const singletonName = "default"

// defaultManagedTagSlug mirrors HeraldConfigSpec.NetBox.ManagedTag.Slug's
// +kubebuilder:default, used whenever the field is unset.
const defaultManagedTagSlug = "netbox-herald-managed"

// resolveManagedTagSlug returns spec.netbox.managedTag.slug, falling back to
// defaultManagedTagSlug when unset.
func resolveManagedTagSlug(spec *heraldv1alpha1.HeraldConfigSpec) string {
	if spec.NetBox.ManagedTag.Slug == "" {
		return defaultManagedTagSlug
	}
	return spec.NetBox.ManagedTag.Slug
}

// HeraldConfigReconciler reconciles a HeraldConfig object
type HeraldConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// OperatorNamespace is the namespace Herald itself runs in, and where
	// spec.netbox's referenced Secrets are read from.
	OperatorNamespace string

	// Store is populated with the current spec and authenticated NetBox
	// client on every successful reconcile, for resource-type controllers
	// to read.
	Store *config.Store
}

// +kubebuilder:rbac:groups=netbox-herald.io,resources=heraldconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=netbox-herald.io,resources=heraldconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=netbox-herald.io,resources=heraldconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile resolves spec.netbox into an authenticated client, verifies
// NetBox is reachable and compatible, bootstraps the managed tag and
// external-ID custom field, and publishes both the spec and client via
// Store for every other controller to use.
func (r *HeraldConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if req.Name != singletonName {
		log.Info("ignoring HeraldConfig: netbox-herald only reconciles the instance named \"default\"", "name", req.Name)
		return ctrl.Result{}, nil
	}

	var cr heraldv1alpha1.HeraldConfig
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			r.Store.Set(nil, nil)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	resyncInterval := cr.Spec.Resync.Interval.Duration
	if resyncInterval <= 0 {
		resyncInterval = 5 * time.Minute
	}

	netboxCfg, err := netbox.LoadConfig(ctx, r.Client, r.OperatorNamespace, cr.Spec.NetBox)
	if err != nil {
		return r.reportUnreachable(ctx, &cr, err)
	}

	netboxClient, err := netboxCfg.Client()
	if err != nil {
		return r.reportUnreachable(ctx, &cr, err)
	}

	version := ""
	if cr.Spec.NetBox.VersionCheck.Enabled {
		version, err = netbox.CheckVersion(ctx, netboxClient)
		if err != nil {
			return r.reportUnreachable(ctx, &cr, err)
		}
	}

	managedTagSlug := resolveManagedTagSlug(&cr.Spec)
	if _, err := netbox.EnsureManagedTag(ctx, netboxClient, managedTagSlug); err != nil {
		return r.reportUnreachable(ctx, &cr, err)
	}
	if err := netbox.EnsureExternalIDCustomField(ctx, netboxClient); err != nil {
		return r.reportUnreachable(ctx, &cr, err)
	}

	r.Store.Set(&cr.Spec, netboxClient)

	now := metav1.Now()
	cr.Status.NetBox = heraldv1alpha1.NetBoxStatus{
		Connected:       true,
		Version:         version,
		LastCheckedTime: &now,
	}
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:    "NetBoxReachable",
		Status:  metav1.ConditionTrue,
		Reason:  "Connected",
		Message: "Successfully connected to NetBox",
	})
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Connected",
		Message: "Connected to NetBox; ready to sync enabled resources",
	})

	if err := r.Status().Update(ctx, &cr); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

// reportUnreachable records a failed connectivity attempt in status, clears
// the shared Store so resource controllers stop using a stale client, and
// requeues with the standard controller-runtime error backoff.
func (r *HeraldConfigReconciler) reportUnreachable(ctx context.Context, cr *heraldv1alpha1.HeraldConfig, cause error) (ctrl.Result, error) {
	r.Store.Set(nil, nil)

	now := metav1.Now()
	cr.Status.NetBox = heraldv1alpha1.NetBoxStatus{
		Connected:       false,
		LastCheckedTime: &now,
	}
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:    "NetBoxReachable",
		Status:  metav1.ConditionFalse,
		Reason:  "ConnectionFailed",
		Message: cause.Error(),
	})
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "NetBoxUnreachable",
		Message: "Cannot sync any resources until NetBox is reachable",
	})

	if updateErr := r.Status().Update(ctx, cr); updateErr != nil {
		logf.FromContext(ctx).Error(updateErr, "failed to update HeraldConfig status after a connectivity failure")
	}

	return ctrl.Result{}, cause
}

// SetupWithManager sets up the controller with the Manager.
func (r *HeraldConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&heraldv1alpha1.HeraldConfig{}).
		Named("heraldconfig").
		Complete(r)
}
