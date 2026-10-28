package main

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/perfrec"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// upOverrides are the values `role_cmd` threads onto every role's own `cellctl desk`
// invocation. They are GLOBAL to one `up` run rather than extra parameters through every cockpit
// arm, exactly as the oracle keeps them, and they apply to EVERY role window this run opens —
// there is deliberately no per-role `--model-<role>` form, because that case is already
// `cellctl desk <cell> <role> --model <m>` on the one window that needs it.
type upOverrides struct {
	Comms       bool
	Model       string
	Harness     string
	Provider    string
	Cadence     string
	TickBudget  string
	roleCadence map[string]*cadenceOptions // resolved per role, including mixed-harness policies
	roleHarness map[string]string
	// Cockpit is the cockpit `up` RESOLVED (never `auto`), threaded onto every window so each
	// exports the same ASSAY_COCKPIT as the surface it was opened in — even when `up --cockpit`
	// overrode cell.env for this run only.
	Cockpit string
}

// roleCmd is the one command a role window runs, identical in every cockpit. Order is fixed
// (--model, then --harness, then --provider, then --cockpit) so a printed command is stable to
// grep against. It is the POSIX rendering; a cockpit arm never calls it directly, it calls
// paneRoleCmd, which picks the rendering the pane's own shell can parse.
func (c *Cell) roleCmd(role, cfg string, o upOverrides) string {
	return c.roleCmdIn(shellPOSIX, selfPath(), role, cfg, o)
}

// paneShell is the shell a cockpit pane hands a command line to. The command is one argv —
// the same words in every shell — but the line that spells it differs: POSIX single quotes
// are not PowerShell syntax, and in PowerShell a leading quoted path is a string expression,
// not an invocation, so the line needs the `&` call operator in front of it.
type paneShell int

const (
	shellPOSIX paneShell = iota
	shellPowerShell
)

// paneShellFor decides which shell a cockpit's pane runs the command in. tmux hosts a POSIX
// shell on every platform it runs on (including an MSYS/Cygwin build on Windows), so its arm is
// POSIX unconditionally. herdr and orca open the platform's default terminal shell, which on
// Windows is PowerShell; everywhere else it is a POSIX shell. Anything else — including an
// unresolved cockpit — keeps the POSIX rendering.
func paneShellFor(goos, cockpit string) paneShell {
	if goos == "windows" && (cockpit == "herdr" || cockpit == "orca") {
		return shellPowerShell
	}
	return shellPOSIX
}

// quote renders one argv word for this shell.
func (sh paneShell) quote(s string) string {
	if sh == shellPowerShell {
		return psQuote(s)
	}
	return cockpitQuote(s)
}

// invoke renders a command line that RUNS the already-quoted program word: bare in POSIX, behind
// the `&` call operator in PowerShell.
func (sh paneShell) invoke(quotedProgram string) string {
	if sh == shellPowerShell {
		return "& " + quotedProgram
	}
	return quotedProgram
}

// paneRoleCmd is roleCmd rendered for the shell the resolved cockpit's pane runs on this host.
// Every cockpit arm, dry-run line and hand-start NOTICE goes through it, so the line a pane is
// handed and the line an operator is told to paste are the same line.
func (c *Cell) paneRoleCmd(role, cfg string, o upOverrides) string {
	return c.roleCmdIn(paneShellFor(runtime.GOOS, o.Cockpit), selfPath(), role, cfg, o)
}

// roleCmdIn renders the role command for one shell. self is the program word — selfPath() in
// production — taken as a parameter so a test can pin the whole line.
func (c *Cell) roleCmdIn(sh paneShell, self, role, cfg string, o upOverrides) string {
	// Cockpits accept a command string today. Quote every argument independently;
	// no model, path or configuration value is executable shell syntax.
	cadence := c.Cadence
	if o.roleCadence != nil {
		cadence = o.roleCadence[role]
	}
	if cadence != nil {
		o.Cadence = cadence.Interval.String()
		o.TickBudget = cadence.Budget.String()
	}
	out := sh.invoke(sh.quote(self))
	root, name := c.reenter()
	harness := o.Harness
	if resolved, ok := o.roleHarness[role]; ok {
		harness = resolved
	} else if harness == "" {
		harness = c.Harness
	}
	if cadence != nil || o.Cadence != "" || harness == "codex" {
		out += " --cells-root " + sh.quote(root)
	}
	out += " desk " + sh.quote(name) + " " + sh.quote(role)
	for _, pair := range [][2]string{{"--kind", c.KindOverride}, {"--model", o.Model}, {"--harness", o.Harness}, {"--provider", o.Provider}, {"--cockpit", o.Cockpit}, {"--cadence", o.Cadence}, {"--tick-budget", o.TickBudget}} {
		if pair[1] != "" {
			out += " " + pair[0] + " " + sh.quote(pair[1])
		}
	}
	return out + " " + sh.quote(cfg)
}

func cockpitQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// psQuote renders s as one PowerShell single-quoted string literal. Inside single quotes
// PowerShell takes every character verbatim — `$`, backtick, backslash, spaces — except a
// single-quote character, which is escaped by doubling it. PowerShell counts the typographic
// quotes U+2018..U+201B as single-quote characters too, so each of those is doubled as well:
// the rule PowerShell's own EscapeSingleQuotedStringContent applies. Bytes that are not valid
// UTF-8 pass through untouched.
//
// Known limit, not a quoting one: Windows PowerShell 5.1 (and pwsh before 7.3) mangles an
// embedded double quote when it builds a NATIVE program's command line from the argument. No
// value roleCmd passes can carry one — a Windows path cannot, and the flag values are names and
// durations — so the literal here stays exact rather than guessing at that legacy behaviour.
func psQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+size])
		switch r {
		case '\'', '\u2018', '\u2019', '\u201a', '\u201b':
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('\'')
	return b.String()
}

// selfPath is the absolute path of this binary. The windows `up` opens re-invoke cellctl from
// the CELL directory, where a relative argv[0] does not resolve — so the re-invocation uses
// this, never argv[0].
func selfPath() string {
	p, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return p
}

// firstWindow decides the window that is NOT a role: the deskd watcher or the deskd itself. A cell
// with no deskd (DESKD is not 1 — the house default) has NO such window — name and cmd come back empty,
// the one-line "no deskd" note goes to up's own stderr exactly once, and every cockpit opens only
// the role windows (tmux creates its session with the first role window instead).
//
// STANDING deskd FROM `up` — the attended affirmation, and how far it carries. `cellctl deskd`
// stands live cross-org reads and so refuses without CELL_ATTENDED=1: the affirmation is the
// operator saying "I am here, at my own shell". `up` is the one-command cockpit, so it passes
// that affirmation to the deskd window it spawns — but it may only do so when `up` is ITSELF
// attended, or the flag would be widened from "an operator ran this" to "anything that ran
// cellctl at all", which is the opposite of what it is for.
//
// Attended means one of two POSITIVE signals: a terminal on stdin, or CELL_ATTENDED=1 already in
// the environment. A cron or CI invocation has neither, and there `up` stands the role windows
// but NOT deskd, and says so. It is never silent in either direction.
//
// The first window lands in the same pane shell as the role windows (paneShellFor), so every
// line it is handed is rendered for that shell too; the POSIX lines are the ones the tmux arm
// has always run.
func (c *Cell) firstWindow(sh paneShell) (name, cmd string) {
	switch {
	case c.Deskd != "1":
		fmt.Fprintf(os.Stderr, "[cell] %s is a %s cell with no deskd (DESKD=1 in cell.env to stand one)\n", c.Name, c.Kind)
		return "", ""
	case c.deskdUp():
		return "deskd", deskdWatchCmd(sh, c.DeskdAddr)
	case isTTY(os.Stdin) || c.Env.GetOr("CELL_ATTENDED", "0") == "1":
		fmt.Printf("[cell] deskd is not up — standing it in-session, carrying YOUR attended affirmation (cellctl up ran attended)\n")
		return "deskd", deskdStandCmd(sh, selfPath(), c.Name)
	default:
		fmt.Fprintln(os.Stderr, "[cell] deskd is not up, and this 'cellctl up' is UNATTENDED (no terminal on stdin, CELL_ATTENDED unset).")
		fmt.Fprintln(os.Stderr, "[cell] role windows will open; deskd will NOT be stood, because minting tokens needs an affirmation this invocation cannot make.")
		fmt.Fprintf(os.Stderr, "[cell] stand it yourself: %s\n", deskdHandStart(sh, c.Name))
		return "deskd", sh.echo(fmt.Sprintf("[deskd] NOT stood — cellctl up ran unattended. Run: %s", deskdHandStart(sh, c.Name)))
	}
}

// echo renders a line that prints msg. The POSIX form keeps the bare single quotes the tmux arm
// has always run; the PowerShell form quotes msg as one literal.
func (sh paneShell) echo(msg string) string {
	if sh == shellPowerShell {
		return "Write-Output " + psQuote(msg)
	}
	return "echo '" + msg + "'"
}

// deskdWatchCmd is the first window when a deskd is already up: poll its /healthz once a minute.
func deskdWatchCmd(sh paneShell, addr string) string {
	if sh == shellPowerShell {
		// curl.exe, never curl: in Windows PowerShell 5.1 `curl` is an alias of Invoke-WebRequest.
		return sh.echo("[deskd] already up on "+addr+" — watching /healthz every 60s") +
			"; while ($true) { (Get-Date).ToUniversalTime().ToString('HH:mm') + 'Z'; $h = (curl.exe -s --max-time 5 " + psQuote("http://"+addr+"/healthz") +
			" | Out-String); if ($h.Length -gt 240) { $h = $h.Substring(0, 240) }; $h; Start-Sleep -Seconds 60 }"
	}
	return fmt.Sprintf("echo '[deskd] already up on %s — watching /healthz every 60s'; while :; do date -u +%%H:%%MZ; curl -s --max-time 5 http://%s/healthz | head -c 240; echo; sleep 60; done", addr, addr)
}

// deskdStandCmd stands deskd in the window, carrying the attended affirmation into it.
func deskdStandCmd(sh paneShell, self, cell string) string {
	if sh == shellPowerShell {
		return "$env:CELL_ATTENDED='1'; " + sh.invoke(psQuote(self)) + " deskd " + psQuote(cell)
	}
	return fmt.Sprintf("CELL_ATTENDED=1 '%s' deskd '%s'", self, cell)
}

// deskdHandStart is the command an operator types to stand deskd by hand.
func deskdHandStart(sh paneShell, cell string) string {
	if sh == shellPowerShell {
		return "$env:CELL_ATTENDED='1'; cellctl deskd " + cell
	}
	return "CELL_ATTENDED=1 cellctl deskd " + cell
}

