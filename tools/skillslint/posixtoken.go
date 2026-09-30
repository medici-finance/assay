// posix-token lint: a desk-role skill body should name a MECHANISM
// (desk-shell.md §Scratch files, §Config home, …), never spell a POSIX-only
// command or path literally, because the prose is followed literally by
// whatever harness/OS the dispatched session runs on — see
// docs/streams/windows-port/brief-12-deposix-skill-prose-and-constants.md,
// which de-POSIXed the five prose sites this row exists to keep de-POSIXed.
//
// This is the "single-point-of-failure" control brief-12 names for the prose
// half of its fix: a POSIX token that slips back into a skill body — someone
// pastes a runnable `mktemp` example back in, or writes `~/.config/assay/…` as
// a literal instead of naming the config-home mechanism — is caught here
// rather than resurfacing silently on the next Windows session that follows
// the prose as written.
//
// ADVISORY, not hard, per the brief's own lint-debt cadence: it prints a
// NOTICE and never moves the exit code, matching hidden.go's context-budget
// NOTICE (same house convention for a judgment-shaped check that a human
// should see but that should not itself block a merge while the rest of the
// tree still carries pre-existing, unaudited occurrences).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// posixTokenPattern matches the three POSIX-only spellings a skill body must
// name as a mechanism instead: a literal mktemp invocation, a /tmp/ path, or a
// literal ~/.config reference.
var posixTokenPattern = regexp.MustCompile(`mktemp|/tmp/|~/\.config`)

// unixExampleFenceMarker is the token a fence's info string (the text on the
// opening ``` line, after the backticks) must carry — case-insensitively — for
// that fenced block to be treated as a deliberate "unix example" exemption
// rather than an accidental POSIX-only literal. A skill author who genuinely
// needs to show a real unix-only command spells it out explicitly this way;
// nothing is exempted by default.
const unixExampleFenceMarker = "unix"

// PosixTokenNotice is one occurrence of a POSIX-only literal in a skill body
// outside a fenced "unix example" block.
type PosixTokenNotice struct {
	Path string
	Line int
	Text string
}

// PosixTokenIssues scans every skillsGlob file under root for posixTokenPattern
// occurrences outside a fenced "unix example" block and returns the files
// checked plus one notice per offending line. Advisory (see package header):
// the caller prints these as NOTICEs and never folds them into the exit code.
func PosixTokenIssues(root string) (checked int, notices []PosixTokenNotice, err error) {
	matches, gerr := filepath.Glob(filepath.Join(root, filepath.FromSlash(skillsGlob)))
	if gerr != nil {
		return 0, nil, fmt.Errorf("glob %s under %s: %w", skillsGlob, root, gerr)
	}
	if len(matches) == 0 {
		return 0, nil, fmt.Errorf("no files match %s under %s — nothing to lint, which is never a pass", skillsGlob, root)
	}
	sort.Strings(matches)

	for _, abs := range matches {
		checked++
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			notices = append(notices, PosixTokenNotice{Path: rel, Line: 0, Text: "cannot read: " + readErr.Error()})
			continue
		}
		notices = append(notices, posixTokenLines(rel, string(raw))...)
	}
	return checked, notices, nil
}

// posixTokenLines scans one file's content line by line, tracking fence state
// so a match inside an exempt "unix example" block is skipped. Exemption is
// per-block: the opening fence's info string decides the whole block.
func posixTokenLines(rel, raw string) []PosixTokenNotice {
	var out []PosixTokenNotice
	inFence := false
	fenceExempt := false
	for i, ln := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				info := strings.ToLower(strings.TrimPrefix(trimmed, "```"))
				fenceExempt = strings.Contains(info, unixExampleFenceMarker)
			} else {
				inFence = false
				fenceExempt = false
			}
			continue
		}
		if inFence && fenceExempt {
			continue
		}
		// A line that already cites desk-shell.md is naming the mechanism, not
		// spelling a bare POSIX literal — the exact convention brief-12's own
		// Verify row 1 uses to accept a resolved-value citation like
		// "(desk-shell.md §Config home: `~/.config/assay`)". Mirroring that
		// exemption here keeps the advisory row consistent with the hard check
		// rather than re-flagging a line the hard check already accepts.
		if strings.Contains(ln, "desk-shell.md") {
			continue
		}
		if loc := posixTokenPattern.FindString(ln); loc != "" {
			out = append(out, PosixTokenNotice{
				Path: rel,
				Line: i + 1,
				Text: fmt.Sprintf("POSIX-only literal %q — name the mechanism instead (desk-shell.md §Scratch files / §Config home), or wrap a deliberate unix-only example in a fenced block whose info string names %q", loc, unixExampleFenceMarker),
			})
		}
	}
	return out
}
