package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// dispatchCLI keeps parsing/configuration distinct from every domain authority.
// Tests replace the typed handoff, never the custody checks in the domain handlers.
type dispatchCLI struct {
	inputs cli.Inputs
	before func() error
	handle func(string, dispatchOpts, string) error
}

// modeArgs is compatibility normalization only: the existing leading selector is
// not a new verb (which would reserve an otherwise valid item key). All flags and
// positional counts are parsed by Cobra/pflag, once. No values are rewritten.
func modeArgs(args []string) (string, []string) {
	mode := ""
	i := 0
	for i < len(args) && (args[i] == "--trace" || args[i] == "-trace" || strings.HasPrefix(args[i], "--trace=") || strings.HasPrefix(args[i], "-trace=")) {
		i++
	}
	if i < len(args) {
		switch args[i] {
		case "--stamp-only", "--check-verifier", "--attest-verifier":
			mode = args[i]
			out := append([]string{}, args[:i]...)
			return mode, append(out, args[i+1:]...)
		case "help":
			if i+1 < len(args) {
				switch args[i+1] {
				case "--stamp-only", "--check-verifier", "--attest-verifier":
					mode = args[i+1]
					out := append([]string{}, args[:i+1]...)
					return mode, append(out, args[i+2:]...)
				}
			}
		}
	}
	return mode, append([]string(nil), args...)
}

func dispatchBindings(mode string) []cli.Binding {
	str := func(k, d, u string) cli.Binding {
		return cli.Binding{Key: k, Flag: k, Kind: cli.String, Default: d, Usage: u}
	}
	integer := func(k, u string) cli.Binding {
		return cli.Binding{Key: k, Flag: k, Kind: cli.Int, Default: 0, Usage: u}
	}
	boolean := func(k, u string) cli.Binding {
		return cli.Binding{Key: k, Flag: k, Kind: cli.Bool, Default: false, Usage: u}
	}
	common := []cli.Binding{boolean("trace", "print scrubbed diagnostics for execution (any position)")}
	switch mode {
	case "--check-verifier", "--attest-verifier":
		return append(common, str("root", "", "existing detached verifier worktree (required)"), str("brief", "", "source brief (required for --check-verifier)"), boolean("dry-run", "validate arguments without mutation"))
	case "--stamp-only":
		return append(common, str("repo", "", "target owner/repo (required)"), integer("pr", "already-open change number (required)"), str("model", "", "exact model selected by the dispatcher (required)"), str("tier", "", "selected execution tier (required)"), str("kit", "worker", "original dispatch kit"), integer("attestation", "worker pre-work attestation record (also read from the change)"), boolean("dry-run", "read and report without mutation"))
	default:
		return append(common,
			str("tier", "any", "execution tier the item demands: strong|any"), str("kit", "worker", "prompt kit: "+joinKits()),
			str("repo", "", "owner/name (default: --root's origin)"), str("root", ".", "item's own repo root"),
			str("claim-root", "", "checkout carrying consumer scripts (default: --root); claims still land in --repo"),
			str("model", "", "exact model selected by the dispatcher"), str("branch", "", "fresh branch, or verified resume source branch"),
			str("brief", "", "item specification path"), boolean("gate-human", "ensure the human decision issue exists"),
			integer("pr", "already-open change to resume"), str("prompt-file", "", "caller task path (never written); emit to a new .emitted.md sibling"),
			boolean("quiet", "suppress per-step OK lines"), boolean("dry-run", "print the plan and prompt; touch nothing"),
			str("worktree", "", "operator-stated existing home, with --dry-run only"), boolean("rework", "follow up a merged delivery on a new branch"),
			boolean("attest-worker", "fresh worker: create, stamp, close and read back pre-work attestation"),
			boolean("cross-repo", "brief homed in tracking repo; deliverable in --repo with Cross-repo-brief trailer"),
			str(deskkit.ScanOverrideFlag, "", deskkit.OutboundOverrideHelp), boolean("kits", "list embedded prompt kits and exit"))
	}
}

