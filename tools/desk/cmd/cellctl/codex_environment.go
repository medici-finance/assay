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
	operatorHome, err := operatorHomeFor(runtime.GOOS, c.Env)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(c.Home) || !filepath.IsAbs(operatorHome) {
		return nil, fmt.Errorf("Codex requires absolute cell and operator home paths")
	}
	codexHome, err := codexHomeFor(runtime.GOOS, c.Env)
	if err != nil {
		return nil, err
	}
	ghConfig, err := ghConfigDirFor(runtime.GOOS, c.Env)
	if err != nil {
		return nil, err
	}
	claudeConfig, err := claudeConfigDirFor(runtime.GOOS, c.Env)
	if err != nil {
		return nil, err
	}
	operatorConfig, err := captureOperatorConfig(c.Env, c.Home, c.Config)
	if err != nil {
		return nil, err
	}
	values := map[string]string{
		"HOME":              c.Home,
		"USERPROFILE":       c.Home,
		"ZDOTDIR":           c.Home,
		"ASSAY_CONFIG_HOME": c.Config,
		"CODEX_HOME":        codexHome,
		"GH_CONFIG_DIR":     ghConfig,
		// Rechecks may include Claude routes even from a Codex desk. Resolve
		// before HOME changes, just like the other operator resource paths.
		"CLAUDE_CONFIG_DIR": claudeConfig,
		"PATH":              filepath.Join(c.Dir, "bin") + string(filepath.ListSeparator) + deskToolsBin(c.Env) + string(filepath.ListSeparator) + c.Env.Get("PATH"),
		// Noninteractive Bash reads BASH_ENV even without login semantics.
		"BASH_ENV": "",
		"ENV":      "",
	}
	// Resolve from cell configuration even when the invoking shell differs. Only the
	// normalized non-secret opt-in crosses argv; never copy arbitrary config values.
	values[envAssayTelemetry] = c.telemetryValue()
	// Record the operator context only when capture found an operator-side
	// resource. From an already cell-scoped parent it stays unset, and the house
	// check then refuses the cell link compared with itself.
	if operatorConfig != "" {
		values[operatorConfigKey] = operatorConfig
	}
	// Forward only named non-secret launch context. Values are config overrides
	// on argv, so copying the entire environment here would disclose credentials.
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "GOCACHE", "GOMODCACHE", "GOPATH", "GOFLAGS", "ASSAY_GO_CACHE_POLICY", "DESK_LOOP", "DESK_SESSION", "DESK_ROOTS", "DESK_CELL", "DESK_ROLE", "DESK_COMMS_GATEWAY", "DESK_COMMS_KEY", "ASSAY_COCKPIT", "ASSAY_REPAIR_ADMISSION", "ASSAY_PERF_PROFILE", "ASSAY_PERF_SESSION_ID", "ASSAY_PERF_AGENT_ID", "ASSAY_PERF_PARENT_AGENT_ID", "ASSAY_PERF_PARENT_INVOCATION_ID", "ASSAY_PERF_ROLE", "ASSAY_PERF_WORK", envCellDBDSN, envCellDBPassfile} {
		for _, kv := range env {
			k, value, ok := strings.Cut(kv, "=")
			if ok && strings.EqualFold(k, key) {
				values[key] = value
			}
		}
	}
	return values, nil
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
	return codexSupervisionPrompt(prompt)
}

// Both host and scrubbed launches use the same reminder without changing their
// shell/environment composition or native Goal admission.
func codexSupervisionPrompt(prompt string) string {
	return prompt + `
Apply the loaded desk-common supervision/context procedure and Codex binding: react to child results, actionable messages and due role checks. Block on existing child/command handles until the next required check, using the longest wait permitted by the exposed tool and harness instructions; do not impose a 60-second polling cap. After an early timeout, re-arm the wait without agent inventories, log rereads or progress pings. Keep one non-model owner for routine CI watching; retain independent final readbacks. If efficient waiting is unavailable, report the gap and use only the existing operator-selected outer cadence after a clean handoff; do not simulate a monitor with repeated Goal turns or silently change launch mode. Keep the turn open while waiting for in-flight work instead of ending it just to trigger another Goal turn. Continue from instructions already in context; reload missing or changed instructions after compaction or source changes. For each new item use a fresh child context (fork_turns="none" when supported), passing the admitted assignment/packet paths and required instructions explicitly; reserve child reuse for the same item and lane. Preserve prior findings, refusals, claims, human holds, independent reviews, stop checks and required freshness checks.`
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

// Native Goal tools own persisted state and continuation. A launch prompt is a
// request, not a receipt: require tool readback before claiming a standing desk.
// Keep this wrapper out of explicit cadence, smoke and non-desk invocations.
func codexStandingPrompt(role, prompt string) string {
	objective := fmt.Sprintf("Run assay:%s as a standing desk: keep handling its authorized queue across turns until the user stops, pauses, or changes this assignment.", role)
	return fmt.Sprintf(`Before invoking the desk skill, establish its native Codex Goal. The user explicitly requests this standing objective: %q
Call get_goal first. Only if no Goal exists, call create_goal with that exact objective and omit token_budget, then call get_goal again. Verify the returned objective and active status; report the actual readback, not a promise to continue. If a Goal already exists, preserve its objective, status, budget, and usage: do not replace, reset, resume, or reactivate it. Continue this bootstrap only if the existing Goal is active and has the exact requested objective; otherwise report the existing state and wait for user direction before starting the desk.
If native Goal tools are unavailable, creation fails, or readback cannot confirm the active objective, report could-not-check: standing Goal not established, and wait for user direction. Do not substitute a prose promise or a headless loop.
An inactive Goal gates standing desk continuation, not separately authorized corrective work. When the user asks to correct a problem and resume, complete the authorized correction first, then call get_goal again; do not ask for the same correction permission twice. Keep stop flags, guard refusals, approvals, isolation and protected-action boundaries authoritative. A plain-language resume request is authorization, not evidence that native status changed. If readback is still inactive and no native resume tool is available, give the user the exact /goal resume control and explain that the available Goal tools cannot reactivate it. Do not call update_goal with an unsupported active status, recreate the Goal, or claim resumption before matching active readback.
Once verified, invoke the skill below. A completed batch, ordinary final response, or empty queue does not complete this standing objective. Continue authorized desk work across native Goal turns, using the skill's idle and stop rules. Human steering, pause/clear, stop flags, approvals, and protected-action boundaries remain authoritative; do not resume or recreate a Goal after the user pauses or clears it. Native continuation is not a durable timer or a guarantee of permanent service.
%s`, objective, prompt)
}
