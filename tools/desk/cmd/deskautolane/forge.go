package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	// ListFileCommits and ListCommitChanges serve the enactment gate's time check: the rulings
	// register's history at the default branch, and the changes behind the commit that last
	// changed the ruling's text (read with GetPullRequest for their merge time).
	ListFileCommits(repo deskkit.ForgeRepo, ref, file string, limit int) ([]deskkit.RepoCommit, error)
	ListCommitChanges(repo deskkit.ForgeRepo, sha string) ([]int, error)
	// The two WRITES this verb owns: the admission/ejection label swap and the one marked
	// ejection comment. Both are gated on the enactment gate (see enactment).
	ApplyLabels(repo deskkit.ForgeRepo, number int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error)
	PostComment(repo deskkit.ForgeRepo, number int, body string) (*deskkit.CommentRef, error)
}

var _ laneForge = deskkit.Forge(nil)

// mintTokenFn and resolveForgeFn are the seams the App-token condition runs through, so a
// test can drive the verb without a real App credential. Production binds the shared deskkit
// resolver — no ambient CLI credential, no host literal of this verb's own. The mint goes
// through deskkit.GitHubRoleToken, the forge-aware GitHub arm: it refuses a repo the roster
// binds to another forge before any mint, so this verb never calls the raw App minter directly.
var (
	mintTokenFn    = deskkit.GitHubRoleToken
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
//  2. R-8's Sign-off line names ONE comment permalink, on a thread in that same register repo,
//     and on the ONE configured sign-off thread (ASSAY_AUTOAPPROVE_SIGNOFF_THREAD) — a comment
//     on any other thread is refused, and an unset thread is could-not-check. That thread is
//     an ISSUE: a pull-request permalink is refused, because a pull-request thread's comment
//     listing can stop before its newest comments, which step 6 must see;
//  3. that comment, fetched, is authored by a forge User (never an App or Bot) who is the
//     roster-pinned blessing authority, login AND numeric id;
//  4. its BODY is an acceptance: its first non-empty line is `Enact: R-8` typed bare, and no
//     word from the rejection/negation lexicon appears in it (deskkit.AutoLaneAcceptance);
//  5. it was CREATED AFTER the latest change to R-8's text above its Sign-off line that the
//     register's path history at the default branch records (rulingTextAnchor). The change's
//     time is its merging PR's merged_at, never a commit date; a change that touches only the
//     Sign-off line does not move it; a recorded change with no merged PR behind it is
//     refused. An acceptance of an older text is not an acceptance of this one. The path
//     history is the forge's simplified one, so it can omit a change (see rulingTextAnchor);
//  6. it is the blessing authority's NEWEST acceptance on the sign-off thread
//     (supersededAcceptance): an acceptance the authority has since replaced enacts nothing.
//     The thread is read whole: the issue listing is walked to its end, and a listing the
//     forge cannot complete is could-not-check.
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
	// The ONE sign-off thread. The register repo pins where the comment may live; the thread
	// pins which conversation it may be in. Unset is could-not-check, never "any thread".
	switch {
	case o.signOffThread <= 0:
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — no sign-off thread is configured (%s), so the acceptance comment's thread "+
				"cannot be checked; an unpinned thread enacts nothing", condRulingSigned,
			deskkit.EnvAutoApproveSignOffThread), nil)
	case item != o.signOffThread:
		return false, deskkit.Refused(fmt.Sprintf(
			"refused: %s — %s's Sign-off names a comment on #%d, not on the configured sign-off thread #%d; a "+
				"comment on any other thread is not the acceptance", condRulingSigned, deskkit.AutoLaneRulingID,
			item, o.signOffThread))
	}
	// The sign-off thread is an ISSUE. Step 6 needs the NEWEST end of the thread, and only an
	// issue's comment listing is walked to its end (or refused at its page cap). A pull-request
	// thread is listed as its first 100 comments, with no error and no sign of the rest, so a
	// later acceptance past them would go unseen and a superseded acceptance would enact.
	if m[3] != "issues" {
		return false, deskkit.Refused(fmt.Sprintf(
			"refused: %s — %s's Sign-off names a comment on pull request #%d; the sign-off thread must be an "+
				"issue, because a pull-request thread's comment listing can stop before its newest comments",
			condRulingSigned, deskkit.AutoLaneRulingID, item))
	}
	comments, err := fg.ListCommentsTyped(deskkit.ForgeRepo{Owner: m[1], Name: m[2]}, item, deskkit.TargetIssue)
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
		if err := acceptanceAfterText(o, fg, rr, db, string(fc.Content), c.CreatedAt); err != nil {
			return false, err
		}
		if err := supersededAcceptance(comments, c); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, deskkit.Unverifiable(fmt.Sprintf(
		"could-not-check: %s — the sign-off permalink's comment was not found on its thread", condRulingSigned), nil)
}

