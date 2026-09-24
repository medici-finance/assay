package main

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- inert by default -------------------------------------------------------------------

// TestAutoLaneShippedStateIsInert — the SHIPPED state: no ASSAY_AUTOAPPROVE_* key set. Every
// verb refuses at `config` before its first forge request, and nothing is written. Fail-first:
// a loader that defaulted any lane value (or a verb that read the forge before the config)
// reddens this.
func TestAutoLaneShippedStateIsInert(t *testing.T) {
	for _, args := range [][]string{
		{verbCheck},
		{verbCheck, "7"},
		{verbRecompute, "7"},
		{verbMerge, "7", "--dry-run"},
		{verbMerge, "7"},
	} {
		e := install(t, "", rulingsSigned)
		code, _, stderr := e.run(args...)
		if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: config") {
			t.Fatalf("%v: exit %d stderr %q — want exit 5 naming config", args, code, stderr)
		}
		if len(e.fg.calls) != 0 {
			t.Fatalf("%v: %d forge request(s) on a closed lane: %v", args, len(e.fg.calls), e.fg.calls)
		}
	}
}

// TestAutoLane_MergeRefuses_UnsignedRuling — every other condition true, R-8's Sign-off line
// EMPTY: `merge` refuses ruling-unsigned with ZERO forge writes. This is the enactment gate
// holding on its own with the score clean (Verify row 8).
func TestAutoLane_MergeRefuses_UnsignedRuling(t *testing.T) {
	for _, args := range [][]string{{verbMerge, "7"}, {verbMerge, "7", "--dry-run"}} {
		e := install(t, fixtureLaneKeys, rulingsUnsigned)
		code, _, stderr := e.run(append(args, "--fpy-file", e.fpy(healthyFPY))...)
		if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: ruling-unsigned") {
			t.Fatalf("%v: exit %d stderr %q — want exit 5 refused: ruling-unsigned", args, code, stderr)
		}
		if w := e.fg.writes(); len(w) != 0 {
			t.Fatalf("%v: %d write(s) on an unsigned lane: %v", args, len(w), w)
		}
	}
}

// TestAutoLane_MissingRulingIs_CouldNotCheck — a register with no R-8 section at all is
// could-not-check (exit 6), never signed and never unsigned.
func TestAutoLane_MissingRulingIs_CouldNotCheck(t *testing.T) {
	e := install(t, fixtureLaneKeys, "# Rulings\n\n## R-1 — other\n\n**Sign-off:**\n")
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "could-not-check: "+condRulingSigned) {
		t.Fatalf("exit %d stderr %q — want exit 6 could-not-check", code, stderr)
	}
}

// TestAutoLane_RulingSignedByNon_AuthorityRefuses — a signed line whose artifact is authored by
// a TRUSTED human who is not the blessing authority does not enact the lane.
func TestAutoLane_RulingSignedByNon_AuthorityRefuses(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[3] = []deskkit.Comment{{DatabaseID: 555, Author: deskkit.Account{Login: "shared-agent", ID: 2002, Type: "User"}, Body: fxEnactBody}}
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "not the configured blessing authority") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

// TestAutoLane_EjectUnenacted_WritesNothing — a PR that would be ejected, on an unsigned lane:
// the verb reports the ejection and writes NOTHING (no label, no comment, no latch).
func TestAutoLane_EjectUnenacted_WritesNothing(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsUnsigned)
	e.fg.pr.Labels = append(e.fg.pr.Labels, deskkit.SizeLabelPrefix+"L")
	code, _, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "NOT written") {
		t.Fatalf("exit %d stderr %q — want an unwritten ejection", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("%d write(s) on an unsigned lane: %v", len(w), w)
	}
	if e.hasAudit(deskkit.AutoLaneVerbEject, deskkit.ResultOK) {
		t.Fatalf("an unenacted ejection latched")
	}
}

// --- config refusals ----------------------------------------------------------------------

