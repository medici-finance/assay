package main

// Acceptance tests for the brief-flow-event/v1 contract (briefevent.go,
// briefstage.go). Runner: tests/brief-flow/07.sh <mode>.
//
// Expected values come from testdata/brief-flow/contract/expect.json, which is
// hand-written from the fixture events and the spec — never produced by the
// code under test. Every assertion prints an ASSERT-FAIL[<id>] marker so the
// runner's mutation mode can prove a deliberately corrupted reducer is caught
// by the named assertion, not by an incidental failure. The fixture root can be
// pointed elsewhere with BRIEFFLOW_FIXTURES.
//
// No assertion logs a refused person identifier: the synthetic values in
// TestBriefEventRoles live only in memory and are checked for ABSENCE.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const bfeRepo = "example-org/example"

func bfeFixtureDir() string {
	if d := os.Getenv("BRIEFFLOW_FIXTURES"); d != "" {
		return d
	}
	return filepath.Join("testdata", "brief-flow", "contract")
}

func bfeFail(t *testing.T, id, format string, args ...any) {
	t.Helper()
	t.Errorf("ASSERT-FAIL["+id+"] "+format, args...)
}

func bfeLines(t *testing.T, name string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(bfeFixtureDir(), name))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] could-not-check: reading %s: %v", name, err)
	}
	var out [][]byte
	for _, l := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

func bfeParseAll(t *testing.T, name string) []*BriefEvent {
	t.Helper()
	var out []*BriefEvent
	for i, l := range bfeLines(t, name) {
		ev, err := ParseBriefEvent(l)
		if err != nil {
			t.Fatalf("ASSERT-FAIL[fixture] %s line %d refused: %v", name, i+1, err)
		}
		out = append(out, ev)
	}
	return out
}

type bfeExpEpisode struct {
	Kind   string `json:"kind"`
	FactID string `json:"fact_id"`
}

type bfeExpHistory struct {
	Seeded       bool   `json:"seeded"`
	Observations int    `json:"observations"`
	LastObserved string `json:"last_observed"`
}

type bfeExpBrief struct {
	Case            string             `json:"case"`
	Stage           string             `json:"stage"`
	Births          int                `json:"births"`
	Aliases         []string           `json:"aliases"`
	Firsts          map[string]*string `json:"firsts"`
	ReadyAtCreation bool               `json:"ready_at_creation"`
	Episodes        []bfeExpEpisode    `json:"episodes"`
	AuthoringPRs    []string           `json:"authoring_prs"`
	Abandoned       []string           `json:"abandoned"`
	OtherMerges     int                `json:"other_merges"`
	Unqualified     int                `json:"unqualified"`
	WithHistory     *bfeExpHistory     `json:"with_history"`
}

type bfeExpect struct {
	Resolve []struct {
		Alias string `json:"alias"`
		At    string `json:"at"`
		UUID  string `json:"uuid"`
		Error string `json:"error"`
	} `json:"resolve"`
	History struct {
		Rows         int               `json:"rows"`
		Refused      int               `json:"refused"`
		Events       int               `json:"events"`
		Facts        int               `json:"facts"`
		LastStatus   map[string]string `json:"last_status"`
		DoneInWindow int               `json:"done_in_window"`
	} `json:"history"`
	Projections int                    `json:"projections"`
	Briefs      map[string]bfeExpBrief `json:"briefs"`
	Flow        struct {
		BriefUUID    string `json:"brief_uuid"`
		ReceiptLines int    `json:"receipt_lines"`
		Facts        int    `json:"facts"`
		Corroborated int    `json:"corroborated"`
		Reviewed     struct {
			FactID          string `json:"fact_id"`
			SourceAuthority string `json:"source_authority"`
			BriefRevision   int    `json:"brief_revision"`
			OwnerCell       string `json:"owner_cell"`
			InitiatorRole   string `json:"initiator_role"`
			ExecutorRole    string `json:"executor_role"`
			Contribution    string `json:"contribution"`
		} `json:"reviewed"`
		DoneOwnerCell string `json:"done_owner_cell"`
		UnknownMember string `json:"unknown_member"`
	} `json:"flow"`
}

func bfeLoadExpect(t *testing.T) bfeExpect {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(bfeFixtureDir(), "expect.json"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] could-not-check: reading expect.json: %v", err)
	}
	var e bfeExpect
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] expect.json: %v", err)
	}
	return e
}

// bfeWorld is the full fixture pipeline: events + receipts into a store,
// alias registry from the effective facts, historian rows through the adapter
// into the same store, then the effective facts after supersession.
type bfeWorld struct {
	store     *EventStore
	reg       *AliasRegistry
	entries   []HistoryEntry
	hist      []*BriefEvent
	refused   []error
	effective []*BriefEvent
	results   map[string]int
}

