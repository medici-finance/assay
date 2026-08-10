package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// wordCount returns the number of words in content, matching wc -w semantics.
func wordCount(content string) int {
	return len(strings.Fields(content))
}

// defaultBudgetSpec is the word-budget check CI enforces
// (.github/workflows/statusgen.yml: `--lint --budget CLAUDE.md:2850`,
// methodology/14, issue #280). Bare `--lint` used to skip it entirely, so a
// contributor could see local green on a PR that CI would fail red on
// nothing but a word count (issue #470). --lint now defaults to this spec so
// local == CI; an explicit --budget still overrides it.
const defaultBudgetSpec = "CLAUDE.md:2850"

// resolveBudgetSpecs applies the --lint default budget spec (issue #470) when
// the caller passed no explicit --budget. Only lint mode gets a default —
// write/check/record are unaffected: they never ran the budget check before
// this fix, and this issue is scoped to the local-vs-CI --lint divergence. An
// explicit --budget always wins outright (override, not additive), so a
// caller that wants a different cap gets exactly that, not the default plus
// theirs.
func resolveBudgetSpecs(mode string, explicit []string) []string {
	if mode == "lint" && len(explicit) == 0 {
		return []string{defaultBudgetSpec}
	}
	return explicit
}

// effectiveBudgetSpecs resolves the budget specs for a run and then drops the
// AUTO-APPLIED default (bare --lint, no --budget) when its file does not exist.
// The default budget is a project-specific convenience (CLAUDE.md); a repo that
// does not have one — e.g. a fresh adopter scaffolded by `statusgen init` — must
// not be red-gated by it on its first CI run. An EXPLICIT --budget is always
// honored and still hard-fails on a missing file (asking for a check on a file
// that is not there is a real error).
func effectiveBudgetSpecs(mode string, explicit []string, root string) []string {
	specs := resolveBudgetSpecs(mode, explicit)
	if mode != "lint" || len(explicit) != 0 {
		return specs
	}
	if len(specs) == 1 && specs[0] == defaultBudgetSpec {
		if path, _, err := parseBudgetSpec(defaultBudgetSpec); err == nil {
			if _, statErr := os.Stat(filepath.Join(root, path)); os.IsNotExist(statErr) {
				return nil
			}
		}
	}
	return specs
}

// parseBudgetSpec splits "path:maxwords" into its components. A malformed spec
// returns a non-nil error (usage error, not a PROBLEM).
func parseBudgetSpec(spec string) (relPath string, maxWords int, err error) {
	colon := strings.LastIndex(spec, ":")
	if colon < 0 {
		return "", 0, fmt.Errorf("--budget: malformed value %q — want path:maxwords", spec)
	}
	relPath = spec[:colon]
	max, err := strconv.Atoi(spec[colon+1:])
	if err != nil || max < 0 {
		return "", 0, fmt.Errorf("--budget: malformed maxwords in %q — want a non-negative integer", spec)
	}
	return relPath, max, nil
}

// checkBudget enforces word-budget caps for the given file specs
// (methodology/37). Each spec is "relpath:maxwords" (repeatable flag).
// Problems are non-fatal hard-check lines surfaced alongside other PROBLEM
// output; a non-nil error means the flag value itself is malformed (usage
// error — the caller should exit before running any checks).
func checkBudget(root string, specs []string) (problems []string, err error) {
	for _, spec := range specs {
		relPath, maxWords, perr := parseBudgetSpec(spec)
		if perr != nil {
			return nil, perr
		}
		fullPath := filepath.Join(root, relPath)
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			problems = append(problems, fmt.Sprintf(
				"%s: budgeted file is missing — %s not found (methodology/37, issue #280)",
				relPath, relPath))
			continue
		}
		wc := wordCount(string(content))
		if wc > maxWords {
			problems = append(problems, fmt.Sprintf(
				"%s: %d words exceeds budget of %d (methodology/14 cap — diet before merging; methodology/37, issue #280)",
				relPath, wc, maxWords))
		}
	}
	return problems, nil
}
