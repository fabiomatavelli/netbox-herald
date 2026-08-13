# Nodes

Controlled by `spec.resources.nodes`.

Herald syncs every Kubernetes `Node` to either a NetBox **DCIM Device** or a
**Virtualization VirtualMachine**, selected via `spec.resources.nodes.mapping`.
Pick `Device` for bare-metal clusters (e.g. Talos on physical hardware) and
`VirtualMachine` for clusters running on VMs.

## Device mapping

```yaml
spec:
  resources:
    nodes:
      enabled: true
      mapping: Device
      device:
        deviceRoleName: kubernetes-node   # required — must already exist in NetBox
        deviceTypeName: generic-server    # required — must already exist in NetBox
        siteName: dc1                     # optional
```

### Required pre-existing NetBox objects

- A **Device Role** matching `deviceRoleName`.
- A **Device Type** (and its Manufacturer) matching `deviceTypeName`.
- A **Site** matching `siteName`, if set.

## VirtualMachine mapping

```yaml
spec:
  resources:
    nodes:
      enabled: true
      mapping: VirtualMachine
      virtualMachine:
        clusterName: my-cluster   # optional — defaults to spec.resources.cluster.name
        platformName: talos-linux # optional
```

### Required pre-existing NetBox objects

- The target **Cluster** (either synced by Herald itself via
  `spec.resources.cluster`, or a pre-existing one referenced by
  `clusterName`).
- A **Platform** matching `platformName`, if set.

## Primary IP address

```yaml
spec:
  resources:
    nodes:
      addressType: InternalIP   # or ExternalIP — default: InternalIP
      interfaceName: primary    # default: primary
```

Applies identically under both `Device` and `VirtualMachine` mapping. Herald
reads every address of `addressType` from the Node's `status.addresses` (an
IPv4 and, on a dual-stack cluster, an IPv6 address), assigns each to a NetBox
Interface (`Device` mapping) or VMInterface (`VirtualMachine` mapping) named
`interfaceName`, and sets it as the Device/VirtualMachine's primary IPv4/IPv6
address. A Node with no address of the configured type is synced without an
Interface or IP Address.

Unlike the Device/VirtualMachine object itself, the Interface and IP Address
aren't found by external ID: the Interface is found by NetBox's own
`(device, name)` / `(virtual_machine, name)` uniqueness constraint, and the
IP Address by the address currently assigned to that interface.

## Pausing vs. deleting on disable

```yaml
spec:
  resources:
    nodes:
      retainOnDisable: false   # default: delete previously-synced NetBox objects on disable
```

See [Disable behavior](../architecture.md#disable-behavior).

## Identity

Each Node's stable external ID is its `.metadata.uid`, so renaming/relabeling
a Node updates the same NetBox object rather than creating a duplicate. See
[Idempotency](../architecture.md#idempotency-managed-tag--external-id).
