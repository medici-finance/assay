// housevalue.go — the unresolved-house-value check, over the WHOLE plugin tree.
//
// The plugin ships more adopter-facing prose than skill bodies: the harness
// references under plugins/assay/references/, the per-directory READMEs, the
// command docs. Every one of them is read by an adopting repo that is not this
// house, so every one of them must name the driver with the neutral token
// `human:<name>` (or a `capability:<name>` binding) rather than resolving it to
// whoever happens to drive it here. A resolved house value in a reference file
// used to pass lint because the lint only ever read plugins/assay/skills/*/SKILL.md
// (#236: two occurrences in a reference, caught by a reviewer, not by CI).
// This check reads every *.md under plugins/ instead.
//
// NEUTRAL BY CONSTRUCTION. The check carries no list of real names — it could not
// be shipped to adopters if it did, and a name list is exactly the artefact the
// `human:<name>` token exists to abolish. It detects the SHAPE instead: a
// capitalised, proper-name-shaped token standing in a DRIVER POSITION. Three
// positions are recognised, because they are the three the corpus actually uses
// for the neutral token:
//
//	dated attribution   `(human:<name>, 2026-07-20)`  → `(Somebody, 2026-07-20)`
//	possessive          "human:<name>'s ruling"       → "Somebody's ruling"
//	driver lead-in      "driver human:<name>"         → "driver Somebody"
//
// Anything shaped like a proper name in one of those positions is a violation
// unless it is a genuine product/tool/platform noun, which is what
// driver-allowlist.txt is for. That file is data, not code: adding a vendor is a
// one-line edit, and adding a PERSON to it is a review finding (see the header of
// the file itself).
//
// The allowlist is embedded rather than read from disk at run time on purpose.
// The file is checked in next to the tool, so it belongs to the TOOL, not to the
// --root being linted; resolving it relative to a cwd that `make skillslint`,
// `go test`, and `go run ./tools/skillslint --root ..` each set differently is a
// could-not-check waiting to happen, and a lint that cannot find its own data
// file must not degrade to a quiet pass. `go:embed` makes the lookup unfailable
// while keeping the single checked-in file the only place to edit.
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

// pluginTreeDir is the subtree this check reads, relative to the repo root.
// EVERY *.md under it, at any depth — skill bodies, references/, READMEs,
// commands/ — not just the skill homes lintSkills reads.
const pluginTreeDir = "plugins"

//go:embed driver-allowlist.txt
var driverAllowlistRaw string

// properNamePattern is the shape of a proper name: one or more capitalised words
// on ONE line. Word-internal digits are allowed (so "R6" and "HP12" are matched
// and then dismissed as acronyms by shape, rather than being invisible to the
// scan). Deliberately ASCII and deliberately line-bounded — a capitalised word at
// the end of one sentence and another at the start of the next are two names, not
// one two-word name.
const properNamePattern = `[A-Z][A-Za-z0-9]*(?:[ \t][A-Z][A-Za-z0-9]*)*`

// isoDatePattern is the date shape the corpus uses for a dated attribution.
const isoDatePattern = `20[0-9]{2}-[0-9]{2}-[0-9]{2}`

var (
	// driverDated: "(Somebody, 2026-08-26)" — the attribution parenthetical, the
	// commonest driver position in the corpus. The comma is optional (the corpus
	// also writes "human:<name> 2026-08-16"), and the separator may be a single
	// line break, because a wrapped paragraph routinely splits the name from its
	// date. Bounded to at most one newline so an unrelated capitalised word two
	// paragraphs above a date cannot be dragged in.
	driverDated = regexp.MustCompile(`(` + properNamePattern + `),?(?:[ \t]+|[ \t]*\r?\n[ \t]*)` + isoDatePattern)

	// driverPossessive: "Somebody's ruling". Both the ASCII and the typographic
	// apostrophe, because prose files carry both.
	driverPossessive = regexp.MustCompile(`(` + properNamePattern + `)(?:'|\x{2019})s\b`)

	// driverLeadIn: "driver Somebody", "the driver is Somebody" — the position the
	// skill frontmatter uses ("driver human:<name>").
	driverLeadIn = regexp.MustCompile(`[Dd]rivers?(?: is| are)?[ \t]+(` + properNamePattern + `)`)
)

// driverPosition names a recognised position for the report, so the message says
// WHY the span was read as a driver rather than only that it was.
type driverPosition struct {
	name string
	re   *regexp.Regexp
}

var driverPositions = []driverPosition{
	{"dated attribution", driverDated},
	{"possessive", driverPossessive},
	{"driver lead-in", driverLeadIn},
}

