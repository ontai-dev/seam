package webhook

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// AdmissionWebhookServer wraps the controller-runtime manager and exposes
// Register methods to wire Seam Core admission webhooks into the manager's
// webhook server.
//
// Three webhooks are registered:
//   - RootBindingWebhookPath ("/validate-lineage-index-immutability"):
//     Rejects UPDATE requests that modify spec.rootBinding on
//     InfrastructureLineageIndex. seam-core-schema.md §3.1.
//   - AuthorshipWebhookPath ("/validate-lineage-index-authorship"):
//     Rejects CREATE and UPDATE requests not from the LineageController
//     ServiceAccount. CLAUDE.md §14 Decision 3.
//   - DomainRefWebhookPath ("/validate-lineage-index-domainref"):
//     Rejects CREATE requests that set spec.domainRef to an unknown value.
//     CLAUDE.md §14 Decision 2.
type AdmissionWebhookServer struct {
	mgr ctrl.Manager
}

// NewAdmissionWebhookServer creates a new AdmissionWebhookServer bound to mgr.
func NewAdmissionWebhookServer(mgr ctrl.Manager) *AdmissionWebhookServer {
	return &AdmissionWebhookServer{mgr: mgr}
}

// RegisterImmutability wires the RootBindingImmutabilityHandler into the
// manager's webhook server at RootBindingWebhookPath.
//
// Must be called after the manager is created and before mgr.Start.
// The manager enforces leader election — the webhook server becomes active
// only after the leader lock is acquired.
func (s *AdmissionWebhookServer) RegisterImmutability() {
	handler := &RootBindingImmutabilityHandler{}
	s.mgr.GetWebhookServer().Register(RootBindingWebhookPath, &admission.Webhook{Handler: handler})
}

// RegisterAuthorship wires the AuthorshipGateHandler into the manager's webhook
// server at AuthorshipWebhookPath.
//
// Must be called after the manager is created and before mgr.Start.
func (s *AdmissionWebhookServer) RegisterAuthorship() {
	handler := &AuthorshipGateHandler{}
	s.mgr.GetWebhookServer().Register(AuthorshipWebhookPath, &admission.Webhook{Handler: handler})
}

// RegisterDomainRef wires the DomainRefValidationHandler into the manager's webhook
// server at DomainRefWebhookPath. It validates spec.domainRef on ILI CREATE requests.
//
// Must be called after the manager is created and before mgr.Start.
func (s *AdmissionWebhookServer) RegisterDomainRef() {
	handler := &DomainRefValidationHandler{}
	s.mgr.GetWebhookServer().Register(DomainRefWebhookPath, &admission.Webhook{Handler: handler})
}
