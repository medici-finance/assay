package deskkit

import (
	"encoding/json"
	"github.com/medici-finance/assay/perfrec"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func flowContext() perfrec.Context {
	return perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "workflow", Present: true}, LookupEnv: func(k string) (string, bool) {
		if k == perfrec.EnvVar {
			return "1", true
		}
		return "", false
	}})
}
func flowSession(k perfrec.ObservationKind) *perfrec.Session {
	return perfrec.BeginObservation(perfrec.Probe{}, nil, flowContext(), k)
}
func flowQuery(k perfrec.ObservationKind, source, coal string, upstream *int64) perfrec.Record {
	s := flowSession(k)
	q := perfrec.QueryObservation{Operation: "query", Source: source, Coalescing: coal, Result: "ok", ProviderCoverage: perfrec.CoverageMissing}
	if k == perfrec.ObservationClient {
		q.LogicalCalls = 1
		q.TransportAttempts = 1
	} else if upstream != nil {
		q.ProviderCoverage = perfrec.CoverageMeasured
		q.ProviderAttempts = upstream
	}
	if !s.SetQuery(q) {
		panic("bad fixture")
	}
	return s.Finish("deskboard", "query", "", 0, "")
}

// regression: class perf-v3-accounting
func TestPerfV3RecordToReportFlow(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	w := perfrec.Writer{}
	state, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	appendRecord := func(r perfrec.Record) {
		t.Helper()
		if err := w.Write(state, r); err != nil {
			t.Fatal(err)
		}
	}
	appendRecord(perfrec.New("deskboard", "query", "none", time.Millisecond, 0, "", time.Now()))
	p := perfrec.Probe{}
	appendRecord(perfrec.Begin(p, p.Mark(), nil).Finish("deskboard", "query", "none", 0, ""))
	zero := int64(0)
	for i := 0; i < 50; i++ {
		appendRecord(flowQuery(perfrec.ObservationClient, "miss", "unavailable", nil))
		appendRecord(flowQuery(perfrec.ObservationHandler, "miss", "waiter", &zero))
	}
	s := flowSession(perfrec.ObservationProvider)
	s.ObserveRequests()
	ms := int64(5)
	points := int64(2)
	for i := 0; i < 3; i++ {
		if !s.ObserveRequest(perfrec.RequestObservation{Class: perfrec.RequestProvider, Origin: perfrec.OriginRefresh, Protocol: "graphql", Resource: "graphql", Outcome: "2xx", DurationMs: &ms, DurationCoverage: perfrec.CoverageMeasured, Points: perfrec.Points{Coverage: perfrec.CoverageMeasured, Unit: perfrec.UnitGitHubPoints, Value: &points}}) {
			t.Fatal("observation refused")
		}
	}
	refresh := s.Finish("deskboard", "query", "", 0, "")
	appendRecord(refresh)
	appendRecord(refresh) // exact replay must not multiply cost
	appendRecord(flowQuery(perfrec.ObservationClient, "hit", "none", nil))
	appendRecord(flowQuery(perfrec.ObservationHandler, "hit", "none", &zero))
	appendRecord(flowQuery(perfrec.ObservationClient, "unavailable", "unavailable", nil)) // missing server stays a gap
	for _, origin := range []perfrec.Origin{perfrec.OriginPassthrough, perfrec.OriginPoll} {
		s := flowSession(perfrec.ObservationProvider)
		n := 1
		if origin == perfrec.OriginPassthrough {
			n = 2
		}
		for i := 0; i < n; i++ {
			s.ObserveRequest(perfrec.RequestObservation{Class: perfrec.RequestProvider, Origin: origin, Protocol: "graphql", Resource: "graphql", Outcome: "2xx", DurationCoverage: perfrec.CoverageMissing, Points: perfrec.Points{Coverage: perfrec.CoverageUnknown, Unit: perfrec.UnitGitHubPoints}})
		}
		appendRecord(s.Finish("deskboard", "query", "", 0, ""))
	}
	denied := flowSession(perfrec.ObservationHandler)
	if !denied.SetQuery(perfrec.QueryObservation{Operation: "query", Result: "denied", Source: "unavailable", Coalescing: "none", ProviderCoverage: perfrec.CoverageMeasured, ProviderAttempts: &zero}) {
		t.Fatal("denial")
	}
	appendRecord(denied.Finish("deskboard", "query", "", 5, ""))
	h, e := LoadPerfHistory(1)
	if e != nil || h.Skipped != 0 {
		t.Fatalf("reader %v skipped%d", e, h.Skipped)
	}
	sum, e := perfrec.ReduceV3(h.Records, perfrec.ProfileTool, "tool", perfrec.V3Filter{})
	if e != nil {
		t.Fatal(e)
	}
	if sum.LogicalQueries != 52 || sum.ServiceTransports != 52 || sum.Handlers != 52 || sum.Duplicates != 1 || sum.HandlerCoveredAttempts != 0 {
		t.Fatalf("query accounting %+v", sum)
	}
	var attempts, measuredPoints, unknown int64
	for _, r := range sum.Requests {
		attempts += r.Attempts
		if r.Points.Coverage == perfrec.CoverageMeasured {
			measuredPoints += *r.Points.Value
		} else {
			unknown += r.PointSamples
		}
	}
	if attempts != 6 || measuredPoints != 6 || unknown != 3 {
		t.Fatalf("requests=%d points=%d unknown=%d", attempts, measuredPoints, unknown)
	}
	if sum.HandlerUpstreamCoverage[perfrec.CoverageMeasured] != 52 || sum.Kinds[perfrec.ObservationClient] != 52 {
		t.Fatal("coverage lost")
	}
	if SummarisePerf(h.Records, 10).Records != 1 || SummarisePerfV2(h.Records, 10).Records != 1 {
		t.Fatal("legacy groups changed")
	}
	if _, e = perfrec.ReduceV3(h.Records, perfrec.ProfileTool, "work", perfrec.V3Filter{}); e == nil {
		t.Fatal("workflow grouping without consent")
	}
	// Plant conflicting replay through the real writer: neither conflicting version is counted.
	refresh.WallMs++
	appendRecord(refresh)
	h, e = LoadPerfHistory(1)
	if e != nil {
		t.Fatal(e)
	}
	sum, e = perfrec.ReduceV3(h.Records, perfrec.ProfileWorkflow, "session", perfrec.V3Filter{})
	if e != nil || sum.Conflicts != 1 {
		t.Fatal("conflicting duplicate not exposed", e)
	}
	// Plant client-plus-server double logical counts at the real measurement boundary.
	if flowSession(perfrec.ObservationHandler).SetQuery(perfrec.QueryObservation{Operation: "query", LogicalCalls: 1, Source: "hit", Coalescing: "none", Result: "ok", ProviderCoverage: perfrec.CoverageMissing}) {
		t.Fatal("handler accepted duplicate logical call")
	}
	// Plant absent IDs into the actual day file: missing invocation is skipped,
	// while an otherwise valid missing-install record remains an explicit invalid source.
	raw, err := json.Marshal(refresh)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	delete(wire, "install")
	missingInstall, _ := json.Marshal(wire)
	wire["install"] = h.Records[0].Install
	delete(wire["v3"].(map[string]any)["attribution"].(map[string]any), "invocationId")
	missingInvocation, _ := json.Marshal(wire)
	day, err := os.OpenFile(filepath.Join(perfrec.Dir(state), time.Now().UTC().Format("2006-01-02")+".jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = day.Write(append(append(append(missingInstall, '\n'), missingInvocation...), '\n')); err != nil {
		t.Fatal(err)
	}
	if err = day.Close(); err != nil {
		t.Fatal(err)
	}
	h, err = LoadPerfHistory(1)
	if err != nil || h.Skipped != 1 {
		t.Fatalf("missing invocation not skipped: %v %d", err, h.Skipped)
	}
	withGaps, err := perfrec.ReduceV3(h.Records, perfrec.ProfileTool, "tool", perfrec.V3Filter{})
	if err != nil || withGaps.Invalid != 1 || withGaps.Records != sum.Records {
		t.Fatalf("missing source identity counted: %v %+v", err, withGaps)
	}
	// Decoder/writer must reject duplicate request groups and false measured-zero points.
	bad := refresh
	bad.V3.Requests = append(bad.V3.Requests, bad.V3.Requests[0])
	if w.Write(state, bad) == nil {
		t.Fatal("duplicate point group accepted")
	}
	bad.V3.Requests = bad.V3.Requests[:1]
	bad.V3.Requests[0].Points.Value = nil
	if _, e = json.Marshal(bad); e == nil {
		t.Fatal("missing cost became measured zero")
	}
}