// upControl carries what the adapter around cmdUp decided: the invocation, whether the machine form
// was asked for, and the stored plan to apply.
type upControl struct {
	cmd     *cobra.Command
	json    bool
	applyID string
	stdout  *os.File // the real stdout while --json has human output on stderr
}

// toStderr moves every human line of a --json run to stderr, so stdout carries the one document.
func (ctl *upControl) toStderr() {
	if ctl.json && ctl.stdout == nil {
		ctl.stdout = os.Stdout
		os.Stdout = os.Stderr
	}
}

// restore gives stdout back before the document is written.
func (ctl *upControl) restore() {
	if ctl.stdout != nil {
		os.Stdout = ctl.stdout
		ctl.stdout = nil
	}
}

// emit writes the one cell-op-result document of a --json run that launched nothing (a dry run,
// or a cell with nothing to open) and exits; without --json it does nothing.
func (ctl *upControl) emit(c *Cell, started time.Time, rows []upRow) {
	if !ctl.json || ctl.applyID != "" {
		return
	}
	ctl.restore()
	r := upResult(c.Name, started, rows)
	emitResult(ctl.cmd, r)
	exitWith(r.ExitCode())
}

func cmdUp(c *Cell, v *cli.Values, cfgIn string, ctl upControl) {
	if c.Kind == "container" || c.Kind == "scrubbed" {
		// Container and scrubbed cells run exactly one desk, and `up` hands it the typed desk
		// options. The host-cockpit flags have no meaning there, and desk has always refused them
		// by name — keep that refusal where it was, now stated for the flags up declares.
		for _, hostOnly := range []string{"no-the-desk", "with-the-desk", "no-attach", "automate", "role", "plan", "apply", "json", "lease-wait"} {
			if v.Source(hostOnly) == cli.Flag {
				die("desk: unknown flag --%s", hostOnly)
			}
		}
	}
	if c.Kind == "container" {
		if strings.Join(c.Roles, " ") != "the-desk" {
			die("container up currently requires ROLES=the-desk; use desk for explicit roles")
		}
		cmdDesk(c, "the-desk", deskInputsFrom(v, cfgIn))
		return
	}
	if c.Kind == "scrubbed" {
		// A scrubbed cell is single-occupancy by construction (one tmux session on its private
		// socket, one mkdir lock) — `up` refuses when the cell is not `stopped` and otherwise
		// boots this cell's first configured role, exactly what `desk` itself would; the
		// attach-on-a-tty behaviour lives in the launch, so every entry point gets it alike.
		if st := c.statusLine(); st != "stopped" {
			die("up: cell '%s' is not stopped (status: %s) — cellctl down %s first", c.Name, st, c.Name)
		}
		first := "the-desk"
		if len(c.Roles) > 0 {
			first = c.Roles[0]
		}
		cmdDesk(c, first, deskInputsFrom(v, cfgIn))
		return
	}

	ctl.toStderr()
	defer ctl.restore()
	started0 := time.Now()
	c.launchStarted = perfrec.ProcessEntry().At()
	noTheDesk, attach, persist := v.Bool("no-the-desk"), !v.Bool("no-attach") && !ctl.json && ctl.applyID == "", v.Bool("set")
	wait := v.Duration("lease-wait")
	if wait <= 0 {
		die("up: --lease-wait must be a positive duration")
	}
	cockpitFlag, automate := v.String("cockpit"), v.String("automate")
	var o upOverrides
	// Only what was GIVEN by flag is threaded onto each role window's own `desk` command line;
	// the cell.env pins are re-read by that desk, so they are not copied here.
	o.Cadence = flagOnly(v, "cadence")
	o.TickBudget = flagOnly(v, "tick-budget")
	o.Model = flagOnly(v, "model")
	o.Provider = v.String("provider")
	o.Harness = v.String("harness")
	if o.Harness != "" && !valueIn(o.Harness, harnessValues) {
		die("up: --harness must be claude or codex, got '%s'", o.Harness)
	}
	if cockpitFlag != "" && !valueIn(cockpitFlag, cockpitValues) {
		die("up: --cockpit must be one of %s, got '%s'", joinPipe(cockpitValues), cockpitFlag)
	}
	if persist && !anyGiven(o.Model, o.Harness, o.Provider, c.KindOverride, cockpitFlag, o.Cadence, o.TickBudget) {
		die("up: --set needs --model, --harness, --provider, --kind or --cockpit — nothing to persist otherwise")
	}
	effectiveHarness := o.Harness
	if effectiveHarness == "" {
		effectiveHarness = c.Harness
	}
	cadence := v.String("cadence")
	budget := v.String("tick-budget")
	if effectiveHarness == "cursor" && cfgIn != "" {
		die("cursor does not accept a Claude config directory")
	}
	cfg := ""
	if effectiveHarness != "cursor" || cfgIn != "" {
		cfg = resolveCfg(c.Env, cfgIn)
	}
	want, src := c.cockpitWant(cockpitFlag)
	res := c.resolveCockpit(want, src)
	if res.Err != "" {
		die("%s", res.Err)
	}
	fmt.Printf("[cockpit] %s (%s)\n", res.Cockpit, res.Why)
	o.Cockpit = res.Cockpit
	if automate != "" && ctl.applyID != "" {
		die("up: --apply cannot be combined with --automate (scheduled automations open no window and hold no lease, so there is nothing to apply)")
	}
	if automate != "" && res.Cockpit != "orca" {
		die("up: --automate is an orca-only shape (scheduled automations behind an exit-code precheck); the resolved cockpit is %s", res.Cockpit)
	}
	roles := c.upTargets(v.Strings("role"), noTheDesk)
	if ctl.applyID != "" && persist {
		die("up: --apply changes state and cannot persist overrides (--set)")
	}

	if effectiveHarness == "cursor" {
		provider := o.Provider
		if provider == "" {
			provider = c.Env.Get("CELL_PROVIDER")
		}
		if err := cursorConfigurationError(c.Kind, provider, false); err != nil {
			die("up: %v", err)
		}
		if _, err := exec.LookPath("agent"); err != nil {
			die("up: Cursor agent executable is unavailable")
		}
		for _, role := range roles {
			if o.Model == "" && !c.resolveRoleModel(role, "cursor").OK {
				die("up: no Cursor model configured for %s", role)
			}
		}
	}
	policy, policySource, err := c.cellModelPolicy()
	if err != nil {
		die("%s", err)
	}
	if policy != nil {
		if effectiveHarness == "cursor" {
			die("model policy does not support cursor; configure Cursor model pins without a model policy")
		}
		if automate != "" {
			die("up: --automate cannot propagate model policy; use live desk windows")
		}
		if persist {
			die("up: --set with a model policy is ambiguous; edit the policy or provider defaults instead")
		}
	}
	// Resolve the cadence with each role's actual harness, not the cell-wide fallback.
	// No harness implies a schedule; an explicit cadence retains its per-role route.
	o.roleCadence = make(map[string]*cadenceOptions, len(roles))
	o.roleHarness = make(map[string]string, len(roles))
	for _, role := range roles {
		harness := effectiveHarness
		if _, err := c.deskCommsEnv(role, nil); err != nil {
			die("up comms: %v; no role windows launched", err)
		}
		if policy != nil {
			route, err := policy.Resolve(role, o.Provider, o.Model, o.Harness)
			if err != nil {
				die("%s", err)
			}
			if err := c.policyPreflight(route, cfg); err != nil {
				fmt.Fprintln(os.Stderr, err)
				die("up: policy preflight failed; no role windows launched")
			}
			harness = route.Harness
			fmt.Printf("[policy] role=%s provider=%s model=%s effort=%s source=%s sha256=%s\n", role, route.Provider, route.Model, route.Effort, policySource, policy.SHA256)
		}
		if _, rc, err := c.roleContextFor(role, harness); err != nil {
			die("up: %v; no role windows launched", err)
		} else if rc != nil && automate != "" {
			die("up: --automate cannot apply the role context declared for %s; use live desk windows", role)
		}
		o.roleHarness[role] = harness
		o.roleCadence[role] = resolveDeskCadence(c.Kind, harness, cadence, budget)
		if automate != "" && (o.roleCadence[role] != nil || harness != "claude") {
			die("up: --automate cannot preserve this cell launch; use --cadence with cockpit terminals")
		}
	}

	// --set persists every override GIVEN, through the same one-backup path `cellctl set` uses:
	// CELL_KIND / CELL_COCKPIT / CELL_HARNESS / CELL_PROVIDER to their own keys, and --model to
	// the ACTIVE harness's <FAMILY>_MODEL_<role> for every role window this run opens — the
	// per-role keys, because that is exactly the set of windows the override applied to.
	var persistKVs []string
	if persist {
		if o.Cadence != "" {
			persistKVs = append(persistKVs, "CELL_CADENCE="+o.Cadence)
		}
		if o.TickBudget != "" {
			persistKVs = append(persistKVs, "CELL_TICK_BUDGET="+o.TickBudget)
		}
		effHarness := o.Harness
		if effHarness == "" {
			effHarness = c.Harness
		}
		if o.Model != "" {
			for _, r := range roles {
				k := "DESK_MODEL_" + underscore(r)
				if effHarness == "codex" {
					k = "CODEX_MODEL_" + underscore(r)
				} else if effHarness == "cursor" {
					k = "CURSOR_MODEL_" + underscore(r)
				}
				persistKVs = append(persistKVs, k+"="+o.Model)
			}
		}
		if o.Harness != "" {
			persistKVs = append(persistKVs, "CELL_HARNESS="+o.Harness)
		}
		if o.Provider != "" {
			persistKVs = append(persistKVs, "CELL_PROVIDER="+o.Provider)
		}
		if c.KindOverride != "" {
			persistKVs = append(persistKVs, "CELL_KIND="+c.KindOverride)
		}
		if cockpitFlag != "" {
			persistKVs = append(persistKVs, "CELL_COCKPIT="+cockpitFlag)
		}
	}

	// The reconcile classification (#2880) is read, never acted on, by a plain `up`: each role is
	// probed up / stale / absent from the same evidence the preflight uses, and stale state is
	// cleared only by an applied plan (`up --plan`, then `up --apply <plan-id>` on a terminal).
	var commsCfg *deskCommsConfig
	if c.Kind == "house" {
		var cfgErr error
		commsCfg, cfgErr = c.loadDeskComms()
		if cfgErr != nil {
			commsCfg = nil // commsPreflight below refuses the configuration in its own words
		}
	}
	// probe is the one place roles are classified; a non-house cell has no role lease to read.
	probe := func(rs []string) ([]upProbe, *reconcileEntry) {
		if c.Kind != "house" {
			out := make([]upProbe, len(rs))
			for i, r := range rs {
				out[i] = upProbe{Role: r, Kind: probeAbsent}
			}
			return out, nil
		}
		ps, plan := c.probeUpRoles(rs, commsCfg)
		return ps, commsStale(plan)
	}
	if ctl.applyID == "" {
		// An applied plan may be what clears the comms state, so its preflight runs after the apply.
		c.commsPreflightStale(commsCfg, c.Env.Get("DRY_RUN") == "1")
		o.Comms, err = c.commsPreflight()
		if err != nil {
			die("up comms: %v; no windows launched", err)
		}
	}
	firstName, firstCmd := c.firstWindow(paneShellFor(runtime.GOOS, res.Cockpit))
	if firstName == "" && len(roles) == 0 && automate == "" && !o.Comms {
		// Nothing to open is not an error (the operator asked for --no-the-desk on a cell whose only
		// role is the-desk), but it is never silent, and no empty session is created for it.
		fmt.Fprintf(os.Stderr, "[cell] %s: nothing to open — no deskd window (DESKD is not 1), no comms window, and no role window (ROLES is empty after --no-the-desk)\n", c.Name)
		ctl.emit(c, started0, nil)
		return
	}

	if c.Env.Get("DRY_RUN") == "1" && ctl.applyID == "" {
		kindShown := c.Kind
		if c.KindOverride != "" {
			kindShown += " (override)"
		}
		fmt.Printf("[dry-run] cell=%s kind=%s cockpit=%s session=%s config=%s\n", c.Name, kindShown, res.Cockpit, c.Session, cfg)
		if persist {
			for _, kv := range persistKVs {
				fmt.Printf("[dry-run] --set: would persist %s into %s/cell.env (not written — dry run)\n", kv, c.Dir)
			}
		}
		if o.Model != "" {
			fmt.Printf("[dry-run] model=%s (override) — applied to every role window below\n", o.Model)
		}
		if o.Harness != "" {
			fmt.Printf("[dry-run] harness=%s — applied to every role window below\n", o.Harness)
		}
		if o.Provider != "" {
			fmt.Printf("[dry-run] provider=%s (override) — applied to every role window below\n", o.Provider)
		}
		if o.Comms {
			fmt.Printf("[dry-run] comms: %s\n", c.commsCmdIn(paneShellFor(runtime.GOOS, res.Cockpit), selfPath()))
		}
		if firstName != "" {
			fmt.Printf("[dry-run] %s: %s\n", firstName, firstCmd)
		}
		if automate != "" {
			var rows []upRow
			for _, r := range roles {
				rows = append(rows, upRow{Role: r, State: upStateWouldStart, Detail: "orca automation on " + automate})
				fmt.Printf("[dry-run] %s: orca automations create --name %s-%s --repo path:%s --trigger '%s' --precheck '%s check %s' --provider claude --prompt /assay:%s\n",
					r, c.Name, r, c.Repo, automate, selfPath(), c.Name, r)
			}
			ctl.emit(c, started0, rows)
			return
		}
		// Each role's disposition comes from the probes a real run acts on. An absent role keeps
		// its line: the command its window would run.
		probes, _ := probe(roles)
		var rows []upRow
		for _, p := range probes {
			cmd := c.paneRoleCmd(p.Role, cfg, o)
			fmt.Println(c.upDryRunLine(p, cmd))
			rows = append(rows, upDryRunRow(c, p))
		}
		ctl.emit(c, started0, rows)
		return
	}
	if persist {
		applyEnvKVs(c.Env, c.Dir+"/cell.env", false, persistKVs)
	}
	if automate != "" {
		// Scheduled automations open no window and hold no lease: nothing to converge.
		if o.Comms {
			if err := c.startComms(res.Cockpit); err != nil {
				die("up comms: %v", err)
			}
		}
		c.upOrca(cfg, roles, nil, nil, wait, automate, firstCmd, o, nil)
		return
	}
	started := time.Now()
	// launch converges the targeted roles once: classify, then open the missing windows one at a
	// time. reconciled names the roles an applied plan just cleared.
	launch := func(reconciled map[string]bool) []upRow {
		rs := append([]string(nil), roles...)
		for _, r := range knownRolesOrdered(reconciled) {
			if !valueIn(r, rs) && r != "comms" {
				rs = append(rs, r)
			}
		}
		c.commsPreflightStale(commsCfg, false)
		probes, _ := probe(rs)
		for _, p := range probes {
			if p.Kind == probeAbsent {
				if _, err := c.prepareLaunch(); err != nil {
					die("up preparation: %v; no windows launched", err)
				}
				break
			}
		}
		if o.Comms && res.Cockpit != "tmux" {
			if err := c.startComms(res.Cockpit); err != nil {
				die("up comms: %v", err)
			}
		}
		report := func(rows []upRow) { printUpTable(os.Stdout, rows) }
		switch res.Cockpit {
		case "tmux":
			return c.upTmux(cfg, probes, reconciled, wait, attach, firstName, firstCmd, o, report)
		case "herdr":
			return c.upHerdr(cfg, probes, reconciled, wait, firstName, firstCmd, o, report)
		default:
			return c.upOrca(cfg, roles, probes, reconciled, wait, "", firstCmd, o, report)
		}
	}
	if ctl.applyID != "" {
		// Under DRY_RUN an apply still takes the terminal and the typed confirm, but the reconcile
		// effect clears nothing and no window is launched.
		dry := c.Env.Get("DRY_RUN") == "1"
		verb := upVerb(commsCfg, dry)
		verb.After = func(c *Cell, r *cellop.Result) {
			defer ctl.restore()
			if r.Outcome != cellop.OK || dry {
				return
			}
			var perr error
			if o.Comms, perr = c.commsPreflight(); perr != nil {
				die("up comms: %v; no windows launched", perr)
			}
			withRoles(r, launch(reconciledRoles(*r)))
		}
		cellopApply(ctl.cmd, c, verb, ctl.applyID)
		return
	}
	rows := launch(nil)
	if ctl.json {
		ctl.restore()
		r := upResult(c.Name, started, rows)
		emitResult(ctl.cmd, r)
		exitWith(r.ExitCode())
	}
	if !upRowsOK(rows) {
		exitWith(1)
	}
}

