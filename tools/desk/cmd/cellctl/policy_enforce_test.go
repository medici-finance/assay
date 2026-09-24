package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Tests for the runtime half of the model policy (policy_enforce.go, assay#1392), ported from
// tools/cellctl/tests/model-policy.test.py's test_child_mapping_allowlist_and_hooks,
// test_competing_allowlist_refuses_before_launch, test_up_preflights_all_roles_before_windows and
// test_real_exec_argv_and_env. Every binary test runs the BUILT cellctl against a filesystem-only
// fixture: stub harnesses, a local git origin, no model endpoint and no credential.

// recordingClaude replaces the fixture's claude stub with one that prints each argv element on its
// own ARG= line (the --settings JSON is compact, so it stays one line).
func recordingClaude(t *testing.T, f *policyFixture) {
	t.Helper()
	script := "#!/bin/sh\ncase \"$1\" in\n --version) echo '2.1.278 (stub)'; exit 0;;\n plugin) exit 0;;\nesac\nfor a in \"$@\"; do printf 'ARG=%s\\n' \"$a\"; done\n"
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func launchArgv(stdout string) []string {
	var argv []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "ARG=") {
			argv = append(argv, strings.TrimPrefix(line, "ARG="))
		}
	}
	return argv
}

type launchSettings struct {
	Env             map[string]string `json:"env"`
	AvailableModels []string          `json:"availableModels"`
	Hooks           map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

// launchPolicyDesk runs a REAL (non-dry-run) `cellctl desk` launch and returns the parsed
// --settings the recording harness received.
func launchPolicyDesk(t *testing.T, f *policyFixture, role string, extra ...string) (launchSettings, []string) {
	t.Helper()
	r := f.run(t, []string{"CELLCTL_DESKWT=0"}, append([]string{"desk", "example", role}, extra...)...)
	if r.code != 0 {
		t.Fatalf("launch %s: exit %d\nstdout:\n%s\nstderr:\n%s", role, r.code, r.stdout, r.stderr)
	}
	argv := launchArgv(r.stdout)
	idx := -1
	for i, a := range argv {
		if a == "--settings" {
			idx = i
		}
	}
	if idx < 0 || idx+1 >= len(argv) {
		t.Fatalf("launch %s: no --settings in the harness argv: %q", role, argv)
	}
	var s launchSettings
	if err := json.Unmarshal([]byte(argv[idx+1]), &s); err != nil {
		t.Fatalf("launch %s: --settings is not JSON: %v\n%s", role, err, argv[idx+1])
	}
	return s, argv
}

// runHook runs a hook command line exactly as Claude Code does (through a shell, the event on
// stdin) with a MINIMAL environment — no CELLS_ROOT — proving the command is self-addressing.
func runHook(t *testing.T, f *policyFixture, command string, event any) (int, string) {
	t.Helper()
	raw, ok := event.(string)
	if !ok {
		b, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		raw = string(b)
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"PATH=" + f.binDir + ":/usr/bin:/bin", "HOME=" + f.cellDir, "KUBECONFIG=/dev/null"}
	cmd.Stdin = strings.NewReader(raw)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	err := cmd.Run()
	if err == nil {
		return 0, errb.String()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), errb.String()
	}
	t.Fatalf("running hook: %v", err)
	return -1, ""
}

func agentEvent(model any) map[string]any {
	return map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Agent", "tool_input": map[string]any{"model": model}}
}

