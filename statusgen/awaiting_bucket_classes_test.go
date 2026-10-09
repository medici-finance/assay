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

// TestAwaitRecordOrdering: a fail record decides nothing only when it is dated
// BEFORE the run that wrote the live PASS; a fail dated the same day or later
// holds the brief, a hold record always does, and a fail beside a live PASS
// whose dates cannot be read is could-not-check.
func TestAwaitRecordOrdering(t *testing.T) {
	rows := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	// The PASS run is dated on the heading above the verdict line, as Evidence
	// usually writes it. settledCI's row is dated 2026-10-01; the run, 10-02.
	settledPass := awaitEvidence{Text: settledCI + "\n### Re-verify 2026-10-02\n\n**VERIFY: PASS**\n"}
	// The same PASS with Verify row 1 left unrun (no Evidence row).
	unrunPass := awaitEvidence{Text: "### Re-verify 2026-10-02\n\n**VERIFY: PASS**\n"}
	rec := func(outcome, kind, ts string) awaitOutcomes {
		return awaitOutcomes{Latest: &awaitOutcome{Outcome: outcome, BlockerKind: kind, BlockerRef: "#41", TS: ts}}
	}
	const before, sameDay, after = "2026-09-27T10:00:00Z", "2026-10-02T23:00:00Z", "2026-10-05T08:00:00Z"
	humanImpl := awaitBrief{Status: "implemented", Gate: "human"}
	modelImpl := awaitBrief{Status: "implemented", Gate: "model"}
	modelVer := awaitBrief{Status: "verified", Gate: "model", Reviewed: "—"}
	irrev := awaitBrief{Status: "implemented", Gate: "model", Irreversible: true}

	t.Run("fail dated before the PASS run is superseded", func(t *testing.T) {
		for _, outcome := range []string{"verify-fail", "fail"} {
			oc := rec(outcome, "implementation", before)
			if got, owner, next := bucketAwaiting(humanImpl, rows, settledPass, oc); got != bucketHumanGate || owner != ownerDriver || next != nextActCloseCard {
				t.Errorf("%s gate human: %v %q (%s), want the driver's sign-off", outcome, got, owner, next)
			}
			if got, _, _ := bucketAwaiting(modelVer, rows, settledPass, oc); got != bucketRunnerPending {
				t.Errorf("%s gate model verified: %v, want runner-pending (CI's flip)", outcome, got)
			}
			if got, _, _ := bucketAwaiting(irrev, rows, settledPass, oc); got != bucketHumanGate {
				t.Errorf("%s irreversible: %v, want human gate", outcome, got)
			}
			if got, _, _ := bucketAwaiting(modelImpl, rows, unrunPass, oc); got != bucketRunnerPending {
				t.Errorf("%s gate model, a row unrun: %v, want runner-pending", outcome, got)
			}
		}
	})

	// Not older than the PASS run (same day or after), or a hold of any date:
	// the record decides.
	type held struct{ name, outcome, ts string }
	var holding []held
	for _, ts := range []string{sameDay, after} {
		holding = append(holding, held{"verify-fail " + ts, "verify-fail", ts}, held{"fail " + ts, "fail", ts})
	}
	for _, ts := range []string{before, sameDay, after, ""} {
		holding = append(holding, held{"blocked " + ts, "blocked", ts}, held{"needs-context " + ts, "needs-context", ts})
	}
	for _, h := range holding {
		t.Run(h.name, func(t *testing.T) {
			kinds := map[string]awaitingBucket{
				"implementation":   bucketRework,
				"check-definition": bucketRework,
				"human-action":     bucketHumanGate,
				"environment":      bucketEnvBlocked,
				"unknown":          bucketDeskActionable,
			}
			for kind, want := range kinds {
				oc := rec(h.outcome, kind, h.ts)
				// C1's fixture: gate model, one row unrun.
				if got, _, next := bucketAwaiting(modelImpl, rows, unrunPass, oc); got != want {
					t.Errorf("gate model, a row unrun, %s: %v (%s), want %v", kind, got, next, want)
				}
				// All rows settled.
				if got, _, next := bucketAwaiting(modelImpl, rows, settledPass, oc); got != want {
					t.Errorf("gate model, settled, %s: %v (%s), want %v", kind, got, next, want)
				}
				// Verified, Reviewed empty: not CI's flip while a record holds it.
				if got, _, next := bucketAwaiting(modelVer, rows, settledPass, oc); got == bucketRunnerPending {
					t.Errorf("gate model verified, %s: runner-pending (%s) over a recorded blocker", kind, next)
				}
				// Gate human: the driver's, and the act names the blocker.
				got, owner, next := bucketAwaiting(humanImpl, rows, settledPass, oc)
				if got != bucketHumanGate || owner != ownerDriver || !strings.HasPrefix(next, nextActHeldPrefix) || !strings.Contains(next, "#41") {
					t.Errorf("gate human, %s: %v %q (%s), want the driver naming the recorded blocker", kind, got, owner, next)
				}
			}
		})
	}

	t.Run("a fail beside a live PASS that cannot be ordered is could-not-check", func(t *testing.T) {
		undated := awaitEvidence{Text: "**VERIFY: PASS**\n"}
		for name, c := range map[string]struct {
			ev awaitEvidence
			ts string
		}{
			"PASS run undated": {undated, sameDay},
			"record ts absent": {settledPass, ""},
			"record ts bad":    {settledPass, "2026-10-02"},
		} {
			for _, b := range []awaitBrief{humanImpl, modelImpl, modelVer} {
				got, _, next := bucketAwaiting(b, rows, c.ev, rec("verify-fail", "implementation", c.ts))
				if got != bucketCouldNotCheck || !strings.Contains(next, "cannot be ordered") {
					t.Errorf("%s / %+v: %v (%s), want could-not-check naming the ordering", name, b, got, next)
				}
			}
		}
	})

	// A FAIL written AFTER the PASS wins: the record routes again.
	failAfter := awaitEvidence{Text: "**VERIFY: PASS** — 2026-10-01\n\n**VERIFY: FAIL** — 2026-10-03\n"}
	if got, _, _ := bucketAwaiting(modelImpl, rows, failAfter, rec("verify-fail", "implementation", "")); got != bucketRework {
		t.Errorf("PASS then FAIL = %v, want implementer rework", got)
	}
}

