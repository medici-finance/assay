package main

import (
	"testing"
)

func headed(t *testing.T, action string, num int, head string) Row {
	t.Helper()
	r := row(t, action, num)
	r.Head = head
	return r
}

// neverDone is the "audit ledger has no record" stub — every verb is fresh.
func neverDone(string, int, string, string) bool { return false }

// TestCoalesceCollapsesRepeatDeltas — the brief's Verify row 3. N sweeps observing the
// SAME (repo, pr, head) must produce exactly ONE outward verb. Between two dispatch
// decisions a standing review window sees every cadence tick plus every event wake; without
// coalescing, one PR sitting at one head would be dispatched once per wake.
//
// The `dispatched=N` line below is the row's captured evidence.
func TestCoalesceCollapsesRepeatDeltas(t *testing.T) {
	var sweeps []*Board
	for i := 0; i < 5; i++ {
		sweeps = append(sweeps, healthyBoard(t, headed(t, "NEEDS-REVIEW", 42, "aaaaaaaaaaaa")))
	}
	got := Dispatched(Coalesce(sweeps, neverDone))
	t.Logf("5 board deltas on one (pr,head) -> dispatched=%d", len(got))
	if len(got) != 1 {
		t.Fatalf("dispatched %d verbs from 5 deltas on one (pr,head), want 1", len(got))
	}
	if got[0].Observations != 5 {
		t.Fatalf("Observations = %d, want 5 — the coalescing evidence must count what it collapsed", got[0].Observations)
	}
}

// TestCoalesceReArmsOnHeadAdvance is the archetype-B property, and the reason this driver
// is not a drain: the same PR at a NEW head is NEW work. A drain would consider the item
// done; the reactor re-arms.
func TestCoalesceReArmsOnHeadAdvance(t *testing.T) {
	sweeps := []*Board{
		healthyBoard(t, headed(t, "NEEDS-REVIEW", 42, "aaaaaaaaaaaa")),
		healthyBoard(t, headed(t, "RE-REVIEW", 42, "bbbbbbbbbbbb")), // head advanced mid-cycle
	}
	got := Dispatched(Coalesce(sweeps, neverDone))
	if len(got) != 2 {
		t.Fatalf("dispatched %d verbs across a head advance, want 2 — a PR at a new head is new work, not a completed item", len(got))
	}
}

// TestCoalesceHonoursTheBoardsMergeCurrVsReReview — the fixture drill the brief names.
// deskboard classifies a head advance as MERGE-CURR (benign keep-current merge, the PR's
// own files unchanged) or RE-REVIEW (own files changed). The reactor HONOURS that split
// and never re-derives it: MERGE-CURR carries no verb and produces no dispatch, at a head
// that is otherwise identical to the RE-REVIEW case.
func TestCoalesceHonoursTheBoardsMergeCurrVsReReview(t *testing.T) {
	benign := Dispatched(Coalesce([]*Board{
		healthyBoard(t, headed(t, "NEEDS-REVIEW", 42, "aaaaaaaaaaaa")),
		healthyBoard(t, headed(t, "MERGE-CURR", 42, "bbbbbbbbbbbb")),
	}, neverDone))
	if len(benign) != 1 {
		t.Fatalf("a MERGE-CURR head advance dispatched %d verbs, want 1 (only the original NEEDS-REVIEW) — "+
			"the board said the advance was benign and the reactor must not overrule it upward", len(benign))
	}
	real := Dispatched(Coalesce([]*Board{
		healthyBoard(t, headed(t, "NEEDS-REVIEW", 42, "aaaaaaaaaaaa")),
		healthyBoard(t, headed(t, "RE-REVIEW", 42, "bbbbbbbbbbbb")),
	}, neverDone))
	if len(real) != 2 {
		t.Fatalf("a RE-REVIEW head advance dispatched %d verbs, want 2", len(real))
	}
}

// TestCoalesceSuppressesAlreadyDone — the audit-ledger idempotency gate, on the same key
// the coalescer groups by, so the two cannot disagree.
func TestCoalesceSuppressesAlreadyDone(t *testing.T) {
	done := func(repo string, pr int, head, verb string) bool {
		return pr == 42 && head == "aaaaaaaaaaaa" && verb == "review"
	}
	got := Coalesce([]*Board{healthyBoard(t, headed(t, "NEEDS-REVIEW", 42, "aaaaaaaaaaaa"))}, done)
	if len(got) != 1 {
		t.Fatalf("planned %d verbs, want 1", len(got))
	}
	if !got[0].Suppressed {
		t.Fatal("a verb already recorded ok/noop at this head was not suppressed — the reactor would post twice")
	}
	if len(Dispatched(got)) != 0 {
		t.Fatal("a suppressed verb still reached the dispatch set")
	}
}

// TestCoalesceRefusesAnUnresolvedHead is the could-not-check arm of the idempotency gate.
// `deskboard actions` carries no head SHA; when the `prs` join cannot supply one, the
// (repo,pr,head,verb) key does not exist, so AlreadyDone cannot be asked. The verb is
// SUPPRESSED — never emitted on the assumption it is probably fine, and never counted as
// already done either.
func TestCoalesceRefusesAnUnresolvedHead(t *testing.T) {
	got := Coalesce([]*Board{healthyBoard(t, headed(t, "NEEDS-REVIEW", 42, HeadUnresolved))}, neverDone)
	if len(got) != 1 {
		t.Fatalf("planned %d verbs, want 1", len(got))
	}
	if !got[0].Suppressed {
		t.Fatal("a verb with an unresolved head was emitted — its idempotency could not be checked")
	}
	if len(Dispatched(got)) != 0 {
		t.Fatal("an unresolvable verb reached the dispatch set")
	}
}

// TestCoalesceIgnoresVerblessRows — SURFACE/WAIT/NO-OP rows must never acquire an outward
// verb by passing through the coalescer.
func TestCoalesceIgnoresVerblessRows(t *testing.T) {
	b := healthyBoard(t,
		headed(t, "MERGE-NOW", 1, "aaaaaaaaaaaa"),
		headed(t, "BLOCKED", 2, "bbbbbbbbbbbb"),
		headed(t, "CI-RED", 3, "cccccccccccc"),
		headed(t, "READY", 4, "dddddddddddd"),
		headed(t, "MERGE-CURR", 5, "eeeeeeeeeeee"),
	)
	if got := Coalesce([]*Board{b}, neverDone); len(got) != 0 {
		t.Fatalf("verbless rows produced %d outward verb(s): %v", len(got), got)
	}
}
