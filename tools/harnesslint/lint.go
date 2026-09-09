// Command harnesslint holds harness-neutrality as a property CI checks, not a
// convention that erodes with the next edit (harness-portability/04).
//
// Two modes:
//
//	bodies   <skillsDir>   — scans <skillsDir>/*/SKILL.md for (a) banned harness
//	                         tool/hook/env tokens and (b) capability references
//	                         outside the closed vocabulary read from the stream
//	                         README. Any hit → exit 1, file:line named.
//	bindings <refsDir>     — asserts every capability in the closed set resolves
//	                         in every <refsDir>/*.md that is part of the binding
//	                         MATRIX, and every skill (enumerated from
//	                         <refsDir>/../skills) has a degradation cell in each
//	                         such file. Any gap → exit 1. A file carrying the
//	                         `assay:harnesslint non-matrix-reference` declaration
//	                         is excluded and ANNOUNCED on stderr — never skipped
//	                         silently, and never skipped without declaring itself.
//
// Three-state instrument (docs/three-state-instrument-rule.md): a parse error,
// an unreadable input, or an empty vocabulary is could-not-check (exit 2), never
// a silent pass. checked-clean is 0, checked-failed is 1.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Exit codes — the three-state contract.
const (
	exitClean  = 0 // checked-clean
	exitFailed = 1 // checked-failed: at least one violation
	exitCannot = 2 // could-not-check: parse error, unreadable input, empty vocab
	exitUsage  = 2 // usage error is also a could-not-check
)

//go:embed banned-tokens.md
var bannedConfig string

// defaultReadmePath is where the closed capability vocabulary lives, relative to
// the current working directory (repo root). The Verify commands run from the
// repo root, so the default resolves; override with --vocab for other callers.
const defaultReadmePath = "docs/streams/harness-portability/README.md"

// vocabMarker opens the machine-readable capability block in the stream README.
// "Amending the set means amending the stream README in the same PR — the lint
// reads the set from one place" (brief facts). This is that one place.
const vocabMarker = "<!-- assay:capability-vocabulary"

// bannedMarker opens the machine-readable banned-token block in banned-tokens.md.
const bannedMarker = "<!-- assay:banned-tokens"

// nonMatrixMarker opens the in-file declaration that takes ONE reference file out
// of the bindings matrix (harness-portability/15). Not every file under
// references/ is a per-harness capability binding: `desk-shell.md` is
// harness-NEUTRAL shell/transport mechanics, so demanding that it resolve every
// capability and carry a degradation cell per skill asks it to be a thing it
// says, in its own first paragraph, that it is not.
//
// The declaration is deliberately shaped like the tool's other markers
// (`assay:capability-vocabulary`, `assay:banned-tokens`) and is deliberately
// NARROW: the skip keys on a file DECLARING itself out, never on a filename the
// tool knows, and never on "this file happens to have no bindings". A reference
// that simply forgot its bindings is still fully checked and still red — that is
// the property TestCheckBindings_UndeclaredReferenceStillChecked pins.
//
// Form (one line, reason required):
//
//	<!-- assay:harnesslint non-matrix-reference — <why this file is not a binding> -->
const nonMatrixMarker = "<!-- assay:harnesslint non-matrix-reference"

// capRefRe matches a capability reference in a body or binding file. Capabilities
// are named with a reserved, unambiguous `capability:<name>` form so the closure
// check has zero false positives against ordinary hyphenated prose (`the-desk`,
// `needs-decision`, …) — a shape match on backticked word-word tokens cannot tell
// those from a capability, so the marker is load-bearing.
var capRefRe = regexp.MustCompile("`capability:([a-zA-Z][a-zA-Z0-9-]*)`")

// bannedToken is one entry from banned-tokens.md.
type bannedToken struct {
	token  string
	reason string
}

// loadBanned parses the embedded banned-tokens config. An empty parse is a
// could-not-check condition (returns a non-nil error), never a silent
// zero-banned pass.
func loadBanned() ([]bannedToken, error) {
	lines, ok := blockLines(bannedConfig, bannedMarker)
	if !ok {
		return nil, fmt.Errorf("banned-tokens config: marker %q not found", bannedMarker)
	}
	var out []bannedToken
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		parts := strings.SplitN(ln, " :: ", 2)
		tok := strings.TrimSpace(parts[0])
		reason := ""
		if len(parts) == 2 {
			reason = strings.TrimSpace(parts[1])
		}
		if tok == "" {
			continue
		}
		out = append(out, bannedToken{token: tok, reason: reason})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("banned-tokens config parsed to zero tokens — refusing to report clean")
	}
	return out, nil
}

