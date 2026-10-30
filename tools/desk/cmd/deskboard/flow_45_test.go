package main

// flow_45_test.go — the deskboard half of the verification-queue contract (brief 45).
//
// The classification is statusgen's (statusgen/awaiting_bucket.go queueFor); this test pins
// that deskboard CONSUMES it rather than re-deriving it. The fixture is the one the
// statusgen and verifyloop tests share, statusgen/testdata/flowqueue/gate-scores.golden.json,
// so the three consumers are checked against ONE set of rows covering every class.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const flowGolden = "../../../../statusgen/testdata/flowqueue/gate-scores.golden.json"

// flowRows loads the shared golden and completes each projected row into the full
// --gate-scores shape (stream and a descending synthetic score).
func flowRows(t *testing.T) []gateScoreRow {
	t.Helper()
	b, err := os.ReadFile(flowGolden)
	if err != nil {
		t.Fatalf("shared queue fixture: %v", err)
	}
	var rows []gateScoreRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("shared queue fixture is not JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("shared queue fixture is empty")
	}
	for i := range rows {
		rows[i].Stream, _, _ = strings.Cut(rows[i].Brief, "/")
		rows[i].Score = 1000 - i
	}
	return rows
}

// flowShim serves each root's rows.json, or fails if the root holds a FAIL file.
const flowShim = `#!/bin/sh
if [ "$1" = "--version" ]; then echo "statusgen/v0.1.0"; exit 0; fi
root=""
while [ $# -gt 0 ]; do
  [ "$1" = "--root" ] && { shift; root="$1"; }
  shift
done
if [ -f "$root/FAIL" ]; then echo "statusgen: simulated failure" >&2; exit 1; fi
cat "$root/rows.json"
`

