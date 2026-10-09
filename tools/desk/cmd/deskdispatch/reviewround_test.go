package main

// reviewround_test.go — the review round's scope and number (#2444): the pure decisions,
// the forge reads behind them, and the statement the assignment carries.
//
// The forge here is fakeRoundForge, an in-memory stand-in for the read-only surface
// (reviewRoundForge). No test in this file or reviewgate_test.go reaches a forge or mints a
// credential.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	rrBase  = "0000000000000000000000000000000000000000"
	rrHead1 = "1111111111111111111111111111111111111111"
	rrHead2 = "2222222222222222222222222222222222222222"
	rrHead3 = "3333333333333333333333333333333333333333"
	rrHead4 = "4444444444444444444444444444444444444444"
	rrMain  = "9999999999999999999999999999999999999999"
	rrRepo  = "example-org/tracker"
)

// fakeRoundForge answers the read-only surface from fixtures and records every call.
type fakeRoundForge struct {
	pr          *deskkit.PullRequest
	prErr       error
	reviews     []deskkit.Review
	reviewsErr  error
	files       []deskkit.ChangedFile
	filesErr    error
	checks      *deskkit.ChecksAtHead
	checksErr   error
	required    []string
	requiredErr error
	labelEvents map[int][]deskkit.LabelEvent
	labelErr    error
	trust       map[int]*deskkit.TrustPayload
	trustErr    error
	issues      map[int]*deskkit.Issue
	issueErr    error
	compares    map[string]*deskkit.RefComparison
	compareErr  map[string]error
	commits     map[string]*deskkit.RepoCommit
	commitErr   error
	calls       []string
}

func (f *fakeRoundForge) GetPullRequest(deskkit.ForgeRepo, int) (*deskkit.PullRequest, error) {
	f.calls = append(f.calls, "GetPullRequest")
	return f.pr, f.prErr
}

func (f *fakeRoundForge) GetIssue(_ deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetIssue %d", n))
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issues[n], nil
}

func (f *fakeRoundForge) ReviewsAtHead(deskkit.ForgeRepo, int) ([]deskkit.Review, error) {
	f.calls = append(f.calls, "ReviewsAtHead")
	return f.reviews, f.reviewsErr
}

func (f *fakeRoundForge) ListChangedFiles(deskkit.ForgeRepo, int) ([]deskkit.ChangedFile, error) {
	f.calls = append(f.calls, "ListChangedFiles")
	return f.files, f.filesErr
}

func (f *fakeRoundForge) ChecksAtHead(deskkit.ForgeRepo, string) (*deskkit.ChecksAtHead, error) {
	f.calls = append(f.calls, "ChecksAtHead")
	return f.checks, f.checksErr
}

func (f *fakeRoundForge) RequiredStatusChecks(deskkit.ForgeRepo, string) ([]string, error) {
	f.calls = append(f.calls, "RequiredStatusChecks")
	return f.required, f.requiredErr
}

func (f *fakeRoundForge) ListLabelEvents(_ deskkit.ForgeRepo, n int) ([]deskkit.LabelEvent, error) {
	f.calls = append(f.calls, fmt.Sprintf("ListLabelEvents %d", n))
	return f.labelEvents[n], f.labelErr
}

func (f *fakeRoundForge) ListIssueLabelEvents(_ deskkit.ForgeRepo, n int) ([]deskkit.LabelEvent, error) {
	f.calls = append(f.calls, fmt.Sprintf("ListIssueLabelEvents %d", n))
	return f.labelEvents[n], f.labelErr
}

func (f *fakeRoundForge) PRTrustEvents(_ deskkit.ForgeRepo, n int) (*deskkit.TrustPayload, error) {
	f.calls = append(f.calls, fmt.Sprintf("PRTrustEvents %d", n))
	return f.trust[n], f.trustErr
}

func (f *fakeRoundForge) IssueTrustEvents(_ deskkit.ForgeRepo, n int) (*deskkit.TrustPayload, error) {
	f.calls = append(f.calls, fmt.Sprintf("IssueTrustEvents %d", n))
	return f.trust[n], f.trustErr
}

func (f *fakeRoundForge) CompareRefs(_ deskkit.ForgeRepo, base, head string) (*deskkit.RefComparison, error) {
	key := base + "..." + head
	f.calls = append(f.calls, "CompareRefs "+key)
	if err := f.compareErr[key]; err != nil {
		return nil, err
	}
	return f.compares[key], nil
}

func (f *fakeRoundForge) GetCommit(_ deskkit.ForgeRepo, sha string) (*deskkit.RepoCommit, error) {
	f.calls = append(f.calls, "GetCommit "+sha)
	if f.commitErr != nil {
		return nil, f.commitErr
	}
	return f.commits[sha], nil
}

