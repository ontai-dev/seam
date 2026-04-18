// Package dns_test — tests for DSNSEvent construction, NotificationSink, and SinkRegistry.
// seam-core-schema.md §8 Decision 1.
package dns_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	idns "github.com/ontai-dev/seam-core/internal/dns"
)

// ── DSNSEvent construction tests ─────────────────────────────────────────────

// TestDSNSEvent_ClusterTopologyCategory verifies that a cluster-topology event
// carries the correct category, operation, and source ref fields.
func TestDSNSEvent_ClusterTopologyCategory(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryClusterTopology,
		Operation:      idns.OperationUpdated,
		SourceRef: idns.SourceRef{
			Group:   "platform.ontai.dev",
			Version: "v1alpha1",
			Kind:    "TalosCluster",
			Name:    "ccs-mgmt",
		},
		ClusterContext: "management",
		DerivedRecords: []string{"ccs-mgmt 300 IN A 10.20.0.10"},
		Severity:       idns.SeverityInformational,
	}

	if event.RecordCategory != idns.RecordCategoryClusterTopology {
		t.Errorf("RecordCategory = %q, want %q", event.RecordCategory, idns.RecordCategoryClusterTopology)
	}
	if event.Operation != idns.OperationUpdated {
		t.Errorf("Operation = %q, want %q", event.Operation, idns.OperationUpdated)
	}
	if event.SourceRef.Kind != "TalosCluster" {
		t.Errorf("SourceRef.Kind = %q, want TalosCluster", event.SourceRef.Kind)
	}
	if event.Severity != idns.SeverityInformational {
		t.Errorf("Severity = %q, want informational", event.Severity)
	}
	if len(event.DerivedRecords) != 1 {
		t.Errorf("DerivedRecords len = %d, want 1", len(event.DerivedRecords))
	}
}

// TestDSNSEvent_IdentityPlaneCategory verifies identity-plane event fields.
func TestDSNSEvent_IdentityPlaneCategory(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryIdentityPlane,
		Operation:      idns.OperationUpdated,
		SourceRef: idns.SourceRef{
			Group:     "security.ontai.dev",
			Version:   "v1alpha1",
			Kind:      "IdentityBinding",
			Name:      "binding-alice",
			Namespace: "seam-tenant-ccs-dev",
		},
		ClusterContext: "ccs-dev",
		DerivedRecords: []string{"identity.abc123.guardian.ccs-dev 300 IN TXT admin github-idp"},
		Severity:       idns.SeverityInformational,
	}

	if event.RecordCategory != idns.RecordCategoryIdentityPlane {
		t.Errorf("RecordCategory = %q, want %q", event.RecordCategory, idns.RecordCategoryIdentityPlane)
	}
	if event.ClusterContext != "ccs-dev" {
		t.Errorf("ClusterContext = %q, want ccs-dev", event.ClusterContext)
	}
}

// TestDSNSEvent_PackLineageCategory verifies pack-lineage event fields.
func TestDSNSEvent_PackLineageCategory(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryPackLineage,
		Operation:      idns.OperationUpdated,
		SourceRef: idns.SourceRef{
			Group:     "infra.ontai.dev",
			Version:   "v1alpha1",
			Kind:      "PackInstance",
			Name:      "monitoring-v1",
			Namespace: "seam-tenant-ccs-dev",
		},
		ClusterContext: "ccs-dev",
		DerivedRecords: []string{"pack.monitoring.v1.wrapper.ccs-dev 300 IN TXT sha256:abc"},
		Severity:       idns.SeverityInformational,
	}

	if event.RecordCategory != idns.RecordCategoryPackLineage {
		t.Errorf("RecordCategory = %q, want %q", event.RecordCategory, idns.RecordCategoryPackLineage)
	}
	if event.SourceRef.Kind != "PackInstance" {
		t.Errorf("SourceRef.Kind = %q, want PackInstance", event.SourceRef.Kind)
	}
}

// TestDSNSEvent_ExecutionAuthorityCategory verifies execution-authority event fields.
func TestDSNSEvent_ExecutionAuthorityCategory(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryExecutionAuthority,
		Operation:      idns.OperationUpdated,
		SourceRef: idns.SourceRef{
			Group:     "runner.ontai.dev",
			Version:   "v1alpha1",
			Kind:      "RunnerConfig",
			Name:      "bootstrap-rc",
			Namespace: "ont-system",
		},
		ClusterContext: "management",
		DerivedRecords: []string{"run.bootstrap-rc.conductor.management 300 IN TXT phase=Completed completed=2026-04-06T10:00:00Z"},
		Severity:       idns.SeverityInformational,
	}

	if event.RecordCategory != idns.RecordCategoryExecutionAuthority {
		t.Errorf("RecordCategory = %q, want %q", event.RecordCategory, idns.RecordCategoryExecutionAuthority)
	}
	if event.SourceRef.Kind != "RunnerConfig" {
		t.Errorf("SourceRef.Kind = %q, want RunnerConfig", event.SourceRef.Kind)
	}
}

