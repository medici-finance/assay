package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// prepEnvironment keeps the compiled CLI real. Only the Python interpreter is
// wrapped to emulate the image UID and fixed launcher path in an offline fixture.
func prepEnvironment(t *testing.T, report bool, uid int) []string {
	t.Helper()
	root := t.TempDir()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("could-not-check: Python required", err)
	}
	launcher, err := filepath.Abs("../../../../containers/k8s/compatibility/desk-session.py")
	if err != nil {
		t.Fatal(err)
	}
	body := "#!/" + strings.TrimPrefix(python, "/") + "\nimport sys, os\n"
	if report {
		body += "print(sys.stdin.read(), end='')\n"
	} else {
		replacement := fmt.Sprintf("LAUNCHER = Path(%q)", launcher)
		body += fmt.Sprintf("os.geteuid = lambda: %d\nos.getegid = lambda: 999\n", uid)
		body += fmt.Sprintf("source = sys.argv[-1].replace(%q, %q)\n", "LAUNCHER = Path('/usr/local/lib/assay/desk-session.py')", replacement)
		body += "exec(compile(source, '<preparation-fixture>', 'exec'), {'__name__': '__main__'})\n"
	}
	if err := os.WriteFile(filepath.Join(root, "python3"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + root + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + root,
		"KUBECONFIG=/dev/null", "PYTHONDONTWRITEBYTECODE=1", "ASSAY_TELEMETRY=0"}
}

func prepArgs(role string) []string {
	return []string{"compat-prepare", "--cell", "example", "--role", role,
		"--agent-version", "2.1.228", "--model", "claude-sonnet-5-5"}
}

func prepJSON(t *testing.T, result cliRun) map[string]any {
	t.Helper()
	if result.Code != 0 || result.Stderr != "" {
		t.Fatalf("exit %d stderr=%s stdout=%s", result.Code, result.Stderr, result.Stdout)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &value); err != nil {
		t.Fatal(err, result.Stdout)
	}
	return value
}

func testCompatPreparationConfigFlow(t *testing.T) {
	t.Helper()
	env := append(prepEnvironment(t, true, 999), "WORK=/unapproved", "MODEL=unapproved", "CELL=unapproved")
	bin := cellctlBinary(t)
	base := prepArgs("worker-desk")
	defaulted := prepJSON(t, execIn(t, bin, env, base...))
	if defaulted["work"] != "/work" || defaulted["model"] != "claude-sonnet-5-5" || defaulted["prepare"] != false || defaulted["readback"] != false {
		t.Fatal("undeclared environment or mode changed defaults", defaulted)
	}
	for _, value := range []string{"/work/a b", `C:\desk\work`, `\\server\share`, "../relative", "-dash=one,two", "é/role"} {
		for _, words := range [][]string{{"--work", value}, {"--work=" + value}} {
			args := append(append([]string{}, base...), words...)
			got := prepJSON(t, execIn(t, bin, env, args...))
			if got["work"] != value {
				t.Fatalf("flag bytes changed: %q -> %v", value, got["work"])
			}
		}
	}
	args := append(append([]string{}, base...), "--work", "first", "--work=", "--agent", "/a", "--version-manifest", "/p", "--instructions", "/i", "--workdir", "/r", "--readback")
	got := prepJSON(t, execIn(t, bin, env, args...))
	for key, want := range map[string]any{"work": "", "agent": "/a", "version_manifest": "/p", "instructions": "/i", "workdir": "/r", "readback": true} {
		if got[key] != want {
			t.Fatalf("%s=%v want %v", key, got[key], want)
		}
	}
}

func testCompatPreparationLegacyForms(t *testing.T) {
	t.Helper()
	bin := cellctlBinary(t)
	env := prepEnvironment(t, true, 999)
	for _, args := range [][]string{
		{"compat-prepare", "--unknown"},
		{"compat-prepare", "--cell", "example"},
		append(prepArgs("worker-desk"), "--prepare", "--readback"),
		append(prepArgs("worker-desk"), "--", "positional"),
		append(prepArgs("worker-desk"), "--work"),
	} {
		got := execIn(t, bin, env, args...)
		if got.Code != 2 || got.Stdout != "" || !strings.Contains(got.Stderr, "--help") {
			t.Fatalf("parse refusal %v: %+v", args, got)
		}
	}
	for _, args := range [][]string{{"compat-prepare", "--help"}, {"compat-prepare", "-h"}, {"help", "compat-prepare"}} {
		got := execIn(t, bin, env, args...)
		if got.Code != 0 || got.Stderr != "" || !strings.Contains(got.Stdout, "--prepare") || strings.Contains(got.Stdout, `"work":`) {
			t.Fatalf("help reached the interpreter or read configuration: %+v", got)
		}
	}
}