func (f *fakeRoundForge) called(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func changed(names ...string) []deskkit.ChangedFile {
	out := make([]deskkit.ChangedFile, 0, len(names))
	for _, n := range names {
		out = append(out, deskkit.ChangedFile{Filename: n, Status: "modified"})
	}
	return out
}

// verdict is one review by the reviewer identity carrying one lane's verdict line.
func verdict(id int64, lane, word, head string, extra ...string) deskkit.Review {
	line := "Verdict: " + word
	if lane == laneSecurity {
		line = "Security-Review: " + word
	}
	state := "CHANGES_REQUESTED"
	if word == "approve" || word == "pass" {
		state = "APPROVED"
	}
	return deskkit.Review{ID: id, Author: deskkit.Account{Login: rpReviewer}, State: state, CommitID: head,
		Body: line + "\n\n" + strings.Join(extra, "\n"), SubmittedAt: "2026-03-04T05:00:00Z"}
}

func findingBlock(ids ...string) string {
	var fs []deskkit.Finding
	for _, id := range ids {
		fs = append(fs, deskkit.Finding{ID: id, Class: "c-" + id, Severity: deskkit.SeverityBlocking,
			Blocker: deskkit.BlockerCodeContent, State: deskkit.StateOpen, Failure: "it fails"})
	}
	return deskkit.RenderFindingBlock(deskkit.FindingBlockV1{Findings: fs})
}

// roundFixture is a change at rrHead2 whose correctness lane reviewed it at rrHead1, with one
// ordinary commit between the two heads touching one of the change's two files. Every read
// succeeds and no gate condition holds: the scope is DELTA and the round is 2.
func roundFixture() *fakeRoundForge {
	return &fakeRoundForge{
		pr: &deskkit.PullRequest{Number: 77, State: "open", HeadSHA: rrHead2, BaseRef: "main", ChangedFiles: 2,
			Body: "Adds the widget.\n"},
		reviews: []deskkit.Review{verdict(501, laneCorrectness, "request-changes", rrHead1, findingBlock("F-1", "F-2"))},
		files:   changed("a.go", "b.go"),
		checks:  &deskkit.ChecksAtHead{},
		compares: map[string]*deskkit.RefComparison{
			"main..." + rrHead1: {Files: changed("a.go", "b.go"), CommitsComplete: true},
			rrHead1 + "..." + rrHead2: {CommitsComplete: true, Commits: []deskkit.RepoCommit{
				{SHA: rrHead2, Parents: []string{rrHead1}}}},
		},
		commits: map[string]*deskkit.RepoCommit{
			rrHead2: {SHA: rrHead2, Parents: []string{rrHead1}, Files: changed("a.go"), FilesComplete: true},
		},
		compareErr:  map[string]error{},
		labelEvents: map[int][]deskkit.LabelEvent{},
		trust:       map[int]*deskkit.TrustPayload{},
		issues:      map[int]*deskkit.Issue{},
	}
}

// useRoundForge wires the round's forge read to f for one test, and binds the reviewer.
func useRoundForge(t *testing.T, f *fakeRoundForge) {
	t.Helper()
	old := reviewRoundForgeFn
	reviewRoundForgeFn = func(repo string) (reviewRoundForge, deskkit.ForgeRepo, error) {
		fr, err := forgeRepoOf(repo)
		return f, fr, err
	}
	t.Cleanup(func() { reviewRoundForgeFn = old })
	useReviewer(t, rpReviewer, true)
}

// roundOf runs the pre-claim read for the correctness lane of change 77.
func roundOf(t *testing.T, f *fakeRoundForge, claimKey string) (reviewRound, error) {
	t.Helper()
	useRoundForge(t, f)
	return prepareReviewRound(dispatchOpts{kit: "review", pr: 77, quiet: true}, dispatchPlan{claimKey: claimKey}, rrRepo)
}

const (
	rrKeyC = "tracker--pr-77"
	rrKeyS = "tracker--pr-77--security"
)

func reasonsOf(d scopeDecision) string {
	var out []string
	for _, r := range d.reasons {
		out = append(out, string(r))
	}
	return strings.Join(out, ",")
}

// deltaFacts is a set of facts for which no full-pass condition holds.
func deltaFacts() scopeFacts {
	return scopeFacts{verdict: true, prevHead: rrHead1, head: rrHead2, commits: 2,
		delta: []string{"a.go", "docs/other.md"}, prevFiles: []string{"a.go", "b.go"}, headFiles: []string{"a.go", "b.go"}}
}

// Each full-pass condition, by name, and the one set of facts that is a delta.
func TestScopeDecisionNamesEachFullPassCondition(t *testing.T) {
	many := func(n int) []string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, fmt.Sprintf("f%02d.go", i))
		}
		return out
	}
	for _, tc := range []struct {
		name string
		edit func(*scopeFacts)
		want string // "" = DELTA
	}{
		{"nothing holds", func(*scopeFacts) {}, ""},
		{"a fact could not be read", func(f *scopeFacts) { f.unknown = "the reviews could not be read" }, "could-not-determine"},
		{"the lane has no verdict of its own", func(f *scopeFacts) { f.verdict = false }, "no-verdict-of-its-own"},
		{"the previous verdict is not pinned to a commit", func(f *scopeFacts) { f.prevHead = "" }, "could-not-determine"},
		{"the previous verdict's head is abbreviated", func(f *scopeFacts) { f.prevHead = "1111111" }, "could-not-determine"},
		{"the dispatched head is not a commit id", func(f *scopeFacts) { f.head = "main" }, "could-not-determine"},
		{"the previous verdict is at the dispatched head", func(f *scopeFacts) { f.prevHead = f.head }, "verdict-already-at-this-head"},
		{"the inter-head diff cannot be computed", func(f *scopeFacts) { f.diffErr = "the forge could not compare the two heads" }, "inter-head-diff-not-computable"},
		{"the interval holds no commit", func(f *scopeFacts) { f.commits = 0 }, "inter-head-diff-not-computable"},
		{"the head file list is empty", func(f *scopeFacts) { f.headFiles = nil }, "could-not-determine"},
		{"the delta has too many commits", func(f *scopeFacts) { f.commits = reviewDeltaMaxCommits + 1 }, "large-delta"},
		{"the delta touches too many of the change's paths", func(f *scopeFacts) {
			f.prevFiles, f.headFiles, f.delta = many(11), many(11), many(11)
		}, "large-delta"},
		{"a merge changed a file the change touches", func(f *scopeFacts) { f.merged = []string{"b.go"} }, "merge-in-files-the-change-touches"},
		{"a merge changed a file the change touched before", func(f *scopeFacts) {
			f.prevFiles = []string{"a.go", "b.go", "gone.go"}
			f.merged = []string{"gone.go"}
		}, "merge-in-files-the-change-touches"},
		{"the change touches a path it did not touch before", func(f *scopeFacts) {
			f.headFiles = []string{"a.go", "b.go", "new.go"}
		}, "path-not-reviewed-before"},
		{"large, merged and a new path together", func(f *scopeFacts) {
			f.commits = reviewDeltaMaxCommits + 1
			f.merged = []string{"a.go"}
			f.headFiles = []string{"a.go", "b.go", "new.go"}
		}, "large-delta,merge-in-files-the-change-touches,path-not-reviewed-before"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := deltaFacts()
			tc.edit(&f)
			d := decideReviewScope(f)
			if got := reasonsOf(d); got != tc.want {
				t.Errorf("reasons = %q, want %q", got, tc.want)
			}
			if d.full != (tc.want != "") {
				t.Errorf("full = %v with reasons %q — a full pass carries a reason and a delta carries none", d.full, tc.want)
			}
			if len(d.detail) != len(d.reasons) {
				t.Errorf("%d reason(s) but %d sentence(s)", len(d.reasons), len(d.detail))
			}
		})
	}
}

// The limits are inclusive: exactly at the limit is a delta, one over is a full pass. A
// merge or a delta path outside the change's own files never makes a full pass.
func TestScopeDecisionBoundaries(t *testing.T) {
	own := func(n int) []string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, fmt.Sprintf("f%02d.go", i))
		}
		return out
	}
	at := deltaFacts()
	at.commits = reviewDeltaMaxCommits
	at.prevFiles, at.headFiles, at.delta = own(reviewDeltaMaxPaths), own(reviewDeltaMaxPaths), own(reviewDeltaMaxPaths)
	if d := decideReviewScope(at); d.full {
		t.Errorf("exactly at both limits is FULL (%s) — the limits are 'more than'", reasonsOf(d))
	} else if d.commits != reviewDeltaMaxCommits || d.ownPaths != reviewDeltaMaxPaths {
		t.Errorf("stated size = %d commits, %d paths; want %d, %d", d.commits, d.ownPaths, reviewDeltaMaxCommits, reviewDeltaMaxPaths)
	}
	if reviewDeltaMaxCommits != 20 || reviewDeltaMaxPaths != 10 {
		t.Errorf("limits are %d commits, %d paths — the declared measure is 20 commits and 10 paths",
			reviewDeltaMaxCommits, reviewDeltaMaxPaths)
	}

	outside := deltaFacts()
	outside.delta = append(own(40), "a.go") // 40 paths of the base branch's, 1 of the change's
	outside.merged = own(40)
	if d := decideReviewScope(outside); d.full || d.ownPaths != 1 {
		t.Errorf("a merge of 40 files the change does not touch: full=%v (%s), own paths %d — want a delta of 1 own path",
			d.full, reasonsOf(d), d.ownPaths)
	}

	dropped := deltaFacts()
	dropped.headFiles = []string{"a.go"} // the change no longer touches b.go: nothing new to review
	if d := decideReviewScope(dropped); d.full {
		t.Errorf("a change that touches FEWER paths than before is FULL (%s)", reasonsOf(d))
	}
}

func TestLaneRoundCountsHeadsNotVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		heads  []string
		head   string
		number int
		first  string
		known  bool
	}{
		{"no earlier verdict", nil, rrHead1, 1, "", true},
		{"one earlier head", []string{rrHead1}, rrHead2, 2, rrHead1, true},
		{"three earlier heads", []string{rrHead1, rrHead2, rrHead3}, rrHead4, 4, rrHead1, true},
		{"two verdicts at one head are one round", []string{rrHead1, rrHead1, rrHead2}, rrHead3, 3, rrHead1, true},
		{"a dispatch at an already reviewed head re-opens that round", []string{rrHead1, rrHead2}, rrHead2, 2, rrHead1, true},
		{"case does not split a head", []string{strings.ToUpper("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1, strings.ToUpper("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), true},
		{"a verdict the forge does not pin", []string{rrHead1, ""}, rrHead3, 0, "", false},
		{"a dispatched head that is not a commit id", []string{rrHead1}, "abc123", 0, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := decideLaneRound(tc.heads, tc.head)
			if r.known != tc.known || r.number != tc.number || r.firstHead != tc.first {
				t.Errorf("round = %+v, want known=%v number=%d first=%q", r, tc.known, tc.number, tc.first)
			}
			if !r.known && r.why == "" {
				t.Error("an undetermined round says nothing about why")
			}
		})
	}
}

