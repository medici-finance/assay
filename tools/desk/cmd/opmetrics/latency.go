package main

// latency.go — decision latency and merged-PR count, from a `gh` JSON export.
//
// WHY THIS TOOL DOES NOT CALL `gh` ITSELF. A metrics collector that shells out is a
// collector whose tests either hit the network or exercise a different code path than
// production. Both are how a green suite ships a broken number. opmetrics is instead a
// pure function of files: the operator's routine runs the documented `gh` command,
// redirects it to a file, and passes it in. The SAME path runs in the tests.
//
// The documented producer (tools/desk/README.md carries it too):
//
//	gh pr list --repo <owner>/<repo> --state merged --limit 200 \
//	   --search "merged:<YYYY-MM-DD>" \
//	   --json number,createdAt,mergedAt,labels > /tmp/opm-gh.json
//
// WHAT THIS CAN AND CANNOT MEASURE. The brief asks for "ready-flip or needs-decision
// label → merge". `gh pr list --json` exposes neither the ready-for-review event nor
// the label-application time; both need a GraphQL timeline query. So the reader
// accepts OPTIONAL `readyAt` / `decisionRequestedAt` fields and, when they are absent,
// falls back to `createdAt` — and RECORDS WHICH BASIS EACH SAMPLE USED in the emit.
// A latency computed from createdAt is an over-estimate of decision latency (it
// includes the whole drafting period), so a day whose samples are all createdAt-based
// is an upper bound, and the emitted basis counts are what tell a reader that.

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

type ghLabel struct {
	Name string `json:"name"`
}

type ghPR struct {
	Number    int       `json:"number"`
	CreatedAt string    `json:"createdAt"`
	MergedAt  string    `json:"mergedAt"`
	Labels    []ghLabel `json:"labels"`
	// Optional enrichments a GraphQL timeline query can supply. Absent in plain
	// `gh pr list --json` output.
	ReadyAt            string `json:"readyAt"`
	DecisionRequestedA string `json:"decisionRequestedAt"`
}

// LatencyRead is the aggregate of one pass over the gh export.
type LatencyRead struct {
	// MergedOnDate is merged PRs whose mergedAt falls on the reported day. It is the
	// denominator of the intervention rate.
	MergedOnDate int
	// Samples is merged PRs for which a start instant was resolvable.
	Samples          int
	P50Minutes       int
	P90Minutes       int
	BasisReady       int
	BasisDecisionReq int
	BasisCreated     int
	Unparseable      int
}

// ReadGH parses the gh export and computes the day's merged count and latency
// percentiles. An unreadable or malformed file is an error — the caller reports
// could-not-check. Zero merged PRs on a readable file is a real zero.
func ReadGH(path string, day time.Time) (LatencyRead, error) {
	var out LatencyRead
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, fmt.Errorf("gh json %s is not readable: %w", path, err)
	}
	var prs []ghPR
	if jerr := json.Unmarshal(raw, &prs); jerr != nil {
		return out, fmt.Errorf("gh json %s is not a JSON array of PRs: %w", path, jerr)
	}

	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	var mins []float64
	for _, p := range prs {
		merged, terr := time.Parse(time.RFC3339, p.MergedAt)
		if terr != nil {
			// A record with no parseable mergedAt is not a merge this tool can
			// attribute to a day. Counted, never guessed.
			out.Unparseable++
			continue
		}
		merged = merged.In(day.Location())
		if merged.Before(dayStart) || !merged.Before(dayEnd) {
			continue
		}
		out.MergedOnDate++

		start, basis, ok := latencyStart(p, day.Location())
		if !ok {
			continue
		}
		d := merged.Sub(start)
		if d < 0 {
			// A start after the merge is a broken record, not a negative latency.
			out.Unparseable++
			continue
		}
		switch basis {
		case "ready":
			out.BasisReady++
		case "decision":
			out.BasisDecisionReq++
		default:
			out.BasisCreated++
		}
		mins = append(mins, d.Minutes())
	}

	out.Samples = len(mins)
	if len(mins) > 0 {
		sort.Float64s(mins)
		out.P50Minutes = int(math.Round(percentile(mins, 50)))
		out.P90Minutes = int(math.Round(percentile(mins, 90)))
	}
	return out, nil
}

// latencyStart resolves the instant the clock starts, preferring the most specific
// signal available. The returned basis is one of "ready", "decision", "created".
func latencyStart(p ghPR, loc *time.Location) (time.Time, string, bool) {
	if t, err := time.Parse(time.RFC3339, p.ReadyAt); err == nil {
		return t.In(loc), "ready", true
	}
	if t, err := time.Parse(time.RFC3339, p.DecisionRequestedA); err == nil {
		return t.In(loc), "decision", true
	}
	if t, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil {
		return t.In(loc), "created", true
	}
	return time.Time{}, "", false
}

// percentile is NEAREST-RANK on an ascending slice: P_k is element ceil(k/100 * n).
// Nearest-rank is chosen over interpolation because these are small daily samples —
// often three or four PRs — where an interpolated "P90" invents a value that no PR
// ever had. Nearest-rank always names a real observation.
func percentile(sorted []float64, k float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(k / 100 * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}
