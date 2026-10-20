package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reportJSON is `deskperf report --json`: the same aggregates as the table, one object.
// Summary is the LEGACY perf-v1 group, unchanged in shape and meaning; PerfV2 is the perf-v2
// group, present only when the window holds perf-v2 records. The two are never combined: their
// walls are measured over different boundaries.
type reportJSON struct {
	Days    int                    `json:"days"`
	Tool    string                 `json:"tool,omitempty"`
	Role    string                 `json:"role,omitempty"`
	Skipped int                    `json:"skipped"`
	Summary deskkit.PerfSummary    `json:"summary"`
	PerfV2  *deskkit.PerfV2Summary `json:"perfV2,omitempty"`
}

func cmdReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("deskperf report", flag.ContinueOnError)
	days := fs.Int("days", deskkit.PerfMaxDays, "UTC days to read (1-7)")
	tool := fs.String("tool", "", "only this tool (canonical key, e.g. deskpr)")
	role := fs.String("role", "", "only this desk role ($DESK_LOOP canonical name)")
	top := fs.Int("top", 10, "how many of the slowest invocations to list")
	asJSON := fs.Bool("json", false, "emit the aggregates as one JSON object")
	if err := parse(fs, args); err != nil {
		return err
	}
	if err := window(*days); err != nil {
		return err
	}
	if *top < 0 {
		return deskkit.Refused(fmt.Sprintf("--top %d: must be 0 or more", *top))
	}
	h, err := load(*days)
	if err != nil {
		return err
	}
	// Step records subdivide an invocation that has its own record: never in this report.
	var verbs []deskkit.PerfRecord
	for _, r := range h.Records {
		if !deskkit.IsPerfStep(r) {
			verbs = append(verbs, r)
		}
	}
	if len(verbs) == 0 {
		return noMatch("a verb-level report (only boot-step records are in the window — try `deskperf boot`)")
	}
	recs := deskkit.FilterPerf(verbs, *tool, *role)
	if len(recs) == 0 {
		return noMatch(filterText(*tool, *role))
	}
	sum := deskkit.SummarisePerf(recs, *top)
	v2 := deskkit.SummarisePerfV2(recs, *top)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		rep := reportJSON{Days: *days, Tool: *tool, Role: *role, Skipped: h.Skipped, Summary: sum}
		if v2.Records > 0 {
			rep.PerfV2 = &v2
		}
		return encodeErr(enc.Encode(rep))
	}
	if sum.Records > 0 {
		if err := legacyTable(stdout, sum, *days, *tool, *role, h.Skipped); err != nil {
			return err
		}
	} else {
		skippedLine(stdout, h.Skipped)
	}
	if v2.Records > 0 {
		if sum.Records > 0 {
			fmt.Fprintln(stdout)
		}
		return v2Table(stdout, v2, *days, *tool, *role)
	}
	return nil
}

// legacyTable renders the perf-v1 group exactly as deskperf always has, so a perf-v1-only
// history reads the same before and after perf-v2.
func legacyTable(stdout io.Writer, sum deskkit.PerfSummary, days int, tool, role string, skipped int) error {
	fmt.Fprintf(stdout, "deskperf report — %d record(s) over the last %d UTC day(s)%s, total wall %d ms\n",
		sum.Records, days, filterSuffix(tool, role), sum.TotalMs)
	skippedLine(stdout, skipped)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TOOL\tCOUNT\tP50 MS\tP90 MS\tMAX MS\tTOTAL MS\tSHARE %\tREFUSAL RATIO")
	for _, t := range sum.Tools {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%.1f\t%.3f\n",
			t.Tool, t.Count, t.P50Ms, t.P90Ms, t.MaxMs, t.TotalMs, t.SharePct, t.RefusalRatio)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}
	return slowestTable(stdout, sum.Slowest)
}