// TestLivePassDate: the PASS run's date is the newest date read up to the
// live PASS line, under lastVerifyVerdict's quotation rules.
func TestLivePassDate(t *testing.T) {
	for name, c := range map[string]struct{ ev, want string }{
		"date on the line":        {"**VERIFY: PASS** — 2026-10-02\n", "2026-10-02"},
		"date on a heading above": {"### Re-verify 2026-10-01\n| 1 | x | 0 | ok | 2026-09-30 | v |\nVERIFY: PASS\n", "2026-10-01"},
		"newest wins":             {"FAIL run 2026-09-27\nVERIFY: FAIL\n### 2026-10-07 re-verify\nVERIFY: PASS\n", "2026-10-07"},
		"later note not read":     {"### 2026-10-02\nVERIFY: PASS\nhold noted 2026-10-09\n", "2026-10-02"},
		"no date":                 {"VERIFY: PASS\n", ""},
		"live verdict FAIL":       {"2026-10-02 VERIFY: PASS\n2026-10-03 VERIFY: FAIL\n", ""},
		"fenced date ignored":     {"```\n2026-12-01\n```\n2026-10-02\nVERIFY: PASS\n", "2026-10-02"},
		"quoted date ignored":     {"> 2026-12-01\n2026-10-02 VERIFY: PASS\n", "2026-10-02"},
		"struck date ignored":     {"~~2026-12-01~~ 2026-10-02 VERIFY: PASS\n", "2026-10-02"},
		"not a calendar date":     {"2026-19-40 VERIFY: PASS\n", ""},
	} {
		if got := livePassDate(c.ev); got != c.want {
			t.Errorf("%s: livePassDate = %q, want %q", name, got, c.want)
		}
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
	// Row 1 reads the records too, so their readability is checked before it:
	// a human-gated PASS beside an unreadable store, a future-dated record or an
	// unrecognised outcome is could-not-check, never the driver's sign-off.
	pass := awaitEvidence{Text: "**VERIFY: PASS**\n"}
	for name, oc := range map[string]awaitOutcomes{
		"unreadable":   {Unreadable: "bad json"},
		"future":       {Future: true, Latest: &awaitOutcome{Outcome: "verified"}},
		"unrecognised": {Latest: &awaitOutcome{Outcome: "weird"}},
	} {
		for _, b := range []awaitBrief{{Status: "implemented", Gate: "human"}, {Status: "verified", Gate: "human"}, {Status: "implemented", Gate: "model", Irreversible: true}} {
			if got, _, next := bucketAwaiting(b, ci, pass, oc); got != bucketCouldNotCheck {
				t.Errorf("%s / %+v: bucket %v (%s), want could-not-check before row 1", name, b, got, next)
			}
		}
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
