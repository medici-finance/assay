// Package bodycheck is deskpost's structural body validator. It layers
// deskpost-specific structure on top of deskkit's shared secret scan — it does NOT
// reimplement the secret scan; the shared scan comes from deskkit. The division of labour:
//
//   - deskkit.BodyCheck  — the shared secret scan (token/PEM/JWT/AKIA/sops/high-entropy
//     runs, 40/64-char lowercase-hex git SHAs exempted). Reused verbatim; every desk
//     tool that writes to GitHub runs it.
//   - this package       — the size cap (≤16 KiB) shared by both verbs, PLUS the review
//     verdict schema (a `## ` heading + a verdict line) that ONLY review bodies carry.
//     Plain comments get size + secret scan only.
//
// The verdict schema is defined here and mirrored in tools/desk/README.md
// (verdict-format section); the security-review
// body ("## Security review" heading + "Security-Review: pass|fail" line) satisfies it.
package bodycheck

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// MaxBodyBytes is the size cap for any text written to GitHub (16 KiB). A body over
// the cap is refused (never truncated).
const MaxBodyBytes = 16 * 1024

// h2Heading matches a Markdown H2 section heading line ("## Something"). At least one is
// required in a review body (the desk's posted-review convention).
var h2Heading = regexp.MustCompile(`(?m)^##[ \t]+\S`)

// verdictLine matches the machine-checkable verdict line a review body must carry. Two
// key forms are accepted so one validator covers both the correctness verdict and the
// security verdict:
//
//	Verdict: approve | request-changes
//	Security-Review: pass | fail
//
// The key is matched case-insensitively; the value token is from the fixed set. This is
// the greppable record deskpost's ready gate (and deskboard) parse.
var verdictLine = regexp.MustCompile(`(?mi)^[ \t]*(Verdict|Security-Review):[ \t]*(approve|request-changes|pass|fail)[ \t]*$`)

// sizeAndScan applies the two checks common to every outward body: the size cap and the
// shared deskkit secret scan. Returned errors are already *deskkit.DeskError (exit 5).
func sizeAndScan(body []byte) error {
	if len(body) > MaxBodyBytes {
		return deskkit.Refused(fmt.Sprintf(
			"refused: body is %d bytes; the limit is %d (16 KiB)", len(body), MaxBodyBytes))
	}
	// Delegate the secret scan to deskkit — NOT reimplemented here.
	return deskkit.BodyCheck(body)
}

// Comment validates a plain PR comment body: size cap + secret scan only (no structure).
func Comment(body []byte) error {
	return sizeAndScan(body)
}

// Review validates a review body: size cap + secret scan, PLUS the verdict schema — a
// `## ` heading and a verdict line. A structurally-incomplete review body is refused
// (exit 5) so a verdict never lands without its machine-checkable record.
func Review(body []byte) error {
	if err := sizeAndScan(body); err != nil {
		return err
	}
	s := string(body)
	if !h2Heading.MatchString(s) {
		return deskkit.Refused(
			"refused: review body has no '## ' heading — the verdict schema requires at least " +
				"one Markdown H2 section (see tools/desk/README.md verdict-format)")
	}
	if !verdictLine.MatchString(s) {
		return deskkit.Refused(
			"refused: review body has no verdict line — it must carry a line " +
				"'Verdict: approve|request-changes' or 'Security-Review: pass|fail' " +
				"(see tools/desk/README.md verdict-format)")
	}
	return nil
}

// The two verdict KINDS a review body can carry. A risk-classed PR requires BOTH at the
// same head, so they are distinct artifacts and must be distinguishable
// by every dedup key that guards the write (#220).
const (
	KindCorrectness = "correctness" // `Verdict: approve|request-changes`
	KindSecurity    = "security"    // `Security-Review: pass|fail`
)

// verdictKeyKind maps a verdict-line KEY (lowercased) to its kind.
var verdictKeyKind = map[string]string{
	"verdict":         KindCorrectness,
	"security-review": KindSecurity,
}

