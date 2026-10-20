// ambient-auth lint: no skill or reference may tell a desk to read a human's
// forge-CLI login with `gh auth token` (#3055).
//
// In an adopter cell every desk role authenticated `gh` that way. The token came
// from the trusted human's personal CLI login. When that login was absent, the
// desks reported "missing desk authentication", although `desktoken <role>` had
// already minted the role App token they should have used. The canonical recipe
// for running `gh` as the role, and the line between a desk failure and a missing
// human login, is desk-shell.md §Forge CLI as the role. This check keeps the
// instruction from coming back in prose.
//
// SCOPE. Every *.md under plugins/assay/skills/ (skill bodies and their own
// references/) and plugins/assay/references/.
//
// TWO EXEMPTIONS, both narrow:
//
//  1. The canonical section itself, between the markers
//     `<!-- BEGIN forge cli as the role -->` and `<!-- END forge cli as the role -->`.
//     It has to name the anti-pattern in order to rule it out.
//  2. The driver's own human path, spelled exactly `gh auth token --user <driver-login>`
//     or `gh auth token --user "<driver-login>"` (the quoted form the Act block
//     uses: unquoted, `<driver-login>` is a redirect and a block that fails to
//     parse runs its live steps when pasted into bash): a command the human driver
//     pastes and runs under their own login (the ask-decision Reply and Act
//     shapes). Only those two exact spellings are exempt, so the same line
//     carrying a second, bare `gh auth token` is still flagged.
//
// HARD (exit 1). The tree carries no unexempt occurrence, so a new one is a
// regression rather than pre-existing lint debt.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ambientAuthDirs are the trees scanned, relative to the repo root.
var ambientAuthDirs = []string{"plugins/assay/skills", "plugins/assay/references"}

const (
	ambientAuthToken      = "gh auth token"
	ambientAuthDriverForm = "gh auth token --user <driver-login>"
	ambientAuthQuotedForm = `gh auth token --user "<driver-login>"`
	ambientAuthBegin      = "<!-- BEGIN forge cli as the role -->"
	ambientAuthEnd        = "<!-- END forge cli as the role -->"
)

// AmbientAuthIssues scans every *.md under ambientAuthDirs and returns the files
// checked plus one Issue per offending line. A non-nil error is could-not-check:
// a missing scan directory, a walk failure, or no markdown at all.
func AmbientAuthIssues(root string) (checked int, issues []Issue, err error) {
	var files []string
	for _, dir := range ambientAuthDirs {
		base := filepath.Join(root, filepath.FromSlash(dir))
		info, serr := os.Stat(base)
		if serr != nil || !info.IsDir() {
			return 0, nil, fmt.Errorf("no %s/ directory under %s — nothing to lint, which is never a pass", dir, root)
		}
		walkErr := filepath.WalkDir(base, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
				files = append(files, path)
			}
			return nil
		})
		if walkErr != nil {
			return 0, nil, fmt.Errorf("walk %s: %w", base, walkErr)
		}
	}
	if len(files) == 0 {
		return 0, nil, fmt.Errorf("no *.md under %s — nothing to lint, which is never a pass", strings.Join(ambientAuthDirs, ", "))
	}
	sort.Strings(files)

	for _, abs := range files {
		checked++
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)
		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("cannot read: %v", readErr)})
			continue
		}
		issues = append(issues, ambientAuthLines(rel, string(raw))...)
	}
	return checked, issues, nil
}

// ambientAuthLines flags each line of one file that carries an unexempt
// `gh auth token`. An unterminated canonical block is itself a finding: without
// its END marker the exemption would silently cover the rest of the file.
func ambientAuthLines(rel, raw string) []Issue {
	var out []Issue
	inBlock := false
	blockStart := 0
	for i, ln := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(ln)
		switch {
		case trimmed == ambientAuthBegin:
			inBlock, blockStart = true, i+1
			continue
		case trimmed == ambientAuthEnd:
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		rest := strings.ReplaceAll(ln, ambientAuthDriverForm, "")
		rest = strings.ReplaceAll(rest, ambientAuthQuotedForm, "")
		if strings.Contains(rest, ambientAuthToken) {
			out = append(out, Issue{Path: rel, Msg: fmt.Sprintf(
				"line %d: `%s` reads a human's forge-CLI login; a desk runs `gh` as its role App — point at desk-shell.md §Forge CLI as the role instead (only the driver's own `%s` or `%s` command is exempt)",
				i+1, ambientAuthToken, ambientAuthDriverForm, ambientAuthQuotedForm)})
		}
	}
	if inBlock {
		out = append(out, Issue{Path: rel, Msg: fmt.Sprintf(
			"line %d: `%s` has no matching `%s`; the exemption would cover the rest of the file",
			blockStart, ambientAuthBegin, ambientAuthEnd)})
	}
	return out
}
