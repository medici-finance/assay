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
// never read, written, or drift-compared.
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

	// A tree with no .git directory at all (a `git archive`
	// export) must not fail lint purely because pre-existing legacy-numeric
	// register ids can no longer be told apart from freshly-minted ones. The
	// fixture has no .git either way (testdata/goodrepo ships plain), so this
	// exercises exactly the field report's repro path: a legacy-numeric
	// finding entry with no git history available at all.
	t.Run("git-archive export: legacy numeric register id does not fail lint", func(t *testing.T) {
		root := fresh(t)
		fdir := filepath.Join(root, "docs", "streams", "findings")
		if err := os.MkdirAll(fdir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fdir, "2026-07-08-legacy.md"), []byte(
			"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Legacy finding\naffects: []\nresolved: true\n---\n\nBody."),
			0o644); err != nil {
			t.Fatal(err)
		}
		if code := run(root, "lint", nil, nil, ""); code != 0 {
			t.Errorf("lint on a git-archive-style tree (no .git) with a legacy-numeric register id exited %d, want 0 — a pre-existing id must not be misjudged as a fresh numeric regression just because git history is unavailable", code)
		}
	})
}

// TestEmptyRootLintsGreen pins the empty-root case: a root whose docs/streams exists
// and is readable, but contains zero stream directories, must not lint clean
// and silent. It sits next to three adjacent cases — a missing docs/streams,
// an unreadable one, and a nonexistent root — which already fail closed
// through loadStreams' os.ReadDir error path; only the "exists but empty"
// case fell through, which is exactly what let a typo'd/renamed/mid-
// restructure docs/streams (or a whole repo silently dropping off a
// multi-root board) look identical to a fine root.
func TestEmptyRootLintsGreen(t *testing.T) {
	emptyRoot := func(t *testing.T) string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
			t.Fatal(err)
		}
		return root
	}

	t.Run("default: fails closed with a PROBLEM, not a silent PASS", func(t *testing.T) {
		root := emptyRoot(t)
		var code int
		stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
		if code != 1 {
			t.Errorf("lint on an empty-but-existing docs/streams exited %d, want 1 (was exit 0/LINT:PASS before the fix)", code)
		}
		if !strings.Contains(stderr, "PROBLEM:") || !strings.Contains(stderr, "0 streams") {
			t.Errorf("expected a PROBLEM naming the 0-stream root; got:\n%s", stderr)
		}
	})

	t.Run("default: write mode is blocked too, not just lint", func(t *testing.T) {
		// The issue calls write mode "worse" than lint: it would silently emit
		// a board whose footer reads 0 streams. Generation must be suppressed
		// the same way it is for a missing docs/streams.
		root := emptyRoot(t)
		if code := run(root, "write", nil, nil, ""); code != 1 {
			t.Errorf("write on an empty-but-existing docs/streams exited %d, want 1", code)
		}
		if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
			t.Errorf("write must not emit a STATUS.md for a 0-stream root by default (stat err = %v)", err)
		}
	})

	t.Run("--allow-empty-root: the genuine bootstrap case stays green, but visibly", func(t *testing.T) {
		root := emptyRoot(t)
		allowEmptyRoot = true
		t.Cleanup(func() { allowEmptyRoot = false })
		var code int
		stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
		if code != 0 {
			t.Errorf("--allow-empty-root lint exited %d, want 0", code)
		}
		if !strings.Contains(stderr, "NOTICE:") || !strings.Contains(stderr, "0 streams") {
			t.Errorf("--allow-empty-root must still surface the state as a NOTICE, not silence; got:\n%s", stderr)
		}
		if strings.Contains(stderr, "PROBLEM:") {
			t.Errorf("--allow-empty-root must not leave a PROBLEM behind; got:\n%s", stderr)
		}
	})

	t.Run("a root with only reserved register dirs (no real streams) still counts as empty", func(t *testing.T) {
		// findings/ and intake/ are registers, not streams (reservedRegisterNames
		// in load.go) — a root that has adopted only those must not be
		// mistaken for a root that has authored a stream.
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams", "findings"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams", "intake"), 0o755); err != nil {
			t.Fatal(err)
		}
		var code int
		stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
		if code != 1 {
			t.Errorf("lint on a registers-only root exited %d, want 1", code)
		}
		if !strings.Contains(stderr, "PROBLEM:") || !strings.Contains(stderr, "0 streams") {
			t.Errorf("expected a PROBLEM naming the 0-stream root; got:\n%s", stderr)
		}
	})

	t.Run("a populated root is unaffected", func(t *testing.T) {
		// Sanity: the fix must not regress the ordinary case.
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		if code := run(root, "lint", nil, nil, ""); code != 0 {
			t.Errorf("lint on a populated root exited %d, want 0", code)
		}
	})
}
