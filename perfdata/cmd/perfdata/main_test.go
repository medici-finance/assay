package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
	"github.com/medici-finance/assay/perfdata/local"
	"github.com/medici-finance/assay/perfdata/report"
)

const flowPolicy = `{"schema":"performance-policy-v1","revision":"r7","profiles":["tool","workflow"],` +
	`"tools":["deskdispatch","statusgen"],"ageCapDays":7,"byteCap":16777216}`

var t0 = time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)

func env(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

func clock(t time.Time) func() time.Time { return func() time.Time { return t } }

type out struct {
	code           int
	stdout, stderr string
}

func invoke(t *testing.T, getenv func(string) string, now time.Time, stdin string, args ...string) out {
	t.Helper()
	var so, se bytes.Buffer
	code := run(args, strings.NewReader(stdin), &so, &se, getenv, clock(now))
	return out{code, so.String(), se.String()}
}

func writePolicy(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(p, []byte(flowPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// draftLine builds one draft input line.
func draftLine(t *testing.T, k perfdata.Kind, occurred string, ctx *perfdata.RawContext, payload any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(perfdata.Draft{Kind: k, OccurredAt: occurred, Ctx: ctx, Payload: b})
	if err != nil {
		t.Fatal(err)
	}
	return string(line) + "\n"
}

func ms(v int64) perfdata.Measurement { return perfdata.Measured(v) }

func at(offsetMs int64) string {
	return perfdata.FormatTime(t0.Add(time.Duration(offsetMs) * time.Millisecond))
}

func wctx(inv, parent string) *perfdata.RawContext {
	return &perfdata.RawContext{Invocation: inv, ParentInvocation: parent, DeskRole: "worker-desk", Role: "implement"}
}

func toolAt(start, wall, own int64) perfdata.ToolCompletion {
	un := perfdata.Unavailable()
	return perfdata.ToolCompletion{Tool: "deskdispatch", Outcome: perfdata.OutcomeOK, StartedAt: at(start), Wall: ms(wall),
		CPU: perfdata.CPU{Own: ms(own), Descendant: un, Inclusive: un}}
}

func i64(v int64) *int64 { return &v }

func storeEntries(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	_ = filepath.Walk(dir, func(p string, _ os.FileInfo, _ error) error {
		if p != dir {
			names = append(names, p)
		}
		return nil
	})
	return names
}

func TestLocalReportFlow(t *testing.T) {
	pol := writePolicy(t)
	armedEnv := env(perfdata.EnvTelemetry, "1", perfdata.EnvProfile, "workflow")

	t.Run("collection off: record exits 0 and creates nothing", func(t *testing.T) {
		in := draftLine(t, perfdata.KindTool, "", wctx("inv-a", ""), toolAt(0, 10, 1))
		for name, ge := range map[string]func(string) string{
			"no telemetry":            env(perfdata.EnvProfile, "workflow"),
			"telemetry, no profile":   env(perfdata.EnvTelemetry, "1"),
			"telemetry=true, profile": env(perfdata.EnvTelemetry, "true", perfdata.EnvProfile, "workflow"),
		} {
			dir := t.TempDir()
			o := invoke(t, ge, t0, in, "record", "--dir", dir, "--policy", pol)
			if o.code != 0 || !strings.Contains(o.stderr, "collection off") {
				t.Fatalf("%s: off must exit 0 and say so: %+v", name, o)
			}
			if got := storeEntries(t, dir); len(got) != 0 {
				t.Fatalf("%s: collection off created %v", name, got)
			}
		}
		// Control: the same input with consent armed does create the store.
		dir := t.TempDir()
		if o := invoke(t, armedEnv, t0, in, "record", "--dir", dir, "--policy", pol); o.code != 0 {
			t.Fatalf("armed control failed: %+v", o)
		}
		if len(storeEntries(t, dir)) == 0 {
			t.Fatal("control: armed record created nothing; the off check proves nothing")
		}
	})

	t.Run("record then report: nested time is a union and cumulative usage is not summed", func(t *testing.T) {
		dir := t.TempDir()
		// Run 1: a parent invocation 0..1000ms with two overlapping children, and the
		// parent's first cumulative usage snapshot.
		in1 := draftLine(t, perfdata.KindTool, "", wctx("inv-parent", ""), toolAt(0, 1000, 100)) +
			draftLine(t, perfdata.KindTool, "", wctx("inv-c1", "inv-parent"), toolAt(200, 400, 150)) +
			draftLine(t, perfdata.KindTool, "", wctx("inv-c2", "inv-parent"), toolAt(500, 400, 150)) +
			draftLine(t, perfdata.KindUsage, at(1000), wctx("inv-parent", ""), perfdata.Usage{Mode: perfdata.UsageCumulative, Model: "model-a",
				InputTokens: i64(100), OutputTokens: i64(10)})
		if o := invoke(t, armedEnv, t0.Add(time.Second), in1, "record", "--dir", dir, "--policy", pol, "--json"); o.code != 0 || !strings.Contains(o.stdout, `"recorded":4`) {
			t.Fatalf("record run 1: %+v", o)
		}
		// Run 2, a separate process run: a later cumulative snapshot of the same series
		// (same local invocation locator, so the same alias) supersedes the first.
		in2 := draftLine(t, perfdata.KindUsage, at(5000), wctx("inv-parent", ""), perfdata.Usage{Mode: perfdata.UsageCumulative, Model: "model-a",
			InputTokens: i64(250), OutputTokens: i64(40)})
		if o := invoke(t, armedEnv, t0.Add(10*time.Second), in2, "record", "--dir", dir, "--policy", pol); o.code != 0 {
			t.Fatalf("record run 2: %+v", o)
		}

		o := invoke(t, nil, t0.Add(time.Minute), "", "report", "--dir", dir, "--policy", pol, "--from", "2026-10-01", "--to", "2026-10-02", "--json")
		if o.code != 0 {
			t.Fatalf("report: %+v", o)
		}
		var r report.Report
		if err := json.Unmarshal([]byte(o.stdout), &r); err != nil {
			t.Fatalf("report JSON: %v", err)
		}
		if r.Snapshot.PolicyRevision != "r7" || r.State != perfdata.CoverageComplete {
			t.Fatalf("snapshot/state: %+v %q", r.Snapshot, r.State)
		}
		if r.Cost.Invocations != 3 || r.Cost.ElapsedMs != 1000 || r.Cost.SummedWallMs != 1800 || r.Cost.CPUOwnMs != 400 {
			t.Fatalf("nested time: elapsed must be the union (1000ms), summed wall kept apart (1800ms), own CPU summed (400ms): %+v", r.Cost)
		}
		if len(r.Usage) != 1 {
			t.Fatalf("one usage row expected: %+v", r.Usage)
		}
		u := r.Usage[0]
		if u.Series != 1 || u.SnapshotsCut != 1 || u.Input.Sum == nil || *u.Input.Sum != 250 || u.Output.Sum == nil || *u.Output.Sum != 40 {
			t.Fatalf("cumulative snapshots must be superseded, not summed (want 250/40, never 350/50): %+v in=%v out=%v", u, u.Input.Sum, u.Output.Sum)
		}
		if u.CacheRead.Sum != nil || u.CacheRead.Unknown != 1 {
			t.Fatalf("an unreported token class is unknown, never zero: %+v", u.CacheRead)
		}
		days := map[string]string{}
		for _, d := range r.Days {
			days[d.Day] = d.State
		}
		if days["2026-10-01"] != report.DayUnknown || days["2026-10-02"] != report.DayMeasured {
			t.Fatalf("a day with nothing recorded is unknown, never zero: %v", days)
		}

		// The text rendering of the same store also runs end to end.
		txt := invoke(t, nil, t0.Add(time.Minute), "", "report", "--dir", dir, "--from", "2026-10-01", "--to", "2026-10-02")
		if txt.code != 0 || !strings.Contains(txt.stdout, "unknown") || !strings.Contains(txt.stdout, "deskdispatch") {
			t.Fatalf("text report: %+v", txt)
		}
	})

	t.Run("a missing store reports missing coverage, never a zero", func(t *testing.T) {
		o := invoke(t, nil, t0, "", "report", "--dir", t.TempDir(), "--from", "2026-10-01", "--to", "2026-10-01", "--json")
		if o.code != 0 {
			t.Fatalf("report on a missing store must succeed: %+v", o)
		}
		var r report.Report
		if err := json.Unmarshal([]byte(o.stdout), &r); err != nil {
			t.Fatal(err)
		}
		if r.State != perfdata.CoverageMissing || len(r.Days) != 1 || r.Days[0].State != report.DayUnknown || len(r.Tools) != 0 {
			t.Fatalf("missing store: %+v", r)
		}
		if r.Cost.Invocations != 0 || r.Cost.ElapsedMs != 0 {
			t.Fatalf("no invented totals: %+v", r.Cost)
		}
	})

	t.Run("a refused draft records nothing and never echoes its content", func(t *testing.T) {
		dir := t.TempDir()
		// A fake credential-shaped value, assembled at run time so the source carries no
		// literal token prefix for secret scanners to flag.
		secret := "gh" + "p_notARealTokenButLooksLikeOne0000000000"
		in := draftLine(t, perfdata.KindTool, "", wctx("inv-a", ""), toolAt(0, 10, 1)) +
			draftLine(t, perfdata.KindPhase, "", wctx("inv-a", ""), map[string]any{"phase": secret, "startedAt": at(0), "wall": ms(1), "cpuOwn": ms(1)})
		o := invoke(t, armedEnv, t0, in, "record", "--dir", dir, "--policy", pol)
		if o.code != 1 || strings.Contains(o.stderr+o.stdout, secret) {
			t.Fatalf("refusal must fail closed without echoing input: %+v", o)
		}
		rr, err := local.Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(rr.Events) != 0 {
			t.Fatalf("a batch with a refused draft records nothing: %d events", len(rr.Events))
		}
	})

	t.Run("zero transport: the shipped packages link no network or process code", func(t *testing.T) {
		if testing.Short() {
			t.Skip("needs the go command")
		}
		cmd := exec.Command("go", "list", "-deps", "./", "../../", "../../local", "../../report")
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list: %v", err)
		}
		deps := strings.Fields(string(b))
		if len(deps) < 20 {
			t.Fatalf("implausibly short dependency list: %v", deps)
		}
		if bad := transportDeps(deps); len(bad) != 0 {
			t.Fatalf("perfdata must not link transport or foreign modules: %v", bad)
		}
		// Planted control: the same check flags a network import, a process launcher
		// and a third-party module.
		planted := append(append([]string(nil), deps...), "net/http", "os/exec", "example.com/client")
		if bad := transportDeps(planted); len(bad) != 3 {
			t.Fatalf("control: the dependency check missed planted transport: %v", bad)
		}
	})
}

var transportRE = regexp.MustCompile(`^(net|net/http|net/rpc|net/smtp|crypto/tls|os/exec|plugin)(/.*)?$|^[^/]+\.[^/]+/`)

// transportDeps returns the dependencies that would give perfdata a way off the machine.
func transportDeps(deps []string) []string {
	var bad []string
	for _, d := range deps {
		if strings.HasPrefix(d, "github.com/medici-finance/assay/perfdata") {
			continue
		}
		if transportRE.MatchString(d) {
			bad = append(bad, d)
		}
	}
	return bad
}
