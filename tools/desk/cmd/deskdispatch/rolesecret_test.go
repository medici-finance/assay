package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rsForge answers only the reads the role-secret admission makes. Every other Forge method
// promotes off the embedded nil interface and would panic if the admission reached for one.
type rsForge struct {
	deskkit.Forge
	issue    *deskkit.Issue
	pr       *deskkit.PullRequest
	tp       *deskkit.TrustPayload
	tpErr    error
	tip      *deskkit.RepoCommit
	cmp      *deskkit.RefComparison
	cmpErr   error
	compared [2]string
	kinds    []deskkit.TargetKind
}

// GetIssueTyped is the only issue read rsForge answers: a bare GetIssue call promotes off the
// nil interface and panics, so the admission cannot fall back to the bare-number resolver.
// It hands the issue back whatever its kind, so the admission's own IsPullRequest check is
// what a pull-request case exercises.
func (f *rsForge) GetIssueTyped(_ deskkit.ForgeRepo, _ int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	f.kinds = append(f.kinds, kind)
	if f.issue == nil {
		return nil, errors.New("no such issue")
	}
	return f.issue, nil
}
func (f *rsForge) IssueTrustEvents(deskkit.ForgeRepo, int) (*deskkit.TrustPayload, error) {
	return f.tp, f.tpErr
}
func (f *rsForge) GetPullRequest(deskkit.ForgeRepo, int) (*deskkit.PullRequest, error) {
	if f.pr == nil {
		return nil, errors.New("no such change")
	}
	return f.pr, nil
}
func (f *rsForge) PRTrustEvents(deskkit.ForgeRepo, int) (*deskkit.TrustPayload, error) {
	return f.tp, f.tpErr
}
func (f *rsForge) GetCommit(deskkit.ForgeRepo, string) (*deskkit.RepoCommit, error) {
	if f.tip == nil {
		return nil, errors.New("no such ref")
	}
	return f.tip, nil
}
func (f *rsForge) CompareRefs(_ deskkit.ForgeRepo, base, head string) (*deskkit.RefComparison, error) {
	f.compared = [2]string{base, head}
	return f.cmp, f.cmpErr
}

const (
	rsHead = "1111111111111111111111111111111111111111"
	rsTip  = "2222222222222222222222222222222222222222"
)

// rsWindow makes this test's window a holding one (or not) and points the admission at f.
func rsWindow(t *testing.T, holding bool, f deskkit.Forge) {
	t.Helper()
	env := []string{"PATH=/bin"}
	if holding {
		env = append(env, "ASSAY_ROLE_SECRET_FAKE_SVC_FILE=/run/secrets/role-fake-svc")
	}
	origEnv, origForge, origGlob := roleSecretEnviron, roleSecretForgeFn, roleSecretMountGlob
	roleSecretEnviron = func() []string { return env }
	roleSecretMountGlob = filepath.Join(t.TempDir(), "role-*")
	roleSecretForgeFn = func(deskkit.ForgeRepo) (deskkit.Forge, error) {
		if f == nil {
			return nil, errors.New("forge unreachable")
		}
		return f, nil
	}
	t.Cleanup(func() { roleSecretEnviron, roleSecretForgeFn, roleSecretMountGlob = origEnv, origForge, origGlob })
}

func rsTrustedChange() *rsForge {
	return &rsForge{
		pr:  &deskkit.PullRequest{Number: 7, Author: deskkit.Account{Login: "ada", ID: 2001}, HeadSHA: rsHead, BaseRef: "main", CrossRepo: deskkit.CrossRepoSame},
		tp:  &deskkit.TrustPayload{Complete: true},
		tip: &deskkit.RepoCommit{SHA: rsTip},
		cmp: &deskkit.RefComparison{CommitsComplete: true, AheadBy: 2, Commits: []deskkit.RepoCommit{
			{SHA: "3333333333333333333333333333333333333333", AuthorLogin: "ada", CommitterLogin: "ada"},
			{SHA: rsHead, AuthorLogin: "shared-agent", CommitterLogin: "ada"},
		}},
	}
}

