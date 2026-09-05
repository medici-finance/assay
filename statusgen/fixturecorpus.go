package main

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
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
//
// A THIRD invariant bounds WHERE it may be declared — see
// fixtureCorpusScopePrefix — and a fourth makes it VISIBLE: every honoured
// corpus is announced with a NOTICE on every run (fixtureCorpusChecks). A
// silent skip is how evidence goes missing.
//
// WHAT THE EXEMPTION DOES NOT TOUCH. Only the two named source-quality scans
// skip a declared corpus: the link/backticked-path check (docFiles /
// backtickPathScope / linkProblems and the identifier-dereference and
// register-ref checks keyed off that same file set) and the human:<name> stamp
// corroboration (isExcludedFixturePath). Everything else still walks those
// files: the leak sweep, the register lints, brief/stream numbering-collision
// detection, and the board generator itself. The marker narrows two checks; it
// does not remove a subtree from the repo's controls.
const fixtureCorpusMarkerName = ".statusgen-fixtures"

// fixtureCorpusScopePrefix bounds WHERE a corpus may declare itself: strictly
// below docs/streams/, i.e. at docs/streams/<something>/ or deeper. Nowhere
// else.
//
// WHY THE BOUND EXISTS. The exemption is a narrowing of two source-quality
// scans, and a narrowing is only legitimate while it is small enough to point
// at. A marker at the repo root, over docs/, or over docs/streams/ itself would
// switch the link check and the stamp scan off for every stream at once — that
// is not an exemption, it is a bypass, and it would be indistinguishable in the
// log from a repo that simply has nothing to report. A marker OUTSIDE docs/
// entirely is a different failure: nothing under it was ever in either scan's
// file set, so it can only mean the author misunderstood the mechanism.
//
// Both are handled the same way and in BOTH directions:
//
//   - INERT. isFixtureCorpusPath refuses an out-of-scope marker outright, so a
//     misplaced marker exempts nothing. The refusal lives in the RESOLVER, not
//     merely in the lint, because --corroborate consults the resolver without
//     ever running the lint — a lint-only refusal would still leave the stamp
//     scan bypassed.
//   - REFUSED OUT LOUD. fixtureCorpusChecks reports every misplaced marker as a
//     --lint PROBLEM, so an author who put it in the wrong place is told rather
//     than left with a mechanism that silently does nothing.
const fixtureCorpusScopePrefix = "docs/streams/"

