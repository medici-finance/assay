package main

// roadmapdora.go — the GROUPED-DORA computation the roadmap pages consume.
// Rehomed out of the removed dora.go standalone --dora commodity surface so
// roadmap.go / roadmap_streampage.go / issues.go still build. These are the
// in-Go DORA tiles the roadmap renders today; an external metrics platform may
// feed them as a follow-up. humanDur lives in methmetrics.go.

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// doraAntiGamingNote is printed in --dora's header (text) and carried in the
// JSON `note` field. Stream-wide rule: these
// metrics are diagnostic, per-project, for continuous improvement — never a
// target, an individual scorecard, or a cross-team comparison.
const doraAntiGamingNote = "DORA metrics are DIAGNOSTIC, per-project, for continuous improvement — " +
	"never a target, an individual scorecard, or a cross-team comparison (Goodhart's law: a measure " +
	"that becomes a target ceases to be a good measure). A metric that starts driving perverse " +
	"behavior is itself a retro finding."

// Canonical metric keys — the JSON `metrics` object always carries exactly
// these five (Verify item 3). Stable identifiers for downstream consumers
// (the retro and --trend).
const (
	doraDeployFreq = "deployment_frequency"
	doraLeadTime   = "change_lead_time"
	doraRecovery   = "failed_deploy_recovery_time"
	doraChangeFail = "change_failure_rate"
	doraRework     = "rework_rate"
)

// defaultDoraWindowDays is the look-back when --since is omitted.
const defaultDoraWindowDays = 28

// DoraMetric is one of the five metrics. Value is a human-readable string (or
// "unknown"); Needs is non-empty ONLY when an input is not automatable and the
// value is therefore a placeholder — the anti-gaming contract. Computed is true
// when Value is derived from real data (even a partial slice), false when it is
// a placeholder.
type DoraMetric struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Family   string `json:"family"` // "throughput" | "instability"
	Value    string `json:"value"`
	Needs    string `json:"needs,omitempty"` // "verify-desk" | "manual" | "verify-desk|manual"
	Computed bool   `json:"computed"`
	Detail   string `json:"detail,omitempty"`
}

// doraPR is the slice of a merged pull request the lead-time / frequency
// computations need.
type doraPR struct {
	Number    int
	CreatedAt time.Time
	MergedAt  time.Time
}

// doraInputs is the pure input to computeDora: everything gathered from the
// history log + git + gh. Kept as data so the metric math is testable over a
// testdata history and fixture git/gh values with no exec/network.
type doraInputs struct {
	Since, Until, Now time.Time
	History           []HistoryEntry
	// HistoryPresent reports that the historian SOURCE (docs/streams/.history.jsonl)
	// exists and was read without error. False means absent (never wired) or
	// unreadable — a could-not-check, distinct from a present-but-quiet log that
	// simply had no transitions in the window. Change lead time reads this to tell
	// "checked, no data" from "could not check at all".
	HistoryPresent  bool
	Commits, Merges int
	GitOK           bool
	MergedPRs       []doraPR
	GHMergedOK      bool
	BugIssues       int
	GHBugsOK        bool
	Rework          reworkInput
}

// reworkInput is the review-derived rework signal over the window's merged PRs —
// the AUTOMATABLE #766(b) row-5 definition (CHANGES_REQUESTED count + re-review
// cycles per merged PR), which replaces the old post-merge-defect framing that
// was genuinely un-automatable. OK=false when the gh review sub-fetch could not
// run (offline): a could-not-check, degraded to a needs marker, never a
// fabricated zero.
type reworkInput struct {
	ChangesRequested int // total CHANGES_REQUESTED reviews across merged PRs in window
	ReReviewCycles   int // CHANGES_REQUESTED reviews followed by any later review (a re-look)
	PRsWithCR        int // merged PRs carrying >=1 CHANGES_REQUESTED
	OK               bool
}

