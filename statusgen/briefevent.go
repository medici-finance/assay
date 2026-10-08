package main

// briefevent — the brief-flow-event/v1 contract's reference validator, fact
// store, alias registry, historian adapter and portable export
// (spec/brief-flow-event-v1.md; the stage reducer is briefstage.go).
//
// The accounting unit is the brief's stable uuid (the brief-v2 `id:`). A pull
// request, a commit or a board row is EVIDENCE about a brief, never a unit of
// its own, and an alias (`<stream>/<NN>`) is an effective-dated locator that
// may later name a different brief — so nothing here ever deduplicates or keys
// by alias or title.
//
// Layering. This file is pure: it parses, validates, stores and projects facts
// handed to it. Reading a forge, a producer receipt or git is the job of the
// existing source adapters; the only adapter here is EventsFromHistory, which
// READS board-historian rows (history.go) without changing that log's format
// or its single writer. Nothing in this file writes the historian log, and
// WriteExport refuses any destination inside a git work tree, so an export can
// never become a tracked file.
//
// Order of checks in ParseBriefEvent is load-bearing: the person-identifier
// scan runs BEFORE schema validation, because the schema validator's messages
// echo offending values and a refused value must never be disclosed. Schema
// messages are additionally reduced to path + reason (bfeRedact) so a role or
// enum field carrying a stray identifier is not echoed either.

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// embeddedEventSchemaFS holds the committed brief-flow-event/v1 schema, compiled
// in for the same reason patterns.go embeds the pattern schema: a pinned
// binary validates without the schema on disk. TestBriefEventSchemaParity pins
// this copy byte-identical to the canonical repo-root file.
//
//go:embed schemas/brief-flow-event-v1.json
var embeddedEventSchemaFS embed.FS

const (
	eventSchemaPath   = "schemas/brief-flow-event-v1.json"
	eventSchemaMarker = "brief-flow-event/v1"
	exportSchemaMark  = "brief-flow-export/v1"
	legacyFactPrefix  = "legacy-v1:"
	exportEventsFile  = "events.jsonl"
	exportManifest    = "manifest.json"
)

// Role vocabulary: role classification only, following the run-record role
// list (`desk | reviewer | verifier | worker | human`) plus an explicit
// `unknown`. Never a login, name, email or other person identifier.
var eventRoles = []string{"desk", "reviewer", "verifier", "worker", "human", "unknown"}

// observationMembers are the members that describe HOW one observation of a
// fact was obtained rather than WHAT the fact is. They are excluded from the
// fact digest, so a live capture and a later import of the same fact (which
// differ in receipt time, precision, authority and the observer's prior
// state) corroborate instead of conflicting.
var observationMembers = map[string]bool{
	"received_at": true, "occurred_at": true, "occurred_after": true,
	"occurred_before": true, "precision": true, "source_authority": true,
	"source_ref": true, "coverage": true, "import_run": true,
	"status_from": true,
}

// Precedence among observations of one fact: authority first, then precision.
// A lower rank wins. An absent value is "unstated" and ranks last.
var authorityRank = map[string]int{"forge": 0, "producer": 1, "git": 2, "historian": 3, "backfill": 4}
var precisionRank = map[string]int{"event": 0, "commit": 1, "poll": 2, "day": 3}

var (
	eventSchemaOnce sync.Once
	eventSchemaNode *schemaNode
	eventSchemaErr  error
)

func embeddedEventSchemaBytes() []byte {
	b, err := embeddedEventSchemaFS.ReadFile(eventSchemaPath)
	if err != nil {
		panic(fmt.Sprintf("statusgen: embedded %s unreadable: %v", eventSchemaPath, err))
	}
	return b
}

func eventSchema() (*schemaNode, error) {
	eventSchemaOnce.Do(func() {
		eventSchemaNode, eventSchemaErr = parseSchema(embeddedEventSchemaBytes())
	})
	return eventSchemaNode, eventSchemaErr
}

