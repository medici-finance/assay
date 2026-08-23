package main

import (
	"regexp"
	"strings"
)

// prlink is the tree-side half of the PR→brief link (derived-board/02). deskpr refuses
// to OPEN a PR whose body lacks a `Brief:` trailer; this package classifies what the
// tree already has, so the derivation (brief 03) reads a verdict instead of guessing
// from branch names. Pure: it takes records, returns verdicts, touches nothing.
//
// The fetch half — turning GitHub's PR list into []PRLinkRecord — belongs to brief 03.

// PRLinkRecord is one open-or-merged PR's link-relevant fields.
type PRLinkRecord struct {
	Number    int
	Body      string
	MergedSHA string
}

// PRLinkVerdict is the classification of one PR's link to its brief(s).
type PRLinkVerdict string

const (
	// PRLinkLinked — the body carries exactly one Brief: trailer.
	PRLinkLinked PRLinkVerdict = "linked"
	// PRLinkUnlinked — the body carries no Brief: trailer.
	PRLinkUnlinked PRLinkVerdict = "unlinked"
	// PRLinkMultiLinked — the body carries more than one Brief: trailer.
	PRLinkMultiLinked PRLinkVerdict = "multi-linked"
)

var (
	rePRLinkBrief = regexp.MustCompile(`^[ \t]*Brief:[ \t]*\S`)
	rePRLinkFence = regexp.MustCompile("^[ \t]*```")
)

// ClassifyPRLink maps each PR number to its link verdict, counting Brief: trailer
// lines outside fenced code blocks (a trailer inside a code sample is documentation,
// not a link — same grammar as deskkit.ParseTrailers).
func ClassifyPRLink(records []PRLinkRecord) map[int]PRLinkVerdict {
	out := make(map[int]PRLinkVerdict, len(records))
	for _, r := range records {
		n := 0
		inFence := false
		for _, raw := range strings.Split(r.Body, "\n") {
			if rePRLinkFence.MatchString(raw) {
				inFence = !inFence
				continue
			}
			if !inFence && rePRLinkBrief.MatchString(raw) {
				n++
			}
		}
		switch {
		case n == 0:
			out[r.Number] = PRLinkUnlinked
		case n == 1:
			out[r.Number] = PRLinkLinked
		default:
			out[r.Number] = PRLinkMultiLinked
		}
	}
	return out
}
