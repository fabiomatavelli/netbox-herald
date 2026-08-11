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
	"github.com/fbreckle/go-netbox/netbox/client/virtualization"
	"github.com/fbreckle/go-netbox/netbox/models"
)

// ClusterSpec describes the desired NetBox Virtualization Cluster
// representing the Kubernetes cluster itself.
type ClusterSpec struct {
	// Name is the Cluster object's name in NetBox.
	Name string

	// ClusterTypeName is the name of a pre-existing NetBox Cluster Type.
	ClusterTypeName string

	// ClusterGroupName is the name of a pre-existing NetBox Cluster Group.
	// Optional; leave empty to not assign a group.
	ClusterGroupName string

	// SiteName is the name of a pre-existing NetBox Site. Optional; leave
	// empty to not assign a site.
	SiteName string

	// ExternalID is the stable identity used to find this Cluster again
	// across renames (the kube-system Namespace's UID).
	ExternalID string

	// ManagedTag is the tag applied to the Cluster, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureCluster creates or updates the NetBox Cluster representing the
// Kubernetes cluster, found by ExternalID rather than name so that renaming
// spec.resources.cluster.name updates the existing object instead of
// creating a duplicate. Returns the NetBox Cluster's ID.
func EnsureCluster(ctx context.Context, client *netboxclient.NetBoxAPI, spec ClusterSpec) (int64, error) {
	existing, err := findClusterByExternalID(ctx, client, *spec.ManagedTag.Slug, spec.ExternalID)
	if err != nil {
		return 0, err
	}

	clusterTypeID, err := LookupClusterTypeID(ctx, client, spec.ClusterTypeName)
	if err != nil {
		return 0, err
	}

	data := &models.WritableCluster{
		Name: &spec.Name,
		Type: &clusterTypeID,
		Tags: []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
		CustomFields: map[string]any{
			ExternalIDFieldName: spec.ExternalID,
		},
	}

	if spec.ClusterGroupName != "" {
		groupID, err := LookupClusterGroupID(ctx, client, spec.ClusterGroupName)
		if err != nil {
			return 0, err
		}
		data.Group = &groupID
	}

	if spec.SiteName != "" {
		siteID, err := LookupSiteID(ctx, client, spec.SiteName)
		if err != nil {
			return 0, err
		}
		scopeType := "dcim.site"
		data.ScopeType = &scopeType
		data.ScopeID = &siteID
	}

	if existing != nil {
		updateResp, err := client.Virtualization.VirtualizationClustersUpdate(
			virtualization.NewVirtualizationClustersUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox cluster %q (id %d): %w", spec.Name, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Virtualization.VirtualizationClustersCreate(
		virtualization.NewVirtualizationClustersCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox cluster %q: %w", spec.Name, err)
	}
	return createResp.Payload.ID, nil
}

// DeleteCluster deletes the NetBox Cluster identified by externalID, if one
// managed by the tag identified by managedTagSlug exists. It's a no-op if no
// matching Cluster is found.
func DeleteCluster(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) error {
	existing, err := findClusterByExternalID(ctx, client, managedTagSlug, externalID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	_, err = client.Virtualization.VirtualizationClustersDelete(
		virtualization.NewVirtualizationClustersDeleteParams().WithContext(ctx).WithID(existing.ID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("deleting NetBox cluster (id %d): %w", existing.ID, err)
	}
	return nil
}

// findClusterByExternalID lists Clusters carrying the managed tag and
// returns the one whose external-ID custom field matches externalID, or nil
// if none do. Clusters without the managed tag are never inspected or
// touched.
func findClusterByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) (*models.Cluster, error) {
	listResp, err := client.Virtualization.VirtualizationClustersList(
		virtualization.NewVirtualizationClustersListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox clusters: %w", err)
	}

	for _, cluster := range listResp.Payload.Results {
		if id, ok := customFieldString(cluster.CustomFields, ExternalIDFieldName); ok && id == externalID {
			return cluster, nil
		}
	}
	return nil, nil
}

// customFieldString extracts a string-valued custom field from a NetBox
// object's CustomFields map (decoded from JSON as map[string]any).
func customFieldString(customFields any, key string) (string, bool) {
	fields, ok := customFields.(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := fields[key]
	if !ok || value == nil {
		return "", false
	}
	s, ok := value.(string)
	return s, ok
}
