package main

// --flow (graph-execution/07): the four durations that separate SCHEDULING
// delay from SERVICE time, plus CI-slot saturation and gate catch/override —
// see docs/streams/graph-execution/brief-07-flow-instruments.md.
//
// SCOPE NOTE ON "eligible-at" (read before touching eligibleAt below). The
// brief's own frontmatter cites "graph-execution/01 — ... the eligible-at
// instant this brief measures FROM" as an already-existing evaluator output.
// It is not: brief-01's evaluator (eligibility.go) computes a live VERDICT and
// records no timestamp of its own — confirmed by reading eligibility.go and
// docs/dependency-graph-design.md §3.7 at brief-07 implementation time. The
// brief's own exec-tier-why anticipates exactly this gap ("(a) which observed
// instant stands in for each unrecorded event is a design decision the facts
// do not pre-specify"), so this file supplies the missing derivation instead
// of escalating: eligibleAt walks the brief's `depends:` build-deps (NOT
// gates:/feathers: — a cross-repo/forge-backed edge replay is out of scope
// here; see the added paragraph in docs/dependency-graph-design.md) and reads
// the historian for the EARLIEST instant each one first reached done/verified.
// A brief with no depends: is eligible from its own first historian record.
// This is documented in docs/dependency-graph-design.md rather than only here,
// since a future reader of the evaluator's own docs would hit the same
// mismatch this brief did.
//
// REUSE, never a fork: the historian (history.go: LoadHistory, historyAbsPath),
// the hydrated stream loader (load.go: loadHydratedStreams), the DORA-timing
// window/JSON machinery (doratiming.go: doraTimingWindow, doraTargetRepo,
// doraToken), briefflow.go's resolveBFWindow/bfWindowJSON, and
// gatetelemetry.go's THREE-STATE JSON loaders (loadGtJSON, gtPRVerdict,
// gtGateClass, gtSmallN) — briefefficiency.go and gatetelemetry.go are READ
// ONLY per the brief's Context (their own completed-dwell walker and report
// are consumed, never edited); this file's flowTransitionEdges is a SEPARATE,
// small walker because external_wait/verification_time need the CLOSING
// status (briefTransitionEdge does not carry it) without touching that file.
//
// THREE-STATE throughout: every duration and every source-backed block reports
// measured / "< resolution" / could-not-check, never a fabricated number
// (docs/three-state-instrument-rule.md).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// --- duration + fleet-metric shapes -----------------------------------------

