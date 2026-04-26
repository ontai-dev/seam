package unit_test

import (
	"encoding/json"
	"testing"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
)

// TestPackOperationResultSpec_RevisionRequired verifies that Revision is
// marshalled without omitempty -- a zero value must appear in the JSON output.
func TestPackOperationResultSpec_RevisionRequired(t *testing.T) {
	spec := seamv1alpha1.PackOperationResultSpec{
		Revision:   0,
		Capability: "pack-deploy",
		Status:     seamv1alpha1.PackResultSucceeded,
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, ok := m["revision"]; !ok {
		t.Error("revision field absent from JSON output; must not use omitempty")
	}
}

// TestPackOperationResultSpec_RevisionSerialization verifies round-trip for
// revision, previousRevisionRef, and talosClusterOperationResultRef.
func TestPackOperationResultSpec_RevisionSerialization(t *testing.T) {
	spec := seamv1alpha1.PackOperationResultSpec{
		Revision:                        7,
		PreviousRevisionRef:             "pack-deploy-result-exec-abc-r6",
		TalosClusterOperationResultRef:  "talos-op-result-xyz",
		PackExecutionRef:                "exec-abc",
		ClusterPackRef:                  "cert-manager-v1.13.3",
		Capability:                      "pack-deploy",
		Status:                          seamv1alpha1.PackResultSucceeded,
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var out seamv1alpha1.PackOperationResultSpec
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if out.Revision != spec.Revision {
		t.Errorf("Revision: got %d, want %d", out.Revision, spec.Revision)
	}
	if out.PreviousRevisionRef != spec.PreviousRevisionRef {
		t.Errorf("PreviousRevisionRef: got %q, want %q", out.PreviousRevisionRef, spec.PreviousRevisionRef)
	}
	if out.TalosClusterOperationResultRef != spec.TalosClusterOperationResultRef {
		t.Errorf("TalosClusterOperationResultRef: got %q, want %q", out.TalosClusterOperationResultRef, spec.TalosClusterOperationResultRef)
	}
}

// TestPackOperationResultSpec_TalosClusterOpRefDefaultsEmpty verifies that
// TalosClusterOperationResultRef is the empty string when not set, and is
// absent from the JSON output (omitempty semantics).
func TestPackOperationResultSpec_TalosClusterOpRefDefaultsEmpty(t *testing.T) {
	spec := seamv1alpha1.PackOperationResultSpec{
		Revision:   1,
		Capability: "pack-deploy",
		Status:     seamv1alpha1.PackResultSucceeded,
	}

	if spec.TalosClusterOperationResultRef != "" {
		t.Errorf("TalosClusterOperationResultRef: expected empty string default, got %q", spec.TalosClusterOperationResultRef)
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, present := m["talosClusterOperationResultRef"]; present {
		t.Error("talosClusterOperationResultRef present in JSON when empty; expected omitted (omitempty)")
	}
	if _, present := m["previousRevisionRef"]; present {
		t.Error("previousRevisionRef present in JSON when empty; expected omitted (omitempty)")
	}
}

// TestPackOperationResultSpec_FirstRevisionNopredecessor verifies that a
// first-write result (revision=1) has empty PreviousRevisionRef.
func TestPackOperationResultSpec_FirstRevisionNopredecessor(t *testing.T) {
	spec := seamv1alpha1.PackOperationResultSpec{
		Revision:   1,
		Capability: "pack-deploy",
		Status:     seamv1alpha1.PackResultSucceeded,
	}

	if spec.PreviousRevisionRef != "" {
		t.Errorf("PreviousRevisionRef: expected empty for revision 1, got %q", spec.PreviousRevisionRef)
	}
}
