package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/qualgen/verifier"
)

// suspects.go is leg 1 of the sweep lane: the deterministic suspects
// front-end. It runs the CONFIGURED set of existing external linters against
// the target repo and NORMALIZES their output into suspect records. It authors
// no source-semantics parser of its own (spec §2 non-goal): it shells out and
// parses tool OUTPUT only. A category with no configured tool, or a configured
// tool that is not installed, is could-not-measure FOR THAT CATEGORY — never a
// silent zero (spec §3.2).

// SweepCategories is the canonical, fixed set of slop categories the lane
// reports on. The set is a concept, not a tool list: which tool (if any) feeds
// each category is per-target CONFIGURATION. A category is always present in
// the report — measured, measured-zero, or could-not-measure — so a report can
// never read "clean" on a category it merely never looked at.
var SweepCategories = []string{
	"dead-code",
	"swallowed-error",
	"module-size",
	"duplication",
}

// defaultSuspectPattern matches the near-universal `file:line[:col]: message`
// diagnostic shape emitted by go vet, staticcheck, and most Unix linters. A
// tool whose output differs supplies its own Pattern in config; qualgen parses
// output, it does not embed a parser per tool.
const defaultSuspectPattern = `^(?P<file>[^:]+):(?P<line>\d+)(?::(?P<col>\d+))?:\s*(?P<msg>.*)$`