// A lane's verdicts are its own: the other lane's, another author's, a body carrying both
// lanes' lines, an unsubmitted review and a review with no verdict line are not counted. A
// DISMISSED verdict is counted: dismissing a review does not undo the round it was.
func TestLaneVerdictsAreTheLanesOwn(t *testing.T) {
	other := verdict(7, laneCorrectness, "approve", rrHead1)
	other.Author.Login = "someone-else"
	both := verdict(8, laneCorrectness, "approve", rrHead1, "Security-Review: pass")
	pending := verdict(9, laneCorrectness, "approve", rrHead1)
	pending.State = "PENDING"
	prose := deskkit.Review{ID: 10, Author: deskkit.Account{Login: rpReviewer}, State: "COMMENTED", CommitID: rrHead1, Body: "a note"}
	dismissed := verdict(5, laneCorrectness, "request-changes", rrHead3)
	dismissed.State = "DISMISSED"
	reviews := []deskkit.Review{
		verdict(1, laneCorrectness, "request-changes", rrHead1),
		verdict(2, laneSecurity, "fail", rrHead1),
		other, both, pending, prose,
		verdict(3, laneCorrectness, "approve", rrHead2),
		verdict(4, laneSecurity, "pass", rrHead3),
		dismissed,
	}
	ids := func(vs []laneVerdict) string {
		var out []string
		for _, v := range vs {
			out = append(out, fmt.Sprintf("%d@%s/%v", v.id, v.head[:1], v.blocking))
		}
		return strings.Join(out, " ")
	}
	if got, want := ids(laneVerdictsOf(reviews, rpReviewer, laneCorrectness)), "1@1/true 3@2/false 5@3/true"; got != want {
		t.Errorf("correctness lane's verdicts = %q, want %q", got, want)
	}
	if got, want := ids(laneVerdictsOf(reviews, rpReviewer, laneSecurity)), "2@1/true 4@3/false"; got != want {
		t.Errorf("security lane's verdicts = %q, want %q", got, want)
	}
	if got := laneVerdictsOf(reviews, rpReviewer, ""); len(got) != 0 {
		t.Errorf("an unknown lane was given %d verdict(s)", len(got))
	}
	if got := laneVerdictsOf(reviews, "", laneCorrectness); len(got) != 0 {
		t.Errorf("an unbound reviewer was given %d verdict(s)", len(got))
	}
}

// mergeAtHead makes the fixture's head a merge of rrMain into the branch at rrHead1. own is
// the merge commit's file list, which a forge reports against the FIRST parent; incoming is
// what the merged-in side changed, read as the comparison of the first parent with the second.
func mergeAtHead(f *fakeRoundForge, own, incoming []deskkit.ChangedFile) {
	f.compares[rrHead1+"..."+rrHead2].Commits = []deskkit.RepoCommit{
		{SHA: rrHead2, Parents: []string{rrHead1, rrMain}}, {SHA: rrMain, Parents: []string{rrBase}}}
	f.commits[rrHead2] = &deskkit.RepoCommit{SHA: rrHead2, Parents: []string{rrHead1, rrMain}, Files: own, FilesComplete: true}
	f.compares[rrHead1+"..."+rrMain] = &deskkit.RefComparison{Files: incoming, CommitsComplete: true}
}

// The read behind the decision: the fixture is a delta, and each way the forge can fall short
// lands on its own named full-pass reason.
func TestRoundReadDecidesScopeFromTheForge(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*fakeRoundForge)
		want string // "" = DELTA
	}{
		{"one commit touching one of the change's files", func(*fakeRoundForge) {}, ""},
		{"the lane has no verdict of its own — only the other lane's", func(f *fakeRoundForge) {
			f.reviews = []deskkit.Review{verdict(601, laneSecurity, "fail", rrHead1)}
		}, "no-verdict-of-its-own"},
		{"the lane's verdict is at this head", func(f *fakeRoundForge) { f.reviews[0].CommitID = rrHead2 }, "verdict-already-at-this-head"},
		{"the forge does not pin the previous verdict", func(f *fakeRoundForge) { f.reviews[0].CommitID = "" }, "could-not-determine"},
		{"the reviews cannot be read", func(f *fakeRoundForge) { f.reviewsErr = errors.New("503") }, "could-not-determine"},
		{"the head file list cannot be read", func(f *fakeRoundForge) { f.filesErr = errors.New("503") }, "could-not-determine"},
		{"the head file list is shorter than the forge's count", func(f *fakeRoundForge) { f.pr.ChangedFiles = 3 }, "could-not-determine"},
		{"the head file list is longer than the forge's count", func(f *fakeRoundForge) { f.pr.ChangedFiles = 1 }, "could-not-determine"},
		{"the change is answered with nothing", func(f *fakeRoundForge) { f.pr = nil }, "could-not-determine"},
		{"the previous head's comparison is answered with nothing", func(f *fakeRoundForge) {
			delete(f.compares, "main..."+rrHead1)
		}, "could-not-determine"},
		{"the two heads' comparison is answered with nothing", func(f *fakeRoundForge) {
			delete(f.compares, rrHead1+"..."+rrHead2)
		}, "inter-head-diff-not-computable"},
		{"an interval commit is answered with nothing", func(f *fakeRoundForge) { delete(f.commits, rrHead2) }, "inter-head-diff-not-computable"},
		{"the base branch is not known", func(f *fakeRoundForge) { f.pr.BaseRef = "" }, "could-not-determine"},
		{"the previous head's file list cannot be read", func(f *fakeRoundForge) {
			f.compareErr["main..."+rrHead1] = errors.New("404")
		}, "could-not-determine"},
		{"the previous head's file list may be cut short", func(f *fakeRoundForge) {
			var names []string
			for i := 0; i < forgeComparePageFiles; i++ {
				names = append(names, fmt.Sprintf("f%03d.go", i))
			}
			f.compares["main..."+rrHead1].Files = changed(names...)
		}, "could-not-determine"},
		{"the forge cannot compare the two heads", func(f *fakeRoundForge) {
			f.compareErr[rrHead1+"..."+rrHead2] = errors.New("unverifiable")
		}, "inter-head-diff-not-computable"},
		{"the interval's commit list is incomplete", func(f *fakeRoundForge) {
			f.compares[rrHead1+"..."+rrHead2].CommitsComplete = false
		}, "inter-head-diff-not-computable"},
		{"the head was rewritten — its chain does not reach the previous head", func(f *fakeRoundForge) {
			f.compares[rrHead1+"..."+rrHead2].Commits = []deskkit.RepoCommit{{SHA: rrHead2, Parents: []string{rrBase}}}
		}, "inter-head-diff-not-computable"},
		{"an interval commit carries no parent evidence", func(f *fakeRoundForge) {
			f.compares[rrHead1+"..."+rrHead2].Commits = []deskkit.RepoCommit{{SHA: rrHead2}}
		}, "inter-head-diff-not-computable"},
		{"a merged-in commit carries no parent evidence", func(f *fakeRoundForge) {
			f.compares[rrHead1+"..."+rrHead2].Commits = []deskkit.RepoCommit{
				{SHA: rrHead2, Parents: []string{rrHead1, rrMain}}, {SHA: rrMain}}
			f.commits[rrHead2] = &deskkit.RepoCommit{SHA: rrHead2, Parents: []string{rrHead1, rrMain},
				Files: changed("elsewhere.go"), FilesComplete: true}
		}, "inter-head-diff-not-computable"},
		{"an interval commit cannot be read", func(f *fakeRoundForge) { f.commitErr = errors.New("503") }, "inter-head-diff-not-computable"},
		{"an interval commit's file list is incomplete", func(f *fakeRoundForge) {
			f.commits[rrHead2].FilesComplete = false
		}, "inter-head-diff-not-computable"},
		// Exactly at the commit limit the delta is not yet large, so every commit is still read:
		// one that cannot be is a full pass, never a delta with no merge or path fact behind it.
		{"the delta is exactly 20 commits and one cannot be read", func(f *fakeRoundForge) {
			var cs []deskkit.RepoCommit
			parent := rrHead1
			for i := 0; i < reviewDeltaMaxCommits-1; i++ {
				sha := fmt.Sprintf("%040x", 0xc000+i)
				cs = append(cs, deskkit.RepoCommit{SHA: sha, Parents: []string{parent}})
				parent = sha
			}
			f.compares[rrHead1+"..."+rrHead2].Commits = append(cs, deskkit.RepoCommit{SHA: rrHead2, Parents: []string{parent}})
		}, "inter-head-diff-not-computable"},
		{"the delta is 21 commits", func(f *fakeRoundForge) {
			var cs []deskkit.RepoCommit
			parent := rrHead1
			for i := 0; i < reviewDeltaMaxCommits; i++ {
				sha := fmt.Sprintf("%040x", 0xa000+i)
				cs = append(cs, deskkit.RepoCommit{SHA: sha, Parents: []string{parent}})
				parent = sha
			}
			cs = append(cs, deskkit.RepoCommit{SHA: rrHead2, Parents: []string{parent}})
			f.compares[rrHead1+"..."+rrHead2].Commits = cs
		}, "large-delta"},
		{"the delta touches 11 of the change's paths", func(f *fakeRoundForge) {
			var names []string
			for i := 0; i < reviewDeltaMaxPaths+1; i++ {
				names = append(names, fmt.Sprintf("f%02d.go", i))
			}
			f.files, f.pr.ChangedFiles = changed(names...), len(names)
			f.compares["main..."+rrHead1].Files = changed(names...)
			f.commits[rrHead2].Files = changed(names...)
		}, "large-delta"},
		{"the head is a merge that changed a file the change touches", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("b.go", "elsewhere.go"), changed("elsewhere.go"))
		}, "merge-in-files-the-change-touches"},
		{"the head is a merge that changed, and brought in, only other files", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("elsewhere.go"), changed("elsewhere.go"))
		}, ""},
		// A conflict in one of the change's files resolved to the branch's side: the file is
		// byte-identical to the first parent, so the merge commit's own list does not name it.
		{"the merged-in side changed a file the change touches, and the branch's side was kept", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("elsewhere.go"), changed("b.go", "elsewhere.go"))
		}, "merge-in-files-the-change-touches"},
		{"the merged-in side changed a file under the name the change renamed it from", func(f *fakeRoundForge) {
			renamed := []deskkit.ChangedFile{{Filename: "a.go"}, {Filename: "b2.go", PreviousFilename: "b.go", Status: "renamed"}}
			f.files, f.compares["main..."+rrHead1].Files = renamed, renamed
			mergeAtHead(f, changed("elsewhere.go"), changed("b.go"))
		}, "merge-in-files-the-change-touches"},
		{"the merged-in side renamed a file the change touches", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("elsewhere.go"), []deskkit.ChangedFile{{Filename: "moved.go", PreviousFilename: "b.go", Status: "renamed"}})
		}, "merge-in-files-the-change-touches"},
		{"the merged-in side cannot be compared", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("elsewhere.go"), changed("elsewhere.go"))
			f.compareErr[rrHead1+"..."+rrMain] = errors.New("503")
		}, "could-not-determine"},
		{"the merged-in side's comparison is answered with nothing", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("elsewhere.go"), nil)
			delete(f.compares, rrHead1+"..."+rrMain)
		}, "could-not-determine"},
		{"the merged-in side's file list may be cut short", func(f *fakeRoundForge) {
			var names []string
			for i := 0; i < forgeComparePageFiles; i++ {
				names = append(names, fmt.Sprintf("m%03d.go", i))
			}
			mergeAtHead(f, changed("elsewhere.go"), changed(names...))
		}, "could-not-determine"},
		{"an octopus merge whose third parent changed a file the change touches", func(f *fakeRoundForge) {
			mergeAtHead(f, changed("elsewhere.go"), changed("elsewhere.go"))
			f.compares[rrHead1+"..."+rrHead2].Commits[0].Parents = []string{rrHead1, rrMain, rrHead3}
			f.compares[rrHead1+"..."+rrHead2].Commits = append(f.compares[rrHead1+"..."+rrHead2].Commits, deskkit.RepoCommit{SHA: rrHead3, Parents: []string{rrBase}})
			f.compares[rrHead1+"..."+rrHead3] = &deskkit.RefComparison{Files: changed("a.go"), CommitsComplete: true}
		}, "merge-in-files-the-change-touches"},
		{"the change now touches a path it did not touch before", func(f *fakeRoundForge) {
			f.files, f.pr.ChangedFiles = changed("a.go", "b.go", "new.go"), 3
			f.commits[rrHead2].Files = changed("new.go")
		}, "path-not-reviewed-before"},
		{"a renamed file's new name is a new path", func(f *fakeRoundForge) {
			f.files = []deskkit.ChangedFile{{Filename: "a.go"}, {Filename: "b2.go", PreviousFilename: "b.go", Status: "renamed"}}
		}, "path-not-reviewed-before"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := roundFixture()
			tc.edit(f)
			rr, err := roundOf(t, f, rrKeyC)
			if err != nil {
				t.Fatalf("held: %v", err)
			}
			if got := reasonsOf(rr.scope); got != tc.want || rr.scope.full != (tc.want != "") {
				t.Errorf("scope: full=%v reasons=%q, want reasons %q", rr.scope.full, got, tc.want)
			}
			for _, c := range f.calls {
				if strings.HasPrefix(c, "GetCommit "+rrMain) {
					t.Error("a commit reachable only through a merge's second parent was read")
				}
			}
		})
	}
}

