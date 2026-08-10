package main

// --bottleneck is the daily factory-floor bottleneck report (methodology-metrics/18):
// it computes per-stage WIP, per-stage dwell (from the historian), locates the
// constraint (highest WIP x median dwell), detects day-over-day shift from the
// previous report, and prescribes a deterministic action keyed to which stage is
// the bottleneck. Theory of Constraints / SCADA lens — a daily diagnostic, never
// a gate.
//
// Output: a dated markdown file under docs/reports/factory-floor/<date>.md
// (append-only history) AND a one-screen summary to stdout. Shift detection reads
// the newest prior dated file; if none exists, it reports "first report — no prior to compare."
//
// Honest-limitation discipline (F-08/F-09): WIP x dwell is a heuristic locator,
// not a proof. The report says "likely constraint" and shows the numbers behind it.
// Dwell for briefs with no historian row falls back to "unknown", counted
// separately, never silently zeroed. The Goodhart header from --dora applies
// verbatim: a constraint metric that becomes a target rots.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// bottleneckAntiGamingNote is the Goodhart header carried by every bottleneck
// report — the same discipline as --dora. A constraint metric that becomes a
// target ceases to be a good constraint signal.
const bottleneckAntiGamingNote = "Bottleneck metrics are DIAGNOSTIC, for continuous improvement — " +
	"never a target or a scorecard (Goodhart's law: a measure that becomes a target ceases to be " +
	"a good measure). WIP x dwell is a heuristic constraint locator, not a proof."

// bottleneckStageOrder is the pipeline order for rendering. done is last because
// it is not a constraint (work has left the pipeline); blocked is off the main
// pipeline but tracked separately.
var bottleneckStageOrder = []string{"todo", "in-progress", "implemented", "verified", "done", "blocked"}

// bottleneckStageLabel maps a lifecycle stage to the transition pair name used in
// the report (e.g. "verified" → "verified→done").
func bottleneckStageLabel(stage string) string {
	switch stage {
	case "todo":
		return "todo→in-progress"
	case "in-progress":
		return "in-progress→implemented"
	case "implemented":
		return "implemented→verified"
	case "verified":
		return "verified→done"
	case "blocked":
		return "blocked"
	case "done":
		return "done (exited)"
	default:
		return stage
	}
}

// bottleneckAction is the deterministic stage→action lookup table. It maps a
// constraint stage to the prescribed ToC action — documented, never freeform
// advice (brief-18 Task item 1).
func bottleneckAction(stage string) string {
	switch stage {
	case "todo":
		return "Dispatch-bound — check dispatch health (fanout throughput, claim contention, span-of-control overflow)."
	case "in-progress":
		return "WIP-bound — stop starting, start finishing; current WIP exceeds delivery capacity."
	case "implemented":
		return "Verify-bound — add verify capacity (verifier workers, verify-desk throughput); CONWIP throttle per I-29."
	case "verified":
		return "Review-bound — parallelize review recording; clear the review queue before starting new implementation."
	case "blocked":
		return "Unblock-bound — resolve blocking findings or staleness before pulling new work."
	case "done":
		return "Done-bound — no action needed; the pipeline has completed its work."
	default:
		return "Unknown constraint — investigate."
	}
}

// StageStats holds per-stage WIP and dwell metrics.
type StageStats struct {
	Stage        string
	WIP          int             // count of briefs currently in this stage
	KnownDwells  []time.Duration // dwell times for briefs with a historian entry for this stage
	UnknownDwell int             // count of briefs with no historian entry for this stage
	MedianDwell  time.Duration   // median of KnownDwells; 0 if none
}

// BottleneckReport is the full computed bottleneck analysis.
type BottleneckReport struct {
	Generated   time.Time
	Date        string // YYYY-MM-DD of the report
	Stages      []StageStats
	Constraint  string // stage name of the likely constraint; "" if unknown
	ShiftedFrom string // previous report's constraint; "" if no shift or first report
	Action      string // prescribed action from the stage→action table
	TotalBriefs int
}

