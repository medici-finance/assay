package main

// dispatch_test.go — the dispatch split and the subagent spend block, and the
// round trip into the autonomy ladder that reads them.
//
// The fixture tree testdata/dispatch holds one session (session-gamma) laid out the
// way the harness lays out a session with subagents:
//
//	proj-gamma/session-gamma.jsonl                    the main transcript
//	proj-gamma/session-gamma/subagents/agent-g*.jsonl one file per subagent run
//
// plus one run (g5) whose sidechain records sit inline in the main transcript.
//
// The expected numbers, by hand from the fixture:
//
//	dispatch   9 on the day (the previous-day dispatch is excluded):
//	           operator 2 (turnOrigin human)
//	           loop     4 (scheduled ×2 in one message, task-notification via origin.kind, peer)
//	           unattributed 3 (unstamped typed prompt ×2 incl. a meta record inside it, sdk)
//	runs       4 on the day: g1, g2, g4 (subagents/), g5 (inline); g3 started the day before
//	tokens     g1 1150 (two streamed copies of one message, counted once) + 2060
//	           + g2 20 + g4 10 + g5 15 = 3255
//	no-op      2: g2 (handback opens "Already merged … nothing to do"), g5 (last text
//	           opens "No changes needed"). g1's handback says "already merged" only in
//	           its SECOND paragraph, not at its opening, and is not a no-op.
//	unread     1: g4 never reported

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fxDispatch = "testdata/dispatch"
	// roundTripGolden is the day-file the statusgen side reads in its round-trip
	// test (statusgen/opmetricsroundtrip_test.go). Both modules pin the SAME file.
	roundTripGolden = "../../../../statusgen/testdata/opmetrics-roundtrip/docs/reports/daily/2026-07-22/opmetrics.json"
)

// ladderView is the consumer's view of the day-file, declared exactly as statusgen's
// opDayFile declares it (autonomy.go). Decoding the EMITTED bytes through it is how
// this module proves it writes the keys the ladder reads — not keys of a similar name.
type ladderView struct {
	Schema   string `json:"schema"`
	Dispatch *struct {
		LoopInitiated     *int `json:"loop_initiated"`
		OperatorInitiated *int `json:"operator_initiated"`
	} `json:"dispatch"`
	TokenEfficiency *struct {
		SubagentTokens  *int `json:"subagent_tokens"`
		MergedPRs       *int `json:"merged_prs"`
		NoopDispatches  *int `json:"noop_dispatches"`
		TotalDispatches *int `json:"total_dispatches"`
	} `json:"token_efficiency"`
}

func emitDispatchFixture(t *testing.T, extra ...string) []byte {
	t.Helper()
	isolateHome(t)
	args := append([]string{
		"--transcripts", fxDispatch,
		"--desk-tools", fxDeskTools,
		"--gh-fixture", fxGH,
		"--date", "2026-07-22",
		"--now", "2026-07-22T15:00:00Z",
		"--tz", "UTC",
		"--stdout",
	}, extra...)
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("emit exited %d: %s", code, stderr.String())
	}
	return stdout.Bytes()
}

func intAt(t *testing.T, name string, p *int, want int) {
	t.Helper()
	if p == nil {
		t.Fatalf("%s is null, want %d", name, want)
	}
	if *p != want {
		t.Fatalf("%s = %d, want %d", name, *p, want)
	}
}

