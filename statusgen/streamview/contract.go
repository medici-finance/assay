// Package streamview is the neutral, versioned wire contract for a stream view:
// one stream, identified by its owning repository and its slug, with an
// optional authored mission, per-section provenance and availability, and an
// explicit change window.
//
// It lives in its own importable package because statusgen itself is
// `package main` and cannot be imported. A consumer (a dashboard service, a
// test harness) imports this package and calls Decode; the producer (statusgen)
// builds a StreamView and calls Encode. The package holds types and invariants
// only — no readiness algorithm, no projection, no persistent store.
//
// The human-readable contract is docs/stream-view-contract.md; every rule
// below is stated there with its rationale.
package streamview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Version is the one contract version this package produces and accepts.
// A document carrying any other value fails Decode with an
// *UnsupportedVersionError — never a best-effort read.
const Version = "stream-view/v1"

// SupportedVersions is the default accept list for Decode and Negotiate, in
// consumer preference order.
var SupportedVersions = []string{Version}

// Availability is the state of one section's source. Missing is never zero,
// empty, done or healthy: a section that could not be read says so here and
// carries no content.
type Availability string

const (
	// Available: every source for the section was read at the recorded revision.
	Available Availability = "available"
	// Partial: some sources were read; Reason names what is missing.
	Partial Availability = "partial"
	// CouldNotCheck: the source was not read or was invalid. No content.
	CouldNotCheck Availability = "could-not-check"
	// NotAssessed: no producer exists for this section yet. No content.
	NotAssessed Availability = "not-assessed"
)

func (a Availability) valid() bool {
	switch a {
	case Available, Partial, CouldNotCheck, NotAssessed:
		return true
	}
	return false
}

// carriesContent reports whether a section in this state may hold content.
func (a Availability) carriesContent() bool { return a == Available || a == Partial }