func wantRefusal(t *testing.T, what string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: admitted", what)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused && code != deskkit.ExitUnverifiable {
		t.Fatalf("%s: exit %d, want a refusal: %v", what, code, err)
	}
	if !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "nothing was claimed") {
		t.Fatalf("%s: %v (want %q)", what, err, want)
	}
}

// In a window holding a role-secret path, an untrusted unblessed item, a
// review kit, an item with no forge item behind it, and a change whose head commits are not
// all linked to roster logins are refused before the claim; a trusted item is admitted.
// regression: #6065
func TestRoleSecretUntrustedHead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	issue := dispatchOpts{item: "assay--issue-12", kit: "worker"}
	change := dispatchOpts{item: "assay--issue-7", kit: "worker", pr: 7}

	t.Run("precheck", func(t *testing.T) {
		rsWindow(t, true, nil)
		wantRefusal(t, "review kit", roleSecretPrecheck(dispatchOpts{item: "assay--issue-12", kit: "review", pr: 7}), "review-kit dispatch")
		wantRefusal(t, "brief item", roleSecretPrecheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}), "not an issue or --pr item")
		wantRefusal(t, "verifier item", roleSecretPrecheck(dispatchOpts{item: "assay--example-a--00", kit: "verifier"}), "not an issue or --pr item")
		if err := roleSecretPrecheck(issue); err != nil {
			t.Fatalf("issue item: %v", err)
		}
		if _, err := roleSecretAdmission(issue, allowedRepo); err == nil {
			t.Fatal("an unreachable forge admitted the item")
		}
	})

	t.Run("non-holding-window-unaffected", func(t *testing.T) {
		rsWindow(t, false, nil)
		if err := roleSecretPrecheck(dispatchOpts{item: "assay--example-a--00", kit: "review"}); err != nil {
			t.Fatal(err)
		}
		if head, err := roleSecretAdmission(change, allowedRepo); err != nil || head != "" {
			t.Fatalf("%q %v", head, err)
		}
	})

	t.Run("issue", func(t *testing.T) {
		body := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for name, tc := range map[string]struct {
			author deskkit.Account
			tp     *deskkit.TrustPayload
			tpErr  error
			pr     bool
			item   string
			want   string // "" = admitted
		}{
			"trusted":              {deskkit.Account{Login: "ada", ID: 2001}, &deskkit.TrustPayload{Complete: true}, nil, false, "", ""},
			"untrusted":            {deskkit.Account{Login: "stranger", ID: 9}, &deskkit.TrustPayload{Complete: true}, nil, false, "", "is not trusted and it carries no current blessing"},
			"blessed":              {deskkit.Account{Login: "stranger", ID: 9}, &deskkit.TrustPayload{Complete: true, BodyEdited: body, Events: []deskkit.ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: body.Add(time.Hour)}}}, nil, false, "", ""},
			"blessing-voided":      {deskkit.Account{Login: "stranger", ID: 9}, &deskkit.TrustPayload{Complete: true, BodyEdited: body.Add(2 * time.Hour), Events: []deskkit.ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: body.Add(time.Hour)}}}, nil, false, "", "carries no current blessing"},
			"incomplete-read":      {deskkit.Account{Login: "ada", ID: 2001}, &deskkit.TrustPayload{Complete: false}, nil, false, "", "carries no current blessing"},
			"unreadable-events":    {deskkit.Account{Login: "ada", ID: 2001}, nil, errors.New("rate limited"), false, "", "trust events"},
			"roster-id-mismatch":   {deskkit.Account{Login: "ada", ID: 77}, &deskkit.TrustPayload{Complete: true}, nil, false, "", "carries no current blessing"},
			"is-a-pull-request":    {deskkit.Account{Login: "ada", ID: 2001}, &deskkit.TrustPayload{Complete: true}, nil, true, "", "dispatch it with --pr"},
			"other-repo-qualifier": {deskkit.Account{Login: "ada", ID: 2001}, &deskkit.TrustPayload{Complete: true}, nil, false, "example-org/tracker:issue-12", "not the dispatch target"},
		} {
			t.Run(name, func(t *testing.T) {
				f := &rsForge{issue: &deskkit.Issue{Number: 12, Author: tc.author, IsPullRequest: tc.pr}, tp: tc.tp, tpErr: tc.tpErr}
				rsWindow(t, true, f)
				o := issue
				if tc.item != "" {
					o.item = tc.item
				}
				_, err := roleSecretAdmission(o, allowedRepo)
				if tc.item == "" && len(f.kinds) == 0 {
					t.Fatal("the admission decided an issue item without reading the issue")
				}
				for _, k := range f.kinds {
					if k != deskkit.TargetIssue {
						t.Fatalf("an issue item was read as kind %q, want %q", k, deskkit.TargetIssue)
					}
				}
				if tc.want == "" {
					if err != nil {
						t.Fatalf("refused: %v", err)
					}
					return
				}
				wantRefusal(t, name, err, tc.want)
			})
		}
	})

	t.Run("change", func(t *testing.T) {
		f := rsTrustedChange()
		rsWindow(t, true, f)
		head, err := roleSecretAdmission(change, allowedRepo)
		if err != nil || head != rsHead {
			t.Fatalf("a trusted same-repo change with roster-linked commits: %q %v", head, err)
		}
		if f.compared != [2]string{rsTip, rsHead} {
			t.Fatalf("compared %v, want the base branch's current tip against the head", f.compared)
		}
		for name, tc := range map[string]struct {
			mutate func(f *rsForge)
			want   string
		}{
			"untrusted-author":       {func(f *rsForge) { f.pr.Author = deskkit.Account{Login: "stranger", ID: 9} }, "is not trusted"},
			"fork-head":              {func(f *rsForge) { f.pr.CrossRepo = deskkit.CrossRepoFork }, "not confirmed to be the base repository"},
			"unknown-head-repo":      {func(f *rsForge) { f.pr.CrossRepo = "" }, "not confirmed to be the base repository"},
			"stranger-commit-author": {func(f *rsForge) { f.cmp.Commits[1].AuthorLogin = "stranger" }, `author "stranger", which is not a roster login`},
			"stranger-committer":     {func(f *rsForge) { f.cmp.Commits[0].CommitterLogin = "web-flow" }, `committer "web-flow", which is not a roster login`},
			"unlinked-author":        {func(f *rsForge) { f.cmp.Commits[0].AuthorLogin = "" }, "has no linked author account"},
			"unlinked-committer":     {func(f *rsForge) { f.cmp.Commits[1].CommitterLogin = "" }, "has no linked committer account"},
			"incomplete-commits":     {func(f *rsForge) { f.cmp.CommitsComplete = false }, "cannot read every commit"},
			"short-commit-list":      {func(f *rsForge) { f.cmp.AheadBy = 3 }, "cannot read every commit"},
			"compare-unreadable":     {func(f *rsForge) { f.cmp, f.cmpErr = nil, errors.New("timeout") }, "cannot read every commit"},
			"base-tip-unreadable":    {func(f *rsForge) { f.tip = nil }, "cannot read the tip"},
			"head-missing":           {func(f *rsForge) { f.pr.HeadSHA = "" }, "head or base branch is missing"},
			"change-unreadable":      {func(f *rsForge) { f.pr = nil }, "cannot read #7"},
		} {
			t.Run(name, func(t *testing.T) {
				f := rsTrustedChange()
				tc.mutate(f)
				rsWindow(t, true, f)
				_, err := roleSecretAdmission(change, allowedRepo)
				wantRefusal(t, name, err, tc.want)
			})
		}
	})

	t.Run("head-pin", func(t *testing.T) {
		if err := roleSecretPinHead("", dispatchPlan{}); err != nil {
			t.Fatal(err)
		}
		if err := roleSecretPinHead(rsHead, dispatchPlan{resume: &resumeSource{head: rsHead}}); err != nil {
			t.Fatal(err)
		}
		wantRefusal(t, "moved head", roleSecretPinHead(rsHead, dispatchPlan{resume: &resumeSource{head: rsTip}}), "head moved after the role-secret trust check")
		wantRefusal(t, "no resume", roleSecretPinHead(rsHead, dispatchPlan{}), "head moved")
	})
}

