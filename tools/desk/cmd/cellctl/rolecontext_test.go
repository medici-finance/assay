package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Tests for the per-role starting context (rolecontext.go, #2438). The in-process half pins the
// declaration's validation and the argv rewrite; the binary half runs the BUILT cellctl against
// the same filesystem-only fixture the model-policy tests use (stub harness, local git origin,
// no model endpoint, no credential).

const exampleRoleContextPath = "../../../cellctl/examples/role-context.json"

// roleContextCell is a temp cell directory holding everything a full declaration names.
func roleContextCell(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"memory/pr-review-desk", "context"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("memory/pr-review-desk/MEMORY.md", "- review notes\n")
	write("context/review.md", "Review one change at a time.\n")
	write("context/fixer.json", `{"name":"fixer","description":"Applies a named fix.","prompt":"Apply the fix you were given.","capabilities":["file-read","shell"]}`)
	return dir
}

func writeRoleContext(t *testing.T, cellDir string, roles map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"version": 1, "roles": roles})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(cellDir, "role-context.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// fullRole exercises every key of the schema at once.
func fullRole(cellDir string) map[string]any {
	return map[string]any{
		"plugins_off":      []string{"extras@example-market"},
		"skills_off":       []string{"weekly-report"},
		"memory_dir":       "memory/pr-review-desk",
		"instructions":     "context/review.md",
		"instructions_off": []string{"**/NOTES.md"},
		"agents":           []string{"builtin:lean-reviewer", "context/fixer.json"},
		"dispatch_agent":   "lean-reviewer",
		"agents_off":       []string{"general-purpose"},
		"tools_off":        []string{"WebFetch", "NotebookEdit"},
		"connectors_off":   true,
	}
}

func loadFull(t *testing.T) (*roleContext, string) {
	t.Helper()
	dir := roleContextCell(t)
	d, err := loadRoleContext(writeRoleContext(t, dir, map[string]any{"pr-review-desk": fullRole(dir)}), dir)
	if err != nil {
		t.Fatal(err)
	}
	return d.Roles["pr-review-desk"], dir
}

// flagValue is the element after flag in argv, "" when the flag is absent.
func flagValue(argv []string, flag string) string {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag {
			return argv[i+1]
		}
	}
	return ""
}

// TestRoleContextUndeclaredLeavesArgvUntouched is the "defaults change nothing" contract at its
// narrowest: no context, and an entry that declares nothing, return the very same argv.
func TestRoleContextUndeclaredLeavesArgvUntouched(t *testing.T) {
	argv := []string{"claude", "--name", "s", "--model", "m", "/assay:the-desk"}
	for name, rc := range map[string]*roleContext{"nil": nil, "empty entry": {Role: "the-desk"}} {
		out, err := applyClaudeRoleContext(argv, rc)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(out, argv) || &out[0] != &argv[0] {
			t.Errorf("%s: argv changed: %q", name, out)
		}
	}
}

func TestRoleContextApply(t *testing.T) {
	rc, dir := loadFull(t)
	argv := []string{"claude", "--name", "s", "--model", "m", "/assay:pr-review-desk"}
	out, err := applyClaudeRoleContext(argv, rc)
	if err != nil {
		t.Fatal(err)
	}
	if out[0] != "claude" || out[len(out)-1] != "/assay:pr-review-desk" {
		t.Fatalf("executable or prompt moved: %q", out)
	}
	if !contains(out, "--strict-mcp-config") {
		t.Errorf("connectors_off did not add --strict-mcp-config: %q", out)
	}
	wantSettings := `{"autoMemoryDirectory":"` + filepath.Join(dir, "memory", "pr-review-desk") + `",` +
		`"claudeMdExcludes":["**/NOTES.md"],"disableClaudeAiConnectors":true,` +
		`"enabledPlugins":{"extras@example-market":false},` +
		`"permissions":{"deny":["WebFetch","NotebookEdit","Agent(general-purpose)"]},` +
		`"skillOverrides":{"weekly-report":"off"}}`
	if got := flagValue(out, "--settings"); got != wantSettings {
		t.Errorf("--settings =\n %s\nwant\n %s", got, wantSettings)
	}
	var agents map[string]map[string]any
	if err := json.Unmarshal([]byte(flagValue(out, "--agents")), &agents); err != nil {
		t.Fatalf("--agents is not JSON: %v", err)
	}
	lean, fixer := agents["lean-reviewer"], agents["fixer"]
	if got := lean["tools"]; !reflect.DeepEqual(got, []any{"Bash", "Read", "Write", "Edit"}) {
		t.Errorf("lean-reviewer tools = %v", got)
	}
	if lean["omitClaudeMd"] != true {
		t.Errorf("lean-reviewer should not inherit instruction files: %v", lean)
	}
	if got := fixer["tools"]; !reflect.DeepEqual(got, []any{"Bash", "Read"}) {
		t.Errorf("fixer tools = %v (capabilities bind in a fixed order)", got)
	}
	if _, ok := fixer["omitClaudeMd"]; ok {
		t.Errorf("an agent that says nothing inherits instructions: %v", fixer)
	}
	for name, def := range agents {
		for k := range def {
			if !contains([]string{"description", "prompt", "tools", "omitClaudeMd"}, k) {
				t.Errorf("agent %s carries %q — a definition may never set a model, permission mode, hook, connector or skill", name, k)
			}
		}
	}
	prompt := flagValue(out, "--append-system-prompt")
	if !strings.HasPrefix(prompt, "Review one change at a time.") || !strings.Contains(prompt, `"lean-reviewer" agent type`) {
		t.Errorf("appended prompt = %q", prompt)
	}
}

