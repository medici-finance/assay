// Per-skill frontmatter conformance limits — the hard limits a real harness
// enforces (agentskills spec, Codex CLI), turned into exit-code-bearing lint
// findings, plus the soft body/bundle budgets those same sources call
// advisory, reported as NOTICEs that never move the exit code.
//
// Two shipped skills (`install`, `pr-review-desk`) already exceeded the
// 1024-character description limit both the agentskills specification and the
// Codex CLI impose, and skillslint's structural check (lint.go) did not
// notice: it checks that a description is present and non-empty, never its
// length. On a current Codex CLI the description is silently cut at 1021
// characters plus "..." (codex-rs/ext/skills/src/render.rs,
// MAX_CATALOG_SKILL_DESCRIPTION_CHARS); an older CLI refused to load the skill
// at all (openai/codex#13941). Either way an adopter on Codex gets a skill
// whose trigger text has lost its tail, silently.
//
// HARD (exit-code-bearing, per skill):
//
//	description length   > 1024 Unicode CODE POINTS, counted with
//	                      utf8.RuneCountInString on the trimmed YAML-loaded
//	                      value — never len() (bytes). A multi-byte
//	                      description under the character limit must not fail
//	                      just because its byte length crosses 1024.
//	name length/pattern   > 64 chars, or not matching the agentskills name
//	                      grammar `^[a-z0-9]+(-[a-z0-9]+)*$` (lowercase
//	                      letters/digits, hyphen-separated, no leading/
//	                      trailing/consecutive hyphen). name == directory is
//	                      already checked in lint.go and is not duplicated
//	                      here; this checks only the SHAPE of the value.
//
// ADVISORY (NOTICE, stderr, never moves the exit code — a judgment call the
// same way hidden.go's context-budget NOTICE is):
//
//	body bytes     > 8000 bytes (Codex truncates an agent-plugin skill body
//	               past MAX_SKILL_PROMPT_BYTES).
//	body lines     > 500 lines (agentskills: "keep your main SKILL.md under
//	               500 lines").
//	body tokens    > 5000 approx tokens, at Codex's own APPROX_BYTES_PER_TOKEN
//	               = 4 bytes/token (agentskills: "< 5000 tokens recommended").
//	bundle chars   the summed description characters across every linted
//	               skill exceeds 8000 (Codex's skills-list budget when the
//	               model's context window is unknown — a KNOWN window instead
//	               gets 2% of it in tokens, a much larger figure, and either
//	               way Codex degrades by shortening descriptions rather than
//	               refusing). One line for the whole bundle, not per skill.
//
// The budget ruling (recorded in harness-portability/17's brief, reversible):
// cutting ~2200 characters of trigger text from 14 skills to satisfy a
// fallback path that only binds when the context window is unknown would harm
// triggering on every harness for a soft, degrading limit. Only the per-skill
// HARD limits — where a harness truncates or refuses one skill outright — gate
// the build.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"unicode/utf8"
)

// Hard per-skill limits. Exceeding either is a lint Issue (exit 1).
const (
	// maxDescriptionChars is the agentskills `description` ceiling and also
	// Codex's MAX_CATALOG_SKILL_DESCRIPTION_CHARS (codex-rs/ext/skills/src/
	// render.rs @ 30fc6864, pinned in the brief's sources).
	maxDescriptionChars = 1024
	// maxNameChars is the agentskills `name` ceiling.
	maxNameChars = 64
)