func bfeBuild(t *testing.T, withHistory bool) *bfeWorld {
	t.Helper()
	w := &bfeWorld{store: NewEventStore(), results: map[string]int{}}
	for _, name := range []string{"events.jsonl", "receipt.jsonl"} {
		for _, ev := range bfeParseAll(t, name) {
			res, err := w.store.Add(ev)
			if err != nil {
				t.Fatalf("ASSERT-FAIL[fixture] %s: %v", name, err)
			}
			w.results[res]++
		}
	}
	eff, problems := w.store.Effective()
	if len(problems) > 0 {
		t.Fatalf("ASSERT-FAIL[fixture] supersession problems: %v", problems)
	}
	w.reg = NewAliasRegistry(eff)
	w.effective = eff
	if !withHistory {
		return w
	}
	entries, err := LoadHistory(filepath.Join(bfeFixtureDir(), "history.jsonl"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] history: %v", err)
	}
	w.entries = entries
	w.hist, w.refused = EventsFromHistory(entries, w.reg, bfeRepo)
	for _, ev := range w.hist {
		if _, err := w.store.Add(ev); err != nil {
			t.Fatalf("ASSERT-FAIL[fixture] history fact: %v", err)
		}
	}
	w.effective, problems = w.store.Effective()
	if len(problems) > 0 {
		t.Fatalf("ASSERT-FAIL[fixture] supersession problems: %v", problems)
	}
	return w
}

func bfeShort(uuid string) string { return uuid[len(uuid)-2:] }

func bfeFirstID(m *Milestone) *string {
	if m == nil {
		return nil
	}
	s := m.FactID
	return &s
}

func bfeStrPtr(p *string) string {
	if p == nil {
		return "<none>"
	}
	return *p
}

// bfeCheckBrief compares the event-derived fields of one projection with its
// hand-written expectation.
func bfeCheckBrief(t *testing.T, id string, p *BriefProjection, want bfeExpBrief) {
	t.Helper()
	if p.Stage != want.Stage {
		bfeFail(t, id, "%s: stage %q, want %q", want.Case, p.Stage, want.Stage)
	}
	if p.Births != want.Births {
		bfeFail(t, id, "%s: births %d, want %d", want.Case, p.Births, want.Births)
	}
	if !reflect.DeepEqual(p.Aliases, want.Aliases) {
		bfeFail(t, id, "%s: aliases %v, want %v", want.Case, p.Aliases, want.Aliases)
	}
	got := map[string]*Milestone{
		"written": p.Firsts.Written, "coded": p.Firsts.Coded, "reviewed": p.Firsts.Reviewed,
		"merged": p.Firsts.Merged, "verified": p.Firsts.Verified, "done": p.Firsts.Done,
	}
	for name, m := range got {
		w, ok := want.Firsts[name]
		if !ok {
			bfeFail(t, id, "%s: expectation lacks first %q", want.Case, name)
			continue
		}
		if g := bfeFirstID(m); bfeStrPtr(g) != bfeStrPtr(w) {
			bfeFail(t, id, "%s: first %s = %s, want %s", want.Case, name, bfeStrPtr(g), bfeStrPtr(w))
		}
	}
	if p.Firsts.Reviewed != nil && p.Firsts.Reviewed.ReadyAtCreation != want.ReadyAtCreation {
		bfeFail(t, id, "%s: ready_at_creation %v, want %v", want.Case, p.Firsts.Reviewed.ReadyAtCreation, want.ReadyAtCreation)
	}
	var eps []bfeExpEpisode
	for _, e := range p.Episodes {
		eps = append(eps, bfeExpEpisode{Kind: e.Kind, FactID: e.FactID})
	}
	if len(eps) != len(want.Episodes) || (len(eps) > 0 && !reflect.DeepEqual(eps, want.Episodes)) {
		bfeFail(t, id, "%s: episodes %v, want %v", want.Case, eps, want.Episodes)
	}
	if !reflect.DeepEqual(p.AuthoringPRs, want.AuthoringPRs) {
		bfeFail(t, id, "%s: authoring PRs %v, want %v", want.Case, p.AuthoringPRs, want.AuthoringPRs)
	}
	if !reflect.DeepEqual(p.Abandoned, want.Abandoned) {
		bfeFail(t, id, "%s: abandoned %v, want %v", want.Case, p.Abandoned, want.Abandoned)
	}
	if p.OtherMerges != want.OtherMerges {
		bfeFail(t, id, "%s: other merges %d, want %d", want.Case, p.OtherMerges, want.OtherMerges)
	}
	if p.Unqualified != want.Unqualified {
		bfeFail(t, id, "%s: unqualified %d, want %d", want.Case, p.Unqualified, want.Unqualified)
	}
}

