package main

import (
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// refine.go — the three standard B-SZZ refinements (spec §5.2), applied in order
// to each blame candidate before it is accepted as an inducing commit:
//
//  1. cosmetic/format-only exclusion: a candidate whose touch of the blamed line
//     was whitespace/format-only did not INDUCE anything — fall through to the
//     prior real inducer.
//  2. postdating exclusion: a candidate whose commit date is AFTER the defect was
//     reported cannot have induced it.
//  3. confidence scorer: score how strongly the surviving blame agrees, carried on
//     the DefectTrace for consumers to weight (a recorded field, never a gate).
//
// All three are READ-ONLY over the repository.

// inducerReason is the outcome of resolving a raw blame candidate through the
// cosmetic fall-through (resolveInducer). Only reasonInducerOK yields a usable
// inducer; the others are why one could not be pinned in a single blame hop.
type inducerReason int

const (
	// reasonInducerOK: a real, non-cosmetic inducing commit was pinned.
	reasonInducerOK inducerReason = iota
	// reasonInducerMultiHop: the blame landed on a merge/squash commit (or a
	// cosmetic chain that cycled) — not attributable to one mainline change in a
	// single hop. Maps to could-not-trace/`multi-hop`.
	reasonInducerMultiHop
	// reasonInducerUnreachable: the cosmetic fall-through walked off the mined
	// history floor (a parent object absent, or the line's content no longer
	// locatable). Maps to could-not-trace/`squash-floor` at the fix level only when
	// nothing else resolves; here it just means this candidate could not be pinned.
	reasonInducerUnreachable
	// reasonInducerBlameError: re-blame during the cosmetic fall-through failed.
	// Maps to could-not-trace/`blame-error`.
	reasonInducerBlameError
)

// resolveInducer takes one raw blame result for a line of file and returns the
// real inducing commit, walking PAST any cosmetic/format-only commits (refinement
// 1). A blamed line whose last-touching commit only reformatted it did not induce
// the defect; the true inducer is the commit before the reformat that last touched
// the line's real content. The walk re-blames the file at each cosmetic commit's
// parent and re-locates the line by whitespace-normalized content, so it survives
// the line-number shifts a reformat introduces.
//
// A merge/squash commit is not attributable in one hop (reasonInducerMultiHop); an
// unreachable parent or an unlocatable line ends the walk (reasonInducerUnreachable);
// a re-blame failure is reasonInducerBlameError. A self-referential cosmetic cycle
// is broken and reported as multi-hop.
//
// get resolves a commit hash to its object (in production, repo.CommitObject); the
// seam keeps the cosmetic walk testable against a real fixture repository without
// mocking go-git's concrete types.
type commitResolver func(hash string) (*object.Commit, error)

func resolveInducer(get commitResolver, file string, li LineInducer) (LineInducer, inducerReason) {
	seen := map[string]bool{}
	cur := li
	for {
		if seen[cur.Commit] {
			// A cosmetic cycle: refuse to loop, report as unattributable.
			return cur, reasonInducerMultiHop
		}
		seen[cur.Commit] = true

		c, err := get(cur.Commit)
		if err != nil {
			return cur, reasonInducerUnreachable
		}
		if c.NumParents() > 1 {
			// A merge/squash commit bundles many changes; the true inducing change
			// is one hop deeper than this blame can see.
			return cur, reasonInducerMultiHop
		}

		cosmetic, err := isCosmeticChange(c, file)
		if err != nil {
			return cur, reasonInducerBlameError
		}
		if !cosmetic {
			// A real, content-changing inducer — accept it.
			return cur, reasonInducerOK
		}

		// Cosmetic: fall through to the commit before this reformat by re-blaming
		// the file at this commit's parent and re-locating the same logical line.
		parent, err := c.Parent(0)
		if err != nil {
			return cur, reasonInducerUnreachable
		}
		blamed, err := blameFile(parent, file)
		if err != nil {
			return cur, reasonInducerBlameError
		}
		next, ok := matchLineByContent(blamed, cur.Text)
		if !ok {
			return cur, reasonInducerUnreachable
		}
		cur = next
	}
}

// isCosmeticChange reports whether commit c's effect on path was whitespace or
// format-only — i.e. the file's content at c is identical to its content at c's
// first parent once ALL whitespace is stripped. Such a commit introduced no real
// line and must never be recorded as an inducer.
//
//   - A root commit (no parent) that carries the file INTRODUCED it: a real change,
//     never cosmetic.
//   - A file absent at the parent was likewise introduced by c: a real change.
//   - A binary/unreadable blob cannot be compared as text — reported as a non-nil
//     error so the caller degrades to could-not-trace, never a silent "cosmetic".
func isCosmeticChange(c *object.Commit, path string) (bool, error) {
	if c.NumParents() == 0 {
		return false, nil
	}
	parent, err := c.Parent(0)
	if err != nil {
		// Cannot see the pre-image to compare — conservatively NOT cosmetic, so a
		// real inducer is never dropped on an unresolvable parent.
		return false, nil
	}
	newContent, err := fileContents(c, path)
	if err != nil {
		// The file is absent at c, or unreadable. Absent-at-c means c did not touch
		// it as text we can compare; treat as non-cosmetic (do not drop a candidate
		// we cannot prove cosmetic).
		return false, nil
	}
	oldContent, err := fileContents(parent, path)
	if err != nil {
		// Absent at the parent: c introduced the file → a real change, not cosmetic.
		return false, nil
	}
	return normalizeWhitespace(oldContent) == normalizeWhitespace(newContent), nil
}

// fileContents returns the text of path in commit c's tree. A missing file or an
// unreadable blob returns a non-nil error.
func fileContents(c *object.Commit, path string) (string, error) {
	f, err := c.File(path)
	if err != nil {
		return "", err
	}
	return f.Contents()
}

// matchLineByContent finds, among a blamed file, the line whose whitespace-
// normalized text equals that of want, and returns its inducer. It is how the
// cosmetic fall-through re-locates a line after a reformat shifted its number.
func matchLineByContent(blamed map[int]LineInducer, want string) (LineInducer, bool) {
	target := normalizeWhitespace(want)
	// Deterministic order: prefer the lowest line number on a tie so the walk is
	// reproducible across runs.
	best := -1
	var found LineInducer
	for n, li := range blamed {
		if normalizeWhitespace(li.Text) != target {
			continue
		}
		if best == -1 || n < best {
			best = n
			found = li
		}
	}
	if best == -1 {
		return LineInducer{}, false
	}
	return found, true
}

// normalizeWhitespace strips every whitespace character, so two lines that differ
// only in indentation, spacing, or trailing whitespace normalize equal. This is
// the operational definition of "cosmetic/format-only" (spec §5.2): a change that
// leaves the non-whitespace content untouched.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
	}), "")
}

