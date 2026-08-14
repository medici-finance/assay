package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// strptr returns a pointer to s — the present/absent shape of the optional
// `measures:` field (nil = the field was never written).
func strptr(s string) *string { return &s }

// measuresBoard builds a two-stream board:
//
//   - "metrics" — the instrumentation stream. Brief 01 declares `measures:`
//     (queue name given by measures); brief 02 declares nothing and is the
//     default-unchanged control.
//   - "queue"  — carries the verification debt that decides whether the
//     measured queue is over its alarm threshold.
//
// breached=true builds a board whose desk-actionable Awaiting queue trips the
// mm/10 alarm (3 implemented vs 0 done → depth > done); breached=false builds
// one comfortably under it (1 implemented vs 5 done).
func measuresBoard(measures *string, breached bool) []*Stream {
	metrics := mkStream("metrics", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "brief-v1", Measures: measures},
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "brief-v1"},
	)
	metrics.LastTouch = day(0)

	var debt []Brief
	if breached {
		debt = []Brief{
			{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1"},
			{Num: "02", Wave: 0, Status: "implemented", Schema: "brief-v1"},
			{Num: "03", Wave: 0, Status: "implemented", Schema: "brief-v1"},
		}
	} else {
		debt = []Brief{
			{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1"},
			{Num: "02", Wave: 0, Status: "done", Schema: "brief-v1"},
			{Num: "03", Wave: 0, Status: "done", Schema: "brief-v1"},
			{Num: "04", Wave: 0, Status: "done", Schema: "brief-v1"},
			{Num: "05", Wave: 0, Status: "done", Schema: "brief-v1"},
			{Num: "06", Wave: 0, Status: "done", Schema: "brief-v1"},
		}
	}
	queue := mkStream("queue", "active", "P1", debt...)
	queue.LastTouch = day(0)
	return []*Stream{metrics, queue}
}

// picked reports whether "<stream>/<num>" appears in the Next-up picks.
func picked(nu NextUp, id string) bool {
	for _, p := range nu.Picks {
		if p.Stream.Name+"/"+p.Brief.Num == id {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fixture sanity: the positive control must actually flip the alarm
// ---------------------------------------------------------------------------

// TestMeasuresFixtureBreachIsReal pins the fixture itself: the "breached" board
// really does trip the mm/10 verification-debt alarm and the "under" board
// really does not. Without this, an exclusion test could pass for the wrong
// reason (e.g. neither board ever breaches and everything is excluded by some
// unrelated rule).
func TestMeasuresFixtureBreachIsReal(t *testing.T) {
	if !debtBreached(measuresBoard(nil, true)) {
		t.Error("breached fixture does not trip the verification-debt alarm")
	}
	if debtBreached(measuresBoard(nil, false)) {
		t.Error("under-threshold fixture trips the verification-debt alarm")
	}
	// The gate and the mm/10 NOTICE must never disagree about what "over
	// threshold" means — they read the SAME predicate.
	for _, breached := range []bool{true, false} {
		streams := measuresBoard(nil, breached)
		if got := debtNotice(streams) != ""; got != debtBreached(streams) {
			t.Errorf("breached=%v: debtNotice fired=%v but debtBreached=%v", breached, got, debtBreached(streams))
		}
	}
}

// ---------------------------------------------------------------------------
// parse: Brief.Measures
// ---------------------------------------------------------------------------

// TestMeasuresParsedFromFrontmatter: `measures:` is an optional but KNOWN
// brief-v1 key. Absent → nil (the neutral default); present → non-nil, even
// when empty.
func TestMeasuresParsedFromFrontmatter(t *testing.T) {
	bf, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", validBriefFM))
	if !ok || err != nil {
		t.Fatalf("valid brief must parse; got ok=%v err=%v", ok, err)
	}
	if bf.Measures != nil {
		t.Errorf("absent measures must parse to nil, got %q", *bf.Measures)
	}

	fm := strings.Replace(validBriefFM, "effort: S\n", "effort: S\nmeasures: verification-debt\n", 1)
	bf, ok, err = parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm))
	if !ok || err != nil {
		t.Fatalf("brief with measures must parse; got ok=%v err=%v", ok, err)
	}
	if bf.Measures == nil || *bf.Measures != "verification-debt" {
		t.Errorf("measures = %v, want \"verification-debt\"", bf.Measures)
	}

	fm = strings.Replace(validBriefFM, "effort: S\n", "effort: S\nmeasures: [a, b]\n", 1)
	if _, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm)); ok || err == nil ||
		!strings.Contains(err.Error(), "measures must be a string") {
		t.Errorf("non-string measures must be a parse error; got ok=%v err=%v", ok, err)
	}
}

