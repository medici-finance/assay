package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleReadme = `---
stream: operator
status: active
priority: P0
track: platform
issues: [49, 51]
---

# Operator Stream
body text
`

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseStreamFrontmatter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "operator")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTemp(t, dir, "README.md", sampleReadme)
	s, err := parseStreamREADME(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "operator" || s.Status != "active" || s.Priority != "P0" || s.Track != "platform" {
		t.Errorf("bad stream: %+v", s)
	}
	if len(s.Issues) != 2 || s.Issues[0] != 49 {
		t.Errorf("bad issues: %v", s.Issues)
	}
	if s.Dir != dir {
		t.Errorf("Dir = %q, want %q", s.Dir, dir)
	}
}

func TestSplitFrontmatterErrors(t *testing.T) {
	if _, _, err := splitFrontmatter("# no frontmatter\n"); err == nil {
		t.Error("want error for missing frontmatter")
	}
	if _, _, err := splitFrontmatter("---\nstream: x\n"); err == nil {
		t.Error("want error for unterminated frontmatter")
	}
}

// TestSplitFrontmatterCRLF is the core regression test for CRLF handling:
// a CRLF-terminated file must split into the same frontmatter/body pair as
// its LF equivalent, not lose its body. splitFrontmatter is now the single
// canonical parser (stream READMEs, brief files, and per-entry register
// files all funnel through it), so proving this once here covers every
// caller.
func TestSplitFrontmatterCRLF(t *testing.T) {
	crlf := "---\r\nstream: x\r\nstatus: active\r\n---\r\n\r\nbody line\r\n"
	fm, body, err := splitFrontmatter(crlf)
	if err != nil {
		t.Fatalf("unexpected error splitting CRLF content: %v", err)
	}
	if !strings.Contains(fm, "stream: x") || !strings.Contains(fm, "status: active") {
		t.Errorf("frontmatter lost content: %q", fm)
	}
	if !strings.Contains(body, "body line") {
		t.Errorf("body was lost (CRLF regression): %q", body)
	}
}

const sampleTable = `
## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed | Notes |
|---|-------|------|--------|--------|----------|----------|-------|
| 01 | [Test infra](./brief-01-test.md) | 0 | M | done | grandfathered | grandfathered | shipped |
| 02 | [WS streaming](./brief-02-ws.md) | 1 | M | todo | — | — | |
| 12a | [Research](./brief-12a.md) | 0 | S | in-progress | — | — | |
`

func TestParseBriefTable(t *testing.T) {
	briefs, err := parseBriefTable(sampleTable)
	if err != nil {
		t.Fatal(err)
	}
	if len(briefs) != 3 {
		t.Fatalf("got %d briefs, want 3", len(briefs))
	}
	b := briefs[0]
	if b.Num != "01" || b.Title != "Test infra" || b.Wave != 0 || b.Effort != "M" ||
		b.Status != "done" || b.Verified != "grandfathered" || b.Reviewed != "grandfathered" {
		t.Errorf("brief 0 wrong: %+v", b)
	}
	if briefs[1].Verified != "" || briefs[1].Reviewed != "" {
		t.Errorf("em-dash should normalize to empty: %+v", briefs[1])
	}
	if briefs[2].Num != "12a" {
		t.Errorf("alphanumeric num lost: %+v", briefs[2])
	}
}

func TestParseBriefTableMissingColumn(t *testing.T) {
	bad := "| # | Brief | Wave | Status |\n|---|---|---|---|\n| 01 | X | 0 | todo |\n"
	if _, err := parseBriefTable(bad); err == nil {
		t.Error("want error for missing Verified/Reviewed columns")
	}
}

func TestParseNoTableIsValid(t *testing.T) {
	briefs, err := parseBriefTable("# Just prose\nno table here\n")
	if err != nil || briefs != nil {
		t.Errorf("no table should be nil, nil; got %v, %v", briefs, err)
	}
}

func TestParseMaxConcurrent(t *testing.T) {
	const fmWithMax = `---
stream: serial
status: active
priority: P0
max-concurrent: 1
---

# Serial Stream
body
`
	dir := filepath.Join(t.TempDir(), "serial")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTemp(t, dir, "README.md", fmWithMax)
	s, err := parseStreamREADME(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.MaxConcurrent == nil {
		t.Fatal("MaxConcurrent should be non-nil when max-concurrent: 1 is present")
	}
	if *s.MaxConcurrent != 1 {
		t.Errorf("MaxConcurrent = %d, want 1", *s.MaxConcurrent)
	}

	// Absent field → nil (existing test already covers via sampleReadme).
	s2, err := parseStreamREADME(writeTemp(t, dir, "README2.md", sampleReadme))
	if err != nil {
		t.Fatal(err)
	}
	if s2.MaxConcurrent != nil {
		t.Errorf("MaxConcurrent should be nil when absent, got %v", *s2.MaxConcurrent)
	}
}
