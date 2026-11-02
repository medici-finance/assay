package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rerunsFixture is the committed graph-execution/02-shaped fixture: the brief file's frontmatter
// and two outcome-record templates. Every date in it is the placeholder @DAY@, replaced with
// yesterday's UTC date at test time so the attempt windows sit inside the 7-day perf window.
// The perf rows are written at test time through the real perfrec resolver and writer
// (fixtureRows), so they always carry the current schema.
const rerunsFixture = "testdata/reruns"

// skillPerfWork is the --perf-work value the verify-desk skill passes for graph-execution/02:
// the frontmatter id assay:assay:graph-execution:02 through PerfBriefRef.
const skillPerfWork = `{"kind":"brief","repo":"medici-finance/assay","brief":"assay:assay:graph-execution:2"}`

func yesterday() string { return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02") }

func fixtureText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(rerunsFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "@DAY@", yesterday())
}

// rerunsHome is seedHome with v3 recording armed.
func rerunsHome(t *testing.T) (stateDir, perfDir string) {
	t.Helper()
	stateDir, perfDir = seedHome(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("USERPROFILE", filepath.Dir(filepath.Dir(stateDir)))
	t.Setenv("ASSAY_PERF_PROFILE", "")
	return stateDir, perfDir
}

// rerunsRoot is a checkout whose origin is the public repo, holding the graph-execution/02
// brief file and the fixture's two outcome records.
func rerunsRoot(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://github.com/medici-finance/assay.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	brief := filepath.Join(root, "docs", "streams", "graph-execution", "brief-02-workflow-patterns.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte(fixtureText(t, "graph-execution-02.frontmatter")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rec := range []string{"record-legacy.json", "record-blocked.json"} {
		placeRecord(t, root, strings.TrimSpace(fixtureText(t, rec)))
	}
	return root
}

