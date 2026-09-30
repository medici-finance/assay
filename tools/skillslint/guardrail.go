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
	// Notes are advisory-only and fire in exactly one case: --allow-ambiguous-
	// extent was passed and SyncGuardrails took the longest-match guess over
	// an ambiguous extent anyway. By DEFAULT an ambiguous extent is refused — it
	// is reported in Unchecked, not here, and that block is not written (other
	// blocks in the same file still can be) — so Notes is no
	// longer the safety mechanism for that case (medici-finance/assay#1692,
	// round 3: it used to be exactly that, and a reviewer showed the "note
	// only" behaviour still let an ambiguous rewrite delete a local rule with
	// only a note that fires on every ordinary edit too). Notes now exists
	// purely so a caller who deliberately opted into the guess still gets an
	// audit trail rather than total silence. See matchExtent's doc comment.
	Notes []string
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
// invented out of thin air: guessing from the NEW text's length is what
// swallowed trailing content on growth and left stale lines on shrink, and
// guessing from HEAD's length alone did the same on a re-run or when the
// source edit was committed before the sync.
//
// This still leaves one narrower ambiguity content alone cannot resolve
// (#1692): when the longest known text matching at the anchor is NOT also the
// newest one matching there. Matching texts always nest, so that means a
// newer known text is a strict prefix of an older one — the block's history
// holds a prefix-shrink (the current text counts as the newest revision) —
// and the copy still matches both sides of it. The bytes then cannot tell a
// copy still genuinely at the older, longer text apart from a copy at the
// newer, shorter text followed by unrelated trailing content — possibly a
// local, site-specific rule someone added right after the block — that
// happens to equal the longer text's own tail. See matchExtent for the exact
// condition, and for why a grow history (older text a prefix of a newer one)
// is not ambiguous. Round 2 of this issue tried "take the longest match, but
// report the tie in rep.Notes so it is never silent"; round 3's review showed
// that does not work: the identical note fired on every ordinary edit as well
// as the genuinely ambiguous one, so it cannot act as a control, and the local
// rule is deleted anyway with only a stderr line to show for it. So by DEFAULT
// (allowAmbiguous == false) an ambiguous block is REFUSED outright:
// could-not-check, naming the file, the two lengths that matched and the exact
// span that would have been removed; that block is not written (another,
// unambiguous block in the same file still is). This does mean the ordinary
// "committed a prefix-shrink, then never synced" case refuses by default, and
// so does every later edit of that block while the copy still matches both
// texts — that case is structurally indistinguishable from the harmful one, so
// refusing it too is the point, not a gap. A caller who has checked by hand
// that the longer match is correct can pass allowAmbiguous == true (the CLI's
// --allow-ambiguous-extent) to take the longest match anyway, exactly as
// round 2 always did; SyncGuardrails still records that override in
// rep.Notes so it is visible, never silent, even when allowed.
//
// This ambiguity default (refuse, with an explicit opt-in) and the recency
// condition that decides what is ambiguous are reversible defaults chosen in
// this change, not rulings made elsewhere.
//
// Separately, a shrink of an uncommitted, unstaged edit that is never itself
// committed or staged (so no revision of it is ever KNOWN) is could-not-check
// only when nothing at the anchor matches at all; when the shrunk text still
// matches as a prefix of what is on disk, the copy reads as already synced
// and the trailing, no-longer-declared lines are left in place rather than
// guessed away.
//
// Every site file this run touches is read exactly ONCE, and every proven
// extent — used to decide both what to remove and what to write — comes from
// that single read; the write itself is then re-checked against a fresh read
// of the same path immediately before it happens, and refused rather than
// applied if the file changed underneath it, and performed via a temp file
// plus atomic rename rather than an in-place truncate-then-write. This closes
// a read-compute-write race a reviewer demonstrated with parallel syncs
// corrupting the file (medici-finance/assay#1692, round 3:
// F-1692-prove-apply-race) — see atomicWriteFile.
func SyncGuardrails(root string, prior []*GuardrailSource, allowAmbiguous bool) (changed []string, rep GuardrailReport, err error) {
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

	// Every site file this run touches is read AT MOST ONCE, cached by path.
	// The same read is what every block's extent is proven against below AND
	// what the write for that path is built from further down — there is no
	// second, later read standing between proving the extent and using it, so
	// a stale proof can no longer be spliced into content the tool has not
	// actually looked at (medici-finance/assay#1692, round 3:
	// F-1692-prove-apply-race). The write loop below still re-reads the file
	// once more, but only to VERIFY nothing changed since this read — never to
	// source the splice.
	type loadedFile struct {
		raw   string // normalised (CRLF -> LF), exactly as read
		lines []string
		mode  os.FileMode
		err   error
	}
	cache := map[string]*loadedFile{}
	load := func(rel string) *loadedFile {
		if l, ok := cache[rel]; ok {
			return l
		}
		l := &loadedFile{mode: 0o644}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		raw, rerr := os.ReadFile(abs)
		if rerr != nil {
			l.err = rerr
		} else {
			l.raw = strings.ReplaceAll(string(raw), "\r\n", "\n")
			l.lines = strings.Split(l.raw, "\n")
			if info, serr := os.Stat(abs); serr == nil {
				l.mode = info.Mode().Perm()
			}
		}
		cache[rel] = l
		return l
	}

	for _, b := range src.Blocks {
		for _, site := range b.Sites {
			l := load(site.Path)
			if l.err != nil {
				rep.Unchecked = append(rep.Unchecked, Issue{Path: site.Path, Msg: fmt.Sprintf("could-not-check: %v", l.err)})
				continue
			}
			want := src.expected(b, site)
			wantLines := strings.Split(want, "\n")
			at, hits := locateBlock(l.lines, want)
			if hits != 1 {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg:  fmt.Sprintf("could-not-check: guardrail %q anchor found %d times — not rewritten", b.ID, hits),
				})
				continue
			}
			// Newest first: want, then prior in the order it was given.
			known := append([]string{want}, priorSiteTexts(prior, b.ID, site)...)
			matched, newest, ok, ambiguous := matchExtent(l.lines, at, known)
			if !ok {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg: fmt.Sprintf("could-not-check: guardrail %q at line %d matches neither the current canonical text nor any of the %d earlier revision(s) of it available from %s, so where the copy ends cannot be proven — not rewritten.\n"+
						"  If the copy holds an earlier UNCOMMITTED edit of the source, restore it (`git checkout -- %s`) or `git add` that source edit before re-running; if it was hand-edited, replace it by hand with the canonical text.",
						b.ID, at+1, len(known)-1, guardrailSourcePath, site.Path),
				})
				continue
			}
			matchedLines := strings.Split(matched, "\n")
			// Name the newer, shorter text that matched truthfully: it is the
			// current canonical text only when that is what matched.
			newerDesc := "a more recent revision's"
			if newest == want {
				newerDesc = "the current canonical text's"
			}
			newestLen := len(strings.Split(newest, "\n"))
			if ambiguous && !allowAmbiguous {
				rep.Unchecked = append(rep.Unchecked, Issue{
					Path: site.Path,
					Msg: fmt.Sprintf("could-not-check: guardrail %q — the removal extent at %s:%d is AMBIGUOUS: both an older revision's %d line(s) and %s %d line(s) match at this anchor (the block once shrank to a prefix of itself). Content alone cannot tell a copy still genuinely at the older, longer text apart from a copy at the newer, shorter text followed by unrelated content — possibly a local, site-specific rule — that happens to equal the longer text's own tail. Refusing rather than guessing; this block is not rewritten.\n"+
						"  Ambiguous span: %s:%d-%d (the %d line(s) the longest-match rule would remove).\n"+
						"  Verify by hand (`git diff -- %s`); if the longer match is genuinely correct here, re-run with --allow-ambiguous-extent to take it. This refusal recurs on every sync of this block for as long as the copy matches both texts.",
						b.ID, site.Path, at+1, len(matchedLines), newerDesc, newestLen, site.Path, at+1, at+len(matchedLines), len(matchedLines), site.Path),
				})
				continue
			}
			if ambiguous {
				// allowAmbiguous is set: the caller explicitly opted into the
				// longest-match guess. Still never silent about it — see
				// GuardrailReport.Notes.
				rep.Notes = append(rep.Notes, fmt.Sprintf(
					"guardrail %q at %s:%d: --allow-ambiguous-extent took the longest match, removing %d line(s) (an older revision's length), though %s %d line(s) also matched at the same anchor. Verify the removed lines by hand (`git diff -- %s`).",
					b.ID, site.Path, at+1, len(matchedLines), newerDesc, newestLen, site.Path))
			}
			e := edit{id: b.ID, at: at, oldLen: len(matchedLines)}
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

		// Build the new content from the SAME read every edit above was
		// proven against — not a fresh read. See the cache comment above.
		l := cache[p]
		before := l.raw
		fileLines := append([]string(nil), l.lines...)
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

		// Re-check, immediately before writing, that the file on disk still
		// holds exactly the bytes this rewrite was proven against. The
		// extent for every edit above was proven against `before` (one read,
		// cached); a concurrent sync (or any other writer) racing this one
		// could have rewritten the file since. Applying this rewrite's
		// splice to content that has since changed is exactly the stale-proof
		// bug a reviewer demonstrated corrupting files under parallel syncs
		// (medici-finance/assay#1692, round 3: F-1692-prove-apply-race) —
		// refuse instead of guessing the file still matches.
		abs := filepath.Join(root, filepath.FromSlash(p))
		freshRaw, rerr := os.ReadFile(abs)
		if rerr != nil {
			rep.Unchecked = append(rep.Unchecked, Issue{Path: p, Msg: fmt.Sprintf("could-not-check: %v", rerr)})
			continue
		}
		if strings.ReplaceAll(string(freshRaw), "\r\n", "\n") != before {
			rep.Unchecked = append(rep.Unchecked, Issue{
				Path: p,
				Msg:  "could-not-check: the file changed on disk between proving the removal extent and writing (a concurrent sync or edit) — not rewritten to avoid applying a stale proof; re-run sync",
			})
			continue
		}

		if werr := atomicWriteFile(abs, []byte(after), l.mode); werr != nil {
			return changed, rep, fmt.Errorf("rewriting %s: %w", p, werr)
		}
		changed = append(changed, p)
	}
	return changed, rep, nil
}