// TestAutoLane_ConfigRefuses_SurfaceOverlap — Verify row 3.
func TestAutoLane_ConfigRefuses_SurfaceOverlap(t *testing.T) {
	keys := strings.Replace(fixtureLaneKeys,
		"example-org/tracker:docs/notes/**:ada,example-org/tracker:BOARD.md:ada",
		"example-org/tracker:.github/workflows/**:ada", 1)
	e := install(t, keys, rulingsSigned)
	if err := os.WriteFile(filepath.Join(e.root, ".assay-surfaces"), []byte(".github/workflows/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := e.run(verbMerge, "7", "--dry-run")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: area overlaps declared surface") ||
		!strings.Contains(stderr, ".github/workflows/**:ada") {
		t.Fatalf("exit %d stderr %q — want exit 5 naming the overlapping entry", code, stderr)
	}
	if !e.hasAudit(deskkit.AutoLaneVerbConfig, deskkit.ResultRefused) {
		t.Fatalf("no autolane:config result=refused audit line")
	}
	if len(e.fg.calls) != 0 {
		t.Fatalf("%d forge request(s) on a refused config", len(e.fg.calls))
	}
}

// TestAutoLane_ConfigRefusesRisk_ClassedArea — the same glob with no local surfaces file is
// still refused, by the compiled risk triggers, before any forge request.
func TestAutoLane_ConfigRefusesRisk_ClassedArea(t *testing.T) {
	keys := strings.Replace(fixtureLaneKeys,
		"example-org/tracker:docs/notes/**:ada,example-org/tracker:BOARD.md:ada",
		"example-org/tracker:.github/workflows/**:ada", 1)
	e := install(t, keys, rulingsSigned)
	code, _, stderr := e.run(verbCheck, "7")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "risk-classed") || len(e.fg.calls) != 0 {
		t.Fatalf("exit %d stderr %q calls %d", code, stderr, len(e.fg.calls))
	}
}

// TestAutoLane_ConfigRefuses_UntrustedOptIn — Verify row 4.
func TestAutoLane_ConfigRefuses_UntrustedOptIn(t *testing.T) {
	keys := strings.Replace(fixtureLaneKeys, "BOARD.md:ada", "BOARD.md:mallory", 1)
	e := install(t, keys, rulingsSigned)
	code, _, stderr := e.run(verbMerge, "7", "--dry-run")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: opt-in login not trusted") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if len(e.fg.calls) != 0 {
		t.Fatalf("%d forge request(s) on a refused config", len(e.fg.calls))
	}
}

// TestAutoLane_ConfigRefuses_PublicRepoArea — a public repo risk-classes every path, so no area
// on one can load.
func TestAutoLane_ConfigRefuses_PublicRepoArea(t *testing.T) {
	keys := strings.Replace(fixtureLaneKeys, "example-org/tracker:BOARD.md:ada", "example-org/open:BOARD.md:ada", 1)
	e := install(t, keys, rulingsSigned)
	code, _, stderr := e.run(verbCheck)
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "risk-classed") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

// TestAutoLane_BaseBranchWithout_SurfacesRefuses — the base branch declares no
// `.assay-surfaces`: the surface tier is absent, and the lane admits nothing there.
func TestAutoLane_BaseBranchWithout_SurfacesRefuses(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.noSurfaces = true
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "declares no surfaces") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
}

// --- ejection -----------------------------------------------------------------------------

func assertEjected(t *testing.T, e *env, code int, stderr, signal string) {
	t.Helper()
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "eject: ") || !strings.Contains(stderr, signal) {
		t.Fatalf("exit %d stderr %q — want exit 5 eject naming %s", code, stderr, signal)
	}
	var labels, comments int
	for _, w := range e.fg.writes() {
		switch w.Op {
		case "ApplyLabels":
			labels++
			if !strings.Contains(w.Body, "remove="+deskkit.AutoLaneLabel) || !strings.Contains(w.Body, "add="+labelAfterFlip) {
				t.Fatalf("label swap %q — want auto-lane removed and %s added", w.Body, labelAfterFlip)
			}
		case "PostComment":
			comments++
			if !strings.Contains(w.Body, deskkit.AutoLaneEjectMarker) || !strings.Contains(w.Body, signal) {
				t.Fatalf("ejection comment %q lacks the marker or the signal", w.Body)
			}
		}
	}
	if labels != 1 || comments != 1 {
		t.Fatalf("writes %v — want exactly one label swap and one comment", e.fg.writes())
	}
	if !e.hasAudit(deskkit.AutoLaneVerbEject, deskkit.ResultOK) {
		t.Fatalf("no autolane:eject result=ok audit line (the latch)")
	}
}

