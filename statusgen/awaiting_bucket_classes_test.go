package main

import (
	"strings"
	"testing"
	"time"
)

// settledCI is an Evidence table whose only Verify row (1) has a complete run.
const settledCI = evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |\n"

// TestBucketAwaitingBlockerKindRouting: a recorded blocker routes by its KIND
// to the party that owns it, whether the record is a fail (verify-fail) or a
// hold (blocked). One fixture per kind per outcome value, so a kind routed
// only for the fail spelling is red.
func TestBucketAwaitingBlockerKindRouting(t *testing.T) {
	rows := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	brief := awaitBrief{Status: "implemented", Gate: "model"}
	cases := []struct {
		kind, ref string
		want      awaitingBucket
		owner     string
	}{
		{"implementation", "#41", bucketRework, ownerWorker},
		{"check-definition", "#41", bucketRework, ownerWorker},
		{"human-action", "#41", bucketHumanGate, ownerDriver},
		{"human-action", "", bucketHumanGate, ownerDriver},
		{"environment", "#41", bucketEnvBlocked, ownerOperator},
		{"environment", "", bucketEnvBlocked, ownerOperator},
		{"implementation", "", bucketDeskActionable, ownerVerifyDesk}, // no issue to cite
		{"unknown", "#41", bucketDeskActionable, ownerVerifyDesk},
		{"", "", bucketDeskActionable, ownerVerifyDesk},
	}
	for _, outcome := range []string{"verify-fail", "fail", "blocked", "needs-context", "needs_context"} {
		for _, c := range cases {
			oc := awaitOutcomes{Latest: &awaitOutcome{Outcome: outcome, BlockerKind: c.kind, BlockerRef: c.ref}}
			got, owner, next := bucketAwaiting(brief, rows, awaitEvidence{Text: ""}, oc)
			if got != c.want || owner != c.owner {
				t.Errorf("%s/%q ref %q: bucket %v owner %q (%s), want bucket %v owner %q", outcome, c.kind, c.ref, got, owner, next, c.want, c.owner)
			}
			if got == bucketRunnerPending {
				t.Errorf("%s/%q: a recorded blocker must never be runner-pending", outcome, c.kind)
			}
		}
	}
}

// TestBucketAwaitingLivePassSupersedesRecord: a fail or hold record is older
// than a live PASS verdict (the fix landed and the brief was re-verified); the
// record decides nothing then.
func TestBucketAwaitingLivePassSupersedesRecord(t *testing.T) {
	rows := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	pass := awaitEvidence{Text: settledCI + "\n**VERIFY: PASS** — 2026-10-02\n"}
	for _, outcome := range []string{"verify-fail", "blocked"} {
		oc := awaitOutcomes{Latest: &awaitOutcome{Outcome: outcome, BlockerKind: "implementation", BlockerRef: "#41"}}
		t.Run("gate human "+outcome, func(t *testing.T) {
			got, owner, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "human"}, rows, pass, oc)
			if got != bucketHumanGate || owner != ownerDriver {
				t.Errorf("bucket %v owner %q, want the driver's human gate (stale %s record must not route to rework)", got, owner, outcome)
			}
		})
		t.Run("gate model verified "+outcome, func(t *testing.T) {
			got, _, _ := bucketAwaiting(awaitBrief{Status: "verified", Gate: "model", Reviewed: "—"}, rows, pass, oc)
			if got != bucketRunnerPending {
				t.Errorf("bucket %v, want runner-pending (CI's flip)", got)
			}
		})
		t.Run("irreversible "+outcome, func(t *testing.T) {
			got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model", Irreversible: true}, rows, pass, oc)
			if got != bucketHumanGate {
				t.Errorf("bucket %v, want human gate", got)
			}
		})
	}
	// A FAIL written AFTER the PASS wins: the record routes again.
	failAfter := awaitEvidence{Text: "**VERIFY: PASS** — 2026-10-01\n\n**VERIFY: FAIL** — 2026-10-03\n"}
	got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, rows, failAfter,
		awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail", BlockerKind: "implementation", BlockerRef: "#41"}})
	if got != bucketRework {
		t.Errorf("PASS then FAIL = %v, want implementer rework", got)
	}
}