// eventKnownMembers is the set of top-level members this reader understands —
// exactly the schema's declared properties. A `must_understand` entry outside
// it refuses the event.
func eventKnownMembers() (map[string]bool, error) {
	s, err := eventSchema()
	if err != nil {
		return nil, err
	}
	props, _ := s.raw["properties"].(map[string]any)
	known := make(map[string]bool, len(props))
	for k := range props {
		known[k] = true
	}
	return known, nil
}

// BriefEvent is one validated observation of a brief lifecycle fact.
type BriefEvent struct {
	FactID          string
	BriefUUID       string
	Alias           string
	Milestone       string
	OccurredAt      time.Time
	ReceivedAt      time.Time
	OccurredAfter   time.Time // zero = unbounded
	OccurredBefore  time.Time // zero = unbounded
	Precision       string
	SourceAuthority string
	Coverage        string
	InitiatorRole   string
	ExecutorRole    string
	BriefRevision   int
	OwnerCell       string
	OwnerStream     string
	Visibility      string
	ContribRef      string
	ContribKind     string
	ContribDraft    bool
	ContribReplaces string
	AcceptCoverage  string
	AcceptRevision  int
	AcceptContribs  []string
	StatusFrom      string
	StatusTo        string
	AliasReason     string
	Supersedes      string

	// Digest is the fact digest: sha256 over the canonical JSON of every
	// member except the observation members.
	Digest string
	// Canonical is this observation's full canonical JSON line (sorted keys,
	// no insignificant whitespace, no HTML escaping). Unknown optional members
	// are retained in it, which is what makes them round-trip.
	Canonical []byte
}

// EffectiveVisibility reads an absent visibility as restricted: nothing is
// published without an explicit `public`.
func (e *BriefEvent) EffectiveVisibility() string {
	if e.Visibility == "public" {
		return "public"
	}
	return "restricted"
}

// Person-identifier refusal. Keys are compared case-insensitively with `-`
// folded to `_`. The value is NEVER part of the error.
var personKeys = map[string]bool{
	"login": true, "user": true, "username": true, "email": true, "mail": true,
	"name": true, "author": true, "actor": true, "person": true,
	"principal": true, "handle": true, "display_name": true,
	"full_name": true, "user_id": true, "committer": true, "assignee": true,
}

var personKeySuffixes = []string{
	"_login", "_email", "_name", "_user", "_username", "_handle",
	"_author", "_person", "_principal", "_mail",
}

var personKeyInfixes = []string{"login", "email", "username", "user_name"}

var emailShaped = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)

func isPersonKey(k string) bool {
	n := strings.ReplaceAll(strings.ToLower(k), "-", "_")
	if personKeys[n] {
		return true
	}
	for _, s := range personKeySuffixes {
		if strings.HasSuffix(n, s) {
			return true
		}
	}
	for _, s := range personKeyInfixes {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}

// scanPerson walks a decoded value in deterministic order and returns the
// first person-identifying member or email-shaped value, by PATH only.
func scanPerson(v any, path string) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if isPersonKey(k) {
				// The key name is part of the refused content too: name the
				// position, never the key or its value.
				return fmt.Errorf("%s: member #%d is person-identifying; actors are recorded as roles only (value not shown)", bfeLabel(path), i+1)
			}
			if err := scanPerson(t[k], joinPath(path, k)); err != nil {
				return err
			}
		}
	case []any:
		for i, el := range t {
			if err := scanPerson(el, fmt.Sprintf("%s[%d]", bfeLabel(path), i)); err != nil {
				return err
			}
		}
	case string:
		if emailShaped.MatchString(t) {
			return fmt.Errorf("%s: email-shaped value refused; actors are recorded as roles only (value not shown)", bfeLabel(path))
		}
	}
	return nil
}

func bfeLabel(path string) string {
	if path == "" {
		return "(root)"
	}
	return path
}

