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

// A double-quoted string so the backtick tag can be embedded literally; the em dashes are UTF-8.
var backtickTitleTable = "\n## Briefs\n\n" +
	"| # | Brief | Wave | Effort | Status | Verified | Reviewed | Notes |\n" +
	"|---|-------|------|--------|--------|----------|----------|-------|\n" +
	"| 10a | [`[assay]` askassay importable — pin the contract and guard it](./brief-10a-askassay-public-pin-contract.md) | 0 | M | todo | — | — | |\n" +
	"| 11 | [[draft] scoping note before the split](./brief-11-draft.md) | 1 | S | todo | — | — | |\n" +
	"| 12 | [Plain unlinked-strip control](./brief-12-plain.md) | 1 | S | todo | — | — | |\n"

// TestParseBriefTableUnwrapsBacktickBracketTitle pins #591: a title whose link TEXT begins with a
// bracket — a backticked tag “ `[assay]` “ or a bare `[draft]` — must strip to a BARE title like
// every other Next-up row, never keeping the README-relative `./brief-…` link (dead from
// STATUS.md). The old `\[([^\]]+)\]\(…\)` regexp stopped at the inner `]` and left the whole link,
// so these cases fail against the pre-fix code. The plain-title row is the control the fix must
// not regress.
func TestParseBriefTableUnwrapsBacktickBracketTitle(t *testing.T) {
	briefs, err := parseBriefTable(backtickTitleTable)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"10a": "`[assay]` askassay importable — pin the contract and guard it",
		"11":  "[draft] scoping note before the split",
		"12":  "Plain unlinked-strip control",
	}
	seen := 0
	for _, b := range briefs {
		w, ok := want[b.Num]
		if !ok {
			t.Fatalf("unexpected brief %q", b.Num)
		}
		seen++
		if b.Title != w {
			t.Errorf("brief %s: title = %q, want %q (link must be stripped)", b.Num, b.Title, w)
		}
		if strings.Contains(b.Title, "](") || strings.Contains(b.Title, "./brief-") {
			t.Errorf("brief %s: title still carries a link target: %q", b.Num, b.Title)
		}
	}
	if seen != len(want) {
		t.Fatalf("parsed %d briefs, want %d", seen, len(want))
	}
}

func TestParseBriefTableMissingColumn(t *testing.T) {
	bad := "| # | Brief | Wave | Status |\n|---|---|---|---|\n| 01 | X | 0 | todo |\n"
	if _, err := parseBriefTable(bad); err == nil {
		t.Error("want error for missing Verified/Reviewed columns")
	}
}

// TestParseBriefTableDecoratedStatusShiftsColumns pins the recurring defect
// from #82: a worker records the PR association IN the Status cell
// (`implemented (#80)`) plus a stray `||`, prepending a cell and shifting every
// column right. splitRow then yields more cells than the header. The parser must
// REJECT the row (a count mismatch in either direction is malformed) with a
// message that names the fix — bare token in the cell, PR link in the trailer —
// rather than silently proceeding and misreporting a downstream `invalid status`.
func TestParseBriefTableDecoratedStatusShiftsColumns(t *testing.T) {
	bad := "## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| implemented (#80) || 02 | [brief](./brief-02.md) | 0 | M | implemented | — | — |\n"
	_, err := parseBriefTable(bad)
	if err == nil {
		t.Fatal("want error for a column-shifted row (decorated Status cell), got nil")
	}
	msg := err.Error()
	for _, want := range []string{"Status cell", "bare", "Brief:", "trailer"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message does not name the fix (missing %q): %s", want, msg)
		}
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
