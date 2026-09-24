package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// laneForge is the subset of deskkit.Forge this verb reads and writes through. It is an
// interface of the verb's own so the test harness can stand in for the far side of the wire
// and RECORD every call, and so the set of operations this verb can perform is visible in one
// place: there is no merge operation on it. deskkit.Forge satisfies it (asserted below).
type laneForge interface {
	GetPullRequest(repo deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error)
	ListChangedFiles(repo deskkit.ForgeRepo, number int) ([]deskkit.ChangedFile, error)
	ReviewsAtHead(repo deskkit.ForgeRepo, number int) ([]deskkit.Review, error)
	ChecksAtHead(repo deskkit.ForgeRepo, sha string) (*deskkit.ChecksAtHead, error)
	RequiredStatusChecks(repo deskkit.ForgeRepo, branch string) ([]string, error)
	ListLabelEvents(repo deskkit.ForgeRepo, number int) ([]deskkit.LabelEvent, error)
	ListComments(repo deskkit.ForgeRepo, number int) ([]deskkit.Comment, error)
	ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error)
	// The two WRITES this verb owns: the admission/ejection label swap and the one marked
	// ejection comment. Both are gated on the enactment gate (see enactment).
	ApplyLabels(repo deskkit.ForgeRepo, number int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error)
	PostComment(repo deskkit.ForgeRepo, number int, body string) (*deskkit.CommentRef, error)
}

var _ laneForge = deskkit.Forge(nil)

// mintTokenFn and resolveForgeFn are the seams the App-token condition runs through, so a
// test can drive the verb without a real App credential. Production binds the shared deskkit
// resolver — no ambient CLI credential, no host literal of this verb's own.
var (
	mintTokenFn    = deskkit.RoleTokenForRepo
	resolveForgeFn = func(fr deskkit.ForgeRepo, role string) (laneForge, error) {
		fg, _, err := deskkit.ResolveForge(fr, role)
		if err != nil {
			return nil, err
		}
		return fg, nil
	}
)

// checkAppToken resolves the reviewer App's forge client, refusing rather than proceeding on
// an ambient credential when it cannot.
func checkAppToken(o *opts, fr deskkit.ForgeRepo) (laneForge, error) {
	role, ok := deskkit.TokenRoleForLoop(laneLoop)
	if !ok {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — loop %s has no App role", condAppToken, laneLoop), nil)
	}
	if _, path, err := mintTokenFn(role, fr.Slug()); err != nil {
		if strings.TrimSpace(path) == "" {
			path = "(the minter named no token path)"
		}
		return nil, deskkit.Refused(fmt.Sprintf(
			"refused: %s — the %s App installation token for %s could not be minted or read (%s): %v. "+
				"deskautolane never falls back to an ambient credential.", condAppToken, role, fr.Owner, path, err))
	}
	fg, err := resolveForgeFn(fr, role)
	if err != nil {
		return nil, err
	}
	o.say("%s OK: reads and writes authenticate as the %s App", condAppToken, role)
	return fg, nil
}

// enactment is the lane's enactment gate: the rulings register's AutoLaneRulingID Sign-off
// line must name ONE comment permalink, and that comment — fetched, never trusted from the
// file — must be authored by the roster-pinned blessing authority (login AND numeric id).
// A file in the caller's own worktree can only change WHICH URL gets fetched; it cannot
// forge the author of the comment at it.
//
// It returns (true, nil) only when every step positively holds. Every other outcome is not
// enacted, with the reason; an unreadable step is could-not-check (unverifiable), never
// "signed" and never "unsigned".
func enactment(o *opts, fg laneForge) (bool, error) {
	so := deskkit.ReadSignOff(o.rulings, deskkit.AutoLaneRulingID)
	switch so.State {
	case deskkit.SignOffUnsigned:
		return false, deskkit.Refused(fmt.Sprintf(
			"refused: ruling-unsigned (condition %s) — %s's Sign-off line in %s is EMPTY. The lane is inert "+
				"until a human records an acceptance artifact on that line; this refusal is the gate working.",
			condRulingSigned, deskkit.AutoLaneRulingID, deskkit.StripControl(o.rulings)))
	case deskkit.SignOffCouldNotRead:
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — %s", condRulingSigned, so.Detail), nil)
	}
	m := deskkit.CommentPermalinkRe.FindStringSubmatch(strings.TrimSpace(so.URL))
	if m == nil {
		return false, deskkit.Refused(fmt.Sprintf(
			"refused: %s — %s's Sign-off names %s, which is not a comment permalink; a thread is not an "+
				"authorization", condRulingSigned, deskkit.AutoLaneRulingID, deskkit.StripControl(so.URL)))
	}
	item, _ := strconv.Atoi(m[4])
	cid, _ := strconv.ParseInt(m[5], 10, 64)
	comments, err := fg.ListComments(deskkit.ForgeRepo{Owner: m[1], Name: m[2]}, item)
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — the sign-off artifact could not be fetched; an unreadable authorization "+
				"is not an authorization", condRulingSigned), err)
	}
	for _, c := range comments {
		if c.DatabaseID != cid {
			continue
		}
		if !deskkit.IsBlessAuthorityIDStrict(c.Author.Login, c.Author.ID) {
			return false, deskkit.Refused(fmt.Sprintf(
				"refused: %s — the sign-off artifact is authored by %s (id %d), which is not the configured "+
					"blessing authority", condRulingSigned, deskkit.StripControl(c.Author.Login), c.Author.ID))
		}
		return true, nil
	}
	return false, deskkit.Unverifiable(fmt.Sprintf(
		"could-not-check: %s — the sign-off permalink's comment was not found on its thread", condRulingSigned), nil)
}