// TestBinaryLaunchInstallsAllowlistAndHooks: a policy launch passes Claude a --settings blob
// whose availableModels is EXACTLY the resolved provider's pinned tier IDs (no Opus 5.0) and
// whose PreModelSwitch / PreToolUse(Agent|Task) hooks call back into this binary.
func TestBinaryLaunchInstallsAllowlistAndHooks(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	s, argv := launchPolicyDesk(t, f, "pr-review-desk")

	want := []string{"claude-fable-5-1", "claude-opus-4-8[1m]", "claude-sonnet-5"}
	got := append([]string(nil), s.AvailableModels...)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("availableModels = %q, want exactly the anthropic tier IDs %q", s.AvailableModels, want)
	}
	for _, m := range s.AvailableModels {
		if strings.Contains(strings.ToLower(m), "opus-5") {
			t.Errorf("allowlist carries a denied model: %s", m)
		}
	}
	if s.Env["ANTHROPIC_MODEL"] != "claude-opus-4-8[1m]" || s.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "high" || s.Env["CLAUDE_CODE_SUBAGENT_MODEL"] != "claude-opus-4-8[1m]" {
		t.Errorf("settings env does not carry the resolved model/effort: %v", s.Env)
	}
	if !contains(argv, "--effort") || !contains(argv, "high") {
		t.Errorf("--effort high missing from argv: %q", argv)
	}
	sw, tu := s.Hooks["PreModelSwitch"], s.Hooks["PreToolUse"]
	if len(sw) != 1 || len(sw[0].Hooks) != 1 || sw[0].Hooks[0].Type != "command" {
		t.Fatalf("PreModelSwitch hook missing: %+v", s.Hooks)
	}
	if len(tu) != 1 || tu[0].Matcher != "Agent|Task" || len(tu[0].Hooks) != 1 {
		t.Fatalf("PreToolUse(Agent|Task) hook missing: %+v", s.Hooks)
	}
	if !strings.Contains(sw[0].Hooks[0].Command, " model-policy hook ") || !strings.Contains(sw[0].Hooks[0].Command, f.cellDir) {
		t.Errorf("hook command does not call back into cellctl for this cell: %s", sw[0].Hooks[0].Command)
	}
}

// TestBinaryHooksRefuseDeniedAndUnpinnedModels ports test_child_mapping_allowlist_and_hooks:
// the hook command taken from a real launch's --settings blocks (exit 2) a denied or unmapped
// child model and a switch to a denied or unpinned ID, and allows mapped aliases / inherit.
func TestBinaryHooksRefuseDeniedAndUnpinnedModels(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	s, _ := launchPolicyDesk(t, f, "pr-review-desk")
	command := s.Hooks["PreToolUse"][0].Hooks[0].Command
	cases := []struct {
		name  string
		event any
		code  int
	}{
		{"child alias opus", agentEvent("opus"), 0},
		{"child inherit", agentEvent("inherit"), 0},
		{"child no model", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Task", "tool_input": map[string]any{"prompt": "x"}}, 0},
		{"child exact pinned id", agentEvent("claude-fable-5-1"), 0},
		// claude-sonnet-5 pins BOTH mid (high) and fast (low): an exact-ID request cannot say
		// which effort it means, so it is refused rather than guessed (request a tier instead).
		{"child ambiguous exact id", agentEvent("claude-sonnet-5"), hookBlockExit},
		{"child denied opus-5", agentEvent("claude-opus-5"), hookBlockExit},
		{"child denied Opus5 spelling", agentEvent("Opus5"), hookBlockExit},
		{"child unmapped", agentEvent("unknown-model"), hookBlockExit},
		{"child non-string model", agentEvent(5), hookBlockExit},
		{"switch to pinned id", map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "claude-opus-4-8[1m]"}, 0},
		{"switch to denied", map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "claude-opus-5", "requested_model": "opus"}, hookBlockExit},
		{"switch to floating alias", map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "opus"}, hookBlockExit},
		{"switch to unpinned id", map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "claude-haiku-4"}, hookBlockExit},
		{"unexpected event", map[string]any{"hook_event_name": "PostToolUse"}, hookBlockExit},
		{"unexpected tool", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{}}, hookBlockExit},
		{"garbage stdin", "not json", hookBlockExit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stderr := runHook(t, f, command, tc.event)
			if code != tc.code {
				t.Errorf("exit %d, want %d; stderr: %s", code, tc.code, stderr)
			}
			if tc.code != 0 && !strings.Contains(stderr, "model-policy:") {
				t.Errorf("a refusal must name the model-policy layer: %s", stderr)
			}
		})
	}
}

