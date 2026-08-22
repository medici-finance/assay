package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture materialises a minimal, CONSISTENT adopter skeleton at umbrella
// v0.12.0 with a v0.13.0 target composition and one in-span migration — the same
// shape as test/fixtures/adopter-repo, built inline so the test is self-contained.
func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".assay-versions", ""+
		"assay v0.12.0\n"+
		"statusgen v0.12.0 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"+
		"desk-tools v0.12.0 bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n")
	write("releases/v0.12.0.yaml", ""+
		"umbrella: v0.12.0\n"+
		"artifacts:\n"+
		"  - artifact: statusgen\n    tag: v0.12.0\n    sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"+
		"  - artifact: desk-tools\n    tag: v0.12.0\n    sha256: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n")
	write("releases/v0.13.0.yaml", ""+
		"umbrella: v0.13.0\n"+
		"artifacts:\n"+
		"  - artifact: statusgen\n    tag: v0.13.0\n    sha256: cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\n"+
		"  - artifact: desk-tools\n    tag: v0.13.0\n    sha256: dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd\n")
	write("migrations/0001-v0.12.0-to-v0.13.0.md", ""+
		"---\nid: \"0001\"\nfrom: v0.12.0\nto: v0.13.0\n"+
		"apply:\n  - ensure-line:\n      file: docs/UPGRADING.txt\n      text: \"v0.13.0: upgraded\"\n---\n\n"+
		"# What changed — v0.12.0 to v0.13.0\n\nFixture release note body.\n")
	return root
}

func run2(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String() + errb.String()
}

// The refusal spine: three distinct exit codes for the three refusal kinds the
// brief's Verify row 7 exercises, plus the unknown-target code.
func TestRefusalExitCodes(t *testing.T) {
	root := writeFixture(t)

	// Wrong version kind — a per-artifact tag.
	code, out := run2(t, "--root", root, "--to", "statusgen/v0.13.0")
	if code != exitWrongVersionKind {
		t.Fatalf("component-prefixed: got exit %d, want %d", code, exitWrongVersionKind)
	}
	if !strings.Contains(out, "umbrella") {
		t.Fatalf("component-prefixed message must name the umbrella; got %q", out)
	}
	for _, bad := range []string{"assum", "nearest"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Fatalf("wrong-kind message must not contain %q; got %q", bad, out)
		}
	}

	// Unknown target — a bare version that names no published umbrella.
	code, out = run2(t, "--root", root, "--to", "v99.0.0")
	if code != exitUnknownTarget {
		t.Fatalf("unpublished target: got exit %d, want %d", code, exitUnknownTarget)
	}
	for _, bad := range []string{"assum", "nearest", "latest"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Fatalf("unknown-target message must not contain %q; got %q", bad, out)
		}
	}

	// Undetermined — no pin file at all.
	if err := os.Remove(filepath.Join(root, ".assay-versions")); err != nil {
		t.Fatal(err)
	}
	code, out = run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitUndetermined {
		t.Fatalf("undetermined: got exit %d, want %d", code, exitUndetermined)
	}
	if !strings.Contains(out, "assay-versions") {
		t.Fatalf("undetermined message must name the missing record; got %q", out)
	}
	if strings.Contains(strings.ToLower(out), "assum") || strings.Contains(strings.ToLower(out), "latest") {
		t.Fatalf("undetermined message must not guess; got %q", out)
	}

	// Distinct codes.
	if exitWrongVersionKind == exitUnknownTarget || exitWrongVersionKind == exitUndetermined || exitUnknownTarget == exitUndetermined {
		t.Fatalf("refusal exit codes must be distinct")
	}
}

func TestInconsistentRefusal(t *testing.T) {
	root := writeFixture(t)
	// statusgen pin disagrees with the v0.12.0 composition.
	os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(""+
		"assay v0.12.0\n"+
		"statusgen v0.13.0 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"+
		"desk-tools v0.12.0 bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"), 0o644)
	code, out := run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitInconsistent {
		t.Fatalf("inconsistent: got exit %d, want %d", code, exitInconsistent)
	}
	if !strings.Contains(out, "statusgen") || !strings.Contains(out, "umbrella") {
		t.Fatalf("inconsistent report must name the disagreeing records; got %q", out)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	root := writeFixture(t)
	before := snapshot(t, root)
	code, out := run2(t, "--root", root, "--dry-run", "--to", "v0.13.0")
	if code != exitOK {
		t.Fatalf("dry-run: got exit %d, want 0", code)
	}
	if !strings.Contains(out, "would run") || !strings.Contains(out, "release note") {
		t.Fatalf("dry-run must preview migrations and release notes; got %q", out)
	}
	if after := snapshot(t, root); after != before {
		t.Fatalf("dry-run must write nothing; tree changed")
	}
}

