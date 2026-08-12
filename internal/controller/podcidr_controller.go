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
	"math/big"
	"net"
	"slices"
	"time"

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

const (
	familyIPv4 = "ipv4"
	familyIPv6 = "ipv6"
)

// podCIDRAddressFamilies are the address families pod CIDR aggregation
// considers, in a stable order so status/logging output is deterministic.
var podCIDRAddressFamilies = []string{familyIPv4, familyIPv6}

// PodCIDRReconciler syncs Kubernetes Node pod CIDRs into NetBox IPAM, as
// either individual Prefixes (one per Node) or a single cluster-wide
// Aggregate per address family, selected by
// spec.resources.podCIDRs.representation.
//
// Both modes watch Node natively: Prefix mode reconciles per-Node like
// NodeReconciler; Aggregate mode recomputes the whole aggregate from every
// currently-known Node on each event, since any Node's pod CIDR changing
// can change the aggregate's bounds.
type PodCIDRReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Store supplies the current HeraldConfig spec and authenticated NetBox
	// client, populated by HeraldConfigReconciler.
	Store *config.Store
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile syncs pod CIDRs to NetBox based on spec.resources.podCIDRs.
func (r *PodCIDRReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	spec, netboxClient := r.Store.Get()
	if spec == nil || netboxClient == nil {
		// NetBox isn't reachable yet. This controller only otherwise wakes
		// on Node changes and real HeraldConfig spec changes (see the
		// GenerationChangedPredicate in SetupWithManager), so poll until the
		// Store is populated instead of waiting for one.
		return ctrl.Result{RequeueAfter: storeNotReadyPollInterval}, nil
	}

	managedTagSlug := resolveManagedTagSlug(spec)
	resyncInterval := resolveResyncInterval(spec)

	if spec.Resources.PodCIDRs.Representation == heraldv1alpha1.PodCIDRRepresentationAggregate {
		return r.reconcileAggregates(ctx, netboxClient, managedTagSlug, resyncInterval, spec)
	}
	return r.reconcilePrefixForNode(ctx, req, netboxClient, managedTagSlug, resyncInterval, spec)
}

// reconcilePrefixForNode syncs the single Node named in req as zero or more
// NetBox Prefixes (Prefix representation mode, the default).
func (r *PodCIDRReconciler) reconcilePrefixForNode(ctx context.Context, req ctrl.Request, netboxClient *netboxclient.NetBoxAPI, managedTagSlug string, resyncInterval time.Duration, spec *heraldv1alpha1.HeraldConfigSpec) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Representation may have just switched from Aggregate to Prefix;
	// clean up any Aggregate left over from that mode so it doesn't linger
	// forever (reconcileAggregates never runs again once Prefix is active).
	if err := deleteStalePodCIDRAggregates(ctx, netboxClient, managedTagSlug); err != nil {
		log.Error(err, "failed to remove stale pod CIDR aggregates after switching to Prefix representation")
		return ctrl.Result{}, r.syncStatus(ctx, managedTagSlug, spec, err)
	}

	var node corev1.Node
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		if apierrors.IsNotFound(err) {
			if delErr := netbox.DeleteNodePodCIDRPrefixes(ctx, netboxClient, managedTagSlug, req.Name); delErr != nil {
				log.Error(delErr, "failed to remove NetBox prefixes for a deleted node", "node", req.Name)
				return ctrl.Result{}, r.syncStatus(ctx, managedTagSlug, spec, delErr)
			}
			return ctrl.Result{}, r.syncStatus(ctx, managedTagSlug, spec, nil)
		}
		return ctrl.Result{}, err
	}

	qualifies := spec.Resources.PodCIDRs.Enabled && len(node.Spec.PodCIDRs) > 0

	if !qualifies {
		// Either pod CIDR sync is off (honor retainOnDisable), or this
		// particular Node just has no pod CIDR assigned yet — always clean
		// up in that second case, since there's nothing to "pause" for an
		// object that no longer matches what Herald syncs at all.
		if spec.Resources.PodCIDRs.Enabled || !spec.Resources.PodCIDRs.RetainOnDisable {
			if err := netbox.DeleteNodePodCIDRPrefixes(ctx, netboxClient, managedTagSlug, node.Name); err != nil {
				log.Error(err, "failed to remove NetBox prefixes for a node no longer synced", "node", node.Name)
				return ctrl.Result{}, r.syncStatus(ctx, managedTagSlug, spec, err)
			}
		}
		if err := r.syncStatus(ctx, managedTagSlug, spec, nil); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: resyncInterval}, nil
	}

	if err := r.syncNodePrefixes(ctx, netboxClient, managedTagSlug, &node); err != nil {
		log.Error(err, "failed to sync pod CIDR to NetBox", "node", node.Name)
		return ctrl.Result{}, r.syncStatus(ctx, managedTagSlug, spec, err)
	}

	log.V(1).Info("synced pod CIDR to NetBox", "node", node.Name)
	if err := r.syncStatus(ctx, managedTagSlug, spec, nil); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

