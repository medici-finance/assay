// Command deskchangelog is the changelog and release-check tooling as one Go command
// (desktools-v2/52): the release-time fragment aggregator, the pull-request changelog gate,
// the release-credit identity classifier, the spec-header release check and the repo-wide
// `## Human decision` default lint. The five legacy entrypoints under tools/changelog,
// tools/release and tools/ are transparent adapters that run one verb of this command with
// their arguments passed through unchanged.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const rootLong = `deskchangelog is the changelog and release-check tooling.

Verbs:
  unreleased-bullets  print the bullet entries under '## Unreleased' (aggregate.py)
  highlights          print the aggregated release highlights (aggregate.py)
  roll                write the dated CHANGELOG.md section (aggregate.py)
  credits             print the pull request each fragment arrived on (aggregate.py)
  check               the pull-request changelog gate (tools/changelog/check.sh)
  credit-identity     classify a pull-request author for release credit
                      (tools/changelog/credit-identity.sh)
  check-spec-header   the release-time spec header version check
                      (tools/release/check-spec-header-version.sh)
  lint-decision-defaults
                      report briefs whose '## Human decision' has no parseable default
                      (tools/lint-human-decision-defaults.sh)

Run 'deskchangelog <verb> --help' for a verb's arguments, inputs and exit codes.`

// verbs are the registered top-level verbs, for the performance recorder's verb match.
var verbs = []string{"unreleased-bullets", "highlights", "roll", "credits", "check", "credit-identity", "check-spec-header", "lint-decision-defaults"}

func main() {
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	os.Exit(deskkit.PerfRunMeasured("deskchangelog", verbs, os.Args[1:], func(a []string, measurement *perfrec.Session) (int, perfrec.Context) {
		sink := func(c perfrec.Context) {
			selected = c
			if measurement != nil {
				measurement.SelectContext(c)
			}
		}
		code := execute(a, os.Stdout, os.Stderr, sink)
		return code, selected
	}))
}

// lookupEnv is the ONE environment read the declared settings make; every variable a verb
// consults is a declared binding. Tests replace it.
var lookupEnv = os.LookupEnv

// onResolve, when set, sees every invocation's resolved settings (the contract tests' window
// on what reached the handler).
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == 0 {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version prints.
func versionLine() string {
	s, b := deskkit.Version()
	return fmt.Sprintf("deskchangelog sourceSHA=%s builtAt=%s releaseTag=%s", s, b, deskkit.ReleaseTagOrDev())
}

// usageErr is a positional-argument refusal: cli.Run prints it once and exits with the
// usage code, before any handler runs.
func usageErr(cmd *cobra.Command) error {
	return fmt.Errorf("usage: %s", cmd.UseLine())
}

// argsBetween refuses a positional count outside [lo, hi] with the verb's usage line.
func argsBetween(lo, hi int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < lo || len(args) > hi {
			return usageErr(cmd)
		}
		return nil
	}
}

// resolve reads one verb's declared settings through lookupEnv.
func resolve(set *cli.Set) (*cli.Values, error) {
	v, err := set.Resolve(cli.Inputs{LookupEnv: lookupEnv})
	if err != nil {
		return nil, err
	}
	if onResolve != nil {
		onResolve(v)
	}
	return v, nil
}

// creditsBinding is aggregate.py's optional `--credits <map-file>`. aggregate.py accepted it
// on every verb and ignored it where it has no meaning; unreleased-bullets and credits keep
// accepting it (hidden) so no legacy command line changes meaning.
func creditsBinding(hidden bool) cli.Binding {
	return cli.Binding{Key: "credits", Kind: cli.String, Flag: "credits", Hidden: hidden,
		Usage: "credits map `file` of <fragment><TAB>@<login> lines; a missing file credits nobody"}
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer, sinks ...func(perfrec.Context)) int {
	var sink func(perfrec.Context)
	if len(sinks) > 0 {
		sink = sinks[0]
	}
	build := func() *cobra.Command {
		root := cli.NewRoot("deskchangelog", "changelog aggregation, the changelog gate and release checks")
		root.Long = rootLong
		// A bare invocation is a usage error, as `aggregate.py` with no verb was.
		root.RunE = func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(stderr, "usage: %s\nRun 'deskchangelog --help' for usage.\n", "deskchangelog <verb> ...")
			return exitErr(cli.ExitUsage)
		}
		root.AddCommand(aggregateCommands(stdout, stderr)...)
		root.AddCommand(
			checkCommand(stdout, stderr),
			creditIdentityCommand(stdout, stderr),
			specHeaderCommand(stdout, stderr),
			lintDecisionCommand(stdout, stderr),
		)
		return root
	}
	return cli.Run(build, args, cli.Options{
		Performance:     sink,
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		UsageExit:       cli.ExitUsage,
	})
}
