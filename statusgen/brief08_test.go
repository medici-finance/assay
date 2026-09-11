package main

// brief08_test.go — the Verify-row controls for forge-neutral/08: statusgen's
// init CI scaffold, its model-path auto-flip, and its dead-claim decay are
// forge-aware and HONEST — a check that cannot serve this forge says so distinctly
// rather than degrading to a GitHub-shaped default or a silent pass.
//
// The controls that matter most are the NEGATIVE ones:
//   - TestInitUnresolvedForgeWritesNoCIHalf — an unresolved forge writes NO CI half
//     rather than defaulting to GitHub (the shape forge-neutral/01 forbids).
//   - TestAutoFlipUncorroboratedStaysVerified — an approval not tied to the head
//     leaves the row `verified` on BOTH forges, so a flip "made to work on GitLab"
//     by accepting the bare approval flag is caught.
//   - TestAutoFlipReviewerLoginFromRoster — the reviewer identity is DERIVED from
//     the forge-qualified roster entry, so no GitLab account is looked for under a
//     literal `[bot]` suffix.
//   - TestClaimDecayThreeStates — ran / failed-this-run / not-applicable are three
//     distinct messages, so a permanent not-applicable never reads as a transient.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ---- init: the CI half is chosen from the forge, and refusal is a real state -------

