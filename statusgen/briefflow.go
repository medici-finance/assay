package main

// Brief-flow metrics (statusgen/07) — the objective, business-value numbers
// that replace commits/merges/PRs and feed the AssayScore roll-up
// (statusgen/08): weighted throughput, lead time by size, flow efficiency,
// first-pass yield, review-rework, decision latency, per-stream stall. They
// are computed by the SAME statusgen binary as the existing instruments
// (--dora-timing, --trend/--verif-backlog, --bottleneck) so a published page
// has one provenance-checked source, and each new metric follows the same
// discipline those already do:
//
//   - EXTENDS, never forks: this file reuses the historian (history.go), the
//     hydrated stream loader (loadHydratedStreams, load.go), the DORA-timing
//     window/percentile/three-state machinery (doratiming.go: doraTimingWindow,
//     doraTimingMetric, aggregateSeconds, inWindow, doraTargetRepo), the
//     Brief:-trailer classifier (prlink.go), the Evidence verdict reader
//     (verifyissues.go: lastVerifyVerdict) and the finding→brief linkage
//     (drivecritical.go: reviewerFindingCritical). See briefefficiency.go,
//     briefflowreview.go and briefdecision.go for the metrics that also need a
//     network seam.
//   - THREE-STATE: every metric's JSON carries a top-level `state` field —
//     "ok" once at least one real data point was read, "could-not-check" for
//     thin/absent data. Never a fabricated 0 (brief-07 facts).
//   - Self-contained sub-commands: none of these read or write STATUS.md.
//
// This file holds the two metrics that are pure functions of the tree state
// (streams + the historian) — weighted throughput (1) and lead time by size
// (2) — plus the per-stream net-flow/stall view (7), which is the same
// substrate viewed per-stream instead of aggregated.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// effortWeight is the brief-flow point value per size (brief-07 facts: "weights
// S=1·M=3·L=8"). Shared by weighted throughput; AssayScore (statusgen/08) reuses
// it rather than re-declaring its own table.
var effortWeight = map[string]float64{"S": 1, "M": 3, "L": 8}

// bfStallDays is the per-stream stall threshold: an active stream with backlog
// and no historian transition in this many days is flagged stalled (brief-07
// Task item 7).
const bfStallDays = 14

// resolveBFWindow parses --since/--until (YYYY-MM-DD, until exclusive) into a
// [since, until) window, defaulting until to now and since to
// defaultDoraWindowDays before until — the same default every brief-flow
// metric shares with --dora-timing (doratiming.go), so a page rendering both
// families side by side compares like windows unless told otherwise.
func resolveBFWindow(since, until string, now time.Time) (sinceT, untilT time.Time, err error) {
	untilT = now
	if until != "" {
		untilT, err = parseSinceDate(until)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--until must be YYYY-MM-DD: %w", err)
		}
	}
	if since != "" {
		sinceT, err = parseSinceDate(since)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--since must be YYYY-MM-DD: %w", err)
		}
	} else {
		sinceT = untilT.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(untilT) {
		return time.Time{}, time.Time{}, fmt.Errorf("--since is after --until")
	}
	return sinceT, untilT, nil
}

// bfWindowJSON renders a resolved window the same shape as --dora-timing
// (doraTimingWindow, doratiming.go) — one window shape across every brief-flow
// emitter.
func bfWindowJSON(since, until time.Time) doraTimingWindow {
	return doraTimingWindow{Since: since.UTC().Format(time.RFC3339), Until: until.UTC().Format(time.RFC3339)}
}

// printBFJSON marshals and prints one report, indented (matching every
// existing --json emitter in this package).
func printBFJSON(v any) int {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(enc))
	return 0
}

// round1 rounds to one decimal place — the same convention doratiming.go's
// pctlHours uses, generalized to a bare float (not tied to the hours unit).
func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}

// --- metric 1: weighted brief throughput -----------------------------------

// bfThroughputSegment is one segment's (authored briefs, or the issue-loop)
// weighted point total for the window.
type bfThroughputSegment struct {
	Points        float64 `json:"points"`
	Count         int     `json:"count"`
	UnknownEffort int     `json:"unknown_effort"` // done events whose brief's effort could not be resolved — never silently dropped
}

type bfThroughputReport struct {
	Generated string              `json:"generated"`
	Window    doraTimingWindow    `json:"window"`
	State     string              `json:"state"` // "ok" | "could-not-check"
	Authored  bfThroughputSegment `json:"authored"`
	IssueLoop bfThroughputSegment `json:"issue_loop"`
	Total     bfThroughputSegment `json:"total"`
}