// goalPriorityOrder is the fixed display order for per-goal DORA grouping
// (example-app first, example-service second, assay supporting).
// Any goal not listed here sorts after platform then alphabetically.
var goalPriorityOrder = []string{"example-app", "example-service", "assay", "platform"}

// doraSmallNThreshold is the minimum n for a group's median to be reported rather
// than suppressed with an n=<x> annotation (brief-26: groups with <5 events).
const doraSmallNThreshold = 5

// goalPriorityRank returns a sort key for the fixed goal priority order.
// Unlisted goals sort after the canonical set, then alphabetically.
func goalPriorityRank(goal string) int {
	for i, g := range goalPriorityOrder {
		if g == goal {
			return i
		}
	}
	return len(goalPriorityOrder)
}

// streamToGoal maps a stream name to its product goal via the stream README's
// serves: frontmatter. An absent/unset serves: maps to "untagged".
func streamToGoal(streams []*Stream) map[string]string {
	m := map[string]string{}
	for _, s := range streams {
		g := s.Serves
		if g == "" {
			g = "untagged"
		}
		m[s.Name] = g
	}
	return m
}

// briefDoneCounts returns, per group key, the count of briefs reaching "done"
// in [since, until] from the historian. The groupKey function maps a brief ID
// to its group key (stream name or goal).
func briefDoneCounts(history []HistoryEntry, since, until time.Time, groupKey func(string) string) map[string]int {
	done := map[string]bool{}  // brief ID -> reached done in window
	counts := map[string]int{} // group -> count
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if e.To == "done" && !ts.Before(since) && !ts.After(until) {
			if done[e.Brief] {
				continue
			}
			done[e.Brief] = true
			gk := groupKey(e.Brief)
			counts[gk]++
		}
	}
	return counts
}

// briefLeadTimes returns, per group key, the implemented->done durations for
// briefs whose done transition falls in [since, until]. The groupKey function
// maps a brief ID to its group key.
func briefLeadTimes(history []HistoryEntry, since, until time.Time, groupKey func(string) string) map[string][]time.Duration {
	firstImpl := map[string]time.Time{}
	out := map[string][]time.Duration{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		switch e.To {
		case "implemented":
			if _, seen := firstImpl[e.Brief]; !seen {
				firstImpl[e.Brief] = ts
			}
		case "done":
			if ts.Before(since) || ts.After(until) {
				continue
			}
			impl, seen := firstImpl[e.Brief]
			if !seen {
				continue
			}
			gk := groupKey(e.Brief)
			out[gk] = append(out[gk], ts.Sub(impl))
		}
	}
	return out
}

// briefReverts counts backward transitions (implemented/verified/done ->
// todo/in-progress/blocked) from the historian per group key as a change-failure
// proxy signal.
func briefReverts(history []HistoryEntry, since, until time.Time, groupKey func(string) string) map[string]int {
	revertFrom := map[string]bool{"implemented": true, "verified": true, "done": true}
	revertTo := map[string]bool{"todo": true, "in-progress": true, "blocked": true}
	out := map[string]int{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if ts.Before(since) || ts.After(until) {
			continue
		}
		if revertFrom[e.From] && revertTo[e.To] {
			gk := groupKey(e.Brief)
			out[gk]++
		}
	}
	return out
}

// findingsPerGroup counts unresolved findings whose Affects entries map to the
// given group key, as a change-failure proxy signal.
func findingsPerGroup(findings []Finding, groupKey func(string) string) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		for _, a := range f.Affects {
			gk := groupKey(a)
			out[gk]++
		}
	}
	return out
}

