package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// buildV4CLI pins the original Go caches before any HOME isolation. Only the
// declared build environment crosses into the child: no credentials, operator
// roster, downloads, automatic toolchain selection or module-file edits.
func buildV4CLI(t *testing.T) string {
	t.Helper()
	goTool, e := exec.LookPath("go")
	if e != nil {
		t.Fatal(e)
	}
	base := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "USERPROFILE=" + os.Getenv("USERPROFILE"), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "KUBECONFIG=" + os.DevNull}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		base = append(base, "SystemRoot="+systemRoot)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cacheCmd := exec.CommandContext(ctx, goTool, "env", "-json", "GOMODCACHE", "GOCACHE", "GOPATH", "GOROOT")
	cacheCmd.Env = base
	raw, e := cacheCmd.Output()
	if e != nil {
		t.Fatalf("resolve original Go caches: %v", e)
	}
	var caches map[string]string
	if e = json.Unmarshal(raw, &caches); e != nil {
		t.Fatal(e)
	}
	buildHome := t.TempDir()
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + buildHome, "USERPROFILE=" + buildHome, "GOENV=off", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "KUBECONFIG=" + os.DevNull, "TMPDIR=" + buildHome, "TMP=" + buildHome, "TEMP=" + buildHome}
	for _, key := range []string{"GOMODCACHE", "GOCACHE", "GOPATH", "GOROOT"} {
		if caches[key] == "" || !filepath.IsAbs(filepath.SplitList(caches[key])[0]) {
			t.Fatalf("invalid original build path %s", key)
		}
		env = append(env, key+"="+caches[key])
	}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		env = append(env, "SystemRoot="+systemRoot)
	}
	pkg, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	name := "deskperf"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.CommandContext(ctx, goTool, "build", "-mod=readonly", "-o", binary, "./cmd/deskperf")
	build.Dir, build.Env = filepath.Clean(filepath.Join(pkg, "..", "..")), env
	t.Cleanup(func() {
		wrote, e := goquiet.Wrote(env)
		if e != nil || len(wrote) > 0 {
			t.Errorf("scratch Go build emitted telemetry: files=%q error=%v", wrote, e)
		}
	})
	if out, e := goquiet.Cmd(build).CombinedOutput(); e != nil {
		t.Fatalf("build current deskperf: %v\n%s", e, out)
	}
	return binary
}

func v4CLIHome(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	state := filepath.Join(home, ".config", "assay")
	if e := os.MkdirAll(state, 0o700); e != nil {
		t.Fatal(e)
	}
	// Explicit synthetic adopter configuration, never the operator's roster.
	const roster = "ASSAY_BLESS_LOGIN=fixture-owner:2001\nASSAY_ALLOWED_REPOS=example/tool:ci:private\n"
	if e := os.WriteFile(filepath.Join(state, "roster.env"), []byte(roster), 0o600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ASSAY_TELEMETRY", "1")
	return home, state
}

func v4CLI(t *testing.T, binary, home string, args ...string) (int, []byte, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = home // no source worktree/run admission can leak in through cwd
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "USERPROFILE=" + home, "ASSAY_CONFIG_HOME=" + filepath.Join(home, ".config", "assay"), "ASSAY_TELEMETRY=1", "DESK_LOOP=worker-desk", "KUBECONFIG=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_TERMINAL_PROMPT=0"}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		cmd.Env = append(cmd.Env, "SystemRoot="+systemRoot)
	}
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	e := cmd.Run()
	if ctx.Err() != nil {
		t.Fatal("compiled deskperf exceeded bounded runtime:", ctx.Err())
	}
	rc := 0
	if e != nil {
		if exit, ok := e.(*exec.ExitError); ok {
			rc = exit.ExitCode()
		} else {
			t.Fatal(e)
		}
	}
	return rc, out.Bytes(), diag.String()
}

func v4CLIJSON(t *testing.T, binary, home string, args ...string) reportJSON {
	t.Helper()
	rc, out, diag := v4CLI(t, binary, home, append([]string{"report", "--json"}, args...)...)
	if rc != 0 {
		t.Fatalf("compiled report %v: rc=%d stderr=%s stdout=%s", args, rc, diag, out)
	}
	var r reportJSON
	if e := json.Unmarshal(out, &r); e != nil {
		t.Fatalf("compiled JSON: %v\n%s", e, out)
	}
	return r
}

