package deskkit

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// autolane_test.go — the lane's decision logic, pure, with the roster predicates stubbed.
// Every fixture value is an example-org placeholder: no deployment's real area, login or
// repo appears here.

const alRepo = "example-org/tracker"

func alValidator() AutoLaneValidator {
	return AutoLaneValidator{
		RepoAllowed:   func(r string) bool { return r == alRepo || r == "example-org/other" },
		LoginMayOptIn: func(l string) bool { return l == "ada" },
	}
}

func alRaw(areas string) map[string]string {
	return map[string]string{
		EnvAutoApproveAreas:     areas,
		EnvAutoApproveEjectLine: "0",
		EnvAutoApproveFPYFloor:  "0.90",
		EnvAutoApproveDailyCap:  "2",
	}
}

func alConfig(t *testing.T, areas string) AutoLaneConfig {
	t.Helper()
	ld := ParseAutoLaneConfig(alRaw(areas), alValidator())
	if ld.State != AutoLaneConfigLoaded {
		t.Fatalf("fixture config did not load: %s — %s", ld.State, ld.Problem)
	}
	return ld.Config
}

// noRisk classes nothing; riskSecrets classes anything under secrets/ — the stand-in for the
// compiled base triggers, so these tests do not depend on a roster.
func noRisk(string, []string) bool { return false }
func riskSecrets(_ string, paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, "secrets/") || strings.HasPrefix(p, ".github/workflows/") {
			return true
		}
	}
	return false
}

// TestAutoLaneConfigUnsetIsClosed — the SHIPPED state. No key set is unconfigured: the lane
// is closed and it is not an error. Fail-first: a parser that defaulted any number would
// return Loaded here.
func TestAutoLaneConfigUnsetIsClosed(t *testing.T) {
	for _, raw := range []map[string]string{nil, {}, {EnvAutoApproveAreas: "  "}} {
		if ld := ParseAutoLaneConfig(raw, alValidator()); ld.State != AutoLaneUnconfigured {
			t.Fatalf("raw %v: state %s, want unconfigured (closed)", raw, ld.State)
		}
	}
}

// TestAutoLane_ConfigPartialIs_Refused — a subset is never a partial lane.
func TestAutoLane_ConfigPartialIs_Refused(t *testing.T) {
	raw := alRaw(alRepo + ":docs/notes/**:ada")
	delete(raw, EnvAutoApproveDailyCap)
	ld := ParseAutoLaneConfig(raw, alValidator())
	if ld.State != AutoLaneConfigRefused || !strings.Contains(ld.Problem, EnvAutoApproveDailyCap) {
		t.Fatalf("partial config: state %s problem %q — want refused naming the missing key", ld.State, ld.Problem)
	}
}

func TestAutoLaneConfigRefusals(t *testing.T) {
	cases := []struct {
		name, key, val, want string
	}{
		{"untrusted opt-in login", EnvAutoApproveAreas, alRepo + ":docs/notes/**:mallory", "opt-in login not trusted"},
		{"repo outside the allowed set", EnvAutoApproveAreas, "example-org/elsewhere:docs/notes/**:ada", "area repo not allowed"},
		{"pattern repo", EnvAutoApproveAreas, "example-org/*:docs/notes/**:ada", "full owner/name slug"},
		{"missing login", EnvAutoApproveAreas, alRepo + ":docs/notes/**", "owner"},
		{"glob outside the subset", EnvAutoApproveAreas, alRepo + ":docs/[a-z]/**:ada", "outside the .assay-surfaces subset"},
		{"glob from the root", EnvAutoApproveAreas, alRepo + ":**/notes.md:ada", "repo root"},
		{"star from the root", EnvAutoApproveAreas, alRepo + ":*.md:ada", "repo root"},
		{"partial double star", EnvAutoApproveAreas, alRepo + ":docs/a**/x:ada", "whole segment"},
		{"relative segment", EnvAutoApproveAreas, alRepo + ":docs/../secrets/**:ada", "relative segment"},
		{"eject line at the signal count switches the ejector off", EnvAutoApproveEjectLine, "6", EnvAutoApproveEjectLine},
		{"negative eject line", EnvAutoApproveEjectLine, "-1", EnvAutoApproveEjectLine},
		{"fpy floor zero switches the kill signal off", EnvAutoApproveFPYFloor, "0", EnvAutoApproveFPYFloor},
		{"fpy floor above one", EnvAutoApproveFPYFloor, "1.5", EnvAutoApproveFPYFloor},
		{"fpy floor NaN", EnvAutoApproveFPYFloor, "NaN", EnvAutoApproveFPYFloor},
		{"daily cap zero", EnvAutoApproveDailyCap, "0", EnvAutoApproveDailyCap},
		{"daily cap unbounded", EnvAutoApproveDailyCap, "100000", EnvAutoApproveDailyCap},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := alRaw(alRepo + ":docs/notes/**:ada")
			raw[c.key] = c.val
			ld := ParseAutoLaneConfig(raw, alValidator())
			if ld.State != AutoLaneConfigRefused {
				t.Fatalf("state %s, want refused", ld.State)
			}
			if !strings.Contains(ld.Problem, c.want) {
				t.Fatalf("problem %q does not name %q", ld.Problem, c.want)
			}
		})
	}
}

