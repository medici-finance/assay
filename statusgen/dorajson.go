package main

// dorajson.go — the internal `--dora-json` emitter: the frozen full-DORA feed
// shape, over the RETAINED grouped-DORA core and the recorded timing substrate.
//
// WHAT THIS IS. The publish path renders its DORA block from ONE frozen inner-feed
// shape (`generated`/`since`/`until`/`state` + `metrics.<key>`, each metric carrying
// a three-state `state`). Two sources emit that shape: an external delivery-metrics
// platform for deployment frequency, and THIS emitter for the three metrics that
// platform cannot source faithfully from our data. The consumer wraps the feed
// VERBATIM into the published snapshot, so this file's output is a public artifact.
//
// WHAT IT EMITS — and deliberately does NOT. Exactly three keys:
//
//	change_lead_time    <- the recorded pr_lead_time series (doratiming.go)
//	time_to_restore     <- the recorded restore_episode series (doratiming.go)
//	change_failure_rate <- the retained grouped core's instability proxy, aggregated
//
// `deployment_frequency` is sourced elsewhere and is NOT emitted here: two emitters
// writing the same key is a collision the frozen shape cannot resolve, and the
// consumer's validator accepts a SUBSET of the canonical keys precisely so each
// source can emit only what it owns.
//
// THIS IS NOT A REVIVAL of the removed standalone `--dora` CLI. That surface's
// grouped back-compat alias (doracli.go, runDora) stays exactly as it is. This is a
// NEW output mode over the same retained computation.
//
// HONEST BLANKS ARE THE POINT. A metric whose substrate holds no records renders
// {computed:false, state:"could-not-check", value:null} with `needs` naming what is
// missing — never a fabricated 0, and never a number borrowed from a different
// measurement wearing this metric's name. In particular change_lead_time is the
// recorded PULL-REQUEST commit→merge interval; the grouped core's brief-lifecycle
// implemented→done duration is a DIFFERENT quantity and is never substituted for it.
//
// PUBLIC-SAFE STRINGS. Every `reason`/`needs`/`probe` string below is serialized
// into the published snapshot. They are written self-contained: no issue refs, no
// stream or brief slugs, no repository names, no on-disk paths. The publish leak
// gate is the backstop, not the design.
//
// OFFLINE. Like the grouped alias, this emitter reads only local append-only logs
// and the in-repo registers — no git and no network. It is deterministic, and its
// numbers are bounded by the stated window plus the one declared result cap below.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// doraTimeToRestore is the frozen contract's key for the restore-interval metric.
// It is deliberately NOT roadmapdora.go's doraRecovery
// ("failed_deploy_recovery_time"): that is the grouped core's own internal key, and
// the published contract froze this spelling. Keeping both spellings explicit stops
// a rename in one from silently re-keying the other.
const doraTimeToRestore = "time_to_restore"

// defaultDoraJSONLimit is the result cap applied to each recorded timing series.
// It is a REAL applied bound, not a decorative one: past it the emitter aggregates
// the most recent `limit` in-window records and says so, so a capped number is
// never published unqualified.
const defaultDoraJSONLimit = 500

// --- the frozen contract shape ---------------------------------------------

// doraJSONMetric is one `metrics.<key>` value object.
//
// The state/computed/needs triple is not redundant — the consumer derives its own
// count from computed+needs while a renderer reads `state`, so the two MUST agree.
// buildDoraJSONMetric is the only constructor, so they cannot drift apart.
//
// Value is a pointer so an uncomputed metric serializes as an explicit `null`
// rather than a `0` that reads as measured.
type doraJSONMetric struct {
	Computed bool               `json:"computed"`
	Needs    []string           `json:"needs,omitempty"`
	State    string             `json:"state"`
	Value    *float64           `json:"value"`
	Unit     string             `json:"unit"`
	Sample   map[string]float64 `json:"sample,omitempty"`
}

// doraJSONCap declares one collection bound, in the same vocabulary the publish
// producer uses for its own probe limits, so a reader meets one shape not two.
// Limit is a pointer: null means "no result cap; bounded only by the stated
// window", which is a different claim from a cap of zero.
type doraJSONCap struct {
	AppliesTo        []string `json:"applies_to"`
	Probe            string   `json:"probe"`
	Limit            *int     `json:"limit"`
	BehaviourAtLimit string   `json:"behaviour_at_limit"`
}