func v4CLIWrite(t *testing.T, state string, records ...perfrec.Record) {
	t.Helper()
	w := &perfrec.Writer{}
	for _, r := range records {
		if e := w.Write(state, r); e != nil {
			t.Fatal("canonical writer:", e)
		}
	}
}

func v4CLISource(t *testing.T, profile, agent string, wall int64) perfrec.Record {
	t.Helper()
	c := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: profile, Present: true}, AgentID: perfrec.Value{Value: agent, Present: true}, Role: perfrec.Value{Value: "implement", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }})
	s := perfrec.BeginSelected(perfrec.SystemProbe(), perfrec.ProcessEntry(), nil, c)
	s.Add(perfrec.CounterHTTP, 1)
	s.ObserveRequests()
	duration, points := int64(5), int64(7)
	if !s.ObserveRequest(perfrec.RequestObservation{Class: perfrec.RequestProvider, Origin: perfrec.OriginDirect, Protocol: "graphql", Resource: "graphql", Outcome: "2xx", DurationCoverage: perfrec.CoverageMeasured, DurationMs: &duration, Points: perfrec.Points{Coverage: perfrec.CoverageMeasured, Unit: perfrec.UnitGitHubPoints, Value: &points}}) {
		t.Fatal("typed observation refused")
	}
	r := s.Finish("issueboard", "issues", "worker-desk", 0, "")
	r.WallMs = wall // synthetic fixture, not performance evidence
	return r
}

