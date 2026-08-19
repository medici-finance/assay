package main

import (
	"encoding/json"
	"fmt"
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

// ghComment is the subset of a GitHub comment the authorization check consumes.
type ghComment struct {
	ID       int64  `json:"id"`
	HTMLURL  string `json:"html_url"`
	IssueURL string `json:"issue_url"`
	Body     string `json:"body"`
	User     struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	} `json:"user"`
}

// fetchComment retrieves the comment a permalink names.
//
// Fail-closed in every direction: an unparseable URL is refused, and a fetch that does
// not come back is Unverifiable (exit 6) with ZERO merges performed. COULD-NOT-CHECK IS
// NOT AUTHORIZATION — a tool that proceeds because it failed to reach the artifact has
// exactly the authorization of one that never looked.
func fetchComment(url string) (ghComment, error) {
	m := deskkit.CommentPermalinkRe.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return ghComment{}, deskkit.Refused(
			"refused: " + deskkit.StripControl(url) + " is not a GitHub comment permalink " +
				"(want https://github.com/<owner>/<repo>/issues|pull/<N>#issuecomment-<id>). " +
				"A link to a thread is not an authorization: a thread is written by whoever shows up.")
	}
	owner, repo, cid := m[1], m[2], m[5]
	raw, err := runGH("api", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/%s/issues/comments/%s", owner, repo, cid))
	if err != nil {
		return ghComment{}, deskkit.Unverifiable(
			"could-not-check: the authorizing comment at "+deskkit.StripControl(url)+
				" could not be fetched — deskmerge refuses. An unreadable authorization is not an "+
				"authorization; zero merges were performed", err)
	}
	var c ghComment
	if jerr := json.Unmarshal([]byte(raw), &c); jerr != nil {
		return ghComment{}, deskkit.Unverifiable(
			"could-not-check: the authorizing comment at "+deskkit.StripControl(url)+
				" did not parse as a GitHub comment", jerr)
	}
	// The permalink's own item path must match the comment's issue_url. Without this a
	// doctored link could display one thread while authorizing from a comment on
	// another.
	wantItem := fmt.Sprintf("/repos/%s/%s/issues/%s", owner, repo, m[4])
	if !strings.HasSuffix(strings.TrimSuffix(c.IssueURL, "/"), wantItem) {
		return ghComment{}, deskkit.Refused(
			"refused: the authorization permalink names " + deskkit.StripControl(owner+"/"+repo+"#"+m[4]) +
				" but the comment it resolves to lives on " + deskkit.StripControl(c.IssueURL) +
				" — the link and the artifact disagree")
	}
	return c, nil
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
