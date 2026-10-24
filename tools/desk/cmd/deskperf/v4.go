package main

import (
	"fmt"
	"github.com/medici-finance/assay/perfrec"
	"io"
)

func v4Table(out io.Writer, s perfrec.V4Summary) {
	fmt.Fprintf(out, "perf-v4 — %d records; %d duplicates; %d conflicting identities excluded; %d invalid\n", s.Records, s.Duplicates, s.Conflicts, s.Invalid)
	for _, g := range s.Groups {
		fmt.Fprintf(out, "%s: %d records; summed own-process wall %s; measured own CPU %s (%d records)\n", g.Key, g.Records, commandDuration(g.SumOwnProcessWallMs), commandDuration(g.SumOwnCPUMs), g.CPURecords)
	}
	fmt.Fprintf(out, "unavailable facets: process %d; attribution %d; requests %d; query %d; query not applicable %d; unsupported extensions %d\n", s.ProcessUnavailable, s.AttributionUnavailable, s.RequestsUnavailable, s.QueryUnavailable, s.QueryNotApplicable, s.UnsupportedExtensions)
	for _, r := range s.Requests {
		points := "unavailable"
		if r.Points.Value != nil {
			points = fmt.Sprint(*r.Points.Value)
		}
		fmt.Fprintf(out, "%s %s %s %s/%s: %d own attempts; measured request work %s; points %s %s (%s, %d samples)", r.Kind, r.Class, r.Origin, r.Protocol, r.Resource, r.Attempts, commandDuration(r.DurationSumMs), points, r.Points.Unit, r.Points.Coverage, r.PointSamples)
		if r.Points.Coverage == perfrec.CoverageEstimated {
			fmt.Fprintf(out, " method %s/%s", r.Points.Method, r.Points.Version)
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, "Intervals overlap; process wall is invocation work, not elapsed human wait or wasted CPU. Unavailable observations remain coverage gaps.")
}
