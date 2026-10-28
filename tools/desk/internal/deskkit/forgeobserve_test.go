package deskkit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"text/scanner"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// regression: class perf-v3-graphql-cost
func TestPerfV3GraphQLCostCoverage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASSAY_TELEMETRY", "1")
	s := flowSession(perfrec.ObservationProvider)
	o := RequestObserver{Sink: s, Origin: perfrec.OriginDirect}
	queries := []string{}
	response := `{"data":{"_assayPerf":{"cost":0,"score":7},"project":{"issue":{"comments":{"nodes":[]},"activity":{"nodes":[]}}}}}`
	leaf := observeRT(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		queries = append(queries, body.Query)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}, "X-RateLimit-Used": []string{"900"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
	})
	g := &GitHubForge{Token: "fixture", Observer: o, Client: &http.Client{Transport: leaf}}
	in := map[string]any{"query": ghListIssuesQuery}
	var out map[string]any
	if err := g.doJSON("POST", "/graphql", in, &out); err != nil {
		t.Fatal(err)
	}
	if in["query"] != ghListIssuesQuery || !strings.Contains(queries[0], "_assayPerf:rateLimit{cost}") {
		t.Fatal("input mutation or missing same-operation metadata")
	}
	first := s.Finish("consumer", "query", "", 0, "")
	if len(first.V3.Requests) != 1 || first.V3.Requests[0].Points.Coverage != perfrec.CoverageMeasured || *first.V3.Requests[0].Points.Value != 0 {
		t.Fatal("returned zero was not measured")
	}
	// An unsupported custom operation must keep its exact query and unknown cost.
	custom := `mutation { performAction }`
	if err := g.doJSON("POST", "/graphql", map[string]any{"query": custom}, &out); err != nil {
		t.Fatal(err)
	}
	if queries[1] != custom {
		t.Fatal("custom operation expanded")
	}
	// An unknown schema must keep its exact business selection even for a known read.
	other := &GitHubForge{Token: "fixture", BaseURL: "https://enterprise.example.test", Observer: o, Client: &http.Client{Transport: leaf}}
	if err := other.doJSON("POST", "/graphql", in, &out); err != nil {
		t.Fatal(err)
	}
	if queries[2] != ghListIssuesQuery {
		t.Fatal("unsupported schema expanded")
	}
	gl := &GitLabForge{Token: "fixture", Observer: o, Client: &http.Client{Transport: leaf}}
	if _, err := gl.IssueTrustEvents(ForgeRepo{Owner: "example", Name: "project"}, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[3], "_assayPerf:queryComplexity{score}") {
		t.Fatal("complexity metadata missing")
	}
	for _, next := range []string{`{"data":{"_assayPerf":{"cost":5}}}`, `{"data":{}}`, `{"data":{"_assayPerf":{"cost":"bad"}}}`} {
		response = next
		if err := g.doJSON("POST", "/graphql", in, &out); err != nil {
			t.Fatal("optional metadata changed business error", err)
		}
	}
	sum := writeForgeRecord(t, s)
	var ghSamples, ghCost, unknown, failed, complexity int64
	for _, r := range sum.Requests {
		if r.Points.Coverage == perfrec.CoverageMeasured && r.Points.Unit == perfrec.UnitGitHubPoints {
			ghSamples += r.PointSamples
		}
		if r.Points.Coverage == perfrec.CoverageUnknown {
			unknown += r.PointSamples
		}
		if r.Points.Coverage == perfrec.CoverageFailed {
			failed += r.PointSamples
		}
		if r.Points.Coverage == perfrec.CoverageMeasured && r.Points.Unit == perfrec.UnitGitHubPoints {
			ghCost += *r.Points.Value
		}
		if r.Points.Coverage == perfrec.CoverageMeasured && r.Points.Unit == perfrec.UnitGitLabComplexity {
			complexity += *r.Points.Value
		}
	}
	if len(queries) != 7 || ghSamples != 2 || ghCost != 5 || unknown != 3 || failed != 1 || complexity != 7 {
		t.Fatalf("queries=%d ghSamples=%d ghCost=%d unknown=%d failed=%d complexity=%d", len(queries), ghSamples, ghCost, unknown, failed, complexity)
	}
	for _, raw := range []string{`{}`, `{"data":{"_assayPerf":null}}`, `{"data":{"_assayPerf":{"cost":null}}}`} {
		if p := decodeRequestPoints([]byte(raw), perfrec.UnitGitHubPoints); p.Coverage != perfrec.CoverageUnknown || p.Value != nil {
			t.Fatal("missing became zero", p)
		}
	}
	for _, raw := range []string{`broken`, `{"data":{"_assayPerf":{"cost":-1}}}`, `{"data":{"_assayPerf":{"cost":"1"}}}`} {
		if p := decodeRequestPoints([]byte(raw), perfrec.UnitGitHubPoints); p.Coverage != perfrec.CoverageFailed || p.Value != nil {
			t.Fatal("malformed became measured", p)
		}
	}
}