// supersededAcceptance is enactment step 6: the named acceptance must be the blessing
// authority's NEWEST acceptance on the sign-off thread. A later acceptance by the same
// authority means the Sign-off line names an act the authority has since replaced, typically
// the acceptance of an earlier text brought back by restoring an old register. The history
// walk in step 5 cannot always see such a restore (see rulingTextAnchor), so this step refuses
// it on the thread's own record. Only a later comment that is itself an acceptance, by a User
// who is the blessing authority, counts: nobody else can refuse the lane by posting on the
// thread. A later acceptance with no readable creation time is could-not-check. The thread's
// listing is the whole thread: step 2 pins it to an issue, whose listing the forge walks to
// its end or refuses.
func supersededAcceptance(comments []deskkit.Comment, named deskkit.Comment) error {
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(named.CreatedAt))
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — the sign-off artifact has no readable creation time", condRulingSigned), nil)
	}
	for _, c := range comments {
		if c.DatabaseID == named.DatabaseID || !strings.EqualFold(strings.TrimSpace(c.Author.Type), "User") ||
			!deskkit.IsBlessAuthorityIDStrict(c.Author.Login, c.Author.ID) {
			continue
		}
		if !readsAsLaterAcceptance(c.Body) {
			continue
		}
		later, perr := time.Parse(time.RFC3339, strings.TrimSpace(c.CreatedAt))
		if perr != nil {
			return deskkit.Unverifiable(fmt.Sprintf(
				"could-not-check: %s — another acceptance by the blessing authority on the sign-off thread "+
					"has no readable creation time, so the named one cannot be shown to be the newest", condRulingSigned), nil)
		}
		if later.After(at) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s — the sign-off artifact is superseded: the blessing authority posted a later "+
					"acceptance on the sign-off thread (comment %d, created %s); an acceptance the authority has "+
					"since replaced enacts nothing", condRulingSigned, c.DatabaseID, later.UTC().Format(time.RFC3339)))
		}
	}
	return nil
}

// readsAsLaterAcceptance is step 6's reading of a later comment by the blessing authority. It
// is WIDER than step 4's: leading whitespace on the first non-empty line is ignored, and the
// negation lexicon applies unchanged. Counting more later comments as acceptances only refuses
// more, so the wider reading narrows what enacts.
func readsAsLaterAcceptance(body string) bool {
	if ok, _ := deskkit.AutoLaneAcceptance(body); ok {
		return true
	}
	lines := strings.Split(body, "\n")
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			lines[i] = strings.TrimLeft(ln, " \t")
			break
		}
	}
	ok, _ := deskkit.AutoLaneAcceptance(strings.Join(lines, "\n"))
	return ok
}

// rulingHistoryLimit bounds the register-history walk: one page of the commits that touched
// the register. A walk that fills the page without finding where R-8's text last changed is
// could-not-check, never "the text never changed".
const rulingHistoryLimit = 50

// acceptanceAfterText is enactment step 5: the acceptance comment must have been created
// strictly AFTER the latest change to R-8's text above its Sign-off line that the register's
// path history records (rulingTextAnchor names what that history can omit).
func acceptanceAfterText(o *opts, fg laneForge, rr deskkit.ForgeRepo, db, current, createdAt string) error {
	accepted, perr := time.Parse(time.RFC3339, strings.TrimSpace(createdAt))
	if perr != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s — the forge reported no readable creation time for the sign-off artifact "+
				"(%q), so it cannot be placed after the ruling's text", condRulingSigned,
			deskkit.StripControl(createdAt)), nil)
	}
	anchor, pr, err := rulingTextAnchor(o, fg, rr, db, current)
	if err != nil {
		return err
	}
	if !accepted.After(anchor) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s — the sign-off artifact was created %s, not after %s's current text merged (#%d, "+
				"merged %s); an acceptance of an earlier text does not accept this one", condRulingSigned,
			accepted.UTC().Format(time.RFC3339), deskkit.AutoLaneRulingID, pr, anchor.UTC().Format(time.RFC3339)))
	}
	o.say("%s OK: the acceptance postdates %s's current text as the path history records it (#%d merged %s)", condRulingSigned,
		deskkit.AutoLaneRulingID, pr, anchor.UTC().Format(time.RFC3339))
	return nil
}

