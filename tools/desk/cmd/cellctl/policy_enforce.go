package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// This file ports the RUNTIME half of the CELL_MODEL_POLICY contract from the shell oracle
// (tools/cellctl/cellctl's model_policy "hook" action, apply_model_policy's --settings blob,
// policy_claude_preflight's settings scan and policy_preflight's up/check loop) — assay#1392.
// policy.go resolves a role at LAUNCH; this file is what keeps a running Claude window, and the
// child agents it dispatches, inside that resolution afterwards:
//
//  1. policyClaudeSettings — the `--settings` JSON a policy launch passes to Claude: the policy
//     env block, an `availableModels` allowlist of exactly the provider's pinned tier IDs, and a
//     PreModelSwitch hook plus a PreToolUse(Agent|Task) hook that call back into this binary.
//  2. cmdModelPolicy — `cellctl model-policy hook …`, the callback. It re-resolves the cell's
//     policy for the launched role and refuses (exit 2, Claude Code's blocking hook status) a
//     switch to an unpinned or denied ID, or a child model that is unmapped, denied, or does not
//     support the parent's effort.
//  3. scanClaudeSettingsConflicts — refuses a user/project/managed settings file whose
//     `availableModels` widens past the policy, or any local `modelOverrides`.
//  4. (*Cell).policyPreflight — the per-role loop `up` and `check` run before any window opens.
//
// The launcher is an operator configuration tool, not a security sandbox against an agent with
// shell access (docs/cellctl-model-policy.md) — but every refusal here fails CLOSED: a hook that
// cannot load the cell, parse its input or resolve the policy blocks, it never allows.

// hookBlockExit is Claude Code's "blocking error" hook exit status: the tool call / model switch
// is refused and stderr is shown to the model. Any other non-zero status is a NON-blocking error
// that lets the action proceed — so every refusal path in the hook must use exactly this code.
const hookBlockExit = 2

// managedSettingsRoots are the administrator-managed Claude Code settings locations the oracle
// scans (managed-settings.json plus managed-settings.d/*.json under each). A package variable so
// in-package tests can point it at a fixture; deliberately NOT configurable from the environment
// or cell.env, which would let a cell opt out of the managed-file scan.
var managedSettingsRoots = []string{"/Library/Application Support/ClaudeCode", "/etc/claude-code"}

// allowedModels is the sorted, de-duplicated set of exact model IDs the resolved provider pins
// across its four tiers — the `availableModels` allowlist a policy launch installs.
func (res *PolicyResolution) allowedModels() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range res.tiers {
		if !seen[t.Model] {
			seen[t.Model] = true
			out = append(out, t.Model)
		}
	}
	sort.Strings(out)
	return out
}

// policyHookCommand is the shell command line Claude Code runs for both hooks. It names the cell
// by its ABSOLUTE directory (so the hook needs no CELLS_ROOT from the window's environment) and
// carries the launch's own resolved provider/harness and explicit request, so the hook resolves
// exactly the route this window was launched on.
func policyHookCommand(self, cellDir string, res *PolicyResolution) string {
	parts := []string{self, "model-policy", "hook", cellDir, res.Role, res.Provider, res.Requested, res.Harness}
	for i, p := range parts {
		parts[i] = bashQuote(p)
	}
	return strings.Join(parts, " ")
}

// policyClaudeSettings is the port of the oracle's `settings` object: the compact JSON passed to
// `claude --settings`. It carries no credential — the env block holds model IDs, the effort and
// (anthropic only) the public base URL; a provider token never enters argv.
func policyClaudeSettings(self, cellDir string, res *PolicyResolution) (string, error) {
	command := policyHookCommand(self, cellDir, res)
	hook := []map[string]any{{"type": "command", "command": command}}
	settings := map[string]any{
		"env":             res.ClaudeEnv,
		"availableModels": res.allowedModels(),
		"hooks": map[string]any{
			"PreModelSwitch": []map[string]any{{"hooks": hook}},
			"PreToolUse":     []map[string]any{{"matcher": "Agent|Task", "hooks": hook}},
		},
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(settings); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// hookFail prints the oracle's refusal shape and exits with the BLOCKING hook status.
func hookFail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "model-policy: "+format+"\n", args...)
	panic(exitCode{hookBlockExit})
}

