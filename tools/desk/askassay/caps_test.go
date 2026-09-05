package askassay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSilentCapRegisterMatchesTheTree — the register is a claim about this
// module, and a claim nobody checks rots. When a cap is added, removed or
// resized without editing the register, this reddens and names the site.
func TestSilentCapRegisterMatchesTheTree(t *testing.T) {
	root := moduleRoot(t)
	drifts, err := AuditListCaps(root, DeclaredListCaps())
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for _, d := range drifts {
		t.Errorf("%s", d)
	}
	if len(DeclaredListCaps()) == 0 {
		t.Fatal("the register is empty — this check would pass vacuously")
	}
	for _, c := range DeclaredListCaps() {
		if strings.TrimSpace(c.Effect) == "" {
			t.Errorf("%s: a cap row with no stated effect is an undeclared cap wearing a row", c.File)
		}
	}
}

// TestNoUndeclaredListCapInTheModule — the register holds the KNOWN caps true;
// this half catches a new one. A list call with a cap and no register row is
// the 500-against-958 shape entering the tree unannounced.
func TestNoUndeclaredListCapInTheModule(t *testing.T) {
	root := moduleRoot(t)
	undeclared, err := UndeclaredCapScan(root, DeclaredListCaps())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, f := range undeclared {
		t.Errorf("UNDECLARED CAP — %s holds a --limit call site with no row in the silent-cap register. Add the row (with its effect) or remove the cap", f)
	}
}

// TestCapAuditReportsABentTree is the positive control for both halves. It
// bends a copy of the tree in each of the ways the register can go wrong and
// requires a named report every time.
func TestCapAuditReportsABentTree(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg := []ListCap{{File: "cmd/x/x.go", Needle: `"--limit", "1000"`, Occurrences: 1, Cap: 1000, Effect: "truncates"}}

	t.Run("clean tree reports nothing", func(t *testing.T) {
		write("cmd/x/x.go", "package x\nvar a = []string{\"--limit\", \"1000\"}\n")
		d, err := AuditListCaps(root, reg)
		if err != nil {
			t.Fatal(err)
		}
		if len(d) != 0 {
			t.Fatalf("a clean tree drifted: %v", d)
		}
	})

	t.Run("cap removed", func(t *testing.T) {
		write("cmd/x/x.go", "package x\nvar a = []string{}\n")
		d, _ := AuditListCaps(root, reg)
		if len(d) != 1 || d[0].Got != 0 {
			t.Fatalf("POSITIVE CONTROL FAILED: a removed cap was not reported: %v", d)
		}
		if !strings.Contains(d[0].String(), "GONE") {
			t.Errorf("the report does not say the cap is gone: %s", d[0])
		}
	})

	t.Run("cap duplicated", func(t *testing.T) {
		write("cmd/x/x.go", "package x\nvar a = []string{\"--limit\", \"1000\"}\nvar b = []string{\"--limit\", \"1000\"}\n")
		d, _ := AuditListCaps(root, reg)
		if len(d) != 1 || d[0].Got != 2 {
			t.Fatalf("POSITIVE CONTROL FAILED: a duplicated cap was not reported: %v", d)
		}
	})

	t.Run("cap only in a comment does not count", func(t *testing.T) {
		write("cmd/x/x.go", "package x\n// we used to pass \"--limit\", \"1000\" here\n")
		d, _ := AuditListCaps(root, reg)
		if len(d) != 1 || d[0].Got != 0 {
			t.Fatalf("POSITIVE CONTROL FAILED: a commented-out cap was counted as a call site: %v", d)
		}
	})

	t.Run("declared file missing", func(t *testing.T) {
		if err := os.Remove(filepath.Join(root, "cmd", "x", "x.go")); err != nil {
			t.Fatal(err)
		}
		d, _ := AuditListCaps(root, reg)
		if len(d) != 1 || d[0].Got != -1 {
			t.Fatalf("POSITIVE CONTROL FAILED: an unreadable declared file was not reported: %v", d)
		}
	})

	t.Run("a new undeclared cap is found", func(t *testing.T) {
		write("cmd/x/x.go", "package x\nvar a = []string{\"--limit\", \"1000\"}\n")
		write("cmd/y/y.go", "package y\nvar b = []string{\"--limit\", \"25\"}\n")
		u, err := UndeclaredCapScan(root, reg)
		if err != nil {
			t.Fatal(err)
		}
		if len(u) != 1 || u[0] != "cmd/y/y.go" {
			t.Fatalf("POSITIVE CONTROL FAILED: a new undeclared cap was not found: %v", u)
		}
	})

	t.Run("test files and testdata are not call sites", func(t *testing.T) {
		write("cmd/z/z_test.go", "package z\nvar c = []string{\"--limit\", \"9\"}\n")
		write("cmd/z/testdata/f.go", "package z\nvar d = []string{\"--limit\", \"9\"}\n")
		u, _ := UndeclaredCapScan(root, reg)
		for _, f := range u {
			if strings.Contains(f, "cmd/z/") {
				t.Errorf("a test fixture was reported as a live cap site: %s", f)
			}
		}
	})
}

// TestEveryCappedQuestionRefusesAtItsCap — the register's numbers and the
// registry's SaturatesAt have to agree, or a question declares a cap it does
// not enforce.
func TestEveryCappedQuestionRefusesAtItsCap(t *testing.T) {
	for _, q := range Questions() {
		if q.SaturatesAt == 0 {
			continue
		}
		a := Computed(q, q.SaturatesAt, testStamp())
		if a.State() != CouldNotCheck {
			t.Errorf("%s: declares SaturatesAt=%d but a read at the cap answered %q", q.ID, q.SaturatesAt, a.State())
		}
		if !strings.Contains(q.Source.Limit, "1000") && !strings.Contains(q.Source.Limit, "cap") {
			t.Errorf("%s: SaturatesAt is set but the Limit text does not state the number a reader would need", q.ID)
		}
	}
}
