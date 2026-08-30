package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// report.go renders the QUALITY.md trend view (spec §9.3) from the committed
// JSONL artifacts. It computes NOTHING: it reads the latest mine snapshot per
// metric out of docs/quality/metrics.jsonl and renders each headline number
// beside its industry-comparable baseline (spec §10), honouring the three-state
// flag on every value (spec §3.2).
//
// SINGLE-WRITER discipline (spec §9.3, STATUS.md model): the committed
// QUALITY.md is written ONLY by CI on main. A local run READS the artifacts and
// renders to stdout, then DISCARDS — it never writes the committed view. The
// --write path that writes the committed file is reachable ONLY when the
// CI-writer guard authorizes it (ciWriterAuthorized); a local --write is
// REFUSED, leaving the committed file untouched.

// qualityView is the committed view's filename under the tracking root's quality
// dir (docs/quality/QUALITY.md), beside the metrics table it is rendered from.
const qualityView = "QUALITY.md"

// ciWriterEnv is the environment variable CI sets to authorize the single
// committed-view write. Its value must be exactly ciWriterToken. A local shell
// does not set it, so a local --write is refused — the guard, not a filename
// convention, is what makes CI the only writer.
const (
	ciWriterEnv   = "QUALGEN_QUALITY_WRITER"
	ciWriterToken = "ci"
)

// ciWriterAuthorized reports whether the current process is the authorized CI
// writer. It reads a single explicit environment variable through the injected
// getenv (tests pass a stub; production passes os.Getenv), so the guard is
// exercised without mutating the real environment.
func ciWriterAuthorized(getenv func(string) string) bool {
	return getenv(ciWriterEnv) == ciWriterToken
}

func runReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "tracking root to read artifacts from (required)")
	baselinesPath := fs.String("baselines", "", "optional JSON file of industry baselines overriding the built-in set")
	write := fs.Bool("write", false, "write the committed QUALITY.md (CI-writer only; a local run is refused and renders to stdout)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		fmt.Fprintln(stderr, "qualgen report: --out <dir> is required (the tracking root to read artifacts from)")
		return 2
	}

	baselines, err := LoadBaselines(*baselinesPath)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen report:", err)
		return 1
	}

	store := NewStore(*out)
	view, err := renderReport(store, baselines)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen report:", err)
		return 1
	}

	if !*write {
		// Local read-and-discard: render to stdout, never touch the committed view.
		fmt.Fprint(stdout, view)
		return 0
	}

	// --write is the committed-view path — guarded to CI. A local run is REFUSED
	// (the block is a stop, not a fallback to a different write): the committed
	// QUALITY.md is left byte-for-byte untouched.
	if !ciWriterAuthorized(os.Getenv) {
		fmt.Fprintf(stderr, "qualgen report: REFUSED — --write is the single-writer committed-view path, reachable only by CI (%s=%s). A local run renders to stdout and discards; the committed QUALITY.md is unchanged.\n", ciWriterEnv, ciWriterToken)
		return 2
	}
	target := filepath.Join(store.dir(), qualityView)
	if err := os.MkdirAll(store.dir(), 0o755); err != nil {
		fmt.Fprintln(stderr, "qualgen report:", err)
		return 1
	}
	if err := os.WriteFile(target, []byte(view), 0o644); err != nil {
		fmt.Fprintln(stderr, "qualgen report:", err)
		return 1
	}
	fmt.Fprintf(stdout, "qualgen report: wrote committed view %s\n", target)
	return 0
}

// latestOf reduces an append-only metrics table (each mine appends a fresh full
// snapshot) to the newest record per key by MinedAt, so a trend consumer reads
// the current value, not a table-position first-match.
func latestOf[T any](recs []T, key func(T) string, when func(T) time.Time) map[string]T {
	latest := map[string]T{}
	at := map[string]time.Time{}
	for _, r := range recs {
		k := key(r)
		if t, seen := at[k]; !seen || when(r).After(t) {
			latest[k] = r
			at[k] = when(r)
		}
	}
	return latest
}

