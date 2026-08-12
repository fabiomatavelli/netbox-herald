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

package netbox

import (
	"context"
	"fmt"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	"github.com/fbreckle/go-netbox/netbox/client/dcim"
	"github.com/fbreckle/go-netbox/netbox/client/virtualization"
	"github.com/fbreckle/go-netbox/netbox/models"
)

// DeviceNodeSpec describes the desired NetBox Device representing a
// bare-metal Kubernetes Node.
type DeviceNodeSpec struct {
	// Name is the Device object's name in NetBox (the Node's name).
	Name string

	// DeviceRoleName is the name of a pre-existing NetBox Device Role.
	DeviceRoleName string

	// DeviceTypeName is the model name of a pre-existing NetBox Device Type.
	DeviceTypeName string

	// SiteName is the name of a pre-existing NetBox Site.
	SiteName string

	// ExternalID is the stable identity used to find this Device again
	// across renames (the Node's UID).
	ExternalID string

	// ManagedTag is the tag applied to the Device, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureDevice creates or updates the NetBox Device representing a
// Kubernetes Node, found by ExternalID rather than name so that renaming the
// Node updates the existing object instead of creating a duplicate. Returns
// the NetBox Device's ID.
func EnsureDevice(ctx context.Context, client *netboxclient.NetBoxAPI, spec DeviceNodeSpec) (int64, error) {
	existing, err := findDeviceByExternalID(ctx, client, *spec.ManagedTag.Slug, spec.ExternalID)
	if err != nil {
		return 0, err
	}

	deviceTypeID, err := LookupDeviceTypeID(ctx, client, spec.DeviceTypeName)
	if err != nil {
		return 0, err
	}
	roleID, err := LookupDeviceRoleID(ctx, client, spec.DeviceRoleName)
	if err != nil {
		return 0, err
	}
	siteID, err := LookupSiteID(ctx, client, spec.SiteName)
	if err != nil {
		return 0, err
	}

	data := &models.WritableDeviceWithConfigContext{
		Name:       &spec.Name,
		DeviceType: &deviceTypeID,
		Role:       &roleID,
		Site:       &siteID,
		Tags:       []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
		CustomFields: map[string]any{
			ExternalIDFieldName: spec.ExternalID,
		},
	}

	if existing != nil {
		updateResp, err := client.Dcim.DcimDevicesUpdate(
			dcim.NewDcimDevicesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox device %q (id %d): %w", spec.Name, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Dcim.DcimDevicesCreate(
		dcim.NewDcimDevicesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox device %q: %w", spec.Name, err)
	}
	return createResp.Payload.ID, nil
}

// DeleteDeviceByExternalID deletes the NetBox Device identified by
// externalID, if one managed by the tag identified by managedTagSlug
// exists. It's a no-op if no matching Device is found.
func DeleteDeviceByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) error {
	existing, err := findDeviceByExternalID(ctx, client, managedTagSlug, externalID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	return deleteDevice(ctx, client, existing.ID)
}

// DeleteDeviceByName deletes the NetBox Device with the given name, if one
// managed by the tag identified by managedTagSlug exists. Used when the
// owning Kubernetes Node has already been removed, so its UID (and thus the
// external-ID lookup used elsewhere) is no longer available. It's a no-op if
// no matching Device is found.
func DeleteDeviceByName(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, name string) error {
	listResp, err := client.Dcim.DcimDevicesList(
		dcim.NewDcimDevicesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithName(&name),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox devices: %w", err)
	}
	for _, device := range listResp.Payload.Results {
		if device.Name != nil && *device.Name == name {
			return deleteDevice(ctx, client, device.ID)
		}
	}
	return nil
}

func deleteDevice(ctx context.Context, client *netboxclient.NetBoxAPI, id int64) error {
	_, err := client.Dcim.DcimDevicesDelete(
		dcim.NewDcimDevicesDeleteParams().WithContext(ctx).WithID(id),
		nil,
	)
	if err != nil {
		return fmt.Errorf("deleting NetBox device (id %d): %w", id, err)
	}
	return nil
}

// CountManagedDevices returns the number of NetBox Devices carrying the tag
// identified by managedTagSlug.
func CountManagedDevices(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string) (int64, error) {
	limit := int64(1)
	resp, err := client.Dcim.DcimDevicesList(
		dcim.NewDcimDevicesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithLimit(&limit),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox devices: %w", err)
	}
	if resp.Payload.Count == nil {
		return 0, nil
	}
	return *resp.Payload.Count, nil
}

// CountManagedVirtualMachines returns the number of NetBox VirtualMachines
// carrying the tag identified by managedTagSlug.
func CountManagedVirtualMachines(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string) (int64, error) {
	limit := int64(1)
	resp, err := client.Virtualization.VirtualizationVirtualMachinesList(
		virtualization.NewVirtualizationVirtualMachinesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithLimit(&limit),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox virtual machines: %w", err)
	}
	if resp.Payload.Count == nil {
		return 0, nil
	}
	return *resp.Payload.Count, nil
}

func findDeviceByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) (*models.DeviceWithConfigContext, error) {
	listResp, err := client.Dcim.DcimDevicesList(
		dcim.NewDcimDevicesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox devices: %w", err)
	}
	for _, device := range listResp.Payload.Results {
		if id, ok := readExternalID(device.CustomFields); ok && id == externalID {
			return device, nil
		}
	}
	return nil, nil
}

// VirtualMachineNodeSpec describes the desired NetBox VirtualMachine
// representing a VM-backed Kubernetes Node.
type VirtualMachineNodeSpec struct {
	// Name is the VirtualMachine object's name in NetBox (the Node's name).
	Name string

	// ClusterName is the name of a pre-existing NetBox Cluster.
	ClusterName string

	// PlatformName is the name of a pre-existing NetBox Platform. Optional;
	// leave empty to not assign a platform.
	PlatformName string

	// ExternalID is the stable identity used to find this VirtualMachine
	// again across renames (the Node's UID).
	ExternalID string

	// ManagedTag is the tag applied to the VirtualMachine, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureVirtualMachine creates or updates the NetBox VirtualMachine
// representing a Kubernetes Node, found by ExternalID rather than name so
// that renaming the Node updates the existing object instead of creating a
// duplicate. Returns the NetBox VirtualMachine's ID.
func EnsureVirtualMachine(ctx context.Context, client *netboxclient.NetBoxAPI, spec VirtualMachineNodeSpec) (int64, error) {
	existing, err := findVirtualMachineByExternalID(ctx, client, *spec.ManagedTag.Slug, spec.ExternalID)
	if err != nil {
		return 0, err
	}

	clusterID, err := LookupClusterID(ctx, client, spec.ClusterName)
	if err != nil {
		return 0, err
	}

	data := &models.WritableVirtualMachineWithConfigContext{
		Name:    &spec.Name,
		Cluster: &clusterID,
		Tags:    []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
		CustomFields: map[string]any{
			ExternalIDFieldName: spec.ExternalID,
		},
	}

	if spec.PlatformName != "" {
		platformID, err := LookupPlatformID(ctx, client, spec.PlatformName)
		if err != nil {
			return 0, err
		}
		data.Platform = &platformID
	}

	if existing != nil {
		updateResp, err := client.Virtualization.VirtualizationVirtualMachinesUpdate(
			virtualization.NewVirtualizationVirtualMachinesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox virtual machine %q (id %d): %w", spec.Name, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Virtualization.VirtualizationVirtualMachinesCreate(
		virtualization.NewVirtualizationVirtualMachinesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox virtual machine %q: %w", spec.Name, err)
	}
	return createResp.Payload.ID, nil
}

// DeleteVirtualMachineByExternalID deletes the NetBox VirtualMachine
// identified by externalID, if one managed by the tag identified by
// managedTagSlug exists. It's a no-op if no matching VirtualMachine is
// found.
func DeleteVirtualMachineByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) error {
	existing, err := findVirtualMachineByExternalID(ctx, client, managedTagSlug, externalID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	return deleteVirtualMachine(ctx, client, existing.ID)
}

// DeleteVirtualMachineByName deletes the NetBox VirtualMachine with the
// given name, if one managed by the tag identified by managedTagSlug
// exists. Used when the owning Kubernetes Node has already been removed, so
// its UID (and thus the external-ID lookup used elsewhere) is no longer
// available. It's a no-op if no matching VirtualMachine is found.
func DeleteVirtualMachineByName(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, name string) error {
	listResp, err := client.Virtualization.VirtualizationVirtualMachinesList(
		virtualization.NewVirtualizationVirtualMachinesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithName(&name),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox virtual machines: %w", err)
	}
	for _, vm := range listResp.Payload.Results {
		if vm.Name != nil && *vm.Name == name {
			return deleteVirtualMachine(ctx, client, vm.ID)
		}
	}
	return nil
}

func deleteVirtualMachine(ctx context.Context, client *netboxclient.NetBoxAPI, id int64) error {
	_, err := client.Virtualization.VirtualizationVirtualMachinesDelete(
		virtualization.NewVirtualizationVirtualMachinesDeleteParams().WithContext(ctx).WithID(id),
		nil,
	)
	if err != nil {
		return fmt.Errorf("deleting NetBox virtual machine (id %d): %w", id, err)
	}
	return nil
}

func findVirtualMachineByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) (*models.VirtualMachineWithConfigContext, error) {
	listResp, err := client.Virtualization.VirtualizationVirtualMachinesList(
		virtualization.NewVirtualizationVirtualMachinesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox virtual machines: %w", err)
	}
	for _, vm := range listResp.Payload.Results {
		if id, ok := readExternalID(vm.CustomFields); ok && id == externalID {
			return vm, nil
		}
	}
	return nil, nil
}