var (
	repoRe     = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
	slugRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	revisionRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

// ValidRepo reports whether repo has the <owner>/<name> form. A "." or ".."
// segment is refused: no forge allows those names, and a consumer that maps a
// repo onto a filesystem path must never be steered outside its root.
func ValidRepo(repo string) bool {
	if !repoRe.MatchString(repo) {
		return false
	}
	for _, seg := range strings.SplitN(repo, "/", 2) {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// ValidRevision reports whether rev is a full (40- or 64-hex) commit id.
// Abbreviated ids and ref names are refused: a binding must be exact.
func ValidRevision(rev string) bool { return revisionRe.MatchString(rev) }

// Key is the canonical stream identity: owning repository plus stream slug.
// It deliberately has no cell, tenant or authorization field — access context
// travels beside the identity (AccessContext), never inside it, so the same
// stream seen through two cells is still one stream.
type Key struct {
	Repo string `json:"repo"`
	Slug string `json:"slug"`
}

// String renders the key as "<owner>/<name>:<slug>". A slug cannot contain
// ':' and a repo cannot contain ':', so the form is unambiguous.
func (k Key) String() string { return k.Repo + ":" + k.Slug }

// Validate checks both halves of the key.
func (k Key) Validate() error {
	if !ValidRepo(k.Repo) {
		return fmt.Errorf("stream key: repo %q is not <owner>/<name>", k.Repo)
	}
	if !slugRe.MatchString(k.Slug) {
		return fmt.Errorf("stream key: slug %q is not a stream slug", k.Slug)
	}
	return nil
}

// Identity is a stream's identity block. Only Key identifies; DisplayName is
// presentation and is never compared.
type Identity struct {
	Key         Key    `json:"key"`
	DisplayName string `json:"display_name"`
}

// SameStream reports whether a and b denote the same stream: equal keys, and
// nothing else. Two repositories with the same slug are two streams; equal
// display names or titles never make two keys one.
func SameStream(a, b Identity) bool { return a.Key == b.Key }

// ContinuesHistory reports whether history recorded under prev may be shown
// as the history of next. In v1 that holds only for an identical key: a
// renamed stream (new slug) or a moved stream (new repo) is a NEW identity
// and starts with no carried history or acknowledgment. v1 has no rename
// migration record; a later contract version may add one explicitly.
func ContinuesHistory(prev, next Key) bool { return prev == next }

// AccessContext is the authorization context the view was produced or served
// under. It is never part of Key and never consulted by SameStream.
type AccessContext struct {
	Cell string `json:"cell,omitempty"`
}

// SourceRef binds a section to one exact source record: a path in an owning
// repository at a full commit revision.
type SourceRef struct {
	Repo     string `json:"repo"`
	Path     string `json:"path"`
	Revision string `json:"revision"`
}

// Validate checks the form of the binding (not that it resolves — that is
// CheckBindings' job, against the owning repository).
func (s SourceRef) Validate() error {
	if !ValidRepo(s.Repo) {
		return fmt.Errorf("source: repo %q is not <owner>/<name>", s.Repo)
	}
	if err := validRelPath(s.Path); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if !ValidRevision(s.Revision) {
		return fmt.Errorf("source: revision %q is not a full commit id", s.Revision)
	}
	return nil
}

func validRelPath(p string) error {
	switch {
	case p == "":
		return errors.New("path is empty")
	case strings.HasPrefix(p, "/"), strings.Contains(p, "\\"):
		return fmt.Errorf("path %q is not repository-relative", p)
	case hasControl(p):
		return fmt.Errorf("path %q contains a control character", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("path %q has an empty, '.' or '..' segment", p)
		}
	}
	return nil
}

// Provenance is the per-section source block every section carries.
type Provenance struct {
	Availability Availability `json:"availability"`
	// Sources are the exact records the section was built from. Required when
	// the section carries content.
	Sources []SourceRef `json:"sources,omitempty"`
	// ObservedAt is when the producer read (or tried to read) the sources.
	ObservedAt time.Time `json:"observed_at"`
	// Reason explains a partial, could-not-check or not-assessed state.
	Reason string `json:"reason,omitempty"`
	// Diagnostics carry the specific defects behind a could-not-check state,
	// such as invalid authored metadata.
	Diagnostics []string `json:"diagnostics,omitempty"`
}

func (p Provenance) validate(section string, hasContent bool) []error {
	var errs []error
	if !p.Availability.valid() {
		return []error{fmt.Errorf("%s: availability %q is not one of available|partial|could-not-check|not-assessed", section, p.Availability)}
	}
	if p.ObservedAt.IsZero() {
		errs = append(errs, fmt.Errorf("%s: observed_at is missing", section))
	}
	if p.Availability.carriesContent() {
		if len(p.Sources) == 0 {
			errs = append(errs, fmt.Errorf("%s: %s section names no source revision", section, p.Availability))
		}
		if p.Availability == Partial && p.Reason == "" {
			errs = append(errs, fmt.Errorf("%s: partial section gives no reason", section))
		}
	} else {
		if hasContent {
			errs = append(errs, fmt.Errorf("%s: %s section carries content — missing must never read as a value", section, p.Availability))
		}
		if p.Reason == "" {
			errs = append(errs, fmt.Errorf("%s: %s section gives no reason", section, p.Availability))
		}
	}
	for _, s := range p.Sources {
		if err := s.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", section, err))
		}
	}
	return errs
}

// MissionOrigin records which source the mission came from, by precedence:
// authored metadata, then legacy README prose, then nothing.
type MissionOrigin string

const (
	// OriginAuthored: the stream README's `mission:` frontmatter block.
	OriginAuthored MissionOrigin = "authored"
	// OriginLegacyProse: no `mission:` block; the outcome is the README's
	// legacy prose tagline. Success criteria are never derived from prose.
	OriginLegacyProse MissionOrigin = "legacy-prose"
	// OriginAbsent: no `mission:` block and no legacy prose outcome.
	OriginAbsent MissionOrigin = "absent"
)

// EvidenceKind classifies an evidence reference.
type EvidenceKind string

const (
	// EvidencePath: a repository-relative file in Repo.
	EvidencePath EvidenceKind = "path"
	// EvidenceForge: an issue or pull request, <owner>/<name>#<number>.
	EvidenceForge EvidenceKind = "forge"
	// EvidenceURL: an https URL. Other schemes are refused.
	EvidenceURL EvidenceKind = "url"
)

// EvidenceRef is one authored reference a success criterion cites.
type EvidenceRef struct {
	Kind   EvidenceKind `json:"kind"`
	Repo   string       `json:"repo,omitempty"`
	Path   string       `json:"path,omitempty"`
	Number int          `json:"number,omitempty"`
	URL    string       `json:"url,omitempty"`
	// Planned marks a path that is an explicitly planned output — expected
	// NOT to exist yet. It is reported as planned, never as resolved.
	Planned bool `json:"planned,omitempty"`
}

// Validate checks the reference's shape for its kind.
func (e EvidenceRef) Validate() error {
	switch e.Kind {
	case EvidencePath:
		if !ValidRepo(e.Repo) {
			return fmt.Errorf("evidence path: repo %q is not <owner>/<name>", e.Repo)
		}
		if err := validRelPath(e.Path); err != nil {
			return fmt.Errorf("evidence path: %w", err)
		}
		if e.Number != 0 || e.URL != "" {
			return errors.New("evidence path: carries forge or url fields")
		}
	case EvidenceForge:
		if !ValidRepo(e.Repo) || e.Number <= 0 {
			return fmt.Errorf("evidence forge ref: want <owner>/<name>#<number>, got %q#%d", e.Repo, e.Number)
		}
		if e.Path != "" || e.URL != "" || e.Planned {
			return errors.New("evidence forge ref: carries path, url or planned fields")
		}
	case EvidenceURL:
		if err := validURL(e.URL); err != nil {
			return err
		}
		if e.Repo != "" || e.Path != "" || e.Number != 0 || e.Planned {
			return errors.New("evidence url: carries repo, path, number or planned fields")
		}
	default:
		return fmt.Errorf("evidence: kind %q is not path|forge|url", e.Kind)
	}
	return nil
}

// urlUnsafe are characters refused in an evidence URL unless percent-encoded:
// whitespace and controls (checked separately), and the delimiters a markdown
// or HTML renderer would treat as syntax.
const urlUnsafe = "<>()[]\"'`\\{}|^"

// validURL accepts an absolute https URL with a host and no raw whitespace,
// control or delimiter characters — so a consumer can place it in a link
// destination without it breaking out. Renderers must still escape it.
func validURL(raw string) error {
	if !strings.HasPrefix(raw, "https://") {
		return fmt.Errorf("evidence url %q: only https URLs are accepted", raw)
	}
	if hasControl(raw) || strings.ContainsAny(raw, " "+urlUnsafe) {
		return fmt.Errorf("evidence url %q: only https URLs are accepted, with whitespace, control and delimiter characters percent-encoded", raw)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Opaque != "" || u.User != nil {
		return fmt.Errorf("evidence url %q: only https URLs with a host (and no user info) are accepted", raw)
	}
	return nil
}

// hasControl reports a C0 control character or DEL anywhere in s.
func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// SuccessCriterion is one authored success criterion with its evidence
// references, in authored order.
type SuccessCriterion struct {
	Criterion string        `json:"criterion"`
	Evidence  []EvidenceRef `json:"evidence,omitempty"`
}

// Mission is the mission section.
type Mission struct {
	Provenance Provenance    `json:"provenance"`
	Origin     MissionOrigin `json:"origin,omitempty"`
	Outcome    string        `json:"outcome,omitempty"`
	// Success is nil unless authored. A legacy stream has no success criteria
	// and the contract never invents them.
	Success     []SuccessCriterion `json:"success,omitempty"`
	Commitments []string           `json:"commitments,omitempty"`
	Exclusions  []string           `json:"exclusions,omitempty"`
}

func (m Mission) hasContent() bool {
	return m.Origin != "" || m.Outcome != "" || len(m.Success) > 0 || len(m.Commitments) > 0 || len(m.Exclusions) > 0
}

func (m Mission) validate() []error {
	errs := m.Provenance.validate("mission", m.hasContent())
	if !m.Provenance.Availability.carriesContent() {
		return errs
	}
	switch m.Origin {
	case OriginAuthored:
		if strings.TrimSpace(m.Outcome) == "" {
			errs = append(errs, errors.New("mission: authored mission has no outcome"))
		}
	case OriginLegacyProse:
		if strings.TrimSpace(m.Outcome) == "" {
			errs = append(errs, errors.New("mission: legacy-prose mission has no outcome"))
		}
		if len(m.Success) > 0 || len(m.Commitments) > 0 || len(m.Exclusions) > 0 {
			errs = append(errs, errors.New("mission: legacy-prose mission carries success criteria, commitments or exclusions — those exist only when authored"))
		}
	case OriginAbsent:
		if m.Outcome != "" || len(m.Success) > 0 || len(m.Commitments) > 0 || len(m.Exclusions) > 0 {
			errs = append(errs, errors.New("mission: absent mission carries content"))
		}
	default:
		errs = append(errs, fmt.Errorf("mission: origin %q is not authored|legacy-prose|absent", m.Origin))
	}
	for i, c := range m.Success {
		if strings.TrimSpace(c.Criterion) == "" {
			errs = append(errs, fmt.Errorf("mission: success[%d] has no criterion", i))
		}
		for j, e := range c.Evidence {
			if err := e.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("mission: success[%d].evidence[%d]: %w", i, j, err))
			}
		}
	}
	return errs
}

// BriefRef is one brief as statusgen's model records it. Num is copied
// verbatim; Status is the value statusgen's brief parser produced (the table
// cell trimmed and lower-cased, the same value the board uses) — never
// re-derived.
type BriefRef struct {
	// ID is the stream-qualified brief id, "<slug>/<num>"; the repository is
	// the view's Key.Repo.
	ID     string `json:"id"`
	Num    string `json:"num"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status"`
}

// Hold is a brief that is not dispatchable, with the exact reason.
type Hold struct {
	Brief  string `json:"brief"`
	Reason string `json:"reason"`
	// Ref is the blocking reference (a brief id or <owner>/<name>#<number>).
	Ref string `json:"ref,omitempty"`
}

// CurrentState is the current-state section. Status is the stream status
// verbatim from the source.
type CurrentState struct {
	Provenance Provenance `json:"provenance"`
	Status     string     `json:"status,omitempty"`
	Briefs     []BriefRef `json:"briefs,omitempty"`
	// Counts is brief count by source lifecycle status. nil (encoded as null)
	// when the section carries no content; an empty map (encoded as {}) is a
	// read stream with no briefs. No omitempty, so both round-trip exactly.
	Counts map[string]int `json:"counts"`
	Next   []string       `json:"next,omitempty"`
	Holds  []Hold         `json:"holds,omitempty"`
}

func (c CurrentState) hasContent() bool {
	return c.Status != "" || len(c.Briefs) > 0 || c.Counts != nil || len(c.Next) > 0 || len(c.Holds) > 0
}

func (c CurrentState) validate() []error {
	errs := c.Provenance.validate("current_state", c.hasContent())
	for i, b := range c.Briefs {
		if b.ID == "" || b.Num == "" || b.Status == "" {
			errs = append(errs, fmt.Errorf("current_state: briefs[%d] lacks id, num or status", i))
		}
	}
	return errs
}

// WindowKindTrailing is the only window kind v1 accepts: a fixed trailing
// interval ending at the observation. v1 has no per-principal cursor, so a
// "since your last visit" window is not expressible.
const WindowKindTrailing = "trailing"

// TrailingWindowLabel and TrailingWindowDuration fix the v1 change window.
const (
	TrailingWindowLabel    = "last 24 hours"
	TrailingWindowDuration = 24 * time.Hour
)

// Window is the explicit change window.
type Window struct {
	Kind  string    `json:"kind"`
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TrailingWindow returns the v1 window ending at end.
func TrailingWindow(end time.Time) Window {
	end = end.UTC()
	return Window{Kind: WindowKindTrailing, Label: TrailingWindowLabel, Start: end.Add(-TrailingWindowDuration), End: end}
}

func (w Window) validate() []error {
	var errs []error
	if w.Kind != WindowKindTrailing {
		errs = append(errs, fmt.Errorf("changes: window kind %q is not supported in %s (only %q)", w.Kind, Version, WindowKindTrailing))
	}
	if w.Label != TrailingWindowLabel {
		errs = append(errs, fmt.Errorf("changes: window label %q is not %q", w.Label, TrailingWindowLabel))
	}
	if w.Start.IsZero() || w.End.IsZero() || !w.End.After(w.Start) {
		errs = append(errs, errors.New("changes: window start/end missing or not increasing"))
	} else if w.End.Sub(w.Start) != TrailingWindowDuration {
		errs = append(errs, fmt.Errorf("changes: window spans %s, not %s", w.End.Sub(w.Start), TrailingWindowDuration))
	}
	return errs
}

// ChangeEvent is one typed, material change inside the window.
type ChangeEvent struct {
	Type        string    `json:"type"`
	Object      string    `json:"object"`
	Consequence string    `json:"consequence,omitempty"`
	Source      SourceRef `json:"source"`
	At          time.Time `json:"at"`
}

// Changes is the changes section. The window is always stated, whatever the
// availability, so a reader never mistakes an unread window for a quiet one.
type Changes struct {
	Provenance Provenance    `json:"provenance"`
	Window     Window        `json:"window"`
	Events     []ChangeEvent `json:"events,omitempty"`
}

func (c Changes) validate() []error {
	errs := c.Provenance.validate("changes", len(c.Events) > 0)
	errs = append(errs, c.Window.validate()...)
	for i, e := range c.Events {
		if e.Type == "" || e.Object == "" || e.At.IsZero() {
			errs = append(errs, fmt.Errorf("changes: events[%d] lacks type, object or time", i))
		}
		if err := e.Source.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("changes: events[%d]: %w", i, err))
		}
	}
	return errs
}

