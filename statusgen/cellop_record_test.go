package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// The explicitly authorized editorial repair must remain consumable by the
// normal completion planner without changing the historical verification facts.
// This uses a disposable offline stream, never the real board.
// regression: #3298
func TestCellopCompletionRecord(t *testing.T) {
	const name = "brief-15-cell-op-machine-seam.md"
	raw, err := os.ReadFile(filepath.Join("..", "docs", "streams", "desk-containers", name))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "desk-containers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	board := `---
stream: desk-containers
status: active
priority: P0
track: platform
---

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 15 | [Cell-operation contract](brief-15-cell-op-machine-seam.md) | 7 | M | implemented | — | — |
`
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte(board), 0o644); err != nil {
		t.Fatal(err)
	}
	// Only the existing parser/flip plan is under test. Human acceptance and
	// decision authority are separate gates; this makes no network call or write.
	at := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	brief := filepath.Join(dir, name)
	// Freeze each case at its own Evidence entry. A later verifier run must
	// neither erase an earlier refusal nor change the retained PASS fixture.
	const failEntry = "### Non-implementer execution witness — 2026-11-01"
	const blockedEntry = "### Independent bounded verification — 2026-11-20"
	body, review, ok := strings.Cut(string(raw), "\n## Review")
	if !ok {
		t.Fatal("Review section missing from the retained record")
	}
	review = "\n## Review" + review
	pass, fail, ok := strings.Cut(body, failEntry)
	if !ok {
		t.Fatal("November 1 FAIL entry missing from the retained record")
	}
	fail, _, _ = strings.Cut(fail, "\n### ")
	historical := pass + failEntry + strings.TrimRight(fail, "\n") + "\n"

	// In a checkout containing the November 20 Evidence, exercise that exact
	// entry. On an older checkout, model the same non-deferred held row.
	held := historical + blockedEntry + "\n| # | Command | Result |\n|---|---|---|\n| 12 | lint | could-not-check |\n\n**VERIFY: BLOCKED**\n"
	if prefix, entry, found := strings.Cut(body, blockedEntry); found {
		entry, _, _ = strings.Cut(entry, "\n### ")
		held = prefix + blockedEntry + entry
	}
	for _, tc := range []struct{ name, input, refusal string }{
		{"historical FAIL", historical + review, "VERIFY: FAIL, not PASS"},
		{"non-deferred held row", held + review, "still says HELD/could-not-check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(brief, []byte(tc.input), 0o644); err != nil {
				t.Fatal(err)
			}
			path, plan, id, err := closeVerifyFlipPlan(root, "desk-containers/15", at)
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("want refusal containing %q, got %v", tc.refusal, err)
			}
			if path != "" || plan != nil || id != "" {
				t.Fatal("refused closure returned a plan")
			}
			got, err := os.ReadFile(brief)
			if err != nil || string(got) != tc.input {
				t.Fatalf("refused closure changed the Evidence fixture (read error %v)", err)
			}
		})
	}
	record := pass + strings.TrimPrefix(review, "\n")
	if err := os.WriteFile(brief, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	_, planned, local, err := closeVerifyFlipPlan(root, "desk-containers/15", at)
	if err != nil {
		t.Fatalf("completion reader refused retained verifier record: %v", err)
	}
	// Preserve the original receipt/source binding and cite the original
	// publication as the source for a canonical republication, not a new run.
	for _, want := range []string{
		"Verification-Attestation: medici-finance/assay#3185 ",
		"source=00d2bb7f165194c0e4c9ef41e5db0cc6d018567d ",
		"https://github.com/medici-finance/assay/pull/3188",
		"https://github.com/medici-finance/assay/pull/4790#issuecomment-6482184018",
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("retained record lost original provenance: %s", want)
		}
	}
	const runner = "assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian)"
	const stamp = "2026-10-16 " + runner
	// This digest pins the primary historical table, not editorial prose or a
	// current execution. Undo only the permitted column split and principal
	// annotation, then compare every original command, expectation, observation,
	// date and runner byte to that immutable record.
	const header = "| # | Command | Expected | Observed (exit + key output) | Date | Runner |\n"
	start := strings.Index(string(raw), header)
	if start < 0 {
		t.Fatal("canonical Evidence table missing")
	}
	table := string(raw)[start:]
	end := strings.Index(table, "\n\n")
	if end < 0 {
		t.Fatal("canonical Evidence table boundary missing")
	}
	lines := strings.Split(table[:end], "\n")
	legacy := "| # | Command | Expected | Observed (exit + key output) | Date / Runner |\n" +
		"|---|---------|----------|------------------------------|---------------|\n"
	for _, line := range lines[2:] {
		cells := briefdoc.SplitRowEscaped(line)
		if len(cells) != 6 {
			t.Fatalf("canonical Evidence row has %d cells: %s", len(cells), line)
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if cells[4] != "2026-10-16" || cells[5] != runner {
			t.Fatalf("historical date/runner changed in row %s", cells[0])
		}
		legacy += "| " + strings.Join(cells[:4], " | ") + " | " + cells[4] + " " +
			strings.TrimSuffix(cells[5], " (on-behalf-of human:ian)") + " |\n"
	}
	const originalTableSHA256 = "1e5140ae5e537970acffb306af6b44541b2139f3775e5fc8804793fa807c0a5e"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(legacy))); got != originalTableSHA256 {
		t.Fatalf("historical Evidence data changed: sha256=%s", got)
	}
	if got := strings.Count(string(raw), "| 2026-10-16 | "+runner+" |"); got != 14 {
		t.Fatalf("attribution columns aligned for %d rows, want all 14 retained rows", got)
	}
	if local != "desk-containers/15" || !strings.Contains(string(planned), "| done | "+stamp+" |") {
		t.Fatalf("completion did not preserve the original verifier stamp: %s", planned)
	}
	after, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != board {
		t.Fatal("planning changed the fixture board")
	}
	retained, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(retained) != record {
		t.Fatal("planning changed retained evidence")
	}
}
