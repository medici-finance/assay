package main

import (
	"sort"
	"time"
)

// features.go is the shared per-file feature assembly the `pr` (brief 08)
// and `check` (brief 09, this brief) modes both call: given a mined
// tracking root and a file path, join that file's already-persisted M1
// hotspot, ownership, and change-coupling families (briefs 02–03) plus the
// M2 traced defect-density family (brief 07, when present) into one
// FileFeatures record. Every field carries its own three-state Measure
// (spec §3.2) — a feature this assembly cannot compute is could-not-measure,
// never a silent zero — and the whole assembly is READ-ONLY over the Store:
// it computes nothing from raw history itself, it joins what `qualgen mine`
// (and, for defect density, a brief-07 trace pass) already persisted.
//
// Brief 09 authored this file first (features.go did not exist on the
// branch when this brief landed); brief 08 reuses it rather than forking a
// second assembly — brief 09's Context: "either order, one shared
// assembly, no fork."

// FileFeatures is one file's brittleness join across the M1/M2 families.
type FileFeatures struct {
	Path string

	// HotspotPercentile is this file's rank in the repo's decayed hotspot
	// distribution (spec §4.3): the fraction of OTHER measured hotspot
	// records this file's own Hotspot value strictly exceeds, in [0,1].
	// could-not-measure when the path has no hotspot record at all (never
	// seen by a mine run) or its own Hotspot value is itself
	// could-not-measure.
	HotspotPercentile Measure[float64]

	// DefectDensity and DefectTraceRate are the traced M2 defect-density
	// family (brief 07, spec §5.3) for this file — always read together
	// (honest-claims discipline, spec §10: a density number never travels
	// without its trace-rate beside it). could-not-measure when brief 07's
	// derived-metrics pass has not appended a defect_density family to this
	// tracking root, or never traced an inducing commit to this file.
	DefectDensity   Measure[float64]
	DefectTraceRate Measure[float64]

	// OwnershipTop is the file-grain top-identity surviving-line share
	// (spec §4.4): the largest single value in the latest ownership
	// record's IdentityShares for this path. could-not-measure when the
	// path has no file-grain ownership record.
	OwnershipTop Measure[float64]

	// CouplingPartners are this file's HISTORICAL change-coupling partners
	// (spec §4.5, Coupled==true pairs involving Path), independent of
	// whether any particular caller's own file set also touches them.
	// `pr` (brief 08) and `check` (brief 09) each subtract their own
	// touched/screened set from this list to get their own missing-partner
	// signal; this assembly stays generic to both callers rather than
	// taking a "touched set" parameter itself.
	CouplingPartners []string
}

// Unmeasured reports whether f carries genuinely NO mined signal for its
// path at all — every family absent or unmeasurable, not merely a measured
// zero within an otherwise-present record. This is the per-file "no
// measurable history" case a caller (check.go) reports as could-not-screen
// rather than presenting an absence of advisories as a clean bill of
// health.
func (f FileFeatures) Unmeasured() bool {
	return f.HotspotPercentile.State == StateCouldNotMeasure &&
		f.OwnershipTop.State == StateCouldNotMeasure &&
		f.DefectDensity.State == StateCouldNotMeasure &&
		len(f.CouplingPartners) == 0
}

// AssembleFileFeatures reads store's persisted M1/M2 metric families and
// joins them for path. It is READ-ONLY: every family it reads was written
// by an earlier `qualgen mine` run (or, for defect density, a brief-07
// trace pass) — this assembly computes no new metric of its own.
func AssembleFileFeatures(store *Store, path string) (FileFeatures, error) {
	hotspots, err := store.ReadHotspots()
	if err != nil {
		return FileFeatures{}, err
	}
	ownership, err := store.ReadOwnership()
	if err != nil {
		return FileFeatures{}, err
	}
	coupling, err := store.ReadCoupling()
	if err != nil {
		return FileFeatures{}, err
	}
	densities, err := store.ReadDefectDensity()
	if err != nil {
		return FileFeatures{}, err
	}

	f := FileFeatures{Path: path}
	f.HotspotPercentile = hotspotPercentile(hotspots, path)
	f.OwnershipTop = ownershipTopShare(ownership, path)
	f.DefectDensity, f.DefectTraceRate = defectDensityFor(densities, path)
	f.CouplingPartners = couplingPartnersOf(coupling, path)
	return f, nil
}