// Decision is one decision owed, from an existing decision source.
type Decision struct {
	ID           string        `json:"id"`
	Alternatives []string      `json:"alternatives,omitempty"`
	Deferral     string        `json:"deferral,omitempty"`
	Owner        string        `json:"owner,omitempty"`
	Evidence     []EvidenceRef `json:"evidence,omitempty"`
	// Revision, when set, is the full commit id the decision's evidence
	// paths are pinned to.
	Revision string `json:"revision,omitempty"`
}

// NeedsYou is the decisions-owed section.
type NeedsYou struct {
	Provenance Provenance `json:"provenance"`
	Decisions  []Decision `json:"decisions,omitempty"`
}

// EvidenceItem is one independently attributable evidence record.
type EvidenceItem struct {
	Claim string      `json:"claim"`
	Ref   EvidenceRef `json:"ref"`
	// Revision, when set, is the full commit id Ref is pinned to.
	Revision     string    `json:"revision,omitempty"`
	ObservedAt   time.Time `json:"observed_at"`
	Verification string    `json:"verification"`
}

// Evidence is the evidence section.
type Evidence struct {
	Provenance Provenance     `json:"provenance"`
	Items      []EvidenceItem `json:"items,omitempty"`
}

// OutcomeMetric is one outcome measurement with its denominators and caveats.
type OutcomeMetric struct {
	Definition  string   `json:"definition"`
	Numerator   int      `json:"numerator"`
	Denominator int      `json:"denominator"`
	Window      string   `json:"window"`
	SampleCount int      `json:"sample_count"`
	Coverage    string   `json:"coverage"`
	Caveats     []string `json:"caveats,omitempty"`
}

