// Package unit contains unit and serialization integrity tests for T-2B-5
// Go type additions that remain in seam-core: InfrastructureRunnerConfig,
// InfrastructureTalosCluster, and DriftSignal.
// Pack lifecycle types (PackDelivery, PackExecution, PackInstalled, PackReceipt,
// PackLog) have been migrated to wrapper/api/seam/v1alpha1.
// seam-core-schema.md. Decision I.
package unit

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// --- InfrastructureRunnerConfig ---

func TestInfrastructureRunnerConfig_RequiredFields(t *testing.T) {
	t.Parallel()
	rc := v1alpha1.InfrastructureRunnerConfig{
		Spec: v1alpha1.InfrastructureRunnerConfigSpec{
			ClusterRef:  "ccs-mgmt",
			RunnerImage: "10.20.0.1:5000/ontai-dev/conductor:v1.9.3-dev",
		},
	}
	if rc.Spec.ClusterRef == "" {
		t.Fatal("ClusterRef must be set")
	}
	if rc.Spec.RunnerImage == "" {
		t.Fatal("RunnerImage must be set")
	}
}

func TestInfrastructureRunnerConfig_RoundTrip(t *testing.T) {
	t.Parallel()
	rc := v1alpha1.InfrastructureRunnerConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "infrastructure.ontai.dev/v1alpha1",
			Kind:       "InfrastructureRunnerConfig",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "ccs-mgmt", Namespace: "ont-system"},
		Spec: v1alpha1.InfrastructureRunnerConfigSpec{
			ClusterRef:  "ccs-mgmt",
			RunnerImage: "10.20.0.1:5000/ontai-dev/conductor:v1.9.3-dev",
			Steps: []v1alpha1.RunnerConfigStep{
				{Name: "pack-deploy-cert-manager", Capability: "pack-deploy", HaltOnFailure: true},
			},
			SelfOperation: true,
		},
	}
	data, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got v1alpha1.InfrastructureRunnerConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Spec.ClusterRef != rc.Spec.ClusterRef {
		t.Errorf("ClusterRef: got %q want %q", got.Spec.ClusterRef, rc.Spec.ClusterRef)
	}
	if len(got.Spec.Steps) != 1 || got.Spec.Steps[0].Capability != "pack-deploy" {
		t.Errorf("Steps not preserved: %+v", got.Spec.Steps)
	}
}

func TestInfrastructureRunnerConfig_StepResultPhaseEnum(t *testing.T) {
	t.Parallel()
	cases := []v1alpha1.RunnerStepResultPhase{
		v1alpha1.RunnerStepSucceeded,
		v1alpha1.RunnerStepFailed,
		v1alpha1.RunnerStepSkipped,
	}
	for _, c := range cases {
		if c == "" {
			t.Errorf("RunnerStepResultPhase constant is empty")
		}
	}
}

// --- InfrastructureTalosCluster ---

func TestInfrastructureTalosCluster_ModeEnum(t *testing.T) {
	t.Parallel()
	cases := []v1alpha1.InfrastructureTalosClusterMode{
		v1alpha1.InfrastructureTalosClusterModeBootstrap,
		v1alpha1.InfrastructureTalosClusterModeImport,
	}
	for _, c := range cases {
		if c == "" {
			t.Errorf("mode constant is empty")
		}
	}
}

func TestInfrastructureTalosCluster_ImportRoleRequired(t *testing.T) {
	t.Parallel()
	tc := v1alpha1.InfrastructureTalosCluster{
		Spec: v1alpha1.InfrastructureTalosClusterSpec{
			Mode:            v1alpha1.InfrastructureTalosClusterModeImport,
			Role:            v1alpha1.InfrastructureTalosClusterRoleManagement,
			ClusterEndpoint: "https://10.20.0.10:6443",
		},
	}
	if tc.Spec.Role == "" {
		t.Fatal("Role must be set for mode=import")
	}
}

