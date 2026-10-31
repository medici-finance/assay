package gitcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #4823
func TestCheckoutOriginContext(t *testing.T) {
	for _, tc := range []struct{ name, origin, want string }{
		{"https-port", "https://github.com:443/example/project.git", "https://github.com:443/example/project.git"},
		{"ssh", "git@github.com:example/project.git", "ssh://github.com/example/project.git"},
		{"foreign", "https://gitlab.example/example/project.git", "https://gitlab.example/example/project.git"},
		{"credentials", "https://fixture-user:fixture-password@github.com/example/project.git?fixture-secret=yes#fixture-secret", "https://github.com/example/project.git"},
		{"missing", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := gittest.NewFixture(t)
			sha := f.CommitFile(t, "check.txt", "check\n", "fixture")
			git := func(args ...string) string {
				t.Helper()
				out, err := f.Git(args...)
				if err != nil {
					t.Fatal(err)
				}
				return strings.TrimSpace(out)
			}
			git("update-ref", "refs/remotes/origin/main", sha)
			if tc.origin != "" {
				git("remote", "add", "origin", tc.origin)
			}
			git("config", "credential.helper", "fixture-secret-helper")
			git("config", "http.extraHeader", "fixture-secret-header")
			git("config", "core.hooksPath", "fixture-secret-hooks")
			before, err := os.ReadFile(filepath.Join(f.Dir, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			// This independent caller plants the same lost-context defect outside deskevidence.
			dest := t.TempDir()
			if err := CheckoutLocalCommit(f.Dir, dest, sha, "refs/remotes/origin/main"); err != nil {
				t.Fatal(err)
			}
			r, err := Open(dest)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.RemoteURL("origin")
			if tc.want == "" {
				if err == nil {
					t.Fatalf("invented origin %q", got)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("planted checkout lost repository authority: origin=%q err=%v want=%q", got, err, tc.want)
			}
			config, err := os.ReadFile(filepath.Join(dest, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(config), "fixture-") {
				t.Fatalf("credentials/config copied: %s", config)
			}
			after, err := os.ReadFile(filepath.Join(f.Dir, ".git", "config"))
			if err != nil || string(after) != string(before) {
				t.Fatal("source configuration changed")
			}
			head, err := r.Resolve("HEAD")
			if err != nil || head.String() != sha {
				t.Fatal("checkout revision changed")
			}
		})
	}
}
