package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Each clause of --adopt-run's proof is load-bearing: break one and the original refusal
// comes back naming it, with the home untouched. The happy row proves the same home is
// handed back on stdout.
func TestAdoptRunClauses(t *testing.T) {
	const runKey = "assay--issue-9"
	cases := []struct {
		name  string
		plant func(t *testing.T, work, home string)
		want  string // "" = adopted
	}{
		{"clean finished home", func(*testing.T, string, string) {}, ""},
		{"untracked file", func(t *testing.T, _, home string) {
			writeFile(t, filepath.Join(home, "scratch.txt"), "x\n")
		}, "uncommitted or untracked"},
		{"unpushed commit", func(t *testing.T, _, home string) {
			mustGit(t, home, "-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-m", "local")
		}, "not the pinned head"},
		{"locked", func(t *testing.T, work, home string) {
			mustGit(t, work, "worktree", "lock", "--reason", "live", home)
		}, "locked (live)"},
		{"other run key", func(t *testing.T, _, home string) {
			mustGit(t, home, "config", "--worktree", "assay.runKey", "assay--issue-8")
		}, "assay.runKey"},
		{"merge in progress", func(t *testing.T, _, home string) {
			head := mustGit(t, home, "rev-parse", "HEAD")
			gitPath := mustGit(t, home, "rev-parse", "--git-path", "MERGE_HEAD")
			if !filepath.IsAbs(gitPath) {
				gitPath = filepath.Join(home, gitPath)
			}
			writeFile(t, gitPath, head+"\n")
		}, "MERGE_HEAD"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			head := mustGit(t, work, "rev-parse", "HEAD")
			upstream := "refs/remotes/origin/feat"
			mustGit(t, work, "update-ref", upstream, head)
			add := []string{"add", "adopt", "--branch", "feat", "--base", head, "--upstream", upstream}
			if rc, stderr := runCapErr(t, add); rc != deskkit.ExitOK {
				t.Fatalf("fixture add rc=%d:\n%s", rc, stderr)
			}
			home := resolvePath(filepath.Join(tmpBaseDir, "tracker-adopt"))
			mustGit(t, home, "config", "extensions.worktreeConfig", "true")
			mustGit(t, home, "config", "--worktree", "assay.runKey", runKey)
			c.plant(t, work, home)

			rc, stdout, stderr := runCapBoth(t, append(add, "--adopt-run", runKey))
			if c.want == "" {
				if rc != deskkit.ExitOK || strings.TrimSpace(stdout) != home {
					t.Fatalf("rc=%d stdout=%q, want the home %s adopted:\n%s", rc, stdout, home, stderr)
				}
				return
			}
			if rc != deskkit.ExitRefused || !strings.Contains(stderr, "never clobbered") || !strings.Contains(stderr, c.want) {
				t.Fatalf("rc=%d, want a refusal naming %q:\n%s", rc, c.want, stderr)
			}
			if _, err := os.Stat(home); err != nil {
				t.Fatalf("refusal touched the home: %v", err)
			}
		})
	}
}

func TestAdoptRunNeedsUpstream(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	rc, stderr := runCapErr(t, []string{"add", "adopt", "--adopt-run", "assay--issue-9"})
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "--adopt-run requires --upstream") {
		t.Fatalf("rc=%d:\n%s", rc, stderr)
	}
}
