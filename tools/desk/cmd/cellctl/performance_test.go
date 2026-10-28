package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
)

// regression: desktools-v2/64
func TestPerfV3RealDeskLaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("existing recording harness fixture uses a Unix shell")
	}
	child := filepath.Join(t.TempDir(), "deskclaim")
	build := exec.Command("go", "build", "-o", child, "../deskclaim")
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("child build: %v %s", err, out)
	}
	for _, harness := range []string{"claude", "codex"} {
		t.Run(harness, func(t *testing.T) {
			f := newPolicyFixture(t, "2.1.278")
			f.prepareLocalLaunch(t)
			home := filepath.Join(f.cellDir, "home")
			authority := filepath.Join(f.cellDir, "authority.role")
			nl := string(rune(10))
			script := strings.Join([]string{"#!/bin/sh", "case $1 in", "--version) echo '2.1.278 (stub)'; exit 0;;", "plugin) exit 0;;", "esac", "export HOME=" + bashQuote(home) + " USERPROFILE=" + bashQuote(home), "printf '%s' \"$DESK_LOOP\" > " + bashQuote(authority), "exec " + bashQuote(child) + " list --perf-loop-role=worker-desk", ""}, nl)
			must(t, os.WriteFile(filepath.Join(f.binDir, harness), []byte(script), 0700))
			path := filepath.Join(f.cellDir, "cell.env")
			base, err := os.ReadFile(path)
			must(t, err)
			must(t, os.WriteFile(path, append(base, []byte("ASSAY_TELEMETRY=1"+nl)...), 0600))
			parent := strings.Repeat("a", 32)
			result := f.run(t, []string{"CELLCTL_DESKWT=0", "PATH=" + f.binDir + ":/usr/bin:/bin", "ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=workflow", "ASSAY_PERF_AGENT_ID=" + parent, "ASSAY_PERF_SESSION_ID=" + strings.Repeat("b", 32), "DESK_LOOP=worker-desk"}, "desk", "example", "pr-review-desk", "--harness", harness, "--cadence", "off")
			// The harness deliberately runs a read-only local tool; its existing
			// activation refusal, if any, remains observable but cannot drop diagnostics.
			files, _ := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "*.jsonl"))
			var records []perfrec.Record
			for _, p := range files {
				raw, err := os.ReadFile(p)
				must(t, err)
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), nl) {
					var r perfrec.Record
					must(t, json.Unmarshal([]byte(line), &r))
					if r.Tool == "deskclaim" {
						records = append(records, r)
					}
				}
			}
			if len(records) != 1 {
				t.Fatalf("real %s launch records=%d code=%d stderr=%s", harness, len(records), result.code, result.stderr)
			}
			operational, err := os.ReadFile(authority)
			must(t, err)
			if string(operational) != "pr-review-desk" {
				t.Fatal("performance attribution changed operational desk authority")
			}
			parentFiles, _ := filepath.Glob(filepath.Join(f.cellDir, ".config", "assay", "perf", "*.jsonl"))
			var parentID string
			for _, path := range parentFiles {
				raw, err := os.ReadFile(path)
				must(t, err)
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), nl) {
					var row perfrec.Record
					must(t, json.Unmarshal([]byte(line), &row))
					if row.Tool == "cellctl" && row.Schema == perfrec.SchemaV3 {
						parentID = row.V3.Attribution.InvocationID
					}
				}
			}
			r := records[0]
			if parentID == "" || r.V3.Attribution.ParentInvocationID != parentID {
				t.Fatal("child parent invocation does not name the recorded cellctl invocation")
			}
			if r.Schema != perfrec.SchemaV3 || r.V3.Attribution.AgentID == parent || r.V3.Attribution.AgentID == "" || r.V3.Attribution.SessionID == "" || r.V3.Attribution.SessionID == strings.Repeat("b", 32) || r.V3.Attribution.DeskRole != "worker-desk" || r.V3.Attribution.Role != "review" || r.V3.Attribution.ParentAgentID != parent || len(r.V3.Attribution.Work) != 0 {
				t.Fatal("real launch lost identity, role or shared-work boundary")
			}
			if strings.Contains(result.stdout+result.stderr, parent) {
				t.Fatal("launcher logged performance identity")
			}
		})
	}
}
