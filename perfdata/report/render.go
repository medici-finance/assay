package report

import (
	"fmt"
	"io"
	"strings"
)

// Render writes the human-readable report. Unknown values print as "unknown", never 0.
func Render(w io.Writer, r Report) error {
	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("performance report (%s)\n", r.Schema)
	p("coverage: %s   cutoff: %s   window: %s..%s\n", r.State, r.Snapshot.Cutoff, dash(r.Snapshot.Window.From), dash(r.Snapshot.Window.To))
	for _, s := range r.Snapshot.Sources {
		p("  source %s: %s events=%d corrupt=%d partialTails=%d unexplainedGaps=%d retired=%d unclean=%d live=%d\n",
			dash(s.Source), s.State, s.Events, s.CorruptLines+s.CorruptMetadata, s.PartialTails, len(s.UnexplainedGaps),
			len(s.Retired), s.UncleanTerminations, s.LiveWriters)
	}
	for _, ri := range r.Snapshot.Reports {
		p("  report input %s: %s%s\n", ri.Report, ri.State, paren(ri.Reason))
	}
	if len(r.Days) > 0 {
		p("days:\n")
		for _, d := range r.Days {
			p("  %s %-10s runs=%d\n", d.Day, d.State, d.Runs)
		}
	}
	p("tools (pooled over the window; never averaged across days):\n")
	for _, t := range r.Tools {
		p("  %-16s %-12s %-8s child=%-5t runs=%-5d wall avg=%s p95=%s (n=%d, unknown=%d)  cpu-own avg=%s p95=%s (unknown=%d)\n",
			t.Tool, dash(t.Verb), t.Version, t.Child, t.Runs, avg(t.Wall), p95(t.Wall), t.Wall.Known, t.Wall.Unknown,
			avg(t.CPUOwn), p95(t.CPUOwn), t.CPUOwn.Unknown)
	}
	c := r.Cost
	p("cost: invocations=%d elapsed(union)=%dms summed-wall=%dms wall-unknown=%d cpu-own=%dms (known=%d unknown=%d) uninstrumented-children=%d\n",
		c.Invocations, c.ElapsedMs, c.SummedWallMs, c.WallUnknown, c.CPUOwnMs, c.CPUOwnKnown, c.CPUOwnUnknown, c.ChildrenUninst)
	for _, ph := range r.Phases {
		p("  phase %-10s n=%d elapsed=%dms summed=%dms cpu-own=%dms (unknown wall=%d cpu=%d)\n",
			ph.Phase, ph.Intervals, ph.ElapsedMs, ph.SummedWallMs, ph.CPUOwnMs, ph.WallUnknown, ph.CPUOwnUnknown)
	}
	iv := r.Invocations
	p("invocations: distinct=%d duplicate-summaries=%d corrections applied=%d unapplied=%d\n",
		iv.Distinct, iv.DuplicateSummaries, iv.CorrectionsApplied, iv.CorrectionsUnapplied)
	a := r.Attempts
	p("attempts: started=%d finished=%d abandoned=%d unfinished=%d orphan-terminal=%d\n", a.Started, a.Finished, a.Abandoned, a.Unfinished, a.OrphanTerminal)
	for _, u := range r.Usage {
		p("usage %s: series=%d mixed=%d input=%s output=%s cache-read=%s cache-write=%s cost-unknown=%d\n",
			u.Model, u.Series, u.MixedSeries, tok(u.Input), tok(u.Output), tok(u.CacheRead), tok(u.CacheWrite), u.CostUnknown)
		for _, ct := range u.Cost {
			p("  cost %s %s %s: %d micro-units over %d series\n", ct.Currency, ct.PricingRevision, ct.Basis, ct.MicroUnits, ct.Series)
		}
	}
	for _, h := range r.HTTP {
		p("http %s %s %s [%s]: attempts=%d retries=%d not-modified=%d failed=%d%s\n",
			h.Forge, h.Class, h.Origin, h.Allocation, h.Attempts, h.Retries, h.NotModified, h.Failed, points(h.Points))
	}
	for _, s := range r.Services {
		p("service %s:\n", s.Operation)
		if s.Client == nil {
			p("  client: unknown\n")
		} else {
			p("  client: logical=%d transport=%d retries=%d denied=%d\n", s.Client.LogicalCalls, s.Client.TransportAttempts, s.Client.Retries, s.Client.Outcomes.Denied)
		}
		if s.Server == nil {
			p("  server: unknown\n")
		} else {
			p("  server: requests=%d hit=%d miss=%d leader=%d waiter=%d\n", s.Server.Requests, s.Server.Cache.Hit, s.Server.Cache.Miss, s.Server.Coalescing.Leader, s.Server.Coalescing.Waiter)
		}
		if s.Upstream.Known {
			p("  upstream provider attempts: %d%s\n", s.Upstream.Attempts, points(s.Upstream.Points))
		} else {
			p("  upstream provider attempts: unknown (%s; at least %d)\n", s.Upstream.Reason, s.Upstream.Attempts)
		}
	}
	if r.Shared.Attempts > 0 {
		p("shared/unallocated upstream attempts: %d\n", r.Shared.Attempts)
	}
	d := r.Collection.Drops
	p("collection: queue-full=%d write-failed=%d flush-timeout=%d refused=%d retired-records=%d retired-events=%d (unsent)\n",
		d.QueueFull, d.WriteFailed, d.FlushTimeout, d.Refused, r.Collection.RetiredRecords, r.Collection.RetiredEvents)
	_, err := io.WriteString(w, b.String())
	return err
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func paren(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

func avg(s Stat) string {
	if s.AvgMs == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.2fms", *s.AvgMs)
}

func p95(s Stat) string {
	if s.P95Ms == nil {
		return "unknown"
	}
	if s.Method != "" && s.Method != "exact-nearest-rank" {
		return fmt.Sprintf("~%dms", *s.P95Ms)
	}
	return fmt.Sprintf("%dms", *s.P95Ms)
}

func tok(t Tokens) string {
	if t.Sum == nil {
		return fmt.Sprintf("unknown(%d series)", t.Unknown)
	}
	if t.Unknown > 0 {
		return fmt.Sprintf("%d+unknown(%d series)", *t.Sum, t.Unknown)
	}
	return fmt.Sprintf("%d", *t.Sum)
}

func points(pt *Points) string {
	if pt == nil {
		return ""
	}
	m, e := "none", "none"
	if pt.MeasuredSum != nil {
		m = fmt.Sprint(*pt.MeasuredSum)
	}
	if pt.EstimatedSum != nil {
		e = fmt.Sprint(*pt.EstimatedSum)
	}
	return fmt.Sprintf(" points measured=%s (n=%d) estimated=%s (n=%d) unknown=%d unsupported=%d failed=%d",
		m, pt.MeasuredSamples, e, pt.EstimatedSamples, pt.Unknown, pt.Unsupported, pt.Failed)
}
