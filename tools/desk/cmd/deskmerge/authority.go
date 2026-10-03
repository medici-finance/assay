package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// authority.go — the R-5 gate. It stands between `deskmerge merge` and every write.
//
// Nothing here trusts a flag, a login handed to it, or a file the caller can write. The
// register STATES a claim ("R-5 was signed, here is the artifact"); the artifact is
// what gets fetched and verified. A caller who edits rulings.md in their own worktree
// only changes WHICH URL gets fetched — they cannot forge the author of the comment at
// it.
//
// R-5 is UNSIGNED as of 2026-08-13, and its own Provenance block says why that matters:
// the rule's substance came from an unrecorded in-session direction, and the only
// only on-thread artifact from the blessing authority ("brief-09 was my suggestion")
// states nothing about two-parent merges, conflict hunks or regenerable-file lists. So
// `merge` refuses today, and that refusal is the tool working, not a bug to route
// around.

// defaultRulingsPath is where R-5 lives in this repo. It is a path, not a URL.
const defaultRulingsPath = "docs/streams/issue-flow/rulings.md"

// rulingID is the ONE ruling deskmerge implements. It is named in the audit line of
// every merge, so a reader of a desk-authored merge commit can go read the authority
// rather than take the tool's word for it.
const rulingID = "R-5"

// grant is the verified outcome of the ruling gate: the human artifact that authorizes
// this run, already fetched and already author-checked.
type grant struct {
	SignOffURL  string
	AuthorLogin string
}

// ghComment is the subset of a comment the authorization check consumes. It is populated
// from Forge.ListCommentsTyped, which carries the author's login, numeric id AND actor type
// (deskkit.Account.Type — empty where the forge did not report one, which verifyHumanAuthor
// refuses rather than reading as "User").
type ghComment struct {
	ID      int64
	HTMLURL string
	Body    string
	User    struct {
		Login string
		ID    int64
		Type  string
	}
}

// fetchComment retrieves the comment a permalink names.
//
// Fail-closed in every direction: an unparseable URL is refused, and a fetch that does
// not come back is Unverifiable (exit 6) with ZERO merges performed. COULD-NOT-CHECK IS
// NOT AUTHORIZATION — a tool that proceeds because it failed to reach the artifact has
// exactly the authorization of one that never looked.
//
// The kind of thread is derived from the permalink: `/pull/<N>` proves a change; `/issues/<N>`
// is read as an issue first and, only when that is could-not-check because the number names a
// change, retried as a change (the forge redirects a PR comment linked under `/issues/`). Which
// thread it was found on never widens what it authorizes — the id match below does.
func fetchComment(url string) (ghComment, error) {
	m := deskkit.CommentPermalinkRe.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return ghComment{}, deskkit.Refused(
			"refused: " + deskkit.StripControl(url) + " is not a GitHub comment permalink " +
				"(want https://github.com/<owner>/<repo>/issues|pull/<N>#issuecomment-<id>). " +
				"A link to a thread is not an authorization: a thread is written by whoever shows up.")
	}
	kind := deskkit.TargetIssue
	if m[3] == "pull" {
		kind = deskkit.TargetChange
	}
	c, err := fetchCommentKinded(url, m, kind)
	if kind == deskkit.TargetIssue && deskkit.IsUnverifiable(err) {
		return fetchCommentKinded(url, m, deskkit.TargetChange)
	}
	return c, err
}

