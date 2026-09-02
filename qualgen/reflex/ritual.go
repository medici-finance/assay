package reflex

import (
	"sort"

	"github.com/medici-finance/assay/qualgen/attribution"
)

// ritual.go implements task item 2: the ritual-effectiveness natural-
// experiment joins (spec §7.2). A fleet generates variation (rich vs
// minimal Verify tables, strong vs default model tier, differing lane
// coverage); this file joins those authoring attributes to downstream
// M1/M2/M3 outcomes already recorded elsewhere — no new mining. The two
// headline joins (cost per durable KLOC by model tier × brittleness band,
// and Verify-depth vs escape rate) are causal-shaped claims and MUST be
// routed through stratify.EmitRitual before they reach an artifact; the
// three industry-named agent metrics are direct corpus-wide proportions
// (spec §7.2, "published under the industry's own names for comparability")
// and are emitted as alarmed budgets instead (never a bare number).

// AuthoredChange is one authoring-attribute join row (task item 2's join
// key): the change's model tier, its brief's Verify-table depth (executable
// row count), lane coverage, and brittleness band, joined to its M1
// durable-change outcome, its authoring cost, and its M2/M3 defect outcome.
// Every field is supplied by the caller, translated from already-recorded
// artifacts — the brief's own frontmatter/exec-tier, the M1 durable-volume
// join (qualgen/dorajoin.DurableVolume), a spend-tracking feed, and the
// attribution ledger (via ResolveEscaped, below) — this package performs no
// mining of its own.
type AuthoredChange struct {
	ChangeID        string
	ModelTier       string
	VerifyDepth     int
	LaneCoverage    []string
	BrittlenessBand BrittlenessBand

	// DurableKLOC is the change's durable-change volume (spec §8's quality
	// denominator) in thousands of lines.
	DurableKLOC Measure[float64]
	// CostUSD is the change's already-recorded authoring cost.
	CostUSD Measure[float64]

	// Escaped is Measured(true) when M3's ledger attributes a defect to
	// this change (any stage), Measured(false) when the corpus was checked
	// and found none, could-not-measure when the join could not resolve it
	// (see ResolveEscaped).
	Escaped Measure[bool]

	// The industry-named agent-metrics inputs (spec §7.2). Each is
	// independently three-state so a change missing one signal still
	// contributes to the others.
	SurvivedNDays       Measure[bool]
	FirstPassApproved   Measure[bool]
	MergedWithoutReview Measure[bool]
	TimeInReviewHours   Measure[float64]
}

// ResolveEscaped determines an authored change's Escaped outcome by wiring
// the brief-10 ledger seam directly (task item 4): a change with no
// associated defect id had zero traced defects (Measured(false)); a change
// naming a defect id is looked up via ReviewEscapeJoin (gateyield.go) — an
// entry present on the ledger (any Lanes, even empty) means the defect IS
// recorded, so Escaped is Measured(true); a defect id absent from the ledger
// index (traced at M2, not yet attributed at M3) is could-not-measure, never
// a guessed false. index is built once per run via BuildLedgerIndex.
func ResolveEscaped(defectID string, index map[string]attribution.LedgerEntry) Measure[bool] {
	if defectID == "" {
		return Measured(false)
	}
	lanes := ReviewEscapeJoin(defectID, index)
	if !lanes.IsMeasured() {
		return CouldNotMeasure[bool](lanes.Reason)
	}
	return Measured(true)
}

// CostPerKLOCReadout is one (model tier × brittleness band) stratum of the
// cost-per-durable-KLOC ritual join (spec §7.2 headline #1).
type CostPerKLOCReadout struct {
	ModelTier   string           `json:"model_tier"`
	Band        BrittlenessBand  `json:"brittleness_band"`
	CostPerKLOC Measure[float64] `json:"cost_per_durable_kloc"`
	// SampleSize is the count of changes in this stratum REGARDLESS of
	// whether cost/KLOC was individually measured for each — reported for
	// honesty even when CostPerKLOC itself is could-not-measure.
	SampleSize int `json:"sample_size"`
}