// TestAutoLane_MergeEjectsOn_SupersededChanges_Requested — Verify row 5: category admit
// satisfied, APPROVED at head, all green, R-8 resolving — and ONE earlier CHANGES_REQUESTED
// superseded by the APPROVED. The score ejects; nothing merges. Fail-first: a score that
// read a latest-per-reviewer view would call this clean.
func TestAutoLane_MergeEjectsOn_SupersededChanges_Requested(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.reviews = []deskkit.Review{
		{ID: 1, Author: deskkit.Account{Login: fxReviewer}, State: "CHANGES_REQUESTED", CommitID: fxOldHead},
		{ID: 2, Author: deskkit.Account{Login: fxReviewer}, State: "APPROVED", CommitID: fxHead},
	}
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", e.fpy(healthyFPY))
	assertEjected(t, e, code, stderr, deskkit.SignalReviewRework)
}

// TestAutoLane_MergeEjectsOnPath_OutsideArea — Verify row 6: reviews clean, ONE changed path
// outside the opted-in globs.
func TestAutoLane_MergeEjectsOnPath_OutsideArea(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.files = append(e.fg.files, deskkit.ChangedFile{Filename: "README.md", Status: "modified"})
	e.fg.pr.ChangedFiles = 3
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", e.fpy(healthyFPY))
	assertEjected(t, e, code, stderr, deskkit.TripPathOutsideArea)
}

// TestAutoLane_MergeEjectsOn_StreamBriefFile — a stream brief file in the diff ejects, even
// though it is a docs path.
func TestAutoLane_MergeEjectsOn_StreamBriefFile(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.files = append(e.fg.files, deskkit.ChangedFile{Filename: "docs/streams/example/brief-04-thing.md", Status: "modified"})
	e.fg.pr.ChangedFiles = 3
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", e.fpy(healthyFPY))
	assertEjected(t, e, code, stderr, deskkit.TripStreamBrief)
}

// TestAutoLane_MergeEjectsOver_EjectLine — one fired signal (size:L) is a score of 1 over the
// line 0: eject.
func TestAutoLane_MergeEjectsOver_EjectLine(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.pr.Labels = append(e.fg.pr.Labels, deskkit.SizeLabelPrefix+"L")
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", e.fpy(healthyFPY))
	assertEjected(t, e, code, stderr, deskkit.SignalSizeLarge)
}

// TestAutoLane_EjectionCommentIs_Idempotent — a second ejection run does not post a second
// comment.
func TestAutoLane_EjectionCommentIs_Idempotent(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.pr.Labels = append(e.fg.pr.Labels, deskkit.SizeLabelPrefix+"L")
	e.run(verbRecompute, "7")
	e.run(verbRecompute, "7")
	n := 0
	for _, w := range e.fg.writes() {
		if w.Op == "PostComment" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d ejection comments across two runs, want 1", n)
	}
}

// TestAutoLane_MergeRefuses_UnreadableChecks — Verify row 7: the checks read fails. That is
// could-not-check (exit 6), audited `unwritten`, and NOTHING is written — a transient read
// failure is not latched as a one-way ejection.
func TestAutoLane_MergeRefuses_UnreadableChecks(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.fail["ChecksAtHead"] = true
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "could-not-check: checks") {
		t.Fatalf("exit %d stderr %q — want exit 6 could-not-check: checks", code, stderr)
	}
	if !e.hasAudit(deskkit.AutoLaneVerbMerge, deskkit.ResultUnwritten) {
		t.Fatalf("no autolane:merge result=unwritten audit line")
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes on an unreadable input: %v", w)
	}
}

// --- lane-armed ---------------------------------------------------------------------------

// TestAutoLaneHoldsBelowFPYFloor — Verify row 9: n=12, fpy 0.75 under floor 0.90 — the lane
// holds. Sibling: the file absent is a hold (could-not-check), never healthy. Fail-first: a
// kill signal that read an absent file as healthy, or compared n alone, stays armed.
func TestAutoLaneHoldsBelowFPYFloor(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	code, stdout, _ := e.run(verbCheck, "--fpy-file", e.fpy(`{"auto-lane":{"n":12,"firstPassYield":0.75}}`))
	if code != deskkit.ExitRefused || !strings.Contains(stdout, "lane: hold (fpy 0.75 < floor 0.90, n=12)") {
		t.Fatalf("exit %d stdout %q", code, stdout)
	}
	t.Run("absent file", func(t *testing.T) {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		code, stdout, _ := e.run(verbCheck)
		if code != deskkit.ExitRefused || !strings.Contains(stdout, "lane: hold (could-not-check)") {
			t.Fatalf("exit %d stdout %q", code, stdout)
		}
	})
	t.Run("merge refuses on hold", func(t *testing.T) {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(`{"auto-lane":{"n":12,"firstPassYield":0.75}}`))
		if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condLaneArmed) {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
	})
}