// TestBriefEventIdentity — Verify row 1: renumber/rename/archive keep the
// uuid, a reused alias names a different brief, legacy ambiguity refuses,
// a historian seed is not a birth, and canonical fact ids corroborate or hold.
func TestBriefEventIdentity(t *testing.T) {
	exp := bfeLoadExpect(t)
	w := bfeBuild(t, true)

	for _, r := range exp.Resolve {
		at, _ := time.Parse(time.RFC3339, r.At)
		got, err := w.reg.Resolve(r.Alias, at)
		switch r.Error {
		case "":
			if err != nil || got != r.UUID {
				bfeFail(t, "identity-resolve", "%s at %s: got %q err %v, want %s", r.Alias, r.At, got, err, r.UUID)
			}
		case "unknown":
			if !errors.Is(err, errAliasUnknown) {
				bfeFail(t, "identity-resolve", "%s at %s: want unknown, got %q err %v", r.Alias, r.At, got, err)
			}
		case "ambiguous":
			if !errors.Is(err, errAliasAmbiguous) || got != "" {
				bfeFail(t, "identity-legacy", "%s at %s: want ambiguous refusal, got %q err %v", r.Alias, r.At, got, err)
			}
		}
	}

	if len(w.entries) != exp.History.Rows {
		bfeFail(t, "identity-history", "history rows %d, want %d", len(w.entries), exp.History.Rows)
	}
	if len(w.refused) != exp.History.Refused {
		bfeFail(t, "identity-legacy", "history refusals %d, want %d (%v)", len(w.refused), exp.History.Refused, w.refused)
	}
	for _, err := range w.refused {
		if !errors.Is(err, errAliasAmbiguous) {
			bfeFail(t, "identity-legacy", "unexpected history refusal: %v", err)
		}
	}
	if len(w.hist) != exp.History.Events {
		bfeFail(t, "identity-history", "historian events %d, want %d", len(w.hist), exp.History.Events)
	}
	ids := map[string]bool{}
	for _, ev := range w.hist {
		ids[ev.FactID] = true
		if ev.InitiatorRole != "unknown" || ev.ExecutorRole != "unknown" {
			bfeFail(t, "identity-history", "historian fact %s carries a role it never observed", ev.FactID)
		}
	}
	// The live row and the replayed row of one transition are ONE fact.
	if len(ids) != exp.History.Facts {
		bfeFail(t, "identity-fact-id", "historian facts %d, want %d (live and backfill rows must share an id)", len(ids), exp.History.Facts)
	}
	want := LegacyFactID("historian", bfeRepo+":example/21", "3333333333333333333333333333333333333333", "status_observed", "in-progress")
	if !ids[want] || !strings.HasPrefix(want, legacyFactPrefix) || len(want) != len(legacyFactPrefix)+24 {
		bfeFail(t, "identity-fact-id", "legacy fact id derivation not stable: %s", want)
	}

	proj := ProjectBriefs(w.effective)
	if len(proj) != exp.Projections {
		bfeFail(t, "identity-uuid-key", "%d projections, want %d (one per brief uuid, never per alias)", len(proj), exp.Projections)
	}
	for uuid, wb := range exp.Briefs {
		if wb.WithHistory == nil {
			continue
		}
		p := proj[uuid]
		n := bfeShort(uuid)
		if p == nil {
			bfeFail(t, "identity-uuid-key", "no projection for brief %s", uuid)
			continue
		}
		if p.Births != wb.Births {
			bfeFail(t, "identity-births-"+n, "%s: births %d, want %d", wb.Case, p.Births, wb.Births)
		}
		if !reflect.DeepEqual(p.Aliases, wb.Aliases) {
			bfeFail(t, "identity-aliases-"+n, "%s: aliases %v, want %v", wb.Case, p.Aliases, wb.Aliases)
		}
		h := wb.WithHistory
		if p.Seeded != h.Seeded || p.Observations != h.Observations || p.LastObserved != h.LastObserved {
			bfeFail(t, "identity-history-"+n, "%s: seeded/observations/last = %v/%d/%q, want %v/%d/%q",
				wb.Case, p.Seeded, p.Observations, p.LastObserved, h.Seeded, h.Observations, h.LastObserved)
		}
	}

	// Canonical fact ids: duplicate is a no-op, an observation-only difference
	// corroborates, a semantic difference under the same id is held.
	line := bfeLineByID(t, "events.jsonl", "f-1103")
	base, err := ParseBriefEvent(line)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	s := NewEventStore()
	if r, _ := s.Add(base); r != "added" {
		bfeFail(t, "identity-conflict", "first add = %q", r)
	}
	if r, _ := s.Add(base); r != "duplicate" {
		bfeFail(t, "identity-conflict", "identical re-add = %q, want duplicate", r)
	}
	later := bfeMutate(t, line, func(m map[string]any) {
		m["received_at"] = "2031-01-01T00:00:00Z"
		m["source_authority"] = "producer"
	})
	if r, err := s.Add(later); r != "corroborated" || err != nil {
		bfeFail(t, "identity-conflict", "observation-only difference = %q %v, want corroborated", r, err)
	}
	changed := bfeMutate(t, line, func(m map[string]any) { m["initiator_role"] = "human" })
	r, err := s.Add(changed)
	if err == nil || r != "" || len(s.Held) != 1 {
		bfeFail(t, "identity-conflict", "semantic difference under one id = %q %v held=%d, want held conflict", r, err, len(s.Held))
	} else if !strings.Contains(err.Error(), base.FactID) || !strings.Contains(err.Error(), base.Digest) {
		bfeFail(t, "identity-conflict", "conflict error must name the fact id and digests: %v", err)
	}
	if got := s.Preferred(base.FactID); got == nil || got.SourceAuthority != base.SourceAuthority {
		bfeFail(t, "identity-conflict", "held conflict must not displace the held fact")
	}
}

