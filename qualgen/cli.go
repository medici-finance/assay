package main

// cli.go is qualgen's command line: a Cobra command tree that parses, validates and
// renders help, and a small declared-settings layer that resolves each verb's settings
// through a fresh Viper instance per invocation. It follows the suite's CLI contract
// (docs/streams/desktools-v2/cli-contract.md). qualgen is a standalone module, so it uses
// Cobra and Viper directly rather than the desk module's adapter.
//
// The contract points this file keeps:
//
//   - Help and version run before any setting is resolved and before any verb handler,
//     so they never read the environment, a file or a repository.
//   - A setting is read only from the sources its declaration names. The one
//     environment variable qualgen reads (the CI-writer authorization of
//     `report --write`) is declared; nothing else is looked up, and there is no config
//     file search, AutomaticEnv or global Viper.
//   - The Go flag package's spellings (-out x, -out=x, -help, -version) keep working,
//     because qualgen's parser was Go's flag package and its callers may use them.
//   - Parse errors exit 2 before any handler runs; each handler keeps its own exit codes.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// exitUsage is the exit code for a parse error, as Go's flag package and the old
// dispatcher used.
const exitUsage = 2

// modeList is the verb list the unknown-mode error has always named.
const modeList = "mine, report, pr, check, sweep, init"

// cliEnv is everything one invocation may reach beyond its arguments: the environment
// lookup (only declared names are ever passed to it) and the verb handlers. Tests swap
// both to count effects; dispatch passes the process environment and the real handlers.
type cliEnv struct {
	lookupEnv func(string) (string, bool)
	verbs     verbHandlers
	// observe, when set, sees each verb's resolved settings before its handler runs.
	observe func(verb string, v *settingValues)
}

// verbHandlers are the typed entry points the command tree hands parsed options to.
type verbHandlers struct {
	mine   func(mineOptions, io.Writer, io.Writer) int
	report func(reportOptions, io.Writer, io.Writer) int
	pr     func(prOptions, io.Writer, io.Writer) int
	check  func(checkOptions, io.Writer, io.Writer) int
	sweep  func(sweepOptions, io.Writer, io.Writer) int
	init   func(initOptions, io.Writer, io.Writer) int
}

func productionVerbs() verbHandlers {
	return verbHandlers{
		mine:   mineMain,
		report: reportMain,
		pr:     prMain,
		check:  checkMain,
		sweep:  sweepMain,
		init:   initMain,
	}
}

// dispatch runs one qualgen command line against the process environment. Exit codes
// follow the statusgen house convention: 0 success, 1 runtime error, 2 usage error.
func dispatch(args []string, stdout, stderr io.Writer) int {
	return runCLI(args, stdout, stderr, cliEnv{lookupEnv: os.LookupEnv, verbs: productionVerbs()})
}

// exitError carries a handler's own non-zero exit code out of Cobra.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func exitWith(code int) error {
	if code == 0 {
		return nil
	}
	return exitError{code}
}

// runCLI builds a fresh command tree, executes args against it and returns the exit code.
func runCLI(args []string, stdout, stderr io.Writer, env cliEnv) int {
	root := newRootCmd(env)
	root.SetIn(strings.NewReader(""))
	root.SetOut(stdout)
	root.SetErr(stderr)
	if args == nil {
		args = []string{}
	}
	root.SetArgs(normalizeGoFlags(root, args))
	err := root.Execute()
	if err == nil {
		return 0
	}
	var ee exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	fmt.Fprintf(stderr, "qualgen: %v\nRun 'qualgen --help' for usage.\n", err)
	return exitUsage
}

