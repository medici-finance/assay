package deskkit

import (
	"sort"

	"github.com/medici-finance/assay/perfrec"
)

// PerfCommandStats is one command's invocation costs over one measurement cohort.
// Schema and Boundary remain separate: caller intervals and process initialization,
// and legacy wrapper intervals, have no common timing denominator. SumWallMs is
// accumulated invocation work, not elapsed time; concurrent or nested runs overlap.
// Missing CPU/HTTP observations never contribute a fabricated zero measurement.
type PerfCommandStats struct {
	Tool            string         `json:"tool"`
	Verb            string         `json:"verb"`
	Schema          string         `json:"schema"`
	Boundary        string         `json:"boundary"`
	Count           int            `json:"count"`
	SumWallMs       int64          `json:"sumWallMs"`
	P50Ms           int64          `json:"p50Ms"`
	P90Ms           int64          `json:"p90Ms"`
	MaxMs           int64          `json:"maxMs"`
	CohortSharePct  float64        `json:"cohortSharePct"`
	Outcomes        map[string]int `json:"outcomes"`
	CPUCount        int            `json:"cpuCount"`
	CPUUnavailable  int            `json:"cpuUnavailable"`
	SumCPUMs        int64          `json:"sumCpuMs"`
	HTTPMeasured    int            `json:"httpMeasured"`
	HTTPUnavailable int            `json:"httpUnavailable"`
	HTTPAttempts    int64          `json:"httpAttempts"`
}

// PerfCommandSummary selects v3 identities once across the full input before
// command filters or buckets. Service observations and legacy boot steps are not
// invocations. Unattributed counts old records excluded by a workflow filter.
type PerfCommandSummary struct {
	Rows         []PerfCommandStats `json:"rows"`
	Unattributed int                `json:"unattributed"`
	NonProcess   int                `json:"nonProcess"`
	V3Duplicates int                `json:"v3Duplicates"`
	V3Conflicts  int                `json:"v3Conflicts"`
	V3Invalid    int                `json:"v3Invalid"`
	V4Duplicates int                `json:"v4Duplicates,omitempty"`
	V4Conflicts  int                `json:"v4Conflicts,omitempty"`
	V4Invalid    int                `json:"v4Invalid,omitempty"`
}

// SummarisePerfCommands is the command-first overview. Input must be the whole
// decoded window: selecting after --tool could hide a conflicting v3 identity.
// The old APIs remain intact for consumers of their detailed report sections.
func SummarisePerfCommands(recs []PerfRecord, permission perfrec.Profile, filter perfrec.V3Filter, tool, role string) (PerfCommandSummary, error) {
	var s PerfCommandSummary
	recs, mixedConflicts := perfrec.ExcludeMixedSchemaIdentities(recs)
	kept, sel, err := perfrec.SelectV3(recs, permission, filter)
	if err != nil {
		return s, err
	}
	s.V3Duplicates, s.V3Conflicts, s.V3Invalid = sel.Duplicates, sel.Conflicts, sel.Invalid

	kept4, sel4, err := perfrec.SelectV4(recs, permission, filter)
	if err != nil {
		return s, err
	}
	s.V4Duplicates, s.V4Conflicts, s.V4Invalid = sel4.Duplicates, sel4.Conflicts+mixedConflicts, sel4.Invalid
	kept = append(kept, kept4...)
	type key struct{ schema, boundary, tool, verb string }
	type bucket struct {
		stats PerfCommandStats
		walls []int64
	}
	by := map[key]*bucket{}
	cohortTotals := map[string]int64{}
	add := func(r PerfRecord) {
		if tool != "" && r.Tool != tool {
			return
		}
		if role != "" && (r.Schema == perfrec.SchemaV3 || r.Schema == perfrec.SchemaV4 || r.Role != role) {
			return
		}
		if r.Schema == perfrec.SchemaV3 && r.V3.Kind != perfrec.ObservationProcess || r.Schema == perfrec.SchemaV4 && r.V4.Kind != perfrec.ObservationProcess {
			s.NonProcess++
			return
		}
		schema, boundary := r.Schema, r.Boundary
		switch schema {
		case PerfSchema:
			boundary = "legacy-wrapper"
		case PerfSchemaV2, perfrec.SchemaV3, perfrec.SchemaV4:
			if boundary != perfrec.BoundaryProcess && boundary != perfrec.BoundaryCaller {
				return
			}
		default:
			return
		}
		if r.WallMs < 0 {
			return
		}
		k := key{schema, boundary, perfrec.Label(r.Tool), perfrec.Label(r.Verb)}
		b := by[k]
		if b == nil {
			b = &bucket{stats: PerfCommandStats{Tool: k.tool, Verb: k.verb, Schema: k.schema, Boundary: k.boundary, Outcomes: map[string]int{}}}
			by[k] = b
		}
		st := &b.stats
		st.Count++
		b.walls = append(b.walls, r.WallMs)
		st.SumWallMs = satAdd(st.SumWallMs, r.WallMs)
		c := schema + "/" + boundary
		cohortTotals[c] = satAdd(cohortTotals[c], r.WallMs)
		outcome := r.Result
		switch outcome {
		case ResultOK, ResultDisabled, ResultRefused, ResultUnwritten, ResultUnverifiable:
		default:
			outcome = "other"
		}
		st.Outcomes[outcome]++
		// Legacy cpuMs is a process-wide total, not a delta over its wall.
		if r.Schema != PerfSchema && r.CPU == perfrec.CPUMeasured && r.CPUMs != nil && *r.CPUMs >= 0 {
			st.CPUCount++
			st.SumCPUMs = satAdd(st.SumCPUMs, *r.CPUMs)
		} else {
			st.CPUUnavailable++
		}
		calls, measured, ok := forgeCountDay(r)
		if ok && measured {
			st.HTTPMeasured++
			st.HTTPAttempts = satAdd(st.HTTPAttempts, calls)
		} else {
			st.HTTPUnavailable++
		}
	}
	for _, r := range kept {
		add(r)
	}
	for _, r := range recs {
		if r.Schema == perfrec.SchemaV3 || r.Schema == perfrec.SchemaV4 || IsPerfStep(r) {
			continue
		}
		if filter.Used() {
			s.Unattributed++
			continue
		}
		add(r)
	}
	for k, b := range by {
		st := &b.stats
		sortInt64(b.walls)
		st.P50Ms, st.P90Ms, st.MaxMs = nearestRank(b.walls, 50), nearestRank(b.walls, 90), b.walls[len(b.walls)-1]
		if total := cohortTotals[k.schema+"/"+k.boundary]; total > 0 {
			st.CohortSharePct = round1(100 * float64(st.SumWallMs) / float64(total))
		}
		s.Rows = append(s.Rows, *st)
	}
	sort.Slice(s.Rows, func(i, j int) bool {
		a, b := s.Rows[i], s.Rows[j]
		if a.SumWallMs != b.SumWallMs {
			return a.SumWallMs > b.SumWallMs
		}
		if a.Schema != b.Schema {
			return a.Schema < b.Schema
		}
		if a.Boundary != b.Boundary {
			return a.Boundary < b.Boundary
		}
		if a.Tool != b.Tool {
			return a.Tool < b.Tool
		}
		return a.Verb < b.Verb
	})
	return s, nil
}
