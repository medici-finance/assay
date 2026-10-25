// Package trailers owns work-link syntax and explicit new-write/historical profiles.
package trailers

import (
	"fmt"
	"regexp"
	"strings"
)

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
	// TrailerCrossRepoBrief is `Cross-repo-brief: <alias>:<stream>/<NN>` — the PR delivers a
	// brief homed (tracked) in ANOTHER repo, named by its graph-repos.yaml alias
	// (medici-finance/assay#2748). Value is the raw `<alias>:<stream>/<NN>`;
	// SplitCrossRepoBriefTrailer validates it.
	TrailerCrossRepoBrief TrailerKind = "cross-repo-brief"
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
	reTrailerXR    = regexp.MustCompile(`^[ \t]*Cross-repo-brief:[ \t]*(\S.*?)[ \t]*$`)
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
	case TrailerCrossRepoBrief:
		return "Cross-repo-brief"
	}
	return string(k)
}

// Line retains original source positions for diagnostics.
type Line struct {
	Number int
	Text   string
}

// OutsideFences ignores both Markdown fence characters, matching character and length.
func OutsideFences(body string) []Line {
	var out []Line
	var marker byte
	var width int
	for i, raw := range strings.Split(body, "\n") {
		t := strings.TrimSpace(raw)
		n := 0
		var c byte
		if len(t) > 0 {
			c = t[0]
		}
		if c == '`' || c == '~' {
			for n < len(t) && t[n] == c {
				n++
			}
		}
		if marker != 0 {
			if c == marker && n >= width && strings.TrimSpace(t[n:]) == "" {
				marker = 0
				width = 0
			}
			continue
		}
		if n >= 3 && (c == '~' || !strings.Contains(t[n:], "`")) {
			marker = c
			width = n
			continue
		}
		out = append(out, Line{i + 1, strings.TrimRight(raw, "\r")})
	}
	return out
}

// Scan is the historical-read profile: it retains mixed/duplicate links as facts.
func Scan(body []byte) []Trailer {
	var out []Trailer
	for _, line := range OutsideFences(string(body)) {
		for _, p := range []struct {
			kind TrailerKind
			re   *regexp.Regexp
		}{{TrailerBrief, reTrailerBrief}, {TrailerIssue, reTrailerIssue}, {TrailerAuthors, reTrailerAuth}, {TrailerCrossRepoBrief, reTrailerXR}} {
			if m := p.re.FindStringSubmatch(line.Text); m != nil {
				out = append(out, Trailer{p.kind, strings.TrimSpace(m[1]), line.Number})
				break
			}
		}
	}
	return out
}

// ParseStrict is the new-write profile: exactly one kind and one occurrence.
func ParseStrict(body []byte) ([]Trailer, error) {
	ts := Scan(body)
	first := map[TrailerKind]int{}
	for _, t := range ts {
		if at := first[t.Kind]; at != 0 {
			return nil, &ErrTrailerDuplicate{t.Kind, at, t.Line}
		}
		first[t.Kind] = t.Line
	}
	if len(ts) > 1 && (first[TrailerAuthors] != 0 || first[TrailerCrossRepoBrief] != 0) {
		e := &ErrTrailerMixed{}
		for _, t := range ts {
			e.Kinds = append(e.Kinds, t.Kind)
			e.Lines = append(e.Lines, t.Line)
		}
		return nil, e
	}
	if first[TrailerBrief] != 0 && first[TrailerIssue] != 0 {
		return nil, &ErrTrailerBoth{first[TrailerBrief], first[TrailerIssue]}
	}
	return ts, nil
}
