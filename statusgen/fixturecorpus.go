package main

import (
	"path/filepath"
	"strings"
)

// fixtureCorpusMarkerName is the DECLARED, opt-in marker a fixture/eval doc
// corpus drops at its root to exclude that subtree from statusgen's
// source-quality scans — the dead-link / backticked-path link check
// (linkcheck.go) and the human:<name> stamp corroboration (corroborate.go).
//
// WHY IT EXISTS. An eval or fixture corpus is a directory of captured `.md`
// run-outputs. Such content legitimately carries forward-references a live brief
// never would: deliverable paths the captured run *will* create, sibling links
// into other fixtures, and literal `human:<name>` strings inside the captured
// text. Scanning it as if it were a hand-authored, live brief reds `--lint`
// (dead links, non-existent backticked paths) and `--corroborate` (stray
// `human:<name>` that maps to no login) on content that is correct exactly as
// captured. A corpus opts OUT of those scans by declaring itself a fixture
// corpus with this marker at its root.
//
// TWO INVARIANTS make this an exclusion and not an evasion surface:
//
//   - DECLARED, never inferred. The exclusion is driven ONLY by the presence of
//     this marker file on disk. A path name (`testdata`, `fixtures`, `examples`),
//     a directory convention, or any heuristic does NOT trigger it. A corpus must
//     opt in explicitly; nothing is excluded that did not ask to be.
//   - FAIL-CLOSED. Absence of the marker means the subtree is scanned exactly as
//     today. A subtree is excluded ONLY where a marker at or above it is present
//     on disk. An unreadable, empty, or absent marker never silently excludes —
//     the default is full scanning.
const fixtureCorpusMarkerName = ".statusgen-fixtures"

// isFixtureCorpusPath reports whether the file at repo-relative path rel (given
// relative to root, in either slash or OS-separator form) lies at or under a
// directory that DECLARES the fixtureCorpusMarkerName marker file on disk.
//
// It walks from the file's own directory up toward — and including — root,
// returning true at the first ancestor that carries the marker. Fail-closed on
// every uncertain input:
//
//   - root == "" — no filesystem to consult, so no marker can be observed and the
//     path is treated as fully scanned (false). Callers that genuinely have no
//     checkout root therefore keep exactly their pre-marker behaviour.
//   - a rel that is empty, "..", or escapes above root — never treated as a
//     corpus (false); the walk never climbs above root and never probes outside
//     the checkout.
//
// The marker's PRESENCE is the whole signal: an ordinary empty file at a corpus
// root is sufficient to declare it, and this consults only whether that file
// exists, never its contents.
func isFixtureCorpusPath(root, rel string) bool {
	if root == "" {
		return false // no filesystem to consult — fail-closed to fully scanned
	}
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || rel == ".." || strings.HasPrefix(rel, "../") {
		return false // empty or escaping — never a corpus, never climb above root
	}
	// Walk directories from the file's own directory up to (and including) root.
	// filepath.Dir on a slash path yields "." at the top, which joins to root
	// itself — so a marker at the repo root is honoured too.
	dir := filepath.Dir(filepath.FromSlash(rel))
	for {
		if fileExists(filepath.Join(root, dir, fixtureCorpusMarkerName)) {
			return true
		}
		if dir == "." || dir == string(filepath.Separator) || dir == "" {
			return false // reached the top; no marker found — fail-closed
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false // defensive: filesystem root reached
		}
		dir = parent
	}
}
