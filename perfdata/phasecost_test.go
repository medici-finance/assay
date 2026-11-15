package perfdata_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
	"github.com/medici-finance/assay/perfdata/report"
)

// fixture accumulates sealed events with increasing sequence numbers.
type fixture struct {
	t   *testing.T
	seq uint64
	evs []perfdata.Event
}

func (f *fixture) add(k perfdata.Kind, p perfdata.Profile, ctx *perfdata.Context, v any) perfdata.Event {
	f.t.Helper()
	b, _ := json.Marshal(v)
	f.seq++
	e, _, err := seal(f.t, k, p, ctx, b, f.seq)
	if err != nil {
		f.t.Fatalf("fixture %s: %v", k, err)
	}
	f.evs = append(f.evs, e)
	return e
}

func at(offsetMs int64) string {
	return perfdata.FormatTime(t0.Add(time.Duration(offsetMs) * time.Millisecond))
}

func tool(start, wall int64, own, desc, incl perfdata.Measurement) perfdata.ToolCompletion {
	return perfdata.ToolCompletion{Tool: "deskdispatch", Outcome: perfdata.OutcomeOK, StartedAt: at(start), Wall: ms(wall),
		CPU: perfdata.CPU{Own: own, Descendant: desc, Inclusive: incl}}
}

func i64(v int64) *int64 { return &v }