// TestRoleContextKeepsLauncherSettings: composed with a --settings the launcher already built
// (the model policy's), every existing key keeps its exact bytes and the context only adds.
func TestRoleContextKeepsLauncherSettings(t *testing.T) {
	rc, _ := loadFull(t)
	policy := `{"availableModels":["m<1>&[1m]"],"env":{"ANTHROPIC_MODEL":"m<1>&[1m]"},"hooks":{"PreToolUse":[{"hooks":[{"command":"'/x y/cellctl' model-policy hook","timeout":10,"type":"command"}],"matcher":"Agent|Task"}]}}`
	argv := []string{"claude", "--effort", "high", "--settings", policy, "--name", "s", "--model", "m", "/assay:pr-review-desk"}
	out, err := applyClaudeRoleContext(argv, rc)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.Join(out, "\x00"), "--settings"); n != 1 {
		t.Fatalf("want exactly one --settings, got %d: %q", n, out)
	}
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal([]byte(policy), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(flagValue(out, "--settings")), &after); err != nil {
		t.Fatal(err)
	}
	for k, v := range before {
		if string(after[k]) != string(v) {
			t.Errorf("launcher settings key %s changed:\n %s\n->\n %s", k, v, after[k])
		}
	}
	if len(after) != len(before)+6 {
		t.Errorf("want the 3 launcher keys plus 6 context keys, got %d", len(after))
	}
	if flagValue(out, "--effort") != "high" || flagValue(out, "--model") != "m" || flagValue(out, "--name") != "s" {
		t.Errorf("launcher flags changed: %q", out)
	}

	// A key the launcher already owns is never overridden — not merged, refused.
	clash := []string{"claude", "--settings", `{"permissions":{"allow":["Bash"]}}`, "/assay:pr-review-desk"}
	if _, err := applyClaudeRoleContext(clash, rc); err == nil || !strings.Contains(err.Error(), `"permissions" is already set by the launcher`) {
		t.Errorf("want a refusal on a colliding settings key, got %v", err)
	}
}

// TestRoleContextSurvivesTickLaunch: the cadence launcher inserts print-mode flags after argv[0]
// and rewrites the final prompt. No flag the context adds may be left without its value or be
// able to swallow the prompt.
func TestRoleContextSurvivesTickLaunch(t *testing.T) {
	rc, _ := loadFull(t)
	out, err := applyClaudeRoleContext([]string{"claude", "--name", "s", "--model", "m", "/assay:pr-review-desk"}, rc)
	if err != nil {
		t.Fatal(err)
	}
	tick, _, err := prepareTickLaunch("claude", out, nil, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tick[:4], []string{"claude", "--print", "--output-format", "text"}) {
		t.Errorf("tick prefix = %q", tick[:4])
	}
	if !strings.HasPrefix(tick[len(tick)-1], "/assay:pr-review-desk\n") {
		t.Errorf("tick prompt = %q", tick[len(tick)-1])
	}
	valueFlags := map[string]bool{"--output-format": true, "--agents": true, "--append-system-prompt": true, "--settings": true, "--name": true, "--model": true}
	for i := 1; i < len(tick)-1; i++ {
		switch {
		case valueFlags[tick[i]]:
			i++
			if i >= len(tick)-1 {
				t.Fatalf("a flag took the prompt as its value: %q", tick)
			}
		case tick[i] == "--print" || tick[i] == "--strict-mcp-config":
		default:
			t.Fatalf("unexpected element %q — every added flag must take zero or one value: %q", tick[i], tick)
		}
	}
}

