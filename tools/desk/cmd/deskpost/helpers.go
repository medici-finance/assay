package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stdout/stderr are package vars so tests can capture them. Production points them at the
// process streams.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// fromErr wraps an existing *deskkit.DeskError (from bodycheck, the API layer, or the
// token mint) into a writeResult, preserving its exit code via resultForErr.
//
// Reserve this for the ACTUAL mutating call (client.postComment / postReview /
// markReadyForReview): that is the one place a failure is genuinely "the call was SENT
// and its outcome could not be confirmed", which is what lets resultForErr's default
// branch (deskkit.ResultUnverifiable) charge the outward-write budget honestly. Every
// OTHER failure in a verb's precondition chain — a GET, a token mint, a trust-gate read,
// a local CI/diff determination — never reached that call, so it must go through
// fromReadErr instead (#448).
func fromErr(verb, repo string, pr int, head string, err error) writeResult {
	return writeResult{
		verb:   verb,
		repo:   repo,
		pr:     pr,
		head:   head,
		result: resultForErr(err),
		detail: err.Error(),
		err:    err,
	}
}

// fromReadErr wraps an error from a PRE-WRITE step — a GET, a token mint, a trust-gate
// read, a body-shape check, a local CI/diff determination — into a writeResult. It is
// fromErr with exactly one difference: an error that resultForErr would classify as the
// generic deskkit.ResultUnverifiable is reclassified to deskkit.ResultUnwritten, because
// nothing this wraps has attempted — let alone landed — an outward write yet.
//
// #448: chargesBudget treats ResultUnverifiable as "may have reached the
// remote", which is the right rule for a genuinely ambiguous POST and the wrong one for a
// read that never got that far. Measured on deskpost's live audit log, 28 of 29
// `unverifiable` lines were exactly this — 24 failed GETs and 4 local CI determinations
// (ciPending / ciEmpty on a CI-required repo) — and only ONE was an actual ambiguous
// write. Every call site below this point in a verb's precondition chain uses this
// wrapper; only the terminal mutating call keeps fromErr.
//
// RateLimited/Refused/Disabled classifications pass through UNCHANGED: those are already
// either correctly non-charging (RateLimited, Disabled) or a compiled-in refusal
// (Refused), so reclassifying them here would be a no-op at best and a silent widening of
// what "no write attempted" covers at worst.
func fromReadErr(verb, repo string, pr int, head string, err error) writeResult {
	result := resultForErr(err)
	if result == deskkit.ResultUnverifiable {
		result = deskkit.ResultUnwritten
	}
	return writeResult{
		verb:   verb,
		repo:   repo,
		pr:     pr,
		head:   head,
		result: result,
		detail: err.Error(),
		err:    err,
	}
}

// withDigest records the bodyDigest on a writeResult (so refusals/noops on review/comment
// still carry the digest in the audit line).
func withDigest(wr writeResult, dig string) writeResult {
	wr.bodyDig = dig
	return wr
}

// short renders a SHA prefix for human-readable messages; the FULL sha still goes to the
// audit HeadSHA field.
//
// It is a DISPLAY helper only. It is deliberately not a comparison helper: runReview
// compares --head to the resolved head at FULL length, and the form gate below
// guarantees both sides are full SHAs before that comparison happens. Before #214 they
// were not — an 8-char --head reached the comparison, failed it, and short() then
// rendered the two sides as `f24818a2` versus `f24818a22037`, a "mismatch" report whose
// two SHAs differ only in length. That reads as a head race (re-review the new head)
// when the actual problem is the argument's form (pass the full SHA). The form gate is
// what keeps this helper from being asked to explain something it cannot.
func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// fullSHALengths are the lengths of a COMPLETE git object id: 40 for SHA-1, 64 for the
// SHA-256 object format. GitHub serves SHA-1 today; 64 is admitted because deskkit's
// secret scanner already defines a git SHA as "exactly 40 or exactly 64 lowercase hex"
// (internal/deskkit/bodycheck.go) and two different definitions of "a SHA" inside one
// tool is how a gate acquires a seam. Admitting 64 widens nothing on its own: runReview
// still requires string equality with the head GitHub actually reports, so a form this
// remote never emits cannot match anything.
var fullSHALengths = map[int]bool{40: true, 64: true}

// isFullSHA reports whether s is a complete, lowercase-hex git object id.
//
// The rule is deliberately narrow — exact length, lowercase only — because it guards a
// verdict's head pin, and the failure it prevents is silent. --head is the SHA a review
// claims to have been formed against; if it can be given in a form that never equals
// what the remote reports, the equality check downstream cannot distinguish "you
// reviewed stale code" from "you typed it short", and a reviewer told the wrong one of
// those does the wrong thing about it.
//
// Uppercase hex is REFUSED rather than normalised, for the same reason. Git and the
// GitHub API emit lowercase; an uppercased SHA is byte-unequal to the resolved head, so
// accepting it here would just relocate the confusing mismatch report one step later.
// Case-folding it instead would be the fail-OPEN choice: it would silently repair an
// argument nothing legitimate produces, which is a signal worth keeping.
func isFullSHA(s string) bool {
	if !fullSHALengths[len(s)] {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// headFormError names the REAL problem in the caller's own terms, so the retry is right
// the first time (#1255's third item: "surface the required SHA form in the refusal
// text"). It discriminates the abbreviated SHA — by far the most common mistake, and the
// one whose downstream symptom is most misleading — from every other malformed value.
func headFormError(head string) string {
	const want = "--head must be the FULL 40-character lowercase-hex commit SHA " +
		"(`gh pr view <N> --json headRefOid -q .headRefOid`)"
	if len(head) < 40 && len(head) > 0 && isHexish(head) {
		return fmt.Sprintf("--head %q is an ABBREVIATED SHA (%d chars) — %s. "+
			"A short SHA can never equal the head GitHub reports, so the verdict would "+
			"be refused as a head mismatch rather than for the reason it actually failed.",
			head, len(head), want)
	}
	return fmt.Sprintf("--head %q is not a commit SHA — %s.", head, want)
}

// isHexish reports whether s is hex in either case — used ONLY to pick the more
// specific error message above. It is never a gate: isFullSHA is.
func isHexish(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return len(s) > 0
}