// fetchCommentKinded lists the comments on the item the permalink names, of the stated kind,
// and returns the one whose database id matches. Listing on the NAMED item makes the old
// issue_url cross-check inherent: a comment id that is not on that item is refused, so a
// doctored link cannot display one thread while authorizing from a comment on another.
func fetchCommentKinded(url string, m []string, kind deskkit.TargetKind) (ghComment, error) {
	owner, repo, itemStr, cidStr := m[1], m[2], m[4], m[5]
	itemN, ierr := strconv.Atoi(itemStr)
	if ierr != nil || itemN <= 0 {
		return ghComment{}, deskkit.Refused("refused: " + deskkit.StripControl(url) + " names an invalid item number")
	}
	cid, cerr := strconv.ParseInt(cidStr, 10, 64)
	if cerr != nil || cid <= 0 {
		return ghComment{}, deskkit.Refused("refused: " + deskkit.StripControl(url) + " names an invalid comment id")
	}
	fg, fr, ferr := forgeFor(owner + "/" + repo)
	if ferr != nil {
		return ghComment{}, deskkit.Unverifiable(
			"could-not-check: the authorizing comment at "+deskkit.StripControl(url)+
				" could not be fetched — deskmerge refuses. An unreadable authorization is not an "+
				"authorization; zero merges were performed", ferr)
	}
	comments, err := fg.ListCommentsTyped(fr, itemN, kind)
	if err != nil {
		return ghComment{}, deskkit.Unverifiable(
			"could-not-check: the authorizing comment at "+deskkit.StripControl(url)+
				" could not be fetched — deskmerge refuses. An unreadable authorization is not an "+
				"authorization; zero merges were performed", err)
	}
	for _, c := range comments {
		if c.DatabaseID != cid {
			continue
		}
		out := ghComment{ID: c.DatabaseID, HTMLURL: c.URL, Body: c.Body}
		out.User.Login = c.Author.Login
		out.User.ID = c.Author.ID
		out.User.Type = c.Author.Type
		if out.HTMLURL == "" {
			out.HTMLURL = deskkit.StripControl(url)
		}
		return out, nil
	}
	return ghComment{}, deskkit.Refused(
		"refused: the authorization permalink names comment " + deskkit.StripControl(cidStr) +
			" on " + deskkit.StripControl(owner+"/"+repo+"#"+itemStr) +
			", but no such comment is on that item — the link and the artifact disagree, or the comment was deleted")
}

// verifyHumanAuthor is the load-bearing identity check, and it is deliberately made of
// two independent conditions rather than one — the same pair deskclose requires, for
// the same reasons.
//
// A shared automation account reports "type": "User" exactly as a person does, so the
// type check alone would admit it. A login is a claim about a name, so the login check
// alone would admit a recycled or squatted one. Both are required:
//
//	type == "User"                          — an App/Bot artifact is never authorization
//	IsBlessAuthorityIDStrict(login, id)     — the ONE roster-pinned human, login AND id
//
// deskkit.TrustedAuthor is deliberately NOT used: it contains every desk App, and the
// desk authorizing its own merges is the precise thing R-5's gate exists to prevent.
func verifyHumanAuthor(c ghComment, what string) error {
	who := deskkit.StripControl(c.User.Login)
	if !strings.EqualFold(c.User.Type, "User") {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is authored by %s (type %s) — an App or Bot artifact is never a human "+
				"authorization, whatever permissions it holds", what, who, deskkit.StripControl(c.User.Type)))
	}
	if !deskkit.IsBlessAuthorityIDStrict(c.User.Login, c.User.ID) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is authored by %s (id %d), which is not the configured blessing authority. "+
				"Reporting type=User is not enough: the desk, the workers and the human share automation "+
				"identities that also report type=User. The authorizing artifact must be authored by the "+
				"single roster-pinned human account.", what, who, c.User.ID))
	}
	return nil
}

// authorize runs the R-5 gate: read the claim, fetch the artifact, verify the author.
//
// The register parse is deskkit.ReadRulingSignOff, a (url, error) adapter over the ONE
// reader deskclose's R-1 gate also delegates to (deskkit.ReadSignOff). So the "is this
// ruling signed?" question has one implementation and not one per tool: an ambiguous or
// multi-URL register reads the same way here as it does for deskclose.
func authorize(rulingsPath string) (grant, error) {
	url, err := deskkit.ReadRulingSignOff(rulingsPath, rulingID, toolName)
	if err != nil {
		return grant{}, err
	}
	c, err := fetchComment(url)
	if err != nil {
		return grant{}, err
	}
	if err := verifyHumanAuthor(c, rulingID+"'s sign-off artifact"); err != nil {
		return grant{}, err
	}
	return grant{SignOffURL: c.HTMLURL, AuthorLogin: c.User.Login}, nil
}
