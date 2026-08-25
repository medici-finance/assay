// guardrail.go — the derive-or-diff half of skillslint.
//
// THE PROBLEM. Four cross-cutting rule blocks (git-push policy, insight-routing,
// escalation labels, no-attribution) were restated near-verbatim across the
// desk-role skills, and had already drifted. The obvious fix — write the rule
// once somewhere and replace every copy with a pointer — is WRONG for these
// blocks: they are load-bearing while a loop is running, and a check a session
// has to go and fetch from another file is a check that does not happen.
//
// THE FIX. One DECLARED SOURCE (.claude/guardrails/GUARDRAILS.md), copies that
// are REGENERATED from it (`skillslint --sync`), and this check as the diff. The
// copies stay resident, so nothing loses residence; they stop being
// hand-maintained, so they cannot drift silently. Same shape as a
// compiled-config file plus a test that is the diff: one source, a generator,
// and a check that is the diff.
//
// A SITE MAY BE COMPARED WITH DECLARED SUBSTITUTIONS. When a source ships both a
// canonical home and a scrubbed published twin, demanding byte-equality on the
// twin would re-introduce the very identifiers the scrub removed. So a site
// tagged `(scrub: bundle)` is compared against the canonical text with the
// source file's declared substitutions applied, in order — the twin is DERIVED,
// not exempted. This bundle declares no scrub rules (the skills here are already
// the generic, adopter-facing text), so every site is compared verbatim.
//
// THREE-STATE, NEVER FAIL-OPEN. Every outcome is checked-clean, checked-failed,
// or could-not-check — and could-not-check is reported as a FAILURE. A missing
// source file, an unreadable site, a site whose anchor line is absent, and a
// declared site path that does not exist are all could-not-check. "Nothing to
// compare" is never a pass; that is the exact fail-open this check exists to
// remove.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// guardrailSourcePath is the ONE declared home for the shared guardrail blocks,
// relative to the repo root. Everything else is derived from it or diffed
// against it.
const guardrailSourcePath = ".claude/guardrails/GUARDRAILS.md"

// GuardrailBlock is one declared rule: its id, its canonical text, and the
// exhaustive list of files that must carry a copy.
type GuardrailBlock struct {
	ID    string
	Text  string // canonical text, newline-joined, no trailing newline
	Sites []GuardrailSite
	Line  int // line in the source file where `## guardrail: <id>` appears
}

// GuardrailSite is one declared copy location.
type GuardrailSite struct {
	Path  string // repo-relative
	Scrub bool   // apply the declared bundle substitutions before comparing
	Line  int    // line in the source file where this site was declared
}

// GuardrailScrub is one declared substitution applied to bundle sites.
type GuardrailScrub struct {
	From string
	To   string
	Line int
}

// GuardrailSource is the parsed declared source.
type GuardrailSource struct {
	Blocks []GuardrailBlock
	Scrubs []GuardrailScrub
}

// GuardrailReport is the three-state outcome of one run.
type GuardrailReport struct {
	// Compared is the number of (block, site) pairs actually read off disk and
	// byte-compared. A run that compared zero pairs proved nothing and is
	// reported as a failure by the caller.
	Compared int
	// Failed are real drift findings: the copy exists and does not match.
	Failed []Issue
	// Unchecked are could-not-check outcomes: the source, a site file, or a
	// block's anchor could not be read or located. NEVER treat as clean.
	Unchecked []Issue
}

// Clean reports whether the run is checked-clean: at least one comparison
// happened and nothing failed or went unchecked.
func (r GuardrailReport) Clean() bool {
	return r.Compared > 0 && len(r.Failed) == 0 && len(r.Unchecked) == 0
}

