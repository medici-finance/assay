package main

// The sweep engine.
//
// Order matters and is the same discipline as the three-state rule: for every
// token entry, look for the CONTROL first, then the token.
//
//   - control found, token absent   → checked-clean   (this alone certifies)
//   - token present                 → checked-failed   (a withheld string is in the tree)
//   - control NOT found             → could-not-check  (the search proved nothing:
//                                                        a zero token count from a
//                                                        broken search is not "clean")
//
// A file that cannot be read is itself a could-not-check: a leak could be hiding
// in the bytes we never saw. A missing tree is could-not-check too — the most
// likely real-world invocation error, and the one that must never certify.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// stateClean etc. are the three per-entry states, spelled exactly as they print
// so the Verify rows that grep the output are greping for a real constant.
const (
	stateClean         = "checked-clean"
	stateFailed        = "checked-failed"
	stateCouldNotCheck = "could-not-check"
)

// fileText is one file's searchable form plus its tree-relative path.
type fileText struct {
	path string
	ex   extracted
}

// corpus is the whole swept tree.
type corpus struct {
	tree         string
	files        []fileText
	unreadable   []string
	unsearchable []string // binaries the sweep cannot exhaustively search (#528)
	nonRegular   []string // symlinks/devices: not readable as content, may hide a token
}

// entryResult is the outcome for one token entry.
type entryResult struct {
	token  string
	state  string
	reason string
	hits   []string // files the token was found in (checked-failed only)
}

// sweepReport is the whole run.
type sweepReport struct {
	tree         string
	results      []entryResult
	unreadable   []string
	unsearchable []string
	nonRegular   []string
	filesSwept   int
	contentHash  string
	provenance   string
}

// buildCorpus reads every regular file under tree and extracts its searchable
// text. A missing tree is could-not-check (exit 3), never an empty clean run.
func buildCorpus(tree string) (*corpus, error) {
	info, err := os.Stat(tree)
	if err != nil {
		return nil, withExit(exitCouldNotCheck,
			"could-not-check: tree %s cannot be read (%v) — a missing or unreadable tree must never certify", tree, err)
	}
	if !info.IsDir() {
		return nil, withExit(exitCouldNotCheck, "could-not-check: %s is not a directory", tree)
	}
	c := &corpus{tree: tree}
	err = filepath.WalkDir(tree, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// A directory we cannot descend is a could-not-check condition, not a
			// stop: record it and keep going so the report names every gap.
			rel, _ := filepath.Rel(tree, p)
			c.unreadable = append(c.unreadable, filepath.ToSlash(rel))
			return nil
		}
		if d.IsDir() {
			// Never descend a .git dir — a staged tree is history-free, but this
			// tool also runs against the source repo (Verify row 4), and history
			// is not what the sweep is about.
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(tree, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if !d.Type().IsRegular() {
			// A non-regular file (symlink, device) is not read as content, so a
			// token can hide where the sweep never looks — in a symlink's TARGET
			// string, which the stager recreates verbatim and git stores as the
			// blob. Treating it as invisible would let a withheld token certify
			// clean (the one thing this tool never does), so it is recorded and
			// turned into could-not-check, the same discipline as an unreadable
			// or unsearchable file. WalkDir does not descend symlinked dirs, so
			// a symlinked directory's whole subtree is covered by this entry.
			c.nonRegular = append(c.nonRegular, rel)
			return nil
		}
		data, rderr := os.ReadFile(p)
		if rderr != nil {
			c.unreadable = append(c.unreadable, rel)
			return nil
		}
		ex := extract(data)
		if ex.kind == "binary-unsearchable" {
			// A binary the sweep cannot prove it exhaustively searched. A token
			// could hide in a compression/encoding layer the sweep does not
			// unwrap, so the file is refused (could-not-check), never silently
			// searched as if its raw bytes were the whole story (#528).
			c.unsearchable = append(c.unsearchable, rel)
			return nil
		}
		c.files = append(c.files, fileText{path: rel, ex: ex})
		return nil
	})
	if err != nil {
		return nil, withExit(exitCouldNotCheck, "could-not-check: walking %s: %v", tree, err)
	}
	sort.Slice(c.files, func(i, j int) bool { return c.files[i].path < c.files[j].path })
	sort.Strings(c.unreadable)
	sort.Strings(c.unsearchable)
	sort.Strings(c.nonRegular)
	return c, nil
}

