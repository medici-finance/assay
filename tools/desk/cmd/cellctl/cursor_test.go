package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCursorModelNamespace(t *testing.T) {
	c := &Cell{Env: envWith(map[string]string{
		"DESK_MODEL_DEFAULT": "claude-pin", "CODEX_MODEL_default": "codex-pin",
		"TIER_MODEL_TOP_CLAUDE": "claude-top", "TIER_MODEL_MID_CODEX": "codex-mid",
	})}
	if got := c.resolveRoleModel("worker-desk", "cursor"); got.OK || !strings.Contains(got.Src, "CURSOR_MODEL_worker_desk") {
		t.Fatalf("Cursor borrowed another harness model: %+v", got)
	}
	c.Env.Put("TIER_MODEL_MID_CURSOR", "cursor-tier")
	if got := c.resolveRoleModel("worker-desk", "cursor"); !got.OK || got.Model != "cursor-tier" {
		t.Fatalf("Cursor tier resolution: %+v", got)
	}
	c.Env.Put("CURSOR_MODEL_default", "cursor-default")
	if got := c.resolveRoleModel("worker-desk", "cursor"); got.Model != "cursor-default" || got.Src != "CURSOR_MODEL_default" {
		t.Fatalf("Cursor default resolution: %+v", got)
	}
	c.Env.Put("CURSOR_MODEL_worker_desk", "cursor-role")
	if got := c.resolveRoleModel("worker-desk", "cursor"); got.Model != "cursor-role" || got.Src != "CURSOR_MODEL_worker_desk" {
		t.Fatalf("Cursor role resolution: %+v", got)
	}
}

func TestCursorLaunchPreservesSelection(t *testing.T) {
	wt := filepath.Join(t.TempDir(), "work space & snow雪")
	model := "model[effort=high,context=1m]"
	got := cursorLaunchArgv("worker-desk", model, "session-1", wt)
	want := []string{"agent", "--workspace", wt, "--model", model, `Invoke the "assay:worker-desk" skill now.`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Cursor argv = %#v, want %#v", got, want)
	}
	input := []string{"PATH=/cell/shim:/tools", "DESK_LOOP=worker-desk", "DESK_SESSION=session-1", "DESK_ROOTS=org/repo=/roots/repo", "CURSOR_API_KEY=synthetic-cursor", "ANTHROPIC_AUTH_TOKEN=synthetic-other", "ANTHROPIC_MODEL=wrong-model", "CLAUDE_CONFIG_DIR=/wrong/config", "CLAUDE_CODE_USE_BEDROCK=1"}
	snapshot := append([]string(nil), input...)
	env := cursorLaunchEnv(input)
	if !reflect.DeepEqual(env, input[:5]) || !reflect.DeepEqual(input, snapshot) {
		t.Fatal("Cursor environment lost cell identity/authentication or mutated the caller")
	}
}

func TestCursorRejectsUnsupportedRouting(t *testing.T) {
	if err := cursorConfigurationError("house", "", false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kind, provider string
		policy         bool
	}{
		{"k8s", "", false}, {"container", "", false}, {"scrubbed", "", false},
		{"house", "glm", false}, {"house", "", true},
	} {
		if cursorConfigurationError(tc.kind, tc.provider, tc.policy) == nil {
			t.Fatalf("accepted unsupported Cursor route %+v", tc)
		}
	}
}

func TestCursorSetUsesCursorNamespace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CELLS_ROOT", root)
	dir := filepath.Join(root, "example")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(file, []byte("CELL=example\nCELL_KIND=house\nCELL_HARNESS='cursor'\nDESK_MODEL_worker_desk=claude-pin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() { cmdSet("example", []string{"worker-desk", "--model", "cursor-pin"}) })
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "CURSOR_MODEL_worker_desk=cursor-pin\n") || !strings.Contains(string(data), "DESK_MODEL_worker_desk=claude-pin\n") {
		t.Fatalf("set crossed model namespaces: %s", data)
	}
	for _, key := range []string{"CURSOR_MODEL_default", "CURSOR_MODEL_worker_desk", "TIER_MODEL_TOP_CURSOR", "TIER_MODEL_MID_CURSOR", "TIER_MODEL_FAST_CURSOR"} {
		if !knownCellEnvKey(key) {
			t.Fatalf("missing Cursor key %s", key)
		}
	}
	if knownCellEnvKey("CURSOR_MODEL_woker_desk") {
		t.Fatal("Cursor role typo accepted")
	}
}

func TestCursorSetRefusalIsAtomic(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cell.env")
	before := []byte("CELL=example\nCELL_KIND=k8s\nCELL_HARNESS=claude\n")
	if err := os.WriteFile(file, before, 0o600); err != nil {
		t.Fatal(err)
	}
	assertDies(t, "Cursor on k8s", func() {
		applyEnvKVs(envWith(nil), file, false, []string{"CELL_HARNESS=cursor", "CURSOR_MODEL_default=cursor-pin"})
	})
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("unsupported Cursor switch modified cell.env")
	}
	entries, _ := os.ReadDir(filepath.Dir(file))
	if len(entries) != 1 {
		t.Fatal("unsupported switch created a backup")
	}
}

func TestCursorSkillDiscoveryRequiresEnabledRoles(t *testing.T) {
	c := &Cell{Repo: t.TempDir(), Dir: t.TempDir(), Roles: []string{"worker-desk", "pr-review-desk"}}
	for _, role := range c.Roles {
		if c.cursorSkillsDiscoverable() {
			t.Fatal("missing role skill passed discovery")
		}
		writeCursorWorkspace(t, filepath.Join(c.Dir, "worktrees", role), role)
	}
	if !c.cursorSkillsDiscoverable() {
		t.Fatal("installed role skills were not found")
	}
}
