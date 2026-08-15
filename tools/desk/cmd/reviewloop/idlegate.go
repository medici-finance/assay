package main

import (
	"fmt"
	"strings"
	"time"
)

// IdleState is the three-state answer to "may the desk say it is caught up?".
// docs/three-state-instrument-rule.md: checked-clean / checked-failed / could-not-check.
// The third state is the whole point. #79 was a two-state instrument — a monitor that
// went quiet produced the same output as a quiet board, and the desk reported all-clear
// while 19 actionable PRs sat unseen.
type IdleState int

const (
	// IdleCouldNotCheck — the board could not be positively read. NOT idle, NOT busy.
	// This is the zero value ON PURPOSE: a caller that forgets to set the state gets
	// could-not-check, never a false all-clear.
	IdleCouldNotCheck IdleState = iota
	// IdleNo — measured actionable rows exist.
	IdleNo
	// IdleYes — measured: a fresh, complete, fully-classified board with zero
	// NEEDS-REVIEW and zero RE-REVIEW.
	IdleYes
)

func (s IdleState) String() string {
	switch s {
	case IdleYes:
		return "IDLE"
	case IdleNo:
		return "NOT-IDLE"
	default:
		return "COULD-NOT-CHECK"
	}
}

// IdleVerdict is the gate's full answer: the state, the measured counts when they were
// measurable, and every reason the board could not be trusted.
type IdleVerdict struct {
	State        IdleState
	NeedsReview  int
	ReReview     int
	BlindReasons []string
}

func (v IdleVerdict) String() string {
	switch v.State {
	case IdleYes:
		return "IDLE — fresh board, complete population, 0 NEEDS-REVIEW, 0 RE-REVIEW"
	case IdleNo:
		return fmt.Sprintf("NOT-IDLE — %d NEEDS-REVIEW, %d RE-REVIEW", v.NeedsReview, v.ReReview)
	default:
		return "COULD-NOT-CHECK — " + strings.Join(v.BlindReasons, "; ")
	}
}

// MaxBoardAge is how stale a sweep may be and still support an idle claim. It matches the
// pr-review-desk skill's fixed-cadence sweep interval (~5 min) with a margin: a board
// older than one cadence tick means the instrument may have stopped, and the skill's own
// rule is that a board older than the cadence interval is blind, not idle.
const MaxBoardAge = 7 * time.Minute

// Idle is the #79 gate, in code. The desk may report caught-up ONLY on IdleYes.
//
// Order matters and is deliberate: every BLINDNESS check runs before the counts, and any
// blindness forces IdleCouldNotCheck regardless of what the visible rows said. A partial
// board can prove work EXISTS (so IdleNo would be safe) but can never prove work is
// ABSENT, and this function's job is only ever the second claim.
//
// Blindness sources, each a real observed failure mode rather than a hypothetical:
//
//   - nil board — the read failed; see ReadBoard. An empty payload never reaches here.
//   - stale — deskboard's own staleness flag, set when its audit trail cannot vouch for
//     the sweep.
//   - board older than MaxBoardAge — the sweep is not fresh; the heartbeat has lapsed.
//   - scope absent or count 0 — the empty-scope trap (deskboard #489): an unconfigured
//     roster makes every sweeping verb return nothing, and nothing is not zero.
//   - PR population absent — the verb read no PR list, which is never "complete".
//   - PR population incomplete — a repo returned open PRs at the `gh pr list --limit`
//     cap (#80), so counts are a FLOOR, not a total, and a hidden NEEDS-REVIEW is
//     exactly what would be missing.
func Idle(b *Board, now time.Time) IdleVerdict {
	v := IdleVerdict{State: IdleCouldNotCheck}
	if b == nil {
		v.BlindReasons = []string{"no board was read at all"}
		return v
	}

	if b.Stale {
		v.BlindReasons = append(v.BlindReasons, "deskboard reported the sweep STALE: "+b.Detail)
	}
	if age := now.Sub(b.AsOf); age > MaxBoardAge {
		v.BlindReasons = append(v.BlindReasons,
			fmt.Sprintf("the board is %s old (> %s cadence) — the sweep heartbeat has lapsed", age.Truncate(time.Second), MaxBoardAge))
	}
	switch {
	case b.Scope == nil:
		v.BlindReasons = append(v.BlindReasons, "the board stated no scope — it covered no repo set, which is not an empty repo set")
	case b.Scope.Count == 0:
		v.BlindReasons = append(v.BlindReasons, "the board's scope is EMPTY (0 repos) — an unconfigured roster sweeps nothing and reports nothing")
	}
	switch {
	case b.Popn == nil:
		v.BlindReasons = append(v.BlindReasons, "the board stated no PR population — this verb read no PR list, which is never 'complete'")
	case !b.Popn.Complete:
		v.BlindReasons = append(v.BlindReasons,
			fmt.Sprintf("the open-PR population is TRUNCATED at the %d cap (%s) — every count is a floor, not a total",
				b.Popn.Cap, strings.Join(b.Popn.TruncatedRepos, ", ")))
	}

	v.NeedsReview = b.CountAction("NEEDS-REVIEW")
	v.ReReview = b.CountAction("RE-REVIEW")

	if len(v.BlindReasons) > 0 {
		return v // could-not-check outranks any count read off an untrusted board
	}
	if v.NeedsReview > 0 || v.ReReview > 0 {
		v.State = IdleNo
		return v
	}
	v.State = IdleYes
	return v
}
