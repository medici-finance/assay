package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFixtureCleanupRemovesReadOnlyTree (#1195): the cleanup installFixtureRoster returns
// must remove the fixture HOME even when something under it is read-only — the shape a
// `go build` run with HOME relocated leaves behind (a module/build cache of 0444 files in
// 0555 directories), on which a bare os.RemoveAll fails with EACCES and, before this test,
// the error was dropped on the floor. The read-only tree is planted by hand so the property
// holds whether or not the build ever writes there again.
//
// This runs INSIDE the package's own TestMain fixture: a second installFixtureRoster is a
// second private HOME, and its cleanup restores HOME to the TestMain one, so the rest of the
// suite is undisturbed.
func TestFixtureCleanupRemovesReadOnlyTree(t *testing.T) {
	cleanup, err := installFixtureRoster()
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	home := os.Getenv("HOME")
	if filepath.Base(home) == "" || !filepath.IsAbs(home) {
		t.Fatalf("fixture HOME %q is not an absolute path", home)
	}
	ro := filepath.Join(home, "go", "pkg", "mod", "cache", "download")
	if err := os.MkdirAll(ro, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ro, "list"), []byte("v1.0.0\n"), 0o444); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, d := range []string{ro, filepath.Dir(ro), filepath.Dir(filepath.Dir(ro))} {
		if err := os.Chmod(d, 0o555); err != nil {
			t.Fatalf("chmod %s: %v", d, err)
		}
	}

	cleanup()

	if _, err := os.Stat(home); !os.IsNotExist(err) {
		// Leave nothing behind even when the assertion fails.
		t.Cleanup(func() {
			_ = filepath.WalkDir(home, func(p string, d os.DirEntry, werr error) error {
				if werr == nil {
					_ = os.Chmod(p, 0o700)
				}
				return nil
			})
			_ = os.RemoveAll(home)
		})
		t.Fatalf("fixture HOME %s still exists after cleanup (stat err = %v) — the cleanup must make "+
			"the tree writable before RemoveAll", home, err)
	}
}