func TestPhaseCostSchema(t *testing.T) {
	un := perfdata.Unavailable()

	t.Run("nested and overlapping invocations: union elapsed, own CPU only", func(t *testing.T) {
		f := &fixture{t: t}
		parent := perfdata.NewAlias()
		pc := &perfdata.Context{Invocation: parent, DeskRole: perfdata.DeskWorker, Role: perfdata.RoleImplement}
		child := func() *perfdata.Context {
			return &perfdata.Context{Invocation: perfdata.NewAlias(), ParentInvocation: parent, DeskRole: perfdata.DeskWorker, Role: perfdata.RoleImplement}
		}
		// Parent 0..1000 with own 100 and inclusive 400 (its children's 300 included).
		p := tool(0, 1000, ms(100), ms(300), ms(400))
		p.Children = perfdata.Children{Started: 3, Uninstrumented: 0}
		f.add(perfdata.KindTool, perfdata.ProfileWorkflow, pc, p)
		f.add(perfdata.KindTool, perfdata.ProfileWorkflow, child(), tool(200, 400, ms(150), un, un)) // 200..600
		f.add(perfdata.KindTool, perfdata.ProfileWorkflow, child(), tool(500, 400, ms(150), un, un)) // 500..900, overlaps
		f.add(perfdata.KindTool, perfdata.ProfileWorkflow, child(), tool(2000, 100, un, un, un))     // disjoint, CPU unknown
		f.add(perfdata.KindPhase, perfdata.ProfileWorkflow, pc, perfdata.Phase{Phase: "page", StartedAt: at(0), Wall: ms(300), CPUOwn: ms(20)})
		f.add(perfdata.KindPhase, perfdata.ProfileWorkflow, pc, perfdata.Phase{Phase: "page", StartedAt: at(100), Wall: ms(300), CPUOwn: un})
		gql := &perfdata.Points{Unit: "github-points", MeasuredSum: i64(5), MeasuredSamples: 1, UnknownSamples: 1}
		f.add(perfdata.KindHTTP, perfdata.ProfileWorkflow, pc, perfdata.HTTPAttempts{Forge: "github", Class: "rest-read", Origin: "direct", Scope: perfdata.ScopeOwn, Attempts: 3, NotModified: 1, Duration: perfdata.DistributionOf(5, 9, 30)})
		f.add(perfdata.KindHTTP, perfdata.ProfileWorkflow, pc, perfdata.HTTPAttempts{Forge: "github", Class: "graphql", Origin: "direct", Scope: perfdata.ScopeOwn, Attempts: 2, Duration: perfdata.DistributionOf(40, 80), Points: gql})

		r := report.Build(report.Input{Events: f.evs, Cutoff: t0})
		c := r.Cost
		if c.Invocations != 4 || c.ElapsedMs != 1100 || c.SummedWallMs != 1900 {
			t.Fatalf("elapsed must be the union (1100) and summed cost separate (1900): %+v", c)
		}
		if c.CPUOwnMs != 400 || c.CPUOwnKnown != 3 || c.CPUOwnUnknown != 1 {
			t.Fatalf("own CPU only (400 over 3 known, 1 unknown), inclusive never added: %+v", c)
		}
		if len(r.Phases) != 1 || r.Phases[0].ElapsedMs != 400 || r.Phases[0].SummedWallMs != 600 || r.Phases[0].CPUOwnMs != 20 || r.Phases[0].CPUOwnUnknown != 1 {
			t.Fatalf("phase union/summed/cpu: %+v", r.Phases)
		}
		var rest, graph *report.HTTPRow
		for i := range r.HTTP {
			switch r.HTTP[i].Class {
			case "rest-read":
				rest = &r.HTTP[i]
			case "graphql":
				graph = &r.HTTP[i]
			}
		}
		if rest == nil || rest.Attempts != 3 || rest.NotModified != 1 || rest.Points != nil {
			t.Fatalf("rest endpoint attempts: %+v", rest)
		}
		if graph == nil || graph.Attempts != 2 || graph.Points == nil || *graph.Points.MeasuredSum != 5 || graph.Points.Unknown != 1 {
			t.Fatalf("graphql quota must keep its unknown sample, never zero it: %+v", graph)
		}
	})

	t.Run("planted invalid shapes are refused", func(t *testing.T) {
		ctx := &perfdata.Context{Invocation: perfdata.NewAlias()}
		cases := []struct {
			name  string
			k     perfdata.Kind
			p     perfdata.Profile
			ctx   *perfdata.Context
			v     any
			class perfdata.RefusalClass
		}{
			{"free text phase", perfdata.KindPhase, perfdata.ProfileTool, ctx, perfdata.Phase{Phase: "waiting on Bob", StartedAt: at(0), Wall: ms(1), CPUOwn: ms(1)}, perfdata.RefusePrivacy},
			{"free text operation", perfdata.KindHandler, perfdata.ProfileTool, ctx, perfdata.Handler{Operation: "GET /x?q=secret", Upstream: perfdata.UpstreamObserved, Duration: perfdata.NewDistribution()}, perfdata.RefusePrivacy},
			{"profile escalation", perfdata.KindTool, perfdata.ProfileTool, &perfdata.Context{Invocation: perfdata.NewAlias(), Agent: perfdata.NewAlias()}, tool(0, 1, ms(1), un, un), perfdata.RefusePrivacy},
			{"attempt under tool profile", perfdata.KindAttempt, perfdata.ProfileTool, ctx, perfdata.Attempt{State: perfdata.AttemptStarted}, perfdata.RefusePrivacy},
			{"unknown as zero", perfdata.KindTool, perfdata.ProfileTool, ctx, tool(0, 1, perfdata.Measurement{Basis: perfdata.BasisUnavailable, Ms: i64(0)}, un, un), perfdata.RefuseMeasurement},
			{"measured without value", perfdata.KindTool, perfdata.ProfileTool, ctx, tool(0, 1, perfdata.Measurement{Basis: perfdata.BasisMeasured}, un, un), perfdata.RefuseMeasurement},
			{"false-zero points sum", perfdata.KindHTTP, perfdata.ProfileTool, ctx, perfdata.HTTPAttempts{Forge: "github", Class: "graphql", Origin: "direct", Scope: perfdata.ScopeOwn, Attempts: 1, Duration: perfdata.NewDistribution(),
				Points: &perfdata.Points{Unit: "github-points", MeasuredSum: i64(0), UnknownSamples: 1}}, perfdata.RefuseMeasurement},
			{"graphql without point coverage", perfdata.KindHTTP, perfdata.ProfileTool, ctx, perfdata.HTTPAttempts{Forge: "github", Class: "graphql", Origin: "direct", Scope: perfdata.ScopeOwn, Attempts: 1, Duration: perfdata.NewDistribution()}, perfdata.RefuseMeasurement},
			{"dropped unknown point sample", perfdata.KindHTTP, perfdata.ProfileTool, ctx, perfdata.HTTPAttempts{Forge: "github", Class: "graphql", Origin: "direct", Scope: perfdata.ScopeOwn, Attempts: 2, Duration: perfdata.NewDistribution(),
				Points: &perfdata.Points{Unit: "github-points", MeasuredSum: i64(4), MeasuredSamples: 1}}, perfdata.RefuseMeasurement},
			{"raw locator", perfdata.KindTool, perfdata.ProfileWorkflow, &perfdata.Context{Invocation: "org/repo#12", DeskRole: perfdata.DeskWorker, Role: perfdata.RoleImplement}, tool(0, 1, ms(1), un, un), perfdata.RefusePrivacy},
			{"parent summary carries descendants", perfdata.KindHTTP, perfdata.ProfileTool, ctx, perfdata.HTTPAttempts{Forge: "github", Class: "rest-read", Origin: "direct", Scope: "inclusive", Attempts: 9, Duration: perfdata.NewDistribution()}, perfdata.RefuseMeasurement},
			{"inclusive CPU disagrees", perfdata.KindTool, perfdata.ProfileTool, ctx, tool(0, 1, ms(10), ms(10), ms(40)), perfdata.RefuseMeasurement},
		}
		for _, c := range cases {
			b, _ := json.Marshal(c.v)
			_, _, err := seal(t, c.k, c.p, c.ctx, b, 1)
			if err == nil {
				t.Fatalf("%s: accepted", c.name)
			}
			if got := perfdata.AsRefusal(err).Class; got != c.class {
				t.Fatalf("%s: class %s, want %s (%v)", c.name, got, c.class, err)
			}
		}
	})

	t.Run("query and cache journey keeps client, server and provider apart", func(t *testing.T) {
		f := &fixture{t: t}
		client := &perfdata.Context{Invocation: perfdata.NewAlias()}
		srv := func() *perfdata.Context { return &perfdata.Context{Invocation: perfdata.NewAlias()} }
		call := func(op string, n int, o perfdata.CallOutcomes) perfdata.ServiceCall {
			return perfdata.ServiceCall{Operation: op, LogicalCalls: n, TransportAttempts: n, Outcomes: o, Duration: perfdata.NewDistribution()}
		}
		handler := func(op string, n int, c perfdata.CacheOutcomes, co perfdata.Coalescing) perfdata.Handler {
			return perfdata.Handler{Operation: op, Requests: n, Cache: c, Coalescing: co, Upstream: perfdata.UpstreamObserved, Duration: perfdata.NewDistribution()}
		}
		// 50 client queries coalesce into one 3-page provider walk.
		s1 := srv()
		f.add(perfdata.KindCall, perfdata.ProfileTool, client, call("board-read", 50, perfdata.CallOutcomes{OK: 50}))
		f.add(perfdata.KindHandler, perfdata.ProfileTool, s1, handler("board-read", 50, perfdata.CacheOutcomes{Miss: 50}, perfdata.Coalescing{Leader: 1, Waiter: 49}))
		f.add(perfdata.KindHTTP, perfdata.ProfileTool, s1, perfdata.HTTPAttempts{Forge: "github", Class: "graphql", Origin: "service-passthrough", Scope: perfdata.ScopeOwn, Attempts: 3, Duration: perfdata.DistributionOf(50, 60, 70),
			Points: &perfdata.Points{Unit: "github-points", MeasuredSum: i64(3), MeasuredSamples: 3}})
		// A background poll in the same server process is shared and unallocated.
		poll := &perfdata.Context{Invocation: s1.Invocation, SharedRefresh: perfdata.NewAlias(), DeskRole: perfdata.DeskUnavailable, Role: perfdata.RoleUnavailable}
		f.add(perfdata.KindHTTP, perfdata.ProfileWorkflow, poll, perfdata.HTTPAttempts{Forge: "github", Class: "rest-read", Origin: "poll", Scope: perfdata.ScopeOwn, Attempts: 4, Duration: perfdata.NewDistribution()})
		// A warm hit with observed upstream makes zero provider attempts.
		f.add(perfdata.KindCall, perfdata.ProfileTool, client, call("board-warm", 1, perfdata.CallOutcomes{OK: 1}))
		f.add(perfdata.KindHandler, perfdata.ProfileTool, srv(), handler("board-warm", 1, perfdata.CacheOutcomes{Hit: 1}, perfdata.Coalescing{}))
		// Passthrough plus one retry counts both actual sends.
		s3 := srv()
		f.add(perfdata.KindCall, perfdata.ProfileTool, client, call("issue-get", 1, perfdata.CallOutcomes{OK: 1}))
		f.add(perfdata.KindHandler, perfdata.ProfileTool, s3, handler("issue-get", 1, perfdata.CacheOutcomes{Bypass: 1}, perfdata.Coalescing{}))
		f.add(perfdata.KindHTTP, perfdata.ProfileTool, s3, perfdata.HTTPAttempts{Forge: "github", Class: "rest-read", Origin: "service-passthrough", Scope: perfdata.ScopeOwn, Attempts: 2, Retries: 1, Failed: 1, Duration: perfdata.DistributionOf(10, 12)})
		// No handler data: the server side and upstream are unknown, not zero.
		f.add(perfdata.KindCall, perfdata.ProfileTool, client, call("pr-list", 2, perfdata.CallOutcomes{OK: 2}))
		// A denied request reaches no provider.
		f.add(perfdata.KindCall, perfdata.ProfileTool, client, call("admin-op", 1, perfdata.CallOutcomes{Denied: 1}))
		f.add(perfdata.KindHandler, perfdata.ProfileTool, srv(), handler("admin-op", 1, perfdata.CacheOutcomes{Bypass: 1}, perfdata.Coalescing{}))

		r := report.Build(report.Input{Events: f.evs, Cutoff: t0})
		ops := map[string]report.ServiceRow{}
		for _, s := range r.Services {
			ops[s.Operation] = s
		}
		br := ops["board-read"]
		if br.Client == nil || br.Client.LogicalCalls != 50 || br.Server == nil || br.Server.Coalescing.Waiter != 49 {
			t.Fatalf("board-read sides: %+v", br)
		}
		if !br.Upstream.Known || br.Upstream.Attempts != 3 || br.Upstream.Points == nil || *br.Upstream.Points.MeasuredSum != 3 {
			t.Fatalf("50 coalesced queries are 3 provider sends and 3 points, never multiplied: %+v", br.Upstream)
		}
		if w := ops["board-warm"].Upstream; !w.Known || w.Attempts != 0 {
			t.Fatalf("warm hit with observed upstream must be a measured 0: %+v", w)
		}
		if ig := ops["issue-get"].Upstream; !ig.Known || ig.Attempts != 2 || ig.ByOrigin["service-passthrough"] != 2 {
			t.Fatalf("passthrough plus retry counts 2 actual sends: %+v", ig)
		}
		if r.Shared.Attempts != 4 || r.Shared.ByOrigin["poll"] != 4 {
			t.Fatalf("background poll is its own shared bucket: %+v", r.Shared)
		}
		for _, h := range r.HTTP {
			if h.Origin == "poll" && h.Allocation != report.AllocShared {
				t.Fatalf("poll allocated to an operation: %+v", h)
			}
		}
		pl := ops["pr-list"]
		if pl.Server != nil || pl.Upstream.Known || pl.Client == nil || pl.Client.LogicalCalls != 2 {
			t.Fatalf("missing handler data must leave server and upstream unknown: %+v", pl)
		}
		if ad := ops["admin-op"]; !ad.Upstream.Known || ad.Upstream.Attempts != 0 || ad.Client.Outcomes.Denied != 1 {
			t.Fatalf("denied request has 0 provider attempts: %+v", ad)
		}
		if br.Client.TransportAttempts == br.Upstream.Attempts {
			t.Fatal("client transport attempts and provider sends must stay separate counts")
		}
		// An unobserved upstream seam makes zero attempts unknown, not zero.
		f2 := &fixture{t: t}
		h := handler("board-warm", 1, perfdata.CacheOutcomes{Hit: 1}, perfdata.Coalescing{})
		h.Upstream = perfdata.UpstreamUnobserved
		f2.add(perfdata.KindHandler, perfdata.ProfileTool, srv(), h)
		if u := report.Build(report.Input{Events: f2.evs}).Services[0].Upstream; u.Known {
			t.Fatalf("unobserved upstream reported as known: %+v", u)
		}
	})
}