// bfeMutate decodes a fixture line, applies f and re-parses it.
func bfeMutate(t *testing.T, line []byte, f func(map[string]any)) *BriefEvent {
	t.Helper()
	ev, err := ParseBriefEvent(bfeMutLine(t, line, f))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] mutated line refused: %v", err)
	}
	return ev
}

func bfeMutLine(t *testing.T, line []byte, f func(map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	f(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	return b
}

// TestBriefEventStage — Verify row 2: first milestones stay distinct from
// episodes, and partial, abandoned or parallel contributions never imply a
// whole-brief stage the acceptance scope does not support.
func TestBriefEventStage(t *testing.T) {
	exp := bfeLoadExpect(t)
	w := bfeBuild(t, false)
	proj := ProjectBriefs(w.effective)
	uuids := make([]string, 0, len(exp.Briefs))
	for u := range exp.Briefs {
		uuids = append(uuids, u)
	}
	sort.Strings(uuids)
	for _, u := range uuids {
		p := proj[u]
		if p == nil {
			bfeFail(t, "identity-uuid-key", "no projection for brief %s", u)
			continue
		}
		bfeCheckBrief(t, "stage-"+bfeShort(u), p, exp.Briefs[u])
	}

	// Replay order is occurrence order, not input order.
	rev := make([]*BriefEvent, len(w.effective))
	for i, ev := range w.effective {
		rev[len(rev)-1-i] = ev
	}
	again := ProjectBriefs(rev)
	if !reflect.DeepEqual(proj, again) {
		bfeFail(t, "stage-order", "projection depends on input order")
	}

	// Completion is never inferred: every verified/done stage has a
	// complete-coverage acceptance fact behind it.
	for u, p := range proj {
		if (p.Stage == "verified" && p.Firsts.Verified == nil) || (p.Stage == "done" && p.Firsts.Done == nil) {
			bfeFail(t, "stage-completion", "brief %s reports %s without a complete acceptance fact", u, p.Stage)
		}
	}
}

// TestBriefEventCompat — Verify row 3: the historian log and its readers are
// unchanged, unknown mandatory semantics refuse, optional members round-trip.
func TestBriefEventCompat(t *testing.T) {
	exp := bfeLoadExpect(t)
	histPath := filepath.Join(bfeFixtureDir(), "history.jsonl")
	raw, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] could-not-check: %v", err)
	}
	entries, err := LoadHistory(histPath)
	if err != nil {
		bfeFail(t, "compat-history", "LoadHistory refused the fixture log: %v", err)
	}
	if got := LastRecordedStatus(entries); !reflect.DeepEqual(got, exp.History.LastStatus) {
		bfeFail(t, "compat-history", "LastRecordedStatus = %v, want %v", got, exp.History.LastStatus)
	}
	since := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC)
	streams := []*Stream{{Name: "example", Briefs: []Brief{{Num: "21", Effort: "M"}}}}
	rep := computeThroughput(streams, entries, since, until)
	if rep.Total.Count != exp.History.DoneInWindow || rep.Total.UnknownEffort != 0 || rep.State != "ok" {
		bfeFail(t, "compat-throughput", "throughput count=%d unknown=%d state=%s, want %d/0/ok",
			rep.Total.Count, rep.Total.UnknownEffort, rep.State, exp.History.DoneInWindow)
	}

	// Reading the log through the adapter leaves the rows untouched, and the
	// historian's own encoder still writes the fixture byte-for-byte.
	before := append([]HistoryEntry(nil), entries...)
	w := bfeBuild(t, false)
	EventsFromHistory(entries, w.reg, bfeRepo)
	if !reflect.DeepEqual(before, entries) {
		bfeFail(t, "compat-history", "adapter mutated historian rows")
	}
	out := filepath.Join(t.TempDir(), "history.jsonl")
	if err := appendHistory(out, entries); err != nil {
		t.Fatalf("ASSERT-FAIL[compat-history] %v", err)
	}
	if got, _ := os.ReadFile(out); !bytes.Equal(got, raw) {
		bfeFail(t, "compat-history", "historian re-encode is not byte-identical to the fixture log")
	}

	base := bfeLineByID(t, "events.jsonl", "f-1103")
	refuse := []struct {
		id  string
		mut func(map[string]any)
	}{
		{"compat-must-understand", func(m map[string]any) { m["must_understand"] = []any{"x_required_future"} }},
		{"compat-schema-version", func(m map[string]any) { m["schema"] = "brief-flow-event/v2" }},
		{"compat-unknown-milestone", func(m map[string]any) { m["milestone"] = "deployed" }},
		{"compat-required", func(m map[string]any) { delete(m, "contribution") }},
		{"compat-required", func(m map[string]any) { delete(m, "brief_uuid") }},
		{"compat-required", func(m map[string]any) { delete(m["contribution"].(map[string]any), "kind") }},
		{"compat-bounds", func(m map[string]any) { m["occurred_before"] = "2030-01-01T00:00:00Z" }},
		{"compat-bounds", func(m map[string]any) { m["received_at"] = "2029-01-01T00:00:00Z" }},
		{"compat-bounds", func(m map[string]any) { m["occurred_at"] = "2030-02-02 09:00:00" }},
		{"compat-uuid", func(m map[string]any) { m["brief_uuid"] = "example/01" }},
	}
	for _, c := range refuse {
		if _, err := ParseBriefEvent(bfeMutLine(t, base, c.mut)); err == nil {
			bfeFail(t, c.id, "a mandatory-semantics violation was accepted")
		}
	}
	if _, err := ParseBriefEvent([]byte(`{"schema":"brief-flow-event/v1","schema":"brief-flow-event/v1"}`)); err == nil {
		bfeFail(t, "compat-duplicate", "duplicate member accepted")
	}
	if _, err := ParseBriefEvent(bfeMutLine(t, base, func(m map[string]any) { m["must_understand"] = []any{"owner_cell"} })); err != nil {
		bfeFail(t, "compat-must-understand", "a known must_understand member was refused: %v", err)
	}
	noVis, _ := ParseBriefEvent(bfeMutLine(t, base, func(m map[string]any) { delete(m, "visibility") }))
	if noVis == nil || noVis.EffectiveVisibility() != "restricted" {
		bfeFail(t, "compat-visibility", "absent visibility must read as restricted")
	}

	// Optional unknown member round-trips through export and import.
	var withExt *BriefEvent
	for _, ev := range bfeParseAll(t, "receipt.jsonl") {
		if bytes.Contains(ev.Canonical, []byte(`"`+exp.Flow.UnknownMember+`"`)) {
			withExt = ev
		}
	}
	if withExt == nil {
		t.Fatalf("ASSERT-FAIL[fixture] no receipt line carries %s", exp.Flow.UnknownMember)
	}
	dir := filepath.Join(t.TempDir(), "export")
	if _, err := WriteExport(dir, []*BriefEvent{withExt}, 0, 0); err != nil {
		t.Fatalf("ASSERT-FAIL[compat-round-trip] export: %v", err)
	}
	back, man, err := ReadExport(dir)
	if err != nil || len(back) != 1 || man.Events != 1 {
		bfeFail(t, "compat-round-trip", "import: %v (%d events)", err, len(back))
	} else if !bytes.Equal(back[0].Canonical, withExt.Canonical) || back[0].Digest != withExt.Digest {
		bfeFail(t, "compat-round-trip", "optional member did not round-trip unchanged")
	}

	// A tampered export is refused, not read.
	ev := filepath.Join(dir, exportEventsFile)
	b, _ := os.ReadFile(ev)
	if err := os.WriteFile(ev, bytes.Replace(b, []byte("example-cell"), []byte("example-cel1"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadExport(dir); err == nil {
		bfeFail(t, "compat-export-digest", "tampered export accepted")
	}
}

// TestBriefEventSchemaParity pins the embedded schema to the canonical file
// and the validator to every keyword the schema uses.
func TestBriefEventSchemaParity(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "schemas", "brief-flow-event-v1.json"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[compat-schema-parity] could-not-check: %v", err)
	}
	if !bytes.Equal(canonical, embeddedEventSchemaBytes()) {
		bfeFail(t, "compat-schema-parity", "statusgen/schemas/brief-flow-event-v1.json differs from schemas/brief-flow-event-v1.json — re-copy the canonical file")
	}
	s, err := eventSchema()
	if err != nil {
		t.Fatalf("ASSERT-FAIL[compat-schema-parity] %v", err)
	}
	assertValidatorCoversKeywords(t, s)
}

// TestBriefEventFlow — Verify row 4: synthetic producer receipts and forge
// events → public event validation → store → projected brief keep the uuid,
// revision, cell and source authority at occurrence; the export round-trips.
func TestBriefEventFlow(t *testing.T) {
	exp := bfeLoadExpect(t)
	if n := len(bfeLines(t, "receipt.jsonl")); n != exp.Flow.ReceiptLines {
		t.Fatalf("ASSERT-FAIL[fixture] receipt lines %d, want %d", n, exp.Flow.ReceiptLines)
	}
	rs := NewEventStore()
	counts := map[string]int{}
	for _, ev := range bfeParseAll(t, "receipt.jsonl") {
		r, err := rs.Add(ev)
		if err != nil {
			bfeFail(t, "flow-corroborate", "receipt refused: %v", err)
		}
		counts[r]++
	}
	if counts["added"] != exp.Flow.Facts || counts["corroborated"] != exp.Flow.Corroborated {
		bfeFail(t, "flow-corroborate", "added=%d corroborated=%d, want %d/%d", counts["added"], counts["corroborated"], exp.Flow.Facts, exp.Flow.Corroborated)
	}

	w := bfeBuild(t, true)
	proj := ProjectBriefs(w.effective)
	p := proj[exp.Flow.BriefUUID]
	if p == nil {
		t.Fatalf("ASSERT-FAIL[identity-uuid-key] no projection for the flow brief")
	}
	bfeCheckBrief(t, "flow-stage", p, exp.Briefs[exp.Flow.BriefUUID])
	r := p.Firsts.Reviewed
	wr := exp.Flow.Reviewed
	if r == nil || r.FactID != wr.FactID || r.SourceAuthority != wr.SourceAuthority || r.BriefRevision != wr.BriefRevision ||
		r.OwnerCell != wr.OwnerCell || r.InitiatorRole != wr.InitiatorRole || r.ExecutorRole != wr.ExecutorRole || r.Contribution != wr.Contribution {
		bfeFail(t, "flow-reviewed", "reviewed milestone %+v, want %+v", r, wr)
	}
	if d := p.Firsts.Done; d == nil || d.OwnerCell != exp.Flow.DoneOwnerCell {
		bfeFail(t, "flow-owner-at-occurrence", "done milestone owner %v, want %s", d, exp.Flow.DoneOwnerCell)
	}
	if h := exp.Briefs[exp.Flow.BriefUUID].WithHistory; p.Seeded != h.Seeded || p.Observations != h.Observations || p.LastObserved != h.LastObserved {
		bfeFail(t, "flow-history", "historian observations %v/%d/%q, want %v/%d/%q", p.Seeded, p.Observations, p.LastObserved, h.Seeded, h.Observations, h.LastObserved)
	}

	// Precedence among observations of one fact.
	mk := func(auth, prec, rx string) *BriefEvent {
		return &BriefEvent{SourceAuthority: auth, Precision: prec, ReceivedAt: mustParseTS(rx), Canonical: []byte(auth + prec + rx)}
	}
	order := []*BriefEvent{
		mk("forge", "day", "2030-01-02T00:00:00Z"),
		mk("producer", "event", "2030-01-01T00:00:00Z"),
		mk("git", "event", "2030-01-01T00:00:00Z"),
		mk("historian", "event", "2030-01-01T00:00:00Z"),
		mk("backfill", "event", "2030-01-01T00:00:00Z"),
		mk("", "event", "2030-01-01T00:00:00Z"),
	}
	for i := 0; i+1 < len(order); i++ {
		if !obsLess(order[i], order[i+1]) || obsLess(order[i+1], order[i]) {
			bfeFail(t, "flow-precedence", "authority %q must outrank %q", order[i].SourceAuthority, order[i+1].SourceAuthority)
		}
	}
	precs := []*BriefEvent{
		mk("forge", "event", "2030-01-03T00:00:00Z"), mk("forge", "commit", "2030-01-01T00:00:00Z"),
		mk("forge", "poll", "2030-01-01T00:00:00Z"), mk("forge", "day", "2030-01-01T00:00:00Z"),
		mk("forge", "", "2030-01-01T00:00:00Z"),
	}
	for i := 0; i+1 < len(precs); i++ {
		if !obsLess(precs[i], precs[i+1]) {
			bfeFail(t, "flow-precedence", "precision %q must outrank %q", precs[i].Precision, precs[i+1].Precision)
		}
	}
	if !obsLess(mk("forge", "event", "2030-01-01T00:00:00Z"), mk("forge", "event", "2030-01-02T00:00:00Z")) {
		bfeFail(t, "flow-precedence", "earlier receipt must win a tie")
	}

	// Durable export → import → identical projection.
	all := w.store.All()
	dir := filepath.Join(t.TempDir(), "export")
	man, err := WriteExport(dir, all, len(w.store.Held), len(w.refused))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[flow-export] %v", err)
	}
	if man.Events != len(all) || man.Coverage.Refused != len(w.refused) || man.Coverage.ByAuthority["forge"] == 0 {
		bfeFail(t, "flow-export", "manifest %+v does not describe the export", man)
	}
	back, _, err := ReadExport(dir)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[flow-export] %v", err)
	}
	s2 := NewEventStore()
	for _, ev := range back {
		if _, err := s2.Add(ev); err != nil {
			bfeFail(t, "flow-export", "re-import: %v", err)
		}
	}
	eff2, _ := s2.Effective()
	if !reflect.DeepEqual(ProjectBriefs(eff2), proj) {
		bfeFail(t, "flow-export", "projection after export/import differs")
	}
}