// computeThroughput sums effort-points (effortWeight) of every `to:"done"`
// historian transition landing in [since, until), segmented issue-loop
// (brief id under the scanStreamName stream) from authored streams — brief-07
// facts: "issue-loop segmented from authored streams". Effort is read from
// CURRENT stream state (a brief long done and later removed from the tree
// resolves to unknown_effort, counted honestly rather than silently dropped
// or fabricated as a zero-weight point).
func computeThroughput(streams []*Stream, history []HistoryEntry, since, until time.Time) bfThroughputReport {
	effortByID := map[string]string{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			effortByID[s.Name+"/"+b.Num] = b.Effort
		}
	}

	rep := bfThroughputReport{}
	for _, e := range history {
		if e.To != "done" || !inWindow(e.Ts, since, until) {
			continue
		}
		seg := &rep.Authored
		if strings.HasPrefix(e.Brief, scanStreamName+"/") {
			seg = &rep.IssueLoop
		}
		w, known := effortWeight[effortByID[e.Brief]]
		if !known {
			seg.UnknownEffort++
			continue
		}
		seg.Points += w
		seg.Count++
	}
	rep.Total = bfThroughputSegment{
		Points:        rep.Authored.Points + rep.IssueLoop.Points,
		Count:         rep.Authored.Count + rep.IssueLoop.Count,
		UnknownEffort: rep.Authored.UnknownEffort + rep.IssueLoop.UnknownEffort,
	}
	if rep.Total.Count > 0 {
		rep.State = "ok"
	} else {
		rep.State = "could-not-check"
	}
	return rep
}

func runThroughput(root, since, until string, asJSON bool) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: throughput:", err)
		return 1
	}
	historyPath := historyAbsPath(root)
	history, herr := LoadHistory(historyPath)
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: throughput:", herr)
		return 1
	}
	rep := computeThroughput(streams, history, sinceT, untilT)
	rep.Generated = now.UTC().Format(time.RFC3339)
	rep.Window = bfWindowJSON(sinceT, untilT)
	if asJSON {
		return printBFJSON(rep)
	}
	fmt.Printf("weighted throughput -- %s ... %s\n", rep.Window.Since, rep.Window.Until)
	fmt.Printf("  authored:   %.0f pts (%d briefs, %d unknown-effort)\n", rep.Authored.Points, rep.Authored.Count, rep.Authored.UnknownEffort)
	fmt.Printf("  issue-loop: %.0f pts (%d briefs, %d unknown-effort)\n", rep.IssueLoop.Points, rep.IssueLoop.Count, rep.IssueLoop.UnknownEffort)
	fmt.Printf("  total:      %.0f pts (%d briefs) [%s]\n", rep.Total.Points, rep.Total.Count, rep.State)
	return 0
}

// historyAbsPath is the one place every brief-flow emitter builds the
// historian path from --root, so a future rename of historyRelPath (history.go)
// only needs to change there.
func historyAbsPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(historyRelPath))
}

// --- metric 2: lead time authored->done, by size ----------------------------

// authoredDateRe extracts a leading YYYY-MM-DD from a brief's free-text
// `authored:` field (brief-v1 allows trailing prose, e.g. this very brief's own
// "2026-08-26 (re-authored clean for the statusgen board)" — brieffile.go only
// requires presence, not a strict date format).
var authoredDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// parseAuthoredDate extracts the day-precision authored date brief-07's facts
// call for ("authored: (day precision)"). ok is false for a brief with no
// leading date — thin data, excluded rather than guessed.
func parseAuthoredDate(raw string) (time.Time, bool) {
	m := authoredDateRe.FindString(strings.TrimSpace(raw))
	if m == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", m)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// authoredBriefInfo is one authored (brief-v1) brief's size + authored date —
// the two inputs lead-time-by-size needs beyond the historian.
type authoredBriefInfo struct {
	Effort   string
	Authored time.Time
}

// loadAuthoredBriefInfo scans every stream's brief-*.md files (briefFilePaths,
// brieffile.go) directly, the same iteration cynefin.go uses, because Authored
// is never hydrated onto the Brief README row (model.go) — only brief-v1 files
// carry it. A malformed/legacy/opted-out file is skipped (parseBriefFile's own
// contract); a valid file with no parseable authored date is skipped here
// (thin data, never guessed).
func loadAuthoredBriefInfo(streams []*Stream) map[string]authoredBriefInfo {
	out := map[string]authoredBriefInfo{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			id, _, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			at, okDate := parseAuthoredDate(bf.Authored)
			if !okDate {
				continue
			}
			out[id] = authoredBriefInfo{Effort: bf.Effort, Authored: at}
		}
	}
	return out
}

