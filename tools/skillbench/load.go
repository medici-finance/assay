// load.go — the reducer's INPUT side: the two-arm artifact layout, read into
// three-state per-run metrics.
//
// Every read here is three-state at heart — present-and-readable, present-and-
// unusable, or absent — and the last two both collapse to `could-not-check` at
// the cell, with a distinguishing note. Nothing in this file shells out, reads
// a token, or talks to GitHub: the artifacts are already on disk, committed by
// whoever produced the runs.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The two arms and their fixed iteration order. Overlay on/off is the ONLY
// difference between them — a dispatch-prompt difference, nothing else (see
// README.md's runbook).
const (
	armWith    = "with-overlay"
	armWithout = "without-overlay"
)

var armOrder = []string{armWith, armWithout}

// Per-run artifact filenames. These three files are the whole input contract.
const (
	fileDiff  = "diff.patch" // the run's git diff
	fileUsage = "usage.json" // OPTIONAL token/cost log
	fileRun   = "run.json"   // wall time + task-check result
)

// Metric keys. tokens and cost_usd are the two metrics that are only present
// when a usage log is (the brief's could-not-check path).
const (
	mDiffLines    = "diff_lines"
	mFilesTouched = "files_touched"
	mTokens       = "tokens"
	mCost         = "cost_usd"
	mWall         = "wall_seconds"
	mCheck        = "check_pass_rate"
)

// cellState is the two rendered states of any figure. `cellCouldNotCheck` is
// never a zero — that separation is the whole point of the tool.
type cellState int

const (
	cellMeasured cellState = iota
	cellCouldNotCheck
)

// metricMeta is the fixed, ordered description of the metric set (adapted from
// the same-agent-with-and-without-overlay benchmark method).
type metricMeta struct {
	Key         string
	Label       string
	Float       bool // render the mean with decimals (cost)
	Rate        bool // value is a 0..1 pass rate; delta is in percentage points
	LowerBetter bool // direction for the verdict block
}

var metricOrder = []metricMeta{
	{Key: mDiffLines, Label: "Diff lines (added+removed)", LowerBetter: true},
	{Key: mFilesTouched, Label: "Files touched", LowerBetter: true},
	{Key: mTokens, Label: "Tokens", LowerBetter: true},
	{Key: mCost, Label: "Cost (USD)", Float: true, LowerBetter: true},
	{Key: mWall, Label: "Wall time (s)", LowerBetter: true},
	{Key: mCheck, Label: "Task-check pass rate", Rate: true, LowerBetter: false},
}

// runMetric is the three-state read of one metric for one run.
type runMetric struct {
	State cellState
	Value float64
	Note  string // why, when could-not-check
}

func measured(v float64) runMetric { return runMetric{State: cellMeasured, Value: v} }
func couldNotCheck(n string) runMetric {
	return runMetric{State: cellCouldNotCheck, Note: n}
}

// run is one run's full set of three-state metrics.
type run struct {
	Name    string
	Metrics map[string]runMetric
}

// arm is one arm's loaded runs.
type arm struct {
	Name    string
	Present bool   // the arm subdirectory existed and was a readable directory
	Note    string // why not, when !Present
	Runs    []run
}

// loadArms reads the two-arm layout. It errors ONLY when the top-level arms
// directory cannot be stat'd — a could-not-check of the whole input. A missing
// or empty arm subdirectory is a state carried on the arm, not an error,
// because "this arm produced no runs" is exactly what a report has to be able
// to say rather than fail on.
func loadArms(dir string) (map[string]*arm, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}
	out := map[string]*arm{}
	for _, name := range armOrder {
		a := &arm{Name: name}
		ap := filepath.Join(dir, name)
		info, err := os.Stat(ap)
		if err != nil || !info.IsDir() {
			a.Present = false
			a.Note = fmt.Sprintf("arm directory %q is not present under %s", name, dir)
			out[name] = a
			continue
		}
		entries, err := os.ReadDir(ap)
		if err != nil {
			a.Present = false
			a.Note = fmt.Sprintf("arm directory %q is present but unreadable: %v", name, err)
			out[name] = a
			continue
		}
		a.Present = true
		var runNames []string
		for _, e := range entries {
			if e.IsDir() {
				runNames = append(runNames, e.Name())
			}
		}
		sort.Strings(runNames)
		for _, rn := range runNames {
			a.Runs = append(a.Runs, loadRun(filepath.Join(ap, rn), rn))
		}
		out[name] = a
	}
	return out, nil
}

