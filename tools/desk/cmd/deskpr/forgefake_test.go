package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgefake_test.go — the recording fake deskkit.Forge deskpr's tests drive since the
// write-verbs-C migration replaced the `gh` shell-out with the forge resolver.
//
// It reads the SAME `FAKEGH_*` environment variables the retired fake-`gh` binary read, so a
// test's existing env-based setup drives it UNCHANGED — only the assertions move from
// "inspect the recorded gh argv" to "inspect the recorded forge op". withEnv installs a fresh
// one into forgeForFn and stashes it in curForge for the assertions; the deskpr suite is serial
// (every test uses t.Setenv, which forbids t.Parallel), so the package-global is safe.

// envForge records the forge ops deskpr performs and answers them from FAKEGH_* env. The
// embedded nil Forge promotes every other method; deskpr must never call one, and a call would
// panic loudly rather than pass silently.
type envForge struct {
	deskkit.Forge // nil — only the methods deskpr uses are overridden
	repo          deskkit.ForgeRepo

	created      *deskkit.DraftChangeInput
	createCalls  int
	openCalls    int
	openBranches []string // the source branch of each OpenChangeForBranch, for the `pr list` synth
	getCalls     int
	getNums      []int // the PR number of each GetPullRequest, for the synthesised `pr view` argv
	edited       *deskkit.EditChangeInput
	editedNum    int
	comments     []string
	commentNums  []int // the PR number of each PostComment ATTEMPT (recorded before any failure)

	// visibilityCalls/visibilityRepo/visibility record the public-repo gate's RepoVisibility
	// read when the production fetcher is routed through THIS fake (assay#1054's regression
	// coverage — see gatewired_test.go's TestCreateGateFetcherRoutesThroughResolvedForge).
	// visibility defaults to "private" (the gate's no-op case) when unset.
	visibilityCalls int
	visibilityRepo  deskkit.ForgeRepo
	visibility      string

	// openMergeHoldCalls / openMergeHoldNum record every OpenMergeHold call this fake sees.
	// openMergeHoldErr, when set, is returned instead of the default typed not-applicable —
	// the GitHub-shaped default every existing test implicitly relies on, so a case that does
	// not touch this field sees byte-identical behaviour to before the merge-hold op set
	// existed.
	openMergeHoldCalls int
	openMergeHoldNum   int
	openMergeHoldErr   error

	// labelChanges / labelNums record every ApplyLabels call this fake sees — the
	// attention-budget/19 --decided path's label write. applyLabelsErr, when set, is returned
	// instead of a success, modelling a forge-side label-write failure that follows an
	// already-landed create/edit.
	labelChanges   []deskkit.LabelChange
	labelNums      []int
	applyLabelsErr error

	// issueNums / trustNums record the `edit --relink-to` issue reads (#5020); ops records the
	// order of the two PR writes ("comment", "edit") so the audit-before-edit rule is assertable.
	issueNums  []int
	issueKinds []deskkit.TargetKind
	trustNums  []int
	ops        []string
}

// synthGH renders the forge ops this fake recorded as canonical gh-shaped pseudo-argvs, so the
// suite's `anyCall(ghCalls(...))` / `countCalls` / `editCalls` / `commentCalls` assertions read
// the migrated forge ops without changing each test. The shapes match what the retired `gh`
// argv carried for the assertions that inspect them.
func (f *envForge) synthGH() [][]string {
	var out [][]string
	for _, b := range f.openBranches {
		// OpenChangeForBranch queries only OPEN changes on the source branch — the equivalent of
		// `gh pr list --head <branch> --state open`. The `--state open` token is the one the
		// merged/closed-exclusion assertion turns on.
		out = append(out, []string{"gh", "pr", "list", "--head", b, "--state", "open"})
	}
	if f.created != nil {
		out = append(out, []string{"gh", "pr", "create", "--draft",
			"--head", f.created.Head, "--base", f.created.Base, "--title", f.created.Title})
	}
	repo := f.repo.Slug()
	for _, n := range f.getNums {
		out = append(out, []string{"gh", "pr", "view", strconv.Itoa(n), "-R", repo, "--json", "mergeable,mergeStateStatus"})
	}
	if f.edited != nil {
		e := []string{"gh", "pr", "edit", strconv.Itoa(f.editedNum), "-R", repo, "--body-file", "<body>"}
		if f.edited.Title != "" {
			e = append(e, "--title", f.edited.Title)
		}
		out = append(out, e)
	}
	for _, n := range f.issueNums {
		out = append(out, []string{"gh", "issue", "view", strconv.Itoa(n), "-R", repo})
	}
	for _, n := range f.trustNums {
		out = append(out, []string{"gh", "api", "graphql", "issue-trust", strconv.Itoa(n)})
	}
	for _, n := range f.commentNums {
		out = append(out, []string{"gh", "pr", "comment", strconv.Itoa(n), "-R", repo, "--body-file", "<note>"})
	}
	return out
}