// A first round reads no comparison and no commit; a delta already too long reads no commit.
func TestRoundReadStopsAtTheFirstDecidingFact(t *testing.T) {
	first := roundFixture()
	first.reviews = nil
	if rr, _ := roundOf(t, first, rrKeyC); !rr.scope.full || first.called("CompareRefs")+first.called("GetCommit")+first.called("ListChangedFiles") != 0 {
		t.Errorf("first round: full=%v, calls %v — want a full pass and no file, comparison or commit read", rr.scope.full, first.calls)
	}
	noBase := roundFixture()
	noBase.pr.BaseRef = ""
	if rr, _ := roundOf(t, noBase, rrKeyC); reasonsOf(rr.scope) != "could-not-determine" || noBase.called("CompareRefs") != 0 {
		t.Errorf("unknown base branch: reasons %q, calls %v — want could-not-determine and no comparison", reasonsOf(rr.scope), noBase.calls)
	}
	long := roundFixture()
	var cs []deskkit.RepoCommit
	parent := rrHead1
	for i := 0; i < reviewDeltaMaxCommits; i++ {
		sha := fmt.Sprintf("%040x", 0xb000+i)
		cs = append(cs, deskkit.RepoCommit{SHA: sha, Parents: []string{parent}})
		parent = sha
	}
	long.compares[rrHead1+"..."+rrHead2].Commits = append(cs, deskkit.RepoCommit{SHA: rrHead2, Parents: []string{parent}})
	if rr, _ := roundOf(t, long, rrKeyC); reasonsOf(rr.scope) != "large-delta" || long.called("GetCommit") != 0 {
		t.Errorf("21-commit delta: reasons %q, %d commit read(s) — want large-delta and none", reasonsOf(rr.scope), long.called("GetCommit"))
	}
}

// Each lane's scope and round come from that lane's own verdicts only.
func TestRoundIsPerLane(t *testing.T) {
	build := func() *fakeRoundForge {
		f := roundFixture()
		f.pr.HeadSHA = rrHead4
		f.reviews = []deskkit.Review{
			verdict(501, laneCorrectness, "request-changes", rrHead1),
			verdict(502, laneCorrectness, "request-changes", rrHead2),
			verdict(503, laneCorrectness, "request-changes", rrHead3),
			verdict(601, laneSecurity, "fail", rrHead3),
		}
		f.compares["main..."+rrHead3] = &deskkit.RefComparison{Files: changed("a.go", "b.go"), CommitsComplete: true}
		f.compares[rrHead3+"..."+rrHead4] = &deskkit.RefComparison{CommitsComplete: true,
			Commits: []deskkit.RepoCommit{{SHA: rrHead4, Parents: []string{rrHead3}}}}
		f.commits[rrHead4] = &deskkit.RepoCommit{SHA: rrHead4, Parents: []string{rrHead3}, Files: changed("a.go"), FilesComplete: true}
		return f
	}
	c, err := roundOf(t, build(), rrKeyC)
	if err != nil {
		t.Fatal(err)
	}
	if !c.round.known || c.round.number != 4 || c.round.firstHead != rrHead1 || c.scope.full || c.prev == nil || c.prev.id != 503 {
		t.Errorf("correctness lane: round %+v, full=%v, prev %+v — want round 4 from %s, DELTA from review 503", c.round, c.scope.full, c.prev, rrHead1)
	}
	s, err := roundOf(t, build(), rrKeyS)
	if err != nil {
		t.Fatal(err)
	}
	if !s.round.known || s.round.number != 2 || s.round.firstHead != rrHead3 || s.scope.full || s.prev == nil || s.prev.id != 601 {
		t.Errorf("security lane: round %+v, full=%v, prev %+v — want round 2 from %s, DELTA from review 601", s.round, s.scope.full, s.prev, rrHead3)
	}
	// A claim key that names no single lane: nothing is attributed to it.
	n, err := roundOf(t, build(), "tracker--pr-77--fact-check")
	if err != nil {
		t.Fatal(err)
	}
	if n.round.known || !n.scope.full || reasonsOf(n.scope) != "could-not-determine" {
		t.Errorf("lane-less key: round %+v, scope %q — want not determined and could-not-determine", n.round, reasonsOf(n.scope))
	}
}

