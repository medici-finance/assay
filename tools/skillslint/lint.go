// skillslint — structural lint for the desk-role skill homes under
// plugins/assay/skills/. These files are the source of truth for the
// instructions every desk-role window loads at boot, so a broken frontmatter
// header or a bare overclaim in one of them ships silently unless something
// reads them.
//
// It reads every plugins/assay/skills/*/SKILL.md and asserts, per file:
//
//	frontmatter present  the file opens with a `---` fence and closes it
//	name present         a top-level `name:` with a non-empty value
//	name == dir          `name:` equals the skill's directory name (so a skill
//	                     cannot be invoked under one id while declaring another)
//	description present  a top-level `description:` with a non-empty value
//	                     (the trigger text the harness matches on; an empty one
//	                     silently never fires)
//	no bare overclaim    no line asserts "unforgeable" / "tamper-evident"
//	                     about a review/App/gate WITHOUT also qualifying or
//	                     retiring the claim on that same line (the App/identity
//	                     is a distinct, auditable actor — attribution, not
//	                     authorization; anyone holding the key can mint it, so a
//	                     bare "unforgeable" is false).
//
// PARSING IS LINE-ORIENTED, NOT STRICT YAML — deliberately. The skill loader in
// the harness reads these headers leniently: real, in-production descriptions
// routinely contain a bare "colon space" (`some phrase: like this`) inside the
// value, which a strict YAML parser rejects as a nested mapping even though the
// harness accepts it and the skill works. A lint that failed those files would
// be wrong about the corpus, not the corpus wrong about YAML. So this reads the
// `name:` / `description:` keys the way the loader does — a top-level key line,
// value to end of line, or a block scalar (`>-`, `|`, …) continued on the
// indented lines beneath it — and checks the two facts that actually matter.
//
// The guardrail derive-or-diff half lives in guardrail.go; this file only checks
// the per-file structural rules above.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skillsGlob is the set of skill files this tool validates, relative to the repo
// root. One skill, one directory, one SKILL.md.
const skillsGlob = "plugins/assay/skills/*/SKILL.md"

// Issue is one lint violation on one skill file.
type Issue struct {
	Path string // repo-relative path of the offending SKILL.md
	Msg  string
}

// LintSkills validates every skillsGlob file under root and returns the number
// of files checked plus one Issue per violation. A non-nil error is a structural
// failure of the check itself (bad root, unreadable glob, zero files) — distinct
// from a per-file Issue, which is a finding about a skill.
func LintSkills(root string) (checked int, issues []Issue, err error) {
	matches, gerr := filepath.Glob(filepath.Join(root, filepath.FromSlash(skillsGlob)))
	if gerr != nil {
		return 0, nil, fmt.Errorf("glob %s under %s: %w", skillsGlob, root, gerr)
	}
	if len(matches) == 0 {
		// Fail closed: nothing to check is never a pass.
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
		dir := filepath.Base(filepath.Dir(abs))

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("cannot read: %v", readErr)})
			continue
		}

		fmText, ok := extractFrontmatter(string(raw))
		if !ok {
			issues = append(issues, Issue{Path: rel, Msg: "missing YAML frontmatter (a leading `---` … `---` block)"})
			continue
		}
		name, hasName := frontmatterName(fmText)
		hasDesc := frontmatterHasDescription(fmText)

		switch {
		case !hasName:
			issues = append(issues, Issue{Path: rel, Msg: "frontmatter `name:` is empty or absent"})
		case name != dir:
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("frontmatter name %q != directory %q — a skill must declare the id it is invoked under", name, dir)})
		}
		if !hasDesc {
			issues = append(issues, Issue{Path: rel, Msg: "frontmatter `description:` is empty or absent — the harness matches triggers on it, so an empty one never fires"})
		}
		for _, bi := range bannedFramingIssues(string(raw)) {
			issues = append(issues, Issue{Path: rel, Msg: bi})
		}
	}
	return checked, issues, nil
}

// bannedFramingWords are the retired overclaim terms: the App/gate is a
// distinct, auditable identity — attribution, not authorization — and none of
// these words describe that honestly. Only the public spellings are enumerated
// here; the pre-publication leak sweep already scans the whole tree for the
// hyphenated variant, so restating it in this list would add no coverage.
var bannedFramingWords = []string{"unforgeable", "tamper-evident"}