// sweep runs every entry against the corpus.
func sweep(tl *TokenList, c *corpus) *sweepReport {
	rep := &sweepReport{
		tree: c.tree, unreadable: c.unreadable, unsearchable: c.unsearchable,
		nonRegular: c.nonRegular, filesSwept: len(c.files),
	}
	for _, e := range tl.Entries {
		rep.results = append(rep.results, evaluate(e, c))
	}
	rep.contentHash = contentHash(c)
	return rep
}

// evaluate is the three-state decision for one entry.
func evaluate(e *Entry, c *corpus) entryResult {
	// Control first — for control-bearing (literal/word) entries only. If the
	// control is not present anywhere in the tree, the search mechanism is unproven
	// for this entry and a zero token count means nothing.
	//
	// A `pattern` entry (e.control == nil) is DISCOVERY-STYLE and skips this gate:
	// a class rule has no single sibling control to prove, so ANY match is a leak
	// and no-match is clean. Its fail-closed proof-of-search is supplied by the
	// run's control-bearing entries (whose controls prove the corpus was read) plus
	// the corpus-level could-not-check for any unreadable/unsearchable/non-regular
	// file — a pattern can only ADD a leak, never mask a could-not-check (worstExit
	// is worst-state-wins). See tokens.go for the full argument.
	if e.control != nil {
		controlFound := false
		for _, f := range c.files {
			if e.control.found(f.ex.text) {
				controlFound = true
				break
			}
		}
		if !controlFound {
			return entryResult{
				token: e.Token, state: stateCouldNotCheck,
				reason: fmt.Sprintf("control %q not found in the tree — the search proved nothing, so a zero token count is not evidence of absence", e.Control),
			}
		}
	}
	// Token next. Collect the files it appears in, minus any allow-listed path
	// and minus any file whose every occurrence is inside a sanctioned longer
	// identifier (the `except` predicate — see rules.go).
	var hits []string
	for _, f := range c.files {
		if !e.matcher.found(f.ex.text) {
			continue
		}
		if allowed(e, f.path) {
			continue
		}
		// The exception predicate applied at FILE granularity. This engine matches
		// per-file rather than per-match, so it cannot report the identifier window
		// a hit sits inside; striking the sanctioned forms out of the whole file
		// text and re-asking is the same question at the resolution this engine
		// has. It agrees with the external engines' per-match answer on whether the
		// FILE is dirty, which is all this engine ever reported anyway.
		if len(e.Except) > 0 && excused(e, f.ex.text) {
			continue
		}
		hits = append(hits, f.path)
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		return entryResult{token: e.Token, state: stateFailed, reason: e.Reason, hits: hits}
	}
	if e.control == nil {
		// A discovery-style pattern has no control to name; its clean state is
		// simply "the class rule matched nothing".
		return entryResult{token: e.Token, state: stateClean, reason: "pattern present as a class rule, 0 hits"}
	}
	return entryResult{token: e.Token, state: stateClean, reason: fmt.Sprintf("control %q present, 0 hits", e.Control)}
}

// allowed reports whether a token hit in path is permitted by an allow rule.
func allowed(e *Entry, path string) bool {
	for _, a := range e.Allow {
		if path == a.Path {
			return true
		}
		if strings.HasSuffix(a.Path, "/") && strings.HasPrefix(path, a.Path) {
			return true
		}
	}
	return false
}

