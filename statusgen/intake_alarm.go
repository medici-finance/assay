package main

import (
	"fmt"
	"strings"
	"time"
)

// intakeUntriagedThreshold is the age in days past which an untriaged intake entry
// triggers a NOTICE (non-fatal alarm). Mirrors the verification-debt alarm pattern.
// 3 days = one triage cycle.
const intakeUntriagedThreshold = 3

// IntakeAlarmResult holds the computed untriaged-intake alarm state.
type IntakeAlarmResult struct {
	Untriaged     int      // total entries with disposition: new (or empty, which defaults to new)
	OverThreshold int      // entries past the age threshold
	OldestID      string   // ID of the oldest untriaged entry with a parseable date ("" if none)
	OldestDays    int      // age in days of the oldest entry
	BadDates      []string // per-entry notices for unparseable date fields (empty if all parseable)
}

// intakeAlarm computes the untriaged-intake alarm state from parsed entries.
// Only disposition: new counts as untriaged; scoped/rejected/watching
// (and any unknown disposition) are triaged and must NOT count.
// Age = now - entry.Date, computed from in-git file content + the clock:
// deterministic, offline, no gh dependency (the issue-loop README's offline
// discipline — --lint must keep working with no network).
func intakeAlarm(entries []intakeEntry, now time.Time) IntakeAlarmResult {
	var res IntakeAlarmResult
	for _, e := range entries {
		d := strings.TrimSpace(e.Disposition)
		if d != "" && d != "new" {
			continue
		}
		res.Untriaged++
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(e.Date))
		if err != nil {
			// Malformed date: still counts as untriaged, but record a per-entry
			// notice naming the ID and the offending value so a typo'd date field
			// never silently exempts an entry from the age alarm. The entry can
			// never contribute to OverThreshold or Oldest* — no credible age.
			res.BadDates = append(res.BadDates,
				fmt.Sprintf("intake entry %s: unparseable date %q — age not computed", e.ID, e.Date))
			continue
		}
		days := int(now.Sub(parsed).Hours() / 24)
		if days > intakeUntriagedThreshold {
			res.OverThreshold++
		}
		if res.OldestID == "" || days > res.OldestDays {
			res.OldestID = e.ID
			res.OldestDays = days
		}
	}
	return res
}

// intakeDebtNotice returns a non-empty NOTICE string when at least one untriaged
// entry is past the age threshold. The NOTICE is advisory only — it never affects
// the exit code. Takes a pre-computed IntakeAlarmResult so callers compute once
// and use for both NOTICE and the board line.
func intakeDebtNotice(r IntakeAlarmResult) string {
	if r.OverThreshold == 0 {
		return ""
	}
	return fmt.Sprintf(
		"intake debt: %d untriaged (%d over %d days, oldest %s at %dd) — triage the front door",
		r.Untriaged, r.OverThreshold, intakeUntriagedThreshold, r.OldestID, r.OldestDays,
	)
}

// intakeScopedUnauthoredThreshold is the age in days past which a `scoped → <stream>`
// intake entry whose target stream was never authored triggers a NOTICE (non-fatal
// alarm). The scoped→authoring queue is a heavier stage than triage,
// so this mirrors the standing-alarm retro-cycle threshold (defaultStandingAgeDays,
// 7 days) rather than the 3-day triage cycle. Advisory only — never affects the exit
// code, never touches integrity logic.
const intakeScopedUnauthoredThreshold = 7

// scopedToStream extracts the leading stream token from a free-text scoped-to
// value. The register convention allows a trailing parenthetical marker
// ("agent-flow-observability (new stream — …)"); only the leading whitespace-
// delimited token names the stream, so matching keys on that alone.
func scopedToStream(scopedTo string) string {
	fields := strings.Fields(scopedTo)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// intakeScopedUnauthoredNotices surfaces intake entries flipped `scoped → <stream>`
// whose target stream was never authored — no docs/streams/<stream>/README.md exists
// (loadStreams only yields stream dirs that HAVE a README, so a scoped-to stream
// token absent from the known-stream set is one that has no authored brief yet) —
// and which have aged past intakeScopedUnauthoredThreshold. This closes the queue gap
// the intake-desk tier gate leaves open: triage only QUEUES stream/brief authoring by
// flipping the entry `scoped → <stream>`, but nothing drains the queue, so a flip can
// dangle indefinitely with no alarm (evidence: I-03 scoped → agent-flow-observability
// dangled 14 days).
//
// Additive advisory NOTICE only — never a hard problem, never touches anti-falsification
// / integrity-check logic. Deterministic and offline: keys on the in-git date
// frontmatter + the clock, no network (the --lint offline discipline). A scoped entry
// with an unparseable date gets a per-entry NOTICE naming the ID and offending value so
// a typo'd date can never silently exempt a dangling flip from the alarm.
func intakeScopedUnauthoredNotices(entries []intakeEntry, streams []*Stream, now time.Time) []string {
	known := make(map[string]bool, len(streams))
	for _, s := range streams {
		known[s.Name] = true
	}
	var out []string
	for _, e := range entries {
		if strings.TrimSpace(e.Disposition) != "scoped" {
			continue
		}
		target := scopedToStream(e.ScopedTo)
		if target == "" {
			continue // no target token to resolve — not an unauthored-stream case
		}
		if known[target] {
			continue // stream authored (has a README + briefs) — queue was drained
		}
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(e.Date))
		if err != nil {
			out = append(out, fmt.Sprintf(
				"intake entry %s: unparseable date %q — scoped→%s age not computed",
				e.ID, e.Date, target))
			continue
		}
		days := int(now.Sub(parsed).Hours() / 24)
		if days > intakeScopedUnauthoredThreshold {
			out = append(out, fmt.Sprintf(
				"scoped-but-unauthored: intake %s scoped → %s for %dd with no docs/streams/%s/README.md — author the stream/brief or re-triage",
				e.ID, target, days, target))
		}
	}
	return out
}

// intakeBoardLine renders the intake-debt board line for STATUS.md (the (b)
// surface from the brief). Rendered whether or not the threshold is breached
// (depth is always visible; the NOTICE fires only on age breach).
func intakeBoardLine(r IntakeAlarmResult) string {
	if r.Untriaged == 0 {
		return "_0 untriaged entries — the front door is clear._"
	}
	s := fmt.Sprintf("_%d untriaged (%d over %d days",
		r.Untriaged, r.OverThreshold, intakeUntriagedThreshold)
	if r.OldestID != "" {
		s += fmt.Sprintf(", oldest %s at %dd", r.OldestID, r.OldestDays)
	}
	s += ") — triage the front door_"
	return s
}