// TestRoleContextCannotWiden pins the two schemas key by key, so a key added later is a reviewed
// decision, and pins what a fully-populated context can put in --settings: only keys that take
// something away.
func TestRoleContextCannotWiden(t *testing.T) {
	tags := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			if tag := rt.Field(i).Tag.Get("json"); tag != "" {
				out = append(out, tag)
			}
		}
		sort.Strings(out)
		return out
	}
	if got, want := tags(roleContextSpec{}), []string{"agents", "agents_off", "connectors_off", "dispatch_agent", "instructions", "instructions_off", "memory_dir", "memory_off", "plugins_off", "skills_off", "tools_off"}; !reflect.DeepEqual(got, want) {
		t.Errorf("role keys = %v, want %v", got, want)
	}
	if got, want := tags(agentDefinition{}), []string{"capabilities", "description", "inherit_instructions", "name", "prompt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("agent definition keys = %v, want %v", got, want)
	}

	rc, _ := loadFull(t)
	rc.Spec.MemoryOff = true // not reachable through the loader together with memory_dir; set here to see every key
	settings := rc.claudeContextSettings()
	allowed := []string{"autoMemoryDirectory", "autoMemoryEnabled", "claudeMdExcludes", "disableClaudeAiConnectors", "enabledPlugins", "permissions", "skillOverrides"}
	for k := range settings {
		if !contains(allowed, k) {
			t.Errorf("context emits settings key %q", k)
		}
	}
	if len(settings) != len(allowed) {
		t.Errorf("a fully populated context should emit all of %v, got %v", allowed, settings)
	}
	for id, on := range settings["enabledPlugins"].(map[string]bool) {
		if on {
			t.Errorf("plugin %s switched ON by a role context", id)
		}
	}
	perms := settings["permissions"].(map[string]any)
	if len(perms) != 1 || perms["deny"] == nil {
		t.Errorf("permissions may carry deny rules only: %v", perms)
	}
	if settings["autoMemoryEnabled"] != false || settings["disableClaudeAiConnectors"] != true {
		t.Errorf("toggles point the wrong way: %v", settings)
	}
}

