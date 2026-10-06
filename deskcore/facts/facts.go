// Package facts defines desk-facts-v1: the versioned, immutable bundle of collected facts that
// the desk tools and the board generator read, so that the same bundle evaluated by the same
// evaluator gives the same answer.
//
// A bundle carries the identity of its facts (snapshot_id, policy_digest and the per-source
// revisions), never the identity of whatever evaluates it: the evaluator reports its own
// version beside the bundle's snapshot_id.
//
// The package is pure. Load reads from an io.Reader the caller supplies, and Query takes the
// caller's "now"; nothing here opens a file, a socket or a process.
//
// The JSON contract is schemas/desk-facts-v1.json at the module root. A consumer that does not
// link this package parses that contract itself and must refuse an unknown schema value
// exactly as Load does.
package facts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/medici-finance/assay/deskcore/domain"
)

// SchemaV1 is the only schema value this package accepts.
const SchemaV1 = "desk-facts-v1"

// ErrUnknownSchema is returned (wrapped) when a bundle's schema value is missing or is not
// SchemaV1. An unknown schema is a refusal, never a best-effort parse.
var ErrUnknownSchema = errors.New("unknown fact bundle schema")

// Completeness says how much of a source was read.
type Completeness string

const (
	// Complete: every page of the source was read.
	Complete Completeness = "complete"
	// Partial: some of the source was read and some was not.
	Partial Completeness = "partial"
	// Failed: nothing usable was read from the source.
	Failed Completeness = "failed"
)

// Source is one place facts were collected from (for example one repository's issue listing),
// with the freshness and completeness of that collection.
type Source struct {
	Identity       string          `json:"identity"`
	Revision       domain.Revision `json:"revision"`
	CollectedAt    time.Time       `json:"collected_at"`
	Completeness   Completeness    `json:"completeness"`
	PaginationDone bool            `json:"pagination_done"`
}

// CollectionError records a read that did not complete. Source may name an identity absent
// from the bundle's sources: a source that could not even be enumerated is still reported.
type CollectionError struct {
	Source string `json:"source"`
	Cause  string `json:"cause"`
}

// Bundle is one desk-facts-v1 snapshot.
type Bundle struct {
	Schema      string    `json:"schema"`
	SnapshotID  string    `json:"snapshot_id"`
	CollectedAt time.Time `json:"collected_at"`
	// PolicyDigest is the digest of the approved policy the facts were collected under, or ""
	// when no policy was bound. It is part of the identity two answers must share.
	PolicyDigest domain.Digest `json:"policy_digest"`
	// GraphRevision is the revision of the reviewed work graph, or "".
	GraphRevision domain.Revision `json:"graph_revision"`
	// PatternDigest is the digest of the work pattern, or "".
	PatternDigest    domain.Digest     `json:"pattern_digest"`
	Sources          []Source          `json:"sources"`
	CollectionErrors []CollectionError `json:"collection_errors"`
	Facts            []domain.Fact     `json:"facts"`
}

var snapshotRe = regexp.MustCompile(`^[0-9a-f]{12,128}$`)

// RequiredKeys lists the keys a bundle's JSON object must carry. Presence is checked separately
// from value because a JSON decoder fills an absent field with its zero value, and an absent
// collection_errors must not read as "no errors".
func (Bundle) RequiredKeys() []string {
	return []string{"schema", "snapshot_id", "collected_at", "policy_digest", "graph_revision", "pattern_digest", "sources", "collection_errors", "facts"}
}

// RequiredKeys lists the keys a source's JSON object must carry: an absent pagination_done must
// not read as a proven false.
func (Source) RequiredKeys() []string {
	return []string{"identity", "revision", "collected_at", "completeness", "pagination_done"}
}

// RequiredKeys lists the keys a collection error's JSON object must carry.
func (CollectionError) RequiredKeys() []string { return []string{"source", "cause"} }

// MaxBundleBytes is the largest bundle Load reads. A larger input is refused rather than read
// into memory without bound.
const MaxBundleBytes = 16 << 20

