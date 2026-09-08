// Package main implements harnessgen — the generator that turns the single
// source of the resident operating rules into every per-harness delivery
// artifact.
//
// The resident rules are the always-loaded operating rules a session runs
// under. Claude Code receives them through a SessionStart hook; Codex receives
// them through AGENTS.md. Two hand-maintained copies of the same ten rules is a
// drift failure, so the rules live in ONE source
// (plugins/assay/resident-rules.md) and each delivery artifact is generated
// from it and byte-compared in CI — divergence becomes impossible, not merely
// detected. This mirrors the STATUS.md single-writer pattern: committed derived
// artifacts, CI proves they match the source.
package main

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// Artifacts is the full set of per-harness delivery files generated from the
// single source. Each field is the exact byte content of one committed file.
type Artifacts struct {
	// ClaudePayload is the SessionStart payload text that
	// plugins/assay/hooks/inject-resident-rules.sh wraps as the JSON
	// {systemMessage: ...}. It MUST remain byte-identical to the text the hook
	// carried before the single-source refactor — rule-content changes are their
	// own PRs, never smuggled into plumbing.
	ClaudePayload string
	// CodexFragment is the AGENTS.md fragment framed as one section, so it
	// survives being composed into an adopter's existing AGENTS.md (HP/01 §3.1:
	// AGENTS.md files concatenate root-to-leaf under a 32 KiB cap).
	CodexFragment string
}

// source is the parsed single source: a header line, ten ordered rules, and a
// footer line. Everything the generator needs to reproduce both artifacts.
type source struct {
	Header string
	Rules  []rule // exactly ten, R1..R10 in order
	Footer string
}

type rule struct {
	N     int    // 1..10
	Title string // heading label, e.g. "EVIDENCE-NOT-CLAIMS"
	Body  string // the rule text as it appears in the payload, without the "N. " prefix
}

var ruleHeadingRE = regexp.MustCompile(`^## R(\d+) `)

// parseSource reads plugins/assay/resident-rules.md content. The machine-read
// content is the `## Header`, `## R<N> <title>`, and `## Footer` sections; a
// section's value is the first non-empty line after its heading. Prose outside
// those sections is documentation and is ignored.
//
// It is deliberately strict: a missing header, a missing footer, fewer than ten
// rules, rules out of order, or a rule with an empty body is an error, not a
// silent partial parse. A generator that silently drops a rule ships a weakened
// method to every session, so an unparseable source is a could-not-check
// condition the caller must surface (non-zero), never a clean pass.
func parseSource(content string) (source, error) {
	var s source
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// section is the current heading we are collecting the first body line for.
	// "" means we are not inside a section whose body we still need.
	type target struct {
		kind string // "header", "footer", or "rule"
		n    int
	}
	var cur *target

	commit := func(t target, body string) error {
		switch t.kind {
		case "header":
			s.Header = body
		case "footer":
			s.Footer = body
		case "rule":
			title := body
			if i := strings.Index(body, ":"); i >= 0 {
				title = body[:i]
			}
			s.Rules = append(s.Rules, rule{N: t.n, Title: strings.TrimSpace(title), Body: body})
		}
		return nil
	}

	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "## Header":
			cur = &target{kind: "header"}
			continue
		case line == "## Footer":
			cur = &target{kind: "footer"}
			continue
		}
		if m := ruleHeadingRE.FindStringSubmatch(line); m != nil {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			cur = &target{kind: "rule", n: n}
			continue
		}
		// Any other `## ` heading closes the current collection window without
		// consuming a body (keeps a stray section from stealing a rule's body).
		if strings.HasPrefix(line, "## ") {
			cur = nil
			continue
		}
		if cur != nil && strings.TrimSpace(line) != "" {
			if err := commit(*cur, line); err != nil {
				return s, err
			}
			cur = nil
		}
	}
	if err := sc.Err(); err != nil {
		return s, fmt.Errorf("reading source: %w", err)
	}

	if strings.TrimSpace(s.Header) == "" {
		return s, fmt.Errorf("source has no `## Header` section with a body line")
	}
	if strings.TrimSpace(s.Footer) == "" {
		return s, fmt.Errorf("source has no `## Footer` section with a body line")
	}
	if len(s.Rules) != 10 {
		return s, fmt.Errorf("source has %d rules, want exactly 10 (R1..R10)", len(s.Rules))
	}
	for i, r := range s.Rules {
		if r.N != i+1 {
			return s, fmt.Errorf("rule %d is numbered R%d — rules must be R1..R10 in order", i+1, r.N)
		}
		if strings.TrimSpace(r.Body) == "" {
			return s, fmt.Errorf("rule R%d has an empty body line", r.N)
		}
	}
	return s, nil
}

