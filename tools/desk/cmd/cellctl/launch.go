package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"golang.org/x/term"
)

// repairAdmissionValue is the ASSAY_REPAIR_ADMISSION value this cell composes into a launched
// desk's environment, or "" when the gate is off or unset. Only the literal "on" composes the
// key — "off", unset, and any other value are IDENTICAL absence, exactly as the deskdispatch
// consumer treats them: deskkit.RepairAdmissionEnabled enables the dispatch-boundary gate ONLY
// on "on", so composing the key with any other value (or an explicit "off") would be
// indistinguishable to the consumer from omitting it, and the shipped default is off. Reading
// c.Env is what makes this durable: it is the process environment with cell.env overlaid, so a
// cell that writes ASSAY_REPAIR_ADMISSION=on into its cell.env turns the gate on for every desk
// it launches, not only for a shell that happened to `export` it once.
func (c *Cell) repairAdmissionValue() string {
	if strings.TrimSpace(c.Env.Get(deskkit.EnvRepairAdmission)) == "on" {
		return "on"
	}
	return ""
}

// deskLaunch is the LIVE (non-dry-run) half of `desk`: persist what --set asked for, enable the
// plugin on the claude arm, fetch the shared checkout under a lock, bring the role worktree up
// to main, generate the shims, and exec the harness.
//
// Every process here goes through os/exec — no syscall, no shell — so the Windows consequence
// this brief names is not made worse.
func (c *Cell) deskLaunch(role, harness, model, modelDisp, session, wt, cfg, provider string, prov Provider, deskRoots string, persist bool, persistKVs []string, policyRes *PolicyResolution) {
	if persist {
		applyEnvKVs(c.Env, filepath.Join(c.Dir, "cell.env"), false, persistKVs)
	}
	// Enabling the assay@assay plugin is a CLAUDE-specific action — skipped on the codex arm
	// (skills are discovered per the codex harness check block) and on a scrubbed cell (there is
	// no host cfg there; the plugin state lives under the cell's own CLAUDE_CONFIG_DIR).
	if harness == "claude" && c.Kind != "scrubbed" {
		// A non-zero exit here is not always a real failure: `claude plugin enable` also exits 1
		// when the plugin is already enabled, printing `… is already enabled` — so the NOTICE
		// fires only when the captured output does NOT match that shape.
		cmd := exec.Command("claude", "plugin", "enable", "assay@assay")
		cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+cfg)
		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "already enabled") {
			fmt.Fprintf(os.Stderr, "NOTICE: could not enable assay@assay in %s — inside the session: /plugin install assay@assay\n", cfg)
		}
	}

	sha := c.fetchMainUnderLock()
	_ = os.MkdirAll(filepath.Join(c.Dir, "worktrees"), 0o755)
	if _, err := os.Stat(filepath.Join(wt, ".git")); err == nil {
		// An existing tree is MERGED up to the fetched main, or the boot stops — never left
		// behind with a notice (#1157).
		c.mergeRoleWorktree(wt, sha)
	} else if dwt, ok := c.worktreeViaDeskwt(role); ok {
		_ = os.Remove(wt)
		if lerr := os.Symlink(dwt, wt); lerr != nil {
			die("desk: could not link %s -> %s: %v", wt, dwt, lerr)
		}
		fmt.Printf("[worktree] created by deskwt role-init: %s (linked from %s)\n", dwt, wt)
	} else {
		branch := fmt.Sprintf("%s-cell/%s-%s", c.Name, role, time.Now().Format("20060102-150405"))
		if err := c.git(c.Repo, "worktree", "add", wt, "-b", branch, sha); err != nil {
			die("desk: could not create worktree %s: %v", wt, err)
		}
	}
	// A live role window's tree is LOCKED, so a worktree prune never takes it from under the
	// session. Re-locking an already-locked tree is a no-op here, not a failure.
	_ = c.git(c.Repo, "worktree", "lock", "--reason", "cellctl "+c.Name+"/"+role+" live session", wt)
	fmt.Printf("[worktree] %s @ %s (locked)\n", wt, short8(sha))

	if c.Deskd == "1" && !c.deskdUp() {
		fmt.Fprintf(os.Stderr, "NOTICE: cell deskd is NOT up on %s — in your shell: CELL_ATTENDED=1 cellctl deskd %s\n", c.DeskdAddr, c.Name)
	}
	c.genShims()
	if harness == "codex" {
		c.ensureCodexResidentRules(wt)
	}
	if c.Kind == "scrubbed" {
		c.scrubbedDeskLaunch(role, harness, model, session, wt)
		return
	}
	providerDisp := provider
	if policyRes != nil {
		providerDisp = policyRes.Provider
	}
	effortDisp := "harness-default"
	if policyRes != nil {
		effortDisp = policyRes.Effort
	}
	fmt.Printf("[launch] %s/%s kind=%s model=%s effort=%s provider=%s harness=%s session=%s config=%s cwd=%s desk_roots=%s (desk verbs → HOME=%s)\n",
		c.Name, role, c.Kind, modelDisp, effortDisp, orDefault(providerDisp, "anthropic"), harness, session, cfg, wt, orDefault(deskRoots, "unset"), c.Home)

	env := os.Environ()
	env = envSet(env, "PATH", filepath.Join(c.Dir, "shim")+":"+c.Env.Get("PATH"))
	env = envSet(env, "DESK_LOOP", role)
	env = envSet(env, "DESK_SESSION", session)
	if deskRoots != "" {
		env = envSet(env, "DESK_ROOTS", deskRoots)
	}
	// The dispatch-boundary repair-admission opt-in (deskkit.EnvRepairAdmission), composed here
	// so a cell can turn the gate on DURABLY through its cell.env rather than only via a one-off
	// `export`. Composed for every role — the gate only actually runs inside deskdispatch (a
	// worker-desk verb), so carrying the key for the other roles is a harmless no-op. Off/unset
	// leaves the key ABSENT, which the consumer reads identically to "off".
	if rav := c.repairAdmissionValue(); rav != "" {
		env = envSet(env, deskkit.EnvRepairAdmission, rav)
	}
	var argv []string
	if harness == "codex" {
		// The same exported env the claude arm gets; CLAUDE_CONFIG_DIR is irrelevant on this
		// arm, so it is not passed, and a provider is a claude-endpoint switch with no codex
		// equivalent, so it is not threaded on either. Under a policy, its -c overrides
		// (model_provider/model_reasoning_effort/agents.default_subagent_*) are propagated —
		// effort propagation into the actual launch argv, not just the model name.
		argv = []string{"codex"}
		if policyRes != nil {
			argv = append(argv, policyRes.CodexArgs...)
		}
		argv = append(argv, "--sandbox", "danger-full-access", "-C", wt, "-m", model,
			fmt.Sprintf("Invoke the %q skill now.", "assay:"+role))
	} else {
		env = envSet(env, "CLAUDE_CONFIG_DIR", cfg)
		if provider != "" {
			// An inherited API key wins over the auth token and silently routes to Anthropic,
			// so it is UNSET first; then the model plus the three tier aliases are pinned to a
			// name the provider's endpoint accepts.
			env = envUnset(env, "ANTHROPIC_API_KEY")
			pmodel := prov.Model
			if pmodel == "" {
				pmodel = model
			}
			env = envSet(env, "ANTHROPIC_BASE_URL", prov.BaseURL)
			env = envSet(env, "ANTHROPIC_AUTH_TOKEN", prov.TokenVal)
			env = envSet(env, "ANTHROPIC_MODEL", model)
			env = envSet(env, "ANTHROPIC_DEFAULT_OPUS_MODEL", pmodel)
			env = envSet(env, "ANTHROPIC_DEFAULT_SONNET_MODEL", pmodel)
			env = envSet(env, "ANTHROPIC_DEFAULT_HAIKU_MODEL", pmodel)
		}
		argv = []string{"claude", "--name", session, "--model", model, "/assay:" + role}
		if policyRes != nil {
			// The policy's own env block (ANTHROPIC_MODEL/CLAUDE_CODE_SUBAGENT_MODEL/
			// CLAUDE_CODE_EFFORT_LEVEL/ANTHROPIC_DEFAULT_*_MODEL, ANTHROPIC_BASE_URL for the
			// anthropic provider) is applied on top of whatever the glm/kimi credential block
			// above just set — this is effort propagation into the launch record: the harness
			// receives the pinned effort both as `--effort` and as CLAUDE_CODE_EFFORT_LEVEL.
			for k, v := range policyRes.ClaudeEnv {
				env = envSet(env, k, v)
			}
			env = envUnset(env, "MAX_THINKING_TOKENS")
			argv = []string{"claude", "--effort", policyRes.Effort, "--name", session, "--model", model, "/assay:" + role}
		}
	}
	runForeground(argv, env, wt)
}

