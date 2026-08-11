# NetBox version compatibility

netbox-herald checks the connected NetBox instance's version against a
supported semver range at every `HeraldConfig` reconcile (via NetBox's
`status` API endpoint), unless disabled with
`spec.netbox.versionCheck.enabled: false`.

| netbox-herald | Supported NetBox versions |
| --- | --- |
| v0.x (unreleased) | `>= 4.3.0, < 5.0.0` |

This range is intentionally broad rather than an explicit per-patch list
(unlike [terraform-provider-netbox](https://github.com/e-breuninger/terraform-provider-netbox),
which tests every patch release in its acceptance matrix). netbox-herald's
CI e2e suite exercises the oldest and newest supported minor versions
rather than every patch, since it has a smaller API surface than a full
CRUD Terraform provider.

If you run a NetBox version outside this range, Herald will report
`status.netbox.connected: false` with an explanatory condition rather than
silently attempting (and possibly failing in confusing ways) to sync.
