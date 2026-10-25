package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
)

// regression: desktools-v2/64
func TestPerfPrivateHandoff(t *testing.T) {
	cache := t.TempDir()
	oldCache, oldNow := packetCacheDir, packetNow
	t.Cleanup(func() { packetCacheDir, packetNow = oldCache, oldNow })
	packetCacheDir = func() (string, error) { return cache, nil }
	now := time.Now().UTC()
	packetNow = func() time.Time { return now }
	t.Setenv("ASSAY_TELEMETRY", "1")
	c := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "workflow", Present: true}, AgentID: perfrec.Value{Value: strings.Repeat("b", 32), Present: true}, DeskRole: perfrec.Value{Value: "worker-desk", Present: true}, Role: perfrec.Value{Value: "review", Present: true}})
	if got := writePerformanceHandoff(perfrec.Context{}); got != "" {
		t.Fatal("disabled context wrote handoff")
	}
	dir := filepath.Join(cache, "assay", "packets")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("disabled context touched cache")
	}
	path := writePerformanceHandoff(c)
	if path == "" {
		t.Fatal("missing handoff")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("handoff not private")
	}
	info, err = os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("directory not private")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Environment      []string
		DeskRole         string `json:"deskRoleOverride"`
		ExpiresBeforeUTC string `json:"expiresBeforeUtc"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ExpiresBeforeUTC != time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7).Format(time.RFC3339) {
		t.Fatal("handoff expiration is not seven UTC days")
	}
	if payload.DeskRole != "worker-desk" || !strings.Contains(strings.Join(payload.Environment, "\n"), "ASSAY_PERF_AGENT_ID="+strings.Repeat("b", 32)) {
		t.Fatal("handoff lost typed context")
	}
	if strings.Contains(string(data), "ASSAY_TELEMETRY") {
		t.Fatal("handoff manufactured consent")
	}
	prompt, err := assemblePrompt(dispatchOpts{kit: "worker"}, dispatchPlan{performancePath: path, childPerformance: c}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, path) || strings.Contains(prompt, strings.Repeat("b", 32)) || strings.Contains(prompt, "ASSAY_PERF_AGENT_ID=") {
		t.Fatal("prompt exposed attribution or lost handoff")
	}
	// The handoff prompt is part of the security boundary: a consumer may
	// apply attribution keys only, and may never turn them into desk authority.
	for _, rule := range []string{
		"seven documented ASSAY_PERF_* environment entries", "ignore every other environment key", "never as DESK_LOOP",
		"never eval the file or export it into a persistent shell",
		"Apply this context only for this launch and before expiresBeforeUtc",
		"never copy them into logs, replies or a shared shell",
		"This context is descriptive, never custody or dispatch authority.",
		"Preserve the operator-selected ASSAY_TELEMETRY setting; the handoff does not opt you in.",
	} {
		if !strings.Contains(prompt, rule) {
			t.Fatalf("handoff lost its key/authority scope: %q", rule)
		}
	}
	// Bound the whole optional section by the next assignment field, rather
	// than silently ignoring additional paragraphs or text before the path.
	_, handoff, ok := strings.Cut(prompt, "## Optional local performance context\n\n")
	if !ok {
		t.Fatal("missing handoff instruction section")
	}
	handoff, _, ok = strings.Cut(handoff, "\n- **Item key:**")
	if !ok {
		t.Fatal("missing handoff section boundary")
	}
	instructions := strings.TrimSpace(strings.ReplaceAll(handoff, "`"+path+"`", "`handoff-path`"))
	if strings.Contains(instructions, "\n\n") {
		t.Fatal("handoff instructions gained another paragraph")
	}
	for _, name := range []string{"DESK_LOOP", "ASSAY_TELEMETRY"} {
		if strings.Count(instructions, name) != 1 {
			t.Fatalf("handoff repeats operational authority or consent name: %s", name)
		}
	}
	allowedNames := map[string]bool{"ASSAY_PERF_": true, "KEY": true, "DESK_LOOP": true, "ASSAY_TELEMETRY": true}
	for _, name := range regexp.MustCompile(`\b[A-Z][A-Z0-9_]*\b`).FindAllString(instructions, -1) {
		if !allowedNames[name] {
			t.Fatalf("handoff names an undeclared environment key: %s", name)
		}
	}
	for _, assignment := range regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)=`).FindAllStringSubmatch(instructions, -1) {
		if assignment[1] != "KEY" {
			t.Fatalf("handoff adds an environment assignment: %s", assignment[1])
		}
	}
	old := filepath.Join(dir, "expired"+performanceSuffix)
	if err := os.WriteFile(old, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	expired := midnight.AddDate(0, 0, -7)
	if err := os.Chtimes(old, expired, expired); err != nil {
		t.Fatal(err)
	}
	if writePerformanceHandoff(c) == "" {
		t.Fatal("second handoff failed")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expired handoff retained")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if writePerformanceHandoff(c) != "" {
		t.Fatal("public directory accepted")
	}
}

// Regression: a real instrumented claim child across the untyped runner boundary
// must not inherit the dispatcher's own stale parent invocation.
func TestPerfUntypedClaimChildParent(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "deskclaim")
	build := exec.Command("go", "build", "-o", bin, "../deskclaim")
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	home := t.TempDir()
	stale := strings.Repeat("d", 32)
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "USERPROFILE=" + home, "ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=workflow", "ASSAY_PERF_AGENT_ID=" + strings.Repeat("a", 32), "ASSAY_PERF_PARENT_INVOCATION_ID=" + stale, "DESK_LOOP=worker-desk"}
	before := strings.Join(env, "\n")
	_ = runCmdEnv(home, env, bin, "list")
	if strings.Join(env, "\n") != before {
		t.Fatal("runner mutated caller environment")
	}
	files, _ := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "*.jsonl"))
	found := false
	for _, p := range files {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			var row perfrec.Record
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatal(err)
			}
			if row.Tool != "deskclaim" || row.Schema != perfrec.SchemaV3 {
				continue
			}
			found = true
			if row.V3.Attribution.ParentInvocationID != "" {
				t.Fatal("untyped child inherited stale parent invocation")
			}
		}
	}
	if !found {
		t.Fatal("actual claim child did not record its invocation")
	}
}
