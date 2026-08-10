package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRegisterMap(t *testing.T) {
	root := t.TempDir()

	// Create findings entry.
	fDir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(fDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fDir, "2026-07-09-test-finding.md", `---
id: F-15
date: "2026-07-09"
title: Test finding
affects: []
resolved: false
---
Body text.
`)

	// Create intake entry.
	iDir := filepath.Join(root, "docs", "streams", "intake")
	if err := os.MkdirAll(iDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, iDir, "2026-07-10-test-intake.md", `---
id: I-23
date: "2026-07-10"
title: Test intake
disposition: new
---
Body text.
`)

	fMap, iMap := buildRegisterMap(root)

	if fMap["F-15"] == "" {
		t.Error("F-15 not found in findings map")
	}
	if iMap["I-23"] == "" {
		t.Error("I-23 not found in intake map")
	}
}

func TestRegisterRefProblems_ValidLink(t *testing.T) {
	root := t.TempDir()

	// Create a finding entry file.
	fDir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(fDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fDir, "2026-07-09-test-finding.md", `---
id: F-15
date: "2026-07-09"
title: Test finding
affects: []
resolved: false
---
Body.
`)

	// Create a brief that links to it correctly.
	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	briefContent := "See [F-15](../findings/2026-07-09-test-finding.md) for details.\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", briefContent)

	problems, notices := registerRefProblems(root, []string{briefPath})
	if len(problems) > 0 {
		t.Errorf("expected no problems for valid link, got: %v", problems)
	}
	if len(notices) > 0 {
		t.Errorf("expected no notices for valid link, got: %v", notices)
	}
}

func TestRegisterRefProblems_DeadLink(t *testing.T) {
	root := t.TempDir()

	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "See [F-99](../findings/nonexistent.md).\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", content)

	problems, _ := registerRefProblems(root, []string{briefPath})
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem for dead link, got %d: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "does not exist") {
		t.Errorf("problem should mention 'does not exist', got: %s", problems[0])
	}
}

func TestRegisterRefProblems_WrongID(t *testing.T) {
	root := t.TempDir()

	// Create finding entry with id: F-15.
	fDir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(fDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fDir, "2026-07-09-test.md", `---
id: F-15
date: "2026-07-09"
title: Test
affects: []
resolved: false
---
Body.
`)

	// Brief links with text [F-16] but targets F-15's file.
	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "See [F-16](../findings/2026-07-09-test.md).\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", content)

	problems, _ := registerRefProblems(root, []string{briefPath})
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem for wrong ID, got %d: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "id: is") {
		t.Errorf("problem should mention id mismatch, got: %s", problems[0])
	}
}

func TestRegisterRefProblems_ViewLinkNotice(t *testing.T) {
	root := t.TempDir()

	// Create FINDINGS.md so the link resolves (file exists check passes).
	fDir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(fDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fDir, "FINDINGS.md", "generated view")

	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "See [F-15](../findings/FINDINGS.md).\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", content)

	problems, notices := registerRefProblems(root, []string{briefPath})
	if len(problems) > 0 {
		t.Errorf("view links should be NOTICE not PROBLEM, got problems: %v", problems)
	}
	if len(notices) != 1 {
		t.Fatalf("expected 1 notice for view link, got %d: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "generated view") {
		t.Errorf("notice should mention 'generated view', got: %s", notices[0])
	}
}

func TestRegisterRefProblems_BareRefIsSilent(t *testing.T) {
	root := t.TempDir()

	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Bare F-15 without a link — should be silent.
	content := "See F-15 for details.\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", content)

	problems, notices := registerRefProblems(root, []string{briefPath})
	if len(problems) > 0 {
		t.Errorf("bare ref should be silent, got problems: %v", problems)
	}
	if len(notices) > 0 {
		t.Errorf("bare ref should be silent, got notices: %v", notices)
	}
}

func TestRegisterRefProblems_FencedLinkIgnored(t *testing.T) {
	root := t.TempDir()

	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Link inside a code fence should be ignored.
	content := "```\n[F-99](../findings/nonexistent.md)\n```\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", content)

	problems, _ := registerRefProblems(root, []string{briefPath})
	if len(problems) > 0 {
		t.Errorf("fenced links should be ignored, got: %v", problems)
	}
}