// Could-not-determine, end to end: no forge wired, a forge that cannot be reached, a change
// that cannot be read, a reviewer that is not bound. Each dispatches (no error) with a full
// pass and the round not determined.
func TestRoundReadFailureIsAFullPassNeverAHold(t *testing.T) {
	t.Run("no forge read is wired", func(t *testing.T) {
		old := reviewRoundForgeFn
		reviewRoundForgeFn = nil
		t.Cleanup(func() { reviewRoundForgeFn = old })
		rr, err := prepareReviewRound(dispatchOpts{kit: "review", pr: 77}, dispatchPlan{claimKey: rrKeyC}, rrRepo)
		if err != nil || rr.read {
			t.Fatalf("round = %+v, err %v — want the zero round and no hold", rr, err)
		}
	})
	for _, tc := range []struct {
		name string
		wire func(t *testing.T)
	}{
		{"the forge cannot be reached", func(t *testing.T) {
			old := reviewRoundForgeFn
			reviewRoundForgeFn = func(string) (reviewRoundForge, deskkit.ForgeRepo, error) {
				return nil, deskkit.ForgeRepo{}, errors.New("no credential")
			}
			t.Cleanup(func() { reviewRoundForgeFn = old })
		}},
		{"the change cannot be read", func(t *testing.T) {
			f := roundFixture()
			f.pr, f.prErr = nil, errors.New("503")
			useRoundForge(t, f)
		}},
		{"the reviewer identity is not bound", func(t *testing.T) {
			useRoundForge(t, roundFixture())
			useReviewer(t, "", false)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.wire(t)
			var rr reviewRound
			var err error
			stderr := captureStderr(t, func() {
				rr, err = prepareReviewRound(dispatchOpts{kit: "review", pr: 77}, dispatchPlan{claimKey: rrKeyC}, rrRepo)
			})
			if err != nil {
				t.Fatalf("a failed read held the dispatch: %v", err)
			}
			if !rr.scope.full || reasonsOf(rr.scope) != "could-not-determine" || rr.round.known {
				t.Errorf("scope %q full=%v, round %+v — want could-not-determine, full, not determined", reasonsOf(rr.scope), rr.scope.full, rr.round)
			}
			if !strings.Contains(stderr, "review round:") || !strings.Contains(stderr, "dispatching") {
				t.Errorf("the failed read was not reported on stderr: %q", stderr)
			}
		})
	}
}

func roundText(rr reviewRound) string {
	var b strings.Builder
	writeReviewRound(&b, rr)
	return b.String()
}

// What the assignment says: the round number and first-review head, the scope, and — on a
// delta — the previous verdict, both heads, and the findings to answer.
func TestAssignmentStatesRoundAndScope(t *testing.T) {
	f := roundFixture()
	f.pr.HeadSHA = rrHead4
	f.reviews = []deskkit.Review{
		verdict(501, laneCorrectness, "request-changes", rrHead1),
		verdict(502, laneCorrectness, "request-changes", rrHead2),
		verdict(503, laneCorrectness, "request-changes", rrHead3, findingBlock("F-2", "F-1", "bad id `x`")),
	}
	f.compares["main..."+rrHead3] = &deskkit.RefComparison{Files: changed("a.go", "b.go"), CommitsComplete: true}
	f.compares[rrHead3+"..."+rrHead4] = &deskkit.RefComparison{CommitsComplete: true,
		Commits: []deskkit.RepoCommit{{SHA: rrHead4, Parents: []string{rrHead3}}}}
	f.commits[rrHead4] = &deskkit.RepoCommit{SHA: rrHead4, Parents: []string{rrHead3}, Files: changed("a.go", "README.md"), FilesComplete: true}
	rr, err := roundOf(t, f, rrKeyC)
	if err != nil {
		t.Fatal(err)
	}
	got := roundText(rr)
	for _, want := range []string{
		"### This round — stated by the dispatcher\n",
		"- Lane: correctness. Round: 4. This lane's FIRST review was at head `" + rrHead1 + "`.\n",
		"- Scope: DELTA (clause 19) — from this lane's previous verdict (review 503, at head `" + rrHead3 + "`) to head `" + rrHead4 + "`: 1 commit(s) touching 1 of the change's own path(s).\n",
		"- Findings to answer, each resolved / not resolved with evidence: 3 in review 503's typed block — `F-1`, `F-2` — 1 more whose ids are not printed here; read them in the review.\n",
		"- The delta: `git diff " + rrHead3 + " " + rrHead4 + "`. If that command fails, or you find this scope wrong, do the full pass and say so.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("assignment lacks %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "bad id") || strings.Contains(got, "README.md") || strings.Contains(got, "a.go") {
		t.Errorf("the assignment carries text from the change (a path, or an id of unexpected shape):\n%s", got)
	}

	// A full pass names its reason; a first round says the lane has not reviewed before.
	f2 := roundFixture()
	f2.reviews = nil
	rr2, _ := roundOf(t, f2, rrKeyS)
	got2 := roundText(rr2)
	for _, want := range []string{
		"- Lane: security. Round: 1 — this lane has not reviewed the change before.\n",
		"- Scope: FULL PASS — reason `no-verdict-of-its-own`: this lane has no verdict of its own on the change.\n",
	} {
		if !strings.Contains(got2, want) {
			t.Errorf("first-round assignment lacks %q\n--- got ---\n%s", want, got2)
		}
	}
	if strings.Contains(got2, "DELTA") {
		t.Errorf("a full-pass assignment mentions a delta:\n%s", got2)
	}

	// Not determined: the cap is said not to apply, and the scope is a full pass.
	for name, zero := range map[string]reviewRound{
		"nothing read": {},
		"round unknown": {read: true, lane: laneCorrectness, round: laneRound{why: "the reviews could not be read"},
			scope: decideReviewScope(scopeFacts{unknown: "the reviews could not be read"})},
	} {
		got := roundText(zero)
		if !strings.Contains(got, "Round: NOT DETERMINED") || !strings.Contains(got, "Clause 20's lane round cap does not apply.") ||
			!strings.Contains(got, "Scope: FULL PASS — reason `could-not-determine`") {
			t.Errorf("%s: assignment does not say the round is undetermined, the cap inert and the scope full:\n%s", name, got)
		}
	}
}

// The body with no typed block, and with an empty one.
func TestAssignmentFindingsLine(t *testing.T) {
	prev := laneVerdict{id: 9, head: rrHead1, body: "Verdict: request-changes\n\n1. The table is wrong.\n"}
	rr := reviewRound{read: true, lane: laneCorrectness, head: rrHead2, prev: &prev,
		round: laneRound{known: true, number: 2, firstHead: rrHead1}, scope: scopeDecision{commits: 1, ownPaths: 1}}
	if got := roundText(rr); !strings.Contains(got, "review 9 carries no readable typed finding block — answer every finding its body states.") {
		t.Errorf("prose-only previous verdict:\n%s", got)
	}
	prev.body = "Verdict: approve\n\n" + deskkit.RenderFindingBlock(deskkit.FindingBlockV1{})
	if got := roundText(rr); !strings.Contains(got, "review 9's typed block lists none.") {
		t.Errorf("empty typed block:\n%s", got)
	}
}

// End to end through run(): a real (stubbed) review dispatch writes the round into the
// prompt, between the head fetch and the claim release; --dry-run reads nothing and says so.
func TestDispatchedReviewPromptCarriesTheRound(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	pinFixtureForge(t, home, allowedRepo)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "review-home"))
	t.Setenv("DESK_LOOP", "pr-review-desk")
	installGHStamp(t)
	stubQueueLabel(t, nil)
	f := roundFixture()
	useRoundForge(t, f)

	args := stampArgs(t, root, "--kit", "review", "--quiet")
	promptFile := promptFileOf(t, args)
	rc, stderr := runCapturingStderr(t, args)
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0\n%s", rc, stderr)
	}
	raw, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(raw)
	fetch := strings.Index(prompt, "checkout FETCH_HEAD")
	round := strings.Index(prompt, "### This round — stated by the dispatcher")
	release := strings.Index(prompt, "Release the dispatch claim once your verdict is posted")
	if fetch < 0 || round < fetch || release < round {
		t.Fatalf("round statement is not between the head fetch and the claim release (fetch %d, round %d, release %d)", fetch, round, release)
	}
	for _, want := range []string{
		"- Lane: correctness. Round: 2. This lane's FIRST review was at head `" + rrHead1 + "`.",
		"- Scope: DELTA (clause 19) — from this lane's previous verdict (review 501, at head `" + rrHead1 + "`) to head `" + rrHead2 + "`",
		"`F-1`, `F-2`",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Count(prompt, "### This round — stated by the dispatcher") != 1 {
		t.Error("the round is stated more than once")
	}

	dry := dispatchPrompt(t, "review", "assay--pr-547", "--pr", "547")
	if !strings.Contains(dry, "- Round: NOT DETERMINED (no forge read ran for this dispatch). Clause 20's lane round cap does not apply.") ||
		!strings.Contains(dry, "- Scope: FULL PASS — reason `could-not-determine`: no forge read ran for this dispatch.") {
		t.Errorf("--dry-run prompt does not state an undetermined round and a full pass")
	}
	if n := f.called("GetPullRequest"); n != 1 {
		t.Errorf("the round read the change %d time(s) across one dispatch and one --dry-run — want 1 (none on --dry-run)", n)
	}
}