const modelPolicyUsage = "cellctl model-policy hook <cell-dir> <role> <provider> <requested> <harness>  (stdin: a Claude Code hook event)"

// cmdModelPolicy is `cellctl model-policy hook <cell-dir> <role> <provider> <requested>
// <harness>`, the callback policyClaudeSettings installs. It is not an operator verb.
func cmdModelPolicy(args []string) {
	// Fail closed: a die() anywhere below (the cell failing to load, say) exits 3, which Claude
	// Code treats as a NON-blocking hook error and lets the action through. Every non-zero exit,
	// and any unexpected panic, is rewritten to the blocking status instead.
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if ec, ok := r.(exitCode); ok && ec.code == 0 {
			panic(r)
		} else if !ok {
			fmt.Fprintf(os.Stderr, "model-policy: internal error: %v\n", r)
		}
		panic(exitCode{hookBlockExit})
	}()

	if len(args) != 6 || args[0] != "hook" {
		hookFail("usage: %s", modelPolicyUsage)
	}
	cellDir, role, provider, requested, harness := args[1], args[2], args[3], args[4], args[5]
	if !filepath.IsAbs(cellDir) {
		hookFail("cell directory must be absolute: %s", cellDir)
	}
	// Address the cell by its own directory: its parent IS the cells root it was loaded from at
	// launch, so the shared provider catalog's default path resolves identically here.
	if err := os.Setenv("CELLS_ROOT", filepath.Dir(cellDir)); err != nil {
		hookFail("%v", err)
	}
	c := loadCell(filepath.Base(cellDir))
	policy, _, err := c.cellModelPolicy()
	if err != nil {
		hookFail("%s", strings.TrimPrefix(err.Error(), "model-policy: "))
	}
	if policy == nil {
		hookFail("no model policy is active for cell %s; restart the window to launch without one", c.Name)
	}
	res, err := policy.Resolve(role, provider, requested, harness)
	if err != nil {
		hookFail("%s", strings.TrimPrefix(err.Error(), "model-policy: "))
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		hookFail("cannot read hook event: %v", err)
	}
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil || event == nil {
		hookFail("invalid hook event JSON")
	}
	name, _ := event["hook_event_name"].(string)
	tool, _ := event["tool_name"].(string)
	switch {
	case name == "PreModelSwitch":
		target, _ := event["to_model"].(string)
		if !contains(res.allowedModels(), target) {
			hookFail("model switch outside pinned IDs: %v", event["to_model"])
		}
		if _, _, err := resolveInTiers(res.tiers, target, res.banned); err != nil {
			hookFail("%s", strings.TrimPrefix(err.Error(), "model-policy: "))
		}
	case name == "PreToolUse" && (tool == "Agent" || tool == "Task"):
		input, ok := event["tool_input"].(map[string]any)
		if !ok {
			hookFail("PreToolUse event has no tool_input object")
		}
		value, present := input["model"]
		if !present || value == nil || value == "" {
			return // no explicit child model: the child inherits the parent's pinned model
		}
		model, ok := value.(string)
		if !ok {
			hookFail("child model must be a string, got %v", value)
		}
		if _, _, err := res.ResolveChild(model); err != nil {
			hookFail("%s", strings.TrimPrefix(err.Error(), "model-policy: "))
		}
	default:
		hookFail("unexpected hook event")
	}
}