// commsPreflightStale refuses a launch over stale comms state. Plain `up` never clears it, on a
// terminal or off one: the inventory and the exact commands are printed (the reconcile preflight's
// own output) and `up --plan` is named as the way to clear it with a typed confirm. A dry run
// prints the inventory and changes nothing.
func (c *Cell) commsPreflightStale(comms *deskCommsConfig, dryRun bool) {
	if c.Kind != "house" || comms == nil {
		return
	}
	if err := c.reconcile(nil, comms, reconcileIO{DryRun: dryRun, Out: os.Stdout}); err != nil {
		fmt.Printf("[reconcile] cellctl up %s --plan writes one plan for the whole cell; cellctl up %s --apply <plan-id> runs it on a terminal\n", c.Name, c.Name)
		die("up: %v", err)
	}
}

// knownRolesOrdered lists the keys of set in the cell-role order.
func knownRolesOrdered(set map[string]bool) []string {
	var out []string
	for _, r := range knownRoles {
		if set[r] {
			out = append(out, r)
		}
	}
	return out
}

// roleGate is what a launch waits on before the next window opens: on a house cell the role's
// lease record (the launch took the lease and passed the scratch sweep, or failed); on a cell with
// no role lease, the fixed stagger the windows have always had, since each one fetches the shared
// .git and starting them at once puts every window into the fetch lock's retry loop together.
func (c *Cell) roleGate(wait time.Duration) func(string, time.Time) (string, error) {
	if c.Kind == "house" {
		return func(role string, since time.Time) (string, error) { return c.awaitRoleLease(role, wait, since) }
	}
	return func(string, time.Time) (string, error) { time.Sleep(upStagger); return "", nil }
}

