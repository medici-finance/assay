package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func init() {
	if os.Getenv("CELLCTL_FEATURE_FIXTURE") != "1" {
		return
	}
	if len(os.Args) != 3 || os.Args[1] != "features" || os.Args[2] != "list" {
		os.Exit(23)
	}
	if expected := os.Getenv("FEATURE_EXPECT_HOME"); expected != "" && os.Getenv("CODEX_HOME") != expected {
		os.Exit(24)
	}
	if expected := os.Getenv("FEATURE_EXPECT_CWD"); expected != "" {
		cwd, _ := os.Getwd()
		real, _ := filepath.EvalSymlinks(expected)
		if cwd != real {
			os.Exit(25)
		}
	}
	if os.Getenv("FEATURE_FAIL") == "1" {
		os.Exit(26)
	}
	_, _ = os.Stdout.WriteString(os.Getenv("FEATURE_OUTPUT"))
	os.Exit(0)
}

func TestCodexFeatureProbeUsesEffectiveCLI(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	name := "codex"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), b, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("CELLCTL_FEATURE_FIXTURE", "1")
	c := &Cell{Repo: t.TempDir(), Home: t.TempDir(), Kind: "scrubbed"}
	t.Setenv("FEATURE_EXPECT_HOME", filepath.Join(c.Home, ".codex"))
	t.Setenv("FEATURE_EXPECT_CWD", c.Repo)
	t.Setenv("FEATURE_OUTPUT", "multi_agent stable true\n")
	if !c.codexMultiAgentOn() {
		t.Fatal("effective default-on rejected")
	}
	t.Setenv("FEATURE_OUTPUT", "multi_agent stable false\n")
	if c.codexMultiAgentOn() {
		t.Fatal("explicit effective false admitted")
	}
	t.Setenv("FEATURE_OUTPUT", "multi_agent stable true\n")
	t.Setenv("FEATURE_FAIL", "1")
	if c.codexMultiAgentOn() {
		t.Fatal("failed CLI probe admitted")
	}
}

func TestCodexCapacityUsesCellPool(t *testing.T) {
	t.Setenv(deskkit.EnvTokenConcurrencyTrip, "")
	c := &Cell{Config: t.TempDir()}
	for _, tc := range []struct{ role, want string }{
		{"worker-desk", "8"}, {"pr-review-desk", "5"}, {"verify-desk", "6"},
		{"intake-desk", "1"}, {"the-desk", "1"},
	} {
		args, err := c.codexCapacityArgs(tc.role)
		want := []string{"-c", "agents.max_concurrent_threads_per_session=" + tc.want}
		if err != nil || !reflect.DeepEqual(args, want) {
			t.Fatalf("%s: %v %v; want %v", tc.role, args, err, want)
		}
	}
	if _, err := c.codexCapacityArgs("unrecognised"); err == nil {
		t.Fatal("unknown role admitted")
	}

	store := filepath.Join(c.Config, "roster", "width")
	if err := os.MkdirAll(store, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store, "worker-desk.json")
	for _, tc := range []struct {
		n    int
		ago  time.Duration
		want string
	}{
		{3, 0, "3"}, {999, 0, "12"}, {3, 2 * time.Hour, "8"},
	} {
		b, _ := json.Marshal(deskkit.WidthEntry{Loop: "worker-desk", Width: tc.n, Updated: time.Now().Add(-tc.ago).UTC().Format(time.RFC3339)})
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		args, err := c.codexCapacityArgs("worker-desk")
		if err != nil || args[1] != "agents.max_concurrent_threads_per_session="+tc.want {
			t.Fatalf("stored width %d age %s: %v %v", tc.n, tc.ago, args, err)
		}
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.codexCapacityArgs("worker-desk"); err == nil {
		t.Fatal("corrupt width became default")
	}
}

func TestCodexEffectiveMultiAgentFeature(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		want         bool
	}{
		{"default-on-stable", "multi_agent stable true\nmulti_agent_mode removed false\n", true},
		{"older-enabled", "multi_agent experimental true\n", true},
		{"explicit-false", "multi_agent stable false\n", false},
		{"absent", "multi_agent_v2 stable true\n", false},
		{"duplicate", "multi_agent stable false\nmulti_agent stable true\n", false},
		{"malformed", "multi_agent stable true unknown\n", false},
		{"unknown-value", "multi_agent stable enabled\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexMultiAgentFeatureEnabled(tc.output); got != tc.want {
				t.Fatalf("got %t", got)
			}
		})
	}
}

func TestCodexCapacityRefreshReplacesAliases(t *testing.T) {
	c := &Cell{Config: t.TempDir()}
	input := []string{"codex", "exec", "-c", "model_reasoning_effort=high", "-c", "agents.max_concurrent_threads_per_session=999", "--config", "agents.max_threads=666", "--config=agents.max_threads=44", "-c=agents.max_concurrent_threads_per_session=55", "-cagents.max_threads=77", "-m", "chosen", "prompt mentions -c agents.max_threads=99"}
	store := filepath.Join(c.Config, "roster", "width")
	if err := os.MkdirAll(store, 0700); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{3, 2} {
		original := append([]string(nil), input...)
		b, _ := json.Marshal(deskkit.WidthEntry{Loop: "worker-desk", Width: width, Updated: time.Now().UTC().Format(time.RFC3339)})
		if err := os.WriteFile(filepath.Join(store, "worker-desk.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := c.refreshCodexCapacity("worker-desk", input)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"codex", "exec", "-c", "model_reasoning_effort=high", "-m", "chosen", "-c", fmt.Sprintf("agents.max_concurrent_threads_per_session=%d", width), input[len(input)-1]}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q want %q", got, want)
		}
		if !reflect.DeepEqual(input, original) {
			t.Fatal("prepared launch was mutated")
		}
		input = got
	}
	if _, err := c.refreshCodexCapacity("worker-desk", nil); err == nil {
		t.Fatal("empty argv admitted")
	}
}