// resolveProjectDir is Python's non-strict Path.resolve(): absolute, with symlinks resolved on
// the longest prefix that exists (the rest is appended as is), so the parent walk below visits
// the real directories.
func resolveProjectDir(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	rest := ""
	cur := abs
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// claudeSettingsPaths is every settings file whose model allowlist Claude Code would merge into
// a window started with this config dir in this project dir, in the oracle's order: the user
// file, then `.claude/settings.json` and `.claude/settings.local.json` in the project and EVERY
// parent up to `/`, then each managed root's managed-settings.json and managed-settings.d/*.json.
func claudeSettingsPaths(config, project string) []string {
	paths := []string{filepath.Join(config, "settings.json")}
	dir := resolveProjectDir(project)
	for {
		paths = append(paths, filepath.Join(dir, ".claude", "settings.json"), filepath.Join(dir, ".claude", "settings.local.json"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, root := range managedSettingsRoots {
		paths = append(paths, filepath.Join(root, "managed-settings.json"))
		extra, _ := filepath.Glob(filepath.Join(root, "managed-settings.d", "*.json"))
		sort.Strings(extra)
		paths = append(paths, extra...)
	}
	return paths
}

// jsonTruthy is Python truthiness over a decoded JSON value — the oracle refuses any TRUTHY
// modelOverrides (a non-empty object/list/string, true, a non-zero number).
func jsonTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

// absentPath reports whether a stat error means "no file here" (Python's Path.exists() → False)
// rather than a read failure the scan must refuse on.
func absentPath(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, syscall.ELOOP)
}

// scanClaudeSettingsConflicts is the second half of the oracle's policy_claude_preflight. Claude
// Code MERGES `availableModels` arrays across settings sources, so a local file listing a model
// outside the policy would widen the launch's allowlist; that, and any local `modelOverrides`
// (which re-map an ID after the policy chose it), refuse the launch. A file that exists but
// cannot be read or parsed refuses too — the scan never treats could-not-check as clean.
func scanClaudeSettingsConflicts(allowed []string, config, project string) error {
	allowedBase := map[string]bool{}
	for _, m := range allowed {
		allowedBase[policyBase(m)] = true
	}
	for _, path := range claudeSettingsPaths(config, project) {
		if _, err := os.Stat(path); err != nil {
			if absentPath(err) {
				continue
			}
			return policyFail("%s: %v", path, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return policyFail("%s: %v", path, err)
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil || data == nil {
			return policyFail("%s: not a JSON settings object", path)
		}
		if values, ok := data["availableModels"]; ok {
			list, isList := values.([]any)
			bad := !isList || len(list) == 0
			for _, v := range list {
				s, isStr := v.(string)
				if !isStr || !allowedBase[policyBase(s)] {
					bad = true
				}
			}
			if bad {
				return policyFail("%s: availableModels conflicts with cell policy", path)
			}
		}
		if jsonTruthy(data["modelOverrides"]) {
			return policyFail("%s: modelOverrides must be removed or reconciled before using a cell policy", path)
		}
	}
	return nil
}

// claudePolicyPreflight is the oracle's policy_claude_preflight: for a Claude-harness policy
// route, the Claude Code version floor and the settings conflict scan for one project dir. A
// Codex route (or no policy) has nothing to check here.
func claudePolicyPreflight(res *PolicyResolution, config, project string) error {
	if res == nil || res.Harness != "claude" {
		return nil
	}
	if err := checkClaudeMinVersion(); err != nil {
		return err
	}
	return scanClaudeSettingsConflicts(res.allowedModels(), config, project)
}

// policyPreflight is the oracle's policy_preflight for one resolved role: the provider's
// credential variable (native providers carry none), the harness binary on PATH, then the Claude
// preflight against the cell's checkout and — when it already exists — the role's own worktree.
// `up` refuses to open ANY window when one role fails it; `check` reports it per role.
func (c *Cell) policyPreflight(res *PolicyResolution, config string) error {
	if res.Provider != "anthropic" && res.Provider != "codex" {
		if _, err := c.providerCredential(res.Provider); err != nil {
			return err
		}
	}
	if !onPath(res.Harness) {
		return policyFail("policy harness is not on PATH: %s", res.Harness)
	}
	if err := claudePolicyPreflight(res, config, c.Repo); err != nil {
		return err
	}
	wt := filepath.Join(c.Dir, "worktrees", res.Role)
	if _, err := os.Stat(filepath.Join(wt, ".git")); err == nil && res.Harness == "claude" {
		return scanClaudeSettingsConflicts(res.allowedModels(), config, wt)
	}
	return nil
}

// policyConfigDir is the Claude config dir a preflight scans when no positional was given:
// $CLAUDE_CONFIG_DIR, else $HOME/.claude — the oracle's `${5:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}}`.
// Unlike resolveCfg it does not require the directory to exist; a missing user file is simply
// not a conflict.
func policyConfigDir(e *Env, in string) string {
	if in != "" {
		return in
	}
	return e.GetOr("CLAUDE_CONFIG_DIR", filepath.Join(e.Get("HOME"), ".claude"))
}