type observeRT func(*http.Request) (*http.Response, error)

func (f observeRT) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func writeForgeRecord(t *testing.T, s *perfrec.Session) perfrec.V3Summary {
	t.Helper()
	state, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	record := s.Finish("deskboard", "query", "", 0, "")
	raw, _ := json.Marshal(record)
	for _, secret := range []string{"secret=value", "Bearer fixture", "token-body-secret", "private/path"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("record exposed provider input")
		}
	}
	if err := (&perfrec.Writer{}).Write(state, record); err != nil {
		t.Fatal(err)
	}
	h, err := LoadPerfHistory(1)
	if err != nil || h.Skipped != 0 {
		t.Fatalf("history: %v skipped %d", err, h.Skipped)
	}
	sum, err := perfrec.ReduceV3(h.Records, perfrec.ProfileTool, "tool", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

// regression: class perf-v3-forge-leaf
func TestPerfV3ForgeAttemptTimingFlow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASSAY_TELEMETRY", "1")
	s := flowSession(perfrec.ObservationProvider)
	observer := RequestObserver{Sink: s, Origin: perfrec.OriginDirect}
	sends := 0
	leaf := observeRT(func(r *http.Request) (*http.Response, error) {
		sends++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})
	g := &GitHubForge{Token: "fixture", BaseURL: "https://api.example.test", Observer: observer, Client: &http.Client{Transport: countRoundTrips(countRoundTrips(leaf))}}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := g.doJSON("GET", "/repos/example/project", nil, &out); err != nil || !out.OK {
		t.Fatal(err, out)
	}
	// A retry is another actual send, even when the business operation is the same.
	if err := g.doJSON("GET", "/repos/example/project", nil, &out); err != nil {
		t.Fatal(err)
	}
	refused := observedTransport(countedTransport{next: observeRT(func(*http.Request) (*http.Response, error) { return nil, errors.New("admission refused") })}, observer, perfrec.RequestProvider, false)
	req, _ := http.NewRequest("GET", "https://api.example.test/private/path?secret=value", nil)
	if _, err := refused.RoundTrip(req); err == nil {
		t.Fatal("refusal vanished")
	}

	terminal := observeRT(func(r *http.Request) (*http.Response, error) {
		sends++
		switch r.URL.Path {
		case "/cancel":
			return nil, context.Canceled
		case "/error":
			return nil, errors.New("token-body-secret")
		}
		return &http.Response{StatusCode: 304, Header: http.Header{}, Body: &slowBody{}}, nil
	})
	rt := observedTransport(countRoundTrips(terminal), observer, perfrec.RequestProvider, false)
	for _, path := range []string{"/cancel", "/error", "/304"} {
		r, _ := http.NewRequest("GET", "https://api.example.test"+path, nil)
		r.Header.Set("Authorization", "Bearer fixture")
		response, _ := rt.RoundTrip(r)
		if response != nil {
			_, _ = io.ReadAll(response.Body)
			_ = response.Body.Close()
		}
	}
	s.ObserveRequests()
	sum := writeForgeRecord(t, s)
	var attempts int64
	for _, r := range sum.Requests {
		attempts += r.Attempts
		if r.DurationCoverage[perfrec.CoverageMeasured] != r.Attempts || r.BytesSamples != r.Attempts {
			t.Fatal("lifetime coverage", r)
		}
	}
	if sends != 5 || attempts != 5 {
		t.Fatalf("actual=%d reported=%d", sends, attempts)
	}
	if len(sum.Requests) != 4 {
		t.Fatal("missing bounded outcomes", sum.Requests)
	}
}

type observeSink struct{ items []perfrec.RequestObservation }

func (s *observeSink) ObserveRequest(o perfrec.RequestObservation) bool {
	s.items = append(s.items, o)
	return true
}

type slowBody struct {
	closed int
	reads  int
}

func (s *slowBody) Read(p []byte) (int, error) {
	time.Sleep(3 * time.Millisecond)
	s.reads++
	copy(p, "x")
	return 1, io.EOF
}
func (s *slowBody) Close() error { s.closed++; return nil }

