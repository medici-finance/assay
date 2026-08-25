package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// doracli.go — the `--dora` CLI emitter, RESTORED as a v0.14.0 back-compat flag.
//
// WHY THIS EXISTS. v0.14.0 removed the standalone DORA/velocity/code-efficiency CLI
// surface (the old dora.go, with runDora/runDoraSeries/runDoraGrouped and their text
// & JSON renderers) when that commodity metrics job moved to DevLake (Ian ruling
// #1213). But the pinned daily-harvest/v0.1.0 collector still shells out to
// `statusgen -dora`, so its removal made daily-harvest die on `flag provided but not
// defined: -dora` for EVERY consumer, not just one. This file re-adds `--dora` so old
// callers work unchanged.
//
// WHAT IT EMITS. Not a re-alias to an unrelated flag — the DORA COMPUTATION itself
// survived the split: computeDoraGrouped (roadmapdora.go) is the grouped-DORA core the
// roadmap pages consume. `--dora` now runs that core and renders it as text (or JSON
// with `--json`), the same output the roadmap tile is built from. The renderers below
// are lifted verbatim from the removed dora.go's grouped path.
//
// OFFLINE, like the roadmap's own DORA tile. doraInputs is built from the historian
// (docs/streams/.history.jsonl) only — no git/gh calls — so the run is deterministic
// and needs none of the removed network-gathering helpers (gatherDoraInputs, the
// merged-PR/commit/bug-issue fetchers). The git/gh-derived rows therefore render as
// explicit "needs:" markers rather than fabricated numbers — a could-not-check, which
// is the same honest degrade the offline roadmap path already takes.
//
// NOTE FOR CONSUMERS. This restores DORA output on the `-dora` flag; if a consumer
// depended on the pre-removal UNGROUPED `runDora` text shape (a different renderer,
// deleted with dora.go), the grouped shape here differs — but the flag parses and
// emits DORA metrics again, which is what un-reds daily-harvest.

// renderDoraGroupedText renders the grouped-DORA report as human-readable text.
// Lifted verbatim from the removed dora.go.
func renderDoraGroupedText(rep DoraGroupedReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DORA metrics by %s -- %s ... %s\n", rep.By, rep.Since, rep.Until)
	fmt.Fprintf(&b, "%s\n", rep.Note)
	fmt.Fprintf(&b, "Per-group proxy definitions:\n")
	fmt.Fprintf(&b, "  deploy freq = briefs reaching done / window weeks (proxy; aggregate uses git commits)\n")
	fmt.Fprintf(&b, "  lead time   = implemented->done median / p90 from historian\n")
	fmt.Fprintf(&b, "  change fail = (unresolved findings + reverts) / done briefs (proxy; aggregate uses bug issues / merged PRs)\n")
	fmt.Fprintf(&b, "  rework      = unknown (needs: verify-desk|manual, same as aggregate)\n")
	fmt.Fprintf(&b, "  MTTR        = global-only in v1 (per-group attribution noisy)\n")
	fmt.Fprintf(&b, "Groups with n < %d briefs annotate every figure with n=<x> (small-n honesty).\n\n", doraSmallNThreshold)

	for _, g := range rep.Groups {
		sn := ""
		if g.SmallN {
			sn = fmt.Sprintf(" [n=%d, small-n]", g.N)
		}
		fmt.Fprintf(&b, "=== %s%s ===\n", g.Label, sn)
		for _, k := range []string{doraDeployFreq, doraLeadTime, doraChangeFail, doraRework} {
			m := g.Metrics[k]
			fmt.Fprintf(&b, "  %-28s %s", m.Name, m.Value)
			if m.Needs != "" {
				fmt.Fprintf(&b, "  [needs: %s]", m.Needs)
			}
			b.WriteString("\n")
			if m.Detail != "" {
				fmt.Fprintf(&b, "      %s\n", m.Detail)
			}
		}
		b.WriteString("\n")
	}

	// Global MTTR
	fmt.Fprintf(&b, "=== MTTR (global) ===\n")
	fmt.Fprintf(&b, "  %-28s %s", rep.GlobalMTTR.Name, rep.GlobalMTTR.Value)
	if rep.GlobalMTTR.Needs != "" {
		fmt.Fprintf(&b, "  [needs: %s]", rep.GlobalMTTR.Needs)
	}
	b.WriteString("\n")
	if rep.GlobalMTTR.Detail != "" {
		fmt.Fprintf(&b, "      %s\n", rep.GlobalMTTR.Detail)
	}

	return b.String()
}

// renderDoraGroupedJSON renders the grouped report as JSON. Lifted verbatim from
// the removed dora.go.
func renderDoraGroupedJSON(rep DoraGroupedReport) string {
	enc, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(enc)
}

// runDora is the `--dora` entrypoint (back-compat). It builds doraInputs offline
// from the historian — exactly as the roadmap's own DORA tile does — then computes
// and renders the grouped report. `by` is "stream" (default) or "goal". Self-
// contained diagnostic sub-command: never reads or writes STATUS.md.
func runDora(root, since, by string, asJSON bool) int {
	if by == "" {
		by = "stream"
	}
	if by != "stream" && by != "goal" {
		fmt.Fprintf(os.Stderr, "statusgen: --by must be stream|goal, got %q\n", by)
		return 1
	}

	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := parseSinceDate(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t
	} else {
		sinceT = until.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	streams, findings, err := loadStreams(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "statusgen: loading streams: %v\n", err)
		return 1
	}

	// Historian-only inputs — no git/gh calls, so this stays offline and
	// deterministic (the grouped compute reads only History/Since/Until/Now); the
	// git/gh-derived rows degrade to explicit "needs:" markers.
	var history []HistoryEntry
	if h, herr := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); herr == nil {
		history = h
	}
	in := doraInputs{
		Since:          sinceT,
		Until:          until,
		Now:            now,
		History:        history,
		HistoryPresent: len(history) > 0,
	}

	rep := computeDoraGrouped(in, streams, findings, by)
	if asJSON {
		fmt.Print(renderDoraGroupedJSON(rep))
		return 0
	}
	fmt.Print(renderDoraGroupedText(rep))
	return 0
}
