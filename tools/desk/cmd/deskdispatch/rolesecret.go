package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The role-secret dispatch boundary (issue #6065, Task 4).
//
// A dispatched agent inherits the desk window's environment, so whatever a window holds, every
// agent it dispatches holds. The launcher exports a role-secret path variable
// (deskkit.RoleSecretEnvPrefix) only into worker and verifier windows. In a window that holds
// one, this verb REFUSES, before the claim and before any child process, every dispatch whose
// work it cannot show to be trusted from forge data:
//
//   - any review-kit dispatch: reviewers never hold a secret, so a holding window never
//     dispatches one;
//   - any item with no forge item behind it (a brief key, a verifier dispatch): the item-trust
//     check has no author or blessing to read for it;
//   - an issue item whose author is not trusted and that carries no current blessing
//     (deskkit.ItemTrustedEvents, with an incomplete event read counted as untrusted);
//   - a --pr item that fails the same check, or whose head commits are not all attributed to
//     roster logins. GitHub records no pusher per commit, and the forge links an account by the
//     email in the commit object, which a pusher chooses, so this is attribution, not proof of
//     who pushed. The rule is the strictest one forge data supports: the head repository is the
//     base repository, the commits the head adds over the base branch's CURRENT tip are read
//     completely, and every one of them has a linked author AND a linked committer that are
//     both trusted roster logins (by login only: the commit record carries no account id).
//     Anything unread or unlinked is untrusted. A writer on the base repository can still
//     forge that attribution.
//
// A window that holds no role-secret variable and has no role-secret mount is unaffected:
// every check here is skipped.
//
// Separately, runCmdEnv strips every role-secret variable from every child this verb starts,
// trusted or not (the closed inventory in exec.go).

// roleSecretEnviron is the environment the holding-window test reads. A seam for tests.
var roleSecretEnviron = os.Environ

// roleSecretForgeFn resolves the forge the admission reads, under the dispatcher's own role
// credential. A seam so tests read a fake forge.
var roleSecretForgeFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, error) {
	fg, _, err := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	return fg, err
}

// roleSecretMountGlob matches the fixed in-container path a bound secret is mounted at
// (containers/secrets.md §4.4). A seam for tests.
var roleSecretMountGlob = "/run/secrets/role-*"

// holdingRoleSecret reports a holding window by the variable OR by the mount: a window whose
// variable was unset still has the file readable at its fixed path, so an existing mount counts
// as holding too. An unreadable pattern matches nothing, which only ever errs toward the
// variable test alone.
func holdingRoleSecret() bool {
	if deskkit.HoldsRoleSecret(roleSecretEnviron()) {
		return true
	}
	m, _ := filepath.Glob(roleSecretMountGlob)
	return len(m) > 0
}

// roleSecretRefusalTail names BOTH triggers holdingRoleSecret reads, so a window held only by its
// mount is not pointed at a variable it does not carry.
const roleSecretRefusalTail = " — this window holds a role secret (an ASSAY_ROLE_SECRET_* variable or a /run/secrets/role-* " +
	"mount), so only work shown to be trusted from forge data is dispatched from it; nothing was claimed."

// roleSecretPrecheck refuses, before anything else runs, the dispatches a holding window never
// admits whatever the forge says: a review kit, and an item with no forge item behind it.
func roleSecretPrecheck(o dispatchOpts) error {
	if !holdingRoleSecret() {
		return nil
	}
	if reviewKit(o.kit) {
		return deskkit.Refused("refused: a review-kit dispatch from a window holding a role secret" + roleSecretRefusalTail)
	}
	if o.pr > 0 {
		return nil
	}
	if _, ok := issueNumFromItem(o.item); ok {
		return nil
	}
	return deskkit.Refused(fmt.Sprintf("refused: %q is not an issue or --pr item, so the item-trust check has no "+
		"author or blessing to read for it", o.item) + roleSecretRefusalTail)
}

// roleSecretAdmission runs the forge half of the boundary for a holding window, once the target
// repo is known and before the claim. It returns the head SHA it admitted for a --pr item ("" for
// an issue item, or when the window holds nothing), which roleSecretPinHead later compares with
// the head the resume worktree is cut from.
func roleSecretAdmission(o dispatchOpts, repo string) (string, error) {
	if !holdingRoleSecret() {
		return "", nil
	}
	if err := roleSecretPrecheck(o); err != nil {
		return "", err
	}
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return "", deskkit.Unverifiable("cannot name the target repository for the role-secret trust check"+roleSecretRefusalTail, err)
	}
	fg, err := roleSecretForgeFn(fr)
	if err != nil || fg == nil {
		return "", deskkit.Unverifiable("cannot reach the forge for the role-secret trust check"+roleSecretRefusalTail, err)
	}
	if o.pr > 0 {
		return roleSecretAdmitChange(fg, fr, o.pr)
	}
	n, _ := issueNumFromItem(o.item)
	if q, ok := itemRepoQualifier(o.item); ok && !strings.EqualFold(q, repo) {
		return "", deskkit.Refused(fmt.Sprintf("refused: item %q names repository %s, not the dispatch target %s", o.item, q, repo) + roleSecretRefusalTail)
	}
	iss, err := fg.GetIssue(fr, n)
	if err != nil || iss == nil {
		return "", deskkit.Unverifiable(fmt.Sprintf("cannot read issue #%d for the role-secret trust check", n)+roleSecretRefusalTail, err)
	}
	if iss.IsPullRequest {
		return "", deskkit.Refused(fmt.Sprintf("refused: issue item #%d is a pull request; dispatch it with --pr", n) + roleSecretRefusalTail)
	}
	tp, err := fg.IssueTrustEvents(fr, n)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf("cannot read issue #%d's trust events", n)+roleSecretRefusalTail, err)
	}
	if !roleSecretItemTrusted(iss.Author, tp) {
		return "", deskkit.Refused(fmt.Sprintf("refused: issue #%d's author %q is not trusted and it carries no current blessing", n, iss.Author.Login) + roleSecretRefusalTail)
	}
	return "", nil
}

