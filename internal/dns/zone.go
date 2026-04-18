// Package dns implements the zone file management and ConfigMap writer for the
// Domain Semantic Name Service (DSNS).
//
// DSNS projects CRD state from the nine root-declaration GVKs to DNS records in
// the seam.ontave.dev zone. seam-core-schema.md §8 Decision 2.
package dns

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Zone is the authoritative zone served by DSNS.
// seam-core-schema.md §8 Decision 3.
const Zone = "seam.ontave.dev"

// DefaultTTL is the TTL applied to all records.
const DefaultTTL = 300

// DSNSNameserver is the NS nameserver record written in the SOA and NS RRs.
const DSNSNameserver = "ns." + Zone

// DSNSSOAContact is the SOA RNAME (responsible mailbox).
const DSNSSOAContact = "hostmaster." + Zone

// RecordType is the DNS resource record type.
type RecordType string

const (
	RecordTypeA   RecordType = "A"
	RecordTypeTXT RecordType = "TXT"
	RecordTypeNS  RecordType = "NS"
)

// Record is a single DNS resource record relative to the zone origin.
// Name is relative: "cluster1" means "cluster1.seam.ontave.dev.".
// Value is the RDATA string (IP for A, quoted text for TXT, FQDN without
// trailing dot for NS — Render appends the dot).
type Record struct {
	Name  string
	Type  RecordType
	TTL   int
	Value string
}

// recordKey returns the deduplication key for a record: "TYPE:lowercase-name".
// Each (type, name) pair occupies at most one slot in the zone.
func recordKey(rtype RecordType, name string) string {
	return string(rtype) + ":" + strings.ToLower(name)
}

// ZoneFile manages an RFC 1035 compliant zone file as an in-memory structure.
//
// ZoneFile is not safe for concurrent use; callers must synchronise externally
// (DSNSState wraps ZoneFile under a mutex).
type ZoneFile struct {
	records map[string]Record // key: recordKey(type, name)
}

// NewZoneFile returns an empty ZoneFile for the seam.ontave.dev zone.
func NewZoneFile() *ZoneFile {
	return &ZoneFile{records: make(map[string]Record)}
}

// AddRecord adds or replaces a record. The (Type, Name) pair is unique;
// a second AddRecord with the same type and name overwrites the first.
// TTL defaults to DefaultTTL when set to zero.
func (z *ZoneFile) AddRecord(r Record) {
	if r.TTL == 0 {
		r.TTL = DefaultTTL
	}
	z.records[recordKey(r.Type, r.Name)] = r
}

// RemoveRecord removes the record identified by (rtype, name). No-op if absent.
func (z *ZoneFile) RemoveRecord(rtype RecordType, name string) {
	delete(z.records, recordKey(rtype, name))
}

// Render produces a complete RFC 1035 zone file string. The output includes:
//   - $ORIGIN and $TTL directives
//   - SOA record with a serial derived from the current Unix timestamp (seconds)
//   - Zone-level NS record
//   - All managed A, TXT, and NS records sorted by key for deterministic output
func (z *ZoneFile) Render() string {
	serial := time.Now().Unix()
	var sb strings.Builder

	fmt.Fprintf(&sb, "$ORIGIN %s.\n", Zone)
	fmt.Fprintf(&sb, "$TTL %d\n\n", DefaultTTL)

	// SOA record
	fmt.Fprintf(&sb, "@ %d IN SOA %s. %s. (\n", DefaultTTL, DSNSNameserver, DSNSSOAContact)
	fmt.Fprintf(&sb, "    %d ; serial\n", serial)
	fmt.Fprintf(&sb, "    3600        ; refresh\n")
	fmt.Fprintf(&sb, "    900         ; retry\n")
	fmt.Fprintf(&sb, "    604800      ; expire\n")
	fmt.Fprintf(&sb, "    %d         ; minimum TTL\n", DefaultTTL)
	fmt.Fprintf(&sb, ")\n\n")

	// Zone-level NS record
	fmt.Fprintf(&sb, "@ %d IN NS %s.\n\n", DefaultTTL, DSNSNameserver)

	// Collect and sort keys for deterministic output
	keys := make([]string, 0, len(z.records))
	for k := range z.records {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		r := z.records[k]
		ttl := r.TTL
		if ttl == 0 {
			ttl = DefaultTTL
		}
		switch r.Type {
		case RecordTypeTXT:
			fmt.Fprintf(&sb, "%s %d IN TXT \"%s\"\n", r.Name, ttl, r.Value)
		case RecordTypeNS:
			// NS RDATA is a domain name; append trailing dot for FQDN.
			fmt.Fprintf(&sb, "%s %d IN NS %s.\n", r.Name, ttl, r.Value)
		default: // RecordTypeA
			fmt.Fprintf(&sb, "%s %d IN A %s\n", r.Name, ttl, r.Value)
		}
	}

	return sb.String()
}