// ComputeCostPerDurableKLOC groups changes by (ModelTier, BrittlenessBand)
// and computes cost per durable KLOC in each stratum: sum(CostUSD) /
// sum(DurableKLOC) over the changes in that stratum whose components are
// individually measured. A change missing either component is excluded from
// the sums (but still counted in SampleSize) — this is a CORPUS aggregation,
// not a single atomic measurement, so one change's missing cost does not
// blank the whole stratum the way qualgen's own atomic quality-denominator
// formula does (dorajoin.ComputeDurableVolume); the exclusion is instead
// visible via SampleSize vs the measured contribution count. A stratum with
// no measured contribution at all, or with a zero measured durable-KLOC
// denominator, reports could-not-measure rather than a division by zero.
// Output is a fixed, sorted order (tier then band) for a diffable readout.
func ComputeCostPerDurableKLOC(changes []AuthoredChange) []CostPerKLOCReadout {
	type key struct {
		tier string
		band BrittlenessBand
	}
	type accum struct {
		costSum float64
		klocSum float64
		n       int
	}
	groups := map[key]*accum{}
	sizes := map[key]int{}
	for _, c := range changes {
		k := key{c.ModelTier, c.BrittlenessBand}
		sizes[k]++
		if !c.CostUSD.IsMeasured() || !c.DurableKLOC.IsMeasured() {
			continue
		}
		a, ok := groups[k]
		if !ok {
			a = &accum{}
			groups[k] = a
		}
		a.costSum += c.CostUSD.Value
		a.klocSum += c.DurableKLOC.Value
		a.n++
	}

	keys := make([]key, 0, len(sizes))
	for k := range sizes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tier != keys[j].tier {
			return keys[i].tier < keys[j].tier
		}
		return keys[i].band < keys[j].band
	})

	out := make([]CostPerKLOCReadout, 0, len(keys))
	for _, k := range keys {
		a := groups[k]
		var cost Measure[float64]
		switch {
		case a == nil || a.n == 0:
			cost = CouldNotMeasure[float64]("no change in this model-tier x brittleness-band stratum has both cost and durable-KLOC measured")
		case a.klocSum == 0:
			cost = CouldNotMeasure[float64]("durable-KLOC summed to zero in this stratum — cost-per-KLOC is undefined, not infinite")
		default:
			cost = Measured(a.costSum / a.klocSum)
		}
		out = append(out, CostPerKLOCReadout{
			ModelTier:   k.tier,
			Band:        k.band,
			CostPerKLOC: cost,
			SampleSize:  sizes[k],
		})
	}
	return out
}

// VerifyDepthEscapeReadout is one (Verify-depth bucket × brittleness band)
// stratum of the Verify-depth-vs-escape-rate ritual join (spec §7.2 headline
// #2).
type VerifyDepthEscapeReadout struct {
	DepthBucket string           `json:"verify_depth_bucket"`
	Band        BrittlenessBand  `json:"brittleness_band"`
	EscapeRate  Measure[float64] `json:"escape_rate"`
	Measured    int              `json:"measured_changes"`
	Unmeasured  int              `json:"unmeasured_changes"`
}

// DepthBucketFunc labels a Verify-table depth (executable row count) into a
// caller-named bucket (e.g. "shallow" / "deep"); DefaultDepthBucket is the
// comparable default.
type DepthBucketFunc func(depth int) string

// defaultDeepVerifyThreshold is the row count at or above which a Verify
// table counts as "deep" for the default bucketing. It is a configuration
// default, not a spec-mandated threshold — a caller with its own convention
// should supply its own DepthBucketFunc to ComputeVerifyDepthVsEscapeRate.
const defaultDeepVerifyThreshold = 5

// DefaultDepthBucket buckets a Verify-table depth into "shallow" (fewer than
// defaultDeepVerifyThreshold executable rows) or "deep" (at least that many).
func DefaultDepthBucket(depth int) string {
	if depth >= defaultDeepVerifyThreshold {
		return "deep"
	}
	return "shallow"
}

// ComputeVerifyDepthVsEscapeRate groups changes by (DepthBucket, Brittleness
// Band) and computes the escape rate in each stratum: the share of changes
// with a MEASURED Escaped outcome that escaped. A change whose Escaped
// outcome is could-not-measure is excluded from the rate but still counted
// in Unmeasured, so a stratum's honest coverage is always visible beside its
// rate. A stratum with no measured Escaped outcome at all reports
// could-not-measure rather than a division by zero. bucketOf may be nil, in
// which case DefaultDepthBucket is used.
func ComputeVerifyDepthVsEscapeRate(changes []AuthoredChange, bucketOf DepthBucketFunc) []VerifyDepthEscapeReadout {
	if bucketOf == nil {
		bucketOf = DefaultDepthBucket
	}
	type key struct {
		bucket string
		band   BrittlenessBand
	}
	type accum struct {
		escaped    int
		measured   int
		unmeasured int
	}
	groups := map[key]*accum{}
	for _, c := range changes {
		k := key{bucketOf(c.VerifyDepth), c.BrittlenessBand}
		a, ok := groups[k]
		if !ok {
			a = &accum{}
			groups[k] = a
		}
		if !c.Escaped.IsMeasured() {
			a.unmeasured++
			continue
		}
		a.measured++
		if c.Escaped.Value {
			a.escaped++
		}
	}

	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bucket != keys[j].bucket {
			return keys[i].bucket < keys[j].bucket
		}
		return keys[i].band < keys[j].band
	})

	out := make([]VerifyDepthEscapeReadout, 0, len(keys))
	for _, k := range keys {
		a := groups[k]
		var rate Measure[float64]
		if a.measured == 0 {
			rate = CouldNotMeasure[float64]("no change in this verify-depth x brittleness-band stratum has a measured escape outcome")
		} else {
			rate = Measured(float64(a.escaped) / float64(a.measured))
		}
		out = append(out, VerifyDepthEscapeReadout{
			DepthBucket: k.bucket,
			Band:        k.band,
			EscapeRate:  rate,
			Measured:    a.measured,
			Unmeasured:  a.unmeasured,
		})
	}
	return out
}

