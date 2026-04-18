// domainref_validation.go contains pure functions and value types for the
// spec.domainRef validation gate on InfrastructureLineageIndex.
//
// DOMAINREF CONTRACT: When spec.domainRef is set on a new InfrastructureLineageIndex,
// it must equal the canonical infrastructure domain reference ("infrastructure.core.ontai.dev").
// Unknown values are rejected immediately. Empty is permitted — the LineageController
// will set the correct value during reconciliation.
//
// seam-core-schema.md §3, CLAUDE.md §14 Decision 2.
package webhook

import "fmt"

// DomainRefWebhookPath is the HTTP path at which the domainRef validation
// admission webhook is registered in the Seam Core webhook server.
const DomainRefWebhookPath = "/validate-lineage-index-domainref"

// ValidInfrastructureDomainRef is the only accepted non-empty value for
// spec.domainRef on InfrastructureLineageIndex instances. It is the
// {name}.{group} reference to the DomainLineageIndex at core.ontai.dev
// that this infrastructure ILI instantiates. CLAUDE.md §14 Decision 2.
const ValidInfrastructureDomainRef = "infrastructure.core.ontai.dev"

// DomainRefValidationRequest is the input to EvaluateDomainRefValidation.
type DomainRefValidationRequest struct {
	// Kind is the resource kind being admitted.
	Kind string
	// Operation is the admission operation type.
	Operation AdmissionOperation
	// DomainRef is the value of spec.domainRef from the incoming object.
	// Empty string means the field is absent.
	DomainRef string
}

// DomainRefValidationDecision is the result of EvaluateDomainRefValidation.
type DomainRefValidationDecision struct {
	// Allowed indicates whether the request is permitted to proceed.
	Allowed bool
	// Reason is a human-readable explanation of the decision. Empty when Allowed=true.
	Reason string
}

// EvaluateDomainRefValidation validates spec.domainRef on CREATE requests for
// InfrastructureLineageIndex. It is a pure function: no side effects, no
// Kubernetes API calls, no I/O.
//
// Evaluation order:
//  1. If Kind is not InfrastructureLineageIndex, allow unconditionally.
//  2. If the operation is not CREATE, allow unconditionally.
//     domainRef is authored at creation time; updates are not re-validated here.
//  3. If domainRef is empty, allow — the LineageController will populate it.
//  4. If domainRef equals ValidInfrastructureDomainRef, allow.
//  5. Otherwise, reject — unknown domain ref value.
func EvaluateDomainRefValidation(req DomainRefValidationRequest) DomainRefValidationDecision {
	if req.Kind != InfrastructureLineageIndexKind {
		return DomainRefValidationDecision{Allowed: true}
	}
	if req.Operation != OperationCreate {
		return DomainRefValidationDecision{Allowed: true}
	}
	if req.DomainRef == "" {
		return DomainRefValidationDecision{Allowed: true}
	}
	if req.DomainRef == ValidInfrastructureDomainRef {
		return DomainRefValidationDecision{Allowed: true}
	}
	return DomainRefValidationDecision{
		Allowed: false,
		Reason: fmt.Sprintf(
			"spec.domainRef must be %q for infrastructure domain ILIs; "+
				"unknown domain ref %q — the only valid traceability link for "+
				"InfrastructureLineageIndex is the infrastructure domain root at core.ontai.dev "+
				"(seam-core-schema.md §3, CLAUDE.md §14 Decision 2)",
			ValidInfrastructureDomainRef, req.DomainRef,
		),
	}
}