// TestAutoLane_ConfigRefuses_SurfaceOverlap — an area that can reach a declared surface is
// refused at load, in either direction of containment.
func TestAutoLane_ConfigRefuses_SurfaceOverlap(t *testing.T) {
	surfaces := []string{".github/workflows/**", "tools/*guard*/**", "docs/leak/**"}
	for _, glob := range []string{".github/workflows/**", "tools/writeguard/**", "docs/**"} {
		c := alConfig(t, alRepo+":"+glob+":ada")
		p := ValidateAutoLaneRepo(c, alRepo, true, surfaces, noRisk)
		if !strings.Contains(p, "refused: area overlaps declared surface") || !strings.Contains(p, glob) {
			t.Fatalf("glob %q: problem %q — want an overlap refusal naming the entry", glob, p)
		}
	}
	c := alConfig(t, alRepo+":docs/notes/**:ada")
	if p := ValidateAutoLaneRepo(c, alRepo, true, surfaces, noRisk); p != "" {
		t.Fatalf("a disjoint area was refused: %q", p)
	}
}

func TestAutoLane_ConfigRefuses_AbsentSurfaces(t *testing.T) {
	c := alConfig(t, alRepo+":docs/notes/**:ada")
	if p := ValidateAutoLaneRepo(c, alRepo, false, nil, noRisk); !strings.Contains(p, "declares no surfaces") {
		t.Fatalf("absent .assay-surfaces: problem %q — want a refusal (no declared surfaces is never safe)", p)
	}
}

func TestAutoLane_ConfigRefusesRisk_ClassedArea(t *testing.T) {
	c := alConfig(t, alRepo+":secrets/public/**:ada")
	if p := ValidateAutoLaneRepo(c, alRepo, true, nil, riskSecrets); !strings.Contains(p, "risk-classed") {
		t.Fatalf("problem %q — want a risk-classed refusal", p)
	}
	// A classifier that cannot answer (nil) is classed, never clean.
	if p := AutoLaneAreaTripwires(c, alRepo, nil); !strings.Contains(p, "risk-classed") {
		t.Fatalf("nil classifier: problem %q — want a refusal", p)
	}
}

// TestAutoLane_ConfigRefusesBrief_ReachingArea — no opt-in may reach a stream brief file.
func TestAutoLane_ConfigRefusesBrief_ReachingArea(t *testing.T) {
	for _, glob := range []string{"docs/streams/**", "docs/streams/*/brief-*.md", "docs/**"} {
		c := alConfig(t, alRepo+":"+glob+":ada")
		if p := AutoLaneAreaTripwires(c, alRepo, noRisk); !strings.Contains(p, "stream brief file") {
			t.Fatalf("glob %q: problem %q — want a stream-brief refusal", glob, p)
		}
	}
	c := alConfig(t, alRepo+":docs/streams/FINDINGS.md:ada")
	if p := AutoLaneAreaTripwires(c, alRepo, noRisk); p != "" {
		t.Fatalf("a single non-brief file was refused: %q", p)
	}
}

// --- category admit ---

func admitIn() AutoLaneAdmitInput {
	return AutoLaneAdmitInput{
		Repo:            alRepo,
		AuthorLogin:     "example-worker-app[bot]",
		WorkerAppLogin:  "example-worker-app[bot]",
		Body:            "A change.\n\nIssue: #12\n",
		ChangedFiles:    []string{"docs/notes/2026-01-01-note.md"},
		SurfacesPresent: true,
		SurfaceGlobs:    []string{".github/workflows/**", "tools/**"},
		RiskClassed:     noRisk,
	}
}

// TestAutoLaneAdmitWithinScope — the admit fixture: every path inside an opted-in area, no
// tripwire. Fail-first: flipping any single tripwire below reddens it (the table test).
func TestAutoLaneAdmitWithinScope(t *testing.T) {
	c := alConfig(t, alRepo+":docs/notes/**:ada")
	if a := AdmitAutoLane(c, admitIn()); !a.Admitted {
		t.Fatalf("within-scope fixture not admitted: %v", a.Details)
	}
}