// bfeRedact reduces a schema-validator message to its path and reason so no
// offending value is echoed (the validator's enum/const/pattern messages
// quote the value they reject).
func bfeRedact(msg string) string {
	if strings.Contains(msg, "missing required key") || strings.Contains(msg, "wrong type") {
		return msg // key names come from the schema; types are type names
	}
	i := strings.Index(msg, ": ")
	if i < 0 {
		return "schema violation (value not shown)"
	}
	reason := "value not allowed"
	switch {
	case strings.Contains(msg, "must equal"):
		reason = "not the required constant"
	case strings.Contains(msg, "is not one of the allowed values"):
		reason = "not an allowed value"
	case strings.Contains(msg, "does not match required pattern"):
		reason = "does not match the required pattern"
	case strings.Contains(msg, "below the minimum"):
		reason = "below the minimum"
	case strings.Contains(msg, "needs at least"):
		reason = "too few items"
	}
	return msg[:i] + ": " + reason + " (value not shown)"
}

// canonicalJSON is the one canonical encoding used for digests, export lines
// and round-trip comparison: object keys sorted, no insignificant whitespace,
// no HTML escaping, numbers as decoded.
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func sha256Tag(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// LegacyFactID derives the deterministic fact id for a source that has no
// immutable event id of its own: the derivation version, namespace, object,
// revision, kind and subject, hashed. Every observer of one fact that knows
// those parts derives the same id.
func LegacyFactID(namespace, object, revision, kind, subject string) string {
	h := sha256.Sum256([]byte(strings.Join([]string{"legacy-v1", namespace, object, revision, kind, subject}, "\x00")))
	return legacyFactPrefix + hex.EncodeToString(h[:])[:24]
}

func bfeStr(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func bfeInt(m map[string]any, k string) int {
	switch n := m[k].(type) {
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func bfeTime(m map[string]any, k string) (time.Time, error) {
	s := bfeStr(m, k)
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: not a valid UTC instant", k)
	}
	return t.UTC(), nil
}

// milestoneNeeds lists the members a milestone cannot be understood without.
var milestoneNeeds = map[string][]string{
	"alias_assigned":   {"alias_reason"},
	"status_observed":  {"status_to"},
	"pr_opened":        {"contribution"},
	"pr_ready":         {"contribution"},
	"pr_draft":         {"contribution"},
	"pr_merged":        {"contribution"},
	"pr_closed":        {"contribution"},
	"pr_reopened":      {"contribution"},
	"acceptance_scope": {"acceptance"},
	"verified":         {"acceptance"},
	"done":             {"acceptance"},
	"retraction":       {"supersedes"},
}

// ParseBriefEvent validates one JSONL line as a brief-flow-event/v1 event and
// returns it. A refusal never quotes a refused value.
func ParseBriefEvent(line []byte) (*BriefEvent, error) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil, errors.New("empty event line")
	}
	if !json.Valid(line) {
		return nil, errors.New("event is not valid JSON")
	}
	var decoded any
	// yaml.v3 decodes JSON integers as int (the schema validator's integer
	// type) and refuses duplicate keys, which encoding/json would silently
	// collapse to last-wins.
	if err := yaml.Unmarshal(line, &decoded); err != nil {
		return nil, errors.New("event is not a single well-formed JSON object (duplicate members are refused)")
	}
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil, errors.New("event is not a JSON object")
	}
	return briefEventFromMap(m)
}

