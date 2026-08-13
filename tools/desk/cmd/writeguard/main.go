package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writeguard does NOT call deskkit.Guard() — it is a PreToolUse deny hook
// invoked per tool-call by the harness (not a loop command); a kill-switch
// check here would break file writes for every session including the stopped
// loops themselves (deskpushguard has a similar exemption).
//
// It DOES read one roster value — ASSAY_WRITEGUARD_CALLOUT, the adopter's
// dangerous-command callout (callout.go). That is a configured control surface,
// so this main declares its tool class and echoes the effective config like
// every other tool that reads one; it is no longer exempt from that rule.
//
// hookInput is the subset of the Claude Code PreToolUse stdin payload the
// guard needs. https://code.claude.com/docs/en/hooks
type hookInput struct {
	Cwd       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type fileToolInput struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
}

type bashToolInput struct {
	Command string `json:"command"`
}

// hookOutput is the PreToolUse JSON decision envelope. The guard blocks via
// {"permissionDecision":"deny"} on exit 0 rather than exit code 2: the hook
// is invoked through `go run`, which does NOT propagate the child's exit
// code (a program exit 2 surfaces as go run exit 1 = non-blocking error).
// JSON deny is parsed on exit 0 and behaves like exit 2 with a structured
// reason fed back to the model.
type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident.
	// ClassWrite is the only defensible class for a GUARD: it reads the config-home
	// file and NEVER the environment, so a steered session cannot hand this process
	// its own dangerous-command policy through an exported variable. ciEligible=false
	// says so even when GITHUB_ACTIONS is set.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run, like every other tool that reads a
	// configured control surface. It goes to stderr, which a PreToolUse hook exiting 0
	// does not surface to the user — so the visibility is there for a debug run and
	// for anyone diagnosing a refusal, without narrating every tool call. A callout
	// that has been silently unconfigured is exactly the change this makes visible.
	deskkit.EchoEffectiveConfig(os.Stderr)

	v, err := run(os.Stdin)
	if err != nil {
		// Fail open: a broken guard must never brick every session.
		// Printing nothing on exit 0 means "no decision".
		return
	}
	if v.Block {
		var out hookOutput
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.PermissionDecision = "deny"
		out.HookSpecificOutput.PermissionDecisionReason = v.Reason
		enc, err := json.Marshal(out)
		if err != nil {
			// Last-resort blocking path; only reachable if marshaling a
			// plain struct fails, but keep the guard from silently allowing.
			fmt.Fprintln(os.Stderr, v.Reason)
			os.Exit(2)
		}
		fmt.Println(string(enc))
	}
}

func run(stdin io.Reader) (Verdict, error) {
	data, err := io.ReadAll(io.LimitReader(stdin, 8<<20))
	if err != nil {
		return Verdict{}, err
	}
	var in hookInput
	if err := json.Unmarshal(data, &in); err != nil {
		return Verdict{}, err
	}

	projectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projectDir == "" {
		projectDir = in.Cwd
	}
	if projectDir == "" {
		return Verdict{}, fmt.Errorf("no project dir")
	}
	sharedRoot, err := mainCheckoutRoot(projectDir)
	if err != nil {
		return Verdict{}, err
	}

	sentinel, _ := sentinelPath() // "" on error: the sentinel gate is then inert
	cfg := Config{
		SharedRoot: sharedRoot,
		ProjectDir: projectDir,
		Cwd:        in.Cwd,
		CwdTop:     worktreeToplevel(in.Cwd),
		// #1035: the shared-homed exemption is opt-in. #1190: the claim is
		// the env token OR a short-lived sentinel file, because the env token
		// alone is unreachable from the Bash tool.
		SharedOK:     sharedOKClaimed(sharedRoot),
		SentinelPath: sentinel,
		// The adopter's dangerous-command callout, "" when unconfigured (the shipped
		// state: compiled generic indicators only). Read from the roster's config-home
		// file, never from the environment — see the class declaration in main().
		Callout: deskkit.WriteguardCalloutPath(),
	}

	// What the tool call targets: a file path, or a Bash command string.
	var target string
	switch in.ToolName {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		var fi fileToolInput
		if err := json.Unmarshal(in.ToolInput, &fi); err != nil {
			return Verdict{}, err
		}
		target = fi.FilePath
		if target == "" {
			target = fi.NotebookPath
		}
	case "Bash":
		var bi bashToolInput
		if err := json.Unmarshal(in.ToolInput, &bi); err != nil {
			return Verdict{}, err
		}
		target = bi.Command
	default:
		return Verdict{}, nil
	}
	if target == "" {
		return Verdict{}, nil
	}

	// The claim sentinel is human-only, and that check runs BEFORE the
	// exemption gate: an exempt session must not be able to renew its own
	// claim indefinitely, and a guarded one must not be able to issue one
	// (#1190 review, blocking 2).
	if v := cfg.CheckClaimSentinel(in.ToolName, target); v.Block {
		return v, nil
	}
	if cfg.Exempt() {
		return Verdict{}, nil
	}
	if in.ToolName == "Bash" {
		return cfg.CheckBash(target), nil
	}
	return cfg.CheckFilePath(in.ToolName, target), nil
}

