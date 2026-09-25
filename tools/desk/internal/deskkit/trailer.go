package deskkit

import (
	"fmt"
	"regexp"
	"strings"
)

// trailer is the PR-body link grammar (example-stream/02). A PR body must carry
// exactly one link trailer — `Brief: <stream>/<NN>` (or the colon form) that names
// the brief the PR delivers, `Authors: <stream>/<NN>[, …]` naming the brief(s) a
// briefs-AUTHORING PR writes, or `Issue: #<N>` for issue-only work — because the
// derived board needs a data edge from the PR to the brief, and a derivation built
// on branch names and body prose is a guess. This is the shared parse; deskpr owns
// the refusal (exit 5) and the file resolution under --root.
//
// WHY `Authors:` IS ITS OWN KIND (#1339). Every reader of the edge — the dispatcher's
// phantom check, the planner's already-represented reconciliation, the derived board —
// reads `Brief:` as "this PR DELIVERS the brief". A PR that only wrote the brief file
// carried `Brief:` too, so once it merged the brief it authored read as delivered and
// was never dispatched. `Authors:` names the same briefs without asserting delivery:
// no reader that keys on TrailerBrief matches it, so an authoring PR can no longer
// shadow the implementation it is waiting for.

// TrailerKind is which link kind a trailer line carries.
type TrailerKind string

const (
	// TrailerBrief is `Brief: ...` — the PR delivers a brief.
	TrailerBrief TrailerKind = "brief"
	// TrailerIssue is `Issue: #<N>` — issue-only work, no brief.
	TrailerIssue TrailerKind = "issue"
	// TrailerAuthors is `Authors: <stream>/<NN>[, <stream>/<NN> …]` — the PR AUTHORS
	// (writes the files of) the named briefs and delivers none of them. Value is the raw
	// list; SplitAuthorsTrailer reduces it to brief ids.
	TrailerAuthors TrailerKind = "authors"
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
	reTrailerAuth  = regexp.MustCompile(`^[ \t]*Authors:[ \t]*(\S.*?)[ \t]*$`)
	reFenceOpen    = regexp.MustCompile("^[ \t]*```")
)

// ErrTrailerDuplicate reports a second Brief: (or second Issue:, or second Authors:) trailer.
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

// ErrTrailerMixed reports a body carrying an Authors: trailer together with a Brief: or
// an Issue: trailer. A PR either authors briefs, delivers one, or does issue-only work;
// one link per body keeps the board's edge unambiguous. (Brief: with Issue: and no
// Authors: stays ErrTrailerBoth, unchanged.) Kinds and Lines are parallel, in body order.
type ErrTrailerMixed struct {
	Kinds []TrailerKind
	Lines []int
}

func (e *ErrTrailerMixed) Error() string {
	parts := make([]string, len(e.Kinds))
	for i, k := range e.Kinds {
		parts[i] = fmt.Sprintf("%s: (line %d)", trailerKeyword(k), e.Lines[i])
	}
	return strings.Join(parts, " and ") + " trailers present — exactly one link per PR body " +
		"(a PR authors briefs, delivers one brief, or does issue-only work)"
}

