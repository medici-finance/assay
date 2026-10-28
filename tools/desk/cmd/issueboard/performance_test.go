package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type perfWire func(*http.Request) (*http.Response, error)

func (f perfWire) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// regression: class issue-projection-v3-composition
func TestProjectionV3ActualRecorder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	config := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	rawRoster, err := os.ReadFile(filepath.Join(config, "roster.env"))
	if err != nil {
		t.Fatal(err)
	}
	var forges []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(rawRoster), "\n") {
		key, values, _ := strings.Cut(line, "=")
		if key == "ASSAY_ALLOWED_REPOS" || key == "ASSAY_SCAN_REPOS" {
			for _, entry := range strings.Split(values, ",") {
				repo, _, _ := strings.Cut(entry, ":")
				if !seen[repo] {
					forges = append(forges, repo+"=github")
					seen[repo] = true
				}
			}
		}
	}
	if len(forges) == 0 {
		t.Fatal("fixture has no allowed repositories")
	}
	if err := os.WriteFile(filepath.Join(config, "roster.env"), append(rawRoster, []byte("\nASSAY_REPO_FORGES="+strings.Join(forges, ",")+"\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSAY_CONFIG_HOME", config)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("DESK_TOOLS_DISABLED", "")
	oldMint, oldRole, oldBase, oldTransport := mintTokenFn, sessionRoleFn, forgeAPIBase, http.DefaultTransport
	defer func() {
		mintTokenFn, sessionRoleFn, forgeAPIBase, http.DefaultTransport = oldMint, oldRole, oldBase, oldTransport
	}()
	mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
	sessionRoleFn = func(string) (string, string, error) { return "worker", "worker-desk", nil }
	forgeAPIBase = ""
	attempts, costSelections := 0, 0
	http.DefaultTransport = perfWire(func(r *http.Request) (*http.Response, error) {
		attempts++
		body := "[]"
		if r.URL.Path == "/repos/example-org/tracker/issues" {
			body = `[{"number":1,"title":"addressed","state":"open","created_at":"2026-01-01T00:00:00Z","user":{"login":"shared-agent","id":2002},"labels":[{"name":"to:worker"}]}]`
		}
		if r.URL.Path == "/graphql" {
			raw, _ := io.ReadAll(r.Body)
			if bytes.Contains(raw, []byte("_assayPerf")) {
				costSelections++
			}
			body = `{"data":{"repository":{"issue":{"comments":{"nodes":[],"pageInfo":{"hasNextPage":false}}}},"_assayPerf":{"cost":2}}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	root := t.TempDir()
	args := []string{"issues", "--root", root, "--projection=inbox", "--to=worker"}
	var baseline, baseErr bytes.Buffer
	if code := execute(args, &baseline, &baseErr, nil); code != 0 {
		t.Fatal(code, baseErr.String())
	}
	baselineAttempts := attempts
	attempts = 0
	var out, errs bytes.Buffer
	if code := measured(append(args, "--perf-profile=workflow", "--perf-role=review", "--perf-session-id=fixture-private-context"), &out, &errs, nil); code != 0 {
		t.Fatal(code, errs.String())
	}
	if projectionStable(out.String()) != projectionStable(baseline.String()) || attempts != baselineAttempts || costSelections != 1 {
		t.Fatal("measurement changed query/output", attempts, baselineAttempts, costSelections)
	}
	h, err := deskkit.LoadPerfHistory(1)
	if err != nil || len(h.Records) != 1 {
		t.Fatal("actual writer/reader", err, len(h.Records))
	}
	sum, err := perfrec.ReduceV3(h.Records, perfrec.ProfileWorkflow, "tool", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var reported, points int64
	for _, r := range sum.Requests {
		reported += r.Attempts
		if r.Points.Unit == perfrec.UnitGitHubPoints && r.Points.Value != nil {
			points += *r.Points.Value
		}
	}
	if reported != int64(attempts) || points != 2 || sum.RequestCoverage[perfrec.CoverageMeasured] != 1 {
		t.Fatalf("actual=%d report=%d points=%d coverage=%v requests=%+v", attempts, reported, points, sum.RequestCoverage, sum.Requests)
	}
	if h.Records[0].V3.Attribution.Role != "review" {
		t.Fatal("parsed context not selected before effects")
	}
	// A second invocation with consent off must neither inherit the observer nor write.
	t.Setenv("ASSAY_TELEMETRY", "")
	attempts = 0
	costSelections = 0
	out.Reset()
	errs.Reset()
	if code := measured(append(args, "--perf-profile=workflow"), &out, &errs, nil); code != 0 || projectionStable(out.String()) != projectionStable(baseline.String()) || attempts != baselineAttempts || costSelections != 0 {
		t.Fatal("consent-off parity/context leaked", code, attempts, costSelections, errs.String())
	}
	files, _ := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "*.jsonl"))
	raw, _ := os.ReadFile(files[0])
	if bytes.Contains(raw, []byte("fixture-token")) || bytes.Contains(raw, []byte("addressed")) {
		t.Fatal("operational text leaked")
	}
	h, err = deskkit.LoadPerfHistory(1)
	if err != nil || len(h.Records) != 1 {
		t.Fatal("consent off wrote", err)
	}
	// Recorder failure must not change output, sends, or exit.
	perfPath := filepath.Join(home, ".config", "assay", "perf")
	if err := os.Rename(perfPath, filepath.Join(home, "saved-perf")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(perfPath, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSAY_TELEMETRY", "1")
	out.Reset()
	errs.Reset()
	attempts = 0
	if code := measured(append(args, "--perf-profile=workflow"), &out, &errs, nil); code != 0 || projectionStable(out.String()) != projectionStable(baseline.String()) || attempts != baselineAttempts {
		t.Fatal("failed recorder affected operation", code, errs.String())
	}
	// Optional context is accepted for help without provider work or disclosure.
	out.Reset()
	errs.Reset()
	attempts = 0
	if code := measured([]string{"--help", "--perf-profile=workflow", "--perf-session-id=fixture-private-context"}, &out, &errs, nil); code != 0 || attempts != 0 || strings.Contains(out.String()+errs.String(), "fixture-private-context") {
		t.Fatal("help performed work or echoed metadata")
	}

}

func projectionStable(s string) string {
	return regexp.MustCompile(`cutoff=[^;]+;`).ReplaceAllString(s, "cutoff=<invocation>;")
}

func measured(args []string, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	return deskkit.PerfRunMeasured("issueboard", perfVerbs, args, performanceCallback(stdout, stderr, pre))
}