// LintPluginTree reads every *.md under <root>/plugins/ and reports one Issue per
// proper-name-shaped token found in a driver position. checked is the number of
// files read. A non-nil error is a failure of the CHECK (no plugin tree, no
// markdown in it, an unreadable directory) — could-not-check, never a pass.
func LintPluginTree(root string) (checked int, issues []Issue, err error) {
	base := filepath.Join(root, filepath.FromSlash(pluginTreeDir))
	info, serr := os.Stat(base)
	if serr != nil || !info.IsDir() {
		return 0, nil, fmt.Errorf("no %s/ directory under %s — nothing to lint, which is never a pass", pluginTreeDir, root)
	}

	var files []string
	walkErr := filepath.WalkDir(base, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return 0, nil, fmt.Errorf("walk %s: %w", base, walkErr)
	}
	if len(files) == 0 {
		return 0, nil, fmt.Errorf("no *.md under %s — nothing to lint, which is never a pass", base)
	}
	sort.Strings(files)

	allow := driverAllowlist()
	for _, abs := range files {
		checked++
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("cannot read: %v", readErr)})
			continue
		}
		for _, msg := range houseValueIssues(string(raw), allow) {
			issues = append(issues, Issue{Path: rel, Msg: msg})
		}
	}
	return checked, issues, nil
}

// houseValueIssues scans src for a proper-name-shaped token in any recognised
// driver position and returns one message per finding, in file order, de-duplicated
// per (line, token) so a name matched by two positions is reported once.
func houseValueIssues(src string, allow map[string]bool) []string {
	type finding struct {
		line  int
		token string
		pos   string
		span  string
	}
	var found []finding
	seen := map[string]bool{}

	for _, dp := range driverPositions {
		for _, m := range dp.re.FindAllStringSubmatchIndex(src, -1) {
			token := src[m[2]:m[3]]
			if permittedDriverToken(token, allow) {
				continue
			}
			line := lineOf(src, m[2])
			key := fmt.Sprintf("%d\x00%s", line, token)
			if seen[key] {
				continue
			}
			seen[key] = true
			found = append(found, finding{line: line, token: token, pos: dp.name, span: oneLine(src[m[0]:m[1]])})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].line != found[j].line {
			return found[i].line < found[j].line
		}
		return found[i].token < found[j].token
	})

	msgs := make([]string, 0, len(found))
	for _, f := range found {
		msgs = append(msgs, fmt.Sprintf(
			"line %d: %q (%s: %q) is a proper-name-shaped token in the driver position — "+
				"the driver is named by the neutral `human:<name>` token (or a `capability:<name>` binding), "+
				"never resolved to a house value; replace it, or if it is a product/tool name add it to "+
				"tools/skillslint/driver-allowlist.txt (never a person's name)",
			f.line, f.token, f.pos, f.span))
	}
	return msgs
}

// permittedDriverToken reports whether a capitalised phrase in a driver position
// is legitimate: an allowlisted phrase, an allowlisted head noun (so "The App's"
// rides on `App`), or an all-caps acronym, which is a shape no house value takes.
func permittedDriverToken(token string, allow map[string]bool) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return true
	}
	if allow[token] {
		return true
	}
	words := strings.Fields(token)
	last := words[len(words)-1]
	if allow[last] {
		return true
	}
	return isNotNameShaped(last)
}

// isNotNameShaped reports whether w is a shape no written name takes, and so is
// out of scope by construction rather than by allowlist:
//
//	a single letter   "track B's", "option A's" — a label, not a name
//	an all-caps id    PR, CI, README, R6, HP12 — an acronym or a rule id
//
// STATED LIMITATION: a name SHOUTED in all caps is therefore not detected. That
// is deliberate — the alternative is to teach this tool which capitalised words
// are people, which is precisely the name list the check must not carry. The
// residual case is a review's to catch, not a lint's.
func isNotNameShaped(w string) bool {
	if len([]rune(w)) < 2 {
		return true
	}
	letters := 0
	for _, r := range w {
		switch {
		case r >= 'A' && r <= 'Z':
			letters++
		case r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return letters >= 1
}

// driverAllowlist parses the embedded allowlist: one entry per line, `#`
// comments and blank lines ignored.
func driverAllowlist() map[string]bool {
	allow := map[string]bool{}
	for _, ln := range strings.Split(driverAllowlistRaw, "\n") {
		if i := strings.Index(ln, "#"); i >= 0 {
			ln = ln[:i]
		}
		if ln = strings.TrimSpace(ln); ln != "" {
			allow[ln] = true
		}
	}
	return allow
}

// lineOf returns the 1-based line number of byte offset off in src.
func lineOf(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return 1 + strings.Count(src[:off], "\n")
}

// oneLine renders a matched span for the report on a single line, so a span that
// wrapped in the source still prints as one file:line record.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
