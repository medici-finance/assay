package main

// deskread_brief33_test.go — the deskread Verify rows of forge-neutral brief 33: the eight new
// kinds that consume ops 55-58 and the new result fields, the `comments` kind's identity fields,
// and its change target.
//
// These rows run the verb against a RECORDING fake Forge: every call the verb makes is recorded
// with its arguments, so a row can assert both what the envelope carries and exactly which
// operation was asked for which target — and, for a refusal, that NO call was made at all. The
// fake proves only the verb's half (address → operation → envelope). That the real backends serve
// these fields is the deskkit rows' job (the goldens and the brief-33 tests in
// internal/deskkit), on recorded fixtures for both forges.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// recCall is one recorded Forge call.
type recCall struct {
	Op     string
	Repo   string
	Number int
	Target deskkit.TargetKind
	Detail string
}

// recForge is the recording fake. It embeds a nil deskkit.Forge, so any operation it does not
// override panics — a kind that reached for an operation it should not touch fails loudly. A
// repo named "locked" or an item numbered 404 answers could-not-check; 405 answers a forge 404.
type recForge struct {
	deskkit.Forge
	mu    sync.Mutex
	calls []recCall
}

func (f *recForge) rec(c recCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *recForge) recorded() []recCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recCall(nil), f.calls...)
}

func failFor(repo deskkit.ForgeRepo, number int) error {
	switch {
	case repo.Name == "locked" || number == 404:
		return deskkit.Unverifiable(fmt.Sprintf("the App installation cannot read %s#%d", repo.Slug(), number), nil)
	case number == 405:
		return &deskkit.ForgeAPIError{Method: http.MethodGet, Path: "/x", Status: http.StatusNotFound}
	}
	return nil
}

func (f *recForge) ListIssues(repo deskkit.ForgeRepo, in deskkit.IssueListQuery) (*deskkit.IssueList, error) {
	f.rec(recCall{Op: "ListIssues", Repo: repo.Slug(), Detail: in.State + "|" + in.Label})
	if err := failFor(repo, 0); err != nil {
		return nil, err
	}
	return &deskkit.IssueList{Issues: []deskkit.IssueSummary{
		{Number: 3, Title: "closed one", State: "closed", CreatedAt: "2026-09-01T00:00:00Z", ClosedAt: "2026-09-02T00:00:00Z",
			Author: deskkit.Account{Login: "someone", ID: 42, Type: "User"}, Labels: []string{"area:x"}},
		{Number: 4, Title: "open one", State: "open", CreatedAt: "2026-09-03T00:00:00Z",
			Author: deskkit.Account{Login: "other", ID: 43}},
	}, Incomplete: true, PageCap: 100}, nil
}

func (f *recForge) ListChanges(repo deskkit.ForgeRepo, states deskkit.ChangeStates) (*deskkit.ChangeList, error) {
	f.rec(recCall{Op: "ListChanges", Repo: repo.Slug(), Detail: fmt.Sprintf("%+v", states)})
	if err := failFor(repo, 0); err != nil {
		return nil, err
	}
	return &deskkit.ChangeList{Changes: []deskkit.ChangeRef{
		{Number: 21, State: "MERGED", HeadSHA: "abc", HeadRef: "feat/x", Title: "t", Body: "Brief: x",
			MergedAt: "2026-09-04T00:00:00Z", Author: deskkit.Account{Login: "example-app[bot]", ID: 300000001, Type: "Bot"},
			BaseRef: "main", CrossRepo: deskkit.CrossRepoFork},
	}, Incomplete: false, PageCap: 5}, nil
}

func (f *recForge) RepoDefaultBranch(repo deskkit.ForgeRepo) (string, error) {
	f.rec(recCall{Op: "RepoDefaultBranch", Repo: repo.Slug()})
	if err := failFor(repo, 0); err != nil {
		return "", err
	}
	return "trunk", nil
}

