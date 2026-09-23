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
//  1. Every file it touches is one an authoring PR writes: a stream's board README
//     (`docs/streams/<stream>/README.md`), a brief file (`docs/streams/<stream>/brief-<NN>-*.md`, any
//     stream, any number), or a changelog fragment (`changelog/<name>.md`; the directory's README is
//     not a fragment). A rename is judged on BOTH halves, so moving a code file into
//     `docs/streams/` still touches code. Any other path, including any other document under
//     `docs/streams/` (an audit, a decision record, a scoping note), makes the PR a delivery.
//  2. It ADDS that brief's own file, `docs/streams/<stream>/brief-<NN>-*.md`.
//
// Rule 1 is deliberately narrower than "anything under docs/streams/". Some briefs deliver a
// document under `docs/streams/`, and a PR may author a brief AND deliver it in one change, which
// adds the brief's own file too. Such a PR passes rule 2, so rule 1 is the rule that keeps it a
// delivery: its deliverable is a path that is neither a board README nor a brief file. Rule 2 keeps
// a PR that only edits an existing brief (evidence, a status flip, a revision) counted as a
// delivery, because that PR did not author the brief. When the file list is empty, or either rule
// fails, the answer is "not authoring-only", which leaves the PR counted as a delivery. That is the
// status quo the check had before this file existed, so a doubtful case costs a refusal, never a
// worker spent on a delivered brief.
//
// The caller owns the file list's completeness. ListChangedFiles is bounded, so a caller must
// reconcile it against PullRequest.ChangedFiles before it asks this question (forge.go).

// BriefAuthoringOnly reports whether a change touching `files` only AUTHORED briefID
// (`<stream>/<NN>`, any form SplitBriefTrailer accepts) rather than delivering it. It returns false
// for an empty list, an unparseable brief id, any file that is not a stream board README, a brief file
// or a changelog fragment, and a list that does not add the brief's own file.
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

// isAuthoringPath reports whether p is a file an authoring PR writes: a stream's board README
// (`docs/streams/<stream>/README.md`), a brief file (`docs/streams/<stream>/brief-<NN>-*.md`), or a
// changelog fragment directly under `changelog/` (not its README). Nothing else under
// `docs/streams/` qualifies, because a document there can be a brief's deliverable.
func isAuthoringPath(p string) bool {
	p = cleanRepoPath(p)
	if p == "" || strings.HasPrefix(p, "/") || p == ".." || strings.HasPrefix(p, "../") {
		return false
	}
	dir, base := path.Split(p)
	if dir == "changelog/" {
		return strings.HasSuffix(base, ".md") && base != ".md" && !strings.EqualFold(base, "README.md")
	}
	stream, ok := strings.CutPrefix(dir, "docs/streams/")
	if !ok {
		return false
	}
	stream = strings.TrimSuffix(stream, "/")
	if stream == "" || strings.Contains(stream, "/") {
		return false
	}
	return base == "README.md" || isBriefFileName(base)
}

// isBriefFileName reports whether base names a brief file, `brief-<NN>-<slug>.md` with a numeric NN.
func isBriefFileName(base string) bool {
	rest, ok := strings.CutPrefix(base, "brief-")
	if !ok {
		return false
	}
	nn, slug, ok := strings.Cut(rest, "-")
	if !ok || nn == "" || !strings.HasSuffix(slug, ".md") || slug == ".md" {
		return false
	}
	for _, c := range nn {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
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