// doraJSONFeed is the whole inner feed. The consumer wraps this object verbatim as
// the published snapshot's DORA block.
type doraJSONFeed struct {
	Generated string                    `json:"generated"`
	Since     string                    `json:"since"`
	Until     string                    `json:"until"`
	State     string                    `json:"state"`
	Reason    string                    `json:"reason,omitempty"`
	Caps      []doraJSONCap             `json:"caps"`
	Metrics   map[string]doraJSONMetric `json:"metrics"`
}

// --- metric construction ----------------------------------------------------

// buildDoraJSONMetric is the ONLY constructor for a metric object, so the
// three-state `state` can never disagree with the computed/needs pair the consumer
// reads. The mapping is the frozen contract's table:
//
//	measured        = computed && no needs
//	partial         = computed && needs
//	could-not-check = !computed  (value is forced to null)
func buildDoraJSONMetric(computed bool, value float64, unit string, needs []string, sample map[string]float64) doraJSONMetric {
	m := doraJSONMetric{Unit: unit, Needs: needs, Sample: sample}
	switch {
	case !computed:
		m.Computed = false
		m.State = "could-not-check"
		m.Value = nil // an uncomputed metric is null, never 0
	case len(needs) > 0:
		m.Computed = true
		m.State = "partial"
		v := value
		m.Value = &v
	default:
		m.Computed = true
		m.State = "measured"
		v := value
		m.Value = &v
	}
	return m
}

// capRecentSeconds applies the declared result cap to an interval series that is
// already window-filtered: it keeps the most recent `limit` records, dropping the
// OLDEST first. Dropping the oldest is the only defensible direction — the recent
// tail is what a delivery metric is about — and it matches how a capped list call
// truncates. Returns the kept intervals and whether the cap actually bit.
//
// `paired` is (terminalInstant, seconds) per record; the instant orders the series.
func capRecentSeconds(paired []doraTimedInterval, limit int) ([]int64, bool) {
	sort.Slice(paired, func(i, j int) bool {
		if paired[i].At.Equal(paired[j].At) {
			return paired[i].Seconds < paired[j].Seconds // stable tiebreak
		}
		return paired[i].At.Before(paired[j].At)
	})
	capped := false
	if limit > 0 && len(paired) > limit {
		paired = paired[len(paired)-limit:]
		capped = true
	}
	out := make([]int64, 0, len(paired))
	for _, p := range paired {
		out = append(out, p.Seconds)
	}
	return out, capped
}

// doraTimedInterval is one recorded interval with the instant that places it in the
// window — the minimum a cap needs in order to keep the most recent records.
type doraTimedInterval struct {
	At      time.Time
	Seconds int64
}

// collectTimedIntervals pulls one record type's in-window intervals out of the
// recorded substrate. `terminal` picks the instant that places a record in the
// window (restored_at for an episode, merged_at for a lead time) — the same
// terminal-instant convention the substrate's own query uses, so the two readers
// cannot disagree about which records a window contains.
func collectTimedIntervals(recs []doraTimingRecord, kind string, since, until time.Time) []doraTimedInterval {
	var out []doraTimedInterval
	for _, r := range recs {
		if r.Type != kind {
			continue
		}
		var ts string
		var secs int64
		switch kind {
		case "restore_episode":
			ts, secs = r.RestoredAt, r.RestoreSeconds
		case "pr_lead_time":
			ts, secs = r.MergedAt, r.LeadSeconds
		default:
			continue
		}
		if !inWindow(ts, since, until) {
			continue
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue // unparseable instant is out, never silently counted
		}
		out = append(out, doraTimedInterval{At: t.UTC(), Seconds: secs})
	}
	return out
}