// Load reads one bundle from r, refusing an input larger than MaxBundleBytes, an unknown
// schema, and anything domain.DecodeStrict refuses: a key that is not a field (compared
// exactly, case included), a key given twice, a missing or null required key, and trailing
// data. A bundle that decodes must then pass Validate.
func Load(r io.Reader) (*Bundle, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBundleBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read fact bundle: %w", err)
	}
	if len(data) > MaxBundleBytes {
		return nil, fmt.Errorf("read fact bundle: input is larger than %d bytes", MaxBundleBytes)
	}
	if err := peekSchema(data); err != nil {
		return nil, err
	}
	var b Bundle
	if err := domain.DecodeStrict(data, &b); err != nil {
		return nil, fmt.Errorf("parse fact bundle: %w", err)
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &b, nil
}

// peekSchema reports an unknown or missing schema value before the strict decode, so that a
// bundle of another version is refused as such rather than for the fields it does not share.
// It only routes the error: every bundle it lets through is then decoded strictly.
func peekSchema(data []byte) error {
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("parse fact bundle: %w", err)
	}
	if head.Schema == nil {
		return fmt.Errorf("%w: no schema field", ErrUnknownSchema)
	}
	if *head.Schema != SchemaV1 {
		return fmt.Errorf("%w: %q (this reader accepts only %q)", ErrUnknownSchema, *head.Schema, SchemaV1)
	}
	return nil
}

// Validate reports every structural defect in b.
func (b *Bundle) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf("fact bundle: "+format, a...)) }

	if b.Schema != SchemaV1 {
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownSchema, b.Schema))
	}
	if !snapshotRe.MatchString(b.SnapshotID) {
		add("snapshot_id %q: want 12 to 128 lowercase hex characters", b.SnapshotID)
	}
	if b.CollectedAt.IsZero() {
		add("collected_at is unset")
	}
	for _, d := range []struct {
		name string
		v    domain.Digest
	}{{"policy_digest", b.PolicyDigest}, {"pattern_digest", b.PatternDigest}} {
		if d.v == "" {
			continue
		}
		if err := d.v.Validate(); err != nil {
			add("%s: %v", d.name, err)
		}
	}
	if b.Sources == nil {
		add("sources is null; an empty collection is []")
	}
	if b.CollectionErrors == nil {
		add("collection_errors is null; no errors is []")
	}
	if b.Facts == nil {
		add("facts is null; no facts is []")
	}

	sources := map[string]Source{}
	for i, s := range b.Sources {
		if s.Identity == "" {
			add("sources[%d]: identity is empty", i)
			continue
		}
		if _, dup := sources[s.Identity]; dup {
			add("sources[%d]: identity %q is declared twice", i, s.Identity)
		}
		sources[s.Identity] = s
		if s.CollectedAt.IsZero() {
			add("source %q: collected_at is unset", s.Identity)
		} else if !b.CollectedAt.IsZero() && s.CollectedAt.After(b.CollectedAt) {
			add("source %q: collected_at %s is after the bundle's %s", s.Identity, s.CollectedAt.Format(time.RFC3339), b.CollectedAt.Format(time.RFC3339))
		}
		switch s.Completeness {
		case Complete:
			if !s.PaginationDone {
				add("source %q: completeness is complete but pagination_done is false", s.Identity)
			}
		case Partial, Failed:
			// The failure must be explained, not just flagged: checked below.
		default:
			add("source %q: completeness %q is not complete, partial or failed", s.Identity, s.Completeness)
		}
	}

	explained := map[string]bool{}
	for i, e := range b.CollectionErrors {
		if e.Source == "" {
			add("collection_errors[%d]: source is empty", i)
		}
		if e.Cause == "" {
			add("collection_errors[%d]: cause is empty", i)
		}
		explained[e.Source] = true
	}
	for _, s := range b.Sources {
		if (s.Completeness == Partial || s.Completeness == Failed) && !explained[s.Identity] {
			add("source %q: completeness is %s but no collection error names it", s.Identity, s.Completeness)
		}
		if s.Completeness == Complete && explained[s.Identity] {
			add("source %q: completeness is complete but a collection error names it", s.Identity)
		}
	}

	ids := map[domain.FactID]bool{}
	for i, f := range b.Facts {
		if f.ID == "" {
			add("facts[%d]: id is empty", i)
		} else if ids[f.ID] {
			add("facts[%d]: id %q is used twice", i, f.ID)
		}
		ids[f.ID] = true
		src, ok := sources[f.Source]
		if !ok {
			add("fact %q: source %q is not declared in sources", f.ID, f.Source)
		} else if src.Completeness == Failed {
			add("fact %q: source %q failed and cannot carry facts", f.ID, f.Source)
		}
		if f.Kind == "" {
			add("fact %q: kind is empty", f.ID)
		}
		if f.Subject == "" {
			add("fact %q: subject is empty", f.ID)
		}
		if f.ObservedAt.IsZero() {
			add("fact %q: observed_at is unset", f.ID)
		}
	}
	return errors.Join(errs...)
}

