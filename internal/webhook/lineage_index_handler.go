package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// RootBindingImmutabilityHandler is a controller-runtime admission.Handler that
// enforces spec.rootBinding immutability on InfrastructureLineageIndex.
//
// It intercepts UPDATE requests and rejects any that modify spec.rootBinding.
// CREATE and DELETE are always permitted.
//
// Decision logic is delegated to EvaluateRootBindingImmutability in
// rootbinding_immutability.go. seam-core-schema.md §3.1.
type RootBindingImmutabilityHandler struct {
	decoder *admission.Decoder
}

// specRootBindingExtract is used for partial JSON unmarshalling of admitted
// InfrastructureLineageIndex resources. Only spec.rootBinding is needed.
type specRootBindingExtract struct {
	Spec struct {
		RootBinding *json.RawMessage `json:"rootBinding"`
	} `json:"spec"`
}

// Handle implements admission.Handler for the rootBinding immutability gate.
func (h *RootBindingImmutabilityHandler) Handle(_ context.Context, req admission.Request) admission.Response {
	newRootBinding, err := extractRootBinding(req.Object.Raw)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	oldRootBinding, err := extractRootBinding(req.OldObject.Raw)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var oldBytes, newBytes []byte
	if oldRootBinding != nil {
		oldBytes = []byte(*oldRootBinding)
	}
	if newRootBinding != nil {
		newBytes = []byte(*newRootBinding)
	}

	decision := EvaluateRootBindingImmutability(RootBindingImmutabilityRequest{
		Kind:              req.Kind.Kind,
		Operation:         AdmissionOperation(req.Operation),
		OldRootBindingRaw: oldBytes,
		NewRootBindingRaw: newBytes,
	})

	if decision.Allowed {
		return admission.Allowed("")
	}
	return admission.Denied(decision.Reason)
}

// InjectDecoder injects the decoder from controller-runtime's webhook builder.
// Stored but not used — spec.rootBinding is extracted from raw JSON to avoid
// a dependency on decoded object types.
func (h *RootBindingImmutabilityHandler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// extractRootBinding extracts spec.rootBinding as a raw JSON value from the
// provided raw object bytes. Returns nil if the field is absent or if raw is
// empty. Returns an error only on malformed JSON.
func extractRootBinding(raw []byte) (*json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var obj specRootBindingExtract
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj.Spec.RootBinding, nil
}

// DomainRefValidationHandler is a controller-runtime admission.Handler that validates
// spec.domainRef on InfrastructureLineageIndex CREATE requests. It rejects any
// creation that sets domainRef to a value other than "infrastructure.core.ontai.dev".
// Empty domainRef is permitted — the LineageController sets it during reconciliation.
//
// Decision logic is delegated to EvaluateDomainRefValidation in domainref_validation.go.
// seam-core-schema.md §3, CLAUDE.md §14 Decision 2.
type DomainRefValidationHandler struct {
	decoder *admission.Decoder
}

// specDomainRefExtract is used for partial JSON unmarshalling of admitted
// InfrastructureLineageIndex resources. Only spec.domainRef is needed.
type specDomainRefExtract struct {
	Spec struct {
		DomainRef string `json:"domainRef"`
	} `json:"spec"`
}

// Handle implements admission.Handler for the domainRef validation gate.
func (h *DomainRefValidationHandler) Handle(_ context.Context, req admission.Request) admission.Response {
	domainRef, err := extractDomainRef(req.Object.Raw)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	decision := EvaluateDomainRefValidation(DomainRefValidationRequest{
		Kind:      req.Kind.Kind,
		Operation: AdmissionOperation(req.Operation),
		DomainRef: domainRef,
	})

	if decision.Allowed {
		return admission.Allowed("")
	}
	return admission.Denied(decision.Reason)
}

// InjectDecoder injects the decoder from controller-runtime's webhook builder.
func (h *DomainRefValidationHandler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// extractDomainRef extracts spec.domainRef as a string from the provided raw
// object bytes. Returns "" if the field is absent or if raw is empty.
// Returns an error only on malformed JSON.
func extractDomainRef(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var obj specDomainRefExtract
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", err
	}
	return obj.Spec.DomainRef, nil
}

// AuthorshipGateHandler is a controller-runtime admission.Handler that enforces
// controller-authorship on InfrastructureLineageIndex.
//
// It intercepts CREATE and UPDATE requests and rejects any whose requesting
// principal is not the LineageController ServiceAccount. DELETE is always permitted.
//
// Decision logic is delegated to EvaluateAuthorshipGate in authorship_gate.go.
// CLAUDE.md §14 Decision 3.
type AuthorshipGateHandler struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler for the controller-authorship gate.
func (h *AuthorshipGateHandler) Handle(_ context.Context, req admission.Request) admission.Response {
	decision := EvaluateAuthorshipGate(AuthorshipGateRequest{
		Kind:           req.Kind.Kind,
		Operation:      AdmissionOperation(req.Operation),
		RequestingUser: req.UserInfo.Username,
	})

	if decision.Allowed {
		return admission.Allowed("")
	}
	return admission.Denied(decision.Reason)
}

// InjectDecoder injects the decoder from controller-runtime's webhook builder.
func (h *AuthorshipGateHandler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}
