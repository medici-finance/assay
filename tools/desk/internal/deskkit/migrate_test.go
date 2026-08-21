package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migFx = "testdata/migrations-fx"

// copyTree copies src into a fresh temp dir and returns it.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestLoadMigrations_ParsesFixture — the fixture migration parses: id, from/to,
// an apply step, and a non-empty human "what changed" body (OVERRIDE 1: the file
// is human + agent readable).
func TestLoadMigrations_ParsesFixture(t *testing.T) {
	migs, err := LoadMigrations(filepath.Join(migFx, MigrationsDir))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(migs) != 1 {
		t.Fatalf("got %d migrations, want 1", len(migs))
	}
	m := migs[0]
	if m.ID != "0001" || m.From != "v0.1.0" || m.To != "v0.2.0" {
		t.Errorf("meta = %q %q->%q", m.ID, m.From, m.To)
	}
	if len(m.Apply) != 1 || m.Apply[0].EnsureLine == nil {
		t.Fatalf("apply steps = %+v", m.Apply)
	}
	if !strings.Contains(strings.ToLower(m.Notes), "what changed") {
		t.Errorf("migration is missing its human-readable 'what changed' notes:\n%s", m.Notes)
	}
}

// TestSelectMigrations_Span — the fixture migration is selected for its own span.
func TestSelectMigrations_Span(t *testing.T) {
	migs, err := LoadMigrations(filepath.Join(migFx, MigrationsDir))
	if err != nil {
		t.Fatal(err)
	}
	sel, err := SelectMigrations(migs, "v0.1.0", "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(sel) != 1 {
		t.Fatalf("selected %d, want 1", len(sel))
	}
	// A span below the migration selects nothing (clean no-op path).
	none, err := SelectMigrations(migs, "v0.0.1", "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("span v0.0.1->v0.1.0 selected %d, want 0", len(none))
	}
	// from > to is refused.
	if _, err := SelectMigrations(migs, "v0.2.0", "v0.1.0"); !IsRefused(err) {
		t.Errorf("from>to should be refused, got %v", err)
	}
}

// TestRunMigrations_Idempotent — applying twice changes nothing the second time.
func TestRunMigrations_Idempotent(t *testing.T) {
	root := copyTree(t, migFx)
	migs, _ := LoadMigrations(filepath.Join(root, MigrationsDir))
	sel, _ := SelectMigrations(migs, "v0.1.0", "v0.2.0")

	first, err := RunMigrations(root, sel, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first[0].Changed {
		t.Errorf("first apply should have changed the tree: %+v", first)
	}
	target := filepath.Join(root, "docs", "UPGRADING.txt")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("target not written: %v", err)
	}

	second, err := RunMigrations(root, sel, false)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Changed {
		t.Errorf("second apply must be a no-op: %+v", second)
	}
	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Errorf("second apply mutated the file:\n%q\n%q", before, after)
	}
}

// TestRunMigrations_DryRunWritesNothing — a dry run plans the change but writes
// no file.
func TestRunMigrations_DryRunWritesNothing(t *testing.T) {
	root := copyTree(t, migFx)
	migs, _ := LoadMigrations(filepath.Join(root, MigrationsDir))
	sel, _ := SelectMigrations(migs, "v0.1.0", "v0.2.0")

	actions, err := RunMigrations(root, sel, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) == 0 || !strings.HasPrefix(actions[0].Desc, "WOULD") {
		t.Errorf("dry-run should report WOULD actions: %+v", actions)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "UPGRADING.txt")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote the target file (err=%v) — it must write nothing", err)
	}
}

// TestParseMigration_FailsClosed — a migration missing its human body, or naming
// an unknown op, fails closed rather than applying blind.
func TestParseMigration_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	noBody := "---\nid: \"9\"\nfrom: v0.1.0\nto: v0.2.0\napply:\n  - ensure-line:\n      file: a\n      text: b\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "9-x.md"), []byte(noBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMigrations(dir); !IsUnverifiable(err) {
		t.Errorf("a migration with no human body must fail closed, got %v", err)
	}

	dir2 := t.TempDir()
	badOp := "---\nid: \"9\"\nfrom: v0.1.0\nto: v0.2.0\napply:\n  - frobnicate:\n      x: y\n---\n\nnotes here\n"
	if err := os.WriteFile(filepath.Join(dir2, "9-x.md"), []byte(badOp), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMigrations(dir2); !IsUnverifiable(err) {
		t.Errorf("an unknown op must fail closed, got %v", err)
	}
}

// TestRunMigrations_SymlinkRedirectRefused — an in-repo symlink whose textual path
// is clean must not let an ensure-line write outside root. The lexical `..` check
// cannot catch this (the path contains no `..`); only symlink resolution does, and
// the write must be refused with nothing written outside root.
func TestRunMigrations_SymlinkRedirectRefused(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	// A symlink INSIDE root pointing OUTSIDE it.
	if err := os.Symlink(outside, filepath.Join(root, "evil")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	mig := Migration{
		ID: "0001", From: "v0.1.0", To: "v0.2.0",
		Apply: []Step{{EnsureLine: &EnsureLine{File: "evil/escaped.txt", Text: "pwned"}}},
	}
	if _, err := RunMigrations(root, []Migration{mig}, false); !IsRefused(err) {
		t.Fatalf("a symlink-redirected write must be refused, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.txt")); !os.IsNotExist(err) {
		t.Errorf("the write escaped root through the symlink (stat err=%v) — it must be blocked", err)
	}
}

// TestRunMigrations_LexicalEscapeRefused — a `..` path component is still refused
// (defence in depth; the lexical check remains alongside symlink resolution).
func TestRunMigrations_LexicalEscapeRefused(t *testing.T) {
	root := t.TempDir()
	mig := Migration{
		ID: "0001", From: "v0.1.0", To: "v0.2.0",
		Apply: []Step{{EnsureLine: &EnsureLine{File: "../escaped.txt", Text: "pwned"}}},
	}
	if _, err := RunMigrations(root, []Migration{mig}, false); !IsRefused(err) {
		t.Fatalf("a `..` path must be refused, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped.txt")); !os.IsNotExist(err) {
		t.Errorf("the write escaped root via `..` (stat err=%v) — it must be blocked", err)
	}
}

// TestLoadMigrations_EmptyDirIsNoError — the common case (no migrations) is cheap
// and silent, not an error.
func TestLoadMigrations_EmptyDirIsNoError(t *testing.T) {
	migs, err := LoadMigrations(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Errorf("absent migrations dir should be no error, got %v", err)
	}
	if len(migs) != 0 {
		t.Errorf("absent dir should yield no migrations, got %d", len(migs))
	}
}