func (f *recForge) GetIssueTyped(repo deskkit.ForgeRepo, number int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	f.rec(recCall{Op: "GetIssueTyped", Repo: repo.Slug(), Number: number, Target: kind})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	if number == 8 { // a change at this number: a kind mismatch the verb must never itemise
		return &deskkit.Issue{Number: 8, State: "open", IsPullRequest: true}, nil
	}
	return &deskkit.Issue{Number: number, State: "closed", Title: "verdict", Body: "the body",
		Author:   deskkit.Account{Login: "example-app[bot]", ID: 300000001, Type: "Bot"},
		ClosedBy: deskkit.Account{Login: "authority", ID: 100001, Type: "User"}}, nil
}

func (f *recForge) IssueStateEvents(repo deskkit.ForgeRepo, number int) (*deskkit.IssueStateHistory, error) {
	f.rec(recCall{Op: "IssueStateEvents", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	return &deskkit.IssueStateHistory{
		Events: []deskkit.IssueStateEvent{
			{Kind: deskkit.IssueEventClosed, Actor: deskkit.Account{Login: "authority", ID: 100001}, CreatedAt: "2026-09-05T00:00:00Z"},
			{Kind: deskkit.IssueEventReopened, CreatedAt: "2026-09-06T00:00:00Z"},
		},
		ClosingChanges: []deskkit.ClosingChange{{Repo: "example-org/alpha", Number: 21, Merged: true,
			Author: deskkit.Account{Login: "example-app[bot]", ID: 300000001, Type: "Bot"}}},
		Complete: false,
	}, nil
}

func (f *recForge) GetPullRequest(repo deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error) {
	f.rec(recCall{Op: "GetPullRequest", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	pr := &deskkit.PullRequest{Number: number, State: "closed", Merged: true, MergedAt: "2026-09-04T00:00:00Z",
		MergeCommitSHA: "m3rg3", HeadSHA: "abc", HeadRef: "feat/x", BaseRef: "main", CrossRepo: deskkit.CrossRepoSame,
		ChangedFiles: 2, Author: deskkit.Account{Login: "someone", ID: 42}}
	if number == 10 {
		pr.ChangedFiles = 3 // more than the file list serves: the list is NOT the whole
	}
	return pr, nil
}

func (f *recForge) ListChangeCommits(repo deskkit.ForgeRepo, number int) (*deskkit.ChangeCommits, error) {
	f.rec(recCall{Op: "ListChangeCommits", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	return &deskkit.ChangeCommits{SHAs: []string{"c1", "c2"}, Complete: false}, nil
}

func (f *recForge) ListChangedFiles(repo deskkit.ForgeRepo, number int) ([]deskkit.ChangedFile, error) {
	f.rec(recCall{Op: "ListChangedFiles", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	return []deskkit.ChangedFile{
		{Filename: "a.go", Status: "modified", Patch: "@@ -1 +1 @@\n-x\n+y"},
		{Filename: "logo.png", Status: "added", PatchAbsent: true},
	}, nil
}

func (f *recForge) ListCommentsTyped(repo deskkit.ForgeRepo, number int, kind deskkit.TargetKind) ([]deskkit.Comment, error) {
	f.rec(recCall{Op: "ListCommentsTyped", Repo: repo.Slug(), Number: number, Target: kind})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	return []deskkit.Comment{
		{ID: "IC_1", DatabaseID: 5001, Body: "signed off", CreatedAt: "2026-09-02T00:00:00Z",
			UpdatedAt: "2026-09-02T01:00:00Z", URL: "https://example.test/c/5001",
			Author: deskkit.Account{Login: "authority", ID: 100001, Type: "User"}},
		// None of the four new fields: renders exactly as before they existed.
		{ID: "IC_2", Body: "plain", CreatedAt: "2026-09-03T00:00:00Z",
			Author: deskkit.Account{Login: "other", ID: 43}},
	}, nil
}

func (f *recForge) IssueTrustEvents(repo deskkit.ForgeRepo, number int) (*deskkit.TrustPayload, error) {
	f.rec(recCall{Op: "IssueTrustEvents", Repo: repo.Slug(), Number: number})
	return &deskkit.TrustPayload{Complete: true}, nil
}

// stubRec installs one recording fake behind every repo coordinate.
func stubRec(t *testing.T) *recForge {
	t.Helper()
	f := &recForge{}
	prev := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return f, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	t.Cleanup(func() { forgeFor = prev })
	return f
}

// rawOut runs the verb and returns stdout, exit and stderr.
func rawOut(args ...string) (string, int, string) {
	var out, errb strings.Builder
	code := run(args, &out, &errb)
	return out.String(), code, errb.String()
}

// --- row 11: each new kind round-trips, carries its fields, and reports partial ------------

func TestDeskreadNewKindsRoundTrip(t *testing.T) {
	t.Run("issue-list", func(t *testing.T) {
		f := stubRec(t)
		env, code, errs := runEnvelope(t, "issue-list", "--repo", "example-org/alpha", "--repo", "example-org/locked",
			"--state", "closed", "--label", "area:x")
		if code != deskkit.ExitOK {
			t.Fatalf("exit=%d; stderr=%s", code, errs)
		}
		if env.Schema != envelopeSchema || env.Kind != "issue-list" {
			t.Errorf("schema/kind = %d/%q", env.Schema, env.Kind)
		}
		if len(env.Repos) != 1 || len(env.Partial) != 1 || env.Partial[0].Repo != "example-org/locked" || env.Partial[0].Reason == "" {
			t.Fatalf("repos=%+v partial=%+v, want alpha read and locked in partial with a reason", env.Repos, env.Partial)
		}
		r := env.Repos[0]
		if r.Incomplete == nil || !*r.Incomplete || r.PageCap != 100 {
			t.Errorf("incomplete/pageCap = %v/%d, want true/100 — an overflowed list must SAY so", r.Incomplete, r.PageCap)
		}
		if len(r.Issues) != 2 || r.Issues[0].State != "closed" || r.Issues[0].ClosedAt != "2026-09-02T00:00:00Z" ||
			r.Issues[0].AuthorType != "User" || r.Issues[1].ClosedAt != "" || r.Issues[1].AuthorType != "" {
			t.Errorf("issues = %+v, want state/closedAt/authorType as served and empty where absent", r.Issues)
		}
		calls := f.recorded()
		if len(calls) != 2 || calls[0].Op != "ListIssues" || !strings.HasPrefix(calls[0].Detail+calls[1].Detail, "closed|area:x") {
			t.Errorf("calls = %+v, want ListIssues with state closed and the one label", calls)
		}
	})

	t.Run("issue-list default state is open", func(t *testing.T) {
		f := stubRec(t)
		if _, code, _ := runEnvelope(t, "issue-list", "--repo", "example-org/alpha"); code != deskkit.ExitOK {
			t.Fatalf("exit=%d", code)
		}
		if c := f.recorded(); len(c) != 1 || c[0].Detail != "open|" {
			t.Errorf("calls = %+v, want ListIssues open with no label", c)
		}
	})

	t.Run("changes", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runEnvelope(t, "changes", "--repo", "example-org/alpha", "--repo", "example-org/locked", "--state", "merged")
		if code != deskkit.ExitOK || len(env.Repos) != 1 || len(env.Partial) != 1 {
			t.Fatalf("exit=%d repos=%+v partial=%+v", code, env.Repos, env.Partial)
		}
		r := env.Repos[0]
		if len(r.Changes) != 1 {
			t.Fatalf("changes = %+v", r.Changes)
		}
		c := r.Changes[0]
		if c.Author == nil || c.Author.Type != "Bot" || c.BaseRef != "main" || c.CrossRepo != deskkit.CrossRepoFork || c.HeadRepo != "" || c.Body != "Brief: x" {
			t.Errorf("change = %+v, want author type, base, fork facts, body; empty headRepo stays empty", c)
		}
		if r.Incomplete == nil || *r.Incomplete || r.PageCap != 5 {
			t.Errorf("incomplete/pageCap = %v/%d, want an explicit false and 5", r.Incomplete, r.PageCap)
		}
		if r.Issues != nil {
			t.Errorf("a changes read carries an issues list: %+v", r.Issues)
		}
		calls := f.recorded()
		if len(calls) != 2 || calls[0].Detail != fmt.Sprintf("%+v", deskkit.ChangeStates{Merged: true}) {
			t.Errorf("calls = %+v, want ListChanges(merged only)", calls)
		}
	})

	t.Run("default-branch", func(t *testing.T) {
		stubRec(t)
		s, code, _ := rawOut("default-branch", "--repo", "example-org/alpha", "--repo", "example-org/locked")
		if code != deskkit.ExitOK {
			t.Fatalf("exit=%d", code)
		}
		var env Envelope
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Repos) != 1 || env.Repos[0].DefaultBranch != "trunk" || len(env.Partial) != 1 {
			t.Fatalf("env = %+v", env)
		}
		if strings.Contains(s, `"issues"`) {
			t.Errorf("a default-branch read renders an issues key:\n%s", s)
		}
	})

	t.Run("issue", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runItemEnvelope(t, "issue", "--issue", "example-org/alpha#7", "--issue", "example-org/alpha#404", "--issue", "example-org/alpha#8")
		if code != deskkit.ExitOK || env.Kind != "issue" || env.Schema != envelopeSchema {
			t.Fatalf("exit=%d env=%+v", code, env)
		}
		if len(env.Items) != 1 || env.Items[0].Number != 7 || env.Items[0].Issue == nil {
			t.Fatalf("items = %+v, want only #7", env.Items)
		}
		is := env.Items[0].Issue
		if is.State != "closed" || is.Body != "the body" || is.Author == nil || is.Author.Type != "Bot" ||
			is.ClosedBy == nil || is.ClosedBy.Login != "authority" || is.ClosedBy.Type != "User" {
			t.Errorf("issue = %+v, want state, body, author type and closer", is)
		}
		if len(env.Partial) != 2 || env.Partial[0].Number != 404 || env.Partial[1].Number != 8 {
			t.Fatalf("partial = %+v, want #404 (unreadable) and #8 (a change at the number — a kind mismatch)", env.Partial)
		}
		for _, c := range f.recorded() {
			if c.Op != "GetIssueTyped" || c.Target != deskkit.TargetIssue {
				t.Errorf("call %+v, want GetIssueTyped on the issue kind only", c)
			}
		}
	})

	t.Run("issue-states", func(t *testing.T) {
		stubRec(t)
		env, code, _ := runItemEnvelope(t, "issue-states", "--issue", "example-org/alpha#7", "--issue", "example-org/alpha#404")
		if code != deskkit.ExitOK || len(env.Items) != 1 || len(env.Partial) != 1 {
			t.Fatalf("exit=%d env=%+v", code, env)
		}
		st := env.Items[0].IssueStates
		if st == nil || st.Complete || len(st.Events) != 2 || st.Events[0].Kind != "closed" || st.Events[1].Actor != nil ||
			len(st.ClosingChanges) != 1 || !st.ClosingChanges[0].Merged || st.ClosingChanges[0].Repo != "example-org/alpha" ||
			st.ClosingChanges[0].Author == nil || st.ClosingChanges[0].Author.Type != "Bot" {
			t.Errorf("issueStates = %+v, want complete=false carried, two events (unreported actor absent), one merged closer", st)
		}
	})

	t.Run("change", func(t *testing.T) {
		stubRec(t)
		env, code, _ := runItemEnvelope(t, "change", "--change", "example-org/alpha#9", "--change", "example-org/alpha#404")
		if code != deskkit.ExitOK || len(env.Items) != 1 || len(env.Partial) != 1 {
			t.Fatalf("exit=%d env=%+v", code, env)
		}
		c := env.Items[0].Change
		if c == nil || c.MergeCommitSHA != "m3rg3" || !c.Merged || c.BaseRef != "main" || c.ChangedFiles != 2 {
			t.Errorf("change = %+v, want the merge-commit SHA and the change facts", c)
		}
	})

	t.Run("change-commits", func(t *testing.T) {
		stubRec(t)
		env, code, _ := runItemEnvelope(t, "change-commits", "--change", "example-org/alpha#9", "--change", "example-org/alpha#404")
		if code != deskkit.ExitOK || len(env.Items) != 1 || len(env.Partial) != 1 {
			t.Fatalf("exit=%d env=%+v", code, env)
		}
		cc := env.Items[0].ChangeCommits
		if cc == nil || cc.Complete || len(cc.SHAs) != 2 {
			t.Errorf("changeCommits = %+v, want two SHAs and complete=false carried", cc)
		}
	})

	t.Run("change-files", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runItemEnvelope(t, "change-files", "--change", "example-org/alpha#9",
			"--change", "example-org/alpha#10", "--change", "example-org/alpha#404")
		if code != deskkit.ExitOK || len(env.Items) != 2 || len(env.Partial) != 1 {
			t.Fatalf("exit=%d env=%+v", code, env)
		}
		full, short := env.Items[0].ChangeFiles, env.Items[1].ChangeFiles
		if full == nil || !full.Complete || full.ChangedFiles != 2 || len(full.Files) != 2 {
			t.Fatalf("#9 changeFiles = %+v, want complete (2 of 2)", full)
		}
		if full.Files[0].Patch == "" || full.Files[0].PatchAbsent || full.Files[1].Patch != "" || !full.Files[1].PatchAbsent {
			t.Errorf("files = %+v, want the patch served and the absent patch STATED", full.Files)
		}
		if short == nil || short.Complete || short.ChangedFiles != 3 {
			t.Errorf("#10 changeFiles = %+v, want complete=false — 2 listed of the change's own 3", short)
		}
		var sawCount bool
		for _, c := range f.recorded() {
			if c.Op == "GetPullRequest" {
				sawCount = true
			}
		}
		if !sawCount {
			t.Error("change-files never read the change's own file count — nothing to reconcile the list against")
		}
	})

	t.Run("all unreadable is exit 6", func(t *testing.T) {
		stubRec(t)
		if _, code, _ := runItemEnvelope(t, "change-commits", "--change", "example-org/alpha#404"); code != deskkit.ExitUnverifiable {
			t.Errorf("exit=%d, want %d", code, deskkit.ExitUnverifiable)
		}
		if _, code, _ := runEnvelope(t, "default-branch", "--repo", "example-org/locked"); code != deskkit.ExitUnverifiable {
			t.Errorf("exit=%d, want %d", code, deskkit.ExitUnverifiable)
		}
	})
}

// --- row 12: addressing refusals, zero forge calls -----------------------------------------

func TestDeskreadNewKindsAddressingRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"issue-list with --issue", []string{"issue-list", "--issue", "example-org/alpha#1"}},
		{"issue-list with --change", []string{"issue-list", "--change", "example-org/alpha#1"}},
		{"issue-list with no --repo", []string{"issue-list", "--state", "all"}},
		{"issue-list unknown --state", []string{"issue-list", "--repo", "example-org/alpha", "--state", "bogus"}},
		{"issue-list a change state", []string{"issue-list", "--repo", "example-org/alpha", "--state", "merged"}},
		{"issue-list comma label", []string{"issue-list", "--repo", "example-org/alpha", "--label", "a,b"}},
		{"issue-list empty label", []string{"issue-list", "--repo", "example-org/alpha", "--label", ""}},
		{"issue-list two --state", []string{"issue-list", "--repo", "example-org/alpha", "--state", "open", "--state", "all"}},
		{"changes with no --state", []string{"changes", "--repo", "example-org/alpha"}},
		{"changes unknown --state", []string{"changes", "--repo", "example-org/alpha", "--state", "bogus"}},
		{"changes an issue state", []string{"changes", "--repo", "example-org/alpha", "--state", "open"}},
		{"changes with --label", []string{"changes", "--repo", "example-org/alpha", "--state", "merged", "--label", "x"}},
		{"changes with --change", []string{"changes", "--change", "example-org/alpha#1", "--state", "all"}},
		{"default-branch with --state", []string{"default-branch", "--repo", "example-org/alpha", "--state", "all"}},
		{"default-branch with --issue", []string{"default-branch", "--issue", "example-org/alpha#1"}},
		{"issue with --repo", []string{"issue", "--repo", "example-org/alpha"}},
		{"issue with --change", []string{"issue", "--change", "example-org/alpha#1"}},
		{"issue with --state", []string{"issue", "--issue", "example-org/alpha#1", "--state", "open"}},
		{"issue-states with --change", []string{"issue-states", "--change", "example-org/alpha#1"}},
		{"issue-states with no address", []string{"issue-states"}},
		{"change with --issue", []string{"change", "--issue", "example-org/alpha#1"}},
		{"change number zero", []string{"change", "--change", "example-org/alpha#0"}},
		{"change bad slug", []string{"change", "--change", "not-a-slug#3"}},
		{"change-commits with --issue", []string{"change-commits", "--issue", "example-org/alpha#1"}},
		{"change-commits with --repo", []string{"change-commits", "--repo", "example-org/alpha"}},
		{"change-files with --repo", []string{"change-files", "--repo", "example-org/alpha"}},
		{"change-files with --label", []string{"change-files", "--change", "example-org/alpha#1", "--label", "x"}},
		{"trust with --change", []string{"trust", "--change", "example-org/alpha#1"}},
		{"comments with both targets", []string{"comments", "--issue", "example-org/alpha#1", "--change", "example-org/alpha#1"}},
		{"comments with neither target", []string{"comments"}},
		{"comments with --repo", []string{"comments", "--repo", "example-org/alpha"}},
		{"--path is not a flag", []string{"change-files", "--change", "example-org/alpha#1", "--path", "a.go"}},
		{"--query is not a flag", []string{"issue-list", "--repo", "example-org/alpha", "--query", "is:closed"}},
		{"--endpoint is not a flag", []string{"default-branch", "--repo", "example-org/alpha", "--endpoint", "/repos/x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := stubRec(t)
			out, code, errs := rawOut(tc.args...)
			if code != deskkit.ExitRefused {
				t.Fatalf("exit=%d, want %d (refused); stderr=%s", code, deskkit.ExitRefused, errs)
			}
			if out != "" {
				t.Errorf("a refused run wrote to stdout: %q", out)
			}
			if c := f.recorded(); len(c) != 0 {
				t.Errorf("a refused run made forge calls: %+v", c)
			}
		})
	}
}

// --- row 26 (deskread leg): the comments kind carries the identity fields -------------------

// preChangeComment is CommentJSON exactly as it stood before forge-neutral brief 33.
type preChangeComment struct {
	AuthorLogin string `json:"authorLogin,omitempty"`
	AuthorID    int64  `json:"authorId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	Body        string `json:"body"`
}

func TestDeskreadCommentsCarryIdentityFields(t *testing.T) {
	stubRec(t)
	out, code, errs := rawOut("comments", "--issue", "example-org/alpha#7")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, errs)
	}
	var env struct {
		Items []struct {
			Comments []json.RawMessage `json:"comments"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("stdout is not the envelope: %v\n%s", err, out)
	}
	if len(env.Items) != 1 || len(env.Items[0].Comments) != 2 {
		t.Fatalf("items = %s", out)
	}
	var full CommentJSON
	if err := json.Unmarshal(env.Items[0].Comments[0], &full); err != nil {
		t.Fatal(err)
	}
	if full.DatabaseID != 5001 || full.AuthorType != "User" || full.URL != "https://example.test/c/5001" ||
		full.UpdatedAt != "2026-09-02T01:00:00Z" || full.CreatedAt != "2026-09-02T00:00:00Z" {
		t.Errorf("comment 0 = %+v, want databaseId, authorType, url and updatedAt as the seam served them", full)
	}

	// A comment carrying none of the four renders BYTE-IDENTICAL to the pre-change shape: the
	// same keys, the same order, nothing filled in. An empty authorType or updatedAt must stay
	// absent — never "User", never the creation time.
	want, _ := json.Marshal(preChangeComment{AuthorLogin: "other", AuthorID: 43, CreatedAt: "2026-09-03T00:00:00Z", Body: "plain"})
	var got bytes.Buffer
	if err := json.Compact(&got, env.Items[0].Comments[1]); err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Errorf("comment 1 = %s\nwant      %s — a comment with none of the new fields must read exactly as before", got.String(), want)
	}
	for _, k := range []string{"databaseId", "authorType", "url", "updatedAt"} {
		if strings.Contains(string(env.Items[0].Comments[1]), `"`+k+`"`) {
			t.Errorf("comment 1 carries %q although the seam reported none", k)
		}
	}
}

