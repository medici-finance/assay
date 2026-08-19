package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- fixture plumbing --------------------------------------------------------------

// loadAFStreams copies the auto-flip fixture tree into a temp root and loads it.
func loadAFStreams(t *testing.T) (string, []*Stream) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/autoflip")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

// afReadme returns the fixture stream README after a run.
func afReadme(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "docs", "streams", "af", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// afRow returns the briefs-table row for brief number num.
func afRow(t *testing.T, readme, num string) string {
	t.Helper()
	for _, line := range strings.Split(readme, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| "+num+" |") {
			return line
		}
	}
	t.Fatalf("no row for brief %s in:\n%s", num, readme)
	return ""
}

// afResult picks one brief's result out of a run.
func afResult(t *testing.T, results []modelFlipResult, brief string) modelFlipResult {
	t.Helper()
	for _, r := range results {
		if r.Brief == brief {
			return r
		}
	}
	t.Fatalf("no result recorded for %s (got %+v)", brief, results)
	return modelFlipResult{}
}

// ---- the fake gh/review seam -------------------------------------------------------

// The four SHAs the fixture universe runs on. They are 40-hex so they exercise
// the same comparison the live source feeds in.
const (
	afHeadSHA  = "1111111111111111111111111111111111111111" // the merged head of PR 101/103
	afStaleSHA = "2222222222222222222222222222222222222222" // an EARLIER commit on PR 102
	afMergedPR = "3333333333333333333333333333333333333333" // merged head of PR 102
	afNoAppSHA = "4444444444444444444444444444444444444444" // merged head of PR 104
	// PR 107 (af/07) merged as a REAL merge commit: its head carries the App
	// approval, its brief file was touched in an INTERMEDIATE (non-head) commit,
	// and the merge commit is a third SHA. Fix A must credit PR 107 from the
	// intermediate commit even though it is neither head nor merge commit.
	afIntHeadSHA  = "5555555555555555555555555555555555555555" // merged head of PR 107 (approval here)
	afIntMergeSHA = "6666666666666666666666666666666666666666" // the merge commit of PR 107
	afIntMidSHA   = "7777777777777777777777777777777777777777" // the intermediate commit that touched brief-07
)

const afReviewer = "rev-app[bot]"

// fakeFlipSource is the test seam. Nothing here touches git or gh.
type fakeFlipSource struct {
	commits map[string][]string // brief file basename -> commit SHAs, newest first
	prs     map[string]int      // commit SHA -> merged PR number (direct short-circuit)
	states  map[int]prReviewState
	errs    map[int]error
	// assoc and prCommits drive the commit SHAs whose resolution must run through
	// the REAL mergedPRForCommit logic (Fix A): assoc is the commit→pulls
	// association the live endpoint returns, prCommits is the introduced-commits
	// oracle. A SHA present in assoc is resolved by the real resolver; any other
	// falls back to the direct prs map.
	assoc     map[string][]ghCommitPR
	prCommits map[int][]string
	// seen records every PR whose review state was fetched, so a test can prove
	// the model path never even LOOKED at a gate:human brief.
	seen []int
}

func (f *fakeFlipSource) CommitsTouching(root, relPath string, limit int) ([]string, error) {
	return f.commits[filepath.Base(relPath)], nil
}

func (f *fakeFlipSource) MergedPRForCommit(repo, sha string) (int, bool, error) {
	// Exercise the real resolver end-to-end for SHAs with an association fixture.
	if prs, ok := f.assoc[sha]; ok {
		return mergedPRForCommit(sha, prs, func(n int) ([]string, error) {
			return f.prCommits[n], nil
		})
	}
	pr, ok := f.prs[sha]
	return pr, ok, nil
}

func (f *fakeFlipSource) ReviewState(repo string, pr int) (prReviewState, error) {
	f.seen = append(f.seen, pr)
	if err, ok := f.errs[pr]; ok {
		return prReviewState{}, err
	}
	st, ok := f.states[pr]
	if !ok {
		return prReviewState{}, errors.New("fake: no state for PR")
	}
	return st, nil
}