// Has reports whether kind is one of the mixed trailers.
func (e *ErrTrailerMixed) Has(kind TrailerKind) bool {
	for _, k := range e.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// trailerKeyword is the literal keyword a kind is written with in a PR body.
func trailerKeyword(k TrailerKind) string {
	switch k {
	case TrailerBrief:
		return "Brief"
	case TrailerIssue:
		return "Issue"
	case TrailerAuthors:
		return "Authors"
	}
	return string(k)
}

// SplitAuthorsTrailer reduces an `Authors:` trailer value — one or more brief ids
// separated by commas and/or whitespace — to canonical `<stream>/<NN>` ids (lower-cased,
// in the order written). Each entry is reduced through SplitBriefTrailer, so every form
// `Brief:` accepts is accepted here and names the same brief. An empty list, an entry
// that is not a brief id, or the same brief named twice is an error: the trailer is a
// data edge, and a malformed edge is refused rather than partially read.
func SplitAuthorsTrailer(v string) ([]string, error) {
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return nil, fmt.Errorf("`Authors:` trailer names no brief")
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(fields))
	for _, f := range fields {
		id := CanonicalBriefID(f)
		if id == "" {
			return nil, fmt.Errorf("`Authors:` entry %q does not name a brief as <stream>/<NN> or <stream>:<NN>", f)
		}
		if seen[id] {
			return nil, fmt.Errorf("`Authors:` names %s twice", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

// SplitBriefTrailer reduces an accepted `Brief:` trailer value to its (stream, NN)
// parts. It is the SINGLE reduction the writer (deskpr's create-time trailer
// validation) and the reader (RepresentedBriefs' phantom key) both share, so a
// colon-form trailer and its slash-form brief id can never be classed differently
// by the two sides — the exact way a phantom double-dispatch slipped through when
// only the writer knew the colon form.
//
// Accepted forms: <stream>/<NN> (brief-v1), <stream>:<NN>, <repo>:<stream>:<NN>, and
// the full <cell>:<repo>:<stream>:<NN> (example-stream/01). For the colon forms the
// LAST two parts are stream and NN; the repo/cell prefixes resolve against
// graph-repos.yaml elsewhere and are not needed for the reduction here. NN must be
// numeric. ok is false when v is not a well-formed brief trailer value.
func SplitBriefTrailer(v string) (stream, nn string, ok bool) {
	v = strings.TrimSpace(v)
	var parts []string
	if strings.Contains(v, ":") {
		parts = strings.Split(v, ":")
		if len(parts) < 2 {
			return "", "", false
		}
		stream, nn = parts[len(parts)-2], parts[len(parts)-1]
	} else {
		parts = strings.Split(v, "/")
		if len(parts) != 2 {
			return "", "", false
		}
		stream, nn = parts[0], parts[1]
	}
	if stream == "" || nn == "" {
		return "", "", false
	}
	for _, c := range nn {
		if c < '0' || c > '9' {
			return "", "", false
		}
	}
	return stream, nn, true
}

// CanonicalBriefID reduces a `Brief:` trailer value to the canonical brief id
// `<stream>/<NN>`, lower-cased — the one spelling both the phantom key and the plan's
// item id use, regardless of whether the trailer was written in the slash or the colon
// form. It returns "" when v is not a well-formed brief trailer value.
func CanonicalBriefID(v string) string {
	stream, nn, ok := SplitBriefTrailer(v)
	if !ok {
		return ""
	}
	return strings.ToLower(stream + "/" + nn)
}

// ParseTrailers returns the link trailers in a PR body. Lines inside fenced code
// blocks (```) are ignored — a trailer inside a code sample is documentation, not
// a link. Exactly one link may exist: a second Brief:, Issue: or Authors: line, or
// two different kinds, is a multiplicity error (the caller refuses). A body with no
// trailer is NOT an error here — the caller decides what absence means (deskpr
// refuses it). An Authors: value is returned raw; SplitAuthorsTrailer validates it.
func ParseTrailers(body []byte) ([]Trailer, error) {
	var trs []Trailer
	var firstBrief, firstIssue, firstAuth int
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
		case reTrailerAuth.MatchString(raw):
			v := strings.TrimSpace(reTrailerAuth.FindStringSubmatch(raw)[1])
			if firstAuth != 0 {
				return nil, &ErrTrailerDuplicate{Kind: TrailerAuthors, FirstLine: firstAuth, Line: line}
			}
			firstAuth = line
			trs = append(trs, Trailer{Kind: TrailerAuthors, Value: v, Line: line})
		}
	}
	if firstAuth != 0 && (firstBrief != 0 || firstIssue != 0) {
		mixed := &ErrTrailerMixed{}
		for _, t := range trs {
			mixed.Kinds = append(mixed.Kinds, t.Kind)
			mixed.Lines = append(mixed.Lines, t.Line)
		}
		return nil, mixed
	}
	if firstBrief != 0 && firstIssue != 0 {
		return nil, &ErrTrailerBoth{BriefLine: firstBrief, IssueLine: firstIssue}
	}
	return trs, nil
}
