// Package gitstub makes a directory look like a repository root to gitcore
// discovery without running git. It lives apart from gittest because gittest
// registers a global -update flag at init, which collides with test packages
// that declare their own.
package gitstub

import (
	"os"
	"path/filepath"
	"testing"
)

// Repo makes dir look like a repository root to gitcore discovery without
// running git: a .git directory holding HEAD, objects/ and refs/. Use it where
// a test needs "a checkout" and never reads an object or a ref.
func Repo(t testing.TB, dir string) {
	t.Helper()
	git := filepath.Join(dir, ".git")
	for _, d := range []string{git, filepath.Join(git, "objects"), filepath.Join(git, "refs")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(git, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
