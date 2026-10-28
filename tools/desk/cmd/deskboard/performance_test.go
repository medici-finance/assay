package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: desktools-v2/64
func TestPerfBoardParsedRecorder(t *testing.T) {
	r := cliRecord(t)
	baselineCode, baselineOut, baselineErr := r.exec("prs")
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "workflow")
	t.Setenv("ASSAY_PERF_ROLE", "implement")
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	args := []string{"prs", "--perf-role=review", "--perf-work="}
	var out, errs bytes.Buffer
	record := func(run func() int) int {
		return deskkit.PerfRunSelected("deskboard", perfVerbs, args, selected, func([]string) int { return run() })
	}
	code := executeRecorded(args, &out, &errs, nil, record, func(c perfrec.Context) { selected = c })
	if code != baselineCode || r.calls != 2 || out.String() != baselineOut || errs.String() != baselineErr {
		t.Fatal("optional flags changed board operation")
	}
	p := filepath.Join(os.Getenv("HOME"), ".config", "assay", "perf", time.Now().UTC().Format("2006-01-02")+".jsonl")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var rec perfrec.Record
	if err := json.Unmarshal(bytes.TrimSpace(raw), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Schema != perfrec.SchemaV3 || rec.V3.Attribution.Role != "review" {
		t.Fatal("board ignored parsed profile/context")
	}
	before := append([]byte(nil), raw...)
	if n := executeRecorded([]string{"--help", "--perf-profile=workflow"}, &out, &errs, nil, record); n != 0 {
		t.Fatal("help refused")
	}
	raw, err = os.ReadFile(p)
	if err != nil || !bytes.Equal(before, raw) {
		t.Fatal("help wrote a measurement")
	}
}