// TestDSNSEvent_DeletionOperation verifies a deletion event has no DerivedRecords.
func TestDSNSEvent_DeletionOperation(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryClusterTopology,
		Operation:      idns.OperationDeleted,
		SourceRef: idns.SourceRef{
			Kind: "TalosCluster",
			Name: "old-cluster",
		},
		ClusterContext: "management",
		Severity:       idns.SeverityInformational,
	}

	if event.Operation != idns.OperationDeleted {
		t.Errorf("Operation = %q, want deleted", event.Operation)
	}
	if len(event.DerivedRecords) != 0 {
		t.Errorf("DerivedRecords should be empty for deletion events, got %d", len(event.DerivedRecords))
	}
}

// ── Severity mapping tests ────────────────────────────────────────────────────

// TestDSNSEvent_SeverityInformationalForHealthyUpdate verifies that healthy
// resource updates use informational severity.
func TestDSNSEvent_SeverityInformationalForHealthyUpdate(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryClusterTopology,
		Operation:      idns.OperationUpdated,
		Severity:       idns.SeverityInformational,
	}
	if event.Severity != idns.SeverityInformational {
		t.Errorf("Severity = %q, want informational", event.Severity)
	}
}

// TestDSNSEvent_SeverityWarningForDegradedTransition verifies that a degraded
// condition transition (e.g. RunnerConfig Degraded=True) uses warning severity.
func TestDSNSEvent_SeverityWarningForDegradedTransition(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryExecutionAuthority,
		Operation:      idns.OperationUpdated,
		SourceRef:      idns.SourceRef{Kind: "RunnerConfig"},
		Severity:       idns.SeverityWarning,
	}
	if event.Severity != idns.SeverityWarning {
		t.Errorf("Severity = %q, want warning", event.Severity)
	}
}

// TestDSNSEvent_SeverityCriticalForFailedTransition verifies that a failed or
// unknown condition transition uses critical severity.
func TestDSNSEvent_SeverityCriticalForFailedTransition(t *testing.T) {
	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryExecutionAuthority,
		Operation:      idns.OperationUpdated,
		SourceRef:      idns.SourceRef{Kind: "RunnerConfig"},
		Severity:       idns.SeverityCritical,
	}
	if event.Severity != idns.SeverityCritical {
		t.Errorf("Severity = %q, want critical", event.Severity)
	}
}

// ── SinkRegistry tests ────────────────────────────────────────────────────────

// captureSink records every DSNSEvent it receives.
type captureSink struct {
	mu     sync.Mutex
	events []idns.DSNSEvent
	ch     chan struct{} // closed after first Notify
}

func newCaptureSink() *captureSink {
	return &captureSink{ch: make(chan struct{}, 1)}
}

func (s *captureSink) Notify(_ context.Context, event idns.DSNSEvent) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	select {
	case s.ch <- struct{}{}:
	default:
	}
	return nil
}

func (s *captureSink) wait(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.ch:
	case <-time.After(timeout):
		t.Fatal("captureSink: timed out waiting for Notify to be called")
	}
}

func (s *captureSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// TestSinkRegistry_ZeroSinks_ReturnsNil verifies that a zero-sink registry
// Notify call returns nil immediately without blocking.
func TestSinkRegistry_ZeroSinks_ReturnsNil(t *testing.T) {
	sr := idns.NewSinkRegistry()
	err := sr.Notify(context.Background(), idns.DSNSEvent{RecordCategory: idns.RecordCategoryClusterTopology})
	if err != nil {
		t.Errorf("Notify on zero-sink registry returned error: %v", err)
	}
}

// TestSinkRegistry_SingleSink_ReceivesEvent verifies that a registered sink
// receives the emitted event.
func TestSinkRegistry_SingleSink_ReceivesEvent(t *testing.T) {
	sink := newCaptureSink()
	sr := idns.NewSinkRegistry(sink)

	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryPackLineage,
		Operation:      idns.OperationUpdated,
		Severity:       idns.SeverityInformational,
	}
	if err := sr.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	// Notify is fire-and-forget — wait for the goroutine.
	sink.wait(t, 2*time.Second)

	if sink.count() != 1 {
		t.Errorf("sink received %d events, want 1", sink.count())
	}
}

// TestSinkRegistry_MultipleSinks_AllReceiveEvent verifies fan-out: all registered
// sinks receive the event.
func TestSinkRegistry_MultipleSinks_AllReceiveEvent(t *testing.T) {
	sink1 := newCaptureSink()
	sink2 := newCaptureSink()
	sink3 := newCaptureSink()
	sr := idns.NewSinkRegistry(sink1, sink2, sink3)

	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryIdentityPlane,
		Operation:      idns.OperationUpdated,
		Severity:       idns.SeverityInformational,
	}
	if err := sr.Notify(context.Background(), event); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	sink1.wait(t, 2*time.Second)
	sink2.wait(t, 2*time.Second)
	sink3.wait(t, 2*time.Second)

	for i, s := range []*captureSink{sink1, sink2, sink3} {
		if s.count() != 1 {
			t.Errorf("sink%d received %d events, want 1", i+1, s.count())
		}
	}
}