func newRootCmd(env cliEnv) *cobra.Command {
	root := &cobra.Command{
		Use:   "qualgen",
		Short: "repo-agnostic git-history quality miner",
		Long: `qualgen — repo-agnostic git-history quality miner

All read modes are read-only against the mined repo; artifacts land only under --out.
init scaffolds an adopter repo (report pack) and never overwrites an existing file.`,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Version:           qualgenVersion,
		// An unknown first word keeps the old dispatcher's message and usage exit.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown mode %q (want one of: %s)", args[0], modeList)
			}
			return nil
		},
		// A bare `qualgen` prints usage to stderr and exits 2, as it always has.
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.ErrOrStderr(), cmd.UsageString())
			return exitError{exitUsage}
		},
	}
	// Declared before Cobra would add its own, so there is no -v shorthand.
	root.Flags().Bool("version", false, "print the release tag and exit")
	// The release tag prints bare, as `qualgen --version` always has.
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(
		newMineCmd(env),
		newReportCmd(env),
		newPRCmd(env),
		newCheckCmd(env),
		newSweepCmd(env),
		newInitCmd(env),
		newVersionCmd(),
	)
	refuseCompletionRequests(root)
	return root
}

// newVersionCmd keeps `qualgen version`, the hidden word form of --version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "version",
		Short:  "print the release tag",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), qualgenVersion)
		},
	}
}

// resolveFor resolves s for verb and hands the values to the test observer, if any. A
// resolution error is a usage error that names the setting and its source, never a value.
func resolveFor(cmd *cobra.Command, env cliEnv, verb string, s *settings) (*settingValues, error) {
	v, err := s.resolve(env.lookupEnv)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "qualgen %s: %v\n", verb, err)
		return nil, exitError{exitUsage}
	}
	if env.observe != nil {
		env.observe(verb, v)
	}
	return v, nil
}

func newMineCmd(env cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mine --repo <dir> --out <dir>",
		Short: "extract history into the tracking root",
		Args:  cobra.NoArgs,
	}
	s := declare(cmd,
		setting{key: "repo", kind: kindString, flag: "repo", def: ".", usage: "path to the git repository to mine (read-only)"},
		setting{key: "out", kind: kindString, flag: "out", def: "", usage: "tracking root for artifacts (required unless --in-repo)"},
		setting{key: "in-repo", kind: kindBool, flag: "in-repo", def: false, usage: "opt in to writing artifacts into the mined repo itself"},
		setting{key: "block-min", kind: kindInt, flag: "block-min", def: DefaultBlockMin, usage: "minimum identical-line run to count as a moved/copied block (M1 §4.1)"},
		setting{key: "churn-window-days", kind: kindInt, flag: "churn-window-days", def: DefaultChurnWindowDays, usage: "churn window in days: a line revised/deleted within this of landing is churned (M1 §4.2)"},
		setting{key: "identity-map", kind: kindString, flag: "identity-map", def: "", usage: "path to a JSON author-identity class map (unmapped authors fall into an explicit 'unclassified' class)"},
		setting{key: "instruction-globs", kind: kindString, flag: "instruction-globs", def: "", usage: "comma-separated instruction-doc globs for the brittleness pass (spec §4.6); empty leaves reference-validity could-not-measure, never a silent zero"},
		setting{key: "fix-source", kind: kindString, flag: "fix-source", def: "", usage: "offline issue/PR snapshot enabling defect identification and B-SZZ tracing (see qualgen/README.md)"},
		setting{key: "defect-labels", kind: kindString, flag: "defect-labels", def: "bug,defect,incident", usage: "comma-separated defect labels used with --fix-source"},
	)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		v, err := resolveFor(cmd, env, "mine", s)
		if err != nil {
			return err
		}
		return exitWith(env.verbs.mine(mineOptions{
			Repo:             v.str("repo"),
			Out:              v.str("out"),
			InRepo:           v.boolean("in-repo"),
			BlockMin:         v.integer("block-min"),
			ChurnWindowDays:  v.integer("churn-window-days"),
			IdentityMap:      v.str("identity-map"),
			InstructionGlobs: v.str("instruction-globs"),
			FixSource:        v.str("fix-source"),
			DefectLabels:     v.str("defect-labels"),
		}, cmd.OutOrStdout(), cmd.ErrOrStderr()))
	}
	return cmd
}

