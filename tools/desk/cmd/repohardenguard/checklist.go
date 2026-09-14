package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// checklist.go — the parser for docs/repo-hardening-checklist.md.
//
// The checklist is the SOURCE of every required value. The guard holds no
// compiled-in expectation of its own, which is the property the tests pin:
// change a required value in the document and the guard's
// verdict changes with it. A guard carrying its own copy of the answers would
// pass a checklist that had drifted away from it.

const (
	rowsBegin = "<!-- repohardenguard:rows:begin -->"
	rowsEnd   = "<!-- repohardenguard:rows:end -->"

	// reposDirective declares, in the document, the complete set of repos the
	// checklist covers. It exists so that a `Repo` cell nobody recognises is a
	// checklist ERROR rather than a row that quietly belongs to no run. Without a
	// declared set the guard cannot tell a typo ("medici-finance/example-k8s")
	// from a legitimate second repo, and the fail-open reading of that ambiguity
	// is what silently deletes a check.
	reposDirective = "<!-- repohardenguard:repos:"
)

// Row is one checklist line: one setting on one repo.
type Row struct {
	ID       string // stable slug, used in output and in evidence
	Repo     string // owner/name — the repo this row is about
	Setting  string // human description
	Gated    string // "admin" (admin-visibility-gated field) or "public"
	Read     string // `read <kind>` or `read file <path>` — see ParseRead
	Field    string // dotted path into the response, or "[name=X].path" for a ruleset
	Required string // literal value, "[]", "contains:<tok>", or "not available — <why>"
	Set      string // what a human runs (or does) to put the value right
	Line     int    // 1-based line number in the checklist, for error messages
}

// ParsedRead is a Row's Read cell parsed into either a hardening-read KIND (op 38) or a
// repo-relative file PATH (op 22, ReadFile) — exactly one of the two is non-empty.
type ParsedRead struct {
	Kind string
	File string
}

// ParseRead parses the Read cell's grammar: `read <kind>` or `read file <path>`. It does NOT
// validate the kind against the closed vocabulary — that is deskkit.ValidateHardeningReadKind's
// job, run at CHECK time (the flow this guard's tests pin is checklist → kind → backend →
// status → verdict, never checklist → backend). A `gh api <endpoint>` cell — the retired
// grammar — is refused BY NAME here, at parse time, naming the enumerated vocabulary so a
// checklist author sees the replacement rather than a generic syntax error.
func (r Row) ParseRead() (ParsedRead, error) {
	f := strings.Fields(r.Read)
	if len(f) >= 2 && f[0] == "gh" && f[1] == "api" {
		return ParsedRead{}, fmt.Errorf(
			"row %q (line %d): Read cell %q uses the retired `gh api <endpoint>` form — the guard now reads "+
				"`read <kind>` (kinds: %s) or `read file <path>`",
			r.ID, r.Line, r.Read, strings.Join(deskkit.HardeningReadKinds(), ", "))
	}
	if len(f) < 2 || f[0] != "read" {
		return ParsedRead{}, fmt.Errorf(
			"row %q (line %d): Read cell %q is not a `read <kind>` or `read file <path>` command", r.ID, r.Line, r.Read)
	}
	if f[1] == "file" {
		if len(f) < 3 {
			return ParsedRead{}, fmt.Errorf("row %q (line %d): `read file` needs a path", r.ID, r.Line)
		}
		return ParsedRead{File: strings.Join(f[2:], " ")}, nil
	}
	if len(f) != 2 {
		return ParsedRead{}, fmt.Errorf(
			"row %q (line %d): Read cell %q has trailing tokens after the kind", r.ID, r.Line, r.Read)
	}
	return ParsedRead{Kind: f[1]}, nil
}

// Checklist is a parsed checklist document: the repos it declares it covers,
// and the rows. Rows are guaranteed by the parser to name only declared repos,
// so a run can account for every row in the file.
type Checklist struct {
	Repos []string // declared by the repos directive, in document order
	Rows  []Row
}

// Covers reports whether repo is one this checklist declares.
func (c *Checklist) Covers(repo string) bool {
	for _, r := range c.Repos {
		if r == repo {
			return true
		}
	}
	return false
}