// renderReport builds the QUALITY.md view string from the artifacts under store.
// It is separated from runReport so tests render without touching stdout or the
// filesystem write path.
func renderReport(store *Store, baselines BaselineSet) (string, error) {
	metrics, err := store.ReadMetrics()
	if err != nil {
		return "", err
	}
	hotspots, err := store.ReadHotspots()
	if err != nil {
		return "", err
	}
	ownership, err := store.ReadOwnership()
	if err != nil {
		return "", err
	}

	// Latest snapshot per (metric,grain,key) for the scalar comparability table.
	latestMetric := latestOf(metrics,
		func(m MetricRecord) string { return m.Metric + "\x00" + m.Grain + "\x00" + m.Key },
		func(m MetricRecord) time.Time { return m.MinedAt })

	var b strings.Builder
	b.WriteString("# QUALITY.md — trend view\n\n")
	b.WriteString("<!-- GENERATED by `qualgen report`. SINGLE-WRITER: CI is the only writer of this committed view; a local run renders to stdout and discards (spec §9.3). Do not hand-edit. -->\n\n")

	// --- M1 comparability metrics: local beside industry, three-state honoured. ---
	b.WriteString("## M1 — comparability metrics\n\n")
	b.WriteString("Industry baselines " + honestClaimsNote + ".\n\n")
	b.WriteString("| Metric | Local | Industry | Basis / window note |\n")
	b.WriteString("|--------|-------|----------|---------------------|\n")
	type row struct {
		label  string
		metric string
	}
	for _, r := range []row{
		{"Copy/paste ratio", MetricCopyPasteRatio},
		{"Duplicate-block rate", MetricDuplicateBlockRate},
		{"Churn / rework rate", MetricChurnRate},
	} {
		key := r.metric + "\x00" + GrainRepo + "\x00"
		local := "not present"
		if m, ok := latestMetric[key]; ok {
			local = renderMeasureFloat(m.Value)
		}
		industry := "no published baseline"
		note := ""
		if base, ok := baselines[r.metric]; ok {
			industry = renderMeasureFloat(base.Value)
			note = base.WindowNote
			if note == "" {
				note = base.Source
			}
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.label, local, industry, mdCell(note))
	}
	// Churn volume context (measured counts, not ratios).
	newLines := scalarString(latestMetric, MetricNewLines)
	churnedLines := scalarString(latestMetric, MetricChurnedLines)
	fmt.Fprintf(&b, "\nChurn volume: %s new line(s), %s churned line(s) within the 14-day window.\n\n", newLines, churnedLines)

	// --- Top-10 hotspots (spec §4.3). ---
	b.WriteString("## Top-10 hotspots\n\n")
	writeHotspots(&b, hotspots)

	// --- Bus-factor alarms (spec §4.4). ---
	b.WriteString("## Bus-factor alarms\n\n")
	writeBusFactorAlarms(&b, ownership)

	// --- Sections populated by later waves — could-not-measure until then. ---
	b.WriteString("## Defect-inducing rate (M2)\n\n")
	b.WriteString("not measured — populated by quality/06–07 (defect-fix identification + SZZ trace). Never rendered as 0.\n\n")
	b.WriteString("## Per-stage ledger (M3)\n\n")
	b.WriteString("not measured — populated by quality/10 (stage attribution). Never rendered as 0.\n\n")
	b.WriteString("## Instruction reference-validity trend\n\n")
	b.WriteString("not measured — the instruction-brittleness family (quality/04) is not yet emitted to the metrics table. Never rendered as 0.\n")

	return b.String(), nil
}

// writeHotspots renders the top-10 hotspots of the latest snapshot by hotspot
// score. A file whose hotspot is could-not-measure is shown as unmeasured, never
// ranked as a zero.
func writeHotspots(b *strings.Builder, hotspots []HotspotRecord) {
	if len(hotspots) == 0 {
		b.WriteString("not measured — no hotspot family in the artifacts.\n\n")
		return
	}
	latest := latestOf(hotspots,
		func(h HotspotRecord) string { return h.Path },
		func(h HotspotRecord) time.Time { return h.MinedAt })
	rows := make([]HotspotRecord, 0, len(latest))
	for _, h := range latest {
		rows = append(rows, h)
	}
	sort.Slice(rows, func(i, j int) bool {
		vi, vj := sortScore(rows[i].Hotspot), sortScore(rows[j].Hotspot)
		if vi != vj {
			return vi > vj
		}
		return rows[i].Path < rows[j].Path
	})
	if len(rows) > 10 {
		rows = rows[:10]
	}
	b.WriteString("| Path | Hotspot | Change freq | Complexity |\n")
	b.WriteString("|------|---------|-------------|------------|\n")
	for _, h := range rows {
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n",
			mdCell(h.Path),
			renderMeasureFloat(h.Hotspot),
			renderMeasureFloat(h.ChangeFrequency),
			renderMeasureFloat(h.ComplexityProxy))
	}
	b.WriteString("\n")
}