func TestApplyThenIdempotent(t *testing.T) {
	root := writeFixture(t)
	code, _ := run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitOK {
		t.Fatalf("apply: got exit %d, want 0", code)
	}
	pin, _ := os.ReadFile(filepath.Join(root, ".assay-versions"))
	if !strings.Contains(string(pin), "assay v0.13.0") || !strings.Contains(string(pin), "statusgen v0.13.0") {
		t.Fatalf("apply must re-pin to the target; got %q", string(pin))
	}
	after := snapshot(t, root)
	code, out := run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitOK {
		t.Fatalf("second apply: got exit %d, want 0", code)
	}
	if !strings.Contains(out, "already at") {
		t.Fatalf("second apply must report already-at; got %q", out)
	}
	if snapshot(t, root) != after {
		t.Fatalf("second apply must be idempotent; tree changed")
	}
}

// TestApplyMigrationFailureDoesNotStrand is the regression guard for the apply
// ordering: migrations run BEFORE the version marker is advanced, so a migration that
// fails part-way leaves the marker at `from` and a re-run retries the outstanding
// work — it never strands a half-done migration behind an "already at" no-op.
//
// It drives a two-step migration whose first ensure-line succeeds and whose second
// escapes root (a refusal RunMigrations surfaces mid-span). The apply must exit
// non-zero AND leave .assay-versions still at v0.12.0 (NOT advanced to the target).
// Under the pre-fix order (re-pin first) the marker would already read v0.13.0 here,
// and a re-run would take the no-op and strand the migration — so this asserts the
// marker was not advanced past a half-done migration. Correcting the migration and
// re-running then completes the move, proving the run was recoverable, not stranded.
func TestApplyMigrationFailureDoesNotStrand(t *testing.T) {
	root := writeFixture(t)
	migPath := filepath.Join(root, "migrations/0001-v0.12.0-to-v0.13.0.md")

	// A migration whose first step is a valid in-root ensure-line and whose second
	// step escapes root — RunMigrations does step one, then refuses step two.
	failing := "---\nid: \"0001\"\nfrom: v0.12.0\nto: v0.13.0\n" +
		"apply:\n" +
		"  - ensure-line:\n      file: docs/step1.txt\n      text: \"v0.13.0: step one done\"\n" +
		"  - ensure-line:\n      file: ../escape.txt\n      text: \"must never be written\"\n" +
		"---\n\n# What changed\n\nMid-apply-failure fixture.\n"
	if err := os.WriteFile(migPath, []byte(failing), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := run2(t, "--root", root, "--to", "v0.13.0")
	if code == exitOK {
		t.Fatalf("apply with a mid-span migration failure must not exit 0; got out=%q", out)
	}

	// The marker must NOT have advanced: a re-run has to be able to retry.
	pin, _ := os.ReadFile(filepath.Join(root, ".assay-versions"))
	if !strings.Contains(string(pin), "assay v0.12.0") {
		t.Fatalf("marker must stay at v0.12.0 after a mid-apply failure (else a re-run strands the migration); got %q", string(pin))
	}
	if strings.Contains(string(pin), "assay v0.13.0") {
		t.Fatalf("marker must NOT be advanced to the target after a mid-apply failure; got %q", string(pin))
	}
	// The escaping step must not have written outside root.
	if _, err := os.Stat(filepath.Join(root, "..", "escape.txt")); err == nil {
		t.Fatalf("escaping ensure-line must not write outside root")
	}

	// Recovery: correct the migration (drop the escaping step) and re-run. Because the
	// marker was preserved at v0.12.0, the span is re-selected and completes; the
	// already-applied step one is idempotent, so it no-ops.
	fixed := "---\nid: \"0001\"\nfrom: v0.12.0\nto: v0.13.0\n" +
		"apply:\n" +
		"  - ensure-line:\n      file: docs/step1.txt\n      text: \"v0.13.0: step one done\"\n" +
		"---\n\n# What changed\n\nCorrected fixture.\n"
	if err := os.WriteFile(migPath, []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitOK {
		t.Fatalf("re-run after correcting the migration must complete; got exit %d, out=%q", code, out)
	}
	pin, _ = os.ReadFile(filepath.Join(root, ".assay-versions"))
	if !strings.Contains(string(pin), "assay v0.13.0") {
		t.Fatalf("re-run must advance the marker to the target once migrations succeed; got %q", string(pin))
	}
	step1, _ := os.ReadFile(filepath.Join(root, "docs/step1.txt"))
	if !strings.Contains(string(step1), "v0.13.0: step one done") {
		t.Fatalf("outstanding migration work must be present after recovery; got %q", string(step1))
	}
}

// snapshot returns a stable string of every file path+content under root, for
// before/after equality checks.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b.WriteString(rel + "\x00" + string(data) + "\x01")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
