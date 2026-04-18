package dns

import (
	"context"
	"strings"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DSNSState is the shared mutable state for all DSNSReconciler instances.
//
// Multiple DSNSReconciler instances (one per GVK) share one DSNSState so that
// the complete zone is maintained as a single in-memory structure and written
// to the dsns-zone ConfigMap atomically on each reconcile. All exported methods
// are safe for concurrent use.
//
// seam-core-schema.md §8 Decision 1 — DSNS shares the existing informer cache;
// the in-memory zone is rebuilt from watch events and does not survive pod restart.
// Controller-runtime triggers reconciles for all objects on startup, so the zone
// is repopulated from live CRD state within one reconcile cycle after restart.
type DSNSState struct {
	mu     sync.Mutex
	zone   *ZoneFile
	owned  map[string][]string // ownerID → record keys owned by that object
	writer *ConfigMapZoneWriter
	sinks  *SinkRegistry
}

// NewDSNSState returns a DSNSState backed by the given client.
// An empty SinkRegistry (zero sinks) is installed by default. Call SetSinks
// to replace it with an active registry before the first reconcile.
// ownerID format: "Kind/namespace/name"
func NewDSNSState(c client.Client) *DSNSState {
	return &DSNSState{
		zone:   NewZoneFile(),
		owned:  make(map[string][]string),
		writer: NewConfigMapZoneWriter(c),
		sinks:  NewSinkRegistry(),
	}
}

// SetSinks replaces the SinkRegistry on this DSNSState. It must be called
// before the first reconcile loop begins and is not safe for concurrent use.
func (s *DSNSState) SetSinks(sr *SinkRegistry) {
	s.sinks = sr
}

// SetStaticRecord adds a static record not associated with any CRD owner.
// Static records persist for the lifetime of the process (e.g. the
// authority.conductor record seeded from CONDUCTOR_SIGNING_KEY_FINGERPRINT).
func (s *DSNSState) SetStaticRecord(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zone.AddRecord(r)
}

// UpdateRecords replaces all records owned by ownerID with records.
// If records is empty this is equivalent to RemoveRecords.
func (s *DSNSState) UpdateRecords(ownerID string, records []Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeOwnerLocked(ownerID)
	if len(records) == 0 {
		return
	}
	keys := make([]string, 0, len(records))
	for _, r := range records {
		s.zone.AddRecord(r)
		keys = append(keys, recordKey(r.Type, r.Name))
	}
	s.owned[ownerID] = keys
}

// RemoveRecords removes all records previously registered under ownerID.
// No-op if ownerID is not known.
func (s *DSNSState) RemoveRecords(ownerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeOwnerLocked(ownerID)
}

// Apply renders the current zone and writes it to the dsns-zone ConfigMap.
// The zone is rendered under the lock; the ConfigMap write is made without the
// lock to avoid holding it during a network call.
//
// If an event is supplied, SinkRegistry.Notify is called after every successful
// ConfigMap write. Sinks are non-blocking — a slow or failing sink never delays
// or errors Apply. Passing no event skips notification.
func (s *DSNSState) Apply(ctx context.Context, events ...DSNSEvent) error {
	s.mu.Lock()
	rendered := s.zone.Render()
	s.mu.Unlock()
	if err := s.writer.ApplyContent(ctx, rendered); err != nil {
		return err
	}
	if len(events) > 0 && s.sinks != nil {
		s.sinks.Notify(ctx, events[0]) //nolint:errcheck // always nil; errors logged in registry
	}
	return nil
}

// ZoneSnapshot returns a copy of the current rendered zone string for testing.
func (s *DSNSState) ZoneSnapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.zone.Render()
}

func (s *DSNSState) removeOwnerLocked(ownerID string) {
	for _, key := range s.owned[ownerID] {
		idx := strings.Index(key, ":")
		if idx < 0 {
			continue
		}
		rtype := RecordType(key[:idx])
		name := key[idx+1:]
		s.zone.RemoveRecord(rtype, name)
	}
	delete(s.owned, ownerID)
}
