package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Condition type constants for SeamMembership.
const (
	// ConditionTypeSeamMembershipAdmitted is true when Guardian has validated and
	// admitted this member to the Seam infrastructure family.
	ConditionTypeSeamMembershipAdmitted = "Admitted"

	// ConditionTypeSeamMembershipValidated is true when structural and
	// cross-reference validation passes (domainIdentityRef matches RBACProfile,
	// RBACProfile is provisioned).
	ConditionTypeSeamMembershipValidated = "Validated"
)

// Condition reason constants for SeamMembership.
const (
	// ReasonMembershipAdmitted is set when all validation passes and Admitted=true.
	ReasonMembershipAdmitted = "MembershipAdmitted"

	// ReasonDomainIdentityMismatch is set when spec.domainIdentityRef does not
	// match the domainIdentityRef on the operator's RBACProfile, or when no
	// RBACProfile with a matching principalRef can be found.
	ReasonDomainIdentityMismatch = "DomainIdentityMismatch"

	// ReasonPrincipalMismatch is set when spec.principalRef does not match any
	// RBACProfile principalRef in the same namespace.
	ReasonPrincipalMismatch = "PrincipalMismatch"

	// ReasonRBACProfileNotProvisioned is set when the matching RBACProfile has
	// not yet reached provisioned=true. The SeamMembershipReconciler requeues
	// until provisioning completes.
	ReasonRBACProfileNotProvisioned = "RBACProfileNotProvisioned"
)

// SeamMembershipSpec defines the desired state of a SeamMembership.
type SeamMembershipSpec struct {
	// AppIdentityRef references the operator's application-layer identity.
	// For Seam family operators this is the operator name (guardian, platform,
	// wrapper, conductor, seam-core, vortex).
	// Format: {name} — references an AppIdentity in the domain-core (future)
	// or the operator's service account name (current).
	AppIdentityRef string `json:"appIdentityRef"`

	// DomainIdentityRef references the DomainIdentity at core.ontai.dev
	// that this operator traces to. Must match the domainIdentityRef on
	// the operator's RBACProfile.
	DomainIdentityRef string `json:"domainIdentityRef"`

	// PrincipalRef is the Kubernetes service account that this operator
	// runs as. Format: system:serviceaccount:{namespace}:{name}
	// Must match the principalRef on the operator's RBACProfile.
	PrincipalRef string `json:"principalRef"`

	// Tier declares the membership tier. Valid values: infrastructure, application.
	// infrastructure: Seam family operators (guardian, platform, wrapper, etc.)
	// application: Application operators (vortex, future app operators)
	// +kubebuilder:validation:Enum=infrastructure;application
	Tier string `json:"tier"`
}

// SeamMembershipStatus defines the observed state of a SeamMembership.
type SeamMembershipStatus struct {
	// Admitted is true when Guardian has validated and admitted this member.
	// +optional
	Admitted bool `json:"admitted,omitempty"`

	// AdmittedAt is the timestamp when Guardian admitted this member.
	// +optional
	AdmittedAt *metav1.Time `json:"admittedAt,omitempty"`

	// PermissionSnapshotRef is the name of the PermissionSnapshot Guardian
	// resolved for this member. Set after admission.
	// +optional
	PermissionSnapshotRef string `json:"permissionSnapshotRef,omitempty"`

	// Conditions is the list of status conditions for this SeamMembership.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SeamMembership is the formal join declaration for an operator wishing to
// become a member of the Seam infrastructure family. Guardian validates and
// admits the membership after verifying the operator's RBACProfile. Operators
// that are not members may not be allocated PermissionSnapshots.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sm
// +kubebuilder:printcolumn:name="Admitted",type=boolean,JSONPath=`.status.admitted`
// +kubebuilder:printcolumn:name="Tier",type=string,JSONPath=`.spec.tier`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type SeamMembership struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SeamMembershipSpec   `json:"spec,omitempty"`
	Status SeamMembershipStatus `json:"status,omitempty"`
}

// SeamMembershipList is the list type for SeamMembership.
//
// +kubebuilder:object:root=true
type SeamMembershipList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SeamMembership `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SeamMembership{}, &SeamMembershipList{})
}