// ---------------------------------------------------------------------------
// eligibility
// ---------------------------------------------------------------------------

// TestMeasuresOverThresholdExcludes is the point of the whole brief: a todo
// brief that declares it measures a queue is NOT dispatchable while that queue
// is over its own alarm threshold. Drain before you instrument the drain.
func TestMeasuresOverThresholdExcludes(t *testing.T) {
	streams := measuresBoard(strptr("verification-debt"), true)
	nu := nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
	if picked(nu, "metrics/01") {
		t.Error("metrics/01 declares measures: verification-debt and the queue is over threshold — it must be excluded")
	}
	if !picked(nu, "metrics/02") {
		t.Error("metrics/02 declares no measures: — it must be unaffected")
	}
	if want := []string{"metrics/01"}; len(nu.MeasuresGated) != 1 || nu.MeasuresGated[0] != want[0] {
		t.Errorf("MeasuresGated = %v, want %v (the exclusion must be surfaced, not swallowed)", nu.MeasuresGated, want)
	}
	if len(nu.MeasuresUnknown) != 0 {
		t.Errorf("MeasuresUnknown = %v, want empty (the depth was readable)", nu.MeasuresUnknown)
	}
}

// TestMeasuresUnderThresholdIncludes: the field is INERT once the queue drains.
// The gate holds work back temporarily; it never retires a brief.
func TestMeasuresUnderThresholdIncludes(t *testing.T) {
	streams := measuresBoard(strptr("verification-debt"), false)
	nu := nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
	if !picked(nu, "metrics/01") {
		t.Error("queue is under threshold — metrics/01 must be eligible again")
	}
	if len(nu.MeasuresGated) != 0 || len(nu.MeasuresUnknown) != 0 {
		t.Errorf("nothing should be held back: gated=%v unknown=%v", nu.MeasuresGated, nu.MeasuresUnknown)
	}
}

// TestMeasuresAbsentUnaffected is the default-unchanged guarantee at unit
// level: a board on which NO brief carries `measures:` produces byte-identical
// picks whether or not the measured queue is breached.
func TestMeasuresAbsentUnaffected(t *testing.T) {
	for _, breached := range []bool{true, false} {
		streams := measuresBoard(nil, breached)
		nu := nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
		if !picked(nu, "metrics/01") || !picked(nu, "metrics/02") {
			t.Errorf("breached=%v: briefs with no measures: field must be untouched; picks=%d", breached, len(nu.Picks))
		}
		if len(nu.MeasuresGated) != 0 || len(nu.MeasuresUnknown) != 0 {
			t.Errorf("breached=%v: no measures: field anywhere, nothing may be held back (gated=%v unknown=%v)",
				breached, nu.MeasuresGated, nu.MeasuresUnknown)
		}
	}
}

// TestMeasuresInProgressUnaffected: the gate holds back DISPATCH, not work
// already in flight. Evicting an in-progress brief from the board would hide
// the thing someone is holding, not prevent it being started.
func TestMeasuresInProgressUnaffected(t *testing.T) {
	streams := measuresBoard(strptr("verification-debt"), true)
	streams[0].Briefs[0].Status = "in-progress"
	nu := nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
	if !picked(nu, "metrics/01") {
		t.Error("an in-progress measures: brief must stay on the board")
	}
}

// TestMeasuresUnknownQueueFailsClosed is the three-state arm. A `measures:`
// naming a queue this build cannot read is COULD-NOT-CHECK, not "no gate": the
// depth is unknowable, so whether the gate should fire is unknowable. We fail
// CLOSED (hold the brief back) and NAME it, so the condition is visible on the
// board rather than silently re-permitting the dispatch the gate exists to stop.
func TestMeasuresUnknownQueueFailsClosed(t *testing.T) {
	for _, name := range []string{"reviw-debt", "", "intake-debt"} {
		streams := measuresBoard(strptr(name), false) // queue UNDER threshold: only the unknown name can hold it
		nu := nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
		if picked(nu, "metrics/01") {
			t.Errorf("measures: %q names an unreadable queue — must fail closed, not fall open", name)
		}
		if len(nu.MeasuresUnknown) != 1 || nu.MeasuresUnknown[0] != "metrics/01" {
			t.Errorf("measures: %q: MeasuresUnknown = %v, want [metrics/01] — could-not-check must be reported", name, nu.MeasuresUnknown)
		}
		if len(nu.MeasuresGated) != 0 {
			t.Errorf("measures: %q: an unreadable queue is could-not-check, not a breach; gated=%v", name, nu.MeasuresGated)
		}
		if !picked(nu, "metrics/02") {
			t.Errorf("measures: %q: the sibling with no measures: must stay eligible", name)
		}
	}
}

