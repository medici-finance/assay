package main

// --assayscore (statusgen/08) — the composite AssayScore: the single
// comparable number rolled up from the brief-flow metrics (statusgen/07).
//
// The formula is FIXED by the settled metric-definitions spec (this brief does
// NOT invent an alternative — it implements the settled one). Recorded in the
// brief-08 facts, the composite is the GEOMETRIC MEAN of four 0–100
// sub-scores — Speed, Flow, Quality, Value:
//
//	AssayScore = (Speed · Flow · Quality · Value) ^ (1/4)
//
// The geometric mean PENALIZES imbalance: a near-zero factor drags the whole
// product toward zero, so a team cannot ace one dimension by tanking another.
// That anti-gaming intent is the reason the aggregation is geometric, not
// arithmetic — it is expressed in the aggregation function itself.
//
// TRANSPARENCY IS THE PRODUCT. The score is NEVER emitted bare: it always ships
// together with (1) the four sub-scores, (2) the raw inputs behind each, and
// (3) the baseline_window each self-relative band resolved to. A wrong
// normalization is then visibly inconsistent with its published parts — the
// SPOF mitigation for a derived summary (brief-08 Context).
//
// THREE-STATE, never a fabricated zero. A dimension whose input is thin/absent
// reports could-not-check and is EXCLUDED from the geometric mean — never
// coerced to 0 (a zero factor would zero the whole score and lie). The
// composite is then the geometric mean over the k AVAILABLE dimensions,
// (∏ available)^(1/k), with `incomplete: true` and the missing dimension(s)
// named. This is the exact contract brief-07 emits and brief-08 computes.
//
// REUSE, never fork: every input is one of the statusgen/07 brief-flow metrics
// already in this package — lead time (briefflow.go), flow efficiency
// (briefefficiency.go), first-pass yield (briefflowreview.go), weighted
// throughput (briefflow.go), decision latency (briefdecision.go). This file
// composes them; it re-implements none of them.