// -----------------------------------------------------------------------
// Industry-named agent metrics (spec §7.2) — emitted as alarmed budgets,
// never a dashboard line.
// -----------------------------------------------------------------------

// Budget is an alarmed-budget readout (spec §7.2: "emitted as alarmed
// budgets, not dashboard lines"): a measured value compared against a
// configured threshold, with Breached explicit so a consumer treats a
// crossing as an alarm condition, never a number to eyeball on a chart.
type Budget struct {
	Metric string           `json:"metric"`
	Value  Measure[float64] `json:"value"`
	// Threshold and Direction are config (spec §9.6: budgets are set only
	// after >= 2 windows of measurement), supplied by the caller — this
	// package holds no opinion on what a healthy threshold is.
	Threshold float64 `json:"threshold"`
	// Direction says which side of Threshold is healthy: "at_least" (value
	// must be >= threshold, e.g. survival/first-pass rates) or "at_most"
	// (value must be <= threshold, e.g. merged-without-review share).
	Direction string `json:"direction"`
	// Breached is true only when Value is measured AND crosses Threshold in
	// the unhealthy direction. An unmeasured Value never sets Breached — a
	// could-not-measure budget is its own, separately visible condition
	// (Value.State), never silently treated as a pass or a false alarm.
	Breached bool `json:"breached"`
}

const (
	directionAtLeast = "at_least"
	directionAtMost  = "at_most"
)

func evaluateBudget(metric string, value Measure[float64], threshold float64, direction string) Budget {
	b := Budget{Metric: metric, Value: value, Threshold: threshold, Direction: direction}
	if !value.IsMeasured() {
		return b
	}
	switch direction {
	case directionAtLeast:
		b.Breached = value.Value < threshold
	case directionAtMost:
		b.Breached = value.Value > threshold
	}
	return b
}

// ComputeAgentPRSurvivalRate computes the share of changes surviving N days
// without revert/rework (spec §7.2) as an at-least budget against threshold.
func ComputeAgentPRSurvivalRate(changes []AuthoredChange, threshold float64) Budget {
	rate := rateOfMeasuredBool(changes, func(c AuthoredChange) Measure[bool] { return c.SurvivedNDays })
	return evaluateBudget("agent_pr_survival_rate", rate, threshold, directionAtLeast)
}

// ComputeFirstPassApprovalRate computes the share of changes approved on
// their first review pass (spec §7.2) as an at-least budget against
// threshold.
func ComputeFirstPassApprovalRate(changes []AuthoredChange, threshold float64) Budget {
	rate := rateOfMeasuredBool(changes, func(c AuthoredChange) Measure[bool] { return c.FirstPassApproved })
	return evaluateBudget("first_pass_approval_rate", rate, threshold, directionAtLeast)
}

// ReviewDisciplineGuardrails bundles the two spec §7.2 guardrail readouts:
// the merged-without-review share and the mean time-in-review, each an
// alarmed budget rather than a dashboard line — the published finding is
// that AI throughput quietly erodes review discipline, and a gated process
// should PROVE it is holding.
type ReviewDisciplineGuardrails struct {
	MergedWithoutReview Budget `json:"merged_without_review"`
	TimeInReview        Budget `json:"time_in_review_hours"`
}

// ComputeReviewDisciplineGuardrails computes both guardrail budgets:
// maxMergedWithoutReviewShare and maxTimeInReviewHours are the configured
// at-most thresholds (spec §9.6: config, set only after corpus maturity).
func ComputeReviewDisciplineGuardrails(changes []AuthoredChange, maxMergedWithoutReviewShare, maxTimeInReviewHours float64) ReviewDisciplineGuardrails {
	mwr := rateOfMeasuredBool(changes, func(c AuthoredChange) Measure[bool] { return c.MergedWithoutReview })
	tir := meanOfMeasured(changes, func(c AuthoredChange) Measure[float64] { return c.TimeInReviewHours })
	return ReviewDisciplineGuardrails{
		MergedWithoutReview: evaluateBudget("merged_without_review_share", mwr, maxMergedWithoutReviewShare, directionAtMost),
		TimeInReview:        evaluateBudget("time_in_review_hours", tir, maxTimeInReviewHours, directionAtMost),
	}
}

func rateOfMeasuredBool(changes []AuthoredChange, get func(AuthoredChange) Measure[bool]) Measure[float64] {
	var yes, measured int
	for _, c := range changes {
		m := get(c)
		if !m.IsMeasured() {
			continue
		}
		measured++
		if m.Value {
			yes++
		}
	}
	if measured == 0 {
		return CouldNotMeasure[float64]("no change in this corpus has a measured outcome for this signal")
	}
	return Measured(float64(yes) / float64(measured))
}

func meanOfMeasured(changes []AuthoredChange, get func(AuthoredChange) Measure[float64]) Measure[float64] {
	var sum float64
	var n int
	for _, c := range changes {
		m := get(c)
		if !m.IsMeasured() {
			continue
		}
		sum += m.Value
		n++
	}
	if n == 0 {
		return CouldNotMeasure[float64]("no change in this corpus has a measured reading for this signal")
	}
	return Measured(sum / float64(n))
}