// TestDayFileCarriesLadderBlocks is the issue's defect stated as a test: the emitted
// day-file must carry the dispatch split and the token block under the keys the
// ladder reads, with the values the fixture implies.
// regression: #5470
func TestDayFileCarriesLadderBlocks(t *testing.T) {
	raw := emitDispatchFixture(t)
	var v ladderView
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("day-file is not JSON: %v", err)
	}
	if v.Dispatch == nil {
		t.Fatal("day-file has no dispatch block — the ladder's rung 3 reads dayfile-no-dispatch-split")
	}
	intAt(t, "dispatch.loop_initiated", v.Dispatch.LoopInitiated, 4)
	intAt(t, "dispatch.operator_initiated", v.Dispatch.OperatorInitiated, 2)
	if v.TokenEfficiency == nil {
		t.Fatal("day-file has no token_efficiency block — the ladder's rung 4 reads dayfile-no-token-block")
	}
	te := v.TokenEfficiency
	intAt(t, "token_efficiency.subagent_tokens", te.SubagentTokens, 3255)
	intAt(t, "token_efficiency.merged_prs", te.MergedPRs, 5)
	intAt(t, "token_efficiency.noop_dispatches", te.NoopDispatches, 2)
	intAt(t, "token_efficiency.total_dispatches", te.TotalDispatches, 4)
}

func TestDispatchBlockCountsEveryOriginBucket(t *testing.T) {
	var stderr bytes.Buffer
	in := fixtureInputs(t.TempDir(), 0)
	in.TranscriptsDirs = []string{fxDispatch}
	rep := Build(in, &stderr)
	d := rep.Dispatch
	if d.Status != StatusOK || d.Reason != ReasonNone {
		t.Fatalf("dispatch block = %s/%s, want ok", d.Status, d.Reason)
	}
	intAt(t, "total_dispatches", d.TotalDispatches, 9)
	intAt(t, "unattributed", d.Unattributed, 3)
	if *d.LoopInitiated+*d.OperatorInitiated+*d.Unattributed != *d.TotalDispatches {
		t.Fatal("loop + operator + unattributed must equal total_dispatches")
	}
	te := rep.TokenEfficiency
	intAt(t, "reports_unread", te.ReportsUnread, 1)
	if te.NoopClassifier != NoopClassifierVersion {
		t.Fatalf("noop_classifier = %q, want %q", te.NoopClassifier, NoopClassifierVersion)
	}
}

// TestDispatchBlocksBlindWhenTranscriptsBlind: an unreadable transcript root is
// could-not-check in BOTH new blocks with every count null — never a zero, which the
// ladder would read as "measured: 0 dispatches".
// regression: #5470
func TestDispatchBlocksBlindWhenTranscriptsBlind(t *testing.T) {
	var stderr bytes.Buffer
	in := fixtureInputs(t.TempDir(), 0)
	in.TranscriptsDirs = []string{filepath.Join(t.TempDir(), "absent")}
	rep := Build(in, &stderr)
	d, te := rep.Dispatch, rep.TokenEfficiency
	if d.Status != StatusCouldNotCheck || d.Reason != ReasonTranscriptsUnread {
		t.Fatalf("dispatch = %s/%s, want could-not-check/%s", d.Status, d.Reason, ReasonTranscriptsUnread)
	}
	if d.TotalDispatches != nil || d.LoopInitiated != nil || d.OperatorInitiated != nil || d.Unattributed != nil {
		t.Fatalf("a blind dispatch block carried a count: %+v", d)
	}
	if te.Status != StatusCouldNotCheck || te.Reason != ReasonTranscriptsUnread {
		t.Fatalf("token_efficiency = %s/%s, want could-not-check/%s", te.Status, te.Reason, ReasonTranscriptsUnread)
	}
	if te.SubagentTokens != nil || te.MergedPRs != nil || te.NoopDispatches != nil || te.TotalDispatches != nil || te.ReportsUnread != nil {
		t.Fatalf("a blind token block carried a count: %+v", te)
	}
}