// ParseGuardrailSource reads and parses guardrailSourcePath under root.
//
// The format is deliberately line-oriented and hand-parsed rather than YAML:
// the file is ALSO the human-readable home for these rules, so it has to read as
// prose. The three constructs the parser cares about are unambiguous:
//
//	## scrub: bundle          opens the substitution table
//	- scrub: <from> => <to>   one substitution (order significant)
//	## guardrail: <id>        opens a block
//	- site: <path>            one copy site
//	- site: <path> (scrub: bundle)
//	```text ... ```           the canonical text (first fence after the sites)
func ParseGuardrailSource(root string) (*GuardrailSource, error) {
	abs := filepath.Join(root, filepath.FromSlash(guardrailSourcePath))
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("could-not-check: cannot read the declared guardrail source %s: %w", guardrailSourcePath, err)
	}

	src := &GuardrailSource{}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")

	var cur *GuardrailBlock
	inScrub := false
	inFence := false
	var fence []string

	flush := func() error {
		if cur == nil {
			return nil
		}
		if cur.Text == "" {
			return fmt.Errorf("%s:%d: guardrail %q declares no canonical text (a ```text fence)", guardrailSourcePath, cur.Line, cur.ID)
		}
		if len(cur.Sites) == 0 {
			return fmt.Errorf("%s:%d: guardrail %q declares no copy sites — a declared source with no consumers is not a check", guardrailSourcePath, cur.Line, cur.ID)
		}
		src.Blocks = append(src.Blocks, *cur)
		cur = nil
		return nil
	}

	for i, ln := range lines {
		n := i + 1
		trimmed := strings.TrimRight(ln, " \t")

		if inFence {
			if trimmed == "```" {
				inFence = false
				cur.Text = strings.Join(fence, "\n")
				fence = nil
				continue
			}
			fence = append(fence, ln)
			continue
		}

		switch {
		case trimmed == "## scrub: bundle":
			if err := flush(); err != nil {
				return nil, err
			}
			inScrub = true

		case strings.HasPrefix(trimmed, "## guardrail: "):
			if err := flush(); err != nil {
				return nil, err
			}
			inScrub = false
			id := strings.TrimSpace(strings.TrimPrefix(trimmed, "## guardrail: "))
			if id == "" {
				return nil, fmt.Errorf("%s:%d: `## guardrail:` with an empty id", guardrailSourcePath, n)
			}
			cur = &GuardrailBlock{ID: id, Line: n}

		case strings.HasPrefix(trimmed, "## "):
			// Any other H2 closes whatever was open. Prose sections between
			// blocks are expected and must not leak into the next block.
			if err := flush(); err != nil {
				return nil, err
			}
			inScrub = false

		case inScrub && strings.HasPrefix(trimmed, "- scrub: "):
			body := strings.TrimPrefix(trimmed, "- scrub: ")
			from, to, ok := strings.Cut(body, " => ")
			if !ok {
				return nil, fmt.Errorf("%s:%d: malformed scrub rule %q — want `- scrub: <from> => <to>`", guardrailSourcePath, n, body)
			}
			from, to = strings.TrimSpace(from), strings.TrimSpace(to)
			if from == "" {
				return nil, fmt.Errorf("%s:%d: scrub rule with an empty `from`", guardrailSourcePath, n)
			}
			src.Scrubs = append(src.Scrubs, GuardrailScrub{From: from, To: to, Line: n})

		case cur != nil && strings.HasPrefix(trimmed, "- site: "):
			body := strings.TrimPrefix(trimmed, "- site: ")
			site := GuardrailSite{Line: n}
			if p, rest, ok := strings.Cut(body, " ("); ok {
				site.Path = strings.TrimSpace(p)
				site.Scrub = strings.TrimSpace(rest) == "scrub: bundle)"
				if !site.Scrub {
					return nil, fmt.Errorf("%s:%d: unknown site qualifier %q — the only one is `(scrub: bundle)`", guardrailSourcePath, n, "("+rest)
				}
			} else {
				site.Path = strings.TrimSpace(body)
			}
			if site.Path == "" {
				return nil, fmt.Errorf("%s:%d: `- site:` with an empty path", guardrailSourcePath, n)
			}
			cur.Sites = append(cur.Sites, site)

		case cur != nil && trimmed == "```text":
			if cur.Text != "" {
				return nil, fmt.Errorf("%s:%d: guardrail %q declares a second ```text fence — one canonical text per block", guardrailSourcePath, n, cur.ID)
			}
			inFence = true
		}
	}
	if inFence {
		return nil, fmt.Errorf("%s: unterminated ```text fence", guardrailSourcePath)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(src.Blocks) == 0 {
		// Fail closed: a source declaring nothing would make every run
		// vacuously clean, which is the fail-open this check exists to remove.
		return nil, fmt.Errorf("could-not-check: %s declares no guardrail blocks — nothing to check is never a pass", guardrailSourcePath)
	}
	if err := guardrailDuplicateIDs(src.Blocks); err != nil {
		return nil, err
	}
	return src, nil
}

func guardrailDuplicateIDs(blocks []GuardrailBlock) error {
	seen := map[string]int{}
	for _, b := range blocks {
		if prev, dup := seen[b.ID]; dup {
			return fmt.Errorf("%s:%d: guardrail id %q is declared twice (first at line %d) — the source must have one entry per rule", guardrailSourcePath, b.Line, b.ID, prev)
		}
		seen[b.ID] = b.Line
	}
	return nil
}

