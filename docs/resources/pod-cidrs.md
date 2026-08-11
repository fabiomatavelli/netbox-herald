# Pod CIDRs

Controlled by `spec.resources.podCIDRs`.

Herald syncs pod CIDR allocations into NetBox IPAM, in one of two
representations selected by `spec.resources.podCIDRs.representation`:

- **`Prefix`** (default) — one NetBox IPAM **Prefix** per Node, using each
  Node's `.spec.podCIDR`. This mirrors the cluster's actual per-node
  allocation.
- **`Aggregate`** — a single NetBox IPAM **Aggregate** covering the
  cluster-wide pod CIDR supernet (derived from the cluster's configured pod
  CIDR range), instead of one Prefix per Node. Use this if you want a
  single high-level record rather than one object per Node.

## Configuration

```yaml
spec:
  resources:
    podCIDRs:
      enabled: true
      representation: Prefix   # or: Aggregate
      retainOnDisable: false    # default: delete previously-synced NetBox objects on disable
```

## Identity

In `Prefix` mode, each synced Prefix's stable external ID is the owning
Node's `.metadata.uid`. In `Aggregate` mode, the single synced Aggregate's
external ID is the `kube-system` Namespace's UID (the same cluster-identity
value used for the [Cluster resource](cluster.md)).
