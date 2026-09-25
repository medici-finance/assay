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
	"os/exec"
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
	return parseGuardrailBytes(raw)
}

// parseGuardrailBytes is the byte-level half of ParseGuardrailSource, split out
// so a PREVIOUS revision of the source (fetched from git, not the working
// tree) can be parsed the same way SyncGuardrails parses the current one — see
// priorGuardrailSources and the `prior` parameter of SyncGuardrails, which use
// earlier texts of a block to prove where its copy ends
// (medici-finance/assay#1690).
func parseGuardrailBytes(raw []byte) (*GuardrailSource, error) {
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
//
// It can only rewrite a block whose EXTENT it can prove (medici-finance/assay#1690).
// A copy carries no end marker (see locateBlock), so the anchor says where a
// block starts but not where it stops. The only safe answer is by content: the
// lines at the anchor must equal, byte-for-byte, a text this block is KNOWN to
// have had at that site — the current canonical text (the copy is already
// synced: no write) or its text in one of `prior`, earlier revisions of the
// declared source (in production, every committed and staged revision, via
// priorGuardrailSources). The removal is exactly the matched text's length;
// the new text is inserted in its place. When no known text matches — no
// history, a history that never held the copy's text, a hand-drifted copy —
// the site is could-not-check and its file is not written. A length is never
// guessed: guessing from the NEW text's length is what swallowed trailing
// content on growth and left stale lines on shrink, and guessing from HEAD's
// length alone did the same on a re-run or when the source edit was committed
// before the sync.
func SyncGuardrails(root string, prior []*GuardrailSource) (changed []string, rep GuardrailReport, err error) {
	src, perr := ParseGuardrailSource(root)
	if perr != nil {
		rep.Unchecked = append(rep.Unchecked, Issue{Path: guardrailSourcePath, Msg: perr.Error()})
		return nil, rep, perr
	}

	// Group by file so a file with two blocks is written once. Every located
	// block is recorded — including an already-synced one that needs no write —
	// so overlapping extents in one file can be refused before anything moves.
	type edit struct {
		id     string
		at     int
		oldLen int      // the PROVEN length of the text being replaced
		lines  []string // nil: already carries the canonical text, no write
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
			wantLines := strings.Split(want, "\n")
			at, hits := locateBlock(fileLines, want)
			if hits != 1 {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg:  fmt.Sprintf("could-not-check: guardrail %q anchor found %d times — not rewritten", b.ID, hits),
				})
				continue
			}
			known := append([]string{want}, priorSiteTexts(prior, b.ID, site)...)
			matched, ok := matchExtent(fileLines, at, known)
			if !ok {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg: fmt.Sprintf("could-not-check: guardrail %q at line %d matches neither the current canonical text nor any of the %d earlier revision(s) of it available from %s, so where the copy ends cannot be proven — not rewritten.\n"+
						"  If the copy holds an earlier UNCOMMITTED edit of the source, restore it (`git checkout -- %s`) or `git add` that source edit before re-running; if it was hand-edited, replace it by hand with the canonical text.",
						b.ID, at+1, len(known)-1, guardrailSourcePath, site.Path),
				})
				continue
			}
			e := edit{id: b.ID, at: at, oldLen: len(strings.Split(matched, "\n"))}
			if matched != want {
				e.lines = wantLines
			}
			perFile[site.Path] = append(perFile[site.Path], e)
		}
	}

	paths := make([]string, 0, len(perFile))
	for p := range perFile {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		edits := perFile[p]
		// Apply from the bottom up so earlier indices stay valid.
		sort.Slice(edits, func(i, j int) bool { return edits[i].at > edits[j].at })
		overlap := false
		for i := 1; i < len(edits); i++ {
			if lo, hi := edits[i], edits[i-1]; lo.at+lo.oldLen > hi.at {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: p,
					Msg:  fmt.Sprintf("could-not-check: guardrails %q (line %d) and %q (line %d) overlap — not rewritten", lo.id, lo.at+1, hi.id, hi.at+1),
				})
				overlap = true
			}
		}
		if overlap {
			continue
		}

		abs := filepath.Join(root, filepath.FromSlash(p))
		raw, rerr := os.ReadFile(abs)
		if rerr != nil {
			rep.Unchecked = append(rep.Unchecked, Issue{Path: p, Msg: fmt.Sprintf("could-not-check: %v", rerr)})
			continue
		}
		before := strings.ReplaceAll(string(raw), "\r\n", "\n")
		fileLines := strings.Split(before, "\n")
		for _, e := range edits {
			if e.lines == nil {
				continue
			}
			out := make([]string, 0, len(fileLines)-e.oldLen+len(e.lines))
			out = append(out, fileLines[:e.at]...)
			out = append(out, e.lines...)
			out = append(out, fileLines[e.at+e.oldLen:]...)
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

// priorSiteTexts returns every distinct text block `blockID` had at `site` in
// the prior revisions, each rendered with that revision's own scrub table.
func priorSiteTexts(prior []*GuardrailSource, blockID string, site GuardrailSite) []string {
	var out []string
	seen := map[string]bool{}
	for _, old := range prior {
		if old == nil {
			continue
		}
		for _, ob := range old.Blocks {
			if ob.ID != blockID {
				continue
			}
			for _, osite := range ob.Sites {
				if osite.Path != site.Path {
					continue
				}
				if t := old.expected(ob, osite); !seen[t] {
					seen[t] = true
					out = append(out, t)
				}
			}
		}
	}
	return out
}

// matchExtent returns the LONGEST of `known` whose lines appear verbatim in
// fileLines starting at `at`. Longest wins because a shorter known text can be
// a prefix of a longer one: after a grow is synced, the copy begins with the
// old, shorter text too, and removing only that would duplicate the new
// block's tail; before a prefix-shrink is synced, the copy begins with the new
// text too, and treating it as synced would leave the old tail behind.
func matchExtent(fileLines []string, at int, known []string) (string, bool) {
	best, found := "", false
	for _, k := range known {
		kl := strings.Split(k, "\n")
		if at+len(kl) > len(fileLines) || strings.Join(fileLines[at:at+len(kl)], "\n") != k {
			continue
		}
		if !found || len(kl) > len(strings.Split(best, "\n")) {
			best, found = k, true
		}
	}
	return best, found
}

// priorGuardrailSources is SyncGuardrails' `prior` in production: every
// revision of the declared source git knows about under `root` — the staged
// copy plus every commit that touched it, newest first — each parsed with the
// same parser as the working tree. These are only CANDIDATE texts: a revision
// is used for a site only when its text matches the copy on disk exactly, so a
// wrong or stale revision cannot cause a write, and an unparseable one is
// skipped (and noted) rather than turned into a guessed length.
//
// Paths are passed as `./`-relative so git resolves them against `root`, not
// the repository top. No history (not a git checkout, no commits, git absent)
// returns nil plus a note; SyncGuardrails then rewrites only copies it can
// prove from the current text, and reports every other as could-not-check.
func priorGuardrailSources(root string) (prior []*GuardrailSource, notes []string) {
	gitOut := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		return cmd.Output()
	}
	rel := "./" + guardrailSourcePath

	var revs []string
	if _, err := gitOut("rev-parse", "--verify", "-q", "HEAD"); err != nil {
		notes = append(notes, fmt.Sprintf("no git history for %s under %s — only copies already carrying the current canonical text can be proven", guardrailSourcePath, root))
	} else {
		out, err := gitOut("log", "--format=%H", "--", rel)
		if err != nil {
			notes = append(notes, fmt.Sprintf("git log of %s failed (%v) — earlier revisions unavailable", guardrailSourcePath, err))
		} else {
			revs = strings.Fields(string(out))
		}
	}

	skipped := 0
	seen := map[string]bool{}
	load := func(spec string, required bool) {
		raw, err := gitOut("show", spec)
		if err != nil {
			if required {
				skipped++
			}
			return
		}
		if seen[string(raw)] {
			return
		}
		seen[string(raw)] = true
		src, perr := parseGuardrailBytes(raw)
		if perr != nil {
			skipped++
			return
		}
		prior = append(prior, src)
	}
	load(":"+rel, false) // the staged copy, if any
	for _, h := range revs {
		load(h+":"+rel, true)
	}
	if skipped > 0 {
		notes = append(notes, fmt.Sprintf("skipped %d revision(s) of %s that could not be read or parsed", skipped, guardrailSourcePath))
	}
	return prior, notes
}
