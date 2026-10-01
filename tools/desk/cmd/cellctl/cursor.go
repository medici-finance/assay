package main

import (
	"fmt"
	"path/filepath"
	"strings"
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

// The repository placement produced by deskinstall --harness cursor is the
// supported discovery path. Test the enabled roles, not merely any skill folder.
func (c *Cell) cursorSkillsDiscoverable() bool {
	for _, role := range c.Roles {
		if !isRegular(filepath.Join(c.Repo, ".cursor", "skills", role, "SKILL.md")) {
			return false
		}
	}
	return len(c.Roles) > 0
}
