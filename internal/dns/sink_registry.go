package dns

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SinkRegistry fans out DSNSEvent notifications to zero or more NotificationSink
// implementations. Each sink is invoked in its own goroutine with a five-second
// individual timeout so that a slow or failing sink never blocks or errors the
// caller. SinkRegistry.Notify itself returns immediately after launching the
// goroutines — it does not wait for sinks to complete.
//
// Zero registered sinks is a valid state: Notify returns nil immediately.
//
// seam-core-schema.md §8 Decision 1.
type SinkRegistry struct {
	sinks []NotificationSink
}

// NewSinkRegistry returns a SinkRegistry with the provided sinks.
// Passing no arguments produces a zero-sink registry — Notify is a no-op.
func NewSinkRegistry(sinks ...NotificationSink) *SinkRegistry {
	return &SinkRegistry{sinks: sinks}
}

// Notify fans out the event to all registered sinks. Each sink is called in its
// own goroutine with a five-second timeout derived from a fresh context (not the
// caller's context, which may be cancelled before the goroutine runs). Errors are
// logged; they are never returned to the caller. Returns nil immediately.
func (r *SinkRegistry) Notify(ctx context.Context, event DSNSEvent) error {
	if len(r.sinks) == 0 {
		return nil
	}
	logger := log.FromContext(ctx).WithName("dsns-sink-registry")
	for i, sink := range r.sinks {
		go func(idx int, s NotificationSink) {
			// Use a fresh context so the goroutine is not subject to cancellation
			// of the reconcile context from which Notify was called.
			sinkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.Notify(sinkCtx, event); err != nil {
				logger.Error(err, "dsns sink notification failed",
					"sinkIndex", idx,
					"category", event.RecordCategory,
					"operation", event.Operation,
					"sourceKind", event.SourceRef.Kind,
					"sourceName", event.SourceRef.Name,
				)
			}
		}(i, sink)
	}
	return nil
}
