package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func fxRoot(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "deskkit", "testdata", "migrations-fx")
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

func TestDeskmigrate_ApplyThenNoop(t *testing.T) {
	root := fxRoot(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--from", "v0.1.0", "--to", "v0.2.0", "--root", root}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("apply exit = %d; stderr=%s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "UPGRADING.txt")); err != nil {
		t.Fatalf("apply did not write target: %v", err)
	}
	// Second run is a clean no-op.
	var out2, errb2 bytes.Buffer
	if code := run([]string{"--from", "v0.1.0", "--to", "v0.2.0", "--root", root}, &out2, &errb2); code != deskkit.ExitOK {
		t.Fatalf("re-run exit = %d", code)
	}
	if !strings.Contains(out2.String(), "no-op") {
		t.Errorf("re-run should report a no-op:\n%s", out2.String())
	}
}

func TestDeskmigrate_DryRunWritesNothing(t *testing.T) {
	root := fxRoot(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--from", "v0.1.0", "--to", "v0.2.0", "--root", root, "--dry-run"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("dry-run exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "UPGRADING.txt")); !os.IsNotExist(err) {
		t.Errorf("dry-run must write nothing (err=%v)", err)
	}
}

func TestDeskmigrate_Notes(t *testing.T) {
	root := fxRoot(t)
	var out, errb bytes.Buffer
	run([]string{"--from", "v0.1.0", "--to", "v0.2.0", "--root", root, "--dry-run", "--notes"}, &out, &errb)
	if !strings.Contains(strings.ToLower(out.String()), "what changed") {
		t.Errorf("--notes must surface the human 'what changed' body:\n%s", out.String())
	}
}

func TestDeskmigrate_EmptySpanNoop(t *testing.T) {
	root := fxRoot(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--from", "v0.0.1", "--to", "v0.1.0", "--root", root}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("empty span exit = %d", code)
	}
	if !strings.Contains(out.String(), "no migrations") {
		t.Errorf("empty span should be a clean no-op:\n%s", out.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// The silent no-op: an empty migration set on a tree that still carries
// `schema: brief-v1` is a refusal, not a clean no-op.
// ─────────────────────────────────────────────────────────────────────────────

// writeBriefV1 drops an unmigrated brief into <root>/docs/streams/<stream>/.
func writeBriefV1(t *testing.T, root, stream, file string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nbrief: " + stream + "/01\ntitle: A brief\nwave: 0\neffort: M\nschema: brief-v1\n---\n\n# Brief 01\n"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDeskmigrate_EmptySelectionOnUnmigratedTreeRefuses is the guard for the
// silent no-op. A tree with no applicable migration but brief-v1 files still in
// it must NOT report success: exit 0 there is indistinguishable from a completed
// migration, and the operator moves on with an unmigrated tree.
//
// Fail-first: against the pre-fix run() — which printed "no migrations for … 
// (clean no-op)" and returned ExitOK the moment len(selected)==0 — this test
// fails on the very first assertion (exit = 0, want 5).
func TestDeskmigrate_EmptySelectionOnUnmigratedTreeRefuses(t *testing.T) {
	root := t.TempDir() // no migrations/ directory at all
	writeBriefV1(t, root, "svc", "brief-01-a.md")

	var out, errb bytes.Buffer
	code := run([]string{"--from", "v1.0.0", "--to", "v1.0.1", "--root", root}, &out, &errb)
	if code == deskkit.ExitOK {
		t.Fatalf("an empty migration set over an unmigrated tree must not exit 0; stdout=%s stderr=%s", out.String(), errb.String())
	}
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	msg := errb.String()
	// The message must name the migrations directory it looked in …
	if !strings.Contains(msg, filepath.Join(root, deskkit.MigrationsDir)) {
		t.Errorf("refusal does not name the migrations directory:\n%s", msg)
	}
	// … and the remedy (vendoring the release's migrations in).
	if !strings.Contains(strings.ToLower(msg), "vendor") {
		t.Errorf("refusal does not name the vendoring step:\n%s", msg)
	}
	// … and the evidence that made it a refusal rather than a no-op.
	if !strings.Contains(msg, "brief-v1") {
		t.Errorf("refusal does not name the brief-v1 files it found:\n%s", msg)
	}
	if strings.Contains(out.String(), "clean no-op") {
		t.Errorf("refusal must not also print a clean no-op:\n%s", out.String())
	}
}

// TestDeskmigrate_EmptySelectionOnCleanTreeIsStillNoop is the other branch: with
// no brief-v1 file anywhere, an empty selection is a genuine clean no-op and
// stays exit 0. The refusal above must not become a blanket one.
func TestDeskmigrate_EmptySelectionOnCleanTreeIsStillNoop(t *testing.T) {
	root := t.TempDir()
	// A migrated tree: docs/streams exists, and everything in it is brief-v2.
	dir := filepath.Join(root, "docs", "streams", "svc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nbrief: example:app:svc:01\ntitle: A brief\nwave: 0\neffort: M\nschema: brief-v2\nversion: 1\n---\n\n# Brief 01\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-01-a.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := run([]string{"--from", "v1.0.0", "--to", "v1.0.1", "--root", root}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want %d (clean no-op); stderr=%s", code, deskkit.ExitOK, errb.String())
	}
	if !strings.Contains(out.String(), "clean no-op") {
		t.Errorf("a migrated tree with no applicable migration should report a clean no-op:\n%s", out.String())
	}
}

// TestDeskmigrate_PrintsSelectedAndPlannedCounts pins the second half of the
// legibility fix: every run states how many migrations were selected and how many
// file actions are planned, so "0 selected" is legible from "selected but already
// applied" without re-deriving either.
func TestDeskmigrate_PrintsSelectedAndPlannedCounts(t *testing.T) {
	for _, c := range []struct{ name, from, to string }{
		{"a span with a migration", "v0.1.0", "v0.2.0"},
		{"an empty span", "v0.0.1", "v0.1.0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := fxRoot(t)
			var out, errb bytes.Buffer
			if code := run([]string{"--from", c.from, "--to", c.to, "--root", root, "--dry-run"}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("exit = %d; stderr=%s", code, errb.String())
			}
			s := out.String()
			if !strings.Contains(s, "migration(s) selected") {
				t.Errorf("run does not print the selected migration count:\n%s", s)
			}
			if !strings.Contains(s, "planned file action(s)") {
				t.Errorf("run does not print the planned file count:\n%s", s)
			}
		})
	}
}