// --- row 29 (deskread leg) and row 30: the change target ------------------------------------

func TestDeskreadCommentsChangeTarget(t *testing.T) {
	t.Run("--change reads TargetChange once", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runItemEnvelope(t, "comments", "--change", "example-org/alpha#7")
		if code != deskkit.ExitOK || env.Schema != envelopeSchema || env.Kind != "comments" || len(env.Items) != 1 {
			t.Fatalf("the change-target case: exit=%d env=%+v", code, env)
		}
		calls := f.recorded()
		if len(calls) != 1 || calls[0].Op != "ListCommentsTyped" || calls[0].Number != 7 || calls[0].Target != deskkit.TargetChange {
			t.Fatalf("the change-target case: calls = %+v, want exactly one ListCommentsTyped(7, change)", calls)
		}
		cs := env.Items[0].Comments
		if cs == nil || len(*cs) != 2 || (*cs)[0].DatabaseID != 5001 || (*cs)[0].AuthorType != "User" ||
			(*cs)[0].URL == "" || (*cs)[0].UpdatedAt == "" {
			t.Errorf("the change-target case: comments = %+v, want the same item shape with the four fields", cs)
		}
	})

	t.Run("--issue reads TargetIssue once", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runItemEnvelope(t, "comments", "--issue", "example-org/alpha#7")
		if code != deskkit.ExitOK || len(env.Items) != 1 || env.Items[0].Comments == nil || len(*env.Items[0].Comments) != 2 {
			t.Fatalf("exit=%d env=%+v", code, env)
		}
		calls := f.recorded()
		if len(calls) != 1 || calls[0].Number != 7 || calls[0].Target != deskkit.TargetIssue {
			t.Fatalf("calls = %+v, want exactly one ListCommentsTyped(7, issue)", calls)
		}
	})

	t.Run("refusals make zero calls", func(t *testing.T) {
		for _, args := range [][]string{
			{"comments", "--issue", "example-org/alpha#7", "--change", "example-org/alpha#7"},
			{"comments"},
			{"trust", "--change", "example-org/alpha#7"},
		} {
			f := stubRec(t)
			out, code, _ := rawOut(args...)
			if code != deskkit.ExitRefused || out != "" || len(f.recorded()) != 0 {
				t.Errorf("the change-target case: %v → exit=%d stdout=%q calls=%+v, want exit 5, nothing written, zero calls",
					args, code, out, f.recorded())
			}
		}
	})

	t.Run("a seam refusal is partial and never retried as an issue", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runItemEnvelope(t, "comments", "--change", "example-org/alpha#7",
			"--change", "example-org/alpha#404", "--change", "example-org/alpha#405")
		if code != deskkit.ExitOK {
			t.Fatalf("the change-target case: exit=%d, want 0 — another item in the set was read", code)
		}
		if len(env.Items) != 1 || len(env.Partial) != 2 || env.Partial[0].Number != 404 || env.Partial[1].Number != 405 {
			t.Fatalf("the change-target case: items=%+v partial=%+v, want #7 read and #404 (refused) / #405 (not found) in partial", env.Items, env.Partial)
		}
		calls := f.recorded()
		if len(calls) != 3 {
			t.Fatalf("the change-target case: calls = %+v, want exactly three — one per change, no retry", calls)
		}
		for _, c := range calls {
			if c.Target != deskkit.TargetChange {
				t.Errorf("the change-target case: call %+v read the issue target — a refused change was retried as an issue", c)
			}
		}
	})

	t.Run("a lone refused change is exit 6", func(t *testing.T) {
		f := stubRec(t)
		env, code, _ := runItemEnvelope(t, "comments", "--change", "example-org/alpha#404")
		if code != deskkit.ExitUnverifiable || len(env.Items) != 0 || len(env.Partial) != 1 {
			t.Fatalf("the change-target case: exit=%d env=%+v, want 6 with the change in partial", code, env)
		}
		if c := f.recorded(); len(c) != 1 || c[0].Target != deskkit.TargetChange {
			t.Errorf("the change-target case: calls = %+v, want one TargetChange read and no retry", c)
		}
	})
}