func briefEventFromMap(m map[string]any) (*BriefEvent, error) {
	if err := scanPerson(m, ""); err != nil {
		return nil, err
	}
	schema, err := eventSchema()
	if err != nil {
		return nil, fmt.Errorf("embedded event schema unreadable: %w", err)
	}
	if v := validateValue(schema, m, ""); len(v) > 0 {
		red := make([]string, len(v))
		for i, msg := range v {
			red[i] = bfeRedact(msg)
		}
		return nil, fmt.Errorf("schema: %s", strings.Join(red, "; "))
	}

	known, err := eventKnownMembers()
	if err != nil {
		return nil, err
	}
	if mu, ok := m["must_understand"].([]any); ok {
		for i, x := range mu {
			name, _ := x.(string)
			if !known[name] {
				return nil, fmt.Errorf("must_understand[%d]: names a member this reader does not understand; event refused", i)
			}
		}
	}

	ev := &BriefEvent{
		FactID:          bfeStr(m, "fact_id"),
		BriefUUID:       bfeStr(m, "brief_uuid"),
		Alias:           bfeStr(m, "alias"),
		Milestone:       bfeStr(m, "milestone"),
		Precision:       bfeStr(m, "precision"),
		SourceAuthority: bfeStr(m, "source_authority"),
		Coverage:        bfeStr(m, "coverage"),
		InitiatorRole:   bfeStr(m, "initiator_role"),
		ExecutorRole:    bfeStr(m, "executor_role"),
		BriefRevision:   bfeInt(m, "brief_revision"),
		OwnerCell:       bfeStr(m, "owner_cell"),
		OwnerStream:     bfeStr(m, "owner_stream"),
		Visibility:      bfeStr(m, "visibility"),
		StatusFrom:      bfeStr(m, "status_from"),
		StatusTo:        bfeStr(m, "status_to"),
		AliasReason:     bfeStr(m, "alias_reason"),
		Supersedes:      bfeStr(m, "supersedes"),
	}
	for _, f := range []struct {
		key string
		dst *time.Time
	}{
		{"occurred_at", &ev.OccurredAt},
		{"received_at", &ev.ReceivedAt},
		{"occurred_after", &ev.OccurredAfter},
		{"occurred_before", &ev.OccurredBefore},
	} {
		t, terr := bfeTime(m, f.key)
		if terr != nil {
			return nil, terr
		}
		*f.dst = t
	}
	if !ev.OccurredAfter.IsZero() && ev.OccurredAt.Before(ev.OccurredAfter) {
		return nil, errors.New("occurred_at: earlier than occurred_after")
	}
	if !ev.OccurredBefore.IsZero() && ev.OccurredAt.After(ev.OccurredBefore) {
		return nil, errors.New("occurred_at: later than occurred_before")
	}
	lower := ev.OccurredAt
	if !ev.OccurredAfter.IsZero() {
		lower = ev.OccurredAfter
	}
	if ev.ReceivedAt.Before(lower) {
		return nil, errors.New("received_at: earlier than the earliest possible occurrence")
	}

	for _, need := range milestoneNeeds[ev.Milestone] {
		if _, ok := m[need]; !ok {
			return nil, fmt.Errorf("%s: milestone %s requires member %q", ev.FactID, ev.Milestone, need)
		}
	}
	if c, ok := m["contribution"].(map[string]any); ok {
		ev.ContribRef = bfeStr(c, "ref")
		ev.ContribKind = bfeStr(c, "kind")
		ev.ContribDraft, _ = c["draft"].(bool)
		ev.ContribReplaces = bfeStr(c, "replaces")
		if ev.Milestone == "pr_opened" {
			if ev.ContribKind == "" {
				return nil, fmt.Errorf("%s: pr_opened requires contribution.kind (authoring PRs never count as coded)", ev.FactID)
			}
			if _, ok := c["draft"]; !ok {
				return nil, fmt.Errorf("%s: pr_opened requires contribution.draft", ev.FactID)
			}
		}
	}
	if a, ok := m["acceptance"].(map[string]any); ok {
		ev.AcceptCoverage = bfeStr(a, "coverage")
		ev.AcceptRevision = bfeInt(a, "revision")
		if list, ok := a["contributions"].([]any); ok {
			for _, x := range list {
				s, _ := x.(string)
				ev.AcceptContribs = append(ev.AcceptContribs, s)
			}
		}
		if ev.Milestone == "acceptance_scope" {
			if _, ok := a["contributions"]; !ok {
				return nil, fmt.Errorf("%s: acceptance_scope requires acceptance.contributions", ev.FactID)
			}
		}
	}
	if ev.Milestone == "retraction" && ev.Supersedes == ev.FactID {
		return nil, fmt.Errorf("%s: a retraction cannot name itself", ev.FactID)
	}

	canon, err := canonicalJSON(m)
	if err != nil {
		return nil, err
	}
	fact := make(map[string]any, len(m))
	for k, v := range m {
		if !observationMembers[k] {
			fact[k] = v
		}
	}
	fcanon, err := canonicalJSON(fact)
	if err != nil {
		return nil, err
	}
	ev.Canonical = canon
	ev.Digest = sha256Tag(fcanon)
	return ev, nil
}