// TestAutoLaneRefusesAtDailyCap — Verify row 10: the audit log carries cap-many
// `autolane:merge result=ok` lines for today; the merge refuses daily-cap. Fail-first: a
// cap compared with > rather than >= lets one more through.
func TestAutoLaneRefusesAtDailyCap(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	one, two := 1, 2
	e.seedAudit(
		deskkit.Entry{Tool: toolName, Verb: deskkit.AutoLaneVerbMerge, Result: deskkit.ResultOK, Repo: fxRepo, PR: &one},
		deskkit.Entry{Tool: toolName, Verb: deskkit.AutoLaneVerbMerge, Result: deskkit.ResultOK, Repo: fxRepo, PR: &two},
	)
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: daily-cap") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	t.Run("one under the cap passes", func(t *testing.T) {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		e.seedAudit(deskkit.Entry{Tool: toolName, Verb: deskkit.AutoLaneVerbMerge, Result: deskkit.ResultOK, Repo: fxRepo, PR: &one})
		if code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY)); code != deskkit.ExitOK {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
	})
}

// --- the merge step's own re-read --------------------------------------------------------

// TestAutoLane_MergeDryRun_HappyPath — Verify row 11, and the admit-within-scope fixture: all
// clean, R-8 resolving. Every condition prints OK in the pinned order and the run ends with
// the would-merge line; ZERO writes.
func TestAutoLane_MergeDryRun_HappyPath(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	code, stdout, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	last := -1
	for _, c := range mergeConditions {
		i := strings.Index(stdout, c+" OK")
		if i < 0 || i < last {
			t.Fatalf("condition %s OK missing or out of order in\n%s", c, stdout)
		}
		last = i
	}
	want := "dry-run: would merge " + fxHead + " into main (merge commit)\n"
	if !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("a dry run wrote: %v", w)
	}
	if !e.hasAudit(deskkit.AutoLaneVerbMerge, deskkit.ResultDryRun) {
		t.Fatalf("no autolane:merge result=dryrun audit line")
	}
}

// TestAutoLane_MergeWithoutDryRun_RefusesMergeWrite — this release carries no merge mutation:
// with every condition true and the lane enacted, `merge` still refuses and writes nothing.
func TestAutoLane_MergeWithoutDryRun_RefusesMergeWrite(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: merge-write") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
	if e.hasAudit(deskkit.AutoLaneVerbMerge, deskkit.ResultOK) {
		t.Fatalf("an autolane:merge result=ok line was written — the daily cap would count it")
	}
}

