package main

// Wait receipts, measurement context and human-intervention records — the
// extension of brief-flow-event/v1 specified in spec/brief-flow-event-v1.md
// §14–§18 — and the two projection profiles built from them.
//
// Everything here is pure: parse, deduplicate, count, project. The only file
// writer (WriteRestrictedProfile) refuses a git work tree, exactly as the
// export writer does. Nothing here reads or writes the board historian; the
// historian refuses human-act lines on its own (history.go), so the two
// layers fail independently.
//
// The committed profile is produced by an ALLOWLIST copy (committedShape):
// only members the allowlist names are copied out of the restricted profile.
// Nothing is produced by deleting known fields, so a member added to the
// restricted side later never reaches committed bytes unless the allowlist is
// changed to name it.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	restrictedProfileSchema = "brief-flow-restricted/v1"
	committedProfileSchema  = "brief-flow-committed/v1"
	// publicationAudience is the only audience a committed profile serves:
	// anyone who can read the repository it is written into.
	publicationAudience = "repository-reader"
	restrictedFile      = "restricted.json"
)

// waitEdges are the five named waits (spec §15), in report order.
var waitEdges = []string{"intake_dispatch", "dispatch_worker", "worker_review", "review_verify", "finding_remediation"}

// ---------------------------------------------------------------------------
// Measurement context (§14.2)

// FlowContext is the optional measurement context of a fact or wait. An empty
// string is "the source did not say"; Known renders it as `unknown`. Nothing
// is inferred, and requested, resolved and served model are never filled in
// from one another.
type FlowContext struct {
	Purpose         string
	PurposeTaxonomy string
	PurposeRevision string
	PolicyRevision  string
	Harness         string
	Build           string
	ModelRequested  string
	ModelResolved   string
	ModelServed     string
	FindingRef      string
	FixRef          string
	External        []ExternalRef
}

// ExternalRef points at an external quality, cost or adoption source. It
// records where a figure lives, never the figure or a verdict about it.
type ExternalRef struct {
	Kind     string
	Source   string
	Ref      string
	Revision string
}

