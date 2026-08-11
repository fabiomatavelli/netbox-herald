# Cluster

Controlled by `spec.resources.cluster`.

Herald syncs a single synthetic "cluster" object representing the
Kubernetes cluster itself to a NetBox **Virtualization Cluster**
(`virtualization.Cluster`).

## Configuration

```yaml
spec:
  resources:
    cluster:
      enabled: true
      name: my-cluster              # required — name given to the Cluster object in NetBox
      clusterTypeName: kubernetes   # required — must already exist in NetBox
      clusterGroupName: production  # optional
      siteName: dc1                 # optional
```

## Required pre-existing NetBox objects

- A **Cluster Type** matching `clusterTypeName`.
- A **Cluster Group** matching `clusterGroupName`, if set.
- A **Site** matching `siteName`, if set.

Herald does not create these — see
[Reference objects](../architecture.md#reference-objects).

## Identity

The Cluster object's stable external ID (used to find it again across
resyncs, independent of its `name`) is the `kube-system` Namespace's
`.metadata.uid`. See [Idempotency](../architecture.md#idempotency-managed-tag--external-id).

## Relationship to other resources

When `spec.resources.nodes.mapping` is `VirtualMachine`, synced Nodes are
attached to this Cluster by default (`spec.resources.nodes.virtualMachine.clusterName`
defaults to `spec.resources.cluster.name`).