// roleSecretItemTrusted is the existing item-trust decision, read fail-closed: an incomplete
// event read is untrusted, and a roster login whose recorded id does not match the account is
// admitted only by a current blessing.
func roleSecretItemTrusted(author deskkit.Account, tp *deskkit.TrustPayload) bool {
	if tp == nil || !tp.Complete {
		return false
	}
	if !deskkit.ItemTrustedEvents(author.Login, tp.BodyEdited, tp.Events) {
		return false
	}
	if deskkit.TrustedAuthor(author.Login) && !deskkit.TrustedAuthorID(author.Login, author.ID) {
		return deskkit.Blessed(tp.BodyEdited, tp.Events)
	}
	return true
}

func roleSecretAdmitChange(fg deskkit.Forge, fr deskkit.ForgeRepo, n int) (string, error) {
	pr, err := fg.GetPullRequest(fr, n)
	if err != nil || pr == nil {
		return "", deskkit.Unverifiable(fmt.Sprintf("cannot read #%d for the role-secret trust check", n)+roleSecretRefusalTail, err)
	}
	tp, err := fg.PRTrustEvents(fr, n)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf("cannot read #%d's trust events", n)+roleSecretRefusalTail, err)
	}
	if !roleSecretItemTrusted(pr.Author, tp) {
		return "", deskkit.Refused(fmt.Sprintf("refused: #%d's author %q is not trusted and it carries no current blessing", n, pr.Author.Login) + roleSecretRefusalTail)
	}
	if pr.CrossRepo != deskkit.CrossRepoSame {
		return "", deskkit.Refused(fmt.Sprintf("refused: #%d's head repository is not confirmed to be the base repository", n) + roleSecretRefusalTail)
	}
	if !resumeHeadRe.MatchString(pr.HeadSHA) || pr.BaseRef == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf("#%d's head or base branch is missing", n)+roleSecretRefusalTail, nil)
	}
	tip, err := fg.GetCommit(fr, pr.BaseRef)
	if err != nil || tip == nil || !resumeHeadRe.MatchString(tip.SHA) {
		return "", deskkit.Unverifiable(fmt.Sprintf("cannot read the tip of #%d's base branch", n)+roleSecretRefusalTail, err)
	}
	cmp, err := fg.CompareRefs(fr, tip.SHA, pr.HeadSHA)
	if err != nil || cmp == nil || !cmp.CommitsComplete || len(cmp.Commits) != cmp.AheadBy {
		return "", deskkit.Unverifiable(fmt.Sprintf("cannot read every commit #%d's head adds over its base branch", n)+roleSecretRefusalTail, err)
	}
	for _, c := range cmp.Commits {
		for _, who := range []struct{ field, login string }{{"author", c.AuthorLogin}, {"committer", c.CommitterLogin}} {
			if who.login == "" {
				return "", deskkit.Refused(fmt.Sprintf("refused: #%d's head commit %s has no linked %s account", n, short12(c.SHA), who.field) + roleSecretRefusalTail)
			}
			if !deskkit.TrustedAuthor(who.login) {
				return "", deskkit.Refused(fmt.Sprintf("refused: #%d's head commit %s has %s %q, which is not a roster login", n, short12(c.SHA), who.field, who.login) + roleSecretRefusalTail)
			}
		}
	}
	return pr.HeadSHA, nil
}

// roleSecretPinHead refuses a resume whose worktree would be cut from a head other than the one
// the admission read, so a push between the check and the allocation is never dispatched.
func roleSecretPinHead(admitted string, plan dispatchPlan) error {
	if admitted == "" {
		return nil
	}
	if plan.resume == nil || plan.resume.head != admitted {
		return deskkit.Refused("refused: the change's head moved after the role-secret trust check" + roleSecretRefusalTail)
	}
	return nil
}

func short12(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// itemRepoQualifier returns the `<owner>/<name>` an item key is qualified with
// (`<owner>/<name>:issue-<N>`), so the trust check never reads one repository's issue for a
// dispatch into another.
func itemRepoQualifier(item string) (string, bool) {
	s := strings.TrimSpace(item)
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", false
	}
	return s[:i], true
}
