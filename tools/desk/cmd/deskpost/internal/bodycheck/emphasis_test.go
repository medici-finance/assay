package bodycheck

import "testing"

// #232 — #219 made the security-verdict check line-anchored and
// order-sensitive, which is the right shape; but every verdict written before that install
// carried Markdown emphasis, and an anchored regexp does not match one. The gate then
// reported "no security verdict at head" on PRs that had one, and — worse — could not see
// a RETRACTION at all.
//
// The two fixtures below are opening lines of reviewer-App reviews on
// example-org/tracker#1284, both head-pinned, both
// `Security-Review: fail`, both invisible to the pre-fix parser. `real1284at76f0a802`'s
// fifth line stands in for a body whose line continues past "…works the same head in
// parallel"; no such truncation is load-bearing — the verdict line is line 3 of each, and
// driving the full bodies through these functions gives the same answer.
//
// The read-path change moves the verdict only for retractions like these two, both of which
// carry Markdown emphasis on a `Security-Review:` line; it recovers no emphasised PASS.
const (
	real1284at676846dd = "Security review — #1284 @ 676846dd\n\n" +
		"**Security-Review: fail**\n\n" +
		"Scope: this is a security-track review only. A correctness reviewer holds a separate " +
		"`CHANGES_REQUESTED` at `611f46bb` whose A–G deleted-capability inventory I have not " +
		"re-litigated; nothing below clears it.\n"
	real1284at76f0a802 = "Security review — #1284 @ 76f0a802\n\n" +
		"**Security-Review: fail**\n\n" +
		"Security track only. A correctness reviewer works the same head in parallel.\n"
)

func TestSecurityVerdictReadPathToleratesEmphasis(t *testing.T) {
	cases := []struct {
		name             string
		body             string
		wantPass, wantFa bool
	}{
		// --- the live population #232 measured ---
		{"real #1284 @676846dd", real1284at676846dd, false, true},
		{"real #1284 @76f0a802", real1284at76f0a802, false, true},

		// --- the shapes agents actually write, all in good faith ---
		{"bold pass", "## Security review\n\n**Security-Review: pass**\n", true, false},
		{"bold fail", "**Security-Review: fail**", false, true},
		{"bold key only", "**Security-Review:** pass", true, false},
		{"italic", "*Security-Review: pass*", true, false},
		{"underscore italic", "_Security-Review: fail_", false, true},
		{"code span", "`Security-Review: pass`", true, false},
		{"bold + trailing space", "  **Security-Review: pass**  ", true, false},

		// --- canonical form still works ---
		{"canonical pass", "## Security review\n\nSecurity-Review: pass\n", true, false},
		{"canonical fail", "Security-Review: fail\n", false, true},

		// --- and the things that must STILL not be a verdict ---
		{"quoted (the documented escape hatch)", "> Security-Review: pass", false, false},
		{"quoted bold", "> **Security-Review: pass**", false, false},
		{"mid-sentence mention", "This does not re-open it: `Security-Review: pass` stands.", false, false},
		{"table cell", "| lane | verdict |\n| security | Security-Review: pass |", false, false},
		{"trailing prose", "**Security-Review: fail**. See below.", false, false},
		{"list item", "- Security-Review: pass", false, false},
		{"wrong token", "**Security-Review: passed**", false, false},
		{"no verdict at all", "## Review\n\nVerdict: approve\n", false, false},

		// --- R1 (PR #399): `*` is ALSO the alternate Markdown bullet. A delete-everywhere
		// strip turned an ordinary list item in an ordinary correctness review into a
		// security PASS that was never posted. These are the exact shapes that regressed.
		{"star bullet", "* Security-Review: pass", false, false},
		{"star bullet bolded", "* **Security-Review: pass**", false, false},
		{"star bullet code span", "* `Security-Review: pass`", false, false},
		{"star bullet fail", "* Security-Review: fail", false, false},
		{"star bullet, tab after", "*\tSecurity-Review: pass", false, false},
		{"nested star bullet", "  * Security-Review: pass", false, false},
		{"numbered list item", "1. Security-Review: pass", false, false},

		// --- NB4 (PR #399): emphasis characters INSIDE the words are not emphasis, and
		// removing them would let a line acquire a keyword it never contained.
		{"star inside the key", "Security-Rev*iew: pass", false, false},
		{"underscore inside the key", "Security_Review: pass", false, false},
		{"backtick inside the value", "Security-Review: pa`ss", false, false},
		{"star inside the value", "Security-Review: pa*ss", false, false},
		{"split key across emphasis", "Security-Re**view: pass", false, false},

		// --- NB1 (PR #399): a fenced example is documentation, not a verdict — on the
		// GRANT path. The FAIL path reads it anyway: see TestFencedFailIsStillRead.
		{"pass inside a fenced block", "Show the line:\n\n```\nSecurity-Review: pass\n```\n", false, false},
		{"bold pass inside a fenced block", "```\n**Security-Review: pass**\n```\n", false, false},
		{"pass inside a tilde fence", "~~~\nSecurity-Review: pass\n~~~\n", false, false},
		{"pass inside a fence with info string", "```text\nSecurity-Review: pass\n```\n", false, false},
		{"triple-backtick span on its own line", "```Security-Review: pass```", false, false},
		{"pass after a fence closes", "```\nexample\n```\n\nSecurity-Review: pass\n", true, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HasSecurityReviewPass(c.body); got != c.wantPass {
				t.Errorf("HasSecurityReviewPass = %v, want %v", got, c.wantPass)
			}
			if got := HasSecurityReviewFail(c.body); got != c.wantFa {
				t.Errorf("HasSecurityReviewFail = %v, want %v", got, c.wantFa)
			}
		})
	}
}

