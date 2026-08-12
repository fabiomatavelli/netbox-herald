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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NodeMapping selects how Kubernetes Nodes are represented in NetBox.
// +kubebuilder:validation:Enum=Device;VirtualMachine
type NodeMapping string

const (
	// NodeMappingDevice maps Nodes to NetBox DCIM Devices (bare-metal clusters).
	NodeMappingDevice NodeMapping = "Device"
	// NodeMappingVirtualMachine maps Nodes to NetBox Virtualization VirtualMachines.
	NodeMappingVirtualMachine NodeMapping = "VirtualMachine"
)

// PodCIDRRepresentation selects how pod CIDRs are represented in NetBox IPAM.
// +kubebuilder:validation:Enum=Prefix;Aggregate
type PodCIDRRepresentation string

const (
	// PodCIDRRepresentationPrefix records each pod CIDR as an IPAM Prefix.
	PodCIDRRepresentationPrefix PodCIDRRepresentation = "Prefix"
	// PodCIDRRepresentationAggregate records pod CIDRs as an IPAM Aggregate.
	PodCIDRRepresentationAggregate PodCIDRRepresentation = "Aggregate"
)

// SecretKeyRef references a key within a Secret in the operator's namespace.
type SecretKeyRef struct {
	// name is the name of the referenced Secret.
	// +required
	Name string `json:"name"`

	// key is the key within the Secret's data to read.
	// +optional
	// +kubebuilder:default="token"
	Key string `json:"key,omitempty"`
}

// TLSConfig controls TLS behavior when connecting to NetBox.
type TLSConfig struct {
	// insecureSkipVerify disables TLS certificate verification. Not recommended
	// outside of local development.
	// +optional
	// +kubebuilder:default=false
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// caBundleSecretRef references a Secret key containing a PEM-encoded CA
	// bundle to trust in addition to the system roots.
	// +optional
	CABundleSecretRef *SecretKeyRef `json:"caBundleSecretRef,omitempty"`
}

