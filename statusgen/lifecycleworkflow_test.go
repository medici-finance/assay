package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func lifecycleWorkflow(t *testing.T) struct {
	On          map[string]any    `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		Steps []struct {
			Name string `yaml:"name"`
			Run  string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
} {
	t.Helper()
	var w struct {
		On          map[string]any    `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	b, err := os.ReadFile(filepath.Join("..", "ci", "staged-workflows", "lifecycle-preflight.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func lifecycleWorkflowStep(t *testing.T, name string) string {
	t.Helper()
	for _, s := range lifecycleWorkflow(t).Jobs["lifecycle-preflight"].Steps {
		if s.Name == name {
			return s.Run
		}
	}
	t.Fatalf("missing lifecycle workflow step %q", name)
	return ""
}

func TestLifecycleWorkflowAlwaysRunsReadOnly(t *testing.T) {
	w := lifecycleWorkflow(t)
	pr, ok := w.On["pull_request"].(map[string]any)
	if !ok {
		t.Fatal("lifecycle check must run on pull_request")
	}
	for _, key := range []string{"paths", "paths-ignore", "branches", "branches-ignore"} {
		if _, ok := pr[key]; ok {
			t.Fatalf("%s filter can hide a delivery", key)
		}
	}
	types, _ := pr["types"].([]any)
	for _, required := range []string{"opened", "synchronize", "reopened", "edited"} {
		found := false
		for _, v := range types {
			if v == required {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s event", required)
		}
	}
	for _, p := range []string{"contents", "pull-requests", "issues"} {
		if w.Permissions[p] != "read" {
			t.Fatalf("%s permission = %q", p, w.Permissions[p])
		}
	}
	for p, mode := range w.Permissions {
		if mode == "write" {
			t.Fatalf("unexpected write permission: %s", p)
		}
	}
	build := lifecycleWorkflowStep(t, "Build the checker from the trusted base")
	for _, want := range []string{"HEAD^1^{commit}", `git archive "$parent" statusgen forgeread perfrec gitquiet`, "lifecycle-base/statusgen"} {
		if !strings.Contains(build, want) {
			t.Fatalf("guard must build its base with sibling modules: missing %s", want)
		}
	}
}

func TestLifecycleWorkflowPropagatesHeldAndUnreadable(t *testing.T) {
	requireTool(t, "bash")
	step := lifecycleWorkflowStep(t, "Validate the proposed lifecycle transition")
	for _, tc := range []struct {
		name, rc, ghFail string
		pass             bool
	}{
		{"admitted", "0", "", true},
		{"held", "1", "", false},
		{"could-not-check", "3", "", false},
		{"unsupported-base", "2", "", false},
		{"unreadable-base", "0", "1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			for name, script := range map[string]string{
				"gh":             "#!/bin/sh\n[ -z \"$GH_FAIL\" ] || exit 1\nprintf 'main\\n'\n",
				"statusgen-base": "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGV_LOG\"\nexit \"$CHECK_RC\"\n",
			} {
				if err := os.WriteFile(filepath.Join(d, name), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			log := filepath.Join(d, "args")
			cmd := exec.Command("bash", "-c", step)
			cmd.Dir = d
			cmd.Env = append(os.Environ(), "PATH="+d+string(os.PathListSeparator)+os.Getenv("PATH"), "RUNNER_TEMP="+d,
				"PR_NUMBER=42", "GITHUB_REPOSITORY=o/r", "EXPECTED_HEAD="+strings.Repeat("a", 40), "BASE_REF=main",
				"CHECK_RC="+tc.rc, "GH_FAIL="+tc.ghFail, "ARGV_LOG="+log)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v err=%v: %s", tc.pass, err, out)
			}
			if tc.ghFail == "" {
				args, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				want := "lifecycle-preflight\n--root\n.\n--repo\no/r\n--pr\n42\n--expected-head\n" + strings.Repeat("a", 40) + "\n--json\n"
				if string(args) != want {
					t.Fatalf("wrong checker contract: %s", args)
				}
			}
		})
	}
}

func TestLifecycleWorkflowRejectsWrongDataHead(t *testing.T) {
	requireTool(t, "bash")
	requireTool(t, "git")
	d := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		cmd.Dir = d
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "main")
	git("commit", "--allow-empty", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "delivery")
	git("commit", "--allow-empty", "-m", "delivery")
	head := git("rev-parse", "HEAD")
	git("checkout", "main")
	git("merge", "--no-ff", "delivery", "-m", "trial merge")
	git("update-ref", "refs/remotes/origin/main", base)
	step := lifecycleWorkflowStep(t, "Bind the data tree to the event head and base")
	for _, tc := range []struct {
		name, head, base string
		pass             bool
	}{
		{"bound", head, base, true},
		{"wrong-head", base, base, false},
		{"unreadable-base", head, strings.Repeat("f", 40), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", step)
			cmd.Dir = d
			cmd.Env = append(os.Environ(), "EXPECTED_HEAD="+tc.head, "EVENT_BASE="+tc.base, "BASE_REF=main")
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v err=%v: %s", tc.pass, err, out)
			}
		})
	}
}