// loadVocabulary reads the closed capability set from the stream README. An
// absent marker or an empty list is could-not-check (non-nil error), never an
// empty-but-passing set.
func loadVocabulary(readmePath string) (map[string]bool, error) {
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, fmt.Errorf("read vocabulary source %s: %w", readmePath, err)
	}
	lines, ok := blockLines(string(raw), vocabMarker)
	if !ok {
		return nil, fmt.Errorf("vocabulary source %s: marker %q not found", readmePath, vocabMarker)
	}
	set := map[string]bool{}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		set[ln] = true
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("vocabulary source %s: capability block is empty — refusing to report clean", readmePath)
	}
	return set, nil
}

// blockLines returns the lines strictly between an opening `marker` and the next
// `-->`. The bool is false when the marker is absent.
func blockLines(text, marker string) ([]string, bool) {
	i := strings.Index(text, marker)
	if i < 0 {
		return nil, false
	}
	rest := text[i+len(marker):]
	// Skip to end of the marker line.
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	} else {
		rest = ""
	}
	end := strings.Index(rest, "-->")
	if end >= 0 {
		rest = rest[:end]
	}
	return strings.Split(rest, "\n"), true
}

// nonMatrixDeclaration reports whether a reference file's body declares itself
// out of the bindings matrix, and returns the declared reason.
//
// Three-state: an unterminated marker or an empty reason is an ERROR
// (could-not-check), never a silent skip and never a silent full check. A bare
// marker with nothing after it would otherwise be the cheapest way to switch the
// guard off for a file, so the reason is mandatory — the same contract
// `banned-tokens.md` holds each banned token to, and the same one harnessgen
// holds an excluded skill to.
func nonMatrixDeclaration(body string) (string, bool, error) {
	i := strings.Index(body, nonMatrixMarker)
	if i < 0 {
		return "", false, nil
	}
	rest := body[i+len(nonMatrixMarker):]
	end := strings.Index(rest, "-->")
	if end < 0 {
		return "", false, fmt.Errorf("non-matrix-reference declaration is never closed (no %q after the marker)", "-->")
	}
	// Strip the separator punctuation the documented form puts between the
	// marker and its reason (an em dash, a hyphen, or a colon).
	reason := strings.TrimSpace(rest[:end])
	reason = strings.TrimSpace(strings.TrimLeft(reason, "—-:"))
	if reason == "" {
		return "", false, fmt.Errorf("non-matrix-reference declaration carries no reason — a bare marker is not a declaration")
	}
	return reason, true, nil
}