// A worker dispatch carries no round statement and makes no round read, even when it names
// a change.
func TestNonReviewDispatchHasNoRound(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	pinFixtureForge(t, home, allowedRepo)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	t.Setenv("DESK_LOOP", "pr-review-desk")
	installGHStamp(t)
	f := roundFixture()
	useRoundForge(t, f)

	args := stampArgs(t, root)
	if rc := run(args); rc != deskkit.ExitOK {
		t.Fatalf("worker dispatch rc = %d", rc)
	}
	raw, err := os.ReadFile(promptFileOf(t, args))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "stated by the dispatcher") {
		t.Error("a worker prompt carries a review round statement")
	}
	if len(f.calls) != 0 {
		t.Errorf("a worker dispatch made round reads: %v", f.calls)
	}
}

// The safety-relevant exception is stated ONCE in the kit and reaches BOTH lane cuts, the
// whole kit, and the prompt each lane is actually dispatched with.
func TestSafetyExceptionIsInBothLaneCuts(t *testing.T) {
	const exception = "**Safety-relevant exception:** it still blocks, with no hand-off, when it is any security-lane fail class, " +
		"a weakening of a control or its assertion, data loss, or exposure of withheld content."
	fold := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for _, lane := range []string{laneCorrectness, laneSecurity, ""} {
		cut := fold(reviewCut(t, lane))
		if n := strings.Count(cut, fold(exception)); n != 1 {
			t.Errorf("lane %q cut states the safety-relevant exception %d time(s), want exactly 1", lane, n)
		}
		if n := strings.Count(cut, "Safety-relevant exception"); n != 1 {
			t.Errorf("lane %q cut names the exception %d time(s), want exactly 1", lane, n)
		}
		for _, want := range []string{
			"## 19. Delta round — only when the assignment states `Scope: DELTA`",
			"**do the full pass and say so in the verdict.**",
			"## 20. Lane round cap — from this lane's fourth round",
			"A finding on code changed since the first review blocks as before.",
			"State, for each late finding, which of these classes it is in and why.",
		} {
			if !strings.Contains(cut, fold(want)) {
				t.Errorf("lane %q cut lacks %q", lane, want)
			}
		}
	}
	for _, item := range []string{"assay--pr-547", "assay--pr-547--security"} {
		prompt := fold(dispatchPrompt(t, "review", item, "--pr", "547"))
		if n := strings.Count(prompt, fold(exception)); n != 1 {
			t.Errorf("%s: the dispatched prompt states the exception %d time(s), want exactly 1", item, n)
		}
	}
}

// The words the assignment uses are the words the kit's clauses key on.
func TestAssignmentAndKitUseTheSameWords(t *testing.T) {
	kit := reviewCut(t, "")
	delta := roundText(reviewRound{read: true, lane: laneSecurity, head: rrHead2,
		prev:  &laneVerdict{id: 1, head: rrHead1, body: "Security-Review: fail"},
		round: laneRound{known: true, number: 2, firstHead: rrHead1}, scope: scopeDecision{commits: 1, ownPaths: 1}})
	full := roundText(reviewRound{})
	for _, w := range []string{"Scope: DELTA", "Scope: FULL PASS"} {
		if !strings.Contains(kit, "`"+w+"`") {
			t.Errorf("the kit does not name %q", w)
		}
	}
	if !strings.Contains(delta, "Scope: DELTA") || strings.Contains(delta, "Scope: FULL PASS") {
		t.Errorf("a delta assignment does not say `Scope: DELTA` alone:\n%s", delta)
	}
	if !strings.Contains(full, "Scope: FULL PASS") || strings.Contains(full, "Scope: DELTA") {
		t.Errorf("a full-pass assignment does not say `Scope: FULL PASS` alone:\n%s", full)
	}
}

// The forge surface the round and the gate can reach has no method that writes.
func TestReviewRoundForgeIsReadOnly(t *testing.T) {
	reads := regexp.MustCompile(`^(Get[A-Z]|List[A-Z]|ReviewsAtHead$|ChecksAtHead$|RequiredStatusChecks$|PRTrustEvents$|IssueTrustEvents$|CompareRefs$)`)
	typ := reflect.TypeOf((*reviewRoundForge)(nil)).Elem()
	if typ.NumMethod() == 0 {
		t.Fatal("the interface has no methods")
	}
	for i := 0; i < typ.NumMethod(); i++ {
		if name := typ.Method(i).Name; !reads.MatchString(name) {
			t.Errorf("reviewRoundForge.%s is not a known read — the round and the gate must not be able to write", name)
		}
	}
	// The same methods exist on the full forge, with the same signatures.
	var _ reviewRoundForge = deskkit.Forge(nil)
}

// The shipped binary wires the live read in main(); nothing else assigns the seam.
func TestMainWiresTheReviewRoundRead(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\treviewRoundForgeFn = liveReviewRoundForge\n") {
		t.Error("main() does not wire reviewRoundForgeFn = liveReviewRoundForge: the shipped binary would never hold and never scope a round")
	}
	if reviewRoundForgeFn != nil {
		t.Error("reviewRoundForgeFn is set outside main(): tests that do not ask for it would read a forge")
	}
}

// The round and the gate take no input but the dispatch's own and the forge's: the three
// files read no environment variable and define no flag, so nothing set outside the command
// line can skip the gate or narrow a scope. A source scan — it sees a direct read only, not
// one made through another package.
func TestReviewRoundReadsNoEnvironmentAndDefinesNoFlag(t *testing.T) {
	for _, name := range []string{"reviewround.go", "reviewroundread.go", "reviewgate.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"Getenv", "LookupEnv", "Environ", "ExpandEnv", "\"flag\"", "flag.", "pflag", "os.Args"} {
			if strings.Contains(string(src), banned) {
				t.Errorf("%s contains %q: the round and the gate must not read the environment or define a flag", name, banned)
			}
		}
	}
}