// --- the shared-OK claim (#1035 opt-in, #1190 reachability) ---
//
// #1035 made the shared-homed exemption opt-in via the WRITEGUARD_SHARED_OK
// env var, "exported once per shell". #1190: that is unreachable from the
// Claude Code Bash tool — every Bash call runs a FRESH shell, and the hook
// reads its OWN process env, so an inline `WRITEGUARD_SHARED_OK=1 cmd`
// prefix can never be seen by the guard. A sanctioned exemption nobody can
// claim is not a gate, it is a dead end that teaches operators to route
// around the guard.
//
// The claim therefore also lives in a sentinel FILE, which survives the
// per-call shell:
//
//	mkdir -p ~/.config/assay && touch ~/.config/assay/writeguard-shared-ok
//
// Two properties keep it from silently re-opening the #1035 hole (a
// dispatched worker booted by `cd <shared>` is shared-homed by construction,
// and would inherit a permanent machine-wide exemption):
//
//   - It EXPIRES (sentinelTTL, default 12h from mtime) — an env export dies
//     with its shell, so the file claim is time-bounded too. Renew: touch.
//   - It can be SCOPED: any non-blank, non-"#" lines are shared-checkout
//     paths the claim applies to. An empty file claims any checkout on this
//     machine (the `touch` ergonomics), a scoped file only the listed ones.
//
// The claim still never exempts a session that is not shared-homed
// (Config.Exempt), which is the load-bearing half of the isolation backstop.
const defaultSentinelTTL = 12 * time.Hour

func sentinelTTL() time.Duration {
	if v := os.Getenv("WRITEGUARD_SHARED_OK_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultSentinelTTL
}

// sentinelPath returns the sentinel file path: WRITEGUARD_SHARED_OK_FILE if
// set, else $XDG_CONFIG_HOME/assay/writeguard-shared-ok, else
// ~/.config/assay/writeguard-shared-ok.
func sentinelPath() (string, error) {
	if p := os.Getenv("WRITEGUARD_SHARED_OK_FILE"); p != "" {
		return p, nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "assay", "writeguard-shared-ok"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("no home directory")
	}
	return filepath.Join(home, ".config", "assay", "writeguard-shared-ok"), nil
}

// sentinelPathForMessage is sentinelPath for block-message text, degrading to
// the documented default when the home directory cannot be resolved.
func sentinelPathForMessage() string {
	p, err := sentinelPath()
	if err != nil {
		return "~/.config/assay/writeguard-shared-ok"
	}
	return p
}

// sharedOKClaimed reports whether the shared-checkout exemption has been
// claimed for sharedRoot — by the env token (human shells) or by a live,
// in-scope sentinel file.
func sharedOKClaimed(sharedRoot string) bool {
	if os.Getenv("WRITEGUARD_SHARED_OK") != "" {
		return true
	}
	p, err := sentinelPath()
	if err != nil {
		return false
	}
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return false
	}
	if time.Since(fi.ModTime()) > sentinelTTL() {
		return false // stale claim: an exemption must not outlive its session
	}
	body, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return sentinelCoversRoot(string(body), sharedRoot)
}

// sentinelCoversRoot reports whether the sentinel body claims sharedRoot: an
// empty body (or one with only comments) claims any checkout; otherwise only
// the listed paths are claimed.
func sentinelCoversRoot(body, sharedRoot string) bool {
	scoped := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		scoped = true
		if samePath(resolvePath(line), resolvePath(sharedRoot)) {
			return true
		}
	}
	return !scoped
}

// worktreeToplevel returns the toplevel of the git worktree containing dir,
// or "" if it cannot be determined (dir empty, not a git tree, git error).
// Used to detect sessions whose CLAUDE_PROJECT_DIR claims the shared
// checkout while the payload cwd actually sits in a linked/ad-hoc worktree
// (#1007) — Exempt treats "" as unknown and falls back to path containment.
func worktreeToplevel(dir string) string {
	if dir == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// mainCheckoutRoot returns the main (shared) checkout root of the repo that
// contains dir: dirname of `git rev-parse --git-common-dir`, which from any
// linked worktree points at <shared>/.git.
func mainCheckoutRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	gd := strings.TrimSpace(string(out))
	if filepath.Base(gd) != ".git" {
		return "", fmt.Errorf("unexpected git common dir layout: %s", gd)
	}
	return filepath.Dir(gd), nil
}