// timingMetric shapes one recorded-interval series into a contract metric.
// An empty series is could-not-check with `needs` naming the missing substrate —
// the emitter is complete, the RECORDS are what is absent, and the two are
// different failures a reader deserves to be able to tell apart.
func timingMetric(intervals []doraTimedInterval, limit int, blankNeed string) doraJSONMetric {
	secs, capped := capRecentSeconds(intervals, limit)
	if len(secs) == 0 {
		return buildDoraJSONMetric(false, 0, "hours", []string{blankNeed}, nil)
	}
	p50 := pctlHours(secs, 0.5)
	sample := map[string]float64{
		"n":         float64(len(secs)),
		"p50_hours": p50,
		"p90_hours": pctlHours(secs, 0.9),
		"cap":       float64(limit),
	}
	var needs []string
	if capped {
		needs = []string{"the recorded series exceeded the stated result cap; this aggregate covers the most recent capped slice of the window, not every record in it"}
	}
	return buildDoraJSONMetric(true, p50, "hours", needs, sample)
}

// --- change_failure_rate ----------------------------------------------------

// changeFailureRate builds the instability metric from the RETAINED grouped core's
// own inputs, aggregated to a single repo-wide ratio.
//
// It reuses briefDoneCounts and briefReverts with a constant group key, so the
// aggregate is computed from the identical definitions the grouped tiles use — a
// second definition of "done" or "revert" here would be a number that disagrees
// with the board for no visible reason.
//
// Findings are counted ONCE EACH, deliberately NOT via findingsPerGroup: that
// helper walks a finding's Affects list and increments a counter per entry, which
// is correct per-group (a finding really does affect each) but double-counts in an
// aggregate — one finding is one failure signal, however many streams it touches.
//
// This is a POOLED ratio (sum of numerators over sum of denominators), never a mean
// of per-group ratios, which would weight a one-brief stream the same as a
// forty-brief one.
//
// It is always `partial`, never `measured`: the canonical metric wants
// change→incident linkage that this proxy does not have, and a proxy published as
// a measured number is the failure the three-state exists to prevent. A window with
// no completed work at all has no denominator and is could-not-check — never a 0%
// that reads as "nothing ever failed".
func changeFailureRate(history []HistoryEntry, findings []Finding, since, until time.Time) doraJSONMetric {
	oneGroup := func(string) string { return "" }
	done := briefDoneCounts(history, since, until, oneGroup)[""]
	reverts := briefReverts(history, since, until, oneGroup)[""]

	unresolved := 0
	for _, f := range findings {
		if !f.Resolved {
			unresolved++
		}
	}

	needs := []string{
		"linkage between a change and the incident it caused; this is an unlinked proxy over completed work items, recorded reversions and open defect records",
	}
	if done == 0 {
		return buildDoraJSONMetric(false, 0, "ratio", append(needs,
			"completed work in the stated window to form a denominator"), nil)
	}
	rate := float64(unresolved+reverts) / float64(done)
	// Round to four places: the inputs are integer counts, so anything beyond this
	// is float noise, and a byte-stable feed must not churn on it.
	rate = float64(int64(rate*10000+0.5)) / 10000
	return buildDoraJSONMetric(true, rate, "ratio", needs, map[string]float64{
		"completed_items":   float64(done),
		"reversions":        float64(reverts),
		"open_defect_notes": float64(unresolved),
	})
}

// --- feed assembly ----------------------------------------------------------

// overallState folds the per-metric states into the block-level one the consumer
// reads. Any measured or partial metric means the block carries SOMETHING, so it is
// never could-not-check; it is only `measured` when every emitted metric is.
func overallState(metrics map[string]doraJSONMetric) string {
	if len(metrics) == 0 {
		return "could-not-check"
	}
	allMeasured, anyComputed := true, false
	for _, m := range metrics {
		if m.State != "measured" {
			allMeasured = false
		}
		if m.Computed {
			anyComputed = true
		}
	}
	switch {
	case allMeasured:
		return "measured"
	case anyComputed:
		return "partial"
	default:
		return "could-not-check"
	}
}