// namePattern is the agentskills `name` grammar: lowercase ascii letters and
// digits, hyphen-separated, no leading/trailing/consecutive hyphen.
var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Soft budgets. Crossing one earns a NOTICE; none of them ever move the exit
// code (see the package header).
const (
	// bodyByteBudget is Codex's MAX_SKILL_PROMPT_BYTES: the byte length past
	// which `load_skill_prompts` truncates an agent-plugin skill's body.
	bodyByteBudget = 8000
	// bodyLineBudget is the agentskills recommendation for a SKILL.md body.
	bodyLineBudget = 500
	// bodyTokenBudget is the agentskills "< 5000 tokens recommended" body
	// budget, approximated the same way Codex approximates it.
	bodyTokenBudget = 5000
	// approxBytesPerToken is Codex's own APPROX_BYTES_PER_TOKEN constant, used
	// here only to approximate bodyTokenBudget in bytes — never as a claim
	// this tool tokenizes accurately.
	approxBytesPerToken = 4
	// bundleDescriptionBudget is Codex's skills-list char budget that applies
	// when the model's context window is unknown (DEFAULT_SKILL_METADATA_
	// CHAR_BUDGET); a known window instead gets 2% of it in tokens, which for
	// any window Codex plausibly runs is a much larger figure. This budget is
	// therefore a floor-case NOTICE, not a claim the bundle is over every
	// window's budget.
	bundleDescriptionBudget = 8000
)

// conformanceDescriptionIssue reports a description over maxDescriptionChars,
// or "" when it is within the limit. desc is the already-trimmed, already
// YAML-loaded string value — counted in runes, never bytes.
func conformanceDescriptionIssue(desc string) string {
	n := utf8.RuneCountInString(desc)
	if n <= maxDescriptionChars {
		return ""
	}
	return fmt.Sprintf(
		"description is %d characters, over the %d-character hard limit (agentskills spec `description`; Codex MAX_CATALOG_SKILL_DESCRIPTION_CHARS truncates a skill's description at 1021 chars + \"...\", and an older Codex CLI refuses to load the skill at all — openai/codex#13941)",
		n, maxDescriptionChars)
}

// conformanceNameIssue reports a name over maxNameChars, or not matching the
// agentskills name grammar, or "" when both are satisfied. name == directory
// is checked separately in lint.go and is not duplicated here.
func conformanceNameIssue(name string) string {
	if n := utf8.RuneCountInString(name); n > maxNameChars {
		return fmt.Sprintf(
			"name is %d characters, over the %d-character hard limit (agentskills spec `name`)",
			n, maxNameChars)
	}
	if !namePattern.MatchString(name) {
		return fmt.Sprintf(
			"name %q does not match the agentskills name pattern `^[a-z0-9]+(-[a-z0-9]+)*$` (lowercase letters/digits, hyphen-separated, no leading/trailing/consecutive hyphen)",
			name)
	}
	return ""
}

// conformanceBodyNotice returns one NOTICE line naming every soft body budget
// raw crosses (bytes, lines, approx tokens), or "" when none is crossed. At
// most one line per skill — the caller never emits more than one of these per
// file.
func conformanceBodyNotice(rel string, raw []byte) string {
	nBytes := len(raw)
	nLines := bytes.Count(raw, []byte("\n")) + 1
	nTokens := nBytes / approxBytesPerToken

	var crossed []string
	if nBytes > bodyByteBudget {
		crossed = append(crossed, fmt.Sprintf("%d bytes (budget %d, Codex plugin-body truncation)", nBytes, bodyByteBudget))
	}
	if nLines > bodyLineBudget {
		crossed = append(crossed, fmt.Sprintf("%d lines (budget %d, agentskills)", nLines, bodyLineBudget))
	}
	if nTokens > bodyTokenBudget {
		crossed = append(crossed, fmt.Sprintf("~%d approx tokens (budget %d, agentskills / Codex bytes-per-token=%d)", nTokens, bodyTokenBudget, approxBytesPerToken))
	}
	if len(crossed) == 0 {
		return ""
	}
	joined := crossed[0]
	for _, c := range crossed[1:] {
		joined += "; " + c
	}
	return fmt.Sprintf("skillslint: NOTICE: %s: soft budget(s) exceeded: %s", rel, joined)
}