// obsLess orders observations of ONE fact by precedence: source authority,
// then precision, then earliest receipt, then canonical bytes.
func obsLess(a, b *BriefEvent) bool {
	ra, rb := rankOf(authorityRank, a.SourceAuthority), rankOf(authorityRank, b.SourceAuthority)
	if ra != rb {
		return ra < rb
	}
	pa, pb := rankOf(precisionRank, a.Precision), rankOf(precisionRank, b.Precision)
	if pa != pb {
		return pa < pb
	}
	if !a.ReceivedAt.Equal(b.ReceivedAt) {
		return a.ReceivedAt.Before(b.ReceivedAt)
	}
	return bytes.Compare(a.Canonical, b.Canonical) < 0
}

func rankOf(ranks map[string]int, v string) int {
	if r, ok := ranks[v]; ok {
		return r
	}
	return len(ranks) // unstated ranks last
}

// HeldConflict is a fact whose id arrived again with a different digest.
type HeldConflict struct {
	FactID     string
	HeldDigest string
	NewDigest  string
}

type storedFact struct {
	digest string
	obs    []*BriefEvent
}

// EventStore holds facts by canonical id. Same id + same digest corroborates
// (or is an idempotent duplicate); same id + different digest is held for
// inspection and never becomes a second fact.
type EventStore struct {
	facts map[string]*storedFact
	Held  []HeldConflict
}

func NewEventStore() *EventStore {
	return &EventStore{facts: map[string]*storedFact{}}
}

// Add stores one observation. It returns "added", "corroborated" or
// "duplicate", or an error naming only the fact id and the two digests.
func (s *EventStore) Add(ev *BriefEvent) (string, error) {
	f, ok := s.facts[ev.FactID]
	if !ok {
		s.facts[ev.FactID] = &storedFact{digest: ev.Digest, obs: []*BriefEvent{ev}}
		return "added", nil
	}
	if f.digest != ev.Digest {
		s.Held = append(s.Held, HeldConflict{FactID: ev.FactID, HeldDigest: f.digest, NewDigest: ev.Digest})
		return "", fmt.Errorf("fact %s: digest %s conflicts with held %s; held for inspection", ev.FactID, ev.Digest, f.digest)
	}
	for _, o := range f.obs {
		if bytes.Equal(o.Canonical, ev.Canonical) {
			return "duplicate", nil
		}
	}
	f.obs = append(f.obs, ev)
	return "corroborated", nil
}

// Preferred returns the winning observation of one fact, or nil.
func (s *EventStore) Preferred(factID string) *BriefEvent {
	f, ok := s.facts[factID]
	if !ok {
		return nil
	}
	best := f.obs[0]
	for _, o := range f.obs[1:] {
		if obsLess(o, best) {
			best = o
		}
	}
	return best
}

// All returns every stored observation (corroborations included), in a
// stable order — what an export carries so an import rebuilds the store.
func (s *EventStore) All() []*BriefEvent {
	var out []*BriefEvent
	for _, f := range s.facts {
		out = append(out, f.obs...)
	}
	sortEvents(out)
	return out
}