// TestAutoLane_AdmitBriefFile_Ejects — a stream brief file in the diff trips the category
// even when an operator's glob would have matched it.
func TestAutoLane_AdmitBriefFile_Ejects(t *testing.T) {
	c := AutoLaneConfig{Areas: []AutoLaneArea{{Repo: alRepo, Glob: "docs/**", Login: "ada"}}, EjectLine: 0, FPYFloor: 0.9, DailyCap: 2}
	in := admitIn()
	in.ChangedFiles = []string{"docs/notes/a.md", "docs/streams/example/brief-03-thing.md"}
	a := AdmitAutoLane(c, in)
	if a.Admitted || !alHas(a.Tripwires, TripStreamBrief) {
		t.Fatalf("brief file in the diff: admitted=%t tripwires=%v — want %s", a.Admitted, a.Tripwires, TripStreamBrief)
	}
}

func TestAutoLaneAdmitTripwires(t *testing.T) {
	c := alConfig(t, alRepo+":docs/notes/**:ada")
	cases := []struct {
		name string
		mut  func(*AutoLaneAdmitInput)
		want string
	}{
		{"human author", func(i *AutoLaneAdmitInput) { i.AuthorLogin = "ada" }, TripAuthorNotWorker},
		{"other App author", func(i *AutoLaneAdmitInput) { i.AuthorLogin = "example-reviewer-app[bot]" }, TripAuthorNotWorker},
		{"bare slug squat", func(i *AutoLaneAdmitInput) { i.AuthorLogin = "example-worker-app" }, TripAuthorNotWorker},
		{"unbound worker role", func(i *AutoLaneAdmitInput) { i.WorkerAppLogin = "" }, TripUnreadable},
		{"no trailer", func(i *AutoLaneAdmitInput) { i.Body = "no link" }, TripNoTrailer},
		{"two trailers", func(i *AutoLaneAdmitInput) { i.Body = "Issue: #1\nIssue: #2\n" }, TripNoTrailer},
		{"path outside area", func(i *AutoLaneAdmitInput) { i.ChangedFiles = append(i.ChangedFiles, "README.md") }, TripPathOutsideArea},
		{"risk classed", func(i *AutoLaneAdmitInput) { i.RiskClassed = func(string, []string) bool { return true } }, TripRiskClassed},
		{"nil classifier", func(i *AutoLaneAdmitInput) { i.RiskClassed = nil }, TripRiskClassed},
		{"surface core computed", func(i *AutoLaneAdmitInput) { i.SurfaceGlobs = []string{"docs/notes/**"} }, TripSurfaceCore},
		{"surface core label", func(i *AutoLaneAdmitInput) { i.Labels = []string{SurfaceCoreLabel} }, TripSurfaceCore},
		{"surfaces absent", func(i *AutoLaneAdmitInput) { i.SurfacesPresent = false }, TripSurfaceAbsent},
		{"surfaces unreadable", func(i *AutoLaneAdmitInput) { i.SurfacesErr = errors.New("500") }, TripUnreadable},
		{"files unreadable", func(i *AutoLaneAdmitInput) { i.FilesErr = errors.New("500") }, TripUnreadable},
		{"no files", func(i *AutoLaneAdmitInput) { i.ChangedFiles = nil }, TripUnreadable},
		{"blank path", func(i *AutoLaneAdmitInput) { i.ChangedFiles = append(i.ChangedFiles, " ") }, TripUnreadable},
		{"reviews unreadable", func(i *AutoLaneAdmitInput) { i.ReviewsErr = errors.New("500") }, TripUnreadable},
		{"security fail at an old head", func(i *AutoLaneAdmitInput) {
			i.Reviews = []Review{{State: "COMMENTED", CommitID: "old", Body: "Security-Review: fail"},
				{State: "APPROVED", CommitID: "new", Body: "Security-Review: pass"}}
		}, TripSecurityFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := admitIn()
			tc.mut(&in)
			a := AdmitAutoLane(c, in)
			if a.Admitted || !alHas(a.Tripwires, tc.want) {
				t.Fatalf("admitted=%t tripwires=%v — want %s", a.Admitted, a.Tripwires, tc.want)
			}
		})
	}
}

// --- score ---

