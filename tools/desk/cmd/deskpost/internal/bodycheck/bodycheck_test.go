package bodycheck

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A clean, well-formed review body (H2 heading + verdict line) that must PASS.
const cleanReview = `## Review

Checked the Verify table; values reconcile and the settlement choice fetches
PublishedPrice at commit a1b2c3. No blockers.

Verdict: approve
`

const cleanComment = `Thanks — re-reviewed the delta; the off-by-one in the cursor is fixed.`

// forty/sixty-four char lowercase-hex git SHAs must PASS the secret scan (they are the
// exempted case — hex SHAs appear constantly in review text).
const sha40 = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
const sha64 = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4e5f6a7b8c9d0e1f2"

func mustRefuse(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected refusal, got nil", what)
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("%s: expected ExitRefused (5), got %v (code %d)", what, err, deskkit.ExitCodeOf(err))
	}
}

func mustPass(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: expected pass, got %v", what, err)
	}
}

// TestReviewSecretScanBothDirections exercises EVERY refusal pattern in the facts
// (delegated to deskkit.BodyCheck) with a positive (refused) case, and a clean-body
// negative (passes) case — deliverable. Because the review validator runs the
// secret scan first, each secret pattern is embedded in an otherwise-valid review body
// so we prove the scan (not the structure check) is what refuses.
func TestReviewSecretScanBothDirections(t *testing.T) {
	valid := "## Review\n\n%s\n\nVerdict: approve\n"
	fill := func(secret string) []byte { return []byte(strings.Replace(valid, "%s", secret, 1)) }

	refusals := []struct {
		name   string
		secret string
	}{
		{"ghp token", "ghp_" + strings.Repeat("A", 36)},
		{"github_pat token", "github" + "_pat_" + strings.Repeat("B", 30)},
		{"ghs token", "ghs_" + strings.Repeat("C", 36)},
		{"gho token", "gho_" + strings.Repeat("D", 36)},
		{"aws key id", "AKIA" + "ABCDEFGHIJKLMNOP"},
		{"pem header", "-----BEGIN RSA " + "PRIVATE KEY-----"},
		{"jwt shape", "eyJhbGciOiJI.eyJzdWIiOiIx.SflKxwRJSMeKKF2QT4"},
		// A REAL sops document block (line-anchored `sops:` key + a `mac:` metadata field)
		// still refuses. The old over-broad rule refused any body containing the bare word
		// "sops"; a bare `sops: encrypted` prose line now PASSES (see negatives below).
		{"sops document block", "sops:\n  mac: ENC[AES256_GCM,data:abc,type:str]"},
		{"ENC marker", "ENC[AES256_GCM,data:abc]"},
		{"high-entropy run", strings.Repeat("Za9", 12)}, // 36 base64ish chars, not a git SHA
	}
	for _, r := range refusals {
		t.Run("refuse/"+r.name, func(t *testing.T) {
			mustRefuse(t, Review(fill(r.secret)), r.name)
		})
	}

	// Negatives — clean review body, and git SHAs pass the scan.
	mustPass(t, Review([]byte(cleanReview)), "clean review")
	mustPass(t, Review(fill("checked at "+sha40)), "40-char git sha")
	mustPass(t, Review(fill("checked at "+sha64)), "64-char git sha")

	// Bare-word "sops" is NOT a sops document and must PASS (the fixed over-match). A prose
	// mention, a config filename, and a bare `sops:` line with no metadata field all carry
	// no secret; the old strings.Contains(s, "sops") refused all of them.
	mustPass(t, Review(fill("we manage secrets with sops (.sops.yaml, sops-gpg)")), "sops prose mention")
	mustPass(t, Review(fill("sops: encrypted at rest per the runbook")), "bare sops: prose line")
}

func TestReviewStructureRequired(t *testing.T) {
	// Missing H2 heading.
	mustRefuse(t, Review([]byte("No heading here.\n\nVerdict: approve\n")), "no h2 heading")
	// Missing verdict line.
	mustRefuse(t, Review([]byte("## Review\n\nLooks good, no issues.\n")), "no verdict line")
	// A bare wrong verdict token is not accepted.
	mustRefuse(t, Review([]byte("## Review\n\nVerdict: maybe\n")), "invalid verdict token")
	// Both present → pass; the security form is also accepted.
	mustPass(t, Review([]byte(cleanReview)), "correctness verdict")
	mustPass(t, Review([]byte("## Security review\n\nNo funds path regressions.\n\nSecurity-Review: pass\n")), "security verdict")
	mustPass(t, Review([]byte("## Security review\n\nauth bypass in the new choice.\n\nSecurity-Review: fail\n")), "security fail verdict")
}

