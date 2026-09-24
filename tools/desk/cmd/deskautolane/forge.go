package main

import (
	"encoding/json"
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
	// ListCommentsTyped reads the sign-off artifact's thread by its STATED kind: a permalink
	// under /issues/ is an issue thread, which the change-kind read cannot serve.
	ListCommentsTyped(repo deskkit.ForgeRepo, number int, kind deskkit.TargetKind) ([]deskkit.Comment, error)
	// RepoHardeningRead (kind repo only) resolves a repo's default branch — where the lane
	// reads the rulings register and .assay-surfaces, and the only base it admits.
	RepoHardeningRead(repo deskkit.ForgeRepo, kind deskkit.HardeningReadKind) (json.RawMessage, error)
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

// enactment is the lane's enactment gate. Every step must positively hold:
//
//  1. the rulings register is read THROUGH THE FORGE, from the register repo (--rulings-repo,
//     default --repo) at that repo's DEFAULT branch — never from the caller's worktree, which
//     may be a checkout of a PR head whose author edited the register;
//  2. R-8's Sign-off line names ONE comment permalink, on a thread in that same register repo
//     — a comment anywhere else is not this register's decision;
//  3. that comment, fetched, is authored by a forge User (never an App or Bot) who is the
//     roster-pinned blessing authority, login AND numeric id;
//  4. its BODY is an acceptance: a line reading exactly `Enact: R-8`, and no rejection or
//     negation anywhere in it (deskkit.AutoLaneAcceptance). A rejection recorded on the
//     Sign-off line, or an unrelated comment by the same human, enacts nothing.
//
// It returns (true, nil) only when every step holds. Every other outcome is not enacted, with
// the reason; an unreadable step is could-not-check (unverifiable), never "signed" and never
// "unsigned".
func enactment(o *opts, fg laneForge) (bool, error) {
	rOwner, rName, _ := strings.Cut(o.rulingsRepo, "/")
	rr := deskkit.ForgeRepo{Owner: rOwner, Name: rName}
	where := rr.Slug() + ":" + o.rulings
	db, err := defaultBranch(o, fg, rr)
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — the default branch of %s could not be resolved, so the rulings register "+
				"cannot be read at it", condRulingSigned, rr.Slug()), err)
	}
	fc, err := fg.ReadFile(rr, deskkit.ReadFileInput{File: o.rulings, Ref: db})
	switch {
	case (err != nil && deskkit.IsForgeNotFound(err)) || (err == nil && (fc == nil || !fc.Exists)):
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — no rulings register at %s on the default branch %s; an absent register "+
				"is not an unsigned one", condRulingSigned, deskkit.StripControl(where), db), nil)
	case err != nil:
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — the rulings register at %s could not be read", condRulingSigned,
			deskkit.StripControl(where)), err)
	}
	so := deskkit.ReadSignOffFromText(string(fc.Content), deskkit.AutoLaneRulingID)
	switch so.State {
	case deskkit.SignOffUnsigned:
		return false, deskkit.Refused(fmt.Sprintf(
			"refused: ruling-unsigned (condition %s) — %s's Sign-off line in %s (default branch %s) is EMPTY. "+
				"The lane is inert until a human records an acceptance artifact on that line; this refusal is "+
				"the gate working.", condRulingSigned, deskkit.AutoLaneRulingID, deskkit.StripControl(where), db))
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
	if !strings.EqualFold(m[1], rr.Owner) || !strings.EqualFold(m[2], rr.Name) {
		return false, deskkit.Refused(fmt.Sprintf(
			"refused: %s — %s's Sign-off names a comment in %s/%s, not in the register's own repo %s; a "+
				"comment elsewhere is not this register's decision", condRulingSigned, deskkit.AutoLaneRulingID,
			deskkit.StripControl(m[1]), deskkit.StripControl(m[2]), rr.Slug()))
	}
	item, _ := strconv.Atoi(m[4])
	cid, _ := strconv.ParseInt(m[5], 10, 64)
	kind := deskkit.TargetIssue
	if m[3] == "pull" {
		kind = deskkit.TargetChange
	}
	comments, err := fg.ListCommentsTyped(deskkit.ForgeRepo{Owner: m[1], Name: m[2]}, item, kind)
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — the sign-off artifact could not be fetched; an unreadable authorization "+
				"is not an authorization", condRulingSigned), err)
	}
	for _, c := range comments {
		if c.DatabaseID != cid {
			continue
		}
		switch t := strings.TrimSpace(c.Author.Type); {
		case t == "":
			return false, deskkit.Unverifiable(fmt.Sprintf(
				"could-not-check: %s — the forge reported no author type for the sign-off artifact, so a "+
					"human author cannot be established", condRulingSigned), nil)
		case !strings.EqualFold(t, "User"):
			return false, deskkit.Refused(fmt.Sprintf(
				"refused: %s — the sign-off artifact is authored by %s (type %s); an App or Bot artifact is never "+
					"a human authorization", condRulingSigned, deskkit.StripControl(c.Author.Login), deskkit.StripControl(t)))
		}
		if !deskkit.IsBlessAuthorityIDStrict(c.Author.Login, c.Author.ID) {
			return false, deskkit.Refused(fmt.Sprintf(
				"refused: %s — the sign-off artifact is authored by %s (id %d), which is not the configured "+
					"blessing authority", condRulingSigned, deskkit.StripControl(c.Author.Login), c.Author.ID))
		}
		if ok, why := deskkit.AutoLaneAcceptance(c.Body); !ok {
			return false, deskkit.Refused(fmt.Sprintf(
				"refused: %s — the sign-off artifact is not an acceptance of %s: %s", condRulingSigned,
				deskkit.AutoLaneRulingID, why))
		}
		return true, nil
	}
	return false, deskkit.Unverifiable(fmt.Sprintf(
		"could-not-check: %s — the sign-off permalink's comment was not found on its thread", condRulingSigned), nil)
}