// Effective returns the preferred observation of every fact after supersession
// and retraction, plus a problem line for each supersession that could not be
// applied. Supersession is applied in a single pass over every stored fact: a
// fact named by any `supersedes` is withdrawn, and a retraction is itself never
// projected. Withdrawing a retraction does not restore its target; a restored
// fact is re-emitted under a fresh id.
func (s *EventStore) Effective() ([]*BriefEvent, []string) {
	var problems []string
	prefs := map[string]*BriefEvent{}
	for id := range s.facts {
		prefs[id] = s.Preferred(id)
	}
	withdrawn := map[string]bool{}
	ids := make([]string, 0, len(prefs))
	for id := range prefs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		ev := prefs[id]
		if ev.Supersedes == "" {
			continue
		}
		target, ok := prefs[ev.Supersedes]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("fact %s supersedes unknown fact %s", id, ev.Supersedes))
		case target.BriefUUID != ev.BriefUUID:
			problems = append(problems, fmt.Sprintf("fact %s supersedes fact %s of another brief; ignored", id, ev.Supersedes))
		default:
			withdrawn[ev.Supersedes] = true
		}
	}
	var out []*BriefEvent
	for _, id := range ids {
		ev := prefs[id]
		if withdrawn[id] || ev.Milestone == "retraction" {
			continue
		}
		out = append(out, ev)
	}
	sortEvents(out)
	return out, problems
}

func sortEvents(evs []*BriefEvent) {
	sort.SliceStable(evs, func(i, j int) bool {
		a, b := evs[i], evs[j]
		if !a.OccurredAt.Equal(b.OccurredAt) {
			return a.OccurredAt.Before(b.OccurredAt)
		}
		if a.FactID != b.FactID {
			return a.FactID < b.FactID
		}
		return bytes.Compare(a.Canonical, b.Canonical) < 0
	})
}

// Alias registry -----------------------------------------------------------

var (
	errAliasUnknown   = errors.New("alias names no brief at that instant")
	errAliasAmbiguous = errors.New("alias names more than one brief; refused")
)

type aliasSpan struct {
	alias, uuid, reason string
	from, until         time.Time // until zero = still in effect
}

// AliasRegistry resolves an alias at an instant from alias_assigned facts.
// Each brief's alias span ends when that brief is assigned its next alias.
type AliasRegistry struct {
	spans []aliasSpan
}

func NewAliasRegistry(evs []*BriefEvent) *AliasRegistry {
	byUUID := map[string][]*BriefEvent{}
	for _, ev := range evs {
		if ev.Milestone == "alias_assigned" {
			byUUID[ev.BriefUUID] = append(byUUID[ev.BriefUUID], ev)
		}
	}
	r := &AliasRegistry{}
	uuids := make([]string, 0, len(byUUID))
	for u := range byUUID {
		uuids = append(uuids, u)
	}
	sort.Strings(uuids)
	for _, u := range uuids {
		list := byUUID[u]
		sortEvents(list)
		for i, ev := range list {
			sp := aliasSpan{alias: ev.Alias, uuid: u, reason: ev.AliasReason, from: ev.OccurredAt}
			if i+1 < len(list) {
				sp.until = list[i+1].OccurredAt
			}
			r.spans = append(r.spans, sp)
		}
	}
	return r
}

