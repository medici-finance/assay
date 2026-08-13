package main

// --trend is the SCADA "historian" view (I-08): it
// rolls the append-only status-transition log
// (docs/streams/.history.jsonl) up into a time series so you can see the
// methodology's state OVER TIME, not just now. Three signals per period:
//
//   - the status funnel (briefs per status at each period's end) — the
//     todo→…→done pipeline moving;
//   - throughput (transitions INTO `done` within the period) — the outcome
//     rate, not just output;
//   - the Awaiting-verification backlog (standing count of briefs at
//     `implemented`/`verified` — merged-but-not-done) — the lead-time debt
//     curve the verify-desk is meant to bend down (Task item 2).
//
// It is a pure READ over the historian log: like --verify-issues/--lint it
// never reads or writes STATUS.md, and it never mutates the log. Rendering is
// terminal-only ascii (small table + sparklines) since STATUS.md is text.
//
// Short/empty history degrades gracefully — fewer than two recorded
// transitions prints "insufficient history" rather than a misleading or
// crashing single-point "trend" (Task item 3).

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// trendStatuses is the funnel order rendered across the table columns — the
// lifecycle (todo → in-progress → implemented → verified → done) plus the
// off-pipeline `blocked` state. Kept in one place so headers and counts agree.
var trendStatuses = []string{"todo", "in-progress", "implemented", "verified", "done", "blocked"}

// trendStatusHeader is the compact column label for each status (the table is
// terminal-width-sensitive).
var trendStatusHeader = map[string]string{
	"todo":        "todo",
	"in-progress": "in-prog",
	"implemented": "impl",
	"verified":    "verif",
	"done":        "done",
	"blocked":     "block",
}

// backlogStatuses are the "merged-but-not-done" lead-time-debt states counted
// into the Awaiting-verification backlog curve (Task item 2).
var backlogStatuses = map[string]bool{"implemented": true, "verified": true}

// sparkRunes is the 8-level ascii sparkline ramp (U+2581..U+2588).
var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// trendBucket is one time period's rolled-up snapshot.
type trendBucket struct {
	start      time.Time
	counts     map[string]int // status → count of briefs in that status at period end
	throughput int            // transitions into `done` observed within the period
	backlog    int            // briefs at implemented|verified at period end
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

// buildTrend replays the log chronologically and snapshots per-period status
// funnel, throughput, and backlog. State is reconstructed from the FULL log
// (so backlog/funnel reflect transitions that happened before --since too);
// only which periods are DISPLAYED is clamped to [since, last]. Returns
// (buckets, true) on success, or (nil, false) when there is insufficient
// history to form a trend.
func buildTrend(entries []HistoryEntry, since time.Time, period string) ([]trendBucket, bool, error) {
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
	// before the first displayed bucket so funnel/backlog start correct.
	state := map[string]string{}
	idx := 0
	for idx < len(sorted) && times[idx].Before(displayStart) {
		state[sorted[idx].Brief] = sorted[idx].To
		idx++
	}

	var buckets []trendBucket
	for bs := displayStart; !bs.After(lastStart); bs = periodStep(bs, period) {
		end := periodStep(bs, period)
		thru := 0
		for idx < len(sorted) && times[idx].Before(end) {
			e := sorted[idx]
			if e.To == "done" && e.From != "done" {
				thru++
			}
			state[e.Brief] = e.To
			idx++
		}
		counts := map[string]int{}
		backlog := 0
		for _, st := range state {
			counts[st]++
			if backlogStatuses[st] {
				backlog++
			}
		}
		buckets = append(buckets, trendBucket{start: bs, counts: counts, throughput: thru, backlog: backlog})
	}
	if len(buckets) < 1 {
		return nil, false, nil
	}
	return buckets, true, nil
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

// renderTrend formats the historian view: a header, a per-period funnel table
// (status counts + throughput + backlog), and the two headline sparklines.
func renderTrend(buckets []trendBucket, period string, since time.Time, transitions int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "statusgen --trend — historian view (%s)\n", period)
	scope := "all history"
	if !since.IsZero() {
		scope = "since " + since.Format("2006-01-02")
	}
	plural := "periods"
	if len(buckets) == 1 {
		plural = "period"
	}
	fmt.Fprintf(&b, "%s · %d %s · %d transitions\n\n", scope, len(buckets), plural, transitions)

	// Header row.
	fmt.Fprintf(&b, "%-12s", "period")
	for _, st := range trendStatuses {
		fmt.Fprintf(&b, " %7s", trendStatusHeader[st])
	}
	fmt.Fprintf(&b, "  %7s %8s\n", "done/p", "backlog")

	dateFmt := "2006-01-02"
	var thru, backlog []int
	for _, bk := range buckets {
		fmt.Fprintf(&b, "%-12s", bk.start.Format(dateFmt))
		for _, st := range trendStatuses {
			fmt.Fprintf(&b, " %7d", bk.counts[st])
		}
		fmt.Fprintf(&b, "  %7d %8d\n", bk.throughput, bk.backlog)
		thru = append(thru, bk.throughput)
		backlog = append(backlog, bk.backlog)
	}

	fmt.Fprintf(&b, "\nthroughput  (→done per period):  %s  (%s)\n",
		sparkline(thru), seriesRange(thru))
	fmt.Fprintf(&b, "awaiting-verification backlog:   %s  (%s)  ← lead-time debt (impl+verif, merged-not-done)\n",
		sparkline(backlog), seriesRange(backlog))
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

// runTrend is the --trend entrypoint. historyPath overrides the default
// docs/streams/.history.jsonl (useful for pointing at an alternate snapshot);
// an empty value uses the default. Returns a process exit code.
func runTrend(root, historyPath, since, period string) int {
	path := historyPath
	if path == "" {
		path = filepath.Join(root, filepath.FromSlash(historyRelPath))
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}

	entries, err := LoadHistory(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: trend:", err)
		return 1
	}

	var sinceT time.Time
	if since != "" {
		sinceT, err = parseSinceDate(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: trend: --since %q is not YYYY-MM-DD: %v\n", since, err)
			return 1
		}
	}

	buckets, ok, err := buildTrend(entries, sinceT, period)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: trend:", err)
		return 1
	}
	if !ok {
		fmt.Println("insufficient history — need at least two recorded transitions to form a trend")
		return 0
	}
	fmt.Print(renderTrend(buckets, period, sinceT, len(entries)))
	return 0
}
