package main

import (
	"bytes"
	"encoding/json"
	"github.com/medici-finance/assay/perfrec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: class perf-v3-report-profile
func TestV3ActualReport(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASSAY_PERF_PROFILE", "")
	id, _ := perfrec.NewID()
	c := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "workflow", Present: true}, AgentID: perfrec.Value{Value: id, Present: true}, Role: perfrec.Value{Value: "implement", Present: true}, LookupEnv: func(k string) (string, bool) {
		if k == perfrec.EnvVar {
			return "1", true
		}
		return "", false
	}})
	p := perfrec.Probe{}
	r := perfrec.BeginSelected(p, p.Mark(), nil, c).Finish("deskboard", "query", "", 0, "")
	if e := (&perfrec.Writer{}).Write(filepath.Join(home, ".config", "assay"), r); e != nil {
		t.Fatal(e)
	}
	var out, diag bytes.Buffer
	if e := cmdReport([]string{"--json"}, &out, &diag); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(out.Bytes(), []byte(id)) {
		t.Fatal("default report exposed workflow identity")
	}
	var report reportJSON
	if e := json.Unmarshal(out.Bytes(), &report); e != nil || report.PerfV3 == nil || report.PerfV3.Records != 1 {
		t.Fatal("v3 report missing", e)
	}
	out.Reset()
	if e := cmdReport([]string{"--json", "--role", "implement"}, &out, &diag); e == nil {
		t.Fatal("legacy role filter bypassed workflow permission")
	}
	out.Reset()
	if e := cmdReport([]string{"--json", "--group-by", "agent"}, &out, &diag); e == nil {
		t.Fatal("group permitted without workflow consent")
	}
	for _, raw := range []string{`{"kind":"arbitrary"}`, `{"kind":"issue","forge":"github","instance":"github.com","repo":"example/one","issue":7} {}`} {
		out.Reset()
		if e := cmdReport([]string{"--json", "--perf-profile", "workflow", "--work", raw}, &out, &diag); e == nil || !strings.Contains(e.Error(), "invalid work filter") || out.Len() != 0 {
			t.Fatalf("invalid work filter was not refused: %v", e)
		}
	}
	out.Reset()
	if e := cmdReport([]string{"--json", "--perf-profile", "workflow", "--group-by", "agent", "--agent-id", id}, &out, &diag); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte(id)) {
		t.Fatal("workflow filter lost identity")
	}
}
