// authorship_gate.go contains only pure functions and value types for the
// controller-authorship admission gate on InfrastructureLineageIndex.
//
// AUTHORSHIP CONTRACT: InfrastructureLineageIndex instances are controller-authored
// exclusively. No human operator, no automation pipeline, and no other controller
// may create or update them. The admission webhook rejects any CREATE or UPDATE
// whose requesting principal is not the LineageController ServiceAccount.
// CLAUDE.md §14 Decision 3.
package webhook

import "fmt"

// AuthorshipWebhookPath is the HTTP path at which the controller-authorship
// admission webhook is registered in the Seam Core webhook server.
const AuthorshipWebhookPath = "/validate-lineage-index-authorship"

// LineageControllerIdentity is the Kubernetes ServiceAccount username for the
// InfrastructureLineageController. In admission request UserInfo, ServiceAccount
// principals are identified as system:serviceaccount:<namespace>:<name>.
// The conceptual identity is governance.infrastructure.ontai.dev/lineage-controller
// per seam-core-schema.md §7 Declaration 4.
//
// The LineageController ServiceAccount is named "lineage-controller" and runs in
// the "seam-system" namespace. CLAUDE.md §14 Decision 3.
const LineageControllerIdentity = "system:serviceaccount:seam-system:lineage-controller"

// AuthorshipGateRequest is the input to EvaluateAuthorshipGate. It contains only
// the fields required for the authorship decision, decoupled from any Kubernetes
// API machinery.
type AuthorshipGateRequest struct {
	// Kind is the resource kind being admitted.
	Kind string
	// Operation is the admission operation type.
	Operation AdmissionOperation
	// RequestingUser is the UserInfo.Username from the admission request.
	// For ServiceAccounts this is system:serviceaccount:<namespace>:<name>.
	RequestingUser string
}

// AuthorshipGateDecision is the result of EvaluateAuthorshipGate.
type AuthorshipGateDecision struct {
	// Allowed indicates whether the request is permitted to proceed.
	Allowed bool
	// Reason is a human-readable explanation of the decision. Empty when Allowed=true.
	Reason string
}

// EvaluateAuthorshipGate applies the controller-authorship policy to an incoming
// admission request for InfrastructureLineageIndex. It is a pure function: no
// side effects, no Kubernetes API calls, no I/O.
//
// Evaluation order:
//  1. If Kind is not InfrastructureLineageIndex, allow unconditionally.
//  2. If the operation is DELETE, allow unconditionally.
//     The authorship gate covers CREATE and UPDATE only.
//  3. If RequestingUser matches LineageControllerIdentity, allow.
//  4. Otherwise, reject — the request is not from the authorized LineageController
//     ServiceAccount. CLAUDE.md §14 Decision 3.
func EvaluateAuthorshipGate(req AuthorshipGateRequest) AuthorshipGateDecision {
	if req.Kind != InfrastructureLineageIndexKind {
		return AuthorshipGateDecision{Allowed: true}
	}

	if req.Operation == OperationDelete {
		return AuthorshipGateDecision{Allowed: true}
	}

	if req.RequestingUser == LineageControllerIdentity {
		return AuthorshipGateDecision{Allowed: true}
	}

	return AuthorshipGateDecision{
		Allowed: false,
		Reason: fmt.Sprintf(
			"InfrastructureLineageIndex instances are controller-authored exclusively; "+
				"only the InfrastructureLineageController ServiceAccount (%q) may create or "+
				"update them — no human operator, automation pipeline, or other controller "+
				"is authorized to write InfrastructureLineageIndex resources "+
				"(CLAUDE.md §14 Decision 3, seam-core-schema.md §7 Declaration 4); "+
				"requesting user %q is not authorized",
			LineageControllerIdentity, req.RequestingUser,
		),
	}
}