// placeRecord lands one record line where deskkit.RecordName puts it — the writer bypassed, as a
// hand-landed record would be. It returns the record's path relative to root.
func placeRecord(t *testing.T, root, line string) string {
	t.Helper()
	name, err := deskkit.RecordName([]byte(line))
	if err != nil {
		t.Fatalf("RecordName: %v", err)
	}
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

// fixtureRow is one perf row of the graph-execution/02 fixture: its ts (HH:MM:SS yesterday)
// and own-process wall. Attempt windows: 1 09:00-09:20, 2 09:30-09:50, 3 10:00-10:30.
type fixtureRow struct {
	at     string
	wallMs int64
}

var fixtureRows = []fixtureRow{
	{"09:05:00", 1200}, {"09:15:00", 800}, // attempt 1, uncredited local-build
	{"09:40:00", 2500},                     // attempt 2, discarded env-fault
	{"10:05:00", 3000}, {"10:25:00", 1000}, // attempt 3, credited
}

// seedRerunsPerf writes fixtureRows as one verifier dispatch would, with the skill's --perf-work.
func seedRerunsPerf(t *testing.T, stateDir string) {
	t.Helper()
	agent := newAgent(t)
	for _, r := range fixtureRows {
		writeVerifyRow(t, stateDir, dayAt(t, r.at), r.wallMs, agent, skillPerfWork)
	}
}

// fixtureWallInside sums the fixture rows whose ts falls inside any of the given windows —
// the expected figure computed from the fixture itself, not restated.
func fixtureWallInside(t *testing.T, windows ...[2]time.Time) int64 {
	t.Helper()
	var sum int64
	for _, r := range fixtureRows {
		at := dayAt(t, r.at)
		for _, w := range windows {
			if !at.Before(w[0]) && !at.After(w[1]) {
				sum += r.wallMs
				break
			}
		}
	}
	return sum
}

// writeVerifyRow writes one workflow perf row through the real perfrec context resolver and
// writer, as a verifier dispatch given --perf-work would: role verify, the given agentId and
// raw work values, a row ts of at and an own-process wall of wallMs.
func writeVerifyRow(t *testing.T, stateDir string, at time.Time, wallMs int64, agent string, work ...string) {
	t.Helper()
	c := perfrec.ResolveContext(perfrec.ContextInput{
		Profile:     perfrec.Value{Value: "workflow", Present: true},
		AgentID:     perfrec.Value{Value: agent, Present: true},
		Role:        perfrec.Value{Value: "verify", Present: true},
		Work:        work,
		WorkPresent: len(work) > 0,
		LookupEnv: func(k string) (string, bool) {
			if k == perfrec.EnvVar {
				return "1", true
			}
			return "", false
		},
	})
	start, calls := at.Add(-time.Duration(wallMs)*time.Millisecond), 0
	p := perfrec.Probe{
		Now: func() time.Time {
			calls++
			if calls == 1 {
				return start
			}
			return at
		},
		Stamp: func() time.Time { return at },
	}
	rec := perfrec.BeginSelected(p, p.Mark(), nil, c).Finish("statusgen", "verifyrun", "", 0, "")
	if err := (&perfrec.Writer{Now: func() time.Time { return at }}).Write(stateDir, rec); err != nil {
		t.Fatalf("perf row: %v", err)
	}
}

func newAgent(t *testing.T) string {
	t.Helper()
	id, err := perfrec.NewID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func dayAt(t *testing.T, hhmmss string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, yesterday()+"T"+hhmmss+"Z")
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func rerunsJSON(t *testing.T, args ...string) rerunsBriefJSON {
	t.Helper()
	rc, out, errOut := runDeskperf(append([]string{"reruns", "--json"}, args...)...)
	if rc != deskkit.ExitOK {
		t.Fatalf("reruns --json: rc %d stderr %s", rc, errOut)
	}
	var got rerunsBriefJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("reruns --json output: %v\n%s", err, out)
	}
	assertNoZeroForUnmeasured(t, out)
	return got
}

// assertNoZeroForUnmeasured walks the raw JSON: wherever a run or side is not measured, every
// measure on it is a {"state","reason"} object, never a number.
func assertNoZeroForUnmeasured(t *testing.T, raw string) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	measures := []string{"rows", "sumOwnProcessWallMs", "sumOwnCpuMs", "cpuRecords", "requestAttempts"}
	check := func(where string, m map[string]any, unmeasured bool) {
		for _, k := range measures {
			v, ok := m[k]
			if !ok {
				continue
			}
			if _, isNum := v.(float64); isNum && unmeasured {
				t.Fatalf("%s: %s is the number %v on a measure that was not taken", where, k, v)
			}
		}
	}
	atts, _ := doc["attempts"].([]any)
	for _, a := range atts {
		m := a.(map[string]any)
		check("attempt", m, m["state"] != stateMeasured)
	}
	for _, side := range []string{"total", "uncreditedPlusDiscarded"} {
		m, _ := doc[side].(map[string]any)
		if m == nil {
			continue
		}
		check(side, m, m["measured"] == float64(0) && m["runs"] != float64(0))
	}
	if un, _ := doc["unassigned"].(map[string]any); un != nil {
		if _, isState := un["rows"].(map[string]any); isState {
			check("unassigned", un, true)
		}
	}
}

func stateOf(t *testing.T, v any) stateJSON {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("want a {state, reason} object, got %#v", v)
	}
	return stateJSON{State: m["state"].(string), Reason: m["reason"].(string)}
}

func num(t *testing.T, v any) int64 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("want a number, got %#v", v)
	}
	return int64(f)
}

