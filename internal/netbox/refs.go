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
)

// netbox-herald never creates reference/taxonomy objects (Sites, Cluster
// Types, Cluster Groups, Device Roles, Device Types, Platforms) — they must
// already exist, and lookups fail clearly by name when they don't.

// LookupSiteID resolves a NetBox Site's ID by name.
func LookupSiteID(ctx context.Context, client *netboxclient.NetBoxAPI, name string) (int64, error) {
	resp, err := client.Dcim.DcimSitesList(
		dcim.NewDcimSitesListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox sites: %w", err)
	}
	for _, site := range resp.Payload.Results {
		if site.Name != nil && *site.Name == name {
			return site.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox site %q not found; it must be created in NetBox before it can be referenced", name)
}

// LookupClusterTypeID resolves a NetBox Cluster Type's ID by name.
func LookupClusterTypeID(ctx context.Context, client *netboxclient.NetBoxAPI, name string) (int64, error) {
	resp, err := client.Virtualization.VirtualizationClusterTypesList(
		virtualization.NewVirtualizationClusterTypesListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox cluster types: %w", err)
	}
	for _, ct := range resp.Payload.Results {
		if ct.Name != nil && *ct.Name == name {
			return ct.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox cluster type %q not found; it must be created in NetBox before it can be referenced", name)
}

// LookupClusterGroupID resolves a NetBox Cluster Group's ID by name.
func LookupClusterGroupID(ctx context.Context, client *netboxclient.NetBoxAPI, name string) (int64, error) {
	resp, err := client.Virtualization.VirtualizationClusterGroupsList(
		virtualization.NewVirtualizationClusterGroupsListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox cluster groups: %w", err)
	}
	for _, cg := range resp.Payload.Results {
		if cg.Name != nil && *cg.Name == name {
			return cg.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox cluster group %q not found; it must be created in NetBox before it can be referenced", name)
}

// LookupClusterID resolves a NetBox Cluster's ID by name. Used to attach
// synced VirtualMachines to the Cluster Herald manages (spec.resources.
// cluster.name), which must already have been synced by the cluster
// resource controller.
func LookupClusterID(ctx context.Context, client *netboxclient.NetBoxAPI, name string) (int64, error) {
	resp, err := client.Virtualization.VirtualizationClustersList(
		virtualization.NewVirtualizationClustersListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox clusters: %w", err)
	}
	for _, cluster := range resp.Payload.Results {
		if cluster.Name != nil && *cluster.Name == name {
			return cluster.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox cluster %q not found; enable and sync spec.resources.cluster first, or point virtualMachine.clusterName at an existing Cluster", name)
}

// LookupDeviceRoleID resolves a NetBox Device Role's ID by name.
func LookupDeviceRoleID(ctx context.Context, client *netboxclient.NetBoxAPI, name string) (int64, error) {
	resp, err := client.Dcim.DcimDeviceRolesList(
		dcim.NewDcimDeviceRolesListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox device roles: %w", err)
	}
	for _, role := range resp.Payload.Results {
		if role.Name != nil && *role.Name == name {
			return role.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox device role %q not found; it must be created in NetBox before it can be referenced", name)
}

// LookupDeviceTypeID resolves a NetBox Device Type's ID by its model name.
func LookupDeviceTypeID(ctx context.Context, client *netboxclient.NetBoxAPI, model string) (int64, error) {
	resp, err := client.Dcim.DcimDeviceTypesList(
		dcim.NewDcimDeviceTypesListParams().WithContext(ctx).WithModel(&model),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox device types: %w", err)
	}
	for _, deviceType := range resp.Payload.Results {
		if deviceType.Model != nil && *deviceType.Model == model {
			return deviceType.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox device type %q not found; it must be created in NetBox before it can be referenced", model)
}

// LookupPlatformID resolves a NetBox Platform's ID by name.
func LookupPlatformID(ctx context.Context, client *netboxclient.NetBoxAPI, name string) (int64, error) {
	resp, err := client.Dcim.DcimPlatformsList(
		dcim.NewDcimPlatformsListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox platforms: %w", err)
	}
	for _, platform := range resp.Payload.Results {
		if platform.Name != nil && *platform.Name == name {
			return platform.ID, nil
		}
	}
	return 0, fmt.Errorf("NetBox platform %q not found; it must be created in NetBox before it can be referenced", name)
}
