package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// codexCommandEnvironment is applied by Codex only to command subprocesses.
// The harness itself keeps the operator's HOME (login, plugins and settings).
// Desk tools get the cell HOME directly, so neither PATH order nor executable
// shell wrappers decide which trust roster they load. USERPROFILE is Go's home
// on Windows; HOME alone would silently select the operator's Windows roster.
func (c *Cell) codexCommandEnvironment(env []string) (map[string]string, error) {
	operatorHome := c.codexOperatorHome(runtime.GOOS)
	if !filepath.IsAbs(c.Home) || !filepath.IsAbs(operatorHome) {
		return nil, fmt.Errorf("Codex requires absolute cell and operator home paths")
	}
	values := map[string]string{
		"HOME":              c.Home,
		"USERPROFILE":       c.Home,
		"ZDOTDIR":           c.Home,
		"ASSAY_CONFIG_HOME": c.Config,
		"CODEX_HOME":        c.Env.GetOr("CODEX_HOME", filepath.Join(operatorHome, ".codex")),
		"GH_CONFIG_DIR":     c.codexGHConfigDir(runtime.GOOS, operatorHome),
		"PATH":              filepath.Join(c.Dir, "bin") + string(filepath.ListSeparator) + deskToolsBin(c.Env) + string(filepath.ListSeparator) + c.Env.Get("PATH"),
		// Noninteractive Bash reads BASH_ENV even without login semantics.
		"BASH_ENV": "",
		"ENV":      "",
	}
	// Forward only named non-secret launch context. Values are config overrides
	// on argv, so copying the entire environment here would disclose credentials.
	for _, key := range []string{"DESK_LOOP", "DESK_SESSION", "DESK_ROOTS", "ASSAY_COCKPIT", "ASSAY_REPAIR_ADMISSION"} {
		for _, kv := range env {
			k, value, ok := strings.Cut(kv, "=")
			if ok && strings.EqualFold(k, key) {
				values[key] = value
			}
		}
	}
	return values, nil
}

func (c *Cell) codexOperatorHome(goos string) string {
	if goos == "windows" {
		return c.Env.GetOr("USERPROFILE", c.Env.Get("HOME"))
	}
	return c.Env.GetOr("HOME", c.Env.Get("USERPROFILE"))
}

// Match gh's own documented precedence before command HOME changes. In
// particular, a native Windows login is normally under AppData/GitHub CLI.
func (c *Cell) codexGHConfigDir(goos, home string) string {
	if value := c.Env.Get("GH_CONFIG_DIR"); value != "" {
		return value
	}
	if value := c.Env.Get("XDG_CONFIG_HOME"); value != "" {
		return filepath.Join(value, filepath.Base(ghConfigRelPath))
	}
	if goos == "windows" {
		if value := c.Env.Get("APPDATA"); value != "" {
			return filepath.Join(value, "GitHub CLI")
		}
	}
	return filepath.Join(home, ghConfigRelPath)
}

func (c *Cell) codexEnvironmentArgs(env []string) ([]string, error) {
	values, err := c.codexCommandEnvironment(env)
	if err != nil {
		return nil, err
	}
	// A saved shell snapshot can reintroduce the operator's HOME/PATH after
	// environment composition. Avoid that and profile startup for this launch,
	// without changing the operator's shell or any persistent harness settings.
	args := []string{"-c", "features.shell_snapshot=false", "-c", "allow_login_shell=false", "-c", "shell_environment_policy.experimental_use_profile=false"}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		// JSON strings are TOML basic strings for these paths and environment
		// values, including Windows backslashes. No command shell interprets them.
		value, _ := json.Marshal(values[key])
		args = append(args, "-c", "shell_environment_policy.set."+key+"="+string(value))
	}
	return args, nil
}

// Codex currently selects its default Unix shell from the account database,
// ignoring SHELL. The exec tool's explicit shell field is the supported override;
// keep the instruction with the role prompt rather than changing the user's shell.
func codexRolePrompt(role, goos, bash string) string {
	prompt := fmt.Sprintf("Invoke the %q skill now.", "assay:"+role)
	if goos != "windows" {
		prompt += fmt.Sprintf("\nFor every exec_command call, explicitly set shell=%q and login=false. Use Bash syntax; do not use the account's default zsh, source shell startup files, or omit the shell override. Pass this command-shell requirement to every subagent.", bash)
	} else {
		prompt += "\nUse the native Windows command shell with profile loading disabled; do not require Bash, zsh, or WSL."
	}
	prompt += "\nEvery deskdispatch that will post a model-attested result must include --model with the actual selected worker model. Check the dispatch receipt for a successful model stamp before spending work on that dispatch; a skipped stamp is not a successful review admission."
	return prompt
}

func codexBashPath() (string, error) {
	if runtime.GOOS == "windows" {
		return "", nil
	}
	path, err := exec.LookPath("bash")
	if err != nil {
		return "", fmt.Errorf("Codex desk commands require Bash on this platform: %w", err)
	}
	return filepath.Abs(path)
}
