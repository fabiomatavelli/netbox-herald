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
	"strings"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	"github.com/fbreckle/go-netbox/netbox/models"
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

// NodeReconciler syncs Kubernetes Nodes into NetBox, as either DCIM Devices
// (bare-metal clusters) or Virtualization VirtualMachines (VM-backed
// clusters), selected by spec.resources.nodes.mapping.
//
// Each Node's stable external ID is its own UID. When a Node is removed
// from the cluster, Reconcile no longer has that UID available (the object
// is gone), so cleanup falls back to finding the managed NetBox object by
// name instead — this always happens, independent of retainOnDisable, since
// there's no live Node left for a "paused" sync to apply to.
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Store supplies the current HeraldConfig spec and authenticated NetBox
	// client, populated by HeraldConfigReconciler.
	Store *config.Store
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile creates, updates, or deletes the NetBox Device or VirtualMachine
// representing a single Kubernetes Node, based on spec.resources.nodes.
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

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

	var node corev1.Node
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		if apierrors.IsNotFound(err) {
			if delErr := r.deleteNodeByName(ctx, netboxClient, managedTagSlug, req.Name); delErr != nil {
				log.Error(delErr, "failed to remove NetBox object for a deleted node", "node", req.Name)
				return ctrl.Result{}, r.syncNodesStatus(ctx, managedTagSlug, spec, delErr)
			}
			return ctrl.Result{}, r.syncNodesStatus(ctx, managedTagSlug, spec, nil)
		}
		return ctrl.Result{}, err
	}

	externalID := string(node.UID)

	if !spec.Resources.Nodes.Enabled {
		if !spec.Resources.Nodes.RetainOnDisable {
			if err := r.deleteNodeByExternalID(ctx, netboxClient, spec, managedTagSlug, externalID); err != nil {
				log.Error(err, "failed to remove NetBox object for a disabled node", "node", node.Name)
				return ctrl.Result{}, r.syncNodesStatus(ctx, managedTagSlug, spec, err)
			}
		}
		if err := r.syncNodesStatus(ctx, managedTagSlug, spec, nil); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: resyncInterval}, nil
	}

	managedTag, err := netbox.EnsureManagedTag(ctx, netboxClient, managedTagSlug)
	if err != nil {
		log.Error(err, "failed to sync node to NetBox", "node", node.Name)
		return ctrl.Result{}, r.syncNodesStatus(ctx, managedTagSlug, spec, err)
	}

	switch spec.Resources.Nodes.Mapping {
	case heraldv1alpha1.NodeMappingDevice:
		err = r.syncDeviceNode(ctx, netboxClient, managedTag, &node, spec, externalID)
	case heraldv1alpha1.NodeMappingVirtualMachine:
		err = r.syncVMNode(ctx, netboxClient, managedTag, &node, spec, externalID)
	default:
		err = fmt.Errorf("spec.resources.nodes.mapping must be %q or %q, got %q",
			heraldv1alpha1.NodeMappingDevice, heraldv1alpha1.NodeMappingVirtualMachine, spec.Resources.Nodes.Mapping)
	}
	if err != nil {
		log.Error(err, "failed to sync node to NetBox", "node", node.Name)
		return ctrl.Result{}, r.syncNodesStatus(ctx, managedTagSlug, spec, err)
	}

	log.V(1).Info("synced node to NetBox", "node", node.Name, "mapping", spec.Resources.Nodes.Mapping)
	if err := r.syncNodesStatus(ctx, managedTagSlug, spec, nil); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

// deleteNodeByExternalID removes the NetBox object for a Node that still
// exists in Kubernetes but is no longer synced (nodes disabled).
func (r *NodeReconciler) deleteNodeByExternalID(ctx context.Context, nbClient *netboxclient.NetBoxAPI, spec *heraldv1alpha1.HeraldConfigSpec, managedTagSlug, externalID string) error {
	if spec.Resources.Nodes.Mapping == heraldv1alpha1.NodeMappingVirtualMachine {
		return netbox.DeleteVirtualMachineByExternalID(ctx, nbClient, managedTagSlug, externalID)
	}
	return netbox.DeleteDeviceByExternalID(ctx, nbClient, managedTagSlug, externalID)
}

