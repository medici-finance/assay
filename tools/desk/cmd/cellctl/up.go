package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// upOverrides are the values `role_cmd` threads onto every role's own `cellctl desk`
// invocation. They are GLOBAL to one `up` run rather than extra parameters through every cockpit
// arm, exactly as the oracle keeps them, and they apply to EVERY role window this run opens —
// there is deliberately no per-role `--model-<role>` form, because that case is already
// `cellctl desk <cell> <role> --model <m>` on the one window that needs it.
type upOverrides struct {
	Model    string
	Harness  string
	Provider string
	// Cockpit is the cockpit `up` RESOLVED (never `auto`), threaded onto every window so each
	// exports the same ASSAY_COCKPIT as the surface it was opened in — even when `up --cockpit`
	// overrode cell.env for this run only.
	Cockpit string
}

// roleCmd is the one command a role window runs, identical in every cockpit. Order is fixed
// (--model, then --harness, then --provider, then --cockpit) so a printed command is stable to
// grep against.
func (c *Cell) roleCmd(role, cfg string, o upOverrides) string {
	out := fmt.Sprintf("'%s' desk '%s' '%s'", selfPath(), c.Name, role)
	if o.Model != "" {
		out += " --model '" + o.Model + "'"
	}
	if o.Harness != "" {
		out += " --harness '" + o.Harness + "'"
	}
	if o.Provider != "" {
		out += " --provider '" + o.Provider + "'"
	}
	if o.Cockpit != "" {
		out += " --cockpit '" + o.Cockpit + "'"
	}
	return out + " '" + cfg + "'"
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

// firstWindow decides the window that is NOT a role: the deskd watcher, the deskd itself, or (on
// a cell with no deskd) a line saying so.
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
func (c *Cell) firstWindow() (name, cmd string) {
	switch {
	case c.Deskd != "1":
		return "cell", fmt.Sprintf("echo '[cell] %s is a %s cell with no deskd (DESKD=1 in cell.env to stand one)'", c.Name, c.Kind)
	case c.deskdUp():
		return "deskd", fmt.Sprintf("echo '[deskd] already up on %s — watching /healthz every 60s'; while :; do date -u +%%H:%%MZ; curl -s --max-time 5 http://%s/healthz | head -c 240; echo; sleep 60; done", c.DeskdAddr, c.DeskdAddr)
	case isTTY(os.Stdin) || c.Env.GetOr("CELL_ATTENDED", "0") == "1":
		fmt.Printf("[cell] deskd is not up — standing it in-session, carrying YOUR attended affirmation (cellctl up ran attended)\n")
		return "deskd", fmt.Sprintf("CELL_ATTENDED=1 '%s' deskd '%s'", selfPath(), c.Name)
	default:
		fmt.Fprintln(os.Stderr, "[cell] deskd is not up, and this 'cellctl up' is UNATTENDED (no terminal on stdin, CELL_ATTENDED unset).")
		fmt.Fprintln(os.Stderr, "[cell] role windows will open; deskd will NOT be stood, because minting tokens needs an affirmation this invocation cannot make.")
		fmt.Fprintf(os.Stderr, "[cell] stand it yourself: CELL_ATTENDED=1 cellctl deskd %s\n", c.Name)
		return "deskd", fmt.Sprintf("echo '[deskd] NOT stood — cellctl up ran unattended. Run: CELL_ATTENDED=1 cellctl deskd %s'", c.Name)
	}
}

func cmdUp(cell string, args []string) {
	kindOverride := prescanKindOverride(args)
	c := loadCellWithKind(cell, kindOverride)

	if c.Kind == "container" {
		if strings.Join(c.Roles, " ") != "the-desk" {
			die("container up currently requires ROLES=the-desk; use desk for explicit roles")
		}
		cmdDesk(c.Name, append([]string{"the-desk"}, args...))
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
		cmdDesk(c.Name, append([]string{first}, args...))
		return
	}

	noTheDesk, attach, persist := false, true, false
	cfgIn, cockpitFlag, automate := "", "", ""
	var o upOverrides
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--no-the-desk":
			noTheDesk = true
		case "--with-the-desk":
		case "--no-attach":
			attach = false
		case "--cockpit":
			cockpitFlag = needFlagValue(args, &i, "--cockpit needs a value (auto|tmux|herdr|orca)")
		case "--automate":
			automate = needFlagValue(args, &i, "--automate needs a trigger (a 5-field cron string or a preset)")
		case "--model":
			o.Model = needFlagValue(args, &i, "--model needs a value")
		case "--provider":
			o.Provider = needFlagValue(args, &i, "--provider needs a value (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)")
		case "--harness":
			o.Harness = needFlagValue(args, &i, "--harness needs a value (claude|codex)")
		case "--kind":
			i++ // consumed by prescanKindOverride above
		case "--set":
			persist = true
		default:
			if strings.HasPrefix(a, "--") {
				die("up: unknown flag %s", a)
			}
			cfgIn = a
		}
	}
	if o.Harness != "" && !valueIn(o.Harness, harnessValues) {
		die("up: --harness must be claude or codex, got '%s'", o.Harness)
	}
	if cockpitFlag != "" && !valueIn(cockpitFlag, cockpitValues) {
		die("up: --cockpit must be one of %s, got '%s'", joinPipe(cockpitValues), cockpitFlag)
	}
	if persist && !anyGiven(o.Model, o.Harness, o.Provider, c.KindOverride, cockpitFlag) {
		die("up: --set needs --model, --harness, --provider, --kind or --cockpit — nothing to persist otherwise")
	}
	cfg := resolveCfg(c.Env, cfgIn)
	want, src := c.cockpitWant(cockpitFlag)
	res := c.resolveCockpit(want, src)
	if res.Err != "" {
		die("%s", res.Err)
	}
	fmt.Printf("[cockpit] %s (%s)\n", res.Cockpit, res.Why)
	o.Cockpit = res.Cockpit
	if automate != "" && res.Cockpit != "orca" {
		die("up: --automate is an orca-only shape (scheduled automations behind an exit-code precheck); the resolved cockpit is %s", res.Cockpit)
	}
	roles := c.upRoles(noTheDesk)

	policy, policySource, err := c.cellModelPolicy()
	if err != nil {
		die("%s", err)
	}
	if policy != nil {
		if automate != "" {
			die("up: --automate cannot propagate model policy; use live desk windows")
		}
		if persist {
			die("up: --set with a model policy is ambiguous; edit the policy or provider defaults instead")
		}
		for _, role := range roles {
			route, err := policy.Resolve(role, o.Provider, o.Model, o.Harness)
			if err != nil {
				die("%s", err)
			}
			fmt.Printf("[policy] role=%s provider=%s model=%s effort=%s source=%s sha256=%s\n", role, route.Provider, route.Model, route.Effort, policySource, policy.SHA256)
		}
	}

	// --set persists every override GIVEN, through the same one-backup path `cellctl set` uses:
	// CELL_KIND / CELL_COCKPIT / CELL_HARNESS / CELL_PROVIDER to their own keys, and --model to
	// the ACTIVE harness's <FAMILY>_MODEL_<role> for every role window this run opens — the
	// per-role keys, because that is exactly the set of windows the override applied to.
	var persistKVs []string
	if persist {
		effHarness := o.Harness
		if effHarness == "" {
			effHarness = c.Harness
		}
		if o.Model != "" {
			for _, r := range roles {
				k := "DESK_MODEL_" + underscore(r)
				if effHarness == "codex" {
					k = "CODEX_MODEL_" + underscore(r)
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

	firstName, firstCmd := c.firstWindow()

	if c.Env.Get("DRY_RUN") == "1" {
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
		fmt.Printf("[dry-run] %s: %s\n", firstName, firstCmd)
		for _, r := range roles {
			if automate != "" {
				fmt.Printf("[dry-run] %s: orca automations create --name %s-%s --repo path:%s --trigger '%s' --precheck '%s check %s' --provider claude --prompt /assay:%s\n",
					r, c.Name, r, c.Repo, automate, selfPath(), c.Name, r)
			} else {
				fmt.Printf("[dry-run] %s: %s\n", r, c.roleCmd(r, cfg, o))
			}
		}
		return
	}
	if persist {
		applyEnvKVs(c.Env, c.Dir+"/cell.env", false, persistKVs)
	}
	switch res.Cockpit {
	case "tmux":
		c.upTmux(cfg, roles, attach, firstName, firstCmd, o)
	case "herdr":
		c.upHerdr(cfg, roles, firstName, firstCmd, o)
	case "orca":
		c.upOrca(cfg, roles, automate, firstCmd, o)
	}
}

// upTmux is the always-works arm.
func (c *Cell) upTmux(cfg string, roles []string, attach bool, firstName, firstCmd string, o upOverrides) {
	if !onPath("tmux") {
		die("tmux not installed")
	}
	if exec.Command("tmux", "has-session", "-t", c.Session).Run() == nil {
		fmt.Printf("[cell] %s already running — attaching\n", c.Session)
		if attach {
			runForeground([]string{"tmux", "attach", "-t", c.Session}, os.Environ(), "")
		}
		return
	}
	_ = exec.Command("tmux", "new-session", "-d", "-s", c.Session, "-n", firstName, "-c", c.Dir,
		firstCmd+"; echo '["+firstName+"] exited'; exec $SHELL").Run()
	for _, role := range roles {
		name := strings.TrimSuffix(role, "-desk")
		switch role {
		case "pr-review-desk":
			name = "review"
		case "the-desk":
			name = "the-desk"
		}
		_ = exec.Command("tmux", "new-window", "-t", c.Session, "-n", name, "-c", c.Dir,
			c.roleCmd(role, cfg, o)+"; echo '["+role+"] exited'; exec $SHELL").Run()
		// Stagger the windows: each one fetches the shared .git, and starting them at once puts
		// every window into the fetch lock's retry loop at the same moment.
		time.Sleep(2 * time.Second)
	}
	_ = exec.Command("tmux", "select-window", "-t", c.Session+":the-desk").Run()
	fmt.Printf("[cell] %s up: %s +%s (config=%s)\n", c.Session, firstName, strings.Join(roles, " "), cfg)
	fmt.Printf("[cell] attach: tmux attach -t %s   ·   down: cellctl down %s\n", c.Session, c.Name)
	if attach {
		runForeground([]string{"tmux", "attach", "-t", c.Session}, os.Environ(), "")
	}
}

// upHerdr opens one LABELLED tab per window running the role's full command line — the same
// string the tmux arm hands `tmux new-window` — so every cockpit runs identically.
//
// `herdr agent start` is NOT a "run this shell command" verb: its AGENT_ARG list is appended
// directly to the KIND's canonical executable, so it cannot host cellctl's composite command
// (fetch the shared repo, cut/fast-forward the worktree, generate shims, cd, THEN exec the
// harness). `herdr pane run <PANE_ID> <COMMAND>` is what actually hosts it.
func (c *Cell) upHerdr(cfg string, roles []string, firstName, firstCmd string, o upOverrides) {
	if !helpHas("create", "herdr", "tab") {
		die("herdr is the resolved cockpit but this build's 'herdr tab --help' advertises no 'create' — nothing can host a role window; re-run with --cockpit tmux")
	}
	if !helpHas("run", "herdr", "pane") {
		die("herdr is the resolved cockpit but this build's 'herdr pane --help' advertises no 'run' — nothing can execute the role command in a tab; re-run with --cockpit tmux")
	}
	if !helpHas("list", "herdr", "workspace") {
		die("herdr is the resolved cockpit but this build's 'herdr workspace --help' advertises no 'list' — cannot tell whether a herdr window is already open to host the role tabs; re-run with --cockpit tmux")
	}
	newWsID, newDefaultTab := "", ""
	if herdrWindowID() == "" {
		newWsID, newDefaultTab = c.herdrStartWindow(c.Name + "-" + firstName)
		fmt.Println("[cell] no herdr window was open — started one")
	}
	c.herdrWindow(c.Name+"-"+firstName, firstCmd, newWsID)
	for _, role := range roles {
		c.herdrWindow(c.Name+"-"+role, c.roleCmd(role, cfg, o), newWsID)
		time.Sleep(2 * time.Second)
	}
	// The freshly-created workspace's own default tab is never wanted — dropped now that the
	// workspace holds the cell's own labelled tabs too. Closing it any earlier (before the
	// workspace holds a second tab) closes the whole workspace with it.
	if newDefaultTab != "" {
		_ = exec.Command("herdr", "tab", "close", newDefaultTab).Run()
	}
	fmt.Printf("[cell] %s up in herdr: %s-%s +%s (config=%s)\n", c.Name, c.Name, firstName, strings.Join(roles, " "), cfg)
	fmt.Printf("[cell] each window is the tab labelled <cell>-<role>   ·   down: cellctl down %s\n", c.Name)
}

func herdrWindowID() string {
	out, err := exec.Command("herdr", "workspace", "list").Output()
	if err != nil {
		return ""
	}
	var d struct {
		Result struct {
			Workspaces []struct {
				WorkspaceID string `json:"workspace_id"`
			} `json:"workspaces"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &d) != nil || len(d.Result.Workspaces) == 0 {
		return ""
	}
	return d.Result.Workspaces[0].WorkspaceID
}

// herdrStartWindow brings up a herdr window (a "workspace") when none was open — the same
// create-if-absent shape as the tmux arm's `has-session || new-session`. It FAILS CLOSED,
// naming herdr and the exact command tried, rather than leaving the tabs nowhere to land.
func (c *Cell) herdrStartWindow(label string) (wsID, defaultTab string) {
	if !helpHas("create", "herdr", "workspace") {
		die("herdr is the resolved cockpit but no herdr window is open and this build's 'herdr workspace --help' advertises no 'create' — nothing can bring one up to host the role tabs (tried: herdr workspace create --label %s); re-run with --cockpit tmux", label)
	}
	out, err := exec.Command("herdr", "workspace", "create", "--label", label).CombinedOutput()
	if err != nil {
		die("herdr is the resolved cockpit but no herdr window is open and 'herdr workspace create --label %s' failed to bring one up: %s; re-run with --cockpit tmux", label, strings.TrimSpace(string(out)))
	}
	var d struct {
		Result struct {
			Workspace struct {
				WorkspaceID string `json:"workspace_id"`
			} `json:"workspace"`
			Tab struct {
				TabID string `json:"tab_id"`
			} `json:"tab"`
		} `json:"result"`
	}
	_ = json.Unmarshal(out, &d)
	if d.Result.Workspace.WorkspaceID == "" {
		die("herdr is the resolved cockpit but 'herdr workspace create --label %s' did not return a workspace_id — nothing can host the role tabs: %s; re-run with --cockpit tmux", label, strings.TrimSpace(string(out)))
	}
	return d.Result.Workspace.WorkspaceID, d.Result.Tab.TabID
}

func (c *Cell) herdrWindow(label, cmd, wsID string) {
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
		return
	}
	if exec.Command("herdr", "pane", "run", paneID, cmd).Run() != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: 'herdr pane run' failed for %s (pane %s) — start it by hand: %s\n", label, paneID, cmd)
	}
}

// upOrca has two shapes. Live: one terminal per role under the cell's checkout, running the same
// `cellctl desk` command every other cockpit runs. Scheduled (--automate): one automation per
// role on the given trigger, fronted by an exit-code precheck — `cellctl check <cell>`, which is
// exit-code honest — so a tick on a cell that is not fit to boot launches no model at all.
//
// Orca's CLI moves fast and its create-a-terminal surface is not the same on every build, so the
// verb and its flags are PROBED from --help and an absent one is reported with the exact command
// to run by hand — never guessed at.
func (c *Cell) upOrca(cfg string, roles []string, automate, firstCmd string, o upOverrides) {
	if automate != "" {
		c.orcaAutomate(roles, automate)
		return
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
	for _, role := range roles {
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
		args = append(args, cmdflag, c.roleCmd(role, cfg, o))
		if exec.Command("orca", args...).Run() != nil {
			fmt.Fprintf(os.Stderr, "NOTICE: 'orca %s' failed — run it by hand: %s\n", strings.Join(args, " "), c.roleCmd(role, cfg, o))
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("[cell] %s up in orca: one terminal per role (%s), config=%s\n", c.Name, strings.Join(roles, " "), cfg)
	fmt.Printf("[cell] the first window's command is not opened in orca — run it yourself if you want it: %s\n", firstCmd)
	fmt.Printf("[cell] down: cellctl down %s\n", c.Name)
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
		fmt.Fprintf(os.Stderr, "  %s\n", c.roleCmd(role, cfg, o))
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
