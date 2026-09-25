package main

import (
	"errors"
	"fmt"
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

	// B1 fixtures: a bulk brief-migration/reformat PR shaped like the motivating
	// case — a docs/streams/**-only diff, ten Brief: trailers — must never be
	// accepted as a brief's delivering PR even though it merges and carries an
	// App approval at its head.
	afBulkMigSHA     = "8888888888888888888888888888888888888888" // the ONLY commit touching af/08; resolves to PR 108
	afBulkMigHeadSHA = "9999999999999999999999999999999999999999" // merged head of PR 108 (the migration), App-approved
	// af/09: the NEWEST commit touching it resolves to the same migration PR
	// 108; an OLDER commit resolves to a genuine single-brief delivery PR 109.
	// The refusal of 108 must not be a dead end — the resolver keeps walking and
	// finds 109, exactly the shape of a brief a migration touched after its real
	// delivery had already landed.
	afRealDeliverySHA     = "aaaa111111111111111111111111111111111111" // the older commit touching af/09; resolves to PR 109
	afRealDeliveryHeadSHA = "bbbb222222222222222222222222222222222222" // merged head of PR 109, App-approved
)

const afReviewer = "rev-app[bot]"

// ghReviewer wraps a single GitHub reviewer login as the forge-aware
// reviewerIdentity autoFlipModel now takes — the accepted-login set the existing
// GitHub fixtures approve under. GitLab fixtures build their own reviewerIdentity
// (see TestAutoFlipGitLabCorroboration).
func ghReviewer(login string) reviewerIdentity {
	if login == "" {
		return reviewerIdentity{}
	}
	return reviewerIdentity{Logins: []string{login}, Forge: forgeGitHub, Display: login}
}

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
	// shapes is the diff/body shape bulkMigrationReason judges, keyed by PR
	// number. A PR absent here defaults to an ordinary single-brief shape (one
	// non-docs/streams file, one trailer) so existing fixtures need no shape of
	// their own — only the bulk-migration fixtures set one explicitly.
	shapes map[int]prShape
	// seen records every PR whose review state was fetched, so a test can prove
	// the model path never even LOOKED at a gate:human brief.
	seen []int
	// shapesSeen records every PR whose shape was fetched.
	shapesSeen []int
	// shapeErrs makes PRShape fail for a PR (e.g. a truncated file list).
	shapeErrs map[int]error
}

// ordinarySingleBriefShape is the default prShape a fixture PR without an
// explicit entry gets: a real delivery PR's shape (one code file, one Brief:
// trailer) — never migration-shaped. The fixture convention is that PR 1NN
// delivers brief af/NN (PR 101 -> af/01, PR 107 -> af/07), so the default's one
// Brief: trailer names that brief and the pre-B1 fixtures need no shape of their
// own.
func ordinarySingleBriefShape(pr int) prShape {
	return prShape{
		Files:         []string{"internal/example/example.go"},
		BriefTrailers: 1,
		Briefs:        []string{fmt.Sprintf("af/%02d", pr-100)},
	}
}

