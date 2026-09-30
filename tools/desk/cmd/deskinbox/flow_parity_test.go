package main

// flow_parity_test.go — TestParityFlow: asserts interpretFlow (flow.go) byte-for-byte
// against the REAL jq program the oracle ships (write_flow_program, assay-inbox.sh's
// JQFLOW heredoc), extracted verbatim at test time and run through the system `jq` binary —
// the same discipline format_parity_test.go established for the format builder, and
// TestParityFlowText does the same for render_flow_text's terminal rendering.
//
// Needs `jq` on the runner (the oracle's own hard dependency). Its absence is
// could-not-check, per the three-state instrument rule — never a silent skip read as a pass
// (requireJQ, shared with format_parity_test.go).

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// extractJQFLOWProgram pulls the write_flow_program() JQFLOW heredoc body verbatim out of
// the oracle script.
func extractJQFLOWProgram(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(oracleScriptRelPath)
	if err != nil {
		t.Skipf("could-not-check: cannot read the oracle script at %s: %v", oracleScriptRelPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	start, end := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "cat > \"$TMP_FLOWFMT\" <<'JQFLOW'") {
			start = i + 1
			continue
		}
		if start != -1 && l == "JQFLOW" {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatalf("could not locate the write_flow_program JQFLOW heredoc in %s (start=%d end=%d) — "+
			"the oracle's own contract may have moved; re-locate the markers before trusting this test",
			oracleScriptRelPath, start, end)
	}
	return strings.Join(lines[start:end], "\n")
}

// extractRenderFlowTextProgram pulls render_flow_text()'s inline `jq -r '...'` program
// verbatim out of the oracle script (it is a quoted string literal, not a heredoc — bounded
// by the `jq -r '` line and the closing `  ' "$TMP_FLOW"` line inside that function).
func extractRenderFlowTextProgram(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(oracleScriptRelPath)
	if err != nil {
		t.Skipf("could-not-check: cannot read the oracle script at %s: %v", oracleScriptRelPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	fn, start, end := -1, -1, -1
	for i, l := range lines {
		if strings.HasPrefix(l, "render_flow_text() {") {
			fn = i
			continue
		}
		if fn != -1 && start == -1 && strings.TrimRight(l, " ") == "  jq -r '" {
			start = i + 1
			continue
		}
		if start != -1 && strings.HasPrefix(l, "  ' \"$TMP_FLOW\"") {
			end = i
			break
		}
	}
	if fn == -1 || start == -1 || end == -1 {
		t.Fatalf("could not locate render_flow_text's inline jq program in %s (fn=%d start=%d end=%d) — "+
			"the oracle's own contract may have moved; re-locate the markers before trusting this test",
			oracleScriptRelPath, fn, start, end)
	}
	return strings.Join(lines[start:end], "\n")
}

func runOracleJQ(t *testing.T, program string, rawFlag bool, inputFiles ...string) []byte {
	t.Helper()
	dir := t.TempDir()
	progFile := filepath.Join(dir, "prog.jq")
	if err := os.WriteFile(progFile, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	args := []string{}
	if rawFlag {
		args = append(args, "-r")
	}
	args = append(args, "-f", progFile)
	args = append(args, inputFiles...)
	cmd := exec.Command("jq", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("jq failed: %v\nstderr: %s", err, errb.String())
	}
	return out.Bytes()
}

// flowFixture is one raw-envelope fixture, spelled in the oracle's OWN field names
// (collect_flow's output shape, assay-inbox.sh:982-1006) so the identical bytes drive both
// the real jq program and this port's json.Unmarshal into rawDoc.
type flowFixture struct {
	name string
	raw  string
}

func flowFixtures() []flowFixture {
	return []flowFixture{
		{
			name: "single cell, all readers ok",
			raw: `{
  "asOf": "2026-09-25T00:00:00Z", "since": "",
  "cells": [{
    "cell": "example-org/example-repo", "root": ".", "sha": "abc1234",
    "bottleneck": {"constraint":"todo","stages":[
      {"stage":"todo","wip":5,"median_dwell":"2d","unknown_dwell":0},
      {"stage":"in-progress","wip":3,"median_dwell":"1d","unknown_dwell":1},
      {"stage":"implemented","wip":2,"median_dwell":"3d","unknown_dwell":0},
      {"stage":"verified","wip":1,"median_dwell":"","unknown_dwell":1},
      {"stage":"done","wip":10,"median_dwell":"","unknown_dwell":0}]},
    "bottleneckErr": "",
    "intake": {"state":"measured","untriaged":4}, "intakeErr": "",
    "netflow": {"state":"ok","streams":[{"arrivals":2,"completions":1},{"arrivals":1,"completions":3}]},
    "netflowErr": ""
  }],
  "throughput": {"bottleneck":"dispatch","stagesRead":4,"stagesTotal":4,"advice":"widen worker-desk",
    "stages": [
      {"stage":"dispatch","loop":"worker-desk","depth":7,"slots":4,"slotsSource":"stored width","ratio":1.75,"maxSlots":8,"boundBy":"ceiling","blind":"","depthNote":"eligible unclaimed todo"},
      {"stage":"review","loop":"pr-review-desk","depth":2,"slots":4,"slotsSource":"shipped default","ratio":0.5,"maxSlots":8,"boundBy":"","blind":"","depthNote":"open PRs no verdict"},
      {"stage":"verify","loop":"verify-desk","depth":1,"slots":2,"slotsSource":"shipped default","ratio":0.5,"maxSlots":4,"boundBy":"","blind":"","depthNote":"awaiting verifier"},
      {"stage":"intake","loop":"intake-desk","depth":0,"slots":1,"slotsSource":"shipped default","ratio":0.0,"maxSlots":2,"boundBy":"","blind":"","depthNote":"untriaged"}]},
  "throughputErr": ""
}`,
		},
		{
			name: "two cells, one fully blind, AT LEAST + could-not-check + n/a all exercised",
			raw: `{
  "asOf": "2026-09-25T00:00:00Z", "since": "2026-09-01",
  "cells": [
    {
      "cell": "example-org/repo-a", "root": "../repo-a", "sha": "aaa1111",
      "bottleneck": {"constraint":"todo","stages":[
        {"stage":"todo","wip":5,"median_dwell":"2d","unknown_dwell":0},
        {"stage":"in-progress","wip":3,"median_dwell":"1d","unknown_dwell":1},
        {"stage":"implemented","wip":2,"median_dwell":"3d","unknown_dwell":0},
        {"stage":"verified","wip":1,"median_dwell":"","unknown_dwell":1},
        {"stage":"done","wip":10,"median_dwell":"","unknown_dwell":0}]},
      "bottleneckErr": "",
      "intake": {"state":"measured","untriaged":4}, "intakeErr": "",
      "netflow": {"state":"ok","streams":[{"arrivals":2,"completions":1}]}, "netflowErr": ""
    },
    {
      "cell": "example-org/repo-b", "root": "../repo-b", "sha": "could-not-check",
      "bottleneck": null, "bottleneckErr": "flag provided but not defined: -bottleneck",
      "intake": null, "intakeErr": "flag provided but not defined: -intake-debt",
      "netflow": null, "netflowErr": "flag provided but not defined: -net-flow"
    }
  ],
  "throughput": {"bottleneck":"dispatch","stagesRead":4,"stagesTotal":4,"advice":"widen worker-desk",
    "stages": [
      {"stage":"dispatch","loop":"worker-desk","depth":7,"slots":4,"slotsSource":"stored width","ratio":1.75,"maxSlots":8,"boundBy":"ceiling","blind":"","depthNote":"eligible unclaimed todo"},
      {"stage":"review","loop":"pr-review-desk","depth":2,"slots":4,"slotsSource":"shipped default","ratio":0.5,"maxSlots":8,"boundBy":"","blind":"","depthNote":"open PRs no verdict"},
      {"stage":"verify","loop":"verify-desk","depth":1,"slots":2,"slotsSource":"shipped default","ratio":0.5,"maxSlots":4,"boundBy":"","blind":"","depthNote":"awaiting verifier"},
      {"stage":"intake","loop":"intake-desk","depth":0,"slots":1,"slotsSource":"shipped default","ratio":0.0,"maxSlots":2,"boundBy":"","blind":"","depthNote":"untriaged"}]},
  "throughputErr": ""
}`,
		},
		{
			name: "throughput itself unread — the whole fleet queue/slots/ratio side goes blind",
			raw: `{
  "asOf": "2026-09-25T00:00:00Z", "since": "",
  "cells": [{
    "cell": "example-org/example-repo", "root": ".", "sha": "abc1234",
    "bottleneck": {"constraint":"done","stages":[
      {"stage":"todo","wip":0,"median_dwell":"","unknown_dwell":0},
      {"stage":"in-progress","wip":0,"median_dwell":"","unknown_dwell":0},
      {"stage":"implemented","wip":0,"median_dwell":"","unknown_dwell":0},
      {"stage":"verified","wip":0,"median_dwell":"","unknown_dwell":0},
      {"stage":"done","wip":3,"median_dwell":"1d","unknown_dwell":0}]},
    "bottleneckErr": "",
    "intake": {"state":"measured","untriaged":0}, "intakeErr": "",
    "netflow": {"state":"ok","streams":[]}, "netflowErr": ""
  }],
  "throughput": null, "throughputErr": "flag provided but not defined: -throughput"
}`,
		},
	}
}

func assertFlowModelMatchesOracle(t *testing.T, name, raw string) flowModel {
	t.Helper()
	program := extractJQFLOWProgram(t)
	dir := t.TempDir()
	rawFile := filepath.Join(dir, "raw.json")
	if err := os.WriteFile(rawFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write raw fixture: %v", err)
	}
	wantBytes := runOracleJQ(t, program, false, rawFile)
	var want flowModel
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("%s: cannot decode oracle jq output: %v\nraw: %s", name, err, wantBytes)
	}

	var doc rawDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("%s: cannot decode fixture into rawDoc: %v", name, err)
	}
	got := interpretFlow(doc)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: flow model mismatch\n got:  %#v\nwant: %#v", name, got, want)
	}
	return want
}

func TestParityFlow(t *testing.T) {
	requireJQ(t)
	for _, fx := range flowFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			assertFlowModelMatchesOracle(t, fx.name, fx.raw)
		})
	}
}

func TestParityFlowText(t *testing.T) {
	requireJQ(t)
	renderProgram := extractRenderFlowTextProgram(t)
	flowProgram := extractJQFLOWProgram(t)
	for _, fx := range flowFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			rawFile := filepath.Join(dir, "raw.json")
			if err := os.WriteFile(rawFile, []byte(fx.raw), 0o644); err != nil {
				t.Fatalf("write raw fixture: %v", err)
			}
			modelFile := filepath.Join(dir, "model.json")
			modelBytes := runOracleJQ(t, flowProgram, false, rawFile)
			if err := os.WriteFile(modelFile, modelBytes, 0o644); err != nil {
				t.Fatalf("write model file: %v", err)
			}
			want := runOracleJQ(t, renderProgram, true, modelFile)

			var doc rawDoc
			if err := json.Unmarshal([]byte(fx.raw), &doc); err != nil {
				t.Fatalf("cannot decode fixture into rawDoc: %v", err)
			}
			m := interpretFlow(doc)
			var buf bytes.Buffer
			renderFlowText(&buf, m)

			if buf.String() != string(want) {
				t.Errorf("%s: rendered text mismatch\n got:\n%s\nwant:\n%s", fx.name, buf.String(), want)
			}
		})
	}
}
