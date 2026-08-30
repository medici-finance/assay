package main

// --decision-latency (brief-07, metric 6): WIP + oldest + latency over the
// "needs-decision" GitHub-issue queue — "the canonical decision-queue
// definition (the same one the anti-starvation floor work relies on)" (brief-
// 07 facts). decisionLabel (decisionissues.go: "needs-decision") IS that
// canonical definition already: it is the one label --decision-issues emits and
// the one the escalation vocabulary reserves for a human fork, so this file
// reuses the constant rather than re-declaring the queue.
//
// REUSE: aggregateSeconds + doraTimingMetric (doratiming.go) already implement
// exactly the p50/p90-with-honest-could-not-check shape latency needs; this
// file computes a []int64 of closed-issue latencies and hands it straight to
// aggregateSeconds rather than re-deriving percentiles.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// dqIssue is one needs-decision issue's timestamps.
type dqIssue struct {
	Number    int
	CreatedAt string
	ClosedAt  string // "" when still open
}

// decisionQueueSource is the network seam: gh issue list for one label/state.
// The production impl shells to gh; tests inject a fake.
type decisionQueueSource interface {
	Issues(repo, label, state string) ([]dqIssue, error)
}

type ghDecisionQueueSource struct{}

func (ghDecisionQueueSource) Issues(repo, label, state string) ([]dqIssue, error) {
	out, err := exec.Command("gh", "issue", "list",
		"--repo", repo, "--state", state, "--label", label, "--limit", "1000",
		"--json", "number,createdAt,closedAt").Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh issue list --repo %s --state %s --label %s: %v %s", repo, state, label, err, detail)
	}
	var issues []dqIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("unmarshal gh issue list for %s: %w", repo, err)
	}
	return issues, nil
}

// bfDecisionLatencyReport is metric 6's JSON shape.
type bfDecisionLatencyReport struct {
	Generated          string           `json:"generated"`
	Window             doraTimingWindow `json:"window"`
	State              string           `json:"state"` // reflects whether the gh reads SUCCEEDED, independent of counts (an empty-but-read queue is "ok")
	Latency            doraTimingMetric `json:"latency"`
	WIP                int              `json:"wip"`
	OldestOpenAgeHours float64          `json:"oldest_open_age_hours,omitempty"`
	OldestOpenIssue    int              `json:"oldest_open_issue,omitempty"`
}

// computeDecisionLatency aggregates closed-issue latency (createdAt->closedAt,
// windowed on closedAt — the terminal instant, same convention as
// computeDoraTiming) and the live WIP/oldest-open gauges (unwindowed — WIP is
// a point-in-time count, not a period total).
func computeDecisionLatency(open, closed []dqIssue, since, until, now time.Time) bfDecisionLatencyReport {
	var latSecs []int64
	for _, is := range closed {
		if is.ClosedAt == "" || !inWindow(is.ClosedAt, since, until) {
			continue
		}
		cAt, err1 := time.Parse(time.RFC3339, is.CreatedAt)
		clAt, err2 := time.Parse(time.RFC3339, is.ClosedAt)
		if err1 != nil || err2 != nil {
			continue
		}
		secs := int64(clAt.Sub(cAt).Seconds())
		if secs < 0 {
			continue
		}
		latSecs = append(latSecs, secs)
	}

	rep := bfDecisionLatencyReport{
		Latency: aggregateSeconds(latSecs),
		WIP:     len(open),
	}

	var oldestAt time.Time
	haveOldest := false
	for _, is := range open {
		cAt, err := time.Parse(time.RFC3339, is.CreatedAt)
		if err != nil {
			continue
		}
		if !haveOldest || cAt.Before(oldestAt) {
			oldestAt, rep.OldestOpenIssue, haveOldest = cAt, is.Number, true
		}
	}
	if haveOldest {
		rep.OldestOpenAgeHours = round1(now.Sub(oldestAt).Hours())
	}
	return rep
}

func runDecisionLatency(root, since, until string, asJSON bool, src decisionQueueSource) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	rep := bfDecisionLatencyReport{Generated: now.UTC().Format(time.RFC3339), Window: bfWindowJSON(sinceT, untilT)}

	repo := bfResolveTarget("decision-latency", root)
	if repo == "" {
		rep.State = "could-not-check"
		return finishDecisionLatency(rep, asJSON)
	}
	open, errO := src.Issues(repo, decisionLabel, "open")
	closed, errC := src.Issues(repo, decisionLabel, "closed")
	if errO != nil || errC != nil {
		rep.State = "could-not-check"
		return finishDecisionLatency(rep, asJSON)
	}
	computed := computeDecisionLatency(open, closed, sinceT, untilT, now)
	computed.Generated = rep.Generated
	computed.Window = rep.Window
	computed.State = "ok" // the source was read successfully; an empty queue is a legitimate zero, not could-not-check
	return finishDecisionLatency(computed, asJSON)
}

func finishDecisionLatency(rep bfDecisionLatencyReport, asJSON bool) int {
	if asJSON {
		return printBFJSON(rep)
	}
	if rep.State == "could-not-check" {
		fmt.Printf("decision latency -- %s ... %s: could-not-check\n", rep.Window.Since, rep.Window.Until)
		return 0
	}
	fmt.Printf("decision latency -- %s ... %s: %s, wip=%d, oldest=%.1fh (#%d)\n",
		rep.Window.Since, rep.Window.Until, renderTimingMetric(rep.Latency), rep.WIP, rep.OldestOpenAgeHours, rep.OldestOpenIssue)
	return 0
}