// afSource builds the fixture universe:
//
//	af/01 -> PR 101, App APPROVED at the merged head           -> flips
//	af/02 -> PR 102, App APPROVED at an EARLIER commit          -> refused (SHA mismatch)
//	af/03 -> PR 103, App APPROVED at the merged head, gate:human -> never reached
//	af/04 -> PR 104, no App review (a human APPROVED instead)   -> refused
//	af/06 -> no commit maps to a merged PR                      -> could-not-check
func afSource() *fakeFlipSource {
	approvedAtHead := prReviewState{Merged: true, HeadSHA: afHeadSHA, Reviews: []ghReview{
		{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: afHeadSHA, Id: "PRR_head"},
	}}
	return &fakeFlipSource{
		commits: map[string][]string{
			"brief-01-model-approved-at-head.md":    {"aaa0000000000000000000000000000000000001"},
			"brief-02-model-stale-approval.md":      {"aaa0000000000000000000000000000000000002"},
			"brief-03-human-verified.md":            {"aaa0000000000000000000000000000000000003"},
			"brief-04-model-no-approval.md":         {"aaa0000000000000000000000000000000000004"},
			"brief-06-model-no-pr.md":               {"aaa0000000000000000000000000000000000006"},
			"brief-07-model-intermediate-commit.md": {afIntMidSHA},
		},
		prs: map[string]int{
			"aaa0000000000000000000000000000000000001": 101,
			"aaa0000000000000000000000000000000000002": 102,
			"aaa0000000000000000000000000000000000003": 103,
			"aaa0000000000000000000000000000000000004": 104,
			// ...0006 deliberately absent: no merged PR resolves.
			// afIntMidSHA deliberately absent here: it must resolve through the
			// REAL resolver (assoc/prCommits), not this direct short-circuit.
		},
		// af/07's brief commit is an intermediate (non-head, non-merge) commit of
		// PR 107. The direct prs map does NOT carry it, so it can only resolve if
		// mergedPRForCommit credits it from PR 107's introduced-commits list.
		assoc: map[string][]ghCommitPR{
			afIntMidSHA: {mkCommitPR(107, afIntMergeSHA, afIntHeadSHA)},
		},
		prCommits: map[int][]string{
			// The commits PR 107 introduced: its head plus the intermediate one.
			107: {afIntHeadSHA, afIntMidSHA},
		},
		states: map[int]prReviewState{
			101: approvedAtHead,
			// The App approved commit ...2222, then more commits landed and
			// ...3333 was merged. A stale approval must not close a brief.
			102: {Merged: true, HeadSHA: afMergedPR, Reviews: []ghReview{
				{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: afStaleSHA, Id: "PRR_stale"},
			}},
			103: approvedAtHead,
			// A HUMAN approved at head; the reviewer App only commented.
			104: {Merged: true, HeadSHA: afNoAppSHA, Reviews: []ghReview{
				{Author: ghAuthor{Login: "some-human"}, State: "APPROVED", CommitOID: afNoAppSHA, Id: "PRR_human"},
				{Author: ghAuthor{Login: afReviewer}, State: "COMMENTED", CommitOID: afNoAppSHA, Id: "PRR_comment"},
			}},
			// PR 107 merged as a real merge commit; the App approved its head.
			107: {Merged: true, HeadSHA: afIntHeadSHA, Reviews: []ghReview{
				{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: afIntHeadSHA, Id: "PRR_int"},
			}},
		},
		errs: map[int]error{},
	}
}

// mkCommitPR builds one commit→pulls association entry (the anonymous Head
// struct makes a literal awkward).
func mkCommitPR(number int, mergeCommitSHA, headSHA string) ghCommitPR {
	p := ghCommitPR{Number: number, MergedAt: "2026-08-14T00:00:00Z", MergeCommitSHA: mergeCommitSHA}
	p.Head.SHA = headSHA
	return p
}

var afNow = time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)

// ---- Task-1 case 1: approved at the merged head -> stamped AND flipped -------------