func slowestTable(stdout io.Writer, slow []deskkit.PerfSlow) error {
	if len(slow) == 0 {
		return nil
	}
	fmt.Fprintf(stdout, "\nslowest %d invocation(s):\n", len(slow))
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TS\tTOOL\tVERB\tWALL MS")
	for _, s := range slow {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", s.TS, s.Tool, s.Verb, s.WallMs)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}
	return nil
}

// v2Table renders the perf-v2 group: its own header naming the boundary and that the figure
// is SUMMED invocation wall (not elapsed time), the coverage lines, the per-tool wall and
// measured-CPU distributions, counter coverage, phase distributions and the slowest list.
func v2Table(stdout io.Writer, v2 deskkit.PerfV2Summary, days int, tool, role string) error {
	fmt.Fprintf(stdout, "perf-v2 — %d record(s) over the last %d UTC day(s)%s, summed invocation wall %d ms "+
		"(a sum of per-invocation walls, not elapsed time; not comparable with the legacy total)\n",
		v2.Records, days, filterSuffix(tool, role), v2.SumWallMs)
	fmt.Fprintf(stdout, "boundary: %s · cpu: %s · phase overlap: %d record(s)\n",
		countList(v2.Boundaries, []string{perfrec.BoundaryProcess, perfrec.BoundaryCaller}),
		countList(v2.CPU, []string{perfrec.CPUMeasured, perfrec.CPUUnsupported, perfrec.CPUFailed, perfrec.CPUMissing}),
		v2.PhaseOverlap)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TOOL\tCOUNT\tP50 MS\tP90 MS\tMAX MS\tSUM WALL MS\tSHARE %\tREFUSAL RATIO\tCPU N\tCPU P50 MS\tCPU P90 MS\tCPU MAX MS")
	for _, t := range v2.Tools {
		cpu := "-\t-\t-"
		if t.CPUCount > 0 {
			cpu = fmt.Sprintf("%d\t%d\t%d", t.CPUP50Ms, t.CPUP90Ms, t.CPUMaxMs)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%.1f\t%.3f\t%d\t%s\n",
			t.Tool, t.Count, t.P50Ms, t.P90Ms, t.MaxMs, t.SumWallMs, t.SharePct, t.RefusalRatio, t.CPUCount, cpu)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}
	fmt.Fprintln(stdout, "\ncounters (TOTAL over measured records only; unavailable = not observed, never zero; "+
		"subprocessMs is inclusive child wall, never added to wall):")
	tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COUNTER\tMEASURED\tUNAVAILABLE\tTOTAL")
	for _, c := range v2.Counters {
		total := "-"
		if c.Measured > 0 {
			total = fmt.Sprint(c.Total)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n", c.Counter, c.Measured, c.Unavailable, total)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}
	if len(v2.Phases) > 0 {
		fmt.Fprintln(stdout, "\nphases (per-record ms inside the invocation wall; never summed with it):")
		tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "TOOL\tPHASE\tRECORDS\tN\tP50 MS\tP90 MS\tMAX MS")
		for _, p := range v2.Phases {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\t%d\n", p.Tool, p.Phase, p.Records, p.N, p.P50Ms, p.P90Ms, p.MaxMs)
		}
		if err := tw.Flush(); err != nil {
			return deskkit.Unverifiable("cannot write the report", err)
		}
	}
	return slowestTable(stdout, v2.Slowest)
}

// countList renders "a N · b M" over the known keys in order, then any other key by name.
func countList(m map[string]int, known []string) string {
	var parts []string
	seen := map[string]bool{}
	for _, k := range known {
		seen[k] = true
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	var rest []string
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func filterText(tool, role string) string {
	switch {
	case tool != "" && role != "":
		return fmt.Sprintf("--tool %s --role %s", tool, role)
	case tool != "":
		return "--tool " + tool
	case role != "":
		return "--role " + role
	}
	return "the request"
}

func filterSuffix(tool, role string) string {
	if tool == "" && role == "" {
		return ""
	}
	return " (" + filterText(tool, role) + ")"
}

func encodeErr(err error) error {
	if err != nil {
		return deskkit.Unverifiable("cannot encode the JSON report", err)
	}
	return nil
}
