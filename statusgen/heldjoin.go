package main

import (
	"regexp"
	"sort"
	"strings"
)

// A HELD or could-not-check marker that an HTML comment splits ("HE<!-- -->LD")
// renders whole, but every line read of the held scan sees only its pieces:
// the inline masker blanks the comment in place, and the flip read's witness
// rows strip it once. These helpers give every read of unroutedHeldLine (the
// verify-gate card, the model autoflip, close-verify and the flip read) one
// normalisation: the whole Evidence is stripped of comments until it stops
// changing, each kept byte remembering its source offset, so a marker a strip
// forms is found and mapped back to the source lines it spans.

// heldRestripCap bounds the strip loop. Evidence whose comments still change
// after this many strips cannot be mapped and is refused (fail closed).
const heldRestripCap = 8

// heldCommentRe matches one complete HTML comment as CommonMark renders it,
// the short forms "<!-->" and "<!--->" included.
var heldCommentRe = regexp.MustCompile(`(?s)<!--(?:>|->|.*?-->)`)

// heldCommentJoins strips complete comments from evidence until the text
// stops changing. It returns the stripped text, the source offset of each of
// its bytes, and whether the text settled within heldRestripCap strips.
func heldCommentJoins(evidence string) (string, []int, bool) {
	text := evidence
	src := make([]int, len(text))
	for i := range src {
		src[i] = i
	}
	for n := 0; n < heldRestripCap; n++ {
		locs := heldCommentRe.FindAllStringIndex(text, -1)
		if locs == nil {
			return text, src, true
		}
		var b strings.Builder
		next := make([]int, 0, len(src))
		at := 0
		for _, l := range locs {
			b.WriteString(text[at:l[0]])
			next = append(next, src[at:l[0]]...)
			at = l[1]
		}
		b.WriteString(text[at:])
		next = append(next, src[at:]...)
		text, src = b.String(), next
	}
	return text, src, !heldCommentRe.MatchString(text)
}

// heldJoinedLines returns the source lines (0-based, as strings.Split(evidence,
// "\n") numbers them) of every match in locs, offsets into the stripped text,
// that a strip formed: a match whose bytes are not contiguous in the source.
// A match already whole in the source is the line scan's to judge.
func heldJoinedLines(evidence string, src []int, locs [][]int) map[int]bool {
	var starts []int
	for i := 0; i < len(evidence); i++ {
		if evidence[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	lineOf := func(off int) int { return sort.SearchInts(starts, off+1) }
	out := map[int]bool{}
	for _, m := range locs {
		joined := false
		for p := m[0] + 1; p < m[1]; p++ {
			if src[p] != src[p-1]+1 {
				joined = true
				break
			}
		}
		if !joined {
			continue
		}
		for l := lineOf(src[m[0]]); l <= lineOf(src[m[1]-1]); l++ {
			out[l] = true
		}
	}
	return out
}