// Outcomes is the outcomes section.
type Outcomes struct {
	Provenance Provenance      `json:"provenance"`
	Metrics    []OutcomeMetric `json:"metrics,omitempty"`
}

// StreamView is one stream's versioned view.
type StreamView struct {
	Contract     string        `json:"contract"`
	Identity     Identity      `json:"identity"`
	Context      AccessContext `json:"context"`
	GeneratedAt  time.Time     `json:"generated_at"`
	Mission      Mission       `json:"mission"`
	CurrentState CurrentState  `json:"current_state"`
	Changes      Changes       `json:"changes"`
	NeedsYou     NeedsYou      `json:"needs_you"`
	Evidence     Evidence      `json:"evidence"`
	Outcomes     Outcomes      `json:"outcomes"`
}

// Validate checks every contract invariant and returns all violations joined,
// or nil.
func (v *StreamView) Validate() error {
	var errs []error
	if v.Contract != Version {
		errs = append(errs, &UnsupportedVersionError{Got: v.Contract, Supported: SupportedVersions})
	}
	if err := v.Identity.Key.Validate(); err != nil {
		errs = append(errs, err)
	}
	if v.GeneratedAt.IsZero() {
		errs = append(errs, errors.New("generated_at is missing"))
	}
	errs = append(errs, v.Mission.validate()...)
	errs = append(errs, v.CurrentState.validate()...)
	errs = append(errs, v.Changes.validate()...)
	// The trailing window ends at the view's observation time: a 24-hour window
	// that ended days before the view must not read as "last 24 hours".
	if !v.GeneratedAt.IsZero() && !v.Changes.Window.End.IsZero() && !v.Changes.Window.End.Equal(v.GeneratedAt) {
		errs = append(errs, fmt.Errorf("changes: window ends %s, not at generated_at %s",
			v.Changes.Window.End.UTC().Format(time.RFC3339), v.GeneratedAt.UTC().Format(time.RFC3339)))
	}
	errs = append(errs, v.NeedsYou.Provenance.validate("needs_you", len(v.NeedsYou.Decisions) > 0)...)
	for i, d := range v.NeedsYou.Decisions {
		if d.ID == "" {
			errs = append(errs, fmt.Errorf("needs_you: decisions[%d] has no id", i))
		}
		if d.Revision != "" && !ValidRevision(d.Revision) {
			errs = append(errs, fmt.Errorf("needs_you: decisions[%d] revision %q is not a full commit id", i, d.Revision))
		}
		for j, e := range d.Evidence {
			if err := e.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("needs_you: decisions[%d].evidence[%d]: %w", i, j, err))
			}
		}
	}
	errs = append(errs, v.Evidence.Provenance.validate("evidence", len(v.Evidence.Items) > 0)...)
	for i, it := range v.Evidence.Items {
		if it.Claim == "" || it.ObservedAt.IsZero() {
			errs = append(errs, fmt.Errorf("evidence: items[%d] lacks claim or observed_at", i))
		}
		if it.Revision != "" && !ValidRevision(it.Revision) {
			errs = append(errs, fmt.Errorf("evidence: items[%d] revision %q is not a full commit id", i, it.Revision))
		}
		if !validVerification[it.Verification] {
			errs = append(errs, fmt.Errorf("evidence: items[%d] verification %q is not one of unverified|verified|failed|not-assessed", i, it.Verification))
		}
		if err := it.Ref.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("evidence: items[%d]: %w", i, err))
		}
	}
	errs = append(errs, v.Outcomes.Provenance.validate("outcomes", len(v.Outcomes.Metrics) > 0)...)
	return errors.Join(errs...)
}