// worstExit turns the report into a single exit code, worst-state-wins, with a
// definite leak dominating an unchecked entry. Both are non-zero; the summary
// line carries every count so nothing is masked by the choice.
func (rep *sweepReport) worstExit() int {
	leak, cnc := false, false
	for _, r := range rep.results {
		switch r.state {
		case stateFailed:
			leak = true
		case stateCouldNotCheck:
			cnc = true
		}
	}
	if len(rep.unreadable) > 0 {
		cnc = true
	}
	if len(rep.unsearchable) > 0 {
		// A binary the sweep could not exhaustively search is the same state as
		// an unreadable file: a leak could be hiding in bytes never fully seen.
		cnc = true
	}
	if len(rep.nonRegular) > 0 {
		// A non-regular file is never read as content, so a token in a symlink's
		// target string (or a device) is never seen — the same blind spot.
		cnc = true
	}
	switch {
	case leak:
		return exitLeak
	case cnc:
		return exitCouldNotCheck
	default:
		return exitClean
	}
}

// counts returns (clean, failed, couldNotCheck) across entries.
func (rep *sweepReport) counts() (int, int, int) {
	var clean, failed, cnc int
	for _, r := range rep.results {
		switch r.state {
		case stateClean:
			clean++
		case stateFailed:
			failed++
		case stateCouldNotCheck:
			cnc++
		}
	}
	return clean, failed, cnc
}

// write prints the per-entry lines, the unreadable-file lines, the summary, and
// — only when the run is CLEAN — the certificate. The summary carries counts so
// a truncated log cannot look like a pass, and the certificate names the tree's
// content hash so it cannot be re-used for a different tree.
func (rep *sweepReport) write(w io.Writer) {
	for _, r := range rep.results {
		if r.state == stateFailed {
			fmt.Fprintf(w, "%s %s — %s in %d file(s): %s\n",
				r.state, r.token, r.reason, len(r.hits), strings.Join(r.hits, ", "))
			continue
		}
		fmt.Fprintf(w, "%s %s — %s\n", r.state, r.token, r.reason)
	}
	for _, u := range rep.unreadable {
		fmt.Fprintf(w, "%s <unreadable> — could not read %s; a leak could hide in bytes never seen\n", stateCouldNotCheck, u)
	}
	for _, u := range rep.unsearchable {
		fmt.Fprintf(w, "%s <unsearchable-binary> — %s is a binary whose content the sweep cannot exhaustively search; a leak could hide in a compression/encoding layer the sweep does not unwrap (#528)\n", stateCouldNotCheck, u)
	}
	for _, u := range rep.nonRegular {
		fmt.Fprintf(w, "%s <non-regular> — %s is not a regular file (symlink/device); a token could hide in its target string, which the sweep does not read\n", stateCouldNotCheck, u)
	}
	clean, failed, cnc := rep.counts()
	// The count labels here deliberately do NOT spell the three state markers
	// (`checked-clean` / `checked-failed` / `could-not-check`). Those literals
	// appear in this tool's output ONLY on the per-entry lines above, so a grep
	// that counts a marker counts entries in that state and nothing else — which
	// is what lets the Verify rows assert "N entries could-not-check" from
	// `grep -cF`. Echoing the marker in a summary label would make a count of the
	// marker on a fully-clean tree non-zero, and the summary would defeat the very
	// distinction it exists to make.
	fmt.Fprintf(w,
		"SUMMARY: entries=%d clean=%d failed=%d unchecked=%d files-swept=%d unreadable=%d unsearchable=%d non-regular=%d tree=%s\n",
		len(rep.results), clean, failed, cnc+len(rep.unreadable)+len(rep.unsearchable)+len(rep.nonRegular), rep.filesSwept, len(rep.unreadable), len(rep.unsearchable), len(rep.nonRegular), rep.tree)
	if rep.worstExit() == exitClean {
		prov := rep.provenance
		if prov == "" {
			prov = "no pubmanifest stage marker (swept anyway)"
		}
		fmt.Fprintf(w,
			"CERTIFICATE: tree=%s content-hash=sha256:%s files=%d entries-passed=%d provenance=%s\n",
			rep.tree, rep.contentHash, rep.filesSwept, clean, prov)
		return
	}
	fmt.Fprintf(w,
		"NOT CERTIFIED: tree=%s content-hash=sha256:%s — %d leak(s), %d unchecked; a tree that could not be fully checked is never certified\n",
		rep.tree, rep.contentHash, failed, cnc+len(rep.unreadable)+len(rep.unsearchable)+len(rep.nonRegular))
}
