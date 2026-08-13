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

// ExternalIDForServiceAddress builds the external ID for one of a Service's
// LoadBalancer addresses. Unlike Cluster or Node, this isn't keyed by the
// Service's UID: once a Service is deleted, Reconcile only has its
// namespace/name (from the reconcile request) to work with, not its UID, so
// namespace/name is used instead — it's still immutable and unique, and
// remains available for cleanup after deletion. A Service can have more
// than one address (e.g. dual-stack), so the address itself is appended;
// serviceExternalIDPrefix groups them back together for that cleanup.
func ExternalIDForServiceAddress(namespace, name, address string) string {
	return serviceExternalIDPrefix(namespace, name) + address
}

func serviceExternalIDPrefix(namespace, name string) string {
	return namespace + "/" + name + "/"
}

// ServiceIPSpec describes the desired NetBox IP Address representing a
// LoadBalancer Service's external IP.
//
// Recorded with no Device/VM parent: NetBox's ipam.Service model expects a
// single owning Device/VM, which Kubernetes Services don't naturally have,
// so a bare IPAM IP Address is the only NetBox object with real external
// meaning to document here.
type ServiceIPSpec struct {
	// Address is the external IP in CIDR notation (e.g. "10.0.0.5/32"). The
	// exact prefix length of the containing subnet isn't knowable from a
	// bare external IP, so a host mask is always used.
	Address string

	// Description records which Kubernetes Service this IP belongs to.
	Description string

	// ExternalID is the stable identity used to find this IP Address again,
	// built by ExternalIDForServiceAddress.
	ExternalID string

	// ManagedTag is the tag applied to the IP Address, identifying it as
	// Herald-managed.
	ManagedTag *models.Tag
}

// EnsureServiceIP creates or updates the NetBox IP Address representing a
// LoadBalancer Service's external IP, found by ExternalID rather than
// address so that an external IP changing while the Service persists
// updates the existing object instead of creating a duplicate. Returns the
// NetBox IP Address's ID.
func EnsureServiceIP(ctx context.Context, client *netboxclient.NetBoxAPI, spec ServiceIPSpec) (int64, error) {
	existing, err := findServiceIPByExternalID(ctx, client, *spec.ManagedTag.Slug, spec.ExternalID)
	if err != nil {
		return 0, err
	}

	data := &models.WritableIPAddress{
		Address:     &spec.Address,
		Description: spec.Description,
		Tags:        []*models.NestedTag{NestedManagedTag(spec.ManagedTag)},
		CustomFields: map[string]any{
			ExternalIDFieldName: spec.ExternalID,
		},
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

// ListServiceIPs returns every NetBox IP Address managed by the tag
// identified by managedTagSlug that belongs to the Service identified by
// namespace/name (see ExternalIDForServiceAddress) — every address it ever
// had synced, using only namespace/name so this works whether or not the
// Service object itself still exists.
func ListServiceIPs(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, namespace, name string) ([]*models.IPAddress, error) {
	prefix := serviceExternalIDPrefix(namespace, name)

	listResp, err := client.Ipam.IpamIPAddressesList(
		ipam.NewIpamIPAddressesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox IP addresses: %w", err)
	}

	var owned []*models.IPAddress
	for _, addr := range listResp.Payload.Results {
		if id, ok := readExternalID(addr.CustomFields); ok && strings.HasPrefix(id, prefix) {
			owned = append(owned, addr)
		}
	}
	return owned, nil
}

// DeleteServiceIPs deletes every NetBox IP Address a Service owns (per
// ListServiceIPs). Used when the Service is deleted, or when Services sync
// is disabled and this Service isn't in the current desired set.
func DeleteServiceIPs(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, namespace, name string) error {
	owned, err := ListServiceIPs(ctx, client, managedTagSlug, namespace, name)
	if err != nil {
		return err
	}
	for _, addr := range owned {
		if err := deleteIPAddress(ctx, client, addr.ID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteServiceIP deletes a single NetBox IP Address by its NetBox ID.
// Exposed for callers (e.g. pruning stale addresses after a Service's
// external IPs change) that already have the object from ListServiceIPs and
// don't want to re-look it up.
func DeleteServiceIP(ctx context.Context, client *netboxclient.NetBoxAPI, id int64) error {
	return deleteIPAddress(ctx, client, id)
}

func deleteIPAddress(ctx context.Context, client *netboxclient.NetBoxAPI, id int64) error {
	_, err := client.Ipam.IpamIPAddressesDelete(
		ipam.NewIpamIPAddressesDeleteParams().WithContext(ctx).WithID(id),
		nil,
	)
	if err != nil {
		return fmt.Errorf("deleting NetBox IP address (id %d): %w", id, err)
	}
	return nil
}

func findServiceIPByExternalID(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug, externalID string) (*models.IPAddress, error) {
	listResp, err := client.Ipam.IpamIPAddressesList(
		ipam.NewIpamIPAddressesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("listing NetBox IP addresses: %w", err)
	}
	for _, addr := range listResp.Payload.Results {
		if id, ok := readExternalID(addr.CustomFields); ok && id == externalID {
			return addr, nil
		}
	}
	return nil, nil
}

// CountManagedServiceIPs returns the number of NetBox IP Addresses carrying
// the tag identified by managedTagSlug.
func CountManagedServiceIPs(ctx context.Context, client *netboxclient.NetBoxAPI, managedTagSlug string) (int64, error) {
	limit := int64(1)
	resp, err := client.Ipam.IpamIPAddressesList(
		ipam.NewIpamIPAddressesListParams().WithContext(ctx).WithTag([]string{managedTagSlug}).WithLimit(&limit),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox IP addresses: %w", err)
	}
	if resp.Payload.Count == nil {
		return 0, nil
	}
	return *resp.Payload.Count, nil
}
