package main

// Acceptance tests for the wait, context and human-intervention records of
// brief-flow-event/v1 (spec §14–§18; briefintervention.go). Runner:
// tests/brief-flow/17.sh <mode>.
//
// Expected values come from testdata/brief-flow/interventions/expect.json,
// hand-written from the fixtures and the spec — never produced by the code
// under test. Every assertion prints an ASSERT-FAIL[<id>] marker so the
// runner's mutation mode can prove a corrupted reducer is caught by the named
// assertion. Fixtures are synthetic; the one forbidden login built here lives
// in memory only and is checked for ABSENCE, never logged.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	actBriefA = "00000000-0000-4000-8000-00000000000a"
	actBriefB = "00000000-0000-4000-8000-00000000000b"
	actBriefC = "00000000-0000-4000-8000-00000000000c"
)

var (
	actFrom = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	actTo   = time.Date(2030, 1, 8, 0, 0, 0, 0, time.UTC)
)

func actFixtureDir() string {
	return filepath.Join("testdata", "brief-flow", "interventions")
}

type actExpect struct {
	Contract struct {
		Briefs map[string]struct {
			Counts      map[string]int `json:"counts"`
			Total       int            `json:"total"`
			Unqualified int            `json:"unqualified"`
		} `json:"briefs"`
		Fleet struct {
			Counts      map[string]int `json:"counts"`
			Total       int            `json:"total"`
			Unqualified int            `json:"unqualified"`
			Unallocated int            `json:"unallocated"`
		} `json:"fleet"`
		UnqualifiedActs []string `json:"unqualified_acts"`
		ConsoleEffects  int      `json:"console_effects"`
	} `json:"contract"`
	Dedup struct {
		Adds        []string       `json:"adds"`
		Before      map[string]int `json:"before"`
		ResubmitAdd string         `json:"resubmit_add"`
		After       map[string]int `json:"after"`
	} `json:"dedup"`
	Waits                 map[string]map[string]any `json:"waits"`
	FlowCommitted         json.RawMessage           `json:"flow_committed"`
	FlowHeld              json.RawMessage           `json:"flow_held_interventions"`
	ForbiddenKeys         []string                  `json:"committed_forbidden_keys"`
	ForbiddenValues       []string                  `json:"committed_forbidden_values"`
	ForbiddenScalarValues []string                  `json:"committed_forbidden_scalars"`
}

func actLoadExpect(t *testing.T) actExpect {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(actFixtureDir(), "expect.json"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] could-not-check: %v", err)
	}
	var e actExpect
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] expect.json: %v", err)
	}
	return e
}

func actLines(t *testing.T, name string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(actFixtureDir(), name))
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

func actParseAll(t *testing.T, name string) []*FlowRecord {
	t.Helper()
	var out []*FlowRecord
	for i, l := range actLines(t, name) {
		r, err := ParseFlowRecord(l)
		if err != nil {
			t.Fatalf("ASSERT-FAIL[fixture] %s line %d refused: %v", name, i+1, err)
		}
		out = append(out, r)
	}
	return out
}

func actStore(t *testing.T, names ...string) *InterventionStore {
	t.Helper()
	s := NewInterventionStore()
	for _, n := range names {
		for _, r := range actParseAll(t, n) {
			if r.Act == nil {
				t.Fatalf("ASSERT-FAIL[fixture] %s holds a %s record", n, r.Kind)
			}
			if _, err := s.Add(r.Act); err != nil {
				t.Fatalf("ASSERT-FAIL[fixture] %s: %v", n, err)
			}
		}
	}
	return s
}

func actWaits(t *testing.T, name string) []*WaitRecord {
	t.Helper()
	var ws []*WaitRecord
	for _, r := range actParseAll(t, name) {
		if r.Wait == nil {
			t.Fatalf("ASSERT-FAIL[fixture] %s holds a %s record", name, r.Kind)
		}
		ws = append(ws, r.Wait)
	}
	return ws
}

func actInput(acts *InterventionStore, briefs ...string) ProjectionInput {
	return ProjectionInput{
		From: actFrom, To: actTo, AsOf: actTo, Build: "example-build-1",
		Briefs: briefs, Acts: acts, ActSource: "example-console", ActCoverage: "observed",
	}
}

// actNorm round-trips v through JSON so maps, ints and floats compare by value.
func actNorm(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	return out
}

// actCounts flattens a projection count block to "category/outcome" → n.
func actCounts(t *testing.T, block map[string]any) map[string]int {
	t.Helper()
	out := map[string]int{}
	list, _ := block["counts"].([]any)
	for _, x := range list {
		c := x.(map[string]any)
		out[c["category"].(string)+"/"+c["outcome"].(string)] = int(actNum(c["count"]))
	}
	return out
}

func actNum(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case float64:
		return n
	}
	return -1
}

