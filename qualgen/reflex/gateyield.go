package reflex

import (
	"sort"

	"github.com/medici-finance/assay/qualgen/attribution"
)

// gateyield.go implements task item 1: M4's gate-yield accounting (spec
// §7.1). Per review lane, this is a JOIN over two already-recorded families —
// pre-merge request-changes findings and M3's per-defect review-escape
// overlay (brief-10, qualgen/attribution) — into catch-rate, escape-rate,
// and latency cost per lane: the evidence to strengthen, re-scope, or retire
// a gate.

// PreMergeFinding is one request-changes finding a review lane recorded on a
// PR BEFORE that PR merged — already-recorded review-verdict data (the same
// per-PR review-verdict shape M3's dossier captures, attribution.
// ReviewVerdict, here supplied for the corpus of ALL reviewed PRs, not only
// the ones that later produced a traced defect). Surface is the flagged
// file/token set the lane called out; it is matched against later-traced
// defect surfaces to decide whether the finding was a genuine catch.
type PreMergeFinding struct {
	Lane    string
	Surface []string
}

// DefectSurface is one traced defect's surface — the token set (files,
// packages, or named surfaces) its inducing change touched. It is read from
// the already-mined M2/M3 corpus (the dossier's DefectSurface, spec §6), not
// re-derived from git here; the caller translates its own artifacts into
// this shape, exactly as qualgen/dorajoin's DurableVolumeInput documents its
// own translation contract.
type DefectSurface struct {
	DefectID string
	Surface  []string
}

// LatencySample is one lane's time-in-review reading for a single PR — an
// already-recorded delivery-metrics figure (spec §8's DORA join reads the
// same kind of timestamp), never computed here.
type LatencySample struct {
	Lane  string
	Hours Measure[float64]
}

// LaneYield is one review lane's gate-yield accounting readout (spec §7.1).
// Catches and Escapes are raw counts (always known, even when the derived
// rates are could-not-measure); CatchRate/EscapeRate/LatencyCost are
// three-state so an unjudged or data-sparse lane is reported honestly rather
// than as a misleading 0.
type LaneYield struct {
	Lane        string           `json:"lane"`
	Catches     int              `json:"catches"`
	Escapes     int              `json:"escapes"`
	CatchRate   Measure[float64] `json:"catch_rate"`
	EscapeRate  Measure[float64] `json:"escape_rate"`
	LatencyCost Measure[float64] `json:"latency_cost_hours"`
}

// GateYieldInput bundles the input families gate-yield joins (task item 1):
// pre-merge findings, the M3 per-stage ledger (read through its documented
// schema, attribution.LedgerEntry — task item 4), the traced-defect surface
// index findings are matched against, and per-lane latency samples. Every
// family is READ, never mined: this package performs no git access
// (TestNoNewMining).
type GateYieldInput struct {
	Findings       []PreMergeFinding
	LedgerEntries  []attribution.LedgerEntry
	DefectSurfaces []DefectSurface
	Latency        []LatencySample
}

// ComputeGateYield runs the per-lane gate-yield join (spec §7.1): for every
// lane appearing in EITHER the findings or the ledger's review-escape
// overlay, count pre-merge catches — a finding whose flagged surface
// intersects at least one traced defect's surface, i.e. evidence the
// concern named a genuine defect-prone area — against the escapes M3's
// review-escape overlay attributes to that lane (attribution.RollupOf,
// brief-10's own tested aggregation, never re-derived here), and compute
// catch-rate / escape-rate / latency cost. A lane with zero judged items
// (no catches and no escapes) reports could-not-measure rates rather than a
// divide-by-zero or a misleading 0 (spec §3.2).
func ComputeGateYield(in GateYieldInput) []LaneYield {
	rollup := attribution.RollupOf(in.LedgerEntries)
	escapesByLane := rollup.ReviewEscapeByLane

	catchesByLane := map[string]int{}
	findingsSeenByLane := map[string]struct{}{}
	for _, f := range in.Findings {
		if f.Lane == "" {
			continue
		}
		findingsSeenByLane[f.Lane] = struct{}{}
		if matchesAnySurface(f.Surface, in.DefectSurfaces) {
			catchesByLane[f.Lane]++
		}
	}

	latencyByLane := aggregateLatency(in.Latency)

	// A lane is IN SCOPE if it was judged at all: it recorded a pre-merge
	// finding (whether or not that finding turned out to be a catch), was
	// attributed an escape, or has a latency sample — never only "had a
	// catch", or a lane whose findings all missed would silently vanish
	// instead of reporting its honest 0 catch-rate.
	lanes := map[string]struct{}{}
	for l := range findingsSeenByLane {
		lanes[l] = struct{}{}
	}
	for l := range escapesByLane {
		lanes[l] = struct{}{}
	}
	for l := range latencyByLane {
		lanes[l] = struct{}{}
	}

	names := make([]string, 0, len(lanes))
	for l := range lanes {
		names = append(names, l)
	}
	sort.Strings(names)

	out := make([]LaneYield, 0, len(names))
	for _, l := range names {
		catches := catchesByLane[l]
		escapes := escapesByLane[l]
		total := catches + escapes

		var catchRate, escapeRate Measure[float64]
		if total == 0 {
			reason := "lane " + l + " has no judged items (no pre-merge catches and no attributed escapes)"
			catchRate = CouldNotMeasure[float64](reason)
			escapeRate = CouldNotMeasure[float64](reason)
		} else {
			catchRate = Measured(float64(catches) / float64(total))
			escapeRate = Measured(float64(escapes) / float64(total))
		}

		latency, ok := latencyByLane[l]
		if !ok {
			latency = CouldNotMeasure[float64]("no latency samples recorded for lane " + l)
		}

		out = append(out, LaneYield{
			Lane:        l,
			Catches:     catches,
			Escapes:     escapes,
			CatchRate:   catchRate,
			EscapeRate:  escapeRate,
			LatencyCost: latency,
		})
	}
	return out
}