// Known renders an unstated context value as the explicit `unknown`.
func Known(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

func flowContextOf(m map[string]any) *FlowContext {
	c, ok := m["context"].(map[string]any)
	if !ok {
		return nil
	}
	fc := &FlowContext{
		PolicyRevision: bfeStr(c, "policy_revision"),
		Harness:        bfeStr(c, "harness"),
		Build:          bfeStr(c, "build"),
		FindingRef:     bfeStr(c, "finding_ref"),
		FixRef:         bfeStr(c, "fix_ref"),
	}
	if p, ok := c["purpose"].(map[string]any); ok {
		fc.Purpose = bfeStr(p, "value")
		fc.PurposeTaxonomy = bfeStr(p, "taxonomy")
		fc.PurposeRevision = bfeStr(p, "effective_revision")
	}
	if md, ok := c["model"].(map[string]any); ok {
		fc.ModelRequested = bfeStr(md, "requested")
		fc.ModelResolved = bfeStr(md, "resolved")
		fc.ModelServed = bfeStr(md, "served")
	}
	if list, ok := c["external"].([]any); ok {
		for _, x := range list {
			e, _ := x.(map[string]any)
			fc.External = append(fc.External, ExternalRef{
				Kind: bfeStr(e, "kind"), Source: bfeStr(e, "source"),
				Ref: bfeStr(e, "ref"), Revision: bfeStr(e, "revision"),
			})
		}
	}
	return fc
}

// ---------------------------------------------------------------------------
// Records

// WaitRecord is one inter-stage wait receipt (§15).
type WaitRecord struct {
	FactID          string
	BriefUUID       string
	Edge            string
	StartedAt       time.Time
	EndedAt         time.Time // zero = still open
	ReceivedAt      time.Time
	Coverage        string
	SourceAuthority string
	StartFact       string
	EndFact         string
	Context         *FlowContext
	Digest          string
	Canonical       []byte
}

// Open reports a wait that has not ended: aging, never a completed duration.
func (w *WaitRecord) Open() bool { return w.EndedAt.IsZero() }

// ActEffect is one correlated effect of a human act.
type ActEffect struct {
	Ref     string
	Outcome string
}

// InterventionRecord is one evidenced human act (§16).
type InterventionRecord struct {
	ActID            string
	Category         string
	Taxonomy         string
	BriefUUIDs       []string // sorted, distinct; empty when Unallocated
	Unallocated      bool
	InitiatorRole    string
	ExecutorRole     string
	Outcome          string
	OccurredAt       time.Time
	ReceivedAt       time.Time // zero = not stated
	SubmittedAt      time.Time // zero = not stated
	Channel          string
	Effects          []ActEffect
	AttributionBasis string // `unknown` when absent
	SourceAuthority  string
	SourceRef        string // canonical JSON of source_ref; restricted pointer
	Digest           string
	Canonical        []byte
}

// Qualified reports whether the act may enter a precise category count: a
// human initiated it, the authorized source itself authenticated that, and
// it carries a precise category. A role other than `human`, on-behalf-of
// app metadata, unknown attribution and `unclassified` all count as
// unqualified — visible, but never a precise count.
func (r *InterventionRecord) Qualified() bool {
	return r.InitiatorRole == "human" && r.AttributionBasis == "authenticated_source" && r.Category != "unclassified"
}

// FlowRecord is one parsed line of any record kind.
type FlowRecord struct {
	Kind string // fact | wait | human_intervention
	Fact *BriefEvent
	Wait *WaitRecord
	Act  *InterventionRecord
}

// ParseFlowRecord validates one JSONL line of any record kind. A fact goes
// through ParseBriefEvent's own path unchanged; the two new kinds share its
// checks (person scan, schema, must_understand) in the same order.
func ParseFlowRecord(line []byte) (*FlowRecord, error) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil, errors.New("empty record line")
	}
	if !json.Valid(line) {
		return nil, errors.New("record is not valid JSON")
	}
	decoded, err := bfeDecodeStrict(line)
	if err != nil {
		return nil, err
	}
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil, errors.New("record is not a JSON object")
	}
	kind, _ := m["record"].(string)
	if _, present := m["record"]; !present || kind == "fact" {
		ev, err := briefEventFromMap(m)
		if err != nil {
			return nil, err
		}
		return &FlowRecord{Kind: "fact", Fact: ev}, nil
	}
	if err := bfeCheckRecord(m); err != nil {
		return nil, err
	}
	switch kind {
	case "wait":
		w, err := waitFromMap(m)
		if err != nil {
			return nil, err
		}
		return &FlowRecord{Kind: kind, Wait: w}, nil
	case "human_intervention":
		a, err := actFromMap(m)
		if err != nil {
			return nil, err
		}
		return &FlowRecord{Kind: kind, Act: a}, nil
	}
	return nil, errors.New("record: unknown record kind (value not shown)")
}

// recordDigest is the §5.2 digest rule applied to any record kind: canonical
// JSON of every member except the observation members. It is the ONE digest
// computation; the fact reader uses it too. `record: fact` is the explicit
// spelling of an absent `record` (§14.1), so it never enters the digest: the
// two spellings of one fact corroborate instead of conflicting.
func recordDigest(m map[string]any) ([]byte, string, error) {
	canon, err := canonicalJSON(m)
	if err != nil {
		return nil, "", err
	}
	fact := make(map[string]any, len(m))
	for k, v := range m {
		if observationMembers[k] {
			continue
		}
		if k == "record" && v == "fact" { // record-fact-digest
			continue
		}
		fact[k] = v
	}
	fcanon, err := canonicalJSON(fact)
	if err != nil {
		return nil, "", err
	}
	return canon, sha256Tag(fcanon), nil
}

func waitFromMap(m map[string]any) (*WaitRecord, error) {
	wm, _ := m["wait"].(map[string]any)
	w := &WaitRecord{
		FactID:          bfeStr(m, "fact_id"),
		BriefUUID:       bfeStr(m, "brief_uuid"),
		Edge:            bfeStr(wm, "edge"),
		Coverage:        bfeStr(m, "coverage"),
		SourceAuthority: bfeStr(m, "source_authority"),
		StartFact:       bfeStr(wm, "start_fact"),
		EndFact:         bfeStr(wm, "end_fact"),
		Context:         flowContextOf(m),
	}
	var err error
	if w.StartedAt, err = bfeTime(wm, "started_at"); err != nil {
		return nil, fmt.Errorf("wait.%w", err)
	}
	if w.EndedAt, err = bfeTime(wm, "ended_at"); err != nil {
		return nil, fmt.Errorf("wait.%w", err)
	}
	if w.ReceivedAt, err = bfeTime(m, "received_at"); err != nil {
		return nil, err
	}
	if !w.EndedAt.IsZero() && w.EndedAt.Before(w.StartedAt) {
		return nil, errors.New("wait.ended_at: earlier than wait.started_at")
	}
	if w.ReceivedAt.Before(w.StartedAt) {
		return nil, errors.New("received_at: earlier than wait.started_at")
	}
	if w.Canonical, w.Digest, err = recordDigest(m); err != nil {
		return nil, err
	}
	return w, nil
}

