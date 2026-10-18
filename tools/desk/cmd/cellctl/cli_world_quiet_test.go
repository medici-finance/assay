//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// regression: #2955
func TestFixtureGitQuiet(t *testing.T) {
	for _, name := range []string{"world", "source", "empty-parent-template"} {
		t.Run(name, func(t *testing.T) {
			if name == "empty-parent-template" {
				// A second caller must get the fixture's own template, even when
				// its parent's template contains no maintenance settings.
				t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())
			}
			var repo string
			if name == "source" {
				repo = filepath.Join(t.TempDir(), "source")
				commitFixtureRepo(t, repo, "fixture")
			} else {
				repo = newCLIWorld(t, "").repoDir
			}
			for _, kv := range gitquiet.Settings {
				got := resetFixtureGit(t, repo, "config", "--local", "--default", "missing", "--get", kv[0])
				if got != kv[1] {
					t.Errorf("fixture %s = %q, want %q", kv[0], got, kv[1])
				}
			}
			// Exercise the same commit that precedes the reset snapshot. The
			// trace is outside the world, so it cannot perturb that snapshot.
			trace := filepath.Join(t.TempDir(), "git.trace")
			t.Setenv("GIT_TRACE", trace)
			resetFixtureGit(t, repo, "commit", "--allow-empty", "-qm", "fixture")
			body, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			for _, fork := range []string{"maintenance run --auto", "gc --auto"} {
				if strings.Contains(string(body), fork) {
					t.Errorf("fixture commit started %s", fork)
				}
			}
		})
	}
}