func newReportCmd(env cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report --out <dir>",
		Short: "render the QUALITY.md trend view",
		Long: "report renders the QUALITY.md trend view to stdout. --write writes the committed view and is\n" +
			"reachable only by CI: it requires " + ciWriterEnv + "=" + ciWriterToken + " in the environment.",
		Args: cobra.NoArgs,
	}
	s := declare(cmd,
		setting{key: "out", kind: kindString, flag: "out", def: "", usage: "tracking root to read artifacts from (required)"},
		setting{key: "baselines", kind: kindString, flag: "baselines", def: "", usage: "optional JSON file of industry baselines overriding the built-in set"},
		setting{key: "write", kind: kindBool, flag: "write", def: false, usage: "write the committed QUALITY.md (CI-writer only; a local run is refused and renders to stdout)"},
		// The CI-writer authorization: environment only, never a flag, no default.
		setting{key: "quality-writer", kind: kindString, env: ciWriterEnv},
	)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		v, err := resolveFor(cmd, env, "report", s)
		if err != nil {
			return err
		}
		return exitWith(env.verbs.report(reportOptions{
			Out:       v.str("out"),
			Baselines: v.str("baselines"),
			Write:     v.boolean("write"),
			Writer:    v.str("quality-writer"),
		}, cmd.OutOrStdout(), cmd.ErrOrStderr()))
	}
	return cmd
}

func newPRCmd(env cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <n> --out <dir> [--repo <dir>] [--head <ref> --base <ref>]",
		Short: "per-PR risk-feature feed",
		Args:  cobra.MaximumNArgs(1),
	}
	s := declare(cmd,
		setting{key: "out", kind: kindString, flag: "out", def: "", usage: "tracking root to read mined artifacts from (required)"},
		setting{key: "repo", kind: kindString, flag: "repo", def: ".", usage: "path to the git repository the PR belongs to (read-only)"},
		setting{key: "head", kind: kindString, flag: "head", def: "", usage: "an open PR's head ref/commit — used together with --base, overrides the merged-PR history search"},
		setting{key: "base", kind: kindString, flag: "base", def: "", usage: "an open PR's base ref/commit — used together with --head"},
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v, err := resolveFor(cmd, env, "pr", s)
		if err != nil {
			return err
		}
		o := prOptions{Out: v.str("out"), Repo: v.str("repo"), Head: v.str("head"), Base: v.str("base")}
		if len(args) == 1 {
			o.PR = args[0]
		}
		return exitWith(env.verbs.pr(o, cmd.OutOrStdout(), cmd.ErrOrStderr()))
	}
	return cmd
}

func newCheckCmd(env cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <paths>... --out <dir> [--repo <dir>]",
		Short: "brittleness screen for a named file set",
		Args:  cobra.ArbitraryArgs,
	}
	s := declare(cmd,
		setting{key: "out", kind: kindString, flag: "out", def: "", usage: "tracking root to read mined artifacts from (required)"},
		setting{key: "repo", kind: kindString, flag: "repo", def: ".", usage: "path to the git repository the screened paths live in (read-only; consulted only for the live reference-rot check)"},
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v, err := resolveFor(cmd, env, "check", s)
		if err != nil {
			return err
		}
		return exitWith(env.verbs.check(checkOptions{
			Paths: append([]string(nil), args...),
			Out:   v.str("out"),
			Repo:  v.str("repo"),
		}, cmd.OutOrStdout(), cmd.ErrOrStderr()))
	}
	return cmd
}

func newSweepCmd(env cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sweep --repo <dir> --out <dir> --config <file> [--reverify-all]",
		Short: "code-slop forensic sweep lane",
		Args:  cobra.NoArgs,
	}
	s := declare(cmd,
		setting{key: "repo", kind: kindString, flag: "repo", def: "", usage: "path to the target git repository to sweep (read-only) (required)"},
		setting{key: "out", kind: kindString, flag: "out", def: "", usage: "tracking root the sweep artifacts land under (required)"},
		setting{key: "config", kind: kindString, flag: "config", def: "", usage: "sweep config: per-category tool set + verifier selection"},
		setting{key: "reverify-all", kind: kindBool, flag: "reverify-all", def: false, usage: "re-verify every current suspect, not just new fingerprints"},
	)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		v, err := resolveFor(cmd, env, "sweep", s)
		if err != nil {
			return err
		}
		return exitWith(env.verbs.sweep(sweepOptions{
			Repo:        v.str("repo"),
			Out:         v.str("out"),
			Config:      v.str("config"),
			ReverifyAll: v.boolean("reverify-all"),
		}, cmd.OutOrStdout(), cmd.ErrOrStderr()))
	}
	return cmd
}