func actBriefBlock(prof map[string]any, uuid string) map[string]any {
	iv, _ := prof["interventions"].(map[string]any)
	list, _ := iv["briefs"].([]any)
	for _, x := range list {
		if m, _ := x.(map[string]any); m["brief"] == uuid {
			return m
		}
	}
	return nil
}

func actFleet(prof map[string]any) map[string]any {
	iv, _ := prof["interventions"].(map[string]any)
	f, _ := iv["fleet"].(map[string]any)
	return f
}

func actMutLine(t *testing.T, line []byte, f func(map[string]any)) []byte {
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

// TestFlowCompat — Verify row 1: every /07 golden fixture reads unchanged
// through the extended reader, the /07 fact reader refuses the new kinds,
// optional members round-trip and an unknown mandatory member refuses.
func TestFlowCompat(t *testing.T) {
	contract := filepath.Join("testdata", "brief-flow", "contract")
	for _, f := range []string{"events.jsonl", "receipt.jsonl", "overlap.jsonl"} {
		b, err := os.ReadFile(filepath.Join(contract, f))
		if err != nil {
			t.Fatalf("ASSERT-FAIL[fixture] could-not-check: %v", err)
		}
		for i, l := range bytes.Split(b, []byte("\n")) {
			if len(bytes.TrimSpace(l)) == 0 {
				continue
			}
			old, oerr := ParseBriefEvent(l)
			rec, rerr := ParseFlowRecord(l)
			if (oerr == nil) != (rerr == nil) {
				bfeFail(t, "flowcompat-golden", "%s line %d: fact reader and flow reader disagree on acceptance", f, i+1)
				continue
			}
			if oerr != nil {
				continue // a golden refusal stays a refusal
			}
			if rec.Kind != "fact" || rec.Fact == nil || rec.Fact.Digest != old.Digest || !bytes.Equal(rec.Fact.Canonical, old.Canonical) {
				bfeFail(t, "flowcompat-golden", "%s line %d: a /07 fact did not read unchanged", f, i+1)
			}
		}
	}
	if _, err := LoadHistory(filepath.Join(contract, "history.jsonl")); err != nil {
		bfeFail(t, "flowcompat-golden", "the /07 historian fixture is refused: %v", err)
	}

	wait := actLines(t, "waits.jsonl")[0]
	act := actLines(t, "contract.jsonl")[0]
	for _, l := range [][]byte{wait, act} {
		if _, err := ParseBriefEvent(l); err == nil {
			bfeFail(t, "flowcompat-kind", "the fact reader accepted a non-fact record kind")
		}
	}
	refuse := []struct {
		id   string
		line []byte
		mut  func(map[string]any)
	}{
		{"flowcompat-must-understand", wait, func(m map[string]any) { m["must_understand"] = []any{"x_required_future"} }},
		{"flowcompat-must-understand", act, func(m map[string]any) { m["must_understand"] = []any{"x_required_future"} }},
		{"flowcompat-kind", act, func(m map[string]any) { m["record"] = "x_future_kind" }},
		{"flowcompat-kind", wait, func(m map[string]any) { m["milestone"] = "written" }},
		{"flowcompat-kind", act, func(m map[string]any) { m["milestone"] = "written" }},
		{"flowcompat-required", wait, func(m map[string]any) { delete(m, "coverage") }},
		{"flowcompat-required", wait, func(m map[string]any) { delete(m["wait"].(map[string]any), "edge") }},
		{"flowcompat-required", act, func(m map[string]any) { delete(m, "outcome") }},
		{"flowcompat-required", act, func(m map[string]any) { delete(m, "initiator_role") }},
		{"flowcompat-required", act, func(m map[string]any) { delete(m, "brief_uuids") }},
		{"flowcompat-required", act, func(m map[string]any) { m["scope"] = "unallocated" }},
		{"flowcompat-bounds", act, func(m map[string]any) { m["brief_uuids"] = []any{actBriefA, actBriefA} }},
		{"flowcompat-bounds", wait, func(m map[string]any) { m["wait"].(map[string]any)["ended_at"] = "2030-01-01T00:00:00Z" }},
		{"flowcompat-bounds", act, func(m map[string]any) { m["received_at"] = "2029-01-01T00:00:00Z" }},
		{"flowcompat-enum", wait, func(m map[string]any) { m["wait"].(map[string]any)["edge"] = "verify_deploy" }},
		{"flowcompat-enum", act, func(m map[string]any) { m["category"] = "approval" }},
		{"flowcompat-enum", act, func(m map[string]any) { m["outcome"] = "succeeded" }},
	}
	for i, c := range refuse {
		if _, err := ParseFlowRecord(actMutLine(t, c.line, c.mut)); err == nil {
			bfeFail(t, c.id, "case %d: a mandatory-semantics violation was accepted", i+1)
		}
	}
	// An optional unknown member round-trips byte-for-byte in canonical form,
	// and a known must_understand member is accepted.
	for _, l := range [][]byte{wait, act} {
		r, err := ParseFlowRecord(actMutLine(t, l, func(m map[string]any) { m["x_note"] = "a/b" }))
		if err != nil {
			bfeFail(t, "flowcompat-round-trip", "an optional unknown member was refused: %v", err)
			continue
		}
		var canon []byte
		if r.Wait != nil {
			canon = r.Wait.Canonical
		} else {
			canon = r.Act.Canonical
		}
		if !bytes.Contains(canon, []byte(`"x_note":"a/b"`)) {
			bfeFail(t, "flowcompat-round-trip", "the optional member did not round-trip")
		}
	}
	// The original source receipt pointer survives parsing as a restricted
	// pointer, unchanged.
	r, _ := ParseFlowRecord(act)
	if r == nil || r.Act.SourceRef != `{"id":"src-act-0001","kind":"console-act","revision":"1"}` {
		bfeFail(t, "flowcompat-receipt", "the source receipt pointer did not round-trip")
	}
}

// TestFlowContext — Verify row 2: optional context round-trips, unknown
// stays unknown, the three model fields stay distinct, and no adoption
// claim, inferred effort or extra milestone appears.
func TestFlowContext(t *testing.T) {
	recs := actParseAll(t, "context.jsonl")
	if len(recs) != 3 {
		t.Fatalf("ASSERT-FAIL[fixture] context.jsonl has %d records, want 3", len(recs))
	}
	full, sparse, none := recs[0].Fact.Context, recs[1].Fact.Context, recs[2].Fact.Context
	if full == nil || sparse == nil {
		t.Fatalf("ASSERT-FAIL[context-round-trip] context was dropped")
	}
	if full.Purpose != "maintenance" || full.PurposeTaxonomy != "purpose-taxonomy/v1" || full.PurposeRevision != "3" ||
		full.PolicyRevision != "policy-7" || full.Harness != "example-harness" || full.Build != "example-build-1" ||
		full.FindingRef != "src-finding-1" || full.FixRef != "src-fix-1" || len(full.External) != 3 {
		bfeFail(t, "context-round-trip", "a source-qualified context member was lost")
	}
	if full.ModelRequested != "model-large" || full.ModelResolved != "model-large-2030-01" || full.ModelServed != "model-small-2030-01" {
		bfeFail(t, "context-model", "requested/resolved/served model were merged or inferred")
	}
	if sparse.Purpose != "unknown" || Known(sparse.ModelResolved) != "unknown" || Known(sparse.ModelServed) != "unknown" ||
		sparse.ModelRequested != "model-small" || Known(sparse.Harness) != "unknown" {
		bfeFail(t, "context-unknown", "an unstated context value was filled in")
	}
	if none != nil {
		bfeFail(t, "context-unknown", "context was invented for a fact that carries none")
	}
	if !bytes.Contains(recs[0].Fact.Canonical, []byte(`"served":"model-small-2030-01"`)) {
		bfeFail(t, "context-round-trip", "context is not in the canonical bytes")
	}
	// Context is observation metadata: adding it never changes the fact.
	plain := actMutLine(t, actLines(t, "context.jsonl")[0], func(m map[string]any) { delete(m, "context") })
	if p, err := ParseFlowRecord(plain); err != nil || p.Fact.Digest != recs[0].Fact.Digest {
		bfeFail(t, "context-digest", "context changed the fact digest")
	}
	ws := actWaits(t, "waits.jsonl")
	if ws[0].Context == nil || ws[0].Context.Purpose != "maintenance" {
		bfeFail(t, "context-round-trip", "wait context was lost")
	}
	// External references point at a source; they carry no figure.
	if _, err := ParseFlowRecord(actMutLine(t, actLines(t, "context.jsonl")[0], func(m map[string]any) {
		m["context"].(map[string]any)["external"] = []any{map[string]any{"kind": "effort", "source": "x"}}
	})); err == nil {
		bfeFail(t, "context-no-claim", "an external reference of kind effort was accepted")
	}

	// The milestone vocabulary is exactly /07's: no extra milestone.
	s, err := eventSchema()
	if err != nil {
		t.Fatalf("ASSERT-FAIL[context-milestones] %v", err)
	}
	props := s.raw["properties"].(map[string]any)
	var ms []string
	for _, x := range props["milestone"].(map[string]any)["enum"].([]any) {
		ms = append(ms, x.(string))
	}
	want := []string{"written", "alias_assigned", "status_observed", "pr_opened", "pr_ready", "pr_draft", "pr_merged",
		"pr_closed", "pr_reopened", "acceptance_scope", "verified", "done", "reopened", "retraction"}
	if !reflect.DeepEqual(ms, want) {
		bfeFail(t, "context-milestones", "milestone enum %v, want /07's %v", ms, want)
	}
	// No schema member asserts adoption, effort, cost or quality as a value.
	raw, _ := json.Marshal(s.raw["properties"])
	for _, k := range []string{`"effort"`, `"human_effort"`, `"adopted"`, `"adoption_rate"`, `"cost_usd"`, `"tokens"`, `"quality_score"`} {
		if bytes.Contains(raw, []byte(k+":")) {
			bfeFail(t, "context-no-claim", "the schema declares a %s member", k)
		}
	}
}

// regression: an act linked to two briefs is one act fleet-wide; unknown coverage is never zero (mutations fleet-union, missing-zero).
// TestActContract — Verify row 3: one decision, one ruling and one
// multi-key console override count 1/1/1 (total 3); failed outcomes and
// effects round-trip; bots, on-behalf-of metadata, unknown attribution and
// unclassified acts never become precise counts.
func TestActContract(t *testing.T) {
	exp := actLoadExpect(t)
	store := actStore(t, "contract.jsonl")
	prof := BuildRestrictedProfile(actInput(store, actBriefA, actBriefB, actBriefC))
	for _, u := range []string{actBriefA, actBriefB, actBriefC} {
		want := exp.Contract.Briefs[u]
		b := actBriefBlock(prof, u)
		if b == nil {
			bfeFail(t, "act-brief", "no block for brief %s", u)
			continue
		}
		if got := actCounts(t, b); !reflect.DeepEqual(got, want.Counts) || actNum(b["total"]) != float64(want.Total) {
			bfeFail(t, "act-brief", "brief %s counts %v total %v, want %v total %d", u, got, b["total"], want.Counts, want.Total)
		}
		if actNum(b["unqualified"]) != float64(want.Unqualified) {
			bfeFail(t, "act-unqualified", "brief %s unqualified %v, want %d", u, b["unqualified"], want.Unqualified)
		}
	}
	f := actFleet(prof)
	if got := actCounts(t, f); !reflect.DeepEqual(got, exp.Contract.Fleet.Counts) || actNum(f["total"]) != float64(exp.Contract.Fleet.Total) {
		bfeFail(t, "act-fleet", "fleet counts %v total %v, want %v total %d", got, f["total"], exp.Contract.Fleet.Counts, exp.Contract.Fleet.Total)
	}
	if actNum(f["unqualified"]) != float64(exp.Contract.Fleet.Unqualified) || actNum(f["unallocated"]) != float64(exp.Contract.Fleet.Unallocated) {
		bfeFail(t, "act-unqualified", "fleet unqualified %v unallocated %v", f["unqualified"], f["unallocated"])
	}
	var unq []string
	seen := map[string]bool{}
	for _, r := range prof["act_rows"].([]map[string]any) {
		if q, _ := r["qualified"].(bool); !q && !seen[r["act_id"].(string)] {
			seen[r["act_id"].(string)] = true
			unq = append(unq, r["act_id"].(string))
		}
	}
	sort.Strings(unq)
	if !reflect.DeepEqual(unq, exp.Contract.UnqualifiedActs) {
		bfeFail(t, "act-unqualified", "unqualified acts %v, want %v", unq, exp.Contract.UnqualifiedActs)
	}

	// Effects and a failed outcome round-trip; a failed act is its own count.
	acts, _ := store.Acts()
	var console *InterventionRecord
	for _, a := range acts {
		if a.Category == "console" && a.Qualified() {
			console = a
		}
	}
	if console == nil || len(console.Effects) != exp.Contract.ConsoleEffects || console.Effects[2].Outcome != "failed" || console.Channel != "console" {
		bfeFail(t, "act-effects", "console effects or channel did not round-trip")
	}
	failed := actStore(t, "contract.jsonl", "failed.jsonl")
	fp := BuildRestrictedProfile(actInput(failed, actBriefC))
	if got := actCounts(t, actBriefBlock(fp, actBriefC)); got["console/failed"] != 1 || got["console/applied"] != 1 {
		bfeFail(t, "act-failed", "a failed act was dropped or merged: %v", got)
	}

	// Unknown coverage: an explicit unknown, never a zero.
	in := actInput(store, actBriefA)
	in.ActCoverage = ""
	up := BuildRestrictedProfile(in)
	for _, blk := range []map[string]any{actBriefBlock(up, actBriefA), actFleet(up)} {
		if blk["state"] != "unknown" {
			bfeFail(t, "act-missing-zero", "unknown coverage rendered state %v", blk["state"])
		}
		for _, k := range []string{"counts", "total", "unqualified"} {
			if _, ok := blk[k]; ok {
				bfeFail(t, "act-missing-zero", "unknown coverage rendered a %s value", k)
			}
		}
	}
}

// regression: a retry or mirror of one act must add zero (mutation act-dedup in tests/brief-flow/17.sh).
// TestActDedup — Verify row 4: a transport retry and a mirror add zero, a
// two-brief ruling counts once per brief and once fleet-wide, and a
// deliberate resubmission adds one.
func TestActDedup(t *testing.T) {
	exp := actLoadExpect(t)
	store := NewInterventionStore()
	var adds []string
	for _, r := range actParseAll(t, "dedup.jsonl") {
		got, err := store.Add(r.Act)
		if err != nil {
			bfeFail(t, "act-dedup", "a retry or mirror was held: %v", err)
		}
		adds = append(adds, got)
	}
	if !reflect.DeepEqual(adds, exp.Dedup.Adds) {
		bfeFail(t, "act-dedup", "adds %v, want %v", adds, exp.Dedup.Adds)
	}
	check := func(label string, want map[string]int) {
		prof := BuildRestrictedProfile(actInput(store, actBriefA, actBriefB))
		for _, u := range []string{actBriefA, actBriefB} {
			if got := actNum(actBriefBlock(prof, u)["total"]); got != float64(want[u]) {
				bfeFail(t, "act-dedup", "%s: brief %s total %v, want %d", label, u, got, want[u])
			}
		}
		if got := actNum(actFleet(prof)["total"]); got != float64(want["fleet"]) {
			bfeFail(t, "act-fleet-union", "%s: fleet total %v, want %d", label, got, want["fleet"])
		}
	}
	check("retry+mirror", exp.Dedup.Before)
	re := actParseAll(t, "resubmit.jsonl")
	if got, err := store.Add(re[0].Act); err != nil || got != exp.Dedup.ResubmitAdd {
		bfeFail(t, "act-resubmit", "resubmission add = %q (%v), want %q", got, err, exp.Dedup.ResubmitAdd)
	}
	check("resubmission", exp.Dedup.After)

	// The mirror is preferred by authority, and still one act.
	acts, held := store.Acts()
	if held != 0 || len(acts) != 2 || acts[0].SourceAuthority != "forge" {
		bfeFail(t, "act-dedup", "acts %d held %d; the forge mirror was not preferred", len(acts), held)
	}

	// The same act id with a different act is a held conflict: excluded from
	// counts, reported in coverage, and the refusal never quotes the id.
	line := actLines(t, "dedup.jsonl")[0]
	conflict, err := ParseFlowRecord(actMutLine(t, line, func(m map[string]any) { m["category"] = "decision" }))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	if _, err := store.Add(conflict.Act); err == nil {
		bfeFail(t, "act-conflict", "a different act under one act id was accepted")
	} else if strings.Contains(err.Error(), "act-0002") {
		bfeFail(t, "act-conflict", "the conflict refusal quoted the act id")
	}
	prof := BuildRestrictedProfile(actInput(store, actBriefA, actBriefB))
	iv := prof["interventions"].(map[string]any)
	if actNum(iv["held"]) != 1 || iv["coverage"] != "partial" || actNum(actFleet(prof)["total"]) != 1 {
		bfeFail(t, "act-conflict", "held %v coverage %v fleet %v; a held conflict must leave counts and mark coverage partial",
			iv["held"], iv["coverage"], actFleet(prof)["total"])
	}
}

// regression: an edge with no completed wait is unknown, never 0 (mutation wait-no-data).
// TestFlowWaits — the five named waits: completed durations in their exit
// window, open waits as aging, retries deduplicated, and an edge with no
// completed wait is unknown, never zero.
func TestFlowWaits(t *testing.T) {
	exp := actLoadExpect(t)
	in := actInput(nil)
	in.Waits = actWaits(t, "waits.jsonl")
	prof := BuildRestrictedProfile(in)
	stats := map[string]map[string]any{}
	for _, x := range actNorm(t, prof["waits"]).([]any) {
		m := x.(map[string]any)
		stats[m["edge"].(string)] = m
	}
	for _, edge := range waitEdges {
		got, want := stats[edge], exp.Waits[edge]
		if got == nil {
			bfeFail(t, "wait-edge", "no statistic for edge %s", edge)
			continue
		}
		delete(got, "held")
		delete(got, "edge")
		id := "wait-stat"
		if want["state"] == "unknown" {
			id = "wait-no-data"
		}
		if !reflect.DeepEqual(got, want) {
			bfeFail(t, id, "edge %s = %v, want %v", edge, got, want)
		}
	}
	// A different wait under one fact id is held: excluded, coverage partial.
	bad := actMutLine(t, actLines(t, "waits.jsonl")[1], func(m map[string]any) {
		m["wait"].(map[string]any)["ended_at"] = "2030-01-02T05:00:00Z"
	})
	r, err := ParseFlowRecord(bad)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	in.Waits = append(in.Waits, r.Wait)
	for _, x := range BuildRestrictedProfile(in)["waits"].([]any) {
		m := x.(map[string]any)
		if m["edge"] == "intake_dispatch" && (actNum(m["n"]) != 2 || actNum(m["held"]) != 1 || m["coverage"] != "partial") {
			bfeFail(t, "wait-held", "conflicting wait: n %v held %v coverage %v, want 2/1/partial", m["n"], m["held"], m["coverage"])
		}
	}
}

// regression: the historian must refuse a human-act line on read (mutation historian-read).
// TestActSinks — Verify row 5: role-only actors agree with the run-record
// contract, a forbidden login is refused without disclosure, and a human-act
// record reaches neither the historian, the board nor a git work tree.
func TestActSinks(t *testing.T) {
	s, err := eventSchema()
	if err != nil {
		t.Fatalf("ASSERT-FAIL[sinks-roles] %v", err)
	}
	doc, err := os.ReadFile(filepath.Join("..", "docs", "streams", "graph-execution", "brief-06-run-records-and-replay.md"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[sinks-roles] could-not-check: %v", err)
	}
	if !bytes.Contains(doc, []byte("`desk | reviewer | verifier | worker | human`")) {
		bfeFail(t, "sinks-roles", "run-record role list not found; the role vocabulary may have drifted")
	}
	props := s.raw["properties"].(map[string]any)
	for _, k := range []string{"initiator_role", "executor_role"} {
		var got []string
		for _, x := range props[k].(map[string]any)["enum"].([]any) {
			got = append(got, x.(string))
		}
		if !reflect.DeepEqual(got, eventRoles) {
			bfeFail(t, "sinks-roles", "%s enum %v, want %v", k, got, eventRoles)
		}
	}
	for _, k := range []string{"login", "user", "actor", "principal", "person"} {
		if _, ok := props[k]; ok {
			bfeFail(t, "sinks-roles", "the schema declares a person member %q", k)
		}
	}

	// A synthetic login, generated now, held in memory only.
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		t.Fatalf("ASSERT-FAIL[sinks-login] could-not-check: %v", err)
	}
	secret := "zq" + hex.EncodeToString(rb)
	act := actLines(t, "contract.jsonl")[0]
	wait := actLines(t, "waits.jsonl")[0]
	cases := []struct {
		line []byte
		mut  func(map[string]any)
	}{
		{act, func(m map[string]any) { m["login"] = secret }},
		{act, func(m map[string]any) { m["initiator_role"] = secret }},
		{act, func(m map[string]any) { m["attribution"].(map[string]any)["approver"] = secret }},
		{act, func(m map[string]any) {
			m["attribution"].(map[string]any)["evidence_ref"] = secret + "@example.invalid"
		}},
		{act, func(m map[string]any) { m["effects"] = []any{map[string]any{"ref": "e-1", "author": secret}} }},
		{wait, func(m map[string]any) { m["reviewer_login"] = secret }},
		{wait, func(m map[string]any) { m["context"].(map[string]any)["harness"] = secret + "@example.invalid" }},
	}
	for i, c := range cases {
		_, err := ParseFlowRecord(actMutLine(t, c.line, c.mut))
		if err == nil {
			bfeFail(t, "sinks-login", "case %d: a person identifier was accepted", i+1)
			continue
		}
		if strings.Contains(err.Error(), secret) {
			bfeFail(t, "sinks-no-disclose", "case %d: the refusal disclosed the refused value", i+1)
		}
	}

	// The historian refuses a human-act line on read and on write; the record
	// pass and backfill fail closed and leave the log byte-identical.
	root := goodRepoRoot(t)
	stubRemoteBranches(t, []string{"main"}, nil)
	logPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	legit := `{"ts":"2030-01-01T00:00:00Z","brief":"alpha/02","from":"","to":"todo","sha":"0000000"}` + "\n"
	planted := append([]byte(legit), append(append([]byte{}, act...), '\n')...)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, planted, 0o644); err != nil {
		t.Fatal(err)
	}
	resetHistoryLoadMemo()
	t.Cleanup(resetHistoryLoadMemo)
	if _, err := LoadHistory(logPath); err == nil {
		bfeFail(t, "sinks-historian-read", "the historian read a human-act line")
	} else if strings.Contains(err.Error(), "act-0001") {
		bfeFail(t, "sinks-no-disclose", "the historian refusal quoted an act value")
	}
	var rc int
	captureStderr(t, func() { rc = recordHistory(root, nil) })
	if rc == 0 {
		bfeFail(t, "sinks-historian-read", "the record pass accepted a log holding a human-act line")
	}
	if err := refuseHumanActBytes(append(append([]byte(legit), act...), '\n')); err == nil {
		bfeFail(t, "sinks-historian-write", "the historian writer check passed a human-act line")
	}
	if err := refuseHumanActBytes([]byte(legit)); err != nil {
		bfeFail(t, "sinks-historian-write", "the historian writer check refused a status row: %v", err)
	}
	if after, _ := os.ReadFile(logPath); !bytes.Equal(after, planted) {
		bfeFail(t, "sinks-historian-read", "the record pass changed the log")
	}

	// The generated board stays free of every actor, role and act field.
	g := installNetGuard(t, true)
	_ = g
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	captureStderr(t, func() { _ = run(root, "write", nil, nil, "") })
	status := readStatus(t, root)
	for _, v := range []string{"act-0001", "act_id", "initiator_role", "executor_role", "human_intervention",
		"src-act-0001", "src-evidence-act-0001", "authenticated_source", "intervention-taxonomy"} {
		if strings.Contains(status, v) {
			bfeFail(t, "sinks-board", "STATUS.md carries a human-act field or value")
		}
	}

	// Backfill refuses the planted log and leaves it untouched.
	bf := initBackfillRepo(t)
	bfLog := filepath.Join(bf, filepath.FromSlash(historyRelPath))
	if err := os.MkdirAll(filepath.Dir(bfLog), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bfLog, planted, 0o644); err != nil {
		t.Fatal(err)
	}
	resetHistoryLoadMemo()
	null, _ := os.Open(os.DevNull)
	defer null.Close()
	if code := runBackfill([]string{"--root", bf}, null, null); code == 0 {
		bfeFail(t, "sinks-backfill", "backfill accepted a log holding a human-act line")
	}
	if after, _ := os.ReadFile(bfLog); !bytes.Equal(after, planted) {
		bfeFail(t, "sinks-backfill", "backfill rewrote a log holding a human-act line")
	}

	// The restricted profile (act ids, roles, source pointers) never lands in
	// a git work tree, and is owner-only where it does land.
	store := actStore(t, "contract.jsonl")
	prof := BuildRestrictedProfile(actInput(store, actBriefA))
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "docs", "restricted")
	if err := WriteRestrictedProfile(target, prof); err == nil {
		bfeFail(t, "sinks-restricted-git", "the restricted profile was written into a git work tree")
	}
	if _, err := os.Stat(target); err == nil {
		bfeFail(t, "sinks-restricted-git", "a refused restricted write created its directory")
	}
	ok := filepath.Join(t.TempDir(), "restricted")
	if err := WriteRestrictedProfile(ok, prof); err != nil {
		t.Fatalf("ASSERT-FAIL[sinks-restricted-mode] %v", err)
	}
	if fi, err := os.Stat(filepath.Join(ok, restrictedFile)); err != nil || fi.Mode().Perm()&0o077 != 0 {
		bfeFail(t, "sinks-restricted-mode", "the restricted profile is readable beyond its owner")
	}

	// The record code has no path to the historian's writer.
	src, err := os.ReadFile("briefintervention.go")
	if err != nil {
		t.Fatalf("ASSERT-FAIL[sinks-no-writer] could-not-check: %v", err)
	}
	for _, call := range []string{"appendHistory(", "recordHistory(", "historyRelPath"} {
		if bytes.Contains(src, []byte(call)) {
			bfeFail(t, "sinks-no-writer", "briefintervention.go references the historian writer (%s)", call)
		}
	}
}

