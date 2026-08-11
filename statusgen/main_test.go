package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesAndChecks(t *testing.T) {
	// copy fixture to a temp dir so the test can write STATUS.md
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	if code := run(root, "write", nil, nil, ""); code != 0 {
		t.Fatalf("write run exited %d", code)
	}
	out, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "alpha") {
		t.Error("STATUS.md missing stream")
	}
	if code := run(root, "check", nil, nil, ""); code != 0 {
		t.Errorf("check run on fresh output exited %d, want 0", code)
	}
	// corrupt STATUS.md → check must fail
	if err := os.WriteFile(filepath.Join(root, "STATUS.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run(root, "check", nil, nil, ""); code != 1 {
		t.Errorf("check run on stale output exited %d, want 1", code)
	}
}

// TestLint covers the --lint mode: every source check runs, but STATUS.md is
// never read, written, or drift-compared (methodology/15).
func TestLint(t *testing.T) {
	fresh := func(t *testing.T) string {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		return root
	}

	t.Run("valid sources pass", func(t *testing.T) {
		if code := run(fresh(t), "lint", nil, nil, ""); code != 0 {
			t.Errorf("lint on valid repo exited %d, want 0", code)
		}
	})

	t.Run("no write — STATUS.md is not created", func(t *testing.T) {
		root := fresh(t) // goodrepo ships no STATUS.md
		if code := run(root, "lint", nil, nil, ""); code != 0 {
			t.Fatalf("lint exited %d", code)
		}
		if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
			t.Error("lint must not write STATUS.md")
		}
	})

	t.Run("no drift compare — a stale STATUS.md still passes lint", func(t *testing.T) {
		root := fresh(t)
		if err := os.WriteFile(filepath.Join(root, "STATUS.md"), []byte("stale — not regenerated"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := run(root, "lint", nil, nil, ""); code != 0 {
			t.Errorf("lint must not compare drift; exited %d, want 0", code)
		}
		if code := run(root, "check", nil, nil, ""); code != 1 {
			t.Errorf("sanity: check must catch the same drift; exited %d, want 1", code)
		}
	})

	t.Run("broken sources fail", func(t *testing.T) {
		root := fresh(t)
		readme := filepath.Join(root, "docs/streams/alpha/README.md")
		b, _ := os.ReadFile(readme)
		if err := os.WriteFile(readme, []byte(strings.Replace(string(b), "status: active", "status: bogus", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := run(root, "lint", nil, nil, ""); code != 1 {
			t.Errorf("lint on invalid stream status exited %d, want 1", code)
		}
	})
}
