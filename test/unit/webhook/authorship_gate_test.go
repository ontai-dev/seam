// Package webhook_test contains unit tests for the EvaluateAuthorshipGate pure
// function defined in authorship_gate.go.
//
// Tests verify: kind filtering, DELETE always allowed, LineageController SA
// identity allowed, all other identities denied, reason content.
// CLAUDE.md §14 Decision 3, seam-core-schema.md §7 Declaration 4.
package webhook_test

import (
	"strings"
	"testing"

	"github.com/ontai-dev/seam/internal/webhook"
)

// --- Non-intercepted kinds ---

// Test A1 — Non-intercepted kind: always allowed regardless of requesting user.
func TestEvaluateAuthorshipGate_NonInterceptedKind_Allowed(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           "SomeOtherKind",
		Operation:      webhook.OperationCreate,
		RequestingUser: "system:serviceaccount:some-ns:some-sa",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for non-intercepted kind; got reason %q", decision.Reason)
	}
}

// Test A2 — Non-intercepted kind UPDATE from non-authorized user: allowed.
func TestEvaluateAuthorshipGate_NonInterceptedKind_Update_Allowed(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           "Deployment",
		Operation:      webhook.OperationUpdate,
		RequestingUser: "some-random-user",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for non-intercepted kind Deployment; got reason %q", decision.Reason)
	}
}

// --- DELETE is always allowed ---

// Test A3 — ILI DELETE: always allowed regardless of requesting user.
// The authorship gate covers CREATE and UPDATE only.
func TestEvaluateAuthorshipGate_ILI_Delete_Allowed(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationDelete,
		RequestingUser: "system:admin",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI DELETE; got reason %q", decision.Reason)
	}
}

// Test A4 — ILI DELETE: allowed even for completely unauthorized user.
func TestEvaluateAuthorshipGate_ILI_Delete_UnauthorizedUser_Allowed(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationDelete,
		RequestingUser: "unknown-human",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI DELETE from any user; got reason %q", decision.Reason)
	}
}

// --- LineageController SA: CREATE and UPDATE allowed ---

// Test A5 — ILI CREATE from LineageController SA: allowed. CLAUDE.md §14 Decision 3.
func TestEvaluateAuthorshipGate_ILI_Create_LineageControllerSA_Allowed(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: webhook.LineageControllerIdentity,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI CREATE from LineageController SA; got reason %q", decision.Reason)
	}
}

// Test A6 — ILI UPDATE from LineageController SA: allowed.
func TestEvaluateAuthorshipGate_ILI_Update_LineageControllerSA_Allowed(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationUpdate,
		RequestingUser: webhook.LineageControllerIdentity,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for ILI UPDATE from LineageController SA; got reason %q", decision.Reason)
	}
}

// --- Non-authorized users: CREATE and UPDATE denied ---

// Test A7 — ILI CREATE from human user: denied. CLAUDE.md §14 Decision 3.
func TestEvaluateAuthorshipGate_ILI_Create_HumanUser_Denied(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: "alice",
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI CREATE from human user")
	}
	if decision.Reason == "" {
		t.Error("expected non-empty reason for denied decision")
	}
}

// Test A8 — ILI UPDATE from human user: denied.
func TestEvaluateAuthorshipGate_ILI_Update_HumanUser_Denied(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationUpdate,
		RequestingUser: "bob",
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI UPDATE from human user")
	}
}

// Test A9 — ILI CREATE from system:admin: denied.
// Even cluster-admin is not permitted to write LineageRecord.
func TestEvaluateAuthorshipGate_ILI_Create_ClusterAdmin_Denied(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: "system:admin",
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI CREATE from system:admin")
	}
}

// Test A10 — ILI CREATE from a different ServiceAccount: denied.
// Only the specific lineage-controller SA is authorized — not any SA.
func TestEvaluateAuthorshipGate_ILI_Create_OtherSA_Denied(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: "system:serviceaccount:seam-system:some-other-controller",
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI CREATE from a different SA in seam-system")
	}
}

// Test A11 — ILI CREATE from empty user (unauthenticated): denied.
func TestEvaluateAuthorshipGate_ILI_Create_EmptyUser_Denied(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: "",
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for ILI CREATE from empty user")
	}
}

// --- Denial reason content ---

// Test A12 — Denial reason references CLAUDE.md §14 Decision 3.
func TestEvaluateAuthorshipGate_DeniedReason_ReferencesDecision3(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: "unauthorized-user",
	})
	if decision.Allowed {
		t.Fatal("expected denied decision")
	}
	if !strings.Contains(decision.Reason, "CLAUDE.md") {
		t.Errorf("expected reason to reference CLAUDE.md; got %q", decision.Reason)
	}
	if !strings.Contains(decision.Reason, "Decision 3") {
		t.Errorf("expected reason to reference Decision 3; got %q", decision.Reason)
	}
}

// Test A13 — Denial reason includes LineageControllerIdentity for observability.
func TestEvaluateAuthorshipGate_DeniedReason_IncludesAuthorizedIdentity(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: "bad-actor",
	})
	if decision.Allowed {
		t.Fatal("expected denied decision")
	}
	if !strings.Contains(decision.Reason, webhook.LineageControllerIdentity) {
		t.Errorf("expected reason to include LineageControllerIdentity; got %q", decision.Reason)
	}
}

// Test A14 — Denial reason includes the requesting user for diagnostics.
func TestEvaluateAuthorshipGate_DeniedReason_IncludesRequestingUser(t *testing.T) {
	requester := "system:serviceaccount:default:some-controller"
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: requester,
	})
	if decision.Allowed {
		t.Fatal("expected denied decision")
	}
	if !strings.Contains(decision.Reason, requester) {
		t.Errorf("expected reason to include requesting user %q; got %q", requester, decision.Reason)
	}
}

// Test A15 — Reason is empty when allowed.
func TestEvaluateAuthorshipGate_AllowedReason_IsEmpty(t *testing.T) {
	decision := webhook.EvaluateAuthorshipGate(webhook.AuthorshipGateRequest{
		Kind:           webhook.LineageRecordKind,
		Operation:      webhook.OperationCreate,
		RequestingUser: webhook.LineageControllerIdentity,
	})
	if !decision.Allowed {
		t.Fatal("expected allowed decision")
	}
	if decision.Reason != "" {
		t.Errorf("expected empty reason for allowed decision; got %q", decision.Reason)
	}
}