func greenChecks() *ChecksAtHead {
	return &ChecksAtHead{CheckRunsTotalCount: 1, CheckRuns: []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success", CompletedAt: "2026-01-01T00:00:00Z"}}}
}

const alReviewer = "example-reviewer-app[bot]"

func cleanScoreIn() AutoLaneScoreInput {
	return AutoLaneScoreInput{
		Head:          "h2",
		Reviews:       []Review{{State: "APPROVED", CommitID: "h2"}},
		Checks:        greenChecks(),
		Model:         ModelStamped,
		Labels:        []string{SizeLabelPrefix + "S"},
		Events:        []LabelEvent{{Name: SizeLabelPrefix + "S", AppliedBy: alReviewer}},
		ReviewerLogin: alReviewer,
	}
}

func TestAutoLaneScoreCleanIsZero(t *testing.T) {
	s := ScoreAutoLane(cleanScoreIn())
	if s.Score != 0 || s.Ejects(0) {
		t.Fatalf("clean fixture scored %d (%v)", s.Score, s.Fired)
	}
}

// TestAutoLaneScoreOverLineEjects — one fired signal against line 0 ejects; the same score
// against line 1 does not. Fail-first: an Ejects that compared >= line+1, or a score that did
// not count, keeps the first case in the lane.
func TestAutoLaneScoreOverLineEjects(t *testing.T) {
	in := cleanScoreIn()
	in.Labels = []string{SizeLabelPrefix + "L"}
	in.Events = nil // size:L fires whoever applied it: only-narrowing
	s := ScoreAutoLane(in)
	if s.Score != 1 || !alHas(s.Fired, SignalSizeLarge) {
		t.Fatalf("size:L fixture: score %d fired %v", s.Score, s.Fired)
	}
	if !s.Ejects(0) {
		t.Fatalf("score 1 over line 0 did not eject")
	}
	if s.Ejects(1) {
		t.Fatalf("score 1 at line 1 ejected — the line is 'above', not 'at'")
	}
}

// TestAutoLane_ScoreSuperseded_ChangesRequested — the laundered-latest-view defect: an
// APPROVED after a CHANGES_REQUESTED still fires review-rework and push-after-request.
func TestAutoLane_ScoreSuperseded_ChangesRequested(t *testing.T) {
	in := cleanScoreIn()
	in.Reviews = []Review{{State: "CHANGES_REQUESTED", CommitID: "h1"}, {State: "APPROVED", CommitID: "h2"}}
	s := ScoreAutoLane(in)
	if !alHas(s.Fired, SignalReviewRework) || !alHas(s.Fired, SignalPushAfterRequest) {
		t.Fatalf("superseded CR: fired %v — want review-rework and push-after-request", s.Fired)
	}
}

func TestAutoLaneScoreSignals(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*AutoLaneScoreInput)
		want string
	}{
		{"unstamped model", func(i *AutoLaneScoreInput) { i.Model = ModelUnknown }, SignalModelUnstamped},
		{"indeterminate model", func(i *AutoLaneScoreInput) { i.Model = ModelIndeterminate }, SignalModelUnstamped},
		{"timeline unreadable", func(i *AutoLaneScoreInput) { i.ModelErr = errors.New("500") }, SignalUnreadable},
		{"reviews unreadable", func(i *AutoLaneScoreInput) { i.ReviewsErr = errors.New("500") }, SignalUnreadable},
		{"checks unreadable", func(i *AutoLaneScoreInput) { i.ChecksErr = errors.New("500") }, SignalUnreadable},
		{"no head", func(i *AutoLaneScoreInput) { i.Head = "" }, SignalUnreadable},
		{"red check", func(i *AutoLaneScoreInput) { i.Checks.CheckRuns[0].Conclusion = "failure" }, SignalCINonsuccess},
		{"cancelled latest", func(i *AutoLaneScoreInput) { i.Checks.CheckRuns[0].Conclusion = "cancelled" }, SignalCINonsuccess},
		{"pending", func(i *AutoLaneScoreInput) { i.Checks.CheckRuns[0].Status = "in_progress" }, SignalUnreadable},
		{"empty rollup", func(i *AutoLaneScoreInput) { i.Checks = &ChecksAtHead{} }, SignalUnreadable},
		{"size label absent", func(i *AutoLaneScoreInput) { i.Labels = nil }, SignalUnreadable},
		{"size label by another identity", func(i *AutoLaneScoreInput) {
			i.Events = []LabelEvent{{Name: SizeLabelPrefix + "S", AppliedBy: "example-worker-app[bot]"}}
		}, SignalUnreadable},
		{"size label unattributed", func(i *AutoLaneScoreInput) { i.Events = nil }, SignalUnreadable},
		{"two size labels", func(i *AutoLaneScoreInput) { i.Labels = append(i.Labels, SizeLabelPrefix+"M") }, SignalUnreadable},
		{"reviewer role unbound", func(i *AutoLaneScoreInput) { i.ReviewerLogin = "" }, SignalUnreadable},
		{"failed status", func(i *AutoLaneScoreInput) {
			i.Checks.StatusTotalCount = 1
			i.Checks.Statuses = []StatusContext{{Context: "leak-sweep", State: "failure"}}
		}, SignalCINonsuccess},
		{"truncated rollup", func(i *AutoLaneScoreInput) { i.Checks.CheckRunsTotalCount = 5 }, SignalUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := cleanScoreIn()
			in.Checks = greenChecks()
			tc.mut(&in)
			s := ScoreAutoLane(in)
			if !alHas(s.Fired, tc.want) || !s.Ejects(0) {
				t.Fatalf("fired %v — want %s to fire and eject", s.Fired, tc.want)
			}
		})
	}
}