func TestAutoFlipAtHead(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, afReviewer, afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	got := afResult(t, results, "af/01")
	if got.Outcome != flipDone {
		t.Fatalf("af/01 outcome = %v (%s), want flipDone", got.Outcome, got.Reason)
	}
	if got.PR != 101 || got.SHA != afHeadSHA {
		t.Errorf("af/01 recorded PR/SHA = %d/%s, want 101/%s", got.PR, got.SHA, afHeadSHA)
	}

	row := afRow(t, afReadme(t, root), "01")
	if !strings.Contains(row, "| done |") {
		t.Errorf("af/01 row was not flipped to done:\n%s", row)
	}
	// The Reviewed stamp must be auditable back to the exact review: date,
	// the reviewing App, the PR number and the head SHA.
	for _, want := range []string{"2026-08-13", afReviewer, "#101", afHeadSHA} {
		if !strings.Contains(row, want) {
			t.Errorf("af/01 Reviewed stamp is missing %q:\n%s", want, row)
		}
	}
	// methodology/19 done-shape: a `done` row's Reviewed cell must start with a date.
	cells := splitRow(row)
	reviewed := strings.TrimSpace(cells[len(cells)-1])
	if !verifiedCellRe.MatchString(reviewed) {
		t.Errorf("Reviewed cell %q does not satisfy the dated done-shape lint", reviewed)
	}
}

// ---- Task-1 case 2: recorded SHA does not match a live App approval ---------------

func TestAutoFlipStaleSHA(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, afReviewer, afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	got := afResult(t, results, "af/02")
	if got.Outcome != flipRefused {
		t.Fatalf("af/02 outcome = %v (%s), want flipRefused — a stale approval must not close a brief", got.Outcome, got.Reason)
	}
	if !strings.Contains(got.Reason, afMergedPR) || !strings.Contains(got.Reason, afStaleSHA) {
		t.Errorf("the refusal must name both the merged head and the approved commit; reason = %q", got.Reason)
	}

	row := afRow(t, afReadme(t, root), "02")
	if !strings.Contains(row, "| verified |") {
		t.Errorf("af/02 must STAY verified:\n%s", row)
	}
	if strings.Contains(row, afReviewer) {
		t.Errorf("af/02 must not be stamped:\n%s", row)
	}
}

// ---- Task-1 case 3: the gate:human path is provably untouched ----------------------

func TestAutoFlipHumanGate(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, afReviewer, afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	for _, r := range results {
		if r.Brief == "af/03" {
			t.Fatalf("af/03 is gate:human and must not even be a CANDIDATE; got %+v", r)
		}
	}
	// af/03's PR carries exactly the approval that flips af/01. The model path
	// must never have fetched it — reaching the check at all is the failure.
	for _, pr := range src.seen {
		if pr == 103 {
			t.Error("the model path fetched the review state of a gate:human brief's PR")
		}
	}
	row := afRow(t, afReadme(t, root), "03")
	if !strings.Contains(row, "| verified |") {
		t.Errorf("af/03 (gate:human) must stay verified:\n%s", row)
	}
	if strings.Contains(row, afReviewer) {
		t.Errorf("af/03 (gate:human) must not be stamped by the model path:\n%s", row)
	}
}

// ---- Task-1 case 4: no App approval -> not flipped --------------------------------

func TestAutoFlipNoApproval(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, afReviewer, afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	got := afResult(t, results, "af/04")
	if got.Outcome != flipRefused {
		t.Fatalf("af/04 outcome = %v (%s), want flipRefused — a human APPROVED review is not the App's", got.Outcome, got.Reason)
	}
	row := afRow(t, afReadme(t, root), "04")
	if !strings.Contains(row, "| verified |") {
		t.Errorf("af/04 must stay verified:\n%s", row)
	}
}

// ---- could-not-check is NOT confirmed ---------------------------------------------

