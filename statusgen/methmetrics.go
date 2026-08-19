package main

// methmetrics.go is the home of the METHODOLOGY-only metrics — the ones no
// off-the-shelf DORA tool (Apache DevLake included) can express, because they
// are derived from this methodology's brief/Evidence/historian semantics rather
// than from raw GitHub events.
//
// oss-replacement/06 split statusgen's commodity engineering metrics (lead time,
// deploy frequency, PR throughput, change-failure/instability, velocity/trend
// curves, roadmap rollups, code-efficiency ratios) OUT to DevLake and deleted
// their files (dora.go, roadmap.go, codeefficiency.go, trend.go). The one
// RETAINED metric that used to live inside the split-out trend.go — the
// awaiting-verification backlog curve — is rehomed here so a straight file
// deletion could not silently drop a methodology metric (the "trend.go trap").
//
// The awaiting-verification backlog curve is the standing count of briefs at
// `implemented`/`verified` — merged-but-not-done — rolled up over time from the
// append-only historian log (docs/streams/.history.jsonl). It is the lead-time
// DEBT curve the verify-desk exists to bend down: a methodology metric because
// `implemented`/`verified` are this methodology's verification-gate states, not
// anything DevLake models. It is surfaced by `--verif-backlog`.
//
// This file also carries the small shared render/time helpers that used to live
// in the split-out files but are still needed by a RETAINED surface:
//   - humanDur  — used by --bottleneck (bottleneck.go), formerly in dora.go;
//   - sparkline / seriesRange / sparkRunes / bucketStart / periodStep /
//     sortedEntries / parseSinceDate — used by the backlog curve below,
//     formerly in trend.go.
//
// Like --bottleneck/--trend before it, --verif-backlog is a pure READ over the
// historian: it never reads or writes STATUS.md and never mutates the log.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sparkRunes is the 8-level ascii sparkline ramp (U+2581..U+2588).
var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// backlogStatuses are the "merged-but-not-done" lead-time-debt states counted
// into the awaiting-verification backlog curve — this methodology's
// verification-gate states.
var backlogStatuses = map[string]bool{"implemented": true, "verified": true}

