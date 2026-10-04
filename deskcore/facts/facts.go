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
	"bytes"
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

// Required JSON keys. Presence is checked separately from value because a JSON decoder fills an
// absent field with its zero value, and an absent pagination_done must not read as a proven
// false any more than an absent collection_errors may read as "no errors".
var (
	bundleKeys = []string{"schema", "snapshot_id", "collected_at", "policy_digest", "graph_revision", "pattern_digest", "sources", "collection_errors", "facts"}
	sourceKeys = []string{"identity", "revision", "collected_at", "completeness", "pagination_done"}
	factKeys   = []string{"id", "source", "kind", "subject", "observed_at"}
)

// Load reads one bundle from r, refusing an unknown schema, an unknown or missing field,
// trailing data, and any bundle that fails Validate.
func Load(r io.Reader) (*Bundle, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read fact bundle: %w", err)
	}
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("parse fact bundle: %w", err)
	}
	if head.Schema == nil {
		return nil, fmt.Errorf("%w: no schema field", ErrUnknownSchema)
	}
	if *head.Schema != SchemaV1 {
		return nil, fmt.Errorf("%w: %q (this reader accepts only %q)", ErrUnknownSchema, *head.Schema, SchemaV1)
	}
	if err := checkKeys(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var b Bundle
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("parse fact bundle: %w", err)
	}
	if dec.More() {
		return nil, errors.New("parse fact bundle: trailing data after the bundle")
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &b, nil
}

func checkKeys(data []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return fmt.Errorf("parse fact bundle: %w", err)
	}
	var errs []error
	errs = append(errs, missing("bundle", top, bundleKeys)...)
	var nested struct {
		Sources []map[string]json.RawMessage `json:"sources"`
		Facts   []map[string]json.RawMessage `json:"facts"`
	}
	if err := json.Unmarshal(data, &nested); err != nil {
		return fmt.Errorf("parse fact bundle: %w", err)
	}
	for i, s := range nested.Sources {
		errs = append(errs, missing(fmt.Sprintf("sources[%d]", i), s, sourceKeys)...)
	}
	for i, f := range nested.Facts {
		errs = append(errs, missing(fmt.Sprintf("facts[%d]", i), f, factKeys)...)
	}
	return errors.Join(errs...)
}

func missing(where string, m map[string]json.RawMessage, keys []string) []error {
	var errs []error
	for _, k := range keys {
		v, ok := m[k]
		if !ok || string(v) == "null" {
			errs = append(errs, fmt.Errorf("fact bundle: %s: required field %q is missing", where, k))
		}
	}
	return errs
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
	// CouldNotCheck: the bundle cannot answer, because the source is absent, failed, or was
	// not read completely and the fact was not found in the part that was read.
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
	if age := now.Sub(src.CollectedAt); age > q.MaxAge {
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

func (b *Bundle) hasError(source string) bool {
	for _, e := range b.CollectionErrors {
		if e.Source == source {
			return true
		}
	}
	return false
}
