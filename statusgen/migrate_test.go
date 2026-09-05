package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// migrateFixtureTree writes a minimal brief-v1 tree (one stream, two briefs, a
// README with a Briefs table, and a graph-repos.yaml with a self alias) under a
// temp dir and returns its root.
func migrateFixtureTree(t *testing.T, withRegistry bool) string {
	t.Helper()
	root := t.TempDir()
	streams := filepath.Join(root, "docs", "streams")
	svc := filepath.Join(streams, "svc")
	if err := os.MkdirAll(svc, 0o755); err != nil {
		t.Fatal(err)
	}
	if withRegistry {
		reg := "schema: graph-repos-v1\ncell: example\nself: app\nrepos:\n  app: {cell: example, repo: example-org/app}\n"
		if err := os.WriteFile(filepath.Join(streams, "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	brief := "---\nbrief: svc/01\ntitle: A brief\nwave: 0\ndepends: []\nunblocks: []\neffort: M\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\nauthored: 2026-07-09 by test\nsources: [\"note\"]\n---\n\n# Brief 01\n\n## Context\nfiles: x\n"
	if err := os.WriteFile(filepath.Join(svc, "brief-01-a.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := "---\nstream: svc\nstatus: active\npriority: P1\ntrack: product\n---\n\n# Svc\n\n## Briefs\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n| 01 | [A brief](./brief-01-a.md) | 0 | M | todo | — | — |\n"
	if err := os.WriteFile(filepath.Join(svc, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMigrate_RewritesBriefAndReadme(t *testing.T) {
	root := migrateFixtureTree(t, true)
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("migrate exit=%d, want 0; stderr=%s", code, errb.String())
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "schema: brief-v2") {
		t.Errorf("schema not rewritten to brief-v2:\n%s", s)
	}
	if strings.Contains(s, "schema: brief-v1") {
		t.Errorf("brief-v1 schema still present:\n%s", s)
	}
	if !regexp.MustCompile(`(?m)^brief: example:app:svc:01$`).MatchString(s) {
		t.Errorf("brief id not rewritten to hierarchical form:\n%s", s)
	}
	if !regexp.MustCompile(`(?m)^version: 1$`).MatchString(s) {
		t.Errorf("version: 1 not added:\n%s", s)
	}
	if !regexp.MustCompile(`(?m)^id: [0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(s) {
		t.Errorf("id: uuid v4 not minted:\n%s", s)
	}
	rd, err := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	rs := string(rd)
	if !strings.Contains(rs, "board: generated") {
		t.Errorf("board: generated not added to README:\n%s", rs)
	}
	if !strings.Contains(rs, briefsMarkerBegin) || !strings.Contains(rs, briefsMarkerEnd) {
		t.Errorf("Briefs table not wrapped in markers:\n%s", rs)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	root := migrateFixtureTree(t, true)
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("first migrate exit=%d; stderr=%s", code, errb.String())
	}
	first, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	firstReadme, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "README.md"))

	out.Reset()
	errb.Reset()
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("second migrate exit=%d; stderr=%s", code, errb.String())
	}
	second, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	secondReadme, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "README.md"))
	if !bytes.Equal(first, second) {
		t.Errorf("brief not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !bytes.Equal(firstReadme, secondReadme) {
		t.Errorf("README not idempotent:\nfirst:\n%s\nsecond:\n%s", firstReadme, secondReadme)
	}
	if !strings.Contains(out.String(), "clean no-op") {
		t.Errorf("second run did not report a clean no-op:\n%s", out.String())
	}
}

func TestMigrate_RefusesWithoutRegistry(t *testing.T) {
	root := migrateFixtureTree(t, false) // no graph-repos.yaml
	var out, errb bytes.Buffer
	code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb)
	if code != migrateExitRefused {
		t.Fatalf("exit=%d, want %d (refused)", code, migrateExitRefused)
	}
	if !strings.Contains(errb.String(), "graph-repos.yaml") {
		t.Errorf("stderr does not name graph-repos.yaml: %s", errb.String())
	}
	// Refusal must leave the brief untouched (still v1).
	got, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	if !strings.Contains(string(got), "schema: brief-v1") {
		t.Errorf("refused run must not mutate the brief; got:\n%s", got)
	}
}

func TestMigrate_DryRunWritesNothing(t *testing.T) {
	root := migrateFixtureTree(t, true)
	before, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	beforeReadme, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "README.md"))
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root, "--dry-run"}, &out, &errb); code != 0 {
		t.Fatalf("dry-run exit=%d; stderr=%s", code, errb.String())
	}
	after, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	afterReadme, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "README.md"))
	if !bytes.Equal(before, after) || !bytes.Equal(beforeReadme, afterReadme) {
		t.Errorf("dry-run wrote to the tree")
	}
	if !strings.Contains(out.String(), "WOULD") {
		t.Errorf("dry-run did not print a WOULD plan:\n%s", out.String())
	}
}

func TestMigrate_UnknownTargetIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"brief-v2-to-v3"}, &out, &errb); code != migrateExitUsage {
		t.Fatalf("exit=%d, want %d", code, migrateExitUsage)
	}
}