// bfLeadSizeStat is one size bucket's lead-time distribution. N==0 renders
// State:"could-not-check" — an honest empty, mirroring doraTimingMetric
// (doratiming.go), but in DAYS (the authored: field's own precision) rather
// than hours.
type bfLeadSizeStat struct {
	MedianDays float64 `json:"median_days,omitempty"`
	P85Days    float64 `json:"p85_days,omitempty"`
	N          int     `json:"n"`
	State      string  `json:"state,omitempty"`
}

type bfLeadTimeReport struct {
	Generated string                    `json:"generated"`
	Window    doraTimingWindow          `json:"window"`
	State     string                    `json:"state"` // "ok" if ANY size bucket has n>=1, else could-not-check
	BySize    map[string]bfLeadSizeStat `json:"by_size"`
}

// pctlDays mirrors doratiming.go's pctlHours convention (nearest-rank index,
// rounded to one decimal) on day-precision durations directly, since
// authored: is day precision (brief-07 facts) — a parallel implementation of
// the same PATTERN for a different unit, not a forked instrument. Used for
// p85 only; the median uses medianDays below.
func pctlDays(days []float64, q float64) float64 {
	sorted := append([]float64(nil), days...)
	sort.Float64s(sorted)
	idx := int(float64(len(sorted)) * q)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return round1(sorted[idx])
}

// medianDays computes the TRUE median (averaging the two central values on an
// even count), NOT nearest-rank pctlDays(0.5) — bottleneck.go's own dwell
// computation deliberately makes this same distinction, with a documented
// past bug as the reason: "the old buggy upper-median... masking this"
// (computeBottleneck's comment on why it averages). Brief-07's Task item 2
// asks for "median + p85" as two separately named statistics, which reads as
// the statistical median paired with a percentile — not two nearest-rank
// percentiles under different names.
func medianDays(days []float64) float64 {
	sorted := append([]float64(nil), days...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return round1(sorted[n/2])
	}
	return round1((sorted[n/2-1] + sorted[n/2]) / 2)
}

// computeLeadTimeBySize builds the authored->done lead-time distribution per
// S/M/L, reporting n alongside per brief-07's Task item 2. A done event whose
// brief has no resolvable authored date/effort (thin/legacy/removed) is
// excluded, never guessed.
func computeLeadTimeBySize(streams []*Stream, history []HistoryEntry, since, until time.Time) bfLeadTimeReport {
	info := loadAuthoredBriefInfo(streams)
	byBucket := map[string][]float64{"S": nil, "M": nil, "L": nil}

	for _, e := range history {
		if e.To != "done" || !inWindow(e.Ts, since, until) {
			continue
		}
		bi, ok := info[e.Brief]
		if !ok || !validEffort[bi.Effort] {
			continue
		}
		doneAt, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		days := doneAt.Sub(bi.Authored).Hours() / 24
		if days < 0 {
			days = 0 // clock/date-precision skew guard — never a negative lead time
		}
		byBucket[bi.Effort] = append(byBucket[bi.Effort], days)
	}

	rep := bfLeadTimeReport{BySize: map[string]bfLeadSizeStat{}}
	anyKnown := false
	for _, size := range []string{"S", "M", "L"} {
		vals := byBucket[size]
		if len(vals) == 0 {
			rep.BySize[size] = bfLeadSizeStat{State: "could-not-check", N: 0}
			continue
		}
		anyKnown = true
		rep.BySize[size] = bfLeadSizeStat{
			MedianDays: medianDays(vals),
			P85Days:    pctlDays(vals, 0.85),
			N:          len(vals),
		}
	}
	if anyKnown {
		rep.State = "ok"
	} else {
		rep.State = "could-not-check"
	}
	return rep
}

func runLeadTimeBySize(root, since, until string, asJSON bool) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: leadtime:", err)
		return 1
	}
	history, herr := LoadHistory(historyAbsPath(root))
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: leadtime:", herr)
		return 1
	}
	rep := computeLeadTimeBySize(streams, history, sinceT, untilT)
	rep.Generated = now.UTC().Format(time.RFC3339)
	rep.Window = bfWindowJSON(sinceT, untilT)
	if asJSON {
		return printBFJSON(rep)
	}
	fmt.Printf("lead time authored->done, by size -- %s ... %s\n", rep.Window.Since, rep.Window.Until)
	for _, size := range []string{"S", "M", "L"} {
		st := rep.BySize[size]
		if st.State == "could-not-check" {
			fmt.Printf("  %s: could-not-check (n=0)\n", size)
			continue
		}
		fmt.Printf("  %s: median=%.1fd p85=%.1fd (n=%d)\n", size, st.MedianDays, st.P85Days, st.N)
	}
	return 0
}