// TestWritePathStaysStrict — #232's fix is READ-side only. The write gate must keep
// refusing an emphasised verdict so new artifacts stay canonical and the verdict KIND
// stays non-caller-controllable. If the strictness ever migrates to the write path,
// VerdictKind becomes partly caller-controllable and the `> ` quoting escape hatch breaks.
func TestWritePathStaysStrict(t *testing.T) {
	emphasised := []byte("## Security review\n\n**Security-Review: pass**\n")

	if err := Review(emphasised); err == nil {
		t.Error("Review accepted an EMPHASISED verdict line — the write path must stay strict " +
			"so the read-side tolerance never becomes the format")
	}
	if _, err := VerdictKind(emphasised); err == nil {
		t.Error("VerdictKind read a kind from an emphasised line — the kind must come from the " +
			"strict form only")
	}
	// The canonical form is of course still accepted, or the tolerance would be the only path.
	canonical := []byte("## Security review\n\nSecurity-Review: pass\n")
	if err := Review(canonical); err != nil {
		t.Errorf("Review rejected the canonical verdict body: %v", err)
	}
	if k, err := VerdictKind(canonical); err != nil || k != KindSecurity {
		t.Errorf("VerdictKind(canonical) = %q, %v; want %q, nil", k, err, KindSecurity)
	}
}

// TestFencedFailIsStillRead pins the ASYMMETRY between the two read paths, which is the
// only pair of choices under which a fence cannot be used to hide a verdict from the gate.
//
// The GRANT path skips fenced lines: a documentation example must not hand out a security
// pass. The BLOCK path does not: the write gate (Review/VerdictKind) is NOT fence-aware, so
// a body whose only verdict line sits inside a fence is postable — and on a
// non-risk-classed path, gate (e0) reading that `fail` is the ONLY thing left to block
// with. Make both paths fence-aware and a retraction becomes invisible; make neither and a
// fenced example grants a pass. The direction of each exclusion is what matters, and both
// point at "blocked".
func TestFencedFailIsStillRead(t *testing.T) {
	for _, body := range []string{
		"Retracting:\n\n```\nSecurity-Review: fail\n```\n",
		"```\n**Security-Review: fail**\n```\n",
		"~~~\nSecurity-Review: fail\n~~~\n",
	} {
		if !HasSecurityReviewFail(body) {
			t.Errorf("HasSecurityReviewFail(%q) = false — a retraction inside a fence must still "+
				"be read, or a fence becomes a place to hide one from gate (e0)", body)
		}
		if HasSecurityReviewPass(body) {
			t.Errorf("HasSecurityReviewPass(%q) = true — want false", body)
		}
	}
}

// TestUnwrapNeverManufacturesAKeyword is the general property behind the R1/NB4 rows above:
// unwrapping is only ever allowed to REMOVE punctuation that is wrapping something, never
// to fuse two alphanumerics together. If that ever breaks, a line that does not contain the
// marker's words in order could acquire them — including turning a `fail` into a `pass`.
//
// The unwrap machinery moved to deskkit (#408, so deskboard can reach the
// same canonical reader — a Go "internal" package is only importable by code rooted under
// its parent, and deskboard is not under cmd/deskpost/...), so the direct property test on
// unwrapEmphasis/secReviewPass/secReviewFail now lives at
// tools/desk/internal/deskkit/verdictmarker_test.go. This package still proves the same
// behaviour end to end through the exported HasSecurityReviewPass/Fail in the table above.

// TestCorrectnessVerdictLine — the correctness marker reads on the SAME terms as the
// security ones, because R2 (PR #399) was exactly the two lanes disagreeing about what a
// body IS. Every acceptance and every refusal here mirrors a row above.
func TestCorrectnessVerdictLine(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"## Review\n\nVerdict: approve\n", true},
		{"Verdict: request-changes", true},
		{"**Verdict: request-changes**", true},
		{"`Verdict: approve`", true},
		{"  **Verdict: approve**  ", true},
		{"* Verdict: approve", false},
		{"- Verdict: approve", false},
		{"> Verdict: approve", false},
		{"| lane | Verdict: approve |", false},
		{"```\nVerdict: approve\n```\n", false},
		{"Verdict: approved", false},
		{"## Security review\n\nSecurity-Review: pass\n", false},
		{"Ver*dict: approve", false},
	}
	for _, c := range cases {
		if got := CorrectnessVerdictLine(c.body); got != c.want {
			t.Errorf("CorrectnessVerdictLine(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}