func mustParseTS(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// bfeRoleEnums finds every `role`-named property with an enum in a schema.
func bfeRoleEnums(v any, out *[][]string) {
	m, ok := v.(map[string]any)
	if !ok {
		if a, ok := v.([]any); ok {
			for _, x := range a {
				bfeRoleEnums(x, out)
			}
		}
		return
	}
	if props, ok := m["properties"].(map[string]any); ok {
		if r, ok := props["role"].(map[string]any); ok {
			if e, ok := r["enum"].([]any); ok {
				var s []string
				for _, x := range e {
					s = append(s, x.(string))
				}
				*out = append(*out, s)
			}
		}
	}
	for _, x := range m {
		bfeRoleEnums(x, out)
	}
}

// TestBriefEventRoles — Verify row 6: role-only actors agree with the
// run-record role list, initiator and executor stay distinct, unknown stays
// unknown, an in-memory forbidden identifier is refused without disclosure,
// and an export can never land in a git work tree or the historian log.
func TestBriefEventRoles(t *testing.T) {
	want := []string{"desk", "reviewer", "verifier", "worker", "human", "unknown"}
	s, err := eventSchema()
	if err != nil {
		t.Fatalf("ASSERT-FAIL[roles-enum] %v", err)
	}
	props := s.raw["properties"].(map[string]any)
	for _, k := range []string{"initiator_role", "executor_role"} {
		var got []string
		for _, x := range props[k].(map[string]any)["enum"].([]any) {
			got = append(got, x.(string))
		}
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(eventRoles, want) {
			bfeFail(t, "roles-enum", "%s enum %v, want %v", k, got, want)
		}
	}

	// The run-record contract's role list (graph-execution/06) is ours minus
	// the explicit unknown.
	doc, err := os.ReadFile(filepath.Join("..", "docs", "streams", "graph-execution", "brief-06-run-records-and-replay.md"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[roles-run-record] could-not-check: %v", err)
	}
	const runRoles = "`desk | reviewer | verifier | worker | human`"
	if !bytes.Contains(doc, []byte(runRoles)) {
		bfeFail(t, "roles-run-record", "run-record role list not found; the role vocabulary may have drifted")
	} else {
		list := strings.Split(strings.Trim(runRoles, "`"), " | ")
		if !reflect.DeepEqual(append(list, "unknown"), want) {
			bfeFail(t, "roles-run-record", "run-record roles %v disagree with %v", list, want)
		}
	}
	pb, err := os.ReadFile(filepath.Join("..", "schemas", "workflow-pattern-v1.json"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[roles-pattern] could-not-check: %v", err)
	}
	ps, _ := parseSchema(pb)
	var enums [][]string
	bfeRoleEnums(ps.raw, &enums)
	if len(enums) == 0 {
		bfeFail(t, "roles-pattern", "no role enum found in the workflow pattern schema")
	}
	known := map[string]bool{}
	for _, r := range want {
		known[r] = true
	}
	for _, e := range enums {
		for _, r := range e {
			if !known[r] {
				bfeFail(t, "roles-pattern", "pattern role %q is outside the event role vocabulary", r)
			}
		}
	}

	// Initiator and executor stay distinct; unknown stays unknown.
	w := bfeBuild(t, true)
	p := ProjectBriefs(w.effective)["00000000-0000-4000-8000-000000000021"]
	if p == nil || p.Firsts.Reviewed == nil || p.Firsts.Reviewed.InitiatorRole != "desk" || p.Firsts.Reviewed.ExecutorRole != "reviewer" {
		bfeFail(t, "roles-distinct", "initiator/executor roles were merged or lost")
	}
	for _, ev := range w.hist {
		if ev.InitiatorRole != "unknown" || ev.ExecutorRole != "unknown" {
			bfeFail(t, "roles-unknown", "unknown role was defaulted to %s/%s", ev.InitiatorRole, ev.ExecutorRole)
		}
	}
	base := bfeLineByID(t, "events.jsonl", "f-1103")
	if _, err := ParseBriefEvent(bfeMutLine(t, base, func(m map[string]any) { delete(m, "executor_role") })); err == nil {
		bfeFail(t, "roles-missing", "an event without an executor role was accepted")
	}

	// A synthetic identifier, generated at run time, never written anywhere.
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		t.Fatalf("ASSERT-FAIL[roles-login] could-not-check: %v", err)
	}
	secret := "zq" + hex.EncodeToString(rb)
	cases := []func(map[string]any){
		func(m map[string]any) { m["login"] = secret },
		func(m map[string]any) { m["Display-Name"] = secret },
		func(m map[string]any) { m["reviewer_login"] = secret },
		func(m map[string]any) { m["contribution"].(map[string]any)["author"] = secret },
		func(m map[string]any) { m["x_note"] = secret + "@example.invalid" },
		func(m map[string]any) { m["initiator_role"] = secret },
		func(m map[string]any) {
			m["source_ref"] = map[string]any{"kind": "forge", "id": secret + "@example.invalid"}
		},
	}
	for i, mut := range cases {
		_, err := ParseBriefEvent(bfeMutLine(t, base, mut))
		if err == nil {
			bfeFail(t, "roles-login", "case %d: a person identifier was accepted", i+1)
			continue
		}
		if strings.Contains(err.Error(), secret) {
			bfeFail(t, "roles-no-disclose", "case %d: the refusal disclosed the refused value", i+1)
		}
	}

	// An event that bypassed ParseBriefEvent cannot carry it out: the export
	// re-validates every line.
	good, _ := ParseBriefEvent(base)
	forged := *good
	forged.Canonical = bfeMutLine(t, base, func(m map[string]any) { m["login"] = secret })
	dir := filepath.Join(t.TempDir(), "export")
	if _, err := WriteExport(dir, []*BriefEvent{good, &forged}, 0, 0); err == nil {
		bfeFail(t, "roles-export-revalidate", "export wrote an event carrying a person identifier")
	} else if strings.Contains(err.Error(), secret) {
		bfeFail(t, "roles-no-disclose", "export refusal disclosed the refused value")
	}
	if _, err := os.Stat(filepath.Join(dir, exportEventsFile)); err == nil {
		bfeFail(t, "roles-export-revalidate", "a refused export still wrote events")
	}
	okDir := filepath.Join(t.TempDir(), "export")
	if _, err := WriteExport(okDir, w.store.All(), 0, 0); err != nil {
		t.Fatalf("ASSERT-FAIL[roles-no-persist] %v", err)
	}
	for _, f := range []string{exportEventsFile, exportManifest} {
		if b, _ := os.ReadFile(filepath.Join(okDir, f)); bytes.Contains(b, []byte(secret)) {
			bfeFail(t, "roles-no-persist", "%s holds the refused value", f)
		}
	}

	// Never into a git work tree, and the historian log is untouched.
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(repo, filepath.FromSlash(historyRelPath))
	raw, _ := os.ReadFile(filepath.Join(bfeFixtureDir(), "history.jsonl"))
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "docs", "streams", "export")
	if _, err := WriteExport(target, w.store.All(), 0, 0); err == nil {
		bfeFail(t, "roles-export-git", "export into a git work tree was accepted")
	}
	if _, err := os.Stat(target); err == nil {
		bfeFail(t, "roles-export-git", "a refused export created its directory")
	}
	if after, _ := os.ReadFile(logPath); !bytes.Equal(after, raw) {
		bfeFail(t, "roles-export-git", "the historian log changed")
	}

	// The event code has no path to the historian's writer.
	for _, f := range []string{"briefevent.go", "briefstage.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("ASSERT-FAIL[roles-no-writer] could-not-check: %v", err)
		}
		for _, call := range []string{"appendHistory(", "recordHistory(", "historyRelPath"} {
			if bytes.Contains(src, []byte(call)) {
				bfeFail(t, "roles-no-writer", "%s references the historian writer (%s)", f, call)
			}
		}
	}
}

// bfeLineByID returns the fixture line carrying fact id id.
func bfeLineByID(t *testing.T, name, id string) []byte {
	t.Helper()
	needle := []byte(`"fact_id":"` + id + `"`)
	for _, l := range bfeLines(t, name) {
		if bytes.Contains(l, needle) {
			return l
		}
	}
	t.Fatalf("ASSERT-FAIL[fixture] %s has no fact %s", name, id)
	return nil
}
