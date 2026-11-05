package deskkit

import (
	"github.com/medici-finance/assay/statusgen/workref/trailers"

	"fmt"
	"strings"

	"github.com/medici-finance/assay/statusgen/workref"
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
type TrailerKind = trailers.TrailerKind

const (
	TrailerBrief          = trailers.TrailerBrief
	TrailerIssue          = trailers.TrailerIssue
	TrailerAuthors        = trailers.TrailerAuthors
	TrailerCrossRepoBrief = trailers.TrailerCrossRepoBrief
)

type Trailer = trailers.Trailer
type ErrTrailerDuplicate = trailers.ErrTrailerDuplicate
type ErrTrailerBoth = trailers.ErrTrailerBoth
type ErrTrailerMixed = trailers.ErrTrailerMixed

// WHY `Cross-repo-brief:` IS ITS OWN KIND (medici-finance/assay#2748). A brief homed in one repo
// may be delivered by a PR in another. A `Brief:` line there would be read by every reader in the
// DELIVERABLE repo as a brief on its own board — deskpr resolves it under --root, the phantom check
// and the board key on it — and the brief is not there, so create and edit refused it, or (with
// the alias prefix SplitBriefTrailer drops) it would name a local brief that does not exist. The
// new kind names the home explicitly and asserts nothing about this repo's board: no reader keyed
// on TrailerBrief matches it, so what an existing `Brief:` line means is unchanged.

// SplitCrossRepoBriefTrailer reduces a `Cross-repo-brief:` value to (alias, stream, NN). The only
// accepted form is `<alias>:<stream>/<NN>` — a lowercase registry alias, one colon, a slash-form
// brief id with a numeric NN — and ok is false for anything else. The alias resolves through
// graph-repos.yaml elsewhere (deskpr checks it is registered and is not the registry's own self).
func SplitCrossRepoBriefTrailer(v string) (alias, stream, nn string, ok bool) {
	parsed, err := workref.Parse(v, workref.LegacyCrossRepo)
	if err != nil {
		return "", "", "", false
	}
	return parsed.Repo, parsed.Stream, parsed.Number, true
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
	parsed, err := workref.Parse(v, workref.LegacyTrailer)
	if err != nil {
		return "", "", false
	}
	return parsed.Stream, parsed.Number, true
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
// a link. Exactly one link may exist: a second Brief:, Issue:, Authors: or Cross-repo-brief: line, or
// two different kinds, is a multiplicity error (the caller refuses). A body with no
// trailer is NOT an error here — the caller decides what absence means (deskpr
// refuses it). An Authors: value is returned raw; SplitAuthorsTrailer validates it.
func ParseTrailers(body []byte) ([]Trailer, error) { return trailers.ParseStrict(body) }

// ResolveWorkTrailer is the explicit typed adapter for negotiated callers. It
// retains exact selectors; existing operational trailer readers remain legacy.
func ResolveWorkTrailer(v string, declaring *workref.Context, registry workref.Registry) (workref.Result, error) {
	return workref.Resolve(v, declaring, registry, workref.TypedBrief)
}
