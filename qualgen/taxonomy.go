package main

import "strings"

// LineClass is the M1 line-operation taxonomy (spec §4.1). Each changed line a
// commit introduces is classified as exactly one of these. Detection follows
// GitClear's published methodology so the derived headline numbers
// (copy/paste ratio, duplicate-block rate) are comparable to their public
// baselines.
//
// A line the quality/01 miner recorded as could-not-measure (a binary or
// otherwise unreadable blob, per FileDiff.Lines' three-state) is NEVER assigned
// a class here: it propagates as could-not-measure and is counted in no
// category (never rounded down to a silent zero, spec §3.2).
type LineClass string

const (
	// ClassAdded — a net-new line: added in a hunk that carried no deletions
	// (or in a wholly new file), matching no relocated or duplicated block.
	ClassAdded LineClass = "added"
	// ClassDeleted — a removed line that is not the source of a detected move.
	ClassDeleted LineClass = "deleted"
	// ClassUpdated — a line added in a hunk that also deleted lines: an
	// in-place modification, not net-new content.
	ClassUpdated LineClass = "updated"
	// ClassMoved — a line inside a block (>= blockMin lines) whose identical
	// run was DELETED elsewhere in the same commit: relocated, source gone.
	ClassMoved LineClass = "moved"
	// ClassCopied — a line inside a block (>= blockMin lines) whose identical
	// run REMAINS (matched a context line that was not deleted): duplicated.
	ClassCopied LineClass = "copied"
	// ClassChurned — an earlier-landed line revised or deleted by the same
	// author-identity class within the churn window. Assigned by churn.go
	// across commits, not by the single-commit classifier here (spec §4.2).
	ClassChurned LineClass = "churned"
)

// DefaultBlockMin is GitClear's published duplicate-block granularity: a run of
// fewer than this many identical lines is NOT a block move/copy (spec §4.1).
const DefaultBlockMin = 4

// CommitTaxonomy is the single-commit line-operation summary the M1 aggregator
// rolls up. Block counts are the load-bearing pair: the headline copy/paste
// ratio is computed over BLOCKS (copied / (moved + copied)), not lines.
type CommitTaxonomy struct {
	SHA string

	// LineClasses counts classified lines by category (could-not-measure lines
	// are excluded — they appear only in CouldNotMeasureLines).
	LineClasses map[LineClass]int

	// MovedBlocks / CopiedBlocks count DETECTED blocks (>= blockMin lines), the
	// grain the copy/paste ratio and duplicate-block rate are defined over.
	MovedBlocks  int
	CopiedBlocks int

	// CouldNotMeasureLines counts lines the miner could not read (binary /
	// unreadable blobs), reported as itself, never merged into a category.
	CouldNotMeasureLines int
}

// classifyCommit classifies every changed line across a commit's file diffs into
// the six-category taxonomy and returns the per-commit summary. It MUTATES the
// LineChange.Class slots of the passed diffs (quality/01 left that field empty
// for exactly this brief to fill), so an artifact writer downstream can persist
// the per-line classification, and returns the block/line counts the aggregator
// needs.
//
// Move-vs-copy is decided at BLOCK granularity across the whole commit: a run of
// >= blockMin added lines whose identical sequence was DELETED (in any file of
// this commit) is `moved` (source gone); a run whose identical sequence appears
// among the REMAINING context lines is `copied` (source remains). A run shorter
// than blockMin is not a block and its lines fall back to added/updated. Move
// takes precedence over copy for an equally-long match (a relocation is not a
// duplication).
//
// The commit total is the sum of the per-file taxonomies classifyCommitByFile
// produces; this thin wrapper exists for callers (and tests) that want the
// whole-commit roll-up without the per-file breakdown.
func classifyCommit(sha string, diffs []FileDiff, blockMin int) CommitTaxonomy {
	total := CommitTaxonomy{SHA: sha, LineClasses: map[LineClass]int{}}
	for _, ft := range classifyCommitByFile(sha, diffs, blockMin) {
		mergeTaxonomy(&total, *ft)
	}
	return total
}