// TestAutoLane_MergeRefusesPrior_EjectionAfter_Readmission — Verify row 17: eject on
// ci-nonsuccess, then clear every head-scoped signal at a NEW head (CI green, approval at the
// new head, the label back on under the reviewer App). The category re-derives clean and the
// score reads 0 — and the merge STILL refuses: the latch, not the recomputed score, decides.
func TestAutoLane_MergeRefusesPrior_EjectionAfter_Readmission(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	fpy := e.fpy(healthyFPY)
	e.fg.checks.CheckRuns[0].Conclusion = "failure"
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", fpy)
	assertEjected(t, e, code, stderr, deskkit.SignalCINonsuccess)

	const newHead = "22222222222222222222"
	fresh := greenForge()
	fresh.pr.HeadSHA = newHead
	fresh.reviews = []deskkit.Review{{ID: 3, Author: deskkit.Account{Login: fxReviewer}, State: "APPROVED", CommitID: newHead}}
	e.fg = fresh
	code, _, stderr = e.run(verbMerge, "7", "--dry-run", "--fpy-file", fpy)
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condPriorEjection) {
		t.Fatalf("exit %d stderr %q — want refused: prior-ejection", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes after the latch: %v", w)
	}
	// recompute never re-admits it either.
	notYetInLane(e)
	if code, _, stderr := e.run(verbRecompute, "7"); code != deskkit.ExitRefused || !strings.Contains(stderr, condPriorEjection) {
		t.Fatalf("recompute on an ejected PR: exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("recompute re-admitted an ejected PR: %v", w)
	}
}

// TestAutoLane_MergeRefusesStale_Approval — Verify row 18: the category admits and the score
// is 0, but the reviewer App's APPROVED is at an older commit.
func TestAutoLane_MergeRefusesStale_Approval(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.reviews = []deskkit.Review{{ID: 1, Author: deskkit.Account{Login: fxReviewer}, State: "APPROVED", CommitID: fxOldHead}}
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condReviewerApproved) {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
}

// TestAutoLane_MergeRefuses_ReadableRedCheck — Verify row 19: the score read CI green, and the
// merge step's own re-read finds a READABLE failure.
func TestAutoLane_MergeRefuses_ReadableRedCheck(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.checks2 = &deskkit.ChecksAtHead{CheckRunsTotalCount: 1, CheckRuns: []deskkit.CheckRun{
		{Name: "test", Status: "completed", Conclusion: "failure", CompletedAt: "2026-01-01T00:09:00Z"}}}
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condChecksGreen) {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
}

// TestAutoLane_MergeRefuses_MissingRequired_Check — a branch-protection-required context that
// never reported is could-not-check, never a pass.
func TestAutoLane_MergeRefuses_MissingRequired_Check(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.required = []string{"leak-sweep"}
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "leak-sweep") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

// TestAutoLane_CINonsuccess_IgnoresSuperseded_Reruns — Verify row 20: five runs of one check name
// at head, the earliest four cancelled/failed and the LAST green — the lane reads CI green.
func TestAutoLane_CINonsuccess_IgnoresSuperseded_Reruns(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.checks = &deskkit.ChecksAtHead{CheckRunsTotalCount: 5, CheckRuns: []deskkit.CheckRun{
		{Name: "prompt", Status: "completed", Conclusion: "cancelled", CompletedAt: "2026-01-01T00:01:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "failure", CompletedAt: "2026-01-01T00:02:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "cancelled", CompletedAt: "2026-01-01T00:03:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "failure", CompletedAt: "2026-01-01T00:04:00Z"},
		{Name: "prompt", Status: "completed", Conclusion: "success", CompletedAt: "2026-01-01T00:05:00Z"},
	}}
	code, stdout, stderr := e.run(verbCheck, "7", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitOK || !strings.Contains(stdout, "score: 0 (line 0; fired: none)") {
		t.Fatalf("exit %d stdout %q stderr %q — want ci-nonsuccess 0", code, stdout, stderr)
	}
}

// TestAutoLane_ForeignLabelIsNot_Admitted — the admission label applied by the worker itself
// reads as NOT admitted.
func TestAutoLane_ForeignLabelIsNot_Admitted(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.events[2].AppliedBy = fxWorker
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "not the reviewer App") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
}

// TestAutoLane_MergeRefusesMoved_Head — the head moves between the first read and the final
// re-read.
func TestAutoLane_MergeRefusesMoved_Head(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.head2 = "33333333333333333333"
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condHeadStable) {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestAutoLane_WrongCallerRole_Refuses(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	t.Setenv("DESK_LOOP", "worker-desk")
	code, _, stderr := e.run(verbMerge, "7", "--dry-run")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condCallerRole) || len(e.fg.calls) != 0 {
		t.Fatalf("exit %d stderr %q calls %d", code, stderr, len(e.fg.calls))
	}
}

// --- recompute: admission ----------------------------------------------------------------

// notYetInLane removes the admission label and its timeline event, leaving the rest.
func notYetInLane(e *env) {
	var labels []string
	for _, l := range e.fg.pr.Labels {
		if l != deskkit.AutoLaneLabel {
			labels = append(labels, l)
		}
	}
	e.fg.pr.Labels = labels
	var events []deskkit.LabelEvent
	for _, ev := range e.fg.events {
		if ev.Name != deskkit.AutoLaneLabel {
			events = append(events, ev)
		}
	}
	e.fg.events = events
}

func TestAutoLane_RecomputeAdmits_WhenEnacted(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	notYetInLane(e)
	code, stdout, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitOK || !strings.Contains(stdout, "admitted:") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	w := e.fg.writes()
	if len(w) != 1 || w[0].Op != "ApplyLabels" || !strings.Contains(w[0].Body, "add="+deskkit.AutoLaneLabel) {
		t.Fatalf("writes %v — want exactly the admission label", w)
	}
}

func TestAutoLane_RecomputeDoesNot_AdmitUnenacted(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsUnsigned)
	notYetInLane(e)
	code, _, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "NOT written") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("an unenacted lane admitted: %v", w)
	}
}