func newInitCmd(env cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [--root <dir>] [--dry-run]",
		Short: "scaffold the report pack (CI workflow + pin) into an adopter repo",
		Args:  cobra.NoArgs,
	}
	s := declare(cmd,
		setting{key: "root", kind: kindString, flag: "root", def: ".", usage: "target repo root to scaffold the report pack into"},
		setting{key: "dry-run", kind: kindBool, flag: "dry-run", def: false, usage: "preview every file that would be created and write nothing"},
	)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		v, err := resolveFor(cmd, env, "init", s)
		if err != nil {
			return err
		}
		return exitWith(env.verbs.init(initOptions{
			Root:   v.str("root"),
			DryRun: v.boolean("dry-run"),
		}, cmd.OutOrStdout(), cmd.ErrOrStderr()))
	}
	return cmd
}

// refuseCompletionRequests makes Cobra's hidden shell-completion entrypoints (__complete
// and __completeNoDesc) refuse like an unknown mode. Cobra registers them whatever
// CompletionOptions say; qualgen has never offered completion.
func refuseCompletionRequests(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:                cobra.ShellCompRequestCmd,
		Aliases:            []string{cobra.ShellCompNoDescRequestCmd},
		Hidden:             true,
		DisableFlagParsing: true,
		Args: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("unknown mode %q (want one of: %s)", cmd.CalledAs(), modeList)
		},
		Run: func(*cobra.Command, []string) {},
	})
}

// normalizeGoFlags rewrites -name and -name=value to --name forms for every long flag the
// tree defines (plus help and version), leaving shorthand clusters, values and everything
// after "--" untouched. A token that is the separate value of a preceding non-boolean
// long flag is never rewritten, so "--out -repo" keeps "-repo" as the value.
func normalizeGoFlags(root *cobra.Command, args []string) []string {
	long := map[string]bool{"help": false, "version": false} // name -> takes a value
	var collect func(c *cobra.Command)
	collect = func(c *cobra.Command) {
		visit := func(f *pflag.Flag) { long[f.Name] = f.NoOptDefVal == "" }
		c.Flags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
		for _, sub := range c.Commands() {
			collect(sub)
		}
	}
	collect(root)
	out := make([]string, 0, len(args))
	valueNext := false
	for i, a := range args {
		if valueNext {
			out = append(out, a)
			valueNext = false
			continue
		}
		if a == "--" {
			return append(out, args[i:]...)
		}
		name := ""
		switch {
		case strings.HasPrefix(a, "--"):
			name = strings.TrimPrefix(a, "--")
		case strings.HasPrefix(a, "-") && len(a) > 2:
			cand := strings.TrimPrefix(a, "-")
			n := cand
			if j := strings.IndexByte(cand, '='); j >= 0 {
				n = cand[:j]
			}
			if _, ok := long[n]; ok {
				a = "-" + a
				name = cand
			}
		}
		if name != "" && !strings.Contains(name, "=") && long[name] {
			valueNext = true
		}
		out = append(out, a)
	}
	return out
}

// --- declared settings ---------------------------------------------------------------

type settingKind int

const (
	kindString settingKind = iota
	kindBool
	kindInt
)

func (k settingKind) String() string {
	return [...]string{"string", "bool", "int"}[k]
}

// settingSource is where a resolved setting came from, highest precedence last.
type settingSource int

const (
	srcUnset settingSource = iota
	srcDefault
	srcEnv
	srcFlag
)

func (s settingSource) String() string {
	return [...]string{"unset", "default", "env", "flag"}[s]
}

// setting declares one setting and every source it may come from: a long flag, one
// environment variable, a default. Precedence is flag > env > default over the declared
// sources. qualgen has no configuration file, so there is no config source.
type setting struct {
	key   string
	kind  settingKind
	flag  string // "" = no flag
	env   string // "" = no environment variable
	def   any    // nil = no default; its type matches kind
	usage string
}

type settings struct {
	cmd  *cobra.Command
	decl []setting
}