// TestBinaryHookFailsClosed: every failure path — the cell gone, the policy removed, a bad
// argument count — exits with the BLOCKING status 2, never cellctl's usual 3 (a non-blocking hook
// error Claude Code would let through).
func TestBinaryHookFailsClosed(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	bin := cellctlBinary(t)
	hook := func(args ...string) string {
		quoted := []string{bashQuote(bin), "model-policy"}
		for _, a := range args {
			quoted = append(quoted, bashQuote(a))
		}
		return strings.Join(quoted, " ")
	}
	ev := agentEvent("opus")
	if code, stderr := runHook(t, f, hook("hook", f.cellDir, "pr-review-desk", "anthropic", "", "claude"), ev); code != 0 {
		t.Fatalf("baseline hook should allow: exit %d %s", code, stderr)
	}
	for name, cmd := range map[string]string{
		"wrong arity":  hook("hook", f.cellDir, "pr-review-desk"),
		"not hook":     hook("resolve", f.cellDir, "pr-review-desk", "anthropic", "", "claude"),
		"relative dir": hook("hook", "example", "pr-review-desk", "anthropic", "", "claude"),
		"missing cell": hook("hook", filepath.Join(f.cellsRoot, "no-such-cell"), "pr-review-desk", "anthropic", "", "claude"),
		"unknown role": hook("hook", f.cellDir, "typo-desk", "anthropic", "", "claude"),
	} {
		if code, stderr := runHook(t, f, cmd, ev); code != hookBlockExit {
			t.Errorf("%s: exit %d, want %d (blocking); stderr: %s", name, code, hookBlockExit, stderr)
		}
	}
	// The policy file removed mid-session: the hook refuses rather than allowing unchecked.
	if err := os.Remove(f.policy); err != nil {
		t.Fatal(err)
	}
	if code, stderr := runHook(t, f, hook("hook", f.cellDir, "pr-review-desk", "anthropic", "", "claude"), ev); code != hookBlockExit {
		t.Errorf("missing policy: exit %d, want %d; stderr: %s", code, hookBlockExit, stderr)
	}
}

// TestBinaryHookRefusesChildWithoutInheritedEffort: a child tier whose supported_efforts lacks
// the parent's effort is refused at runtime, not silently run at an effort it cannot honour.
func TestBinaryHookRefusesChildWithoutInheritedEffort(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.rewritePolicy(t, func(m map[string]any) {
		fast := m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["fast"].(map[string]any)
		fast["effort"] = "low"
		fast["supported_efforts"] = []any{"low"}
	})
	cmd := strings.Join([]string{bashQuote(cellctlBinary(t)), "model-policy", "hook", bashQuote(f.cellDir), "pr-review-desk", "anthropic", "''", "claude"}, " ")
	code, stderr := runHook(t, f, cmd, agentEvent("haiku"))
	if code != hookBlockExit || !strings.Contains(stderr, "inherited effort high") {
		t.Fatalf("child without the parent's effort: exit %d, stderr %s", code, stderr)
	}
}

// TestBinaryHookFollowsSharedProviderCatalog: a window launched from the shared providers.json
// catalog (no CELL_MODEL_POLICY file) gets the same allowlist + hooks, and its hook resolves the
// catalog route — the glm worker's allowlist is glm's IDs, and an Anthropic ID is refused there.
func TestBinaryHookFollowsSharedProviderCatalog(t *testing.T) {
	f := catalogFixture(t)
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	s, _ := launchPolicyDesk(t, f, "worker-desk", "--provider", "glm")
	for _, m := range s.AvailableModels {
		if !strings.HasPrefix(m, "glm-") {
			t.Errorf("glm window allowlist carries a non-glm model: %q", s.AvailableModels)
		}
	}
	command := s.Hooks["PreModelSwitch"][0].Hooks[0].Command
	if code, stderr := runHook(t, f, command, map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "glm-5.3[1m]"}); code != 0 {
		t.Errorf("switch to a pinned glm ID refused: %d %s", code, stderr)
	}
	if code, _ := runHook(t, f, command, map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "claude-opus-4-8[1m]"}); code != hookBlockExit {
		t.Errorf("switch to another provider's ID allowed on the glm window: exit %d", code)
	}
}