// computeBottleneck derives the bottleneck report from current streams and the
// historian log. Pure and deterministic — same inputs → same report.
func computeBottleneck(streams []*Stream, history []HistoryEntry, now time.Time) BottleneckReport {
	rep := BottleneckReport{
		Generated: now,
		Date:      now.Format("2006-01-02"),
	}

	// 1. Per-stage WIP from current stream state.
	wipByStage := map[string]int{}
	total := 0
	for _, s := range streams {
		for _, b := range s.Briefs {
			wipByStage[b.Status]++
			total++
		}
	}
	rep.TotalBriefs = total

	// 2. Per-stage dwell from the historian.
	// For each brief, find the most recent transition INTO its current status.
	// The dwell time is now minus that transition timestamp.
	// Replay the log in order — the last `to` == current status wins.
	type briefState struct {
		status string
		id     string // "stream/NN"
	}
	var briefs []briefState
	for _, s := range streams {
		for _, b := range s.Briefs {
			briefs = append(briefs, briefState{status: b.Status, id: s.Name + "/" + b.Num})
		}
	}

	enteredAt := map[string]time.Time{} // brief id → when it entered its current status
	for _, e := range history {
		for _, bs := range briefs {
			if bs.id != e.Brief || e.To != bs.status {
				continue
			}
			ts, err := time.Parse(time.RFC3339, e.Ts)
			if err != nil {
				continue
			}
			enteredAt[bs.id] = ts
		}
	}

	// Collect per-stage dwells.
	dwellsByStage := map[string][]time.Duration{}
	unknownByStage := map[string]int{}

	for _, bs := range briefs {
		ts, ok := enteredAt[bs.id]
		if !ok {
			unknownByStage[bs.status]++
			continue
		}
		dwell := now.Sub(ts)
		if dwell < 0 {
			dwell = 0 // clock skew
		}
		dwellsByStage[bs.status] = append(dwellsByStage[bs.status], dwell)
	}

	// Build StageStats in pipeline order.
	for _, stage := range bottleneckStageOrder {
		dwells := dwellsByStage[stage]
		sort.Slice(dwells, func(i, j int) bool { return dwells[i] < dwells[j] })
		var median time.Duration
		if n := len(dwells); n > 0 {
			if n%2 == 1 {
				median = dwells[n/2]
			} else {
				// True median for even count: average the two central values.
				median = (dwells[n/2-1] + dwells[n/2]) / 2
			}
		}
		rep.Stages = append(rep.Stages, StageStats{
			Stage:        stage,
			WIP:          wipByStage[stage],
			KnownDwells:  dwells,
			UnknownDwell: unknownByStage[stage],
			MedianDwell:  median,
		})
	}

	// 3. Locate the constraint: highest WIP × median dwell among stages with
	//    known dwells. done and blocked are excluded — done has exited the
	//    pipeline, blocked is off the main flow.
	constraintStages := []string{"todo", "in-progress", "implemented", "verified"}
	var bestStage string
	var bestScore float64
	for _, stage := range constraintStages {
		for i := range rep.Stages {
			if rep.Stages[i].Stage != stage {
				continue
			}
			st := &rep.Stages[i]
			if st.WIP == 0 || st.MedianDwell == 0 {
				continue
			}
			score := float64(st.WIP) * st.MedianDwell.Seconds()
			if score > bestScore {
				bestScore = score
				bestStage = stage
			}
		}
	}

	// Fallback: if no main-pipeline stage has WIP with dwell, check blocked.
	if bestStage == "" {
		for i := range rep.Stages {
			if rep.Stages[i].Stage != "blocked" {
				continue
			}
			st := &rep.Stages[i]
			if st.WIP > 0 && st.MedianDwell > 0 {
				bestStage = "blocked"
				break
			}
		}
	}

	rep.Constraint = bestStage
	if bestStage != "" {
		rep.Action = bottleneckAction(bestStage)
	}
	return rep
}

// detectShift reads the newest prior report file (by date < today) and extracts the
// constraint stage from it. Returns "" if no prior report exists or it can't be parsed.
func detectShift(root string, today time.Time) string {
	dir := filepath.Join(root, "docs", "reports", "factory-floor")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	todayStr := today.Format("2006-01-02")
	var newestDate string // lexicographic max among dates before today
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		dateStr := strings.TrimSuffix(name, ".md")
		// Must be a valid date string and strictly before today.
		if dateStr >= todayStr || len(dateStr) != 10 {
			continue
		}
		if dateStr > newestDate {
			newestDate = dateStr
		}
	}
	if newestDate == "" {
		return ""
	}
	priorPath := filepath.Join(dir, newestDate+".md")
	data, err := os.ReadFile(priorPath)
	if err != nil {
		return ""
	}
	// Parse the "Likely constraint" line: look for "**Likely constraint:** `X`"
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "**Likely constraint:**") {
			continue
		}
		// Extract the stage name from backtick-quoted text.
		start := strings.Index(trimmed, "`")
		if start < 0 {
			continue
		}
		end := strings.Index(trimmed[start+1:], "`")
		if end < 0 {
			continue
		}
		return trimmed[start+1 : start+1+end]
	}
	return ""
}

