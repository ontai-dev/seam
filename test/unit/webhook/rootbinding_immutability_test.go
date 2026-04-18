// Package webhook_test contains unit tests for the EvaluateRootBindingImmutability
// pure function defined in rootbinding_immutability.go.
//
// Tests verify: kind filtering, operation filtering (CREATE/DELETE always allowed),
// rootBinding equality comparison (both nil, identical, whitespace differences,
// field changes, nil↔present transitions), and denial reason content.
// seam-core-schema.md §3.1, domain-core-schema.md §2.1.
package webhook_test

import (
	"strings"
	"testing"

	"github.com/ontai-dev/seam-core/internal/webhook"
)

// --- Non-intercepted kinds ---

// Test R1 — Non-intercepted kind: always allowed regardless of rootBinding change.
func TestEvaluateRootBindingImmutability_NonInterceptedKind_Allowed(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              "SomeOtherKind",
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: []byte(`{"rootKind":"TalosCluster"}`),
		NewRootBindingRaw: []byte(`{"rootKind":"Different"}`),
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for non-intercepted kind; got reason %q", decision.Reason)
	}
}

// Test R2 — Non-intercepted kind: Deployment UPDATE with different rootBinding → allowed.
func TestEvaluateRootBindingImmutability_Deployment_Update_DifferentBinding_Allowed(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              "Deployment",
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: nil,
		NewRootBindingRaw: []byte(`{"rootKind":"X"}`),
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for Deployment (non-intercepted); got reason %q", decision.Reason)
	}
}

// --- CREATE is always allowed ---

// Test R3 — InfrastructureLineageIndex CREATE: always allowed.
// rootBinding is authored at creation time — CREATE is the authoritative event.
func TestEvaluateRootBindingImmutability_ILI_Create_Allowed(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationCreate,
		OldRootBindingRaw: nil,
		NewRootBindingRaw: []byte(`{"rootKind":"TalosCluster","rootName":"cluster-1","rootNamespace":"ont-system","rootUID":"abc","rootObservedGeneration":1}`),
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI CREATE; got reason %q", decision.Reason)
	}
}

// --- DELETE is always allowed ---

// Test R4 — InfrastructureLineageIndex DELETE: always allowed.
func TestEvaluateRootBindingImmutability_ILI_Delete_Allowed(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationDelete,
		OldRootBindingRaw: []byte(`{"rootKind":"TalosCluster"}`),
		NewRootBindingRaw: nil,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI DELETE; got reason %q", decision.Reason)
	}
}

// --- UPDATE with unchanged rootBinding ---

// Test R5 — ILI UPDATE: both rootBinding nil → allowed.
func TestEvaluateRootBindingImmutability_ILI_Update_BothNil_Allowed(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: nil,
		NewRootBindingRaw: nil,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI UPDATE with both rootBinding nil; got reason %q", decision.Reason)
	}
}

// Test R6 — ILI UPDATE: identical rootBinding JSON → allowed.
func TestEvaluateRootBindingImmutability_ILI_Update_IdenticalBinding_Allowed(t *testing.T) {
	binding := []byte(`{"rootKind":"TalosCluster","rootName":"cluster-1","rootNamespace":"ont-system","rootUID":"abc-123","rootObservedGeneration":1}`)
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: binding,
		NewRootBindingRaw: binding,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for identical rootBinding; got reason %q", decision.Reason)
	}
}

// Test R7 — ILI UPDATE: semantically equal rootBinding with whitespace differences → allowed.
func TestEvaluateRootBindingImmutability_ILI_Update_WhitespaceDifference_Allowed(t *testing.T) {
	old := []byte(`{"rootKind":"TalosCluster","rootName":"cluster-1"}`)
	newVal := []byte(`{ "rootKind": "TalosCluster", "rootName": "cluster-1" }`)
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: old,
		NewRootBindingRaw: newVal,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for semantically equal rootBinding (whitespace differs); got reason %q", decision.Reason)
	}
}

// --- UPDATE with changed rootBinding → denied ---

// Test R8 — ILI UPDATE: rootBinding nil → present → denied.
func TestEvaluateRootBindingImmutability_ILI_Update_NilToPresent_Denied(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: nil,
		NewRootBindingRaw: []byte(`{"rootKind":"TalosCluster"}`),
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI UPDATE adding rootBinding (nil→present)")
	}
	if decision.Reason == "" {
		t.Error("expected non-empty reason for denied decision")
	}
}

// Test R9 — ILI UPDATE: rootBinding present → nil → denied.
func TestEvaluateRootBindingImmutability_ILI_Update_PresentToNil_Denied(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: []byte(`{"rootKind":"TalosCluster"}`),
		NewRootBindingRaw: nil,
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI UPDATE removing rootBinding (present→nil)")
	}
}

// Test R10 — ILI UPDATE: rootKind field changed → denied.
func TestEvaluateRootBindingImmutability_ILI_Update_RootKindChanged_Denied(t *testing.T) {
	old := []byte(`{"rootKind":"TalosCluster","rootName":"cluster-1"}`)
	newVal := []byte(`{"rootKind":"PackExecution","rootName":"cluster-1"}`)
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: old,
		NewRootBindingRaw: newVal,
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI UPDATE with rootKind changed")
	}
}

// Test R11 — ILI UPDATE: rootUID changed → denied.
// Changing rootUID would break the causal chain anchor. seam-core-schema.md §3.3.
func TestEvaluateRootBindingImmutability_ILI_Update_RootUIDChanged_Denied(t *testing.T) {
	old := []byte(`{"rootKind":"TalosCluster","rootName":"cluster-1","rootUID":"original-uid"}`)
	newVal := []byte(`{"rootKind":"TalosCluster","rootName":"cluster-1","rootUID":"replacement-uid"}`)
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: old,
		NewRootBindingRaw: newVal,
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI UPDATE with rootUID changed")
	}
}

// Test R12 — JSON "null" value treated as absent (equal to nil) → allowed.
func TestEvaluateRootBindingImmutability_Update_NullAndNil_TreatedEqual_Allowed(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: []byte("null"),
		NewRootBindingRaw: nil,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true when old=null and new=nil (both absent); got reason %q", decision.Reason)
	}
}

// --- Denial reason content ---

// Test R13 — Denial reason references seam-core-schema.md §3.1.
func TestEvaluateRootBindingImmutability_DeniedReason_ReferencesSchema(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: nil,
		NewRootBindingRaw: []byte(`{"rootKind":"X"}`),
	})
	if decision.Allowed {
		t.Fatal("expected denied decision")
	}
	if !strings.Contains(decision.Reason, "seam-core-schema.md") {
		t.Errorf("expected reason to reference seam-core-schema.md; got %q", decision.Reason)
	}
}

// Test R14 — Reason is empty when allowed.
func TestEvaluateRootBindingImmutability_AllowedReason_IsEmpty(t *testing.T) {
	decision := webhook.EvaluateRootBindingImmutability(webhook.RootBindingImmutabilityRequest{
		Kind:              webhook.InfrastructureLineageIndexKind,
		Operation:         webhook.OperationUpdate,
		OldRootBindingRaw: nil,
		NewRootBindingRaw: nil,
	})
	if !decision.Allowed {
		t.Fatal("expected allowed decision")
	}
	if decision.Reason != "" {
		t.Errorf("expected empty reason for allowed decision; got %q", decision.Reason)
	}
}