// sortedKeys returns a set's keys sorted, for deterministic output.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// skillFiles returns the sorted list of SKILL.md paths under skillsDir/*/SKILL.md.
func skillFiles(skillsDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(skillsDir, "*", "SKILL.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// skillNames returns the sorted set of skill directory names (each dir that
// carries a SKILL.md) under skillsDir.
func skillNames(skillsDir string) ([]string, error) {
	files, err := skillFiles(skillsDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range files {
		names = append(names, filepath.Base(filepath.Dir(f)))
	}
	sort.Strings(names)
	return names, nil
}

// checkBodies scans every SKILL.md under skillsDir for banned tokens and for
// capability references outside vocab. Returns the violation lines (empty ==
// clean) and a non-nil error only for a could-not-check condition.
func checkBodies(skillsDir string, vocab map[string]bool, banned []bannedToken) ([]string, error) {
	files, err := skillFiles(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("glob skills under %s: %w", skillsDir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no SKILL.md under %s/*/SKILL.md — nothing to check, which is never a pass", skillsDir)
	}
	var violations []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		lines := strings.Split(string(raw), "\n")
		for i, ln := range lines {
			lineNo := i + 1
			for _, bt := range banned {
				if strings.Contains(ln, bt.token) {
					violations = append(violations, fmt.Sprintf(
						"%s:%d: banned harness token %q — %s", f, lineNo, bt.token, bt.reason))
				}
			}
			for _, m := range capRefRe.FindAllStringSubmatch(ln, -1) {
				name := m[1]
				if !vocab[name] {
					violations = append(violations, fmt.Sprintf(
						"%s:%d: capability %q is not in the closed vocabulary (amend the stream README to add it)", f, lineNo, name))
				}
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

// checkBindings asserts vocabulary closure and per-skill degradation coverage
// across every references/*.md that is part of the binding MATRIX. skillsDir
// supplies the skill roster the cells are checked against. Returns the violation
// lines, the announcement lines for every file skipped by declaration, and a
// non-nil error only for a could-not-check condition.
//
// Closure and cell coverage are separable: closure needs only the reference
// files, cell coverage needs the skill roster. A caller may hand a references
// directory copied away from its sibling `skills/` (the row-3a mutation does
// exactly this) — a closure failure there is a real checked-failed and must be
// reported as such, not masked by the absent roster. Only when closure is clean
// AND the roster cannot be read do we fall to could-not-check.
//
// A file that DECLARES itself a non-matrix reference (see nonMatrixMarker) is
// excluded from both dimensions and returned in the skipped list — a file the
// check chose not to look at is a could-not-check for that file, and the
// three-state rule says it is reported as itself, never dropped silently. If the
// declaration takes out every file in the directory there is no matrix left, and
// that is could-not-check for the run rather than a clean sweep.
func checkBindings(refsDir, skillsDir string, vocab map[string]bool) ([]string, []string, error) {
	refFiles, err := filepath.Glob(filepath.Join(refsDir, "*.md"))
	if err != nil {
		return nil, nil, fmt.Errorf("glob references under %s: %w", refsDir, err)
	}
	if len(refFiles) == 0 {
		return nil, nil, fmt.Errorf("no *.md under %s — nothing to check, which is never a pass", refsDir)
	}
	sort.Strings(refFiles)
	caps := sortedKeys(vocab)

	// Read every reference file up front; an unreadable one is could-not-check.
	bodies := make(map[string]string, len(refFiles))
	for _, rf := range refFiles {
		raw, err := os.ReadFile(rf)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", rf, err)
		}
		bodies[rf] = string(raw)
	}

	// (0) Partition by declaration. Only DECLARED files leave the matrix; a
	// malformed declaration is could-not-check, not a skip and not a pass.
	var matrixFiles, skipped []string
	for _, rf := range refFiles {
		reason, declared, derr := nonMatrixDeclaration(bodies[rf])
		if derr != nil {
			return nil, nil, fmt.Errorf("%s: %w", rf, derr)
		}
		if declared {
			skipped = append(skipped, fmt.Sprintf("%s — %s", rf, reason))
			continue
		}
		matrixFiles = append(matrixFiles, rf)
	}
	if len(matrixFiles) == 0 {
		return nil, skipped, fmt.Errorf(
			"every *.md under %s declares itself a non-matrix reference — no binding matrix left to check, which is never a pass", refsDir)
	}

	// (1) Vocabulary closure — reference files only.
	var violations []string
	for _, rf := range matrixFiles {
		for _, c := range caps {
			// A capability resolves when the binding file names it in the
			// reserved `capability:<name>` form (same convention as the bodies).
			if !strings.Contains(bodies[rf], "capability:"+c) {
				violations = append(violations, fmt.Sprintf(
					"%s: capability %q does not resolve — no `capability:%s` binding present", rf, c, c))
			}
		}
	}

	// (2) Per-skill degradation coverage — needs the roster. If the roster is
	// unreadable/empty, that is could-not-check for THIS dimension only; a
	// closure failure already found (1) still dominates as checked-failed.
	skills, skillsErr := skillNames(skillsDir)
	if skillsErr != nil || len(skills) == 0 {
		if len(violations) > 0 {
			sort.Strings(violations)
			return violations, skipped, nil
		}
		if skillsErr != nil {
			return nil, skipped, fmt.Errorf("enumerate skills under %s: %w", skillsDir, skillsErr)
		}
		return nil, skipped, fmt.Errorf("no skills found under %s — cannot check per-skill cells", skillsDir)
	}
	for _, rf := range matrixFiles {
		for _, s := range skills {
			// A degradation cell for a skill names it as a backticked token in
			// the binding file's degradation table.
			if !strings.Contains(bodies[rf], "`"+s+"`") {
				violations = append(violations, fmt.Sprintf(
					"%s: no degradation cell for skill %q (its `%s` row is missing)", rf, s, s))
			}
		}
	}
	sort.Strings(violations)
	return violations, skipped, nil
}
