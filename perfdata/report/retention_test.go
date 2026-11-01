package report_test

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
	"github.com/medici-finance/assay/perfdata/local"
	"github.com/medici-finance/assay/perfdata/report"
)

var armed = perfdata.Consent{Armed: true, Profile: perfdata.ProfileTool}

func retentionPolicy(t *testing.T, distributionDays int) perfdata.Policy {
	t.Helper()
	p, err := perfdata.DecodePolicy([]byte(fmt.Sprintf(`{"schema":"performance-policy-v1","revision":"r1","profiles":["tool"],`+
		`"tools":["statusgen"],"ageCapDays":1,"byteCap":16777216,"distributionDays":%d}`, distributionDays)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var invN int

func wallDraft(at time.Time, wall int64) perfdata.Draft {
	invN++
	tc := perfdata.ToolCompletion{Tool: "statusgen", Verb: "lint", Outcome: perfdata.OutcomeOK, StartedAt: perfdata.FormatTime(at),
		Wall: perfdata.Measured(wall), CPU: perfdata.CPU{Own: perfdata.Measured(1), Descendant: perfdata.Unavailable(), Inclusive: perfdata.Unavailable()}}
	b, _ := json.Marshal(tc)
	return perfdata.Draft{Kind: perfdata.KindTool, Ctx: &perfdata.RawContext{Invocation: fmt.Sprintf("inv-%d", invN)}, Payload: b}
}

// writeAt opens the store as of now, writes drafts, and closes it cleanly.
func writeAt(t *testing.T, dir string, pol perfdata.Policy, now time.Time, drafts ...perfdata.Draft) {
	t.Helper()
	st, err := local.Open(dir, armed, pol, local.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) > 0 {
		if _, err := st.Write(drafts, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
}

func buildFrom(t *testing.T, dir string, now time.Time, from, to string) report.Report {
	t.Helper()
	rr, err := local.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	return report.Build(report.Input{Events: rr.Events, Coverage: rr.Coverage, Rollups: rr.Rollups, Cutoff: now,
		Window: perfdata.Window{From: from, To: to}})
}

func dayStates(r report.Report) map[string]string {
	m := map[string]string{}
	for _, d := range r.Days {
		m[d.Day] = d.State
	}
	return m
}

func TestReportDistributionRetention(t *testing.T) {
	d1 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

	t.Run("a retired raw day pools with live raw days; percentiles are never averaged", func(t *testing.T) {
		dir := t.TempDir()
		pol := retentionPolicy(t, 30)
		writeAt(t, dir, pol, d1, wallDraft(d1, 1000))
		var nine []perfdata.Draft
		for i := 0; i < 9; i++ {
			nine = append(nine, wallDraft(d3.Add(time.Duration(i)*time.Minute), 10))
		}
		writeAt(t, dir, pol, d3, nine...) // opening on day 3 retires day 1's raw segment into its rollup

		rr, err := local.Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(rr.Rollups) != 1 || rr.Rollups[0].Day != "2026-10-01" || len(rr.Events) != 9 {
			t.Fatalf("day 1 must survive only as a rollup: rollups=%+v events=%d", rr.Rollups, len(rr.Events))
		}
		r := buildFrom(t, dir, d3, "2026-10-01", "2026-10-03")
		want := map[string]string{"2026-10-01": report.DayRollup, "2026-10-02": report.DayUnknown, "2026-10-03": report.DayMeasured}
		if got := dayStates(r); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("day states: got %v want %v", got, want)
		}
		if len(r.Tools) != 1 {
			t.Fatalf("one pooled tool row expected: %+v", r.Tools)
		}
		w := r.Tools[0].Wall
		if r.Tools[0].Runs != 10 || w.Known != 10 || w.AvgMs == nil || *w.AvgMs != 109 {
			t.Fatalf("pooled mean must weight by count (109ms), not average daily means (505ms): %+v avg=%v", r.Tools[0], deref(w.AvgMs))
		}
		if w.P95Ms == nil || *w.P95Ms != 1000 || w.Method != perfdata.MethodExact {
			t.Fatalf("pooled p95 must be read from the pooled samples (1000ms exact), not averaged across days: p95=%v method=%q", derefI(w.P95Ms), w.Method)
		}
		perDay := map[string]float64{}
		for _, td := range r.ToolDays {
			perDay[td.Day] = deref(td.Wall.AvgMs)
		}
		if perDay["2026-10-01"] != 1000 || perDay["2026-10-03"] != 10 {
			t.Fatalf("per-day rows keep their own day: %v", perDay)
		}
		var runs int64
		for _, d := range r.Days {
			if d.Day == "2026-10-02" && d.Runs != 0 {
				t.Fatalf("an unrecorded day has no runs: %+v", d)
			}
			runs += d.Runs
		}
		if runs != 10 {
			t.Fatalf("day runs must total 10: %+v", r.Days)
		}
	})

	t.Run("an expired rollup reads as retired, never as a zero day", func(t *testing.T) {
		dir := t.TempDir()
		pol := retentionPolicy(t, 2)
		writeAt(t, dir, pol, d1, wallDraft(d1, 1000))
		writeAt(t, dir, pol, d3, wallDraft(d3, 10)) // day 1 is rolled up, then its rollup expires (2-day retention)
		r := buildFrom(t, dir, d3, "2026-10-01", "2026-10-03")
		if got := dayStates(r)["2026-10-01"]; got != report.DayRetired {
			t.Fatalf("expired distribution day must read retired (counted loss), got %q in %+v", got, r.Days)
		}
		if len(r.Tools) != 1 || r.Tools[0].Runs != 1 || deref(r.Tools[0].Wall.AvgMs) != 10 {
			t.Fatalf("a retired day contributes no values: %+v", r.Tools)
		}
	})

	t.Run("without distribution retention a retired day is retired, not rolled up", func(t *testing.T) {
		dir := t.TempDir()
		pol := retentionPolicy(t, 0)
		writeAt(t, dir, pol, d1, wallDraft(d1, 1000))
		writeAt(t, dir, pol, d3, wallDraft(d3, 10))
		r := buildFrom(t, dir, d3, "2026-10-01", "2026-10-03")
		if got := dayStates(r); got["2026-10-01"] != report.DayRetired || got["2026-10-02"] != report.DayUnknown {
			t.Fatalf("day states: %v", got)
		}
	})

	t.Run("beyond the sample cap the percentile is a named bin estimate within its bin", func(t *testing.T) {
		mk := func(day string, seed int64) perfdata.DailyRollup {
			r := perfdata.NewRollup(day)
			var ms []int64
			for i := int64(0); i < 3000; i++ {
				ms = append(ms, 1+(i*seed)%2000)
			}
			r.Rows = []perfdata.RollupRow{{Tool: "statusgen", Verb: "lint", Version: "v1", OS: "linux", Arch: "amd64", Runs: 3000,
				Wall: perfdata.DistributionOf(ms...), CPUOwn: perfdata.DistributionOf(ms...)}}
			return r
		}
		a, b := mk("2026-10-01", 7), mk("2026-10-02", 13)
		all := append(append([]int64(nil), a.Rows[0].Wall.Samples...), b.Rows[0].Wall.Samples...)
		sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
		trueP95 := all[int(math.Ceil(0.95*float64(len(all))))-1]
		var sum int64
		for _, v := range all {
			sum += v
		}
		r := report.Build(report.Input{Rollups: []perfdata.DailyRollup{a, b}, Cutoff: d3, Window: perfdata.Window{From: "2026-10-01", To: "2026-10-02"}})
		if len(r.Tools) != 1 {
			t.Fatalf("one pooled row expected: %+v", r.Tools)
		}
		if r.State != perfdata.CoverageMissing {
			t.Fatalf("rollups without a coverage manifest cannot claim complete coverage: %q", r.State)
		}
		w := r.Tools[0].Wall
		if w.Known != 6000 || w.Method != perfdata.MethodEstimate || w.P95Ms == nil {
			t.Fatalf("6000 samples exceed the cap: the p95 must be a named estimate: %+v", w)
		}
		lo, hi := perfdata.BinBounds(perfdata.Bin(trueP95))
		if *w.P95Ms < lo || *w.P95Ms >= hi {
			t.Fatalf("estimate %d is outside the true p95 %d's bin [%d,%d)", *w.P95Ms, trueP95, lo, hi)
		}
		wantAvg := float64(int64(float64(sum)/6000*100+0.5)) / 100
		if deref(w.AvgMs) != wantAvg {
			t.Fatalf("the mean stays exact past the cap: got %v want %v", deref(w.AvgMs), wantAvg)
		}
		for _, d := range r.Days {
			if d.State != report.DayRollup || d.Runs != 3000 {
				t.Fatalf("rollup-only days: %+v", r.Days)
			}
		}
	})
}

func deref(p *float64) float64 {
	if p == nil {
		return math.NaN()
	}
	return *p
}

func derefI(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
