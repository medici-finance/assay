package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// The command tree (desktools-v2/16). Cobra owns argv: flag spelling, help, version and parse
// errors. Viper (through internal/cli Declare/Resolve) owns the resolution of every declared
// setting from its allowed sources. Neither owns an ADMISSION decision: every refusal that grants
// or withholds authority — a kind the cell is not provisioned for, the Opus rule, a policy that
// forbids --set, a harness outside the closed set, a container cell's host-only flags — stays in
// the domain code the handlers call (desk.go, up.go, down.go, cell.go, policy*.go), which runs
// the same checks whoever built the typed options. A parser that selects the wrong option can
// therefore be refused below it, and cobra_test.go proves that with the adapter bypassed.
//
// Handlers still end by panicking an exitCode (die / exitWith), recovered once in runTree, so the
// deferred lock and lease cleanup under them keeps running exactly as before.

//go:embed help/concepts.txt
var helpConcepts string

// versionLine is what --version and `version` print.
func versionLine() string { return versionString(cellctlVersion, readBuildInfo) }

// hookVerb reports whether args (after a leading --cells-root selector) name the runtime
// model-policy hook, whose every non-zero exit must be the BLOCKING status.
func hookVerb(args []string) bool {
	a := commandArgs(args)
	return len(a) > 0 && a[0] == "model-policy"
}

// rawVerb reports the internal entrypoints whose arguments are an opaque argv, never flags.
func rawVerb(args []string) bool {
	a := commandArgs(args)
	if len(a) == 0 {
		return false
	}
	switch a[0] {
	case "model-policy", "cache-run", "container-run":
		return true
	}
	return false
}

// runTree executes one invocation against a fresh tree and returns its exit code. It is the
// only place that turns the exit panic into a code.
func runTree(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			ec, ok := r.(exitCode)
			if !ok {
				panic(r)
			}
			code = ec.code
		}
		// Fail closed. A parse failure of the hook's own argv exits with the usage code (3),
		// which Claude Code treats as a NON-blocking hook error and lets the action through.
		// Whatever stopped the hook, a non-zero result is the blocking status.
		if code != 0 && hookVerb(args) {
			code = hookBlockExit
		}
	}()
	return cli.Run(buildRoot, args, cli.Options{
		IO:              cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		UsageExit:       3,
		GoFlagCompat:    !rawVerb(args),
	})
}

// selectCellsRoot applies the --cells-root selector. The legacy parser accepted it only as the
// first token; the persistent flag accepts it anywhere, with the same absolute-path refusal.
func selectCellsRoot(path string) {
	if !filepath.IsAbs(path) {
		die("--cells-root requires an absolute registry path and a command")
	}
	if err := os.Setenv("CELLS_ROOT", path); err != nil {
		die("cannot select cell registry: %v", err)
	}
}

// rawArgs strips a leading --cells-root <abs> from a flag-less command's argv (Cobra does not
// parse flags for it) and applies it; everything else is the command's own argv, untouched.
func rawArgs(args []string) []string {
	if len(args) > 0 && args[0] == "--cells-root" {
		if len(args) < 3 {
			die("--cells-root requires an absolute registry path and a command")
		}
		selectCellsRoot(args[1])
		return args[2:]
	}
	return args
}

const noEcho = "cellctl/no-roster-echo"

// bStr, bBool and friends keep the declarations below to one line each.
func bStr(key, usage string, env ...string) cli.Binding {
	return cli.Binding{Key: key, Kind: cli.String, Flag: key, Usage: usage, Env: env}
}
func bBool(key, usage string) cli.Binding {
	return cli.Binding{Key: key, Kind: cli.Bool, Flag: key, Usage: usage}
}
func bDef(key, usage, def string) cli.Binding {
	return cli.Binding{Key: key, Kind: cli.String, Flag: key, Usage: usage, Default: def}
}

// procEnv looks a variable up in the process environment; an empty value is unset, which is how
// every legacy reader (c.Env.Get, os.Getenv) already treated it.
func procEnv(k string) (string, bool) {
	v := os.Getenv(k)
	return v, v != ""
}