// The skill and the README name the code's own reasons, limits and marker, and the skill
// states the safety-relevant exception once. A document that drifts from a constant fails here.
func TestReviewRoundDocsNameTheCodesOwnWords(t *testing.T) {
	fold := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return fold(string(b))
	}
	skill := read(scanRefusalSkillPath)
	readme := read("../../README.md")

	const exception = "**Safety-relevant exception:** it still blocks, with no hand-off, when it is any security-lane fail class, " +
		"a weakening of a control or its assertion, data loss, or exposure of withheld content."
	if n := strings.Count(skill, exception); n != 1 {
		t.Errorf("the skill states the safety-relevant exception %d time(s), want exactly 1", n)
	}
	if n := strings.Count(skill, "Safety-relevant exception"); n != 1 {
		t.Errorf("the skill names the exception %d time(s), want exactly 1", n)
	}

	// The skill restates kit clauses 19 and 20 for the desk. Each restated rule is held here,
	// so the copy cannot drift from the clause: the round the cap binds from, what a delta
	// round covers, what makes a stated scope wrong, and what the cap never touches.
	for _, want := range []string{
		"From round four, a finding first raised in that round, in code unchanged since that first-review head, is advisory",
		"\"Any security-lane fail class\" is any finding the security lane would fail the change on",
		"a late finding the reviewer cannot place with confidence blocks",
		"the cap never changes a security verdict",
		"A finding whose evidence did not exist at the first-review head is not a late finding",
		"kit clauses 2, 5, 7, 8 and 15 bind in every round",
		"everything that verdict recorded as could-not-check, not run or incomplete",
		"the head-level duties, which stand in every round",
		"or its merged-in side did, whichever way a conflict was resolved",
		"a different head than the forge records for it or carries a typed block that cannot be read",
		"names another head than the assignment gives for it, or calls itself incomplete",
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("the skill does not state %q", want)
		}
	}
	for _, want := range []string{
		"or whose merged-in side changed one, whichever way a conflict was resolved",
		"a different head than the forge records for it",
		"or carries a typed finding block that cannot be read, the scope is `could-not-determine`",
		"A verdict that names no full commit id in those places changes nothing",
		"under the dispatcher role's existing credential",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("the README does not state %q", want)
		}
	}

	limits := fmt.Sprintf("more than %d commits, or more than %d of the", reviewDeltaMaxCommits, reviewDeltaMaxPaths)
	longLimits := fmt.Sprintf("more than %d commits, or touches more than %d of the", reviewDeltaMaxCommits, reviewDeltaMaxPaths)
	for doc, want := range map[string]string{"skill": limits, "README": longLimits} {
		text := map[string]string{"skill": skill, "README": readme}[doc]
		if !strings.Contains(text, want) {
			t.Errorf("the %s does not state the large-delta limits as %q", doc, want)
		}
	}
	for _, r := range []gateReason{holdRequiredCheckRed, holdStaleDescription, holdDecisionNotRuled, holdLaneAwaitsRuling} {
		for doc, text := range map[string]string{"skill": skill, "README": readme} {
			if !strings.Contains(text, "| `"+string(r)+"` |") {
				t.Errorf("the %s has no table row for gate reason %q", doc, r)
			}
		}
	}
	for _, r := range []scopeReason{reasonUndetermined, reasonNoVerdict, reasonSameHead, reasonNoDiff, reasonLargeDelta, reasonMergeInOwnFiles, reasonNewPath} {
		if !strings.Contains(readme, "| `"+string(r)+"` |") {
			t.Errorf("the README has no table row for scope reason %q", r)
		}
	}
	for doc, text := range map[string]string{"skill": skill, "README": readme} {
		if !strings.Contains(text, heldMarker) {
			t.Errorf("the %s does not name the hold's first line (%q)", doc, heldMarker)
		}
		if !strings.Contains(text, "`"+decisionLabel+"`") {
			t.Errorf("the %s does not name the decision label", doc)
		}
	}
	for _, spec := range []string{"reviewround-mutations.json", "reviewgate-mutations.json"} {
		if _, err := os.Stat(spec); err != nil {
			t.Errorf("the README cites %s, which is not beside this test: %v", spec, err)
		}
		if !strings.Contains(readme, "cmd/deskdispatch/"+spec) {
			t.Errorf("the README does not cite %s", spec)
		}
	}
}

// A round that carries no previous verdict is never written as a delta, whatever its scope
// field says: there is nothing to measure a delta from.
func TestAssignmentNeverStatesADeltaWithoutAPreviousVerdict(t *testing.T) {
	var b strings.Builder
	writeReviewRound(&b, reviewRound{read: true, lane: "correctness", head: rrHead2})
	got := b.String()
	if !strings.Contains(got, "- Scope: FULL PASS — reason `could-not-determine`: no previous verdict to measure a delta from.") {
		t.Errorf("a round with no previous verdict is not stated as a full pass:\n%s", got)
	}
	if strings.Contains(got, "DELTA") {
		t.Errorf("a round with no previous verdict is stated as a delta:\n%s", got)
	}
}

// fourRoundFixture is roundFixture moved on to rrHead4, with one correctness verdict at each
// of the three earlier heads: round 4, a delta from review 503.
func fourRoundFixture() *fakeRoundForge {
	f := roundFixture()
	f.pr.HeadSHA = rrHead4
	f.reviews = []deskkit.Review{
		verdict(501, laneCorrectness, "request-changes", rrHead1),
		verdict(502, laneCorrectness, "request-changes", rrHead2),
		verdict(503, laneCorrectness, "request-changes", rrHead3),
	}
	f.compares["main..."+rrHead3] = &deskkit.RefComparison{Files: changed("a.go", "b.go"), CommitsComplete: true}
	f.compares[rrHead3+"..."+rrHead4] = &deskkit.RefComparison{CommitsComplete: true,
		Commits: []deskkit.RepoCommit{{SHA: rrHead4, Parents: []string{rrHead3}}}}
	f.commits[rrHead4] = &deskkit.RepoCommit{SHA: rrHead4, Parents: []string{rrHead3}, Files: changed("a.go"), FilesComplete: true}
	return f
}

// What a verdict's own record names as a head: the three text shapes followed by a full commit
// id, and the typed block's two fields. Nothing else in the body is read, and an id is
// compared without regard to letter case.
func TestLaneVerdictsNameTheirHeads(t *testing.T) {
	const (
		a = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		b = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		c = "cccccccccccccccccccccccccccccccccccccccc"
		l = "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef0123" // 64 hex digits
	)
	for _, tc := range []struct {
		name, body        string
		reviewed, origins string
		unreadable        bool
	}{
		{"nothing named", "Verdict: approve\n\nNo finding.", "", "", false},
		{"Head reviewed, in a sentence", "Verdict: approve\n\nLane: correctness, first review. Head reviewed: `" + a + "` (two commits).", a, "", false},
		{"Head reviewed, emphasised, no backtick", "**Head reviewed:** " + a, a, "", false},
		{"Head opening a line", "Verdict: approve\nHead: " + a + "\n", a, "", false},
		{"Head opening a list, quote or emphasised line", "- Head: `" + a + "`\n> **Head:** `" + b + "`\n* head: " + c, a + " " + b + " " + c, "", false},
		{"Head not opening its line", "The base's Head: " + a + " and overhead: " + b, "", "", false},
		{"at head on the opening line", "Security lane, first review, full pass at head `" + a + "`.\n\nSecurity-Review: fail", a, "", false},
		{"at head on the opening line, after the verdict line", "\nVerdict: approve\n\nFull pass at head " + a + ".", a, "", false},
		{"at head below the opening line", "Verdict: approve\n\nA re-review.\n\nFirst raised at head `" + a + "`.", "", "", false},
		{"a sha-256 id", "Head: " + l, l, "", false},
		{"too short, too long, not hex", "Head: aaaaaaa\nHead: " + a + "a\nHead reviewed: `" + a[:39] + "g`", "", "", false},
		{"upper case", "HEAD REVIEWED: `" + strings.ToUpper(a) + "`", strings.ToUpper(a), "", false},
		{"other ids", "Verdict: approve\n\nmain is `" + a + "`, merge-base " + b + ". Input revision: `" + c + "`.", "", "", false},
		{"the typed block", "Verdict: request-changes\n\n" + headedBlock(a, b), b, a, false},
		{"the typed block, origin only", "Verdict: approve\n\n" + headedBlock(a, ""), "", a, false},
		{"the typed block, abbreviated", "Verdict: approve\n\n" + headedBlock("aaaaaaa", "bbbbbbb"), "", "", false},
		{"text and typed block", "Head: " + a + "\n\nVerdict: request-changes\n\n" + headedBlock(c, b), a + " " + b, c, false},
		{"a typed block that cannot be read", "Verdict: approve\n\n<!-- assay:review-finding:v1\n{\"findings\":[{\"evidenceHead\":\"" + a + "\"\n-->", "", "", true},
		{"a typed block that is not closed", "Head: " + a + "\n<!-- assay:review-finding:v1\n{}", a, "", true},
	} {
		reviewed, origins, readable := verdictNamedHeads(tc.body)
		if readable == tc.unreadable {
			t.Errorf("%s: readable = %v, want %v", tc.name, readable, !tc.unreadable)
		}
		if got := strings.Join(reviewed, " "); got != tc.reviewed {
			t.Errorf("%s: heads named as reviewed = %q, want %q", tc.name, got, tc.reviewed)
		}
		if got := strings.Join(origins, " "); got != tc.origins {
			t.Errorf("%s: origin heads = %q, want %q", tc.name, got, tc.origins)
		}
	}
	at := func(id int64, head, body string) laneVerdict { return laneVerdict{id: id, head: head, body: body} }
	for _, tc := range []struct {
		name     string
		verdicts []laneVerdict
		want     string // the review id the answer names; "" = not disputed
	}{
		{"no verdict", nil, ""},
		{"nothing named", []laneVerdict{at(1, a, "Verdict: approve")}, ""},
		{"agrees, in another letter case", []laneVerdict{at(1, a, "Head: "+strings.ToUpper(a)), at(2, strings.ToUpper(b), headedBlock(a, b))}, ""},
		{"the second of three is disputed", []laneVerdict{at(1, a, "Head: "+a), at(2, b, "Head: "+c), at(3, c, "Head: "+a)}, "review 2 "},
		{"an origin head of a later verdict", []laneVerdict{at(1, a, headedBlock(b, a)), at(2, b, "")}, "review 1 "},
		{"a typed block that cannot be read", []laneVerdict{at(1, a, "Head: "+a), at(2, b, "Head: "+b+"\n<!-- assay:review-finding:v1\n{}")}, "review 2 carries a typed finding block that cannot be read"},
	} {
		got := disputedVerdictHead(tc.verdicts)
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: disputed = %q, want it to name %q", tc.name, got, tc.want)
		}
	}
}