// regression: #4163
// TestReruns_GraphExecution02Fixture is Verify row 2: the issue's own case — one blocked
// record with attempts [uncredited local-build, discarded env-fault, credited] and one earlier
// legacy record with none — counts 4 runs, and the cost block's uncredited-plus-discarded wall
// is exactly the fixture rows inside attempts 1-2.
func TestReruns_GraphExecution02Fixture(t *testing.T) {
	stateDir, _ := rerunsHome(t)
	seedRerunsPerf(t, stateDir)
	root := rerunsRoot(t)

	rc, out, errOut := runDeskperf("reruns", "--root", root, "--brief", "graph-execution/02")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc %d stderr %s", rc, errOut)
	}
	t.Logf("reruns output:\n%s", out)
	lines := strings.Split(out, "\n")
	if lines[0] != "graph-execution/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded" {
		t.Fatalf("count line = %q", lines[0])
	}
	a1 := [2]time.Time{dayAt(t, "09:00:00"), dayAt(t, "09:20:00")}
	a2 := [2]time.Time{dayAt(t, "09:30:00"), dayAt(t, "09:50:00")}
	a3 := [2]time.Time{dayAt(t, "10:00:00"), dayAt(t, "10:30:00")}
	wantWaste := fixtureWallInside(t, a1, a2)
	wantTotal := fixtureWallInside(t, a1, a2, a3)
	if wantWaste == 0 || wantTotal <= wantWaste {
		t.Fatalf("fixture rows do not cover the windows: waste %d total %d", wantWaste, wantTotal)
	}
	for _, want := range []string{
		"uncredited+discarded: own-process wall " + itoa64(wantWaste) + " ms",
		"total: own-process wall " + itoa64(wantTotal) + " ms",
		"share: ", "% of the measured total own-process wall",
		"verifier tokens: could-not-check (desk-supervision/34 records worker dispatches only)",
		"not counted: verifier dispatches that landed no outcome record",
		"could-not-check (no window",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q", want)
		}
	}

	got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
	if got.Runs != 4 || num(t, got.Waste.SumOwnProcessWallMs) != wantWaste || num(t, got.Total.SumOwnProcessWallMs) != wantTotal {
		t.Fatalf("json: runs %d waste %v total %v", got.Runs, got.Waste.SumOwnProcessWallMs, got.Total.SumOwnProcessWallMs)
	}
	if _, ok := got.Share.(string); !ok {
		t.Fatalf("share not printed: %#v", got.Share)
	}
	// The fixture rows carry no CPU measure: own CPU is could-not-check, never 0 ms.
	if s := stateOf(t, got.Waste.SumOwnCPUMs); s.State != stateCouldNotCheck || s.Reason != noCPU {
		t.Fatalf("own CPU with no CPU-carrying row: %+v", s)
	}
	if got.VerifierTokens.State != stateCouldNotCheck || got.WorkRef.Brief != "assay:assay:graph-execution:2" {
		t.Fatalf("tokens %+v ref %+v", got.VerifierTokens, got.WorkRef)
	}

	// Without --brief: one count line per brief.
	rc, out, _ = runDeskperf("reruns", "--root", root)
	if rc != deskkit.ExitOK || !strings.HasPrefix(out, "graph-execution/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded\n") {
		t.Fatalf("all-briefs view: rc %d\n%s", rc, out)
	}
	// No record for the brief: unverifiable.
	if rc, _, errOut := runDeskperf("reruns", "--root", root, "--brief", "graph-execution/03"); rc != deskkit.ExitUnverifiable ||
		!strings.Contains(errOut, "no record for graph-execution/03") {
		t.Fatalf("missing brief: rc %d %s", rc, errOut)
	}
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// regression: #4163
// TestReruns_InvalidStoredRecordNotCounted is Verify row 4: a record landed with the writer
// bypassed, whose attempts put credited first, is listed invalid with its path and not counted.
func TestReruns_InvalidStoredRecordNotCounted(t *testing.T) {
	rerunsHome(t)
	root := rerunsRoot(t)
	day := yesterday()
	bad := `{"ts":"` + day + `T15:00:00Z","brief":"graph-execution/02","outcome":"blocked","sha":"0000004",` +
		`"blocker_kind":"environment","blocker_ref":"example-org/example#12","started":"` + day + `T13:00:00Z",` +
		`"finished":"` + day + `T14:00:00Z","rows_total":9,"attempts":[` +
		`{"attempt":1,"disposition":"credited","started":"` + day + `T13:00:00Z","finished":"` + day + `T13:10:00Z","rows_run":9},` +
		`{"attempt":2,"disposition":"uncredited","reason":"not-credited","started":"` + day + `T13:20:00Z","finished":"` + day + `T13:30:00Z","rows_run":9}]}`
	path := placeRecord(t, root, bad)

	rc, out, errOut := runDeskperf("reruns", "--root", root, "--brief", "graph-execution/02")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc %d stderr %s", rc, errOut)
	}
	if !strings.HasPrefix(out, "graph-execution/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded\n") {
		t.Fatalf("the invalid record was counted:\n%s", out)
	}
	if want := "invalid " + path + ": refused: attempts: attempt 1: rule credited-last"; !strings.Contains(out, want) {
		t.Fatalf("output lacks %q:\n%s", want, out)
	}
	got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
	if got.Runs != 4 || len(got.Invalid) != 1 || got.Invalid[0].Path != path {
		t.Fatalf("json: runs %d invalid %+v", got.Runs, got.Invalid)
	}
}

