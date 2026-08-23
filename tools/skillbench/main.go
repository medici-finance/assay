// skillbench — an AI-FREE reducer over session artifacts that measures what a
// skill or prompt overlay does to a coding session, adapting a same-agent-
// with-and-without-overlay benchmark method to the house.
//
// It is a pure reducer, NOT an agent runner. It never shells out to an agent,
// never reads GitHub or git, and makes no network call at all. Its only input
// is a directory laid out as two arms — `with-overlay/` and `without-overlay/`
// — each holding one subdirectory per run of committed artifacts (the run's
// git diff, an optional token/cost usage log, and a small run.json carrying
// wall time and the task-check result). Its only output is a markdown report of
// per-metric deltas with a per-metric `n`. Producing the runs themselves is a
// runbook step (dispatch N workers per arm on the same fixture tasks) — see
// README.md — and is deliberately OUTSIDE this program.
//
// The one discipline this tool exists to keep is the house three-state rule
// (the house AI-free collector pattern is the exemplar): A COULD-NOT-CHECK MUST NEVER BECOME A
// MEASURED VALUE. A run missing its usage log renders tokens/cost as
// `could-not-check`, never a zero; a delta is emitted only when both arms
// measured that metric over at least one run. The state is rendered in the
// cell, so a gap can never be read as a number.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stderr))
}

// run is the testable entry point. Exit codes are deliberately coarse: the
// three-state discipline lives in the REPORT CELLS, not the process exit, so a
// degraded-but-emittable report still exits 0 (its gaps are on the page). A
// nonzero exit means the tool could not run at all, not that a metric was
// missing.
//
//	0 — a report was written
//	1 — a usage error (bad flags, unwritable output)
//	2 — the arms directory itself could not be read (could-not-check the input)
func runCLI(argv []string, stderr *os.File) int {
	fs := flag.NewFlagSet("skillbench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		armsDir = fs.String("arms", "", "directory holding with-overlay/ and without-overlay/ arm subdirectories (required)")
		out     = fs.String("out", "", "report output path (default: reports/skillbench/<date>-<slug>.md under the repo root)")
		slug    = fs.String("overlay-slug", "", "overlay slug for the report header and default filename (default: base name of --arms)")
		dateStr = fs.String("date", "", "report date YYYY-MM-DD (default: today, UTC)")
	)
	if err := fs.Parse(argv); err != nil {
		return 1
	}
	if *armsDir == "" {
		fmt.Fprintln(stderr, "skillbench: --arms is required")
		fs.Usage()
		return 1
	}

	date, err := resolveDate(*dateStr)
	if err != nil {
		fmt.Fprintln(stderr, "skillbench:", err)
		return 1
	}

	overlaySlug := *slug
	if overlaySlug == "" {
		overlaySlug = slugify(filepath.Base(filepath.Clean(*armsDir)))
	}

	arms, err := loadArms(*armsDir)
	if err != nil {
		// The arms directory itself is unreachable: this is could-not-check the
		// whole input, distinct from a run inside it lacking a metric.
		fmt.Fprintln(stderr, "skillbench: cannot read arms directory:", err)
		return 2
	}

	rep := reduce(arms, overlaySlug, date)
	md := renderReport(rep)

	outPath := *out
	if outPath == "" {
		outPath = defaultOutPath(*armsDir, date, overlaySlug)
	}
	if dir := filepath.Dir(outPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(stderr, "skillbench: cannot create output directory:", err)
			return 1
		}
	}
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		fmt.Fprintln(stderr, "skillbench: cannot write report:", err)
		return 1
	}
	fmt.Fprintf(stderr, "skillbench: wrote %s\n", outPath)
	return 0
}

// resolveDate parses --date or defaults to today (UTC). UTC avoids skew between
// a local run and CI.
func resolveDate(s string) (string, error) {
	if s == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", fmt.Errorf("--date %q must be YYYY-MM-DD", s)
	}
	return t.Format("2006-01-02"), nil
}

// slugify reduces an arbitrary label to a filename-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "overlay"
	}
	return out
}

// defaultOutPath places the report at reports/skillbench/<date>-<slug>.md under
// the repo root, derived from the --arms path when it sits inside the tree, and
// otherwise relative to the working directory.
func defaultOutPath(armsDir, date, slug string) string {
	name := fmt.Sprintf("%s-%s.md", date, slug)
	abs, err := filepath.Abs(armsDir)
	if err == nil {
		// tools/skillbench/fixtures/<x> -> repo root is four up; but the arms
		// dir can be anywhere, so only special-case the in-tree fixtures path.
		if i := strings.Index(abs, string(filepath.Separator)+"tools"+string(filepath.Separator)+"skillbench"+string(filepath.Separator)); i >= 0 {
			root := abs[:i]
			return filepath.Join(root, "reports", "skillbench", name)
		}
	}
	return filepath.Join("reports", "skillbench", name)
}