// bannedFramingNegations immediately PRECEDING a banned word turn the claim
// into its own negation — "advisory, not unforgeable". Checked only in the
// short window right before the word: checking the whole line is what lets a
// regression slip through, since "the unforgeable desk App, not a shared
// account" contains "not " too — just negating something else.
var bannedFramingNegations = []string{"not ", "n't "}

// bannedFramingNegationWindow is how many characters immediately before a
// banned word are searched for a negation. Long enough for "advisory, not "
// or "isn't really " to land adjacent to the word; short enough that an
// unrelated "not" later — or earlier but not adjacent — in the same
// sentence can't launder a bare claim.
const bannedFramingNegationWindow = 24

// bannedFramingRetirements anywhere on the line show the claim is being
// cited as a past/retired overclaim rather than asserted now — e.g. "that
// was wrong and is retired", "kin are retired as overclaims".
var bannedFramingRetirements = []string{"retired", "false", "overclaim", "wrong"}

// bannedFramingIssues scans raw line by line for a banned framing word
// (bannedFramingWords) asserted with no negation immediately before it and no
// retirement language anywhere on the line, and returns one message per
// offending line. Line-scoped (not whole-file) so a qualifier elsewhere in
// the file cannot silently launder an unrelated bare claim.
func bannedFramingIssues(raw string) []string {
	var msgs []string
	for i, ln := range strings.Split(raw, "\n") {
		lower := strings.ToLower(ln)
		for _, word := range bannedFramingWords {
			idx := strings.Index(lower, word)
			if idx < 0 {
				continue
			}
			start := idx - bannedFramingNegationWindow
			if start < 0 {
				start = 0
			}
			preceding := lower[start:idx]
			qualified := false
			for _, neg := range bannedFramingNegations {
				if strings.Contains(preceding, neg) {
					qualified = true
					break
				}
			}
			if !qualified {
				for _, r := range bannedFramingRetirements {
					if strings.Contains(lower, r) {
						qualified = true
						break
					}
				}
			}
			if !qualified {
				msgs = append(msgs, fmt.Sprintf("line %d: bare %q claim with no negation immediately before it and no retirement language on the line — the App/gate is attribution, not authorization; say so or cite the retirement, don't assert the overclaim", i+1, word))
			}
			break // one banned word per line is enough to report
		}
	}
	return msgs
}

// extractFrontmatter returns the text between the leading `---` fence and the
// next `---` line. A file that does not open with a `---` fence, or never closes
// it, has no frontmatter (ok == false).
func extractFrontmatter(src string) (text string, ok bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

// frontmatterName returns the value of the top-level `name:` key. A top-level
// key starts at column 0 (indented "name:" lines inside a block scalar are not
// keys).
func frontmatterName(fmText string) (name string, ok bool) {
	for _, ln := range strings.Split(fmText, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if v, is := topLevelValue(ln, "name"); is {
			v = strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`))
			return v, v != ""
		}
	}
	return "", false
}

// frontmatterHasDescription reports whether a top-level `description:` key has a
// non-empty value, inline or as a block scalar continued on the indented lines
// below it.
func frontmatterHasDescription(fmText string) bool {
	lines := strings.Split(fmText, "\n")
	for i, ln := range lines {
		ln = strings.TrimRight(ln, "\r")
		v, is := topLevelValue(ln, "description")
		if !is {
			continue
		}
		v = strings.TrimSpace(v)
		if isBlockScalarIndicator(v) {
			// Value is on the indented lines that follow.
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimRight(lines[j], "\r")
				if strings.TrimSpace(next) == "" {
					continue
				}
				return len(next) > 0 && (next[0] == ' ' || next[0] == '\t')
			}
			return false
		}
		return strings.TrimSpace(strings.Trim(v, `"'`)) != ""
	}
	return false
}

// topLevelValue reports whether ln is a top-level `<key>:` line (column 0, no
// leading indentation) and, if so, returns the text after the colon.
func topLevelValue(ln, key string) (value string, ok bool) {
	prefix := key + ":"
	if !strings.HasPrefix(ln, prefix) {
		return "", false
	}
	return ln[len(prefix):], true
}

// isBlockScalarIndicator reports whether v is a YAML block-scalar header
// (`>`, `|`, optionally with a chomping/indentation indicator like `>-`).
func isBlockScalarIndicator(v string) bool {
	if v == "" {
		return false
	}
	if v[0] != '>' && v[0] != '|' {
		return false
	}
	// The rest may be a chomping/indent indicator and/or a comment; treat any
	// `>`/`|` opener as a block scalar for presence purposes.
	return true
}
