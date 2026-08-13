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
	"strconv"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	"github.com/fbreckle/go-netbox/netbox/client/dcim"
	"github.com/fbreckle/go-netbox/netbox/client/ipam"
	"github.com/fbreckle/go-netbox/netbox/client/virtualization"
	"github.com/fbreckle/go-netbox/netbox/models"
)

// NetBox content types an IP Address can be assigned to, used as
// WritableIPAddress.AssignedObjectType.
const (
	AssignedObjectTypeDeviceInterface = "dcim.interface"
	AssignedObjectTypeVMInterface     = "virtualization.vminterface"
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

	// PrimaryIPv4ID, if non-nil, is the NetBox ID of the IP Address to set
	// as this Device's primary IPv4 address. Left nil until the address and
	// its owning Interface have been created.
	PrimaryIPv4ID *int64

	// PrimaryIPv6ID mirrors PrimaryIPv4ID for the primary IPv6 address.
	PrimaryIPv6ID *int64
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
		PrimaryIp4: spec.PrimaryIPv4ID,
		PrimaryIp6: spec.PrimaryIPv6ID,
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
	if err := DeleteDeviceInterfaces(ctx, client, managedTagSlug, existing.ID); err != nil {
		return err
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
			if err := DeleteDeviceInterfaces(ctx, client, managedTagSlug, device.ID); err != nil {
				return err
			}
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

	// PrimaryIPv4ID, if non-nil, is the NetBox ID of the IP Address to set
	// as this VirtualMachine's primary IPv4 address. Left nil until the
	// address and its owning VMInterface have been created.
	PrimaryIPv4ID *int64

	// PrimaryIPv6ID mirrors PrimaryIPv4ID for the primary IPv6 address.
	PrimaryIPv6ID *int64
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
		PrimaryIp4: spec.PrimaryIPv4ID,
		PrimaryIp6: spec.PrimaryIPv6ID,
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
	if err := DeleteVMInterfaces(ctx, client, managedTagSlug, existing.ID); err != nil {
		return err
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
			if err := DeleteVMInterfaces(ctx, client, managedTagSlug, vm.ID); err != nil {
				return err
			}
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

// DeviceInterfaceSpec describes the desired NetBox Interface representing a
// Device-mapped Kubernetes Node's network interface.
type DeviceInterfaceSpec struct {
	// DeviceID is the NetBox ID of the owning Device.
	DeviceID int64

	// Name is the Interface's name.
	Name string

	// ManagedTag is the tag applied to the Interface, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureDeviceInterface creates or updates the NetBox Interface representing
// a Device-mapped Kubernetes Node's network interface, found by
// (device, name) — NetBox enforces this pair is unique, so no external-ID
// custom field is needed here. Returns the Interface's ID.
func EnsureDeviceInterface(ctx context.Context, client *netboxclient.NetBoxAPI, spec DeviceInterfaceSpec) (int64, error) {
	existing, err := findDeviceInterfaceByName(ctx, client, spec.DeviceID, spec.Name)
	if err != nil {
		return 0, err
	}

	interfaceType := "virtual"
	data := &models.WritableInterface{
		Device: &spec.DeviceID,
		Name:   &spec.Name,
		Type:   &interfaceType,
		Tags:   []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
	}

	if existing != nil {
		updateResp, err := client.Dcim.DcimInterfacesUpdate(
			dcim.NewDcimInterfacesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox interface %q (id %d): %w", spec.Name, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Dcim.DcimInterfacesCreate(
		dcim.NewDcimInterfacesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox interface %q: %w", spec.Name, err)
	}
	return createResp.Payload.ID, nil
}

// DeleteDeviceInterfaces deletes every NetBox IP Address assigned to any
// Interface on the Device identified by deviceID and carrying the tag
// identified by managedTagSlug, then deletes the Interface(s) themselves.
// Called before deleting the parent Device so cleanup does not depend on
// NetBox cascading Interface/IP Address deletion; harmless no-op if it
// already did.
func DeleteDeviceInterfaces(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string, deviceID int64) error {
	deviceIDStr := strconv.FormatInt(deviceID, 10)

	ipListResp, err := client.Ipam.IpamIPAddressesList(
		ipam.NewIpamIPAddressesListParams().WithContext(ctx).WithDeviceID(&deviceIDStr).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox IP addresses for device (id %d): %w", deviceID, err)
	}
	for _, addr := range ipListResp.Payload.Results {
		if err := deleteIPAddress(ctx, client, addr.ID); err != nil {
			return err
		}
	}

	ifaceListResp, err := client.Dcim.DcimInterfacesList(
		dcim.NewDcimInterfacesListParams().WithContext(ctx).WithDeviceID(&deviceIDStr).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox interfaces for device (id %d): %w", deviceID, err)
	}
	for _, iface := range ifaceListResp.Payload.Results {
		if _, err := client.Dcim.DcimInterfacesDelete(
			dcim.NewDcimInterfacesDeleteParams().WithContext(ctx).WithID(iface.ID),
			nil,
		); err != nil {
			return fmt.Errorf("deleting NetBox interface (id %d): %w", iface.ID, err)
		}
	}

	return nil
}

func findDeviceInterfaceByName(ctx context.Context, client *netboxclient.NetBoxAPI, deviceID int64, name string) (*models.Interface, error) {
	deviceIDStr := strconv.FormatInt(deviceID, 10)
	listResp, err := client.Dcim.DcimInterfacesList(
		dcim.NewDcimInterfacesListParams().WithContext(ctx).WithDeviceID(&deviceIDStr).WithName(&name),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox interfaces: %w", err)
	}
	for _, iface := range listResp.Payload.Results {
		if iface.Name != nil && *iface.Name == name {
			return iface, nil
		}
	}
	return nil, nil
}

// VMInterfaceSpec describes the desired NetBox VMInterface representing a
// VirtualMachine-mapped Kubernetes Node's network interface.
type VMInterfaceSpec struct {
	// VirtualMachineID is the NetBox ID of the owning VirtualMachine.
	VirtualMachineID int64

	// Name is the VMInterface's name.
	Name string

	// ManagedTag is the tag applied to the VMInterface, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureVMInterface creates or updates the NetBox VMInterface representing a
// VirtualMachine-mapped Kubernetes Node's network interface, found by
// (virtual_machine, name) — NetBox enforces this pair is unique, so no
// external-ID custom field is needed here. Returns the VMInterface's ID.
func EnsureVMInterface(ctx context.Context, client *netboxclient.NetBoxAPI, spec VMInterfaceSpec) (int64, error) {
	existing, err := findVMInterfaceByName(ctx, client, spec.VirtualMachineID, spec.Name)
	if err != nil {
		return 0, err
	}

	data := &models.WritableVMInterface{
		VirtualMachine: &spec.VirtualMachineID,
		Name:           &spec.Name,
		Tags:           []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
	}

	if existing != nil {
		updateResp, err := client.Virtualization.VirtualizationInterfacesUpdate(
			virtualization.NewVirtualizationInterfacesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox VM interface %q (id %d): %w", spec.Name, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Virtualization.VirtualizationInterfacesCreate(
		virtualization.NewVirtualizationInterfacesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox VM interface %q: %w", spec.Name, err)
	}
	return createResp.Payload.ID, nil
}

// DeleteVMInterfaces deletes every NetBox IP Address assigned to any
// VMInterface on the VirtualMachine identified by virtualMachineID and
// carrying the tag identified by managedTagSlug, then deletes the
// VMInterface(s) themselves. Called before deleting the parent
// VirtualMachine so cleanup does not depend on NetBox cascading
// VMInterface/IP Address deletion; harmless no-op if it already did.
func DeleteVMInterfaces(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string, virtualMachineID int64) error {
	vmIDStr := strconv.FormatInt(virtualMachineID, 10)

	ipListResp, err := client.Ipam.IpamIPAddressesList(
		ipam.NewIpamIPAddressesListParams().WithContext(ctx).WithVirtualMachineID(&vmIDStr).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox IP addresses for virtual machine (id %d): %w", virtualMachineID, err)
	}
	for _, addr := range ipListResp.Payload.Results {
		if err := deleteIPAddress(ctx, client, addr.ID); err != nil {
			return err
		}
	}

	ifaceListResp, err := client.Virtualization.VirtualizationInterfacesList(
		virtualization.NewVirtualizationInterfacesListParams().WithContext(ctx).WithVirtualMachineID(&vmIDStr).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox VM interfaces for virtual machine (id %d): %w", virtualMachineID, err)
	}
	for _, iface := range ifaceListResp.Payload.Results {
		if _, err := client.Virtualization.VirtualizationInterfacesDelete(
			virtualization.NewVirtualizationInterfacesDeleteParams().WithContext(ctx).WithID(iface.ID),
			nil,
		); err != nil {
			return fmt.Errorf("deleting NetBox VM interface (id %d): %w", iface.ID, err)
		}
	}

	return nil
}

func findVMInterfaceByName(ctx context.Context, client *netboxclient.NetBoxAPI, virtualMachineID int64, name string) (*models.VMInterface, error) {
	vmIDStr := strconv.FormatInt(virtualMachineID, 10)
	listResp, err := client.Virtualization.VirtualizationInterfacesList(
		virtualization.NewVirtualizationInterfacesListParams().WithContext(ctx).WithVirtualMachineID(&vmIDStr).WithName(&name),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox VM interfaces: %w", err)
	}
	for _, iface := range listResp.Payload.Results {
		if iface.Name != nil && *iface.Name == name {
			return iface, nil
		}
	}
	return nil, nil
}

// NodeIPSpec describes the desired NetBox IP Address representing one of a
// Kubernetes Node's addresses, assigned to its Device's Interface or
// VirtualMachine's VMInterface.
type NodeIPSpec struct {
	// Address is the Node's address in CIDR notation (e.g. "10.0.0.5/32").
	// The exact prefix length of the containing subnet isn't knowable from a
	// bare Node address, so a host mask is always used.
	Address string

	// AssignedObjectType is the NetBox content type of the parent the IP
	// Address is assigned to: AssignedObjectTypeDeviceInterface or
	// AssignedObjectTypeVMInterface.
	AssignedObjectType string

	// AssignedObjectID is the NetBox ID of the parent Interface/VMInterface
	// (from EnsureDeviceInterface/EnsureVMInterface).
	AssignedObjectID int64

	// Description records which Kubernetes Node this IP belongs to.
	Description string

	// ManagedTag is the tag applied to the IP Address, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureNodeIP creates or updates the NetBox IP Address for one of a synced
// Node's addresses, found by (assigned interface, address) — the interface
// itself is already found idempotently by name, so "the address currently
// assigned to it" is a sufficient, simpler identity here than an
// external-ID custom field. Returns the IP Address's ID.
func EnsureNodeIP(ctx context.Context, client *netboxclient.NetBoxAPI, spec NodeIPSpec) (int64, error) {
	existing, err := findNodeIP(ctx, client, spec.AssignedObjectType, spec.AssignedObjectID, spec.Address)
	if err != nil {
		return 0, err
	}

	data := &models.WritableIPAddress{
		Address:            &spec.Address,
		AssignedObjectType: &spec.AssignedObjectType,
		AssignedObjectID:   &spec.AssignedObjectID,
		Description:        spec.Description,
		Tags:               []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
	}

	if existing != nil {
		updateResp, err := client.Ipam.IpamIPAddressesUpdate(
			ipam.NewIpamIPAddressesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox IP address %q (id %d): %w", spec.Address, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Ipam.IpamIPAddressesCreate(
		ipam.NewIpamIPAddressesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox IP address %q: %w", spec.Address, err)
	}
	return createResp.Payload.ID, nil
}

func findNodeIP(ctx context.Context, client *netboxclient.NetBoxAPI, assignedObjectType string, assignedObjectID int64, address string) (*models.IPAddress, error) {
	idStr := strconv.FormatInt(assignedObjectID, 10)
	params := ipam.NewIpamIPAddressesListParams().WithContext(ctx)
	switch assignedObjectType {
	case AssignedObjectTypeDeviceInterface:
		params = params.WithInterfaceID(&idStr)
	case AssignedObjectTypeVMInterface:
		params = params.WithVminterfaceID(&idStr)
	default:
		return nil, fmt.Errorf("unsupported assigned object type %q", assignedObjectType)
	}

	listResp, err := client.Ipam.IpamIPAddressesList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox IP addresses: %w", err)
	}
	for _, addr := range listResp.Payload.Results {
		if addr.Address != nil && *addr.Address == address {
			return addr, nil
		}
	}
	return nil, nil
}
