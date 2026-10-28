// Command deskcalibrate runs the monthly reviewer-calibration sample
// (verify-integrity/10): it picks ten PRs the reviewer App APPROVED in the prior
// calendar month, refuses a re-review that is not independent of the reviewer's
// own model vendor (the calibration SPOF), and renders the agreement metric as a
// numerator/denominator FRACTION.
//
//	deskcalibrate sample --month YYYY-MM --seed N (--vendor V | --human LOGIN) \
//	                     (--prs a,b,c | --candidates-file F) [--reviewer-vendor V] [--n 10]
//	        # emit the seeded ten-PR sample + the re-review worklist. Refuses (exit 5)
//	        # when the re-reviewer vendor equals the reviewer role's vendor.
//
//	deskcalibrate report --results F
//	        # render the monthly report markdown from a re-review results JSON on
//	        # stdout; the operator redirects it into the report file. Refuses a
//	        # same-vendor sample or an empty (0-verdict) result.
//
// deskcalibrate performs NO forge writes. It reads ASSAY_REVIEWER_VENDOR to learn
// the reviewer role's vendor (documented in roster.env); the candidate APPROVED
// PRs are supplied to it (a forge read the operator runs) so the tool itself
// stays offline and its logic stays unit-testable.
//
// Exit codes (deskkit contract): 0 ok · 5 refused · 6 unverifiable.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskcalibrate -- monthly reviewer-calibration sample + report.

USAGE:
  deskcalibrate sample --month YYYY-MM --seed N (--vendor V | --human LOGIN) \
                       (--prs a,b,c | --candidates-file F) [--reviewer-vendor V] [--n 10]
  deskcalibrate report --results results.json

The re-review MUST use a different model vendor than the reviewer role's App
(ASSAY_REVIEWER_VENDOR) or a human checklist — a same-vendor re-review measures
nothing. deskcalibrate REFUSES a same-vendor sample.