// headedBlock is a typed finding block of one finding that records the two heads a finding
// carries: where it was first raised, and where this record's evidence was gathered.
func headedBlock(origin, evidence string) string {
	return deskkit.RenderFindingBlock(deskkit.FindingBlockV1{Findings: []deskkit.Finding{{ID: "H-1", Class: "c", Severity: deskkit.SeverityBlocking,
		Blocker: deskkit.BlockerCodeContent, State: deskkit.StateOpen, Failure: "it fails", OriginHead: origin, EvidenceHead: evidence}}})
}

// The forge's record of the commit a verdict was submitted at is what the delta base, the
// round count and the first-review head are read from. Where a counted verdict's own record
// names a full commit id as the head it reviewed — on a recognised head line of its text, or
// in its typed block — the two must agree: a disagreement on ANY of the lane's verdicts is
// could-not-determine — a full pass, the round not determined. A record that names no full
// id in a recognised place changes nothing.
func TestRoundReadComparesTheHeadAVerdictNamesWithTheForges(t *testing.T) {
	say := func(f *fakeRoundForge, i int, lines ...string) *fakeRoundForge {
		f.reviews[i].Body += "\n" + strings.Join(lines, "\n")
		return f
	}
	body := func(f *fakeRoundForge, i int, text string) *fakeRoundForge {
		f.reviews[i].Body = text
		return f
	}
	for _, tc := range []struct {
		name  string
		f     *fakeRoundForge
		want  string // scope reasons; "" = DELTA
		round int    // 0 = not determined
		first string
	}{
		// --- the text ---
		{"the body names the head the forge records", say(roundFixture(), 0, "Head reviewed: `"+rrHead1+"` (one commit)."), "", 2, rrHead1},
		{"every body of four rounds names the forge's head",
			say(say(say(fourRoundFixture(), 0, "Head: "+rrHead1), 1, "Correctness lane, round 2. Head reviewed: `"+rrHead2+"`"), 2, "- **Head:** `"+rrHead3+"`"), "", 4, rrHead1},
		{"the opening line names the forge's head", body(roundFixture(), 0,
			"Correctness lane, first review, full pass at head `"+rrHead1+"`.\n\nVerdict: request-changes\n"), "", 2, rrHead1},
		{"the body names no full commit id", say(roundFixture(), 0, "Head reviewed: `1111111` (abbreviated)."), "", 2, rrHead1},
		{"a full id is named, in no recognised place", say(roundFixture(), 0,
			"The finding first raised at head `"+rrBase+"` stands; main is `"+rrMain+"`, merge-base `"+rrBase+"`.",
			"Input revision: `"+rrBase+"`. Overhead: "+rrBase+"; ahead: `"+rrBase+"`",
			"Head: "+rrBase+"0 is one digit too long to be a commit id"), "", 2, rrHead1},
		{"the LATEST verdict's body names another head", say(roundFixture(), 0, "Head reviewed: `"+rrBase+"`"), "could-not-determine", 0, ""},
		{"the latest of four names another head", say(fourRoundFixture(), 2, "Head: "+rrHead2), "could-not-determine", 0, ""},
		{"the FIRST verdict's body names another head", say(fourRoundFixture(), 0, "Head reviewed: `"+rrBase+"`"), "could-not-determine", 0, ""},
		{"a middle verdict's body names another head", say(fourRoundFixture(), 1, "> head: `"+rrHead1+"`"), "could-not-determine", 0, ""},
		{"the opening line names another head", body(roundFixture(), 0,
			"Verdict: request-changes\n\nCorrectness lane, first review, full pass at head `"+rrBase+"`.\n"), "could-not-determine", 0, ""},
		{"one recognised line agrees and a second does not", say(roundFixture(), 0, "Head: "+rrHead1, "Head reviewed: `"+rrBase+"`"), "could-not-determine", 0, ""},
		{"one recognised line does not agree and a later one does", say(roundFixture(), 0, "Head reviewed: `"+rrBase+"`", "Head: "+rrHead1), "could-not-determine", 0, ""},
		// --- the typed block ---
		{"the typed block records the forge's head", body(roundFixture(), 0,
			"Verdict: request-changes\n\n"+headedBlock(rrHead1, rrHead1)), "", 2, rrHead1},
		{"a carried finding names the earlier head it was first raised at", body(fourRoundFixture(), 2,
			"Verdict: request-changes\n\n"+headedBlock(rrHead1, rrHead3)), "", 4, rrHead1},
		{"the typed block's heads are abbreviated", body(roundFixture(), 0,
			"Verdict: request-changes\n\n"+headedBlock("0000000", "0000000")), "", 2, rrHead1},
		{"the latest verdict's evidence was gathered at another head", body(roundFixture(), 0,
			"Verdict: request-changes\n\n"+headedBlock(rrHead1, rrBase)), "could-not-determine", 0, ""},
		{"the first verdict's finding was first raised at a head the lane holds no verdict at", body(fourRoundFixture(), 0,
			"Verdict: request-changes\n\n"+headedBlock(rrBase, "")), "could-not-determine", 0, ""},
		{"a later verdict's finding was first raised at a head the lane holds no verdict at", body(fourRoundFixture(), 2,
			"Verdict: request-changes\n\n"+headedBlock(rrBase, rrHead3)), "could-not-determine", 0, ""},
		{"a finding was first raised at a head the lane reviewed only LATER", body(fourRoundFixture(), 0,
			"Verdict: request-changes\n\n"+headedBlock(rrHead2, rrHead1)), "could-not-determine", 0, ""},
		{"a verdict's typed block cannot be read", body(fourRoundFixture(), 1,
			"Verdict: request-changes\n\nHead: "+rrHead2+"\n\n<!-- assay:review-finding:v1\n{\"schema\":\"other\"}\n-->"), "could-not-determine", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rr reviewRound
			var err error
			stderr := captureStderr(t, func() { rr, err = roundOf(t, tc.f, rrKeyC) })
			if err != nil {
				t.Fatalf("held: %v", err)
			}
			if got := reasonsOf(rr.scope); got != tc.want || rr.scope.full != (tc.want != "") {
				t.Errorf("scope: full=%v reasons=%q, want reasons %q", rr.scope.full, got, tc.want)
			}
			if rr.round.known != (tc.round != 0) || rr.round.number != tc.round || rr.round.firstHead != tc.first {
				t.Errorf("round = %+v, want known=%v number=%d first=%q", rr.round, tc.round != 0, tc.round, tc.first)
			}
			text := roundText(rr)
			if tc.want == "" {
				if stderr != "" {
					t.Errorf("an agreeing or silent record was reported on stderr: %q", stderr)
				}
				return
			}
			why := "names, in its own text or typed block, a different head than the forge records for it"
			if strings.Contains(tc.name, "cannot be read") {
				why = "carries a typed finding block that cannot be read, so the head it records cannot be checked"
			}
			if !strings.Contains(text, "Round: NOT DETERMINED (this lane's review 50") || strings.Count(text, why) != 2 ||
				!strings.Contains(text, "Clause 20's lane round cap does not apply.") {
				t.Errorf("the assignment does not say which review disagreed, twice (round and scope), with the cap inert:\n%s", text)
			}
			if strings.Contains(text, "DELTA") || strings.Contains(text, rrBase) {
				t.Errorf("the assignment states a delta, or prints the id the record named:\n%s", text)
			}
			if !strings.Contains(stderr, why) || !strings.Contains(stderr, "dispatching") {
				t.Errorf("the disagreement was not reported on stderr: %q", stderr)
			}
		})
	}
}