// actFlowProfiles builds the flow fixture's restricted profile and both
// committed variants (with and without a recorded publication policy).
func actFlowProfiles(t *testing.T) (map[string]any, map[string]any, map[string]any, *PublicationPolicy) {
	t.Helper()
	store := NewInterventionStore()
	in := actInput(store, actBriefA, actBriefB)
	for _, r := range actParseAll(t, "flow.jsonl") {
		switch {
		case r.Act != nil:
			if _, err := store.Add(r.Act); err != nil {
				t.Fatalf("ASSERT-FAIL[fixture] %v", err)
			}
		case r.Wait != nil:
			in.Waits = append(in.Waits, r.Wait)
		}
	}
	pol := &PublicationPolicy{Audience: publicationAudience, Revision: "publication-policy-1", AuthorizedSources: []string{"example-console"}}
	in.Policy = pol
	restricted := BuildRestrictedProfile(in)
	in.Policy = nil
	unpolicied := BuildRestrictedProfile(in)
	return restricted, BuildCommittedProfile(restricted, pol), BuildCommittedProfile(unpolicied, nil), pol
}

// actWalk collects every member name and every scalar string value.
func actWalk(v any, keys, vals map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, sub := range x {
			keys[k] = true
			actWalk(sub, keys, vals)
		}
	case []any:
		for _, sub := range x {
			actWalk(sub, keys, vals)
		}
	case string:
		vals[x] = true
	}
}