func TestRoleContextRefusals(t *testing.T) {
	role := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return map[string]any{"pr-review-desk": m}
	}
	for _, tc := range []struct {
		name, want string
		roles      map[string]any
		files      map[string]string
	}{
		{"unknown role", `unknown role "review-desk"`, map[string]any{"review-desk": map[string]any{}}, nil},
		{"unknown key", `unknown field "plugins_on"`, role("plugins_on", []string{"x@y"}), nil},
		{"widening key", `unknown field "permissions"`, role("permissions", map[string]any{"allow": []string{"Bash"}}), nil},
		{"plugin skill", "plugin's skill", role("skills_off", []string{"assay:the-desk"}), nil},
		{"permission rule as tool", `"Bash(rm *)" is not a valid name`, role("tools_off", []string{"Bash(rm *)"}), nil},
		{"duplicate", "listed twice", role("tools_off", []string{"WebFetch", "WebFetch"}), nil},
		{"memory both", "mutually exclusive", role("memory_dir", "memory/pr-review-desk", "memory_off", true), nil},
		{"memory dir missing", "memory_dir", role("memory_dir", "memory/absent"), nil},
		{"memory dir is a file", "is not a directory", role("memory_dir", "context/review.md"), nil},
		{"instructions missing", "instructions:", role("instructions", "context/absent.md"), nil},
		{"instructions empty", "empty", role("instructions", "context/blank.md"), map[string]string{"context/blank.md": " \n"}},
		{"relative exclude", "absolute path glob", role("instructions_off", []string{"NOTES.md"}), nil},
		{"unknown builtin", "unknown built-in agent definition", role("agents", []string{"builtin:absent"}), nil},
		{"agent file missing", "cannot read agent definition", role("agents", []string{"context/absent.json"}), nil},
		{"agent sets a model", `unknown field "model"`, role("agents", []string{"context/a.json"}),
			map[string]string{"context/a.json": `{"name":"a","description":"d","prompt":"p","capabilities":["shell"],"model":"big"}`}},
		{"agent sets a permission mode", `unknown field "permissionMode"`, role("agents", []string{"context/a.json"}),
			map[string]string{"context/a.json": `{"name":"a","description":"d","prompt":"p","capabilities":["shell"],"permissionMode":"bypassPermissions"}`}},
		{"agent unknown capability", `unknown capability "dispatch"`, role("agents", []string{"context/a.json"}),
			map[string]string{"context/a.json": `{"name":"a","description":"d","prompt":"p","capabilities":["dispatch"]}`}},
		{"agent no capabilities", "capabilities is required", role("agents", []string{"context/a.json"}),
			map[string]string{"context/a.json": `{"name":"a","description":"d","prompt":"p"}`}},
		{"agent name twice", `two definitions are named "lean-reviewer"`, role("agents", []string{"builtin:lean-reviewer", "context/a.json"}),
			map[string]string{"context/a.json": `{"name":"lean-reviewer","description":"d","prompt":"p","capabilities":["shell"]}`}},
		{"dispatch agent not installed", "is not one of this role's agents", role("dispatch_agent", "lean-reviewer"), nil},
		{"dispatch agent denied", "also in agents_off", role("agents", []string{"builtin:lean-reviewer"}, "dispatch_agent", "lean-reviewer", "agents_off", []string{"lean-reviewer"}), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := roleContextCell(t)
			for rel, body := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := loadRoleContext(writeRoleContext(t, dir, tc.roles), dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want a refusal containing %q, got %v", tc.want, err)
			}
		})
	}

	t.Run("file shape", func(t *testing.T) {
		dir := roleContextCell(t)
		p := filepath.Join(dir, "role-context.json")
		for body, want := range map[string]string{
			`{"roles":{}}`:                       `"version" must be 1`,
			`{"version":2,"roles":{}}`:           `"version" must be 1`,
			`{"version":1,"roles":{},"extra":1}`: `unknown field "extra"`,
			`{"version":1,"roles":{}} {}`:        "unexpected data",
			`not json`:                           "invalid character",
		} {
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadRoleContext(p, dir); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s: want %q, got %v", body, want, err)
			}
		}
		if _, err := loadRoleContext(filepath.Join(dir, "absent.json"), dir); err == nil {
			t.Error("a missing declaration must be refused")
		}
		if _, err := loadRoleContext(dir, dir); err == nil {
			t.Error("a directory is not a declaration")
		}
	})
}

// TestRoleContextOversizedArgumentRefused: instruction text is carried in argv, so the launch
// refuses rather than hand the harness an argument the host would reject.
func TestRoleContextOversizedArgumentRefused(t *testing.T) {
	rc := &roleContext{Role: "the-desk", InstructionsText: strings.Repeat("x", roleContextMaxArg+1)}
	rc.Spec.Instructions = "big.md"
	if _, err := applyClaudeRoleContext([]string{"claude", "/assay:the-desk"}, rc); err == nil || !strings.Contains(err.Error(), "over the") {
		t.Fatalf("want an oversize refusal, got %v", err)
	}
}

