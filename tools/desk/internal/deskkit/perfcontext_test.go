package deskkit

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
)

// regression: desktools-v2/64
func TestPerfV3LauncherChildFlow(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "deskclaim")
	build := exec.Command("go", "build", "-o", bin, "./cmd/deskclaim")
	build.Dir = filepath.Join("..", "..")
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	parent := perfrec.ResolveContext(perfrec.ContextInput{LookupEnv: func(k string) (string, bool) {
		v, ok := map[string]string{"ASSAY_TELEMETRY": "1", "ASSAY_PERF_PROFILE": "workflow", "ASSAY_PERF_SESSION_ID": strings.Repeat("a", 32), "ASSAY_PERF_AGENT_ID": strings.Repeat("b", 32), "DESK_LOOP": "worker-desk"}[k]
		return v, ok
	}})
	home := t.TempDir()
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "USERPROFILE=" + home, "KUBECONFIG=/dev/null", "ASSAY_TELEMETRY=1", "DESK_LOOP=pr-review-desk"}
	brief := perfrec.WorkRef{Kind: "brief", Repo: "example/project", Brief: "example:project:work:1", Primary: true}
	issue := perfrec.WorkRef{Kind: "issue", Forge: "github", Instance: "github.com", Repo: "example/project", Issue: 2, Primary: true}
	contexts := []perfrec.Context{PerfAgentContext(parent, "implement", []perfrec.WorkRef{brief}, nil), PerfAgentContext(parent, "review", []perfrec.WorkRef{issue}, nil)}
	// The existing native fixture observes role after its own launch composition.
	// Also exercise the shared executor directly: diagnostic desk attribution must
	// preserve the caller-selected operational loop before a child handler starts.
	probeHome := t.TempDir()
	probeSource := filepath.Join(probeHome, "role.go")
	if err := os.WriteFile(probeSource, []byte("package main\nimport (\"fmt\"; \"os\")\nfunc main(){fmt.Print(os.Getenv(\"DESK_LOOP\"))}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(probeHome, "role-probe")
	if out, err := goquiet.Cmd(exec.Command("go", "build", "-o", probe, probeSource)).CombinedOutput(); err != nil {
		t.Fatalf("probe build: %v %s", err, out)
	}
	authority := Run(ToolCall{Name: probe, Env: env, Context: WithPerfContext(context.Background(), contexts[0])})
	if authority.Stdout != "pr-review-desk" || authority.ExitCode != 0 {
		t.Fatal("shared executor changed operational desk authority")
	}

	var wg sync.WaitGroup
	results := make([]ToolRun, 2)
	for i, c := range contexts {
		wg.Add(1)
		go func(i int, c perfrec.Context) {
			defer wg.Done()
			// The explicit flag preserves originating attribution while the operational
			// child loop remains independently selected by its launcher.
			results[i] = Run(ToolCall{Name: bin, Args: []string{"list", "--perf-loop-role=worker-desk"}, Env: env, Dir: home, Context: WithPerfContext(context.Background(), c)})
		}(i, c)
	}
	wg.Wait()
	for _, r := range results {
		if strings.Contains(r.Stdout+r.Stderr, "perf-agent") {
			t.Fatal("metadata echoed")
		}
	}
	day := filepath.Join(home, ".config", "assay", "perf", time.Now().UTC().Format("2006-01-02")+".jsonl")
	records, skipped, err := readPerfDayFile(day)
	if err != nil || skipped != 0 || len(records) != 2 {
		t.Fatalf("writer/reader records=%d skipped=%d err=%v", len(records), skipped, err)
	}
	seen := map[string]bool{}
	inv := map[string]bool{}
	for _, r := range records {
		if r.Schema != perfrec.SchemaV3 {
			t.Fatal("selected child did not write v3")
		}
		d := r.V3.Attribution
		if d.AgentID == parent.Data().AgentID || d.SessionID == parent.Data().SessionID || d.ParentAgentID != parent.Data().AgentID || d.DeskRole != "worker-desk" {
			t.Fatal("child inherited parent identity/authority")
		}
		if inv[d.InvocationID] {
			t.Fatal("child reused invocation")
		}
		inv[d.InvocationID] = true
		found := false
		for _, c := range contexts {
			cd := c.Data()
			if d.AgentID == cd.AgentID {
				found = true
				seen[d.Role] = true
				if d.InvocationID == c.InvocationID() || d.ParentInvocationID != c.InvocationID() || len(d.Work) != 1 || d.Work[0] != cd.Work[0] {
					t.Fatal("parent invocation or work crossed launch")
				}
			}
		}
		if !found {
			t.Fatal("child agent missing")
		}
	}
	if !seen["review"] || !seen["implement"] {
		t.Fatal("assigned role lost")
	}
	summary, err := perfrec.ReduceV3(records, perfrec.ProfileWorkflow, "work", perfrec.V3Filter{})
	if err != nil || summary.Records != 2 || len(summary.Groups) != 2 {
		t.Fatal("reader merged distinct subjects")
	}
	// Retry is a new tool invocation under the same agent, not a replacement.
	retry := Run(ToolCall{Name: bin, Args: []string{"list"}, Env: env, Dir: home, Context: WithPerfContext(context.Background(), contexts[0])})
	_ = retry
	records, _, _ = readPerfDayFile(day)
	if len(records) != 3 || inv[records[2].V3.Attribution.InvocationID] {
		t.Fatal("retry reused invocation")
	}
	// Tool profile strips inherited linkage even at the typed child seam.
	tool := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }})
	clean := perfrec.ContextEnv(env, tool, true)
	cenv := func(k string) (string, bool) {
		for _, kv := range clean {
			name, v, _ := strings.Cut(kv, "=")
			if k == name {
				return v, true
			}
		}
		return "", false
	}
	td := perfrec.ResolveContext(perfrec.ContextInput{LookupEnv: cenv}).Data()
	if td.AgentID != "" || td.SessionID != "" || len(td.Work) != 0 || td.ParentInvocationID != "" {
		t.Fatal("tool profile leaked workflow")
	}
	// Off consent and absent protocol admission never introduce an opt-in. There
	// is no service protocol in this boundary, so no service context is invented.
	offHome := t.TempDir()
	offEnv := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + offHome, "USERPROFILE=" + offHome, "ASSAY_TELEMETRY=0", "DESK_LOOP=worker-desk"}
	off := Run(ToolCall{Name: bin, Args: []string{"list"}, Env: offEnv, Dir: offHome, Context: WithPerfContext(context.Background(), contexts[0])})
	_ = off
	if files, _ := filepath.Glob(filepath.Join(offHome, ".config", "assay", "perf", "*.jsonl")); len(files) != 0 {
		t.Fatal("launcher created consent")
	}
	// An explicit unknown caller leaves the parent unknown.
	unknown := perfrec.ChildContext(perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "workflow", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }}), "review", nil, nil)
	if unknown.Data().ParentAgentID != "" {
		t.Fatal("unknown parent guessed")
	}
}

// regression: desktools-v2/64
func TestPerfChildEnvironmentCopies(t *testing.T) {
	base := []string{"DESK_LOOP=worker-desk", "ASSAY_TELEMETRY=0", "ASSAY_PERF_WORK=stale"}
	before := append([]string(nil), base...)
	_ = PerfContextEnv(base, perfrec.Context{}, true)
	a, _ := json.Marshal(base)
	b, _ := json.Marshal(before)
	if !bytes.Equal(a, b) {
		t.Fatal("child context mutated shared environment")
	}
}