// TestBucketAwaitingRowGuards: each condition of rows 2, 3 and 5 is load
// bearing. Each fixture fails when exactly one half of the condition is
// dropped (the mutation the class guard in the PR body applies).
func TestBucketAwaitingRowGuards(t *testing.T) {
	ci := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	t.Run("row2 needs a kind that the worker owns", func(t *testing.T) {
		for _, kind := range []string{"environment", "human-action", "unknown", ""} {
			got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci, awaitEvidence{},
				awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail", BlockerKind: kind, BlockerRef: "#41"}})
			if got == bucketRework {
				t.Errorf("kind %q with an issue ref routed to implementer rework", kind)
			}
		}
	})
	t.Run("row3 needs an empty Reviewed cell", func(t *testing.T) {
		ev := awaitEvidence{Text: settledCI + "\n**VERIFY: PASS**\n"}
		got, _, _ := bucketAwaiting(awaitBrief{Status: "verified", Gate: "model", Reviewed: "2026-10-02 human:ian"}, ci, ev,
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "verified"}})
		if got == bucketRunnerPending {
			t.Error("a verified gate:model brief with a filled Reviewed cell is not waiting for CI's flip")
		}
		got, _, _ = bucketAwaiting(awaitBrief{Status: "verified", Gate: "model", Reviewed: "—"}, ci, ev,
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "verified"}})
		if got != bucketRunnerPending {
			t.Errorf("empty Reviewed = %v, want runner-pending", got)
		}
	})
	t.Run("row5 needs no FAIL in the Evidence", func(t *testing.T) {
		// A pass record is not a blocker, so only the Evidence verdict stops row 5.
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci,
			awaitEvidence{Text: "**VERIFY: FAIL** — 2026-10-02\n"},
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "verified"}})
		if got == bucketRunnerPending {
			t.Error("a FAIL verdict in the Evidence must not be runner-pending")
		}
	})
	t.Run("row5 needs no recorded blocker", func(t *testing.T) {
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci, awaitEvidence{},
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "blocked", BlockerKind: "implementation"}})
		if got == bucketRunnerPending {
			t.Error("a recorded hold must not be runner-pending")
		}
	})
	t.Run("row5 needs a row left to run", func(t *testing.T) {
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci, awaitEvidence{Text: settledCI}, awaitOutcomes{})
		if got == bucketRunnerPending {
			t.Error("every row settled: the runner has nothing to run, so the brief is not runner-pending")
		}
		if got != bucketDeskActionable {
			t.Errorf("all rows settled, no verdict = %v, want the desk's triage", got)
		}
	})
}

// TestBucketAwaitingUnrecognisedAndFutureAreCouldNotCheck: an outcome value
// no vocabulary names, or a record dated beyond the clock-skew tolerance,
// never routes to a bucket silently.
func TestBucketAwaitingUnrecognisedAndFutureAreCouldNotCheck(t *testing.T) {
	ci := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	brief := awaitBrief{Status: "implemented", Gate: "model"}
	for _, outcome := range []string{"verify-failed", "VERIFY-PASS", "weird", ""} {
		got, _, next := bucketAwaiting(brief, ci, awaitEvidence{},
			awaitOutcomes{Latest: &awaitOutcome{Outcome: outcome, BlockerKind: "implementation", BlockerRef: "#41"}})
		if got != bucketCouldNotCheck || !strings.Contains(next, "unrecognised") {
			t.Errorf("outcome %q: bucket %v (%s), want could-not-check naming the unrecognised value", outcome, got, next)
		}
	}
	got, _, next := bucketAwaiting(brief, ci, awaitEvidence{},
		awaitOutcomes{Future: true, Latest: &awaitOutcome{Outcome: "verified"}})
	if got != bucketCouldNotCheck || !strings.Contains(next, "clock-skew") {
		t.Errorf("future-dated record: bucket %v (%s), want could-not-check naming the skew", got, next)
	}
	// Case and padding of a known value still classify.
	for _, outcome := range []string{" Verify-Fail ", "BLOCKED"} {
		if classifyOutcome(outcome) == outcomeUnrecognised {
			t.Errorf("classifyOutcome(%q) = unrecognised", outcome)
		}
	}
}

// TestAwaitOutcomesFutureEndToEnd: the future flag survives the real record
// reader. A record dated far ahead never wins latest-per-brief, so without the
// flag the brief would read as having no record.
func TestAwaitOutcomesFutureEndToEnd(t *testing.T) {
	root := t.TempDir()
	future := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	writeOutcomeRecord(t, root, "fs", "01-20300101T000000Z-aaaaaaaaaaaa.json",
		`{"brief":"fs/01","ts":"`+future+`","outcome":"verified"}`)
	writeOutcomeRecord(t, root, "fs", "02-20260715T000000Z-bbbbbbbbbbbb.json",
		`{"brief":"fs/02","ts":"2026-07-15T00:00:00Z","outcome":"mystery"}`)
	s := &Stream{Name: "fs", Root: root}
	if oc := awaitOutcomesFor(s, "01"); !oc.Future {
		t.Errorf("future-dated record not flagged: %+v", oc)
	}
	if oc := awaitOutcomesFor(s, "02"); oc.Future || oc.Latest == nil || classifyOutcome(oc.Latest.Outcome) != outcomeUnrecognised {
		t.Errorf("unrecognised record not carried: %+v", oc)
	}
	if oc := awaitOutcomesFor(s, "03"); oc.Future || oc.Latest != nil {
		t.Errorf("brief with no record read as having one: %+v", oc)
	}
}