// State is the answer to "is this fact so?". A stale fact and a known negative are distinct,
// and neither is the same as could-not-check.
type State string

const (
	// Present: the fact was observed in a source collected within the freshness bound.
	Present State = "present"
	// KnownNegative: the source was read completely, within the freshness bound, and the fact
	// is not in it.
	KnownNegative State = "known-negative"
	// Stale: the source was collected longer ago than the freshness bound allows. A stale
	// answer carries any matching facts, but it is neither present nor a known negative.
	Stale State = "stale"
	// CouldNotCheck: the bundle cannot answer, because the source is absent, failed, was
	// collected after the caller's "now", or was not read completely and the fact was not
	// found in the part that was read.
	CouldNotCheck State = "could-not-check"
)

// Query asks for the facts of one kind about one subject from one source. MaxAge is the
// freshness bound; it must be positive.
type Query struct {
	Source  string
	Kind    string
	Subject string
	MaxAge  time.Duration
}

// Answer is the result of a Query.
type Answer struct {
	State  State
	Facts  []domain.Fact
	Reason string
}

// Query answers q against the bundle as of now.
func (b *Bundle) Query(q Query, now time.Time) Answer {
	if q.MaxAge <= 0 {
		return Answer{State: CouldNotCheck, Reason: "no freshness bound was given"}
	}
	var src *Source
	for i := range b.Sources {
		if b.Sources[i].Identity == q.Source {
			src = &b.Sources[i]
			break
		}
	}
	if src == nil {
		return Answer{State: CouldNotCheck, Reason: fmt.Sprintf("source %q is not in the bundle", q.Source)}
	}
	if src.Completeness == Failed {
		return Answer{State: CouldNotCheck, Reason: fmt.Sprintf("source %q failed to collect", q.Source)}
	}
	var found []domain.Fact
	for _, f := range b.Facts {
		if f.Source == q.Source && f.Kind == q.Kind && f.Subject == q.Subject {
			found = append(found, f)
		}
	}
	age, ok := ageAt(now, src.CollectedAt)
	if !ok || b.CollectedAt.After(now) {
		return Answer{State: CouldNotCheck, Reason: fmt.Sprintf("source %q or its bundle was collected after now (%s); a future time cannot prove freshness", q.Source, now.Format(time.RFC3339))}
	}
	if age > q.MaxAge {
		return Answer{State: Stale, Facts: found, Reason: fmt.Sprintf("source %q was collected %s ago, beyond the %s bound", q.Source, age.Round(time.Second), q.MaxAge)}
	}
	if len(found) > 0 {
		return Answer{State: Present, Facts: found}
	}
	if src.Completeness == Complete && src.PaginationDone && !b.hasError(q.Source) {
		return Answer{State: KnownNegative, Reason: fmt.Sprintf("source %q was read completely and holds no such fact", q.Source)}
	}
	return Answer{State: CouldNotCheck, Reason: fmt.Sprintf("source %q was read only in part and the fact was not in the part read", q.Source)}
}

// ageAt is the one place an age is computed from a timestamp. It reports false when t is after
// now: a negative age would pass every freshness bound, so a future time proves nothing. No skew
// tolerance is allowed.
func ageAt(now, t time.Time) (time.Duration, bool) {
	if t.After(now) {
		return 0, false
	}
	return now.Sub(t), true
}

func (b *Bundle) hasError(source string) bool {
	for _, e := range b.CollectionErrors {
		if e.Source == source {
			return true
		}
	}
	return false
}