// RowsFor returns the rows for repo, and the per-repo census of every row in the
// document. The census is what lets a run prove nothing went missing: rows for
// this repo plus rows for the others must equal the file's total.
func (c *Checklist) RowsFor(repo string) (mine []Row, census map[string]int) {
	census = map[string]int{}
	for _, r := range c.Rows {
		census[r.Repo]++
		if r.Repo == repo {
			mine = append(mine, r)
		}
	}
	return mine, census
}

// gatedAdmin reports whether an absent value on this row means "the token could
// not see it" rather than "the setting is off". Admin-visibility-gated fields
// read as null/absent for a non-admin token whether the feature is on or off, so
// on those rows an absence is could-not-check and NEVER absent-therefore-wrong.
func (r Row) gatedAdmin() bool { return strings.EqualFold(r.Gated, "admin") }

// notAvailable reports whether the row records a setting that does not exist on
// this repo's plan or visibility. Recording it is the deliverable; it is neither
// a pass nor a failure.
func (r Row) notAvailable() bool {
	return strings.HasPrefix(strings.ToLower(r.Required), "not available")
}

// parseRepos reads the repos directive. Exactly one must be present: zero leaves
// the covered set undefined, and two make it undecidable.
func parseRepos(lines []string) ([]string, error) {
	var repos []string
	found := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, reposDirective) {
			continue
		}
		if found >= 0 {
			return nil, fmt.Errorf("checklist has two %s directives (lines %d and %d) — which set of repos the document covers is undecidable", reposDirective, found+1, i+1)
		}
		found = i
		body := strings.TrimSuffix(strings.TrimPrefix(t, reposDirective), "-->")
		seen := map[string]bool{}
		for _, f := range strings.Split(body, ",") {
			f = strings.Trim(strings.TrimSpace(f), "`")
			if f == "" {
				continue
			}
			if !strings.Contains(f, "/") {
				return nil, fmt.Errorf("checklist line %d: declared repo %q is not in owner/name form", i+1, f)
			}
			if seen[f] {
				return nil, fmt.Errorf("checklist line %d: repo %q is declared twice", i+1, f)
			}
			seen[f] = true
			repos = append(repos, f)
		}
	}
	if found < 0 {
		return nil, fmt.Errorf("checklist is missing the %s … --> directive — without a declared set of repos the guard cannot tell a typo'd Repo cell from a legitimate one, and an unrecognised cell would silently drop its row", reposDirective)
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("checklist line %d: the %s directive declares no repos", found+1, reposDirective)
	}
	return repos, nil
}

// ParseChecklistFile reads path and returns the parsed checklist.
func ParseChecklistFile(path string) (*Checklist, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the checklist %q: %w — the guard has no compiled-in fallback and will not guess", path, err)
	}
	return ParseChecklist(string(b))
}

// ParseChecklist pulls the machine-readable table out of the checklist markdown.
//
// Fail-closed on every structural surprise: a missing marker, a truncated table,
// a row with the wrong column count, a Repo cell nobody declared, a Read cell
// pointing at a different repo than its own row claims. A parser that skipped
// any of those would silently stop checking a setting, which is the same class
// of failure as the two-state read this whole tool exists to prevent.
func ParseChecklist(src string) (*Checklist, error) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	declared, err := parseRepos(lines)
	if err != nil {
		return nil, err
	}

	start, end := -1, -1
	for i, ln := range lines {
		switch strings.TrimSpace(ln) {
		case rowsBegin:
			if start >= 0 {
				return nil, fmt.Errorf("checklist has two %s markers (lines %d and %d) — which table is authoritative is undecidable", rowsBegin, start+1, i+1)
			}
			start = i
		case rowsEnd:
			if end >= 0 {
				return nil, fmt.Errorf("checklist has two %s markers (lines %d and %d)", rowsEnd, end+1, i+1)
			}
			end = i
		}
	}
	if start < 0 || end < 0 {
		return nil, fmt.Errorf("checklist is missing the %s / %s markers — the guard reads only the marked table and refuses to guess which table is the rows", rowsBegin, rowsEnd)
	}
	if end < start {
		return nil, fmt.Errorf("checklist markers are inverted: %s at line %d comes after %s at line %d", rowsEnd, end+1, rowsBegin, start+1)
	}

	var rows []Row
	seen := map[string]int{}
	for i := start + 1; i < end; i++ {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(raw, "|") {
			return nil, fmt.Errorf("checklist line %d inside the marked table is not a table row: %q", i+1, raw)
		}
		cells := splitRow(raw)
		if len(cells) != 8 {
			return nil, fmt.Errorf("checklist line %d has %d columns, want 8 (ID, Repo, Setting, Gated, Read, Field, Required, Set)", i+1, len(cells))
		}
		if cells[0] == "ID" { // header
			continue
		}
		if isSeparator(cells) {
			continue
		}
		r := Row{
			ID: cells[0], Repo: cells[1], Setting: cells[2], Gated: cells[3],
			Read: cells[4], Field: cells[5], Required: cells[6], Set: cells[7],
			Line: i + 1,
		}
		if err := validate(r, declared); err != nil {
			return nil, err
		}
		if prev, dup := seen[r.ID]; dup {
			return nil, fmt.Errorf("checklist row id %q appears twice (lines %d and %d) — ids name rows in evidence and must be unique", r.ID, prev, r.Line)
		}
		seen[r.ID] = r.Line
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("checklist has no rows between the markers — an empty checklist would exit 0 and certify nothing")
	}
	return &Checklist{Repos: declared, Rows: rows}, nil
}