// TestRoleContextOversizedDeclarationRefusedAtLoad: the same limit is hit when the declaration is
// LOADED (so `check` reports it and `desk` stops before a worktree exists), here by two agent
// definitions that each fit the per-file bound and together do not fit one argument.
func TestRoleContextOversizedDeclarationRefusedAtLoad(t *testing.T) {
	dir := roleContextCell(t)
	for _, name := range []string{"one", "two"} {
		raw, err := json.Marshal(map[string]any{
			"name": name, "description": "d", "prompt": strings.Repeat("x", roleContextMaxFile-256),
			"capabilities": []string{"shell"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "context", name+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := writeRoleContext(t, dir, map[string]any{"pr-review-desk": map[string]any{"agents": []string{"context/one.json", "context/two.json"}}})
	if _, err := loadRoleContext(p, dir); err == nil || !strings.Contains(err.Error(), "over the") || !strings.Contains(err.Error(), "role pr-review-desk") {
		t.Fatalf("want an oversize refusal naming the role, got %v", err)
	}
}

// TestRoleContextShippedExample: the example a cell copies validates as shipped, and the lean
// reviewer it installs is exactly shell + file read/write with no inherited instruction files.
func TestRoleContextShippedExample(t *testing.T) {
	raw, err := os.ReadFile(exampleRoleContextPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := roleContextCell(t)
	p := filepath.Join(dir, "role-context.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := loadRoleContext(p, dir)
	if err != nil {
		t.Fatalf("the shipped example does not validate: %v", err)
	}
	rc := d.Roles["pr-review-desk"]
	if rc == nil || len(rc.Agents) != 1 || rc.Agents[0].Name != "lean-reviewer" || rc.Spec.DispatchAgent != "lean-reviewer" {
		t.Fatalf("example should install and dispatch the lean reviewer: %+v", rc)
	}
	a := rc.Agents[0]
	if !reflect.DeepEqual(a.Capabilities, []string{"shell", "file-read", "file-write"}) || a.inheritsInstructions() {
		t.Errorf("lean reviewer = %+v", a)
	}
	if got := builtinAgentNames(); !reflect.DeepEqual(got, []string{"builtin:lean-reviewer"}) {
		t.Errorf("shipped definitions = %v", got)
	}
}

// ── built binary ────────────────────────────────────────────────────────────────────────────

func (f *policyFixture) declareRoleContext(t *testing.T, roles map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(f.cellDir, "memory", "pr-review-desk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.cellDir, "memory", "pr-review-desk", "MEMORY.md"), []byte("- review notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRoleContext(t, f.cellDir, roles)
	envPath := filepath.Join(f.cellDir, "cell.env")
	raw, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "CELL_ROLE_CONTEXT=") {
		if err := os.WriteFile(envPath, append(raw, []byte("CELL_ROLE_CONTEXT=role-context.json\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func reviewRole() map[string]any {
	return map[string]any{
		"memory_dir":     "memory/pr-review-desk",
		"agents":         []string{"builtin:lean-reviewer"},
		"dispatch_agent": "lean-reviewer",
		"agents_off":     []string{"general-purpose"},
		"connectors_off": true,
	}
}

// TestBinaryRoleContextLaunch runs REAL launches of the built binary. With a declaration for one
// role: that role's harness argv gains the context, the model policy's settings keys are byte
// for byte what they were, and a role the declaration does not name launches with exactly the
// argv it had before the cell declared anything.
func TestBinaryRoleContextLaunch(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)

	_, reviewBefore := launchPolicyDesk(t, f, "pr-review-desk")
	_, deskBefore := launchPolicyDesk(t, f, "the-desk")

	f.declareRoleContext(t, map[string]any{"pr-review-desk": reviewRole()})
	_, reviewAfter := launchPolicyDesk(t, f, "pr-review-desk")
	_, deskAfter := launchPolicyDesk(t, f, "the-desk")

	// The session name carries a launch timestamp; everything else must be identical.
	unnamed := func(argv []string) []string {
		out := append([]string(nil), argv...)
		for i := 0; i+1 < len(out); i++ {
			if out[i] == "--name" {
				out[i+1] = strings.TrimRight(out[i+1], "0123456789TZ")
			}
		}
		return out
	}
	deskBefore, deskAfter, reviewBefore, reviewAfter = unnamed(deskBefore), unnamed(deskAfter), unnamed(reviewBefore), unnamed(reviewAfter)
	if !reflect.DeepEqual(deskAfter, deskBefore) {
		t.Errorf("a role with no entry changed its launch:\n before %q\n after  %q", deskBefore, deskAfter)
	}
	if reviewAfter[len(reviewAfter)-1] != "/assay:pr-review-desk" {
		t.Errorf("prompt is not the last argument: %q", reviewAfter)
	}
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal([]byte(flagValue(reviewBefore, "--settings")), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(flagValue(reviewAfter, "--settings")), &after); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"availableModels", "env", "hooks"} {
		if len(before[k]) == 0 || string(before[k]) != string(after[k]) {
			t.Errorf("model policy settings key %s changed under a role context:\n %s\n->\n %s", k, before[k], after[k])
		}
	}
	memory, _ := filepath.EvalSymlinks(filepath.Join(f.cellDir, "memory", "pr-review-desk"))
	var gotMemory string
	_ = json.Unmarshal(after["autoMemoryDirectory"], &gotMemory)
	if resolved, _ := filepath.EvalSymlinks(gotMemory); resolved != memory || !filepath.IsAbs(gotMemory) {
		t.Errorf("autoMemoryDirectory = %q, want the absolute role directory %q", gotMemory, memory)
	}
	if string(after["permissions"]) != `{"deny":["Agent(general-purpose)"]}` || string(after["disableClaudeAiConnectors"]) != "true" {
		t.Errorf("context settings = %s / %s", after["permissions"], after["disableClaudeAiConnectors"])
	}
	if !contains(reviewAfter, "--strict-mcp-config") || !strings.Contains(flagValue(reviewAfter, "--agents"), `"lean-reviewer"`) ||
		!strings.Contains(flagValue(reviewAfter, "--append-system-prompt"), `"lean-reviewer" agent type`) {
		t.Errorf("context flags missing: %q", reviewAfter)
	}
	for _, flag := range []string{"--effort", "--name", "--model"} {
		if flagValue(reviewAfter, flag) != flagValue(reviewBefore, flag) || flagValue(reviewAfter, flag) == "" {
			t.Errorf("launcher flag %s changed: %q -> %q", flag, flagValue(reviewBefore, flag), flagValue(reviewAfter, flag))
		}
	}
}

// TestBinaryRoleContextSilentWhenUndeclared: a cell that declares nothing prints nothing about
// role context anywhere — the property that keeps the Go binary and the oracle byte-identical.
func TestBinaryRoleContextSilentWhenUndeclared(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	for _, n := range []string{"tmux", "codex"} {
		if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []runResult{f.dryRunDesk(t, "pr-review-desk"), f.run(t, nil, "check", "example")} {
		if out := r.stdout + r.stderr; strings.Contains(out, "context") {
			t.Errorf("an undeclared cell mentions role context:\n%s", out)
		}
	}
}

func TestBinaryRoleContextDryRunAndCheck(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	for _, n := range []string{"tmux", "codex"} {
		if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.declareRoleContext(t, map[string]any{"pr-review-desk": reviewRole()})

	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "[dry-run] context role=pr-review-desk source=") ||
		!strings.Contains(r.stdout, "agents=lean-reviewer dispatch_agent=lean-reviewer agents_off=general-purpose tools_off=none connectors=off") ||
		!strings.Contains(r.stdout, "[dry-run] context flags: --strict-mcp-config --agents=<") ||
		!strings.Contains(r.stdout, "--settings+=autoMemoryDirectory,disableClaudeAiConnectors,permissions") {
		t.Fatalf("dry run does not show the role's context: %+v", r)
	}
	if strings.Contains(r.stdout, "You are a dispatched reviewer") {
		t.Errorf("a dry run prints sizes, not the agent prompt: %s", r.stdout)
	}
	if r := f.dryRunDesk(t, "the-desk"); r.code != 0 || !strings.Contains(r.stdout, "context role=the-desk") || !strings.Contains(r.stdout, "(nothing declared for this role)") {
		t.Errorf("a role with no entry should say so: %+v", r)
	}

	r = f.run(t, nil, "check", "example")
	for _, want := range []string{
		"  ok    role context: source=",
		"  ok    role context: pr-review-desk plugins_off=none skills_off=none memory=",
		"  n/a   role context: the-desk — nothing declared; starts with the cell-wide context",
		"  warn  role context: pr-review-desk agent lean-reviewer does not inherit",
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("check is missing %q:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, "MISS  role context") {
		t.Errorf("a valid declaration should have no MISS row:\n%s", r.stdout)
	}

	// An empty role directory is valid but starts the role with no memory at all — say so.
	if err := os.Remove(filepath.Join(f.cellDir, "memory", "pr-review-desk", "MEMORY.md")); err != nil {
		t.Fatal(err)
	}
	if r := f.run(t, nil, "check", "example"); !strings.Contains(r.stdout, "has no MEMORY.md") {
		t.Errorf("check should warn about an unseeded memory directory:\n%s", r.stdout)
	}
}

// TestBinaryRoleContextRefusals: whatever is wrong with a declaration, `check` names it as a
// MISS and exits non-zero, `desk` refuses before launching anything, and `up` opens no window.
func TestBinaryRoleContextRefusals(t *testing.T) {
	stubs := func(t *testing.T, f *policyFixture) {
		for _, n := range []string{"tmux", "codex"} {
			if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Run("missing memory directory", func(t *testing.T) {
		f := newPolicyFixture(t, "2.1.278")
		stubs(t, f)
		f.prepareLocalLaunch(t)
		recordingClaude(t, f)
		role := reviewRole()
		role["memory_dir"] = "memory/absent"
		f.declareRoleContext(t, map[string]any{"pr-review-desk": role})

		if r := f.run(t, nil, "check", "example"); r.code == 0 || !strings.Contains(r.stdout, "  MISS  role-context: role pr-review-desk: memory_dir ") {
			t.Errorf("check should MISS and fail: %+v", r)
		}
		r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "pr-review-desk")
		if r.code == 0 || !strings.Contains(r.stderr, "role-context: role pr-review-desk: memory_dir") || len(launchArgv(r.stdout)) != 0 {
			t.Errorf("desk should refuse before launching: %+v", r)
		}
		if _, err := os.Stat(filepath.Join(f.cellDir, "worktrees", "pr-review-desk")); err == nil {
			t.Error("the refusal came after the role worktree was created")
		}
		// One broken entry stops every window, including roles the declaration does not name.
		r = f.run(t, []string{"DRY_RUN=1", "PATH=" + f.binDir + ":/usr/bin:/bin"}, "up", "example", "--cockpit", "tmux", "--no-attach")
		if r.code == 0 || !strings.Contains(r.stderr, "role-context: role pr-review-desk: memory_dir") || !strings.Contains(r.stderr, "no role windows launched") || strings.Contains(r.stdout, "[dry-run] ") {
			t.Errorf("up should open no window: %+v", r)
		}
	})
	t.Run("role on a harness with no binding", func(t *testing.T) {
		f := newPolicyFixture(t, "2.1.278")
		stubs(t, f)
		// The example policy routes intake-desk to codex.
		f.declareRoleContext(t, map[string]any{"intake-desk": map[string]any{"tools_off": []string{"WebFetch"}}})
		if r := f.run(t, nil, "check", "example"); r.code == 0 || !strings.Contains(r.stdout, "  MISS  role context: intake-desk runs on codex") {
			t.Errorf("check should MISS: %+v", r)
		}
		if r := f.dryRunDesk(t, "intake-desk"); r.code == 0 || !strings.Contains(r.stderr, "bound for the claude harness only") {
			t.Errorf("desk should refuse: %+v", r)
		}
		// Roles on the bound harness are not held hostage by it.
		if r := f.dryRunDesk(t, "pr-review-desk"); r.code != 0 {
			t.Errorf("a claude role should still plan: %+v", r)
		}
	})
	t.Run("container cell", func(t *testing.T) {
		f := newPolicyFixture(t, "2.1.278")
		// No model policy here: the context's own kind rule is what must refuse.
		env := "CELL=example\nCELL_KIND=house\nCELL_ROOTS=example-org/example-repo=" + f.repoDir + "\nCELL_REPO=" + f.repoDir + "\n"
		if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0o644); err != nil {
			t.Fatal(err)
		}
		f.declareRoleContext(t, map[string]any{"pr-review-desk": reviewRole()})
		launcher := filepath.Join(f.binDir, "fake-launcher")
		if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		r := f.run(t, []string{"DRY_RUN=1", "CELL_CONTAINER_LAUNCHER=" + launcher}, "desk", "example", "pr-review-desk", "--kind", "container")
		if r.code == 0 || !strings.Contains(r.stderr, "CELL_ROLE_CONTEXT currently requires a house or k8s cell") {
			t.Errorf("a container cell must refuse a role context: %+v", r)
		}
	})
}

// TestBinarySetAcceptsRoleContextKey: the key is a known cell.env key, so `cellctl set` takes it
// without --force.
func TestBinarySetAcceptsRoleContextKey(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	r := f.run(t, nil, "set", "example", "CELL_ROLE_CONTEXT=role-context.json")
	if r.code != 0 || !strings.Contains(r.stdout, "CELL_ROLE_CONTEXT") {
		t.Fatalf("set CELL_ROLE_CONTEXT: %+v", r)
	}
}
