package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reportJSON is `deskperf report --json`: the same aggregates as the table, one object.
type reportJSON struct {
	Days    int                 `json:"days"`
	Tool    string              `json:"tool,omitempty"`
	Role    string              `json:"role,omitempty"`
	Skipped int                 `json:"skipped"`
	Summary deskkit.PerfSummary `json:"summary"`
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

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return encodeErr(enc.Encode(reportJSON{Days: *days, Tool: *tool, Role: *role, Skipped: h.Skipped, Summary: sum}))
	}
	fmt.Fprintf(stdout, "deskperf report — %d record(s) over the last %d UTC day(s)%s, total wall %d ms\n",
		sum.Records, *days, filterSuffix(*tool, *role), sum.TotalMs)
	skippedLine(stdout, h.Skipped)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TOOL\tCOUNT\tP50 MS\tP90 MS\tMAX MS\tTOTAL MS\tSHARE %\tREFUSAL RATIO")
	for _, t := range sum.Tools {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%.1f\t%.3f\n",
			t.Tool, t.Count, t.P50Ms, t.P90Ms, t.MaxMs, t.TotalMs, t.SharePct, t.RefusalRatio)
	}
	if err := tw.Flush(); err != nil {
		return deskkit.Unverifiable("cannot write the report", err)
	}
	if len(sum.Slowest) > 0 {
		fmt.Fprintf(stdout, "\nslowest %d invocation(s):\n", len(sum.Slowest))
		tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "TS\tTOOL\tVERB\tWALL MS")
		for _, s := range sum.Slowest {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", s.TS, s.Tool, s.Verb, s.WallMs)
		}
		if err := tw.Flush(); err != nil {
			return deskkit.Unverifiable("cannot write the report", err)
		}
	}
	return nil
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
