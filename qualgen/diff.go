package main

import (
	"strings"

	fdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
)

// FileDiff is the per-(commit, file) diff record — the diff table. One FileDiff
// marshals to one line in diffs.jsonl. Part of the frozen interface contract
// (contract item 3); later briefs read it, they do not redefine it.
//
// The three-state instrument invariant (spec §3.2) lives on the Lines field:
//   - Measured   → the file was line-diffed; Lines carries the ordered hunks.
//   - MeasuredZero → a text file that changed by zero lines (e.g. a mode-only
//     or rename-only change). A genuine zero, distinct from could-not-measure.
//   - CouldNotMeasure → a binary or otherwise unreadable blob that cannot be
//     line-diffed; Lines carries a reason and NO line data.
type FileDiff struct {
	CommitSHA string     `json:"commit_sha"`
	OldPath   string     `json:"old_path"`
	NewPath   string     `json:"new_path"`
	Kind      ChangeKind `json:"kind"`
	Binary    bool       `json:"binary"`

	// Lines is the three-state line record. Its Value (when Measured) is the
	// ordered hunk list; a binary/unreadable blob is CouldNotMeasure with a
	// reason; a zero-line change is MeasuredZero.
	Lines Measure[[]Hunk] `json:"lines"`
}

// ChangeKind is how the file entry changed between the parent tree and this
// commit's tree.
type ChangeKind string

const (
	ChangeAdded    ChangeKind = "added"
	ChangeDeleted  ChangeKind = "deleted"
	ChangeModified ChangeKind = "modified"
)

// Hunk is one contiguous block of the diff. The skeleton emits a single hunk
// spanning the whole file patch (start = 1, counts = totals); later briefs MAY
// refine hunk boundaries. The ordered LineChange list is the load-bearing part.
type Hunk struct {
	OldStart int          `json:"old_start"`
	OldLines int          `json:"old_lines"`
	NewStart int          `json:"new_start"`
	NewLines int          `json:"new_lines"`
	Lines    []LineChange `json:"lines"`
}

// LineChange is one changed or context line. Op is the raw git operation; Class
// is an EMPTY slot the M1 taxonomy brief (quality/02) fills with
// added/updated/moved/copied/churned — this skeleton never sets it.
type LineChange struct {
	Op      LineOp `json:"op"`
	Content string `json:"content"`
	Class   string `json:"class,omitempty"`
}

// LineOp is the raw per-line git operation.
type LineOp string

const (
	OpAdd     LineOp = "add"
	OpDel     LineOp = "del"
	OpContext LineOp = "context"
)

// Key is the stable reference a Commit uses to point at this FileDiff record. It
// is unique per (commit, resulting path); a delete keys on the old path.
func (fd FileDiff) Key() string {
	p := fd.NewPath
	if p == "" {
		p = fd.OldPath
	}
	return fd.CommitSHA + ":" + p
}

// fileDiffFromPatch builds a FileDiff from a go-git file patch, applying the
// three-state rule. It is written against the format/diff interfaces (not
// concrete go-git types) so it is unit-testable with fakes — that is how
// TestDiffThreeStateDistinguishesZeroFromUnmeasured drives the zero-vs-unmeasured
// boundary deterministically without a git binary.
func fileDiffFromPatch(commitSHA string, fp fdiff.FilePatch) FileDiff {
	from, to := fp.Files()
	fd := FileDiff{CommitSHA: commitSHA}
	if from != nil {
		fd.OldPath = from.Path()
	}
	if to != nil {
		fd.NewPath = to.Path()
	}
	fd.Kind = classifyKind(from, to)

	// A binary (or unreadable) blob cannot be line-diffed: could-not-measure,
	// never a silent zero.
	if fp.IsBinary() {
		fd.Binary = true
		fd.Lines = CouldNotMeasure[[]Hunk]("binary or unreadable blob: not line-diffable")
		return fd
	}

	lines, changed := chunksToLines(fp.Chunks())

	// A text file that changed by zero lines (mode-only / rename-only) is a
	// genuine measured-zero — distinct from the binary could-not-measure above.
	if changed == 0 {
		fd.Lines = MeasuredZero[[]Hunk]()
		return fd
	}

	oldLines, newLines := 0, 0
	for _, lc := range lines {
		switch lc.Op {
		case OpContext:
			oldLines++
			newLines++
		case OpDel:
			oldLines++
		case OpAdd:
			newLines++
		}
	}
	fd.Lines = Measured([]Hunk{{
		OldStart: 1,
		OldLines: oldLines,
		NewStart: 1,
		NewLines: newLines,
		Lines:    lines,
	}})
	return fd
}

// classifyKind derives the change kind from the presence of the from/to files.
// Rename detection is deliberately out of scope for the skeleton (go-git's tree
// diff does not detect renames by default); a rename surfaces as a delete + add
// pair, which is honest and refinable by a later brief.
func classifyKind(from, to fdiff.File) ChangeKind {
	switch {
	case from == nil && to != nil:
		return ChangeAdded
	case from != nil && to == nil:
		return ChangeDeleted
	default:
		return ChangeModified
	}
}

// chunksToLines flattens go-git's chunk list into the ordered LineChange list,
// splitting multi-line chunk content into one LineChange per line. It returns
// the lines and the count of ADD/DEL (i.e. genuinely changed) lines, which the
// caller uses to decide measured vs measured-zero.
func chunksToLines(chunks []fdiff.Chunk) ([]LineChange, int) {
	var out []LineChange
	changed := 0
	for _, ch := range chunks {
		op := mapOp(ch.Type())
		content := ch.Content()
		// A chunk's content may hold several newline-separated lines; emit one
		// LineChange each. A trailing empty segment (content ended in "\n") is
		// not a real line and is dropped.
		segs := strings.Split(content, "\n")
		if len(segs) > 0 && segs[len(segs)-1] == "" {
			segs = segs[:len(segs)-1]
		}
		for _, s := range segs {
			out = append(out, LineChange{Op: op, Content: s})
			if op == OpAdd || op == OpDel {
				changed++
			}
		}
	}
	return out, changed
}

// mapOp translates a go-git diff operation to the qualgen LineOp.
func mapOp(t fdiff.Operation) LineOp {
	switch t {
	case fdiff.Add:
		return OpAdd
	case fdiff.Delete:
		return OpDel
	default:
		return OpContext
	}
}
