package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// InfrastructureTalosClusterMode declares whether the cluster is bootstrapped or imported.
// +kubebuilder:validation:Enum=bootstrap;import
type InfrastructureTalosClusterMode string

const (
	InfrastructureTalosClusterModeBootstrap InfrastructureTalosClusterMode = "bootstrap"
	InfrastructureTalosClusterModeImport    InfrastructureTalosClusterMode = "import"
)

// InfrastructureTalosClusterRole declares the role of the cluster in the Seam topology.
// Mandatory on mode=import.
// +kubebuilder:validation:Enum=management;tenant
type InfrastructureTalosClusterRole string

const (
	InfrastructureTalosClusterRoleManagement InfrastructureTalosClusterRole = "management"
	InfrastructureTalosClusterRoleTenant     InfrastructureTalosClusterRole = "tenant"
)

// InfrastructureTalosClusterOrigin records how the cluster came to exist.
// +kubebuilder:validation:Enum=bootstrapped;imported
type InfrastructureTalosClusterOrigin string

const (
	InfrastructureTalosClusterOriginBootstrapped InfrastructureTalosClusterOrigin = "bootstrapped"
	InfrastructureTalosClusterOriginImported     InfrastructureTalosClusterOrigin = "imported"
)

// InfrastructureProvider declares the infrastructure provider backing a TalosCluster.
// +kubebuilder:validation:Enum=native;capi;screen
type InfrastructureProvider string

const (
	// InfrastructureProviderNative is the default provider. The operator manages
	// cluster lifecycle directly: management cluster via a bootstrap Conductor Job
	// (capi.enabled=false), target clusters via the CAPI path (capi.enabled=true).
	InfrastructureProviderNative InfrastructureProvider = "native"

	// InfrastructureProviderCAPI is an explicit alias for the CAPI-backed target
	// cluster path. Functionally equivalent to InfrastructureProviderNative when
	// spec.capi.enabled=true. Reserved for future explicit-provider semantics.
	InfrastructureProviderCAPI InfrastructureProvider = "capi"

	// InfrastructureProviderScreen is reserved for the future Screen operator (INV-021).
	// No implementation until a Governor-approved ADR. When observed, the reconciler
	// sets ScreenProviderNotImplemented=True and halts.
	InfrastructureProviderScreen InfrastructureProvider = "screen"
)