// upStagger paces the windows of a cell that has no role lease to wait on.
var upStagger = 2 * time.Second

// upTmux is the always-works arm. A session that already exists is not re-created: only the windows
// of roles that are not running are added to it.
func (c *Cell) upTmux(cfg string, probes []upProbe, reconciled map[string]bool, wait time.Duration, attach bool, firstName, firstCmd string, o upOverrides, report func([]upRow)) []upRow {
	if !onPath("tmux") {
		die("tmux not installed")
	}
	exists := exec.Command("tmux", "has-session", "-t", c.Session).Run() == nil
	if exists {
		fmt.Printf("[cell] %s already running — adding the windows of roles that are not up\n", c.Session)
	} else if firstName != "" {
		// The order is fixed — the deskd window, then comms, then the roles — and whichever window
		// comes first creates the session. A cell with no deskd gets no placeholder pane to hold the
		// session open: comms creates it when configured (openCommsSurface), else the first role window.
		_ = exec.Command("tmux", "new-session", "-d", "-s", c.Session, "-n", firstName, "-c", c.Dir,
			firstCmd+"; echo '["+firstName+"] exited'; exec $SHELL").Run()
	}
	if o.Comms {
		if err := c.startComms("tmux"); err != nil {
			die("up comms: %v", err)
		}
	}
	open := func(role string) error {
		name := strings.TrimSuffix(role, "-desk")
		switch role {
		case "pr-review-desk":
			name = "review"
		case "the-desk":
			name = "the-desk"
		}
		verb := []string{"new-window", "-t", c.Session}
		if exec.Command("tmux", "has-session", "-t", c.Session).Run() != nil {
			verb = []string{"new-session", "-d", "-s", c.Session}
		}
		out, err := exec.Command("tmux", append(verb, "-n", name, "-c", c.Dir,
			c.paneRoleCmd(role, cfg, o)+"; echo '["+role+"] exited'; exec $SHELL")...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tmux %s: %s", verb[0], orDefault(firstOutLine(string(out)), err.Error()))
		}
		return nil
	}
	rows := c.convergeRoles(probes, reconciled, c.roleGate(wait), open)
	_ = exec.Command("tmux", "select-window", "-t", c.Session+":the-desk").Run()
	report(rows)
	fmt.Printf("[cell] %s up: %s (config=%s)\n", c.Session, upWindowList(firstName, "", upRoleNames(probes)), cfg)
	fmt.Printf("[cell] attach: tmux attach -t %s   ·   down: cellctl down %s\n", c.Session, c.Name)
	if attach {
		runForeground([]string{"tmux", "attach", "-t", c.Session}, os.Environ(), "")
	}
	return rows
}