// Resolve returns the brief uuid an alias named at `at`. An alias no span
// covers is errAliasUnknown; an alias whose covering spans belong to more than
// one brief is errAliasAmbiguous — whether the claims start together or one
// after another, since a span ends only when its OWN brief is re-aliased.
// Neither is ever guessed.
func (r *AliasRegistry) Resolve(alias string, at time.Time) (string, error) {
	uuids := map[string]bool{}
	for _, sp := range r.spans {
		if sp.alias != alias || at.Before(sp.from) {
			continue
		}
		if !sp.until.IsZero() && !at.Before(sp.until) {
			continue
		}
		uuids[sp.uuid] = true // alias-span-cover
	}
	switch len(uuids) {
	case 0:
		return "", fmt.Errorf("%s at %s: %w", alias, at.UTC().Format(time.RFC3339), errAliasUnknown)
	case 1:
		for u := range uuids {
			return u, nil
		}
	}
	return "", fmt.Errorf("%s at %s: %d candidate briefs: %w", alias, at.UTC().Format(time.RFC3339), len(uuids), errAliasAmbiguous)
}

// Historian adapter --------------------------------------------------------

// EventsFromHistory reads board-historian rows as status_observed facts. It
// never writes the log. A live row is a poll observation (the regen run saw
// the status at ts; the change happened at or before it); a backfill row is a
// commit-time reconstruction. Both derive the SAME fact id from the
// historian's own idempotency parts {brief, to, sha}, so a live row and a
// replayed row of one transition are one fact. A seed row (from "") records
// state and is never a birth. Roles are unknown: the historian records none.
// A row whose alias cannot be resolved to exactly one brief is refused, never
// guessed.
func EventsFromHistory(entries []HistoryEntry, reg *AliasRegistry, repo string) ([]*BriefEvent, []error) {
	var out []*BriefEvent
	var refused []error
	for i, e := range entries {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			refused = append(refused, fmt.Errorf("history row %d: unparseable ts", i+1))
			continue
		}
		ts = ts.UTC()
		var authority, precision string
		switch e.Source {
		case "":
			authority, precision = "historian", "poll"
		case "backfill":
			authority, precision = "backfill", "commit"
		default:
			refused = append(refused, fmt.Errorf("history row %d: unknown row source; refused", i+1))
			continue
		}
		uuid, rerr := reg.Resolve(e.Brief, ts)
		if rerr != nil {
			refused = append(refused, fmt.Errorf("history row %d: %w", i+1, rerr))
			continue
		}
		stamp := ts.Format(time.RFC3339)
		m := map[string]any{
			"schema":           eventSchemaMarker,
			"fact_id":          LegacyFactID("historian", repo+":"+e.Brief, e.SHA, "status_observed", e.To),
			"brief_uuid":       uuid,
			"alias":            e.Brief,
			"milestone":        "status_observed",
			"occurred_at":      stamp,
			"received_at":      stamp,
			"precision":        precision,
			"source_authority": authority,
			"source_ref":       map[string]any{"kind": "history-row", "revision": e.SHA},
			"coverage":         "observed",
			"initiator_role":   "unknown",
			"executor_role":    "unknown",
			"status_from":      e.From,
			"status_to":        e.To,
		}
		if authority == "historian" {
			m["occurred_before"] = stamp
		}
		ev, perr := briefEventFromMap(m)
		if perr != nil {
			refused = append(refused, fmt.Errorf("history row %d: %w", i+1, perr))
			continue
		}
		out = append(out, ev)
	}
	return out, refused
}

// Portable export ----------------------------------------------------------

// ExportCoverage summarises what an export holds and what it does not.
type ExportCoverage struct {
	ByAuthority map[string]int `json:"by_authority"`
	ByCoverage  map[string]int `json:"by_coverage"`
	Earliest    string         `json:"earliest"`
	Latest      string         `json:"latest"`
	Held        int            `json:"held"`
	Refused     int            `json:"refused"`
}

// ExportManifest is manifest.json beside events.jsonl.
type ExportManifest struct {
	Schema      string         `json:"schema"`
	EventSchema string         `json:"event_schema"`
	Events      int            `json:"events"`
	Digest      string         `json:"digest"`
	Coverage    ExportCoverage `json:"coverage"`
}