func (f *fakeFlipSource) PRShape(repo string, pr int) (prShape, error) {
	f.shapesSeen = append(f.shapesSeen, pr)
	if err, ok := f.shapeErrs[pr]; ok {
		return prShape{}, err
	}
	if s, ok := f.shapes[pr]; ok {
		return s, nil
	}
	return ordinarySingleBriefShape(pr), nil
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
			"brief-08-model-bulk-migration-only.md": {afBulkMigSHA},
			// Newest first: the migration commit is checked (and refused) before
			// the resolver walks back to the older, real delivery commit.
			"brief-09-model-migration-then-real-pr.md": {afBulkMigSHA, afRealDeliverySHA},
		},
		prs: map[string]int{
			"aaa0000000000000000000000000000000000001": 101,
			"aaa0000000000000000000000000000000000002": 102,
			"aaa0000000000000000000000000000000000003": 103,
			"aaa0000000000000000000000000000000000004": 104,
			// ...0006 deliberately absent: no merged PR resolves.
			// afIntMidSHA deliberately absent here: it must resolve through the
			// REAL resolver (assoc/prCommits), not this direct short-circuit.
			afBulkMigSHA:      108,
			afRealDeliverySHA: 109,
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
			// PR 108 (af/08, af/09): the bulk brief-migration PR. It is merged AND
			// App-approved at its head — on the pre-B1 resolver this alone was
			// enough to flip a brief. B1 still reads this review state (every
			// candidate the walk reaches must be approved at its own head), then
			// walks past the PR on its shape.
			108: {Merged: true, HeadSHA: afBulkMigHeadSHA, Reviews: []ghReview{
				{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: afBulkMigHeadSHA, Id: "PRR_bulkmig"},
			}},
			// PR 109 (af/09 only): the genuine, older single-brief delivery PR.
			109: {Merged: true, HeadSHA: afRealDeliveryHeadSHA, Reviews: []ghReview{
				{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: afRealDeliveryHeadSHA, Id: "PRR_real"},
			}},
		},
		shapes: map[int]prShape{
			// Shaped like the motivating migration: docs/streams/**-only diff, ten
			// Brief:/Authors: trailers — a bulk migration, not a delivering PR.
			108: {
				Files: []string{
					"docs/streams/security-hardening/brief-01.md",
					"docs/streams/security-hardening/brief-02.md",
					"docs/streams/other-stream/brief-03.md",
				},
				BriefTrailers: 10,
				Briefs:        []string{"security-hardening/01", "security-hardening/02", "other-stream/03"},
			},
			// An ordinary single-brief delivery PR: touches real code plus its own
			// Verify fixture, one Brief: trailer. Explicit here (rather than relying
			// on the fake's default) so the fixture reads standalone.
			109: {
				Files:         []string{"internal/security/hardening.go", "internal/security/hardening_test.go"},
				BriefTrailers: 1,
				Briefs:        []string{"af/09"},
			},
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

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
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

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
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

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
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

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
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

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
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

	results, err := autoFlipModel(root, streams, src, reviewerIdentity{}, afNow, false)
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

	results, err := autoFlipModel(root, streams, afSource(), ghReviewer(afReviewer), afNow, true)
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

// TestAutoflipRefusesHeldPass pins the held-contradiction rule: a
// **VERIFY: PASS** marker is not a flip signal on its own when the same
// Evidence entry ALSO says HELD or could-not-check on a row not marked
// deferred — even with a valid App approval at the merged head (af/01's own
// wiring, reused here unmodified), the flip must be REFUSED. Marking that
// same row deferred removes the contradiction and the identical approval
// flips it.
func TestAutoflipRefusesHeldPass(t *testing.T) {
	root, streams := loadAFStreams(t)
	s := streams[0]
	path := filepath.Join(s.Dir, "brief-01-model-approved-at-head.md")
	src := afSource() // af/01 -> PR 101, App APPROVED at the merged head

	heldEvidence := "**VERIFY: PASS (1/1 offline-runnable rows)**\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |\n" +
		"| 2 | `go test ./missing/...` | — | could-not-check: package removed | 2026-07-08 | fixture-verifier |\n"

	got := decideModelFlip(root, s, path, "af/01", heldEvidence, src, ghReviewer(afReviewer))
	if got.Outcome == flipDone {
		t.Fatalf("PASS + could-not-check on a non-deferred row must be refused, got flipDone (%s)", got.Reason)
	}
	if !strings.Contains(strings.ToLower(got.Reason), "could-not-check") {
		t.Errorf("refusal reason must name the contradicting row; got %q", got.Reason)
	}
	if len(src.seen) != 0 {
		t.Errorf("the held-contradiction check must short-circuit BEFORE any reviewer fetch; src.seen = %v", src.seen)
	}

	deferredEvidence := "**VERIFY: PASS (1/1 offline-runnable rows)**\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |\n" +
		"| 2 | `go test ./missing/...` | — | could-not-check: deferred to follow-up brief verify-integrity/05 | 2026-07-08 | fixture-verifier |\n"

	got2 := decideModelFlip(root, s, path, "af/01", deferredEvidence, src, ghReviewer(afReviewer))
	if got2.Outcome != flipDone {
		t.Fatalf("the same row deferred to a NAMED follow-up (routing phrase + reference) must flip once the App approval corroborates it, got %v (%s)", got2.Outcome, got2.Reason)
	}

	// Regression (reviewer PR #1304): the exclusion must anchor on a genuine
	// routed deferral (routing phrase AND a corroborating reference), never a
	// bare or NEGATED substring "deferred". Ordinary prose that says a row was
	// NOT deferred and is still broken must NOT suppress the contradiction —
	// the gate fails CLOSED. This string contains "deferred to" but no
	// reference, so it is not a routed row.
	negatedEvidence := "**VERIFY: PASS (1/1 offline-runnable rows)**\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |\n" +
		"| 2 | `go test ./missing/...` | — | could-not-check: this was NOT deferred to anyone, still broken | 2026-07-08 | fixture-verifier |\n"

	got3 := decideModelFlip(root, s, path, "af/01", negatedEvidence, src, ghReviewer(afReviewer))
	if got3.Outcome == flipDone {
		t.Fatalf("a negated/unreferenced \"deferred\" must NOT suppress the could-not-check contradiction — the gate must fail closed, got flipDone (%s)", got3.Reason)
	}
	if !strings.Contains(strings.ToLower(got3.Reason), "could-not-check") {
		t.Errorf("refusal reason must name the contradicting row; got %q", got3.Reason)
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

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
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

// ---- B1: bulk brief-migration PR refusal ------------------------------------------

// TestAutoFlipRefusesBulkMigrationPR is the fail-first proof for B1: a PR shaped
// like the motivating migration (docs/streams/**-only diff, ten Brief:/Authors:
// trailers) is merged and carries an App APPROVED review at its own head —
// before this fix that was sufficient to flip the brief, wrongly crediting the
// migration as its delivering PR. It must now be REFUSED as a candidate; with no
// other PR in the commit window, the brief stays verified as a could-not-check,
// never a flip.
func TestAutoFlipRefusesBulkMigrationPR(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	got := afResult(t, results, "af/08")
	if got.Outcome == flipDone {
		t.Fatalf("af/08's only candidate PR is bulk-migration-shaped — must never flip, got flipDone (%s)", got.Reason)
	}
	if !strings.Contains(got.Reason, "108") {
		t.Errorf("the refusal reason must name the refused candidate PR #108; got %q", got.Reason)
	}
	row := afRow(t, afReadme(t, root), "08")
	if !strings.Contains(row, "| verified |") {
		t.Errorf("af/08 must stay verified:\n%s", row)
	}
	if strings.Contains(row, afReviewer) {
		t.Errorf("af/08 must not be stamped from a refused migration PR:\n%s", row)
	}
	// Walking past a candidate never skips ITS approval check (S-F1): PR 108's
	// review state must have been read before its shape was judged.
	fetched := false
	for _, pr := range src.seen {
		if pr == 108 {
			fetched = true
		}
	}
	if !fetched {
		t.Error("PR 108's review state was never fetched — a walked-past candidate must still carry the App approval at its own merged head")
	}
}

// TestAutoFlipSkipsMigrationFindsRealDeliveryPR proves the refusal is not a dead
// end and does not over-tighten: af/09's newest commit resolves to the SAME
// bulk-migration PR 108, but an older commit in the same window resolves to a
// genuine single-brief delivery PR (109) — real code touched, one Brief:
// trailer, App-approved at its own head. The resolver must skip 108 and flip
// citing 109, the real delivering PR.
func TestAutoFlipSkipsMigrationFindsRealDeliveryPR(t *testing.T) {
	root, streams := loadAFStreams(t)
	src := afSource()

	results, err := autoFlipModel(root, streams, src, ghReviewer(afReviewer), afNow, false)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}

	got := afResult(t, results, "af/09")
	if got.Outcome != flipDone {
		t.Fatalf("af/09 outcome = %v (%s), want flipDone via the real delivery PR 109", got.Outcome, got.Reason)
	}
	if got.PR != 109 || got.SHA != afRealDeliveryHeadSHA {
		t.Fatalf("af/09 recorded PR/SHA = %d/%s, want 109/%s — the migration PR 108 must never be credited",
			got.PR, got.SHA, afRealDeliveryHeadSHA)
	}
	row := afRow(t, afReadme(t, root), "09")
	if !strings.Contains(row, "| done |") {
		t.Errorf("af/09 row was not flipped to done:\n%s", row)
	}
	if !strings.Contains(row, "#109") {
		t.Errorf("af/09's Reviewed stamp must cite PR #109 (the real delivery), not #108 (the migration):\n%s", row)
	}
	if strings.Contains(row, "#108") {
		t.Errorf("af/09's Reviewed stamp must never cite the migration PR #108:\n%s", row)
	}
}

// TestBulkMigrationReasonShape unit-tests the shape rule directly, independent
// of the resolver plumbing: docs/streams/**-only diffs and high trailer counts
// are refused; an ordinary single-brief delivery shape (and one that names a
// couple of closely related briefs) is not.
func TestBulkMigrationReasonShape(t *testing.T) {
	cases := []struct {
		name   string
		shape  prShape
		refuse bool
	}{
		{"docs-only many files", prShape{Files: []string{
			"docs/streams/a/brief-01.md", "docs/streams/a/README.md", "docs/streams/b/brief-02.md",
		}, BriefTrailers: 0}, true},
		{"high trailer count, no files", prShape{Files: nil, BriefTrailers: 10}, true},
		{"high trailer count with code files", prShape{
			Files:         []string{"internal/foo/foo.go"},
			BriefTrailers: 5,
		}, true},
		{"real delivery: code + one trailer", prShape{
			Files:         []string{"internal/foo/foo.go", "internal/foo/foo_test.go"},
			BriefTrailers: 1,
		}, false},
		{"boundary: code + exactly three trailers is refused", prShape{
			Files:         []string{"internal/foo/foo.go"},
			BriefTrailers: 3,
		}, true},
		{"small stack: code + two trailers", prShape{
			Files:         []string{"internal/foo/foo.go"},
			BriefTrailers: 2,
		}, false},
		{"docs file alongside real code is NOT migration-shaped", prShape{
			Files:         []string{"internal/foo/foo.go", "docs/streams/a/brief-01.md"},
			BriefTrailers: 1,
		}, false},
		{"empty shape (no files read) is not refused on files alone", prShape{
			Files: nil, BriefTrailers: 0,
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := bulkMigrationReason(c.shape)
			if c.refuse && got == "" {
				t.Errorf("shape %+v should be refused as bulk-migration-shaped, got no reason", c.shape)
			}
			if !c.refuse && got != "" {
				t.Errorf("shape %+v should NOT be refused, got reason %q", c.shape, got)
			}
		})
	}
}

// ---- B1 review round: approval-first walk, trailer attribution, full file list ----

// walkSource builds a fake in which one brief file ("brief-50-walk.md") is
// touched by the given candidate PRs, newest first — commit i resolves to
// prs[i]. states and shapes are taken as given; an absent state is a fake error.
func walkSource(prs []int, states map[int]prReviewState, shapes map[int]prShape) *fakeFlipSource {
	f := &fakeFlipSource{
		commits: map[string][]string{},
		prs:     map[string]int{},
		states:  states,
		shapes:  shapes,
		errs:    map[int]error{},
	}
	for i, n := range prs {
		sha := fmt.Sprintf("%040x", 0xc0de00+i)
		f.commits["brief-50-walk.md"] = append(f.commits["brief-50-walk.md"], sha)
		f.prs[sha] = n
	}
	return f
}

// decideWalk runs decideModelFlip for the synthetic brief af/50 over src.
func decideWalk(t *testing.T, src *fakeFlipSource) modelFlipResult {
	t.Helper()
	root, streams := loadAFStreams(t)
	s := streams[0]
	return decideModelFlip(root, s, filepath.Join(s.Dir, "brief-50-walk.md"), "af/50", "", src, ghReviewer(afReviewer))
}

// approvedAt is a merged PR state with the reviewer App APPROVED at head.
func approvedAt(head string) prReviewState {
	return prReviewState{Merged: true, HeadSHA: head, Reviews: []ghReview{
		{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: head},
	}}
}

// migration736Shape is shaped like this repo's own brief-v2 flag-day migration:
// mostly docs/streams files plus a CI patch, a changelog fragment and an upgrade
// note, and ONE Brief: trailer naming a different brief. Neither shape signal
// fires on it — only the trailer attribution catches it.
var migration736Shape = prShape{
	Files: []string{
		".github/assay-statusgen.reconcile.patch",
		"changelog/flagday-brief-v2.md",
		"docs/UPGRADING.txt",
		"docs/streams/af/brief-50-walk.md",
		"docs/streams/other/brief-01.md",
		"docs/streams/other/brief-02.md",
	},
	BriefTrailers: 1,
	Briefs:        []string{"derived-board/07"},
}

// realDelivery50 is af/50's genuine delivering PR shape.
var realDelivery50 = prShape{Files: []string{"internal/walk/walk.go"}, BriefTrailers: 1, Briefs: []string{"af/50"}}

// TestAutoFlipWalkPastStillRequiresApproval is the fail-first proof for S-F1:
// the NEWEST PR touching the brief is docs-only (walk-past-shaped) and carries
// NO reviewer-App approval; an OLDER PR is the real, approved delivery. On main
// the newest PR was the candidate and its missing approval refused the flip.
// Walking past it must not change that — the brief must be refused, naming the
// unapproved PR, never flipped citing the older one.
func TestAutoFlipWalkPastStillRequiresApproval(t *testing.T) {
	src := walkSource([]int{150, 151},
		map[int]prReviewState{
			150: {Merged: true, HeadSHA: afNoAppSHA, Reviews: []ghReview{
				{Author: ghAuthor{Login: "some-human"}, State: "APPROVED", CommitOID: afNoAppSHA},
			}},
			151: approvedAt(afRealDeliveryHeadSHA),
		},
		map[int]prShape{
			150: {Files: []string{"docs/streams/af/brief-50-walk.md"}, BriefTrailers: 1, Briefs: []string{"af/50"}},
			151: realDelivery50,
		})
	got := decideWalk(t, src)
	if got.Outcome == flipDone {
		t.Fatalf("an unapproved newer PR (#150) must block the flip even when it is walk-past-shaped; got flipDone citing #%d", got.PR)
	}
	if got.Outcome != flipRefused || got.PR != 150 {
		t.Errorf("want the refusal main gives for the unapproved #150; got outcome %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipWalkPastBodyTrailerStillRequiresApproval is the same guard for the
// trailer signal: an unapproved newer PR whose (post-merge editable) body names
// another brief must still block the flip.
func TestAutoFlipWalkPastBodyTrailerStillRequiresApproval(t *testing.T) {
	src := walkSource([]int{152, 151},
		map[int]prReviewState{
			152: {Merged: true, HeadSHA: afNoAppSHA},
			151: approvedAt(afRealDeliveryHeadSHA),
		},
		map[int]prShape{
			152: {Files: []string{"internal/x/x.go"}, BriefTrailers: 1, Briefs: []string{"other/01"}},
			151: realDelivery50,
		})
	if got := decideWalk(t, src); got.Outcome == flipDone || got.PR != 152 {
		t.Fatalf("an unapproved newer PR (#152) must block the flip whatever its body says; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipWalksPastMigrationNamingAnotherBrief is the fail-first proof for
// F1: a #736-shaped migration (mixed files, one Brief: trailer naming a
// DIFFERENT brief, App-approved) is the newest candidate. It must be walked
// past and the older, real delivery PR credited.
func TestAutoFlipWalksPastMigrationNamingAnotherBrief(t *testing.T) {
	src := walkSource([]int{160, 161},
		map[int]prReviewState{
			160: approvedAt(afBulkMigHeadSHA),
			161: approvedAt(afRealDeliveryHeadSHA),
		},
		map[int]prShape{160: migration736Shape, 161: realDelivery50})
	got := decideWalk(t, src)
	if got.Outcome != flipDone || got.PR != 161 || got.SHA != afRealDeliveryHeadSHA {
		t.Fatalf("want flipDone crediting #161 @ %s; got %v PR #%d @ %s (%s)", afRealDeliveryHeadSHA, got.Outcome, got.PR, got.SHA, got.Reason)
	}
}

// TestAutoFlipMigrationAloneNeverFlips: with only the #736-shaped migration in
// the window there is no delivering PR — could-not-check, naming the walk.
func TestAutoFlipMigrationAloneNeverFlips(t *testing.T) {
	src := walkSource([]int{160},
		map[int]prReviewState{160: approvedAt(afBulkMigHeadSHA)},
		map[int]prShape{160: migration736Shape})
	got := decideWalk(t, src)
	if got.Outcome != flipUnchecked {
		t.Fatalf("a window holding only another brief's PR must be could-not-check; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
	if !strings.Contains(got.Reason, "#160") || !strings.Contains(got.Reason, "derived-board/07") {
		t.Errorf("reason must name the walked-past #160 and the brief it names; got %q", got.Reason)
	}
}

// TestAutoFlipUnattributableIsCouldNotCheck: an approved PR with no Brief:
// trailer (an Issue:-only PR), or with two, is not credited on a guess.
func TestAutoFlipUnattributableIsCouldNotCheck(t *testing.T) {
	for name, shape := range map[string]prShape{
		"no Brief: trailer":   {Files: []string{"internal/x/x.go"}},
		"two Brief: trailers": {Files: []string{"internal/x/x.go"}, BriefTrailers: 2, Briefs: []string{"af/50", "af/51"}},
	} {
		t.Run(name, func(t *testing.T) {
			src := walkSource([]int{170, 151},
				map[int]prReviewState{170: approvedAt(afHeadSHA), 151: approvedAt(afRealDeliveryHeadSHA)},
				map[int]prShape{170: shape, 151: realDelivery50})
			got := decideWalk(t, src)
			if got.Outcome != flipUnchecked || got.PR != 170 {
				t.Fatalf("want could-not-check at #170; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}
}

// TestAutoFlipShapeReadErrorIsCouldNotCheck: a PRShape error (e.g. a truncated
// file list) is could-not-check, never a walk-past.
func TestAutoFlipShapeReadErrorIsCouldNotCheck(t *testing.T) {
	src := walkSource([]int{180, 151},
		map[int]prReviewState{180: approvedAt(afHeadSHA), 151: approvedAt(afRealDeliveryHeadSHA)},
		map[int]prShape{151: realDelivery50})
	src.shapeErrs = map[int]error{180: errors.New("PR #180 reports 168 changed files but 100 were listed")}
	if got := decideWalk(t, src); got.Outcome != flipUnchecked || got.PR != 180 {
		t.Fatalf("want could-not-check at #180; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestShapeFromListingRefusesTruncation is the fail-first proof for S-F2: a file
// list shorter than the PR's own changed_files count (gh's 100-entry cap on a
// 168-file PR) must be an error, never judged as the whole diff.
func TestShapeFromListingRefusesTruncation(t *testing.T) {
	files := make([]string, 100)
	for i := range files {
		files[i] = fmt.Sprintf("docs/streams/s/brief-%03d.md", i)
	}
	if _, err := shapeFromListing(736, "", 168, files); err == nil {
		t.Fatal("a 100-entry list for a 168-file PR must be refused as truncated")
	}
	shape, err := shapeFromListing(1, "Brief: af/50\n", 100, files)
	if err != nil {
		t.Fatalf("a complete list must be accepted: %v", err)
	}
	if len(shape.Files) != 100 || len(shape.Briefs) != 1 || shape.Briefs[0] != "af/50" {
		t.Errorf("shape = %d files, briefs %v", len(shape.Files), shape.Briefs)
	}
}

// TestParseBriefLikeTrailers pins the trailer read: fence-aware, Authors:
// counted but never an attribution, every accepted Brief: form canonicalized.
func TestParseBriefLikeTrailers(t *testing.T) {
	body := "Intro.\n\n```\nBrief: fenced/01\n```\n\nAuthors: a/01, a/02\nBrief: assay:assay:Derived-Board:07.\r\n"
	n, briefs := parseBriefLikeTrailers(body)
	if n != 2 {
		t.Errorf("count = %d, want 2 (fenced trailer skipped, Authors: counted)", n)
	}
	if len(briefs) != 1 || briefs[0] != "derived-board/07" {
		t.Errorf("briefs = %v, want [derived-board/07]", briefs)
	}
}

// TestAttributeCandidate unit-tests the attribution rule, including a brief-v2
// hierarchical brief id matched against the short trailer form.
func TestAttributeCandidate(t *testing.T) {
	code := []string{"internal/x/x.go"}
	cases := []struct {
		name  string
		shape prShape
		brief string
		want  candidateVerdict
	}{
		{"single trailer naming this brief", prShape{Files: code, BriefTrailers: 1, Briefs: []string{"af/50"}}, "af/50", candidateCredit},
		{"v2 brief id vs short trailer", prShape{Files: code, BriefTrailers: 1, Briefs: []string{"derived-board/02"}}, "assay:assay:derived-board:02", candidateCredit},
		{"trailer names another brief", prShape{Files: code, BriefTrailers: 1, Briefs: []string{"af/51"}}, "af/50", candidateWalkPast},
		{"#736 shape", migration736Shape, "af/50", candidateWalkPast},
		{"docs-only naming this brief (Verify PR)", prShape{Files: []string{"docs/streams/af/README.md"}, BriefTrailers: 1, Briefs: []string{"af/50"}}, "af/50", candidateWalkPast},
		{"Authors:-only authoring PR", prShape{Files: code, BriefTrailers: 1}, "af/50", candidateWalkPast},
		{"no trailer at all", prShape{Files: code}, "af/50", candidateUnattributable},
		{"two trailers incl. this brief", prShape{Files: code, BriefTrailers: 2, Briefs: []string{"af/50", "af/51"}}, "af/50", candidateUnattributable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, why := attributeCandidate(c.shape, c.brief); got != c.want {
				t.Errorf("attributeCandidate = %v (%s), want %v", got, why, c.want)
			}
		})
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
	results, err := autoFlipModel(root, streams, afSource(), reviewerIdentity{}, afNow, false)
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