func upRoleNames(probes []upProbe) []string {
	out := make([]string, len(probes))
	for i, p := range probes {
		out[i] = p.Role
	}
	return out
}

// upHerdr opens one LABELLED tab per window running the role's full command line — the same
// string the tmux arm hands `tmux new-window` — so every cockpit runs identically.
//
// `herdr agent start` is NOT a "run this shell command" verb: its AGENT_ARG list is appended
// directly to the KIND's canonical executable, so it cannot host cellctl's composite command
// (fetch the shared repo, cut/fast-forward the worktree, generate shims, cd, THEN exec the
// harness). `herdr pane run <PANE_ID> <COMMAND>` is what actually hosts it.
func (c *Cell) upHerdr(cfg string, probes []upProbe, reconciled map[string]bool, wait time.Duration, firstName, firstCmd string, o upOverrides, report func([]upRow)) []upRow {
	if !helpHas("create", "herdr", "tab") {
		die("herdr is the resolved cockpit but this build's 'herdr tab --help' advertises no 'create' — nothing can host a role window; re-run with --cockpit tmux")
	}
	if !helpHas("run", "herdr", "pane") {
		die("herdr is the resolved cockpit but this build's 'herdr pane --help' advertises no 'run' — nothing can execute the role command in a tab; re-run with --cockpit tmux")
	}
	if !helpHas("list", "herdr", "workspace") {
		die("herdr is the resolved cockpit but this build's 'herdr workspace --help' advertises no 'list' — cannot tell whether a herdr window is already open to host the role tabs; re-run with --cockpit tmux")
	}
	w, err := c.ensureHerdrWorkspace()
	if err != nil {
		die("herdr workspace: %v", err)
	}
	newWsID, newDefaultTab := w.ID, w.DefaultTab
	if firstName != "" {
		c.herdrWindow(c.Name+"-"+firstName, firstCmd, newWsID)
	}
	rows := c.convergeRoles(probes, reconciled, c.roleGate(wait), func(role string) error {
		return c.herdrWindow(c.Name+"-"+role, c.paneRoleCmd(role, cfg, o), newWsID)
	})
	// The freshly-created workspace's own default tab is never wanted — dropped now that the
	// workspace holds the cell's own labelled tabs too. Closing it any earlier (before the
	// workspace holds a second tab) closes the whole workspace with it.
	if newDefaultTab != "" {
		if err := c.dropHerdrDefault(w); err != nil {
			die("Herdr default tab: %v", err)
		}
	}
	report(rows)
	fmt.Printf("[cell] %s up in herdr: %s (config=%s)\n", c.Name, upWindowList(firstName, c.Name+"-", upRoleNames(probes)), cfg)
	fmt.Printf("[cell] each window is the tab labelled <cell>-<role>   ·   down: cellctl down %s\n", c.Name)
	return rows
}