import (
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

// assayBaselineMinObs is the baseline guard: an unbounded metric's trailing
// -90-day reference band is computed from p10/p90 of its own observations, but
// ONLY when at least this many observations exist. Below it, the tool declines
// to invent a band from noise and the dimension reports could-not-check
// (three-state discipline). The settled spec fixes this floor at 5.
const assayBaselineMinObs = 5

// assayBaselineDays is the trailing window the self-relative reference bands
// are computed over — "the org's own trailing-90-day baseline" (settled spec).
// Self-relative because cross-org benchmarks do not exist yet: the score says
// "better or worse than this org's own recent self", stated plainly as a
// limitation and never implying an industry benchmark.
const assayBaselineDays = 90

// assayValueWeekDays buckets the trailing-90-day Value baseline into weekly
// observations — the band is p10/p90 of the trailing-90-day WEEKLY V_raw
// (settled spec), so ~13 weekly points feed the guard above.
const assayValueWeekDays = 7

// assayDimOrder fixes the four dimensions' iteration order so the geometric
// mean, the `missing` list, and any diagnostic output are deterministic.
var assayDimOrder = []string{"speed", "flow", "quality", "value"}

// clamp01 clamps to [0,1] — the settled spec's `clamp(x, 0, 1)` on the
// normalized position within a reference band.
func clamp01(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
}

// assaySubScore is one dimension's resolved 0–100 sub-score, or its
// could-not-check state. Score is a pointer so a could-not-check dimension
// serializes as an explicit `"score": null` (never a fabricated 0) while an
// available dimension carries its number.
type assaySubScore struct {
	Score  *float64 `json:"score"`            // 0–100 when State=="ok"; null otherwise
	State  string   `json:"state"`            // "ok" | "could-not-check"
	Reason string   `json:"reason,omitempty"` // why, when could-not-check
}

func assayCNC(reason string) assaySubScore {
	return assaySubScore{State: "could-not-check", Reason: reason}
}

func assayOK(score float64) assaySubScore {
	s := round1(score)
	return assaySubScore{Score: &s, State: "ok"}
}

// assayBand is a trailing-90-day reference band [p10, p90] for an unbounded
// metric, plus its observation count and state. State is could-not-check when
// the baseline guard trips (fewer than assayBaselineMinObs observations) — the
// tool declines to invent a band from noise.
type assayBand struct {
	Lo    float64 `json:"band_lo,omitempty"`
	Hi    float64 `json:"band_hi,omitempty"`
	N     int     `json:"n"`
	State string  `json:"state"` // "ok" | "could-not-check"
}

// computeAssayBand builds the [p10, p90] reference band from an observation
// slice, enforcing the baseline guard. Percentiles (reused pctlDays, the
// nearest-rank helper the brief-flow metrics already use) mean a single
// outlier cannot move the band.
func computeAssayBand(obs []float64) assayBand {
	if len(obs) < assayBaselineMinObs {
		return assayBand{N: len(obs), State: "could-not-check"}
	}
	return assayBand{
		Lo:    pctlDays(obs, 0.10),
		Hi:    pctlDays(obs, 0.90),
		N:     len(obs),
		State: "ok",
	}
}

// boundedSubScore maps an already-bounded ratio (Flow's FE, Quality's FPY —
// both in [0,1]) directly onto 0–100: `ratio × 100`. No baseline is needed
// (the settled spec: "two of the four are already bounded ratios and map
// directly").
func boundedSubScore(ratio float64, state, dim string) assaySubScore {
	if state != "ok" {
		return assayCNC(dim + " primitive could-not-check")
	}
	return assayOK(clamp01(ratio) * 100)
}

// speedSubScore inverse-normalizes lead time (lower is better) against its
// trailing-90-day band:
//
//	Speed = 100 × clamp( (band_hi − L) / (band_hi − band_lo), 0, 1 )
//
// A lead time at the fast end of the band → ~100; at the slow end → ~0.
func speedSubScore(leadMedian float64, leadState string, band assayBand) assaySubScore {
	if leadState != "ok" {
		return assayCNC("speed lead-time could-not-check")
	}
	if band.State != "ok" {
		return assayCNC(fmt.Sprintf("speed baseline guard: %d trailing-90d observations (<%d)", band.N, assayBaselineMinObs))
	}
	denom := band.Hi - band.Lo
	if denom <= 0 {
		return assayCNC("speed baseline band is degenerate (band_hi == band_lo)")
	}
	return assayOK(100 * clamp01((band.Hi-leadMedian)/denom))
}

// valueSubScore forward-normalizes V_raw (higher is better) against its
// trailing-90-day weekly band:
//
//	Value = 100 × clamp( (V_raw − band_lo) / (band_hi − band_lo), 0, 1 )
func valueSubScore(vRaw float64, vState string, band assayBand) assaySubScore {
	if vState != "ok" {
		return assayCNC("value V_raw could-not-check")
	}
	if band.State != "ok" {
		return assayCNC(fmt.Sprintf("value baseline guard: %d trailing-90d weekly observations (<%d)", band.N, assayBaselineMinObs))
	}
	denom := band.Hi - band.Lo
	if denom <= 0 {
		return assayCNC("value baseline band is degenerate (band_hi == band_lo)")
	}
	return assayOK(100 * clamp01((vRaw-band.Lo)/denom))
}

// assayComposite is the geometric mean over the AVAILABLE (state=="ok")
// sub-scores. A could-not-check dimension is EXCLUDED (never coerced to 0) and
// named in `missing`, with `incomplete` set. A legitimately-zero sub-score
// (e.g. Speed at/beyond the slow end of its band) IS included — that is the
// anti-gaming behaviour, not a missing dimension. Top-level state is
// could-not-check only when NO dimension was available.
func assayComposite(subs map[string]assaySubScore) (score *float64, incomplete bool, missing []string, state string) {
	prod := 1.0
	k := 0
	for _, dim := range assayDimOrder {
		ss := subs[dim]
		if ss.State != "ok" || ss.Score == nil {
			incomplete = true
			missing = append(missing, dim)
			continue
		}
		prod *= *ss.Score
		k++
	}
	if k == 0 {
		return nil, true, missing, "could-not-check"
	}
	g := round1(math.Pow(prod, 1.0/float64(k)))
	return &g, incomplete, missing, "ok"
}

// --- JSON shape --------------------------------------------------------------

// assaySpeedInput is the raw material behind the Speed sub-score, emitted for
// transparency (a reader sees exactly what "self-relative" resolved to).
type assaySpeedInput struct {
	MedianLeadDays float64 `json:"median_lead_days,omitempty"`
	N              int     `json:"n"`
	State          string  `json:"state"`
}

type assayFlowInput struct {
	Efficiency float64 `json:"efficiency,omitempty"` // FE, 0..1
	State      string  `json:"state"`
}

type assayQualityInput struct {
	FirstPass int     `json:"first_pass"`
	N         int     `json:"n"`
	Yield     float64 `json:"yield,omitempty"` // FPY, 0..1
	State     string  `json:"state"`
}

type assayValueInput struct {
	ThroughputPoints float64 `json:"throughput_points"`
	DecisionHours    float64 `json:"decision_hours,omitempty"`
	VRaw             float64 `json:"v_raw,omitempty"`
	State            string  `json:"state"`
	// Note documents that the human-decision-hours denominator currently uses
	// the decision-queue-dwell term (the feasible, git/forge-derived one); the
	// review-latency and gate-touch terms named by the spec are future
	// substrate, the same proxy discipline Flow ships under.
	Note string `json:"note,omitempty"`
}

type assayInputs struct {
	Speed   assaySpeedInput   `json:"speed"`
	Flow    assayFlowInput    `json:"flow"`
	Quality assayQualityInput `json:"quality"`
	Value   assayValueInput   `json:"value"`
}

// assayBaselineWindow is the exact date range plus each unbounded dimension's
// resolved band — emitted so "self-relative" is auditable.
type assayBaselineWindow struct {
	Since string    `json:"since"`
	Until string    `json:"until"`
	Speed assayBand `json:"speed"`
	Value assayBand `json:"value"`
}

// assayScoreReport is the emitted document:
// {score, subscores:{speed,flow,quality,value}, inputs:{...}, baseline_window,
//  state, incomplete} — exactly the shape brief-07 declared and brief-08 fills.
type assayScoreReport struct {
	Generated      string                   `json:"generated"`
	Window         doraTimingWindow         `json:"window"`
	State          string                   `json:"state"`
	Score          *float64                 `json:"score"`
	Incomplete     bool                     `json:"incomplete"`
	Missing        []string                 `json:"missing,omitempty"`
	Subscores      map[string]assaySubScore `json:"subscores"`
	Inputs         assayInputs              `json:"inputs"`
	BaselineWindow assayBaselineWindow      `json:"baseline_window"`
}

// --- wiring ------------------------------------------------------------------

// leadTimeObservations returns every authored->done lead time (in days) whose
// `to:"done"` transition lands in [since, until) — the raw per-brief sample the
// Speed median and its trailing-90-day band are both built from. Same
// resolution rules as computeLeadTimeBySize (briefflow.go): a done event whose
// brief has no resolvable authored date/effort is excluded, never guessed.
func leadTimeObservations(info map[string]authoredBriefInfo, history []HistoryEntry, since, until time.Time) []float64 {
	var days []float64
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
		d := doneAt.Sub(bi.Authored).Hours() / 24
		if d < 0 {
			d = 0 // date-precision skew guard — never a negative lead time
		}
		days = append(days, d)
	}
	return days
}

