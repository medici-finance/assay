package main

// --flow-efficiency (brief-07, metric 3): touch / (touch + wait), from
// historian dwell. There is no work-start event instrumented anywhere in this
// tree today (grepped: no producer of such an event exists), so this ships the
// PROXY the brief's own facts explicitly allow ("flow efficiency needs a
// work-start event for honest touch/wait; ship a proxy if that event is thin")
// rather than inventing a new event stream — climbing the reuse ladder: the
// historian (history.go) already carries every stage transition with a
// timestamp, which is enough to derive a defensible touch/wait split without
// new instrumentation.
//
// PROXY DEFINITION. Only "in-progress" dwell counts as TOUCH (the one stage
// that is actually active implementation work); every other pre-done stage
// (todo, implemented, verified) counts as WAIT (queued, or queued for
// verification/review). A dwell is counted only once it is COMPLETE — i.e.
// there is a later historian record for the same brief that closes the
// interval — so an in-flight brief's still-open current stage is never
// guessed at (the same "no fabricated interval" discipline as
// doratiming.go's matchEpisodes). The interval's TERMINAL instant (when it
// left that stage) is what the [since, until) window bounds, mirroring
// computeDoraTiming's convention.
//
// THREE-STATE: brief-07 facts say to "emit could-not-check until enough
// post-instrumentation data exists" — gtSmallN (gatetelemetry.go's existing
// small-n threshold, reused rather than re-declared) draws that line: fewer
// completed intervals than that in the window is thin data, could-not-check.

import (
	"fmt"
	"os"
	"sort"
	"time"
)

// bfFlowEfficiencyReport is metric 3's JSON shape.
type bfFlowEfficiencyReport struct {
	Generated      string           `json:"generated"`
	Window         doraTimingWindow `json:"window"`
	State          string           `json:"state"`                // "ok" | "could-not-check"
	Efficiency     float64          `json:"efficiency,omitempty"` // touch / (touch+wait); omitted when could-not-check
	TouchSeconds   int64            `json:"touch_seconds"`
	WaitSeconds    int64            `json:"wait_seconds"`
	TouchIntervals int              `json:"touch_intervals"`
	WaitIntervals  int              `json:"wait_intervals"`
}

// briefTransitionEdge is one COMPLETED dwell: brief entered `status` at start
// and left it at end (the next historian record for that brief).
type briefTransitionEdge struct {
	Brief  string
	Status string
	Start  time.Time
	End    time.Time
}

// completedTransitionEdges walks the historian, grouped and sorted per brief,
// and returns every COMPLETED dwell (an entry followed by a later entry for
// the same brief) — the general-purpose primitive flow efficiency, and any
// future per-stage duration metric, builds on. A brief's current (still open)
// stage never appears here: only a later record closes an edge.
func completedTransitionEdges(history []HistoryEntry) []briefTransitionEdge {
	byBrief := map[string][]HistoryEntry{}
	for _, e := range history {
		byBrief[e.Brief] = append(byBrief[e.Brief], e)
	}
	var edges []briefTransitionEdge
	for brief, entries := range byBrief {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Ts < entries[j].Ts })
		for i := 0; i+1 < len(entries); i++ {
			start, err1 := time.Parse(time.RFC3339, entries[i].Ts)
			end, err2 := time.Parse(time.RFC3339, entries[i+1].Ts)
			if err1 != nil || err2 != nil || end.Before(start) {
				continue
			}
			edges = append(edges, briefTransitionEdge{
				Brief:  brief,
				Status: entries[i].To,
				Start:  start,
				End:    end,
			})
		}
	}
	return edges
}

// computeFlowEfficiency aggregates completed dwells whose TERMINAL instant
// (when the brief left that stage) falls in [since, until) into touch
// ("in-progress") and wait (every other pre-done stage) totals.
func computeFlowEfficiency(history []HistoryEntry, since, until time.Time) bfFlowEfficiencyReport {
	var touchSecs, waitSecs int64
	var touchN, waitN int
	for _, edge := range completedTransitionEdges(history) {
		if edge.End.Before(since) || !edge.End.Before(until) {
			continue
		}
		secs := int64(edge.End.Sub(edge.Start).Seconds())
		if secs < 0 {
			continue
		}
		switch edge.Status {
		case "in-progress":
			touchSecs += secs
			touchN++
		case "todo", "implemented", "verified":
			waitSecs += secs
			waitN++
		default:
			// "done"/"blocked" dwells are neither touch nor wait for this
			// proxy — done has exited the pipeline, blocked is off the main
			// flow (same exclusion bottleneck.go's constraint-locator makes).
		}
	}

	rep := bfFlowEfficiencyReport{
		TouchSeconds:   touchSecs,
		WaitSeconds:    waitSecs,
		TouchIntervals: touchN,
		WaitIntervals:  waitN,
	}
	total := touchN + waitN
	if total < gtSmallN || touchSecs+waitSecs == 0 {
		rep.State = "could-not-check"
		return rep
	}
	rep.State = "ok"
	// round to 3 decimals (round1 rounds to 1dp, too coarse for a 0..1 ratio).
	ratio := float64(touchSecs) / float64(touchSecs+waitSecs)
	rep.Efficiency = float64(int64(ratio*1000+0.5)) / 1000
	return rep
}

func runFlowEfficiency(root, since, until string, asJSON bool) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	history, herr := LoadHistory(historyAbsPath(root))
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: flow-efficiency:", herr)
		return 1
	}
	rep := computeFlowEfficiency(history, sinceT, untilT)
	rep.Generated = now.UTC().Format(time.RFC3339)
	rep.Window = bfWindowJSON(sinceT, untilT)
	if asJSON {
		return printBFJSON(rep)
	}
	if rep.State == "could-not-check" {
		fmt.Printf("flow efficiency -- %s ... %s: could-not-check (%d completed dwell intervals)\n",
			rep.Window.Since, rep.Window.Until, rep.TouchIntervals+rep.WaitIntervals)
		return 0
	}
	fmt.Printf("flow efficiency -- %s ... %s: %.2f (touch=%ds over %d, wait=%ds over %d)\n",
		rep.Window.Since, rep.Window.Until, rep.Efficiency, rep.TouchSeconds, rep.TouchIntervals, rep.WaitSeconds, rep.WaitIntervals)
	return 0
}