// atomicWriteFile writes data to a temp file created in path's own directory,
// then renames it over path — the same temp-file-plus-rename shape this
// repo's other in-place regenerators use for exactly this reason (for
// example tools/desk/cmd/desksupervise/status.go's writeStatusJSON,
// tools/desk/cmd/deskmonitor/state.go, tools/desk/internal/commsqueue's
// queue.go): a reader, or a concurrent writer of the same path, always sees
// either the whole old file or the whole new one, never bytes from both. The
// in-place os.WriteFile this replaced does an OS-level truncate-then-write,
// which is exactly what let two concurrent `--sync` runs interleave their
// writes into a single corrupted file (medici-finance/assay#1692, round 3:
// F-1692-prove-apply-race, reproduced with parallel syncs before this fix).
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".skillslint-tmp-*")
	if err != nil {
		return fmt.Errorf("cannot create a temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("cannot write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot rename %s to %s: %w", tmpName, path, err)
	}
	return nil
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
//
// `known` must be ordered NEWEST FIRST: `want` (the current canonical text) at
// index 0, then the earlier revisions as priorGuardrailSources returns them
// (staged, then commits in `git log` order). NEWEST is the first of them that
// matches at the anchor.
//
// AMBIGUOUS reports the one case "longest wins" cannot get right by content
// alone: the longest match is not also the newest match. Two matching texts
// always nest (the shorter is a prefix of the longer), so this happens exactly
// when a NEWER known text is a strict prefix of an OLDER one, that is, when
// the block's history has a prefix-shrink in it (current text included) and
// the copy still matches both sides of it. Then a copy genuinely still at the
// older, longer text is indistinguishable, byte for byte, from a copy at the
// newer, shorter text followed by unrelated content — possibly a local,
// site-specific rule — that happens to equal the longer text's own tail
// (#1692). When the newer, shorter text is `want` this is the unsynced or
// committed-first prefix-shrink; when it is an earlier revision, the block has
// been edited again since a committed prefix-shrink. Either way the caller
// (SyncGuardrails) refuses by default, naming the file, the two lengths that
// matched and the span that would have been removed, and takes the longest
// match only when the caller has explicitly opted in (--allow-ambiguous-extent).
//
// A grow history is NOT ambiguous: after a committed append-grow the older,
// shorter text is a prefix of the newer, longer one, so both match at a copy
// synced to the newer text — but the longest match is also the newest, which
// is what a synced copy holds. Counting that as a tie (as an earlier draft
// did, by flagging any two matching lengths) refused every later edit of a
// block that had ever grown. What this re-admits, deliberately: content right
// under a copy that exactly repeats the lines a later grow added is taken as
// part of the block and replaced by a later edit. Two routes lead there: a
// copy that MISSED a sync, still at an older text (at least two revisions
// behind the source); and, more ordinarily, a site-local line that a later
// grow promoted verbatim into the canonical block. After such a grow the copy
// is byte-identical to the grown text, so CheckGuardrails reports it synced,
// and the next edit replaces the promoted line. That is defensible (the line
// became canonical), but it is no longer the site's own text.
//
// See SyncGuardrails' doc comment for the full history: round 2 tried
// reporting the tie instead of refusing it, and round 3's review showed that
// did not actually protect anything, because the same note fired on every
// ordinary edit too.
func matchExtent(fileLines []string, at int, known []string) (matched, newest string, ok, ambiguous bool) {
	for _, k := range known {
		kl := strings.Split(k, "\n")
		if at+len(kl) > len(fileLines) || strings.Join(fileLines[at:at+len(kl)], "\n") != k {
			continue
		}
		if !ok {
			newest = k
		}
		if !ok || len(kl) > len(strings.Split(matched, "\n")) {
			matched, ok = k, true
		}
	}
	ambiguous = ok && matched != newest
	return matched, newest, ok, ambiguous
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
