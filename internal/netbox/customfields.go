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
	"github.com/fbreckle/go-netbox/netbox/client/extras"
	"github.com/fbreckle/go-netbox/netbox/models"
)

// ExternalIDFieldName is the internal (API) name of the custom field
// netbox-herald uses to store a NetBox object's stable external ID (the
// owning Kubernetes object's UID), so it can find the same object again
// across renames.
const ExternalIDFieldName = "netbox_herald_external_id"

// externalIDFieldObjectTypes lists every NetBox object type netbox-herald
// may write ExternalIDFieldName onto.
var externalIDFieldObjectTypes = []string{
	"dcim.device",
	"virtualization.virtualmachine",
	"ipam.ipaddress",
	"ipam.prefix",
	"ipam.aggregate",
	"virtualization.cluster",
}

// EnsureExternalIDCustomField ensures the ExternalIDFieldName custom field
// exists and is scoped to every object type netbox-herald writes to,
// creating it if missing. It does not narrow the field's object types if it
// already exists with a broader scope than externalIDFieldObjectTypes.
func EnsureExternalIDCustomField(ctx context.Context, client *netboxclient.NetBoxAPI) error {
	name := ExternalIDFieldName
	listResp, err := client.Extras.ExtrasCustomFieldsList(
		extras.NewExtrasCustomFieldsListParams().WithContext(ctx).WithName(&name),
		nil,
	)
	if err != nil {
		return fmt.Errorf("listing NetBox custom fields: %w", err)
	}

	for _, field := range listResp.Payload.Results {
		if field.Name != nil && *field.Name == name {
			return nil
		}
	}

	label := "Herald External ID"
	description := "Stable external ID (Kubernetes object UID) netbox-herald uses to find and update the same NetBox object across renames. Managed by netbox-herald; do not edit."
	_, err = client.Extras.ExtrasCustomFieldsCreate(
		extras.NewExtrasCustomFieldsCreateParams().WithContext(ctx).WithData(&models.WritableCustomField{
			Name:        &name,
			Label:       label,
			Description: description,
			Type:        "text",
			ObjectTypes: externalIDFieldObjectTypes,
		}),
		nil,
	)
	if err != nil {
		return fmt.Errorf("creating NetBox custom field %q: %w", name, err)
	}

	return nil
}
