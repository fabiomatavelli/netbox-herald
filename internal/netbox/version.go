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

	"github.com/Masterminds/semver/v3"
	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	"github.com/fbreckle/go-netbox/netbox/client/status"
)

// SupportedVersionRange is the semver constraint netbox-herald has been
// tested against. See docs/netbox-compatibility.md.
const SupportedVersionRange = ">= 4.3.0, < 5.0.0"

var supportedConstraint *semver.Constraints

func init() {
	c, err := semver.NewConstraint(SupportedVersionRange)
	if err != nil {
		panic(fmt.Sprintf("netbox: invalid SupportedVersionRange: %v", err))
	}
	supportedConstraint = c
}

// CheckVersion fetches the connected NetBox instance's version and returns
// it, along with an error if it falls outside SupportedVersionRange.
func CheckVersion(ctx context.Context, client *netboxclient.NetBoxAPI) (string, error) {
	resp, err := client.Status.StatusList(status.NewStatusListParams().WithContext(ctx), nil)
	if err != nil {
		return "", fmt.Errorf("fetching NetBox status: %w", err)
	}

	payload, ok := resp.Payload.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected NetBox status response shape")
	}

	rawVersion, ok := payload["netbox-version"].(string)
	if !ok || rawVersion == "" {
		return "", fmt.Errorf("NetBox status response missing netbox-version")
	}

	v, err := semver.NewVersion(rawVersion)
	if err != nil {
		return rawVersion, fmt.Errorf("parsing NetBox version %q: %w", rawVersion, err)
	}

	if !supportedConstraint.Check(v) {
		return rawVersion, fmt.Errorf("NetBox version %s does not satisfy supported range %s", rawVersion, SupportedVersionRange)
	}

	return rawVersion, nil
}