// deleteNodeByName removes the NetBox object for a Node that no longer
// exists in Kubernetes at all, so its UID is unavailable. The current
// mapping mode may have changed since the object was created, so both
// Device and VirtualMachine are checked.
func (r *NodeReconciler) deleteNodeByName(ctx context.Context, nbClient *netboxclient.NetBoxAPI, managedTagSlug, name string) error {
	if err := netbox.DeleteDeviceByName(ctx, nbClient, managedTagSlug, name); err != nil {
		return err
	}
	return netbox.DeleteVirtualMachineByName(ctx, nbClient, managedTagSlug, name)
}

// syncDeviceNode ensures the NetBox Device representing node, plus (when the
// Node has an address of the configured type) an Interface and IP Address
// assigned to it, and the Device's primary IPv4/IPv6.
func (r *NodeReconciler) syncDeviceNode(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTag *models.Tag, node *corev1.Node, spec *heraldv1alpha1.HeraldConfigSpec, externalID string) error {
	ipv4CIDR, ipv6CIDR, err := nodePrimaryAddresses(node, spec.Resources.Nodes.AddressType)
	if err != nil {
		return err
	}

	deviceSpec := netbox.DeviceNodeSpec{
		Name:           node.Name,
		DeviceRoleName: spec.Resources.Nodes.Device.DeviceRoleName,
		DeviceTypeName: spec.Resources.Nodes.Device.DeviceTypeName,
		SiteName:       spec.Resources.Nodes.Device.SiteName,
		ExternalID:     externalID,
		ManagedTag:     managedTag,
	}

	deviceID, err := netbox.EnsureDevice(ctx, netboxClient, deviceSpec)
	if err != nil {
		return err
	}

	if ipv4CIDR == "" && ipv6CIDR == "" {
		return nil
	}

	ifaceID, err := netbox.EnsureDeviceInterface(ctx, netboxClient, netbox.DeviceInterfaceSpec{
		DeviceID:   deviceID,
		Name:       spec.Resources.Nodes.InterfaceName,
		ManagedTag: managedTag,
	})
	if err != nil {
		return err
	}

	description := fmt.Sprintf("Kubernetes Node %s", node.Name)
	if deviceSpec.PrimaryIPv4ID, err = ensureNodeIPID(ctx, netboxClient, ipv4CIDR, netbox.AssignedObjectTypeDeviceInterface, ifaceID, description, managedTag); err != nil {
		return err
	}
	if deviceSpec.PrimaryIPv6ID, err = ensureNodeIPID(ctx, netboxClient, ipv6CIDR, netbox.AssignedObjectTypeDeviceInterface, ifaceID, description, managedTag); err != nil {
		return err
	}

	_, err = netbox.EnsureDevice(ctx, netboxClient, deviceSpec)
	return err
}

// syncVMNode ensures the NetBox VirtualMachine representing node, plus (when
// the Node has an address of the configured type) a VMInterface and IP
// Address assigned to it, and the VirtualMachine's primary IPv4/IPv6.
func (r *NodeReconciler) syncVMNode(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, managedTag *models.Tag, node *corev1.Node, spec *heraldv1alpha1.HeraldConfigSpec, externalID string) error {
	ipv4CIDR, ipv6CIDR, err := nodePrimaryAddresses(node, spec.Resources.Nodes.AddressType)
	if err != nil {
		return err
	}

	clusterName := spec.Resources.Nodes.VirtualMachine.ClusterName
	if clusterName == "" {
		clusterName = spec.Resources.Cluster.Name
	}

	vmSpec := netbox.VirtualMachineNodeSpec{
		Name:         node.Name,
		ClusterName:  clusterName,
		PlatformName: spec.Resources.Nodes.VirtualMachine.PlatformName,
		ExternalID:   externalID,
		ManagedTag:   managedTag,
	}

	vmID, err := netbox.EnsureVirtualMachine(ctx, netboxClient, vmSpec)
	if err != nil {
		return err
	}

	if ipv4CIDR == "" && ipv6CIDR == "" {
		return nil
	}

	ifaceID, err := netbox.EnsureVMInterface(ctx, netboxClient, netbox.VMInterfaceSpec{
		VirtualMachineID: vmID,
		Name:             spec.Resources.Nodes.InterfaceName,
		ManagedTag:       managedTag,
	})
	if err != nil {
		return err
	}

	description := fmt.Sprintf("Kubernetes Node %s", node.Name)
	if vmSpec.PrimaryIPv4ID, err = ensureNodeIPID(ctx, netboxClient, ipv4CIDR, netbox.AssignedObjectTypeVMInterface, ifaceID, description, managedTag); err != nil {
		return err
	}
	if vmSpec.PrimaryIPv6ID, err = ensureNodeIPID(ctx, netboxClient, ipv6CIDR, netbox.AssignedObjectTypeVMInterface, ifaceID, description, managedTag); err != nil {
		return err
	}

	_, err = netbox.EnsureVirtualMachine(ctx, netboxClient, vmSpec)
	return err
}

