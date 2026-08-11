# Architecture

## Scope

netbox-herald runs as a single Deployment inside one Kubernetes cluster and
syncs that cluster's state into one NetBox instance. It does not manage
multiple clusters or multiple NetBox instances from a single deployment.

## Configuration: the `HeraldConfig` custom resource

All of Herald's behavior — NetBox connection details and which resource
types are synced — is controlled by a single, cluster-scoped `HeraldConfig`
custom resource named `default`. Herald only reconciles the CR named
`default`; this is enforced by convention for now (a validating webhook may
enforce it strictly in a later phase).

Because it's a normal Kubernetes object, changes take effect live:
`kubectl edit heraldconfig default` and flip `spec.resources.nodes.enabled`
— no restart of the operator required.

## Controller design

Herald uses **always-on watches with a per-reconcile enabled check**, not
dynamic controller start/stop:

- Every resource controller (`cluster`, `nodes`, `services`, `podCIDRs`)
  registers with the `controller-runtime` Manager at startup, so their
  caches are always warm.
- A shared, in-memory, mutex-protected config store
  (`internal/config/store.go`) holds the current `HeraldConfigSpec` and the
  authenticated NetBox client. Only the `HeraldConfig` reconciler writes to
  it; every other controller reads from it at the top of `Reconcile()`
  instead of hitting the API server again.
- If a resource type is disabled, its controller updates its
  `status.resources.<type>` to reflect that and returns without making any
  NetBox API calls.
- Each resource controller also watches `HeraldConfig` itself, so flipping
  an enable flag immediately triggers reconciliation of every currently
  known object of that type, rather than waiting for the next native
  Kubernetes event or the periodic resync.
- A periodic full resync (`spec.resync.interval`, default 5m) catches drift
  that watches might miss.

This was chosen over dynamically registering/tearing down controllers
within a live Manager, since `controller-runtime` has no supported API for
that, and building a custom supervisor for it would add real complexity and
risk (cache races, non-standard shutdown) for a single-cluster operator
with modest resource cardinality.

## Idempotency: managed tag + external ID

NetBox has no concept of "owned by this controller" out of the box, so
Herald establishes its own:

1. On `HeraldConfig` reconcile, Herald ensures (creating if missing):
   - a NetBox tag (slug from `spec.netbox.managedTag.slug`, default
     `netbox-herald-managed`);
   - a custom field `netbox_herald_external_id` (text), scoped to every
     NetBox content type Herald writes to (`dcim.device`,
     `virtualization.virtualmachine`, `ipam.ipaddress`, `ipam.prefix`,
     `ipam.aggregate`, `virtualization.cluster`).
2. The external ID for a given Kubernetes object is its stable UID
   (`.metadata.uid`) — immutable for the object's lifetime, even across
   renames. The synthetic "cluster" object's external ID is the
   `kube-system` Namespace's UID, since Kubernetes has no first-class
   cluster-identity object.
3. On every reconcile, Herald looks up the NetBox object by
   `cf_netbox_herald_external_id=<uid>` first. If found, it updates it in
   place; if not, it creates it and applies the managed tag.

This makes renames safe (a relabeled Node or renamed Service updates the
same NetBox object instead of creating a duplicate) and guarantees Herald
never touches a NetBox object it didn't create — objects lacking the
managed tag are never modified or deleted, regardless of what they're
named.

## Disable behavior

By default, disabling a resource type **deletes** every NetBox object
Herald manages for that type (identified via the managed tag + external
ID) — config is treated as the source of truth, so turning a resource off
means NetBox shouldn't keep stale records for it either.

Setting `retainOnDisable: true` on a resource type's config opts into the
more conservative alternative instead: Herald stops creating/updating
NetBox objects for that type but leaves whatever it already created
untouched, in case an accidental flag flip shouldn't be allowed to delete
data in NetBox.

## Reference objects

Herald never auto-creates NetBox's structural/taxonomy objects — Sites,
Device Roles, Device Types, Cluster Types, Cluster Groups, Platforms. These
must already exist in NetBox and are referenced by name in `HeraldConfig`.
If a referenced object is missing, Herald surfaces a clear error via
`status.conditions` and the relevant `status.resources.<type>.lastError`
field, and does not sync that resource type until it's fixed. This keeps
Herald's responsibility bounded to syncing *live cluster state* — it
doesn't invent infrastructure taxonomy on your behalf.

## Extensibility

New resource types are added as new, optional, non-breaking fields under
`spec.resources` (per Kubernetes API conventions, adding an optional field
is not a breaking change). `spec.resources.additional` is a map-based
escape hatch for experimenting with a resource type before it's promoted to
a fully-typed field.
