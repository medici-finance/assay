package deskkit

// commentlink.go — the ONE parser for a link to a single comment on a forge object.
//
// A comment link is what a desk verb is handed as AUTHORITY: "the human ruled here". The link
// is never the authority itself. A caller fetches the comment it names from the forge and
// verifies the author; this file only says WHICH comment, on WHICH forge, host, project and
// object, the link claims to name, so the caller can refuse a link that points anywhere other
// than where it is acting.
//
// The accepted forms, one per forge:
//
//	GitHub   https://github.com/<owner>/<repo>/issues|pull/<N>#issuecomment-<id>
//	GitLab   https://<host>/<group>/<project>/-/issues|merge_requests/<N>#note_<id>
//
// The GitHub form is CommentPermalinkRe, unchanged. The GitLab form reuses the item-URL
// grammar ParseItemRef reads (itemRefURLRe), so a project path that does not reduce to one
// `owner/name` coordinate is refused here exactly as it is there. Everything else is refused:
// a link with no comment anchor names a whole thread, and a thread is written by whoever
// shows up; an anchor shape this file does not know names a forge it cannot vouch for.
//
// The forge a link names is read from its SHAPE (the anchor and the path separator), never
// trusted as a fact about the repository. The caller compares it with the forge the
// repository actually resolves to, and refuses a mismatch.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CommentLink is one parsed comment link (CommentRef, in forge.go, is a posted comment's handle).
type CommentLink struct {
	// Forge is the forge the link's shape names: ForgeGitHub or ForgeGitLab.
	Forge ForgeKind
	// Host is the link's host, lowercased. It carries no port and no user info: a link that
	// has either is refused at parse.
	Host string
	// Item is the object the comment is on. Its Repo and Number are always set, and its Kind
	// is always stated (the URL path says which kind).
	Item ItemRef
	// CommentID is the comment's numeric id (a GitHub comment database id, a GitLab note id).
	CommentID int64
}

// gitlabNoteAnchorRe matches the anchor GitLab writes for one note.
var gitlabNoteAnchorRe = regexp.MustCompile(`^note_([0-9]+)$`)

// commentHostRe is the host a comment link may carry: a bare DNS name. A port or user info
// is refused rather than stripped, because the host is compared with the forge's own host and
// a stripped value would compare equal to a host the link did not name.
var commentHostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// CommentPermalinkForms is the ONE wording of the accepted comment-link forms, quoted by every
// refusal that has to say what it would have taken.
func CommentPermalinkForms() string {
	return "GitHub https://github.com/<owner>/<repo>/issues|pull/<N>#issuecomment-<id>, or GitLab " +
		"https://<host>/<group>/<project>/-/issues|merge_requests/<N>#note_<id>"
}

// ParseCommentPermalink parses one comment link. A link that is not one of the forms
// CommentPermalinkForms names is REFUSED; it is never resolved to a guess.
func ParseCommentPermalink(s string) (CommentLink, error) {
	trimmed := strings.TrimSpace(s)
	if m := CommentPermalinkRe.FindStringSubmatch(trimmed); m != nil {
		n, err := parseItemRefNumber(trimmed, m[4])
		if err != nil {
			return CommentLink{}, err
		}
		cid, err := parseCommentID(trimmed, m[5])
		if err != nil {
			return CommentLink{}, err
		}
		kind := TargetChange
		if m[3] == "issues" {
			kind = TargetIssue
		}
		return CommentLink{
			Forge: ForgeGitHub, Host: "github.com",
			Item:      ItemRef{Repo: m[1] + "/" + m[2], Number: n, Kind: kind},
			CommentID: cid,
		}, nil
	}

	base, anchor, hasAnchor := strings.Cut(trimmed, "#")
	if !hasAnchor || anchor == "" {
		return CommentLink{}, Refused(fmt.Sprintf(
			"refused: %s has no comment anchor, so it names a whole thread — a thread is written by whoever "+
				"shows up and is not an authorization. Link the one comment: %s",
			StripControl(trimmed), CommentPermalinkForms()))
	}
	am := gitlabNoteAnchorRe.FindStringSubmatch(anchor)
	if am == nil {
		return CommentLink{}, refuseCommentLink(trimmed, "its comment anchor is not one this desk knows")
	}
	u, ok, err := parseItemURL(base)
	if !ok {
		return CommentLink{}, refuseCommentLink(trimmed, "it is not a link to a numbered issue or merge request")
	}
	if err != nil {
		return CommentLink{}, err
	}
	switch {
	case u.scheme != "https":
		return CommentLink{}, refuseCommentLink(trimmed, "it is not https")
	case !commentHostRe.MatchString(u.host):
		return CommentLink{}, refuseCommentLink(trimmed, "its host carries a port or user info")
	case !u.dashSep || (u.segment != "issues" && u.segment != "merge_requests"):
		// A note anchor on a path GitLab does not write (no `/-/`, or GitHub's pull segment)
		// is not a GitLab link, and it is not a GitHub one either.
		return CommentLink{}, refuseCommentLink(trimmed, "its path is not the GitLab shape a note anchor belongs to")
	}
	cid, err := parseCommentID(trimmed, am[1])
	if err != nil {
		return CommentLink{}, err
	}
	return CommentLink{
		Forge: ForgeGitLab, Host: strings.ToLower(u.host), Item: u.ref, CommentID: cid,
	}, nil
}

func refuseCommentLink(link, why string) error {
	return Refused(fmt.Sprintf("refused: %s is not a comment link this desk accepts — %s. The accepted forms are %s",
		StripControl(link), why, CommentPermalinkForms()))
}

func parseCommentID(link, digits string) (int64, error) {
	id, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || id <= 0 {
		return 0, Refused(fmt.Sprintf("refused: %s names an invalid comment id", StripControl(link)))
	}
	return id, nil
}