func TestSizeCap(t *testing.T) {
	big := make([]byte, MaxBodyBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	mustRefuse(t, Comment(big), "oversized comment")
	mustRefuse(t, Review(big), "oversized review")

	atCap := []byte("## Review\n\nVerdict: approve\n")
	mustPass(t, Review(atCap), "small review under cap")
}

func TestCommentScanOnly(t *testing.T) {
	// Comments need no structure — a plain sentence passes.
	mustPass(t, Comment([]byte(cleanComment)), "clean comment")
	mustPass(t, Comment([]byte("no structure needed here at all")), "unstructured comment")
	// But the secret scan still applies to comments.
	mustRefuse(t, Comment([]byte("token ghp_"+strings.Repeat("A", 36))), "comment with token")
}

func TestHasSecurityReviewPass(t *testing.T) {
	pass := []struct {
		name string
		body string
	}{
		{"canonical", "## Security review\n\nclean.\n\nSecurity-Review: pass\n"},
		{"trailing ws", "Security-Review: pass   \n"},
		{"case-insensitive key", "security-review: pass\n"},
		{"crlf", "Security-Review: pass\r\n"},
	}
	for _, p := range pass {
		t.Run("pass/"+p.name, func(t *testing.T) {
			if !HasSecurityReviewPass(p.body) {
				t.Fatalf("expected pass line detected in %q", p.body)
			}
		})
	}
	notpass := []struct {
		name string
		body string
	}{
		{"fail verdict", "## Security review\n\nSecurity-Review: fail\n"},
		{"no verdict", "## Review\n\nVerdict: approve\n"},
		{"pass-mention-inline", "The Security-Review: pass line is required but not present verbatim here as prose."},
		{"empty", ""},
	}
	for _, n := range notpass {
		t.Run("notpass/"+n.name, func(t *testing.T) {
			if HasSecurityReviewPass(n.body) {
				t.Fatalf("did not expect a pass line in %q", n.body)
			}
		})
	}
}

// TestVerdictKindSeparatesTheTwoRequiredVerdicts pins the #220 discriminator. A
// risk-classed PR needs a correctness verdict AND a security verdict at the same head,
// and in the all-clear case both post as `--verdict approve` — so the KIND is the only
// thing that tells the two writes apart in the idempotency key.
func TestVerdictKindSeparatesTheTwoRequiredVerdicts(t *testing.T) {
	kinds := []struct {
		name string
		body string
		want string
	}{
		{"correctness approve", cleanReview, KindCorrectness},
		{"correctness request-changes", "## Review\n\nblocker in the settlement path.\n\nVerdict: request-changes\n", KindCorrectness},
		{"security pass", "## Security review\n\nno funds-path regressions.\n\nSecurity-Review: pass\n", KindSecurity},
		{"security fail", "## Security review\n\nauth bypass in the new choice.\n\nSecurity-Review: fail\n", KindSecurity},
		{"case-insensitive key", "## Security review\n\nclean.\n\nsecurity-review: pass\n", KindSecurity},
		{"leading whitespace", "## Review\n\nfine.\n\n  Verdict: approve\n", KindCorrectness},
		{"repeated same-kind line", "## Review\n\nVerdict: approve\n\nrestating it:\n\nVerdict: approve\n", KindCorrectness},
		{"other-lane line quoted, not claimed", "## Review\n\nthe security lane posted `Security-Review: pass` separately.\n\n> Security-Review: pass\n\nVerdict: approve\n", KindCorrectness},
	}
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			got, err := VerdictKind([]byte(k.body))
			if err != nil {
				t.Fatalf("VerdictKind = error %v, want %q", err, k.want)
			}
			if got != k.want {
				t.Fatalf("VerdictKind = %q, want %q", got, k.want)
			}
			// Whatever Review accepts, VerdictKind must be able to classify: a body that
			// passes the schema and then has no readable kind would be an unpostable review.
			mustPass(t, Review([]byte(k.body)), "schema for "+k.name)
		})
	}
}

// TestVerdictKindFailsClosed — an absent or ambiguous kind must REFUSE (exit 5), never
// return a blank that would collapse the idempotency key back to the flag-only form that
// merged the two verdicts (#220).
func TestVerdictKindFailsClosed(t *testing.T) {
	bad := []struct {
		name string
		body string
	}{
		{"no verdict line at all", "## Review\n\nlooks fine to me.\n"},
		{"empty body", ""},
		{"verdict token not in the fixed set", "## Review\n\nVerdict: maybe\n"},
		{"key mentioned only in prose", "## Review\n\nI would call this a Verdict: approve situation, roughly.\n"},
		{"BOTH kinds in one body", "## Review\n\nfine.\n\nVerdict: approve\n\nSecurity-Review: pass\n"},
		{"both kinds, security first", "## Security review\n\nclean.\n\nSecurity-Review: pass\n\nVerdict: approve\n"},
		{"both kinds, opposite values", "## Review\n\nVerdict: request-changes\n\nSecurity-Review: pass\n"},
	}
	for _, b := range bad {
		t.Run(b.name, func(t *testing.T) {
			got, err := VerdictKind([]byte(b.body))
			mustRefuse(t, err, b.name)
			if got != "" {
				t.Fatalf("VerdictKind returned kind %q alongside a refusal — a caller ignoring "+
					"the error must not receive a usable key", got)
			}
		})
	}
}