// TestAutoLane_CINonsuccess_IgnoresSuperseded_Reruns — five runs of one check name at head, the
// earliest four cancelled/failed and the LAST green: the signal judges the latest run only.
func TestAutoLane_CINonsuccess_IgnoresSuperseded_Reruns(t *testing.T) {
	in := cleanScoreIn()
	in.Checks = &ChecksAtHead{CheckRunsTotalCount: 5, CheckRuns: []CheckRun{
		{Name: "prompt", Status: "completed", Conclusion: "cancelled", CompletedAt: "2026-01-01T00:01:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "failure", CompletedAt: "2026-01-01T00:02:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "success", CompletedAt: "2026-01-01T00:05:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "cancelled", CompletedAt: "2026-01-01T00:03:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "failure", CompletedAt: "2026-01-01T00:04:00Z"},
	}}
	if s := ScoreAutoLane(in); alHas(s.Fired, SignalCINonsuccess) {
		t.Fatalf("latest run green, earlier runs superseded — ci-nonsuccess fired anyway (%s)", s.CIDetail)
	}
	// And the converse: the LATEST run cancelled reads non-green.
	in.Checks.CheckRuns[2].CompletedAt = "2026-01-01T00:00:30Z"
	if s := ScoreAutoLane(in); !alHas(s.Fired, SignalCINonsuccess) {
		t.Fatalf("latest run failed — ci-nonsuccess did not fire")
	}
}

func TestConclusionGreenSet(t *testing.T) {
	for _, c := range []string{"success", "NEUTRAL", " skipped "} {
		if !ConclusionGreen(c) {
			t.Errorf("%q not green", c)
		}
	}
	for _, c := range []string{"cancelled", "failure", "timed_out", "action_required", "stale", "", "weird"} {
		if ConclusionGreen(c) {
			t.Errorf("%q read green", c)
		}
	}
}

// --- kill signal ---

// TestAutoLane_KillSignalBelow_FloorDisarms — n >= 10 and fpy under the floor HOLDS. Fail-first:
// a reader that compared against n alone, or read fpy <= floor as healthy, stays armed.
func TestAutoLane_KillSignalBelow_FloorDisarms(t *testing.T) {
	k := EvalAutoLaneKillSignal([]byte(`{"auto-lane":{"n":12,"firstPassYield":0.75}}`), 0.90)
	if k.Armed() || k.Line != "lane: hold (fpy 0.75 < floor 0.90, n=12)" {
		t.Fatalf("below floor: armed=%t line %q", k.Armed(), k.Line)
	}
	k = EvalAutoLaneKillSignal([]byte(`{"auto-lane":{"n":12,"firstPassYield":0.95}}`), 0.90)
	if !k.Armed() || k.State != KillSignalHealthy {
		t.Fatalf("above floor: armed=%t line %q", k.Armed(), k.Line)
	}
	k = EvalAutoLaneKillSignal([]byte(`{"auto-lane":{"n":4,"firstPassYield":0.10}}`), 0.90)
	if !k.Armed() || k.State != KillSignalEarly {
		t.Fatalf("under 10 merges: armed=%t line %q — want early (could-not-check for FPY, lane runs)", k.Armed(), k.Line)
	}
	k = EvalAutoLaneKillSignal([]byte(`{"auto-lane":{"n":0,"firstPassYield":"could-not-check","note":"x"}}`), 0.90)
	if k.State != KillSignalEarly {
		t.Fatalf("typed absence n=0: %q — want early", k.Line)
	}
}