// regression: class perf-v3-forge-body-parity
func TestPerfV3ForgeObservationParity(t *testing.T) {
	sink := &observeSink{}
	body := &slowBody{}
	base := observeRT(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 304, Body: body, Header: http.Header{}}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.test/resource", nil)
	rt := observedTransport(countRoundTrips(base), RequestObserver{sink, perfrec.OriginPoll}, perfrec.RequestProvider, false)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.items) != 0 || body.reads != 0 {
		t.Fatal("eager read or early timing")
	}
	p := make([]byte, 1)
	n, readErr := resp.Body.Read(p)
	if n != 1 || readErr != io.EOF || string(p) != "x" {
		t.Fatal("read contract changed")
	}
	_ = resp.Body.Close()
	_ = resp.Body.Close()
	if len(sink.items) != 1 || body.closed != 2 {
		t.Fatal("duplicate observation or changed Close")
	}
	o := sink.items[0]
	if o.Outcome != "304" || o.DurationMs == nil || *o.DurationMs < 3 || *o.Bytes != 1 {
		t.Fatal("body excluded", o)
	}
	// Authentication work must stay separate from provider work.
	a := ObserveAuthClient(&http.Client{Transport: base}, RequestObserver{sink, perfrec.OriginDirect})
	resp, err = a.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if len(sink.items) != 2 || sink.items[1].Class != perfrec.RequestAuthentication {
		t.Fatal("auth conflated")
	}
	// The deferred GraphQL decoder may close after repeated EOF reads. Its
	// lifetime still ends at the first EOF, independently of metadata decoding.
	gqlReq, _ := http.NewRequest("POST", "https://api.example.test/graphql", nil)
	gqlRT := observedTransport(countRoundTrips(observeRT(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("x"))}, nil
	})), RequestObserver{sink, perfrec.OriginDirect}, perfrec.RequestProvider, false)
	gqlResponse, err := gqlRT.RoundTrip(gqlReq)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(gqlResponse.Body)
	held := gqlResponse.Body.(*observedBody)
	firstEOF := held.end
	time.Sleep(time.Millisecond)
	_, _ = gqlResponse.Body.Read(make([]byte, 1))
	if !held.end.Equal(firstEOF) {
		t.Fatal("first EOF lifetime cutoff moved on a repeated read")
	}
	_ = gqlResponse.Body.Close()
	// Original read and cancellation errors remain the caller's exact errors.
	cut := errors.New("body cut off")
	for _, failure := range []error{cut, context.Canceled} {
		body := &errorBody{err: failure, closeErr: cut}
		rt := observedTransport(countRoundTrips(observeRT(func(*http.Request) (*http.Response, error) { return &http.Response{StatusCode: 200, Body: body}, nil })), RequestObserver{sink, perfrec.OriginDirect}, perfrec.RequestProvider, false)
		response, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		if err != failure || string(data) != "partial" {
			t.Fatal("original read failure changed", err, string(data))
		}
		if response.Body.Close() != cut {
			t.Fatal("close failure changed")
		}
	}
	// Parent and child own separate sessions. An uninstrumented child remains
	// a coverage gap; parent observations cannot manufacture its sends.
	parent, child := flowSession(perfrec.ObservationProcess), flowSession(perfrec.ObservationProcess)
	for _, session := range []*perfrec.Session{parent, child} {
		rt := observedTransport(countRoundTrips(base), RequestObserver{session, perfrec.OriginDirect}, perfrec.RequestProvider, false)
		response, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		r := session.Finish("consumer", "query", "", 0, "")
		if len(r.V3.Requests) != 1 || r.V3.Requests[0].Attempts != 1 {
			t.Fatal("parent-child double count")
		}
	}
	unknown := flowSession(perfrec.ObservationProcess).Finish("consumer", "query", "", 0, "")
	if unknown.V3.RequestCoverage == perfrec.CoverageMeasured || len(unknown.V3.Requests) != 0 {
		t.Fatal("unobserved child manufactured zero")
	}
	testForgeOverhead(t)
}

type errorBody struct {
	err      error
	closeErr error
	read     bool
}

func (b *errorBody) Read(p []byte) (int, error) {
	if b.read {
		return 0, b.err
	}
	b.read = true
	return copy(p, "partial"), b.err
}
func (b *errorBody) Close() error { return b.closeErr }

func testForgeOverhead(t *testing.T) {
	testForgeSample(t, false)
	testForgeSample(t, true)
}

