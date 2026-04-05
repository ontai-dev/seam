package e2e

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WebhookMode values for the seam.ontai.dev/webhook-mode namespace label.
// These must match the values enforced by the guardian NamespaceModeResolver.
const (
	// WebhookModeExempt skips admission enforcement for the namespace.
	// Used for: seam-system, kube-system (bootstrap phase).
	WebhookModeExempt = "exempt"

	// WebhookModeObserve logs would-be denials but does not reject.
	// Used during the observe-only window before full enforcement.
	WebhookModeObserve = "observe"

	// WebhookModeEnforce rejects resources that fail admission.
	// Used for all tenant namespaces once guardian is operational.
	WebhookModeEnforce = "enforce"
)

// webhookModeLabel is the label key read by guardian's NamespaceModeResolver.
const webhookModeLabel = "seam.ontai.dev/webhook-mode"

// NamespaceEnsurer creates a namespace if it does not already exist and stamps
// it with the seam.ontai.dev/webhook-mode label required by the guardian
// admission webhook (guardian-schema.md §5, session/34 WS1).
//
// If the namespace already exists the label is not changed — callers that need
// to change the label on an existing namespace must do so explicitly.
//
// This is idempotent: calling Ensure twice is safe.
type NamespaceEnsurer struct {
	client *ClusterClient
}

// NewNamespaceEnsurer constructs a NamespaceEnsurer for the given cluster client.
func NewNamespaceEnsurer(client *ClusterClient) *NamespaceEnsurer {
	return &NamespaceEnsurer{client: client}
}

// Ensure creates the named namespace if absent, stamping it with the given
// webhook-mode label value. Returns nil if the namespace already exists (no update).
// Returns an error if creation fails.
func (n *NamespaceEnsurer) Ensure(ctx context.Context, name, webhookMode string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				webhookModeLabel: webhookMode,
			},
		},
	}

	_, err := n.client.Typed.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Namespace exists — acceptable, no label change.
			return nil
		}
		return fmt.Errorf("e2e: cluster %q: NamespaceEnsurer: create namespace %q: %w",
			n.client.Name, name, err)
	}

	return nil
}

// EnsureSeamSystem creates seam-system with webhook-mode=exempt.
// This is the operator namespace on the management cluster. guardian-schema.md §3
// bootstrap sequence requires this label before webhook registration.
func (n *NamespaceEnsurer) EnsureSeamSystem(ctx context.Context) error {
	return n.Ensure(ctx, "seam-system", WebhookModeExempt)
}

// EnsureOntSystem creates ont-system with webhook-mode=exempt.
// This is the Conductor namespace. Exists on every cluster.
func (n *NamespaceEnsurer) EnsureOntSystem(ctx context.Context) error {
	return n.Ensure(ctx, "ont-system", WebhookModeExempt)
}

// EnsureTenantNamespace creates seam-tenant-{clusterName} with webhook-mode=enforce.
// Only Platform creates tenant namespaces (CP-INV-004). This helper is for e2e
// fixtures that pre-create the namespace before testing the reconciler.
func (n *NamespaceEnsurer) EnsureTenantNamespace(ctx context.Context, clusterName string) error {
	return n.Ensure(ctx, "seam-tenant-"+clusterName, WebhookModeEnforce)
}