func actFromMap(m map[string]any) (*InterventionRecord, error) {
	r := &InterventionRecord{
		ActID:            bfeStr(m, "act_id"),
		Category:         bfeStr(m, "category"),
		Taxonomy:         bfeStr(m, "taxonomy"),
		InitiatorRole:    bfeStr(m, "initiator_role"),
		ExecutorRole:     bfeStr(m, "executor_role"),
		Outcome:          bfeStr(m, "outcome"),
		SourceAuthority:  bfeStr(m, "source_authority"),
		AttributionBasis: "unknown",
	}
	_, hasBriefs := m["brief_uuids"]
	_, hasScope := m["scope"]
	if hasBriefs == hasScope {
		return nil, errors.New("human_intervention: exactly one of brief_uuids and scope is required")
	}
	r.Unallocated = hasScope
	if list, ok := m["brief_uuids"].([]any); ok {
		seen := map[string]bool{}
		for i, x := range list {
			s, _ := x.(string)
			if seen[s] {
				return nil, fmt.Errorf("brief_uuids[%d]: repeats a uuid already listed", i)
			}
			seen[s] = true
			r.BriefUUIDs = append(r.BriefUUIDs, s)
		}
		sort.Strings(r.BriefUUIDs)
	}
	if a, ok := m["attribution"].(map[string]any); ok {
		if b := bfeStr(a, "basis"); b != "" {
			r.AttributionBasis = b
		}
	}
	if sub, ok := m["submission"].(map[string]any); ok {
		r.Channel = bfeStr(sub, "channel")
		t, err := bfeTime(sub, "submitted_at")
		if err != nil {
			return nil, fmt.Errorf("submission.%w", err)
		}
		r.SubmittedAt = t
	}
	if list, ok := m["effects"].([]any); ok {
		for _, x := range list {
			e, _ := x.(map[string]any)
			r.Effects = append(r.Effects, ActEffect{Ref: bfeStr(e, "ref"), Outcome: bfeStr(e, "outcome")})
		}
	}
	if sr, ok := m["source_ref"]; ok {
		b, err := canonicalJSON(sr)
		if err != nil {
			return nil, err
		}
		r.SourceRef = string(b)
	}
	var err error
	if r.OccurredAt, err = bfeTime(m, "occurred_at"); err != nil {
		return nil, err
	}
	if r.ReceivedAt, err = bfeTime(m, "received_at"); err != nil {
		return nil, err
	}
	if !r.ReceivedAt.IsZero() && r.ReceivedAt.Before(r.OccurredAt) {
		return nil, errors.New("received_at: earlier than occurred_at")
	}
	if r.Canonical, r.Digest, err = recordDigest(m); err != nil {
		return nil, err
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// Deduplication (§17.1)

type storedAct struct {
	obs  []*InterventionRecord
	held bool
}

// InterventionStore holds acts by act id. A transport retry (byte-identical)
// and a mirror (same act, other source or receipt time) of one act add zero;
// the same act id with a different act is a held conflict, excluded from
// counts and reported in coverage. A deliberate resubmission carries a new
// act id and adds one.
type InterventionStore struct {
	acts  map[string]*storedAct
	order []string
}

func NewInterventionStore() *InterventionStore {
	return &InterventionStore{acts: map[string]*storedAct{}}
}

// Add stores one observation of an act and returns "added", "duplicate" or
// "corroborated", or an error for a held conflict. Errors never quote the
// act id: it is a restricted pointer.
func (s *InterventionStore) Add(r *InterventionRecord) (string, error) {
	st, ok := s.acts[r.ActID]
	if !ok { // act-dedup
		s.acts[r.ActID] = &storedAct{obs: []*InterventionRecord{r}}
		s.order = append(s.order, r.ActID)
		return "added", nil
	}
	if st.obs[0].Digest != r.Digest {
		st.held = true
		return "", errors.New("human_intervention: act id already holds a different act; held conflict (act id not shown)")
	}
	for _, o := range st.obs {
		if bytes.Equal(o.Canonical, r.Canonical) {
			return "duplicate", nil
		}
	}
	st.obs = append(st.obs, r)
	return "corroborated", nil
}

// Acts returns the preferred observation of every act that is not held, in
// first-seen order, and the number of held acts.
func (s *InterventionStore) Acts() ([]*InterventionRecord, int) {
	var out []*InterventionRecord
	held := 0
	for _, id := range s.order {
		st := s.acts[id]
		if st.held {
			held++
			continue
		}
		best := st.obs[0]
		for _, o := range st.obs[1:] {
			if actLess(o, best) {
				best = o
			}
		}
		out = append(out, best)
	}
	return out, held
}

func actLess(a, b *InterventionRecord) bool {
	ra, rb := rankOf(authorityRank, a.SourceAuthority), rankOf(authorityRank, b.SourceAuthority)
	if ra != rb {
		return ra < rb
	}
	if !a.ReceivedAt.Equal(b.ReceivedAt) {
		return a.ReceivedAt.Before(b.ReceivedAt)
	}
	return bytes.Compare(a.Canonical, b.Canonical) < 0
}

// dedupWaits applies the §5.3 rule to wait receipts by fact id: duplicates and
// corroborations collapse to one wait; a different wait under one id is held.
func dedupWaits(ws []*WaitRecord) ([]*WaitRecord, int) {
	type slot struct {
		w    *WaitRecord
		held bool
	}
	by := map[string]*slot{}
	var order []string
	for _, w := range ws {
		s, ok := by[w.FactID]
		if !ok {
			by[w.FactID] = &slot{w: w}
			order = append(order, w.FactID)
			continue
		}
		if s.w.Digest != w.Digest {
			s.held = true
			continue
		}
		if rankOf(authorityRank, w.SourceAuthority) < rankOf(authorityRank, s.w.SourceAuthority) {
			s.w = w
		}
	}
	var out []*WaitRecord
	held := 0
	for _, id := range order {
		if by[id].held {
			held++
			continue
		}
		out = append(out, by[id].w)
	}
	return out, held
}

// ---------------------------------------------------------------------------
// Projection (§18)

// PublicationPolicy is a recorded decision that intervention counts drawn
// from the named sources may be published to an audience. A reader's or an
// operator's permission to READ a source is not this decision.
type PublicationPolicy struct {
	Audience          string
	Revision          string
	AuthorizedSources []string
}

func (p *PublicationPolicy) permits(source string) bool {
	if p == nil || p.Audience != publicationAudience || p.Revision == "" || source == "" {
		return false
	}
	for _, s := range p.AuthorizedSources {
		if s == source {
			return true
		}
	}
	return false
}

// ProjectionInput is everything one projection reads.
type ProjectionInput struct {
	From, To time.Time // window: [From, To)
	AsOf     time.Time
	Build    string
	// Briefs are the authorized brief keys the report covers. Every key gets a
	// block, so a brief with no acts is distinguishable from one never read.
	Briefs []string
	Acts   *InterventionStore
	// ActSource names the intervention source; ActCoverage is whether its read
	// over the window was complete (`observed`, `partial`, `unknown`; empty is
	// `unknown`). Unknown coverage makes every intervention count unknown.
	ActSource   string
	ActCoverage string
	Waits       []*WaitRecord
}

// leaf marks an allowlisted scalar; a map[string]any nests; a []any holds
// the shape of each list element.
type leaf struct{}

// committedShape is the committed profile's ALLOWLIST: the only members that
// may reach committed bytes. Scope/brief keys, window, as-of, build and
// schema versions, numeric flow values, category/outcome counts and coverage
// summaries. No actor or role field, act id, source/effect/receipt pointer,
// watermark or manifest is named, so none can be copied.
var committedShape = map[string]any{
	"schema":          leaf{},
	"event_schema":    leaf{},
	"build":           leaf{},
	"window":          map[string]any{"from": leaf{}, "to": leaf{}},
	"as_of":           leaf{}, // committed-as-of
	"policy_revision": leaf{},
	"interventions": map[string]any{
		"state": leaf{}, "coverage": leaf{}, "reason": leaf{}, "held": leaf{},
		"briefs": []any{map[string]any{
			"brief": leaf{}, "state": leaf{}, "coverage": leaf{},
			"counts": []any{map[string]any{"category": leaf{}, "outcome": leaf{}, "count": leaf{}}},
			"total":  leaf{}, "unqualified": leaf{},
		}},
		"fleet": map[string]any{
			"state": leaf{}, "coverage": leaf{},
			"counts": []any{map[string]any{"category": leaf{}, "outcome": leaf{}, "count": leaf{}}},
			"total":  leaf{}, "unqualified": leaf{}, "unallocated": leaf{},
		},
	},
	"waits": []any{map[string]any{
		"edge": leaf{}, "state": leaf{}, "coverage": leaf{}, "n": leaf{}, "open": leaf{},
		"held": leaf{}, "median_seconds": leaf{}, "p85_seconds": leaf{},
	}},
}

// allowCopy copies from v only what shape names. Unknown members are never
// copied; nothing is deleted.
func allowCopy(v any, shape any) (any, bool) {
	switch sh := shape.(type) {
	case leaf:
		switch v.(type) {
		case map[string]any, []any, []map[string]any:
			return nil, false // a leaf never carries structure
		}
		return v, true
	case map[string]any:
		src, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		out := map[string]any{}
		for k, sub := range sh {
			if x, present := src[k]; present {
				if c, ok := allowCopy(x, sub); ok {
					out[k] = c
				}
			}
		}
		return out, true
	case []any:
		var items []any
		switch t := v.(type) {
		case []any:
			items = t
		case []map[string]any:
			for _, x := range t {
				items = append(items, x)
			}
		default:
			return nil, false
		}
		out := []any{}
		for _, x := range items {
			if c, ok := allowCopy(x, sh[0]); ok {
				out = append(out, c)
			}
		}
		return out, true
	}
	return nil, false
}

// actRow is one restricted (act, brief) row; Brief is "" for an unallocated
// act. Restricted only: it carries the act id and both roles.
func actRows(acts []*InterventionRecord, from, to time.Time) []map[string]any {
	var rows []map[string]any
	for _, a := range acts {
		if a.OccurredAt.Before(from) || !a.OccurredAt.Before(to) {
			continue
		}
		briefs := a.BriefUUIDs
		if a.Unallocated {
			briefs = []string{""}
		}
		for _, b := range briefs {
			rows = append(rows, map[string]any{
				"act_id": a.ActID, "brief": b, "category": a.Category, "outcome": a.Outcome,
				"taxonomy": a.Taxonomy, "initiator_role": a.InitiatorRole, "executor_role": a.ExecutorRole,
				"attribution": a.AttributionBasis, "qualified": a.Qualified(),
				"occurred_at": a.OccurredAt.Format(time.RFC3339), "source_ref": a.SourceRef,
				"effects": len(a.Effects),
			})
		}
	}
	return rows
}

// countBlock groups the qualified rows by category and outcome. Unqualified
// rows are counted, never placed in a precise category.
func countBlock(rows []map[string]any) map[string]any {
	type key struct{ cat, out string }
	n := map[key]int{}
	total, unq := 0, 0
	for _, r := range rows {
		if q, _ := r["qualified"].(bool); !q {
			unq++
			continue
		}
		n[key{r["category"].(string), r["outcome"].(string)}]++
		total++
	}
	keys := make([]key, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].cat != keys[j].cat {
			return keys[i].cat < keys[j].cat
		}
		return keys[i].out < keys[j].out
	})
	counts := []any{}
	for _, k := range keys {
		counts = append(counts, map[string]any{"category": k.cat, "outcome": k.out, "count": n[k]})
	}
	return map[string]any{"counts": counts, "total": total, "unqualified": unq}
}