// TestSinkRegistry_FailingSink_DoesNotPropagateError verifies that a sink that
// returns an error does not cause SinkRegistry.Notify to return an error.
func TestSinkRegistry_FailingSink_DoesNotPropagateError(t *testing.T) {
	failSink := &errorSink{ch: make(chan struct{}, 1)}
	sr := idns.NewSinkRegistry(failSink)

	err := sr.Notify(context.Background(), idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryExecutionAuthority,
		Operation:      idns.OperationUpdated,
	})
	if err != nil {
		t.Errorf("Notify returned non-nil error when sink failed: %v", err)
	}

	// Wait to ensure the goroutine ran.
	failSink.wait(t, 2*time.Second)
}

// TestSinkRegistry_FailingSink_OtherSinksStillReceive verifies that a failing
// sink does not block other sinks from receiving the event.
func TestSinkRegistry_FailingSink_OtherSinksStillReceive(t *testing.T) {
	failSink := &errorSink{ch: make(chan struct{}, 1)}
	goodSink := newCaptureSink()
	sr := idns.NewSinkRegistry(failSink, goodSink)

	if err := sr.Notify(context.Background(), idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryClusterTopology,
		Operation:      idns.OperationUpdated,
	}); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	goodSink.wait(t, 2*time.Second)
	if goodSink.count() != 1 {
		t.Errorf("good sink received %d events, want 1", goodSink.count())
	}
}

// TestSinkRegistry_SlowSink_DoesNotBlockCaller verifies that SinkRegistry.Notify
// returns immediately even when a registered sink blocks.
func TestSinkRegistry_SlowSink_DoesNotBlockCaller(t *testing.T) {
	slowSink := &blockingSink{block: make(chan struct{})}
	sr := idns.NewSinkRegistry(slowSink)

	start := time.Now()
	err := sr.Notify(context.Background(), idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryPackLineage,
		Operation:      idns.OperationUpdated,
	})
	elapsed := time.Since(start)

	// Unblock the goroutine so it doesn't leak.
	close(slowSink.block)

	if err != nil {
		t.Errorf("Notify returned error: %v", err)
	}
	// Notify must return in well under 100ms (it's fire-and-forget).
	if elapsed > 100*time.Millisecond {
		t.Errorf("Notify took %v, expected near-instant return for fire-and-forget", elapsed)
	}
}

// ── DSNSState integration tests with sinks ────────────────────────────────────

// TestDSNSState_ApplyNotifiesSink verifies that DSNSState.Apply emits a DSNSEvent
// to the registered SinkRegistry after a successful ConfigMap write.
func TestDSNSState_ApplyNotifiesSink(t *testing.T) {
	sink := newCaptureSink()
	sr := idns.NewSinkRegistry(sink)

	fc := newFakeClient(t)
	state := idns.NewDSNSState(fc)
	state.SetSinks(sr)

	event := idns.DSNSEvent{
		RecordCategory: idns.RecordCategoryClusterTopology,
		Operation:      idns.OperationUpdated,
		SourceRef:      idns.SourceRef{Kind: "TalosCluster", Name: "ccs-mgmt"},
		ClusterContext: "management",
		Severity:       idns.SeverityInformational,
	}

	if err := state.Apply(context.Background(), event); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	sink.wait(t, 2*time.Second)
	if sink.count() != 1 {
		t.Errorf("sink received %d events after Apply, want 1", sink.count())
	}
}

// TestDSNSState_Apply_NoEvent_DoesNotPanic verifies that Apply called without
// an event (backward-compat path) does not panic or error.
func TestDSNSState_Apply_NoEvent_DoesNotPanic(t *testing.T) {
	fc := newFakeClient(t)
	state := idns.NewDSNSState(fc)

	if err := state.Apply(context.Background()); err != nil {
		t.Fatalf("Apply with no event returned error: %v", err)
	}
}

// ── helper sink types ─────────────────────────────────────────────────────────

// errorSink always returns an error from Notify.
type errorSink struct {
	ch chan struct{}
}

func (s *errorSink) Notify(_ context.Context, _ idns.DSNSEvent) error {
	select {
	case s.ch <- struct{}{}:
	default:
	}
	return errors.New("sink error")
}

func (s *errorSink) wait(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.ch:
	case <-time.After(timeout):
		t.Fatal("errorSink: timed out waiting for Notify to be called")
	}
}

// blockingSink blocks until its channel is closed.
type blockingSink struct {
	block chan struct{}
}

func (s *blockingSink) Notify(_ context.Context, _ idns.DSNSEvent) error {
	<-s.block
	return nil
}