// ToolConfig configures the linter feeding ONE category. Command is the argv
// run (in the target repo's directory) whose stdout is parsed; an empty Command
// means the category has no tool and is could-not-measure. Rule labels which
// check the tool represents (carried into every suspect it nominates). Pattern
// overrides defaultSuspectPattern when the tool's output shape differs; it must
// name capture groups file, line, and msg (col and end optional).
type ToolConfig struct {
	Command []string `json:"command"`
	Rule    string   `json:"rule,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

// SweepConfig is the whole sweep configuration (--config): the per-category
// tool set and the verifier-adapter selection. It is deliberately data-only so
// a target's tool set and agent choice are configuration, never code.
type SweepConfig struct {
	// Tools maps a canonical category to its linter. A category absent from
	// this map (or present with an empty Command) is could-not-measure.
	Tools map[string]ToolConfig `json:"tools"`
	// Verifier selects the leg-2 agent adapter.
	Verifier VerifierConfig `json:"verifier"`
}

// VerifierConfig selects the leg-2 verifier. Only the offline "fixture"
// reference adapter is wired in this brief; a live headless-agent kind is added
// here as configuration without touching the orchestrator.
type VerifierConfig struct {
	Kind string `json:"kind"`
	// Scripts is the scripted-verdict file for kind "fixture".
	Scripts string `json:"scripts,omitempty"`
}

// CategoryResult is leg 1's outcome for one category: the suspects it
// nominated, plus the three-state measure describing whether the category could
// be measured at all. Count carries the number of suspects when measured; a
// could-not-measure category names why.
type CategoryResult struct {
	Category string
	Suspects []verifier.Suspect
	State    Measure[int]
}

// commandRunner runs a linter argv in dir and returns its stdout. It is a seam
// so tests drive canned outputs without a live toolchain; execRunner is the
// production implementation. A non-nil error with lookErr true means the tool
// binary is not installed (the could-not-measure trigger distinct from a tool
// that ran and failed).
type commandRunner func(dir string, argv []string) (stdout string, lookErr bool, err error)

// execRunner is the production commandRunner: it resolves argv[0] on PATH
// (reporting a not-found as lookErr) and runs the command read-only in dir.
func execRunner(dir string, argv []string) (string, bool, error) {
	if len(argv) == 0 {
		return "", false, fmt.Errorf("empty command")
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", true, fmt.Errorf("tool %q not installed: %w", argv[0], err)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.Output()
	// Linters conventionally exit non-zero when they find issues; that is a
	// successful measurement, not a failure. Only a genuine start/IO failure
	// (already screened for not-installed above) is treated as an error.
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return string(out), false, nil
		}
		return string(out), false, err
	}
	return string(out), false, nil
}

// runSuspects runs leg 1 over every canonical category using cfg and runner.
// repoPath is the target repo (read-only). It returns one CategoryResult per
// canonical category, in SweepCategories order, so the report always covers the
// full set.
func runSuspects(repoPath string, cfg SweepConfig, runner commandRunner) []CategoryResult {
	results := make([]CategoryResult, 0, len(SweepCategories))
	for _, cat := range SweepCategories {
		results = append(results, runCategory(repoPath, cat, cfg.Tools[cat], runner))
	}
	return results
}

// runCategory runs the one configured linter for a category and normalizes its
// output. No command configured, or a not-installed tool, yields
// could-not-measure. A tool that ran but nominated nothing is measured-zero —
// distinct from could-not-measure by construction.
func runCategory(repoPath, category string, tc ToolConfig, runner commandRunner) CategoryResult {
	res := CategoryResult{Category: category}
	if len(tc.Command) == 0 {
		res.State = CouldNotMeasure[int](fmt.Sprintf("no tool configured for category %q", category))
		return res
	}

	out, lookErr, err := runner(repoPath, tc.Command)
	if lookErr {
		res.State = CouldNotMeasure[int](fmt.Sprintf("tool %q for category %q not installed", tc.Command[0], category))
		return res
	}
	if err != nil {
		res.State = CouldNotMeasure[int](fmt.Sprintf("running %q for category %q: %v", strings.Join(tc.Command, " "), category, err))
		return res
	}

	pattern := tc.Pattern
	if pattern == "" {
		pattern = defaultSuspectPattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		res.State = CouldNotMeasure[int](fmt.Sprintf("compiling suspect pattern for category %q: %v", category, err))
		return res
	}

	suspects, perr := parseSuspects(category, tc, re, out)
	if perr != nil {
		res.State = CouldNotMeasure[int](fmt.Sprintf("parsing %q output for category %q: %v", tc.Command[0], category, perr))
		return res
	}
	res.Suspects = suspects
	if len(suspects) == 0 {
		res.State = MeasuredZero[int]()
		return res
	}
	res.State = Measured(len(suspects))
	return res
}

// parseSuspects normalizes one tool's stdout into suspect records via re. Each
// matched line becomes a suspect; a line the regex does not match is skipped
// (tool banners, summary lines) rather than failing the whole category.
func parseSuspects(category string, tc ToolConfig, re *regexp.Regexp, out string) ([]verifier.Suspect, error) {
	fileIdx := re.SubexpIndex("file")
	lineIdx := re.SubexpIndex("line")
	msgIdx := re.SubexpIndex("msg")
	endIdx := re.SubexpIndex("end")
	if fileIdx < 0 || lineIdx < 0 {
		return nil, fmt.Errorf("suspect pattern must name capture groups 'file' and 'line'")
	}

	var suspects []verifier.Suspect
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, err := strconv.Atoi(m[lineIdx])
		if err != nil {
			continue
		}
		end := start
		if endIdx >= 0 && m[endIdx] != "" {
			if e, err := strconv.Atoi(m[endIdx]); err == nil {
				end = e
			}
		}
		file := filepath.ToSlash(strings.TrimSpace(m[fileIdx]))
		_ = msgIdx // the message is retained verbatim in RawEvidence below
		suspects = append(suspects, verifier.Suspect{
			Fingerprint: Fingerprint(category, file, start, end),
			Category:    category,
			File:        file,
			LineStart:   start,
			LineEnd:     end,
			Tool:        tc.Command[0],
			Rule:        tc.Rule,
			RawEvidence: line,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return suspects, nil
}

// Fingerprint is a suspect's stable identity across runs: category + normalized
// path + line-window, hashed. Two runs that flag the same region in the same
// category produce the same fingerprint, which is what lets the standing lane
// partition new / persistent / cleared and never re-verify an unchanged
// suspect.
func Fingerprint(category, file string, lineStart, lineEnd int) string {
	norm := filepath.ToSlash(file)
	h := sha1.Sum([]byte(fmt.Sprintf("%s\x00%s\x00%d-%d", category, norm, lineStart, lineEnd)))
	return hex.EncodeToString(h[:])
}
