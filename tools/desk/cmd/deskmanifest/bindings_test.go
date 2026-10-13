package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bindingFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeKeysMD(t, root, "`assay.harness` is `exclusive: true`\n")
	for _, name := range []string{"claude", "cursor"} {
		writeManifest(t, root, name, "component: assay/"+name+"\nversion: 1.0.0\nprovides:\n  - key: assay.harness\n    flavour: "+name+"\n    evidence: "+name+".installed\ninject:\n  required: []\napply: []\n")
		if err := os.WriteFile(filepath.Join(root, name+".installed"), []byte("installed"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest(t, root, "hooks", "component: assay/hooks\nversion: 1.0.0\nprovides: [assay.hooks]\ninject:\n  required:\n    - key: assay.harness\n      flavour: claude\napply: []\n")
	return root
}
func writeBinding(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "components", "bindings.yaml"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// regression: #2701
func TestExplicitHarnessBinding(t *testing.T) {
	root := bindingFixture(t)
	if report, code := lint(root); code != exitProblems || !strings.Contains(report, "2 ACTIVE providers") {
		t.Fatalf("absent selector changed ambiguity: %d %s", code, report)
	}
	for _, name := range []string{"claude", "cursor"} {
		writeBinding(t, root, "schema: 1\nbindings:\n  assay.harness: assay/"+name+"\n")
		report, code := lintReport(root, true)
		if code != exitClean || !strings.Contains(report, "assay/"+name+": ACTIVE") {
			t.Fatalf("selection failed: %d %s", code, report)
		}
		want := "assay/hooks: ACTIVE"
		if name == "cursor" {
			want = "assay/hooks: INACTIVE"
		}
		if !strings.Contains(report, want) {
			t.Fatalf("selection did not propagate to dependencies: %s", report)
		}
	}
	if err := os.Remove(filepath.Join(root, "cursor.installed")); err != nil {
		t.Fatal(err)
	}
	if report, code := lint(root); code == exitClean || !strings.Contains(report, "selected provider assay/cursor is INACTIVE") {
		t.Fatalf("selection bypassed evidence: %d %s", code, report)
	}
}

// regression: #2701
func TestBindingRefusals(t *testing.T) {
	for _, body := range []string{"schema: 1\nbindings: {}\n", "schema: 2\nbindings: {assay.harness: assay/claude}\n", "schema: 1\nextra: ignored\nbindings: {assay.harness: assay/claude}\n", "schema: 1\nbindings: {assay.harness: assay/unknown}\n", "schema: 1\nbindings: {assay.hooks: assay/hooks}\n", "schema: 1\nbindings: {assay.harness: assay/hooks}\n", "schema: 1\nbindings: {assay.harness: assay/claude}\n---\nschema: 1\n", "schema: 1\nbindings:\n  assay.harness: assay/claude\n  assay.harness: assay/cursor\n"} {
		root := bindingFixture(t)
		writeBinding(t, root, body)
		if report, code := lint(root); code == exitClean {
			t.Fatalf("invalid binding admitted: %s\n%s", body, report)
		}
	}
}

// regression: #2701
func TestBindingNeedsActiveDependency(t *testing.T) {
	root := bindingFixture(t)
	writeManifest(t, root, "service", "component: assay/service\nversion: 1.0.0\nprovides:\n  - key: assay.service\n    evidence: missing-service\ninject:\n  required: []\napply: []\n")
	p := filepath.Join(root, "cursor", "component.yaml")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "required: []", "required:\n    - key: assay.service", 1))
	if err = os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "schema: 1\nbindings:\n  assay.harness: assay/cursor\n")
	report, code := lint(root)
	if code != exitProblems || !strings.Contains(report, "selected provider assay/cursor is INACTIVE") {
		t.Fatalf("selection bypassed dependency: %d %s", code, report)
	}
}