// TestBinarySettingsConflictScan ports test_competing_allowlist_refuses_before_launch and widens
// it to every source the oracle scans: the user config, the project and its parents, and
// settings.local.json — reported as a refusal naming the file, before any worktree exists.
func TestBinarySettingsConflictScan(t *testing.T) {
	for _, tc := range []struct {
		name, rel string // rel: relative to the fixture root; "cfg" = the Claude config dir
		body      string
		refuse    string // "" = must pass
	}{
		{"user widens", "cfg/settings.json", `{"availableModels":["claude-opus-5"]}`, "availableModels conflicts"},
		{"user unrelated model", "cfg/settings.json", `{"availableModels":["some-other-model"]}`, "availableModels conflicts"},
		{"user empty list", "cfg/settings.json", `{"availableModels":[]}`, "availableModels conflicts"},
		{"user not a list", "cfg/settings.json", `{"availableModels":"claude-sonnet-5"}`, "availableModels conflicts"},
		{"user subset ok", "cfg/settings.json", `{"availableModels":["claude-sonnet-5","CLAUDE-OPUS-4-8"]}`, ""},
		{"user overrides", "cfg/settings.json", `{"modelOverrides":{"opus":"claude-opus-5"}}`, "modelOverrides must be removed"},
		{"user empty overrides ok", "cfg/settings.json", `{"modelOverrides":{}}`, ""},
		{"user unparseable", "cfg/settings.json", `{not json`, "not a JSON settings object"},
		{"project local widens", "repo/.claude/settings.local.json", `{"availableModels":["gpt-5.6-terra"]}`, "availableModels conflicts"},
		{"project parent widens", ".claude/settings.json", `{"availableModels":["claude-opus-5"]}`, "availableModels conflicts"},
		{"unrelated settings ok", "repo/.claude/settings.json", `{"permissions":{"allow":["Bash(ls)"]}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPolicyFixture(t, "2.1.278")
			root := filepath.Dir(f.cellsRoot)
			path := filepath.Join(root, tc.rel)
			if strings.HasPrefix(tc.rel, "cfg/") {
				path = filepath.Join(f.cfgDir, strings.TrimPrefix(tc.rel, "cfg/"))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			r := f.dryRunDesk(t, "pr-review-desk")
			if tc.refuse == "" {
				if r.code != 0 {
					t.Fatalf("must pass: exit %d %s", r.code, r.stderr)
				}
				return
			}
			if r.code == 0 || !strings.Contains(r.stderr, tc.refuse) || !strings.Contains(r.stderr, path) {
				t.Fatalf("want refusal %q naming %s, got exit %d stderr %s", tc.refuse, path, r.code, r.stderr)
			}
			if _, err := os.Stat(filepath.Join(f.cellDir, "worktrees")); err == nil {
				t.Errorf("a refused preflight must not have cut a worktree")
			}
		})
	}
	// A codex route installs no Claude settings, so a Claude-only file never blocks it.
	f := newPolicyFixture(t, "2.1.278")
	if err := os.WriteFile(filepath.Join(f.cfgDir, "settings.json"), []byte(`{"availableModels":["claude-opus-5"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := f.dryRunDesk(t, "intake-desk"); r.code != 0 {
		t.Errorf("codex role blocked by a Claude settings file: %s", r.stderr)
	}
}

// TestBinaryWorktreeRecheckedBeforeLaunch: the settings scan is re-run against the role worktree
// once it exists. A widening file in a parent of the WORKTREE (but not of the cell's checkout)
// passes the pre-worktree scan and must still refuse before the harness starts.
func TestBinaryWorktreeRecheckedBeforeLaunch(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	bad := filepath.Join(f.cellDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(`{"availableModels":["claude-opus-5"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := f.dryRunDesk(t, "pr-review-desk"); r.code != 0 {
		t.Fatalf("dry run (checkout only) should pass: %s", r.stderr)
	}
	r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "pr-review-desk")
	if r.code == 0 || !strings.Contains(r.stderr, "availableModels conflicts") || !strings.Contains(r.stderr, bad) {
		t.Fatalf("worktree recheck missed: exit %d stderr %s", r.code, r.stderr)
	}
	if strings.Contains(r.stdout, "ARG=") {
		t.Errorf("the harness ran despite the refusal: %s", r.stdout)
	}
}

// TestBinaryUpPreflightsEveryRole ports test_up_preflights_all_roles_before_windows: `up` refuses
// before opening ANY window when one role's preflight fails — a missing provider credential, a
// widening settings file, or the role's harness missing from PATH — without printing a token.
func TestBinaryUpPreflightsEveryRole(t *testing.T) {
	stub := func(t *testing.T, f *policyFixture, names ...string) {
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	isolatedPath := func(f *policyFixture) string { return "PATH=" + f.binDir + ":/usr/bin:/bin" }
	up := func(t *testing.T, f *policyFixture, env ...string) runResult {
		return f.run(t, append([]string{"DRY_RUN=1", isolatedPath(f)}, env...), "up", "example", "--cockpit", "tmux", "--no-attach")
	}
	t.Run("baseline", func(t *testing.T) {
		f := newPolicyFixture(t, "2.1.278")
		stub(t, f, "tmux", "codex")
		if r := up(t, f); r.code != 0 || !strings.Contains(r.stdout, "[dry-run] worker-desk:") {
			t.Fatalf("baseline up: %+v", r)
		}
	})
	for _, tc := range []struct {
		name, want string
		setup      func(t *testing.T, f *policyFixture) []string
	}{
		{"missing credential", "ZAI_API_KEY", func(t *testing.T, f *policyFixture) []string {
			stub(t, f, "tmux", "codex")
			return []string{"ZAI_API_KEY="}
		}},
		{"widening settings", "availableModels conflicts", func(t *testing.T, f *policyFixture) []string {
			stub(t, f, "tmux", "codex")
			if err := os.WriteFile(filepath.Join(f.cfgDir, "settings.json"), []byte(`{"availableModels":["claude-opus-5"]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			return nil
		}},
		{"harness not on PATH", "policy harness is not on PATH: codex", func(t *testing.T, f *policyFixture) []string {
			stub(t, f, "tmux")
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPolicyFixture(t, "2.1.278")
			env := tc.setup(t, f)
			r := up(t, f, env...)
			if r.code == 0 || !strings.Contains(r.stderr, "no role windows launched") || !strings.Contains(r.stderr, tc.want) {
				t.Fatalf("want refusal naming %q: %+v", tc.want, r)
			}
			if strings.Contains(r.stdout, "[dry-run] ") {
				t.Errorf("up planned windows despite a failed preflight: %s", r.stdout)
			}
			if strings.Contains(r.stdout+r.stderr, "fixture-kimi") || strings.Contains(r.stdout+r.stderr, "fixture-zai") {
				t.Errorf("a credential value was printed: %s%s", r.stdout, r.stderr)
			}
		})
	}
}

// TestBinaryCheckRunsPolicyPreflight: `check` reports the same per-role preflight as a row —
// ok on a clean cell, MISS naming the conflicting file when a settings file widens the policy.
func TestBinaryCheckRunsPolicyPreflight(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	for _, n := range []string{"tmux", "codex"} {
		if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := f.run(t, nil, "check", "example")
	for _, role := range []string{"the-desk", "pr-review-desk", "worker-desk", "intake-desk"} {
		if !strings.Contains(r.stdout, "  ok    model policy: "+role+"\n") {
			t.Errorf("check has no ok preflight row for %s:\n%s", role, r.stdout)
		}
	}
	bad := filepath.Join(f.cfgDir, "settings.json")
	if err := os.WriteFile(bad, []byte(`{"modelOverrides":{"opus":"x"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r = f.run(t, nil, "check", "example")
	if r.code == 0 || !strings.Contains(r.stdout, "  MISS  model policy: pr-review-desk — model-policy: "+bad+": modelOverrides must be removed") {
		t.Errorf("check did not MISS the conflicting settings file (exit %d):\n%s", r.code, r.stdout)
	}
	// The codex-harness role has no Claude settings to conflict with.
	if !strings.Contains(r.stdout, "  ok    model policy: intake-desk\n") {
		t.Errorf("codex role should stay ok:\n%s", r.stdout)
	}
}

// TestManagedSettingsScanned covers the administrator-managed roots, which the binary tests
// cannot plant without writing system paths: managed-settings.json and managed-settings.d/*.json
// are both scanned, and an unrelated managed file passes.
func TestManagedSettingsScanned(t *testing.T) {
	saved := managedSettingsRoots
	t.Cleanup(func() { managedSettingsRoots = saved })
	root := t.TempDir()
	managedSettingsRoots = []string{filepath.Join(root, "managed")}
	project := filepath.Join(root, "project")
	cfg := filepath.Join(root, "cfg")
	allowed := []string{"claude-opus-4-8[1m]", "claude-sonnet-5"}
	write := func(rel, body string) string {
		p := filepath.Join(root, "managed", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := scanClaudeSettingsConflicts(allowed, cfg, project); err != nil {
		t.Fatalf("no files at all must pass: %v", err)
	}
	write("managed-settings.json", `{"availableModels":["claude-sonnet-5"]}`)
	if err := scanClaudeSettingsConflicts(allowed, cfg, project); err != nil {
		t.Fatalf("a narrower managed allowlist must pass: %v", err)
	}
	p := write("managed-settings.d/20-models.json", `{"availableModels":["claude-opus-5"]}`)
	err := scanClaudeSettingsConflicts(allowed, cfg, project)
	if err == nil || !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "availableModels conflicts") {
		t.Fatalf("managed-settings.d widening not refused: %v", err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	p = write("managed-settings.json", `{"modelOverrides":{"claude-sonnet-5":"x"}}`)
	if err := scanClaudeSettingsConflicts(allowed, cfg, project); err == nil || !strings.Contains(err.Error(), p) {
		t.Fatalf("managed modelOverrides not refused: %v", err)
	}
}