func normCoverage(c string) string {
	if c == "observed" || c == "partial" {
		return c
	}
	return "unknown"
}

// interventionBlock builds the per-brief and fleet counts. Unknown coverage
// gives an explicit unknown state and no number: missing input is never zero.
func interventionBlock(in ProjectionInput, rows []map[string]any, held int) map[string]any {
	cov := normCoverage(in.ActCoverage)
	if held > 0 && cov == "observed" {
		cov = "partial" // a held conflict is a gap in what can be counted
	}
	known := cov == "observed" || cov == "partial" // coverage-known
	block := map[string]any{"coverage": cov, "held": held}
	if !known {
		block["state"] = "unknown"
	} else {
		block["state"] = "published"
	}
	var briefs []any
	for _, b := range in.Briefs {
		bb := map[string]any{"brief": b, "coverage": cov}
		if !known {
			bb["state"] = "unknown"
		} else {
			var mine []map[string]any
			for _, r := range rows {
				if r["brief"] == b {
					mine = append(mine, r)
				}
			}
			for k, v := range countBlock(mine) {
				bb[k] = v
			}
			bb["state"] = "observed"
		}
		briefs = append(briefs, bb)
	}
	if briefs == nil {
		briefs = []any{}
	}
	block["briefs"] = briefs
	fleet := map[string]any{"coverage": cov}
	if !known {
		fleet["state"] = "unknown"
	} else {
		// The fleet total is the DISTINCT UNION of acts: an act linked to two
		// briefs is one act fleet-wide.
		seen := map[string]bool{}
		var distinct []map[string]any
		unalloc := 0
		for _, r := range rows {
			id := r["act_id"].(string)
			if seen[id] { // fleet-union
				continue
			}
			seen[id] = true
			distinct = append(distinct, r)
			if r["brief"] == "" {
				unalloc++
			}
		}
		for k, v := range countBlock(distinct) {
			fleet[k] = v
		}
		fleet["unallocated"] = unalloc
		fleet["state"] = "observed"
	}
	block["fleet"] = fleet
	return block
}