func newDispatchCLI(mode string, env dispatchCLI) *cobra.Command {
	use := "deskdispatch <item-key> [flags]"
	short := "per-item dispatch ceremony (engine seam: DISPATCH)"
	if mode != "" {
		use = "deskdispatch " + mode + " [flags]"
		short = "existing run admission: " + mode
	}
	root := cli.NewRoot(use, short)
	root.Long = short + "\n\nExecution retains domain custody, claims, worktree isolation and rollback.\n" +
		"Modes: --stamp-only, --check-verifier, --attest-verifier. Request each mode's --help.\n" +
		"Dispatch steps: " + strings.Join(dispatchSteps, ", ") + ".\n" +
		"Operational reference: docs/desk-tools/deskdispatch.md"
	set := cli.Declare(root, dispatchBindings(mode)...)
	if mode == "" {
		root.Args = cobra.MaximumNArgs(1)
	} else {
		root.Args = cobra.NoArgs
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		v, err := set.Resolve(env.inputs)
		if err != nil {
			return deskkit.Refused(err.Error())
		}
		if mode == "" && v.Bool("kits") {
			executionFlag := false
			cmd.Flags().Visit(func(f *pflag.Flag) {
				if f.Name != "kits" && f.Name != "trace" {
					executionFlag = true
				}
			})
			if len(args) != 0 || executionFlag {
				return &deskkit.DeskError{Code: cli.ExitUsage, Msg: "--kits accepts no execution arguments"}
			}
			for _, k := range kitNames() {
				fmt.Fprintln(cmd.OutOrStdout(), k)
			}
			return nil
		}
		if mode == "" && len(args) == 0 {
			return deskkit.Refused("deskdispatch requires an <item-key>")
		}
		if v.Source("trace") == cli.Flag {
			deskkit.SetTrace(v.Bool("trace"))
		}
		if env.before != nil {
			if err := env.before(); err != nil {
				return err
			}
		}
		o := dispatchOpts{dryRun: v.Bool("dry-run")}
		if mode != "--stamp-only" {
			o.root = v.String("root")
			o.brief = v.String("brief")
		}
		override := ""
		switch mode {
		case "--check-verifier", "--attest-verifier":
		case "--stamp-only":
			o.stampOnly = true
			o.repo = v.String("repo")
			o.pr = v.Int("pr")
			o.model = v.String("model")
			o.tier = v.String("tier")
			o.kit = v.String("kit")
			o.attestation = v.Int("attestation")
		default:
			o.item = args[0]
			o.tier = v.String("tier")
			o.kit = v.String("kit")
			o.repo = v.String("repo")
			o.claimRoot = v.String("claim-root")
			o.model = v.String("model")
			o.branch = v.String("branch")
			o.gateHuman = v.Bool("gate-human")
			o.pr = v.Int("pr")
			o.promptFile = v.String("prompt-file")
			o.quiet = v.Bool("quiet")
			o.worktree = v.String("worktree")
			o.rework = v.Bool("rework")
			o.attestWorker = v.Bool("attest-worker")
			o.crossRepo = v.Bool("cross-repo")
			override = v.String(deskkit.ScanOverrideFlag)
		}
		return env.handle(mode, o, override)
	}
	return root
}

func executeMode(mode string, o dispatchOpts, override string) error {
	switch mode {
	case "--stamp-only":
		return executeStampOnly(o)
	case "--check-verifier", "--attest-verifier":
		return executeVerifier(mode, o)
	default:
		return executeDispatch(o, override)
	}
}

func cliVersion() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskdispatch sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

func parseDispatchCLI(args []string, env dispatchCLI, streams cli.IO) int {
	mode, args := modeArgs(args)
	return cli.Run(func() *cobra.Command { return newDispatchCLI(mode, env) }, args, cli.Options{
		IO: streams, Version: cliVersion(), VersionTemplate: "{{.Version}}\n", GoFlagCompat: true,
		ExitCode: func(err error) int { deskkit.ReportError(streams.Err, err); return deskkit.ExitCodeOf(err) },
	})
}

func runCommand(args []string, production bool) int {
	env := dispatchCLI{handle: executeMode}
	env.before = func() error {
		if production {
			deskkit.SetToolClass(deskkit.ClassForTool(false))
			deskkit.EchoEffectiveConfig(os.Stderr)
			if !deskkit.CheckVerbActivation(os.Stderr) {
				return deskkit.Unverifiable("deskdispatch activation could not be established", nil)
			}
			return nil // Guard and diagnostics remain inside the measured execution.
		}
		if err := deskkit.Guard(); err != nil {
			return err
		}
		deskkit.WarnIfUnpinned(os.Stderr)
		return nil
	}
	if production {
		env.handle = func(mode string, o dispatchOpts, override string) error {
			var result error
			deskkit.PerfRun(toolName, perfVerbs, args, func([]string) int {
				if result = deskkit.Guard(); result == nil {
					deskkit.WarnIfUnpinned(os.Stderr)
					result = executeMode(mode, o, override)
				}
				return deskkit.ExitCodeOf(result)
			})
			return result
		}
	}
	return parseDispatchCLI(args, env, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
}

// Compatibility entrypoints used by existing direct-domain regression tests. They
// all enter the same Cobra tree; none retains a Go FlagSet or a second parser.
func cmdDispatch(args []string) error {
	var result error
	env := dispatchCLI{handle: func(mode string, o dispatchOpts, override string) error {
		result = executeMode(mode, o, override)
		return result
	}}
	code := parseDispatchCLI(args, env, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
	if result != nil {
		return result
	}
	if code != 0 {
		return &deskkit.DeskError{Code: code, Msg: "deskdispatch: invalid command line"}
	}
	return nil
}
func cmdStampOnly(args []string) error { return cmdDispatch(append([]string{"--stamp-only"}, args...)) }
func cmdVerifierAttestation(mode string, args []string) error {
	return cmdDispatch(append([]string{mode}, args...))
}

func dispatchHelp() string {
	var b bytes.Buffer
	parseDispatchCLI([]string{"--help"}, dispatchCLI{}, cli.IO{Out: &b, Err: &b})
	return b.String()
}
