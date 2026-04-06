package dns

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMap key and identity constants for the dsns-zone ConfigMap.
// seam-core-schema.md §8 Decision 2.
const (
	ZoneConfigMapName      = "dsns-zone"
	ZoneConfigMapNamespace = "ont-system"
	ZoneDataKey            = "zone.db"

	// ZoneLabelKey is applied to the dsns-zone ConfigMap so admission webhooks
	// can identify it for the controller-authorship gate.
	ZoneLabelKey   = "seam.ontai.dev/dsns-zone"
	ZoneLabelValue = "true"

	// ZoneOwnerAnnotationKey records the governance.infrastructure.ontai.dev owner.
	// seam-core-schema.md §7 Declaration 4.
	ZoneOwnerAnnotationKey = "governance.infrastructure.ontai.dev/owner"
	ZoneOwnerAnnotationVal = "seam-core"
)

// ConfigMapZoneWriter writes rendered zone file content to the dsns-zone
// ConfigMap in ont-system. DSNSReconciler is the sole caller; the admission
// webhook enforces write exclusivity for the ConfigMap. seam-core-schema.md §8 Decision 2.
type ConfigMapZoneWriter struct {
	client client.Client
}

// NewConfigMapZoneWriter returns a ConfigMapZoneWriter backed by the given client.
func NewConfigMapZoneWriter(c client.Client) *ConfigMapZoneWriter {
	return &ConfigMapZoneWriter{client: c}
}

// Apply renders the ZoneFile and writes it to the dsns-zone ConfigMap.
// It is a thin wrapper around ApplyContent.
func (w *ConfigMapZoneWriter) Apply(ctx context.Context, zf *ZoneFile) error {
	return w.ApplyContent(ctx, zf.Render())
}

// ApplyContent writes content to the dsns-zone ConfigMap.
// If the ConfigMap does not exist it is created with the correct label and
// governance annotation. If it already exists it is patched via MergeFrom.
func (w *ConfigMapZoneWriter) ApplyContent(ctx context.Context, content string) error {
	existing := &corev1.ConfigMap{}
	err := w.client.Get(ctx, client.ObjectKey{
		Name:      ZoneConfigMapName,
		Namespace: ZoneConfigMapNamespace,
	}, existing)

	if apierrors.IsNotFound(err) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ZoneConfigMapName,
				Namespace: ZoneConfigMapNamespace,
				Labels: map[string]string{
					ZoneLabelKey: ZoneLabelValue,
				},
				Annotations: map[string]string{
					ZoneOwnerAnnotationKey: ZoneOwnerAnnotationVal,
				},
			},
			Data: map[string]string{
				ZoneDataKey: content,
			},
		}
		return w.client.Create(ctx, cm)
	}
	if err != nil {
		return fmt.Errorf("get dsns-zone ConfigMap: %w", err)
	}

	patch := client.MergeFrom(existing.DeepCopy())
	if existing.Data == nil {
		existing.Data = make(map[string]string)
	}
	existing.Data[ZoneDataKey] = content
	return w.client.Patch(ctx, existing, patch)
}