// renderBottleneckText renders the bottleneck report as a one-screen stdout
// summary (the same format as the markdown file, but terminal-friendly).
func renderBottleneckText(rep BottleneckReport, shiftedFrom string) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	w("# Daily factory-floor bottleneck report — %s", rep.Date)
	w("")
	w("_%s_", bottleneckAntiGamingNote)
	w("")
	w("## Per-stage WIP & dwell")
	w("")
	w("| Stage | WIP | Median dwell | Unknown dwell | WIP x dwell (score) |")
	w("|---|---|---|---|---|")
	for _, st := range rep.Stages {
		dwellStr := "—"
		scoreStr := "—"
		if st.MedianDwell > 0 {
			dwellStr = humanDur(st.MedianDwell)
			scoreStr = fmt.Sprintf("%.0f", float64(st.WIP)*st.MedianDwell.Seconds())
		}
		unknownStr := ""
		if st.UnknownDwell > 0 {
			unknownStr = fmt.Sprintf("%d", st.UnknownDwell)
		} else {
			unknownStr = "0"
		}
		label := bottleneckStageLabel(st.Stage)
		w("| %s | %d | %s | %s | %s |", label, st.WIP, dwellStr, unknownStr, scoreStr)
	}

	w("")
	if rep.Constraint == "" {
		w("**Likely constraint:** unknown — no stage has both WIP and known dwell time. The historian may not have enough data yet.")
	} else {
		w("**Likely constraint:** `%s`", bottleneckStageLabel(rep.Constraint))
		w("")
		if shiftedFrom != "" && shiftedFrom != rep.Constraint {
			w("**Shift detected:** the constraint moved from `%s` → `%s` (ToC signal: relieve one station, the next becomes the bottleneck).", bottleneckStageLabel(shiftedFrom), bottleneckStageLabel(rep.Constraint))
		} else if shiftedFrom != "" && shiftedFrom == rep.Constraint {
			w("**No shift:** the constraint remains at `%s` — same as the previous report.", bottleneckStageLabel(rep.Constraint))
		} else {
			w("_First report — no prior to compare for shift detection._")
		}
		w("")
		w("**Prescribed action:** %s", rep.Action)
	}

	w("")
	w("_%d briefs total across all streams · historian: docs/streams/.history.jsonl · generated %s_",
		rep.TotalBriefs, rep.Date)
	return b.String()
}

// renderBottleneckMarkdown renders the same report as a markdown file for the
// append-only history under docs/reports/factory-floor/.
func renderBottleneckMarkdown(rep BottleneckReport, shiftedFrom string) string {
	// The markdown and text formats are identical for this report — both are
	// markdown. The file is the canonical record; stdout is the same content.
	return renderBottleneckText(rep, shiftedFrom)
}

// writeBottleneckReport writes the report to docs/reports/factory-floor/<date>.md.
// Creates the directory if it doesn't exist.
func writeBottleneckReport(root string, rep BottleneckReport, shiftedFrom string) error {
	dir := filepath.Join(root, "docs", "reports", "factory-floor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, rep.Date+".md")
	return os.WriteFile(path, []byte(renderBottleneckMarkdown(rep, shiftedFrom)), 0o644)
}

// runBottleneck is the --bottleneck entrypoint. It loads streams + history,
// computes the bottleneck report, writes the dated file, and prints the stdout
// summary. Self-contained diagnostic sub-command — never reads or writes
// STATUS.md (same discipline as --dora / --trend).
func runBottleneck(root string) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: bottleneck:", err)
		return 1
	}

	historyPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	history, _ := LoadHistory(historyPath) // missing/unreadable log → nil, not an error

	now := nowFunc()
	rep := computeBottleneck(streams, history, now)

	shiftedFrom := detectShift(root, now)

	// Write the dated report file (append-only history).
	if err := writeBottleneckReport(root, rep, shiftedFrom); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: bottleneck:", err)
		return 1
	}

	// Stdout summary.
	fmt.Print(renderBottleneckText(rep, shiftedFrom))
	return 0
}
