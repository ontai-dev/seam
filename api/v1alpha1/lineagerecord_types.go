// Package v1alpha1 LineageRecord is the infrastructure domain sealed causal chain
// index. Renamed from InfrastructureLineageIndex (MIGRATION-3.8).
//
// The LineageController manages the lifecycle of LineageRecord CRs. It is the
// sole principal permitted to create or update these instances per CLAUDE.md
// Decision 3.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ontai-dev/seam-core/pkg/conditions"
	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// ConditionTypeLineageSynced and ReasonLineageControllerAbsent are re-exported from
// pkg/conditions as the canonical source of truth. seam-core-schema.md §7
// Declaration 5. Consumers should prefer importing pkg/conditions directly.
const (
	ConditionTypeLineageSynced    = conditions.ConditionTypeLineageSynced
	ReasonLineageControllerAbsent = conditions.ReasonLineageControllerAbsent
)

// LineageRecordRootBinding records the root declaration that anchors this
// lineage record. All fields are immutable after admission.
type LineageRecordRootBinding struct {
	// RootKind is the kind of the root declaration (e.g., TalosCluster, PackDelivery).
	RootKind string `json:"rootKind"`

	// RootName is the name of the root declaration.
	RootName string `json:"rootName"`

	// RootNamespace is the namespace of the root declaration.
	RootNamespace string `json:"rootNamespace"`

	// RootUID is the UID of the root declaration at time of record creation.
	RootUID types.UID `json:"rootUID"`

	// RootObservedGeneration is the metadata.generation of the root declaration
	// when this record was created.
	RootObservedGeneration int64 `json:"rootObservedGeneration"`

	// DeclaringPrincipal is the identity of the human operator or automation
	// principal that applied the root declaration CR. Stamped by the admission
	// webhook via annotation infrastructure.ontai.dev/declaring-principal at
	// CREATE time. Immutable after rootBinding is sealed.
	// +optional
	DeclaringPrincipal string `json:"declaringPrincipal,omitempty"`
}

// DescendantEntry records a single derived object in the lineage record.
// Entries are appended monotonically. An entry is never modified or removed
// except by the retention enforcement loop.
type DescendantEntry struct {
	// Group is the API group of the derived object (e.g., platform.ontai.dev).
	Group string `json:"group"`

	// Version is the API version of the derived object (e.g., v1alpha1).
	Version string `json:"version"`

	// Kind is the kind of the derived object.
	Kind string `json:"kind"`

	// Name is the name of the derived object.
	Name string `json:"name"`

	// Namespace is the namespace of the derived object.
	Namespace string `json:"namespace"`

	// UID is the UID of the derived object.
	UID types.UID `json:"uid"`

	// SeamOperator is the name of the Seam Operator that created this derived object.
	SeamOperator string `json:"seamOperator"`

	// CreationRationale is the reason this derived object was created, drawn from
	// the Seam Core controlled vocabulary (pkg/lineage.CreationRationale).
	//
	// +kubebuilder:validation:Enum=ClusterProvision;ClusterDecommission;SecurityEnforcement;PackExecution;VirtualizationFulfillment;ConductorAssignment;VortexBinding
	CreationRationale lineage.CreationRationale `json:"creationRationale"`

	// RootGenerationAtCreation is the metadata.generation of the root declaration
	// at the time this derived object was created.
	RootGenerationAtCreation int64 `json:"rootGenerationAtCreation"`

	// CreatedAt is the time this descendant entry was appended to the registry.
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// ActorRef is the identity propagated from rootBinding.declaringPrincipal.
	// +optional
	ActorRef string `json:"actorRef,omitempty"`
}

// InfrastructurePolicyBindingStatus records the InfrastructurePolicy and
// InfrastructureProfile bound to the root declaration at last evaluation.
type InfrastructurePolicyBindingStatus struct {
	// DomainPolicyRef is the name of the InfrastructurePolicy bound to the root declaration.
	// +optional
	DomainPolicyRef string `json:"domainPolicyRef,omitempty"`

	// DomainProfileRef is the name of the InfrastructureProfile bound to the root declaration.
	// +optional
	DomainProfileRef string `json:"domainProfileRef,omitempty"`

	// PolicyGenerationAtLastEvaluation is the metadata.generation of the bound
	// InfrastructurePolicy at the time of the last policy evaluation cycle.
	// +optional
	PolicyGenerationAtLastEvaluation int64 `json:"policyGenerationAtLastEvaluation,omitempty"`

	// DriftDetected is true if drift was detected at the last evaluation.
	// +optional
	DriftDetected bool `json:"driftDetected,omitempty"`
}