// scrubbedDeskLaunch is the live scrubbed launch: take the session lock, then run
// `env -i <allowlist> <harness argv>` as the SOLE command of a new tmux session on this cell's
// PRIVATE SOCKET. `env -i` inside that pane discards whatever ambient environment the private
// tmux SERVER itself was started with, so nothing of the launching shell reaches the harness
// regardless of what launched the socket.
func (c *Cell) scrubbedDeskLaunch(role, harness, model, session, wt string) {
	if _, err := exec.LookPath("tmux"); err != nil {
		die("desk: tmux is required for a scrubbed cell (the private-socket session) and is not on PATH")
	}
	_ = os.MkdirAll(filepath.Join(c.Dir, "run"), 0o700)
	sockpath := filepath.Join(c.Dir, "run", "tmux.sock")
	sessname := c.Name + "-cell"
	lockdir := filepath.Join(c.Dir, "run", "lock.d")
	// The lock is taken BEFORE the tmux new-session call — this is what a race between two
	// concurrent `desk` invocations serialises on. The pid it holds is provisional (this
	// process) until the pane exists, at which point it is rewritten to the PANE's pid, the
	// process that actually outlives this invocation when it is not attaching.
	c.takeSessionLock(lockdir, sessname)
	if exec.Command("tmux", "-S", sockpath, "has-session", "-t", sessname).Run() == nil {
		_ = os.RemoveAll(lockdir)
		die("desk: session %s is already running on the private socket (%s) — cellctl down %s to release", sessname, sockpath, c.Name)
	}
	env := c.scrubbedComposeEnv(role, harness, session)
	argv := harnessArgv(harness, role, model, session, wt)
	var cmdline strings.Builder
	cmdline.WriteString("env -i")
	for _, kv := range env.Pairs {
		cmdline.WriteString(" ")
		cmdline.WriteString(bashQuote(kv))
	}
	for _, a := range argv {
		cmdline.WriteString(" ")
		cmdline.WriteString(bashQuote(a))
	}
	fmt.Printf("[launch] %s/%s kind=scrubbed model=%s harness=%s session=%s cwd=%s (env -i composed — see docs/cellctl.md Scrubbed cells)\n",
		c.Name, role, model, harness, sessname, wt)
	_ = exec.Command("tmux", "-S", sockpath, "new-session", "-d", "-s", sessname, "-c", wt, cmdline.String()).Run()
	if out, err := exec.Command("tmux", "-S", sockpath, "list-panes", "-t", sessname, "-F", "#{pane_pid}").Output(); err == nil {
		if pane := firstLine(string(out)); pane != "" {
			writePid(lockdir, pane)
		}
	}
	if isTTY(os.Stdin) {
		runForeground([]string{"tmux", "-S", sockpath, "attach", "-t", sessname}, os.Environ(), "")
		return
	}
	fmt.Printf("[cell] attach: tmux -S %s attach -t %s   ·   down: cellctl down %s\n", sockpath, sessname, c.Name)
}

// runForeground replaces this process with argv as far as the caller can tell: it hands over
// stdio, waits, and exits with the child's status. It goes through os/exec rather than an
// exec(2) syscall so the package adds no direct syscall use (row 9) — the observable difference
// is one extra process in the tree.
func runForeground(argv []string, env []string, dir string) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		exitWith(exitStatus(err))
	}
	exitWith(0)
}

func envSet(env []string, k, v string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, k+"=") {
			out = append(out, kv)
		}
	}
	return append(out, k+"="+v)
}

func envUnset(env []string, k string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, k+"=") {
			out = append(out, kv)
		}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func short8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// isTTY is bash's `[[ -t 0 ]]` — a real isatty(3), not a mode test.
//
// This FAILS CLOSED and the distinction matters: `os.ModeCharDevice` is set for /dev/null too,
// so a mode test reports an unattended run (stdin from /dev/null, a pipe, cron, CI) as attended.
// On `cellctl up` that is exactly the widening the attended affirmation exists to prevent — from
// "an operator ran this" to "anything that ran cellctl at all" — and it would carry that
// affirmation into a credential mint. Anything that is not a terminal answers false.
func isTTY(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