// TestCLICompatPreparation exercises real preparation through the compiled CLI,
// the embedded domain, and the existing image launcher against synthetic files.
func TestCLICompatPreparation(t *testing.T) {
	bin := cellctlBinary(t)
	t.Run("flag-config-flow", testCompatPreparationConfigFlow)
	t.Run("legacy-forms-help-refusals", testCompatPreparationLegacyForms)
	t.Run("delegate-has-no-parser", func(t *testing.T) {
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Fatal(err)
		}
		shim, err := filepath.Abs("../../../../containers/k8s/compatibility/prepare.py")
		if err != nil {
			t.Fatal(err)
		}
		check := `import ast,sys
source=ast.parse(open(sys.argv[1]).read())
expected=ast.parse("import os\nimport sys\nos.execv('/opt/assay-preparation/cellctl', ['cellctl', 'compat-prepare', *sys.argv[1:]])")
assert ast.dump(ast.Module(body=source.body[1:], type_ignores=[])) == ast.dump(expected), 'delegate must contain only argv forwarding'
`
		if out, err := exec.Command(python, "-B", "-c", check, shim).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	})
	for _, role := range []string{"the-desk", "intake-desk", "worker-desk", "pr-review-desk", "verify-desk"} {
		t.Run(role, func(t *testing.T) {
			work := filepath.Join(t.TempDir(), "cold work é")
			if err := os.MkdirAll(filepath.Join(work, "repository"), 0o700); err != nil {
				t.Fatal(err)
			}
			agent := filepath.Join(work, "agent")
			marker := filepath.Join(work, "agent-started")
			if err := os.WriteFile(agent, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(work, "package.json")
			if err := os.WriteFile(manifest, []byte(`{"name":"@anthropic-ai/claude-code","version":"2.1.228"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			instructions := filepath.Join(work, "role.md")
			if err := os.WriteFile(instructions, []byte("synthetic role"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := append(prepArgs(role), "--work", work, "--agent", agent, "--version-manifest", manifest, "--instructions", instructions)
			env := prepEnvironment(t, false, 999)
			before := snapshotTree(t, work)
			plan := prepJSON(t, execIn(t, bin, env, args...))
			if plan["mode"] != "plan" || snapshotTree(t, work) != before {
				t.Fatal("default plan wrote to volume", plan)
			}
			held := execIn(t, bin, prepEnvironment(t, false, 0), args...)
			if held.Code != 78 || !strings.Contains(held.Stderr, "UID/GID 999") || snapshotTree(t, work) != before {
				t.Fatalf("preparer identity bypassed: %+v", held)
			}
			made := prepJSON(t, execIn(t, bin, env, append(append([]string{}, args...), "--prepare")...))
			raw, err := os.ReadFile(filepath.Join(work, ".assay-compat/state.json"))
			if err != nil {
				t.Fatal(err)
			}
			var state map[string]any
			if err := json.Unmarshal(raw, &state); err != nil {
				t.Fatal(err)
			}
			if state["state"] != "prepared" || state["launch_id"] != made["launch_id"] {
				t.Fatal("handoff not bound to real state", state, made)
			}
			saved := snapshotTree(t, work)
			readback := prepJSON(t, execIn(t, bin, env, append(append([]string{}, args...), "--readback")...))
			if !reflect.DeepEqual(made, readback) || snapshotTree(t, work) != saved {
				t.Fatal("readback changed prepared state")
			}
			again := execIn(t, bin, env, append(append([]string{}, args...), "--prepare")...)
			if again.Code != 78 || snapshotTree(t, work) != saved {
				t.Fatalf("repeated preparation altered history: %+v", again)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("agent invoked", err)
			}
		})
	}
}
