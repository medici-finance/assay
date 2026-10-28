package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reportJSON is `deskperf report --json`: the same aggregates as the table, one object.
// Summary is the LEGACY perf-v1 group, unchanged in shape and meaning; PerfV2 is the perf-v2
// group, present only when the window holds perf-v2 records. The two are never combined: their
// walls are measured over different boundaries. Forge is the forge-call view over both groups
// (a forge call spends the same budget whichever schema recorded it), present only when the
// window holds a perf-v1 or perf-v2 invocation record.
type reportJSON struct {
	Days     int                        `json:"days"`
	Commands deskkit.PerfCommandSummary `json:"commands"`
	Tool     string                     `json:"tool,omitempty"`
	Role     string                     `json:"role,omitempty"`
	Skipped  int                        `json:"skipped"`
	Summary  deskkit.PerfSummary        `json:"summary"`
	PerfV3   *perfrec.V3Summary         `json:"perfV3,omitempty"`
	PerfV4   *perfrec.V4Summary         `json:"perfV4,omitempty"`
	PerfV2   *deskkit.PerfV2Summary     `json:"perfV2,omitempty"`
	Forge    *deskkit.PerfForgeSummary  `json:"forge,omitempty"`
	// RequestsByTool is the perf-v3 request rollup split per tool (the PerfV3 section's
	// requests are merged over the whole window): per tool, request attempts by
	// protocol/resource class, histogram-estimated latency p50/p90, and rate-limit
	// points summed where measured.
	RequestsByTool []deskkit.PerfToolRequests `json:"requestsByTool,omitempty"`
}

// reportDayJSON is `deskperf report --by-day --json`: one object holding the per-day
// timeseries — a separate top-level shape from reportJSON, so the day view never reads as
// a pooled window.
type reportDayJSON struct {
	Days        int                    `json:"days"`
	Tool        string                 `json:"tool,omitempty"`
	Role        string                 `json:"role,omitempty"`
	Skipped     int                    `json:"skipped"`
	ByDay       deskkit.PerfDaySummary `json:"byDay"`
	PerfV4      *perfrec.V4Summary     `json:"perfV4,omitempty"`
	PerfV4ByDay []deskkit.PerfV4Day    `json:"perfV4ByDay,omitempty"`
}

func cmdReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("deskperf report", flag.ContinueOnError)
	days := fs.Int("days", deskkit.PerfMaxDays, "UTC days to read (1-7)")
	tool := fs.String("tool", "", "only this tool (canonical key, e.g. deskpr)")
	role := fs.String("role", "", "only this desk role ($DESK_LOOP canonical name)")
	top := fs.Int("top", 10, "how many of the slowest invocations to list")
	byDay := fs.Bool("by-day", false, "report a per-tool timeseries bucketed by UTC day instead of the pooled window")
	profile := fs.String("perf-profile", os.Getenv("ASSAY_PERF_PROFILE"), "workflow permits local attribution grouping and filtering")
	group := fs.String("group-by", "tool", "v3 group: tool, work, session, agent, deskRole, role (--by-day groups by tool only)")
	session := fs.String("session-id", "", "v3 workflow session filter")
	agent := fs.String("agent-id", "", "v3 workflow agent filter")
	deskRole := fs.String("desk-role", "", "v3 workflow originating desk filter")
	assigned := fs.String("assigned-role", "", "v3 workflow execution role filter")
	work := fs.String("work", "", "v3 workflow typed JSON-object locator filter")
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
	if *byDay && *group != "tool" {
		return deskkit.Refused(fmt.Sprintf("--group-by %s: --by-day rows are tool × UTC day, so --by-day takes no other grouping", *group))
	}
	filter := perfrec.V3Filter{SessionID: *session, AgentID: *agent, DeskRole: *deskRole, Role: *assigned}
	if *work != "" {
		var w perfrec.WorkRef
		dec := json.NewDecoder(strings.NewReader(*work))
		dec.DisallowUnknownFields()
		if dec.Decode(&w) != nil || dec.Decode(new(any)) != io.EOF {
			return deskkit.Refused("invalid work filter")
		}
		filter.Work = &w
	}
	permission := perfrec.Profile(*profile)
	if permission != "" && permission != perfrec.ProfileTool && permission != perfrec.ProfileWorkflow && permission != "off" {
		return deskkit.Refused("invalid performance profile")
	}
	if _, err := perfrec.ReduceV3(nil, permission, *group, filter); err != nil {
		return deskkit.Refused(err.Error())
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
	allVerbs := verbs
	verbs, mixedConflicts := perfrec.ExcludeMixedSchemaIdentities(verbs)
	if len(verbs) == 0 {
		if mixedConflicts > 0 {
			return noMatch(fmt.Sprintf("mixed-schema identity selection excluded %d conflicting identities", mixedConflicts))
		}
		return noMatch("a verb-level report (only boot-step records are in the window — try `deskperf boot`)")
	}
	// Identity selection must precede tool/role filtering: a conflicting copy
	// under another tool still excludes the invocation from every report section.
	keptV3, selection, err := perfrec.SelectV3(verbs, permission, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	keptV4, selection4, err := perfrec.SelectV4(verbs, permission, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	recs := deskkit.FilterPerf(append(keptV3, keptV4...), *tool, *role)
	for _, r := range verbs {
		if r.Schema == perfrec.SchemaV3 || r.Schema == perfrec.SchemaV4 || (*tool != "" && r.Tool != *tool) || (*role != "" && r.Role != *role) {
			continue
		}
		if filter.Used() {
			continue
		} // historical records have no supported workflow attribution
		recs = append(recs, r)
	}
	workflowFilter := *group != "tool" || *session != "" || *agent != "" || *deskRole != "" || *assigned != "" || *work != ""
	// Keep absent tool/role and workflow misses distinct after identity selection.
	if len(recs) == 0 && (*tool != "" || *role != "") && (!workflowFilter || len(deskkit.FilterPerf(verbs, *tool, *role)) == 0) {
		if selection.Conflicts+selection.Invalid+selection4.Conflicts+selection4.Invalid+mixedConflicts > 0 {
			return noMatch(fmt.Sprintf("%s (whole-window identity selection excluded %d conflicting identities and %d invalid records)", filterText(*tool, *role), selection.Conflicts+selection4.Conflicts+mixedConflicts, selection.Invalid+selection4.Invalid))
		}
		return noMatch(filterText(*tool, *role))
	}
	sum := deskkit.SummarisePerf(recs, *top)
	v2 := deskkit.SummarisePerfV2(recs, *top)
	forge := deskkit.SummarisePerfForge(recs)
	v3, err := perfrec.ReduceV3(recs, permission, *group, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	v3.Duplicates, v3.Conflicts, v3.Invalid = selection.Duplicates, selection.Conflicts, selection.Invalid
	v4, err := perfrec.ReduceV4(recs, permission, *group, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	v4.Duplicates, v4.Conflicts, v4.Invalid = selection4.Duplicates, selection4.Conflicts+mixedConflicts, selection4.Invalid
	if workflowFilter && v3.Records+v4.Records == 0 {
		// Preserve the historical diagnostic when the window contains no v4.
		for _, r := range verbs {
			if r.Schema == perfrec.SchemaV4 {
				return deskkit.Refused("no v3/v4 record matches the workflow filter")
			}
		}
		return deskkit.Refused("no v3 record matches the workflow filter")
	}
	if *byDay {
		return reportByDay(recs, permission, filter, *days, *tool, *role, h.Skipped, *asJSON, selection, &v4, stdout)
	}
	commands, err := deskkit.SummarisePerfCommands(allVerbs, permission, filter, *tool, *role)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	byTool, err := deskkit.SummarisePerfV3RequestsByTool(recs, permission, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		rep := reportJSON{Days: *days, Commands: commands, Tool: *tool, Role: *role, Skipped: h.Skipped, Summary: sum}
		if v3.Records > 0 || v3.Conflicts > 0 || v3.Invalid > 0 {
			rep.PerfV3 = &v3
		}
		if v4.Records > 0 || v4.Conflicts > 0 || v4.Invalid > 0 {
			rep.PerfV4 = &v4
		}
		if v2.Records > 0 {
			rep.PerfV2 = &v2
		}
		if forge.Runs > 0 {
			rep.Forge = &forge
		}
		if len(byTool) > 0 {
			rep.RequestsByTool = byTool
		}
		return encodeErr(enc.Encode(rep))
	}
	if err := commandTable(stdout, commands, *days); err != nil {
		return err
	}
	fmt.Fprintln(stdout)
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
		if err := v2Table(stdout, v2, *days, *tool, *role); err != nil {
			return err
		}
	}
	if forge.Runs > 0 {
		fmt.Fprintln(stdout)
		if err := forgeTable(stdout, forge); err != nil {
			return err
		}
		if v3.Records > 0 {
			fmt.Fprintln(stdout, "(the pooled forge section above leaves out perf-v3 http counters, which report their requests "+
				"in the perf-v3 section; --by-day pools them into its per-day forge count, so its totals can be higher)")
		}
	}
	if v3.Records > 0 || v3.Conflicts > 0 || v3.Invalid > 0 {
		if sum.Records > 0 || v2.Records > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "perf-v3 — %d records; %d duplicates; %d conflicting identities excluded; %d invalid\n", v3.Records, v3.Duplicates, v3.Conflicts, v3.Invalid)
		fmt.Fprintf(stdout, "logical service queries %d; service transport attempts %d; handlers %d (separate boundaries)\n", v3.LogicalQueries, v3.ServiceTransports, v3.Handlers)
		for _, g := range v3.Groups {
			fmt.Fprintf(stdout, "%s: %d records; summed own-process wall %d ms; measured own CPU %d ms (%d records)\n", g.Key, g.Records, g.SumOwnProcessWallMs, g.SumOwnCPUMs, g.CPURecords)
		}
		for _, r := range v3.Requests {
			points := "unavailable"
			if r.Points.Value != nil {
				points = fmt.Sprint(*r.Points.Value)
			}
			fmt.Fprintf(stdout, "%s %s %s %s/%s: %d own attempts; measured request work %d ms; points %s %s (%s, %d samples)\n", r.Kind, r.Class, r.Origin, r.Protocol, r.Resource, r.Attempts, r.DurationSumMs, points, r.Points.Unit, r.Points.Coverage, r.PointSamples)
		}
		fmt.Fprintln(stdout, "Intervals overlap; request/child/handler time is never added to process elapsed time. Missing measurements remain coverage gaps.")
	}
	if len(byTool) > 0 {
		fmt.Fprintln(stdout)
		if err := requestRollupTable(stdout, byTool); err != nil {
			return err
		}
	}
	if v4.Records+v4.Conflicts+v4.Invalid > 0 {
		v4Table(stdout, v4)
	}
	return nil
}

// reportByDay is the `deskperf report --by-day` view: one row per tool × UTC day instead of
// the pooled window. Walls stay per schema group; the forge count pools every schema as one
// budget; each row carries its perf-v3 request rollup.
func reportByDay(recs []deskkit.PerfRecord, permission perfrec.Profile, filter perfrec.V3Filter,
	days int, tool, role string, skipped int, asJSON bool, selection perfrec.V3Selection, v4 *perfrec.V4Summary, stdout io.Writer) error {
	s, err := deskkit.SummarisePerfDays(recs, permission, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	s.V3Duplicates, s.V3Conflicts, s.V3Invalid = selection.Duplicates, selection.Conflicts, selection.Invalid
	v4Days, err := deskkit.SummarisePerfV4Days(recs, permission, filter)
	if err != nil {
		return deskkit.Refused(err.Error())
	}
	if v4.Records+v4.Conflicts+v4.Invalid == 0 {
		v4 = nil
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return encodeErr(enc.Encode(reportDayJSON{Days: days, Tool: tool, Role: role, Skipped: skipped, ByDay: s, PerfV4: v4, PerfV4ByDay: v4Days}))
	}
	if err := dayTable(stdout, s, v4, days, tool, role, skipped); err != nil {
		return err
	}
	for _, day := range v4Days {
		fmt.Fprintf(stdout, "\n%s %s — ", day.Day, day.Tool)
		v4DayTable(stdout, day.Summary)
	}
	return nil
}

// dayTable renders the --by-day timeseries: a header with the undated count, one wall table
// (a row per tool × day × schema group), one forge table (a row per tool × day, pooling
// every schema), and the perf-v3 request rollup under each tool × day that has one.
func dayTable(stdout io.Writer, s deskkit.PerfDaySummary, v4 *perfrec.V4Summary, days int, tool, role string, skipped int) error {
	fmt.Fprintf(stdout, "deskperf report --by-day — %d invocation record(s) over the last %d UTC day(s)%s",
		s.Records, days, filterSuffix(tool, role))
	if s.Undated > 0 {
		fmt.Fprintf(stdout, ", %d undated (timestamp did not parse: counted, never assigned a day)", s.Undated)
	}
	fmt.Fprintln(stdout)
	skippedLine(stdout, skipped)
	if s.V3Records > 0 || s.V3Duplicates > 0 || s.V3Conflicts > 0 || s.V3Invalid > 0 {
		fmt.Fprintf(stdout, "perf-v3 — %d record(s) kept; %d duplicate(s); %d conflicting identities excluded; %d invalid "+
			"(selected over the whole window before any day bucket)\n", s.V3Records, s.V3Duplicates, s.V3Conflicts, s.V3Invalid)
	}
	if v4 != nil {
		fmt.Fprintf(stdout, "perf-v4 — %d record(s) kept; %d duplicate(s); %d conflicting identities excluded; %d invalid "+
			"(selected over the whole window before any day bucket)\n", v4.Records, v4.Duplicates, v4.Conflicts, v4.Invalid)
	}
	if s.Unattributed > 0 {
		fmt.Fprintf(stdout, "%d legacy/perf-v2 record(s) left out: the workflow filter matches perf-v3 attribution, which they do not carry\n",
			s.Unattributed)
	}
	fmt.Fprintln(stdout, "walls stay per schema group (each schema measures over its own boundary — legacy, perf-v2 and "+
		"perf-v3 walls are never mixed); the forge count pools legacy forgeCalls and the perf-v2/perf-v3 http counter as one budget")

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DAY\tTOOL\tSCHEMA\tRECORDS\tP50 MS\tP90 MS\tMAX MS\tSUM MS")
	for _, row := range s.Rows {
		for _, g := range []struct {
			name string
			w    *deskkit.PerfDayWall
		}{{perfrec.Schema, row.PerfV1}, {perfrec.SchemaV2, row.PerfV2}, {perfrec.SchemaV3, row.PerfV3}} {
			if g.w == nil {
				continue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
				row.Day, row.Tool, g.name, g.w.Records, g.w.P50Ms, g.w.P90Ms, g.w.MaxMs, g.w.SumMs)
		}
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}

	fmt.Fprintln(stdout, "\nforge calls by UTC day (pooled over every schema; unavailable = not observed, never zero):")
	if s.V3Records > 0 {
		fmt.Fprintln(stdout, "(the pooled report's forge section leaves out perf-v3 http counters, so these totals can be higher than it)")
	}
	tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DAY\tTOOL\tCALLS\tMEASURED RUNS\tUNAVAILABLE RUNS")
	for _, row := range s.Rows {
		calls := "-"
		if row.ForgeMeasured > 0 {
			calls = fmt.Sprint(row.ForgeCalls)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\n", row.Day, row.Tool, calls, row.ForgeMeasured, row.ForgeUnavailable)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}

	anyRequests := false
	for _, row := range s.Rows {
		if len(row.Requests) == 0 {
			continue
		}
		anyRequests = true
		fmt.Fprintf(stdout, "\n%s %s — perf-v3 requests:\n", row.Day, row.Tool)
		if err := requestRows(stdout, row.Requests); err != nil {
			return err
		}
	}
	if anyRequests {
		fmt.Fprintln(stdout, "request-latency p50/p90 are estimated from the durationBins histogram — an estimate names no "+
			"observed duration; points are summed where measured only")
	}
	return nil
}

// requestRollupTable renders the pooled per-tool perf-v3 request rollup.
func requestRollupTable(stdout io.Writer, byTool []deskkit.PerfToolRequests) error {
	fmt.Fprintln(stdout, "perf-v3 requests by tool (latency p50/p90 estimated from the durationBins histogram — an "+
		"estimate names no observed duration; points summed where measured only):")
	for _, t := range byTool {
		fmt.Fprintf(stdout, "\n%s (%d records):\n", t.Tool, t.Records)
		if err := requestRows(stdout, t.Requests); err != nil {
			return err
		}
	}
	return nil
}

// requestRows renders one request rollup block: attempts by protocol/resource class, the
// estimated latency percentiles, the observed max and summed measured work, and the
// measured points sum.
func requestRows(stdout io.Writer, rows []deskkit.PerfRequestRollup) error {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tCLASS\tORIGIN\tPROTOCOL/RESOURCE\tOUTCOME\tATTEMPTS\tP50 MS~\tP90 MS~\tMAX MS\tSUM MS\tPOINTS")
	for _, q := range rows {
		p50, p90 := "-", "-"
		if q.P50Ms != nil {
			p50 = fmt.Sprint(*q.P50Ms)
		}
		if q.P90Ms != nil {
			p90 = fmt.Sprint(*q.P90Ms)
		}
		points := "unavailable"
		if q.Points != nil {
			points = fmt.Sprintf("%d %s", *q.Points, q.PointsUnit)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s/%s\t%s\t%d\t%s\t%s\t%d\t%d\t%s\n",
			q.Kind, q.Class, q.Origin, q.Protocol, q.Resource, q.Outcome, q.Attempts, p50, p90, q.MaxMs, q.DurationSumMs, points)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
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

// forgeTable renders the forge-call section: one header line with the totals and the busiest
// hour against the App budget floor (PerfForgeBudgetPerHour says why that is a gauge, not a
// bound), then one table per tool and one per release build. A row with
// no measured run prints "-" for every figure — unavailable is not observed, never zero.
func forgeTable(stdout io.Writer, f deskkit.PerfForgeSummary) error {
	peak := "-"
	if f.PeakHour != "" {
		peak = fmt.Sprintf("%s %d call(s), %.2f%%", f.PeakHour, f.PeakHourCalls, f.PeakHourBudgetPct)
	}
	fmt.Fprintf(stdout, "forge calls — %d call(s) over %d run(s) (legacy forgeCalls %d, perf-v2 http %d, unavailable %d); "+
		"busiest UTC hour %s of the %d/hour App budget floor\n",
		f.Calls, f.Runs, f.FromV1, f.FromV2, f.Unavailable, peak, f.BudgetPerHour)
	fmt.Fprintf(stdout, "(HTTP attempts, not GraphQL points; unavailable = not observed, never zero; the hour pools every "+
		"tool and App installation against one installation's %d/hour REST floor, so a percentage gauges pressure "+
		"and is no installation's measured draw)\n", f.BudgetPerHour)
	for _, g := range []struct {
		head string
		rows []deskkit.PerfForgeStats
	}{{"TOOL", f.Tools}, {"RELEASE", f.Releases}} {
		fmt.Fprintln(stdout)
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, g.head+"\tRUNS\tUNAVAILABLE\tCALLS\tP50/RUN\tP90/RUN\tMAX/RUN\tSHARE %\tPEAK HOUR CALLS\tPEAK HOUR %\tP90 RUN %")
		for _, r := range g.rows {
			figs := "-\t-\t-\t-\t-\t-\t-\t-"
			if r.Measured > 0 {
				figs = fmt.Sprintf("%d\t%d\t%d\t%d\t%.1f\t%d\t%.2f\t%.2f",
					r.Calls, r.P50PerRun, r.P90PerRun, r.MaxPerRun, r.SharePct, r.PeakHourCalls, r.PeakHourBudgetPct, r.P90RunBudgetPct)
			}
			fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n", r.Key, r.Runs, r.Unavailable, figs)
		}
		if err := tw.Flush(); err != nil {
			return deskkit.Unverifiable("cannot write the report", err)
		}
	}
	return nil
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

// commandTable leads with the operator's question: which command consumes the
// most measured invocation work? Cohorts have separate shares and latency ranks.
func commandTable(stdout io.Writer, s deskkit.PerfCommandSummary, days int) error {
	fmt.Fprintln(stdout, "command performance — largest accumulated invocation costs first; shares stay within timing cohorts")
	fmt.Fprintf(stdout, "Window: last %d UTC day(s), including the current partial UTC day.\n", days)
	fmt.Fprintln(stdout, "Summed invocation work is not elapsed time; nested/concurrent runs overlap. CPU is own measured CPU; HTTP is measured attempts, not points. Missing measurements are not zero. Legacy CPU is unavailable here because its process-wide total is not an interval delta.")
	if s.Unattributed+s.NonProcess+s.V3Duplicates+s.V3Conflicts+s.V3Invalid > 0 {
		fmt.Fprintf(stdout, "excluded: %d unattributed; %d non-process observations; %d v3 duplicates; %d conflicting identities; %d invalid\n", s.Unattributed, s.NonProcess, s.V3Duplicates, s.V3Conflicts, s.V3Invalid)
	}
	if s.V4Duplicates+s.V4Conflicts+s.V4Invalid > 0 {
		fmt.Fprintf(stdout, "excluded perf-v4: %d duplicates; %d conflicting identities; %d invalid\n", s.V4Duplicates, s.V4Conflicts, s.V4Invalid)
	}
	return commandRows(stdout, s.Rows)
}

func commandRows(stdout io.Writer, rows []deskkit.PerfCommandStats) error {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TOOL\tCOMMAND\tCOUNT\tSUM WALL\tCOHORT SHARE %\tP50\tP90\tMAX\tOUTCOMES\tOWN CPU [N/MISSING]\tHTTP [N/MISSING]\tTIMING COHORT")
	for _, r := range rows {
		cpu, http := "-", "-"
		if r.CPUCount > 0 {
			cpu = commandDuration(r.SumCPUMs)
		}
		if r.HTTPMeasured > 0 {
			http = fmt.Sprint(r.HTTPAttempts)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%.1f\t%s\t%s\t%s\t%s\t%s [%d/%d]\t%s [%d/%d]\t%s/%s\n", r.Tool, r.Verb, r.Count, commandDuration(r.SumWallMs), r.CohortSharePct, commandDuration(r.P50Ms), commandDuration(r.P90Ms), commandDuration(r.MaxMs), commandOutcomes(r.Outcomes), cpu, r.CPUCount, r.CPUUnavailable, http, r.HTTPMeasured, r.HTTPUnavailable, r.Boundary, r.Schema)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}
	return nil
}

func commandOutcomes(outcomes map[string]int) string {
	var parts []string
	for _, k := range []string{"ok", "disabled", "refused", "unwritten", "unverifiable", "other"} {
		if n := outcomes[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", k, n))
		}
	}
	return strings.Join(parts, ",")
}

// commandDuration renders existing millisecond measurements without multiplying
// into time.Duration, which overflows on valid saturated cumulative totals.
// JSON retains exact milliseconds; long text durations omit subsecond detail.
func commandDuration(ms int64) string {
	if ms >= 3600000 {
		return fmt.Sprintf("%dh%02dm%02ds", ms/3600000, (ms%3600000)/60000, (ms%60000)/1000)
	}
	if ms >= 60000 {
		return fmt.Sprintf("%dm%02ds", ms/60000, (ms%60000)/1000)
	}
	if ms >= 1000 {
		return fmt.Sprintf("%d.%03ds", ms/1000, ms%1000)
	}
	return fmt.Sprintf("%dms", ms)
}