// matchesAnySurface reports whether surface shares at least one token with
// any recorded defect surface — the "flagged surface matches a later trace"
// test (spec §7.1). An empty finding surface matches nothing: a finding that
// named no surface cannot be credited with catching one.
func matchesAnySurface(surface []string, defects []DefectSurface) bool {
	if len(surface) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(surface))
	for _, s := range surface {
		set[s] = struct{}{}
	}
	for _, d := range defects {
		for _, s := range d.Surface {
			if _, ok := set[s]; ok {
				return true
			}
		}
	}
	return false
}

// aggregateLatency averages the measured Hours per lane. A lane whose
// samples are all could-not-measure still reports a distinct could-not-
// measure entry (rather than being absent), so ComputeGateYield's caller can
// tell "no samples recorded" apart from "samples recorded but unmeasured."
func aggregateLatency(samples []LatencySample) map[string]Measure[float64] {
	sums := map[string]float64{}
	counts := map[string]int{}
	sawUnmeasured := map[string]bool{}
	for _, s := range samples {
		if s.Lane == "" {
			continue
		}
		if !s.Hours.IsMeasured() {
			sawUnmeasured[s.Lane] = true
			continue
		}
		sums[s.Lane] += s.Hours.Value
		counts[s.Lane]++
	}
	out := map[string]Measure[float64]{}
	for lane, n := range counts {
		out[lane] = Measured(sums[lane] / float64(n))
	}
	for lane := range sawUnmeasured {
		if _, ok := out[lane]; !ok {
			out[lane] = CouldNotMeasure[float64]("lane " + lane + " has latency samples recorded but none are measured")
		}
	}
	return out
}

// -----------------------------------------------------------------------
// Task item 4 — wiring the brief-10 seams directly (review-escape overlay
// join by defect id, three-state on a missing entry).
// -----------------------------------------------------------------------

// BuildLedgerIndex indexes ledger entries by DefectID for ReviewEscapeJoin
// lookups. Callers wanting rollup-consistent current state (superseded
// entries excluded) should resolve that first — attribution.RollupOf does
// this internally for the aggregate counts ComputeGateYield uses; this index
// is for POINT lookups of a single defect's overlay (ritual.go's per-change
// join), so it is built directly from the entries supplied, latest write
// wins on a duplicate DefectID (mirroring the ledger's own append-then-
// amend-by-tombstone ordering when entries are read in file order).
func BuildLedgerIndex(entries []attribution.LedgerEntry) map[string]attribution.LedgerEntry {
	idx := make(map[string]attribution.LedgerEntry, len(entries))
	for _, e := range entries {
		idx[e.DefectID] = e
	}
	return idx
}

// ReviewEscapeJoin looks up the review-escape overlay for one defect id
// against a ledger index (task item 4: "fail three-state (could-not-join)
// rather than guess when a defect has no overlay entry"). A defect id
// present in the index returns its Lanes as Measured — even an EMPTY Lanes
// list is a measured fact (nothing approved, or no verdicts recorded;
// attribution.stage.go's own reviewEscape() already treats this as honest,
// not a gap). A defect id ABSENT from the index returns could-not-measure:
// this package never guesses "no lanes" for a defect it was never able to
// join against the ledger at all.
func ReviewEscapeJoin(defectID string, index map[string]attribution.LedgerEntry) Measure[[]string] {
	entry, ok := index[defectID]
	if !ok {
		return CouldNotMeasure[[]string]("could-not-join: no review-escape overlay entry recorded for defect " + defectID)
	}
	// attribution's own writer always sorts+de-dupes Lanes (stage.go's
	// reviewEscape()); this defensively re-canonicalizes so a caller
	// building a LedgerEntry by hand (a fixture, a hand-repaired tombstone)
	// can never leak a non-canonical order into a downstream comparison.
	seen := map[string]struct{}{}
	lanes := make([]string, 0, len(entry.ReviewEscape.Lanes))
	for _, l := range entry.ReviewEscape.Lanes {
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		lanes = append(lanes, l)
	}
	sort.Strings(lanes)
	return Measured(lanes)
}
