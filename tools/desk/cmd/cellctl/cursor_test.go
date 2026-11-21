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
	for _, setting := range []string{"", "off"} {
		cell := &Cell{Cadence: resolveDeskCadence("house", "cursor", setting, "")}
		for _, role := range knownRoles {
			got := cell.cursorLaunchArgv(role, model, wt)
			want := []string{"agent", "--workspace", wt, "--model", model}
			if len(got) != len(want)+1 || !reflect.DeepEqual(got[:len(want)], want) {
				t.Fatalf("Cursor argv with cadence %q = %#v, want prefix %#v", setting, got, want)
			}
			prompt := got[len(want)]
			if !strings.Contains(prompt, `Invoke the "assay:`+role+`" skill now.`) ||
				!strings.Contains(prompt, ".cursor/references/cursor.md") ||
				!strings.Contains(prompt, "Interactive desk cadence") {
				t.Fatalf("Cursor launch for %s with cadence %q omitted standing boot: %q", role, setting, prompt)
			}
		}
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
	captureStdout(t, func() { inproc("set", "example", "worker-desk", "--model", "cursor-pin") })
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

// regression: #6646
func TestCursorBootNamesStrongModel(t *testing.T) {
	wt := t.TempDir()
	cell := &Cell{Env: envWith(map[string]string{
		"CURSOR_MODEL_default": "auto", "TIER_MODEL_TOP_CURSOR": "strong-model-x",
	})}
	for _, role := range knownRoles {
		if role == "verify-desk" {
			continue // the routine verify drain never launches a strong model: TestCursorVerifyBootHolds
		}
		got := cell.cursorLaunchArgv(role, "auto", wt)
		prompt := got[len(got)-1]
		for _, want := range []string{`"strong-model-x"`, "not by itself a reason to hold", "unconfirmed, weaker, or an unconfirmed inherit"} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("%s boot prompt lacks %q: %q", role, want, prompt)
			}
		}
	}
}

// regression: #6646
func TestCursorVerifyBootHolds(t *testing.T) {
	wt := t.TempDir()
	for _, env := range []map[string]string{
		{"CURSOR_MODEL_default": "auto", "TIER_MODEL_TOP_CURSOR": "strong-model-x"},
		{"CURSOR_MODEL_default": "auto"},
		{"TIER_MODEL_TOP_CURSOR": "Auto"},
	} {
		got := (&Cell{Env: envWith(env)}).cursorLaunchArgv("verify-desk", "auto", wt)
		p := got[len(got)-1]
		for _, bad := range []string{"set the subagent to the configured", "and launch it", "not by itself a reason to hold"} {
			if strings.Contains(p, bad) {
				t.Fatalf("verify-desk boot prompt carries the general launch text %q (%v): %q", bad, env, p)
			}
		}
		for _, want := range []string{"never launches a stronger configured model", "holds an exec-tier: strong brief", "gate: human"} {
			if !strings.Contains(p, want) {
				t.Fatalf("verify-desk boot prompt lacks %q (%v): %q", want, env, p)
			}
		}
		if env["TIER_MODEL_TOP_CURSOR"] == "strong-model-x" && !strings.Contains(p, `"strong-model-x"`) {
			t.Fatalf("verify-desk boot prompt does not name the configured id for the gate: human re-verify: %q", p)
		}
	}
}

// regression: #6646
func TestCursorBootNeverNamesAutoStrong(t *testing.T) {
	wt := t.TempDir()
	for _, env := range []map[string]string{
		{"CURSOR_MODEL_default": "auto"},
		{"CURSOR_MODEL_default": "auto", "TIER_MODEL_TOP_CURSOR": "auto"},
		{"TIER_MODEL_TOP_CURSOR": "Auto"},
		{"TIER_MODEL_TOP_CURSOR": "  "},
	} {
		prompt := (&Cell{Env: envWith(env)}).cursorLaunchArgv("worker-desk", "auto", wt)
		p := prompt[len(prompt)-1]
		if strings.Contains(p, "set the subagent to the configured") || !strings.Contains(p, "is held unless this session's own model is confirmed strong") {
			t.Fatalf("unconfigured strong tier did not keep the hold (%v): %q", env, p)
		}
	}
	if p := (&Cell{}).cursorLaunchArgv("worker-desk", "m", wt); !strings.Contains(p[len(p)-1], "Strong tier:") {
		t.Fatal("nil Env cell lost the strong-tier note")
	}
	tick := &Cell{Cadence: resolveDeskCadence("house", "cursor", "30m", "")}
	if p := tick.cursorLaunchArgv("worker-desk", "m", wt); p[len(p)-1] != `Invoke the "assay:worker-desk" skill now.` {
		t.Fatalf("headless tick prompt must stay minimal: %q", p[len(p)-1])
	}
}
