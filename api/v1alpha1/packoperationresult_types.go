package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackOperationResultStatus constants mirror runnerlib.ResultStatus. Defined
// here to avoid importing conductor into seam-core (would create a cycle).

// PackResultStatus is the terminal status of a pack-deploy capability execution.
// +kubebuilder:validation:Enum=Succeeded;Failed
type PackResultStatus string

const (
	PackResultSucceeded PackResultStatus = "Succeeded"
	PackResultFailed    PackResultStatus = "Failed"
)

// PackUpgradeDirection records the version transition direction for a PackInstance.
// +kubebuilder:validation:Enum=Initial;Upgrade;Rollback;Redeploy
type PackUpgradeDirection string

const (
	PackUpgradeDirectionInitial  PackUpgradeDirection = "Initial"
	PackUpgradeDirectionUpgrade  PackUpgradeDirection = "Upgrade"
	PackUpgradeDirectionRollback PackUpgradeDirection = "Rollback"
	PackUpgradeDirectionRedeploy PackUpgradeDirection = "Redeploy"
)

// PackOperationDeployedResource records a single Kubernetes resource applied
// during a pack-deploy execution.
type PackOperationDeployedResource struct {
	// APIVersion is the Kubernetes apiVersion (e.g., apps/v1, v1).
	APIVersion string `json:"apiVersion"`

	// Kind is the Kubernetes resource Kind (e.g., Deployment, Namespace).
	Kind string `json:"kind"`

	// Namespace is the resource namespace. Empty for cluster-scoped resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the resource name.
	Name string `json:"name"`
}

// PackOperationArtifact is a structured reference to an artifact produced
// by a pack-deploy execution. Never contains raw artifact content.
type PackOperationArtifact struct {
	// Name is a logical identifier for this artifact.
	Name string `json:"name"`

	// Kind declares the artifact type. One of: ConfigMap, Secret, OCIImage, S3Object.
	// +kubebuilder:validation:Enum=ConfigMap;Secret;OCIImage;S3Object
	Kind string `json:"kind"`

	// Reference is the fully qualified reference for the artifact kind.
	Reference string `json:"reference"`

	// Checksum is the content-addressed checksum. Format: sha256:<hex>.
	// +optional
	Checksum string `json:"checksum,omitempty"`
}

// PackOperationStepResult is the execution result for one step within a
// multi-step capability.
type PackOperationStepResult struct {
	// Name is the step identifier within the capability.
	Name string `json:"name"`

	// Status is the terminal status of this step.
	// +kubebuilder:validation:Enum=Succeeded;Failed
	Status PackResultStatus `json:"status"`

	// StartedAt is the time this step began execution.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is the time this step finished execution.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Message provides additional context about the step outcome.
	// +optional
	Message string `json:"message,omitempty"`
}

// PackOperationFailureReason is a structured failure description.
type PackOperationFailureReason struct {
	// Category classifies the failure domain.
	// +kubebuilder:validation:Enum=ValidationFailure;CapabilityUnavailable;ExecutionFailure;ExternalDependencyFailure;InvariantViolation;LicenseViolation;StorageUnavailable
	Category string `json:"category"`

	// Reason is a human-readable description of the specific failure.
	Reason string `json:"reason"`

	// FailedStep is the name of the step that failed. Empty for single-step capabilities.
	// +optional
	FailedStep string `json:"failedStep,omitempty"`
}

