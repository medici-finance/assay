package main

// brief.go — the ADDITIVE half of the human-decision gate: reading a brief's OWN frontmatter
// to decide whether it gates on a human, so a `--brief` dispatch fires the decision-issue
// gate even when the caller did not pass an explicit `--gate-human`.
//
// WHY THIS EXISTS. The contract the desk skills state is: a gate:human item dispatches
// NORMALLY, but the dispatcher passes `--gate-human` OR a `--brief` whose own metadata gates
// on a human. The metadata half was the missing half — deskdispatch fired the gate ONLY on
// the explicit flag, keyed on nothing the brief itself said. So a `gate: human` brief passed
// by `--brief` alone (its `irreversible:` answer `no`) printed "decision-gate SKIPPED: item
// is not human-gated" and dispatched a worker with NO decision issue filed: the exact EMPTY
// decision surface the gate exists to prevent. This reads the one frontmatter field that
// settles it.
//
// WHY A REGEX EXTRACTOR, NOT A YAML DEP. It mirrors verifyloop/fanoutloop's parseFrontmatter:
// a deliberately small line/regex extractor so the desk module stays self-contained
// (statusgen, which owns the brief-v1 schema, is package main and unimportable). The field is
// `gate` and the human-gating value is `human` — the exact spelling fanoutloop's dispatcher
// keys on (strings.EqualFold(Gate, "human")).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// briefGateRe matches the top-level `gate:` frontmatter line.
var briefGateRe = regexp.MustCompile(`(?m)^gate:\s*(.*)$`)

// briefGatesHuman reports whether the brief's own frontmatter gates on a human
// (`gate: human`, case-insensitive) — the metadata half of the human-decision gate contract.
//
// It is best-effort by design: a brief that cannot be read, carries no frontmatter fence, or
// names no gate yields FALSE, and the caller falls back to the explicit `--gate-human` flag,
// which stays the guaranteed path. It never returns a false POSITIVE that would file a
// spurious decision issue, and its miss on an unreadable file is covered by the explicit flag.
func briefGatesHuman(root, brief string) bool {
	return strings.EqualFold(briefFrontmatterGate(root, brief), "human")
}

// briefFrontmatterGate returns the trimmed value of the brief's top-level `gate:` frontmatter
// field, or "" when the file cannot be read, opens with no `---` fence, or names no gate. The
// brief path is resolved against --root when relative — the same base the decision script is
// invoked from, so the file this reads is the file that script derives the issue from.
func briefFrontmatterGate(root, brief string) string {
	path := brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, brief)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	body := briefFrontmatterBlock(string(raw))
	if body == "" {
		return ""
	}
	m := briefGateRe.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(m[1]), `"'`)
}

// briefFrontmatterBlock returns the text between the leading `---` fences, or "" if the
// content does not open with one. Mirrors verifyloop.frontmatterBlock.
func briefFrontmatterBlock(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}
