package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: class perf-v3-auth-invocation
func TestAuthInvocationRecord(t *testing.T) {
	srv := coverageServer(t, []coverageFixture{{id: 17, login: "example-org", acctType: "Organization", selection: "all", repos: []string{"example-org/project"}, token: "fixture-auth-secret"}})
	defer srv.Close()
	setupCoverage(t, "reviewer", srv)
	t.Setenv("ASSAY_TELEMETRY", "1")
	p := perfrec.SystemProbe()
	s := perfrec.Begin(p, p.Mark(), nil)
	sink := func(c perfrec.Context) { s.SelectContext(c) }
	if err := cmdCoverageMeasured([]string{"reviewer", "--perf-profile=tool"}, s, sink); err != nil {
		t.Fatal(err)
	}
	r := s.Finish("desktoken", "coverage", "", 0, "")
	if r.V3 == nil || r.V3.RequestCoverage != perfrec.CoverageMeasured {
		t.Fatal("no complete auth inventory")
	}
	var sends int64
	for _, request := range r.V3.Requests {
		sends += request.Attempts
		if request.Class != perfrec.RequestAuthentication || request.Resource != "authentication" || request.Points.Unit != perfrec.UnitNone {
			t.Fatal("authentication mixed with provider", request)
		}
	}
	if sends != 3 {
		t.Fatalf("actual three auth sends reported %d", sends)
	}
	raw, _ := json.Marshal(r)
	for _, secret := range []string{"fixture-auth-secret", "example-org", "Authorization", "Bearer"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("auth content exposed")
		}
	}
	// Execute the compiled command entrypoint in a disposable test subprocess,
	// then use the production mixed reader and reducer on its actual day file.
	cmd := exec.Command(os.Args[0], "-test.run=^TestAuthProcessHelper$")
	cmd.Env = append(os.Environ(), "ASSAY_AUTH_TEST_HELPER=1", "ASSAY_AUTH_TEST_URL="+srv.URL)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compiled command: %v %s", err, output)
	}
	history, err := deskkit.LoadPerfHistory(1)
	if err != nil || history.Skipped != 0 {
		t.Fatal("compiled record reader", err)
	}
	summary, err := perfrec.ReduceV3(history.Records, perfrec.ProfileTool, "tool", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var attempts int64
	for _, request := range summary.Requests {
		attempts += request.Attempts
		if request.Class != perfrec.RequestAuthentication {
			t.Fatal("compiled auth class")
		}
	}
	if summary.Records != 1 || attempts != 3 {
		t.Fatalf("compiled command records=%d attempts=%d", summary.Records, attempts)
	}
	// The actual parser accepts a profile while process consent stays off, and
	// no optional observation client or request inventory becomes active.
	t.Setenv("ASSAY_TELEMETRY", "")
	ctx := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}})
	if measuredAuth(ctx, s) != httpClient {
		t.Fatal("consent-off client changed")
	}
}

func TestAuthProcessHelper(t *testing.T) {
	if os.Getenv("ASSAY_AUTH_TEST_HELPER") != "1" {
		return
	}
	httpClient = &http.Client{Transport: &rewriteTransport{orig: os.Getenv("ASSAY_AUTH_TEST_URL")}}
	os.Args = []string{"desktoken", "coverage", "reviewer", "--perf-profile=tool"}
	main()
}