// rulingTextAnchor finds when R-8's current text landed, as the path history records it: it
// walks the register's history at the default branch, newest first, to the most recent commit
// whose version of R-8's text above the Sign-off line differs from the version before it (a
// commit that only touched the Sign-off line, or another ruling, is passed over), and returns
// the merged_at of the change that merged that commit into the default branch — the latest, when more than one did. It
// never uses a commit date. No merged change behind that commit is a refusal; every read
// that fails, or a history page that ends without finding the change, is could-not-check.
//
// The walk is only as complete as the forge's path-filtered history, and that history is
// SIMPLIFIED: where a merge commit leaves the register byte-identical to one parent, the other
// parent's line — and any text change on it — is not listed. So a merge that restores an old
// register can hide the text change it reverts, and the anchor falls back to the older text's
// merge. supersededAcceptance refuses the case where the authority accepted the newer text on
// the sign-off thread; an older acceptance revived with NO later acceptance on the thread is a
// known residual of this walk, and it must close before the lane gains a merge write.
func rulingTextAnchor(o *opts, fg laneForge, rr deskkit.ForgeRepo, db, current string) (time.Time, int, error) {
	cnc := func(detail string, err error) (time.Time, int, error) {
		return time.Time{}, 0, deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — %s", condRulingSigned, detail), err)
	}
	want, found := deskkit.AutoLaneRulingText(current, deskkit.AutoLaneRulingID)
	if !found {
		return cnc("the register carries no "+deskkit.AutoLaneRulingID+" text to date", nil)
	}
	hist, err := fg.ListFileCommits(rr, db, o.rulings, rulingHistoryLimit)
	if err != nil {
		return cnc("the rulings register's history could not be read", err)
	}
	if len(hist) == 0 {
		return cnc("the forge reports no commit that touched the rulings register", nil)
	}
	textAt := func(sha string) (string, bool, error) {
		fc, rerr := fg.ReadFile(rr, deskkit.ReadFileInput{File: o.rulings, Ref: sha})
		switch {
		case rerr != nil && deskkit.IsForgeNotFound(rerr):
			return "", false, nil
		case rerr != nil:
			return "", false, rerr
		case fc == nil || !fc.Exists:
			return "", false, nil
		}
		t, ok := deskkit.AutoLaneRulingText(string(fc.Content), deskkit.AutoLaneRulingID)
		return t, ok, nil
	}
	newest, nfound, err := textAt(hist[0].SHA)
	if err != nil {
		return cnc("the rulings register could not be read at "+short(hist[0].SHA), err)
	}
	if !nfound || newest != want {
		// The history's newest version is not the text just read at the default branch: the
		// register moved between the two reads, or the history is not the default branch's.
		return cnc("the rulings register's newest recorded version does not match the default branch's", nil)
	}
	anchorAt := -1
	for i := range hist {
		if i+1 == len(hist) {
			if len(hist) >= rulingHistoryLimit {
				return cnc(fmt.Sprintf("the last %d changes to the rulings register leave %s's text unchanged, "+
					"so where it last changed is past the history read", len(hist), deskkit.AutoLaneRulingID), nil)
			}
			anchorAt = i // the oldest commit that touched the register introduced this text
			break
		}
		prev, pfound, perr := textAt(hist[i+1].SHA)
		if perr != nil {
			return cnc("the rulings register could not be read at "+short(hist[i+1].SHA), perr)
		}
		if !pfound || prev != want {
			anchorAt = i
			break
		}
	}
	sha := hist[anchorAt].SHA
	nums, err := fg.ListCommitChanges(rr, sha)
	if err != nil {
		return cnc("the changes behind "+short(sha)+" could not be read", err)
	}
	var latest time.Time
	latestPR := 0
	for _, n := range nums {
		pr, rerr := fg.GetPullRequest(rr, n)
		if rerr != nil {
			return cnc(fmt.Sprintf("change #%d behind %s could not be read", n, short(sha)), rerr)
		}
		if !(pr.Merged || strings.TrimSpace(pr.MergedAt) != "") || strings.TrimSpace(pr.BaseRef) != db {
			continue
		}
		at, terr := time.Parse(time.RFC3339, strings.TrimSpace(pr.MergedAt))
		if terr != nil {
			return cnc(fmt.Sprintf("change #%d is merged but reports no readable merge time", n), nil)
		}
		if latestPR == 0 || at.After(latest) {
			latest, latestPR = at, n
		}
	}
	if latestPR == 0 {
		return time.Time{}, 0, deskkit.Refused(fmt.Sprintf(
			"refused: %s — the latest change to %s's text (%s) has no change merged into %s behind it; a text "+
				"that did not land through a merged change is never enacted", condRulingSigned,
			deskkit.AutoLaneRulingID, short(sha), db))
	}
	return latest, latestPR, nil
}