// regression: nothing outside the allowlist reaches committed bytes (mutation committed-allowlist).
// TestActPublication — Verify row 6: both profiles carry the same numbers,
// nothing outside the allowlist reaches committed bytes, and without a
// recorded publication policy interventions are held while waits render.
func TestActPublication(t *testing.T) {
	exp := actLoadExpect(t)
	restricted, committed, held, pol := actFlowProfiles(t)
	b, err := CommittedBytes(committed)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[pub-allowlist] the committed profile failed its own check: %v", err)
	}
	r, c := actNorm(t, restricted).(map[string]any), actNorm(t, committed).(map[string]any)
	if !reflect.DeepEqual(r["interventions"], c["interventions"]) || !reflect.DeepEqual(r["waits"], c["waits"]) {
		bfeFail(t, "pub-same-numbers", "restricted and committed profiles disagree on counts or wait statistics")
	}
	keys, vals := map[string]bool{}, map[string]bool{}
	actWalk(c, keys, vals)
	for _, k := range exp.ForbiddenKeys {
		if keys[k] {
			bfeFail(t, "pub-allowlist", "committed profile carries forbidden member %q", k)
		}
	}
	for _, v := range exp.ForbiddenValues {
		if bytes.Contains(b, []byte(v)) {
			bfeFail(t, "pub-allowlist", "committed bytes carry a restricted value (%s…)", v)
		}
	}
	for _, v := range exp.ForbiddenScalarValues {
		if vals[v] {
			bfeFail(t, "pub-allowlist", "committed profile carries an actor value %q", v)
		}
	}
	// The restricted side does hold them, so their absence above is the
	// allowlist at work, not an empty fixture.
	rb, _ := json.Marshal(restricted)
	for _, v := range []string{"act-0501", "example-console", "src-act-0501"} {
		if !bytes.Contains(rb, []byte(v)) {
			t.Fatalf("ASSERT-FAIL[fixture] the restricted profile lacks %s; the allowlist check would be vacuous", v)
		}
	}

	// No policy, a policy for another audience, or one that does not name the
	// source: interventions held, waits still render.
	other := *pol
	other.Audience = "operators"
	unnamed := *pol
	unnamed.AuthorizedSources = []string{"another-source"}
	noRev := *pol
	noRev.Revision = ""
	var wantHeld any
	if err := json.Unmarshal(exp.FlowHeld, &wantHeld); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	for i, p := range []map[string]any{held, BuildCommittedProfile(restricted, &other),
		BuildCommittedProfile(restricted, &unnamed), BuildCommittedProfile(restricted, &noRev)} {
		n := actNorm(t, p).(map[string]any)
		if !reflect.DeepEqual(n["interventions"], wantHeld) {
			bfeFail(t, "pub-held", "case %d: interventions %v, want held", i+1, n["interventions"])
		}
		if !reflect.DeepEqual(n["waits"], c["waits"]) {
			bfeFail(t, "pub-held", "case %d: wait statistics did not render while interventions were held", i+1)
		}
		if _, err := CommittedBytes(p); err != nil {
			bfeFail(t, "pub-allowlist", "case %d: held profile failed the allowlist: %v", i+1, err)
		}
	}
	if _, ok := actNorm(t, held).(map[string]any)["policy_revision"]; ok {
		bfeFail(t, "pub-held", "a profile with no policy claims a policy revision")
	}

	// The independent checker refuses a planted member at any depth, and its
	// refusal never names the member.
	for _, plant := range []func(map[string]any){
		func(m map[string]any) { m["act_source"] = "example-console" },
		func(m map[string]any) { m["interventions"].(map[string]any)["receipt_ref"] = "planted-receipt-0501" },
		func(m map[string]any) {
			m["interventions"].(map[string]any)["briefs"].([]any)[0].(map[string]any)["initiator_role"] = "human"
		},
		func(m map[string]any) { m["waits"].([]any)[0].(map[string]any)["start_fact"] = "x" },
		func(m map[string]any) { m["build"] = map[string]any{"manifest": "x"} },
	} {
		m := actNorm(t, committed).(map[string]any)
		plant(m)
		pb, _ := json.Marshal(m)
		if err := CheckCommitted(pb); err == nil {
			bfeFail(t, "pub-checker", "a planted member passed the committed check")
		} else if strings.Contains(err.Error(), "receipt_ref") || strings.Contains(err.Error(), "initiator_role") {
			bfeFail(t, "pub-checker", "the refusal named the planted member")
		}
	}
}

