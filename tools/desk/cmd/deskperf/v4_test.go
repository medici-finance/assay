package main

import (
	"bytes"
	"encoding/json"
	"github.com/medici-finance/assay/perfrec"
	"os"
	"path/filepath"
	"testing"
)

// regression: #4360
func TestV4ReadReportFlow(t *testing.T) {
	// Build before this test redirects HOME; then exercise the shipped entry point.
	testV4CompiledFlow(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASSAY_PERF_PROFILE", "")
	id, _ := perfrec.NewID()
	c := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "workflow", Present: true}, AgentID: perfrec.Value{Value: id, Present: true}, Role: perfrec.Value{Value: "implement", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }})
	s := perfrec.BeginSelected(perfrec.SystemProbe(), perfrec.ProcessEntry(), []string{"fetch"}, c)
	s.Observe(perfrec.CounterHTTP)
	r := s.Finish("issueboard", "issues", "", 0, "")
	v4, e := perfrec.ToV4(r)
	if e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(home, ".config", "assay")
	if e = (&perfrec.Writer{}).Write(state, v4); e != nil {
		t.Fatal(e)
	}
	var out, diag bytes.Buffer
	if e = cmdReport([]string{"--json"}, &out, &diag); e != nil {
		t.Fatal(e)
	}
	var report reportJSON
	if e = json.Unmarshal(out.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	if report.PerfV4 == nil || report.PerfV4.Records != 1 || report.PerfV3 != nil || report.PerfV2 != nil || report.Summary.Records != 0 {
		t.Fatalf("mixed schema accounting: %s", out.Bytes())
	}
	if len(report.Commands.Rows) != 1 || report.Commands.Rows[0].Schema != perfrec.SchemaV4 || report.Commands.Rows[0].HTTPMeasured != 1 || report.Commands.Rows[0].HTTPAttempts != 0 {
		t.Fatal("v4 command overview lost coverage", out.String())
	}
	if bytes.Contains(out.Bytes(), []byte(id)) {
		t.Fatal("default report leaked workflow identity")
	}
	for _, args := range [][]string{{"--json", "--role", "implement"}, {"--json", "--perf-profile", "workflow", "--role", "implement", "--assigned-role", "implement"}, {"--json", "--assigned-role", "implement"}} {
		out.Reset()
		if e = cmdReport(args, &out, &diag); e == nil {
			t.Fatal("legacy role or missing workflow permission admitted", args)
		}
	}
	out.Reset()
	if e = cmdReport([]string{"--json", "--perf-profile", "workflow", "--group-by", "agent", "--agent-id", id}, &out, &diag); e != nil || !bytes.Contains(out.Bytes(), []byte(id)) {
		t.Fatal("supported workflow attribution lost", e)
	}
	out.Reset()
	if e = cmdReport([]string{"--by-day", "--json"}, &out, &diag); e != nil {
		t.Fatal(e)
	}
	var days reportDayJSON
	if e = json.Unmarshal(out.Bytes(), &days); e != nil || days.PerfV4 == nil || len(days.PerfV4ByDay) != 1 || days.ByDay.Records != 0 {
		t.Fatal("day cohorts mixed", e, out.String())
	}
	out.Reset()
	if e = cmdReport(nil, &out, &diag); e != nil || !bytes.Contains(out.Bytes(), []byte("perf-v4")) {
		t.Fatal("v4 human report missing", e)
	}
	// Real day-file mutation models absent attribution. Core role must not match.
	files, _ := filepath.Glob(filepath.Join(perfrec.Dir(state), "*.jsonl"))
	raw, _ := os.ReadFile(files[0])
	var wire map[string]any
	if e = json.Unmarshal(bytes.TrimSpace(raw), &wire); e != nil {
		t.Fatal(e)
	}
	delete(wire["extensions"].(map[string]any), "attribution")
	raw, _ = json.Marshal(wire)
	if e = os.WriteFile(files[0], append(raw, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	out.Reset()
	if e = cmdReport([]string{"--json", "--perf-profile", "workflow", "--assigned-role", "implement"}, &out, &diag); e == nil {
		t.Fatal("core role bypassed unavailable attribution")
	}
	out.Reset()
	if e = cmdReport([]string{"--json"}, &out, &diag); e != nil {
		t.Fatal("missing optional facet discarded core", e)
	}
}