// loadRun reads one run directory into three-state metrics. Every absent or
// unusable artifact yields could-not-check for the metrics it would have fed —
// never a zero.
func loadRun(dir, name string) run {
	r := run{Name: name, Metrics: map[string]runMetric{}}

	// diff.patch -> diff_lines, files_touched
	if data, err := os.ReadFile(filepath.Join(dir, fileDiff)); err != nil {
		note := fmt.Sprintf("no %s in run %q", fileDiff, name)
		r.Metrics[mDiffLines] = couldNotCheck(note)
		r.Metrics[mFilesTouched] = couldNotCheck(note)
	} else {
		add, del, files := parseDiff(data)
		r.Metrics[mDiffLines] = measured(float64(add + del))
		r.Metrics[mFilesTouched] = measured(float64(files))
	}

	// usage.json -> tokens, cost_usd. OPTIONAL: a missing usage log is the
	// canonical could-not-check case and must never render as a measured zero.
	if data, err := os.ReadFile(filepath.Join(dir, fileUsage)); err != nil {
		note := fmt.Sprintf("no %s — a missing usage log is not a zero", fileUsage)
		r.Metrics[mTokens] = couldNotCheck(note)
		r.Metrics[mCost] = couldNotCheck(note)
	} else {
		var u struct {
			Tokens  *float64 `json:"tokens"`
			CostUSD *float64 `json:"cost_usd"`
		}
		if err := json.Unmarshal(data, &u); err != nil {
			note := fmt.Sprintf("%s did not parse: %v", fileUsage, err)
			r.Metrics[mTokens] = couldNotCheck(note)
			r.Metrics[mCost] = couldNotCheck(note)
		} else {
			if u.Tokens != nil {
				r.Metrics[mTokens] = measured(*u.Tokens)
			} else {
				r.Metrics[mTokens] = couldNotCheck(fmt.Sprintf("%s has no tokens field", fileUsage))
			}
			if u.CostUSD != nil {
				r.Metrics[mCost] = measured(*u.CostUSD)
			} else {
				r.Metrics[mCost] = couldNotCheck(fmt.Sprintf("%s has no cost_usd field", fileUsage))
			}
		}
	}

	// run.json -> wall_seconds, check_pass_rate
	if data, err := os.ReadFile(filepath.Join(dir, fileRun)); err != nil {
		note := fmt.Sprintf("no %s in run %q", fileRun, name)
		r.Metrics[mWall] = couldNotCheck(note)
		r.Metrics[mCheck] = couldNotCheck(note)
	} else {
		var m struct {
			WallSeconds *float64 `json:"wall_seconds"`
			Check       *string  `json:"check"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			note := fmt.Sprintf("%s did not parse: %v", fileRun, err)
			r.Metrics[mWall] = couldNotCheck(note)
			r.Metrics[mCheck] = couldNotCheck(note)
		} else {
			if m.WallSeconds != nil {
				r.Metrics[mWall] = measured(*m.WallSeconds)
			} else {
				r.Metrics[mWall] = couldNotCheck(fmt.Sprintf("%s has no wall_seconds field", fileRun))
			}
			if m.Check != nil {
				switch strings.ToLower(strings.TrimSpace(*m.Check)) {
				case "pass":
					r.Metrics[mCheck] = measured(1)
				case "fail":
					r.Metrics[mCheck] = measured(0)
				default:
					r.Metrics[mCheck] = couldNotCheck(
						fmt.Sprintf("%s check=%q is neither \"pass\" nor \"fail\"", fileRun, *m.Check))
				}
			} else {
				r.Metrics[mCheck] = couldNotCheck(fmt.Sprintf("%s has no check field", fileRun))
			}
		}
	}

	return r
}

// parseDiff counts added and removed content lines and touched files from a
// unified git diff. File-header lines (`+++`, `---`) are content-neutral and
// excluded; a file is counted per `diff --git` header.
func parseDiff(b []byte) (added, removed, files int) {
	for _, ln := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(ln, "diff --git "):
			files++
		case strings.HasPrefix(ln, "+++"), strings.HasPrefix(ln, "---"):
			// file headers, not content
		case strings.HasPrefix(ln, "+"):
			added++
		case strings.HasPrefix(ln, "-"):
			removed++
		}
	}
	return
}