// regression: #4163
// TestReruns_NeverEstimates is Verify row 5: a measure that is missing, pruned or ambiguous is
// a stated state, never a 0 and never apportioned; rows outside every window are unassigned.
func TestReruns_NeverEstimates(t *testing.T) {
	t.Run("empty perf dir", func(t *testing.T) {
		rerunsHome(t)
		root := rerunsRoot(t)
		rc, out, _ := runDeskperf("reruns", "--root", root, "--brief", "graph-execution/02")
		if rc != deskkit.ExitOK || !strings.HasPrefix(out, "graph-execution/02: 4 verification runs, 1 uncredited, 1 discarded, 1 unrecorded\n") ||
			!strings.Contains(out, "could-not-check (no perf rows: no perf history") {
			t.Fatalf("rc %d\n%s", rc, out)
		}
		got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
		for _, v := range []any{got.Total.SumOwnProcessWallMs, got.Waste.SumOwnProcessWallMs, got.Share, got.Unassigned["rows"]} {
			if s := stateOf(t, v); s.State != stateCouldNotCheck {
				t.Fatalf("state %+v", s)
			}
		}
		for _, a := range got.Attempts {
			if a.Attempt > 0 && !strings.HasPrefix(a.StateReason, "no perf rows: ") {
				t.Fatalf("attempt %d: %s %s", a.Attempt, a.State, a.StateReason)
			}
		}
	})
	t.Run("window older than --days", func(t *testing.T) {
		stateDir, _ := rerunsHome(t)
		seedRerunsPerf(t, stateDir)
		root := rerunsRoot(t)
		rc, out, _ := runDeskperf("reruns", "--root", root, "--brief", "graph-execution/02", "--days", "1")
		if rc != deskkit.ExitOK || !strings.Contains(out, "could-not-check (pruned") {
			t.Fatalf("rc %d\n%s", rc, out)
		}
		got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02", "--days", "1")
		for _, a := range got.Attempts {
			if a.Attempt > 0 && (a.State != stateCouldNotCheck || !strings.HasPrefix(a.StateReason, "pruned")) {
				t.Fatalf("attempt %d: %s %s", a.Attempt, a.State, a.StateReason)
			}
		}
		if s := stateOf(t, got.Waste.SumOwnProcessWallMs); s.State != stateCouldNotCheck {
			t.Fatalf("waste %+v", s)
		}
	})
	t.Run("two agentIds in one window", func(t *testing.T) {
		stateDir, _ := rerunsHome(t)
		root := rerunsRoot(t)
		one, two := newAgent(t), newAgent(t)
		writeVerifyRow(t, stateDir, dayAt(t, "09:05:00"), 1000, one, skillPerfWork)
		writeVerifyRow(t, stateDir, dayAt(t, "09:10:00"), 7000, two, skillPerfWork)
		writeVerifyRow(t, stateDir, dayAt(t, "09:40:00"), 2000, one, skillPerfWork)
		writeVerifyRow(t, stateDir, dayAt(t, "10:10:00"), 3000, one, skillPerfWork)
		rc, out, _ := runDeskperf("reruns", "--root", root, "--brief", "graph-execution/02")
		if rc != deskkit.ExitOK || !strings.Contains(out, "ambiguous (2 distinct agentIds in one window") {
			t.Fatalf("rc %d\n%s", rc, out)
		}
		got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
		if got.Attempts[1].Attempt != 1 || got.Attempts[1].State != stateAmbiguous {
			t.Fatalf("attempt 1: %+v", got.Attempts[1])
		}
		if s := stateOf(t, got.Attempts[1].SumOwnProcessWallMs); s.State != stateAmbiguous {
			t.Fatalf("ambiguous attempt carries a figure: %+v", got.Attempts[1])
		}
		// Neither sum holds any of the ambiguous window's 8000 ms, whole or split.
		if w, tot := num(t, got.Waste.SumOwnProcessWallMs), num(t, got.Total.SumOwnProcessWallMs); w != 2000 || tot != 5000 {
			t.Fatalf("ambiguous window leaked into the sums: waste %d total %d", w, tot)
		}
	})
	t.Run("row outside every window", func(t *testing.T) {
		stateDir, _ := rerunsHome(t)
		root := rerunsRoot(t)
		agent := newAgent(t)
		writeVerifyRow(t, stateDir, dayAt(t, "09:05:00"), 1000, agent, skillPerfWork)
		writeVerifyRow(t, stateDir, dayAt(t, "10:10:00"), 3000, agent, skillPerfWork)
		writeVerifyRow(t, stateDir, dayAt(t, "12:00:00"), 4400, agent, skillPerfWork)
		got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
		if num(t, got.Unassigned["rows"]) != 1 || num(t, got.Unassigned["sumOwnProcessWallMs"]) != 4400 {
			t.Fatalf("unassigned %+v", got.Unassigned)
		}
		if w, tot := num(t, got.Waste.SumOwnProcessWallMs), num(t, got.Total.SumOwnProcessWallMs); w != 1000 || tot != 4000 {
			t.Fatalf("unassigned row folded into a sum: waste %d total %d", w, tot)
		}
	})
}