// applyScrub returns text with every declared substitution applied in order.
func (s *GuardrailSource) applyScrub(text string) string {
	for _, sc := range s.Scrubs {
		text = strings.ReplaceAll(text, sc.From, sc.To)
	}
	return text
}

// expected returns the exact text a given site must carry for a given block.
func (s *GuardrailSource) expected(b GuardrailBlock, site GuardrailSite) string {
	if site.Scrub {
		return s.applyScrub(b.Text)
	}
	return b.Text
}

// locateBlock finds the one place in fileLines where expected's FIRST line
// occurs, and returns the 0-based index of that line.
//
// Anchoring on the first line (rather than on an injected marker comment) keeps
// the SKILL.md files free of machine syntax — they are read by a model at boot,
// and HTML comments in the middle of a list are both noise and a markdown
// hazard. The cost is that the anchor must be unique: 0 hits means the copy was
// deleted or its opening line was edited, >1 means the anchor is not
// discriminating. Both are could-not-check, not clean.
func locateBlock(fileLines []string, expected string) (idx int, hits int) {
	first := strings.Split(expected, "\n")[0]
	idx = -1
	for i, ln := range fileLines {
		if strings.TrimRight(ln, " \t\r") == strings.TrimRight(first, " \t\r") {
			hits++
			if idx < 0 {
				idx = i
			}
		}
	}
	return idx, hits
}

// CheckGuardrails compares every declared copy against the declared source.
// It never mutates anything; SyncGuardrails is the regenerating half.
func CheckGuardrails(root string) GuardrailReport {
	var rep GuardrailReport

	src, err := ParseGuardrailSource(root)
	if err != nil {
		rep.Unchecked = append(rep.Unchecked, Issue{Path: guardrailSourcePath, Msg: err.Error()})
		return rep
	}

	// A scrub rule nobody needs is a claim nobody checks. Catch it here rather
	// than letting a stale substitution sit in the table looking authoritative.
	used := make([]bool, len(src.Scrubs))
	for _, b := range src.Blocks {
		for i, sc := range src.Scrubs {
			if strings.Contains(b.Text, sc.From) {
				used[i] = true
			}
		}
	}
	for i, sc := range src.Scrubs {
		if !used[i] {
			rep.Failed = append(rep.Failed, Issue{
				Path: guardrailSourcePath,
				Msg: fmt.Sprintf("line %d: scrub rule %q => %q matches no declared guardrail text — a substitution nobody applies is a stale claim; delete it or fix it",
					sc.Line, sc.From, sc.To),
			})
		}
	}

	// Cache each site file once: several blocks share the same SKILL.md.
	type loaded struct {
		lines []string
		err   error
	}
	cache := map[string]loaded{}
	read := func(rel string) loaded {
		if l, ok := cache[rel]; ok {
			return l
		}
		raw, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		var l loaded
		if rerr != nil {
			l.err = rerr
		} else {
			l.lines = strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		}
		cache[rel] = l
		return l
	}

	for _, b := range src.Blocks {
		seenSite := map[string]bool{}
		for _, site := range b.Sites {
			if seenSite[site.Path] {
				rep.Failed = append(rep.Failed, Issue{
					Path: guardrailSourcePath,
					Msg:  fmt.Sprintf("line %d: guardrail %q declares site %s twice", site.Line, b.ID, site.Path),
				})
				continue
			}
			seenSite[site.Path] = true

			l := read(site.Path)
			if l.err != nil {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg:  fmt.Sprintf("could-not-check: guardrail %q declares this site but it cannot be read: %v", b.ID, l.err),
				})
				continue
			}

			want := src.expected(b, site)
			at, hits := locateBlock(l.lines, want)
			switch {
			case hits == 0:
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg: fmt.Sprintf("could-not-check: guardrail %q — its anchor line is not present, so the copy could not be located.\n  anchor: %q\n  Either the copy was deleted (remove the site from %s) or its first line was hand-edited (run `make guardrail-sync`).",
						b.ID, strings.Split(want, "\n")[0], guardrailSourcePath),
				})
				continue
			case hits > 1:
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg: fmt.Sprintf("could-not-check: guardrail %q — its anchor line occurs %d times, so the copy is ambiguous.\n  anchor: %q",
						b.ID, hits, strings.Split(want, "\n")[0]),
				})
				continue
			}

			rep.Compared++
			wantLines := strings.Split(want, "\n")
			if at+len(wantLines) > len(l.lines) {
				rep.Failed = append(rep.Failed, Issue{
					Path: site.Path,
					Msg:  fmt.Sprintf("line %d: guardrail %q is truncated — the file ends inside the block", at+1, b.ID),
				})
				continue
			}
			got := strings.Join(l.lines[at:at+len(wantLines)], "\n")
			if got != want {
				rep.Failed = append(rep.Failed, Issue{
					Path: site.Path,
					Msg:  fmt.Sprintf("line %d: guardrail %q has drifted from %s%s.\n%s", at+1, b.ID, guardrailSourcePath, scrubNote(site), guardrailDiff(want, got)),
				})
			}
		}
	}
	return rep
}

