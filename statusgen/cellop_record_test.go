package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
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
	_, planned, local, err := closeVerifyFlipPlan(root, "desk-containers/15", time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC))
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
		cells := splitRowEscaped(line)
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
	if string(retained) != string(raw) {
		t.Fatal("planning changed retained evidence")
	}
}
