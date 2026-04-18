// Package webhook_test contains unit tests for EvaluateDomainRefValidation.
//
// Tests verify: kind filtering, operation filtering (only CREATE intercepted),
// empty domainRef admitted, valid domainRef admitted, and unknown domainRef rejected.
// seam-core-schema.md §3, CLAUDE.md §14 Decision 2.
package webhook_test

import (
	"strings"
	"testing"

	"github.com/ontai-dev/seam-core/internal/webhook"
)

// TestEvaluateDomainRefValidation_NonILIKind_Allowed verifies that non-ILI kinds
// are always admitted regardless of domainRef value.
func TestEvaluateDomainRefValidation_NonILIKind_Allowed(t *testing.T) {
	decision := webhook.EvaluateDomainRefValidation(webhook.DomainRefValidationRequest{
		Kind:      "SomeOtherKind",
		Operation: webhook.OperationCreate,
		DomainRef: "unknown.value",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for non-ILI kind; got reason %q", decision.Reason)
	}
}

// TestEvaluateDomainRefValidation_UpdateOperation_Allowed verifies that UPDATE
// operations are always admitted — domainRef is validated only at CREATE.
func TestEvaluateDomainRefValidation_UpdateOperation_Allowed(t *testing.T) {
	decision := webhook.EvaluateDomainRefValidation(webhook.DomainRefValidationRequest{
		Kind:      "InfrastructureLineageIndex",
		Operation: webhook.OperationUpdate,
		DomainRef: "some.unknown.ref",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for UPDATE operation; got reason %q", decision.Reason)
	}
}

// TestEvaluateDomainRefValidation_DeleteOperation_Allowed verifies that DELETE
// operations are always admitted.
func TestEvaluateDomainRefValidation_DeleteOperation_Allowed(t *testing.T) {
	decision := webhook.EvaluateDomainRefValidation(webhook.DomainRefValidationRequest{
		Kind:      "InfrastructureLineageIndex",
		Operation: webhook.OperationDelete,
		DomainRef: "some.unknown.ref",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for DELETE operation; got reason %q", decision.Reason)
	}
}

// TestEvaluateDomainRefValidation_EmptyDomainRef_Allowed verifies that a CREATE
// with an empty domainRef is admitted — the LineageController will populate it.
func TestEvaluateDomainRefValidation_EmptyDomainRef_Allowed(t *testing.T) {
	decision := webhook.EvaluateDomainRefValidation(webhook.DomainRefValidationRequest{
		Kind:      "InfrastructureLineageIndex",
		Operation: webhook.OperationCreate,
		DomainRef: "",
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for empty domainRef; got reason %q", decision.Reason)
	}
}

// TestEvaluateDomainRefValidation_ValidDomainRef_Allowed verifies that a CREATE
// with domainRef="infrastructure.core.ontai.dev" is admitted.
func TestEvaluateDomainRefValidation_ValidDomainRef_Allowed(t *testing.T) {
	decision := webhook.EvaluateDomainRefValidation(webhook.DomainRefValidationRequest{
		Kind:      "InfrastructureLineageIndex",
		Operation: webhook.OperationCreate,
		DomainRef: webhook.ValidInfrastructureDomainRef,
	})
	if !decision.Allowed {
		t.Errorf("expected Allowed=true for valid domainRef %q; got reason %q",
			webhook.ValidInfrastructureDomainRef, decision.Reason)
	}
}

// TestEvaluateDomainRefValidation_UnknownDomainRef_Denied verifies that a CREATE
// with an unrecognised domainRef is rejected with an informative message.
func TestEvaluateDomainRefValidation_UnknownDomainRef_Denied(t *testing.T) {
	unknown := "unknown.domain.example.com"
	decision := webhook.EvaluateDomainRefValidation(webhook.DomainRefValidationRequest{
		Kind:      "InfrastructureLineageIndex",
		Operation: webhook.OperationCreate,
		DomainRef: unknown,
	})
	if decision.Allowed {
		t.Error("expected Allowed=false for unknown domainRef; got Allowed=true")
	}
	if !strings.Contains(decision.Reason, unknown) {
		t.Errorf("expected reason to contain unknown value %q; got: %s", unknown, decision.Reason)
	}
	if !strings.Contains(decision.Reason, webhook.ValidInfrastructureDomainRef) {
		t.Errorf("expected reason to contain valid value %q; got: %s",
			webhook.ValidInfrastructureDomainRef, decision.Reason)
	}
}