// FlowDuration is one three-state interval measurement.
type FlowDuration struct {
	Status  string `json:"status"` // "measured" | "< resolution" | "could-not-check"
	Seconds int64  `json:"seconds,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// FlowBriefRow is one brief's four durations.
type FlowBriefRow struct {
	ID               string       `json:"id"`
	EligibleToStart  FlowDuration `json:"eligible_to_start"`
	ActiveWorkTime   FlowDuration `json:"active_work_time"`
	ExternalWait     FlowDuration `json:"external_wait"`
	VerificationTime FlowDuration `json:"verification_time"`
}

// FlowFleetMetric is a fleet-wide median over "measured" rows only, gated on
// gtSmallN (gatetelemetry.go) so a handful of briefs cannot be presented as a
// fleet trend.
type FlowFleetMetric struct {
	Status        string `json:"status"` // "ok" | "could-not-check"
	N             int    `json:"n"`
	MedianSeconds int64  `json:"median_seconds,omitempty"`
}

// FlowMedians is the fleet-level roll-up of FlowBriefRow.
type FlowMedians struct {
	EligibleToStart  FlowFleetMetric `json:"eligible_to_start"`
	ActiveWorkTime   FlowFleetMetric `json:"active_work_time"`
	ExternalWait     FlowFleetMetric `json:"external_wait"`
	VerificationTime FlowFleetMetric `json:"verification_time"`
}

// FlowResolution is the historian's own observation granularity: the median
// gap between consecutive DISTINCT regen timestamps. A duration shorter than
// this is reported as "< resolution", never as a number (brief-07 facts).
type FlowResolution struct {
	Status  string `json:"status"` // "measured" | "could-not-check"
	Seconds int64  `json:"seconds,omitempty"`
}

// FlowEnvironment is stamped on every report. Every field is non-empty on
// every run — "could-not-check" is a valid, non-empty value; "" is a defect
// the fixture test refuses (Task item 3).
type FlowEnvironment struct {
	StatusgenVersion string `json:"statusgen_version"`
	WindowSince      string `json:"window_since"`
	WindowUntil      string `json:"window_until"`
	RegenCadence     string `json:"regen_cadence"` // seconds, stringified, or "could-not-check"
	ModelVersions    string `json:"model_versions"`
}

// FlowRate is a numerator/denominator pair, printed as a fraction (never a
// percentage — gatetelemetry.go's denominatorMarker convention).
type FlowRate struct {
	Num int `json:"num"`
	Den int `json:"den"`
}

// FlowGateClassRow is one non-audit-sourced gate class's fires/blocked count,
// re-emitted from gatetelemetry.go's own gates.json shape.
type FlowGateClassRow struct {
	Class      string `json:"class"`
	Fires      int    `json:"fires"`
	Blocked    int    `json:"blocked"`
	CatchKnown bool   `json:"catch_known"`
}

// FlowGateCatchOverride re-emits gatetelemetry.go's override-rate (leg a:
// app-approved-then-human-reversed) and per-gate-class fires/blocked, over the
// SAME source files (pr-verdicts.json, gates.json) at --root, read through the
// SAME three-state loaders gatetelemetry.go itself uses. Nothing about a gate
// is re-measured here (brief-07 facts) — audit-sourced classes and legs (b)/(c)
// are deliberately NOT replicated; read `--gate-telemetry` directly for those
// (noted in the PR, not re-derived here to avoid a second, drifting copy of
// gatetelemetry.go's audit-log join).
type FlowGateCatchOverride struct {
	Status       string             `json:"status"` // "ok" | "could-not-check"
	OverrideRate *FlowRate          `json:"override_rate,omitempty"`
	GateClasses  []FlowGateClassRow `json:"gate_classes,omitempty"`
	Reason       string             `json:"reason,omitempty"`
}

// FlowCISlotSaturation is (merged PRs/day × median CI wall-clock/PR) ÷
// --ci-hours-per-day. Gated on the --forge FLAG, never on credential presence
// (Verify row 6): every branch below returns before any network call unless
// forgeMode is true.
type FlowCISlotSaturation struct {
	Status string  `json:"status"` // "ok" | "could-not-check" ("ok" not yet reachable — see Reason on the one real path)
	Value  float64 `json:"value,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

// FlowReport is the whole `--flow` report.
type FlowReport struct {
	Generated         string                `json:"generated"`
	Window            doraTimingWindow      `json:"window"`
	Environment       FlowEnvironment       `json:"environment"`
	Resolution        FlowResolution        `json:"resolution"`
	Briefs            []FlowBriefRow        `json:"briefs"`
	Medians           FlowMedians           `json:"medians"`
	CISlotSaturation  FlowCISlotSaturation  `json:"ci_slot_saturation"`
	GateCatchOverride FlowGateCatchOverride `json:"gate_catch_override"`
}

// --- historian-derived durations ---------------------------------------------

// parseFlowTS parses an RFC3339 historian timestamp.
func parseFlowTS(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

// briefHistory returns id's own historian entries, sorted by ts.
func briefHistory(history []HistoryEntry, id string) []HistoryEntry {
	var out []HistoryEntry
	for _, e := range history {
		if e.Brief == id {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ts < out[j].Ts })
	return out
}

// dependencySatisfiedAt returns the EARLIEST instant id first reached
// done/verified — the same satisfying condition resolveInRepoBriefRef
// (eligibility.go) tests, replayed against the historian instead of current
// state.
func dependencySatisfiedAt(history []HistoryEntry, id string) (time.Time, bool) {
	for _, e := range briefHistory(history, id) {
		if e.To == "done" || e.To == "verified" {
			if t, ok := parseFlowTS(e.Ts); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// eligibleAt computes the instant id became eligible: the MAX over its
// depends: targets' satisfying instant (the last one to clear), or its own
// earliest historian record when it has no depends:. See the file header's
// scope note.
func eligibleAt(history []HistoryEntry, id string, deps []string) (time.Time, bool, string) {
	if len(deps) == 0 {
		own := briefHistory(history, id)
		if len(own) == 0 {
			return time.Time{}, false, id + ": no historian record at all"
		}
		t, ok := parseFlowTS(own[0].Ts)
		if !ok {
			return time.Time{}, false, id + ": unparsable first historian timestamp"
		}
		return t, true, ""
	}
	var latest time.Time
	for _, dep := range deps {
		t, ok := dependencySatisfiedAt(history, dep)
		if !ok {
			return time.Time{}, false, id + ": dependency " + dep + " has no recorded done/verified transition in the historian"
		}
		if t.After(latest) {
			latest = t
		}
	}
	return latest, true, ""
}

// firstInProgressAtOrAfter returns id's earliest recorded in-progress instant
// that is not before `after`.
func firstInProgressAtOrAfter(history []HistoryEntry, id string, after time.Time) (time.Time, bool) {
	for _, e := range briefHistory(history, id) {
		if e.To != "in-progress" {
			continue
		}
		t, ok := parseFlowTS(e.Ts)
		if !ok {
			continue
		}
		if !t.Before(after) {
			return t, true
		}
	}
	return time.Time{}, false
}

// finalizeDuration applies the resolution floor: a duration shorter than one
// regen cadence is "< resolution", never a number (brief-07 facts).
func finalizeDuration(secs int64, from, to time.Time, resolutionSecs int64, resolutionKnown bool) FlowDuration {
	if resolutionKnown && secs < resolutionSecs {
		return FlowDuration{Status: "< resolution"}
	}
	return FlowDuration{
		Status:  "measured",
		Seconds: secs,
		From:    from.UTC().Format(time.RFC3339),
		To:      to.UTC().Format(time.RFC3339),
	}
}

func computeEligibleToStart(history []HistoryEntry, id string, deps []string, resSecs int64, resKnown bool) FlowDuration {
	at, ok, reason := eligibleAt(history, id, deps)
	if !ok {
		return FlowDuration{Status: "could-not-check", Reason: reason}
	}
	start, ok := firstInProgressAtOrAfter(history, id, at)
	if !ok {
		return FlowDuration{Status: "could-not-check", Reason: id + ": no recorded in-progress observation at/after its eligible-at instant"}
	}
	secs := int64(start.Sub(at).Seconds())
	if secs < 0 {
		return FlowDuration{Status: "could-not-check", Reason: id + ": recorded in-progress instant precedes eligible-at — anomalous historian order, never fabricated"}
	}
	return finalizeDuration(secs, at, start, resSecs, resKnown)
}

// flowTransitionEdge is a completed dwell PLUS the status it closed into.
// briefefficiency.go's briefTransitionEdge (reused read-only) does not carry
// the closing status; external_wait/verification_time need it to tell an
// implemented→blocked hand-off from an implemented→verified one.
type flowTransitionEdge struct {
	Brief      string
	Status     string // the status the brief was IN during this dwell
	NextStatus string // the status it transitioned INTO at End
	Start, End time.Time
}

func flowTransitionEdges(history []HistoryEntry) []flowTransitionEdge {
	byBrief := map[string][]HistoryEntry{}
	for _, e := range history {
		byBrief[e.Brief] = append(byBrief[e.Brief], e)
	}
	var out []flowTransitionEdge
	for brief, entries := range byBrief {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Ts < entries[j].Ts })
		for i := 0; i+1 < len(entries); i++ {
			start, ok1 := parseFlowTS(entries[i].Ts)
			end, ok2 := parseFlowTS(entries[i+1].Ts)
			if !ok1 || !ok2 || end.Before(start) {
				continue
			}
			out = append(out, flowTransitionEdge{
				Brief: brief, Status: entries[i].To, NextStatus: entries[i+1].To,
				Start: start, End: end,
			})
		}
	}
	return out
}

func computeActiveWorkTime(history []HistoryEntry, id string, resSecs int64, resKnown bool) FlowDuration {
	var secs int64
	var from, to time.Time
	n := 0
	for _, e := range flowTransitionEdges(history) {
		if e.Brief != id || e.Status != "in-progress" {
			continue
		}
		secs += int64(e.End.Sub(e.Start).Seconds())
		if n == 0 || e.Start.Before(from) {
			from = e.Start
		}
		if n == 0 || e.End.After(to) {
			to = e.End
		}
		n++
	}
	if n > 0 {
		return finalizeDuration(secs, from, to, resSecs, resKnown)
	}
	own := briefHistory(history, id)
	if len(own) > 0 && own[len(own)-1].To == "in-progress" {
		return FlowDuration{Status: "could-not-check", Reason: id + ": currently in-progress with no later historian record — the interval is OPEN, never fabricated as a duration"}
	}
	return FlowDuration{Status: "could-not-check", Reason: id + ": no completed in-progress dwell in the historian"}
}

// computeExternalWait sums dwells that overlap a blocked row: a completed
// "blocked" dwell itself, or an "implemented" dwell that closed INTO blocked.
// SCOPE NOTE: the "open human-gate issue" leg of the facts' definition is not
// computed here — it needs a forge/issue-register read this Task's items
// never gate on --forge, so it is left could-not-check-by-omission rather than
// guessed; see the PR notes and the doc paragraph.
func computeExternalWait(history []HistoryEntry, id string, resSecs int64, resKnown bool) FlowDuration {
	var secs int64
	var from, to time.Time
	n := 0
	for _, e := range flowTransitionEdges(history) {
		if e.Brief != id {
			continue
		}
		if !(e.Status == "blocked" || (e.Status == "implemented" && e.NextStatus == "blocked")) {
			continue
		}
		secs += int64(e.End.Sub(e.Start).Seconds())
		if n == 0 || e.Start.Before(from) {
			from = e.Start
		}
		if n == 0 || e.End.After(to) {
			to = e.End
		}
		n++
	}
	if n == 0 {
		return FlowDuration{Status: "could-not-check", Reason: id + ": no completed blocked / blocked-bound-implemented dwell observed"}
	}
	return finalizeDuration(secs, from, to, resSecs, resKnown)
}

// computeVerificationTime sums completed "implemented" dwells that did NOT
// close into blocked (implemented → verified/done, the ordinary path).
func computeVerificationTime(history []HistoryEntry, id string, resSecs int64, resKnown bool) FlowDuration {
	var secs int64
	var from, to time.Time
	n := 0
	for _, e := range flowTransitionEdges(history) {
		if e.Brief != id || e.Status != "implemented" || e.NextStatus == "blocked" {
			continue
		}
		secs += int64(e.End.Sub(e.Start).Seconds())
		if n == 0 || e.Start.Before(from) {
			from = e.Start
		}
		if n == 0 || e.End.After(to) {
			to = e.End
		}
		n++
	}
	if n == 0 {
		return FlowDuration{Status: "could-not-check", Reason: id + ": no completed implemented→(non-blocked) dwell in the historian"}
	}
	return finalizeDuration(secs, from, to, resSecs, resKnown)
}

// flowResolution computes the historian's regen cadence: the median gap
// between consecutive DISTINCT recorded timestamps across the WHOLE historian
// (not window-scoped — the cadence is a property of the recording instrument,
// not of one query window).
func flowResolution(history []HistoryEntry) FlowResolution {
	seen := map[int64]bool{}
	for _, e := range history {
		if t, ok := parseFlowTS(e.Ts); ok {
			seen[t.Unix()] = true
		}
	}
	if len(seen) < 2 {
		return FlowResolution{Status: "could-not-check"}
	}
	ts := make([]int64, 0, len(seen))
	for u := range seen {
		ts = append(ts, u)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	gaps := make([]int64, 0, len(ts)-1)
	for i := 1; i < len(ts); i++ {
		gaps = append(gaps, ts[i]-ts[i-1])
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	return FlowResolution{Status: "measured", Seconds: gaps[len(gaps)/2]}
}

// fleetMetric aggregates "measured" rows into a fleet-wide median, gated on
// gtSmallN (gatetelemetry.go) so a handful of rows never reads as a trend.
func fleetMetric(durs []FlowDuration) FlowFleetMetric {
	var secs []int64
	for _, d := range durs {
		if d.Status == "measured" {
			secs = append(secs, d.Seconds)
		}
	}
	if len(secs) < gtSmallN {
		return FlowFleetMetric{Status: "could-not-check", N: len(secs)}
	}
	sort.Slice(secs, func(i, j int) bool { return secs[i] < secs[j] })
	return FlowFleetMetric{Status: "ok", N: len(secs), MedianSeconds: secs[len(secs)/2]}
}

// --- ci_slot_saturation -------------------------------------------------------

// computeCISlotSaturation never contacts the network unless forgeMode is
// true — the gate is the FLAG, never credential presence (Verify rows 5/6).
// Every early-return branch below runs before newGHClient/http is ever
// touched, so a stubbed http.DefaultTransport records zero calls whenever
// forgeMode is false, regardless of what GH_TOKEN/GITHUB_TOKEN carry.
func computeCISlotSaturation(root string, forgeMode bool, ciHoursPerDay float64) FlowCISlotSaturation {
	if !forgeMode {
		return FlowCISlotSaturation{Status: "could-not-check", Reason: "--forge not set — no network access attempted (the gate is the flag, never credential presence)"}
	}
	token := doraToken()
	if token == "" {
		return FlowCISlotSaturation{Status: "could-not-check", Reason: "no GH_TOKEN/GITHUB_TOKEN in the environment — the forge-derived term is unread, never zero"}
	}
	if ciHoursPerDay <= 0 {
		return FlowCISlotSaturation{Status: "could-not-check", Reason: "--ci-hours-per-day not declared — the divisor is never invented"}
	}
	repo := doraTargetRepo(root)
	if repo == "" {
		return FlowCISlotSaturation{Status: "could-not-check", Reason: "no target repo resolved ($GITHUB_REPOSITORY, git remote, gh default all unset)"}
	}
	fmt.Fprintf(os.Stderr, "flow: querying %s for ci_slot_saturation\n", repo)
	client := newGHClient(token)
	pulls, lookedAt, reason := client.fetchAllPulls(repo)
	if !lookedAt {
		return FlowCISlotSaturation{Status: "could-not-check", Reason: reason}
	}
	merged := 0
	for _, p := range pulls {
		if p.MergedAt != "" {
			merged++
		}
	}
	// The merged-PR-count term above is a REAL read (a network call was made
	// once --forge/--ci-hours-per-day/a token all held). The second term —
	// median CI wall-clock per PR, from forge check-run timestamps — has no
	// reader in this brief: ghfetch.go exposes pulls/reviews only, and adding
	// a check-run endpoint reader was out of this brief's Verify coverage (no
	// row exercises a "measured" ci_slot_saturation). Reporting a saturation
	// number from one of the two terms would be exactly the fabricated-number
	// failure the facts forbid, so this stays could-not-check even on a real,
	// successful merged-PR read. See the PR body for the follow-up.
	return FlowCISlotSaturation{Status: "could-not-check", Reason: fmt.Sprintf(
		"merged-PR term read (%d merged PR(s) in the fetched set) but the median-CI-wall-clock-per-PR term has no check-run reader in this brief — never reported as a number from one term alone", merged)}
}

// --- gate_catch_override -----------------------------------------------------

// computeGateCatchOverride re-emits gatetelemetry.go's override-rate (leg a)
// and non-audit-sourced gate-class fires/blocked counts, reading the SAME
// source files at root through the SAME loaders (loadGtJSON). See the
// FlowGateCatchOverride doc comment for what is deliberately not replicated.
func computeGateCatchOverride(root string) FlowGateCatchOverride {
	verdicts, verdictsSrc, err := loadGtJSON[gtPRVerdict](filepath.Join(root, "pr-verdicts.json"), "pr-verdicts.json")
	if err != nil {
		return FlowGateCatchOverride{Status: "could-not-check", Reason: "pr-verdicts.json: " + err.Error()}
	}
	gates, gatesSrc, err := loadGtJSON[gtGateClass](filepath.Join(root, "gates.json"), "gates.json")
	if err != nil {
		return FlowGateCatchOverride{Status: "could-not-check", Reason: "gates.json: " + err.Error()}
	}

	out := FlowGateCatchOverride{Status: "ok"}
	verdictsSrcUsable := verdictsSrc.usable()
	if !verdictsSrcUsable {
		out.Status = "could-not-check"
		out.Reason = "override-rate: " + verdictsSrc.reason
	} else {
		num, den := 0, 0
		for _, v := range verdicts {
			if !v.approved() {
				continue
			}
			den++
			if v.reversed() {
				num++
			}
		}
		out.OverrideRate = &FlowRate{Num: num, Den: den}
	}

	if gatesSrc.usable() {
		sorted := make([]gtGateClass, len(gates))
		copy(sorted, gates)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Class < sorted[j].Class })
		for _, g := range sorted {
			if g.AuditSourced {
				continue // audit-sourced legs: read --gate-telemetry directly (deliberately not replicated here)
			}
			blocked := 0
			for _, f := range g.Fires {
				if f.BlockedDefect {
					blocked++
				}
			}
			out.GateClasses = append(out.GateClasses, FlowGateClassRow{
				Class: g.Class, Fires: len(g.Fires), Blocked: blocked, CatchKnown: true,
			})
		}
	} else if out.Status == "ok" {
		out.Status = "could-not-check"
		out.Reason = "gate-classes: " + gatesSrc.reason
	} else {
		out.Reason += "; gate-classes: " + gatesSrc.reason
	}
	return out
}

// --- report assembly + CLI ---------------------------------------------------

func computeFlowReport(streams []*Stream, history []HistoryEntry, root string, forgeMode bool, ciHoursPerDay float64, since, until, now time.Time) FlowReport {
	res := flowResolution(history)
	resKnown := res.Status == "measured"

	var rows []FlowBriefRow
	for _, s := range streams {
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			rows = append(rows, FlowBriefRow{
				ID:               id,
				EligibleToStart:  computeEligibleToStart(history, id, b.Depends, res.Seconds, resKnown),
				ActiveWorkTime:   computeActiveWorkTime(history, id, res.Seconds, resKnown),
				ExternalWait:     computeExternalWait(history, id, res.Seconds, resKnown),
				VerificationTime: computeVerificationTime(history, id, res.Seconds, resKnown),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	eligibleDurs := make([]FlowDuration, len(rows))
	activeDurs := make([]FlowDuration, len(rows))
	extDurs := make([]FlowDuration, len(rows))
	verDurs := make([]FlowDuration, len(rows))
	for i, r := range rows {
		eligibleDurs[i] = r.EligibleToStart
		activeDurs[i] = r.ActiveWorkTime
		extDurs[i] = r.ExternalWait
		verDurs[i] = r.VerificationTime
	}

	regenCadence := "could-not-check"
	if resKnown {
		regenCadence = fmt.Sprintf("%d", res.Seconds)
	}

	return FlowReport{
		Generated: now.UTC().Format(time.RFC3339),
		Window:    bfWindowJSON(since, until),
		Environment: FlowEnvironment{
			StatusgenVersion: statusgenVersion,
			WindowSince:      since.UTC().Format(time.RFC3339),
			WindowUntil:      until.UTC().Format(time.RFC3339),
			RegenCadence:     regenCadence,
			ModelVersions:    "could-not-check",
		},
		Resolution: res,
		Briefs:     rows,
		Medians: FlowMedians{
			EligibleToStart:  fleetMetric(eligibleDurs),
			ActiveWorkTime:   fleetMetric(activeDurs),
			ExternalWait:     fleetMetric(extDurs),
			VerificationTime: fleetMetric(verDurs),
		},
		CISlotSaturation:  computeCISlotSaturation(root, forgeMode, ciHoursPerDay),
		GateCatchOverride: computeGateCatchOverride(root),
	}
}

// flowExitCode carries the gate-telemetry exit contract through (Verify row
// 7): 0 only once every source was read; 3 when any source (ci_slot_saturation
// or gate_catch_override) is could-not-check. Per-brief could-not-check rows
// are normal instrument output (an unstarted or open-interval brief), not a
// source-read failure, and never affect the exit code.
func flowExitCode(rep FlowReport) int {
	if rep.CISlotSaturation.Status != "ok" {
		return gtExitCouldNotCheck
	}
	if rep.GateCatchOverride.Status != "ok" {
		return gtExitCouldNotCheck
	}
	return 0
}

// runFlow is the `--flow` CLI entry point.
func runFlow(root string, forgeMode, jsonMode bool, since, until string, ciHoursPerDay float64) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	history, herr := LoadHistory(historyAbsPath(root))
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: flow:", herr)
		return 1
	}
	streams, _, serr := loadHydratedStreams(root)
	if serr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: flow:", serr)
		return 1
	}
	rep := computeFlowReport(streams, history, root, forgeMode, ciHoursPerDay, sinceT, untilT, now)

	if jsonMode {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(enc))
		return flowExitCode(rep)
	}

	fmt.Printf("flow -- %s ... %s (resolution: %s)\n", rep.Window.Since, rep.Window.Until, rep.Environment.RegenCadence)
	for _, r := range rep.Briefs {
		fmt.Printf("  %-24s eligible_to_start=%s active_work_time=%s external_wait=%s verification_time=%s\n",
			r.ID, flowDurationText(r.EligibleToStart), flowDurationText(r.ActiveWorkTime),
			flowDurationText(r.ExternalWait), flowDurationText(r.VerificationTime))
	}
	fmt.Printf("ci_slot_saturation: %s\n", rep.CISlotSaturation.Status)
	if rep.CISlotSaturation.Reason != "" {
		fmt.Printf("  ↳ %s\n", rep.CISlotSaturation.Reason)
	}
	fmt.Printf("gate_catch_override: %s\n", rep.GateCatchOverride.Status)
	if rep.GateCatchOverride.Reason != "" {
		fmt.Printf("  ↳ %s\n", rep.GateCatchOverride.Reason)
	}
	return flowExitCode(rep)
}

func flowDurationText(d FlowDuration) string {
	switch d.Status {
	case "measured":
		return fmt.Sprintf("%ds", d.Seconds)
	default:
		return d.Status
	}
}