// p90Duration returns the 90th percentile of the durations.
func p90Duration(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)) * 0.9)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// medianDur returns the median of the durations.
func medianDur(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

// DoraGroup holds the four DORA metrics for one grouping dimension
// (a stream or a product goal).
type DoraGroup struct {
	Key     string                `json:"key"`
	Label   string                `json:"label"`
	N       int                   `json:"n"`
	SmallN  bool                  `json:"small_n"`
	Metrics map[string]DoraMetric `json:"metrics"`
}

// DoraGroupedReport is the full grouped-DORA output.
type DoraGroupedReport struct {
	Since      string      `json:"since"`
	Until      string      `json:"until"`
	Generated  string      `json:"generated"`
	Note       string      `json:"note"`
	By         string      `json:"by"`
	Groups     []DoraGroup `json:"groups"`
	GlobalMTTR DoraMetric  `json:"global_mttr"`
}

// groupKeyForBrief maps a brief ID (e.g. "example-app/01") to a group
// key. For "stream" mode this is the stream name; for "goal" mode this is the
// goal name (from the stream->goal map). An unrecognized brief ID gets "unknown".
func groupKeyForBrief(briefID string, by string, s2g map[string]string) string {
	parts := strings.SplitN(briefID, "/", 2)
	stream := parts[0]
	switch by {
	case "stream":
		return stream
	case "goal":
		if g, ok := s2g[stream]; ok {
			return g
		}
		return "untagged"
	}
	return "unknown"
}

// groupKeyForFindingAffects maps a finding Affects entry (e.g. "stream-name" or
// "stream-name/brief-01") to a group key. Strips any brief suffix.
func groupKeyForFindingAffects(affects string, by string, s2g map[string]string) string {
	stream := affects
	if idx := strings.Index(affects, "/"); idx >= 0 {
		stream = affects[:idx]
	}
	switch by {
	case "stream":
		return stream
	case "goal":
		if g, ok := s2g[stream]; ok {
			return g
		}
		return "untagged"
	}
	return "unknown"
}

// computeDoraGrouped computes the four per-group DORA metrics + global MTTR.
func computeDoraGrouped(in doraInputs, streams []*Stream, findings []Finding, by string) DoraGroupedReport {
	rep := DoraGroupedReport{
		Since:     in.Since.Format("2006-01-02"),
		Until:     in.Until.Format("2006-01-02"),
		Generated: in.Now.UTC().Format(time.RFC3339),
		Note:      doraAntiGamingNote,
		By:        by,
	}
	days := in.Until.Sub(in.Since).Hours() / 24
	if days < 1 {
		days = 1
	}
	weeks := days / 7
	if weeks < 1 {
		weeks = 1
	}

	s2g := streamToGoal(streams)

	gkB := func(briefID string) string { return groupKeyForBrief(briefID, by, s2g) }
	gkF := func(affects string) string { return groupKeyForFindingAffects(affects, by, s2g) }

	doneN := briefDoneCounts(in.History, in.Since, in.Until, gkB)
	lts := briefLeadTimes(in.History, in.Since, in.Until, gkB)
	reverts := briefReverts(in.History, in.Since, in.Until, gkB)
	findingsG := findingsPerGroup(findings, gkF)

	// Collect all group keys and sort them deterministically.
	allKeys := map[string]bool{}
	for k := range doneN {
		allKeys[k] = true
	}
	for k := range lts {
		allKeys[k] = true
	}
	for k := range reverts {
		allKeys[k] = true
	}
	for k := range findingsG {
		allKeys[k] = true
	}
	// Ensure every stream's key appears, even with zero data.
	for _, s := range streams {
		k := s.Name
		if by == "goal" {
			if g, ok := s2g[s.Name]; ok {
				k = g
			} else {
				k = "untagged"
			}
		}
		allKeys[k] = true
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		if by == "goal" {
			ri, rj := goalPriorityRank(sortedKeys[i]), goalPriorityRank(sortedKeys[j])
			if ri != rj {
				return ri < rj
			}
			if ri >= len(goalPriorityOrder) && rj >= len(goalPriorityOrder) {
				return sortedKeys[i] < sortedKeys[j]
			}
		}
		return sortedKeys[i] < sortedKeys[j]
	})

	for _, k := range sortedKeys {
		n := doneN[k]
		smallN := n > 0 && n < doraSmallNThreshold
		group := DoraGroup{
			Key:     k,
			Label:   groupLabel(k, by),
			N:       n,
			SmallN:  smallN,
			Metrics: map[string]DoraMetric{},
		}

		nNote := ""
		if smallN {
			nNote = fmt.Sprintf(" (n=%d)", n)
		}

		// 1. Deployment frequency -- briefs done per group / week (proxy).
		df := DoraMetric{
			Key: doraDeployFreq, Name: "Deployment frequency", Family: "throughput",
			Computed: n > 0,
		}
		if n > 0 {
			df.Value = fmt.Sprintf("%.1f briefs/week", float64(n)/weeks)
			df.Detail = fmt.Sprintf("%d brief(s) reaching done in window (proxy: brief-completion rate%s; aggregate deploy freq uses git commits)", n, nNote)
		} else {
			df.Value = "no briefs done in window"
			df.Detail = "brief-completion proxy -- no briefs reached done in this group during the window"
		}
		group.Metrics[doraDeployFreq] = df

		// 2. Change lead time -- implemented->done median + p90.
		lt := DoraMetric{
			Key: doraLeadTime, Name: "Change lead time", Family: "throughput",
		}
		if durs, ok := lts[k]; ok && len(durs) > 0 {
			lt.Computed = true
			med := medianDur(durs)
			p90 := p90Duration(durs)
			lt.Value = fmt.Sprintf("%s / %s", humanDur(med), humanDur(p90))
			detail := fmt.Sprintf("median/p90 implemented->done over %d brief(s)%s", len(durs), nNote)
			if smallN && len(durs) > 0 {
				detail += " [small-n: a median over <5 briefs is an anecdote, not a metric]"
			}
			lt.Detail = detail
		} else {
			lt.Value = "unknown"
			lt.Detail = "no implemented->done transitions recorded for this group in window"
		}
		group.Metrics[doraLeadTime] = lt

		// 3. Change-failure proxy -- (findings + reverts) / done briefs.
		cf := DoraMetric{
			Key: doraChangeFail, Name: "Change failure rate", Family: "instability",
		}
		fCount := findingsG[k]
		rCount := reverts[k]
		if n > 0 && (fCount > 0 || rCount > 0) {
			rate := float64(fCount+rCount) / float64(n)
			cf.Computed = true
			cf.Value = fmt.Sprintf("%.0f%% (proxy)", rate*100)
			cf.Detail = fmt.Sprintf("%d finding(s) + %d revert(s) / %d done brief(s) = %.0f%% (proxy: findings/reverts over done briefs)%s",
				fCount, rCount, n, rate*100, nNote)
		} else if n == 0 {
			cf.Value = "unknown"
			cf.Detail = "no briefs done in window -- cannot compute a rate"
		} else {
			cf.Value = "0% (proxy)"
			cf.Detail = fmt.Sprintf("0 findings/reverts / %d done brief(s) = 0%% (proxy)%s", n, nNote)
		}
		group.Metrics[doraChangeFail] = cf

		// 4. Rework rate -- NOT automatable (same as the aggregate).
		group.Metrics[doraRework] = DoraMetric{
			Key: doraRework, Name: "Rework rate", Family: "instability",
			Value: "unknown", Needs: "verify-desk|manual", Computed: false,
			Detail: fmt.Sprintf("follow-up briefs/bugs from post-merge defects / merged requires post-merge-defect classification (verify-desk/manual)%s", nNote),
		}

		rep.Groups = append(rep.Groups, group)
	}

	// Global MTTR -- stays global-only in v1 (brief-26).
	rep.GlobalMTTR = DoraMetric{
		Key: doraRecovery, Name: "Failed-deploy recovery time (global)", Family: "throughput",
		Value: "unknown", Needs: "verify-desk|manual", Computed: false,
		Detail: "MTTR stays global-only in v1 -- per-group attribution is noisy; broken-main->green duration is not automated. Supply from verify-desk incident records or manual timing.",
	}

	return rep
}

// groupLabel returns a human-readable label for a group key.
func groupLabel(key, by string) string {
	if by == "goal" && key == "untagged" {
		return "untagged (no serves: tag)"
	}
	return key
}