func validate(r Row, declared []string) error {
	for name, v := range map[string]string{"ID": r.ID, "Repo": r.Repo, "Gated": r.Gated, "Read": r.Read, "Required": r.Required} {
		if v == "" {
			return fmt.Errorf("checklist line %d: the %s cell is empty", r.Line, name)
		}
	}
	if !strings.Contains(r.Repo, "/") {
		return fmt.Errorf("checklist line %d: Repo %q is not in owner/name form", r.Line, r.Repo)
	}
	// A Repo cell nobody declared is a hard error, never a skip. A run scopes
	// rows by this cell, so an unrecognised value would produce a row that is
	// present in the document, reads as covered, and is never evaluated for any
	// repo — the count stays plausible while the coverage shrinks. One character
	// of case is enough to cause it, so the match is exact.
	if !slices.Contains(declared, r.Repo) {
		return fmt.Errorf("checklist line %d: row %q names repo %q, which the %s directive does not declare (declared: %s) — a Repo cell that matches no declared repo would never be checked by any run, so it is an error rather than a silent skip",
			r.Line, r.ID, r.Repo, reposDirective, strings.Join(declared, ", "))
	}
	if !strings.EqualFold(r.Gated, "admin") && !strings.EqualFold(r.Gated, "public") {
		return fmt.Errorf("checklist line %d: Gated is %q, want admin or public — the value decides whether an absent field is could-not-check or wrong, so there is no safe default", r.Line, r.Gated)
	}
	// The Read cell must parse under the new grammar — `read <kind>` / `read file <path>` —
	// so a checklist author sees the enumerated replacement rather than a generic syntax
	// error, and the retired `gh api <endpoint>` form is refused BY NAME. This also
	// structurally retires the former repo/endpoint mis-scoping hazard: a `read <kind>` cell
	// names no repo at all (the row's OWN Repo cell is the only source the guard ever reads
	// one from — the Checker always calls the Forge for r.Repo), so a Read cell can no
	// longer address a DIFFERENT repo than the row claims to be about. Not-available rows
	// are held to the same parse, since their Read cell is documentation a reader may run.
	parsed, err := r.ParseRead()
	if err != nil {
		return err
	}
	// A `read <kind>` row needs a Field selector into the returned document; a `read file
	// <path>` row does not — presence IS the check, and the path already named what matters.
	if !r.notAvailable() && parsed.File == "" && (r.Field == "" || r.Field == "-") {
		return fmt.Errorf("checklist line %d: row %q is checkable but names no Field to read", r.Line, r.ID)
	}
	return nil
}

// splitRow splits a markdown table row on the pipes and cleans each cell.
// Cells never contain a pipe: the stream convention bans `\|` alternation, and a
// pipe inside a cell would be indistinguishable from a column break here.
func splitRow(line string) []string {
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.Trim(strings.TrimSpace(p), "`"))
	}
	return out
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// RowRepos returns the distinct repos actually named by rows, in first-seen
// order. The parser guarantees this is a subset of Checklist.Repos; it is kept
// separate from the declared list so a run can report a declared repo that has
// no rows rather than assume the two agree.
func RowRepos(rows []Row) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range rows {
		if !seen[r.Repo] {
			seen[r.Repo] = true
			out = append(out, r.Repo)
		}
	}
	return out
}