// TestMeasuresNotAScoreInput pins the F-09 boundary: the gate excludes,
// it never re-ranks. Every brief that IS eligible scores exactly what it scored
// before the field existed.
func TestMeasuresNotAScoreInput(t *testing.T) {
	base := nextUp(measuresBoard(nil, true), ClaimView{Source: ClaimSource{Known: true}}, nil)
	gated := nextUp(measuresBoard(strptr("verification-debt"), true), ClaimView{Source: ClaimSource{Known: true}}, nil)
	for _, p := range gated.Picks {
		id := p.Stream.Name + "/" + p.Brief.Num
		for _, q := range base.Picks {
			if q.Stream.Name+"/"+q.Brief.Num == id && q.Score != p.Score {
				t.Errorf("%s scored %d with the gate and %d without — the gate must not touch scores", id, p.Score, q.Score)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// board rendering
// ---------------------------------------------------------------------------

// TestMeasuresBoardSurfacesHeldBack: both held-back states are RENDERED. The
// failure mode this guards is the silent one — briefs vanishing from a board
// with nothing on the page to say why (the class that hid 596 briefs from a
// dispatcher on 2026-08-13).
func TestMeasuresBoardSurfacesHeldBack(t *testing.T) {
	streams := measuresBoard(strptr("verification-debt"), true)
	nu := nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "DRAIN BEFORE INSTRUMENT") || !strings.Contains(out, "metrics/01") {
		t.Errorf("breached board must name the held-back brief:\n%s", out)
	}

	streams = measuresBoard(strptr("no-such-queue"), false)
	nu = nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
	out = emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "COULD NOT CHECK") || !strings.Contains(out, "metrics/01") {
		t.Errorf("could-not-check board must name the held-back brief:\n%s", out)
	}

	// The neutral default renders neither line.
	streams = measuresBoard(nil, true)
	nu = nextUp(streams, ClaimView{Source: ClaimSource{Known: true}}, nil)
	out = emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(out, "DRAIN BEFORE INSTRUMENT") {
		t.Errorf("no brief carries measures: — the board must not mention the gate:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// lint
// ---------------------------------------------------------------------------

// measuresLintRoot writes a one-stream root whose brief 01 carries the given
// `measures:` frontmatter line (empty string = no line at all) and returns the
// hard problems from checkBriefFiles.
func measuresLintRoot(t *testing.T, measuresLine string) []string {
	t.Helper()
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams", "metrics")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\n" +
		"stream: metrics\n" +
		"status: active\n" +
		"priority: P1\n" +
		"track: test\n" +
		"---\n" +
		"# Metrics\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [One](./brief-01.md) | 0 | S | todo | — | — |\n"
	if err := os.WriteFile(filepath.Join(sdir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := "---\n" +
		"brief: metrics/01\n" +
		"title: Brief One\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		measuresLine +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-07-08\n" +
		"sources: [test]\n" +
		"---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(sdir, "brief-01.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := checkBriefFiles(streams)
	// The wiring is checked here too: the parsed field must reach the Brief row,
	// or the eligibility gate above is reading a field nothing ever populates.
	if measuresLine != "" && len(streams) == 1 && len(streams[0].Briefs) == 1 {
		if streams[0].Briefs[0].Measures == nil {
			t.Error("measures: present in frontmatter but not wired into the Brief row")
		}
	}
	return problems
}

// TestMeasuresUnknownQueueLintProblem: a typo'd queue name is a HARD lint
// PROBLEM, never a silent no-op. The wired set is one queue today.
func TestMeasuresUnknownQueueLintProblem(t *testing.T) {
	if p := measuresLintRoot(t, "measures: verification-debt\n"); hasProblem(p, "measures") {
		t.Errorf("the wired queue name must lint clean; got:\n%s", strings.Join(p, "\n"))
	}
	for _, bad := range []string{"verification-dbet", "\"\"", "queue-depth"} {
		p := measuresLintRoot(t, "measures: "+bad+"\n")
		if !hasProblem(p, "measures", "verification-debt") {
			t.Errorf("measures: %s must be a PROBLEM naming the valid set; got:\n%s", bad, strings.Join(p, "\n"))
		}
	}
	if p := measuresLintRoot(t, ""); hasProblem(p, "measures") {
		t.Errorf("an absent measures: must never be flagged; got:\n%s", strings.Join(p, "\n"))
	}
}
