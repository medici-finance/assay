package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Object-kind resolution (#296).
//
// GitHub numbers issues and pull requests from ONE per-repo sequence, so `<number>` on
// the command line names exactly one object — but which KIND is not knowable from the
// number. deskpost used to read every number through GET /pulls/{n} alone: given an issue
// number that 404s, and the 404 surfaced as exit 6 (unverifiable). That was wrong twice
// over. The desk could not comment on an issue at all — so the register updates that are
// most of its outward writes went out through a raw `gh issue comment`, outside the write
// budget, the audit trail, the kill switch, and the body checks. And exit 6 means "the
// write may or may not have landed", which is the one thing a 404 on a lookup positively
// rules out; a caller obeying the exit-code contract retries a write that never happened.
//
// The resolution here never guesses. A 404 from /pulls/{n} is only a PROMPT to ask the
// issues endpoint, which serves both kinds and carries the documented discriminator
// (issueInfo.PullRequest, present iff the number is a PR). Every answer is a positive
// read of the object, and the three ways that read can fail are kept distinct:
//
//   - the number is an issue          → resolved, kind=issue (comment proceeds);
//   - the number exists as a PR but /pulls read failed → the ORIGINAL error, unchanged
//     (we know the kind, not the object — nothing may be inferred from that);
//   - the number is neither           → unverifiable, naming both lookups.

// targetKind is the resolved object kind of a bare `<number>` argument.
type targetKind int

const (
	kindPR targetKind = iota
	kindIssue
)

func (k targetKind) String() string {
	if k == kindIssue {
		return "issue"
	}
	return "PR"
}

// target is a resolved comment target: which kind the number turned out to be, its author
// identity (the trust-gate input, available on both kinds), and — for a PR only — the
// head SHA the comment is recorded against.
type target struct {
	kind        targetKind
	number      int
	head        string // PR only; an issue has no head, and "" is recorded as such
	authorLogin string
	authorID    int64
}

// resolveTarget resolves a bare number to the object it names. PR first: that keeps the
// PR path — every existing caller — at exactly one REST read, and only an issue number
// (or a genuinely absent one) pays for the second.
//
// One shape it does NOT defend against, because no verb in this binary ever has: a 200 with
// an EMPTY body leaves prInfo zero-valued and getPR returns nil, so this yields kind=PR with
// head "" and authorLogin "". That is pre-existing in doJSONRetry and reaches every verb,
// not just this one — and it fails closed here: an empty login is not a trusted identity
// (deskkit.TrustedAuthorID("", 0) is false) and carries no blessing, so the trust gate
// refuses before anything is written. Worth a decode-level guard one day; not one this
// function can add without pretending the problem is local to it.
func resolveTarget(c *ghClient, n int) (*target, error) {
	p, perr := c.getPR(n)
	if perr == nil {
		return &target{kind: kindPR, number: n, head: p.Head.SHA,
			authorLogin: p.User.Login, authorID: p.User.ID}, nil
	}
	if !isNotFound(perr) {
		return nil, perr // 403/5xx/transport: says nothing about the kind
	}

	iss, ierr := c.getIssue(n)
	if ierr != nil {
		if isNotFound(ierr) {
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"#%d is neither a pull request nor an issue in %s/%s (or this App installation "+
					"cannot see it) — check the number and the repo", n, c.owner, c.repo), nil)
		}
		return nil, ierr
	}
	if iss.PullRequest != nil {
		// The number IS a pull request whose /pulls read 404'd — a state we cannot explain
		// (a race with a delete, a permissions edge). Report the original failure rather
		// than commenting on it as if it were an issue.
		return nil, perr
	}
	return &target{kind: kindIssue, number: n,
		authorLogin: iss.User.Login, authorID: iss.User.ID}, nil
}

// requirePRErr upgrades a getPR failure for the PR-ONLY verbs (`review`, `ready`) when the
// number turns out to name an ISSUE. Wrong object type is a REFUSAL (exit 5) naming the
// verb that does work, not the generic exit 6: the two call for opposite responses from
// the caller — 6 says "your write may have landed, verify before retrying", while this
// says "nothing was written and nothing will be until you change the argument". It also
// stops the mistake charging the outward-write budget, which ResultUnverifiable does and
// ResultRefused does not (deskkit/ratelimit.go).
//
// It only ever fires on a 404, and only after a positive issue read: a non-404 failure and
// an unreadable number both fall through to the original error, unchanged.
func requirePRErr(c *ghClient, repo string, n int, err error) error {
	if !isNotFound(err) {
		return err
	}
	iss, ierr := c.getIssue(n)
	if ierr != nil || iss.PullRequest != nil {
		return err
	}
	return deskkit.Refused(fmt.Sprintf(
		"refused: #%d is an ISSUE, not a pull request — `review` and `ready` act only on PRs. "+
			"To comment on it: deskpost comment %s %d --body-file F", n, repo, n))
}
