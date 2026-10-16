package deskkit

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// All paths read stay in tools/desk. No cross-module fixture is required.
func TestEveryDeskCommandRecordsPerformance(t *testing.T) {
	exemptions := map[string]string{"deskperf": "the read-only reporter must not add samples while inspecting a history", "muhar": "developer mutation harness, not a desk operation; preserves its own subprocess/exit cleanup"}
	files, err := filepath.Glob(filepath.Join("..", "..", "cmd", "*", "main.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("command inventory: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range files {
		name := filepath.Base(filepath.Dir(p))
		if _, ok := exemptions[name]; ok {
			seen[name] = true
			continue
		}
		if got := perfTool(name); got != name {
			t.Errorf("%s records as %s instead of its own tool key", name, got)
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "main" {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "PerfRun" {
					return true
				}
				if len(call.Args) != 4 {
					t.Errorf("%s: wrong performance entrypoint", name)
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok {
					t.Errorf("%s: tool key must be a literal", name)
					return true
				}
				key, err := strconv.Unquote(lit.Value)
				if err != nil || key != name {
					t.Errorf("%s: wrong recorded key %s", name, lit.Value)
				}
				count++
				return true
			})
		}
		if count != 1 {
			t.Errorf("%s has %d performance wrappers; want exactly one", name, count)
		}
	}
	for name := range exemptions {
		if !seen[name] {
			t.Errorf("stale performance exemption %s", name)
		}
	}
}

// Build actual binaries; help/version invocations need no forge or live credentials.
func TestNewDeskCommandsAppendPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("binary integration")
	}
	state := t.TempDir()
	bin := t.TempDir()
	names := []string{"deskfile", "deskreply", "scanloop", "issueboard", "reviewloop", "verifyloop", "deskevidence", "deskflip", "desksupervise"}
	for _, name := range names {
		exe := filepath.Join(bin, name)
		if os.PathSeparator == '\\' {
			exe += ".exe"
		}
		build := exec.Command("go", "build", "-o", exe, "./cmd/"+name)
		build.Dir = filepath.Join("..", "..")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v %s", name, err, out)
		}
		var optedInOutput []byte
		var optedInExit int
		for _, enabled := range []string{"1", "0"} {
			cmd := exec.Command(exe, "--version")
			env := []string{}
			for _, kv := range os.Environ() {
				k, _, _ := strings.Cut(kv, "=")
				if k != "HOME" && k != "USERPROFILE" && k != "ASSAY_TELEMETRY" && k != "DESK_LOOP" {
					env = append(env, kv)
				}
			}
			cmd.Env = append(env, "HOME="+state, "USERPROFILE="+state, "ASSAY_TELEMETRY="+enabled, "DESK_LOOP=worker-desk")
			out, _ := cmd.CombinedOutput()
			if cmd.ProcessState == nil {
				t.Fatalf("%s did not start", name)
			}
			if enabled == "1" {
				optedInOutput, optedInExit = out, cmd.ProcessState.ExitCode()
			} else if string(out) != string(optedInOutput) || cmd.ProcessState.ExitCode() != optedInExit {
				t.Fatalf("%s: telemetry changed output or exit status", name)
			}
		}
	}
	day := time.Now().UTC().Format("2006-01-02") + ".jsonl"
	b, err := os.ReadFile(filepath.Join(state, ".config", "assay", "perf", day))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var r PerfRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		if r.Schema != PerfSchema {
			t.Fatal("wrong schema")
		}
		counts[r.Tool]++
	}
	for _, name := range names {
		if counts[name] != 1 {
			t.Errorf("%s: got %d records; opted-in + opted-out invocations must yield one", name, counts[name])
		}
	}
	if len(counts) != len(names) {
		t.Fatalf("unexpected tool keys: %v", counts)
	}
}
