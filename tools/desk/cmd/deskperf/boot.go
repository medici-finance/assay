package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// bootJSON is `deskperf boot --json`.
type bootJSON struct {
	Days    int                     `json:"days"`
	Role    string                  `json:"role,omitempty"`
	Skipped int                     `json:"skipped"`
	Summary deskkit.PerfBootSummary `json:"summary"`
}

func cmdBoot(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("deskperf boot", flag.ContinueOnError)
	days := fs.Int("days", deskkit.PerfMaxDays, "UTC days to read (1-7)")
	role := fs.String("role", "", "only this desk role ($DESK_LOOP canonical name)")
	asJSON := fs.Bool("json", false, "emit the aggregates as one JSON object")
	if err := parse(fs, args); err != nil {
		return err
	}
	if err := window(*days); err != nil {
		return err
	}
	h, err := load(*days)
	if err != nil {
		return err
	}
	var steps []deskkit.PerfRecord
	for _, r := range h.Records {
		if deskkit.IsPerfStep(r) {
			steps = append(steps, r)
		}
	}
	if len(steps) == 0 {
		return noMatch("a boot-step report (no deskboot step record in the window)")
	}
	recs := deskkit.FilterPerf(steps, "", *role)
	if len(recs) == 0 {
		return noMatch(filterText("", *role))
	}
	sum := deskkit.SummariseBootSteps(recs)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return encodeErr(enc.Encode(bootJSON{Days: *days, Role: *role, Skipped: h.Skipped, Summary: sum}))
	}
	fmt.Fprintf(stdout, "deskperf boot — %d step record(s) over the last %d UTC day(s)%s\n",
		sum.Records, *days, filterSuffix("", *role))
	skippedLine(stdout, h.Skipped)
	for _, r := range sum.Roles {
		fmt.Fprintf(stdout, "\nrole %s — summed boot cost %d ms\n", r.Role, r.TotalMs)
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "STEP\tCOUNT\tP50 MS\tP90 MS\tMAX MS")
		for _, s := range r.Steps {
			fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\n", s.Step, s.Count, s.P50Ms, s.P90Ms, s.MaxMs)
		}
		if err := tw.Flush(); err != nil {
			return deskkit.Unverifiable("cannot write the report", err)
		}
	}
	return nil
}
