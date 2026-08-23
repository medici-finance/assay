package deskkit

import (
	"fmt"
	"regexp"
	"strings"
)

// trailer is the PR-body link grammar (example-stream/02). A PR body must carry
// exactly one link trailer — `Brief: <stream>/<NN>` (or the colon form) that names
// the brief the PR delivers, or `Issue: #<N>` for issue-only work — because the
// derived board needs a data edge from the PR to the brief, and a derivation built
// on branch names and body prose is a guess. This is the shared parse; deskpr owns
// the refusal (exit 5) and the file resolution under --root.

// TrailerKind is which link kind a trailer line carries.
type TrailerKind string

const (
	// TrailerBrief is `Brief: ...` — the PR delivers a brief.
	TrailerBrief TrailerKind = "brief"
	// TrailerIssue is `Issue: #<N>` — issue-only work, no brief.
	TrailerIssue TrailerKind = "issue"
)

// Trailer is one parsed trailer line. Line is the 1-based line number in the body.
type Trailer struct {
	Kind  TrailerKind
	Value string // trimmed value after the colon; for Issue, the bare number ("123")
	Line  int
}

var (
	reTrailerBrief = regexp.MustCompile(`^[ \t]*Brief:[ \t]*(\S.*?)[ \t]*$`)
	reTrailerIssue = regexp.MustCompile(`^[ \t]*Issue:[ \t]*#?[ \t]*([0-9]+)[ \t]*$`)
	reFenceOpen    = regexp.MustCompile("^[ \t]*```")
)

// ErrTrailerDuplicate reports a second Brief: (or second Issue:) trailer.
type ErrTrailerDuplicate struct {
	Kind      TrailerKind
	FirstLine int
	Line      int
}

func (e *ErrTrailerDuplicate) Error() string {
	return fmt.Sprintf("duplicate %s: trailer (first at line %d, again at line %d) — exactly one link per PR body",
		e.Kind, e.FirstLine, e.Line)
}

// ErrTrailerBoth reports a body carrying both a Brief: and an Issue: trailer.
type ErrTrailerBoth struct {
	BriefLine int
	IssueLine int
}

func (e *ErrTrailerBoth) Error() string {
	return fmt.Sprintf("both Brief: (line %d) and Issue: (line %d) trailers present — exactly one link per PR body",
		e.BriefLine, e.IssueLine)
}

// ParseTrailers returns the link trailers in a PR body. Lines inside fenced code
// blocks (```) are ignored — a trailer inside a code sample is documentation, not
// a link. Exactly one link may exist: a second Brief: or Issue: line, or one of
// each, is a multiplicity error (the caller refuses). A body with no trailer is
// NOT an error here — the caller decides what absence means (deskpr refuses it).
func ParseTrailers(body []byte) ([]Trailer, error) {
	var trs []Trailer
	var firstBrief, firstIssue int
	inFence := false
	for i, raw := range strings.Split(string(body), "\n") {
		line := i + 1
		if reFenceOpen.MatchString(raw) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case reTrailerBrief.MatchString(raw):
			v := strings.TrimSpace(reTrailerBrief.FindStringSubmatch(raw)[1])
			if firstBrief != 0 {
				return nil, &ErrTrailerDuplicate{Kind: TrailerBrief, FirstLine: firstBrief, Line: line}
			}
			firstBrief = line
			trs = append(trs, Trailer{Kind: TrailerBrief, Value: v, Line: line})
		case reTrailerIssue.MatchString(raw):
			v := reTrailerIssue.FindStringSubmatch(raw)[1]
			if firstIssue != 0 {
				return nil, &ErrTrailerDuplicate{Kind: TrailerIssue, FirstLine: firstIssue, Line: line}
			}
			firstIssue = line
			trs = append(trs, Trailer{Kind: TrailerIssue, Value: v, Line: line})
		}
	}
	if firstBrief != 0 && firstIssue != 0 {
		return nil, &ErrTrailerBoth{BriefLine: firstBrief, IssueLine: firstIssue}
	}
	return trs, nil
}