func TestAutoLane_KillSignal_UnreadableHolds(t *testing.T) {
	for _, data := range []string{``, `not json`, `{}`, `{"size:L":{"n":3}}`, `{"auto-lane":{}}`,
		`{"auto-lane":{"n":20,"firstPassYield":"could-not-check"}}`, `{"auto-lane":{"n":20,"firstPassYield":7}}`} {
		if k := EvalAutoLaneKillSignal([]byte(data), 0.9); k.Armed() {
			t.Fatalf("content %q read armed (%q)", data, k.Line)
		}
	}
	if k := ReadAutoLaneKillSignal("", 0.9); k.Armed() || k.Line != "lane: hold (could-not-check)" {
		t.Fatalf("no file configured: %q", k.Line)
	}
	if k := ReadAutoLaneKillSignal(t.TempDir()+"/absent.json", 0.9); k.Armed() || k.Line != "lane: hold (could-not-check)" {
		t.Fatalf("absent file: %q", k.Line)
	}
}

// --- audit latches ---

func alEntry(verb, result, repo string, pr int, ts string) Entry {
	return Entry{Tool: AutoLaneToolName, Verb: verb, Result: result, Repo: repo, PR: &pr, TS: ts}
}

func TestAutoLane_MergesOnCountsOnly_TodaysOKMerges(t *testing.T) {
	day := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	entries := []Entry{
		alEntry(AutoLaneVerbMerge, ResultOK, alRepo, 1, "2026-03-04T01:00:00Z"),
		alEntry(AutoLaneVerbMerge, ResultOK, alRepo, 2, "2026-03-04T23:00:00Z"),
		alEntry(AutoLaneVerbMerge, ResultOK, alRepo, 3, "2026-03-03T23:59:59Z"),
		alEntry(AutoLaneVerbMerge, ResultRefused, alRepo, 4, "2026-03-04T02:00:00Z"),
		alEntry(AutoLaneVerbMerge, ResultDryRun, alRepo, 5, "2026-03-04T02:00:00Z"),
		alEntry(AutoLaneVerbMerge, ResultOK, "example-org/other", 6, "2026-03-04T02:00:00Z"),
		alEntry(AutoLaneVerbMerge, ResultOK, alRepo, 7, "garbled"),
	}
	if n := AutoLaneMergesOn(entries, alRepo, day); n != 3 {
		t.Fatalf("counted %d, want 3 (two today + one unparseable counted fail-closed)", n)
	}
}

func TestAutoLanePriorEjectionLatch(t *testing.T) {
	entries := []Entry{
		alEntry(AutoLaneVerbEject, ResultUnverifiable, alRepo, 9, "2026-03-04T01:00:00Z"),
		alEntry(AutoLaneVerbMerge, ResultRefused, alRepo, 10, "2026-03-04T01:00:00Z"),
	}
	if !AutoLanePriorEjection(entries, alRepo, 9) {
		t.Fatalf("an eject line of any result must latch")
	}
	if AutoLanePriorEjection(entries, alRepo, 10) || AutoLanePriorEjection(entries, "example-org/other", 9) {
		t.Fatalf("the latch leaked across PRs or repos")
	}
}

func TestAutoLaneLabelApplier(t *testing.T) {
	ev := []LabelEvent{
		{Name: AutoLaneLabel, AppliedBy: "someone"},
		{Name: AutoLaneLabel, AppliedBy: "someone", Removed: true},
		{Name: AutoLaneLabel, AppliedBy: "example-reviewer-app[bot]"},
	}
	if who, ok := AutoLaneLabelApplier([]string{AutoLaneLabel}, ev); !ok || who != "example-reviewer-app[bot]" {
		t.Fatalf("applier %q ok=%t", who, ok)
	}
	if _, ok := AutoLaneLabelApplier([]string{AutoLaneLabel}, ev[:2]); ok {
		t.Fatalf("a removed application still attributed")
	}
	if _, ok := AutoLaneLabelApplier(nil, ev); ok {
		t.Fatalf("attributed a label the PR does not carry")
	}
}

func TestGlobSamplesExpandDoubleStar(t *testing.T) {
	got := GlobSamples(".github/workflows/**")
	for _, want := range []string{".github/workflows", ".github/workflows/x", ".github/workflows/x/y"} {
		if !alHas(got, want) {
			t.Fatalf("samples %v missing %q", got, want)
		}
	}
}

