package main

// Awaiting-age: the "Awaiting verification /
// review" board gains a merged→verified AGE per row — how long the brief has sat
// in its CURRENT awaiting status (implemented or verified) according to the
// status historian. Render-only: ages are never a
// Next-up score input.
//
// Source of truth: the last history entry whose "to" equals the brief's current
// status. A brief with no such entry (historian predates it, or the log is
// absent — e.g. a fresh checkout) renders "—", never a guess.

import (
	"fmt"
	"time"
)

// awaitingAges maps "<stream>/<NN>" → rendered age for every brief whose last
// recorded transition INTO its current status is found in the history log.
// current maps briefID → current status (only awaiting briefs need be present,
// but extra entries are harmless). Ages render coarse — "<1h", "Nh" (< 48h),
// else "Nd" — so the generated board doesn't churn every regen minute.
func awaitingAges(entries []HistoryEntry, current map[string]string, now time.Time) map[string]string {
	enteredAt := awaitingEnteredAt(entries, current)
	out := make(map[string]string, len(enteredAt))
	for id, ts := range enteredAt {
		out[id] = renderAge(now.Sub(ts))
	}
	return out
}

// awaitingEnteredAt maps "<stream>/<NN>" → the time the brief entered its
// CURRENT status, per the historian: the last transition INTO that status.
// Absent from the map = the historian has no such record (its history predates
// the log, the log is missing, or the timestamp is malformed) — the age is
// UNKNOWN, and callers must render "—" rather than substitute a zero time,
// which would read as infinitely old.
//
// This is the raw, unrendered form of awaitingAges. It exists so callers that
// need to ORDER by age (the human-gate sign-off digest and the per-stream
// age-at-gate metric, methodology-metrics/38) sort real instants instead of
// parsing display strings like "10d" back into durations — one derivation of
// "when did this enter the queue", used by every consumer.
func awaitingEnteredAt(entries []HistoryEntry, current map[string]string) map[string]time.Time {
	// Last transition INTO the brief's current status wins (replay in order).
	enteredAt := make(map[string]time.Time)
	for _, e := range entries {
		status, tracked := current[e.Brief]
		if !tracked || e.To != status {
			continue
		}
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue // malformed ts: skip — render "—" rather than guess
		}
		enteredAt[e.Brief] = ts
	}
	return enteredAt
}

// renderAge renders a duration coarsely: "<1h", "Nh" under 48h, else "Nd".
// Negative durations (clock skew) clamp to "<1h" — never render negative age.
func renderAge(d time.Duration) string {
	if d < time.Hour {
		return "<1h"
	}
	if d < 48*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