func scrubNote(site GuardrailSite) string {
	if site.Scrub {
		return " (with the declared bundle scrub applied)"
	}
	return ""
}

// guardrailDiff renders a compact per-line want/got so the failure message says
// WHICH line drifted, not merely that something did.
func guardrailDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	var b strings.Builder
	for i := range w {
		if i < len(g) && w[i] == g[i] {
			continue
		}
		fmt.Fprintf(&b, "    want[%d]: %q\n", i+1, w[i])
		if i < len(g) {
			fmt.Fprintf(&b, "    got [%d]: %q\n", i+1, g[i])
		} else {
			fmt.Fprintf(&b, "    got [%d]: <missing>\n", i+1)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// SyncGuardrails REGENERATES every declared copy from the declared source. This
// is the half that makes the copies derived rather than hand-maintained: the fix
// for a drift finding is to edit the source and re-run this, never to edit the
// copy.
//
// It can only rewrite a block it can locate. A site whose anchor is missing or
// ambiguous is returned as an unchecked Issue and left untouched — silently
// guessing where a rule belongs is worse than failing.
func SyncGuardrails(root string) (changed []string, rep GuardrailReport, err error) {
	src, perr := ParseGuardrailSource(root)
	if perr != nil {
		rep.Unchecked = append(rep.Unchecked, Issue{Path: guardrailSourcePath, Msg: perr.Error()})
		return nil, rep, perr
	}

	// Group by file so a file with two blocks is written once.
	type edit struct {
		at    int
		lines []string
	}
	perFile := map[string][]edit{}

	for _, b := range src.Blocks {
		for _, site := range b.Sites {
			raw, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(site.Path)))
			if rerr != nil {
				rep.Unchecked = append(rep.Unchecked, Issue{Path: site.Path, Msg: fmt.Sprintf("could-not-check: %v", rerr)})
				continue
			}
			fileLines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
			want := src.expected(b, site)
			at, hits := locateBlock(fileLines, want)
			if hits != 1 {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg:  fmt.Sprintf("could-not-check: guardrail %q anchor found %d times — not rewritten", b.ID, hits),
				})
				continue
			}
			perFile[site.Path] = append(perFile[site.Path], edit{at: at, lines: strings.Split(want, "\n")})
		}
	}

	paths := make([]string, 0, len(perFile))
	for p := range perFile {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		abs := filepath.Join(root, filepath.FromSlash(p))
		raw, rerr := os.ReadFile(abs)
		if rerr != nil {
			rep.Unchecked = append(rep.Unchecked, Issue{Path: p, Msg: fmt.Sprintf("could-not-check: %v", rerr)})
			continue
		}
		before := strings.ReplaceAll(string(raw), "\r\n", "\n")
		fileLines := strings.Split(before, "\n")
		edits := perFile[p]
		// Apply from the bottom up so earlier indices stay valid; every block
		// here is the same length as what it replaces, but bottom-up costs
		// nothing and survives a future variable-length block.
		sort.Slice(edits, func(i, j int) bool { return edits[i].at > edits[j].at })
		for _, e := range edits {
			end := e.at + len(e.lines)
			if end > len(fileLines) {
				rep.Unchecked = append(rep.Unchecked, Issue{Path: p, Msg: "could-not-check: block runs past end of file — not rewritten"})
				continue
			}
			out := make([]string, 0, len(fileLines))
			out = append(out, fileLines[:e.at]...)
			out = append(out, e.lines...)
			out = append(out, fileLines[end:]...)
			fileLines = out
		}
		after := strings.Join(fileLines, "\n")
		if after == before {
			continue
		}
		info, serr := os.Stat(abs)
		mode := os.FileMode(0o644)
		if serr == nil {
			mode = info.Mode().Perm()
		}
		if werr := os.WriteFile(abs, []byte(after), mode); werr != nil {
			return changed, rep, fmt.Errorf("rewriting %s: %w", p, werr)
		}
		changed = append(changed, p)
	}
	return changed, rep, nil
}
