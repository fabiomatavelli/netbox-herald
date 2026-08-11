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

// EnsureManagedTag returns the ID of the NetBox tag matching slug, creating
// it if it doesn't already exist. netbox-herald applies this tag to every
// object it creates, and only ever updates or deletes objects that carry it.
func EnsureManagedTag(ctx context.Context, client *netboxclient.NetBoxAPI, slug string) (int64, error) {
	listResp, err := client.Extras.ExtrasTagsList(
		extras.NewExtrasTagsListParams().WithContext(ctx).WithSlug(&slug),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("listing NetBox tags: %w", err)
	}

	for _, tag := range listResp.Payload.Results {
		if tag.Slug != nil && *tag.Slug == slug {
			return tag.ID, nil
		}
	}

	name := "netbox-herald managed"
	createResp, err := client.Extras.ExtrasTagsCreate(
		extras.NewExtrasTagsCreateParams().WithContext(ctx).WithData(&models.Tag{
			Name:        &name,
			Slug:        &slug,
			Description: "Objects managed by netbox-herald. Do not edit or remove this tag manually.",
			Color:       "0d6efd",
		}),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating NetBox managed tag %q: %w", slug, err)
	}

	return createResp.Payload.ID, nil
}