// VersionCheckConfig controls the NetBox version compatibility check performed
// at startup and on every HeraldConfig reconcile.
type VersionCheckConfig struct {
	// enabled controls whether Herald verifies the connected NetBox instance's
	// version falls within its supported range before syncing any resources.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

// ManagedTagConfig controls the NetBox tag Herald uses to mark objects it owns.
type ManagedTagConfig struct {
	// slug is the slug of the NetBox tag applied to every object Herald
	// creates, and used to identify objects safe for Herald to update or
	// delete. Herald never modifies NetBox objects lacking this tag.
	// +optional
	// +kubebuilder:default="netbox-herald-managed"
	Slug string `json:"slug,omitempty"`
}

// NetBoxConfig describes how to connect to the target NetBox instance.
type NetBoxConfig struct {
	// url is the base URL of the NetBox instance, e.g. https://netbox.example.com.
	// +required
	URL string `json:"url"`

	// tokenSecretRef references the Secret key containing the NetBox API
	// token used to authenticate.
	// +required
	TokenSecretRef SecretKeyRef `json:"tokenSecretRef"`

	// tls controls TLS behavior when connecting to NetBox.
	// +optional
	TLS TLSConfig `json:"tls,omitempty"`

	// requestTimeoutSeconds bounds how long a single NetBox API request may
	// take before it is aborted.
	// +optional
	// +kubebuilder:default=30
	RequestTimeoutSeconds int32 `json:"requestTimeoutSeconds,omitempty"`

	// versionCheck controls the NetBox version compatibility check.
	// +optional
	VersionCheck VersionCheckConfig `json:"versionCheck,omitempty"`

	// managedTag controls the NetBox tag Herald uses to mark objects it owns.
	// +optional
	ManagedTag ManagedTagConfig `json:"managedTag,omitempty"`
}

// ResyncConfig controls periodic full resynchronization of every enabled
// resource type, independent of watch-triggered reconciles.
type ResyncConfig struct {
	// interval is how often Herald performs a full resync of every enabled
	// resource type, in addition to reconciling on every relevant change.
	// +optional
	// +kubebuilder:default="5m"
	Interval metav1.Duration `json:"interval,omitempty"`
}

// ClusterResourceConfig controls syncing of the Kubernetes cluster itself
// into a NetBox Virtualization Cluster.
type ClusterResourceConfig struct {
	// enabled controls whether the cluster resource is synced to NetBox.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// name is the name given to the Cluster object in NetBox.
	// +optional
	Name string `json:"name,omitempty"`

	// clusterTypeName is the name of a pre-existing NetBox Cluster Type.
	// Herald does not create Cluster Types; they must already exist.
	// +optional
	ClusterTypeName string `json:"clusterTypeName,omitempty"`

	// clusterGroupName is the name of a pre-existing NetBox Cluster Group.
	// +optional
	ClusterGroupName string `json:"clusterGroupName,omitempty"`

	// siteName is the name of a pre-existing NetBox Site to associate with
	// the cluster.
	// +optional
	SiteName string `json:"siteName,omitempty"`
}

// DeviceMappingConfig configures Node-to-Device mapping.
type DeviceMappingConfig struct {
	// deviceRoleName is the name of a pre-existing NetBox Device Role assigned
	// to synced Devices.
	// +optional
	DeviceRoleName string `json:"deviceRoleName,omitempty"`

	// deviceTypeName is the name of a pre-existing NetBox Device Type assigned
	// to synced Devices.
	// +optional
	DeviceTypeName string `json:"deviceTypeName,omitempty"`

	// siteName is the name of a pre-existing NetBox Site assigned to synced
	// Devices.
	// +optional
	SiteName string `json:"siteName,omitempty"`
}

// VirtualMachineMappingConfig configures Node-to-VirtualMachine mapping.
type VirtualMachineMappingConfig struct {
	// clusterName is the name of the NetBox Cluster synced VirtualMachines
	// belong to. Defaults to resources.cluster.name when unset.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// platformName is the name of a pre-existing NetBox Platform assigned to
	// synced VirtualMachines.
	// +optional
	PlatformName string `json:"platformName,omitempty"`
}

// NodesResourceConfig controls syncing of Kubernetes Nodes into NetBox.
type NodesResourceConfig struct {
	// enabled controls whether Nodes are synced to NetBox.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// mapping selects whether Nodes are represented as NetBox Devices
	// (bare-metal) or VirtualMachines. Required when enabled is true.
	// +optional
	Mapping NodeMapping `json:"mapping,omitempty"`

	// retainOnDisable controls whether previously-synced NetBox objects for
	// this resource type are kept when it is disabled. Defaults to false, so
	// disabling deletes every NetBox object Herald manages for this resource
	// type; set to true to instead pause syncing and leave existing objects
	// untouched.
	// +optional
	// +kubebuilder:default=false
	RetainOnDisable bool `json:"retainOnDisable,omitempty"`

	// device configures Node-to-Device mapping. Only used when mapping is
	// "Device".
	// +optional
	Device DeviceMappingConfig `json:"device,omitempty"`

	// virtualMachine configures Node-to-VirtualMachine mapping. Only used
	// when mapping is "VirtualMachine".
	// +optional
	VirtualMachine VirtualMachineMappingConfig `json:"virtualMachine,omitempty"`
}

// ServicesResourceConfig controls syncing of Kubernetes Services into NetBox.
//
// Only Services of type LoadBalancer with an assigned external IP are
// synced, recorded as a NetBox IPAM IP Address with no Device/VM parent.
// NetBox's Service model expects a single owning Device/VM, which Kubernetes
// Services do not naturally have; ClusterIP and NodePort Services carry no
// externally-meaningful address to document and are not synced.
type ServicesResourceConfig struct {
	// enabled controls whether LoadBalancer Services are synced to NetBox.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// retainOnDisable controls whether previously-synced NetBox objects for
	// this resource type are kept when it is disabled. Defaults to false, so
	// disabling deletes every NetBox object Herald manages for this resource
	// type; set to true to instead pause syncing and leave existing objects
	// untouched.
	// +optional
	// +kubebuilder:default=false
	RetainOnDisable bool `json:"retainOnDisable,omitempty"`
}

// PodCIDRsResourceConfig controls syncing of pod CIDRs into NetBox IPAM.
type PodCIDRsResourceConfig struct {
	// enabled controls whether pod CIDRs are synced to NetBox.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// retainOnDisable controls whether previously-synced NetBox objects for
	// this resource type are kept when it is disabled. Defaults to false, so
	// disabling deletes every NetBox object Herald manages for this resource
	// type; set to true to instead pause syncing and leave existing objects
	// untouched.
	// +optional
	// +kubebuilder:default=false
	RetainOnDisable bool `json:"retainOnDisable,omitempty"`

	// representation selects whether pod CIDRs are recorded as individual
	// IPAM Prefixes (one per Node) or a single cluster-wide IPAM Aggregate
	// per address family.
	// +optional
	// +kubebuilder:default=Prefix
	Representation PodCIDRRepresentation `json:"representation,omitempty"`

	// rirName is the name of a pre-existing NetBox RIR (Regional Internet
	// Registry) assigned to synced Aggregates. Required when representation
	// is "Aggregate"; NetBox's Aggregate model requires one (e.g. an
	// "RFC1918" RIR for private pod CIDR ranges). Unused when representation
	// is "Prefix".
	// +optional
	RIRName string `json:"rirName,omitempty"`
}

// GenericResourceConfig is a minimal enable/disable toggle for resource
// types that have not yet been promoted to a fully-typed field under
// ResourcesConfig. It exists so new resource types can be experimented with
// without a breaking CRD schema change; once a resource type stabilizes it
// should be promoted to its own typed field.
type GenericResourceConfig struct {
	// enabled controls whether this resource type is synced to NetBox.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// retainOnDisable controls whether previously-synced NetBox objects for
	// this resource type are kept when it is disabled. Defaults to false, so
	// disabling deletes every NetBox object Herald manages for this resource
	// type; set to true to instead pause syncing and leave existing objects
	// untouched.
	// +optional
	// +kubebuilder:default=false
	RetainOnDisable bool `json:"retainOnDisable,omitempty"`
}

// ResourcesConfig controls which resource types Herald syncs to NetBox.
// Every resource type can be individually enabled or disabled.
type ResourcesConfig struct {
	// cluster controls syncing of the Kubernetes cluster itself.
	// +optional
	Cluster ClusterResourceConfig `json:"cluster,omitempty"`

	// nodes controls syncing of Kubernetes Nodes.
	// +optional
	Nodes NodesResourceConfig `json:"nodes,omitempty"`

	// services controls syncing of Kubernetes Services.
	// +optional
	Services ServicesResourceConfig `json:"services,omitempty"`

	// podCIDRs controls syncing of pod CIDRs.
	// +optional
	PodCIDRs PodCIDRsResourceConfig `json:"podCIDRs,omitempty"`

	// additional holds configuration for experimental resource types that
	// have not yet been promoted to a typed field above, keyed by resource
	// name.
	// +optional
	Additional map[string]GenericResourceConfig `json:"additional,omitempty"`
}

// HeraldConfigSpec defines the desired state of HeraldConfig.
type HeraldConfigSpec struct {
	// netbox describes how to connect to the target NetBox instance.
	// +required
	NetBox NetBoxConfig `json:"netbox"`

	// resync controls periodic full resynchronization.
	// +optional
	Resync ResyncConfig `json:"resync,omitempty"`

	// resources controls which resource types are synced to NetBox, with
	// per-type configuration.
	// +optional
	Resources ResourcesConfig `json:"resources,omitempty"`
}

// NetBoxStatus reports the last observed connectivity state of the
// configured NetBox instance.
type NetBoxStatus struct {
	// connected reports whether the last connectivity check succeeded.
	// +optional
	Connected bool `json:"connected,omitempty"`

	// version is the NetBox version last observed via its status endpoint.
	// +optional
	Version string `json:"version,omitempty"`

	// lastCheckedTime is when connectivity was last checked.
	// +optional
	LastCheckedTime *metav1.Time `json:"lastCheckedTime,omitempty"`
}

// ResourceSyncStatus reports the observed sync state of a single resource
// type.
type ResourceSyncStatus struct {
	// enabled mirrors the resource type's current enabled state.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// lastSyncTime is when this resource type was last successfully synced.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// observedGeneration is the .metadata.generation of the HeraldConfig
	// last reconciled for this resource type.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// objectCount is the number of NetBox objects currently managed for this
	// resource type.
	// +optional
	ObjectCount int32 `json:"objectCount,omitempty"`

	// lastError is the error message from the most recent failed sync
	// attempt, if any. Cleared on the next successful sync.
	// +optional
	LastError string `json:"lastError,omitempty"`
}

// ResourceStatuses reports the observed sync state of every resource type.
type ResourceStatuses struct {
	// cluster reports the sync state of the cluster resource.
	// +optional
	Cluster ResourceSyncStatus `json:"cluster,omitempty"`

	// nodes reports the sync state of the nodes resource.
	// +optional
	Nodes ResourceSyncStatus `json:"nodes,omitempty"`

	// services reports the sync state of the services resource.
	// +optional
	Services ResourceSyncStatus `json:"services,omitempty"`

	// podCIDRs reports the sync state of the pod CIDRs resource.
	// +optional
	PodCIDRs ResourceSyncStatus `json:"podCIDRs,omitempty"`
}

// HeraldConfigStatus defines the observed state of HeraldConfig.
type HeraldConfigStatus struct {
	// conditions represent the current state of the HeraldConfig resource.
	// Each condition has a unique type and reflects the status of a specific
	// aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": Herald is connected to NetBox and syncing enabled resources
	// - "NetBoxReachable": the configured NetBox instance is reachable and
	//   its version is compatible
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// netbox reports the last observed connectivity state of the configured
	// NetBox instance.
	// +optional
	NetBox NetBoxStatus `json:"netbox,omitempty"`

	// resources reports the observed sync state of every resource type.
	// +optional
	Resources ResourceStatuses `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="NetBox Connected",type="boolean",JSONPath=".status.netbox.connected"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// HeraldConfig is the Schema for the heraldconfigs API. It is cluster-scoped
// and singleton: Herald only reconciles the HeraldConfig named "default".
type HeraldConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of HeraldConfig
	// +required
	Spec HeraldConfigSpec `json:"spec"`

	// status defines the observed state of HeraldConfig
	// +optional
	Status HeraldConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// HeraldConfigList contains a list of HeraldConfig
type HeraldConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HeraldConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &HeraldConfig{}, &HeraldConfigList{})
		return nil
	})
}