// conformanceBundleNotice returns the ONE bundle-wide NOTICE when the summed
// description characters across every linted skill exceed
// bundleDescriptionBudget, or "" when under it. The rendered skills-list line
// for each skill costs its name and a locator on top of the description
// itself, so the true list cost is higher than sumDescChars alone — the
// notice states that explicitly rather than implying the sum is the whole
// cost.
func conformanceBundleNotice(checked, sumDescChars int) string {
	if sumDescChars <= bundleDescriptionBudget {
		return ""
	}
	return fmt.Sprintf(
		"skillslint: NOTICE: bundle: %d skill(s), summed description characters %d exceeds the %d-character budget (Codex's skills-list budget when the model's context window is unknown) — the rendered list line (name + locator) costs more than the description alone",
		checked, sumDescChars, bundleDescriptionBudget)
}

// LintSkillsDir is the --skills-dir adopter-reach path: it validates every
// <dir>/*/SKILL.md file with the SAME structural + conformance checks
// LintSkills runs over the fixed plugins/assay/skills layout, but over an
// arbitrary directory of one subdirectory per skill. It does not run the
// house-value, hidden-character, guardrail or enforcement-block halves — those
// check THIS repo's own tree and do not apply to an adopter's directory.
func LintSkillsDir(dir string) (checked int, issues []Issue, err error) {
	pattern := filepath.Join(dir, "*", "SKILL.md")
	matches, gerr := filepath.Glob(pattern)
	if gerr != nil {
		return 0, nil, fmt.Errorf("glob %s: %w", pattern, gerr)
	}
	if len(matches) == 0 {
		// Fail closed: nothing to check is never a pass.
		return 0, nil, fmt.Errorf("no files match %s — nothing to lint, which is never a pass", pattern)
	}
	sort.Strings(matches)
	checked, issues = lintSkillMatches(matches, dir)
	return checked, issues, nil
}

// ConformanceNotices computes the advisory (never exit-affecting) per-skill
// body-budget lines and the one bundle-wide description-budget line, over
// every skillsGlob file under root — the default (--root) path.
func ConformanceNotices(root string) (checked int, notices []string, err error) {
	matches, gerr := filepath.Glob(filepath.Join(root, filepath.FromSlash(skillsGlob)))
	if gerr != nil {
		return 0, nil, fmt.Errorf("glob %s under %s: %w", skillsGlob, root, gerr)
	}
	if len(matches) == 0 {
		return 0, nil, fmt.Errorf("no files match %s under %s — nothing to lint, which is never a pass", skillsGlob, root)
	}
	sort.Strings(matches)
	return conformanceNoticesFor(matches, root)
}

// ConformanceNoticesDir is the --skills-dir analogue of ConformanceNotices.
func ConformanceNoticesDir(dir string) (checked int, notices []string, err error) {
	pattern := filepath.Join(dir, "*", "SKILL.md")
	matches, gerr := filepath.Glob(pattern)
	if gerr != nil {
		return 0, nil, fmt.Errorf("glob %s: %w", pattern, gerr)
	}
	if len(matches) == 0 {
		return 0, nil, fmt.Errorf("no files match %s — nothing to lint, which is never a pass", pattern)
	}
	sort.Strings(matches)
	return conformanceNoticesFor(matches, dir)
}

// conformanceNoticesFor is the shared body for ConformanceNotices and
// ConformanceNoticesDir: matches is already sorted and globbed, base is the
// directory relative paths are computed against.
func conformanceNoticesFor(matches []string, base string) (checked int, notices []string, err error) {
	sumDescChars := 0
	for _, abs := range matches {
		checked++
		rel, rerr := filepath.Rel(base, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			// Already reported as an Issue by LintSkills[Dir]; a notice pass
			// that cannot read the file simply contributes nothing to it.
			continue
		}
		if n := conformanceBodyNotice(rel, raw); n != "" {
			notices = append(notices, n)
		}
		if fmText, ok := extractFrontmatter(string(raw)); ok {
			if fields, perr := parseFrontmatter(fmText); perr == nil {
				if desc, derr := frontmatterString(fields, "description"); derr == nil {
					sumDescChars += utf8.RuneCountInString(desc)
				}
			}
		}
	}
	if n := conformanceBundleNotice(checked, sumDescChars); n != "" {
		notices = append(notices, n)
	}
	return checked, notices, nil
}