// TestDispatchSplitBlindWhenNoOriginRecorded: dispatches with no origin stamp at all
// are could-not-check with both sides null, not a 0/0 split.
// regression: #5470
func TestDispatchSplitBlindWhenNoOriginRecorded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","timestamp":"2026-07-22T08:00:00Z","sessionId":"s-none","userType":"external","message":{"role":"user","content":"an unstamped prompt"}}
{"type":"assistant","timestamp":"2026-07-22T08:01:00Z","sessionId":"s-none","message":{"id":"a1","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{}}]}}
`
	if err := os.WriteFile(filepath.Join(dir, "s-none.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	in := fixtureInputs(t.TempDir(), 0)
	in.TranscriptsDirs = []string{filepath.Dir(dir)}
	d := Build(in, &stderr).Dispatch
	if d.Status != StatusCouldNotCheck || d.Reason != ReasonDispatchOriginUnrecorded {
		t.Fatalf("dispatch = %s/%s, want could-not-check/%s", d.Status, d.Reason, ReasonDispatchOriginUnrecorded)
	}
	if d.LoopInitiated != nil || d.OperatorInitiated != nil {
		t.Fatalf("an origin-blind split carried a side: %+v", d)
	}
	intAt(t, "total_dispatches", d.TotalDispatches, 1)
	intAt(t, "unattributed", d.Unattributed, 1)
}

// TestOriginlessMetaRecordKeepsTurnOrigin: injected context inside a human turn (a
// meta prompt record with no origin stamp) leaves the turn's origin alone, so a
// dispatch after it is still operator-initiated (N2 of the round-1 review on #5478).
func TestOriginlessMetaRecordKeepsTurnOrigin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","timestamp":"2026-07-22T08:00:00Z","sessionId":"s-meta","userType":"external","turnOrigin":"human","message":{"role":"user","content":"a human prompt"}}
{"type":"user","timestamp":"2026-07-22T08:00:01Z","sessionId":"s-meta","userType":"external","isMeta":true,"message":{"role":"user","content":"injected context"}}
{"type":"assistant","timestamp":"2026-07-22T08:01:00Z","sessionId":"s-meta","message":{"id":"a1","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{}}]}}
`
	if err := os.WriteFile(filepath.Join(dir, "s-meta.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	in := fixtureInputs(t.TempDir(), 0)
	in.TranscriptsDirs = []string{filepath.Dir(dir)}
	d := Build(in, &stderr).Dispatch
	if d.Status != StatusOK {
		t.Fatalf("dispatch = %s/%s, want ok", d.Status, d.Reason)
	}
	intAt(t, "total_dispatches", d.TotalDispatches, 1)
	intAt(t, "operator_initiated", d.OperatorInitiated, 1)
	intAt(t, "unattributed", d.Unattributed, 0)
}

// TestTokenBlockMergedPRsNullWithoutGH: no gh export means merged_prs is null with
// the gh reason, while the transcript-derived counts still stand.
func TestTokenBlockMergedPRsNullWithoutGH(t *testing.T) {
	var stderr bytes.Buffer
	in := fixtureInputs(t.TempDir(), 0)
	in.TranscriptsDirs = []string{fxDispatch}
	in.GHJSON = ""
	te := Build(in, &stderr).TokenEfficiency
	if te.MergedPRs != nil {
		t.Fatalf("merged_prs = %d without a gh export, want null", *te.MergedPRs)
	}
	if te.Status != StatusOK || te.Reason != ReasonGHNotSupplied {
		t.Fatalf("token_efficiency = %s/%s, want ok/%s", te.Status, te.Reason, ReasonGHNotSupplied)
	}
	intAt(t, "total_dispatches", te.TotalDispatches, 4)
}

// TestMultiRootSubagentDirDoesNotHideParent is the dedup defect this change had to
// fix first: a subagent transcript carries its parent's sessionId, the walk reaches
// <session>/subagents/ before <session>.jsonl, and the parent — with every operator
// turn and dispatch in it — was skipped as a duplicate.
// regression: #5470
func TestMultiRootSubagentDirDoesNotHideParent(t *testing.T) {
	out, _, _, err := ReadOperatorMessagesMulti([]string{fxDispatch}, fixtureDay())
	if err != nil {
		t.Fatalf("ReadOperatorMessagesMulti: %v", err)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("operator turns = %d, want 3 — the parent transcript was dropped as a duplicate of its own subagent files", len(out.Messages))
	}
	if out.Files != 5 {
		t.Fatalf("files read = %d, want 5 (the parent and its four subagent files)", out.Files)
	}
	// A subagent is not a session: only the parent contributes a span.
	if len(out.Spans) != 1 {
		t.Fatalf("session spans = %d, want 1", len(out.Spans))
	}
}

func TestMapOriginTable(t *testing.T) {
	cases := []struct {
		turn, kind string
		want       dispatchOrigin
	}{
		{"human", "", originOperator},
		{"", "human", originOperator},
		{"scheduled", "", originLoop},
		{"peer", "peer", originLoop},
		{"task_notification", "", originLoop},
		{"", "task-notification", originLoop},
		{"auto_continuation", "", originLoop},
		{"", "auto-continuation", originLoop},
		{"sdk", "", originUnattributed},
		{"", "", originUnattributed},
		{"something-new", "", originUnattributed},
		// turnOrigin wins over origin.kind when both are present.
		{"human", "peer", originOperator},
	}
	for _, c := range cases {
		if got := mapOrigin(c.turn, c.kind); got != c.want {
			t.Errorf("mapOrigin(%q, %q) = %d, want %d", c.turn, c.kind, got, c.want)
		}
	}
}

func TestNoopReportClassifier(t *testing.T) {
	cases := []struct {
		report string
		want   bool
	}{
		{"Already merged on main; nothing to do.", true},
		{"Nothing to do — the fix is present.", true},
		{"No changes needed: the check already passes.", true},
		{"No change is required here.", true},
		{"This dispatch was a no-op.", true},
		{"The work is already implemented upstream.", true},
		{"Implemented the widget and opened a draft.", false},
		{"Implemented the widget.\n\nThe sibling change was already merged.", false},
		{strings.Repeat("x", 400) + " already merged", false},
		{"", false},

		// A report that opens with a heading, a bold label or a rule: the classifier
		// reads the first line AFTER them (B1 of the round-1 review on #5478).
		{"## Summary\n\nAlready merged on main; nothing to do.", true},
		{"**Result**\n\nNothing to do.", true},
		{"__Result:__\n\nNothing to do.", true},
		{"# Report\n---\n\nNo changes needed.", true},
		{"***\nNo-op: the fix is already on main.", true},
		{"**Result:** Nothing to do; the check passes.", true},
		{"**Nothing to do.** The fix is on main.", true},
		{"**Nothing to do.**", true},
		{"- Already merged on main.", true},
		{"## Summary", false},
		{"## Summary\n\nImplemented the widget.\n\nNothing to do on the CLI.", false},
		{"**Result**\n\nImplemented the widget; the helper already exists so I reused it.", false},

		// A phrase that appears mid-paragraph is not the report opening with it.
		{"Implemented the retry loop and opened a draft PR; the helper already exists so I reused it.", false},
		{"Made the Windows guard a no-op and added tests; PR opened.", false},
		{"Fixed the parser. No changes needed to the CLI flags.", false},
		{"**Result:** Fixed the parser; nothing to do on the CLI.", false},
	}
	for _, c := range cases {
		if got := isNoopReport(c.report); got != c.want {
			t.Errorf("isNoopReport(%q) = %v, want %v", c.report, got, c.want)
		}
	}
}

// TestRoundTripGoldenMatchesProducer pins the producer half of the round trip: the
// day-file statusgen's ladder test reads is byte-identical to what this producer
// emits over the dispatch fixture. Regenerate with OPMETRICS_UPDATE_GOLDEN=1.
// regression: #5470
func TestRoundTripGoldenMatchesProducer(t *testing.T) {
	raw := emitDispatchFixture(t)
	if os.Getenv("OPMETRICS_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(roundTripGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(roundTripGolden, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(roundTripGolden)
	if err != nil {
		t.Fatalf("reading the round-trip golden %s: %v", roundTripGolden, err)
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("the producer's output drifted from the day-file statusgen's ladder test reads.\n"+
			"Regenerate with OPMETRICS_UPDATE_GOLDEN=1 and re-run statusgen's round-trip test.\ngot:\n%s", raw)
	}
}
