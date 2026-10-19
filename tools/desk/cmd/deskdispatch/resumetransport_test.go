package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// regression: #3327
func TestResumeRoleTransport(t *testing.T) {
	s := &stub{}
	home, _ := s.install(t)
	refreshResume = liveRefreshResume
	const sourceRepo = "example-resume/tracker"
	cfg := filepath.Join(home, ".config", "assay", "roster.env")
	if err := os.WriteFile(cfg, []byte(strings.ReplaceAll(fixtureRoster, "example-org/tracker", sourceRepo)), 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	// Native Git must never contact the configured network origin on the red run.
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "fetch" {
			return exec.Command("/bin/sh", "-c", "echo 'raw Git has no role credential' >&2; exit 1")
		}
		return exec.Command(name, args...)
	}
	fixture := gittest.NewFixture(t)
	fixture.Git("branch", "fix/source")
	fixture.Git("remote", "add", "origin", "https://github.com/"+sourceRepo+".git")
	head, err := fixture.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	dest := filepath.Join(project, sourceRepo+".git")
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := fixture.Git("clone", "--bare", fixture.Dir, dest); err != nil {
		t.Fatal(out, err)
	}
	requests := gittest.RoleHTTP(t, project, "resume-role-fixture")
	mints := 0
	restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mints++
		if role != "desk" || owner != deskkit.OwnerOf(sourceRepo) {
			t.Fatalf("wrong custody %s/%s", role, owner)
		}
		path := filepath.Join(home, "resume.token")
		err := os.WriteFile(path, []byte("resume-role-fixture"), 0600)
		return path, "", err
	})
	defer restore()
	got, err := worktreeBase(dispatchOpts{root: fixture.Dir, repo: sourceRepo}, "fix/source", head)
	if err != nil || got != head {
		t.Fatalf("role refresh: got %q, err %v", got, err)
	}
	if _, err := fixture.Git("rev-parse", "--verify", "refs/remotes/origin/main"); err == nil {
		t.Fatal("resume widened its tracking-ref write scope")
	}
	if mints != 1 || requests.Requests.Load() == 0 {
		t.Fatalf("credential boundary not reached: mints=%d requests=%d", mints, requests.Requests.Load())
	}
	readResumeChange = func(dispatchOpts, string) (deskkit.PullRequest, error) {
		return deskkit.PullRequest{State: "open", HeadRef: "fix/source", HeadSHA: head, CrossRepo: deskkit.CrossRepoSame}, nil
	}
	plan := dispatchPlan{repo: "example-org/agents"}
	if err := resolveResume(dispatchOpts{root: fixture.Dir, pr: 42, kit: "worker"}, &plan); err == nil {
		t.Fatal("auto-derived target repository was not bound to source refresh")
	}
	if _, err := worktreeBase(dispatchOpts{root: fixture.Dir, repo: sourceRepo}, "fix/source", resumeSHA); err == nil {
		t.Fatal("moved forge head accepted")
	}
}
