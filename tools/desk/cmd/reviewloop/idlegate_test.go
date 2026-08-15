package main

import (
	"strings"
	"testing"
	"time"
)

const fixedNow = "2026-08-13T12:00:00Z"

func at(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return ts
}

// healthyBoard is the POSITIVE CONTROL for every blindness test below: a board that is
// fresh, scoped, complete and quiet. Without it, a gate that returned COULD-NOT-CHECK
// unconditionally would pass every negative test — which is the shape of a check that
// cannot fail.
func healthyBoard(t *testing.T, rows ...Row) *Board {
	t.Helper()
	return &Board{
		AsOf:  at(t, fixedNow),
		Rows:  rows,
		Scope: &BoardScope{Repos: []string{"example-org/example-repo"}, Count: 1, Source: "test"},
		Popn:  &PRPopulation{Complete: true, Cap: 100},
	}
}

func row(t *testing.T, action string, num int) Row {
	t.Helper()
	ru, err := LookupAction(action)
	if err != nil {
		t.Fatalf("LookupAction(%q): %v", action, err)
	}
	return Row{Repo: "example-org/example-repo", Number: num, Action: action, Head: "abc123def456", Rule: ru}
}

// TestIdleGate_QuietBoardIsIdle is the positive control. The gate MUST be able to say
// IDLE, or every assertion below is vacuous.
func TestIdleGate_QuietBoardIsIdle(t *testing.T) {
	b := healthyBoard(t, row(t, "READY", 1), row(t, "MERGE-CURR", 2), row(t, "WAIT-CI", 3))
	v := Idle(b, at(t, fixedNow))
	if v.State != IdleYes {
		t.Fatalf("quiet fresh board = %s (%v), want IDLE — the gate cannot pass its own positive control", v.State, v.BlindReasons)
	}
}

// TestIdleGate_RefusesIdleWithNeedsReview is the #79 test named by the brief's Verify row
// 2: idle must be REFUSED while a NEEDS-REVIEW row exists.
func TestIdleGate_RefusesIdleWithNeedsReview(t *testing.T) {
	b := healthyBoard(t, row(t, "NEEDS-REVIEW", 7), row(t, "READY", 8))
	v := Idle(b, at(t, fixedNow))
	if v.State == IdleYes {
		t.Fatalf("gate reported IDLE with a NEEDS-REVIEW row present — this is #79 rebuilt")
	}
	if v.State != IdleNo {
		t.Fatalf("state = %s, want NOT-IDLE", v.State)
	}
	if v.NeedsReview != 1 {
		t.Fatalf("NeedsReview = %d, want 1", v.NeedsReview)
	}
}

// TestIdleGate_RefusesIdleWithReReview — the other half of the gate. RE-REVIEW is the
// state where a WORKER is waiting on the desk, so a false idle here is worse, not milder.
func TestIdleGate_RefusesIdleWithReReview(t *testing.T) {
	b := healthyBoard(t, row(t, "RE-REVIEW", 9))
	if v := Idle(b, at(t, fixedNow)); v.State != IdleNo {
		t.Fatalf("state = %s (%v), want NOT-IDLE with a RE-REVIEW row", v.State, v.BlindReasons)
	}
}

// TestIdleGate_BlindNotIdle sweeps every way of failing to read the board and requires
// COULD-NOT-CHECK from each. Each case starts from healthyBoard and breaks ONE thing, so
// a case that stops being load-bearing shows up as the positive control passing.
func TestIdleGate_BlindNotIdle(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Board)
		now    string
		want   string // substring the blind reason must contain
	}{
		{"nil board", nil, fixedNow, "no board was read at all"},
		{"deskboard says STALE", func(b *Board) { b.Stale = true; b.Detail = "audit gap" }, fixedNow, "STALE"},
		{"sweep older than the cadence", nil, "2026-08-13T12:30:00Z", "heartbeat has lapsed"},
		{"no scope stated", func(b *Board) { b.Scope = nil }, fixedNow, "covered no repo set"},
		{"empty scope (roster unconfigured)", func(b *Board) { b.Scope.Count = 0 }, fixedNow, "scope is EMPTY"},
		{"no PR population stated", func(b *Board) { b.Popn = nil }, fixedNow, "never 'complete'"},
		{"PR population truncated at the cap", func(b *Board) {
			b.Popn = &PRPopulation{Complete: false, Cap: 100, TruncatedRepos: []string{"example-org/example-repo"}}
		}, fixedNow, "TRUNCATED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b *Board
			if tc.name != "nil board" {
				b = healthyBoard(t, row(t, "READY", 1))
				if tc.mutate != nil {
					tc.mutate(b)
				}
			}
			v := Idle(b, at(t, tc.now))
			if v.State != IdleCouldNotCheck {
				t.Fatalf("state = %s, want COULD-NOT-CHECK — a board that cannot be read must never report idle", v.State)
			}
			if !strings.Contains(strings.Join(v.BlindReasons, "; "), tc.want) {
				t.Fatalf("blind reasons %v do not name %q", v.BlindReasons, tc.want)
			}
		})
	}
}

// TestIdleGate_BlindnessOutranksAQuietCount is the trap the #79 incident actually walked
// into: a board that could not be fully read, whose VISIBLE rows happened to be quiet. The
// counts are 0/0 and every visible row is benign — and the answer must still be
// COULD-NOT-CHECK, because a truncated board can prove work exists and can never prove
// work is absent.
func TestIdleGate_BlindnessOutranksAQuietCount(t *testing.T) {
	b := healthyBoard(t, row(t, "READY", 1))
	b.Popn = &PRPopulation{Complete: false, Cap: 100, TruncatedRepos: []string{"example-org/example-repo"}}
	v := Idle(b, at(t, fixedNow))
	if v.State == IdleYes {
		t.Fatalf("a quiet TRUNCATED board reported IDLE — 0 visible actionable rows is not 0 actionable rows")
	}
	if v.NeedsReview != 0 || v.ReReview != 0 {
		t.Fatalf("counts should still be reported for triage, got %d/%d", v.NeedsReview, v.ReReview)
	}
}

// TestIdleGate_ZeroValueIsCouldNotCheck pins the zero-value choice. A caller that
// constructs an IdleVerdict and forgets to set State must get could-not-check, never a
// false all-clear.
func TestIdleGate_ZeroValueIsCouldNotCheck(t *testing.T) {
	var v IdleVerdict
	if v.State != IdleCouldNotCheck {
		t.Fatalf("zero-value IdleVerdict.State = %s, want COULD-NOT-CHECK", v.State)
	}
}