// throughputPointsInWindow sums authored-brief effort-points of `to:"done"`
// transitions in [since, until) — the Value numerator (weighted throughput),
// issue-loop segmented out, reusing computeThroughput's Authored segment.
func throughputPointsInWindow(streams []*Stream, history []HistoryEntry, since, until time.Time) float64 {
	return computeThroughput(streams, history, since, until).Authored.Points
}

// decisionHoursInWindow sums the resolution latency (hours) of every decision
// closed in [since, until) — the decision-queue-dwell term of the Value
// denominator. Windowed on the close instant, matching computeDecisionLatency.
func decisionHoursInWindow(closed []dqIssue, since, until time.Time) (hours float64, n int) {
	for _, is := range closed {
		if is.ClosedAt == "" || !inWindow(is.ClosedAt, since, until) {
			continue
		}
		cAt, err1 := time.Parse(time.RFC3339, is.CreatedAt)
		clAt, err2 := time.Parse(time.RFC3339, is.ClosedAt)
		if err1 != nil || err2 != nil {
			continue
		}
		h := clAt.Sub(cAt).Hours()
		if h < 0 {
			continue
		}
		hours += h
		n++
	}
	return hours, n
}

// weeklyValueObservations builds the trailing-90-day WEEKLY V_raw series the
// Value band is computed from: for each 7-day bucket back from `until`, weekly
// throughput points ÷ weekly decision-hours. A week with no decision-hours
// cannot form a ratio and is skipped (never a divide-by-zero or a fabricated
// value). The resulting slice feeds computeAssayBand's p10/p90 + guard.
func weeklyValueObservations(streams []*Stream, history []HistoryEntry, closed []dqIssue, until time.Time) []float64 {
	var obs []float64
	for wkEnd := until; wkEnd.After(until.AddDate(0, 0, -assayBaselineDays)); wkEnd = wkEnd.AddDate(0, 0, -assayValueWeekDays) {
		wkStart := wkEnd.AddDate(0, 0, -assayValueWeekDays)
		pts := throughputPointsInWindow(streams, history, wkStart, wkEnd)
		hrs, _ := decisionHoursInWindow(closed, wkStart, wkEnd)
		if hrs <= 0 {
			continue
		}
		obs = append(obs, pts/hrs)
	}
	return obs
}

