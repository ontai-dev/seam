// Package dns — DSNSEvent and NotificationSink define the extension point through
// which DSNS zone changes are broadcast to external consumers.
//
// After every successful ConfigMap write, DSNSState emits a DSNSEvent to all
// registered NotificationSink implementations via SinkRegistry. Sinks are called
// concurrently and never block or error the reconcile path.
//
// seam-core-schema.md §8 Decision 1.
package dns

import "context"

// Record category constants classify events by semantic domain.
const (
	RecordCategoryClusterTopology    = "cluster-topology"
	RecordCategoryIdentityPlane      = "identity-plane"
	RecordCategoryPackLineage        = "pack-lineage"
	RecordCategoryExecutionAuthority = "execution-authority"
)

// Severity constants indicate the operational significance of a DSNSEvent.
// Severity is determined by the condition state of the source resource at the
// time records are derived.
const (
	// SeverityInformational is used for create and update operations on healthy resources.
	SeverityInformational = "informational"

	// SeverityWarning is used when a condition transitions to a degraded state
	// (e.g. RunnerConfig Degraded=True — execution completed with failures).
	SeverityWarning = "warning"

	// SeverityCritical is used when a condition transitions to a failed or unknown state.
	SeverityCritical = "critical"
)

// Operation constants describe the type of change that produced a DSNSEvent.
const (
	OperationCreated = "created"
	OperationUpdated = "updated"
	OperationDeleted = "deleted"
)

// SourceRef identifies the Kubernetes resource that produced the DNS records.
type SourceRef struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
}

// DSNSEvent is emitted by DSNSState after a successful zone ConfigMap write.
// It describes which resource triggered the change, what DNS records were
// written to the zone file, and the operational significance of the event.
//
// seam-core-schema.md §8 Decision 1.
type DSNSEvent struct {
	// RecordCategory classifies the event by semantic domain.
	// One of: cluster-topology, identity-plane, pack-lineage, execution-authority.
	RecordCategory string

	// Operation describes the type of DNS zone change: created, updated, or deleted.
	Operation string

	// SourceRef identifies the Kubernetes resource that triggered the zone write.
	SourceRef SourceRef

	// ClusterContext is the cluster name associated with the event. Derived from
	// the seam-tenant-{cluster} namespace convention when present; "management"
	// for objects in non-tenant namespaces.
	ClusterContext string

	// DerivedRecords contains the DNS record strings written to the zone file for
	// this event. Empty for deletion events where records were removed.
	DerivedRecords []string

	// Severity indicates operational significance.
	//   informational — healthy create or update
	//   warning       — degraded condition transition (e.g. RunnerConfig Degraded=True)
	//   critical      — failed or unknown condition transition
	Severity string
}

// NotificationSink receives DSNSEvent notifications after every successful zone
// ConfigMap write. Implementations must be non-blocking — they should enqueue
// internally or delegate to a background goroutine. The SinkRegistry wraps each
// Notify call with a five-second per-sink timeout, so any blocking beyond that
// is cancelled automatically.
//
// seam-core-schema.md §8 Decision 1.
type NotificationSink interface {
	Notify(ctx context.Context, event DSNSEvent) error
}
