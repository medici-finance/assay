package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// throughputFixture stands up a board where every stage EXCEPT intake is readable: the fake
// statusgen supplies the dispatch and verify populations across two roots, and the fake gh
// supplies one open PR per watched repo with no review at head.
func throughputFixture(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // isolate the width store from the developer's own
	installFakeStatusgen(t)
	twoRoots(t)
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":1,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},"createdAt":"`+
			time.Now().UTC().Format(time.RFC3339)+`","labels":[],"headRefOid":"abc123",`+
			`"headRefName":"b","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
}

func runThroughput(t *testing.T, args ...string) throughputReport {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(append([]string{"throughput"}, args...), &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("throughput exited %d, stderr=%s", code, errb.String())
	}
	var rep throughputReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("throughput output is not JSON: %v\n%s", err, out.String())
	}
	return rep
}

func stage(t *testing.T, rep throughputReport, name string) stageRow {
	t.Helper()
	for _, s := range rep.Stages {
		if s.Stage == name {
			return s
		}
	}
	t.Fatalf("no %q stage in the report; a stage that is absent reads as a stage with no queue", name)
	return stageRow{}
}

// TestThroughput_ReportsEveryStageOnAFixtureBoard is the happy path: all four stages are
// PRESENT, each names the loop whose width sizes it, and each carries a depth or an explicit
// could-not-check. A stage silently missing from the list would read as a drained queue.
func TestThroughput_ReportsEveryStageOnAFixtureBoard(t *testing.T) {
	throughputFixture(t)
	rep := runThroughput(t)

	if rep.StagesTotal != 4 || len(rep.Stages) != 4 {
		t.Fatalf("report carries %d stages (total declared %d), want 4 present", len(rep.Stages), rep.StagesTotal)
	}
	wantLoop := map[string]string{
		"dispatch": "worker-desk",
		"review":   "pr-review-desk",
		"verify":   "verify-desk",
		"intake":   "intake-desk",
	}
	for name, loop := range wantLoop {
		s := stage(t, rep, name)
		if s.Loop != loop {
			t.Errorf("stage %q names loop %q, want %q — the signal must name the knob it wants moved", name, s.Loop, loop)
		}
		if s.Depth == nil && s.Blind == "" {
			t.Errorf("stage %q has neither a depth nor a stated reason it could not be read", name)
		}
		if s.Depth != nil && s.Ratio == nil {
			t.Errorf("stage %q has a depth but no ratio — a depth with no slots is not comparable", name)
		}
	}
}

// TestThroughput_DepthsComeFromTheExistingDerivations proves the reuse claim rather than
// asserting it in a comment: the depths this verb reports must equal what `dispatch` and
// `awaiting` report on the same fixture. If throughput ever grows its own parser, these
// numbers are what diverge first.
func TestThroughput_DepthsComeFromTheExistingDerivations(t *testing.T) {
	throughputFixture(t)
	rep := runThroughput(t)

	var out, errb bytes.Buffer
	if code := run([]string{"dispatch"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("dispatch exited %d: %s", code, errb.String())
	}
	var drep dispatchReport
	if err := json.Unmarshal(out.Bytes(), &drep); err != nil {
		t.Fatalf("dispatch output: %v", err)
	}
	d := stage(t, rep, "dispatch")
	if d.Depth == nil || *d.Depth != drep.Eligible {
		t.Errorf("dispatch depth = %v, want `deskboard dispatch`'s eligible count %d — the signal must "+
			"DERIVE from that report, never re-parse the board", d.Depth, drep.Eligible)
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"awaiting"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("awaiting exited %d: %s", code, errb.String())
	}
	var nrep nextupReport
	if err := json.Unmarshal(out.Bytes(), &nrep); err != nil {
		t.Fatalf("awaiting output: %v", err)
	}
	want := 0
	for _, r := range nrep.Rows {
		if r.Status == "implemented" {
			want++
		}
	}
	v := stage(t, rep, "verify")
	if v.Depth == nil || *v.Depth != want {
		t.Errorf("verify depth = %v, want the `implemented` rows of `deskboard awaiting` (%d)", v.Depth, want)
	}
}

// TestThroughput_SlotsAreTheResolvedWidthAndMoveWithIt is the link between the signal and the
// knob: after a width is set, the denominator changes on the very next read. Without this,
// "the desk widens a pool and the signal reflects it" is a claim nothing checks.
func TestThroughput_SlotsAreTheResolvedWidthAndMoveWithIt(t *testing.T) {
	throughputFixture(t)

	before := stage(t, runThroughput(t), "review")
	def, _ := deskkit.DefaultWidth("pr-review-desk")
	if before.Slots != def {
		t.Fatalf("review slots = %d before any width is set, want the shipped default %d", before.Slots, def)
	}

	if err := deskkit.SaveWidth(&deskkit.WidthEntry{
		Loop: "pr-review-desk", Width: 8, SetBy: "the-desk",
		Updated: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveWidth: %v", err)
	}

	after := stage(t, runThroughput(t), "review")
	if after.Slots != 8 {
		t.Errorf("review slots = %d after widening to 8, want 8 — the signal must read the width the "+
			"knob wrote, through the same reader the loop uses", after.Slots)
	}
	if !strings.Contains(after.SlotsSource, "the-desk") {
		t.Errorf("slotsSource = %q; it must say WHERE the number came from, since %d means something "+
			"different as a default than as a width somebody set", after.SlotsSource, after.Slots)
	}
}

// TestThroughput_BlindStageIsExcludedFromBottleneck is the three-state rule where it bites.
// A stage whose depth could not be read must be neither the bottleneck nor a zero: counting
// it as zero would present an unreadable queue as a drained one and steer the desk to widen
// the wrong stage.
func TestThroughput_BlindStageIsExcludedFromBottleneck(t *testing.T) {
	throughputFixture(t)
	rep := runThroughput(t)

	in := stage(t, rep, "intake")
	if in.Depth != nil {
		t.Fatalf("fixture error: intake should be unread here, got depth %d", *in.Depth)
	}
	if in.Blind == "" {
		t.Error("an unread stage must SAY it was unread")
	}
	if in.Ratio != nil {
		t.Error("an unread stage must have no ratio — it is not comparable, so it cannot be ranked")
	}
	if rep.Bottleneck == "intake" {
		t.Error("a stage nothing measured was named the bottleneck")
	}
	if rep.StagesRead >= rep.StagesTotal {
		t.Errorf("stagesRead=%d of %d, but intake was not read — the coverage count must be honest",
			rep.StagesRead, rep.StagesTotal)
	}
	if !strings.Contains(rep.Advice, rep.Bottleneck) && rep.Bottleneck != "" {
		t.Errorf("advice %q does not name the bottleneck %q", rep.Advice, rep.Bottleneck)
	}
}

// TestThroughput_AdviceIsAnExactCommandOrSaysWhyNot: the coordinator acts on ONE line, so it
// must either carry a command that would be accepted, or state plainly that widening is not
// the lever. Advice naming a width the bound would refuse is worse than no advice.
func TestThroughput_AdviceIsAnExactCommandOrSaysWhyNot(t *testing.T) {
	throughputFixture(t)
	rep := runThroughput(t)

	if rep.Bottleneck == "" {
		if !strings.Contains(rep.Advice, "no bottleneck") && !strings.Contains(rep.Advice, "COULD-NOT-CHECK") {
			t.Fatalf("no bottleneck, but the advice does not say so: %q", rep.Advice)
		}
		return
	}
	s := stage(t, rep, rep.Bottleneck)
	if s.Slots >= s.MaxSlots {
		if !strings.Contains(rep.Advice, "ALREADY AT") {
			t.Errorf("the bottleneck is at its ceiling but the advice does not say so: %q", rep.Advice)
		}
		return
	}
	if !strings.Contains(rep.Advice, "deskroster set --role "+s.Loop+" --width ") {
		t.Fatalf("advice must carry the exact widening command; got %q", rep.Advice)
	}
	// The width the advice recommends must actually be accepted by the bound — advice the
	// knob would refuse is a loop the coordinator cannot close.
	if err := deskkit.CheckWidth(s.Loop, s.MaxSlots); err != nil {
		t.Errorf("advice recommends width %d for %s, which the bound REFUSES: %v", s.MaxSlots, s.Loop, err)
	}
}

// TestThroughput_FlagAliasMatchesTheSubcommand: the verb was specified as `--throughput`, and
// deskboard's modes are subcommands. Both spellings must reach the same report — a flag that
// silently did nothing would look like a board with no bottleneck.
func TestThroughput_FlagAliasMatchesTheSubcommand(t *testing.T) {
	throughputFixture(t)

	var a, b, ea, eb bytes.Buffer
	if code := run([]string{"throughput"}, &a, &ea); code != deskkit.ExitOK {
		t.Fatalf("subcommand exited %d: %s", code, ea.String())
	}
	if code := run([]string{"--throughput"}, &b, &eb); code != deskkit.ExitOK {
		t.Fatalf("--throughput exited %d: %s", code, eb.String())
	}
	var ra, rb throughputReport
	if err := json.Unmarshal(a.Bytes(), &ra); err != nil {
		t.Fatalf("subcommand output: %v", err)
	}
	if err := json.Unmarshal(b.Bytes(), &rb); err != nil {
		t.Fatalf("--throughput output: %v", err)
	}
	if len(ra.Stages) != len(rb.Stages) || ra.Bottleneck != rb.Bottleneck {
		t.Errorf("the flag alias and the subcommand disagree: %d stages/%q vs %d stages/%q",
			len(ra.Stages), ra.Bottleneck, len(rb.Stages), rb.Bottleneck)
	}
}

// TestThroughput_StatesBothCoverageAxes: this verb sweeps the repo set (review depth) AND the
// configured roots (dispatch/verify depths), so it must state both. Omitting either would
// claim a coverage it does not have — the #359 rule, applied to a verb that spans two axes.
func TestThroughput_StatesBothCoverageAxes(t *testing.T) {
	throughputFixture(t)
	var out, errb bytes.Buffer
	if code := run([]string{"throughput"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("throughput exited %d: %s", code, errb.String())
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("throughput output: %v", err)
	}
	sc, ok := m["scope"].(map[string]any)
	if !ok {
		t.Fatal("throughput sweeps the repo set and must carry `scope`")
	}
	if int(sc["count"].(float64)) != len(deskkit.AllowedRepos()) {
		t.Errorf("scope count %v disagrees with the set the review sweep iterates", sc["count"])
	}
	roots, ok := m["roots"].([]any)
	if !ok || len(roots) == 0 {
		t.Errorf("throughput reads statusgen ROOTS for two of its four depths and must name them; got %v", m["roots"])
	}
}