// writeBusFactorAlarms lists paths whose latest ownership record raises a
// bus-factor-of-one identity concentration or a role-SPOF (spec §4.4).
func writeBusFactorAlarms(b *strings.Builder, ownership []OwnershipRecord) {
	if len(ownership) == 0 {
		b.WriteString("not measured — no ownership family in the artifacts.\n\n")
		return
	}
	latest := latestOf(ownership,
		func(o OwnershipRecord) string { return o.Grain + "\x00" + o.Path },
		func(o OwnershipRecord) time.Time { return o.MinedAt })
	rows := make([]OwnershipRecord, 0, len(latest))
	for _, o := range latest {
		rows = append(rows, o)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	var alarms []OwnershipRecord
	for _, o := range rows {
		if o.RoleSPOF || (o.BusFactorIdentity.State == StateMeasured && o.BusFactorIdentity.Value <= 1) {
			alarms = append(alarms, o)
		}
	}
	if len(alarms) == 0 {
		b.WriteString("none — no path raised a bus-factor-of-one or role-SPOF alarm in the latest snapshot.\n\n")
		return
	}
	b.WriteString("| Path | Bus factor (identity) | Role SPOF |\n")
	b.WriteString("|------|-----------------------|-----------|\n")
	for _, o := range alarms {
		spof := "no"
		if o.RoleSPOF {
			spof = "yes"
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", mdCell(o.Path), renderMeasureInt(o.BusFactorIdentity), spof)
	}
	b.WriteString("\n")
}

// scalarString renders a repo-grain scalar metric's latest value, or a
// not-present marker when the metric is absent from the artifacts.
func scalarString(latest map[string]MetricRecord, metric string) string {
	if m, ok := latest[metric+"\x00"+GrainRepo+"\x00"]; ok {
		return renderMeasureFloat(m.Value)
	}
	return "not present"
}

// renderMeasureFloat renders a three-state float Measure: a real number for
// measured, "0" for a genuine measured-zero, and "not measured (<reason>)" for
// could-not-measure — NEVER a bare 0 for could-not-measure (spec §3.2).
func renderMeasureFloat(m Measure[float64]) string {
	switch m.State {
	case StateMeasured:
		return formatFloat(m.Value)
	case StateMeasuredZero:
		return "0"
	case StateCouldNotMeasure:
		return "not measured (" + m.Reason + ")"
	default:
		return "not measured (no state)"
	}
}

// renderMeasureInt renders a three-state int Measure with the same discipline.
func renderMeasureInt(m Measure[int]) string {
	switch m.State {
	case StateMeasured:
		return strconv.Itoa(m.Value)
	case StateMeasuredZero:
		return "0"
	case StateCouldNotMeasure:
		return "not measured (" + m.Reason + ")"
	default:
		return "not measured (no state)"
	}
}

// sortScore maps a hotspot Measure to a sort key: could-not-measure sorts last
// (below any real value) so an unmeasured file never tops the ranking as if it
// were the worst hotspot.
func sortScore(m Measure[float64]) float64 {
	if m.State == StateCouldNotMeasure {
		return -1
	}
	return m.Value
}

// formatFloat renders a metric value for the view. A whole-number value (a
// line count such as new_lines) renders as a plain integer — never scientific
// notation — while a fractional ratio renders with 4 significant figures, so
// 0.42 stays "0.42" (the dereference Verify #3 pins) and a large count stays
// "736159" rather than "7.362e+05".
func formatFloat(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', 4, 64)
}

// mdCell escapes a pipe so a path or note never breaks the surrounding table.
func mdCell(s string) string {
	if s == "" {
		return "—"
	}
	return strings.ReplaceAll(s, "|", "\\|")
}