// fixtureCorpusScopeOK reports whether dir (repo-relative, slash or
// OS-separator form) is a legal place to declare a fixture corpus: strictly
// below docs/streams/. The trailing slash on fixtureCorpusScopePrefix is
// load-bearing exactly as it is on corroborateExcludedFixturePrefix — it is the
// path boundary, so a lookalike sibling ("docsx/streams/s", "docs/streamsx/s")
// does not match. A cleaned path that escapes the root, or that is the root,
// docs/ or docs/streams/ itself, is refused.
func fixtureCorpusScopeOK(dir string) bool {
	d := strings.TrimSpace(filepath.ToSlash(dir))
	if d == "" {
		return false
	}
	d = path.Clean(d)
	if d == "." || d == "/" || d == ".." || strings.HasPrefix(d, "../") || strings.HasPrefix(d, "/") {
		return false
	}
	if !strings.HasPrefix(d, fixtureCorpusScopePrefix) {
		return false
	}
	// At least one path segment below docs/streams/ (docs/streams/ itself, which
	// would cover every stream, does not reach here — it has no trailing
	// remainder after Clean).
	return strings.TrimPrefix(d, fixtureCorpusScopePrefix) != ""
}

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
//   - a marker OUTSIDE docs/streams/<corpus>/ — ignored outright
//     (fixtureCorpusScopeOK). The walk still climbs to the root, but only an
//     in-scope ancestor can declare anything, so a marker at the repo root, over
//     docs/, or over docs/streams/ itself exempts nothing.
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
	// Walk directories from the file's own directory up toward root. filepath.Dir
	// yields "." at the top; the scope gate below rejects it, so the repo root
	// itself can never declare a corpus.
	dir := filepath.Dir(filepath.FromSlash(rel))
	for {
		// SCOPE GATE. A marker is consulted only where it is legal to declare one
		// (fixtureCorpusScopePrefix). An out-of-scope marker — repo root, docs/,
		// docs/streams/ itself, anywhere outside docs/ — is INERT here rather than
		// merely complained about by the lint, because --corroborate reaches this
		// resolver without running the lint at all.
		if fixtureCorpusScopeOK(dir) && fileExists(filepath.Join(root, dir, fixtureCorpusMarkerName)) {
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

// fixtureCorpusExemption is one DECLARED corpus observed on disk: the directory
// that carries the marker, and how many files sit under it (the marker itself
// not counted). The count is what makes the NOTICE reviewable — "exempted"
// without a magnitude tells a reader nothing about what stopped being scanned.
type fixtureCorpusExemption struct {
	Dir   string // repo-relative, slash form, e.g. docs/streams/s/evals
	Files int    // files under Dir, excluding the marker
}

// scanFixtureCorpora enumerates every fixtureCorpusMarkerName on disk under root
// and partitions it: honoured corpora (in scope, sorted by path) and misplaced
// marker paths (out of scope, therefore inert).
//
// It reuses repoFiles, the same tracked-shaped enumeration the shard check
// takes, so the sweep skips .git/node_modules/vendor and nothing else — a
// marker hidden in an ordinary source directory is still FOUND and reported,
// not quietly ignored. A walk error is returned rather than swallowed: an
// unreadable tree is a could-not-check, never "no markers declared"
// (docs/three-state-instrument-rule.md).
func scanFixtureCorpora(root string) (honoured []fixtureCorpusExemption, misplaced []string, err error) {
	if root == "" {
		return nil, nil, nil
	}
	files, err := repoFiles(root)
	if err != nil {
		return nil, nil, err
	}
	var dirs []string
	for _, f := range files {
		if path.Base(f) == fixtureCorpusMarkerName {
			dirs = append(dirs, path.Dir(f)) // "." for a marker at the repo root
		}
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		markerPath := path.Join(d, fixtureCorpusMarkerName)
		if !fixtureCorpusScopeOK(d) {
			misplaced = append(misplaced, markerPath)
			continue
		}
		n := 0
		for _, f := range files {
			if f == markerPath {
				continue // the declaration itself is not corpus content
			}
			if strings.HasPrefix(f, d+"/") {
				n++
			}
		}
		honoured = append(honoured, fixtureCorpusExemption{Dir: d, Files: n})
	}
	return honoured, misplaced, nil
}

// fixtureCorpusExemptedLine is the ONE announcement shape, shared by --lint and
// --corroborate so the same exemption reads identically wherever it surfaces.
func fixtureCorpusExemptedLine(e fixtureCorpusExemption) string {
	unit := "files"
	if e.Files == 1 {
		unit = "file"
	}
	return fmt.Sprintf(
		"fixture corpus exempted: %s (%d %s) — declared by %s. The link/backticked-path check and the human:<name> stamp scan skip this subtree; every other check (leak sweep, register lints, numbering collisions, the board itself) still reads it.",
		e.Dir, e.Files, unit, fixtureCorpusMarkerName)
}

// fixtureCorpusMisplacedLine is the refusal shape for an out-of-scope marker.
func fixtureCorpusMisplacedLine(markerPath string) string {
	return fmt.Sprintf(
		"fixture-corpus marker %s is out of scope and INERT — it exempts nothing. A corpus may declare itself only strictly under %s<corpus>/. A marker at the repo root, over docs/, or over %s itself would switch both scans off for every stream at once: that is not an exemption, it is a bypass. Move the marker into the corpus directory, or delete it.",
		markerPath, fixtureCorpusScopePrefix, strings.TrimSuffix(fixtureCorpusScopePrefix, "/"))
}

// fixtureCorpusChecks is the --lint surface of the mechanism. It returns one
// NOTICE per honoured corpus (so an exemption is never silent — it is in the log
// of every run, with the subtree named and its file count) and one PROBLEM per
// misplaced marker (fail-closed on ambiguity: a marker outside
// docs/streams/<corpus>/ is refused, not guessed at).
//
// A tree that declares nothing produces neither, so the mechanism is invisible
// until a corpus opts in.
func fixtureCorpusChecks(root string) (problems, notices []string) {
	honoured, misplaced, err := scanFixtureCorpora(root)
	if err != nil {
		// Could-not-check: the sweep could not read the tree, so this run cannot
		// say whether any corpus is declared. Surfaced as a problem rather than
		// reported as "none" (docs/three-state-instrument-rule.md).
		return []string{fmt.Sprintf("fixture-corpus marker sweep: %v — could-not-check: this run cannot state which subtrees (if any) are exempted from the link and stamp scans", err)}, nil
	}
	for _, e := range honoured {
		notices = append(notices, fixtureCorpusExemptedLine(e))
	}
	for _, m := range misplaced {
		problems = append(problems, fixtureCorpusMisplacedLine(m))
	}
	return problems, notices
}

// emitFixtureCorpusNotices writes the same visibility lines on the --corroborate
// surface, where the stamp scan honours the very same markers but no lint runs.
// Both classes land as NOTICEs here: --corroborate's exit code means "a human
// stamp was DISPROVED", and a misplaced marker is inert, so it disproves
// nothing — the line says plainly that --lint is the gate that refuses it.
func emitFixtureCorpusNotices(root string, w io.Writer) {
	honoured, misplaced, err := scanFixtureCorpora(root)
	if err != nil {
		fmt.Fprintf(w, "NOTICE: fixture-corpus marker sweep: %v — could-not-check: this run cannot state which subtrees (if any) are exempted from the stamp scan\n", err)
		return
	}
	for _, e := range honoured {
		fmt.Fprintln(w, "NOTICE:", fixtureCorpusExemptedLine(e))
	}
	for _, m := range misplaced {
		fmt.Fprintf(w, "NOTICE: %s statusgen --lint refuses it as a PROBLEM.\n", fixtureCorpusMisplacedLine(m))
	}
}