func testForgeSample(t *testing.T, graphql bool) {
	t.Helper()
	// Matched CPU/wall samples include real backend decoding, aggregation and
	// local record append. The fake provider removes network variability; these
	// numbers establish instrumentation cost, not a production latency claim.
	t.Setenv("ASSAY_TELEMETRY", "1")
	state := t.TempDir()
	writer := perfrec.Writer{}
	var samples [2][]int64
	var cpu [2]int64
	sends := [2]int{}
	for sample := 0; sample < 20; sample++ {
		for _, mode := range []int{sample % 2, 1 - sample%2} {
			probe := perfrec.SystemProbe()
			start := probe.Now()
			before, _ := probe.CPU()
			session := flowSession(perfrec.ObservationProvider)
			observer := RequestObserver{}
			if mode == 1 {
				observer = RequestObserver{session, perfrec.OriginDirect}
			}
			g := &GitHubForge{Token: "fixture", Observer: observer, Client: &http.Client{Transport: observeRT(func(*http.Request) (*http.Response, error) {
				sends[mode]++
				payload := `{"result":"same"}`
				if graphql && mode == 1 {
					payload = `{"result":"same","data":{"_assayPerf":{"cost":2}}}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
			})}}
			for request := 0; request < 100; request++ {
				var out struct{ Result string }
				method, path := "GET", "/repos/example/project"
				var in any
				if graphql {
					method, path, in = "POST", "/graphql", map[string]any{"query": ghListIssuesQuery}
				}
				if err := g.doJSON(method, path, in, &out); err != nil || out.Result != "same" {
					t.Fatal("on/off business parity", err)
				}
			}
			if mode == 1 {
				if err := writer.Write(state, session.Finish("consumer", "query", "", 0, "")); err != nil {
					t.Fatal(err)
				}
			}
			after, _ := probe.CPU()
			cpu[mode] += after - before
			samples[mode] = append(samples[mode], time.Since(start).Nanoseconds())
		}
	}
	for mode := range samples {
		sort.Slice(samples[mode], func(i, j int) bool { return samples[mode][i] < samples[mode][j] })
	}
	if sends[0] != 2000 || sends[1] != 2000 {
		t.Fatal("measurement changed provider sends", sends)
	}
	t.Logf("matched fake-provider graphql=%v cycles n=20, requests/cycle=100, concurrency=1; off wall ns p50=%d p95=%d max=%d CPUtotalms=%d; on incl metadata/append wall ns p50=%d p95=%d max=%d CPUtotalms=%d; actual sends off=%d on=%d", graphql, samples[0][9], samples[0][18], samples[0][19], cpu[0], samples[1][9], samples[1][18], samples[1][19], cpu[1], sends[0], sends[1])
}

// The admitted compiled query subset is one operation with no trailing definition.
// This pins the actual splice boundary without adding a production query parser.
func TestPerfCostOperationShape(t *testing.T) {
	queries := append(ghPointQueries[:], gitlabTrustQuery)
	for i, query := range queries {
		var scan scanner.Scanner
		scan.Init(strings.NewReader(query))
		scan.Error = func(_ *scanner.Scanner, msg string) { t.Errorf("query %d lexical shape: %s", i, msg) }
		if scan.Scan() != scanner.Ident || scan.TokenText() != "query" {
			t.Fatalf("query %d is outside the admitted operation shape", i)
		}
		depth, found := 0, false
		for tok := scan.Scan(); tok != scanner.EOF; tok = scan.Scan() {
			switch tok {
			case '{':
				depth++
				found = true
			case '}':
				depth--
				if depth < 0 {
					t.Fatalf("query %d unmatched close", i)
				}
				if found && depth == 0 && scan.Scan() != scanner.EOF {
					t.Fatalf("query %d has a trailing definition after the operation", i)
				}
			}
		}
		if !found || depth != 0 {
			t.Fatalf("query %d incomplete operation", i)
		}
	}
}

func TestPerfTrustMetadataErrorFailsClosed(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			session := flowSession(perfrec.ObservationProvider)
			observer := RequestObserver{Sink: session, Origin: perfrec.OriginDirect}
			sends := 0
			client := &http.Client{Transport: observeRT(func(r *http.Request) (*http.Response, error) {
				sends++
				var input struct{ Query string }
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(input.Query, "_assayPerf:") {
					t.Fatal("fixture did not exercise the extended trust operation")
				}
				// Valid business data beside errors must pin the GraphQL-error
				// refusal itself, not pass through a separate null-item refusal.
				data := `{"repository":{"issue":{"lastEditedAt":null,"comments":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}`
				if provider == "gitlab" {
					// Non-null valid business data must not hide a rejected metadata
					// selection behind the independent null-noteable refusal.
					data = `{"project":{"issue":{"comments":{"nodes":[],"pageInfo":{"hasNextPage":false}},"activity":{"nodes":[],"pageInfo":{"hasPreviousPage":false}}}}}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"errors":[{"message":"unknown optional metadata field"}],"data":` + data + `}`))}, nil
			})}
			repo := ForgeRepo{Owner: "example", Name: "project"}
			var payload *TrustPayload
			var err error
			if provider == "github" {
				payload, err = (&GitHubForge{Token: "fixture", Observer: observer, Client: client}).IssueTrustEvents(repo, 1)
			} else {
				payload, err = (&GitLabForge{Token: "fixture", Observer: observer, Client: client}).IssueTrustEvents(repo, 1)
			}
			if err == nil || payload != nil || sends != 1 {
				t.Fatalf("metadata rejection must remain could-not-check without recovery sends: payload=%v err=%v sends=%d", payload, err, sends)
			}
		})
	}
}