// TestInitScaffoldsGitLabCIHalf (Verify row 2). A GitLab forge writes .gitlab-ci.yml
// and NOT the (inert on GitLab) GitHub workflow — only the matching half.
func TestInitScaffoldsGitLabCIHalf(t *testing.T) {
	dir := t.TempDir()
	if code := runInitForge(dir, forgeGitLab, true, "", false); code != 0 {
		t.Fatalf("runInitForge(gitlab) exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); err != nil {
		t.Errorf("gitlab forge must scaffold .gitlab-ci.yml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(".github/workflows/assay-statusgen.yml"))); !os.IsNotExist(err) {
		t.Errorf("gitlab forge must NOT scaffold the inert GitHub workflow (err=%v)", err)
	}
}

// TestInitScaffoldsGitHubCIHalfUnchanged (Verify row 3). The GitHub path writes
// exactly the ten paths it writes today — a regression here would break every
// existing adopter. The `  created  ` lines are the authority for "exactly ten".
func TestInitScaffoldsGitHubCIHalfUnchanged(t *testing.T) {
	dir := t.TempDir()
	stream := initStreamName(dir)
	out := captureStdout(t, func() {
		if code := runInitForge(dir, forgeGitHub, true, "", false); code != 0 {
			t.Fatalf("runInitForge(github) exit = %d, want 0", code)
		}
	})
	for _, rel := range []string{
		"docs/streams/README.md",
		"docs/streams/FINDINGS.md",
		"docs/streams/INTAKE.md",
		"docs/streams/RETRO.md",
		"docs/streams/" + stream + "/README.md",
		"docs/streams/" + stream + "/brief-01-first-brief.md",
		".assay-versions",
		".github/workflows/assay-statusgen.yml",
		"CLAUDE.md",
		"AGENTS.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("github scaffold missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); !os.IsNotExist(err) {
		t.Errorf("github scaffold must NOT write .gitlab-ci.yml")
	}
	if n := strings.Count(out, "  created  "); n != 10 {
		t.Errorf("github scaffold created %d paths, want exactly 10:\n%s", n, out)
	}
}

// TestInitUnresolvedForgeWritesNoCIHalf (Verify row 4 — NEGATIVE). A readable origin
// remote whose host names neither forge writes NO CI file and the output says why;
// the test fails if the GitHub workflow appears by default (the exact shape
// forge-neutral/01 forbids defaulting to).
func TestInitUnresolvedForgeWritesNoCIHalf(t *testing.T) {
	dir := t.TempDir()
	out := captureStdout(t, func() {
		// remotePresent=true with an unrecognised host — the unresolved case.
		if code := runInitForge(dir, forgeUnknown, true, "git.example.com", false); code != 0 {
			t.Fatalf("runInitForge(unknown, remote present) exit = %d, want 0", code)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(".github/workflows/assay-statusgen.yml"))); !os.IsNotExist(err) {
		t.Errorf("an unresolved forge must NOT default to the GitHub workflow (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); !os.IsNotExist(err) {
		t.Errorf("an unresolved forge must NOT write .gitlab-ci.yml (err=%v)", err)
	}
	// The non-CI scaffold is still laid down — only the CI half is withheld.
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Errorf("the non-CI scaffold must still be written: %v", err)
	}
	if n := strings.Count(out, "  created  "); n != 9 {
		t.Errorf("an unresolved forge must create 9 paths (no CI half), got %d:\n%s", n, out)
	}
	// The output says WHY, names the host it could not classify, and points at the
	// two resolutions (origin remote / --forge) rather than advice about a file that
	// does not exist.
	for _, want := range []string{"NO CI half", "git.example.com", "--forge"} {
		if !strings.Contains(out, want) {
			t.Errorf("the no-CI-half output must contain %q; got:\n%s", want, out)
		}
	}
}

// TestInitNextStepsNamesWrittenFile (Verify row 5). The closing text names the file
// actually created, asserted for both forges — a scaffold that tells a GitLab
// adopter to commit a GitHub workflow is advice that cannot be followed (#349).
func TestInitNextStepsNamesWrittenFile(t *testing.T) {
	ghNext := captureStdout(t, func() {
		if code := runInitForge(t.TempDir(), forgeGitHub, true, "", false); code != 0 {
			t.Fatalf("github init exit = %d", code)
		}
	})
	if !strings.Contains(ghNext, ".github/workflows/assay-statusgen.yml") {
		t.Errorf("github next-steps must name the workflow it wrote:\n%s", ghNext)
	}
	if strings.Contains(ghNext, ".gitlab-ci.yml") {
		t.Errorf("github next-steps must not name the gitlab file it did not write:\n%s", ghNext)
	}

	glNext := captureStdout(t, func() {
		if code := runInitForge(t.TempDir(), forgeGitLab, true, "", false); code != 0 {
			t.Fatalf("gitlab init exit = %d", code)
		}
	})
	if !strings.Contains(glNext, ".gitlab-ci.yml") {
		t.Errorf("gitlab next-steps must name the .gitlab-ci.yml it wrote:\n%s", glNext)
	}
	if strings.Contains(glNext, ".github/workflows/assay-statusgen.yml") {
		t.Errorf("gitlab next-steps must not name the github workflow it did not write:\n%s", glNext)
	}
}

// TestInitGitLabPipelineIsTwoHalves (Verify row 10). The generated .gitlab-ci.yml
// carries a lint job on a change AND a board-regeneration job on the default branch,
// committing from ONE writer identity — so a GitLab adopter's board has a single
// writer, exactly as the GitHub half gives a GitHub adopter.
func TestInitGitLabPipelineIsTwoHalves(t *testing.T) {
	dir := t.TempDir()
	if code := runInitForge(dir, forgeGitLab, true, "", false); code != 0 {
		t.Fatalf("runInitForge(gitlab) exit = %d, want 0", code)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatalf("read .gitlab-ci.yml: %v", err)
	}
	gl := string(raw)
	// Half 1 — lint on a change (a merge-request pipeline).
	for _, want := range []string{
		"statusgen-lint",
		"statusgen --lint",
		`$CI_PIPELINE_SOURCE == "merge_request_event"`,
	} {
		if !strings.Contains(gl, want) {
			t.Errorf("gitlab pipeline missing lint-half marker %q", want)
		}
	}
	// Half 2 — regenerate + commit the board on the default branch.
	for _, want := range []string{
		"statusgen-regen",
		"statusgen --root .",
		"$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH",
		"git commit",
		"git push",
	} {
		if !strings.Contains(gl, want) {
			t.Errorf("gitlab pipeline missing regen-half marker %q", want)
		}
	}
	// ONE writer identity: a single committing account, plus the no-loop marker so
	// the board push does not re-trigger the regen pipeline.
	if n := strings.Count(gl, "git config user.email"); n != 1 {
		t.Errorf("gitlab pipeline must commit from ONE writer identity; got %d user.email settings:\n%s", n, gl)
	}
	if !strings.Contains(gl, "skip-status-regen") {
		t.Errorf("gitlab pipeline missing the [skip-status-regen] no-loop marker")
	}
}

// ---- auto-flip: corroboration per forge, from the forge-qualified reviewer --------

const glReviewer = "example-reviewer-sa"

// glReviewerIdent is the GitLab reviewer identity — the bare service-account
// username, no `[bot]` decoration (that decoration is GitHub-only).
func glReviewerIdent() reviewerIdentity {
	return reviewerIdentity{Logins: []string{glReviewer}, Forge: forgeGitLab, Display: glReviewer}
}

// glAf01Source builds a GitLab fake source in which af/01's merge request (PR 101)
// is approved by the reviewer and carries the given notes. The at-head verdict on
// GitLab CE lives in a note pinning the head SHA, not in the approval flag.
func glAf01Source(notes []glNote) *fakeFlipSource {
	return &fakeFlipSource{
		commits: map[string][]string{
			"brief-01-model-approved-at-head.md": {"aaa0000000000000000000000000000000000001"},
		},
		prs: map[string]int{"aaa0000000000000000000000000000000000001": 101},
		states: map[int]prReviewState{
			101: {Merged: true, HeadSHA: afHeadSHA, Approvals: []string{glReviewer}, Notes: notes},
		},
		errs: map[int]error{},
	}
}

// TestAutoFlipGitLabCorroboration (Verify row 6). A GitLab approval by the accepted
// reviewer WITH a head-pinned note flips the row; the same approval WITHOUT the head
// pin (or with a note pinning a stale SHA) does not — the approval flag alone is not
// an at-head verdict on CE.
func TestAutoFlipGitLabCorroboration(t *testing.T) {
	// WITH a head-pinned note -> flips.
	root, streams := loadAFStreams(t)
	withPin := glAf01Source([]glNote{{Author: glReviewer, Body: "verified at head " + afHeadSHA}})
	results, err := autoFlipModel(root, streams, withPin, glReviewerIdent(), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	if got := afResult(t, results, "af/01"); got.Outcome != flipDone {
		t.Fatalf("a gitlab approval WITH a head-pinned note must flip; got %v (%s)", got.Outcome, got.Reason)
	}
	if row := afRow(t, afReadme(t, root), "01"); !strings.Contains(row, "| done |") {
		t.Errorf("af/01 must be flipped to done on gitlab:\n%s", row)
	}

	// WITHOUT any pinning note -> does NOT flip.
	root2, streams2 := loadAFStreams(t)
	results2, err := autoFlipModel(root2, streams2, glAf01Source(nil), glReviewerIdent(), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	if afResult(t, results2, "af/01").Outcome == flipDone {
		t.Fatal("a gitlab approval WITHOUT a head-pinned note must NOT flip — the approval flag persists across a push on CE")
	}
	if row := afRow(t, afReadme(t, root2), "01"); !strings.Contains(row, "| verified |") {
		t.Errorf("af/01 must stay verified without a head pin:\n%s", row)
	}

	// A note pinning a STALE sha is likewise not an at-head verdict.
	root3, streams3 := loadAFStreams(t)
	stale := glAf01Source([]glNote{{Author: glReviewer, Body: "approved at " + afStaleSHA}})
	results3, err := autoFlipModel(root3, streams3, stale, glReviewerIdent(), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	if afResult(t, results3, "af/01").Outcome == flipDone {
		t.Fatal("a note pinning a STALE sha must NOT flip")
	}
}

// TestAutoFlipUncorroboratedStaysVerified (Verify row 7 — NEGATIVE, BOTH forges). A
// review that cannot be corroborated at head leaves the row at `verified` and is
// reported loudly. Without this row the brief could ship an auto-flip that advances
// everything and still pass row 6.
func TestAutoFlipUncorroboratedStaysVerified(t *testing.T) {
	// GitHub: an APPROVED review against a STALE commit (not the merged head).
	ghRoot, ghStreams := loadAFStreams(t)
	ghRes, err := autoFlipModel(ghRoot, ghStreams, afSource(), ghReviewer(afReviewer), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel(github): %v", err)
	}
	if got := afResult(t, ghRes, "af/02"); got.Outcome != flipRefused {
		t.Fatalf("github: a stale-sha approval must be refused, not flipped; got %v (%s)", got.Outcome, got.Reason)
	}
	if row := afRow(t, afReadme(t, ghRoot), "02"); !strings.Contains(row, "| verified |") {
		t.Errorf("github: af/02 must stay verified:\n%s", row)
	}

	// GitLab: approved, but the reviewer's note pins a STALE sha.
	glRoot, glStreams := loadAFStreams(t)
	glRes, err := autoFlipModel(glRoot, glStreams,
		glAf01Source([]glNote{{Author: glReviewer, Body: "verified at " + afStaleSHA}}),
		glReviewerIdent(), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel(gitlab): %v", err)
	}
	if got := afResult(t, glRes, "af/01"); got.Outcome != flipRefused {
		t.Fatalf("gitlab: an approval whose note pins a stale sha must be refused; got %v (%s)", got.Outcome, got.Reason)
	}
	if row := afRow(t, afReadme(t, glRoot), "01"); !strings.Contains(row, "| verified |") {
		t.Errorf("gitlab: af/01 must stay verified when not corroborated at head:\n%s", row)
	}

	// The refusal is surfaced loudly (a REFUSED line), never a silent pass.
	var out strings.Builder
	reportAutoFlipModel(&out, &out, glRes, glReviewerIdent().Display, false)
	if !strings.Contains(out.String(), "REFUSED") {
		t.Errorf("an uncorroborated gitlab row must be reported REFUSED loudly:\n%s", out.String())
	}
}

// TestAutoFlipReviewerLoginFromRoster (Verify row 8). The reviewer identity is
// DERIVED from the forge-qualified roster entry: a GitLab reviewer is accepted under
// the bare username (no `[bot]`), and a GitHub reviewer under BOTH forge-qualified
// renderings (`<slug>[bot]` and `app/<slug>`). The test fails if a literal `[bot]`
// suffix is still appended regardless of forge (the #349 defect).
func TestAutoFlipReviewerLoginFromRoster(t *testing.T) {
	// A GitLab reviewer — no `[bot]` account exists on GitLab.
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:      "ada:100001",
		scanEnvTrustedLogins:   "ada:100001",
		scanEnvTrustedBotSlugs: "reviewer=gitlab:example-reviewer-sa:42",
	})
	gl := modelReviewer()
	if gl.Forge != forgeGitLab {
		t.Fatalf("reviewer forge = %v, want gitlab (from the forge-qualified roster entry)", gl.Forge)
	}
	if !loginInSet(gl.Logins, "example-reviewer-sa") {
		t.Errorf("gitlab reviewer must be accepted under the bare username; logins = %v", gl.Logins)
	}
	for _, l := range gl.Logins {
		if strings.Contains(l, "[bot]") {
			t.Errorf("a gitlab reviewer login must NOT carry a literal [bot] suffix; got %q", l)
		}
	}

	// A GitHub reviewer — both renderings, derived (not a bare slug with [bot] glued
	// on, which would yield ONLY <slug>[bot] and never app/<slug>).
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:      "ada:100001",
		scanEnvTrustedLogins:   "ada:100001",
		scanEnvTrustedBotSlugs: "reviewer=github:example-reviewer-app:300000005",
	})
	gh := modelReviewer()
	if gh.Forge != forgeGitHub {
		t.Fatalf("reviewer forge = %v, want github", gh.Forge)
	}
	if !loginInSet(gh.Logins, "example-reviewer-app[bot]") || !loginInSet(gh.Logins, "app/example-reviewer-app") {
		t.Errorf("github reviewer must be accepted under both forge-qualified renderings; logins = %v", gh.Logins)
	}
}

// ---- claim decay: three distinct states, never a not-applicable dressed as a fail -

// TestClaimDecayThreeStates (Verify row 9 — NEGATIVE). ran / failed-this-run /
// not-applicable-on-this-forge produce three distinct messages; the test fails if
// any two are identical or if not-applicable reuses the transient
// "authenticate and regenerate" wording.
func TestClaimDecayThreeStates(t *testing.T) {
	branches := []string{"main", "feat/x-merged"}

	// State 1 — RAN. GitHub remote, gh reachable, a merged corpse dropped, NO notice.
	stubRemoteOriginURL(t, "https://github.com/acme/repo.git", nil)
	stubMergedClosedBranches(t, map[string]bool{"feat/x-merged": true}, nil)
	var ranGot []string
	ran := captureStderr(t, func() { ranGot = decayDeadClaims("/repo", branches) })
	if len(ranGot) != 1 || ranGot[0] != "main" {
		t.Errorf("RAN: expected the corpse dropped, got %v", ranGot)
	}
	if strings.Contains(ran, "NOTICE") {
		t.Errorf("RAN: a successful decay must emit no NOTICE; got:\n%s", ran)
	}

	// State 2 — FAILED THIS RUN. gh unreadable on a non-gitlab remote -> transient
	// "unavailable ... regenerate with gh" wording, the full branch set kept.
	stubRemoteOriginURL(t, "https://github.com/acme/repo.git", nil)
	stubMergedClosedBranches(t, nil, errors.New("gh: not authenticated"))
	var failedGot []string
	failed := captureStderr(t, func() { failedGot = decayDeadClaims("/repo", branches) })
	if !reflect.DeepEqual(failedGot, branches) {
		t.Errorf("FAILED: must keep the full set; got %v", failedGot)
	}
	if !strings.Contains(failed, "decay unavailable") || !strings.Contains(failed, "regenerate") {
		t.Errorf("FAILED: must use the transient 'unavailable ... regenerate' wording; got:\n%s", failed)
	}

	// State 3 — NOT APPLICABLE ON THIS FORGE. GitLab remote -> distinct wording, the
	// gh lister never shelled, the full set kept.
	stubRemoteOriginURL(t, "git@gitlab.com:g/p.git", nil)
	called := false
	prev := listMergedClosedBranches
	listMergedClosedBranches = func(string) (map[string]bool, error) { called = true; return nil, nil }
	t.Cleanup(func() { listMergedClosedBranches = prev })
	var naGot []string
	na := captureStderr(t, func() { naGot = decayDeadClaims("/repo", branches) })
	if called {
		t.Error("NOT-APPLICABLE: must not shell the gh lister on a gitlab remote")
	}
	if !reflect.DeepEqual(naGot, branches) {
		t.Errorf("NOT-APPLICABLE: must keep the full set; got %v", naGot)
	}
	if !strings.Contains(na, "NOT APPLICABLE") {
		t.Errorf("NOT-APPLICABLE: must say NOT APPLICABLE; got:\n%s", na)
	}
	if strings.Contains(na, "regenerate with") {
		t.Errorf("NOT-APPLICABLE must not reuse the transient 'regenerate' wording; got:\n%s", na)
	}

	// The three states must be pairwise distinct.
	if ran == failed || ran == na || failed == na {
		t.Errorf("the three decay states must produce distinct stderr; ran=%q failed=%q na=%q", ran, failed, na)
	}
}
