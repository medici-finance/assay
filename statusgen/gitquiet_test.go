package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #1608
func TestFixtureGitQuiet(t *testing.T) {
	for _, site := range []string{"transition", "history", "child"} {
		t.Run(site, func(t *testing.T) {
			var root string
			switch site {
			case "transition":
				root = contextFixture(t, nil, nil).root
			case "history":
				root = t.TempDir()
				initRepoAt(t, root, fixedDate)
			case "child":
				root = t.TempDir()
				cmd := exec.Command("git", "init", "-q", root)
				cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("child init: %v: %s", err, out)
				}
			}
			for key, want := range map[string]string{"maintenance.auto": "false", "gc.auto": "0"} {
				out, err := exec.Command("git", "-C", root, "config", "--local", "--default", "missing", "--get", key).CombinedOutput()
				if err != nil || strings.TrimSpace(string(out)) != want {
					t.Errorf("%s: %s = %q (%v), want %s", site, key, strings.TrimSpace(string(out)), err, want)
				}
			}
			trace := filepath.Join(t.TempDir(), "trace")
			runGitEnv(t, root, []string{"GIT_TRACE=" + trace}, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-q", "-m", "quiet")
			body, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "maintenance run --auto") || strings.Contains(string(body), "gc --auto") {
				t.Errorf("%s: fixture commit forked automatic maintenance", site)
			}
		})
	}
}

// regression: #1608
func TestQuietEnvPreserved(t *testing.T) {
	want := []string{"PATH=bin", "GIT_TEMPLATE_DIR=owned-template", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_DATE=stamp"}
	got := fixtureGitEnv(append(append([]string{}, want...), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=true", "GIT_NO_REPLACE_OBJECTS=1"))
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("fixture env = %q, want %q", got, want)
	}
}