// --- metric 7: per-stream net flow + stall flag -----------------------------

// bfStreamFlow is one stream's arrivals/completions balance for the window
// plus its live stall flag (independent of the window — see computeNetFlow).
type bfStreamFlow struct {
	Stream         string `json:"stream"`
	Arrivals       int    `json:"arrivals"`
	Completions    int    `json:"completions"`
	NetFlow        int    `json:"net_flow"`
	Backlog        int    `json:"backlog"` // current todo+in-progress WIP
	Stalled        bool   `json:"stalled"`
	LastTransition string `json:"last_transition,omitempty"` // RFC3339; absent = no historian record ever
}

type bfNetFlowReport struct {
	Generated string           `json:"generated"`
	Window    doraTimingWindow `json:"window"`
	State     string           `json:"state"` // "ok" once >=1 stream is read, else could-not-check
	Streams   []bfStreamFlow   `json:"streams"`
}

// computeNetFlow derives arrivals (a brief's FIRST historian record — from:"" —
// landing in the window) minus completions (to:"done" landing in the window)
// per stream (brief-07 Task item 7: "arrivals - completions"). Stall
// ("active ∧ backlog>0 ∧ no transition >=14d") is a LIVE staleness check
// relative to `now`, deliberately independent of the reporting window — a
// stream can be net-flow-positive this window yet still be the one that has
// not moved in 20 days.
func computeNetFlow(streams []*Stream, history []HistoryEntry, since, until, now time.Time) bfNetFlowReport {
	arrivals := map[string]int{}
	completions := map[string]int{}
	lastTs := map[string]time.Time{}

	streamOf := func(brief string) string {
		name, _, ok := strings.Cut(brief, "/")
		if !ok {
			return brief
		}
		return name
	}

	for _, e := range history {
		stream := streamOf(e.Brief)
		if inWindow(e.Ts, since, until) {
			if e.From == "" {
				arrivals[stream]++
			}
			if e.To == "done" {
				completions[stream]++
			}
		}
		if ts, err := time.Parse(time.RFC3339, e.Ts); err == nil {
			if cur, ok := lastTs[stream]; !ok || ts.After(cur) {
				lastTs[stream] = ts
			}
		}
	}

	rep := bfNetFlowReport{}
	for _, s := range streams {
		backlog := 0
		for _, b := range s.Briefs {
			if b.Status == "todo" || b.Status == "in-progress" {
				backlog++
			}
		}
		last, haveLast := lastTs[s.Name]
		stale := !haveLast || now.Sub(last) >= bfStallDays*24*time.Hour
		row := bfStreamFlow{
			Stream:      s.Name,
			Arrivals:    arrivals[s.Name],
			Completions: completions[s.Name],
			NetFlow:     arrivals[s.Name] - completions[s.Name],
			Backlog:     backlog,
			Stalled:     s.Status == "active" && backlog > 0 && stale,
		}
		if haveLast {
			row.LastTransition = last.UTC().Format(time.RFC3339)
		}
		rep.Streams = append(rep.Streams, row)
	}
	sort.Slice(rep.Streams, func(i, j int) bool { return rep.Streams[i].Stream < rep.Streams[j].Stream })
	if len(rep.Streams) > 0 {
		rep.State = "ok"
	} else {
		rep.State = "could-not-check"
	}
	return rep
}

func runNetFlow(root, since, until string, asJSON bool) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: net-flow:", err)
		return 1
	}
	history, herr := LoadHistory(historyAbsPath(root))
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: net-flow:", herr)
		return 1
	}
	rep := computeNetFlow(streams, history, sinceT, untilT, now)
	rep.Generated = now.UTC().Format(time.RFC3339)
	rep.Window = bfWindowJSON(sinceT, untilT)
	if asJSON {
		return printBFJSON(rep)
	}
	fmt.Printf("per-stream net flow -- %s ... %s\n", rep.Window.Since, rep.Window.Until)
	for _, row := range rep.Streams {
		stall := ""
		if row.Stalled {
			stall = "  STALLED"
		}
		fmt.Printf("  %-24s arrivals=%d completions=%d net=%d backlog=%d%s\n",
			row.Stream, row.Arrivals, row.Completions, row.NetFlow, row.Backlog, stall)
	}
	return 0
}
