package main

// perfhttp_test.go — the `http` counter counts each direct HTTP attempt statusgen makes at its
// forge seams, exactly once, and the count reaches the perf-v2 record.

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

type perfFakeDoer struct {
	calls int
	err   error
}

func (d *perfFakeDoer) Do(*http.Request) (*http.Response, error) {
	d.calls++
	if d.err != nil {
		return nil, d.err
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]"))}, nil
}

// Statusgen's forge calls were invisible to deskperf's http counter.
// regression: #3101
func TestPerfHTTPCountsEachCall(t *testing.T) {
	// GitHub REST seam (ghfetch.go): a transport failure is still an attempt.
	before := perfHTTPCalls.Load()
	ok := &perfFakeDoer{}
	c := &ghClient{doer: ok, base: "http://forge.invalid"}
	for i := 0; i < 3; i++ {
		if _, _, err := c.get("http://forge.invalid/repos/o/r/pulls"); err != nil {
			t.Fatal(err)
		}
	}
	bad := &perfFakeDoer{err: errors.New("down")}
	if _, _, err := (&ghClient{doer: bad}).get("http://forge.invalid/x"); err == nil {
		t.Fatal("want a transport error")
	}
	if got := perfHTTPCalls.Load() - before; got != 4 || ok.calls+bad.calls != 4 {
		t.Errorf("REST seam: counter +%d for %d calls, want one per call", got, ok.calls+bad.calls)
	}

	// GitLab claim-decay seam (claimdecay_gitlab.go).
	saved := gitlabHTTPDoer
	t.Cleanup(func() { gitlabHTTPDoer = saved })
	gl := &perfFakeDoer{}
	gitlabHTTPDoer = gl
	before = perfHTTPCalls.Load()
	if _, err := (gitlabAPI{header: "PRIVATE-TOKEN"}).getMRPage("http://forge.invalid/mr"); err != nil {
		t.Fatal(err)
	}
	if got := perfHTTPCalls.Load() - before; got != 1 || gl.calls != 1 {
		t.Errorf("GitLab seam: counter +%d for %d calls, want 1", got, gl.calls)
	}

	// GitHub GraphQL claim-decay seam (claimdecay_github.go), over a real local server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	t.Cleanup(srv.Close)
	before = perfHTTPCalls.Load()
	if _, err := githubGraphQLPost(srv.URL, "t", "test", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if got := perfHTTPCalls.Load() - before; got != 1 {
		t.Errorf("GraphQL seam: counter +%d for 1 call, want 1", got)
	}

	// Telemetry sender (telemetry.go): empty in every shipped build, so point it at the
	// local server for this one call. A redirect followed inside the client is one attempt.
	savedEndpoint := telemetryEndpoint
	t.Cleanup(func() { telemetryEndpoint = savedEndpoint })
	telemetryEndpoint = srv.URL
	before = perfHTTPCalls.Load()
	if err := sendTelemetry(TelemetryPayload{}); err != nil {
		t.Fatal(err)
	}
	if got := perfHTTPCalls.Load() - before; got != 1 {
		t.Errorf("telemetry seam: counter +%d for 1 call, want 1", got)
	}
}

// The counted forge calls must land in the record as a measured http counter.
// regression: #3101
func TestPerfRecordCarriesHTTP(t *testing.T) {
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	probe := perfrec.Probe{Now: func() time.Time { clock = clock.Add(time.Millisecond); return clock }}
	saved := perfSession
	t.Cleanup(func() { perfSession = saved })
	perfSession = perfrec.Begin(probe, probe.Mark(), perfPhases)

	before := perfHTTPCalls.Load()
	stop := perfPhase("parse")
	perfCountHTTP()
	perfCountHTTP()
	stop()
	rec, ok := perfBuildRecord(0)
	if !ok {
		t.Fatal("no record from a live session")
	}
	if rec.Schema != perfrec.SchemaV2 {
		t.Errorf("schema = %q, want %q", rec.Schema, perfrec.SchemaV2)
	}
	if got, want := rec.Counters[perfrec.CounterHTTP], perfHTTPCalls.Load(); got != want || want-before != 2 {
		t.Errorf("http = %d, want %d (two calls in this session)", got, want)
	}
	if len(rec.Phases) != 1 || rec.Phases[0].Name != "parse" {
		t.Errorf("phases = %+v, want one parse phase", rec.Phases)
	}

	perfSession = nil
	if _, ok := perfBuildRecord(0); ok {
		t.Error("a nil session (switch off) must build no record")
	}
}
