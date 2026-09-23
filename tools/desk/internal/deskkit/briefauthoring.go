package deskkit

import (
	"path"
	"strings"
)

// briefauthoring.go — telling a PR that AUTHORED a brief apart from one that DELIVERED it.
//
// THE DEFECT IT CLOSES. The already-represented reconciliation (prrepr.go) keys on a PR body's
// `Brief:` trailer. A docs-only PR that writes a new brief carries that trailer too, so once it
// merges the brief it authored reads as delivered, and a fresh dispatch of that brief is refused
// forever. The trailer alone cannot tell the two apart; the PR's changed-file list can.
//
// THE CLASSIFICATION. A PR is the AUTHORING of brief `<stream>/<NN>`, and not its delivery, only
// when BOTH hold:
//
//  1. Every file it touches is methodology prose: a path under `docs/streams/`, or a changelog
//     fragment (`changelog/<name>.md`; the directory's README is not a fragment). A rename is
//     judged on BOTH halves, so moving a code file into `docs/streams/` still touches code. One
//     path outside that set, and the PR is a delivery.
//  2. It ADDS that brief's own file, `docs/streams/<stream>/brief-<NN>-*.md`.
//
// Rule 2 is load-bearing, not decoration. A brief whose deliverable is itself a document under
// `docs/streams/` (an audit, a scoping note) is delivered by a PR that also touches only
// `docs/streams/` plus the board README, so rule 1 alone would read that delivery as authoring and
// re-open a delivered brief. A delivery never adds its own brief's file (the brief already
// exists), while an authoring PR always does. When the file list is empty, or either rule fails,
// the answer is "not authoring-only", which leaves the PR counted as a delivery. That is the
// status quo the check had before this file existed, so a doubtful case costs a refusal, never a
// worker spent on a delivered brief.
//
// The caller owns the file list's completeness. ListChangedFiles is bounded, so a caller must
// reconcile it against PullRequest.ChangedFiles before it asks this question (forge.go).

// BriefAuthoringOnly reports whether a change touching `files` only AUTHORED briefID
// (`<stream>/<NN>`, any form SplitBriefTrailer accepts) rather than delivering it. It returns false
// for an empty list, an unparseable brief id, any file outside `docs/streams/` and changelog
// fragments, and a list that does not add the brief's own file.
func BriefAuthoringOnly(briefID string, files []ChangedFile) bool {
	stream, nn, ok := SplitBriefTrailer(briefID)
	if !ok || len(files) == 0 {
		return false
	}
	authored := false
	for _, f := range files {
		if !isAuthoringPath(f.Filename) {
			return false
		}
		if f.PreviousFilename != "" && !isAuthoringPath(f.PreviousFilename) {
			return false
		}
		if strings.EqualFold(strings.TrimSpace(f.Status), "added") && isBriefFileFor(f.Filename, stream, nn) {
			authored = true
		}
	}
	return authored
}

// cleanRepoPath normalises a forge-reported repo-relative path so a `./` prefix or a `..` segment
// cannot make a path outside `docs/streams/` look like one inside it.
func cleanRepoPath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" {
		return ""
	}
	return path.Clean(p)
}

// isAuthoringPath reports whether p is methodology prose an authoring PR may touch: a path under
// `docs/streams/`, or a changelog fragment directly under `changelog/` (not its README).
func isAuthoringPath(p string) bool {
	p = cleanRepoPath(p)
	if p == "" || strings.HasPrefix(p, "/") || p == ".." || strings.HasPrefix(p, "../") {
		return false
	}
	if strings.HasPrefix(p, "docs/streams/") {
		return true
	}
	dir, base := path.Split(p)
	return dir == "changelog/" && strings.HasSuffix(base, ".md") && base != ".md" &&
		!strings.EqualFold(base, "README.md")
}

// isBriefFileFor reports whether p is brief <stream>/<NN>'s own file,
// `docs/streams/<stream>/brief-<NN>-*.md` (the layout deskpr's trailer validation resolves).
func isBriefFileFor(p, stream, nn string) bool {
	dir, base := path.Split(cleanRepoPath(p))
	if !strings.EqualFold(dir, "docs/streams/"+stream+"/") {
		return false
	}
	prefix := "brief-" + nn + "-"
	return strings.HasPrefix(base, prefix) && strings.HasSuffix(base, ".md") && len(base) > len(prefix)+len(".md")
}