// doraJSONCaps declares every collection bound the feed's numbers carry. The two
// recorded series are result-capped; the instability proxy reads its registers
// whole and is bounded only by the window, which is stated as a null limit rather
// than a zero (a cap of zero would claim nothing was read).
func doraJSONCaps(limit int) []doraJSONCap {
	l := limit
	return []doraJSONCap{
		{
			AppliesTo:        []string{"change_lead_time"},
			Probe:            "recorded pull-request commit-to-merge intervals, from the append-only timing log",
			Limit:            &l,
			BehaviourAtLimit: "at most this many records are aggregated; past the cap the most recent records in the window are used and the metric is marked partial",
		},
		{
			AppliesTo:        []string{"time_to_restore"},
			Probe:            "recorded main-branch red-to-green intervals, from the append-only timing log",
			Limit:            &l,
			BehaviourAtLimit: "at most this many records are aggregated; past the cap the most recent records in the window are used and the metric is marked partial",
		},
		{
			AppliesTo:        []string{"change_failure_rate"},
			Probe:            "completed work items, recorded reversions and open defect records, from the in-repo registers",
			Limit:            nil,
			BehaviourAtLimit: "no result cap; bounded only by the stated window",
		},
	}
}

// computeDoraJSONFeed is the pure builder — every input is a value, so the whole
// contract shape is testable with no clock, no filesystem and no network.
func computeDoraJSONFeed(recs []doraTimingRecord, history []HistoryEntry, findings []Finding, since, until, now time.Time, limit int) doraJSONFeed {
	metrics := map[string]doraJSONMetric{
		doraLeadTime: timingMetric(
			collectTimedIntervals(recs, "pr_lead_time", since, until), limit,
			"recorded pull-request commit-to-merge history; the timing log holds no such record for this window yet, so this interval is unread rather than zero"),
		doraTimeToRestore: timingMetric(
			collectTimedIntervals(recs, "restore_episode", since, until), limit,
			"recorded main-branch red-to-green history; the timing log holds no such record for this window yet, so this interval is unread rather than zero"),
		doraChangeFail: changeFailureRate(history, findings, since, until),
	}

	feed := doraJSONFeed{
		Generated: now.UTC().Format(time.RFC3339),
		Since:     since.UTC().Format(time.RFC3339),
		Until:     until.UTC().Format(time.RFC3339),
		State:     overallState(metrics),
		Caps:      doraJSONCaps(limit),
		Metrics:   metrics,
	}
	if feed.State != "measured" {
		feed.Reason = "this feed carries only the metrics computed from this project's own recorded history; deployment frequency is sourced separately and is absent by design. A metric reading could-not-check has no recorded data for the stated window — it is unread, never a measured zero."
	}
	return feed
}

// --- the CLI entry point ----------------------------------------------------

// runDoraJSON is the `statusgen --dora-json` entry point: a self-contained,
// STATUS.md-free, offline emit of the frozen inner feed to stdout, the same
// discipline as --dora / --dora-timing. Output is byte-stable — the encoder sorts
// map keys and every field is either a fixed string or a rounded number — so an
// unchanged input produces an unchanged feed.
func runDoraJSON(root, since, until string, limit int) int {
	now := nowFunc()

	untilT := now
	if until != "" {
		t, err := parseSinceDate(until)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --until must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		untilT = t
	}
	var sinceT time.Time
	if since != "" {
		t, err := parseSinceDate(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t
	} else {
		sinceT = untilT.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(untilT) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is after --until")
		return 1
	}
	if limit <= 0 {
		fmt.Fprintf(os.Stderr, "statusgen: --dora-limit must be a positive result cap, got %d\n", limit)
		return 1
	}

	// The recorded timing substrate. A MISSING log is not an error — it is a
	// project that has not recorded an interval yet, and the two timing metrics
	// then render could-not-check. A MALFORMED log is a hard error: silently
	// skipping a corrupt machine log would publish a number over a partial read.
	recs, err := loadDoraTimingRecords(filepath.Join(root, filepath.FromSlash(doraTimingRelPath)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: dora-json:", err)
		return 1
	}

	streams, findings, err := loadStreams(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "statusgen: loading streams: %v\n", err)
		return 1
	}
	_ = streams // the aggregate ratio needs no per-stream grouping

	var history []HistoryEntry
	if h, herr := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); herr == nil {
		history = h
	}

	feed := computeDoraJSONFeed(recs, history, findings, sinceT, untilT, now, limit)
	enc, merr := json.MarshalIndent(feed, "", "  ")
	if merr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: dora-json:", merr)
		return 1
	}
	fmt.Println(string(enc))
	return 0
}