// InfrastructureLocalObjectRef is a reference to a Kubernetes object by name and namespace.
type InfrastructureLocalObjectRef struct {
	// Name is the object name.
	Name string `json:"name"`

	// Namespace is the object namespace. May be empty for cluster-scoped objects.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// InfrastructureCAPICiliumPackRef is a reference to the cluster-specific Cilium ClusterPack.
// The pack is pre-compiled on the workstation and is cluster-endpoint-specific.
// platform-schema.md §2.3.
type InfrastructureCAPICiliumPackRef struct {
	// Name is the ClusterPack CR name for the Cilium pack.
	Name string `json:"name"`

	// Version is the ClusterPack version string.
	Version string `json:"version"`
}

// InfrastructureCAPIWorkerPool declares a worker node pool for a CAPI-managed target cluster.
// Each pool maps to a MachineDeployment + SeamInfrastructureMachineTemplate.
type InfrastructureCAPIWorkerPool struct {
	// Name is the pool identifier. Used as the MachineDeployment name suffix.
	Name string `json:"name"`

	// Replicas is the desired number of worker nodes in this pool.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// SeamInfrastructureMachineNames lists the SeamInfrastructureMachine CR names
	// pre-provisioned for this pool. One per node.
	// +optional
	SeamInfrastructureMachineNames []string `json:"seamInfrastructureMachineNames,omitempty"`
}

// InfrastructureCAPIControlPlaneConfig declares the control plane configuration for a CAPI
// target cluster.
type InfrastructureCAPIControlPlaneConfig struct {
	// Replicas is the desired number of control plane nodes.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// InfrastructureCAPIConfig holds CAPI integration settings for a target cluster.
// Only consulted when capi.enabled=true. platform-schema.md §5.
type InfrastructureCAPIConfig struct {
	// Enabled determines whether this TalosCluster uses the CAPI path.
	// True for all target clusters. False for the management cluster.
	Enabled bool `json:"enabled"`

	// TalosVersion is the Talos version to use for TalosConfigTemplate and
	// CABPT machineconfig generation. Required when Enabled=true.
	// +optional
	TalosVersion string `json:"talosVersion,omitempty"`

	// KubernetesVersion is the Kubernetes version for TalosControlPlane.
	// Required when Enabled=true.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// ControlPlane holds control plane configuration. Required when Enabled=true.
	// +optional
	ControlPlane *InfrastructureCAPIControlPlaneConfig `json:"controlPlane,omitempty"`

	// Workers is the list of worker node pools.
	// +optional
	Workers []InfrastructureCAPIWorkerPool `json:"workers,omitempty"`

	// CiliumPackRef references the cluster-specific Cilium ClusterPack.
	// Applied as the first pack after the CAPI cluster reaches Running state.
	// Required when Enabled=true. platform-schema.md §2.3.
	// +optional
	CiliumPackRef *InfrastructureCAPICiliumPackRef `json:"ciliumPackRef,omitempty"`
}

// InfrastructureTalosClusterSpec is the declared desired state of an InfrastructureTalosCluster.
// platform-schema.md §4.
// +kubebuilder:validation:XValidation:rule="self.mode != 'import' || (has(self.role) && self.role != '')",message="role is required when mode is import"
type InfrastructureTalosClusterSpec struct {
	// Mode declares whether this cluster is bootstrapped from scratch or imported.
	// +kubebuilder:validation:Enum=bootstrap;import
	Mode InfrastructureTalosClusterMode `json:"mode"`

	// Role declares the cluster role in the Seam topology. Mandatory on mode=import.
	// +kubebuilder:validation:Enum=management;tenant
	// +optional
	Role InfrastructureTalosClusterRole `json:"role,omitempty"`

	// TalosVersion is the Talos OS version for this cluster. Used by Conductor to select
	// a compatible runner image. INV-012.
	// +optional
	TalosVersion string `json:"talosVersion,omitempty"`

	// KubernetesVersion is the Kubernetes version for this cluster. Set from
	// bootstrap.kubernetesVersion or derived automatically from the Talos version
	// support matrix when not explicitly provided. Informational — upgrade CRs
	// (UpgradePolicy with UpgradeTypeKubernetes) govern the actual k8s version.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// VersionUpgrade, when set to true, triggers a cluster-level rolling Talos upgrade
	// to the version declared in spec.talosVersion. The platform reconciler creates an
	// UpgradePolicy CR automatically and clears this field after the UpgradePolicy is
	// created. Applicable only to cluster-wide Talos version upgrades — individual node
	// operations, etcd maintenance, and other day-2 operations are not affected.
	// For management clusters, the Conductor executor upgrades the leader node last and
	// the platform operator releases its lease before the leader node reboots.
	// +optional
	VersionUpgrade bool `json:"versionUpgrade,omitempty"`

	// ClusterEndpoint is the cluster VIP or primary API endpoint IP. Required on mode=import.
	// Optional for bootstrap mode (endpoint derived from bootstrap Job output).
	// +optional
	ClusterEndpoint string `json:"clusterEndpoint,omitempty"`

	// NodeAddresses is the list of node IPs belonging to this cluster. Used by
	// DSNSReconciler to populate A records in the seam DNS zone. platform-schema.md §5.
	// +optional
	NodeAddresses []string `json:"nodeAddresses,omitempty"`

	// CAPI holds CAPI integration settings. When absent, the cluster uses direct bootstrap.
	// +optional
	CAPI *InfrastructureCAPIConfig `json:"capi,omitempty"`

	// InfrastructureProvider declares the infrastructure provider backing this cluster.
	// Defaults to native when absent. The only reserved future value is screen (INV-021).
	// +kubebuilder:validation:Enum=native;capi;screen
	// +kubebuilder:default=native
	// +optional
	InfrastructureProvider InfrastructureProvider `json:"infrastructureProvider,omitempty"`

	// KubeconfigSecretRef is the name of the Secret containing the kubeconfig for this cluster.
	// Required on mode=import. Not used when CAPI manages the cluster lifecycle.
	// +optional
	KubeconfigSecretRef string `json:"kubeconfigSecretRef,omitempty"`

	// TalosconfigSecretRef is the name of the Secret containing the talosconfig for this cluster.
	// +optional
	TalosconfigSecretRef string `json:"talosconfigSecretRef,omitempty"`

	// Lineage is the sealed causal chain record for this root declaration. Immutable after creation.
	// +optional
	Lineage *lineage.SealedCausalChain `json:"lineage,omitempty"`

	// PkiRotationThresholdDays is the number of days before cert expiry at which
	// the TalosCluster reconciler auto-creates a PKIRotation CR. Default 0 means
	// use the platform operator built-in default (30 days). platform-schema.md §13.
	// +optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	PkiRotationThresholdDays int32 `json:"pkiRotationThresholdDays,omitempty"`

	// HardeningProfileRef references a HardeningProfile CR to apply at bootstrap.
	// ONT-native: platform creates a NodeMaintenance with operation=hardening-apply and
	// label ontai.dev/hardening-trigger=bootstrap in seam-tenant-{cluster} after Ready.
	// CAPI: MachineConfigPatches and SysctlParams are merged into TalosConfigTemplate at
	// provisioning time. platform-schema.md §11.
	// +optional
	HardeningProfileRef *InfrastructureLocalObjectRef `json:"hardeningProfileRef,omitempty"`
}

// InfrastructureTalosClusterStatus is the observed state of an InfrastructureTalosCluster.
type InfrastructureTalosClusterStatus struct {
	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Origin records how this cluster came under Seam governance.
	// +optional
	Origin InfrastructureTalosClusterOrigin `json:"origin,omitempty"`

	// ObservedTalosVersion is the Talos version last confirmed running by a successful
	// day-2 upgrade operation (UpgradePolicy). The platform reconciler uses this to
	// prevent spec.talosVersion from regressing the cluster below its current version.
	// Set after each successful talos-upgrade or stack-upgrade UpgradePolicy.
	// +optional
	ObservedTalosVersion string `json:"observedTalosVersion,omitempty"`

	// CAPIClusterRef is a reference to the owned CAPI Cluster object in the tenant
	// namespace. Only set for CAPI-managed clusters (capi.enabled=true).
	// +optional
	CAPIClusterRef *InfrastructureLocalObjectRef `json:"capiClusterRef,omitempty"`

	// Conditions is the list of status conditions for this TalosCluster.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PkiExpiryDate is the earliest certificate expiry across the talosconfig and
	// kubeconfig Secrets for this cluster. Set by the TalosCluster reconciler.
	// When this date approaches spec.pkiRotationThresholdDays, a PKIRotation CR
	// is created automatically. platform-schema.md §13.
	// +optional
	PkiExpiryDate *metav1.Time `json:"pkiExpiryDate,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=itc
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=".spec.mode"
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=".spec.role"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructureTalosCluster is the seam-core CRD for a Talos cluster under Seam governance.
// platform-schema.md §4. Decision H.
type InfrastructureTalosCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureTalosClusterSpec   `json:"spec,omitempty"`
	Status InfrastructureTalosClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureTalosClusterList contains a list of InfrastructureTalosCluster.
type InfrastructureTalosClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureTalosCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructureTalosCluster{}, &InfrastructureTalosClusterList{})
}