// ensureNodeIPID ensures a NetBox IP Address for cidr assigned to the
// Interface/VMInterface identified by assignedObjectType/assignedObjectID,
// returning its ID. cidr may be empty (the Node has no address of the
// configured type/family), in which case it's a no-op returning nil.
func ensureNodeIPID(ctx context.Context, netboxClient *netboxclient.NetBoxAPI, cidr, assignedObjectType string, assignedObjectID int64, description string, managedTag *models.Tag) (*int64, error) {
	if cidr == "" {
		return nil, nil
	}
	id, err := netbox.EnsureNodeIP(ctx, netboxClient, netbox.NodeIPSpec{
		Address:            cidr,
		AssignedObjectType: assignedObjectType,
		AssignedObjectID:   assignedObjectID,
		Description:        description,
		ManagedTag:         managedTag,
	})
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// nodePrimaryAddresses returns node's first IPv4 and first IPv6 address of
// addressType from node.Status.Addresses, in CIDR host-mask notation (see
// cidrForIP), or "" for either that isn't present — Nodes can be dual-stack,
// carrying more than one address of the same Type.
func nodePrimaryAddresses(node *corev1.Node, addressType heraldv1alpha1.NodeAddressType) (ipv4CIDR, ipv6CIDR string, err error) {
	wantType := corev1.NodeAddressType(addressType)
	for _, addr := range node.Status.Addresses {
		if addr.Type != wantType {
			continue
		}
		cidr, cidrErr := cidrForIP(addr.Address)
		if cidrErr != nil {
			return "", "", fmt.Errorf("node %s: %w", node.Name, cidrErr)
		}
		if strings.HasSuffix(cidr, "/32") {
			if ipv4CIDR == "" {
				ipv4CIDR = cidr
			}
		} else if ipv6CIDR == "" {
			ipv6CIDR = cidr
		}
	}
	return ipv4CIDR, ipv6CIDR, nil
}

// syncNodesStatus recomputes status.resources.nodes on the HeraldConfig
// singleton after this Node's sync attempt. lastError from the most recent
// attempt is recorded; a nil syncErr clears any previously reported error.
func (r *NodeReconciler) syncNodesStatus(ctx context.Context, managedTagSlug string, spec *heraldv1alpha1.HeraldConfigSpec, syncErr error) error {
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
		Enabled:            spec.Resources.Nodes.Enabled,
		ObservedGeneration: cr.Generation,
	}

	var count int64
	var countErr error
	if spec.Resources.Nodes.Mapping == heraldv1alpha1.NodeMappingVirtualMachine {
		count, countErr = netbox.CountManagedVirtualMachines(ctx, netboxClient, managedTagSlug)
	} else {
		count, countErr = netbox.CountManagedDevices(ctx, netboxClient, managedTagSlug)
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

	cr.Status.Resources.Nodes = status
	return r.Status().Update(ctx, &cr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Watches(
			&heraldv1alpha1.HeraldConfig{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllNodes),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Named("node").
		Complete(r)
}

// enqueueAllNodes lists every Node and enqueues a reconcile request for each
// one, so that changes to HeraldConfig (enabling/disabling node sync,
// changing the mapping mode, etc.) are applied to every known Node
// immediately rather than waiting for their next native event.
func (r *NodeReconciler) enqueueAllNodes(ctx context.Context, _ client.Object) []ctrl.Request {
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
