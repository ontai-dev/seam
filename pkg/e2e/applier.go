package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// CRApplier applies arbitrary CR YAML to a cluster via server-side apply.
//
// The caller provides the GVR explicitly rather than relying on runtime REST
// mapper discovery — this avoids a round-trip during suite setup and makes the
// test dependency on the GVR explicit. The test author knows what they are applying.
//
// Apply is idempotent: calling Apply twice with the same YAML is safe.
// The field manager is always "seam-e2e" to distinguish test-applied resources.
type CRApplier struct {
	client *ClusterClient
}

// NewCRApplier constructs a CRApplier for the given cluster client.
func NewCRApplier(client *ClusterClient) *CRApplier {
	return &CRApplier{client: client}
}

// Apply applies the given YAML manifest to the cluster via server-side apply.
// The manifest must declare a valid apiVersion, kind, metadata.name, and optionally
// metadata.namespace. The GVR must match the resource declared in the manifest.
//
// Returns the applied object as returned by the API server, or an error.
func (a *CRApplier) Apply(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	manifestYAML []byte,
) (*unstructured.Unstructured, error) {
	// Convert YAML → JSON (unstructured requires JSON).
	jsonBytes, err := yaml.YAMLToJSON(manifestYAML)
	if err != nil {
		return nil, fmt.Errorf("e2e: cluster %q: CRApplier: parse YAML: %w", a.client.Name, err)
	}

	// Decode into unstructured to extract name and namespace.
	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal(jsonBytes, &obj.Object); err != nil {
		return nil, fmt.Errorf("e2e: cluster %q: CRApplier: decode manifest: %w", a.client.Name, err)
	}

	name := obj.GetName()
	if name == "" {
		return nil, fmt.Errorf("e2e: cluster %q: CRApplier: manifest has no metadata.name", a.client.Name)
	}
	namespace := obj.GetNamespace()

	patchOpts := metav1.PatchOptions{
		FieldManager: "seam-e2e",
		Force:        boolPtr(true),
	}

	var result *unstructured.Unstructured
	if namespace == "" {
		result, err = a.client.Dynamic.Resource(gvr).Patch(
			ctx, name, types.ApplyPatchType, jsonBytes, patchOpts,
		)
	} else {
		result, err = a.client.Dynamic.Resource(gvr).Namespace(namespace).Patch(
			ctx, name, types.ApplyPatchType, jsonBytes, patchOpts,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("e2e: cluster %q: CRApplier: server-side apply %s/%s/%s: %w",
			a.client.Name, gvr.Resource, namespace, name, err)
	}

	return result, nil
}

func boolPtr(b bool) *bool { return &b }
