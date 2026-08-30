package main

import "testing"

// --- fixture helpers: build FileDiff records directly (no git), so the
// classifier is dereferenced against exact, adversarial line shapes. ---

func addLine(s string) LineChange { return LineChange{Op: OpAdd, Content: s} }
func delLine(s string) LineChange { return LineChange{Op: OpDel, Content: s} }
func ctxLine(s string) LineChange { return LineChange{Op: OpContext, Content: s} }

// measuredDiff builds a measured FileDiff for one file from an ordered line set.
func measuredDiff(sha, path string, lines ...LineChange) FileDiff {
	return FileDiff{
		CommitSHA: sha,
		OldPath:   path,
		NewPath:   path,
		Kind:      ChangeModified,
		Lines:     Measured([]Hunk{{Lines: lines}}),
	}
}

// classOf returns the class assigned to the i-th ADD line of the first file, in
// order (deleted/context lines are skipped), for compact assertions.
func addClasses(fd FileDiff) []string {
	var out []string
	for _, h := range fd.Lines.Value {
		for _, lc := range h.Lines {
			if lc.Op == OpAdd {
				out = append(out, lc.Class)
			}
		}
	}
	return out
}

// TestTaxonomyMovedVsCopied is Verify row 2: a relocated block classifies
// `moved` (its source lines were DELETED — source gone) and a duplicated block
// classifies `copied` (its source lines REMAIN as context) — the two are never
// conflated.
func TestTaxonomyMovedVsCopied(t *testing.T) {
	block := []string{
		"func alpha(x int) int {",
		"    y := x * 2",
		"    z := y + 1",
		"    return z",
	}

	t.Run("relocated block (source deleted) is moved", func(t *testing.T) {
		// The same 4-line block is added here and deleted elsewhere in the
		// commit: a relocation. No copy remains.
		fd := measuredDiff("c1", "a.go",
			delLine(block[0]), delLine(block[1]), delLine(block[2]), delLine(block[3]),
			ctxLine("// unrelated context"),
			addLine(block[0]), addLine(block[1]), addLine(block[2]), addLine(block[3]),
		)
		ct := classifyCommit("c1", []FileDiff{fd}, DefaultBlockMin)
		if ct.MovedBlocks != 1 || ct.CopiedBlocks != 0 {
			t.Fatalf("expected 1 moved / 0 copied block, got moved=%d copied=%d", ct.MovedBlocks, ct.CopiedBlocks)
		}
		for i, c := range addClasses(fd) {
			if c != string(ClassMoved) {
				t.Fatalf("added line %d: got class %q, want moved", i, c)
			}
		}
	})

	t.Run("duplicated block (source remains) is copied", func(t *testing.T) {
		// The 4-line block appears as context (it REMAINS) and is also added:
		// a duplication. Nothing was deleted.
		fd := measuredDiff("c2", "b.go",
			ctxLine(block[0]), ctxLine(block[1]), ctxLine(block[2]), ctxLine(block[3]),
			ctxLine("// separator"),
			addLine(block[0]), addLine(block[1]), addLine(block[2]), addLine(block[3]),
		)
		ct := classifyCommit("c2", []FileDiff{fd}, DefaultBlockMin)
		if ct.CopiedBlocks != 1 || ct.MovedBlocks != 0 {
			t.Fatalf("expected 1 copied / 0 moved block, got moved=%d copied=%d", ct.MovedBlocks, ct.CopiedBlocks)
		}
		for i, c := range addClasses(fd) {
			if c != string(ClassCopied) {
				t.Fatalf("added line %d: got class %q, want copied", i, c)
			}
		}
	})
}

// TestBlockMatchThreshold is Verify row 3: a 3-line identical run is NOT a block
// move/copy at the default N=4, a 4-line run IS — and the threshold is honored
// as configurable (N=3 promotes the 3-line run to a block).
func TestBlockMatchThreshold(t *testing.T) {
	three := []string{"line one distinct", "line two distinct", "line three distinct"}
	four := append(append([]string{}, three...), "line four distinct")

	// 3-line duplicated run, default N=4 → below threshold, not a block.
	fd3 := measuredDiff("c3", "f.go",
		ctxLine(three[0]), ctxLine(three[1]), ctxLine(three[2]),
		ctxLine("// gap"),
		addLine(three[0]), addLine(three[1]), addLine(three[2]),
	)
	ct3 := classifyCommit("c3", []FileDiff{fd3}, DefaultBlockMin)
	if ct3.CopiedBlocks != 0 || ct3.MovedBlocks != 0 {
		t.Fatalf("3-line run at N=4 must NOT be a block, got moved=%d copied=%d", ct3.MovedBlocks, ct3.CopiedBlocks)
	}

	// 4-line duplicated run, default N=4 → a block.
	fd4 := measuredDiff("c4", "g.go",
		ctxLine(four[0]), ctxLine(four[1]), ctxLine(four[2]), ctxLine(four[3]),
		ctxLine("// gap"),
		addLine(four[0]), addLine(four[1]), addLine(four[2]), addLine(four[3]),
	)
	ct4 := classifyCommit("c4", []FileDiff{fd4}, DefaultBlockMin)
	if ct4.CopiedBlocks != 1 {
		t.Fatalf("4-line run at N=4 must be 1 copied block, got moved=%d copied=%d", ct4.MovedBlocks, ct4.CopiedBlocks)
	}

	// The threshold is configurable: lower it to 3 and the 3-line run becomes a
	// block — proving N is honored, not hard-coded.
	ct3b := classifyCommit("c3", []FileDiff{measuredDiff("c3", "f.go",
		ctxLine(three[0]), ctxLine(three[1]), ctxLine(three[2]),
		ctxLine("// gap"),
		addLine(three[0]), addLine(three[1]), addLine(three[2]),
	)}, 3)
	if ct3b.CopiedBlocks != 1 {
		t.Fatalf("3-line run at N=3 must be 1 copied block, got moved=%d copied=%d", ct3b.MovedBlocks, ct3b.CopiedBlocks)
	}
}
