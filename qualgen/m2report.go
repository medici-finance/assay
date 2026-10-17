package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const metricM2Run = "defect_trace_run"

// M2Run identifies one completed, all-history M2 snapshot. Raw fix and trace
// schemas stay unchanged; this metrics-family record supplies provenance and
// membership so corrections cannot leave stale files in the current readout.
type M2Run struct {
	Metric          string            `json:"metric"`
	Repository      string            `json:"repository"`
	HeadSHA         string            `json:"head_sha"`
	SourceDigest    string            `json:"source_sha256"`
	DefectLabels    []string          `json:"defect_labels"`
	Candidates      int               `json:"candidates"`
	Unidentified    int               `json:"unidentified"`
	Traced          int               `json:"traced"`
	TracedNone      int               `json:"traced_none"`
	CouldNotTrace   int               `json:"could_not_trace"`
	MergedPRCount   int               `json:"merged_pr_count"`
	FixCommits      []string          `json:"fix_commits"`
	TraceHashes     map[string]string `json:"trace_sha256"`
	TierComposition TierComposition   `json:"tier_composition"`
	MinedAt         time.Time         `json:"mined_at"`
}

func (s *Store) latestM2Run() (*M2Run, error) {
	runs, err := readFamily[M2Run](s, map[string]bool{metricM2Run: true})
	if err != nil {
		return nil, err
	}
	var latest *M2Run
	for _, r := range runs {
		if latest == nil || !r.MinedAt.Before(latest.MinedAt) {
			copy := r
			latest = &copy
		}
	}
	return latest, nil
}

func (s *Store) m2Stale(run *M2Run) (bool, error) {
	header, err := s.ReadHeader()
	if err != nil {
		return false, err
	}
	return run != nil && header != nil && run.HeadSHA != header.TipSHA, nil
}

func writeM2Section(b *strings.Builder, store *Store) error {
	b.WriteString("## Defect-inducing rate (M2)\n\n")
	run, err := store.latestM2Run()
	if err != nil {
		return err
	}
	stale, err := store.m2Stale(run)
	if err != nil {
		return err
	}
	if stale {
		b.WriteString("not measured for the current mined HEAD — the defect snapshot is stale; rerun mine with an updated --fix-source. Prior traces remain historical evidence.\n\n")
		return nil
	}
	traces, err := store.ReadTraces()
	if err != nil {
		return err
	}
	current := map[string]DefectTrace{}
	for _, trace := range traces {
		if run != nil {
			digest, err := traceHash(trace)
			if err != nil {
				return err
			}
			if run.TraceHashes[trace.FixCommit] != digest {
				continue
			}
		}
		current[trace.FixCommit] = trace
	}
	if run == nil && len(current) == 0 {
		b.WriteString("not measured — no defect trace corpus; run mine with --fix-source. Never rendered as 0.\n\n")
		return nil
	}
	if run != nil {
		keep := map[string]DefectTrace{}
		for _, sha := range run.FixCommits {
			trace, ok := current[sha]
			if !ok {
				return fmt.Errorf("completed M2 snapshot references missing trace %s", sha)
			}
			keep[sha] = trace
		}
		current = keep
		fmt.Fprintf(b, "Repository: `%s`; HEAD: `%s`; source SHA-256: `%s`.\n\n", mdCell(run.Repository), run.HeadSHA, run.SourceDigest)
		fmt.Fprintf(b, "First-parent candidates: %d; unresolved fix identification: %d; source-declared merged-PR denominator: %d (0 means unavailable).\n\n", run.Candidates, run.Unidentified, run.MergedPRCount)
	} else {
		b.WriteString("Trace artifacts supplied without a completed CLI snapshot; source/identification coverage is unavailable.\n\n")
	}
	traced, none, unavailable := 0, 0, 0
	tiers := TierComposition{}
	keys := make([]string, 0, len(current))
	for key, trace := range current {
		keys = append(keys, key)
		switch trace.TraceState {
		case TraceTraced:
			traced++
		case TraceTracedNone:
			none++
		case TraceCouldNotTrace:
			unavailable++
		default:
			return fmt.Errorf("unknown defect trace state %q", trace.TraceState)
		}
		switch trace.EvidenceTier {
		case Tier1:
			tiers.Tier1Count++
		case Tier2:
			tiers.Tier2Count++
		case Tier3:
			tiers.Tier3Count++
		}
	}
	fmt.Fprintf(b, "Trace outcomes: %d traced, %d traced-none, %d could-not-trace. Evidence tiers: %d tier 1, %d tier 2, %d tier 3 (keyword fallback).\n\n", traced, none, unavailable, tiers.Tier1Count, tiers.Tier2Count, tiers.Tier3Count)
	metrics, err := readFamily[DefectMetricRecord](store, map[string]bool{MetricDefectInducingRate: true, MetricFixLatency: true, MetricDefectTraceRate: true})
	if err != nil {
		return err
	}
	if run != nil {
		var matching []DefectMetricRecord
		for _, m := range metrics {
			if m.MinedAt.Equal(run.MinedAt) {
				matching = append(matching, m)
			}
		}
		metrics = matching
	}
	latest := latestOf(metrics, func(m DefectMetricRecord) string { return m.Metric }, func(m DefectMetricRecord) time.Time { return m.MinedAt })
	b.WriteString("| Metric | Value | Trace rate | Evidence tiers (1 / 2 / 3) |\n|--------|-------|------------|----------------------------|\n")
	for _, metric := range []string{MetricDefectInducingRate, MetricDefectTraceRate, MetricFixLatency} {
		m, ok := latest[metric]
		if !ok {
			fmt.Fprintf(b, "| %s | not measured (derived metric absent) | not measured | unavailable |\n", metric)
			continue
		}
		fmt.Fprintf(b, "| %s | %s | %s | %d / %d / %d |\n", metric, renderMeasureFloat(m.Value), renderMeasureFloat(m.TraceRate), m.TierComposition.Tier1Count, m.TierComposition.Tier2Count, m.TierComposition.Tier3Count)
	}
	b.WriteString("\nInducing rate is distinct inducing commits / source-declared merged PRs over full reachable history; it is not a probability and may exceed 1. Incomplete identification is unavailable. Trace coverage and evidence tiers qualify every result; tracing does not establish causation.\n\n")
	if len(keys) > 0 {
		sort.Strings(keys)
		b.WriteString("| Fix commit | Trace outcome | Inducing commits | Evidence / unavailable reason |\n|------------|---------------|------------------|-------------------------------|\n")
		for _, key := range keys {
			t := current[key]
			fmt.Fprintf(b, "| `%s` | %s | %s | %s; %s |\n", key, t.TraceState, strings.Join(t.InducingCommits, ", "), t.EvidenceTier, mdCell(string(t.CouldNotTraceReason)))
		}
		b.WriteString("\nEvidence: `docs/quality/defects.jsonl`; metrics and completed-snapshot provenance: `docs/quality/metrics.jsonl`.\n\n")
	}
	return nil
}