// --- pins ---------------------------------------------------------------------------------

// TestAutoLaneMergeConditionOrder pins the ordered condition chain.
func TestAutoLaneMergeConditionOrder(t *testing.T) {
	want := []string{"caller-role", "config", "app-token", "pr-open-ready", "prior-ejection", "area-admit",
		"score", "reviewer-approved", "checks-green", "mergeable", "lane-armed", "ruling-signed", "head-stable"}
	if !reflect.DeepEqual(mergeConditions, want) {
		t.Fatalf("mergeConditions = %v\nwant %v", mergeConditions, want)
	}
}

// TestFixtureRosterFileMatches holds testdata/roster.env to the harness's fixture strings, so
// the file the brief names as the fixture is the one the tests actually load.
func TestFixtureRosterFileMatches(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "roster.env"))
	if err != nil {
		t.Fatal(err)
	}
	var kv []string
	for _, ln := range strings.Split(string(b), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" && !strings.HasPrefix(ln, "#") {
			kv = append(kv, ln)
		}
	}
	got := strings.Join(kv, "\n") + "\n"
	want := fixtureRosterBase + fixtureLaneKeys
	if got != want {
		t.Fatalf("testdata/roster.env drifted from the harness fixture:\n%s", got)
	}
}

// --- review round 1: the enactment gate reads an ACCEPTANCE, from the forge ------------------

// assertNotEnacted runs an admission recompute on a PR not yet in the lane and requires the
// admission to be REFUSED unwritten, naming want.
func assertNotEnacted(t *testing.T, e *env, want string) {
	t.Helper()
	notYetInLane(e)
	code, _, stderr := e.run(verbRecompute, "7")
	if code == deskkit.ExitOK || !strings.Contains(stderr, "NOT written") || !strings.Contains(stderr, want) {
		t.Fatalf("exit %d stderr %q — want an unwritten admission naming %q", code, stderr, want)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("the lane wrote on a non-acceptance: %v", w)
	}
}

// TestAutoLane_EnactRefuses_RejectionBody — the blessing authority's comment on the Sign-off
// line RECORDS A REJECTION: nothing is enacted. Fail-first: the gate at 92d2221 checked the
// author only, and this admitted (exit 0, add=auto-lane).
func TestAutoLane_EnactRefuses_RejectionBody(t *testing.T) {
	for _, body := range []string{
		"Rejected. R-8 is NOT accepted; do not enact the lane.",
		"Enact: R-8\n\nOn reflection: rejected.",
	} {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		e.fg.comments[3][0].Body = body
		assertNotEnacted(t, e, "not an acceptance")
	}
}

// TestAutoLane_EnactRefuses_BodyWithoutRuling — "Accepted." does not name R-8, and a line that
// mentions the ruling in prose is not the enactment line.
func TestAutoLane_EnactRefuses_BodyWithoutRuling(t *testing.T) {
	for _, body := range []string{"Accepted.", "thanks, typo fixed", "I think Enact: R-8 is fine"} {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		e.fg.comments[3][0].Body = body
		assertNotEnacted(t, e, "not an acceptance")
	}
}

// TestAutoLane_EnactRefuses_NonUserAuthor — an artifact whose author the forge types as a Bot
// is refused even at the pinned login and id; an untyped author is could-not-check.
func TestAutoLane_EnactRefuses_NonUserAuthor(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[3][0].Author.Type = "Bot"
	assertNotEnacted(t, e, "type Bot")

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[3][0].Author.Type = ""
	assertNotEnacted(t, e, "no author type")
}

// TestAutoLane_EnactRefuses_ThreadOutside_RegisterRepo — the Sign-off names an authority
// comment on a thread in ANOTHER repo: refused before the thread is even fetched.
func TestAutoLane_EnactRefuses_ThreadOutside_RegisterRepo(t *testing.T) {
	url := "https://github.com/example-org/open/issues/99#issuecomment-777"
	e := install(t, fixtureLaneKeys, strings.Replace(rulingsSigned, fxSignURL, url, 1))
	e.fg.comments[99] = []deskkit.Comment{{DatabaseID: 777,
		Author: deskkit.Account{Login: "ada", ID: 2001, Type: "User"}, Body: fxEnactBody}}
	assertNotEnacted(t, e, "not in the register's own repo")
	for _, c := range e.fg.calls {
		if c.Op == "ListCommentsTyped" {
			t.Fatalf("the gate fetched a thread outside the register repo: %v", c)
		}
	}
}