func alHas(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// TestAutoLane_PendingChecks_NotDemotion — CI not yet decided is could-not-check, never the
// ci-nonsuccess demotion: a PR whose CI is still running must not be ejected one-way for it.
// Fail-first: at 92d2221 both fired ci-nonsuccess.
func TestAutoLane_PendingChecks_NotDemotion(t *testing.T) {
	for _, mut := range []func(*AutoLaneScoreInput){
		func(i *AutoLaneScoreInput) { i.Checks.CheckRuns[0].Status = "in_progress" },
		func(i *AutoLaneScoreInput) { i.Checks = &ChecksAtHead{} },
	} {
		in := cleanScoreIn()
		mut(&in)
		s := ScoreAutoLane(in)
		if alHas(s.Fired, SignalCINonsuccess) || !alHas(s.Unreadable, "checks") {
			t.Fatalf("fired %v unreadable %v — want checks unreadable, no ci-nonsuccess", s.Fired, s.Unreadable)
		}
	}
}

// TestAutoLaneAcceptance — the enactment gate's body test: the FIRST non-empty line is the
// enactment line typed bare, and no word from the negation lexicon appears anywhere.
func TestAutoLaneAcceptance(t *testing.T) {
	for _, ok := range []string{
		"Enact: R-8", "Enact: R-8\r\n", "\n\nEnact: R-8\n", "Enact: R-8  \n\nReviewed the narrowed text.",
	} {
		if got, why := AutoLaneAcceptance(ok); !got {
			t.Errorf("%q: not accepted (%s)", ok, why)
		}
	}
	for _, bad := range []string{
		"", "Accepted.", "R-8 accepted", "Enact: R-80", "Enact: R-8 please", "> Enact: R-8",
		"Rejected. R-8 is NOT accepted; do not enact the lane.",
		"Enact: R-8\nrejected", "Enact: R-8\nRevoked.", "Enact: R-8\nwithdrawn", "Enact: R-8\nNot accepted.",
	} {
		if got, _ := AutoLaneAcceptance(bad); got {
			t.Errorf("%q: accepted", bad)
		}
	}
}

// TestAutoLane_EnactLineMust_BeFirstAndBare — the Enact line must OPEN the body, typed bare.
// Fail-first: at 7cbc29f the matcher took the line anywhere in the body, case-folded, with
// leading whitespace, so every body below was accepted.
func TestAutoLane_EnactLineMust_BeFirstAndBare(t *testing.T) {
	for _, bad := range []string{
		"Reviewed the lane.\n\nEnact: R-8\n", // not the first non-empty line
		"To arm the lane a human would reply:\n\n```\nEnact: R-8\n```\n",
		"```\nEnact: R-8\n```",     // fenced
		"`Enact: R-8`",             // backticked
		">Enact: R-8",              // quoted
		"  Enact: R-8",             // indented
		"    Enact: R-8",           // an indented code block
		"enact: R-8", "ENACT: R-8", // not the exact spelling
		"Enact:R-8", "Enact:  R-8",
		"**Enact: R-8**",
	} {
		if got, _ := AutoLaneAcceptance(bad); got {
			t.Errorf("%q: accepted", bad)
		}
	}
}

// TestAutoLaneRulingText — the text a time check compares: R-8's heading and body above its
// Sign-off line. Filling or re-pointing the Sign-off line, or editing another ruling, leaves
// it unchanged; an edit above the line changes it.
func TestAutoLaneRulingText(t *testing.T) {
	const unsigned = "# Rulings\n\n## R-7 — other\n\nSeven.\n\n**Sign-off:**\n\n## R-8 — the lane\n\nNarrowed text.\n\n**Sign-off:**\n"
	base, ok := AutoLaneRulingText(unsigned, "R-8")
	if !ok || !strings.Contains(base, "Narrowed text.") || strings.Contains(base, "Sign-off") || strings.Contains(base, "Seven") {
		t.Fatalf("R-8 text = %q (found %t)", base, ok)
	}
	same := []string{
		strings.Replace(unsigned, "**Sign-off:**\n", "**Sign-off:** https://example.test/x\n", 2),
		strings.Replace(unsigned, "## R-8 — the lane\n\nNarrowed text.\n\n**Sign-off:**\n",
			"## R-8 — the lane\n\nNarrowed text.\n\n**Sign-off:**\nhttps://example.test/y\n", 1),
		strings.Replace(unsigned, "Seven.", "Seven, amended.", 1),
		unsigned + "\nA note below the sign-off.\n",
	}
	for _, v := range same {
		if got, _ := AutoLaneRulingText(v, "R-8"); got != base {
			t.Errorf("a change outside R-8's text above the Sign-off line moved it:\n%q\nvs\n%q", got, base)
		}
	}
	for _, v := range []string{
		strings.Replace(unsigned, "Narrowed text.", "Narrowed text, again.", 1),
		strings.Replace(unsigned, "## R-8 — the lane", "## R-8 — the auto lane", 1),
		strings.Replace(unsigned, "Narrowed text.\n", "Narrowed text. \n", 1),
	} {
		if got, _ := AutoLaneRulingText(v, "R-8"); got == base {
			t.Errorf("an edit to R-8's text above the Sign-off line did not change it: %q", v)
		}
	}
	if _, ok := AutoLaneRulingText("# Rulings\n\n## R-80 — not it\n\n**Sign-off:**\n", "R-8"); ok {
		t.Error("R-80's section read as R-8's")
	}
}

// TestAutoLane_SignOffThread_Config — the optional sign-off thread: absent loads with the thread
// UNSET (0); a positive integer loads; anything else refuses the lane.
func TestAutoLane_SignOffThread_Config(t *testing.T) {
	raw := alRaw(alRepo + ":docs/notes/**:ada")
	if ld := ParseAutoLaneConfig(raw, alValidator()); ld.State != AutoLaneConfigLoaded || ld.Config.SignOffThread != 0 {
		t.Fatalf("absent thread: %s thread %d — %s", ld.State, ld.Config.SignOffThread, ld.Problem)
	}
	raw[EnvAutoApproveSignOffThread] = "42"
	if ld := ParseAutoLaneConfig(raw, alValidator()); ld.State != AutoLaneConfigLoaded || ld.Config.SignOffThread != 42 {
		t.Fatalf("thread 42: %s thread %d — %s", ld.State, ld.Config.SignOffThread, ld.Problem)
	}
	for _, bad := range []string{"", "0", "-3", "#42", "42x", "example-org/tracker#42", "+42"} {
		raw[EnvAutoApproveSignOffThread] = bad
		if ld := ParseAutoLaneConfig(raw, alValidator()); ld.State != AutoLaneConfigRefused ||
			!strings.Contains(ld.Problem, EnvAutoApproveSignOffThread) {
			t.Errorf("thread %q: %s — want refused naming the key", bad, ld.State)
		}
	}
	only := map[string]string{EnvAutoApproveSignOffThread: "42"}
	if ld := ParseAutoLaneConfig(only, alValidator()); ld.State != AutoLaneUnconfigured {
		t.Fatalf("the thread alone opened the lane: %s", ld.State)
	}
}

// TestAutoLaneEjectedOnForge — the forge half of the latch: the marked comment, by the
// reviewer App only.
func TestAutoLaneEjectedOnForge(t *testing.T) {
	marked := Comment{Author: Account{Login: alReviewer}, Body: AutoLaneEjectMarker + "\nejected"}
	if !AutoLaneEjectedOnForge([]Comment{{Body: "hi"}, marked}, alReviewer) {
		t.Fatalf("the reviewer App's marked comment did not latch")
	}
	if AutoLaneEjectedOnForge([]Comment{marked}, "") {
		t.Fatalf("an unbound reviewer role matched")
	}
	other := marked
	other.Author.Login = "example-worker-app[bot]"
	if AutoLaneEjectedOnForge([]Comment{other, {Author: Account{Login: alReviewer}, Body: "no marker"}}, alReviewer) {
		t.Fatalf("an unmarked or foreign comment latched")
	}
}

// TestAutoLane_NeverAdmitPaths — the compiled never-admit set: refused at load where an area
// reaches it, and tripped per path at admit.
func TestAutoLane_NeverAdmitPaths(t *testing.T) {
	for _, glob := range []string{"docs/streams/issue-flow/**", "docs/streams/issue-flow/rulings.md", ".claude/**"} {
		c := AutoLaneConfig{Areas: []AutoLaneArea{{Repo: alRepo, Glob: glob, Login: "ada"}}}
		if p := AutoLaneAreaTripwires(c, alRepo, noRisk); !strings.Contains(p, "never-admit") {
			t.Fatalf("glob %q: problem %q — want a never-admit refusal", glob, p)
		}
	}
	for _, glob := range []string{"docs/notes/**", "docs/research/**/*.md", "BOARD.md"} {
		c := alConfig(t, alRepo+":"+glob+":ada")
		if p := AutoLaneAreaTripwires(c, alRepo, noRisk); p != "" {
			t.Fatalf("glob %q was refused: %q", glob, p)
		}
	}
	c := AutoLaneConfig{Areas: []AutoLaneArea{{Repo: alRepo, Glob: "docs/**", Login: "ada"}}}
	for _, f := range []string{"docs/CLAUDE.md", "docs/x/AGENTS.md", "docs/p/SKILL.md", "docs/.claude/settings.json",
		"docs/.mcp.json", "docs/.assay-surfaces", "docs/streams/x/rulings.md"} {
		in := admitIn()
		in.ChangedFiles = []string{f}
		if a := AdmitAutoLane(c, in); a.Admitted || !alHas(a.Tripwires, TripNeverAdmit) {
			t.Fatalf("%s: tripwires %v — want %s", f, a.Tripwires, TripNeverAdmit)
		}
	}
}