// postdatesReport reports whether an inducer's commit date is strictly AFTER the
// defect-report date — refinement 2. A change made after the bug was already
// reported cannot have induced it, so such a candidate is filtered.
//
// When no report date is known (haveReport == false) the refinement is INAPPLICABLE
// and returns false: a candidate is never dropped on a report date the run could
// not resolve (that would be a silent could-not-measure masquerading as a filter).
func postdatesReport(inducerDate, reportDate time.Time, haveReport bool) bool {
	if !haveReport {
		return false
	}
	return inducerDate.After(reportDate)
}

// scoreConfidence scores blame agreement for a traced fix — refinement 3. It is a
// RECORDED field, never a gate (spec §5.2). The score rewards agreement: a single
// surviving inducer is unambiguous (1.0); the more distinct inducers the blame
// splits across, the lower the confidence that any one is THE inducer.
//
//   - survivors: the count of DISTINCT inducing commits that survived refinements.
//   - candidates: the count of raw blame candidates considered (every blamed line).
//     A survivors set much smaller than the candidate pool means the refinements
//     did real filtering work, which the score also reflects.
//
// No survivors is a measured-zero confidence (the trace resolved traced-none), not
// a could-not-measure — the instrument ran and agreement is genuinely zero.
func scoreConfidence(candidates, survivors int) Measure[float64] {
	if survivors <= 0 {
		return MeasuredZero[float64]()
	}
	// Agreement: 1 distinct inducer → 1.0; k inducers → 1/k.
	agreement := 1.0 / float64(survivors)
	// Survival: how much of the raw candidate pool the refinements let through.
	// Bounded to [0,1]; when candidates < survivors (deduping collapsed many lines
	// onto few commits) survival is capped at 1.0.
	denom := candidates
	if denom < survivors {
		denom = survivors
	}
	survival := float64(survivors) / float64(denom)
	score := agreement * survival
	if score <= 0 {
		return MeasuredZero[float64]()
	}
	return Measured(score)
}
