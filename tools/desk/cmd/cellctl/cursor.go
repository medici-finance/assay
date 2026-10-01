package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Cursor Agent is the agent executable, not the Cursor desktop editor. Keep
// arguments separate and use only flags advertised by its CLI. Session identity
// travels through DESK_SESSION because this CLI has no --name flag.
func cursorLaunchArgv(role, model, _ string, wt string) []string {
	return []string{"agent", "--workspace", wt, "--model", model,
		fmt.Sprintf("Invoke the %q skill now.", "assay:"+role)}
}

// Cursor authenticates through its own configuration. Preserve the composed
// role, roots and shim path, without passing Claude's endpoint/model overrides.
func cursorLaunchEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if key == "CLAUDE_CONFIG_DIR" || strings.HasPrefix(key, "ANTHROPIC_") || strings.HasPrefix(key, "CLAUDE_CODE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// Cursor has no mapping in the Claude/Codex model-policy schema. Refuse rather
// than falling through to a different harness or borrowing a Claude endpoint.
func cursorConfigurationError(kind, provider string, hasPolicy bool) error {
	if kind != "house" {
		return fmt.Errorf("cursor currently requires a house cell; %s is unsupported", kind)
	}
	if hasPolicy {
		return fmt.Errorf("cursor does not support CELL_MODEL_POLICY; use an explicit Cursor model or CURSOR_MODEL_<role>/CURSOR_MODEL_default")
	}
	if provider != "" {
		return fmt.Errorf("cursor uses its own authentication; CELL_PROVIDER and --provider are unsupported")
	}
	return nil
}

// Source-checkout placement is not inherited by a git worktree when the files
// are uncommitted. Discovery must describe the workspace the role will execute in.
func (c *Cell) cursorSkillsDiscoverable() bool {
	for _, role := range c.Roles {
		if cursorWorkspaceError(filepath.Join(c.Dir, "worktrees", role), role) != nil {
			return false
		}
	}
	return len(c.Roles) > 0
}

// prepareCursorWorkspace verifies the installer placement in the actual role
// workspace. It never copies instructions or weakens/replaces user configuration.
func (c *Cell) prepareCursorWorkspace(role, wt string) error {
	if err := cursorWorkspaceError(wt, role); err != nil {
		forge := c.Forge
		if forge == "" {
			forge = "github"
		}
		quoted := bashQuote(wt)
		if runtime.GOOS == "windows" {
			quoted = "'" + strings.ReplaceAll(wt, "'", "''") + "'"
		}
		return fmt.Errorf("%w; install into the actual role workspace: deskinstall --harness cursor --forge %s --repo %s", err, forge, quoted)
	}
	if c.Cadence != nil {
		// Project permissions are workspace-relative too. Do not enable unattended
		// writes when local source-checkout denials disappeared in the worktree.
		source := filepath.Join(c.Repo, ".cursor", "cli.json")
		expected, err := os.ReadFile(source)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot read source Cursor project permissions: %w", err)
		}
		if err == nil {
			actual, readErr := os.ReadFile(filepath.Join(wt, ".cursor", "cli.json"))
			if readErr != nil || !bytes.Equal(expected, actual) {
				return fmt.Errorf("Cursor cadence requires the source project permissions from %s in %s unchanged; place or commit that config into the actual role workspace before retrying", source, filepath.Join(wt, ".cursor", "cli.json"))
			}
		}
	}
	return nil
}

func cursorWorkspaceError(wt, role string) error {
	if !valueIn(role, knownRoles) {
		return fmt.Errorf("unknown Cursor role %q", role)
	}
	for _, rel := range []string{
		filepath.Join(".cursor", "skills", role, "SKILL.md"),
		filepath.Join(".cursor", "references", "cursor.md"),
		filepath.Join(".cursor", "references", "desk-shell.md"),
		filepath.Join(".cursor", "references", "tick-contract.md"),
	} {
		path := filepath.Join(wt, rel)
		if !isRegular(path) {
			return fmt.Errorf("Cursor workspace is missing required file %s", path)
		}
		if data, err := os.ReadFile(path); err != nil || len(bytes.TrimSpace(data)) == 0 {
			return fmt.Errorf("Cursor workspace required file is unreadable or empty: %s", path)
		}
	}
	agents, err := os.ReadFile(filepath.Join(wt, "AGENTS.md"))
	const begin, end = "<!-- assay:bindings:begin -->", "<!-- assay:bindings:end -->"
	text := string(agents)
	b, e := strings.Index(text, begin), strings.Index(text, end)
	if err != nil || strings.Count(text, begin) != 1 || strings.Count(text, end) != 1 || e <= b+len(begin) || strings.TrimSpace(text[b+len(begin):e]) == "" {
		return fmt.Errorf("Cursor workspace lacks complete Assay bindings in %s", filepath.Join(wt, "AGENTS.md"))
	}
	return nil
}

// Cursor's supported unattended route is print+force; force still honors explicit
// denials. Refuse older/unknown executables whose help does not state that rule.
// No --trust, sandbox override, config rewrite, or automatic MCP grant is added.
func cursorHeadlessPreflight(executable string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	help, err := exec.CommandContext(ctx, executable, "--help").Output()
	if err != nil {
		return fmt.Errorf("cannot verify Cursor unattended permission support: %w", err)
	}
	return cursorHeadlessHelpError(string(help))
}

func cursorHeadlessHelpError(help string) error {
	normalized := strings.Join(strings.Fields(help), " ")
	if !strings.Contains(normalized, "--print") || !strings.Contains(normalized, "--output-format") || !strings.Contains(normalized, "--force Force allow commands unless explicitly denied") {
		return fmt.Errorf("Cursor cadence requires agent --help to advertise print mode and --force honoring explicit denials; this installed agent is unsupported")
	}
	return nil
}