// classifyCommitByFile is classifyCommit's per-file core: it classifies the
// commit's changed lines exactly as classifyCommit does (same commit-wide block
// detection, same LineChange.Class mutation) but attributes each classified line
// and detected block to the FILE whose added lines carry it, keyed by the file's
// path. Summing the returned taxonomies reproduces the whole-commit total.
//
// Per-file (not whole-commit) attribution is what lets the aggregator's
// per-package grain be precise: a commit spanning packages A and B credits a
// copied/moved block only to the package that gained its ADDED lines, never to
// every package the commit happened to touch. Block detection stays commit-wide
// (a block moved from B to A matches B's deletions against A's adds), so the
// destination package A — where the added lines land — is the correct owner;
// B's package sees only the (uncounted) deletion.
func classifyCommitByFile(sha string, diffs []FileDiff, blockMin int) map[string]*CommitTaxonomy {
	if blockMin < 1 {
		blockMin = DefaultBlockMin
	}
	byFile := map[string]*CommitTaxonomy{}
	taxOf := func(p string) *CommitTaxonomy {
		ct := byFile[p]
		if ct == nil {
			ct = &CommitTaxonomy{SHA: sha, LineClasses: map[LineClass]int{}}
			byFile[p] = ct
		}
		return ct
	}
	diffPath := func(fd *FileDiff) string {
		if fd.NewPath != "" {
			return fd.NewPath
		}
		return fd.OldPath
	}

	// Source pools for block matching, gathered across every measured file diff
	// in the commit: deleted-line contents (a match here means the source is
	// GONE → moved) and context-line contents (a match here means the source
	// REMAINS → copied).
	var deletedPool, contextPool []string
	// addedFiles keeps, per file, its path, the ordered pointers to its added
	// LineChanges, and whether that file also deleted lines (drives
	// added-vs-updated).
	type addedFile struct {
		path         string
		lines        []*LineChange
		hasDeletions bool
	}
	var addedFiles []addedFile

	for i := range diffs {
		fd := &diffs[i]
		p := diffPath(fd)
		if fd.Lines.State != StateMeasured {
			// Binary / unreadable / measured-zero: no per-line taxonomy. A
			// could-not-measure diff contributes to that count; a measured-zero
			// (mode/rename-only) simply has no changed lines to classify.
			if fd.Lines.State == StateCouldNotMeasure {
				taxOf(p).CouldNotMeasureLines++
			}
			continue
		}
		// Register the file so a measured-but-unclassifiable file (e.g. only
		// deletions) still appears as its own — measured-zero — grain rather
		// than being folded into another package.
		taxOf(p)
		af := addedFile{path: p}
		for hi := range fd.Lines.Value {
			hunk := &fd.Lines.Value[hi]
			for li := range hunk.Lines {
				lc := &hunk.Lines[li]
				switch lc.Op {
				case OpDel:
					af.hasDeletions = true
					lc.Class = string(ClassDeleted)
					if matchable(lc.Content) {
						deletedPool = append(deletedPool, lc.Content)
					}
				case OpContext:
					if matchable(lc.Content) {
						contextPool = append(contextPool, lc.Content)
					}
				case OpAdd:
					af.lines = append(af.lines, lc)
				}
			}
		}
		addedFiles = append(addedFiles, af)
	}

	// Classify each file's added lines. Block detection scans the file's added
	// sequence greedily: at each unclaimed index, take the longest contiguous
	// run that matches a run in the deleted pool (moved) or the context pool
	// (copied); if that run is >= blockMin, claim it as one block and advance.
	for _, af := range addedFiles {
		ct := taxOf(af.path)
		content := make([]string, len(af.lines))
		for i, lc := range af.lines {
			content[i] = lc.Content
		}
		i := 0
		for i < len(af.lines) {
			// A blank / trivial line never starts a block (a run of blank lines
			// is not a duplicated code block).
			if !matchable(content[i]) {
				classifySingle(af.lines[i], af.hasDeletions, ct.LineClasses)
				i++
				continue
			}
			moved := longestPrefixMatch(content[i:], deletedPool)
			copied := longestPrefixMatch(content[i:], contextPool)
			switch {
			case moved >= blockMin && moved >= copied:
				for k := 0; k < moved; k++ {
					af.lines[i+k].Class = string(ClassMoved)
					ct.LineClasses[ClassMoved]++
				}
				ct.MovedBlocks++
				i += moved
			case copied >= blockMin:
				for k := 0; k < copied; k++ {
					af.lines[i+k].Class = string(ClassCopied)
					ct.LineClasses[ClassCopied]++
				}
				ct.CopiedBlocks++
				i += copied
			default:
				classifySingle(af.lines[i], af.hasDeletions, ct.LineClasses)
				i++
			}
		}
	}
	return byFile
}

// classifySingle assigns a single added line that is not part of any detected
// block: `updated` when its file also deleted lines (an in-place modification),
// `added` otherwise (net-new content).
func classifySingle(lc *LineChange, fileHasDeletions bool, counts map[LineClass]int) {
	cls := ClassAdded
	if fileHasDeletions {
		cls = ClassUpdated
	}
	lc.Class = string(cls)
	counts[cls]++
}

// longestPrefixMatch returns the length of the longest prefix of target that
// occurs as a CONTIGUOUS run anywhere in source. The run is bounded by a
// non-matchable (blank/trivial) line so that whitespace can never pad a block
// to the threshold.
func longestPrefixMatch(target, source []string) int {
	best := 0
	for j := 0; j < len(source); j++ {
		k := 0
		for j+k < len(source) && k < len(target) &&
			source[j+k] == target[k] && matchable(target[k]) {
			k++
		}
		if k > best {
			best = k
		}
	}
	return best
}

// matchable reports whether a line may participate in block matching. A blank or
// brace-only line is structurally trivial and excluded, so a run of blank lines
// or lone braces cannot be mistaken for a duplicated code block.
func matchable(content string) bool {
	t := strings.TrimSpace(content)
	switch t {
	case "", "{", "}", "(", ")", "[", "]":
		return false
	}
	return true
}
