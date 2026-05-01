package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// InfrastructureLifecyclePolicy controls artifact retention behavior.
// wrapper-schema.md §3 ClusterPack spec.lifecyclePolicies.
type InfrastructureLifecyclePolicy struct {
	// RetainOnDeletion controls whether the OCI artifact is retained when the
	// ClusterPack CR is deleted. Default: true (artifact retained).
	// +optional
	// +kubebuilder:default=true
	RetainOnDeletion bool `json:"retainOnDeletion,omitempty"`
}

// InfrastructurePackRegistryRef identifies the OCI artifact for a ClusterPack.
type InfrastructurePackRegistryRef struct {
	// URL is the OCI registry URL including image name.
	URL string `json:"url"`

	// Digest is the OCI image digest (e.g., sha256:abc123...). Immutable after creation.
	Digest string `json:"digest"`
}

// InfrastructurePackExecutionStage is a single stage in the pack execution order.
type InfrastructurePackExecutionStage struct {
	// Name is the stage name. Must be one of: rbac, storage, stateful, stateless.
	// +kubebuilder:validation:Enum=rbac;storage;stateful;stateless
	Name string `json:"name"`

	// Manifests is the list of manifest names to apply in this stage.
	// +optional
	Manifests []string `json:"manifests,omitempty"`
}

// InfrastructurePackProvenance records build-time metadata for audit and traceability.
type InfrastructurePackProvenance struct {
	// BuildID is the CI/CD build identifier that produced this pack.
	// +optional
	BuildID string `json:"buildID,omitempty"`

	// BuildTimestamp is when the pack artifact was produced.
	// +optional
	BuildTimestamp *metav1.Time `json:"buildTimestamp,omitempty"`

	// SourceRef is the git reference (commit SHA or tag) from which the pack was built.
	// +optional
	SourceRef string `json:"sourceRef,omitempty"`
}

// InfrastructureClusterPackSpec defines the desired state of an InfrastructureClusterPack.
// All fields are immutable after creation. wrapper-schema.md §3.
type InfrastructureClusterPackSpec struct {
	// Version is the semantic version of this pack. Immutable after creation.
	Version string `json:"version"`

	// RegistryRef identifies the OCI artifact for this pack. Immutable after creation.
	RegistryRef InfrastructurePackRegistryRef `json:"registryRef"`

	// Checksum is the content-addressed checksum of the full artifact manifest set.
	// +optional
	Checksum string `json:"checksum,omitempty"`

	// RBACDigest is the OCI digest of the RBAC layer of this ClusterPack artifact.
	// Contains ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding manifests.
	// +optional
	RBACDigest string `json:"rbacDigest,omitempty"`

	// WorkloadDigest is the OCI digest of the workload layer of this ClusterPack artifact.
	// Applied after guardian RBACProfile reaches provisioned=true. wrapper-schema.md §4.
	// +optional
	WorkloadDigest string `json:"workloadDigest,omitempty"`

	// ClusterScopedDigest is the OCI digest of the cluster-scoped non-RBAC layer.
	// Applied after guardian RBAC intake and before workload manifests. wrapper-schema.md §4.
	// +optional
	ClusterScopedDigest string `json:"clusterScopedDigest,omitempty"`

	// SourceBuildRef is an opaque reference to the build that produced this pack. Informational.
	// +optional
	SourceBuildRef string `json:"sourceBuildRef,omitempty"`

	// ExecutionOrder defines the ordered stages in which pack manifests are applied.
	// +optional
	ExecutionOrder []InfrastructurePackExecutionStage `json:"executionOrder,omitempty"`

	// Provenance records build-time metadata for audit and traceability.
	// +optional
	Provenance *InfrastructurePackProvenance `json:"provenance,omitempty"`

	// BasePackName is the logical pack name shared across versions (e.g., "nginx-ingress").
	// +optional
	BasePackName string `json:"basePackName,omitempty"`

	// TargetClusters is the list of cluster names to which this ClusterPack should be delivered.
	// +optional
	TargetClusters []string `json:"targetClusters,omitempty"`

	// ChartVersion is the version of the Helm chart used to compile this pack.
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`

	// ChartURL is the URL of the Helm chart repository used to compile this pack.
	// +optional
	ChartURL string `json:"chartURL,omitempty"`

	// ChartName is the name of the Helm chart used to compile this pack.
	// +optional
	ChartName string `json:"chartName,omitempty"`

	// HelmVersion is the version of the Helm SDK used to render this pack.
	// +optional
	HelmVersion string `json:"helmVersion,omitempty"`

	// ValuesFile is the path to the values file used during pack compilation.
	// For Helm packs: the user-supplied values file merged with chart defaults at render time.
	// For kustomize/raw packs: the overlay or patch file applied during the external build.
	// Informational -- recorded so admins can trace which customization produced this artifact.
	// +optional
	ValuesFile string `json:"valuesFile,omitempty"`

	// LifecyclePolicies controls artifact retention behavior.
	// +optional
	LifecyclePolicies *InfrastructureLifecyclePolicy `json:"lifecyclePolicies,omitempty"`

	// Lineage is the sealed causal chain record for this root declaration.
	// Authored once at object creation time and immutable thereafter.
	// seam-core-schema.md §5, CLAUDE.md §14 Decision 1.
	// +optional
	Lineage *lineage.SealedCausalChain `json:"lineage,omitempty"`

	// RollbackToRevision instructs the wrapper to restore this ClusterPack to the
	// artifact version deployed at POR revision N-1. When set, the ClusterPackReconciler
	// reads PreviousClusterPackVersion/PreviousRBACDigest/PreviousWorkloadDigest from
	// the current POR, patches spec.version and spec.*Digest to those values, removes
	// the spec-checksum-snapshot annotation, and clears this field. Only one-step
	// rollback (to revision N-1) is supported per invocation. Set to 0 to disable.
	// Governor-controlled only. wrapper-schema.md §6.2. seam-core-schema.md §7.8.
	// +optional
	RollbackToRevision int64 `json:"rollbackToRevision,omitempty"`
}

// InfrastructureClusterPackStatus is the observed state of an InfrastructureClusterPack.
type InfrastructureClusterPackStatus struct {
	// Signed indicates whether the conductor signing loop has signed this pack.
	// +optional
	Signed bool `json:"signed,omitempty"`

	// PackSignature is the base64-encoded Ed25519 signature produced by the management cluster conductor.
	// +optional
	PackSignature string `json:"packSignature,omitempty"`

	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this ClusterPack.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=icp
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Signed",type=boolean,JSONPath=".status.signed"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructureClusterPack is the seam-core CRD for pack registration.
// Records an OCI artifact that has been compiled and is ready for runtime delivery.
// Spec is immutable after creation. wrapper-schema.md §3.
type InfrastructureClusterPack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureClusterPackSpec   `json:"spec,omitempty"`
	Status InfrastructureClusterPackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureClusterPackList contains a list of InfrastructureClusterPack.
type InfrastructureClusterPackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureClusterPack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructureClusterPack{}, &InfrastructureClusterPackList{})
}