// TestActFlow — Verify row 7: synthetic receipts → validation → restricted
// projection → committed aggregate keep uuid, category, outcome and
// coverage, and nothing outside the allowlist.
func TestActFlow(t *testing.T) {
	exp := actLoadExpect(t)
	_, committed, _, _ := actFlowProfiles(t)
	var want any
	if err := json.Unmarshal(exp.FlowCommitted, &want); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture] %v", err)
	}
	got := actNorm(t, committed)
	if !reflect.DeepEqual(got, want) {
		gb, _ := json.Marshal(got)
		wb, _ := json.Marshal(want)
		bfeFail(t, "flow-end-to-end", "committed profile\n got %s\nwant %s", gb, wb)
	}
	b, err := CommittedBytes(committed)
	if err != nil {
		bfeFail(t, "flow-end-to-end", "committed bytes refused: %v", err)
	}
	again, _ := CommittedBytes(committed)
	if !bytes.Equal(b, again) {
		bfeFail(t, "flow-end-to-end", "committed bytes are not deterministic")
	}
	if !bytes.Contains(b, []byte(actBriefA)) || !bytes.Contains(b, []byte(`"category":"ruling"`)) ||
		!bytes.Contains(b, []byte(`"outcome":"rejected"`)) || !bytes.Contains(b, []byte(`"coverage":"observed"`)) {
		bfeFail(t, "flow-end-to-end", "uuid, category, outcome or coverage was lost")
	}
}
