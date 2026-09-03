package consumers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/qualgen/filer"
)

// autofile.go turns the quality signal into advisory, budgeted refactor work
// (spec §9.5). It CONSUMES two quality/03 families — decaying hotspots above a
// threshold and duplicate-block clusters — composes ONE refactor item per
// distinct target (deduped by path), routes each through a pluggable
// filer.IssueFiler, and enforces a filing budget the caller sets.
//
// Two postures this file exists to hold, both structural rather than
// documentary:
//
//   - NEVER self-dispatching. There is no assign, start, or dispatch path here.
//     Every composed item is advisory (filer.NewAdvisoryItem), and the report
//     it produces is something a human or an intake process triages. The word
//     "dispatch" appears in this package only to say it does not happen.
//   - Budgeted, degrading to dry-run. At most Budget items are actually filed
//     in one run; every candidate beyond the budget is composed and logged as a
//     dry-run, never filed (spec §9.5's "over budget → dry-run/logged, not
//     unbounded filing"). The count of candidates is unbounded; the count of
//     WRITES is bounded by the budget.

// HotspotSignal is one decaying-hotspot reading quality/03 produced for a path
// (qualgen/hotspot.go's HotspotRecord, translated to the seam this consumer
// reads — the same "join input, not a shared mining type" discipline
// qualgen/dorajoin and qualgen/reflex use, since package main cannot be
// imported). Score is three-state: only a measured score can be ABOVE a
// threshold, so a could-not-measure hotspot never files (it did not clear the
// bar; it was never measured against it).
type HotspotSignal struct {
	Path  string
	Score Measure[float64]
}

// DuplicateCluster is one duplicate-block cluster quality/03 detected — a set of
// paths sharing a duplicated block. Its target (the dedup key and the item's
// TargetPath) is the lexically-first path in the cluster, so two runs over the
// same cluster compose the same single item.
type DuplicateCluster struct {
	// ID is a stable identifier for the cluster (e.g. a block hash); it is
	// carried into the item body for traceability but is NOT the dedup key.
	ID string
	// Paths are the files the duplicated block appears in. Empty clusters are
	// skipped (there is no target to file against).
	Paths []string
}

// AutofileConfig configures one autofile run.
type AutofileConfig struct {
	// HotspotThreshold is the decaying-hotspot score at or below which a hotspot
	// is NOT filed. A hotspot files only when its Score is measured AND strictly
	// greater than this threshold.
	HotspotThreshold float64
	// Budget is the maximum number of items ACTUALLY filed in this run. Once the
	// budget is spent, every further candidate degrades to a dry-run (composed
	// and logged, never filed). A Budget of zero files nothing — every candidate
	// dry-runs — which is a legitimate "logged only" posture, not an error.
	Budget int
	// Filer is the sink composed items are routed through. Required.
	Filer filer.IssueFiler
	// RefactorLabel, when non-empty, is added to every composed item alongside
	// the mandatory advisory label.
	RefactorLabel string
}

// AutofileReport is the result of an autofile run: one FiledResult per distinct
// target, in deterministic (path-sorted) order, plus the run's counts.
type AutofileReport struct {
	// Items is one entry per distinct target, sorted by target path. Each is a
	// filer.FiledResult, so a caller sees exactly what was filed or dry-run and
	// can log the composed body of every item.
	Items []filer.FiledResult `json:"items"`
	// Filed is the number of items actually written to the tracker; DryRun is the
	// number composed-and-logged only (over budget, or an adapter in dry-run).
	Filed  int `json:"filed"`
	DryRun int `json:"dry_run"`
	// Budget echoes the configured budget the run enforced.
	Budget int `json:"budget"`
}