func installFlowShim(t *testing.T) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "statusgen")
	if err := os.WriteFile(bin, []byte(flowShim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(statusgenBinEnv, bin)
}

func putRows(t *testing.T, root string, rows []gateScoreRow) {
	t.Helper()
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rows.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func stripQueue(rows []gateScoreRow) []gateScoreRow {
	out := make([]gateScoreRow, len(rows))
	for i, r := range rows {
		out[i] = gateScoreRow{Brief: r.Brief, Score: r.Score, Stream: r.Stream, Status: r.Status}
	}
	return out
}

func flowReport(t *testing.T, tolerant bool) (*nextupReport, error) {
	t.Helper()
	rs, err := resolveRootsOnce()
	if err != nil {
		t.Fatalf("resolveRootsOnce: %v", err)
	}
	return awaitingQueue(Header{}, "awaiting", rs, tolerant)
}

// regression: F-queue-contract
func TestFlowQueue(t *testing.T) {
	t.Run("unsupported vocabulary is coverage gap", func(t *testing.T) {
		row := gateScoreRow{Brief: "second/01", Status: "implemented", QueueVersion: queueContractVersion, QueueClass: "future-class", QueueValid: true}
		coverage := coverageOf(deskkit.RootConfig{Repo: "example-org/second"}, []gateScoreRow{row}, nil)
		if rowSupported(row) || coverage.State != rootUnsupported || coverage.Unknown != 1 {
			t.Fatal("future class counted as healthy", coverage)
		}
	})
	installFlowShim(t)
	golden := flowRows(t)
	wantClass := map[string]string{}
	for _, r := range golden {
		wantClass[r.Brief] = r.QueueClass
	}

	t.Run("classes", func(t *testing.T) {
		root := makeRoot(t, "toolkit", true)
		putRows(t, root, golden)
		t.Setenv(deskkit.RootsEnv, "medici-finance/assay="+root)
		rep, err := flowReport(t, false)
		if err != nil {
			t.Fatal(err)
		}
		// The shared fixture holds two rows statusgen could not classify: that is a
		// degraded reading with those rows counted as unknown, never a clean one.
		if c := rep.QueueCoverage[0]; c.State != rootDegraded || c.Unknown != 2 || rep.QueueGaps != 1 {
			t.Fatalf("coverage = %+v, want degraded with 2 unknown rows", rep.QueueCoverage)
		}
		if len(rep.Rows) != len(golden) {
			t.Fatalf("rows = %d, want %d", len(rep.Rows), len(golden))
		}
		for _, r := range rep.Rows {
			want := wantClass[r.Brief]
			if !queueValidFor(golden, r.Brief) {
				want = queueClassUnreadable
			}
			if r.Class != want {
				t.Errorf("%s class = %q, want %q", r.Brief, r.Class, want)
			}
		}
		// The rendered board names the class column and the class counts.
		var sb strings.Builder
		renderAwaiting(Header{}, *rep).render(&sb)
		for _, w := range []string{"CLASS", "queue classes:", queueClassJudgment, queueClassHuman} {
			if !strings.Contains(sb.String(), w) {
				t.Errorf("awaiting board does not show %q:\n%s", w, sb.String())
			}
		}
	})

	t.Run("depth counts only judgment", func(t *testing.T) {
		readable := withoutUnreadable(golden)
		root := makeRoot(t, "toolkit", true)
		putRows(t, root, readable)
		t.Setenv(deskkit.RootsEnv, "medici-finance/assay="+root)
		rep, err := flowReport(t, true)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		for _, r := range readable {
			if r.Status == "implemented" && r.QueueClass == queueClassJudgment {
				want++
			}
		}
		depth, partial := verifyDepth(rep)
		if want == 0 || depth != want || partial != "" {
			t.Fatalf("depth = %d (partial %q), want %d complete; runner, continuation, human, "+
				"paused and parked rows are not verifier work", depth, partial, want)
		}
		if all := len(readable); depth >= all {
			t.Fatalf("depth %d equals the whole queue %d: non-actionable rows were counted", depth, all)
		}

		// With the unclassifiable rows back, the same depth is a flagged lower bound.
		putRows(t, root, golden)
		rep, err = flowReport(t, true)
		if err != nil {
			t.Fatal(err)
		}
		if d, partial := verifyDepth(rep); d != want || partial == "" {
			t.Fatalf("depth = %d partial %q, want %d flagged partial", d, partial, want)
		}
	})

	t.Run("old statusgen is a gap not zero", func(t *testing.T) {
		root := makeRoot(t, "toolkit", true)
		putRows(t, root, stripQueue(golden))
		t.Setenv(deskkit.RootsEnv, "medici-finance/assay="+root)
		rep, err := flowReport(t, true)
		if err != nil {
			t.Fatal(err)
		}
		if rep.QueueCoverage[0].State != rootUnsupported || rep.QueueGaps != 1 {
			t.Fatalf("coverage = %+v, want unsupported", rep.QueueCoverage)
		}
		depth, partial := verifyDepth(rep)
		if depth != 0 || partial == "" {
			t.Fatalf("unclassified rows must be a flagged lower bound, got depth %d partial %q", depth, partial)
		}
		for _, r := range rep.Rows {
			if r.Class != queueClassUnclassified {
				t.Errorf("%s class = %q, want unclassified", r.Brief, r.Class)
			}
		}
	})

	t.Run("mixed roots keep partial coverage", func(t *testing.T) {
		good := makeRoot(t, "toolkit", false)
		old := makeRoot(t, "tracker", true)
		dead := makeRoot(t, "agents", false)
		putRows(t, good, withoutUnreadable(golden))
		putRows(t, old, stripQueue(golden))
		if err := os.WriteFile(filepath.Join(dead, "FAIL"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(deskkit.RootsEnv, "medici-finance/assay="+good+",example-org/tracker="+old+
			",example-org/agents="+dead)

		// The `awaiting` verb stays fail-closed on an unreadable root.
		if _, err := flowReport(t, false); err == nil {
			t.Fatal("awaiting must fail closed when a root cannot be read")
		}
		rep, err := flowReport(t, true)
		if err != nil {
			t.Fatal(err)
		}
		states := map[string]string{}
		for _, c := range rep.QueueCoverage {
			states[c.Repo] = c.State
		}
		if states["medici-finance/assay"] != rootHealthy ||
			states["example-org/tracker"] != rootUnsupported ||
			states["example-org/agents"] != rootUnreadable {
			t.Fatalf("coverage states = %v", states)
		}
		depth, partial := verifyDepth(rep)
		want, _ := verifyDepth(&nextupReport{Rows: classified(golden)})
		if depth != want || partial == "" {
			t.Fatalf("depth = %d partial %q: the healthy root's reading (%d) must survive beside the gaps",
				depth, partial, want)
		}
		if rep.QueueGaps != 2 {
			t.Errorf("QueueGaps = %d, want 2", rep.QueueGaps)
		}
	})

	t.Run("a gap suppresses widening", func(t *testing.T) {
		d := 9
		rep := &throughputReport{
			Stages: []stageRow{{
				Stage: "verify", Loop: "verify-desk", Depth: &d, Slots: 1, MaxSlots: 4,
				Ratio: f64p(9), Partial: "assay unreadable: boom",
			}},
			Bottleneck: "verify", StagesRead: 1, StagesTotal: 1,
		}
		adv := throughputAdvice(rep)
		if !strings.Contains(adv, "COVERAGE-GAP") || strings.Contains(adv, "deskroster set") {
			t.Fatalf("advice on a partial depth must name the gap and not widen: %q", adv)
		}
		rep.Stages[0].Partial = ""
		if adv := throughputAdvice(rep); !strings.Contains(adv, "deskroster set") {
			t.Fatalf("a complete depth may widen: %q", adv)
		}
		// And an empty-looking board with a gap is not "no bottleneck".
		z := 0
		rep2 := &throughputReport{
			Stages:     []stageRow{{Stage: "verify", Depth: &z, Partial: "assay unreadable: boom"}},
			StagesRead: 1, StagesTotal: 1,
		}
		if adv := throughputAdvice(rep2); strings.Contains(adv, "no bottleneck") {
			t.Fatalf("an unreadable root must not read as an empty queue: %q", adv)
		}
	})

	t.Run("the bottleneck is withheld on a partial depth", bottleneckWithheldOnPartialDepth)
}

// A partial verify depth is a lower bound, so the JSON bottleneck field must not name a
// stage computed from it (F3): withheld and marked, never published beside advice that
// says COVERAGE-GAP.
func bottleneckWithheldOnPartialDepth(t *testing.T) {
	d, r := 9, 2
	rep := &throughputReport{Stages: []stageRow{
		{Stage: "dispatch", Loop: "worker-desk", Depth: &r, Slots: 2, MaxSlots: 4, Ratio: f64p(1)},
		{Stage: "verify", Loop: "verify-desk", Depth: &d, Slots: 1, MaxSlots: 4, Ratio: f64p(9),
			Partial: "assay unreadable: boom"},
	}}
	pickBottleneck(rep)
	if rep.Bottleneck != "" {
		t.Fatalf("bottleneck %q published from a partial verify depth", rep.Bottleneck)
	}
	if rep.BottleneckWithheld == "" || !strings.Contains(rep.BottleneckWithheld, "verify") {
		t.Fatalf("withheld reason must name the partial stage, got %q", rep.BottleneckWithheld)
	}
	if rep.StagesRead != 2 {
		t.Errorf("StagesRead = %d, want 2 (partial stages are still read)", rep.StagesRead)
	}
	if adv := throughputAdvice(rep); !strings.Contains(adv, "COVERAGE-GAP") {
		t.Errorf("advice = %q", adv)
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"bottleneck":""`) || !strings.Contains(string(b), `"bottleneckWithheld"`) {
		t.Errorf("JSON must carry an empty bottleneck and the withheld marker: %s", b)
	}

	// Control: with no partial stage the bottleneck is still named and nothing is withheld.
	rep.Stages[1].Partial = ""
	rep.Bottleneck, rep.BottleneckWithheld, rep.StagesRead = "", "", 0
	pickBottleneck(rep)
	if rep.Bottleneck != "verify" || rep.BottleneckWithheld != "" {
		t.Errorf("complete depth: bottleneck %q withheld %q, want verify and none", rep.Bottleneck, rep.BottleneckWithheld)
	}
}

func withoutUnreadable(rows []gateScoreRow) []gateScoreRow {
	var out []gateScoreRow
	for _, r := range rows {
		if r.QueueValid {
			out = append(out, r)
		}
	}
	return out
}

func queueValidFor(rows []gateScoreRow, brief string) bool {
	for _, r := range rows {
		if r.Brief == brief {
			return r.QueueValid
		}
	}
	return false
}

// classified turns golden rows into nextupRows the way awaitingQueue does.
func classified(rows []gateScoreRow) []nextupRow {
	out := make([]nextupRow, len(rows))
	for i, r := range rows {
		out[i] = nextupRow{Brief: r.Brief, Status: r.Status, Class: classOf(r)}
	}
	return out
}