// Through the verb itself: a holding window's review-kit dispatch exits 5
// before any child process (so before any claim), and no child deskdispatch starts carries a
// role-secret variable.
// regression: #6065
func TestRoleSecretDispatchRefusesBeforeClaim(t *testing.T) {
	_, calls := ddWorktreeEnv(t)
	rsWindow(t, true, nil)
	_, errOut, rc := ddRunCapture(t, []string{"assay--issue-12", "--root", t.TempDir(), "--kit", "review", "--pr", "7", "--repo", allowedRepo, "--dry-run"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc %d, want %d: %s", rc, deskkit.ExitRefused, errOut)
	}
	if !strings.Contains(errOut, "review-kit dispatch from a window holding a role secret") {
		t.Fatalf("stderr: %s", errOut)
	}
	if len(*calls) != 0 {
		t.Fatalf("a refused dispatch started children: %v", *calls)
	}

	t.Setenv("ASSAY_ROLE_SECRET_FAKE_SVC_FILE", "/run/secrets/role-fake-svc")
	inherited := roleSecretChildEnv(nil)
	if inherited == nil {
		t.Fatal("a holding process left its child to inherit the environment")
	}
	for _, kv := range inherited {
		if deskkit.IsRoleSecretEnvName(strings.SplitN(kv, "=", 2)[0]) {
			t.Fatalf("child environment carries %s", kv)
		}
	}
	// The children below run for real (the fake seam above answers with no output), and each
	// must print an environment, so a check that reads nothing cannot pass.
	fake := execCommand
	execCommand = exec.Command
	t.Cleanup(func() { execCommand = fake })
	for _, res := range []runResult{
		runCmd(t.TempDir(), "env"),
		runCmdEnv(t.TempDir(), append(os.Environ(), "assay_role_secret_lower_file=/x"), "env"),
	} {
		if !strings.Contains(res.stdout, "PATH=") {
			t.Fatalf("the env child printed no environment: %q (%v)", res.stdout, res.err)
		}
		if strings.Contains(strings.ToUpper(res.stdout), "ASSAY_ROLE_SECRET_") {
			t.Fatalf("a child saw a role-secret variable: %s", res.stdout)
		}
	}
}

// A window whose role-secret variable was unset still holds the secret while its file is mounted
// at the fixed path, so the mount alone makes it a holding window.
// regression: #6065
func TestRoleSecretMountHolds(t *testing.T) {
	rsWindow(t, false, nil)
	if holdingRoleSecret() {
		t.Fatal("a window with neither the variable nor the mount held a secret")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "role-fake-svc"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	roleSecretMountGlob = filepath.Join(dir, "role-*")
	if !holdingRoleSecret() {
		t.Fatal("a mounted role secret with no variable was not a holding window")
	}
	err := roleSecretPrecheck(dispatchOpts{item: "assay--issue-12", kit: "review", pr: 7})
	wantRefusal(t, "mount-only review kit", err, "review-kit dispatch")
	// A mount-only window carries no variable, so the refusal must name the mount as a trigger
	// too, or it points the operator at a variable the window does not have.
	if !strings.Contains(err.Error(), "/run/secrets/role-*") {
		t.Fatalf("mount-only refusal does not name the mount trigger: %v", err)
	}
}

// The package's other tests never call rsWindow, so they must not read the real environment or
// the real mount path: run inside a holding container, they would otherwise hit the holding-window
// refusals for reasons unrelated to what they test. TestMain pins both seams to an unheld window.
// regression: #6065
func TestRoleSecretSeamsPinned(t *testing.T) {
	if deskkit.HoldsRoleSecret(os.Environ()) {
		t.Fatal("the test process kept a role-secret variable, so every child env is rewritten")
	}
	t.Setenv("ASSAY_ROLE_SECRET_FAKE_SVC_FILE", "/run/secrets/role-fake-svc")
	if holdingRoleSecret() {
		t.Fatal("a test outside rsWindow read the process environment as a holding window")
	}
	if m, _ := filepath.Glob(roleSecretMountGlob); len(m) > 0 || roleSecretMountGlob == "/run/secrets/role-*" {
		t.Fatalf("a test outside rsWindow reads the real mount path %q", roleSecretMountGlob)
	}
}
