package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/gitcore/gittest"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
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
		{"linked-global", "global", common, true, false},
		{"linked-worktree", "worktree", "", true, true},
		{"main-worktree", "worktree", "", false, true},
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
			switch tc.scope {
			case "":
			case "global":
				// The shared fixture pins its own global config to the null device, so
				// write this rule into the test-owned file the reader sees as global.
				local("-C", dir, "config", "--file", global, "url."+selected+".insteadOf", common)
			default:
				local("-C", dir, "config", "--"+tc.scope, "url."+selected+".insteadOf", common)
			}
			before := regression.TreeDigest(t, root)
			got, err := checkedOriginURL(dir)
			if tc.want == "" {
				if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || got != "" {
					t.Errorf("origin read = %q, %v; want unmerged worktree scope", got, err)
				}
			} else if err != nil || got != tc.want {
				t.Errorf("origin read = %q, %v; want %q", got, err, tc.want)
			}
			if after := regression.TreeDigest(t, root); after != before {
				t.Error("origin read changed fixture configuration or repository state")
			}
		})
	}
}