// declare registers each setting's flag on cmd. A malformed declaration is a programming
// error and panics, so it fails qualgen's own tests rather than a caller.
func declare(cmd *cobra.Command, decl ...setting) *settings {
	seen := map[string]bool{}
	for _, d := range decl {
		if d.key == "" || seen[d.key] {
			panic(fmt.Sprintf("qualgen: setting %q declared empty or twice", d.key))
		}
		seen[d.key] = true
		if d.flag == "" && d.env == "" && d.def == nil {
			panic(fmt.Sprintf("qualgen: setting %q declares no source", d.key))
		}
		if d.flag == "" {
			continue
		}
		fs := cmd.Flags()
		switch d.kind {
		case kindString:
			def, _ := d.def.(string)
			fs.String(d.flag, def, d.usage)
		case kindBool:
			def, _ := d.def.(bool)
			fs.Bool(d.flag, def, d.usage)
		case kindInt:
			def, _ := d.def.(int)
			fs.Int(d.flag, def, d.usage)
		}
	}
	return &settings{cmd: cmd, decl: decl}
}

// settingValues are one invocation's resolved settings.
type settingValues struct {
	vals map[string]resolvedSetting
}

type resolvedSetting struct {
	src settingSource
	v   any
}

// resolve reads every declared setting from its declared sources through a fresh Viper
// instance. Each source lives under its own key prefix, so IsSet answers "did THIS source
// set the key" and a present-but-empty value is told apart from an absent one. Only the
// environment names the declarations list are passed to lookupEnv. Call it inside RunE,
// never before help or version.
func (s *settings) resolve(lookupEnv func(string) (string, bool)) (*settingValues, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	envLayer := map[string]any{}
	for _, d := range s.decl {
		if d.flag != "" {
			if f := s.cmd.Flags().Lookup(d.flag); f != nil && f.Changed {
				val, err := flagValue(s.cmd.Flags(), d)
				if err != nil {
					return nil, fmt.Errorf("setting %q from flag: not a valid %s", d.key, d.kind)
				}
				v.Set("flag::"+d.key, val)
			}
		}
		if d.env != "" {
			if val, ok := lookupEnv(d.env); ok {
				envLayer[d.key] = val
			}
		}
		if d.def != nil {
			v.SetDefault("default::"+d.key, d.def)
		}
	}
	if err := v.MergeConfigMap(map[string]any{"env": envLayer}); err != nil {
		return nil, fmt.Errorf("resolving settings: %v", err)
	}
	out := &settingValues{vals: map[string]resolvedSetting{}}
	for _, d := range s.decl {
		for _, src := range []settingSource{srcFlag, srcEnv, srcDefault} {
			k := src.String() + "::" + d.key
			if !v.IsSet(k) {
				continue
			}
			typed, err := convertSetting(d.kind, v.Get(k))
			if err != nil {
				return nil, fmt.Errorf("setting %q from %s: not a valid %s", d.key, src, d.kind)
			}
			out.vals[d.key] = resolvedSetting{src: src, v: typed}
			break
		}
	}
	return out, nil
}

func flagValue(fs *pflag.FlagSet, d setting) (any, error) {
	switch d.kind {
	case kindBool:
		return fs.GetBool(d.flag)
	case kindInt:
		return fs.GetInt(d.flag)
	}
	return fs.GetString(d.flag)
}

func convertSetting(k settingKind, raw any) (any, error) {
	switch k {
	case kindString:
		if s, ok := raw.(string); ok {
			return s, nil
		}
	case kindBool:
		if b, ok := raw.(bool); ok {
			return b, nil
		}
	case kindInt:
		if n, ok := raw.(int); ok {
			return n, nil
		}
	}
	return nil, fmt.Errorf("not a %s", k)
}

// source reports which source set key, or srcUnset.
func (v *settingValues) source(key string) settingSource { return v.vals[key].src }

func (v *settingValues) str(key string) string   { s, _ := v.vals[key].v.(string); return s }
func (v *settingValues) boolean(key string) bool { b, _ := v.vals[key].v.(bool); return b }
func (v *settingValues) integer(key string) int  { n, _ := v.vals[key].v.(int); return n }