func v4CLIConvert(t *testing.T, r perfrec.Record) perfrec.Record {
	t.Helper()
	r, e := perfrec.ToV4(r)
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func testV4CompiledFlow(t *testing.T) {
	t.Helper()
	binary := buildV4CLI(t) // bounded build ONCE for all runtime cases
	agent, _ := perfrec.NewID()
	for _, profile := range []string{"tool", "workflow"} {
		t.Run("compiled-"+profile, func(t *testing.T) {
			home, state := v4CLIHome(t)
			v4CLIWrite(t, state, v4CLIConvert(t, v4CLISource(t, profile, agent, 23)))
			r := v4CLIJSON(t, binary, home)
			if r.Summary.Records != 0 || r.PerfV2 != nil || r.PerfV3 != nil || r.PerfV4 == nil || r.PerfV4.Records != 1 || r.PerfV4.Groups[0].SumOwnProcessWallMs != 23 {
				t.Fatalf("cohorts mixed: %+v", r)
			}
			if r.PerfV4.QueryNotApplicable != 1 || r.PerfV4.QueryUnavailable != 0 {
				t.Fatal("process query coverage is not not-applicable")
			}
			if len(r.Commands.Rows) != 1 || r.Commands.Rows[0].HTTPAttempts != 1 || r.Commands.Rows[0].Schema != perfrec.SchemaV4 {
				t.Fatalf("compiled command costs: %+v", r.Commands)
			}
			q := r.PerfV4.Requests
			if len(q) != 1 || q[0].Attempts != 1 || q[0].PointSamples != 1 || q[0].Points.Value == nil || *q[0].Points.Value != 7 {
				t.Fatalf("measured charges lost: %+v", q)
			}
			raw, _ := json.Marshal(r)
			if bytes.Contains(raw, []byte(agent)) {
				t.Fatal("default compiled report leaked workflow identity")
			}
			rc, text, diag := v4CLI(t, binary, home, "report")
			if rc != 0 || !bytes.Contains(text, []byte("points 7 github-points")) || !bytes.Contains(text, []byte("1 samples")) {
				t.Fatalf("actual charge absent: rc=%d stdout=%s stderr=%s", rc, text, diag)
			}
			if profile == "workflow" {
				r = v4CLIJSON(t, binary, home, "--perf-profile", "workflow", "--assigned-role", "implement", "--group-by", "agent", "--agent-id", agent)
				if r.PerfV4 == nil || r.PerfV4.Records != 1 || r.PerfV4.Groups[0].Key != agent {
					t.Fatal("supported workflow grouping lost")
				}
			}
			for _, args := range [][]string{{"--assigned-role", "implement"}, {"--role", "worker-desk"}, {"--perf-profile", "workflow", "--role", "implement", "--assigned-role", "implement"}} {
				rc, out, _ := v4CLI(t, binary, home, append([]string{"report", "--json"}, args...)...)
				if rc != deskkit.ExitRefused || len(out) != 0 {
					t.Fatalf("unsupported role combination admitted: %v rc=%d stdout=%s", args, rc, out)
				}
			}
			// Exercise the actual shipped guard; fixture setup never disables it.
			if e := os.WriteFile(filepath.Join(state, "STOP"), []byte("synthetic test stop\n"), 0o600); e != nil {
				t.Fatal(e)
			}
			rc, out, diag := v4CLI(t, binary, home, "report", "--json")
			if rc != deskkit.ExitDisabled || len(out) != 0 || !strings.Contains(diag, "stop flag active") {
				t.Fatalf("native Guard bypassed: rc=%d stdout=%s stderr=%s", rc, out, diag)
			}
			audit, e := os.ReadFile(filepath.Join(state, "audit.jsonl"))
			if e != nil {
				t.Fatal("native Guard did not retain its audit row:", e)
			}
			rows := bytes.Split(bytes.TrimSpace(audit), []byte("\n"))
			var last struct{ Tool, Verb, Result string }
			if e = json.Unmarshal(rows[len(rows)-1], &last); e != nil || last.Tool != "deskperf" && last.Tool != "deskperf.exe" || last.Verb != "guard" || last.Result != "disabled" {
				t.Fatalf("native Guard audit: %s / %v", audit, e)
			}
		})
	}
	t.Run("compiled-mixed-history", func(t *testing.T) {
		home, state := v4CLIHome(t)
		v4CLIWrite(t, state, mixedRecords(t)...)
		v3tool, v3workflow := v4CLISource(t, "tool", agent, 13), v4CLISource(t, "workflow", agent, 17)
		v4tool, v4workflow := v4CLIConvert(t, v4CLISource(t, "tool", agent, 23)), v4CLIConvert(t, v4CLISource(t, "workflow", agent, 29))
		v4CLIWrite(t, state, v3tool, v3workflow, v4tool, v4workflow, v4workflow)
		r := v4CLIJSON(t, binary, home)
		if r.Summary.Records != 3 || r.Summary.TotalMs != 2100 || r.PerfV2 == nil || r.PerfV2.Records != 3 || r.PerfV2.SumWallMs != 3600 || r.PerfV3 == nil || r.PerfV3.Records != 2 || r.PerfV4 == nil || r.PerfV4.Records != 2 || r.PerfV4.Duplicates != 1 {
			t.Fatalf("mixed histories/replay charged incorrectly: %+v", r)
		}
		count := 0
		for _, row := range r.Commands.Rows {
			count += row.Count
		}
		if count != 10 {
			t.Fatalf("boot steps or replay charged: %d", count)
		}
		r = v4CLIJSON(t, binary, home, "--role", "worker-desk")
		if r.PerfV3 != nil || r.PerfV4 != nil || r.Summary.Records != 2 || r.PerfV2 == nil || r.PerfV2.Records != 2 {
			t.Fatalf("legacy role changed cohort semantics: %+v", r)
		}
		r = v4CLIJSON(t, binary, home, "--perf-profile", "workflow", "--assigned-role", "implement")
		if r.Summary.Records != 0 || r.PerfV2 != nil || r.Forge != nil || r.PerfV3 == nil || r.PerfV3.Records != 1 || r.PerfV4 == nil || r.PerfV4.Records != 1 {
			t.Fatalf("workflow filter admitted historical/tool observations: %+v", r)
		}
		rc, out, diag := v4CLI(t, binary, home, "report", "--by-day", "--json")
		var day reportDayJSON
		if rc != 0 || json.Unmarshal(out, &day) != nil || day.PerfV4 == nil || day.PerfV4.Records != 2 || len(day.PerfV4ByDay) != 1 || day.PerfV4ByDay[0].Summary.Records != 2 {
			t.Fatalf("compiled day cohorts: rc=%d %s %s", rc, out, diag)
		}
	})
	t.Run("compiled-unavailable-attribution", func(t *testing.T) {
		home, state := v4CLIHome(t)
		v4CLIWrite(t, state, v4CLIConvert(t, v4CLISource(t, "workflow", agent, 23)))
		files, e := filepath.Glob(filepath.Join(perfrec.Dir(state), "*.jsonl"))
		if e != nil || len(files) != 1 {
			t.Fatal("expected canonical day file", e)
		}
		raw, e := os.ReadFile(files[0])
		if e != nil {
			t.Fatal(e)
		}
		var wire map[string]any
		if e = json.Unmarshal(bytes.TrimSpace(raw), &wire); e != nil {
			t.Fatal(e)
		}
		delete(wire["extensions"].(map[string]any), "attribution")
		raw, e = json.Marshal(wire)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(files[0], append(raw, '\n'), 0o600); e != nil {
			t.Fatal(e)
		}
		r := v4CLIJSON(t, binary, home)
		if r.PerfV4 == nil || r.PerfV4.Records != 1 || r.PerfV4.AttributionUnavailable != 1 {
			t.Fatal("optional facet lost the core")
		}
		rc, out, _ := v4CLI(t, binary, home, "report", "--json", "--perf-profile", "workflow", "--assigned-role", "implement")
		if rc != deskkit.ExitRefused || len(out) != 0 {
			t.Fatal("core role admitted unavailable attribution")
		}
	})
	t.Run("compiled-observation-boundaries", func(t *testing.T) {
		home, state := v4CLIHome(t)
		for _, kind := range []perfrec.ObservationKind{perfrec.ObservationClient, perfrec.ObservationHandler, perfrec.ObservationProvider} {
			r := v4CLISource(t, "workflow", agent, 991)
			r.V3.Kind, r.CPU, r.CPUMs, r.Boundary = kind, perfrec.CPUUnsupported, nil, perfrec.BoundaryCaller
			v4CLIWrite(t, state, v4CLIConvert(t, r))
		}
		r := v4CLIJSON(t, binary, home)
		if r.PerfV4 == nil || r.PerfV4.Records != 3 || r.PerfV4.QueryUnavailable != 2 || r.PerfV4.QueryNotApplicable != 1 {
			t.Fatalf("query kind coverage lost: %+v", r.PerfV4)
		}
		if len(r.PerfV4.Groups) != 1 || r.PerfV4.Groups[0].SumOwnProcessWallMs != 0 || r.PerfV4.Groups[0].CPURecords != 0 || len(r.Commands.Rows) != 0 || r.Commands.NonProcess != 3 {
			t.Fatalf("non-process work charged as invocation work: %+v", r)
		}
	})
	t.Run("compiled-cross-schema-identity", func(t *testing.T) {
		home, state := v4CLIHome(t)
		old := v4CLISource(t, "workflow", agent, 101)
		current := v4CLIConvert(t, old)
		old.Tool, current.Tool = "statusgen", "deskboard"
		v4CLIWrite(t, state, old, current, v4CLIConvert(t, v4CLISource(t, "tool", agent, 23)))
		r := v4CLIJSON(t, binary, home)
		if r.PerfV3 != nil || r.PerfV4 == nil || r.PerfV4.Records != 1 || r.PerfV4.Conflicts != 1 || len(r.PerfV4.Groups) != 1 || r.PerfV4.Groups[0].SumOwnProcessWallMs != 23 {
			t.Fatalf("cross-schema source double charged: %+v", r)
		}
		if len(r.Commands.Rows) != 1 || r.Commands.Rows[0].Count != 1 || r.Commands.Rows[0].SumWallMs != 23 || r.Commands.V4Conflicts != 1 {
			t.Fatalf("command overview double charged: %+v", r.Commands)
		}
		for _, tool := range []string{"statusgen", "deskboard"} {
			rc, out, _ := v4CLI(t, binary, home, "report", "--json", "--tool", tool)
			if rc != deskkit.ExitRefused || len(out) != 0 {
				t.Fatalf("filter hid cross-schema collision: %s", tool)
			}
		}
	})
}