// validVerification is the closed set of evidence verification states.
var validVerification = map[string]bool{"unverified": true, "verified": true, "failed": true, "not-assessed": true}

// UnsupportedVersionError is returned for a document whose contract version
// the consumer does not accept. It is never downgraded to a best-effort read.
type UnsupportedVersionError struct {
	Got       string
	Supported []string
}

func (e *UnsupportedVersionError) Error() string {
	if e.Got == "" {
		return fmt.Sprintf("stream view: contract version missing (supported: %s)", strings.Join(e.Supported, ", "))
	}
	return fmt.Sprintf("stream view: unsupported contract version %q (supported: %s)", e.Got, strings.Join(e.Supported, ", "))
}

// Negotiate picks the contract version a producer emits for a consumer:
// the FIRST entry of accepted (the consumer's preference order) that the
// producer offers. No common version is an error — never a silent fallback
// to the producer's newest or oldest.
func Negotiate(offered, accepted []string) (string, error) {
	for _, a := range accepted {
		for _, o := range offered {
			if a == o {
				return a, nil
			}
		}
	}
	return "", fmt.Errorf("stream view: no common contract version (offered %v, accepted %v)", offered, accepted)
}

// Encode validates v and returns its JSON form. Field order is fixed by the
// struct definitions and map keys are sorted, so equal views encode to equal
// bytes.
func Encode(v *StreamView) ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}

// Decode is the documented consumer entry point. It reads the contract
// version FIRST and refuses any version not in accept (SupportedVersions when
// accept is empty) before decoding the body; it then decodes strictly —
// an unknown field is an error, because within one contract version the
// field set is fixed — and validates every invariant.
func Decode(data []byte, accept ...string) (*StreamView, error) {
	if len(accept) == 0 {
		accept = SupportedVersions
	}
	var head struct {
		Contract *string `json:"contract"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("stream view: %w", err)
	}
	got := ""
	if head.Contract != nil {
		got = *head.Contract
	}
	ok := false
	for _, a := range accept {
		if got != "" && got == a {
			ok = true
		}
	}
	if !ok {
		return nil, &UnsupportedVersionError{Got: got, Supported: accept}
	}
	if got != Version {
		// accept named a version this package cannot decode.
		return nil, &UnsupportedVersionError{Got: got, Supported: SupportedVersions}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var v StreamView
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("stream view: %w", err)
	}
	if dec.More() {
		return nil, errors.New("stream view: trailing data after the document")
	}
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return &v, nil
}