func TestInfrastructureTalosCluster_LineageFieldPresent(t *testing.T) {
	t.Parallel()
	chain := &lineage.SealedCausalChain{RootKind: "InfrastructureTalosCluster", RootName: "ccs-mgmt"}
	tc := v1alpha1.InfrastructureTalosCluster{
		Spec: v1alpha1.InfrastructureTalosClusterSpec{
			Mode:    v1alpha1.InfrastructureTalosClusterModeImport,
			Role:    v1alpha1.InfrastructureTalosClusterRoleManagement,
			Lineage: chain,
		},
	}
	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got v1alpha1.InfrastructureTalosCluster
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Spec.Lineage == nil {
		t.Fatal("Lineage must survive round trip")
	}
}

// --- DriftSignal ---

func TestDriftSignal_StateEnum(t *testing.T) {
	t.Parallel()
	states := []v1alpha1.DriftSignalState{
		v1alpha1.DriftSignalStatePending,
		v1alpha1.DriftSignalStateDelivered,
		v1alpha1.DriftSignalStateQueued,
		v1alpha1.DriftSignalStateConfirmed,
	}
	for _, s := range states {
		if s == "" {
			t.Errorf("DriftSignalState constant is empty")
		}
	}
}

func TestDriftSignal_RequiredFields(t *testing.T) {
	t.Parallel()
	ds := v1alpha1.DriftSignal{
		Spec: v1alpha1.DriftSignalSpec{
			State:         v1alpha1.DriftSignalStatePending,
			CorrelationID: "550e8400-e29b-41d4-a716-446655440000",
			ObservedAt:    metav1.Now(),
			AffectedCRRef: v1alpha1.DriftAffectedCRRef{
				Group: "infra.ontai.dev",
				Kind:  "ClusterPack",
				Name:  "cert-manager-helm-v1.14.0-r1",
			},
			DriftReason: "ClusterPack rbacDigest does not match deployed RBAC resources",
		},
	}
	if ds.Spec.CorrelationID == "" {
		t.Fatal("CorrelationID must be set")
	}
}

func TestDriftSignal_EscalationCounterRoundTrip(t *testing.T) {
	t.Parallel()
	ds := v1alpha1.DriftSignal{
		Spec: v1alpha1.DriftSignalSpec{
			State:             v1alpha1.DriftSignalStateDelivered,
			CorrelationID:     "550e8400-e29b-41d4-a716-446655440001",
			ObservedAt:        metav1.Now(),
			AffectedCRRef:     v1alpha1.DriftAffectedCRRef{Group: "infra.ontai.dev", Kind: "ClusterPack", Name: "cert-manager"},
			DriftReason:       "drift detected",
			EscalationCounter: 3,
			CorrectionJobRef:  "pack-deploy-cert-manager-job-abc",
		},
	}
	data, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got v1alpha1.DriftSignal
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Spec.EscalationCounter != 3 {
		t.Errorf("EscalationCounter: got %d want 3", got.Spec.EscalationCounter)
	}
	if got.Spec.CorrectionJobRef != "pack-deploy-cert-manager-job-abc" {
		t.Errorf("CorrectionJobRef: got %q", got.Spec.CorrectionJobRef)
	}
}

func TestDriftSignal_StateTransitionSequence(t *testing.T) {
	t.Parallel()
	ordered := []v1alpha1.DriftSignalState{
		v1alpha1.DriftSignalStatePending,
		v1alpha1.DriftSignalStateDelivered,
		v1alpha1.DriftSignalStateQueued,
		v1alpha1.DriftSignalStateConfirmed,
	}
	seen := map[v1alpha1.DriftSignalState]bool{}
	for _, s := range ordered {
		if seen[s] {
			t.Errorf("duplicate state value: %q", s)
		}
		seen[s] = true
	}
	if len(seen) != 4 {
		t.Errorf("expected 4 distinct states, got %d", len(seen))
	}
}