// insideGitWorkTree reports whether dir (or its nearest existing ancestor) is
// inside a git work tree: any ancestor holding a `.git` entry.
func insideGitWorkTree(dir string) (bool, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false, err
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, serr := os.Stat(filepath.Join(d, ".git")); serr == nil {
			return true, nil
		}
		if filepath.Dir(d) == d {
			return false, nil
		}
	}
}

// WriteExport writes events.jsonl and manifest.json into dir. It refuses a
// destination inside a git work tree (an export never becomes a tracked file)
// and re-validates every event first, so an in-memory event that bypassed
// ParseBriefEvent cannot carry a refused member out.
func WriteExport(dir string, evs []*BriefEvent, held, refused int) (*ExportManifest, error) {
	inside, err := insideGitWorkTree(dir)
	if err != nil {
		return nil, err
	}
	if inside {
		return nil, errors.New("export refused: destination is inside a git work tree; exports never enter tracked files")
	}
	sorted := make([]*BriefEvent, len(evs))
	copy(sorted, evs)
	sortEvents(sorted)
	cov := ExportCoverage{ByAuthority: map[string]int{}, ByCoverage: map[string]int{}, Held: held, Refused: refused}
	var buf bytes.Buffer
	for i, ev := range sorted {
		again, perr := ParseBriefEvent(ev.Canonical)
		if perr != nil {
			return nil, fmt.Errorf("export event %d: %w", i+1, perr)
		}
		buf.Write(again.Canonical)
		buf.WriteByte('\n')
		auth := again.SourceAuthority
		if auth == "" {
			auth = "unstated"
		}
		cov.ByAuthority[auth]++
		cov.ByCoverage[again.Coverage]++
		stamp := again.OccurredAt.Format(time.RFC3339Nano)
		if cov.Earliest == "" || again.OccurredAt.Before(bfeMustTime(cov.Earliest)) {
			cov.Earliest = stamp
		}
		if cov.Latest == "" || again.OccurredAt.After(bfeMustTime(cov.Latest)) {
			cov.Latest = stamp
		}
	}
	man := &ExportManifest{
		Schema:      exportSchemaMark,
		EventSchema: eventSchemaMarker,
		Events:      len(sorted),
		Digest:      sha256Tag(buf.Bytes()),
		Coverage:    cov,
	}
	mb, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := writeAtomic(filepath.Join(dir, exportEventsFile), buf.Bytes()); err != nil {
		return nil, err
	}
	if err := writeAtomic(filepath.Join(dir, exportManifest), append(mb, '\n')); err != nil {
		return nil, err
	}
	return man, nil
}

func bfeMustTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadExport reads an export back, refusing an unknown manifest schema, a
// count or digest mismatch, or any line that fails event validation.
func ReadExport(dir string) ([]*BriefEvent, *ExportManifest, error) {
	mb, err := os.ReadFile(filepath.Join(dir, exportManifest))
	if err != nil {
		return nil, nil, err
	}
	var man ExportManifest
	if err := json.Unmarshal(mb, &man); err != nil {
		return nil, nil, errors.New("manifest: not valid JSON")
	}
	if man.Schema != exportSchemaMark || man.EventSchema != eventSchemaMarker {
		return nil, nil, errors.New("manifest: unknown export or event schema version; refused")
	}
	body, err := os.ReadFile(filepath.Join(dir, exportEventsFile))
	if err != nil {
		return nil, nil, err
	}
	if got := sha256Tag(body); got != man.Digest {
		return nil, nil, fmt.Errorf("export digest %s does not match manifest %s", got, man.Digest)
	}
	var evs []*BriefEvent
	for i, line := range bytes.Split(body, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		ev, perr := ParseBriefEvent(line)
		if perr != nil {
			return nil, nil, fmt.Errorf("export line %d: %w", i+1, perr)
		}
		evs = append(evs, ev)
	}
	if len(evs) != man.Events {
		return nil, nil, fmt.Errorf("export holds %d events, manifest says %d", len(evs), man.Events)
	}
	return evs, &man, nil
}
