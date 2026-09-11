package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgefake_test.go — the recording fake deskkit.Forge deskfile's tests drive since the
// write-verbs-C migration replaced the `gh` shell-out with the forge resolver. It reads the SAME
// FAKEGH_* environment variables the retired fake-`gh` binary read, so a test's env setup drives
// it unchanged; only the assertions move from "inspect the recorded gh argv" to "inspect the
// recorded forge op". withEnv installs a fresh one into forgeForFn and stashes it in curForge;
// the suite is serial (t.Setenv forbids t.Parallel), so the package-global is safe.

type dfForge struct {
	deskkit.Forge // nil — only the methods deskfile uses are overridden
	fr            deskkit.ForgeRepo

	searchErr    error // when set, SearchIssues returns it (models a backend could-not-check)
	searchCalls  int
	lastQuery    string
	labelCalls   int
	getNums      []int
	filed        *deskkit.IssueInput
	appliedLabel []string // labels ApplyLabels added after a filing
	comments     []int    // PostComment target numbers
}

func (f *dfForge) SearchIssues(repo deskkit.ForgeRepo, in deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	f.searchCalls++
	f.lastQuery = in.Query
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if os.Getenv("FAKEGH_SEARCH_FAIL") != "" || os.Getenv("FAKEGH_SEARCH_EMPTY") != "" {
		// An unanswerable search is the could-not-check the caller fails CLOSED on.
		return nil, deskkit.Unverifiable("could-not-check: the issue search did not complete", nil)
	}
	raw := os.Getenv("FAKEGH_SEARCH_HITS")
	if raw == "" {
		raw = "[]"
	}
	var hits []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal([]byte(raw), &hits); err != nil {
		return nil, deskkit.Unverifiable("bad FAKEGH_SEARCH_HITS", err)
	}
	out := make([]deskkit.IssueSearchResult, 0, len(hits))
	for _, h := range hits {
		labels := make([]string, 0, len(h.Labels))
		for _, l := range h.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, deskkit.IssueSearchResult{Number: h.Number, Title: h.Title, State: "open", Labels: labels, URL: h.URL})
	}
	return out, nil
}

func (f *dfForge) ListLabels(repo deskkit.ForgeRepo) ([]string, error) {
	f.labelCalls++
	if os.Getenv("FAKEGH_LABEL_FAIL") != "" || os.Getenv("FAKEGH_LABEL_EMPTY") != "" {
		return nil, deskkit.Unverifiable("could-not-check: the label listing did not complete", nil)
	}
	raw := os.Getenv("FAKEGH_LABELS")
	if raw == "" {
		raw = "[]"
	}
	var got []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		return nil, deskkit.Unverifiable("bad FAKEGH_LABELS", err)
	}
	names := make([]string, 0, len(got))
	for _, g := range got {
		names = append(names, g.Name)
	}
	return names, nil
}

func (f *dfForge) GetIssue(repo deskkit.ForgeRepo, number int) (*deskkit.Issue, error) {
	f.getNums = append(f.getNums, number)
	state := os.Getenv("FAKEGH_ISSUE_STATE")
	if state == "" {
		state = "OPEN"
	}
	url := os.Getenv("FAKEGH_ISSUE_URL")
	if url == "" {
		url = fmt.Sprintf("https://github.com/%s/issues/%d", repo.Slug(), number)
	}
	return &deskkit.Issue{Number: number, State: state, URL: url}, nil
}

func (f *dfForge) FileIssue(repo deskkit.ForgeRepo, in deskkit.IssueInput) (*deskkit.IssueRef, error) {
	// Record the ATTEMPT (before any failure), so the synthesised `issue create` argv reflects
	// that the create was reached even when the forge refuses it (FAKEGH_CREATE_FAIL).
	c := in
	f.filed = &c
	if os.Getenv("FAKEGH_CREATE_FAIL") != "" {
		return nil, deskkit.Unverifiable("HTTP 500: create failed", nil)
	}
	return &deskkit.IssueRef{Number: 101, URL: fmt.Sprintf("https://github.com/%s/issues/101", repo.Slug())}, nil
}

func (f *dfForge) ApplyLabels(repo deskkit.ForgeRepo, number int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	for _, l := range change.Add {
		f.appliedLabel = append(f.appliedLabel, l.Name)
	}
	return &deskkit.LabelOutcome{Added: f.appliedLabel}, nil
}

func (f *dfForge) PostComment(repo deskkit.ForgeRepo, number int, body string) (*deskkit.CommentRef, error) {
	if os.Getenv("FAKEGH_CREATE_FAIL") != "" {
		return nil, deskkit.Unverifiable("HTTP 500: comment failed", nil)
	}
	f.comments = append(f.comments, number)
	return &deskkit.CommentRef{URL: fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-1", repo.Slug(), number)}, nil
}

// synthGH renders the forge ops as canonical gh-shaped pseudo-argvs so the suite's
// anyCall(ghCalls(...)) / writes() assertions read the migrated ops without changing each test.
func (f *dfForge) synthGH() [][]string {
	repo := f.fr.Slug()
	var out [][]string
	for i := 0; i < f.searchCalls; i++ {
		// Interleave the query TOKENS (space-split from the free-text query the backend
		// received) between "issues" and "--repo", the position the retired `gh search issues`
		// argv carried them — so the "no qualifier / not phrase-quoted" assertions read them.
		c := []string{"gh", "search", "issues"}
		if f.lastQuery != "" {
			c = append(c, strings.Fields(f.lastQuery)...)
		}
		c = append(c, "--repo", repo, "--state", "open")
		out = append(out, c)
	}
	for i := 0; i < f.labelCalls; i++ {
		out = append(out, []string{"gh", "label", "list", "--repo", repo})
	}
	for _, n := range f.getNums {
		out = append(out, []string{"gh", "issue", "view", strconv.Itoa(n), "--repo", repo})
	}
	if f.filed != nil {
		c := []string{"gh", "issue", "create", "--repo", repo, "--title", f.filed.Title}
		for _, l := range f.appliedLabel {
			c = append(c, "--label", l)
		}
		out = append(out, c)
	}
	for _, n := range f.comments {
		out = append(out, []string{"gh", "issue", "comment", strconv.Itoa(n), "--repo", repo})
	}
	return out
}

// curForge is the fake installed by the most recent withEnv, for assertions.
var curForge *dfForge

func installFakeForge(t *testing.T) *dfForge {
	t.Helper()
	f := &dfForge{}
	// deskfile mints a token before resolving the forge; the fake bypasses ForgeFor, but set a
	// token so any code that reads ghToken (none, post-migration) is satisfied.
	oldTok := ghToken
	ghToken = "fake-token"
	old := forgeForFn
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, deskkit.ForgeKind, error) {
		owner, name, _ := cutSlug(repo)
		f.fr = deskkit.ForgeRepo{Owner: owner, Name: name}
		// The kind mirrors what production's resolver would answer from the planted roster
		// (plantRosterWithForges → ASSAY_REPO_FORGES), defaulting to GitHub when the roster is
		// silent — so a GitLab-configured test sees the GitLab label-create hint (#887 item 2).
		kind := deskkit.ForgeGitHub
		if k, ok := deskkit.EffectiveConfig().RepoForges[strings.ToLower(repo)]; ok {
			kind = deskkit.ForgeKind(k)
		}
		return f, f.fr, kind, nil
	}
	t.Cleanup(func() { forgeForFn = old; ghToken = oldTok })
	return f
}

func cutSlug(repo string) (string, string, bool) {
	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			return repo[:i], repo[i+1:], true
		}
	}
	return repo, "", false
}
