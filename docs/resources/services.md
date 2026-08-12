# Services

Controlled by `spec.resources.services`.

NetBox's IPAM **Service** model (`ipam.Service`) expects a single owning
Device or VirtualMachine, which Kubernetes Services don't naturally have —
especially `LoadBalancer` and `NodePort` Services, which are often backed by
pods spread across many Nodes. Rather than force a synthetic parent onto
every Service, Herald takes a narrower, more meaningful approach:

**Herald only syncs `type: LoadBalancer` Services that have an assigned
external IP**, recording each as a NetBox **IPAM IP Address**
(`ipam.IPAddress`) with **no** Device/VM parent. `ClusterIP` and `NodePort`
Services are not synced — they have no externally-meaningful address worth
documenting in NetBox.

## Configuration

```yaml
spec:
  resources:
    services:
      enabled: true
      retainOnDisable: false   # default: delete previously-synced NetBox objects on disable
```

There are no additional required fields — no pre-existing NetBox reference
objects are needed for this resource type, since synced IP Addresses have
no Device/VM parent.

## Identity

Unlike Cluster or Node, a synced address's stable external ID is **not**
the Service's `.metadata.uid`. A Service can have more than one external IP
(dual-stack), and once a Service is deleted, Herald's cleanup reconcile
only has its namespace/name available (from the reconcile request) — not
its UID. So each address's external ID is a `<namespace>/<name>/<address>`
composite instead: stable and unique per address, and enough on its own to
find and remove every address a given Service ever had synced, even after
the Service itself is gone.

If a `LoadBalancer` Service's external IP changes while the Service
persists, Herald creates a NetBox IP Address for the new address and
removes the one for the old address — nothing is left orphaned.

## Future direction

If a clear, non-synthetic way to associate Services with a Device/VM parent
emerges (e.g. reliably attributing a `LoadBalancer` IP to a specific
ingress/load-balancer node), this mapping may be extended. See
[Extensibility](../architecture.md#extensibility).