func (c *Cell) herdrWindow(label, cmd, wsID string) error {
	args := []string{"tab", "create", "--label", label}
	if wsID != "" {
		args = []string{"tab", "create", "--workspace", wsID, "--label", label}
	}
	out, err := exec.Command("herdr", args...).CombinedOutput()
	paneID := ""
	if err == nil {
		var d struct {
			Result struct {
				RootPane struct {
					PaneID string `json:"pane_id"`
				} `json:"root_pane"`
			} `json:"result"`
		}
		_ = json.Unmarshal(out, &d)
		paneID = d.Result.RootPane.PaneID
	}
	if paneID == "" {
		fmt.Fprintf(os.Stderr, "NOTICE: 'herdr tab create --label %s' did not return a pane id — start it by hand: %s\n", label, cmd)
		return fmt.Errorf("herdr tab create --label %s returned no pane id", label)
	}
	if exec.Command("herdr", "pane", "run", paneID, cmd).Run() != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: 'herdr pane run' failed for %s (pane %s) — start it by hand: %s\n", label, paneID, cmd)
		return fmt.Errorf("herdr pane run failed for %s", label)
	}
	return nil
}

// upOrca has two shapes. Live: one terminal per role under the cell's checkout, running the same
// `cellctl desk` command every other cockpit runs. Scheduled (--automate): one automation per
// role on the given trigger, fronted by an exit-code precheck — `cellctl check <cell>`, which is
// exit-code honest — so a tick on a cell that is not fit to boot launches no model at all.
//
// Orca's CLI moves fast and its create-a-terminal surface is not the same on every build, so the
// verb and its flags are PROBED from --help and an absent one is reported with the exact command
// to run by hand — never guessed at.
func (c *Cell) upOrca(cfg string, roles []string, probes []upProbe, reconciled map[string]bool, wait time.Duration, automate, firstCmd string, o upOverrides, report func([]upRow)) []upRow {
	if automate != "" {
		c.orcaAutomate(roles, automate)
		return nil
	}
	verb, ok := orcaTerminalVerb()
	if !ok {
		c.orcaByHand("'orca terminal --help' advertises no create-like subcommand on this build", cfg, roles, o)
	}
	h := helpText("orca", "terminal", verb)
	cmdflag, ok := firstFlag(h, "--command", "--cmd", "--exec", "--run")
	if !ok {
		c.orcaByHand(fmt.Sprintf("'orca terminal %s --help' advertises no flag that carries a command", verb), cfg, roles, o)
	}
	// Orca registers GIT REPOSITORIES only. The cell directory is config, not a checkout, so the
	// cell's own checkout is what gets registered and selected; the role commands are absolute
	// and carry the cell name themselves.
	orcaDir := orDefault(c.Repo, c.Dir)
	wtflag, hasWT := firstFlag(h, "--worktree")
	cwdflag, hasCwd := "", false
	if hasWT {
		if helpHas("add", "orca", "repo") {
			if exec.Command("orca", "repo", "add", "--path", orcaDir).Run() != nil {
				fmt.Fprintf(os.Stderr, "NOTICE: 'orca repo add --path %s' failed — terminal create below may 404 with selector_not_found until %s is registered by hand ('orca repo add --path %s')\n", orcaDir, orcaDir, orcaDir)
			}
		}
	} else {
		cwdflag, hasCwd = firstFlag(h, "--cwd", "--path", "--directory")
	}
	nameflag, hasName := firstFlag(h, "--name", "--title", "--label")
	if !hasWT && !hasCwd {
		fmt.Fprintf(os.Stderr, "NOTICE: 'orca terminal %s' advertises no working-directory flag — the terminals open wherever orca puts them; the command itself is absolute either way\n", verb)
	}
	rows := c.convergeRoles(probes, reconciled, c.roleGate(wait), func(role string) error {
		args := []string{"terminal", verb}
		switch {
		case hasWT:
			args = append(args, wtflag, "path:"+orcaDir)
		case hasCwd:
			args = append(args, cwdflag, orcaDir)
		}
		if hasName {
			args = append(args, nameflag, c.Name+"-"+role)
		}
		args = append(args, cmdflag, c.paneRoleCmd(role, cfg, o))
		if err := exec.Command("orca", args...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "NOTICE: 'orca %s' failed — run it by hand: %s\n", strings.Join(args, " "), c.paneRoleCmd(role, cfg, o))
			return fmt.Errorf("orca terminal %s failed: %v", verb, err)
		}
		return nil
	})
	report(rows)
	fmt.Printf("[cell] %s up in orca: one terminal per role (%s), config=%s\n", c.Name, strings.Join(roles, " "), cfg)
	if firstCmd != "" {
		fmt.Printf("[cell] the first window's command is not opened in orca — run it yourself if you want it: %s\n", firstCmd)
	}
	fmt.Printf("[cell] down: cellctl down %s\n", c.Name)
	return rows
}

