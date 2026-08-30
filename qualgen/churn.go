package main

import "time"

// Churn / rework rate (spec §4.2). Churn is the industry's headline
// AI-code-quality signal: a landed line revised or deleted within a short window
// of landing — "premature or low-quality commit" churn that roughly doubled as
// models started writing more code. It is reported as churned-lines / new-lines,
// per author-identity class, computed the published way so the number is
// comparable to public baselines (GitClear's 14-day window; a second anchor,
// Faros AI's rework rate, uses a 14–30-day window — a target may set either).
//
// This is a diff-based approximation, honest about what a miner can see: a
// landed added line (file F, content X) is counted CHURNED when a LATER commit
// within the window, by the SAME author-identity class, deletes or revises a
// line with content X in file F. Cross-commit line identity by (file, content)
// is the standard diff-based churn technique; it cannot follow a line through a
// rename, which is disclosed as a horizon limit of the mine, not silently.

// DefaultChurnWindowDays is GitClear's published churn window (spec §4.2);
// configurable, 14d the comparable default.
const DefaultChurnWindowDays = 14

// ChurnCounts is the churn numerator/denominator for one aggregation key.
type ChurnCounts struct {
	// NewLines is landed added/updated lines — the denominator.
	NewLines int
	// ChurnedLines is the subset revised or deleted within the window by the
	// same author-identity class — the numerator.
	ChurnedLines int
}

// Rate returns churned/new as a three-state measure. A key with landed lines but
// zero churn is measured-zero (the instrument ran, the genuine answer is zero);
// a key with NO landed lines is could-not-measure (there is nothing to rate),
// never a silent or misleading zero (spec §3.2).
func (c ChurnCounts) Rate() Measure[float64] {
	if c.NewLines == 0 {
		return CouldNotMeasure[float64]("no landed lines in this key: rework rate is undefined")
	}
	if c.ChurnedLines == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(c.ChurnedLines) / float64(c.NewLines))
}

// ChurnResult is the churn computation over the whole mined corpus.
type ChurnResult struct {
	// Overall is churn across all identity classes.
	Overall ChurnCounts
	// ByClass is churn partitioned by author-identity class (human / agent /
	// automation / unclassified, or a target's own classes).
	ByClass map[string]*ChurnCounts
}

// landedLine is a still-eligible landed line awaiting a possible churn event.
type landedLine struct {
	file    string
	content string
	when    time.Time
	class   string
	churned bool
}

// computeChurn walks the commits in chronological order and computes churn.
// commits MUST be oldest-first (the order the Store's append-only table records
// them). diffsByCommit maps a commit SHA to its measured file diffs; classOf
// returns a commit's author-identity class; window is the churn window.
//
// For each commit: its DELETIONS are matched against prior landed lines (same
// file, same content, same identity class, within the window) — the first such
// unmatched landed line is marked churned — and THEN its own added lines are
// registered as landed. Processing deletions before this commit's own adds means
// a commit can never churn a line it introduced in the same commit.
func computeChurn(commits []Commit, diffsByCommit map[string][]FileDiff, classOf func(Commit) string, window time.Duration) ChurnResult {
	res := ChurnResult{ByClass: map[string]*ChurnCounts{}}
	// landed indexed by (file, content) → the eligible landed lines, oldest
	// first, so a deletion consumes the oldest matching landed line.
	landed := map[string][]*landedLine{}
	key := func(file, content string) string { return file + "\x00" + content }

	counts := func(class string) *ChurnCounts {
		c := res.ByClass[class]
		if c == nil {
			c = &ChurnCounts{}
			res.ByClass[class] = c
		}
		return c
	}

	for _, com := range commits {
		class := classOf(com)
		when := com.AuthorWhen

		// 1) Deletions: a revised/deleted line is a `del` op. Match each against
		//    the oldest eligible landed line of the same identity class within
		//    the window.
		for _, fd := range diffsByCommit[com.SHA] {
			if fd.Lines.State != StateMeasured {
				continue
			}
			for _, hunk := range fd.Lines.Value {
				for _, lc := range hunk.Lines {
					if lc.Op != OpDel || !matchable(lc.Content) {
						continue
					}
					file := fd.OldPath
					if file == "" {
						file = fd.NewPath
					}
					bucket := landed[key(file, lc.Content)]
					for _, ll := range bucket {
						if ll.churned || ll.class != class {
							continue
						}
						if when.Sub(ll.when) < 0 || when.Sub(ll.when) > window {
							continue
						}
						ll.churned = true
						res.Overall.ChurnedLines++
						counts(class).ChurnedLines++
						break
					}
				}
			}
		}

		// 2) Register this commit's added/updated lines as landed new-lines.
		for _, fd := range diffsByCommit[com.SHA] {
			if fd.Lines.State != StateMeasured {
				continue
			}
			file := fd.NewPath
			if file == "" {
				file = fd.OldPath
			}
			for _, hunk := range fd.Lines.Value {
				for _, lc := range hunk.Lines {
					if lc.Op != OpAdd || !matchable(lc.Content) {
						continue
					}
					res.Overall.NewLines++
					counts(class).NewLines++
					ll := &landedLine{file: file, content: lc.Content, when: when, class: class}
					landed[key(file, lc.Content)] = append(landed[key(file, lc.Content)], ll)
				}
			}
		}
	}
	return res
}
