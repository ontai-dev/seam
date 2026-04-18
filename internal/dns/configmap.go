package dns

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ConfigMap key and identity constants for the dsns-zone ConfigMap.
// seam-core-schema.md §8 Decision 2.
const (
	ZoneConfigMapName      = "dsns-zone"
	ZoneConfigMapNamespace = "ont-system"
	ZoneDataKey            = "zone.db"

	// ZoneMirrorNamespace is the namespace where dsns-zone is mirrored for CoreDNS.
	// CoreDNS runs in kube-system and can only mount ConfigMaps from its own namespace.
	// The mirror is best-effort: a failed mirror write is logged as a warning but
	// does not fail reconciliation. The primary in ont-system is always authoritative.
	ZoneMirrorNamespace = "kube-system"

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

// ApplyContent writes content to the dsns-zone ConfigMap in ont-system (primary)
// and mirrors it to kube-system so CoreDNS can mount it. The primary write is
// authoritative: if it fails the error is returned and the mirror is skipped. If
// the mirror write fails, a warning is logged but no error is returned.
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
		if err := w.client.Create(ctx, cm); err != nil {
			return err
		}
		w.applyMirror(ctx, content)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get dsns-zone ConfigMap: %w", err)
	}

	patch := client.MergeFrom(existing.DeepCopy())
	if existing.Data == nil {
		existing.Data = make(map[string]string)
	}
	existing.Data[ZoneDataKey] = content
	if err := w.client.Patch(ctx, existing, patch); err != nil {
		return err
	}
	w.applyMirror(ctx, content)
	return nil
}

// applyMirror writes content to the dsns-zone ConfigMap in kube-system.
// CoreDNS runs in kube-system and can only mount ConfigMaps from its own namespace.
// The mirror is created if absent with the same labels and governance annotation as
// the primary. Failures are logged as warnings; the caller does not receive an error.
func (w *ConfigMapZoneWriter) applyMirror(ctx context.Context, content string) {
	logger := log.FromContext(ctx)

	existing := &corev1.ConfigMap{}
	err := w.client.Get(ctx, client.ObjectKey{
		Name:      ZoneConfigMapName,
		Namespace: ZoneMirrorNamespace,
	}, existing)

	if apierrors.IsNotFound(err) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ZoneConfigMapName,
				Namespace: ZoneMirrorNamespace,
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
		if createErr := w.client.Create(ctx, cm); createErr != nil {
			logger.Error(createErr, "mirror dsns-zone create failed — primary write in ont-system remains authoritative",
				"mirrorNamespace", ZoneMirrorNamespace)
		}
		return
	}
	if err != nil {
		logger.Error(err, "mirror dsns-zone get failed — primary write in ont-system remains authoritative",
			"mirrorNamespace", ZoneMirrorNamespace)
		return
	}

	patch := client.MergeFrom(existing.DeepCopy())
	if existing.Data == nil {
		existing.Data = make(map[string]string)
	}
	existing.Data[ZoneDataKey] = content
	if patchErr := w.client.Patch(ctx, existing, patch); patchErr != nil {
		logger.Error(patchErr, "mirror dsns-zone patch failed — primary write in ont-system remains authoritative",
			"mirrorNamespace", ZoneMirrorNamespace)
	}
}