// hotspotPercentile ranks path's latest hotspot value among the latest
// snapshot's OTHER measured hotspot records: the fraction of them it
// strictly exceeds, in [0,1]. A single-file corpus (nothing else to rank
// against) is measured-zero — not could-not-measure — the instrument ran
// and there was simply nothing else to be above.
func hotspotPercentile(hotspots []HotspotRecord, path string) Measure[float64] {
	latest := latestOf(hotspots,
		func(h HotspotRecord) string { return h.Path },
		func(h HotspotRecord) time.Time { return h.MinedAt })
	mine, ok := latest[path]
	if !ok {
		return CouldNotMeasure[float64]("no hotspot record for this path: it was never seen by a `qualgen mine` run against this tracking root")
	}
	if mine.Hotspot.State != StateMeasured {
		if mine.Hotspot.State == StateMeasuredZero {
			return MeasuredZero[float64]()
		}
		return CouldNotMeasure[float64]("this path's own hotspot could not be measured: " + mine.Hotspot.Reason)
	}
	others, below := 0, 0
	for p, h := range latest {
		if p == path || h.Hotspot.State != StateMeasured {
			continue
		}
		others++
		if mine.Hotspot.Value > h.Hotspot.Value {
			below++
		}
	}
	if others == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(below) / float64(others))
}

// ownershipTopShare returns the largest identity share in path's latest
// file-grain ownership record.
func ownershipTopShare(records []OwnershipRecord, path string) Measure[float64] {
	fileRecords := make([]OwnershipRecord, 0, len(records))
	for _, r := range records {
		if r.Grain == "file" {
			fileRecords = append(fileRecords, r)
		}
	}
	latest := latestOf(fileRecords,
		func(o OwnershipRecord) string { return o.Path },
		func(o OwnershipRecord) time.Time { return o.MinedAt })
	rec, ok := latest[path]
	if !ok {
		return CouldNotMeasure[float64]("no file-grain ownership record for this path")
	}
	if rec.SurvivingLines == 0 {
		return MeasuredZero[float64]()
	}
	top := 0.0
	for _, share := range rec.IdentityShares {
		if share > top {
			top = share
		}
	}
	return Measured(top)
}

// defectDensityFor returns path's traced M2 defect-density value together
// with its trace-rate (honest-claims discipline: never one without the
// other). Absent when brief 07's derived-metrics pass has not appended a
// defect_density family to this tracking root at all, or never traced an
// inducer to path.
func defectDensityFor(records []DefectMetricRecord, path string) (Measure[float64], Measure[float64]) {
	latest := latestOf(records,
		func(r DefectMetricRecord) string { return r.Key },
		func(r DefectMetricRecord) time.Time { return r.MinedAt })
	rec, ok := latest[path]
	if !ok {
		reason := "no traced defect-density record for this path (brief 07's B-SZZ trace pass has not run against this tracking root, or traced no inducing commit to it)"
		return CouldNotMeasure[float64](reason), CouldNotMeasure[float64](reason)
	}
	return rec.Value, rec.TraceRate
}

// couplingPartnersOf collects path's historical change-coupling partners
// (Coupled==true pairs involving path), from the latest coupling snapshot.
func couplingPartnersOf(records []CouplingRecord, path string) []string {
	latest := latestOf(records,
		func(c CouplingRecord) string { return c.PathA + "\x00" + c.PathB },
		func(c CouplingRecord) time.Time { return c.MinedAt })
	var out []string
	for _, r := range latest {
		if !r.Coupled {
			continue
		}
		switch path {
		case r.PathA:
			out = append(out, r.PathB)
		case r.PathB:
			out = append(out, r.PathA)
		}
	}
	sort.Strings(out)
	return out
}