Exit: 0 ok · 5 refused · 6 unverifiable.`

func main() {
	os.Exit(deskkit.PerfRun("deskcalibrate", nil, os.Args[1:], func(a []string) int {
		return execute(a, os.Stdout, os.Stderr, startup)
	}))
}

// startup is what every real run does first, after its command line parsed and its values
// passed their checks, and never for help or version (help reads nothing).
//
// deskcalibrate reads the roster to learn the reviewer role's vendor, so ciEligible=false:
// config-home file only, never the environment, in CI as well as locally — the declaration
// every acting read verb makes.
func startup(stderr io.Writer) int {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(stderr)
	if !deskkit.CheckVerbActivation(stderr) {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// run is execute without the process startup.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, nil)
}

// sampleOpts are the typed values `sample` hands its handler.
type sampleOpts struct {
	month, vendor, human string
	reviewerVendor       string // resolved: --reviewer-vendor when non-blank, else the roster
	prs, candidatesFile  string
	seed                 int64
	n                    int
}

// sampleFn and reportFn are the typed handlers the command tree calls; the contract tests
// replace them to observe the values that reach them.
var (
	sampleFn = cmdSample
	reportFn = cmdReport
)

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// versionLine is what --version, -version and the legacy `version` word print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskcalibrate sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// sampleBindings are the `sample` settings: flags and defaults, plus the reviewer role's
// vendor from the environment. That vendor is two keys so a blank --reviewer-vendor still
// falls through to ASSAY_REVIEWER_VENDOR, exactly as the hand-rolled parser read it.
func sampleBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "month", Kind: cli.String, Flag: "month", Usage: "prior calendar month, YYYY-MM (recorded in the sample)"},
		// A string, parsed as a 64-bit integer, so the seed's range is the same on every platform.
		{Key: "seed", Kind: cli.String, Flag: "seed", Usage: "sampling seed `N` (recorded; same seed → same sample)"},
		{Key: "vendor", Kind: cli.String, Flag: "vendor", Usage: "re-reviewer model vendor (must differ from the reviewer role's vendor)"},
		{Key: "human", Kind: cli.String, Flag: "human", Usage: "re-reviewer human login (a human checklist is independent of every model vendor)"},
		{Key: "reviewer-vendor", Kind: cli.String, Flag: "reviewer-vendor",
			Usage: "reviewer role's model vendor (default: " + deskkit.EnvReviewerVendor + " from roster.env)"},
		{Key: "roster-vendor", Kind: cli.String, Env: []string{deskkit.EnvReviewerVendor}},
		{Key: "prs", Kind: cli.String, Flag: "prs", Usage: "comma-separated candidate APPROVED PR numbers"},
		{Key: "candidates-file", Kind: cli.String, Flag: "candidates-file", Usage: "file of candidate APPROVED PR numbers, one per line"},
		// -n is one letter, so the Go-flag spelling is the shorthand.
		{Key: "n", Kind: cli.Int, Flag: "n", Short: "n", Default: DefaultSampleSize, Usage: "sample size"},
	}
}

// reportBindings are the `report` settings. A results file without a reviewer vendor takes
// the roster's, as before.
func reportBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "results", Kind: cli.String, Flag: "results", Usage: "re-review results JSON (a deskcalibrate Report)"},
		{Key: "roster-vendor", Kind: cli.String, Env: []string{deskkit.EnvReviewerVendor}},
	}
}

// execute runs one invocation against a fresh command tree. pre is the process startup (nil
// in the in-package tests).
func execute(args []string, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("deskcalibrate", "monthly reviewer-calibration sample + report")
		root.Long = usage
		// An unknown subcommand is a refusal before anything runs.
		root.Args = func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown subcommand %s", strconv.Quote(args[0]))
			}
			return nil
		}
		// No subcommand is a refusal with the usage on stderr, as before.
		root.RunE = func(*cobra.Command, []string) error {
			fmt.Fprintln(stderr, usage)
			return exitErr(deskkit.ExitRefused)
		}

		sample := &cobra.Command{Use: "sample", Short: "emit the seeded sample + the re-review worklist", Args: cobra.ArbitraryArgs}
		sset := cli.Declare(sample, sampleBindings()...)
		sample.RunE = func(*cobra.Command, []string) error {
			return finish(stderr, runSample(sset, stdout, stderr, pre))
		}

		report := &cobra.Command{Use: "report", Short: "render the monthly report markdown from a results JSON", Args: cobra.ArbitraryArgs}
		rset := cli.Declare(report, reportBindings()...)
		report.RunE = func(*cobra.Command, []string) error {
			return finish(stderr, runReport(rset, stdout, stderr, pre))
		}

		root.AddCommand(sample, report, &cobra.Command{Use: "version", Short: "print the version", Hidden: true,
			Args: cobra.ArbitraryArgs,
			RunE: func(*cobra.Command, []string) error {
				fmt.Fprintln(stdout, versionLine())
				return nil
			}})
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy refusal code (5), not the adapter's default.
		UsageExit:    deskkit.ExitRefused,
		GoFlagCompat: true,
	})
}

// finish reports a handler error the way the hand-rolled dispatcher did and keeps its code.
func finish(stderr io.Writer, err error) error {
	if err == nil {
		return nil
	}
	var coded exitErr
	if e, ok := err.(exitErr); ok {
		coded = e
	} else {
		fmt.Fprintln(stderr, "deskcalibrate:", err.Error())
		coded = exitErr(deskkit.ExitCodeOf(err))
	}
	return coded
}

func resolve(set *cli.Set) (*cli.Values, error) {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		return nil, deskkit.Refused(err.Error())
	}
	if onResolve != nil {
		onResolve(v)
	}
	return v, nil
}

// runStartup runs the process startup when there is one.
func runStartup(stderr io.Writer, pre func(io.Writer) int) error {
	if pre == nil {
		return nil
	}
	if code := pre(stderr); code != deskkit.ExitOK {
		return exitErr(code)
	}
	return nil
}

// runSample resolves and checks `sample`'s values, then runs the startup and the handler, in
// that order: a bad value is refused before anything is read or echoed.
func runSample(set *cli.Set, stdout, stderr io.Writer, pre func(io.Writer) int) error {
	v, err := resolve(set)
	if err != nil {
		return err
	}
	o := sampleOpts{
		month: v.String("month"), vendor: v.String("vendor"), human: v.String("human"),
		reviewerVendor: resolveReviewerVendor(v.String("reviewer-vendor"), v.String("roster-vendor")),
		prs:            v.String("prs"), candidatesFile: v.String("candidates-file"), n: v.Int("n"),
	}
	if strings.TrimSpace(o.month) == "" {
		return deskkit.Refused("--month YYYY-MM is required (the sample is recorded against it)")
	}
	if v.Source("seed") != cli.Flag {
		return deskkit.Refused("--seed is required and recorded — an unseeded sample is not reproducible")
	}
	if o.seed, err = strconv.ParseInt(v.String("seed"), 0, 64); err != nil {
		return deskkit.Refused(fmt.Sprintf("invalid value %q for flag --seed: parse error", v.String("seed")))
	}
	if err := runStartup(stderr, pre); err != nil {
		return err
	}
	return sampleFn(o, stdout)
}

func runReport(set *cli.Set, stdout, stderr io.Writer, pre func(io.Writer) int) error {
	v, err := resolve(set)
	if err != nil {
		return err
	}
	results := v.String("results")
	if strings.TrimSpace(results) == "" {
		return deskkit.Refused("--results <file> is required")
	}
	if err := runStartup(stderr, pre); err != nil {
		return err
	}
	return reportFn(results, v.String("roster-vendor"), stdout)
}

// resolveReviewerVendor prefers an explicit --reviewer-vendor, else the roster's
// ASSAY_REVIEWER_VENDOR. A missing value is left empty; CheckIndependence turns
// that into the fail-closed refusal.
func resolveReviewerVendor(flagVal, rosterVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		return flagVal
	}
	return rosterVal
}

func cmdSample(o sampleOpts, stdout io.Writer) error {
	rr := ReReviewer{Vendor: o.vendor, Human: o.human}
	if rr.isHuman() && strings.TrimSpace(o.vendor) != "" {
		return deskkit.Refused("pass either --vendor or --human, not both")
	}
	rv := o.reviewerVendor
	// SPOF check first: refuse before doing any sampling work.
	if err := CheckIndependence(rv, rr); err != nil {
		return err
	}

	candidates, err := loadCandidates(o.prs, o.candidatesFile)
	if err != nil {
		return err
	}
	sample, err := SeededSample(candidates, o.seed, o.n)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "reviewer-calibration sample — %s\n", o.month)
	fmt.Fprintf(stdout, "reviewer-vendor: %s\n", normalizeVendor(rv))
	fmt.Fprintf(stdout, "re-reviewer: %s\n", rr.Identity())
	fmt.Fprintf(stdout, "seed: %d\n", o.seed)
	fmt.Fprintf(stdout, "candidates: %d\n", len(candidates))
	fmt.Fprintf(stdout, "sample (%d):\n", len(sample))
	for _, pr := range sample {
		fmt.Fprintf(stdout, "  - #%d\n", pr)
	}
	fmt.Fprintf(stdout, "\nnext: re-review each PR above under %s, then feed the outcomes to `deskcalibrate report`.\n", rr.Identity())
	return nil
}

func cmdReport(results, rosterVendor string, stdout io.Writer) error {
	raw, err := os.ReadFile(results)
	if err != nil {
		return deskkit.RefusedWithCause("cannot read results file", err)
	}
	var rep Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		return deskkit.RefusedWithCause("results file does not parse as a calibration Report", err)
	}
	if strings.TrimSpace(rep.ReviewerVendor) == "" {
		rep.ReviewerVendor = rosterVendor
	}
	md, err := rep.Render()
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, md)
	return nil
}

// loadCandidates reads the candidate APPROVED PR numbers from --prs or
// --candidates-file (exactly one).
func loadCandidates(prs, file string) ([]int, error) {
	prs = strings.TrimSpace(prs)
	file = strings.TrimSpace(file)
	switch {
	case prs != "" && file != "":
		return nil, deskkit.Refused("pass either --prs or --candidates-file, not both")
	case prs != "":
		return parsePRList(strings.Split(prs, ","))
	case file != "":
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, deskkit.RefusedWithCause("cannot read candidates file", err)
		}
		return parsePRList(strings.Fields(string(raw)))
	default:
		return nil, deskkit.Refused("supply candidate APPROVED PRs with --prs or --candidates-file")
	}
}

func parsePRList(tokens []string) ([]int, error) {
	out := make([]int, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#"))
		if t == "" {
			continue
		}
		v, err := strconv.Atoi(t)
		if err != nil {
			return nil, deskkit.Refused(fmt.Sprintf("candidate %q is not a PR number", t))
		}
		if v <= 0 {
			return nil, deskkit.Refused(fmt.Sprintf("candidate %d is not a positive PR number", v))
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, deskkit.Refused("no candidate PR numbers parsed")
	}
	return out, nil
}