// gatherQuality assembles the first-pass-yield inputs the same way
// runFirstPassYield does (briefflowreview.go) and returns its report. Offline —
// no reachable target repo, or gh unavailable — it degrades to could-not-check
// rather than erroring, exactly like the standalone --first-pass-yield mode.
func gatherQuality(streams []*Stream, findings []Finding, history []HistoryEntry, root string, since, until time.Time, src bfPRSource) bfFirstPassReport {
	evidenceByID := map[string]string{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			evidenceByID[s.Name+"/"+b.Num] = b.Evidence
		}
	}
	var doneIDs []string
	for _, e := range history {
		if e.To == "done" && inWindow(e.Ts, since, until) {
			doneIDs = append(doneIDs, e.Brief)
		}
	}
	repo := bfResolveTarget("assayscore/quality", root)
	if repo == "" {
		return bfFirstPassReport{State: "could-not-check"}
	}
	prs, perr := src.MergedPRs(repo, since)
	if perr != nil {
		return bfFirstPassReport{State: "could-not-check"}
	}
	byBrief, unlinked := resolvePRsByBrief(prs)
	return computeFirstPassYield(doneIDs, byBrief, unlinked,
		func(pr int) ([]ghReview, error) { return src.Reviews(repo, pr) }, evidenceByID, findings)
}

// computeAssayScore is the pure composition: given each dimension's resolved
// sub-score plus the raw inputs and bands, it assembles the report. Kept
// separate from the data-gathering (runAssayScore) so the golden test can
// dereference the FORMULA against a known-input fixture with no network or
// filesystem.
func computeAssayScore(
	speed, flow, quality, value assaySubScore,
	inputs assayInputs,
	baseline assayBaselineWindow,
) assayScoreReport {
	subs := map[string]assaySubScore{
		"speed": speed, "flow": flow, "quality": quality, "value": value,
	}
	score, incomplete, missing, state := assayComposite(subs)
	sort.Strings(missing)
	return assayScoreReport{
		State:          state,
		Score:          score,
		Incomplete:     incomplete,
		Missing:        missing,
		Subscores:      subs,
		Inputs:         inputs,
		BaselineWindow: baseline,
	}
}