// resolve reads the command's declared settings. env is the layer the Env source reads: the
// loaded cell's environment (process env with cell.env overlaid) when there is a cell, else the
// process environment.
func resolve(set *cli.Set, c *Cell) *cli.Values {
	look := procEnv
	if c != nil {
		look = func(k string) (string, bool) { v := c.Env.Get(k); return v, v != "" }
	}
	v, err := set.Resolve(cli.Inputs{LookupEnv: look})
	if err != nil {
		die("%v", err)
	}
	return v
}

// flagOnly is the value a setting took from the command line, "" otherwise: the form `up`
// threads onto each role window, where the cell.env pin is re-read by that window's own desk.
func flagOnly(v *cli.Values, key string) string {
	if v.Source(key) == cli.Flag {
		return v.String(key)
	}
	return ""
}

// needValue preserves the legacy refusal of a flag given an EMPTY value.
func needValue(v *cli.Values, key, msg string) {
	if v.Source(key) == cli.Flag && v.String(key) == "" {
		die("%s", msg)
	}
}

// kindFlag is the --kind override of a verb, validated before anything loads: loadCell is where
// the kind's own preconditions are asserted, so the override has to be in force by then.
func kindFlag(cmd *cobra.Command) string {
	f := cmd.Flags().Lookup("kind")
	if f == nil || !f.Changed {
		return ""
	}
	k := f.Value.String()
	if k == "" {
		die("--kind needs a value (%s)", joinPipe(kindValues))
	}
	if !valueIn(k, kindValues) {
		die("--kind must be one of %s, got '%s'", joinPipe(kindValues), k)
	}
	return k
}

func cellArg(pos []string) string { return needCell(pos) }

// echoRoster is P3: the effective roster, once per run, before anything that acts. It is a
// pre-run hook rather than a line in main so help and --version stay pure.
var echoRoster = func() { echoEffectiveConfig() }

// declared records every binding of the tree being built, so a test can prove which sources the
// contract opens (no config-map source, no undeclared environment name). buildRoot resets it.
var declared []cli.Binding

func declare(cmd *cobra.Command, bs ...cli.Binding) *cli.Set {
	declared = append(declared, bs...)
	return cli.Declare(cmd, bs...)
}

// buildRoot builds one fresh tree. Nothing here reads the environment, a file or a cell.
func buildRoot() *cobra.Command {
	declared = nil
	root := cli.NewRoot("cellctl", "start, stop and scaffold an Assay CELL on one laptop")
	root.Long = "cellctl starts, stops and scaffolds an Assay CELL on one laptop (the laptop route).\n\n" +
		"Every command takes the cell as its first argument.\n\n" + helpConcepts
	root.Example = "  cellctl ls\n  cellctl new mycell --kind house --repo ~/src/repo --roots 'o/r=/abs/path'\n  cellctl up mycell\n  cellctl --cells-root /abs/registry show mycell"
	declare(root, cli.Binding{Key: "cells-root", Kind: cli.String, Flag: "cells-root", Persistent: true,
		Usage: "absolute path of the cell registry to use for this run (overrides CELLS_ROOT)"})
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations[noEcho] == "1" {
			return nil
		}
		echoRoster()
		if f := cmd.Flag("cells-root"); f != nil && f.Changed {
			selectCellsRoot(f.Value.String())
		}
		return nil
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if f := cmd.Flag("cells-root"); f != nil && f.Changed {
			die("--cells-root requires an absolute registry path and a command")
		}
		return cmd.Help()
	}
	root.AddCommand(
		versionCmd(), lsCmd(), newCmd(), setCmd(), showCmd(), checkCmd(), deskdCmd(), deskCmd(), upCmd(), downCmd(),
		smokeCmd(), statusCmd(), cadenceCmd(), scratchCmd(), cacheCmd(), commsCmd(),
		providersCmd(), modelPolicyCmd(), cacheRunCmd(), containerRunCmd(),
	)
	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       "print the release tag this copy ships at",
		Long:        "Print the release tag this copy ships at (\"dev-<commit>\" for a source checkout), the same contract\n`statusgen --version` uses, so a stale copy is detectable.",
		Annotations: map[string]string{noEcho: "1"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(versionLine())
			return nil
		},
	}
}

func lsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "list the cells under the registry",
		Long:  "List the cells under CELLS_ROOT. No cells yet is a legitimate state: nothing is printed and the exit is 0.",
		Args:  cobra.ArbitraryArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { cmdLs(); return nil },
	}
}

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <cell> [CLAUDE_CONFIG_DIR]",
		Short: "audit the preconditions of a cell",
		Long: "Preconditions per the cell's kind and forge (cell.env, roster, keys or token store, binaries, repo, roots,\n" +
			"plugin). A precondition for the OTHER kind/forge is reported n/a or MISS, never silently skipped. When\n" +
			"CELL_HARNESS=codex it also checks the codex harness block (binary, auth, multi_agent, resident rules,\n" +
			"skills discovery). It prints one \"model pin\" row per role naming that role's harness and its RESOLVED\n" +
			"model; a role with no per-harness pin and no tier match is a MISS here, before boot, not a startup failure.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cell := cellArg(args)
			cfg := ""
			if len(args) > 1 {
				cfg = args[1]
			}
			cmdCheck(cell, cfg)
			return nil
		},
	}
}

func deskdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deskd <cell>",
		Short: "stand the cell's persistent deskd",
		Long:  "Stand the cell's persistent deskd (attended: mints per-org read tokens).",
		Args:  cobra.ArbitraryArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { cmdDeskd(cellArg(args)); return nil },
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <cell>",
		Short: "report a cell's session state",
		Long: "Scrubbed cell: `running <session>`, `stopped` or `stale-lock <pid>`. House cell: one cadence line per role.\n" +
			"A read: exit 0 unless the cell fails to load.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error { cmdStatus(cellArg(args)); return nil },
	}
}

// launchBindings are the per-run choices `desk` and `up` share. Each may come from its flag or,
// where one is named, from the cell's environment (cell.env overlaid on the process environment);
// the flag wins.
func launchBindings() []cli.Binding {
	return []cli.Binding{
		bStr("model", "model for THIS run only, passed through verbatim to the harness", "DESK_MODEL_OVERRIDE"),
		bBool("set", "persist every override given into cell.env"),
		bStr("provider", "provider for this run (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)"),
		bStr("harness", "harness for this run ("+joinPipe(harnessValues)+")"),
		bStr("kind", "cell kind for this run ("+joinPipe(kindValues)+")"),
		bStr("cockpit", "cockpit surface ("+joinPipe(cockpitValues)+")"),
		bStr("cadence", "role cadence: a duration, or off", "CELL_CADENCE"),
		bStr("tick-budget", "per-pass tick budget (a duration)", "CELL_TICK_BUDGET"),
	}
}

// launchNeeds are the legacy refusals of an empty flag value, in the legacy words.
func launchNeeds(v *cli.Values) {
	needValue(v, "cadence", "--cadence needs a duration or off")
	needValue(v, "tick-budget", "--tick-budget needs a duration")
	needValue(v, "model", "--model needs a value")
	needValue(v, "provider", "--provider needs a value (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)")
	needValue(v, "harness", "--harness needs a value (claude|codex)")
	needValue(v, "cockpit", "--cockpit needs a value ("+joinPipe(cockpitValues)+")")
}

// deskInputsFrom lowers the resolved settings to the typed options the launch code takes.
func deskInputsFrom(v *cli.Values, cfgIn string) deskInputs {
	return deskInputs{
		CfgIn:       cfgIn,
		Model:       v.String("model"),
		Provider:    v.String("provider"),
		Harness:     v.String("harness"),
		Cockpit:     v.String("cockpit"),
		Cadence:     v.String("cadence"),
		TickBudget:  v.String("tick-budget"),
		CadenceFlag: flagOnly(v, "cadence"),
		BudgetFlag:  flagOnly(v, "tick-budget"),
		Persist:     v.Bool("set"),
	}
}

// lastBare is the legacy "last bare token is the config directory" rule.
func lastBare(pos []string) string {
	if len(pos) == 0 {
		return ""
	}
	return pos[len(pos)-1]
}

func deskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desk <cell> <role> [CLAUDE_CONFIG_DIR]",
		Short: "boot one role window",
		Long: "One role window: its own worktree, the real HOME, the cell's shims.\n\n" +
			"--model overrides the cell.env pin for THIS run only (it wins over DESK_MODEL_<role> and DESK_MODEL_DEFAULT);\n" +
			"DESK_MODEL_OVERRIDE is the equivalent environment form. The the-desk Opus refusal applies to an override\n" +
			"exactly as it does to a pin. --harness overrides CELL_HARNESS for this run only. --set persists every\n" +
			"override given (the model pin, CELL_HARNESS, CELL_PROVIDER, CELL_KIND, CELL_COCKPIT) through the same\n" +
			"one-backup path `cellctl set` uses; an override not given is never re-written. --kind, --cockpit and\n" +
			"--provider override cell.env's CELL_KIND, CELL_COCKPIT and CELL_PROVIDER for this run; a --kind the cell\n" +
			"is not provisioned for is refused naming the missing key.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd, launchBindings()...)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cellArg(args)
		if len(args) < 2 || args[1] == "" {
			fmt.Fprintln(os.Stderr, "cellctl: "+deskUsage)
			exitWith(1)
		}
		c := loadCellWithKind(args[0], kindFlag(cmd))
		v := resolve(set, c)
		launchNeeds(v)
		cmdDesk(c, args[1], deskInputsFrom(v, lastBare(args[2:])))
		return nil
	}
	return cmd
}

func upCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <cell> [CLAUDE_CONFIG_DIR]",
		Short: "open one window per role in the resolved cockpit",
		Long: "One window per role in the resolved COCKPIT: a tmux session \"<cell>-cell\", a labelled herdr tab per role,\n" +
			"or an orca terminal per role (deskd + the-desk + the loop roles; a house cell opens no deskd window unless\n" +
			"DESKD=1). --cockpit overrides cell.env CELL_COCKPIT for this run.\n\n" +
			"--model, --harness, --kind and --provider override the pin for EVERY role window this run opens (per run,\n" +
			"not persisted, the same precedence as `cellctl desk`). --set persists every override given, with one cell.env\n" +
			"backup first. --automate '<cron>' is an ORCA-only shape: one scheduled automation per role, fronted by an\n" +
			"exit-code precheck, instead of a live terminal. DRY_RUN=1 prints the resolved cockpit and the per-role\n" +
			"commands and launches nothing.\n\n" +
			"On a container or scrubbed cell up hands the single desk the same typed options; the host-cockpit flags\n" +
			"(--no-the-desk, --with-the-desk, --no-attach, --automate) are refused there.",
		Args: cobra.ArbitraryArgs,
	}
	bs := append(launchBindings(),
		bBool("no-the-desk", "do not open the the-desk window"),
		bBool("with-the-desk", "open the the-desk window (the default; accepted for compatibility)"),
		bBool("no-attach", "do not attach to the session after opening it"),
		bStr("automate", "orca only: schedule one automation per role (a 5-field cron string or a preset)"),
	)
	set := declare(cmd, bs...)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		c := loadCellWithKind(cell, kindFlag(cmd))
		v := resolve(set, c)
		launchNeeds(v)
		needValue(v, "automate", "--automate needs a trigger (a 5-field cron string or a preset)")
		cmdUp(c, v, lastBare(args[1:]))
		return nil
	}
	return cmd
}

func downCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down <cell>",
		Short: "tear a cell's session down",
		Long: "Tear the session (and this cell's deskd) down. What `up` opened in a non-tmux cockpit is closed where that\n" +
			"cockpit offers a verb for it, and named for you to close by hand where it does not. A container or scrubbed\n" +
			"cell accepts no host cockpit or deskd flags.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd,
		bBool("keep-deskd", "leave the cell's deskd running"),
		bStr("cockpit", "cockpit surface ("+joinPipe(cockpitValues)+")"),
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		given := len(args) > 1 || cmd.Flags().Changed("keep-deskd") || cmd.Flags().Changed("cockpit")
		cmdDown(cell, resolve(set, nil), args[1:], given)
		return nil
	}
	return cmd
}

func smokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "smoke <cell>",
		Short: "one-shot readiness probe of a scrubbed cell",
		Long: "Scrubbed-cell only: a one-shot, tool-free, read-only readiness probe. The harness answers `READY` or the\n" +
			"verb exits 1 naming what it said instead. Never a Verify row on a live harness; DRY_RUN=1 prints the plan\n" +
			"and runs nothing.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd,
		bStr("harness", "harness to probe (claude|codex)"),
		bStr("model", "model to probe with"),
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		cmdSmoke(cell, resolve(set, nil), args[1:])
		return nil
	}
	return cmd
}

func showCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <cell>",
		Short: "print the effective per-run choices of a cell",
		Long: "A READ: one `[show] KEY=VALUE (flag|cell.env|default)` line per per-run choice (CELL_KIND, CELL_COCKPIT,\n" +
			"CELL_HARNESS, CELL_PROVIDER) and one `[show] model <role>=<m> (source)` line per role: the effective values,\n" +
			"with the flags given applied, so what an invocation would resolve to is greppable before booting it.\n" +
			"Launches and writes nothing.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd,
		bStr("kind", "cell kind ("+joinPipe(kindValues)+")"),
		bStr("cockpit", "cockpit surface ("+joinPipe(cockpitValues)+")"),
		bStr("harness", "harness ("+joinPipe(harnessValues)+")"),
		bStr("provider", "provider"),
		bStr("model", "model"),
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		c := loadCellWithKind(cell, kindFlag(cmd))
		cmdShow(c, resolve(set, nil), args[1:])
		return nil
	}
	return cmd
}

func setCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <cell> KEY=VALUE... | <cell> <role> --model <m> | <cell> [flags]",
		Short: "persist a change into a cell's cell.env",
		Long: "Persist a change into <cell>/cell.env in place: rewrite an existing KEY= line or append a new one, comment\n" +
			"lines and ordering otherwise untouched. One backup (cell.env.bak-<ts>) is written before the first edit; a\n" +
			"KEY that is not a known cell.env key is refused unless --force; each key's before/after value is printed;\n" +
			"the the-desk/Opus refusal applies to DESK_MODEL_the_desk.\n\n" +
			"Forms: KEY=VALUE pairs; role-sugar `<cell> <role> [--harness claude|codex] --model <m>`, which computes the\n" +
			"model-pin KEY from the ACTIVE harness (DESK_MODEL_<role> on claude, CODEX_MODEL_<role> on codex); and the\n" +
			"flag form `<cell> [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>]`, sugar for the matching\n" +
			"KEY=VALUE (CELL_KIND / CELL_COCKPIT / CELL_HARNESS / CELL_PROVIDER), validated by the same rules.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd,
		bBool("force", "write a KEY that is not a known cell.env key"),
		bStr("harness", "harness ("+joinPipe(harnessValues)+")"),
		bStr("model", "model pin (needs a role)"),
		bStr("kind", "CELL_KIND ("+joinPipe(kindValues)+")"),
		bStr("cockpit", "CELL_COCKPIT ("+joinPipe(cockpitValues)+")"),
		bStr("provider", "CELL_PROVIDER"),
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		cmdSet(cell, resolve(set, nil), args[1:])
		return nil
	}
	return cmd
}

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <cell>",
		Short: "scaffold a cell",
		Long: "Scaffold a cell directory under the registry.\n\n" +
			"--kind k8s (default): a full cell with its own deskd, roster and App keys under home/ (--cells-yaml <file>\n" +
			"and the forge custody flags). --forge defaults to github. github: --orgs a,b --deskd-app-pem <pem>\n" +
			"[--deskd-app-id-var VAR] (the PEM is required on the github path only; VAR defaults to DESK_APP_ID).\n" +
			"gitlab: --group <group> [--gitlab-api-base URL] [--gitlab-token-store DIR], no App PEM and no --orgs.\n\n" +
			"--kind house: a LOCAL cell on the operator's laptop; the desks reuse the operator's config home by symlink,\n" +
			"no deskd is required, every role still boots in its own LOCKED worktree. --roots '<owner>/<repo>=<abs path>,...'\n" +
			"[--roles \"<role> ...\"] [--port N].\n\n" +
			"--kind container: --repo <repo-id> with --launcher <absolute-executable> (an operator-owned launcher, no\n" +
			"credentials copied) or --container-config <absolute JSON file> (the native Go runtime; see docs/cellctl.md).\n\n" +
			"--kind scrubbed: --repo <checkout> --repo-slug <owner/repo> [--roots '...'] [--roles \"...\"], a host-local\n" +
			"cell whose harness runs inside an environment cellctl fully COMPOSES. See docs/cellctl.md.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd,
		bDef("kind", "k8s|house|container|scrubbed", "k8s"),
		bStr("container-config", "absolute JSON file for the native container runtime"),
		bStr("launcher", "absolute operator-owned container launcher"),
		bDef("forge", "github|gitlab", "github"),
		bStr("repo", "checkout path (or repo id for a container cell)"),
		bStr("cells-yaml", "this cell's slice of cells.yaml"),
		bStr("orgs", "github: comma-separated orgs"),
		bStr("repo-slug", "scrubbed: <owner>/<repo>"),
		bStr("deskd-app-pem", "github: the deskd App PEM"),
		bDef("deskd-app-id-var", "github: apps.env variable holding the App id", "DESK_APP_ID"),
		bDef("port", "house: deskd port", "8787"),
		bStr("group", "gitlab: group"),
		bStr("gitlab-api-base", "gitlab: API base URL"),
		bStr("gitlab-token-store", "gitlab: role token store directory"),
		bStr("roots", "the DESK_ROOTS map: '<owner>/<repo>=<abs path>,...'"),
		bDef("roles", "space-separated roles", rolesDefault),
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			die("new: unexpected extra argument '%s' (the cell name is '%s')", args[1], args[0])
		}
		v := resolve(set, nil)
		needValue(v, "container-config", "--container-config needs an absolute JSON file")
		needValue(v, "launcher", "--launcher needs an absolute executable")
		needValue(v, "repo-slug", "--repo-slug needs a value (<owner>/<repo>)")
		cell := ""
		if len(args) == 1 {
			cell = args[0]
		}
		cmdNew(cell, v)
		return nil
	}
	return cmd
}

func cadenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cadence <cell> status|stop|resume|recover [role]",
		Short: "inspect or control a house cell's role cadence",
		Long: "Host role cadence: the foreground Go supervisor, independent of cockpit/harness wake tools.\n\n" +
			"  cellctl up <cell> --cockpit herdr --harness codex --cadence 30m --tick-budget 20m\n" +
			"  cellctl desk <cell> <role> --cadence 30m [--tick-budget 20m]\n" +
			"  cellctl cadence <cell> status|stop|resume [role]\n" +
			"  cellctl cadence <cell> recover <role> --confirm-stopped\n\n" +
			"CELL_CADENCE / CELL_TICK_BUDGET persist the same settings; cadence off disables it. The foreground process\n" +
			"survives model turns. Restart resumes its checkpoint; an unfinished prior child refuses until inspected and\n" +
			"explicitly recovered. No OS startup service is installed.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd, bBool("confirm-stopped", "recover only: confirm the prior harness and all its children have stopped"))
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		cmdCadence(cell, args[1:], resolve(set, nil).Bool("confirm-stopped"))
		return nil
	}
	return cmd
}

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache <cell> status|clean|recover",
		Short: "report or clean a cell's managed Go caches",
		Long: "Managed Go caches (opt-in CELL_GO_CACHE=on in cell.env; macOS/Linux).\n\n" +
			"  cellctl cache <cell> status    dry-run JSON; no cleanup\n" +
			"  cellctl cache <cell> clean     inactive caches, only under pressure\n" +
			"  cellctl cache <cell> recover --confirm-stopped\n\n" +
			"Defaults: 8 GiB logical-byte budget, 10 GiB filesystem free-space floor. Configure CELL_GO_CACHE_ROOT /\n" +
			"CELL_GO_CACHE_BYTES / CELL_GO_CACHE_MIN_FREE. Recovery requires external proof ALL cache consumers have\n" +
			"stopped. See docs/cellctl-go-cache.md.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd, bBool("confirm-stopped", "recover only: confirm every cache consumer has stopped"))
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		cmdCache(cell, args[1:], resolve(set, nil).Bool("confirm-stopped"))
		return nil
	}
	return cmd
}

func commsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comms <cell> check|run|recover",
		Short: "check, run or recover a cell's interim comms service",
		Long: "Set the manifest: cellctl set <cell> CELL_COMMS_CONFIG=<absolute-path>. `up` opens one configured interim\n" +
			"service window; `down` stops it. `recover` needs --confirm-stopped. See docs/cellctl-comms.md.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd, bBool("confirm-stopped", "recover only: confirm the prior gateway, drain and every owned child have stopped"))
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cell := cellArg(args)
		cmdComms(cell, args[1:], resolve(set, nil).Bool("confirm-stopped"))
		return nil
	}
	return cmd
}

func scratchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scratch <cell> run|ack|sweep|inventory [flags] [-- command [args]]",
		Short: "managed task scratch and evidence handoff",
		Long: "Managed task scratch and evidence handoff; see docs/cellctl-scratch.md. `run` executes the command after\n" +
			"`--` in a private TMPDIR under a lease; `ack` hands the evidence off; `sweep` removes expired scratch (dry run\n" +
			"unless --apply); `inventory` reads a legacy root.",
		Args: cobra.ArbitraryArgs,
	}
	set := declare(cmd,
		bBool("apply", "apply cleanup; default is dry-run"),
		cli.Binding{Key: "max-age", Kind: cli.Duration, Flag: "max-age", Usage: "diagnostic retention age (default from the cell's scratch policy)"},
		cli.Binding{Key: "max-bytes", Kind: cli.Int, Flag: "max-bytes", Usage: "retained diagnostic byte budget (default from the cell's scratch policy)"},
		bStr("task", "task identity"),
		bStr("session", "session identity", "DESK_SESSION"),
		bStr("source", "source Git checkout (required for run)"),
		bDef("revision", "source revision", "HEAD"),
		bBool("snapshot", "materialize all tracked files, never working-directory copies"),
		cli.Binding{Key: "snapshot-bytes", Kind: cli.Int, Flag: "snapshot-bytes", Usage: "snapshot plus declared input byte limit", Default: 256 * 1024 * 1024},
		cli.Binding{Key: "input", Kind: cli.StringArray, Flag: "input", Usage: "explicit extra source-relative file needed by this task; repeatable"},
		bStr("id", "owned task id", "ASSAY_SCRATCH_ID"),
		bStr("receipt", "canonical evidence destination, after verified handoff"),
		bBool("resumable", "retain for resumption (inactive task only)"),
		bStr("path", "legacy root for read-only inventory"),
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		d := cmd.ArgsLenAtDash()
		if d == 0 {
			cellArg(nil)
		}
		cell := cellArg(args)
		if len(args) < 2 || d == 1 {
			die("scratch requires run, ack, sweep, or inventory")
		}
		// The command to run is what follows `--`; with no `--`, anything past the action
		// is taken as the command, as it always was.
		rest, command := args[1:], args[2:]
		if d >= 0 {
			rest, command = args[1:d], args[d:]
		}
		cmdScratch(cell, rest[0], resolve(set, nil), command)
		return nil
	}
	return cmd
}

// The four internal entrypoints below take an opaque argv (a hook's positional arguments, a
// command line to supervise), so Cobra parses no flags for them; rawArgs applies a leading
// --cells-root exactly as the legacy selector did. They are hidden from help.

func providersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "providers init",
		Short: "create the shared providers.json",
		Long:  "Create CELLS_ROOT/providers.json (per-provider desk models/effort; <cell>/providers.json overrides) without overwriting an existing one.",
		Args:  cobra.ArbitraryArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { cmdProviders(args); return nil },
	}
}

func modelPolicyCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "model-policy hook <cell-dir> <role> <provider> <requested> <harness> <policy-sha256>",
		Short:              "runtime model-policy hook (internal)",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               func(cmd *cobra.Command, args []string) error { cmdModelPolicy(rawArgs(args)); return nil },
	}
}

func cacheRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "cache-run <executable> [args...]",
		Short:              "run a command under the cache supervisor (internal)",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               func(cmd *cobra.Command, args []string) error { cmdCacheRun(rawArgs(args)); return nil },
	}
}

func containerRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "container-run <cell> <role> <a> <b>",
		Short:              "container console entrypoint (internal)",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               func(cmd *cobra.Command, args []string) error { cmdContainerRun(rawArgs(args)); return nil },
	}
}
