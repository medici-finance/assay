package main

import (
	"strings"
	"testing"
	"time"
)

func TestAwaitingAges(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	entries := []HistoryEntry{
		// a/01: in-progress → implemented 3h before now (the entry that counts).
		{Ts: "2026-07-12T06:00:00Z", Brief: "a/01", From: "todo", To: "in-progress"},
		{Ts: "2026-07-12T09:00:00Z", Brief: "a/01", From: "in-progress", To: "implemented"},
		// a/02: implemented 3 days ago, then verified 30 minutes ago — current
		// status is verified, so the VERIFIED entry is the one that counts.
		{Ts: "2026-07-09T12:00:00Z", Brief: "a/02", From: "in-progress", To: "implemented"},
		{Ts: "2026-07-12T11:30:00Z", Brief: "a/02", From: "implemented", To: "verified"},
		// a/03: history says done — but current says implemented (drift): no
		// entry INTO implemented exists, so no age is fabricated.
		{Ts: "2026-07-09T12:00:00Z", Brief: "a/03", From: "verified", To: "done"},
		// a/04: implemented 5 days ago (long-lived awaiting row).
		{Ts: "2026-07-07T12:00:00Z", Brief: "a/04", From: "in-progress", To: "implemented"},
		// a/05: malformed timestamp — skipped, no age.
		{Ts: "not-a-time", Brief: "a/05", From: "in-progress", To: "implemented"},
		// a/06 is tracked but has NO history entry at all.
	}
	current := map[string]string{
		"a/01": "implemented",
		"a/02": "verified",
		"a/03": "implemented",
		"a/04": "implemented",
		"a/05": "implemented",
		"a/06": "implemented",
	}

	ages := awaitingAges(entries, current, now)

	for id, want := range map[string]string{
		"a/01": "3h",
		"a/02": "<1h",
		"a/04": "5d",
	} {
		if got := ages[id]; got != want {
			t.Errorf("ages[%q] = %q, want %q", id, got, want)
		}
	}
	for _, id := range []string{"a/03", "a/05", "a/06"} {
		if got, ok := ages[id]; ok {
			t.Errorf("ages[%q] = %q, want absent (renders as —)", id, got)
		}
	}
}

func TestRenderAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "<1h"},
		{-2 * time.Hour, "<1h"}, // clock skew clamps, never negative
		{time.Hour, "1h"},
		{47 * time.Hour, "47h"},
		{48 * time.Hour, "2d"},
		{10 * 24 * time.Hour, "10d"},
	}
	for _, c := range cases {
		if got := renderAge(c.d); got != c.want {
			t.Errorf("renderAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// The Awaiting board renders the age column from the ages map, and "—" when
// the id is missing (mm/17, #282).
func TestEmitAwaitingAgeColumn(t *testing.T) {
	s := mkStream("frontend", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"},
		Brief{Num: "02", Wave: 0, Status: "implemented"},
	)
	s.LastTouch = day(5)
	out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, nil, nil), map[string]string{"frontend/01": "3h"}, IntakeAlarmResult{}, nil, "")
	for _, want := range []string{
		"| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |",
		"| frontend | 01 | implemented | 2000 | 0 | 3h | — | — |",
		"| frontend | 02 | implemented | 2000 | 0 | — | — | — |",
	} {
		if !contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && strings.Contains(s, sub) }