// TestAutoLane_EnactIgnores_LocalRegister — the caller's tree (a PR-head checkout) carries a
// SIGNED register, the default branch's copy is UNSIGNED: the lane is not enacted, and the
// register is read at the default branch through the forge. Fail-first: the gate at 92d2221
// read <root>/docs/streams/issue-flow/rulings.md and admitted.
func TestAutoLane_EnactIgnores_LocalRegister(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsUnsigned)
	e.localRulings(rulingsSigned)
	assertNotEnacted(t, e, "ruling-unsigned")
	read := false
	for _, c := range e.fg.calls {
		if c.Op == "ReadFile" && c.Arg == fxRepo+":"+deskkit.AutoLaneRulingsPath+"@main" {
			read = true
		}
	}
	if !read {
		t.Fatalf("the register was not read through the forge at the default branch: %v", e.fg.calls)
	}
}

// TestAutoLane_RulingsFlagRefuses_OutsideNeverAdmit — a register path a lane area could reach
// is refused before any read.
func TestAutoLane_RulingsFlagRefuses_OutsideNeverAdmit(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	code, _, stderr := e.run(verbCheck, "--rulings", "docs/notes/rulings.md")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "never-admit") || len(e.fg.calls) != 0 {
		t.Fatalf("exit %d stderr %q calls %d", code, stderr, len(e.fg.calls))
	}
}

// --- review round 1: the ejection latch is on the forge too ----------------------------------

// TestAutoLane_ForgeEjectMarker_Latches — a FRESH HOME (empty audit log) and a PR whose thread
// carries the reviewer App's ejection comment: recompute does not re-admit it, merge refuses
// prior-ejection, check reports it. Fail-first: at 92d2221 recompute re-admitted (exit 0).
func TestAutoLane_ForgeEjectMarker_Latches(t *testing.T) {
	marker := []deskkit.Comment{{Author: deskkit.Account{Login: fxReviewer},
		Body: deskkit.AutoLaneEjectComment([]string{deskkit.SignalCINonsuccess}, fxOldHead)}}

	e := install(t, fixtureLaneKeys, rulingsSigned)
	notYetInLane(e)
	e.fg.comments[fxPR] = marker
	code, _, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, condPriorEjection) {
		t.Fatalf("recompute: exit %d stderr %q — want refused: prior-ejection", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("recompute re-admitted a forge-ejected PR: %v", w)
	}

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[fxPR] = marker
	code, _, stderr = e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "refused: "+condPriorEjection) {
		t.Fatalf("merge: exit %d stderr %q — want refused: prior-ejection", code, stderr)
	}

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[fxPR] = marker
	if _, stdout, _ := e.run(verbCheck, "7", "--fpy-file", e.fpy(healthyFPY)); !strings.Contains(stdout, "prior-ejection: true") {
		t.Fatalf("check did not report the forge-side latch:\n%s", stdout)
	}
}

