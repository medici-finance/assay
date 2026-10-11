package main

// transcribeverdict_postcondition.go — the EMITS-AND-ASSERTS seam of
// --transcribe-verdict (write-boundary self-proof).
//
// applyVerdictDelta returning nil is not proof the artifact it owed landed: a
// write that silently drops a group, an append that lands outside the section the
// board reads, or a README flip the board parser does not see would all exit
// clean. A transcribe step that exits 0 while the Evidence/flip it owed never
// lands is the exact shape this seam kills. So after a successful apply the run
// RE-READS what it wrote and asserts, before it reports success:
//
//  1. non-empty: a delta that owed writes touched exactly the files it owed (one
//     per distinct Evidence path, one per flip) — never zero;
//  2. each Evidence line is present, as a whole line, inside its brief's
//     `## Evidence` section;
//  3. wired to its consumer: statusgen's OWN board loader, run over the written
//     tree, reads every flipped brief at status `verified` carrying the stamp the
//     lane wrote.
//
// A mismatch is a non-zero exit with a POSTCONDITION FAILED line, never exit 0.
// This tool never mutates a GitHub issue (see runTranscribeVerdict), so filing a
// repair issue on that exit belongs to the caller's transport, exactly as the
// flood (exit 3) and refusal paths already do.
//
// The seam ADDS an assertion after the write. It is reached only on the armed,
// non-dry-run path, after the R-6 enactment gate and every trust predicate have
// already run unchanged — it relaxes none of them.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// applyVerdictDeltaFn is the apply step runTranscribeVerdict calls. Production:
// applyVerdictDelta. A test substitutes an apply that reports success without
// landing the artifact, which is the shape the postcondition must catch.
var applyVerdictDeltaFn = applyVerdictDelta

// assertVerdictApplied is the postcondition. touched is what the apply reported.
func assertVerdictApplied(root string, d verdictDelta, touched int) error {
	paths := map[string]bool{}
	for _, a := range d.Appends {
		paths[a.Path] = true
	}
	want := len(paths) + len(d.Flips)
	if want > 0 && touched != want {
		return fmt.Errorf("the apply reported %d file(s) touched, but the delta owed %d (%d Evidence file(s), %d flip(s))",
			touched, want, len(paths), len(d.Flips))
	}

	for _, a := range d.Appends {
		b, err := os.ReadFile(a.Path)
		if err != nil {
			return fmt.Errorf("%s: cannot re-read the brief after the apply: %w", a.Brief, err)
		}
		if !evidenceSectionHasLine(string(b), a.Line) {
			return fmt.Errorf("%s row %d: the Evidence line the lane owed is not in the brief's `## Evidence` section after the apply",
				a.Brief, a.Row)
		}
	}

	if len(d.Flips) == 0 {
		return nil
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		return fmt.Errorf("the board no longer loads after the apply: %w", err)
	}
	for _, f := range d.Flips {
		b, ok := findBriefByReadme(streams, f.ReadmePath, f.Num)
		if !ok {
			return fmt.Errorf("%s: the board loader finds no row #%s in %s after the apply", f.Brief, f.Num, f.ReadmePath)
		}
		if b.Status != "verified" {
			return fmt.Errorf("%s: the board loader reads status %q after the apply, not verified", f.Brief, b.Status)
		}
		if strings.TrimSpace(b.Verified) != strings.TrimSpace(f.Stamp) {
			return fmt.Errorf("%s: the board loader reads Verified %q after the apply, not the stamp the lane wrote (%q)",
				f.Brief, b.Verified, f.Stamp)
		}
	}
	return nil
}

// evidenceSectionHasLine reports whether line appears, on whole-line boundaries,
// inside raw's `## Evidence` section (from that heading to the next `## `
// heading). line may span several lines; it must appear contiguously.
func evidenceSectionHasLine(raw, line string) bool {
	want := strings.TrimRight(line, "\n")
	var section []string
	in := false
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if !in {
			in = t == "## Evidence"
			continue
		}
		if strings.HasPrefix(t, "## ") {
			break
		}
		section = append(section, l)
	}
	if !in || want == "" {
		return false
	}
	body := "\n" + strings.Join(section, "\n") + "\n"
	return strings.Contains(body, "\n"+want+"\n")
}

// findBriefByReadme locates brief num in the stream whose README is readmePath.
func findBriefByReadme(streams []*Stream, readmePath, num string) (Brief, bool) {
	target := filepath.Clean(readmePath)
	for _, s := range streams {
		if filepath.Clean(filepath.Join(s.Dir, "README.md")) != target {
			continue
		}
		for _, b := range s.Briefs {
			if b.Num == num {
				return b, true
			}
		}
	}
	return Brief{}, false
}