// PackOperationResultSpec is the complete result document written by the
// Conductor execute-mode Job before exit. Written by conductor; read by wrapper.
// seam-core-schema.md §8, Decision 11.
type PackOperationResultSpec struct {
	// Revision is the monotonically increasing revision counter for this pack operation
	// sequence. Incremented each time a new result supersedes the previous one. The
	// single-active-revision pattern ensures exactly one PackOperationResult exists per
	// ClusterPack operation sequence at any time.
	Revision int64 `json:"revision"`

	// PreviousRevisionRef is the name of the PackOperationResult CR that was deleted when
	// this revision was written. Enables chain reconstruction for the full operation history.
	// Absent for revision 1 (no predecessor).
	// +optional
	PreviousRevisionRef string `json:"previousRevisionRef,omitempty"`

	// TalosClusterOperationResultRef is reserved for future cross-reference to a
	// TalosCluster-scoped OperationResult. Stub field; not populated by any current
	// controller.
	// +optional
	TalosClusterOperationResultRef string `json:"talosClusterOperationResultRef,omitempty"`

	// PackExecutionRef is the name of the PackExecution CR that triggered this operation.
	// +optional
	PackExecutionRef string `json:"packExecutionRef,omitempty"`

	// ClusterPackRef is the name of the ClusterPack CR that was deployed.
	// +optional
	ClusterPackRef string `json:"clusterPackRef,omitempty"`

	// TargetClusterRef is the name of the target cluster this operation ran against.
	// +optional
	TargetClusterRef string `json:"targetClusterRef,omitempty"`

	// Capability is the name of the Conductor capability that produced this result.
	Capability string `json:"capability"`

	// Phase identifies the RunnerConfig phase this result belongs to.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Status is the terminal status of the capability execution.
	// +kubebuilder:validation:Enum=Succeeded;Failed
	Status PackResultStatus `json:"status"`

	// StartedAt is the time the capability execution began.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is the time the capability execution finished.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// FailureReason is populated when Status is Failed. Nil on success.
	// +optional
	FailureReason *PackOperationFailureReason `json:"failureReason,omitempty"`

	// DeployedResources is the list of Kubernetes resources applied during this
	// execution. Populated by pack-deploy on success. Used by PackInstanceReconciler
	// for deletion cleanup.
	// +optional
	DeployedResources []PackOperationDeployedResource `json:"deployedResources,omitempty"`

	// Artifacts is the list of artifacts produced by this execution.
	// +optional
	Artifacts []PackOperationArtifact `json:"artifacts,omitempty"`

	// Steps contains individual step results for multi-step capabilities.
	// +optional
	Steps []PackOperationStepResult `json:"steps,omitempty"`

	// ClusterPackVersion is the ClusterPack spec.version deployed in this operation.
	// Populated by the Conductor executor at write time. Rollback anchor: the wrapper
	// reads this field from the target POR revision to restore when spec.rollbackToRevision
	// is set. Superseded PORs are retained with ontai.dev/superseded=true label to
	// enable N-step rollback. seam-core-schema.md §7.8.
	// +optional
	ClusterPackVersion string `json:"clusterPackVersion,omitempty"`

	// RBACDigest is the OCI digest of the RBAC layer deployed in this operation.
	// Copied from ClusterPack.spec.rbacDigest at deploy time. Rollback anchor.
	// +optional
	RBACDigest string `json:"rbacDigest,omitempty"`

	// WorkloadDigest is the OCI digest of the workload layer deployed in this operation.
	// Copied from ClusterPack.spec.workloadDigest at deploy time. Rollback anchor.
	// +optional
	WorkloadDigest string `json:"workloadDigest,omitempty"`

}

// PackOperationResultStatus is the observed state of a PackOperationResult.
// Currently empty; reserved for future controller-set conditions.
type PackOperationResultStatus struct {
	// ObservedGeneration is the last generation processed by any consumer.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=por
// +kubebuilder:printcolumn:name="Capability",type=string,JSONPath=`.spec.capability`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.spec.status`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.targetClusterRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PackOperationResult is the immutable result record written by the Conductor
// execute-mode Job after a pack-deploy capability completes. It replaces the
// ConfigMap output channel, providing a versioned, richly-typed CR that the
// wrapper PackExecutionReconciler reads to advance PackExecution status and
// that the lineagesink can consume as additional data. One PackOperationResult
// per PackExecution, created in namespace seam-tenant-{clusterName}.
// seam-core-schema.md §8, Decision 11.
type PackOperationResult struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackOperationResultSpec   `json:"spec,omitempty"`
	Status PackOperationResultStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PackOperationResultList contains a list of PackOperationResult.
type PackOperationResultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackOperationResult `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PackOperationResult{}, &PackOperationResultList{})
}
