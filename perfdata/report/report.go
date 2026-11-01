// Package report builds the offline local performance report from stored
// performance-event-v1 records, the per-source coverage manifest and retained daily
// rollups. It is a pure function of its input: no I/O, no clock, no network. Rerunning
// it over the same input gives the same output.
//
// The rules it enforces are the contract's counting rules:
//
//   - Distributions are pooled, never averaged. Daily means and percentiles are never
//     combined; the pooled distribution is built first and read once.
//   - Elapsed time is the union of intervals; summed cost is reported beside it, never
//     instead of it. Only own CPU is summed; inclusive CPU is never added to children.
//   - A cumulative usage snapshot replaces the previous one of its series; deltas add.
//   - Missing is not zero: unknown values are counted as unknown, never as 0, and a day
//     or a service side with no data is reported as unknown.
//   - Background refresh and poll requests are a shared, unallocated bucket.
package report

import (
	"sort"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

// Schema is the report schema identifier.
const Schema = "performance-report-v1"

// Input is everything one report reads.
type Input struct {
	Events   []perfdata.Event
	Coverage []perfdata.SourceCoverage
	Rollups  []perfdata.DailyRollup
	// Cutoff is the snapshot time; PolicyRevision the active policy's revision, if any.
	Cutoff         time.Time
	PolicyRevision string
	// Window bounds the reported days (inclusive, YYYY-MM-DD). Empty bounds are taken
	// from the data.
	Window perfdata.Window
}

// Report is the offline report.
type Report struct {
	Schema      string            `json:"schema"`
	Snapshot    perfdata.Snapshot `json:"snapshot"`
	State       string            `json:"state"`
	Days        []Day             `json:"days"`
	Tools       []ToolRow         `json:"tools"`
	ToolDays    []ToolRow         `json:"toolDays"`
	Cost        CostSummary       `json:"cost"`
	Phases      []PhaseRow        `json:"phases"`
	Invocations Invocations       `json:"invocations"`
	Attempts    Attempts          `json:"attempts"`
	Usage       []UsageRow        `json:"usage"`
	HTTP        []HTTPRow         `json:"http"`
	Services    []ServiceRow      `json:"services"`
	Shared      Upstream          `json:"sharedUpstream"`
	Collection  Collection        `json:"collection"`
}

// Day states.
const (
	DayMeasured = "measured"  // raw events present
	DayRollup   = "rolled-up" // only a retained daily rollup
	DayRetired  = "retired"   // retired without a rollup: counted loss, values unknown
	DayUnknown  = "unknown"   // nothing recorded: unknown, never zero
)

// Day is one reported day.
type Day struct {
	Day   string `json:"day"`
	State string `json:"state"`
	Runs  int64  `json:"runs"`
}

// Stat is a pooled distribution summary with explicit known and unknown counts.
type Stat struct {
	Known   int64    `json:"known"`
	Unknown int64    `json:"unknown"`
	AvgMs   *float64 `json:"avgMs,omitempty"`
	P95Ms   *int64   `json:"p95Ms,omitempty"`
	MaxMs   *int64   `json:"maxMs,omitempty"`
	Method  string   `json:"method,omitempty"`
}

func statOf(d perfdata.Distribution, unknown int64) Stat {
	s := Stat{Known: d.Count, Unknown: unknown}
	if m, ok := d.Mean(); ok {
		m = float64(int64(m*100+0.5)) / 100
		s.AvgMs = &m
	}
	if p, method, ok := d.Percentile(95); ok {
		s.P95Ms, s.Method = &p, method
		mx := d.MaxMs
		s.MaxMs = &mx
	}
	return s
}

// ToolRow is one pooled tool row; Day is empty for the window-pooled rows.
type ToolRow struct {
	Day      string            `json:"day,omitempty"`
	Tool     string            `json:"tool"`
	Verb     string            `json:"verb,omitempty"`
	DeskRole perfdata.DeskRole `json:"deskRole,omitempty"`
	Version  string            `json:"version"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	Child    bool              `json:"child,omitempty"`
	Runs     int64             `json:"runs"`
	Wall     Stat              `json:"wall"`
	CPUOwn   Stat              `json:"cpuOwn"`
}

// CostSummary contrasts elapsed time (union of intervals) with summed cost, over the
// deduplicated tool completions in the window.
type CostSummary struct {
	Invocations    int   `json:"invocations"`
	ElapsedMs      int64 `json:"elapsedMs"`
	SummedWallMs   int64 `json:"summedWallMs"`
	WallUnknown    int   `json:"wallUnknown"`
	CPUOwnMs       int64 `json:"cpuOwnMs"`
	CPUOwnKnown    int   `json:"cpuOwnKnown"`
	CPUOwnUnknown  int   `json:"cpuOwnUnknown"`
	ChildrenUninst int   `json:"childrenUninstrumented"`
}

// PhaseRow is one phase's elapsed and summed cost.
type PhaseRow struct {
	Phase         perfdata.PhaseName `json:"phase"`
	Intervals     int                `json:"intervals"`
	ElapsedMs     int64              `json:"elapsedMs"`
	SummedWallMs  int64              `json:"summedWallMs"`
	WallUnknown   int                `json:"wallUnknown"`
	CPUOwnMs      int64              `json:"cpuOwnMs"`
	CPUOwnUnknown int                `json:"cpuOwnUnknown"`
}

// Invocations counts deduplication and corrections.
type Invocations struct {
	Distinct             int `json:"distinct"`
	DuplicateSummaries   int `json:"duplicateSummaries"`
	CorrectionsApplied   int `json:"correctionsApplied"`
	CorrectionsUnapplied int `json:"correctionsUnapplied"`
}

// Attempts summarizes attempt lifecycles.
type Attempts struct {
	Started        int `json:"started"`
	Finished       int `json:"finished"`
	Abandoned      int `json:"abandoned"`
	Unfinished     int `json:"unfinished"`
	OrphanTerminal int `json:"orphanTerminal"`
}

// Tokens is a token total over the series that know it, with the unknown series count.
type Tokens struct {
	Sum     *int64 `json:"sum,omitempty"`
	Known   int    `json:"known"`
	Unknown int    `json:"unknown"`
}

func (t *Tokens) add(v *int64) {
	if v == nil {
		t.Unknown++
		return
	}
	if t.Sum == nil {
		t.Sum = new(int64)
	}
	*t.Sum += *v
	t.Known++
}

// CostTotal is a cost total for one currency, pricing revision and basis; amounts with
// different keys are never added together.
type CostTotal struct {
	Currency        string         `json:"currency"`
	PricingRevision string         `json:"pricingRevision"`
	Basis           perfdata.Basis `json:"basis"`
	MicroUnits      int64          `json:"microUnits"`
	Series          int            `json:"series"`
}

// UsageRow is model usage per model over its series (invocation + model).
type UsageRow struct {
	Model        string      `json:"model"`
	Series       int         `json:"series"`
	MixedSeries  int         `json:"mixedSeries"`
	Input        Tokens      `json:"inputTokens"`
	Output       Tokens      `json:"outputTokens"`
	CacheRead    Tokens      `json:"cacheReadTokens"`
	CacheWrite   Tokens      `json:"cacheWriteTokens"`
	Cost         []CostTotal `json:"cost"`
	CostUnknown  int         `json:"costUnknown"`
	SnapshotsCut int         `json:"supersededSnapshots"`
}

// Points is pooled GraphQL point coverage; measured and estimated never mix.
type Points struct {
	Unit             perfdata.PointUnit `json:"unit"`
	MeasuredSum      *int64             `json:"measuredSum,omitempty"`
	MeasuredSamples  int                `json:"measuredSamples"`
	EstimatedSum     *int64             `json:"estimatedSum,omitempty"`
	EstimatedSamples int                `json:"estimatedSamples"`
	Unknown          int                `json:"unknown"`
	Unsupported      int                `json:"unsupported"`
	Failed           int                `json:"failed"`
}

// HTTPRow is actual HTTP attempts per forge, endpoint class, origin and allocation.
type HTTPRow struct {
	Forge       perfdata.Forge         `json:"forge"`
	Class       perfdata.EndpointClass `json:"class"`
	Origin      perfdata.Origin        `json:"origin"`
	Allocation  string                 `json:"allocation"` // own | shared-unallocated
	Attempts    int                    `json:"attempts"`
	Retries     int                    `json:"retries"`
	NotModified int                    `json:"notModified"`
	Failed      int                    `json:"failed"`
	Duration    Stat                   `json:"duration"`
	Points      *Points                `json:"points,omitempty"`
}

// Allocations.
const (
	AllocOwn    = "own"
	AllocShared = "shared-unallocated"
)

// ClientSide is the client view of a service operation.
type ClientSide struct {
	LogicalCalls      int                   `json:"logicalCalls"`
	TransportAttempts int                   `json:"transportAttempts"`
	Retries           int                   `json:"retries"`
	Outcomes          perfdata.CallOutcomes `json:"outcomes"`
	Duration          Stat                  `json:"duration"`
}

// ServerSide is the handler view of a service operation.
type ServerSide struct {
	Requests   int                    `json:"requests"`
	Cache      perfdata.CacheOutcomes `json:"cache"`
	Coalescing perfdata.Coalescing    `json:"coalescing"`
	Duration   Stat                   `json:"duration"`
}

// Upstream is provider attempts made on behalf of service work. Known is false when the
// handler side is missing or any handler left its upstream seams unobserved; then the
// counts are a lower bound and never a measured zero.
type Upstream struct {
	Known    bool           `json:"known"`
	Attempts int            `json:"attempts"`
	ByOrigin map[string]int `json:"byOrigin"`
	Points   *Points        `json:"points,omitempty"`
	Reason   string         `json:"reason,omitempty"`
}

// ServiceRow keeps the client and server sides of one operation apart.
type ServiceRow struct {
	Operation string      `json:"operation"`
	Client    *ClientSide `json:"client"`
	Server    *ServerSide `json:"server"`
	Upstream  Upstream    `json:"upstream"`
}

// Collection is collection health and recorded loss.
type Collection struct {
	Drops          perfdata.Drops `json:"drops"`
	RetiredRecords int            `json:"retiredRecords"`
	RetiredEvents  int            `json:"retiredEvents"`
	WriteLatency   Stat           `json:"writeLatency"`
	Duplicates     int            `json:"duplicateRecords"`
}

// Build computes the report.
func Build(in Input) Report {
	r := Report{Schema: Schema, Days: []Day{}, Tools: []ToolRow{}, ToolDays: []ToolRow{}, Phases: []PhaseRow{},
		Usage: []UsageRow{}, HTTP: []HTTPRow{}, Services: []ServiceRow{}}
	events := sortEvents(in.Events)
	events, r.Invocations.CorrectionsApplied, r.Invocations.CorrectionsUnapplied = applyCorrections(events)

	type typed struct {
		e perfdata.Event
		v any
	}
	var ts []typed
	for _, e := range events {
		v, err := perfdata.DecodePayload(e.Kind, e.Payload)
		if err != nil {
			continue // DecodeEvent already validated stored events; foreign input is skipped
		}
		ts = append(ts, typed{e, v})
	}

	// Tool completions: one summary per invocation.
	seenInv := map[string]bool{}
	daily := map[string]*perfdata.DailyRollup{}
	var tcs []toolEvent
	for _, t := range ts {
		tc, ok := t.v.(*perfdata.ToolCompletion)
		if !ok {
			continue
		}
		if inv := invocation(t.e); inv != "" {
			if seenInv[inv] {
				r.Invocations.DuplicateSummaries++
				continue
			}
			seenInv[inv] = true
		}
		tcs = append(tcs, toolEvent{t.e, *tc})
		day := perfdata.DayOf(tc.StartedAt)
		dr := daily[day]
		if dr == nil {
			nr := perfdata.NewRollup(day)
			dr = &nr
			daily[day] = dr
		}
		dr.AddEvent(t.e, *tc)
	}
	r.Invocations.Distinct = len(tcs)
	rawDays := map[string]bool{}
	for d := range daily {
		rawDays[d] = true
	}
	rollDays := map[string]bool{}
	for _, ro := range in.Rollups {
		rollDays[ro.Day] = true
		if dr := daily[ro.Day]; dr != nil {
			_ = dr.MergeRollup(ro)
		} else {
			c := perfdata.NewRollup(ro.Day)
			_ = c.MergeRollup(ro)
			daily[ro.Day] = &c
		}
	}
	r.Tools, r.ToolDays = toolRows(daily)
	r.Cost = costOf(tcs)

	var phases []perfdata.Phase
	attempts := map[string]map[perfdata.AttemptState]bool{}
	usage := map[string][]usageEvent{}
	httpRows := map[string]*HTTPRow{}
	httpDist := map[string]perfdata.Distribution{}
	clients := map[string]*ClientSide{}
	clientDur := map[string]perfdata.Distribution{}
	servers := map[string]*ServerSide{}
	serverDur := map[string]perfdata.Distribution{}
	handlerInvOps := map[string]map[string]bool{} // server invocation -> operations
	unobserved := map[string]bool{}
	var passthrough []httpEvent
	shared := Upstream{Known: true, ByOrigin: map[string]int{}}
	latency := perfdata.NewDistribution()
	for _, t := range ts {
		switch v := t.v.(type) {
		case *perfdata.Phase:
			phases = append(phases, *v)
		case *perfdata.Attempt:
			a := t.e.Ctx.Attempt
			if attempts[a] == nil {
				attempts[a] = map[perfdata.AttemptState]bool{}
			}
			attempts[a][v.State] = true
		case *perfdata.Usage:
			k := invocation(t.e) + "\x00" + v.Model
			usage[k] = append(usage[k], usageEvent{t.e, *v})
		case *perfdata.HTTPAttempts:
			alloc := AllocOwn
			if v.Origin == "poll" || v.Origin == "refresh" || (t.e.Ctx != nil && t.e.Ctx.SharedRefresh != "") {
				alloc = AllocShared
			}
			k := string(v.Forge) + "\x00" + string(v.Class) + "\x00" + string(v.Origin) + "\x00" + alloc
			row := httpRows[k]
			if row == nil {
				row = &HTTPRow{Forge: v.Forge, Class: v.Class, Origin: v.Origin, Allocation: alloc}
				httpRows[k] = row
			}
			row.Attempts += v.Attempts
			row.Retries += v.Retries
			row.NotModified += v.NotModified
			row.Failed += v.Failed
			row.Points = addPoints(row.Points, v.Points)
			h := httpEvent{t.e, *v}
			httpDist[k], _ = perfdata.Merge(httpDist[k], v.Duration, perfdata.RollupSampleCap)
			switch {
			case alloc == AllocShared:
				shared.Attempts += v.Attempts
				shared.ByOrigin[string(v.Origin)] += v.Attempts
				shared.Points = addPoints(shared.Points, v.Points)
			case v.Origin == "service-passthrough" || v.Origin == "repair":
				passthrough = append(passthrough, h)
			}
		case *perfdata.ServiceCall:
			c := clients[v.Operation]
			if c == nil {
				c = &ClientSide{}
				clients[v.Operation] = c
			}
			c.LogicalCalls += v.LogicalCalls
			c.TransportAttempts += v.TransportAttempts
			c.Retries += v.Retries
			c.Outcomes.OK += v.Outcomes.OK
			c.Outcomes.Error += v.Outcomes.Error
			c.Outcomes.Timeout += v.Outcomes.Timeout
			c.Outcomes.Cancelled += v.Outcomes.Cancelled
			c.Outcomes.Denied += v.Outcomes.Denied
			clientDur[v.Operation], _ = perfdata.Merge(clientDur[v.Operation], v.Duration, perfdata.RollupSampleCap)
		case *perfdata.Handler:
			s := servers[v.Operation]
			if s == nil {
				s = &ServerSide{}
				servers[v.Operation] = s
			}
			s.Requests += v.Requests
			s.Cache.Hit += v.Cache.Hit
			s.Cache.Miss += v.Cache.Miss
			s.Cache.Stale += v.Cache.Stale
			s.Cache.Bypass += v.Cache.Bypass
			s.Cache.Unavailable += v.Cache.Unavailable
			s.Coalescing.Leader += v.Coalescing.Leader
			s.Coalescing.Waiter += v.Coalescing.Waiter
			serverDur[v.Operation], _ = perfdata.Merge(serverDur[v.Operation], v.Duration, perfdata.RollupSampleCap)
			inv := invocation(t.e)
			if handlerInvOps[inv] == nil {
				handlerInvOps[inv] = map[string]bool{}
			}
			handlerInvOps[inv][v.Operation] = true
			if v.Upstream != perfdata.UpstreamObserved {
				unobserved[v.Operation] = true
			}
		case *perfdata.Health:
			latency, _ = perfdata.Merge(latency, v.WriteLatency, perfdata.RollupSampleCap)
		}
	}
	r.Phases = phaseRows(phases)
	r.Attempts = attemptSummary(attempts)
	r.Usage = usageRows(usage)
	r.HTTP = httpRowList(httpRows, httpDist)
	r.Services, r.Shared = serviceRows(clients, clientDur, servers, serverDur, handlerInvOps, unobserved, passthrough, shared)

	// Collection, coverage and days.
	r.Collection.WriteLatency = statOf(latency, 0)
	for _, c := range in.Coverage {
		r.Collection.Drops.Add(c.Drops)
		for _, rt := range c.Retired {
			r.Collection.RetiredRecords++
			r.Collection.RetiredEvents += rt.Events
		}
	}
	r.Collection.Duplicates = len(in.Events) - len(dedupeIDs(in.Events))
	r.State = overallState(in.Coverage)
	win := in.Window
	r.Days = days(win, rawDays, rollDays, daily, in.Coverage, &win)
	r.Snapshot = perfdata.Snapshot{Schema: perfdata.SnapshotSchema, Cutoff: perfdata.FormatTime(in.Cutoff), Timezone: "UTC",
		Window: win, PolicyRevision: in.PolicyRevision,
		Instrument: perfdata.Instrument{Name: perfdata.InstrumentName, Version: perfdata.InstrumentVersion},
		Sources:    append([]perfdata.SourceCoverage{}, in.Coverage...), Reports: perfdata.SnapshotReports()}
	return r
}

type toolEvent struct {
	e  perfdata.Event
	tc perfdata.ToolCompletion
}

type usageEvent struct {
	e perfdata.Event
	u perfdata.Usage
}

type httpEvent struct {
	e perfdata.Event
	h perfdata.HTTPAttempts
}

func invocation(e perfdata.Event) string {
	if e.Ctx == nil {
		return ""
	}
	return e.Ctx.Invocation
}

// sortEvents orders events by identity so the report is deterministic, dropping exact
// identity repeats (the reader already counted them).
func sortEvents(in []perfdata.Event) []perfdata.Event {
	out := dedupeIDs(in)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Epoch != b.Epoch {
			return a.Epoch < b.Epoch
		}
		return a.Seq < b.Seq
	})
	return out
}

func dedupeIDs(in []perfdata.Event) []perfdata.Event {
	seen := map[string]bool{}
	out := make([]perfdata.Event, 0, len(in))
	for _, e := range in {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		out = append(out, e)
	}
	return out
}

// applyCorrections excludes every event a correction names when the correction's
// digest matches the stored record; a correction whose target is absent (retired, or
// never recorded) or whose digest differs is counted as unapplied.
func applyCorrections(events []perfdata.Event) ([]perfdata.Event, int, int) {
	byID := map[string]perfdata.Event{}
	for _, e := range events {
		byID[e.ID] = e
	}
	excluded := map[string]bool{}
	applied, unapplied := 0, 0
	for _, e := range events {
		if e.Kind != perfdata.KindCorrection {
			continue
		}
		v, err := perfdata.DecodePayload(e.Kind, e.Payload)
		if err != nil {
			continue
		}
		c := v.(*perfdata.Correction)
		t, ok := byID[c.Corrects]
		if !ok {
			unapplied++
			continue
		}
		b, err := t.Encode()
		if err != nil || perfdata.Digest(b) != c.Digest {
			unapplied++
			continue
		}
		if !excluded[c.Corrects] {
			excluded[c.Corrects] = true
			applied++
		}
	}
	out := events[:0:0]
	for _, e := range events {
		if !excluded[e.ID] && e.Kind != perfdata.KindCorrection {
			out = append(out, e)
		}
	}
	return out, applied, unapplied
}

func toolRows(daily map[string]*perfdata.DailyRollup) (pooled, perDay []ToolRow) {
	pooled, perDay = []ToolRow{}, []ToolRow{}
	all := perfdata.NewRollup("0000-01-01")
	dayKeys := make([]string, 0, len(daily))
	for d := range daily {
		dayKeys = append(dayKeys, d)
	}
	sort.Strings(dayKeys)
	for _, d := range dayKeys {
		dr := daily[d]
		for _, row := range dr.Rows {
			perDay = append(perDay, rowOf(d, row))
		}
		// Pool across days by re-keying every day's rows into one rollup: the
		// distributions merge (count, sum, bins, samples), and the percentile is read
		// once from the pooled result — never averaged across days.
		cp := *dr
		cp.Day = all.Day
		_ = all.MergeRollup(cp)
	}
	for _, row := range all.Rows {
		pooled = append(pooled, rowOf("", row))
	}
	return pooled, perDay
}

func rowOf(day string, r perfdata.RollupRow) ToolRow {
	return ToolRow{Day: day, Tool: r.Tool, Verb: r.Verb, DeskRole: r.DeskRole, Version: r.Version, OS: r.OS, Arch: r.Arch,
		Child: r.Child, Runs: r.Runs, Wall: statOf(r.Wall, r.WallUnknown), CPUOwn: statOf(r.CPUOwn, r.CPUUnknown)}
}

type interval struct{ a, b int64 }

// union returns the total length covered by the intervals.
func union(iv []interval) int64 {
	if len(iv) == 0 {
		return 0
	}
	sort.Slice(iv, func(i, j int) bool { return iv[i].a < iv[j].a })
	var total int64
	cur := iv[0]
	for _, x := range iv[1:] {
		if x.a <= cur.b {
			cur.b = max(cur.b, x.b)
			continue
		}
		total += cur.b - cur.a
		cur = x
	}
	return total + cur.b - cur.a
}

func intervalOf(startedAt string, wall perfdata.Measurement) (interval, bool) {
	t, ok := perfdata.ParseTime(startedAt)
	if !ok || !wall.Known() {
		return interval{}, false
	}
	a := t.UnixMilli()
	return interval{a, a + *wall.Ms}, true
}

func costOf(tcs []toolEvent) CostSummary {
	var c CostSummary
	var iv []interval
	for _, t := range tcs {
		c.Invocations++
		if x, ok := intervalOf(t.tc.StartedAt, t.tc.Wall); ok {
			iv = append(iv, x)
			c.SummedWallMs += *t.tc.Wall.Ms
		} else {
			c.WallUnknown++
		}
		// Own CPU only: a parent's inclusive CPU already contains its children's own CPU.
		if t.tc.CPU.Own.Known() {
			c.CPUOwnMs += *t.tc.CPU.Own.Ms
			c.CPUOwnKnown++
		} else {
			c.CPUOwnUnknown++
		}
		c.ChildrenUninst += t.tc.Children.Uninstrumented
	}
	c.ElapsedMs = union(iv)
	return c
}

func phaseRows(ps []perfdata.Phase) []PhaseRow {
	by := map[perfdata.PhaseName]*PhaseRow{}
	ivs := map[perfdata.PhaseName][]interval{}
	for _, p := range ps {
		row := by[p.Phase]
		if row == nil {
			row = &PhaseRow{Phase: p.Phase}
			by[p.Phase] = row
		}
		row.Intervals++
		if x, ok := intervalOf(p.StartedAt, p.Wall); ok {
			ivs[p.Phase] = append(ivs[p.Phase], x)
			row.SummedWallMs += *p.Wall.Ms
		} else {
			row.WallUnknown++
		}
		if p.CPUOwn.Known() {
			row.CPUOwnMs += *p.CPUOwn.Ms
		} else {
			row.CPUOwnUnknown++
		}
	}
	out := []PhaseRow{}
	for k, row := range by {
		row.ElapsedMs = union(ivs[k])
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Phase < out[j].Phase })
	return out
}

func attemptSummary(m map[string]map[perfdata.AttemptState]bool) Attempts {
	var a Attempts
	for _, st := range m {
		started := st[perfdata.AttemptStarted]
		terminal := st[perfdata.AttemptFinished] || st[perfdata.AttemptAbandoned]
		if started {
			a.Started++
		}
		if st[perfdata.AttemptFinished] {
			a.Finished++
		} else if st[perfdata.AttemptAbandoned] {
			a.Abandoned++
		}
		switch {
		case started && !terminal:
			a.Unfinished++
		case !started && terminal:
			a.OrphanTerminal++
		}
	}
	return a
}

// usageRows reduces each series: the latest cumulative snapshot stands for the series
// (earlier snapshots are superseded, never added); deltas add; a series mixing both
// modes is counted as mixed and its values are unknown.
func usageRows(series map[string][]usageEvent) []UsageRow {
	by := map[string]*UsageRow{}
	costs := map[string]map[string]*CostTotal{}
	keys := make([]string, 0, len(series))
	for k := range series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		evs := series[k]
		model := evs[0].u.Model
		row := by[model]
		if row == nil {
			row = &UsageRow{Model: model, Cost: []CostTotal{}}
			by[model] = row
			costs[model] = map[string]*CostTotal{}
		}
		row.Series++
		modes := map[perfdata.UsageMode]bool{}
		for _, ue := range evs {
			modes[ue.u.Mode] = true
		}
		var in, out, cr, cw *int64
		var cost []perfdata.Cost
		costKnown := true
		switch {
		case len(modes) > 1:
			row.MixedSeries++
			costKnown = false
		case modes[perfdata.UsageCumulative]:
			latest := evs[0]
			for _, ue := range evs[1:] {
				if later(ue.e, latest.e) {
					latest = ue
				}
			}
			row.SnapshotsCut += len(evs) - 1
			in, out, cr, cw = latest.u.InputTokens, latest.u.OutputTokens, latest.u.CacheReadTokens, latest.u.CacheWriteTokens
			if latest.u.Cost != nil {
				cost = []perfdata.Cost{*latest.u.Cost}
			} else {
				costKnown = false
			}
		default:
			in, out, cr, cw = sumDelta(evs, func(u perfdata.Usage) *int64 { return u.InputTokens }),
				sumDelta(evs, func(u perfdata.Usage) *int64 { return u.OutputTokens }),
				sumDelta(evs, func(u perfdata.Usage) *int64 { return u.CacheReadTokens }),
				sumDelta(evs, func(u perfdata.Usage) *int64 { return u.CacheWriteTokens })
			for _, ue := range evs {
				if ue.u.Cost == nil {
					costKnown = false
					break
				}
				cost = append(cost, *ue.u.Cost)
			}
		}
		row.Input.add(in)
		row.Output.add(out)
		row.CacheRead.add(cr)
		row.CacheWrite.add(cw)
		if !costKnown {
			row.CostUnknown++
			continue
		}
		counted := map[string]bool{}
		for _, c := range cost {
			ck := c.Currency + "\x00" + c.PricingRevision + "\x00" + string(c.Basis)
			t := costs[model][ck]
			if t == nil {
				t = &CostTotal{Currency: c.Currency, PricingRevision: c.PricingRevision, Basis: c.Basis}
				costs[model][ck] = t
			}
			t.MicroUnits += c.MicroUnits
			if !counted[ck] {
				counted[ck] = true
				t.Series++
			}
		}
	}
	out := []UsageRow{}
	for m, row := range by {
		ck := make([]string, 0, len(costs[m]))
		for k := range costs[m] {
			ck = append(ck, k)
		}
		sort.Strings(ck)
		for _, k := range ck {
			row.Cost = append(row.Cost, *costs[m][k])
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

func later(a, b perfdata.Event) bool {
	if a.OccurredAt != b.OccurredAt {
		return a.OccurredAt > b.OccurredAt
	}
	if a.Epoch != b.Epoch {
		return a.Epoch > b.Epoch
	}
	return a.Seq > b.Seq
}

func sumDelta(evs []usageEvent, f func(perfdata.Usage) *int64) *int64 {
	var s int64
	for _, ue := range evs {
		v := f(ue.u)
		if v == nil {
			return nil // one unknown delta makes the series total unknown
		}
		s += *v
	}
	return &s
}

func addPoints(acc *Points, p *perfdata.Points) *Points {
	if p == nil {
		return acc
	}
	if acc == nil {
		acc = &Points{Unit: p.Unit}
	}
	addSum := func(dst **int64, v *int64) {
		if v == nil {
			return
		}
		if *dst == nil {
			*dst = new(int64)
		}
		**dst += *v
	}
	addSum(&acc.MeasuredSum, p.MeasuredSum)
	addSum(&acc.EstimatedSum, p.EstimatedSum)
	acc.MeasuredSamples += p.MeasuredSamples
	acc.EstimatedSamples += p.EstimatedSamples
	acc.Unknown += p.UnknownSamples
	acc.Unsupported += p.UnsupportedSamples
	acc.Failed += p.FailedSamples
	return acc
}

func httpRowList(m map[string]*HTTPRow, dist map[string]perfdata.Distribution) []HTTPRow {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]HTTPRow, 0, len(keys))
	for _, k := range keys {
		row := *m[k]
		row.Duration = statOf(dist[k], 0)
		out = append(out, row)
	}
	return out
}

func serviceRows(clients map[string]*ClientSide, clientDur map[string]perfdata.Distribution, servers map[string]*ServerSide,
	serverDur map[string]perfdata.Distribution, handlerInvOps map[string]map[string]bool, unobserved map[string]bool,
	passthrough []httpEvent, shared Upstream) ([]ServiceRow, Upstream) {
	ops := map[string]bool{}
	for k := range clients {
		ops[k] = true
	}
	for k := range servers {
		ops[k] = true
	}
	ups := map[string]*Upstream{}
	for op := range ops {
		u := &Upstream{ByOrigin: map[string]int{}}
		switch {
		case servers[op] == nil:
			u.Reason = "handler-not-observed"
		case unobserved[op]:
			u.Reason = "upstream-not-observed"
		default:
			u.Known = true
		}
		ups[op] = u
	}
	unattributed := 0
	for _, h := range passthrough {
		ops := handlerInvOps[invocation(h.e)]
		if len(ops) != 1 {
			unattributed += h.h.Attempts
			continue
		}
		for op := range ops {
			u := ups[op]
			u.Attempts += h.h.Attempts
			u.ByOrigin[string(h.h.Origin)] += h.h.Attempts
			u.Points = addPoints(u.Points, h.h.Points)
		}
	}
	if unattributed > 0 {
		shared.ByOrigin["unattributed"] += unattributed
		shared.Attempts += unattributed
	}
	keys := make([]string, 0, len(ops))
	for k := range ops {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []ServiceRow{}
	for _, op := range keys {
		row := ServiceRow{Operation: op, Upstream: *ups[op]}
		if c := clients[op]; c != nil {
			cc := *c
			cc.Duration = statOf(clientDur[op], 0)
			row.Client = &cc
		}
		if s := servers[op]; s != nil {
			ss := *s
			ss.Duration = statOf(serverDur[op], 0)
			row.Server = &ss
		}
		out = append(out, row)
	}
	return out, shared
}

func overallState(cov []perfdata.SourceCoverage) string {
	if len(cov) == 0 {
		return perfdata.CoverageMissing
	}
	rank := map[string]int{perfdata.CoverageComplete: 0, perfdata.CoverageEmptyComplete: 0, perfdata.CoveragePartial: 1, perfdata.CoverageCorrupt: 2, perfdata.CoverageMissing: 3}
	worst := cov[0].State
	allEmpty := true
	for _, c := range cov {
		if rank[c.State] > rank[worst] {
			worst = c.State
		}
		if c.State != perfdata.CoverageEmptyComplete {
			allEmpty = false
		}
	}
	if worst == perfdata.CoverageEmptyComplete && !allEmpty {
		return perfdata.CoverageComplete
	}
	if worst == perfdata.CoverageComplete && allEmpty {
		return perfdata.CoverageEmptyComplete
	}
	return worst
}

// days lists every day of the window with its state. A day with neither raw events nor
// a rollup is retired when a recorded retirement covers it and unknown otherwise —
// never a measured zero.
func days(win perfdata.Window, raw, roll map[string]bool, daily map[string]*perfdata.DailyRollup,
	cov []perfdata.SourceCoverage, out *perfdata.Window) []Day {
	var lo, hi string
	for d := range daily {
		if lo == "" || d < lo {
			lo = d
		}
		if d > hi {
			hi = d
		}
	}
	from, to := win.From, win.To
	if from == "" {
		from = lo
	}
	if to == "" {
		to = hi
	}
	*out = perfdata.Window{From: from, To: to}
	res := []Day{}
	if from == "" || to == "" {
		return res
	}
	start, ok1 := perfdata.ParseTime(from + "T00:00:00.000Z")
	end, ok2 := perfdata.ParseTime(to + "T00:00:00.000Z")
	if !ok1 || !ok2 || end.Before(start) || end.Sub(start) > 400*24*time.Hour {
		return res
	}
	retired := map[string]bool{}
	for _, c := range cov {
		for _, r := range c.Retired {
			if r.FromDay == "" {
				continue
			}
			for d := r.FromDay; d <= r.ToDay; d = nextDay(d) {
				retired[d] = true
			}
		}
	}
	for t := start; !t.After(end); t = t.AddDate(0, 0, 1) {
		d := t.Format("2006-01-02")
		day := Day{Day: d}
		switch {
		case raw[d]:
			day.State = DayMeasured
		case roll[d]:
			day.State = DayRollup
		case retired[d]:
			day.State = DayRetired
		default:
			day.State = DayUnknown
		}
		if dr := daily[d]; dr != nil {
			for _, row := range dr.Rows {
				day.Runs += row.Runs
			}
		}
		res = append(res, day)
	}
	return res
}

func nextDay(d string) string {
	t, ok := perfdata.ParseTime(d + "T00:00:00.000Z")
	if !ok {
		return "9999-12-31"
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}
