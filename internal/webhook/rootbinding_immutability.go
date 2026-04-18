// Package webhook provides admission decision logic and server registration for
// the Seam Core admission webhook.
//
// This file (rootbinding_immutability.go) contains only pure functions and
// value types for the spec.rootBinding immutability gate on
// InfrastructureLineageIndex. It has no imports from
// sigs.k8s.io/controller-runtime/pkg/webhook.
//
// IMMUTABILITY CONTRACT: spec.rootBinding on InfrastructureLineageIndex is
// authored once at creation time and sealed permanently. The admission webhook
// rejects any UPDATE request that modifies any field in spec.rootBinding.
// seam-core-schema.md §3.1, domain-core-schema.md §2.1.
package webhook

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// RootBindingWebhookPath is the HTTP path at which the rootBinding immutability
// admission webhook is registered in the Seam Core webhook server.
const RootBindingWebhookPath = "/validate-lineage-index-immutability"

// InfrastructureLineageIndexKind is the kind name for InfrastructureLineageIndex.
// The immutability gate intercepts only this kind.
const InfrastructureLineageIndexKind = "InfrastructureLineageIndex"

// AdmissionOperation is the type of operation for an incoming admission request.
type AdmissionOperation string

const (
	// OperationCreate represents a resource creation request.
	OperationCreate AdmissionOperation = "CREATE"
	// OperationUpdate represents a resource update request.
	OperationUpdate AdmissionOperation = "UPDATE"
	// OperationDelete represents a resource deletion request.
	OperationDelete AdmissionOperation = "DELETE"
)

// RootBindingImmutabilityRequest is the input to EvaluateRootBindingImmutability.
// It contains only the fields required for the immutability decision, decoupled
// from any Kubernetes API machinery.
type RootBindingImmutabilityRequest struct {
	// Kind is the resource kind being admitted (e.g., "InfrastructureLineageIndex").
	Kind string
	// Operation is the admission operation type.
	Operation AdmissionOperation
	// OldRootBindingRaw is the raw JSON bytes of spec.rootBinding from the
	// existing (old) object. Nil or empty if the field was absent.
	OldRootBindingRaw []byte
	// NewRootBindingRaw is the raw JSON bytes of spec.rootBinding from the
	// incoming (new) object. Nil or empty if the field is absent.
	NewRootBindingRaw []byte
}

// RootBindingImmutabilityDecision is the result of EvaluateRootBindingImmutability.
type RootBindingImmutabilityDecision struct {
	// Allowed indicates whether the request is permitted to proceed.
	Allowed bool
	// Reason is a human-readable explanation of the decision. Empty when Allowed=true.
	Reason string
}

// EvaluateRootBindingImmutability applies the spec.rootBinding immutability policy
// to an incoming admission request for InfrastructureLineageIndex. It is a pure
// function: no side effects, no Kubernetes API calls, no I/O.
//
// Evaluation order:
//  1. If Kind is not InfrastructureLineageIndex, allow unconditionally.
//  2. If the operation is not UPDATE (CREATE, DELETE), allow unconditionally.
//     rootBinding is authored at CREATE time. DELETE is always permitted.
//  3. If old and new spec.rootBinding are semantically equal (both absent, or
//     both present and structurally identical), allow.
//  4. Otherwise, reject — spec.rootBinding has been modified.
//     seam-core-schema.md §3.1, domain-core-schema.md §2.1.
func EvaluateRootBindingImmutability(req RootBindingImmutabilityRequest) RootBindingImmutabilityDecision {
	if req.Kind != InfrastructureLineageIndexKind {
		return RootBindingImmutabilityDecision{Allowed: true}
	}

	if req.Operation != OperationUpdate {
		return RootBindingImmutabilityDecision{Allowed: true}
	}

	if rootBindingRawEqual(req.OldRootBindingRaw, req.NewRootBindingRaw) {
		return RootBindingImmutabilityDecision{Allowed: true}
	}

	return RootBindingImmutabilityDecision{
		Allowed: false,
		Reason: fmt.Sprintf(
			"spec.rootBinding on %s is immutable after creation and cannot be modified; "+
				"the rootBinding section identifies the root declaration that anchors this "+
				"lineage index and is sealed at admission — any field change is rejected "+
				"(seam-core-schema.md §3.1, domain-core-schema.md §2.1); "+
				"to record a different root binding, create a new InfrastructureLineageIndex",
			InfrastructureLineageIndexKind,
		),
	}
}

// rootBindingRawEqual reports whether two raw JSON rootBinding values are
// semantically equal. Both absent is equal. One absent and one present is not
// equal. Both present: unmarshal to interface{} and use reflect.DeepEqual for
// structural equality regardless of byte-level formatting differences.
func rootBindingRawEqual(a, b []byte) bool {
	aEmpty := len(a) == 0 || string(a) == "null"
	bEmpty := len(b) == 0 || string(b) == "null"

	if aEmpty && bEmpty {
		return true
	}
	if aEmpty != bEmpty {
		return false
	}

	var va, vb interface{}
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}
