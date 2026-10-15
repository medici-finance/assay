// Command intake-eval scores intake-routing decisions against a gold set.
//
// It reads a directory of gold-set fixtures (one YAML file per inbound, each
// carrying the exit it should have taken) and a predictions file (fixture id →
// predicted exit), and reports precision, recall, F1 and support for each of
// the five intake exits. Given a threshold config it also returns a PASS/FAIL
// verdict, wired to the process exit code. The tool bundles no corpus: the
// consumer supplies the gold set at run time.
//
// Exit codes: 0 scored (PASS, no thresholds, or advisory), 1 verdict FAIL,
// 2 usage error, unreadable input or a gold set that fails preflight lint.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	exitOK    = 0
	exitFail  = 1
	exitInput = 2
)

// Report is the --json output.
type Report struct {
	Tool               string   `json:"tool"`
	MethodologyVersion string   `json:"methodology_version"`
	Fixtures           int      `json:"fixtures"`
	PendingHuman       []string `json:"pending_human"`
	Stale              []string `json:"stale"`
	*Score
	AccuracyNote string   `json:"accuracy_note"`
	Verdict      *Verdict `json:"verdict"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("intake-eval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	gold := fs.String("gold", "", "directory of gold-set fixtures (one *.yaml per inbound) (required)")
	predPath := fs.String("pred", "", "predictions file: YAML or JSON mapping of fixture id to predicted exit (required)")
	thrPath := fs.String("thresholds", "", "threshold config (JSON); enables the PASS/FAIL verdict")
	basePath := fs.String("baseline", "", "previous --json report whose per-exit F1 is the regression baseline")
	mver := fs.String("methodology-version", "", "current methodology version; fixtures behind it are flagged STALE (default: newest on any fixture)")
	asJSON := fs.Bool("json", false, "emit the machine-readable JSON report instead of the table")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: intake-eval --gold <dir> --pred <file> [--thresholds <file>] [--baseline <report.json>] [--methodology-version <v>] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitInput
	}
	if *gold == "" || *predPath == "" || fs.NArg() > 0 {
		fs.Usage()
		return exitInput
	}
	if *basePath != "" && *thrPath == "" {
		fmt.Fprintln(stderr, "intake-eval: --baseline needs --thresholds (the regression bound lives there)")
		return exitInput
	}

	gs, err := loadGold(*gold, *mver)
	if err != nil {
		fmt.Fprintln(stderr, "intake-eval:", err)
		return exitInput
	}
	pred, err := loadPredictions(*predPath)
	if err != nil {
		fmt.Fprintln(stderr, "intake-eval:", err)
		return exitInput
	}
	sc, err := score(gs, pred)
	if err != nil {
		fmt.Fprintln(stderr, "intake-eval:", err)
		return exitInput
	}
	rep := &Report{
		Tool:               "intake-eval",
		MethodologyVersion: gs.CurrentVersion,
		Fixtures:           len(gs.Fixtures),
		PendingHuman:       nonNil(gs.PendingHuman),
		Stale:              nonNil(gs.Stale),
		Score:              sc,
		AccuracyNote:       "informational only; the verdict is per-exit and never uses accuracy",
	}
	if *thrPath != "" {
		t, err := loadThresholds(*thrPath)
		if err != nil {
			fmt.Fprintln(stderr, "intake-eval:", err)
			return exitInput
		}
		if *basePath != "" {
			b, err := loadBaselineReport(*basePath)
			if err != nil {
				fmt.Fprintln(stderr, "intake-eval:", err)
				return exitInput
			}
			t.Baseline = b
		}
		v, err := evaluate(t, sc)
		if err != nil {
			fmt.Fprintln(stderr, "intake-eval:", err)
			return exitInput
		}
		rep.Verdict = v
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(stderr, "intake-eval:", err)
			return exitInput
		}
	} else {
		writeTable(stdout, rep)
	}
	if rep.Verdict != nil && rep.Verdict.Result == "FAIL" && !rep.Verdict.Advisory {
		return exitFail
	}
	return exitOK
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func fmtMetric(v *float64) string {
	if v == nil {
		return "unmeasured"
	}
	return fmt.Sprintf("%.3f", *v)
}

func writeTable(w io.Writer, r *Report) {
	fmt.Fprintf(w, "intake-eval: %d scored of %d fixtures (methodology %s)\n", r.Scored, r.Fixtures, r.MethodologyVersion)
	fmt.Fprintf(w, "%-18s %7s %9s %10s %10s %10s\n", "EXIT", "SUPPORT", "PREDICTED", "PRECISION", "RECALL", "F1")
	for _, e := range exits {
		x := r.Exits[e]
		fmt.Fprintf(w, "%-18s %7d %9d %10s %10s %10s\n", e, x.Support, x.Predicted, fmtMetric(x.Precision), fmtMetric(x.Recall), fmtMetric(x.F1))
	}
	fmt.Fprintf(w, "accuracy %s (%s)\n", fmtMetric(r.Accuracy), r.AccuracyNote)
	if len(r.Missing) > 0 {
		fmt.Fprintf(w, "missing predictions (%d, scored as misses): %s\n", len(r.Missing), strings.Join(r.Missing, ", "))
	}
	if len(r.IgnoredExcluded) > 0 {
		fmt.Fprintf(w, "predictions for excluded fixtures ignored: %d\n", len(r.IgnoredExcluded))
	}
	if len(r.PendingHuman) > 0 {
		fmt.Fprintf(w, "excluded, awaiting human confirmation (%d): %s\n", len(r.PendingHuman), strings.Join(r.PendingHuman, ", "))
	}
	if len(r.Stale) > 0 {
		fmt.Fprintf(w, "STALE: %d fixture(s) behind methodology %s: %s\n", len(r.Stale), r.MethodologyVersion, strings.Join(r.Stale, ", "))
	}
	v := r.Verdict
	if v == nil {
		fmt.Fprintln(w, "verdict: none (no --thresholds)")
		return
	}
	label := v.Result
	if v.Advisory {
		label += " (advisory: reported, not enforced)"
	}
	fmt.Fprintf(w, "verdict: %s\n", label)
	for _, c := range v.Checks {
		if c.Status == "pass" {
			continue
		}
		what := fmt.Sprintf("%s floor %.3f", c.Metric, c.Floor)
		if c.Rule == "regression" {
			what = fmt.Sprintf("f1 regression floor %.3f (baseline %.3f)", c.Floor, *c.Baseline)
		}
		fmt.Fprintf(w, "  %-10s %-18s %s, got %s\n", strings.ToUpper(c.Status), c.Exit, what, fmtMetric(c.Value))
	}
}