func TestRegisterRefProblems_PathTraversal(t *testing.T) {
	root := t.TempDir()

	briefDir := filepath.Join(root, "docs", "streams", "teststream")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Link that traverses outside docs/streams/ should be a PROBLEM.
	content := "See [F-15](../../../etc/passwd).\n"
	briefPath := writeTemp(t, briefDir, "brief-01-test.md", content)

	problems, _ := registerRefProblems(root, []string{briefPath})
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem for path traversal, got %d: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "path traversal") {
		t.Errorf("problem should mention path traversal, got: %s", problems[0])
	}
}

func TestRegisterRefProblems_ExternalURLSkipped(t *testing.T) {
	root := t.TempDir()

	docDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// An F-NN link whose target is an external URL is a prose citation, not a
	// repo-relative register reference — it must not be flagged (assay-toolkit#59).
	content := "The honest limit, per [F-08](https://github.com/example-org/tracker).\n"
	docPath := writeTemp(t, docDir, "desk-console-design.md", content)

	problems, notices := registerRefProblems(root, []string{docPath})
	if len(problems) > 0 {
		t.Errorf("external-URL register citation should be silent, got problems: %v", problems)
	}
	if len(notices) > 0 {
		t.Errorf("external-URL register citation should be silent, got notices: %v", notices)
	}
}

func TestReplaceBareRefs(t *testing.T) {
	// Setup: brief at docs/streams/methodology/brief-01.md
	// Entry at docs/streams/findings/2026-07-09-foo.md (id: F-15)
	briefPath := filepath.Join("docs", "streams", "methodology", "brief-01.md")

	fMap := map[string]string{
		"F-15": filepath.Join("docs", "streams", "findings", "2026-07-09-foo.md"),
	}
	iMap := map[string]string{
		"I-23": filepath.Join("docs", "streams", "intake", "2026-07-10-bar.md"),
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantCnt int
	}{
		{
			name:    "bare F ref",
			input:   "See F-15 for details.",
			want:    "See [F-15](../findings/2026-07-09-foo.md) for details.",
			wantCnt: 1,
		},
		{
			name:    "bare I ref",
			input:   "From I-23.",
			want:    "From [I-23](../intake/2026-07-10-bar.md).",
			wantCnt: 1,
		},
		{
			name:    "already linked",
			input:   "See [F-15](some-path.md).",
			want:    "See [F-15](some-path.md).",
			wantCnt: 0,
		},
		{
			name:    "inline code",
			input:   "Run `F-15` as-is.",
			want:    "Run `F-15` as-is.",
			wantCnt: 0,
		},
		{
			name:    "multiple refs",
			input:   "See F-15 and I-23 for context.",
			want:    "See [F-15](../findings/2026-07-09-foo.md) and [I-23](../intake/2026-07-10-bar.md) for context.",
			wantCnt: 2,
		},
		{
			name:    "unknown ID",
			input:   "Reference F-99 is unknown.",
			want:    "Reference F-99 is unknown.",
			wantCnt: 0,
		},
		{
			name:    "sources YAML",
			input:   "  - \"F-15 (finding)\"",
			want:    "  - \"[F-15](../findings/2026-07-09-foo.md) (finding)\"",
			wantCnt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cnt := replaceBareRefs(tt.input, fMap, iMap, briefPath)
			if cnt != tt.wantCnt {
				t.Errorf("count = %d, want %d", cnt, tt.wantCnt)
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestStripFences(t *testing.T) {
	input := "before\n```\nfenced [F-15](link)\n```\nafter\n"
	got := stripFences(input)
	if strings.Contains(got, "[F-15]") {
		t.Error("fenced content should be stripped")
	}
	if !strings.Contains(got, "before") {
		t.Error("non-fenced content should be kept")
	}
	if !strings.Contains(got, "after") {
		t.Error("non-fenced content after fence should be kept")
	}
}
