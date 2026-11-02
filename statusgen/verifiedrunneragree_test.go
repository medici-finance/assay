package main

import (
	"os"
	"strings"
	"testing"
)

// rerunNotices copies the shared re-run-attribution fixture tree into a temp
// root, loads the streams, and returns the Verified-cell/Evidence-runner
// disagreement notices.
func rerunNotices(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/rerunattr")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return verifiedRunnerDisagreementNotices(streams)
}

// TestRerunStaleCellIsFlagged is the fail-first case for the whole issue: a brief
// whose Verify rows were RE-RUN by a different actor (the shepherd, after a fix)
// while the Verified cell still names the ORIGINAL verifier must be flagged —
// the cell and the Evidence table disagree about who verified.
func TestRerunStaleCellIsFlagged(t *testing.T) {
	notices := rerunNotices(t)
	if !hasProblem(notices, "rerun/brief-01", "k3-verifier", "opus-verifier", "2 of 3") {
		t.Errorf("a stale Verified cell (names k3-verifier; Evidence majority is opus-verifier on 2 of 3 rows) must be flagged; got:\n%s", strings.Join(notices, "\n"))
	}
}

// TestRerunUpdatedCellIsClean is the other half of the fix: once the Verified
// cell is updated to name the actor who actually (re-)ran the majority of the
// rows, the cell MATCHES the Evidence and no disagreement is reported.
func TestRerunUpdatedCellIsClean(t *testing.T) {
	notices := rerunNotices(t)
	if hasProblem(notices, "rerun/brief-02") {
		t.Errorf("a Verified cell updated to the re-run actor (opus-verifier) matches the Evidence and must not be flagged; got:\n%s", strings.Join(notices, "\n"))
	}
}

// TestRerunNoMajorityIsSilent pins the strict-majority guard: an even split with
// no single dominant runner has no actor for the cell to disagree with, so the
// check stays silent rather than making a judgement call.
func TestRerunNoMajorityIsSilent(t *testing.T) {
	notices := rerunNotices(t)
	if hasProblem(notices, "rerun/brief-03") {
		t.Errorf("a brief with no majority Evidence runner must not be flagged; got:\n%s", strings.Join(notices, "\n"))
	}
}

// TestRerunImplementedExempt pins the status scope: an `implemented` row is out
// of scope even with the same disagreement shape.
func TestRerunImplementedExempt(t *testing.T) {
	notices := rerunNotices(t)
	if hasProblem(notices, "rerun/brief-04") {
		t.Errorf("an implemented-status brief is out of scope for this check; got:\n%s", strings.Join(notices, "\n"))
	}
}

// --- unit-level tests of the helpers ---

func TestRunnerKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"opus-verifier", "opus-verifier"},
		{"Opus-Verifier", "opus-verifier"},
		{"opus-verifier (non-implementer)", "opus-verifier"},
		{"assay-worker-app[bot]", "assay-worker-app"},
		{"  k3-verifier  ", "k3-verifier"},
		{"", ""},
	}
	for _, c := range cases {
		if got := runnerKey(c.in); got != c.want {
			t.Errorf("runnerKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunnersAgree(t *testing.T) {
	cases := []struct {
		cell, evidence string
		want           bool
		why            string
	}{
		{"opus-verifier", "opus-verifier", true, "identical tokens agree"},
		{"sonnet-verifier", "sonnet (non-implementer)", true, "the cell token contains the Evidence key — same actor, decorated"},
		{"opus-verifier", "opus-verifier[bot]", true, "a [bot] suffix on the Evidence side does not split the actor"},
		{"k3-verifier", "assay-worker-app", false, "two different actors do not agree"},
		{"k3-verifier", "opus-verifier", false, "two distinct verifier tokens do not agree"},
		{"", "opus-verifier", true, "an empty side never manufactures a disagreement"},
		{"assay-verifier-app[bot]", "model-verifier (assay-verifier-app[bot]) @ abc123", true, "an explicitly named forge actor is an alias, not a different executor"},
		{"assay-verifier-app[bot]", "model-verifier (assay-worker-app[bot]) @ abc123", false, "different explicit forge actors must still disagree"},
		{"assay-verifier-app[bot]", "model-verifier (assay-verifier-app[bot]-copy)", false, "an alias-shaped prefix of a different actor is not an alias"},
		{"assay-verifier-app[bot]", "assay-worker-app[bot] (published by assay-verifier-app[bot])", false, "two distinct declared actors cannot establish an alias"},
		// regression: S1 attribution-alias-overmatch — a leading actor written
		// without [bot] is still an actor, and a later mention cannot mask it.
		{"assay-verifier-app[bot]", "assay-worker-app (for assay-verifier-app[bot])", false, "a bare leading login is a distinct actor, not a model"},
		{"assay-verifier-app[bot]", "assay-worker-app re-ran; see assay-verifier-app[bot]", false, "a later mention cannot override the leading actor"},
		{"assay-worker-app", "assay-verifier-app (assay-worker-app[bot])", false, "the mirror direction disagrees too"},
		{"assay-verifier-app[bot]", "assay-verifier-app (on-behalf-of human:x)", true, "the same bare login still agrees"},
		{"assay-verifier-app[bot]", "model-verifier (x/assay-verifier-app[bot])", false, "a connector before the alias is not a declared alias"},
	}
	for _, c := range cases {
		if got := runnersAgree(c.cell, c.evidence); got != c.want {
			t.Errorf("runnersAgree(%q, %q) = %v, want %v — %s", c.cell, c.evidence, got, c.want, c.why)
		}
	}
}

// regression: #4132 a same-day conflict is not chronology.
func TestEvidenceRunnerSameDayIsNotChronology(t *testing.T) {
	rows := "| # | Date | Runner |\n|---|---|---|\n" +
		"| 1 | 2026-09-25 | implementer |\n\n" +
		"| # | Date | Runner |\n|---|---|---|\n" +
		"| 1 | 2026-09-25 | independent-verifier |\n"
	if raw, n, total, ok := evidenceRunnerTally(rows); ok || raw != "" || n != 0 || total != 1 {
		t.Fatalf("same-date conflicting actors cannot establish which ran last: %q %d/%d ok=%v", raw, n, total, ok)
	}
	// A dated later run resolves the ambiguity; no row is permanently excluded.
	rows += "\n| # | Date | Runner |\n|---|---|---|\n| 1 | 2026-09-26 | independent-verifier |\n"
	if raw, n, total, ok := evidenceRunnerTally(rows); !ok || raw != "independent-verifier" || n != 1 || total != 1 {
		t.Fatalf("later dated run must resolve the actor: %q %d/%d ok=%v", raw, n, total, ok)
	}
}

// regression: #4132 ambiguous rows stay in the denominator.
func TestAmbiguousRunnersCannotInflateAMajority(t *testing.T) {
	rows := "| # | Date | Runner |\n|---|---|---|\n" +
		"| 1 | 2026-09-25 | implementer |\n| 2 | 2026-09-25 | independent-verifier |\n| 3 | 2026-09-25 | independent-verifier |\n\n" +
		"| # | Date | Runner |\n|---|---|---|\n| 2 | 2026-09-25 | other-verifier |\n| 3 | 2026-09-25 | other-verifier |\n"
	_, n, total, _ := evidenceRunnerTally(rows)
	if n != 1 || total != 3 || n*2 > total {
		t.Fatalf("ambiguous rows remain in the denominator: %d/%d", n, total)
	}
}

// regression: A3 non-transitive same-day tie — each same-day entry is compared
// with every other, not only with the first one seen.
func TestSameDayTieChecksAllEntries(t *testing.T) {
	rows := "| # | Date | Runner |\n|---|---|---|\n| 1 | 2026-09-25 | opus |\n\n" +
		"| # | Date | Runner |\n|---|---|---|\n| 1 | 2026-09-25 | opus-verifier |\n\n" +
		"| # | Date | Runner |\n|---|---|---|\n| 1 | 2026-09-25 | opus-worker |\n"
	if raw, n, total, ok := evidenceRunnerTally(rows); ok || n != 0 || total != 1 {
		t.Fatalf("same-day opus-verifier vs opus-worker is a conflict: %q %d/%d ok=%v", raw, n, total, ok)
	}
}

// TestEmptyKeyRowInDenominator pins that a complete row whose runner keys
// to nothing (a parenthetical-only label) is unknown, not omitted.
//
// regression: #4132 an unkeyed row stays in the denominator.
func TestEmptyKeyRowInDenominator(t *testing.T) {
	rows := "| # | Date | Runner |\n|---|---|---|\n" +
		"| 1 | 2026-09-25 | (non-implementer) |\n| 2 | 2026-09-25 | opus-verifier |\n"
	if raw, n, total, ok := evidenceRunnerTally(rows); !ok || raw != "opus-verifier" || n != 1 || total != 2 {
		t.Fatalf("an unkeyed row stays in the denominator: %q %d/%d ok=%v", raw, n, total, ok)
	}
}

// TestEvidenceRunnerTally pins the per-row LATEST-dated crediting and the
// declared-Runner-column guard that decide who the Evidence records as the runner.
func TestEvidenceRunnerTally(t *testing.T) {
	// A row re-run at a later date is credited to the RE-RUN's runner, not the
	// original, so the tally names who most recently ran each row.
	rerun := "| # | Date | Runner |\n|---|------|--------|\n" +
		"| 1 | 2026-07-27 | k3-verifier |\n\n" +
		"| # | Date | Runner |\n|---|------|--------|\n" +
		"| 1 | 2026-07-29 | opus-verifier |"
	raw, count, total, ok := evidenceRunnerTally(rerun)
	if !ok || total != 1 || count != 1 || runnerKey(raw) != "opus-verifier" {
		t.Errorf("re-run tally = (%q, %d, %d, %v), want the row credited to opus-verifier (1 of 1)", raw, count, total, ok)
	}

	// A majority across distinct rows.
	majority := "| # | Date | Runner |\n|---|------|--------|\n" +
		"| 1 | 2026-07-27 | k3-verifier |\n" +
		"| 2 | 2026-07-29 | opus-verifier |\n" +
		"| 3 | 2026-07-29 | opus-verifier |"
	raw, count, total, ok = evidenceRunnerTally(majority)
	if !ok || total != 3 || count != 2 || runnerKey(raw) != "opus-verifier" {
		t.Errorf("majority tally = (%q, %d, %d, %v), want opus-verifier 2 of 3", raw, count, total, ok)
	}

	// A table with no declared Runner column contributes nothing — the last cell
	// is free-text output, not an attribution (same guard as the verifier floor).
	noColumn := "| # | Command | Exit | Key output |\n|---|---------|------|------------|\n" +
		"| 1 | `go test ./...` | 0 | all green |"
	if _, _, _, ok := evidenceRunnerTally(noColumn); ok {
		t.Error("a table with no declared Runner column must yield ok=false (nothing to compare against)")
	}

	// An empty section compares against nothing.
	if _, _, _, ok := evidenceRunnerTally(""); ok {
		t.Error("an empty Evidence section must yield ok=false")
	}
}
