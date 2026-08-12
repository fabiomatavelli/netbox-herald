# Pod CIDRs

Controlled by `spec.resources.podCIDRs`.

Herald syncs Node pod CIDR allocations (`.spec.podCIDRs`) into NetBox IPAM,
in one of two representations selected by
`spec.resources.podCIDRs.representation`:

- **`Prefix`** (default) — one NetBox IPAM **Prefix** per Node pod CIDR.
  This mirrors the cluster's actual per-node allocation, including
  dual-stack clusters where a Node has more than one pod CIDR.
- **`Aggregate`** — a single NetBox IPAM **Aggregate** per address family
  (IPv4 and/or IPv6), instead of one Prefix per Node. Each Aggregate is
  computed as the smallest CIDR block that fully encloses every currently-
  known Node's pod CIDR(s) in that family — recomputed from every Node on
  each reconcile, so it always reflects the live set of Nodes.

Switching `representation` cleans up the previous mode's objects
automatically: switching to `Aggregate` removes any existing per-node
Prefixes, and switching back to `Prefix` removes the Aggregate(s).

## Configuration

```yaml
spec:
  resources:
    podCIDRs:
      enabled: true
      representation: Prefix   # or: Aggregate
      retainOnDisable: false   # default: delete previously-synced NetBox objects on disable
      # rirName: RFC1918       # required only when representation is Aggregate
```

## Required pre-existing NetBox objects

- **`Prefix` mode**: none.
- **`Aggregate` mode**: a **RIR** (Regional Internet Registry) matching
  `rirName` — NetBox's Aggregate model requires one. For private pod CIDR
  ranges (e.g. `10.244.0.0/16`), a RIR named something like `RFC1918`
  works well.

Herald does not create the RIR — see
[Reference objects](../architecture.md#reference-objects).

## Identity

Neither representation uses a Kubernetes object's `.metadata.uid` as the
external ID, unlike Cluster or Node:

- In `Prefix` mode, a Node's pod CIDR(s) can change, and — like
  [Services](services.md) — Herald's cleanup reconcile for a deleted Node
  only has its name available (from the reconcile request), not its UID.
  So each Prefix's external ID is a `<node-name>/<cidr>` composite instead.
- In `Aggregate` mode, there's no single owning Kubernetes object at all —
  the Aggregate is computed from every Node together. Each family's
  Aggregate uses a fixed external ID (`cluster-pod-cidrs/ipv4` or
  `cluster-pod-cidrs/ipv6`), so there's always at most one managed
  Aggregate per family.
