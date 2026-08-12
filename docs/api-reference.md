# API Reference

## Packages
- [netbox-herald.io/v1alpha1](#netbox-heraldiov1alpha1)


## netbox-herald.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the herald v1alpha1 API group.

### Resource Types
- [HeraldConfig](#heraldconfig)



#### ClusterResourceConfig



ClusterResourceConfig controls syncing of the Kubernetes cluster itself
into a NetBox Virtualization Cluster.



_Appears in:_
- [ResourcesConfig](#resourcesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls whether the cluster resource is synced to NetBox. | false | Optional: \{\} <br /> |
| `name` _string_ | name is the name given to the Cluster object in NetBox. |  | Optional: \{\} <br /> |
| `clusterTypeName` _string_ | clusterTypeName is the name of a pre-existing NetBox Cluster Type.<br />Herald does not create Cluster Types; they must already exist. |  | Optional: \{\} <br /> |
| `clusterGroupName` _string_ | clusterGroupName is the name of a pre-existing NetBox Cluster Group. |  | Optional: \{\} <br /> |
| `siteName` _string_ | siteName is the name of a pre-existing NetBox Site to associate with<br />the cluster. |  | Optional: \{\} <br /> |


#### DeviceMappingConfig



DeviceMappingConfig configures Node-to-Device mapping.



_Appears in:_
- [NodesResourceConfig](#nodesresourceconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRoleName` _string_ | deviceRoleName is the name of a pre-existing NetBox Device Role assigned<br />to synced Devices. |  | Optional: \{\} <br /> |
| `deviceTypeName` _string_ | deviceTypeName is the name of a pre-existing NetBox Device Type assigned<br />to synced Devices. |  | Optional: \{\} <br /> |
| `siteName` _string_ | siteName is the name of a pre-existing NetBox Site assigned to synced<br />Devices. |  | Optional: \{\} <br /> |


#### GenericResourceConfig



GenericResourceConfig is a minimal enable/disable toggle for resource
types that have not yet been promoted to a fully-typed field under
ResourcesConfig. It exists so new resource types can be experimented with
without a breaking CRD schema change; once a resource type stabilizes it
should be promoted to its own typed field.



_Appears in:_
- [ResourcesConfig](#resourcesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls whether this resource type is synced to NetBox. | false | Optional: \{\} <br /> |
| `retainOnDisable` _boolean_ | retainOnDisable controls whether previously-synced NetBox objects for<br />this resource type are kept when it is disabled. Defaults to false, so<br />disabling deletes every NetBox object Herald manages for this resource<br />type; set to true to instead pause syncing and leave existing objects<br />untouched. | false | Optional: \{\} <br /> |


#### HeraldConfig



HeraldConfig is the Schema for the heraldconfigs API. It is cluster-scoped
and singleton: Herald only reconciles the HeraldConfig named "default".





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `netbox-herald.io/v1alpha1` | | |
| `kind` _string_ | `HeraldConfig` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[HeraldConfigSpec](#heraldconfigspec)_ | spec defines the desired state of HeraldConfig |  | Required: \{\} <br /> |
| `status` _[HeraldConfigStatus](#heraldconfigstatus)_ | status defines the observed state of HeraldConfig |  | Optional: \{\} <br /> |


#### HeraldConfigSpec



HeraldConfigSpec defines the desired state of HeraldConfig.



_Appears in:_
- [HeraldConfig](#heraldconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `netbox` _[NetBoxConfig](#netboxconfig)_ | netbox describes how to connect to the target NetBox instance. |  | Required: \{\} <br /> |
| `resync` _[ResyncConfig](#resyncconfig)_ | resync controls periodic full resynchronization. |  | Optional: \{\} <br /> |
| `resources` _[ResourcesConfig](#resourcesconfig)_ | resources controls which resource types are synced to NetBox, with<br />per-type configuration. |  | Optional: \{\} <br /> |


#### HeraldConfigStatus



HeraldConfigStatus defines the observed state of HeraldConfig.



_Appears in:_
- [HeraldConfig](#heraldconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ | conditions represent the current state of the HeraldConfig resource.<br />Each condition has a unique type and reflects the status of a specific<br />aspect of the resource.<br />Standard condition types include:<br />- "Ready": Herald is connected to NetBox and syncing enabled resources<br />- "NetBoxReachable": the configured NetBox instance is reachable and<br />  its version is compatible<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |
| `netbox` _[NetBoxStatus](#netboxstatus)_ | netbox reports the last observed connectivity state of the configured<br />NetBox instance. |  | Optional: \{\} <br /> |
| `resources` _[ResourceStatuses](#resourcestatuses)_ | resources reports the observed sync state of every resource type. |  | Optional: \{\} <br /> |


#### ManagedTagConfig



ManagedTagConfig controls the NetBox tag Herald uses to mark objects it owns.



_Appears in:_
- [NetBoxConfig](#netboxconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `slug` _string_ | slug is the slug of the NetBox tag applied to every object Herald<br />creates, and used to identify objects safe for Herald to update or<br />delete. Herald never modifies NetBox objects lacking this tag. | netbox-herald-managed | Optional: \{\} <br /> |


#### NetBoxConfig



NetBoxConfig describes how to connect to the target NetBox instance.



_Appears in:_
- [HeraldConfigSpec](#heraldconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | url is the base URL of the NetBox instance, e.g. https://netbox.example.com. |  | Required: \{\} <br /> |
| `tokenSecretRef` _[SecretKeyRef](#secretkeyref)_ | tokenSecretRef references the Secret key containing the NetBox API<br />token used to authenticate. |  | Required: \{\} <br /> |
| `tls` _[TLSConfig](#tlsconfig)_ | tls controls TLS behavior when connecting to NetBox. |  | Optional: \{\} <br /> |
| `requestTimeoutSeconds` _integer_ | requestTimeoutSeconds bounds how long a single NetBox API request may<br />take before it is aborted. | 30 | Optional: \{\} <br /> |
| `versionCheck` _[VersionCheckConfig](#versioncheckconfig)_ | versionCheck controls the NetBox version compatibility check. |  | Optional: \{\} <br /> |
| `managedTag` _[ManagedTagConfig](#managedtagconfig)_ | managedTag controls the NetBox tag Herald uses to mark objects it owns. |  | Optional: \{\} <br /> |


#### NetBoxStatus



NetBoxStatus reports the last observed connectivity state of the
configured NetBox instance.



_Appears in:_
- [HeraldConfigStatus](#heraldconfigstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `connected` _boolean_ | connected reports whether the last connectivity check succeeded. |  | Optional: \{\} <br /> |
| `version` _string_ | version is the NetBox version last observed via its status endpoint. |  | Optional: \{\} <br /> |
| `lastCheckedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#time-v1-meta)_ | lastCheckedTime is when connectivity was last checked. |  | Optional: \{\} <br /> |


#### NodeMapping

_Underlying type:_ _string_

NodeMapping selects how Kubernetes Nodes are represented in NetBox.

_Validation:_
- Enum: [Device VirtualMachine]

_Appears in:_
- [NodesResourceConfig](#nodesresourceconfig)

| Field | Description |
| --- | --- |
| `Device` | NodeMappingDevice maps Nodes to NetBox DCIM Devices (bare-metal clusters).<br /> |
| `VirtualMachine` | NodeMappingVirtualMachine maps Nodes to NetBox Virtualization VirtualMachines.<br /> |


#### NodesResourceConfig



NodesResourceConfig controls syncing of Kubernetes Nodes into NetBox.



_Appears in:_
- [ResourcesConfig](#resourcesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls whether Nodes are synced to NetBox. | false | Optional: \{\} <br /> |
| `mapping` _[NodeMapping](#nodemapping)_ | mapping selects whether Nodes are represented as NetBox Devices<br />(bare-metal) or VirtualMachines. Required when enabled is true. |  | Enum: [Device VirtualMachine] <br />Optional: \{\} <br /> |
| `retainOnDisable` _boolean_ | retainOnDisable controls whether previously-synced NetBox objects for<br />this resource type are kept when it is disabled. Defaults to false, so<br />disabling deletes every NetBox object Herald manages for this resource<br />type; set to true to instead pause syncing and leave existing objects<br />untouched. | false | Optional: \{\} <br /> |
| `device` _[DeviceMappingConfig](#devicemappingconfig)_ | device configures Node-to-Device mapping. Only used when mapping is<br />"Device". |  | Optional: \{\} <br /> |
| `virtualMachine` _[VirtualMachineMappingConfig](#virtualmachinemappingconfig)_ | virtualMachine configures Node-to-VirtualMachine mapping. Only used<br />when mapping is "VirtualMachine". |  | Optional: \{\} <br /> |


#### PodCIDRRepresentation

_Underlying type:_ _string_

PodCIDRRepresentation selects how pod CIDRs are represented in NetBox IPAM.

_Validation:_
- Enum: [Prefix Aggregate]

_Appears in:_
- [PodCIDRsResourceConfig](#podcidrsresourceconfig)

| Field | Description |
| --- | --- |
| `Prefix` | PodCIDRRepresentationPrefix records each pod CIDR as an IPAM Prefix.<br /> |
| `Aggregate` | PodCIDRRepresentationAggregate records pod CIDRs as an IPAM Aggregate.<br /> |


#### PodCIDRsResourceConfig



PodCIDRsResourceConfig controls syncing of pod CIDRs into NetBox IPAM.



_Appears in:_
- [ResourcesConfig](#resourcesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls whether pod CIDRs are synced to NetBox. | false | Optional: \{\} <br /> |
| `retainOnDisable` _boolean_ | retainOnDisable controls whether previously-synced NetBox objects for<br />this resource type are kept when it is disabled. Defaults to false, so<br />disabling deletes every NetBox object Herald manages for this resource<br />type; set to true to instead pause syncing and leave existing objects<br />untouched. | false | Optional: \{\} <br /> |
| `representation` _[PodCIDRRepresentation](#podcidrrepresentation)_ | representation selects whether pod CIDRs are recorded as individual<br />IPAM Prefixes (one per Node) or a single cluster-wide IPAM Aggregate<br />per address family. | Prefix | Enum: [Prefix Aggregate] <br />Optional: \{\} <br /> |
| `rirName` _string_ | rirName is the name of a pre-existing NetBox RIR (Regional Internet<br />Registry) assigned to synced Aggregates. Required when representation<br />is "Aggregate"; NetBox's Aggregate model requires one (e.g. an<br />"RFC1918" RIR for private pod CIDR ranges). Unused when representation<br />is "Prefix". |  | Optional: \{\} <br /> |


#### ResourceStatuses



ResourceStatuses reports the observed sync state of every resource type.



_Appears in:_
- [HeraldConfigStatus](#heraldconfigstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cluster` _[ResourceSyncStatus](#resourcesyncstatus)_ | cluster reports the sync state of the cluster resource. |  | Optional: \{\} <br /> |
| `nodes` _[ResourceSyncStatus](#resourcesyncstatus)_ | nodes reports the sync state of the nodes resource. |  | Optional: \{\} <br /> |
| `services` _[ResourceSyncStatus](#resourcesyncstatus)_ | services reports the sync state of the services resource. |  | Optional: \{\} <br /> |
| `podCIDRs` _[ResourceSyncStatus](#resourcesyncstatus)_ | podCIDRs reports the sync state of the pod CIDRs resource. |  | Optional: \{\} <br /> |


#### ResourceSyncStatus



ResourceSyncStatus reports the observed sync state of a single resource
type.



_Appears in:_
- [ResourceStatuses](#resourcestatuses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled mirrors the resource type's current enabled state. |  | Optional: \{\} <br /> |
| `lastSyncTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#time-v1-meta)_ | lastSyncTime is when this resource type was last successfully synced. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | observedGeneration is the .metadata.generation of the HeraldConfig<br />last reconciled for this resource type. |  | Optional: \{\} <br /> |
| `objectCount` _integer_ | objectCount is the number of NetBox objects currently managed for this<br />resource type. |  | Optional: \{\} <br /> |
| `lastError` _string_ | lastError is the error message from the most recent failed sync<br />attempt, if any. Cleared on the next successful sync. |  | Optional: \{\} <br /> |


#### ResourcesConfig



ResourcesConfig controls which resource types Herald syncs to NetBox.
Every resource type can be individually enabled or disabled.



_Appears in:_
- [HeraldConfigSpec](#heraldconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cluster` _[ClusterResourceConfig](#clusterresourceconfig)_ | cluster controls syncing of the Kubernetes cluster itself. |  | Optional: \{\} <br /> |
| `nodes` _[NodesResourceConfig](#nodesresourceconfig)_ | nodes controls syncing of Kubernetes Nodes. |  | Optional: \{\} <br /> |
| `services` _[ServicesResourceConfig](#servicesresourceconfig)_ | services controls syncing of Kubernetes Services. |  | Optional: \{\} <br /> |
| `podCIDRs` _[PodCIDRsResourceConfig](#podcidrsresourceconfig)_ | podCIDRs controls syncing of pod CIDRs. |  | Optional: \{\} <br /> |
| `additional` _object (keys:string, values:[GenericResourceConfig](#genericresourceconfig))_ | additional holds configuration for experimental resource types that<br />have not yet been promoted to a typed field above, keyed by resource<br />name. |  | Optional: \{\} <br /> |


#### ResyncConfig



ResyncConfig controls periodic full resynchronization of every enabled
resource type, independent of watch-triggered reconciles.



_Appears in:_
- [HeraldConfigSpec](#heraldconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#duration-v1-meta)_ | interval is how often Herald performs a full resync of every enabled<br />resource type, in addition to reconciling on every relevant change. | 5m | Optional: \{\} <br /> |


#### SecretKeyRef



SecretKeyRef references a key within a Secret in the operator's namespace.



_Appears in:_
- [NetBoxConfig](#netboxconfig)
- [TLSConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the name of the referenced Secret. |  | Required: \{\} <br /> |
| `key` _string_ | key is the key within the Secret's data to read. | token | Optional: \{\} <br /> |


#### ServicesResourceConfig



ServicesResourceConfig controls syncing of Kubernetes Services into NetBox.

Only Services of type LoadBalancer with an assigned external IP are
synced, recorded as a NetBox IPAM IP Address with no Device/VM parent.
NetBox's Service model expects a single owning Device/VM, which Kubernetes
Services do not naturally have; ClusterIP and NodePort Services carry no
externally-meaningful address to document and are not synced.



_Appears in:_
- [ResourcesConfig](#resourcesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls whether LoadBalancer Services are synced to NetBox. | false | Optional: \{\} <br /> |
| `retainOnDisable` _boolean_ | retainOnDisable controls whether previously-synced NetBox objects for<br />this resource type are kept when it is disabled. Defaults to false, so<br />disabling deletes every NetBox object Herald manages for this resource<br />type; set to true to instead pause syncing and leave existing objects<br />untouched. | false | Optional: \{\} <br /> |


#### TLSConfig



TLSConfig controls TLS behavior when connecting to NetBox.



_Appears in:_
- [NetBoxConfig](#netboxconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `insecureSkipVerify` _boolean_ | insecureSkipVerify disables TLS certificate verification. Not recommended<br />outside of local development. | false | Optional: \{\} <br /> |
| `caBundleSecretRef` _[SecretKeyRef](#secretkeyref)_ | caBundleSecretRef references a Secret key containing a PEM-encoded CA<br />bundle to trust in addition to the system roots. |  | Optional: \{\} <br /> |


#### VersionCheckConfig



VersionCheckConfig controls the NetBox version compatibility check performed
at startup and on every HeraldConfig reconcile.



_Appears in:_
- [NetBoxConfig](#netboxconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls whether Herald verifies the connected NetBox instance's<br />version falls within its supported range before syncing any resources. | true | Optional: \{\} <br /> |


#### VirtualMachineMappingConfig



VirtualMachineMappingConfig configures Node-to-VirtualMachine mapping.



_Appears in:_
- [NodesResourceConfig](#nodesresourceconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterName` _string_ | clusterName is the name of the NetBox Cluster synced VirtualMachines<br />belong to. Defaults to resources.cluster.name when unset. |  | Optional: \{\} <br /> |
| `platformName` _string_ | platformName is the name of a pre-existing NetBox Platform assigned to<br />synced VirtualMachines. |  | Optional: \{\} <br /> |


