package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// planFile is one source file and how many mutants a dry run found runnable in it.
type planFile struct {
	Name     string
	Runnable int
}

// plan is the slice of a package's files one day's run covers.
type plan struct {
	Bin      int // 0-based index of today's bin
	Bins     int
	Capacity int // mutants the budget is expected to fit
	Files    []string
	Runnable int
	Exclude  string // one Gremlins --exclude-files regex; empty when nothing is excluded
}

// capacityFor estimates how many mutants fit in the budget. Gremlins re-runs the
// package's tests once per mutant, so one mutant costs about one coverage run,
// and the run first spends one coverage run gathering coverage.
func capacityFor(budget, coverage float64, workers int) int {
	per := math.Max(coverage, 1)
	left := budget - coverage
	if left < per || workers < 1 {
		return 1
	}
	return int(left * float64(workers) / per)
}

// makePlan packs the files, in name order, into contiguous bins of at most
// capacity runnable mutants and picks bin day mod bins. A file larger than the
// capacity gets a bin of its own and will run out of budget; its finished
// mutants are still read back from the log. Rotating by day means every file is
// visited once every `Bins` days, instead of the same first files every day.
func makePlan(files []planFile, capacity, day int) (plan, error) {
	sorted := append([]planFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	var bins [][]planFile
	n := 0
	for _, f := range sorted {
		if strings.Contains(f.Name, ",") {
			// Gremlins splits --exclude-files on commas, so the regex would break.
			return plan{}, fmt.Errorf("file name %q contains a comma; it cannot be expressed as an exclusion", f.Name)
		}
		if len(bins) == 0 || (n+f.Runnable > capacity && n > 0) {
			bins = append(bins, nil)
			n = 0
		}
		bins[len(bins)-1] = append(bins[len(bins)-1], f)
		n += f.Runnable
	}
	if len(bins) == 0 {
		return plan{Bins: 1, Capacity: capacity}, nil
	}
	if day < 0 {
		day = -day
	}
	p := plan{Bin: day % len(bins), Bins: len(bins), Capacity: capacity}
	for _, f := range bins[p.Bin] {
		p.Files = append(p.Files, f.Name)
		p.Runnable += f.Runnable
	}
	if len(bins) > 1 {
		var ex []string
		for i, b := range bins {
			if i == p.Bin {
				continue
			}
			for _, f := range b {
				ex = append(ex, regexp.QuoteMeta(f.Name))
			}
		}
		p.Exclude = "^(?:" + strings.Join(ex, "|") + ")$"
	}
	return p, nil
}

// dryRunFiles reads the runnable count per file from a Gremlins --dry-run report.
func dryRunFiles(path string) ([]planFile, error) {
	ms, err := readReport(path)
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	var out []planFile
	for _, m := range ms {
		i, ok := idx[m.File]
		if !ok {
			i = len(out)
			idx[m.File] = i
			out = append(out, planFile{Name: m.File})
		}
		if m.Status == stRunnable {
			out[i].Runnable++
		}
	}
	return out, nil
}

var coverageRE = regexp.MustCompile(`Gathering coverage\.\.\. done in (\S+)`)

// coverageFromLog reads the coverage time Gremlins prints, e.g.
// "Gathering coverage... done in 2.3147645s". It is the uncached cost of one
// run of the package's tests, which is what one mutant costs.
func coverageFromLog(path string) (float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	m := coverageRE.FindSubmatch(b)
	if m == nil {
		return 0, fmt.Errorf("%s: no coverage timing line", path)
	}
	d, err := time.ParseDuration(string(m[1]))
	if err != nil {
		return 0, fmt.Errorf("%s: coverage timing %q: %v", path, m[1], err)
	}
	return d.Seconds(), nil
}

// runPlan prints two lines: a human summary, then the exclusion regex (empty
// when today's bin is the whole package).
func runPlan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dry := fs.String("dry-run", "", "Gremlins --dry-run JSON report for the package")
	budget := fs.Float64("budget", 600, "seconds the package's run may take")
	coverage := fs.Float64("coverage", 0, "seconds the package's coverage run took (uncached); -log overrides")
	logPath := fs.String("log", "", "the dry run's log, to read the coverage time from")
	workers := fs.Int("workers", 2, "Gremlins --workers")
	day := fs.Int("day", 0, "day index; today's bin is day mod bins")
	if err := fs.Parse(args); err != nil {
		return exitBroke
	}
	if *dry == "" {
		fmt.Fprintln(stderr, "plan: -dry-run is required")
		return exitBroke
	}
	if *logPath != "" {
		c, err := coverageFromLog(*logPath)
		if err != nil {
			fmt.Fprintln(stderr, "plan:", err)
			return exitBroke
		}
		*coverage = c
	}
	files, err := dryRunFiles(*dry)
	if err != nil {
		fmt.Fprintln(stderr, "plan:", err)
		return exitBroke
	}
	p, err := makePlan(files, capacityFor(*budget, *coverage, *workers), *day)
	if err != nil {
		fmt.Fprintln(stderr, "plan:", err)
		return exitBroke
	}
	fmt.Fprintf(stdout, "bin %d/%d files %d runnable %d capacity %d\n", p.Bin+1, p.Bins, len(p.Files), p.Runnable, p.Capacity)
	fmt.Fprintln(stdout, p.Exclude)
	return 0
}