// humanDur formats a duration as days (≥1d) or hours, e.g. "6.0d", "4.5h".
// Retained here for --bottleneck after dora.go was split out (oss-replacement/06).
func humanDur(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if days := d.Hours() / 24; days >= 1 {
		return fmt.Sprintf("%.1fd", days)
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// parseSinceDate parses a --since value (YYYY-MM-DD, interpreted as 00:00 UTC).
func parseSinceDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// bucketStart truncates t to the start of its period (00:00 UTC that day for
// daily; the Monday of that week for weekly).
func bucketStart(t time.Time, period string) time.Time {
	t = t.UTC()
	y, m, d := t.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if period == "daily" {
		return day
	}
	// weekly: back up to Monday (Go's Weekday has Sunday==0).
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

// periodStep advances a bucket start to the next period's start.
func periodStep(start time.Time, period string) time.Time {
	if period == "daily" {
		return start.AddDate(0, 0, 1)
	}
	return start.AddDate(0, 0, 7)
}

// sortedEntries returns entries sorted by parsed timestamp (ascending). A
// malformed timestamp is a hard error — the log is a machine format.
func sortedEntries(entries []HistoryEntry) ([]HistoryEntry, []time.Time, error) {
	type te struct {
		e HistoryEntry
		t time.Time
	}
	tes := make([]te, 0, len(entries))
	for _, e := range entries {
		t, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			return nil, nil, fmt.Errorf("malformed timestamp %q for %s: %w", e.Ts, e.Brief, err)
		}
		tes = append(tes, te{e, t.UTC()})
	}
	sort.SliceStable(tes, func(i, j int) bool { return tes[i].t.Before(tes[j].t) })
	outE := make([]HistoryEntry, len(tes))
	outT := make([]time.Time, len(tes))
	for i, x := range tes {
		outE[i] = x.e
		outT[i] = x.t
	}
	return outE, outT, nil
}

// sparkline renders a compact ascii sparkline for a series. An all-equal (or
// single-point) series renders a flat mid-level bar rather than misleading
// full/empty blocks.
func sparkline(vals []int) string {
	if len(vals) == 0 {
		return ""
	}
	min, max := vals[0], vals[0]
	for _, v := range vals {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for _, v := range vals {
		if max == min {
			b.WriteRune(sparkRunes[len(sparkRunes)/2])
			continue
		}
		i := (v - min) * (len(sparkRunes) - 1) / (max - min)
		b.WriteRune(sparkRunes[i])
	}
	return b.String()
}

// seriesRange renders a "first → last" summary (or a single value) for a series.
func seriesRange(vals []int) string {
	if len(vals) == 0 {
		return ""
	}
	if len(vals) == 1 {
		return fmt.Sprintf("%d", vals[0])
	}
	return fmt.Sprintf("%d → %d", vals[0], vals[len(vals)-1])
}

// backlogPoint is one time period's awaiting-verification backlog snapshot.
type backlogPoint struct {
	start   time.Time
	backlog int // briefs at implemented|verified at period end
}

// buildVerifBacklog replays the historian chronologically and snapshots the
// per-period awaiting-verification backlog (standing count of briefs at
// `implemented`/`verified` — merged-but-not-done). State is reconstructed from
// the FULL log (so the backlog reflects transitions before --since too); only
// which periods are DISPLAYED is clamped to [since, last]. Returns
// (points, true) on success, or (nil, false) when there is insufficient history
// to form a curve.
//
// This is the RETAINED half of the former trend.go rollup: the commodity status
// funnel and throughput signals were split out to DevLake; the
// verification-debt curve is a methodology metric and stays here.
func buildVerifBacklog(entries []HistoryEntry, since time.Time, period string) ([]backlogPoint, bool, error) {
	if len(entries) < 2 {
		return nil, false, nil
	}
	sorted, times, err := sortedEntries(entries)
	if err != nil {
		return nil, false, err
	}

	firstStart := bucketStart(times[0], period)
	lastStart := bucketStart(times[len(times)-1], period)
	displayStart := firstStart
	if !since.IsZero() {
		if sb := bucketStart(since, period); sb.After(displayStart) {
			displayStart = sb
		}
	}
	if displayStart.After(lastStart) {
		// --since is past every recorded transition: nothing to show.
		return nil, false, nil
	}

	// Replay: maintain running per-brief status. Pre-roll entries that fall
	// before the first displayed bucket so the backlog starts correct.
	state := map[string]string{}
	idx := 0
	for idx < len(sorted) && times[idx].Before(displayStart) {
		state[sorted[idx].Brief] = sorted[idx].To
		idx++
	}

	var points []backlogPoint
	for bs := displayStart; !bs.After(lastStart); bs = periodStep(bs, period) {
		end := periodStep(bs, period)
		for idx < len(sorted) && times[idx].Before(end) {
			state[sorted[idx].Brief] = sorted[idx].To
			idx++
		}
		backlog := 0
		for _, st := range state {
			if backlogStatuses[st] {
				backlog++
			}
		}
		points = append(points, backlogPoint{start: bs, backlog: backlog})
	}
	if len(points) < 1 {
		return nil, false, nil
	}
	return points, true, nil
}

// renderVerifBacklog formats the awaiting-verification backlog curve: a header,
// a per-period table, and the headline sparkline. Terminal-only ascii, like
// --bottleneck and the former --trend.
func renderVerifBacklog(points []backlogPoint, period string, since time.Time, transitions int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "statusgen --verif-backlog — awaiting-verification backlog curve (%s)\n", period)
	fmt.Fprintf(&b, "_methodology metric (lead-time debt: impl+verif, merged-not-done) — retained; commodity DORA/velocity is DevLake's (oss-replacement/06)_\n")
	scope := "all history"
	if !since.IsZero() {
		scope = "since " + since.Format("2006-01-02")
	}
	plural := "periods"
	if len(points) == 1 {
		plural = "period"
	}
	fmt.Fprintf(&b, "%s · %d %s · %d transitions\n\n", scope, len(points), plural, transitions)

	fmt.Fprintf(&b, "%-12s %8s\n", "period", "backlog")
	dateFmt := "2006-01-02"
	var backlog []int
	for _, p := range points {
		fmt.Fprintf(&b, "%-12s %8d\n", p.start.Format(dateFmt), p.backlog)
		backlog = append(backlog, p.backlog)
	}

	fmt.Fprintf(&b, "\nawaiting-verification backlog:   %s  (%s)  ← lead-time debt (impl+verif, merged-not-done)\n",
		sparkline(backlog), seriesRange(backlog))
	return b.String()
}

// runVerifBacklog is the --verif-backlog entrypoint. historyPath overrides the
// default docs/streams/.history.jsonl (useful for pointing at an alternate
// snapshot); an empty value uses the default. Returns a process exit code.
// Self-contained diagnostic sub-command — never reads or writes STATUS.md.
func runVerifBacklog(root, historyPath, since, period string) int {
	path := historyPath
	if path == "" {
		path = filepath.Join(root, filepath.FromSlash(historyRelPath))
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}

	entries, err := LoadHistory(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: verif-backlog:", err)
		return 1
	}

	var sinceT time.Time
	if since != "" {
		sinceT, err = parseSinceDate(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: verif-backlog: --since %q is not YYYY-MM-DD: %v\n", since, err)
			return 1
		}
	}

	points, ok, err := buildVerifBacklog(entries, sinceT, period)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: verif-backlog:", err)
		return 1
	}
	if !ok {
		fmt.Println("insufficient history — need at least two recorded transitions to form a backlog curve")
		return 0
	}
	fmt.Print(renderVerifBacklog(points, period, sinceT, len(entries)))
	return 0
}
