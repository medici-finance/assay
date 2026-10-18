package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// regression: #3281
func TestClaimReaderCompatibility(t *testing.T) {
	const common = "https://github.com/example/common.git"
	const selected = "https://github.com/example/selected.git"
	for _, tc := range []struct {
		name, scope, want string
		linked, extension bool
	}{
		{"ordinary", "", common, false, false},
		{"ordinary-global", "global", common, false, false},
		{"linked", "", common, true, false},
		{"linked-global", "global", selected, true, false},
		{"linked-worktree", "worktree", selected, true, true},
		{"main-worktree", "worktree", selected, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			regression.IsolateGit(t)
			root := t.TempDir()
			global := filepath.Join(root, "global")
			if err := os.WriteFile(global, nil, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", filepath.Join(root, "home"))
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			t.Setenv("GIT_TEMPLATE_DIR", gitquiet.TemplateDir(t))
			f := &gittest.Fixture{Dir: filepath.Join(root, "main")}
			if err := os.MkdirAll(f.Dir, 0700); err != nil {
				t.Fatal(err)
			}
			local := func(args ...string) string {
				t.Helper()
				out, err := f.Git(args...)
				if err != nil {
					t.Fatal(err)
				}
				return out
			}
			local("init", "-q", "-b", "main")
			local("commit", "-q", "--allow-empty", "-m", "fixture")
			local("remote", "add", "origin", common)
			if tc.extension {
				local("config", "extensions.worktreeConfig", "true")
			}
			linked := filepath.Join(root, "linked")
			local("worktree", "add", "-q", "-b", "fixture", linked)
			dir := f.Dir
			if tc.linked {
				dir = linked
			}
			if tc.scope == "global" {
				// Fixture.Git disables ambient globals. Seed only this owned file;
				// originRemoteURL still reads it through the test's explicit env.
				local("-C", dir, "config", "--file", global, "url."+selected+".insteadOf", common)
			} else if tc.scope != "" {
				local("-C", dir, "config", "--"+tc.scope, "url."+selected+".insteadOf", common)
			}
			before := regression.TreeDigest(t, root)
			old, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chdir(old); err != nil {
					t.Error(err)
				}
			})
			if got := originRemoteURL(); got != tc.want {
				t.Errorf("origin compatibility = %q, want %q", got, tc.want)
			}
			if after := regression.TreeDigest(t, root); after != before {
				t.Error("origin read changed fixture configuration or repository state")
			}
		})
	}
}