// generate turns a parsed source into every delivery artifact.
func generate(s source) Artifacts {
	return Artifacts{
		ClaudePayload: claudePayload(s),
		CodexFragment: codexFragment(s),
	}
}

// generateFromString parses then generates in one step.
func generateFromString(content string) (Artifacts, error) {
	s, err := parseSource(content)
	if err != nil {
		return Artifacts{}, err
	}
	return generate(s), nil
}

// claudePayload reproduces the exact text the SessionStart hook heredoc carried
// before the refactor: header line, a blank line, each rule as "N. <body>"
// separated by blank lines, a blank line, then the footer — with a single
// trailing newline (jq -Rs reads the whole file, trailing newline included).
func claudePayload(s source) string {
	var b strings.Builder
	b.WriteString(s.Header)
	b.WriteString("\n\n")
	for i, r := range s.Rules {
		fmt.Fprintf(&b, "%d. %s", r.N, r.Body)
		if i < len(s.Rules)-1 {
			b.WriteString("\n\n")
		}
	}
	b.WriteString("\n\n")
	b.WriteString(s.Footer)
	b.WriteString("\n")
	return b.String()
}

// cursorRules frames the same rules as a Cursor `.cursor/rules/*.mdc` rule file:
// an `.mdc` YAML frontmatter block (`alwaysApply: true` so the rules load every
// session, `description` for the rules index) followed by the same header, ten
// numbered rules, and footer. Cursor reads `.cursor/rules/*.mdc` natively
// (HP/12 §2.1); this is the Cursor-native resident-rules channel, generated from
// the SAME single source as the Claude payload and the Codex AGENTS.md fragment,
// so the three cannot drift. (Cursor ALSO reads the shared Codex AGENTS.md
// fragment natively — the adopt flow offers either.) Per Ian's 2026-08-26 ruling,
// both Cursor surfaces are targeted, headless-first.
func cursorRules(s source) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: Assay resident operating rules — the always-loaded operating rules an Assay session runs under\n")
	b.WriteString("alwaysApply: true\n")
	b.WriteString("---\n")
	b.WriteString("\n")
	b.WriteString("<!-- GENERATED from plugins/assay/resident-rules.md by `harnessgen cursor`. Do not hand-edit; run the generator. -->\n")
	b.WriteString("\n")
	b.WriteString(s.Header)
	b.WriteString("\n\n")
	for _, r := range s.Rules {
		fmt.Fprintf(&b, "%d. %s\n", r.N, r.Body)
	}
	b.WriteString("\n")
	b.WriteString(s.Footer)
	b.WriteString("\n")
	return b.String()
}

// codexFragment frames the same rules as a single AGENTS.md section. It is
// section-headed (`## `) so it composes into an adopter's existing AGENTS.md as
// one block among their own content (HP/01 §3.1), and destination-agnostic —
// installing it into a repo-root AGENTS.md or ~/.codex/AGENTS.md is the adopt
// flow's job (HP/06, HP/07), not this generator's. Per HP/03 A1, Codex CLI is
// the v1 target.
func codexFragment(s source) string {
	var b strings.Builder
	b.WriteString("## Assay resident operating rules\n")
	b.WriteString("\n")
	b.WriteString("<!-- GENERATED from plugins/assay/resident-rules.md by `harnessgen resident`. Do not hand-edit; run the generator. -->\n")
	b.WriteString("\n")
	b.WriteString(s.Header)
	b.WriteString("\n\n")
	for _, r := range s.Rules {
		fmt.Fprintf(&b, "%d. %s\n", r.N, r.Body)
	}
	b.WriteString("\n")
	b.WriteString(s.Footer)
	b.WriteString("\n")
	return b.String()
}