// upWindowList is the summary line's window list: "<prefix><first> +<roles>" when there is a
// first (deskd) window, else the role windows alone — a cell with no deskd names no first window.
func upWindowList(firstName, prefix string, roles []string) string {
	if firstName == "" {
		return strings.Join(roles, " ")
	}
	return prefix + firstName + " +" + strings.Join(roles, " ")
}

func orcaTerminalVerb() (string, bool) {
	for _, v := range []string{"create", "new", "open", "start", "run"} {
		if helpHas(v, "orca", "terminal") {
			return v, true
		}
	}
	return "", false
}

func (c *Cell) orcaByHand(why, cfg string, roles []string, o upOverrides) {
	fmt.Fprintf(os.Stderr, "NOTICE: %s\n", why)
	fmt.Fprintf(os.Stderr, "NOTICE: open these by hand (one terminal each, in %s):\n", c.Dir)
	for _, role := range roles {
		fmt.Fprintf(os.Stderr, "  %s\n", c.paneRoleCmd(role, cfg, o))
	}
	die("orca is the resolved cockpit but this build cannot be driven to open the windows — the commands above are the whole of what a cockpit runs, and --cockpit tmux opens them for you")
}

func (c *Cell) orcaAutomate(roles []string, trigger string) {
	h := helpText("orca", "automations", "create")
	for _, f := range []string{"--name", "--repo", "--trigger", "--precheck", "--prompt"} {
		if _, ok := firstFlag(h, f); !ok {
			die("orca --automate needs '%s' on 'orca automations create', and this build's help does not advertise it — the precheck-fronted schedule is the whole point of the shape, so it is not approximated", f)
		}
	}
	provider, hasProvider := firstFlag(h, "--provider", "--agent")
	fmt.Fprintln(os.Stderr, "NOTICE: a scheduled run is launched by orca, not by this script — it runs the role skill in a worktree orca cuts, so it does NOT carry the cell's shim PATH, DESK_ROOTS or pinned model. Use the live-terminal shape (drop --automate) where those matter.")
	for _, role := range roles {
		args := []string{"automations", "create", "--name", c.Name + "-" + role, "--repo", "path:" + c.Repo,
			"--trigger", trigger, "--precheck", selfPath() + " check " + c.Name, "--prompt", "/assay:" + role}
		if hasProvider {
			args = append(args, provider, "claude")
		}
		if exec.Command("orca", args...).Run() != nil {
			fmt.Fprintf(os.Stderr, "NOTICE: 'orca %s' failed — create that automation by hand\n", strings.Join(args, " "))
		}
	}
	fmt.Printf("[cell] %s scheduled in orca: one automation per role (%s) on '%s', precheck '%s check %s'\n",
		c.Name, strings.Join(roles, " "), trigger, selfPath(), c.Name)
	fmt.Println("[cell] a precheck that exits non-zero records a skipped run and launches no model")
}