// regression: #4163
// TestReruns_JoinsQualifiedWorkRefOnly is Verify row 12's join half: rows written through the
// real perfrec resolver with the skill's qualified --perf-work join; rows written with the
// <stream>/<NN> id carry no work at all, so the attempt is could-not-check (no perf rows).
func TestReruns_JoinsQualifiedWorkRefOnly(t *testing.T) {
	ref, err := deskkit.PerfBriefRef("assay:assay:graph-execution:02")
	if err != nil || !strings.Contains(skillPerfWork, `"brief":"`+ref+`"`) {
		t.Fatalf("the skill's value is not PerfBriefRef's: %q %v", ref, err)
	}
	t.Run("qualified ref joins", func(t *testing.T) {
		stateDir, _ := rerunsHome(t)
		root := rerunsRoot(t)
		agent := newAgent(t)
		writeVerifyRow(t, stateDir, dayAt(t, "09:05:00"), 1500, agent, skillPerfWork)
		writeVerifyRow(t, stateDir, dayAt(t, "10:05:00"), 2500, agent, skillPerfWork)
		got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
		if num(t, got.Waste.SumOwnProcessWallMs) != 1500 || num(t, got.Total.SumOwnProcessWallMs) != 4000 {
			t.Fatalf("qualified rows did not join: waste %v total %v", got.Waste.SumOwnProcessWallMs, got.Total.SumOwnProcessWallMs)
		}
	})
	t.Run("short id carries no work", func(t *testing.T) {
		stateDir, perfDir := rerunsHome(t)
		root := rerunsRoot(t)
		agent := newAgent(t)
		short := `{"kind":"brief","repo":"medici-finance/assay","brief":"graph-execution/02"}`
		writeVerifyRow(t, stateDir, dayAt(t, "09:05:00"), 1500, agent, short)
		writeVerifyRow(t, stateDir, dayAt(t, "10:05:00"), 2500, agent, short)
		b, err := os.ReadFile(filepath.Join(perfDir, yesterday()+".jsonl"))
		if err != nil || strings.Contains(string(b), "graph-execution/02") || !strings.Contains(string(b), `"work":"invalid"`) {
			t.Fatalf("perfrec kept a <stream>/<NN> work ref (or wrote nothing): %v\n%s", err, b)
		}
		got := rerunsJSON(t, "--root", root, "--brief", "graph-execution/02")
		for _, a := range got.Attempts {
			if a.Attempt > 0 && (a.State != stateCouldNotCheck || !strings.HasPrefix(a.StateReason, "no perf rows: ")) {
				t.Fatalf("attempt %d reported a cost from rows with no work: %s %s", a.Attempt, a.State, a.StateReason)
			}
		}
		if s := stateOf(t, got.Total.SumOwnProcessWallMs); s.State != stateCouldNotCheck {
			t.Fatalf("total %+v", s)
		}
	})
}
