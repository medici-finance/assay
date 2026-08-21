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
