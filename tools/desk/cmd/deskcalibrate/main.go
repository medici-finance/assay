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
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

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
	// deskcalibrate reads the roster to learn the reviewer role's vendor, so
	// ciEligible=false: config-home file only, never the environment, in CI as
	// well as locally — the declaration every acting read verb makes.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskcalibrate sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "sample":
		err = cmdSample(rest, stdout)
	case "report":
		err = cmdReport(rest, stdout)
	default:
		fmt.Fprintf(stderr, "deskcalibrate: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(stderr, "deskcalibrate:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}

// resolveReviewerVendor prefers an explicit --reviewer-vendor, else the roster's
// ASSAY_REVIEWER_VENDOR. A missing value is left empty; CheckIndependence turns
// that into the fail-closed refusal.
func resolveReviewerVendor(flagVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		return flagVal
	}
	return os.Getenv(deskkit.EnvReviewerVendor)
}

func cmdSample(args []string, stdout *os.File) error {
	fs := flag.NewFlagSet("sample", flag.ContinueOnError)
	month := fs.String("month", "", "prior calendar month, YYYY-MM (recorded in the sample)")
	seed := fs.Int64("seed", 0, "sampling seed (recorded; same seed → same sample)")
	seedSet := false
	vendor := fs.String("vendor", "", "re-reviewer model vendor (must differ from the reviewer role's vendor)")
	human := fs.String("human", "", "re-reviewer human login (a human checklist is independent of every model vendor)")
	reviewerVendor := fs.String("reviewer-vendor", "", "reviewer role's model vendor (default: "+deskkit.EnvReviewerVendor+" from roster.env)")
	prs := fs.String("prs", "", "comma-separated candidate APPROVED PR numbers")
	candidatesFile := fs.String("candidates-file", "", "file of candidate APPROVED PR numbers, one per line")
	n := fs.Int("n", DefaultSampleSize, "sample size")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(err.Error())
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})
	if strings.TrimSpace(*month) == "" {
		return deskkit.Refused("--month YYYY-MM is required (the sample is recorded against it)")
	}
	if !seedSet {
		return deskkit.Refused("--seed is required and recorded — an unseeded sample is not reproducible")
	}

	rr := ReReviewer{Vendor: *vendor, Human: *human}
	if rr.isHuman() && strings.TrimSpace(*vendor) != "" {
		return deskkit.Refused("pass either --vendor or --human, not both")
	}
	rv := resolveReviewerVendor(*reviewerVendor)
	// SPOF check first: refuse before doing any sampling work.
	if err := CheckIndependence(rv, rr); err != nil {
		return err
	}

	candidates, err := loadCandidates(*prs, *candidatesFile)
	if err != nil {
		return err
	}
	sample, err := SeededSample(candidates, *seed, *n)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "reviewer-calibration sample — %s\n", *month)
	fmt.Fprintf(stdout, "reviewer-vendor: %s\n", normalizeVendor(rv))
	fmt.Fprintf(stdout, "re-reviewer: %s\n", rr.Identity())
	fmt.Fprintf(stdout, "seed: %d\n", *seed)
	fmt.Fprintf(stdout, "candidates: %d\n", len(candidates))
	fmt.Fprintf(stdout, "sample (%d):\n", len(sample))
	for _, pr := range sample {
		fmt.Fprintf(stdout, "  - #%d\n", pr)
	}
	fmt.Fprintf(stdout, "\nnext: re-review each PR above under %s, then feed the outcomes to `deskcalibrate report`.\n", rr.Identity())
	return nil
}

func cmdReport(args []string, stdout *os.File) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	results := fs.String("results", "", "re-review results JSON (a deskcalibrate Report)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(err.Error())
	}
	if strings.TrimSpace(*results) == "" {
		return deskkit.Refused("--results <file> is required")
	}
	raw, err := os.ReadFile(*results)
	if err != nil {
		return deskkit.RefusedWithCause("cannot read results file", err)
	}
	var rep Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		return deskkit.RefusedWithCause("results file does not parse as a calibration Report", err)
	}
	if strings.TrimSpace(rep.ReviewerVendor) == "" {
		rep.ReviewerVendor = os.Getenv(deskkit.EnvReviewerVendor)
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
