package main

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// envClaudeCodePromptSuggestion is the Claude-only knob that turns off Claude Code's next-prompt
// suggestions. It is composed as a LAUNCH-TIME DEFAULT (assay#1436) on every claude-arm launch —
// scrubbed, house/k8s, and the shell-parity oracle alike — so it covers a scrubbed desk/smoke
// session and a scheduler the same way an interactive `export` never does. Codex has no
// equivalent surface, so this key is never composed on the codex arm; see harnessArgv/deskLaunch.
const envClaudeCodePromptSuggestion = "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"

// scrubbedEnvKeys is the allowlist a scrubbed cell's launch is COMPOSED from: `env -i` plus
// exactly these, in this order — the parent shell contributes nothing by default. This list is
// the SINGLE source the [plan] lines, the live `env -i` launch and docs/cellctl.md's env table
// all read — the scrubbed-cell brief states it once, and the port's own brief diffs the two
// implementations against each other.
//
// The three harness-conditional entries (CODEX_HOME / CLAUDE_CONFIG_DIR / the prompt-suggestion
// default) are gated the same way: CODEX_HOME and CLAUDE_CONFIG_DIR are mutually exclusive on the
// ACTIVE harness, and the prompt-suggestion default is claude-only — codex never carries it. On
// top of that, DESK_ROOTS/TERM/LANG are present only when the source they come from (cell.env
// CELL_ROOTS; the launching shell) carries one.
var scrubbedEnvKeys = []string{
	"HOME", "ZDOTDIR", "SHELL", "PATH", "TMPDIR", "KUBECONFIG", "ASSAY_CONFIG_HOME",
	"GH_CONFIG_DIR", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM", "GIT_TERMINAL_PROMPT",
	"CODEX_HOME", "CLAUDE_CONFIG_DIR", envClaudeCodePromptSuggestion,
	"DESK_LOOP", "DESK_SESSION", "DESK_ROOTS", "TERM", "LANG",
}

// scrubbedHarnessPath resolves the harness binary ONCE, on the PARENT shell's PATH — the one
// parent-derived element the composed PATH keeps, named and pinned, never a bare inherited PATH.
// Nothing downstream can find it once the launch switches to `env -i`, so an unfindable harness
// is a refusal here.
func scrubbedHarnessPath(harness string) string {
	p, err := exec.LookPath(harness)
	if err != nil {
		die("desk: '%s' not found on PATH — install it before booting a scrubbed cell", harness)
	}
	abs, err := filepath.Abs(filepath.Dir(p))
	if err != nil {
		die("desk: cannot resolve the directory of '%s'", harness)
	}
	return abs
}

// scrubbedEnvValue is the VALUE one allowlist KEY resolves to for this launch, or "" when the
// key does not apply to this run (the harness-conditional pair, or a source-dependent
// pass-through) — the caller SKIPS an empty value rather than exporting `KEY=`.
//
// Switching on scrubbedEnvKeys itself is what keeps the [plan] lines, the live launch and this
// function from drifting apart: a KEY added to the list with no case arm here is a silent no-op,
// caught by the parity diff.
func (c *Cell) scrubbedEnvValue(key, role, harness, session, composedPath string) string {
	switch key {
	case "HOME":
		return c.Home
	case "ZDOTDIR":
		return c.Home
	case "SHELL":
		// The oracle emits $BASH — the interpreter bash itself resolved for the script, which
		// under `#!/usr/bin/env bash` is the FIRST bash on PATH. The port has no interpreter of
		// its own, so it resolves the same thing the same way rather than passing the launching
		// shell's $SHELL through (which would be a parent-derived value the scrub exists to
		// keep out).
		return lookBash()
	case "PATH":
		return composedPath
	case "TMPDIR":
		return filepath.Join(c.Dir, "tmp")
	case "KUBECONFIG":
		// The cluster-isolation control. Row 5's mutation drops exactly this line, and row 13
		// re-proves it is still here in a build with no `parity` tag.
		return "/dev/null"
	case "ASSAY_CONFIG_HOME":
		return c.Config
	case "GH_CONFIG_DIR":
		return filepath.Join(c.Home, ghConfigRelPath)
	case "GIT_CONFIG_GLOBAL":
		return filepath.Join(c.Home, ".gitconfig")
	case "GIT_CONFIG_NOSYSTEM":
		return "1"
	case "GIT_TERMINAL_PROMPT":
		return "0"
	case "CODEX_HOME":
		if harness == "codex" {
			return filepath.Join(c.Home, ".codex")
		}
		return ""
	case "CLAUDE_CONFIG_DIR":
		if harness == "claude" {
			return filepath.Join(c.Home, ".claude")
		}
		return ""
	case envClaudeCodePromptSuggestion:
		// Claude-only default (assay#1436): disable next-prompt suggestions for a scrubbed
		// desk/smoke session the same way the non-scrubbed launch does — codex has no
		// equivalent knob, so this key is absent on that arm.
		if harness == "claude" {
			return "false"
		}
		return ""
	case "DESK_LOOP":
		return role
	case "DESK_SESSION":
		return session
	case "DESK_ROOTS":
		return c.Env.Get("CELL_ROOTS")
	case "TERM":
		return c.Env.Get("TERM")
	case "LANG":
		return c.Env.Get("LANG")
	}
	return ""
}

// ComposedEnv is what `env -i` is handed (Pairs, order irrelevant there) and what the [plan]
// lines print (Sorted, KEY-sorted).
type ComposedEnv struct {
	Pairs  []string
	Sorted []string
}

// scrubbedComposeEnv walks scrubbedEnvKeys to build the composed-environment allowlist.
// CELL_PATH, when cell.env sets it, overrides only the TRAILING system part of the composed
// PATH — the shim prefix and the resolved harness dir are never overridden.
func (c *Cell) scrubbedComposeEnv(role, harness, session string) ComposedEnv {
	harnessDir := scrubbedHarnessPath(harness)
	tail := "/usr/bin:/bin:/usr/sbin:/sbin"
	if v := c.Env.Get("CELL_PATH"); v != "" {
		tail = v
	}
	composedPath := strings.Join([]string{
		filepath.Join(c.Dir, "shim"), deskToolsBin(c.Env), harnessDir, tail,
	}, ":")
	var ce ComposedEnv
	for _, k := range scrubbedEnvKeys {
		v := c.scrubbedEnvValue(k, role, harness, session, composedPath)
		if v == "" {
			continue
		}
		ce.Pairs = append(ce.Pairs, k+"="+v)
	}
	ce.Sorted = append([]string(nil), ce.Pairs...)
	sort.Strings(ce.Sorted)
	return ce
}

// lookBash is the composed SHELL value: the first `bash` on PATH, which is exactly what bash
// puts in $BASH when the oracle is started through `#!/usr/bin/env bash`.
func lookBash() string {
	if p, err := exec.LookPath("bash"); err == nil {
		if abs, aerr := filepath.Abs(p); aerr == nil {
			return abs
		}
		return p
	}
	return "/bin/bash"
}