func (f *envForge) OpenChangeForBranch(repo deskkit.ForgeRepo, branch string) (*deskkit.PullRequest, error) {
	f.openCalls++
	f.openBranches = append(f.openBranches, branch)
	if os.Getenv("FAKEGH_LIST_HAS_PR") != "1" {
		return nil, nil
	}
	// The forge query is source_branch=<branch>: a divergent FAKEGH_LIST_HEAD models a PR whose
	// SOURCE branch is not this one, which the forge would not return for this branch.
	if head := os.Getenv("FAKEGH_LIST_HEAD"); head != "" && head != branch {
		return nil, nil
	}
	draft := os.Getenv("FAKEGH_LIST_DRAFT") != "0"
	return &deskkit.PullRequest{
		Number:  42,
		URL:     fmt.Sprintf("https://github.com/%s/pull/42", repo.Slug()),
		Draft:   draft,
		State:   "open",
		HeadRef: branch,
	}, nil
}

func (f *envForge) CreateDraftChange(repo deskkit.ForgeRepo, in deskkit.DraftChangeInput) (*deskkit.PullRef, error) {
	f.createCalls++
	c := in
	f.created = &c
	return &deskkit.PullRef{
		Number: 101,
		URL:    fmt.Sprintf("https://github.com/%s/pull/101", repo.Slug()),
	}, nil
}

// OpenMergeHold defaults to the GitHub-shaped typed not-applicable — byte-identical to how
// this fake behaved before the merge-hold op set existed, for every test that does not set
// openMergeHoldErr.
func (f *envForge) OpenMergeHold(repo deskkit.ForgeRepo, number int) (string, error) {
	f.openMergeHoldCalls++
	f.openMergeHoldNum = number
	if f.openMergeHoldErr != nil {
		return "", f.openMergeHoldErr
	}
	return "", deskkit.ErrMergeHoldNotApplicable
}

func (f *envForge) GetPullRequest(repo deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error) {
	f.getCalls++
	f.getNums = append(f.getNums, number)
	body := os.Getenv("FAKEGH_PR_BODY")
	if body == "" {
		body = "Brief: fixture/01\n"
	}
	title := os.Getenv("FAKEGH_PR_TITLE")
	if title == "" {
		title = "fixture PR title"
	}
	// FAKEGH_PR_LABELS (comma-separated) is the label set the change currently carries — the
	// attention-budget/19 `edit --decided` no-op path reconciles a missing desk-decided label
	// against it. Unset serves no labels, byte-identical to this fake before the knob existed.
	var labels []string
	if l := os.Getenv("FAKEGH_PR_LABELS"); l != "" {
		labels = strings.Split(l, ",")
	}
	// FAKEGH_PR_STATE / FAKEGH_PR_HEAD / FAKEGH_PR_OID shape the change's state, head branch
	// and head commit for the `edit --pr N` own-PR guard (#1901). Unset serves an open change
	// with no head facts, byte-identical to this fake before the knobs existed.
	state := os.Getenv("FAKEGH_PR_STATE")
	if state == "" {
		state = "open"
	}
	// FAKEGH_PR_CROSS shapes PullRequest.CrossRepo (#2085): unset serves a same-repo head,
	// "fork" a fork head, and "unknown" the forge's empty could-not-check answer.
	cross := deskkit.CrossRepoSame
	switch os.Getenv("FAKEGH_PR_CROSS") {
	case "fork":
		cross = deskkit.CrossRepoFork
	case "unknown":
		cross = ""
	}
	return &deskkit.PullRequest{
		Number:    number,
		URL:       fmt.Sprintf("https://github.com/%s/pull/%d", repo.Slug(), number),
		State:     state,
		Body:      body,
		Title:     title,
		Mergeable: f.mergeable(),
		Labels:    labels,
		HeadRef:   os.Getenv("FAKEGH_PR_HEAD"),
		HeadSHA:   os.Getenv("FAKEGH_PR_OID"),
		CrossRepo: cross,
	}, nil
}

