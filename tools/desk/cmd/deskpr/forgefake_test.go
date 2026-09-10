package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"

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
	getNums     []int // the PR number of each GetPullRequest, for the synthesised `pr view` argv
	edited      *deskkit.EditChangeInput
	editedNum   int
	comments    []string
	commentNums []int // the PR number of each PostComment ATTEMPT (recorded before any failure)
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
	return &deskkit.PullRequest{
		Number:    number,
		URL:       fmt.Sprintf("https://github.com/%s/pull/%d", repo.Slug(), number),
		State:     "open",
		Body:      body,
		Title:     title,
		Mergeable: f.mergeable(),
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

func (f *envForge) EditChange(repo deskkit.ForgeRepo, number int, in deskkit.EditChangeInput) error {
	f.editedNum = number
	e := in
	f.edited = &e
	if os.Getenv("FAKEGH_EDIT_FAIL") == "1" {
		return deskkit.Unverifiable("HTTP 422: Validation Failed", nil)
	}
	return nil
}

func (f *envForge) PostComment(repo deskkit.ForgeRepo, number int, body string) (*deskkit.CommentRef, error) {
	f.commentNums = append(f.commentNums, number) // the ATTEMPT, recorded before any failure
	if os.Getenv("FAKEGH_COMMENT_FAIL") == "1" {
		return nil, deskkit.Unverifiable("HTTP 502: Bad Gateway", nil)
	}
	f.comments = append(f.comments, body)
	return &deskkit.CommentRef{
		ID:  "c1",
		URL: fmt.Sprintf("https://github.com/%s/pull/%d#issuecomment-1", repo.Slug(), number),
	}, nil
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
		return f, f.repo, nil
	}
	t.Cleanup(func() { forgeForFn = old })
	return f
}