// TestAutoLane_UnreadableThreadIs_CouldNotCheck — the PR's comments cannot be read and the
// audit log records nothing: prior-ejection is could-not-check, never "not ejected".
func TestAutoLane_UnreadableThreadIs_CouldNotCheck(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.fail["ListComments"] = true
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "could-not-check: "+condPriorEjection) {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

// --- review round 1: a dry run writes nothing ------------------------------------------------

// TestAutoLane_MergeDryRun_WouldEject_WritesNothing — an enacted lane, a PR that fails the
// score: `merge --dry-run` reports the ejection and performs NONE of it — no label, no comment,
// no latch — so a second dry run is not refused prior-ejection. Fail-first: at 92d2221 the dry
// run swapped the labels, posted the comment and latched.
func TestAutoLane_MergeDryRun_WouldEject_WritesNothing(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.pr.Labels = append(e.fg.pr.Labels, deskkit.SizeLabelPrefix+"L")
	fpy := e.fpy(healthyFPY)
	for i := 0; i < 2; i++ {
		code, stdout, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", fpy)
		if code != deskkit.ExitRefused || !strings.Contains(stdout, "dry-run: would eject: "+deskkit.SignalSizeLarge) {
			t.Fatalf("run %d: exit %d stdout %q stderr %q — want a would-eject", i, code, stdout, stderr)
		}
		if strings.Contains(stderr, condPriorEjection) {
			t.Fatalf("run %d: a dry run latched the PR: %q", i, stderr)
		}
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("a dry run wrote: %v", w)
	}
	for _, a := range e.audit() {
		if a.Verb == deskkit.AutoLaneVerbEject {
			t.Fatalf("a dry run wrote the ejection latch: %+v", a)
		}
	}
}

// --- review round 1: base, never-admit, size, pending ---------------------------------------

// TestAutoLane_NonDefaultBaseRefuses — a PR against a branch other than the default branch is
// not in the lane, whatever that branch declares; an unresolvable default branch is
// could-not-check.
func TestAutoLane_NonDefaultBaseRefuses(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.pr.BaseRef = "author-branch"
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "not the default branch") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, c := range e.fg.calls {
		if c.Op == "ReadFile" && strings.Contains(c.Arg, "@author-branch") {
			t.Fatalf("an input was read at the author-chosen base: %v", c)
		}
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.fail["RepoHardeningRead"] = true
	code, _, stderr = e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("unresolvable default branch: exit %d stderr %q — want 6", code, stderr)
	}
}

// TestAutoLane_ConfigRefuses_RegisterReachingArea — an area over the register's directory is
// refused at load, before any forge request. Fail-first: at 92d2221 it loaded and a PR
// editing the register was admitted.
func TestAutoLane_ConfigRefuses_RegisterReachingArea(t *testing.T) {
	keys := strings.Replace(fixtureLaneKeys, "example-org/tracker:docs/notes/**:ada",
		"example-org/tracker:"+path.Dir(deskkit.AutoLaneRulingsPath)+"/**:ada", 1)
	e := install(t, keys, rulingsSigned)
	code, _, stderr := e.run(verbCheck, "7")
	if code != deskkit.ExitRefused || !strings.Contains(stderr, "never-admit") || len(e.fg.calls) != 0 {
		t.Fatalf("exit %d stderr %q calls %d", code, stderr, len(e.fg.calls))
	}
}

// TestAutoLane_InstructionFileEjects — an agent-instruction file inside an opted-in area trips
// the category at admit.
func TestAutoLane_InstructionFileEjects(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.files = append(e.fg.files, deskkit.ChangedFile{Filename: "docs/notes/CLAUDE.md", Status: "added"})
	e.fg.pr.ChangedFiles = 3
	code, _, stderr := e.run(verbMerge, "7", "--fpy-file", e.fpy(healthyFPY))
	assertEjected(t, e, code, stderr, deskkit.TripNeverAdmit)
}

// TestAutoLane_SizeLabelUntrusted_CouldNotCheck — the size label absent (the labeler has not
// run), or set by the PR's author: could-not-check, never "not large", and never latched.
func TestAutoLane_SizeLabelUntrusted_CouldNotCheck(t *testing.T) {
	strip := func(e *env) {
		var labels []string
		for _, l := range e.fg.pr.Labels {
			if l != fxSizeS {
				labels = append(labels, l)
			}
		}
		e.fg.pr.Labels = labels
	}
	e := install(t, fixtureLaneKeys, rulingsSigned)
	strip(e)
	code, _, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitUnverifiable || len(e.fg.writes()) != 0 {
		t.Fatalf("absent size label: exit %d stderr %q writes %v — want 6, unwritten", code, stderr, e.fg.writes())
	}

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.events = append(e.fg.events, deskkit.LabelEvent{Name: fxSizeS, AppliedBy: fxWorker})
	code, _, stderr = e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitUnverifiable || len(e.fg.writes()) != 0 {
		t.Fatalf("author-applied size label: exit %d stderr %q — want 6", code, stderr)
	}
}

// TestAutoLane_PendingChecksAre_CouldNotCheck — CI still running on an in-lane PR is
// could-not-check at recompute (exit 6, nothing written, no latch), never a one-way ejection.
func TestAutoLane_PendingChecksAre_CouldNotCheck(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.checks.CheckRuns[0].Status = "in_progress"
	code, _, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "could-not-check: checks") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("pending CI ejected: %v", w)
	}
}
