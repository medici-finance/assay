// skillslint — structural lint for the desk-role skill homes under
// plugins/assay/skills/. These files are the source of truth for the
// instructions every desk-role window loads at boot, so a broken frontmatter
// header or a bare overclaim in one of them ships silently unless something
// reads them.
//
// It reads every plugins/assay/skills/*/SKILL.md and asserts, per file:
//
//	frontmatter present  the file opens with a `---` fence and closes it
//	frontmatter parses   the fenced block is a YAML document that loads, and
//	                     loads as a MAPPING (see below)
//	name present         a `name:` key whose value is a non-empty string
//	name == dir          `name:` equals the skill's directory name (so a skill
//	                     cannot be invoked under one id while declaring another)
//	description present  a `description:` key whose value is a non-empty string
//	                     (the trigger text the harness matches on; an empty one
//	                     silently never fires)
//	no bare overclaim    no line asserts "unforgeable" / "tamper-evident"
//	                     about a review/App/gate WITHOUT also qualifying or
//	                     retiring the claim on that same line (the App/identity
//	                     is a distinct, auditable actor — attribution, not
//	                     authorization; anyone holding the key can mint it, so a
//	                     bare "unforgeable" is false).
//
// PARSING IS A REAL YAML LOAD (gopkg.in/yaml.v3, the parser the rest of this
// repo already depends on). It used to be line-oriented, on the theory that a
// plain scalar carrying a bare "colon space" (`some phrase: like this`) works
// anyway because the loaders are lenient. That theory was false: a colon-space
// in a plain scalar is a NESTED MAPPING to a YAML parser, the whole frontmatter
// document then fails to load, and a harness that builds its skill roster by
// loading that document sees no name and no description — so the skill silently
// never surfaces. Two shipped skills had exactly that defect while this lint
// reported PASS on both (#1115). Reading the header the way a consumer reads it
// is the only way this check can speak for one.
//
// The repair for an unparseable description is a folded block scalar
// (`description: >-` with the text on the indented lines beneath it), never a
// reworded description: the text is adopter-facing trigger text the harness
// matches on, so it is preserved byte-for-byte and only its QUOTING changes.
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

	"gopkg.in/yaml.v3"
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
		fields, perr := parseFrontmatter(fmText)
		if perr != nil {
			// A frontmatter block that does not load is not partially
			// readable: every key beneath it is unreadable too, so report the
			// parse failure alone rather than a cascade of "name: absent".
			issues = append(issues, Issue{Path: rel, Msg: perr.Error()})
			continue
		}

		name, nameErr := frontmatterString(fields, "name")
		switch {
		case nameErr != nil:
			issues = append(issues, Issue{Path: rel, Msg: nameErr.Error()})
		case name != dir:
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("frontmatter name %q != directory %q — a skill must declare the id it is invoked under", name, dir)})
		}
		if _, descErr := frontmatterString(fields, "description"); descErr != nil {
			issues = append(issues, Issue{Path: rel, Msg: descErr.Error()})
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

// parseFrontmatter loads the fenced block as a YAML document and returns its
// top-level keys. The error it returns is the lint message verbatim: a block
// that does not load, or loads as anything but a mapping, has no readable keys
// at all, so the caller reports this one finding instead of a cascade of
// "key absent" findings derived from a document nothing could read.
func parseFrontmatter(fmText string) (map[string]any, error) {
	var doc any
	if err := yaml.Unmarshal([]byte(fmText), &doc); err != nil {
		return nil, fmt.Errorf("frontmatter does not parse as YAML: %s — a plain scalar carrying a colon-space is read as a nested mapping; quote the value or make it a folded block scalar (`>-`), keeping the text unchanged", compactYAMLError(err))
	}
	if doc == nil {
		return nil, fmt.Errorf("frontmatter is empty — it must be a YAML mapping carrying `name:` and `description:`")
	}
	fields, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("frontmatter is not a YAML mapping (it loads as %T) — it must be a mapping carrying `name:` and `description:`", doc)
	}
	return fields, nil
}

// frontmatterString returns the value of key, requiring that it is present, a
// STRING, and non-empty. A key that loads as a list, a number or a nested
// mapping is present but is not the text every consumer reads it as.
func frontmatterString(fields map[string]any, key string) (string, error) {
	raw, present := fields[key]
	if !present || raw == nil {
		return "", fmt.Errorf("frontmatter `%s:` is empty or absent%s", key, frontmatterKeyWhy(key))
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("frontmatter `%s:` must be a string, not %T%s", key, raw, frontmatterKeyWhy(key))
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("frontmatter `%s:` is empty or absent%s", key, frontmatterKeyWhy(key))
	}
	return strings.TrimSpace(s), nil
}

// frontmatterKeyWhy is the per-key tail explaining what breaks when the key is
// unusable, appended to both the absent and the wrong-type message so the
// reason travels with either failure.
func frontmatterKeyWhy(key string) string {
	if key == "description" {
		return " — the harness matches triggers on it, so an empty one never fires"
	}
	return ""
}

// compactYAMLError flattens a yaml.v3 error onto one line. A TypeError carries
// one message per offending line, and a lint finding is read as a single line.
func compactYAMLError(err error) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(err.Error(), "\n", " ")), " ")
}