// mergeable models GitHub's async mergeable computation (#1264) the same way the retired fake
// did: while a counter file sits at or below FAKEGH_UNKNOWN_UNTIL, report UNKNOWN and bump it.
func (f *envForge) mergeable() string {
	m := os.Getenv("FAKEGH_MERGEABLE")
	if m == "" {
		m = "MERGEABLE"
	}
	if cf := os.Getenv("FAKEGH_VIEW_COUNT_FILE"); cf != "" {
		until := 0
		fmt.Sscanf(os.Getenv("FAKEGH_UNKNOWN_UNTIL"), "%d", &until)
		n := 0
		if b, err := os.ReadFile(cf); err == nil {
			fmt.Sscanf(string(b), "%d", &n)
		}
		n++
		_ = os.WriteFile(cf, []byte(fmt.Sprintf("%d", n)), 0o600)
		if n <= until {
			m = "UNKNOWN"
		}
	}
	switch m {
	case "MERGEABLE":
		return deskkit.Mergeable
	case "CONFLICTING":
		return deskkit.MergeableConflicting
	default:
		return deskkit.MergeableUnknown
	}
}

// RepoVisibility answers the public-repo gate's live-visibility read from THIS fake — the
// resolved forge backend — rather than any hardcoded GitHub-only client. Defaults to
// "private" (the gate's no-write-authorization-needed case) so tests that never set
// f.visibility keep sailing through the gate exactly as before this method existed.
func (f *envForge) RepoVisibility(repo deskkit.ForgeRepo) (string, error) {
	f.visibilityCalls++
	f.visibilityRepo = repo
	if f.visibility != "" {
		return f.visibility, nil
	}
	return "private", nil
}

func (f *envForge) EditChange(repo deskkit.ForgeRepo, number int, in deskkit.EditChangeInput) error {
	f.editedNum = number
	f.ops = append(f.ops, "edit")
	e := in
	f.edited = &e
	if os.Getenv("FAKEGH_EDIT_FAIL") == "1" {
		return deskkit.Unverifiable("HTTP 422: Validation Failed", nil)
	}
	return nil
}

func (f *envForge) PostCommentTyped(repo deskkit.ForgeRepo, number int, kind deskkit.TargetKind, body string) (*deskkit.CommentRef, error) {
	if kind != deskkit.TargetChange {
		return nil, deskkit.Refused("fixture: deskpr comments land on a PR, never another kind") // #6264
	}
	f.commentNums = append(f.commentNums, number) // the ATTEMPT, recorded before any failure
	f.ops = append(f.ops, "comment")
	if os.Getenv("FAKEGH_COMMENT_FAIL") == "1" {
		return nil, deskkit.Unverifiable("HTTP 502: Bad Gateway", nil)
	}
	f.comments = append(f.comments, body)
	return &deskkit.CommentRef{
		ID:  "c1",
		URL: fmt.Sprintf("https://github.com/%s/pull/%d#issuecomment-1", repo.Slug(), number),
	}, nil
}

// ApplyLabels records the attention-budget/19 --decided label write. Every existing test
// that never sets --decided never calls this at all, so it changes nothing for them.
func (f *envForge) ApplyLabels(repo deskkit.ForgeRepo, number int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	f.labelNums = append(f.labelNums, number)
	f.labelChanges = append(f.labelChanges, change)
	if f.applyLabelsErr != nil {
		return nil, f.applyLabelsErr
	}
	var added []string
	for _, l := range change.Add {
		added = append(added, l.Name)
	}
	return &deskkit.LabelOutcome{Added: added, Removed: change.Remove}, nil
}

// curForge is the fake installed by the most recent withEnv, for assertions.
var curForge *envForge

// installFakeForge points forgeForFn at a fresh envForge and returns it.
func installFakeForge(t *testing.T) *envForge {
	t.Helper()
	f := &envForge{}
	old := forgeForFn
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name := splitOwnerRepo(repo)
		f.repo = deskkit.ForgeRepo{Owner: owner, Name: name}
		return deskkit.OutboundChecked(f, "worker"), f.repo, nil
	}
	t.Cleanup(func() { forgeForFn = old })
	return f
}