// Autofile composes advisory refactor items from the hotspot and cluster
// signals, dedupes them by target path, routes each through cfg.Filer, and
// enforces cfg.Budget. It performs no dispatch of any kind.
//
// A candidate is produced for:
//   - each hotspot whose Score is measured and strictly above cfg.HotspotThreshold;
//   - each non-empty duplicate cluster.
//
// Candidates are deduped by target path (a path that is both an above-threshold
// hotspot and a cluster target yields ONE item). Items are then filed in
// path-sorted order until the budget is spent, after which each remaining item
// is filed with dryRun = true (degraded to logged).
func Autofile(cfg AutofileConfig, hotspots []HotspotSignal, clusters []DuplicateCluster) (AutofileReport, error) {
	if cfg.Filer == nil {
		return AutofileReport{}, fmt.Errorf("autofile: no IssueFiler configured")
	}
	if cfg.Budget < 0 {
		return AutofileReport{}, fmt.Errorf("autofile: negative budget %d", cfg.Budget)
	}

	// Compose one candidate item per distinct target path. A map keyed by target
	// path is the dedup: the first candidate to claim a path wins, and hotspots
	// are considered before clusters so an above-threshold hotspot's richer body
	// is preferred when a path is both.
	byTarget := map[string]filer.RefactorItem{}
	order := []string{}
	claim := func(target string, item filer.RefactorItem) {
		if target == "" {
			return
		}
		if _, seen := byTarget[target]; seen {
			return
		}
		byTarget[target] = item
		order = append(order, target)
	}

	for _, h := range hotspots {
		if strings.TrimSpace(h.Path) == "" {
			continue
		}
		// Only a MEASURED score can be above the threshold. A could-not-measure
		// or measured-zero score never files — it did not clear the bar.
		if h.Score.State != StateMeasured || !(h.Score.Value > cfg.HotspotThreshold) {
			continue
		}
		claim(h.Path, hotspotItem(h, cfg))
	}

	for _, c := range clusters {
		target := primaryPath(c.Paths)
		if target == "" {
			continue
		}
		claim(target, clusterItem(target, c, cfg))
	}

	sort.Strings(order)

	report := AutofileReport{Budget: cfg.Budget}
	filed := 0
	for _, target := range order {
		item := byTarget[target]
		// Budget gate: while there is budget left, attempt a real file; once
		// spent, force dry-run so the item is composed and logged but not filed.
		overBudget := filed >= cfg.Budget
		res, err := cfg.Filer.File(item, overBudget)
		if err != nil {
			return AutofileReport{}, fmt.Errorf("autofile: filing item for %q: %w", target, err)
		}
		if res.Filed {
			filed++
			report.Filed++
		} else {
			report.DryRun++
		}
		report.Items = append(report.Items, res)
	}
	return report, nil
}

// hotspotItem composes the advisory item for one above-threshold hotspot. The
// body references the hotspot's path and score so a reader (and Verify #3) can
// confirm the item dereferences the RIGHT hotspot, not merely that some item was
// produced.
func hotspotItem(h HotspotSignal, cfg AutofileConfig) filer.RefactorItem {
	title := fmt.Sprintf("Refactor hotspot: %s", h.Path)
	body := fmt.Sprintf(
		"Advisory refactor candidate — decaying-hotspot score %.4f is above the threshold %.4f.\n\n"+
			"Target file: %s\n\n"+
			"This is an ADVISORY item for triage. It was auto-filed from quality/03 hotspot mining "+
			"and does not assign, start, or dispatch any work.",
		h.Score.Value, cfg.HotspotThreshold, h.Path,
	)
	return filer.NewAdvisoryItem(title, body, h.Path, refactorLabels(cfg)...)
}

// clusterItem composes the advisory item for one duplicate-block cluster.
func clusterItem(target string, c DuplicateCluster, cfg AutofileConfig) filer.RefactorItem {
	title := fmt.Sprintf("De-duplicate block cluster at %s", target)
	body := fmt.Sprintf(
		"Advisory refactor candidate — duplicate-block cluster %q spans %d file(s): %s.\n\n"+
			"Primary target file: %s\n\n"+
			"This is an ADVISORY item for triage. It was auto-filed from quality/03 duplicate-block "+
			"detection and does not assign, start, or dispatch any work.",
		c.ID, len(c.Paths), strings.Join(sortedCopy(c.Paths), ", "), target,
	)
	return filer.NewAdvisoryItem(title, body, target, refactorLabels(cfg)...)
}

func refactorLabels(cfg AutofileConfig) []string {
	if strings.TrimSpace(cfg.RefactorLabel) == "" {
		return nil
	}
	return []string{cfg.RefactorLabel}
}

// primaryPath is the lexically-first non-empty path in a cluster — the stable
// dedup key. Returns "" for an empty/all-blank cluster.
func primaryPath(paths []string) string {
	best := ""
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if best == "" || p < best {
			best = p
		}
	}
	return best
}

func sortedCopy(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
