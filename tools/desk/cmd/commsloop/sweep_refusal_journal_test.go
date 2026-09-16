package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// sweep_refusal_journal_test.go — the FAIL-FIRST drill for the sweep half
// of #1165: the gateway's refusal lines (`kind:"refused"`, exactly the
// shape cmd/commsgw's refusal.go writes) are planted in journal.log, and
// the sweep must COUNT them per sender and per lane pair and report a
// threshold breach as a finding.
//
// DELIBERATELY written against the symbols the merge-base already had
// (Sweep, SweepDeps{Trust}, writeJournalLines, the three-state constants)
// with the line shape as raw JSON and the finding kind as a string literal,
// so that against the unfixed sweep it compiles and reports could-not-check
// ("unrecognised kind") instead of checked-failed — a clean red, not a
// compile error. The finer contracts (per-lane axis, the disabled and
// custom thresholds, the unrecognised-refusal-kind corruption rule, the
// gateway-cell scoping, the writer/reader shape agreement) live in
// sweep_refusal_test.go and use the new symbols.

// refusedLine renders one gateway refusal line by hand, field for field
// the shape commsqueue.RefusalRecord marshals to.
func refusedLine(at time.Time, id, cell, refusal string, from comms.SenderID, to comms.Lane, verb string) string {
	return fmt.Sprintf(`{"time":%q,"kind":"refused","id":%q,"cell":%q,"refusal":%q,"from":{"cell":%q,"role":%q},"to":{"cell":%q,"role":%q},"verb":%q,"rawDigest":"deadbeef"}`,
		at.Format(time.RFC3339Nano), id, cell, refusal, from.Cell, from.Role, to.Cell, to.Role, verb)
}

// TestSweepRefusalThresholdBreach plants ten refusals (the documented
// default threshold) from ONE presented sender, on ONE lane pair, and
// expects checked-failed with a refusal-threshold finding naming that
// sender — and, one line short of the threshold, checked-clean with zero
// findings (the positive control that keeps the threshold a threshold).
func TestSweepRefusalThresholdBreach(t *testing.T) {
	const threshold = 10 // DefaultRefusalThreshold, spelled as a literal on purpose (see file doc)
	from := comms.SenderID{Cell: "cell-a", Role: "the-desk"}
	to := comms.Lane{Cell: "cell-a", Role: "worker-desk"}
	kinds := []string{"lane-denied", "bad-signature", "replay", "expired", "cross-cell-pair"}

	plant := func(n int) []string {
		lines := make([]string, 0, n)
		for i := 0; i < n; i++ {
			lines = append(lines, refusedLine(sweepTestNow.Add(time.Duration(i)*time.Second),
				fmt.Sprintf("probe-%d", i), "cell-a", kinds[i%len(kinds)], from, to, "handoff"))
		}
		return lines
	}

	t.Run("at threshold: breach", func(t *testing.T) {
		root := t.TempDir()
		writeJournalLines(t, root, plant(threshold)...)

		report := Sweep(root, "cell-a", time.Time{}, SweepDeps{})

		if report.State != SweepCheckedFailed {
			t.Fatalf("state = %s, want %s — %d refusals from one sender must breach the default threshold (findings=%v couldNotCheck=%v)",
				report.State, SweepCheckedFailed, threshold, report.Findings, report.CouldNotCheckReasons)
		}
		var senderHit bool
		for _, f := range report.Findings {
			if string(f.Kind) != "refusal-threshold" {
				t.Fatalf("unexpected finding kind %q: %+v", f.Kind, f)
			}
			if strings.HasPrefix(f.ID, "sender:") {
				senderHit = true
				if !strings.Contains(f.ID, "cell-a/the-desk") {
					t.Fatalf("sender finding must name the presented sender, got %+v", f)
				}
				if !strings.Contains(f.Detail, fmt.Sprintf("%d gateway refusal(s)", threshold)) || !strings.Contains(f.Detail, "lane-denied=2") {
					t.Fatalf("finding detail must carry the count and the per-kind breakdown, got %q", f.Detail)
				}
			}
		}
		if !senderHit {
			t.Fatalf("no per-sender refusal-threshold finding in %v", report.Findings)
		}
		if len(report.CouldNotCheckReasons) != 0 {
			t.Fatalf("refusal lines are a recognised record kind, got could-not-check: %v", report.CouldNotCheckReasons)
		}
	})

	t.Run("below threshold: clean", func(t *testing.T) {
		root := t.TempDir()
		writeJournalLines(t, root, plant(threshold-1)...)

		report := Sweep(root, "cell-a", time.Time{}, SweepDeps{})

		if report.State != SweepCheckedClean {
			t.Fatalf("state = %s, want %s — %d refusals is under the threshold (findings=%v couldNotCheck=%v)",
				report.State, SweepCheckedClean, threshold-1, report.Findings, report.CouldNotCheckReasons)
		}
		if report.Checked != threshold-1 {
			t.Fatalf("every refusal line is counted as checked, Checked=%d want %d", report.Checked, threshold-1)
		}
	})
}