// medianSeconds is the §10 median: the middle value, or the mean of the two
// central values when n is even.
func medianSeconds(d []float64) float64 {
	if len(d) == 0 {
		return 0
	}
	s := append([]float64(nil), d...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// p85Seconds is the §10 nearest rank: the value at rank ⌈0.85·n⌉ (1-based).
func p85Seconds(d []float64) float64 {
	if len(d) == 0 {
		return 0
	}
	s := append([]float64(nil), d...)
	sort.Float64s(s)
	r := int(math.Ceil(0.85 * float64(len(s))))
	if r < 1 {
		r = 1
	}
	return s[r-1]
}

// coverageRank (briefstage.go) orders observed < partial < unknown.

// waitBlocks computes per-edge wait statistics. A duration belongs to the
// window containing its exit; open waits are aging, never durations. An edge
// with no completed wait is `unknown` and carries no statistic.
func waitBlocks(in ProjectionInput, ws []*WaitRecord, held map[string]int) ([]any, []map[string]any) {
	var stats []any
	var rows []map[string]any
	for _, edge := range waitEdges {
		var durs []float64
		open := 0
		cov := ""
		for _, w := range ws {
			if w.Edge != edge {
				continue
			}
			if cov == "" || coverageRank[normCoverage(w.Coverage)] > coverageRank[cov] {
				cov = normCoverage(w.Coverage)
			}
			row := map[string]any{
				"fact_id": w.FactID, "brief": w.BriefUUID, "edge": edge,
				"started_at": w.StartedAt.Format(time.RFC3339), "start_fact": w.StartFact,
				"end_fact": w.EndFact, "coverage": w.Coverage,
			}
			if w.Open() {
				if w.StartedAt.Before(in.To) {
					open++
				}
			} else {
				row["ended_at"] = w.EndedAt.Format(time.RFC3339)
				if !w.EndedAt.Before(in.From) && w.EndedAt.Before(in.To) {
					durs = append(durs, w.EndedAt.Sub(w.StartedAt).Seconds())
				}
			}
			rows = append(rows, row)
		}
		if cov == "" {
			cov = "unknown"
		}
		if held[edge] > 0 && cov == "observed" {
			cov = "partial"
		}
		st := map[string]any{"edge": edge, "coverage": cov, "n": len(durs), "open": open, "held": held[edge]}
		if len(durs) == 0 { // wait-no-data
			st["state"] = "unknown"
		} else {
			st["state"] = "observed"
			st["median_seconds"] = medianSeconds(durs)
			st["p85_seconds"] = p85Seconds(durs)
		}
		stats = append(stats, st)
	}
	return stats, rows
}

// BuildRestrictedProfile builds the operator-only profile: every act and wait
// row (act ids, roles, source pointers) plus the counts and statistics
// derived from them. It is never written into a git work tree.
func BuildRestrictedProfile(in ProjectionInput) map[string]any {
	var acts []*InterventionRecord
	heldActs := 0
	if in.Acts != nil {
		acts, heldActs = in.Acts.Acts()
	}
	rows := actRows(acts, in.From, in.To)
	ws, _ := dedupWaits(in.Waits)
	heldByEdge := heldWaitsByEdge(in.Waits)
	stats, waitRows := waitBlocks(in, ws, heldByEdge)
	if rows == nil {
		rows = []map[string]any{}
	}
	if waitRows == nil {
		waitRows = []map[string]any{}
	}
	prof := map[string]any{
		"schema":        restrictedProfileSchema,
		"event_schema":  eventSchemaMarker,
		"build":         in.Build,
		"window":        map[string]any{"from": in.From.Format(time.RFC3339), "to": in.To.Format(time.RFC3339)},
		"as_of":         in.AsOf.Format(time.RFC3339),
		"act_source":    in.ActSource,
		"act_rows":      rows,
		"wait_rows":     waitRows,
		"interventions": interventionBlock(in, rows, heldActs),
		"waits":         stats,
	}
	// No policy revision: the restricted profile gates nothing. The committed
	// profile cites the policy that decided its own publication (§18.3).
	return prof
}

func heldWaitsByEdge(ws []*WaitRecord) map[string]int {
	first := map[string]*WaitRecord{}
	heldIDs := map[string]bool{}
	for _, w := range ws {
		if f, ok := first[w.FactID]; ok {
			if f.Digest != w.Digest {
				heldIDs[w.FactID] = true
			}
			continue
		}
		first[w.FactID] = w
	}
	out := map[string]int{}
	for id := range heldIDs {
		out[first[id].Edge]++
	}
	return out
}

// BuildCommittedProfile projects a restricted profile to the committed
// profile by allowlist copy. Without a recorded publication policy that
// authorizes the intervention source for repository readers, the
// intervention block is HELD — no counts — while wait statistics render.
// `policy_revision` is set here, from that same policy, and only when it
// authorizes publication: no revision is ever carried over from the input.
func BuildCommittedProfile(restricted map[string]any, policy *PublicationPolicy) map[string]any {
	src := map[string]any{}
	for k, v := range restricted {
		if k != "policy_revision" { // policy-revision-own
			src[k] = v
		}
	}
	src["schema"] = committedProfileSchema
	source, _ := restricted["act_source"].(string)
	if !policy.permits(source) {
		src["interventions"] = map[string]any{
			"state":  "held",
			"reason": "no recorded publication policy authorizes this intervention source for repository readers",
		}
	}
	if policy.permits(source) {
		src["policy_revision"] = policy.Revision
	}
	out, _ := allowCopy(src, committedShape)
	return out.(map[string]any)
}

// CheckCommitted independently re-walks committed bytes and refuses any
// member the allowlist does not name, at any depth. A writer of committed
// output runs it on the exact bytes it is about to write.
func CheckCommitted(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return errors.New("committed profile is not valid JSON")
	}
	return checkCommittedShape(v, committedShape, "")
}

func checkCommittedShape(v any, shape any, path string) error {
	switch sh := shape.(type) {
	case leaf:
		switch v.(type) {
		case map[string]any, []any:
			return fmt.Errorf("%s: structure where a value is allowed; refused", bfeLabel(path))
		}
		return nil
	case map[string]any:
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: not an object; refused", bfeLabel(path))
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			sub, ok := sh[k]
			if !ok {
				// The member name is not schema-authored: show its position only.
				return fmt.Errorf("%s: member #%d is outside the committed allowlist; refused (member not shown)", bfeLabel(path), i+1)
			}
			if err := checkCommittedShape(m[k], sub, joinPath(path, k)); err != nil {
				return err
			}
		}
		return nil
	case []any:
		list, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: not a list; refused", bfeLabel(path))
		}
		for i, x := range list {
			if err := checkCommittedShape(x, sh[0], fmt.Sprintf("%s[%d]", bfeLabel(path), i)); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("committed allowlist malformed")
}

// CommittedBytes renders a committed profile and checks it against the
// allowlist before returning it.
func CommittedBytes(prof map[string]any) ([]byte, error) {
	b, err := canonicalJSON(prof)
	if err != nil {
		return nil, err
	}
	if err := CheckCommitted(b); err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// WriteRestrictedProfile writes the restricted profile into dir as
// restricted.json. Like the export writer it refuses a git work tree,
// creates dir owner-only and writes the file owner-only.
func WriteRestrictedProfile(dir string, prof map[string]any) error {
	inside, err := insideGitWorkTree(dir)
	if err != nil {
		return err
	}
	if inside {
		return errors.New("restricted profile refused: destination is inside a git work tree; restricted data never enters tracked files")
	}
	b, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, restrictedFile), append(b, '\n'))
}