// syncNodePrefixes ensures every one of node's current pod CIDRs has a
// matching NetBox Prefix, then prunes any previously-synced Prefix no
// longer among them.
func (r *PodCIDRReconciler) syncNodePrefixes(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTagSlug string, node *corev1.Node) error {
	managedTag, err := netbox.EnsureManagedTag(ctx, netboxClient, managedTagSlug)
	if err != nil {
		return err
	}

	description := fmt.Sprintf("Kubernetes Node %s pod CIDR", node.Name)

	var desiredExternalIDs []string
	for _, cidr := range node.Spec.PodCIDRs {
		externalID := netbox.ExternalIDForNodePodCIDR(node.Name, cidr)
		desiredExternalIDs = append(desiredExternalIDs, externalID)

		if _, err := netbox.EnsurePodCIDRPrefix(ctx, netboxClient, netbox.PodCIDRPrefixSpec{
			CIDR:        cidr,
			Description: description,
			ExternalID:  externalID,
			ManagedTag:  managedTag,
		}); err != nil {
			return err
		}
	}

	owned, err := netbox.ListNodePodCIDRPrefixes(ctx, netboxClient, managedTagSlug, node.Name)
	if err != nil {
		return err
	}
	for _, p := range owned {
		id, _ := p.CustomFields.(map[string]any)[netbox.ExternalIDFieldName].(string)
		if slices.Contains(desiredExternalIDs, id) {
			continue
		}
		if err := netbox.DeletePodCIDRPrefix(ctx, netboxClient, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// reconcileAggregates recomputes the single cluster-wide Aggregate (per
// address family) from every currently-known Node's pod CIDRs.
func (r *PodCIDRReconciler) reconcileAggregates(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTagSlug string, resyncInterval time.Duration, spec *heraldv1alpha1.HeraldConfigSpec) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if err := r.syncAggregates(ctx, netboxClient, managedTagSlug, spec); err != nil {
		log.Error(err, "failed to sync pod CIDR aggregates to NetBox")
		return ctrl.Result{}, r.syncStatus(ctx, managedTagSlug, spec, err)
	}

	log.V(1).Info("synced pod CIDR aggregates to NetBox")
	if err := r.syncStatus(ctx, managedTagSlug, spec, nil); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

func (r *PodCIDRReconciler) syncAggregates(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTagSlug string, spec *heraldv1alpha1.HeraldConfigSpec) error {
	if !spec.Resources.PodCIDRs.Enabled {
		if spec.Resources.PodCIDRs.RetainOnDisable {
			return nil
		}
		for _, family := range podCIDRAddressFamilies {
			if err := netbox.DeletePodCIDRAggregate(ctx, netboxClient, managedTagSlug, netbox.ExternalIDForPodCIDRAggregate(family)); err != nil {
				return err
			}
		}
		return nil
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}

	// Representation may have just switched from Prefix to Aggregate;
	// clean up any per-node Prefixes left over from that mode so they don't
	// linger forever (reconcilePrefixForNode never runs again once
	// Aggregate is active).
	for _, node := range nodes.Items {
		if err := netbox.DeleteNodePodCIDRPrefixes(ctx, netboxClient, managedTagSlug, node.Name); err != nil {
			return err
		}
	}

	var allCIDRs []string
	for _, node := range nodes.Items {
		allCIDRs = append(allCIDRs, node.Spec.PodCIDRs...)
	}

	supernets, err := computeSupernets(allCIDRs)
	if err != nil {
		return err
	}

	managedTag, err := netbox.EnsureManagedTag(ctx, netboxClient, managedTagSlug)
	if err != nil {
		return err
	}

	for _, family := range podCIDRAddressFamilies {
		externalID := netbox.ExternalIDForPodCIDRAggregate(family)

		cidr, ok := supernets[family]
		if !ok {
			// No Node currently has a pod CIDR in this family; clean up any
			// stale Aggregate from when one previously existed.
			if err := netbox.DeletePodCIDRAggregate(ctx, netboxClient, managedTagSlug, externalID); err != nil {
				return err
			}
			continue
		}

		if _, err := netbox.EnsurePodCIDRAggregate(ctx, netboxClient, netbox.PodCIDRAggregateSpec{
			CIDR:        cidr,
			RIRName:     spec.Resources.PodCIDRs.RIRName,
			Description: fmt.Sprintf("Kubernetes pod CIDR aggregate (%s)", family),
			ExternalID:  externalID,
			ManagedTag:  managedTag,
		}); err != nil {
			return err
		}
	}
	return nil
}

// deleteStalePodCIDRAggregates deletes any Aggregate (either address
// family) left over from a previous Aggregate representation, so switching
// back to Prefix mode doesn't leave them behind indefinitely.
func deleteStalePodCIDRAggregates(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTagSlug string) error {
	for _, family := range podCIDRAddressFamilies {
		if err := netbox.DeletePodCIDRAggregate(ctx, netboxClient, managedTagSlug, netbox.ExternalIDForPodCIDRAggregate(family)); err != nil {
			return err
		}
	}
	return nil
}

// computeSupernets returns, for each address family present in cidrs, the
// smallest CIDR block containing every one of that family's input CIDRs in
// full (not just their network addresses).
func computeSupernets(cidrs []string) (map[string]string, error) {
	type bounds struct {
		bits     int
		min, max *big.Int
	}
	families := make(map[string]*bounds)

	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			return nil, fmt.Errorf("invalid pod CIDR %q: %w", c, err)
		}

		bitLen := len(ipnet.IP) * 8
		family := familyIPv4
		if bitLen == 128 {
			family = familyIPv6
		}

		start := new(big.Int).SetBytes(ipnet.IP)
		ones, _ := ipnet.Mask.Size()
		size := new(big.Int).Lsh(big.NewInt(1), uint(bitLen-ones))
		end := new(big.Int).Sub(new(big.Int).Add(start, size), big.NewInt(1))

		if b, ok := families[family]; ok {
			if start.Cmp(b.min) < 0 {
				b.min = start
			}
			if end.Cmp(b.max) > 0 {
				b.max = end
			}
		} else {
			families[family] = &bounds{bits: bitLen, min: start, max: end}
		}
	}

	result := make(map[string]string, len(families))
	for family, b := range families {
		// The smallest prefix enclosing [min, max] is found by masking off
		// every bit at and after the first bit where min and max differ.
		diff := new(big.Int).Xor(b.min, b.max)
		prefixLen := max(b.bits-diff.BitLen(), 0)

		mask := net.CIDRMask(prefixLen, b.bits)
		networkBytes := make([]byte, b.bits/8)
		b.min.FillBytes(networkBytes)
		network := net.IP(networkBytes).Mask(mask)

		result[family] = (&net.IPNet{IP: network, Mask: mask}).String()
	}
	return result, nil
}

// syncStatus recomputes status.resources.podCIDRs on the HeraldConfig
// singleton after a sync attempt. lastError from the most recent attempt is
// recorded; a nil syncErr clears any previously reported error.
func (r *PodCIDRReconciler) syncStatus(ctx context.Context, managedTagSlug string, spec *heraldv1alpha1.HeraldConfigSpec, syncErr error) error {
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
		Enabled:            spec.Resources.PodCIDRs.Enabled,
		ObservedGeneration: cr.Generation,
	}

	var count int64
	var countErr error
	if spec.Resources.PodCIDRs.Representation == heraldv1alpha1.PodCIDRRepresentationAggregate {
		count, countErr = netbox.CountManagedAggregates(ctx, netboxClient, managedTagSlug)
	} else {
		count, countErr = netbox.CountManagedPrefixes(ctx, netboxClient, managedTagSlug)
	}
	if countErr == nil {
		status.ObjectCount = int32(count)
	}

	if syncErr != nil {
		status.LastError = syncErr.Error()
	} else {
		now := metav1.Now()
		status.LastSyncTime = &now
	}

	cr.Status.Resources.PodCIDRs = status
	return r.Status().Update(ctx, &cr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodCIDRReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Watches(
			&heraldv1alpha1.HeraldConfig{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllNodes),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Named("podcidr").
		Complete(r)
}

// enqueueAllNodes lists every Node and enqueues a reconcile request for each
// one, so that changes to HeraldConfig (enabling/disabling pod CIDR sync,
// switching representation, etc.) are applied immediately rather than
// waiting for the next native Node event.
func (r *PodCIDRReconciler) enqueueAllNodes(ctx context.Context, _ client.Object) []ctrl.Request {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list nodes for HeraldConfig change")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
	}
	return requests
}
