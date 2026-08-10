package main

import (
	"path"
	"regexp"
	"strings"
)

// entryLinkTargetRe matches the target of a markdown inline link, capturing the
// "](" delimiter (group 1) and the URL/path token up to the first whitespace or
// closing paren (group 2). An optional link title (`](path "title")`) is left
// untouched because the token stops at whitespace.
var entryLinkTargetRe = regexp.MustCompile(`(\]\()([^)\s]+)`)

// rebaseEntryBodyLinks re-homes the relative markdown links in a per-entry
// register body so they still resolve after the body is hoisted into the
// generated view.
//
// Per-entry register files live one directory DEEPER than their generated view:
// entries are docs/streams/findings/<slug>.md but the FINDINGS.md view lives at
// docs/streams/. A link authored relative to the entry file (e.g. a bare sibling
// filename [F-05](2026-07-08-….md), correct from findings/) resolves to a
// non-existent path once copied verbatim into docs/streams/FINDINGS.md — the
// dead-link and register-reference lints then fail on the generated view (#827).
//
// This rewrites each relative target from "relative to docs/streams/<subdir>/"
// to "relative to docs/streams/" by prefixing the entry's subdir and cleaning:
//
//	2026-07-08-x.md        -> findings/2026-07-08-x.md
//	./x.md                 -> findings/x.md
//	../methodology/y.md    -> methodology/y.md
//
// Absolute URLs, mailto:, root-absolute paths and bare anchors are left as-is.
func rebaseEntryBodyLinks(body, subdir string) string {
	if body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Code fences may contain literal shell/markup that must not be rewritten.
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = entryLinkTargetRe.ReplaceAllStringFunc(line, func(m string) string {
			sm := entryLinkTargetRe.FindStringSubmatch(m)
			return sm[1] + rebaseLinkTarget(sm[2], subdir)
		})
	}
	return strings.Join(lines, "\n")
}

// rebaseLinkTarget re-bases a single relative link target from the entry-file's
// directory (docs/streams/<subdir>/) to the view directory (docs/streams/). It
// preserves any trailing #anchor and never touches non-relative targets.
func rebaseLinkTarget(target, subdir string) string {
	if target == "" ||
		strings.HasPrefix(target, "#") ||
		strings.HasPrefix(target, "/") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.Contains(target, "://") {
		return target
	}
	anchor := ""
	if i := strings.Index(target, "#"); i >= 0 {
		anchor = target[i:]
		target = target[:i]
	}
	if target == "" {
		return anchor
	}
	return path.Join(subdir, target) + anchor
}
