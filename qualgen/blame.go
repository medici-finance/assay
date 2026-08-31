package main

import (
	"fmt"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// blame.go — the blame helper the B-SZZ trace engine (szz.go) calls. Given a file
// and a set of line numbers at a given commit, it resolves the commit that last
// touched each line (spec §5.2). It is READ-ONLY over the repository.
//
// Three-state boundary (spec §3.2): blame is an instrument, so its own failure is
// reported as ITSELF, never as an empty-but-successful answer. A blob that cannot
// be blamed — a path absent at the commit, a binary/unreadable object, an
// unreachable parent in the blame walk — returns a NON-NIL error. The trace engine
// turns that error into a could-not-trace/`blame-error` outcome; a caller must
// never read a returned error as "zero inducers".

// LineInducer is the blame answer for one line: the commit that last introduced
// the line's current text, that commit's author date, and the line text.
//
//   - Commit and Date are the load-bearing fields the refinements read: Date drives
//     the postdating filter (refine.go), Commit is the candidate inducer.
//   - Text lets the cosmetic fall-through (refine.go) re-locate the same logical
//     line across the small line-number shifts a reformat introduces, by matching
//     whitespace-normalized content rather than a brittle line number.
type LineInducer struct {
	Commit string
	Date   time.Time
	Text   string
}

// blameFile blames EVERY line of path at commit c, returning a map keyed by the
// 1-based line number. It is the raw blame primitive; blameLines narrows it to the
// specific lines a fix changed, and the cosmetic fall-through re-blames whole files
// to match a line by content.
//
// A blame that cannot run returns a non-nil error (the three-state boundary above),
// never a partial map.
func blameFile(c *object.Commit, path string) (map[int]LineInducer, error) {
	res, err := git.Blame(c, path)
	if err != nil {
		return nil, fmt.Errorf("blame %s at %s: %w", path, shortSHA(c.Hash.String()), err)
	}
	out := make(map[int]LineInducer, len(res.Lines))
	for i, ln := range res.Lines {
		out[i+1] = LineInducer{
			Commit: ln.Hash.String(),
			Date:   ln.Date.UTC(),
			Text:   ln.Text,
		}
	}
	return out, nil
}

// blameLines blames path at commit c and returns only the requested 1-based line
// numbers (the old-side lines a fix deleted or modified). A requested line beyond
// the file's length at c is silently omitted from the result — a fix may delete
// trailing lines that never existed in a shorter parent image — but that is NOT an
// error: the caller distinguishes "no lines to blame" (a blameless, addition-only
// fix — handled before this is ever called) from "blame failed" (this returns an
// error) by which of the two it observes.
func blameLines(c *object.Commit, path string, lines []int) (map[int]LineInducer, error) {
	all, err := blameFile(c, path)
	if err != nil {
		return nil, err
	}
	out := make(map[int]LineInducer, len(lines))
	for _, n := range lines {
		if li, ok := all[n]; ok {
			out[n] = li
		}
	}
	return out, nil
}