// GetIssueTyped answers `edit --relink-to`'s read of the NEW issue (#5020) from FAKEGH_ISSUE_*:
// MISSING=1 is a 404, READ_FAIL=1 an HTTP 502, SPLIT_FAIL a failed issue read with a change at
// the same number (below), NIL=1 a (nil, nil) answer, PR=1 a pull request,
// STATE (default open), AUTHOR login:id (default the fixture's trusted ada:2001), LABELS
// (comma-separated), TITLE and BODY.
//
// The kind mismatch matches deskkit.GitHubForge.GetIssueTyped exactly: GitHub numbers issues
// and PRs in one sequence, and its typed read answers a PR number read as an issue (or an
// issue number read as a change) with a could-not-check error, never a record.
func (f *envForge) GetIssueTyped(repo deskkit.ForgeRepo, number int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	f.issueNums = append(f.issueNums, number)
	f.issueKinds = append(f.issueKinds, kind)
	if os.Getenv("FAKEGH_ISSUE_MISSING") == "1" {
		return nil, &deskkit.ForgeAPIError{Status: 404}
	}
	if os.Getenv("FAKEGH_ISSUE_READ_FAIL") == "1" {
		return nil, deskkit.Unverifiable("HTTP 502: Bad Gateway", nil)
	}
	// FAKEGH_ISSUE_SPLIT_FAIL models a forge with SEPARATE issue and change sequences
	// (GitLab's #N and !N) whose ISSUE read fails while a change exists at the same number:
	// a status (e.g. 503, 403) fails it the way GitLabForge.mapErr does, with a ForgeAPIError
	// cause ("503-echo" adds the mismatch wording to that answer's text); "transport" fails it
	// with no status but a transport cause. The change read
	// answers an open change at the number.
	if sf := os.Getenv("FAKEGH_ISSUE_SPLIT_FAIL"); sf != "" {
		if kind == deskkit.TargetChange {
			return &deskkit.Issue{Number: number, State: "open", IsPullRequest: true,
				Author: deskkit.Account{Login: "ada", ID: 2001}}, nil
		}
		path := fmt.Sprintf("/projects/%s/issues/%d", repo.Slug(), number)
		if sf == "transport" {
			return nil, deskkit.Unverifiable("could-not-check: GET "+path+" did not complete: connection reset",
				errors.New("connection reset"))
		}
		// "echo" is a 503 whose forge-supplied body text happens to carry the single-sequence
		// kind-mismatch wording: a status-bearing failure is a failure whatever it says.
		msg := ""
		if strings.HasSuffix(sf, "-echo") {
			sf = strings.TrimSuffix(sf, "-echo")
			msg = ": #" + strconv.Itoa(number) + " is a pull request, not an issue"
		}
		st, _ := strconv.Atoi(sf)
		return nil, deskkit.Unverifiable(fmt.Sprintf("could-not-check: GET %s — HTTP %d%s", path, st, msg),
			&deskkit.ForgeAPIError{Status: st, Method: "GET", Path: path, Body: msg})
	}
	if os.Getenv("FAKEGH_ISSUE_NIL") == "1" {
		return nil, nil
	}
	isPR := os.Getenv("FAKEGH_ISSUE_PR") == "1"
	// FAKEGH_ISSUE_UNTYPED=1 models a backend whose issue read does not refuse a pull
	// request number but returns the record flagged IsPullRequest (the GitHub backend
	// refuses it, as above); it pins the gate's own IsPullRequest check.
	untyped := os.Getenv("FAKEGH_ISSUE_UNTYPED") == "1"
	if isPR && kind == deskkit.TargetIssue && !untyped {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d is a pull request, not an issue — state the kind you mean (--kind pr)",
			repo.Slug(), number), nil)
	}
	if !isPR && kind == deskkit.TargetChange {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d is an issue, not a pull request — state the kind you mean (--kind issue)",
			repo.Slug(), number), nil)
	}
	state := os.Getenv("FAKEGH_ISSUE_STATE")
	if state == "" {
		state = "open"
	}
	author := os.Getenv("FAKEGH_ISSUE_AUTHOR")
	if author == "" {
		author = "ada:2001"
	}
	login, idStr, _ := strings.Cut(author, ":")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	var labels []string
	if l := os.Getenv("FAKEGH_ISSUE_LABELS"); l != "" {
		labels = strings.Split(l, ",")
	}
	return &deskkit.Issue{
		Number:        number,
		State:         state,
		Author:        deskkit.Account{Login: login, ID: id},
		IsPullRequest: isPR,
		Title:         os.Getenv("FAKEGH_ISSUE_TITLE"),
		Body:          os.Getenv("FAKEGH_ISSUE_BODY"),
		Labels:        labels,
	}, nil
}

// IssueTrustEvents answers the trust gate's read: FAKEGH_ISSUE_BLESSED=1 serves a blessing
// comment from the fixture's blessing authority after the last body edit; TRUST_FAIL=1 an
// unreadable record; TRUST_INCOMPLETE=1 a record the forge could not page to the end.
func (f *envForge) IssueTrustEvents(repo deskkit.ForgeRepo, number int) (*deskkit.TrustPayload, error) {
	f.trustNums = append(f.trustNums, number)
	if os.Getenv("FAKEGH_ISSUE_TRUST_FAIL") == "1" {
		return nil, deskkit.Unverifiable("HTTP 502: Bad Gateway", nil)
	}
	edited := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tp := &deskkit.TrustPayload{BodyEdited: edited, Complete: os.Getenv("FAKEGH_ISSUE_TRUST_INCOMPLETE") != "1"}
	if os.Getenv("FAKEGH_ISSUE_BLESSED") == "1" {
		tp.Events = []deskkit.ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: edited.Add(time.Hour)}}
	}
	return tp, nil
}