// runAssayScore gathers the four brief-flow inputs, normalizes each per the
// settled spec, and emits the composite. prSrc/dqSrc are the same network seams
// the standalone brief-flow modes use, injected for testability; offline they
// degrade the network-fed dimensions (Quality, Value) to could-not-check.
func runAssayScore(root, since, until string, asJSON bool, prSrc bfPRSource, dqSrc decisionQueueSource) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	streams, findings, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: assayscore:", err)
		return 1
	}
	history, herr := LoadHistory(historyAbsPath(root))
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: assayscore:", herr)
		return 1
	}

	baselineSince := untilT.AddDate(0, 0, -assayBaselineDays)

	// --- Speed: lead-time median (window) inverse-normalized vs the
	//     trailing-90-day band of per-brief lead times.
	authoredInfo := loadAuthoredBriefInfo(streams)
	windowLeads := leadTimeObservations(authoredInfo, history, sinceT, untilT)
	speedBand := computeAssayBand(leadTimeObservations(authoredInfo, history, baselineSince, untilT))
	speedInput := assaySpeedInput{N: len(windowLeads), State: "could-not-check"}
	var speedSub assaySubScore
	if len(windowLeads) == 0 {
		speedSub = assayCNC("speed lead-time could-not-check")
	} else {
		med := medianDays(windowLeads)
		speedInput.MedianLeadDays = med
		speedInput.State = "ok"
		speedSub = speedSubScore(med, "ok", speedBand)
	}

	// --- Flow: flow efficiency × 100 (already a bounded ratio).
	fe := computeFlowEfficiency(history, sinceT, untilT)
	flowInput := assayFlowInput{Efficiency: fe.Efficiency, State: fe.State}
	flowSub := boundedSubScore(fe.Efficiency, fe.State, "flow")

	// --- Quality: first-pass yield × 100 (already a bounded ratio).
	fpy := gatherQuality(streams, findings, history, root, sinceT, untilT, prSrc)
	qualityInput := assayQualityInput{FirstPass: fpy.FirstPass, N: fpy.N, State: fpy.State}
	var qualitySub assaySubScore
	if fpy.State == "ok" && fpy.N > 0 {
		ratio := float64(fpy.FirstPass) / float64(fpy.N)
		qualityInput.Yield = round1(ratio*1000) / 1000
		qualitySub = boundedSubScore(ratio, "ok", "quality")
	} else {
		qualitySub = assayCNC("quality first-pass-yield could-not-check")
	}

	// --- Value: weighted throughput ÷ human-decision-hours, forward-normalized
	//     vs the trailing-90-day weekly band. Denominator = the decision-queue
	//     -dwell term (feasible today); review-latency + gate-touch terms are
	//     future substrate (documented in the emitted note).
	valuePoints := throughputPointsInWindow(streams, history, sinceT, untilT)
	valueInput := assayValueInput{
		ThroughputPoints: valuePoints,
		State:            "could-not-check",
		Note:             "denominator = decision-queue dwell; review-latency + gate-touch terms are future substrate",
	}
	valueBand := assayBand{State: "could-not-check"}
	var valueSub assaySubScore
	closed, dqErr := valueDecisions(root, dqSrc)
	switch {
	case dqErr != nil:
		valueSub = assayCNC("value V_raw could-not-check (decision queue unreadable)")
	default:
		hrs, _ := decisionHoursInWindow(closed, sinceT, untilT)
		if hrs <= 0 {
			valueSub = assayCNC("value V_raw could-not-check (no in-window decision-hours)")
			break
		}
		vRaw := valuePoints / hrs
		valueInput.DecisionHours = round1(hrs)
		valueInput.VRaw = round1(vRaw*1000) / 1000
		valueInput.State = "ok"
		valueBand = computeAssayBand(weeklyValueObservations(streams, history, closed, untilT))
		valueSub = valueSubScore(vRaw, "ok", valueBand)
	}

	rep := computeAssayScore(speedSub, flowSub, qualitySub, valueSub,
		assayInputs{Speed: speedInput, Flow: flowInput, Quality: qualityInput, Value: valueInput},
		assayBaselineWindow{
			Since: baselineSince.UTC().Format(time.RFC3339),
			Until: untilT.UTC().Format(time.RFC3339),
			Speed: speedBand,
			Value: valueBand,
		})
	rep.Generated = now.UTC().Format(time.RFC3339)
	rep.Window = bfWindowJSON(sinceT, untilT)

	if asJSON {
		return printBFJSON(rep)
	}
	return printAssayScoreText(rep)
}

// valueDecisions fetches the closed decision-queue issues the Value denominator
// and its weekly band read. Returns an error only when the source could not be
// read (offline / gh failure) — an empty-but-read queue is a legitimate zero,
// surfaced downstream as could-not-check for the ratio (no hours to divide by).
func valueDecisions(root string, dqSrc decisionQueueSource) ([]dqIssue, error) {
	repo := bfResolveTarget("assayscore/value", root)
	if repo == "" {
		return nil, fmt.Errorf("no target repo resolved")
	}
	return dqSrc.Issues(repo, decisionLabel, "closed")
}

// printAssayScoreText renders the human-readable form — the composite, then
// each sub-score with its state, so a terminal reader sees the same
// parts-and-whole transparency the JSON carries.
func printAssayScoreText(rep assayScoreReport) int {
	if rep.Score == nil {
		fmt.Printf("AssayScore -- %s ... %s: could-not-check (no dimension available)\n", rep.Window.Since, rep.Window.Until)
	} else {
		flag := ""
		if rep.Incomplete {
			flag = fmt.Sprintf(" (incomplete; missing: %v)", rep.Missing)
		}
		fmt.Printf("AssayScore -- %s ... %s: %.1f%s\n", rep.Window.Since, rep.Window.Until, *rep.Score, flag)
	}
	for _, dim := range assayDimOrder {
		ss := rep.Subscores[dim]
		if ss.State == "ok" && ss.Score != nil {
			fmt.Printf("  %-7s %.1f\n", dim, *ss.Score)
		} else {
			fmt.Printf("  %-7s could-not-check (%s)\n", dim, ss.Reason)
		}
	}
	return 0
}
