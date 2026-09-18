package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// forgebackedoffline_test.go — forge-neutral/18 row 5 (TestForgeBackedChecksReportCouldNotCheckOffline).
//
// With the offline reader wired (forgeReaderForRun's DEFAULT, newOfflineReader()), every
// forge-backed check must render could-not-check AS ITSELF — never a clean/zero claim manufactured
// from data that was never read. The test fails if any of them renders clean, and it enumerates
// the checks — via forgeReaderForRunConsumerCount, a source-level count of the ONE call shape a
// forgeReaderForRun consumer takes (`..., forgeReaderForRun)`) — so a newly migrated Task 6 call
// site that forgets to add its row here is caught by that count drifting, rather than silently
// passing an enumeration that no longer matches the tree.
func TestForgeBackedChecksReportCouldNotCheckOffline(t *testing.T) {
	const wantConsumers = 1 // openIssueDebtNotice, main.go:411 — the ONLY forgeReaderForRun consumer today.
	if got := forgeReaderForRunConsumerCount(t); got != wantConsumers {
		t.Fatalf("forgeReaderForRun has %d consumer call site(s) in statusgen/*.go (non-test), this test's "+
			"table covers %d — a Task 6 migration added (or removed) a forge-backed check without this row "+
			"table being updated to match. Add its offline-behaviour assertion here before landing that change.",
			got, wantConsumers)
	}

	reader := newOfflineReader()

	t.Run("openIssueDebtNotice: absent, never a zero-debt claim", func(t *testing.T) {
		got := openIssueDebtNotice(7, reader)
		if got != "" {
			t.Fatalf("openIssueDebtNotice with the offline reader = %q, want \"\" — an absent line, "+
				"never a rendered could-not-check STRING and never a fabricated debt count", got)
		}
		// The stronger property this row exists for: the line must never claim ZERO debt. Absence
		// is sanctioned here ONLY because this is an opt-in advisory NOTICE with no separate
		// "0 debt, clean" rendering to confuse it with (row 6 pins the opt-in behaviour itself) —
		// grep-proving that no such string exists for THIS call shape.
		if strings.Contains(got, "0 open") {
			t.Fatalf("openIssueDebtNotice must never render a zero-debt claim when it did not look: %q", got)
		}
	})
}

// forgeReaderForRunConsumerCount counts non-test source lines in statusgen/ that pass
// forgeReaderForRun as a call argument (the `..., forgeReaderForRun)` shape every consumer uses,
// per main.go:411's own call). It is the row's enumeration guard: a Task 6 migration wiring a
// NEW forge-backed check through forgeReaderForRun changes this count, which fails the test above
// until a matching subtest is added.
func forgeReaderForRunConsumerCount(t *testing.T) int {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		count += strings.Count(string(b), ", forgeReaderForRun)")
	}
	return count
}

// TestForgeReaderForRunConsumerCountMatchesGrep is a +dereference sanity check on the instrument
// above: it must agree with an independent, argv-only (no shell interpolation) grep invocation,
// so a bug in the Go-side counter cannot itself hide a drifted enumeration.
func TestForgeReaderForRunConsumerCountMatchesGrep(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	goFiles, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	var nonTest []string
	for _, f := range goFiles {
		if !strings.HasSuffix(f, "_test.go") {
			nonTest = append(nonTest, f)
		}
	}
	if _, err := exec.LookPath("grep"); err != nil || len(nonTest) == 0 {
		t.Skip("grep not on PATH, or no non-test .go files found")
	}
	args := append([]string{"-h", "-o", ", forgeReaderForRun)"}, nonTest...)
	out, _ := exec.Command("grep", args...).Output() // grep exits 1 on no match — not an error here
	want := len(strings.Split(strings.TrimSpace(string(out)), "\n"))
	if strings.TrimSpace(string(out)) == "" {
		want = 0
	}
	if got := forgeReaderForRunConsumerCount(t); got != want {
		t.Fatalf("Go-side count = %d, grep count = %d — the enumeration instrument disagrees with the source", got, want)
	}
}