// VerdictKind returns the KIND of verdict a review body carries — the discriminator that
// separates a correctness verdict from a security verdict.
//
// Why this exists (#220): both kinds are submitted with the SAME GitHub
// review event — `Security-Review: pass` and a correctness `approve` are both `--verdict
// approve` → `APPROVE`. deskpost's idempotency key was built from the verdict FLAG alone,
// so on a risk-classed PR — the one case that requires both verdicts at one head — the
// second verdict to arrive looked like a duplicate of the first and was silently dropped
// as a no-op, leaving the gate with one artifact where it requires two. The failure was
// permissive, silent, and order-dependent (it only bit when both lanes resolved to the
// same flag, i.e. the all-clear that unblocks a merge). Keying on the kind as well as the
// flag is what makes the two writes distinct.
//
// It FAILS CLOSED (Refused → exit 5) rather than falling back to a kind-less key:
//
//   - NO verdict line: unreachable through Review (which refuses first), but a caller
//     that reaches here another way must not get a key that merges kinds.
//   - BOTH kinds in one body: genuinely ambiguous — which of the two required artifacts
//     is this? Guessing either way can drop a required verdict. A body that needs to
//     QUOTE the other lane's verdict line should quote it (a leading `> ` or any prefix
//     that is not whitespace stops it matching), which is also what keeps a review body
//     from being misread by every other tool that greps these lines.
func VerdictKind(body []byte) (string, error) {
	found := map[string]bool{}
	for _, m := range verdictLine.FindAllStringSubmatch(string(body), -1) {
		if k, ok := verdictKeyKind[strings.ToLower(m[1])]; ok {
			found[k] = true
		}
	}
	switch {
	case len(found) == 1:
		for k := range found {
			return k, nil
		}
	case len(found) == 0:
		return "", deskkit.Refused(
			"refused: review body carries no verdict line, so its verdict KIND cannot be " +
				"determined — it must carry 'Verdict: approve|request-changes' or " +
				"'Security-Review: pass|fail' (see tools/desk/README.md verdict-format)")
	}
	return "", deskkit.Refused(
		"refused: review body carries BOTH a 'Verdict:' line and a 'Security-Review:' line — " +
			"a review posts exactly ONE verdict kind, and the two are separate required " +
			"artifacts on a risk-classed PR. Post them as two reviews; to reference the other " +
			"lane's line in prose, quote it (prefix with '> ') so it is not read as this " +
			"body's verdict")
}

// HasSecurityReviewPass reports whether body carries the security-review PASS verdict
// line. Used by deskpost's ready gate (e): a risk-classed PR flips
// only when an App review at the CURRENT head carries this line.
//
// DELEGATES to deskkit.HasSecurityReviewPass (#408) — deskkit is the
// canonical reader, reachable from both this package and deskboard (a Go "internal"
// package is only importable by code rooted under its parent, and deskboard is not under
// cmd/deskpost/...). This wrapper exists so deskpost's call sites (ready.go) do not need
// to change. See deskkit/verdictmarker.go for the fence/emphasis rules.
func HasSecurityReviewPass(body string) bool {
	return deskkit.HasSecurityReviewPass(body)
}

// HasSecurityReviewFail reports whether body carries the security-review FAIL verdict
// line. Without it a retraction is invisible: the ready gate only ever
// looked for `pass`, so a `pass` followed by a `fail` at the SAME head still read as
// green (#216). Callers reduce pass/fail in submission order.
//
// DELEGATES to deskkit.HasSecurityReviewFail (#408) — see
// HasSecurityReviewPass above and deskkit/verdictmarker.go for the rationale and the
// fence/emphasis rules (deliberately asymmetric between pass and fail).
func HasSecurityReviewFail(body string) bool {
	return deskkit.HasSecurityReviewFail(body)
}

// CorrectnessVerdictLine reports whether body carries a CORRECTNESS verdict line
// (`Verdict: approve|request-changes`) on the same emphasis-tolerant reading the security
// markers get.
//
// WHY IT IS EXPORTED AND WHY IT IS TOLERANT (#238, R2 on PR #399): the two
// lanes must agree on what a body IS. The correctness lane excluded a body from its
// reduction only when the STRICT VerdictKind called it security, while the security lane
// admitted it on the TOLERANT read — so `**Security-Review: pass**`, the exact form both
// live fixtures use, was security to one lane and correctness to the other, and one
// APPROVED review satisfied the correctness gate over a live blocking correctness verdict.
// The lane split is only real if both lanes classify on the same reading.
func CorrectnessVerdictLine(body string) bool {
	return deskkit.HasVerdictMarkerLine(body, correctnessMarker, deskkit.SkipFenced)
}

var correctnessMarker = regexp.MustCompile(`(?i)^[ \t]*Verdict:[ \t]*(approve|request-changes)[ \t\r]*$`)