func TestAutoFlipUnreadable(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()
	// PR 101 becomes unreadable: an unfetchable review is could-not-check.
	src.errs[101] = errors.New("gh: 503 upstream")

	results, err := autoFlipModel(root, streams, src, afReviewer, afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	if got := afResult(t, results, "af/01"); got.Outcome != flipUnchecked {
		t.Errorf("af/01 with an unreadable review = %v, want flipUnchecked", got.Outcome)
	}
	// af/06 resolves no merged PR at all — also could-not-check, never "clean".
	if got := afResult(t, results, "af/06"); got.Outcome != flipUnchecked {
		t.Errorf("af/06 with no merged PR = %v, want flipUnchecked", got.Outcome)
	}
	for _, num := range []string{"01", "06"} {
		if row := afRow(t, afReadme(t, root), num); !strings.Contains(row, "| verified |") {
			t.Errorf("af/%s must stay verified when the check could not be made:\n%s", num, row)
		}
	}
}

// TestAutoFlipNoReviewer pins the fail-closed direction on
// the roster: with no `reviewer=` App bound there is no identity whose approval
// could be corroborated, so every candidate is could-not-check.
func TestAutoFlipNoReviewer(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, "", afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	for _, r := range results {
		if r.Outcome != flipUnchecked {
			t.Errorf("%s = %v with no reviewer App configured, want flipUnchecked", r.Brief, r.Outcome)
		}
	}
	if len(src.seen) != 0 {
		t.Errorf("nothing should be fetched with no reviewer App configured; fetched %v", src.seen)
	}
	if strings.Contains(afReadme(t, root), "| done |") && !strings.Contains(afReadme(t, root), "| 05 |") {
		t.Error("a row was flipped with no reviewer App configured")
	}
}

// TestAutoFlipDryRun pins that the reporting mode is read-only.
func TestAutoFlipDryRun(t *testing.T) {
	root, streams := loadAFStreams(t)
	before := afReadme(t, root)

	results, err := autoFlipModel(root, streams, afSource(), afReviewer, afNow, true)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	if got := afResult(t, results, "af/01"); got.Outcome != flipDone {
		t.Errorf("af/01 dry-run outcome = %v, want flipDone (the DECISION is unchanged)", got.Outcome)
	}
	if afReadme(t, root) != before {
		t.Error("--dry-run must not write the README")
	}
}

// TestAutoFlipNoOverride pins the no-override requirement: the
// flag surface must expose no way to skip corroboration, and no environment
// variable may stand in for an App approval. A future --force/--assume-approved
// reddens this test.
func TestAutoFlipNoOverride(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("autoflip.go")
	if err != nil {
		t.Fatal(err)
	}
	// main.go: only names that could only ever mean "skip this corroboration".
	// A generic `--force` on some unrelated mode is not this test's business.
	for _, banned := range []string{"force-flip", "assume-approved", "assume_approved", "skip-corroborat", "no-corroborat"} {
		if strings.Contains(strings.ToLower(string(raw)), "\""+banned) {
			t.Errorf("main.go declares a flag containing %q — the model-path flip has no override", banned)
		}
	}
	// autoflip.go: nothing in the flip's own code may so much as read an
	// override, including a bare `force`.
	for _, banned := range []string{"force", "assume-approved", "assume_approved", "skip-corroborat", "no-corroborat"} {
		if strings.Contains(strings.ToLower(string(src)), banned) {
			t.Errorf("autoflip.go mentions %q — the model-path flip has no override", banned)
		}
	}
	if strings.Contains(string(src), "os.Getenv") || strings.Contains(string(src), "LookupEnv") {
		t.Error("autoflip.go reads the environment directly — corroboration must not be env-skippable")
	}
}

// ---- Fix A: the merge-committed intermediate-commit resolver -----------------------

// TestMergedPRForCommitIntermediate pins the resolver's POSITIVE case: a PR
// merged as a real merge commit, whose brief file was touched in an intermediate
// (non-head, non-merge) commit, is credited from its introduced-commits list.
// This is the exact shape of the 8 stuck briefs.
func TestMergedPRForCommitIntermediate(t *testing.T) {
	prs := []ghCommitPR{mkCommitPR(201, afIntMergeSHA, afIntHeadSHA)}
	// The intermediate SHA is neither the head nor the merge commit.
	if strings.EqualFold(afIntMidSHA, afIntHeadSHA) || strings.EqualFold(afIntMidSHA, afIntMergeSHA) {
		t.Fatal("fixture bug: the intermediate SHA must differ from head and merge commit")
	}
	oracle := func(n int) ([]string, error) {
		if n != 201 {
			t.Fatalf("oracle asked about PR %d, want 201", n)
		}
		return []string{afIntHeadSHA, afIntMidSHA, "aaaa000000000000000000000000000000000009"}, nil
	}
	n, ok, err := mergedPRForCommit(afIntMidSHA, prs, oracle)
	if err != nil {
		t.Fatalf("mergedPRForCommit: %v", err)
	}
	if !ok || n != 201 {
		t.Fatalf("an intermediate commit of a merge-committed PR must resolve to it; got (%d, %v)", n, ok)
	}
}

// TestMergedPRForCommitFastPath pins that the head / merge-commit exact match
// still wins WITHOUT ever consulting the introduced-commits oracle.
func TestMergedPRForCommitFastPath(t *testing.T) {
	prs := []ghCommitPR{mkCommitPR(202, afIntMergeSHA, afIntHeadSHA)}
	oracle := func(n int) ([]string, error) {
		t.Fatalf("the fast path must not call the introduced-commits oracle (PR %d)", n)
		return nil, nil
	}
	for _, sha := range []string{afIntHeadSHA, afIntMergeSHA} {
		n, ok, err := mergedPRForCommit(sha, prs, oracle)
		if err != nil || !ok || n != 202 {
			t.Errorf("fast path for %s = (%d, %v, %v), want (202, true, nil)", sha, n, ok, err)
		}
	}
}

// TestMergedPRForCommitMerelyContains pins the NEGATIVE case that preserves the
// original integrity intent: a PR that merely CONTAINS an already-landed commit
// on its branch (so the commit is absent from the commits IT introduced) is NOT
// credited. Crediting it would let an unrelated PR's review close a brief.
func TestMergedPRForCommitMerelyContains(t *testing.T) {
	landed := "bbbb000000000000000000000000000000000001" // landed earlier via another PR
	prs := []ghCommitPR{mkCommitPR(203, afIntMergeSHA, afIntHeadSHA)}
	oracle := func(n int) ([]string, error) {
		// PR 203 introduced only its own two commits; `landed` is an ancestor of
		// its base and so is absent — it is merely CONTAINED, not introduced.
		return []string{afIntHeadSHA, "cccc000000000000000000000000000000000002"}, nil
	}
	n, ok, err := mergedPRForCommit(landed, prs, oracle)
	if err != nil {
		t.Fatalf("mergedPRForCommit: %v", err)
	}
	if ok {
		t.Fatalf("a PR that merely CONTAINS an already-landed commit must not be credited; got PR %d", n)
	}
}

// TestMergedPRForCommitSkipsUnmerged pins that an OPEN associated PR is never a
// resolution, on either path.
func TestMergedPRForCommitSkipsUnmerged(t *testing.T) {
	open := ghCommitPR{Number: 204} // MergedAt "" -> not merged
	open.Head.SHA = afIntMidSHA
	prs := []ghCommitPR{open}
	oracle := func(n int) ([]string, error) { return []string{afIntMidSHA}, nil }
	if n, ok, _ := mergedPRForCommit(afIntMidSHA, prs, oracle); ok {
		t.Fatalf("an unmerged PR must never resolve; got PR %d", n)
	}
}

// TestAutoFlipIntermediateCommitFlips is the END-TO-END proof for Fix A: af/07's
// brief file was touched in an intermediate commit of PR 107 (merged as a real
// merge commit), and the whole pipeline — resolver through flip — closes it.
func TestAutoFlipIntermediateCommitFlips(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, afReviewer, afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	got := afResult(t, results, "af/07")
	if got.Outcome != flipDone {
		t.Fatalf("af/07 outcome = %v (%s), want flipDone via the intermediate-commit resolver", got.Outcome, got.Reason)
	}
	if got.PR != 107 || got.SHA != afIntHeadSHA {
		t.Errorf("af/07 recorded PR/SHA = %d/%s, want 107/%s", got.PR, got.SHA, afIntHeadSHA)
	}
	row := afRow(t, afReadme(t, root), "07")
	if !strings.Contains(row, "| done |") {
		t.Errorf("af/07 row was not flipped to done:\n%s", row)
	}
	for _, want := range []string{"2026-08-13", afReviewer, "#107", afIntHeadSHA} {
		if !strings.Contains(row, want) {
			t.Errorf("af/07 Reviewed stamp is missing %q:\n%s", want, row)
		}
	}
}

// ---- Fix B: could-not-check exit policy --------------------------------------------

// TestReportAutoFlipStructuralNonFatal pins that a STRUCTURALLY-unresolvable
// could-not-check (no merged PR resolves) returns 0 with a loud NOTICE — the
// standing main-red repair. It must NOT be treated as a misconfiguration.
func TestReportAutoFlipStructuralNonFatal(t *testing.T) {
	results := []modelFlipResult{
		{Brief: "af/01", Outcome: flipDone, Stamp: "2026-08-13 rev-app[bot] (approved PR #101 @ x)"},
		{Brief: "af/06", Outcome: flipUnchecked, Reason: "no merged pull request resolves from the last 25 commits touching brief-06.md"},
	}
	var out, errw strings.Builder
	if code := reportAutoFlipModel(&out, &errw, results, afReviewer, false); code != 0 {
		t.Fatalf("a structural could-not-check must be non-fatal (exit 0), got %d", code)
	}
	if !strings.Contains(out.String(), "af/06 NOTICE COULD-NOT-CHECK") {
		t.Errorf("the structural could-not-check must print a NOTICE line:\n%s", out.String())
	}
	if !strings.Contains(errw.String(), "NOTICE") || !strings.Contains(errw.String(), "non-fatal") {
		t.Errorf("stderr must carry a loud non-fatal NOTICE:\n%s", errw.String())
	}
}

// TestReportAutoFlipMisconfigFatal pins that a fixable MISCONFIGURATION (no
// reviewer App bound / no owning repo) STAYS fatal — the operator-fixable case
// exit-1 was designed for.
func TestReportAutoFlipMisconfigFatal(t *testing.T) {
	results := []modelFlipResult{
		{Brief: "af/01", Outcome: flipUnchecked, Misconfig: true, Reason: "no `reviewer=` App bound in ASSAY_TRUSTED_BOT_SLUGS"},
	}
	if code := reportAutoFlipModel(io.Discard, io.Discard, results, "", false); code != 1 {
		t.Fatalf("a fixable misconfiguration must stay fatal (exit 1), got %d", code)
	}
}

// TestReportAutoFlipMisconfigDominates pins that when both kinds are present, the
// fatal misconfiguration wins the exit code (the operator-fixable break is the
// one that must not be swallowed by a non-fatal NOTICE).
func TestReportAutoFlipMisconfigDominates(t *testing.T) {
	results := []modelFlipResult{
		{Brief: "af/06", Outcome: flipUnchecked, Reason: "no merged pull request resolves"},
		{Brief: "af/01", Outcome: flipUnchecked, Misconfig: true, Reason: "no owning repo"},
	}
	if code := reportAutoFlipModel(io.Discard, io.Discard, results, afReviewer, false); code != 1 {
		t.Fatalf("misconfig must dominate the exit code, got %d", code)
	}
}

// TestAutoFlipNoReviewerMisconfig pins that the no-reviewer path flags Misconfig
// on every candidate AND stays fatal through the report — the fail-closed
// direction on the roster is preserved.
func TestAutoFlipNoReviewerMisconfig(t *testing.T) {
	root, streams := loadAFStreams(t)
	results, err := autoFlipModel(root, streams, afSource(), "", afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected candidates even with no reviewer App")
	}
	for _, r := range results {
		if !r.Misconfig {
			t.Errorf("%s with no reviewer App must be flagged Misconfig (got %+v)", r.Brief, r)
		}
	}
	if code := reportAutoFlipModel(io.Discard, io.Discard, results, "", false); code != 1 {
		t.Errorf("no reviewer App bound must be fatal, got %d", code)
	}
}

// TestAutoFlipStamp pins the stamp shape independently of a run.
func TestAutoFlipStamp(t *testing.T) {
	got := modelReviewedStamp(afNow, afReviewer, 101, afHeadSHA)
	want := "2026-08-13 rev-app[bot] (approved PR #101 @ " + afHeadSHA + ")"
	if got != want {
		t.Errorf("stamp = %q, want %q", got, want)
	}
	if strings.Contains(got, "|") {
		t.Error("the stamp must not contain a pipe — it is written into a markdown table cell")
	}
}