// OutcomeType is the terminal lifecycle classification for a derived object.
//
// +kubebuilder:validation:Enum=Succeeded;Failed;Drifted;Superseded
type OutcomeType string

const (
	OutcomeTypeSucceeded  OutcomeType = "Succeeded"
	OutcomeTypeFailed     OutcomeType = "Failed"
	OutcomeTypeDrifted    OutcomeType = "Drifted"
	OutcomeTypeSuperseded OutcomeType = "Superseded"
)

// OutcomeEntry records the terminal outcome for a derived object tracked in
// DescendantRegistry. Entries are appended by LineageController when a terminal
// condition is observed. Entries are never modified or removed.
type OutcomeEntry struct {
	// DerivedObjectUID is the UID matching a derived object entry in DescendantRegistry.
	DerivedObjectUID types.UID `json:"derivedObjectUID"`

	// OutcomeType is the terminal classification of the derived object lifecycle.
	OutcomeType OutcomeType `json:"outcomeType"`

	// OutcomeTimestamp is the time when the terminal condition was observed.
	OutcomeTimestamp metav1.Time `json:"outcomeTimestamp"`

	// OutcomeRef is the name of the OperationResult or terminal condition reason.
	// +optional
	OutcomeRef string `json:"outcomeRef,omitempty"`

	// OutcomeDetail is a brief human-readable summary of the outcome.
	// +optional
	OutcomeDetail string `json:"outcomeDetail,omitempty"`
}

// LineageRetentionPolicy declares how stale descendant entries and the record
// itself are collected when the root declaration or its derived objects are deleted.
type LineageRetentionPolicy struct {
	// DescendantRetentionDays is the number of days a stale descendant entry is
	// retained after its referenced object is confirmed not-found.
	//
	// Defaults to 30. Minimum is 1.
	//
	// +optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	DescendantRetentionDays int32 `json:"descendantRetentionDays,omitempty"`

	// DeleteWithRoot controls whether this LineageRecord is garbage collected when
	// its root declaration is deleted.
	//
	// Defaults to true.
	//
	// +optional
	// +kubebuilder:default=true
	DeleteWithRoot bool `json:"deleteWithRoot"`
}

// LineageRecordSpec is the spec of a LineageRecord.
type LineageRecordSpec struct {
	// RootBinding records the root declaration that anchors this lineage record.
	// Immutable after admission.
	RootBinding LineageRecordRootBinding `json:"rootBinding"`

	// DomainRef references the DomainLineageIndex at core.ontai.dev that
	// this LineageRecord instantiates.
	// +kubebuilder:validation:Optional
	DomainRef string `json:"domainRef,omitempty"`

	// DescendantRegistry is the list of all objects derived from the root
	// declaration. Appended monotonically.
	// +optional
	DescendantRegistry []DescendantEntry `json:"descendantRegistry,omitempty"`

	// PolicyBindingStatus records the bound policy and profile at last evaluation.
	// +optional
	PolicyBindingStatus *InfrastructurePolicyBindingStatus `json:"policyBindingStatus,omitempty"`

	// OutcomeRegistry is the append-only registry of terminal outcomes for derived
	// objects tracked in DescendantRegistry.
	// +optional
	OutcomeRegistry []OutcomeEntry `json:"outcomeRegistry,omitempty"`

	// RetentionPolicy declares garbage collection behavior.
	// +optional
	RetentionPolicy *LineageRetentionPolicy `json:"retentionPolicy,omitempty"`
}

// LineageRecordStatus is the observed state of a LineageRecord.
type LineageRecordStatus struct {
	// ObservedGeneration is the last generation processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the standard Kubernetes condition array.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=lr
// +kubebuilder:printcolumn:name="RootKind",type=string,JSONPath=`.spec.rootBinding.rootKind`
// +kubebuilder:printcolumn:name="RootName",type=string,JSONPath=`.spec.rootBinding.rootName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LineageRecord is the sealed causal chain index for a root declaration in the
// Seam infrastructure domain. Renamed from InfrastructureLineageIndex (MIGRATION-3.8).
//
// One LineageRecord is created per root declaration by the LineageController.
// Controller-authored exclusively -- CLAUDE.md Decision 3.
type LineageRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LineageRecordSpec   `json:"spec,omitempty"`
	Status LineageRecordStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LineageRecordList contains a list of LineageRecord.
type LineageRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LineageRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LineageRecord{}, &LineageRecordList{})
}
