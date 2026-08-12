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
	"strings"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	"github.com/fbreckle/go-netbox/netbox/client/ipam"
	"github.com/fbreckle/go-netbox/netbox/models"
)

// ExternalIDForNodePodCIDR builds the external ID for one of a Node's pod
// CIDRs. Like Service addresses, a Node can have more than one (dual-stack),
// and Prefix has no name field to fall back on when the Node is gone, so
// nodePodCIDRExternalIDPrefix (namespace-less: Nodes are cluster-scoped)
// groups them back together for cleanup via the Node's name alone.
func ExternalIDForNodePodCIDR(nodeName, cidr string) string {
	return nodePodCIDRExternalIDPrefix(nodeName) + cidr
}

func nodePodCIDRExternalIDPrefix(nodeName string) string {
	return nodeName + "/"
}

// PodCIDRPrefixSpec describes the desired NetBox IPAM Prefix representing
// one of a Node's pod CIDRs.
type PodCIDRPrefixSpec struct {
	// CIDR is the pod CIDR in CIDR notation (e.g. "10.244.1.0/24").
	CIDR string

	// Description records which Kubernetes Node this CIDR belongs to.
	Description string

	// ExternalID is the stable identity used to find this Prefix again,
	// built by ExternalIDForNodePodCIDR.
	ExternalID string

	// ManagedTag is the tag applied to the Prefix, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsurePodCIDRPrefix creates or updates the NetBox IPAM Prefix representing
// one of a Node's pod CIDRs, found by ExternalID. Returns the NetBox
// Prefix's ID.
func EnsurePodCIDRPrefix(ctx context.Context, client *netboxclient.NetBoxAPI, spec PodCIDRPrefixSpec) (int64, error) {
	existing, err := findPrefixByExternalID(ctx, client, *spec.ManagedTag.Slug, spec.ExternalID)
	if err != nil {
		return 0, err
	}

	data := &models.WritablePrefix{
		Prefix:      &spec.CIDR,
		Description: spec.Description,
		Tags:        []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
		CustomFields: map[string]any{
			ExternalIDFieldName: spec.ExternalID,
		},
	}

	if existing != nil {
		updateResp, err := client.Ipam.IpamPrefixesUpdate(
			ipam.NewIpamPrefixesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox prefix %q (id %d): %w", spec.CIDR, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Ipam.IpamPrefixesCreate(
		ipam.NewIpamPrefixesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox prefix %q: %w", spec.CIDR, err)
	}
	return createResp.Payload.ID, nil
}

// ListNodePodCIDRPrefixes returns every NetBox Prefix managed by the tag
// identified by managedTagSlug that belongs to the Node identified by name
// (see ExternalIDForNodePodCIDR) — every pod CIDR it ever had synced, using
// only the Node's name so this works whether or not the Node object itself
// still exists.
func ListNodePodCIDRPrefixes(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, nodeName string) ([]*models.Prefix, error) {
	prefix := nodePodCIDRExternalIDPrefix(nodeName)

	listResp, err := client.Ipam.IpamPrefixesList(
		ipam.NewIpamPrefixesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox prefixes: %w", err)
	}

	var owned []*models.Prefix
	for _, p := range listResp.Payload.Results {
		if id, ok := readExternalID(p.CustomFields); ok && strings.HasPrefix(id, prefix) {
			owned = append(owned, p)
		}
	}
	return owned, nil
}

// DeleteNodePodCIDRPrefixes deletes every NetBox Prefix a Node owns (per
// ListNodePodCIDRPrefixes). Used when the Node is deleted, or when pod CIDR
// sync is disabled and this Node isn't in the current desired set.
func DeleteNodePodCIDRPrefixes(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, nodeName string) error {
	owned, err := ListNodePodCIDRPrefixes(ctx, client, managedTagSlug, nodeName)
	if err != nil {
		return err
	}
	for _, p := range owned {
		if err := deletePrefix(ctx, client, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// DeletePodCIDRPrefix deletes a single NetBox Prefix by its NetBox ID.
// Exposed for callers (e.g. pruning stale CIDRs after a Node's pod CIDRs
// change) that already have the object from ListNodePodCIDRPrefixes and
// don't want to re-look it up.
func DeletePodCIDRPrefix(ctx context.Context, client *netboxclient.NetBoxAPI, id int64) error {
	return deletePrefix(ctx, client, id)
}

func deletePrefix(ctx context.Context, client *netboxclient.NetBoxAPI, id int64) error {
	_, err := client.Ipam.IpamPrefixesDelete(
		ipam.NewIpamPrefixesDeleteParams().WithContext(ctx).WithID(id),
		nil,
	)
	if err != nil {
		return fmt.Errorf("deleting NetBox prefix (id %d): %w", id, err)
	}
	return nil
}

func findPrefixByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) (*models.Prefix, error) {
	listResp, err := client.Ipam.IpamPrefixesList(
		ipam.NewIpamPrefixesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox prefixes: %w", err)
	}
	for _, p := range listResp.Payload.Results {
		if id, ok := readExternalID(p.CustomFields); ok && id == externalID {
			return p, nil
		}
	}
	return nil, nil
}

// CountManagedPrefixes returns the number of NetBox Prefixes carrying the
// tag identified by managedTagSlug.
func CountManagedPrefixes(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string) (int64, error) {
	limit := int64(1)
	resp, err := client.Ipam.IpamPrefixesList(
		ipam.NewIpamPrefixesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithLimit(&limit),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox prefixes: %w", err)
	}
	if resp.Payload.Count == nil {
		return 0, nil
	}
	return *resp.Payload.Count, nil
}

// ExternalIDForPodCIDRAggregate builds the external ID for the single
// cluster-wide Aggregate covering one address family's pod CIDRs.
func ExternalIDForPodCIDRAggregate(family string) string {
	return "cluster-pod-cidrs/" + family
}

// PodCIDRAggregateSpec describes the desired NetBox IPAM Aggregate covering
// every pod CIDR in one address family.
type PodCIDRAggregateSpec struct {
	// CIDR is the smallest block containing every currently-known pod CIDR
	// in this address family (e.g. "10.244.0.0/16").
	CIDR string

	// RIRName is the name of a pre-existing NetBox RIR. NetBox's Aggregate
	// model requires one.
	RIRName string

	// Description records that this Aggregate is computed from live pod
	// CIDRs, not manually managed.
	Description string

	// ExternalID is the stable identity used to find this Aggregate again,
	// built by ExternalIDForPodCIDRAggregate.
	ExternalID string

	// ManagedTag is the tag applied to the Aggregate, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsurePodCIDRAggregate creates or updates the single NetBox IPAM
// Aggregate covering every currently-known pod CIDR in one address family,
// found by ExternalID. Returns the NetBox Aggregate's ID.
func EnsurePodCIDRAggregate(ctx context.Context, client *netboxclient.NetBoxAPI, spec PodCIDRAggregateSpec) (int64, error) {
	existing, err := findAggregateByExternalID(ctx, client, *spec.ManagedTag.Slug, spec.ExternalID)
	if err != nil {
		return 0, err
	}

	rirID, err := LookupRIRID(ctx, client, spec.RIRName)
	if err != nil {
		return 0, err
	}

	data := &models.WritableAggregate{
		Prefix:      &spec.CIDR,
		Rir:         &rirID,
		Description: spec.Description,
		Tags:        []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
		CustomFields: map[string]any{
			ExternalIDFieldName: spec.ExternalID,
		},
	}

	if existing != nil {
		updateResp, err := client.Ipam.IpamAggregatesUpdate(
			ipam.NewIpamAggregatesUpdateParams().WithContext(ctx).WithID(existing.ID).WithData(data),
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("updating NetBox aggregate %q (id %d): %w", spec.CIDR, existing.ID, err)
		}
		return updateResp.Payload.ID, nil
	}

	createResp, err := client.Ipam.IpamAggregatesCreate(
		ipam.NewIpamAggregatesCreateParams().WithContext(ctx).WithData(data),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox aggregate %q: %w", spec.CIDR, err)
	}
	return createResp.Payload.ID, nil
}

// DeletePodCIDRAggregate deletes the NetBox Aggregate identified by
// externalID, if one managed by the tag identified by managedTagSlug
// exists. It's a no-op if no matching Aggregate is found.
func DeletePodCIDRAggregate(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) error {
	existing, err := findAggregateByExternalID(ctx, client, managedTagSlug, externalID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	_, err = client.Ipam.IpamAggregatesDelete(
		ipam.NewIpamAggregatesDeleteParams().WithContext(ctx).WithID(existing.ID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("deleting NetBox aggregate (id %d): %w", existing.ID, err)
	}
	return nil
}

func findAggregateByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) (*models.Aggregate, error) {
	listResp, err := client.Ipam.IpamAggregatesList(
		ipam.NewIpamAggregatesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox aggregates: %w", err)
	}
	for _, a := range listResp.Payload.Results {
		if id, ok := readExternalID(a.CustomFields); ok && id == externalID {
			return a, nil
		}
	}
	return nil, nil
}

// CountManagedAggregates returns the number of NetBox Aggregates carrying
// the tag identified by managedTagSlug.
func CountManagedAggregates(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string) (int64, error) {
	limit := int64(1)
	resp, err := client.Ipam.IpamAggregatesList(
		ipam.NewIpamAggregatesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithLimit(&limit),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox aggregates: %w", err)
	}
	if resp.Payload.Count == nil {
		return 0, nil
	}
	return *resp.Payload.Count, nil
}
