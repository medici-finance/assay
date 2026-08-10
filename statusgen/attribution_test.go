package main

import (
	"os"
	"strings"
	"testing"
)

// attrProblems copies the shared attribution fixture tree into a temp root,
// loads the streams, and returns the runner-attribution problems.
func attrProblems(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/attribution")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return attributionProblems(streams)
}

func TestAttributionHealthyBriefPasses(t *testing.T) {
	problems := attrProblems(t)
	if hasProblem(problems, "attr/brief-01") {
		t.Errorf("healthy verified brief (dated cell, distinct verifier, independent evidence row) should raise no problems; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestAttributionBadVerifiedFormat(t *testing.T) {
	problems := attrProblems(t)
	if !hasProblem(problems, "attr/brief-02", "Verified cell must name a dated runner") {
		t.Errorf("want a bad-Verified-format problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestAttributionSelfVerificationByName(t *testing.T) {
	problems := attrProblems(t)
	if !hasProblem(problems, "attr/brief-03", "self-verification") {
		t.Errorf("want a self-verification problem when verifier token == author token; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestAttributionVerifierNamesImplementer(t *testing.T) {
	problems := attrProblems(t)
	if !hasProblem(problems, "attr/brief-04", "self-verification") {
		t.Errorf("want a self-verification problem when the Verified cell names the implementer; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestAttributionEvidenceAllImplementer(t *testing.T) {
	problems := attrProblems(t)
	if !hasProblem(problems, "attr/brief-05", "independent (non-implementer) Evidence row") {
		t.Errorf("want an evidence-independence problem when every Evidence row is implementer-attributed; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestAttributionImplementedStatusExempt(t *testing.T) {
	problems := attrProblems(t)
	if hasProblem(problems, "attr/brief-06") {
		t.Errorf("an implemented-status brief-v1 brief is out of scope for this check; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestAttributionLegacyDoneExempt(t *testing.T) {
	problems := attrProblems(t)
	if hasProblem(problems, "attr/brief-07") {
		t.Errorf("a legacy no-frontmatter done brief must be exempt; got:\n%s", strings.Join(problems, "\n"))
	}
}

// --- unit-level tests of the helpers ---

func TestAuthorToken(t *testing.T) {
	cases := []struct {
		authored string
		tok      string
		ok       bool
	}{
		{"2026-07-08 by Fable session (test)", "fable", true},
		{"2026-07-08 by Sonnet-Verifier", "sonnet-verifier", true},
		{"2026-07-08", "", false}, // bare-date authored: no " by " marker
		{"", "", false},
	}
	for _, c := range cases {
		tok, ok := authorToken(c.authored)
		if tok != c.tok || ok != c.ok {
			t.Errorf("authorToken(%q) = (%q,%v), want (%q,%v)", c.authored, tok, ok, c.tok, c.ok)
		}
	}
}

func TestSelfVerificationReason(t *testing.T) {
	if r := selfVerificationReason("2026-07-08 by Fable session", "2026-07-08 fable"); r == "" {
		t.Error("verifier token equal to author token should be flagged")
	}
	if r := selfVerificationReason("2026-07-08 by Fable session", "2026-07-08 implementer"); r == "" {
		t.Error("a verifier token containing \"implementer\" should be flagged")
	}
	if r := selfVerificationReason("2026-07-08 by Fable session", "2026-07-08 self"); r == "" {
		t.Error("a verifier token equal to \"self\" should be flagged")
	}
	if r := selfVerificationReason("2026-07-08 by Fable session", "2026-07-08 sonnet-verifier"); r != "" {
		t.Errorf("a distinct verifier token should not be flagged; got %q", r)
	}
	// No " by " marker in authored: nothing to compare against, so this
	// comparison is skipped rather than guessed at.
	if r := selfVerificationReason("2026-07-08", "2026-07-08 fable"); r != "" {
		t.Errorf("with no author token to compare, no reason should be reported; got %q", r)
	}
}

func TestEvidenceHasIndependentRow(t *testing.T) {
	implementerOnly := "| # | Command | Runner |\n|---|---|---|\n| 1 | x | implementer (Opus) |\n"
	if evidenceHasIndependentRow(implementerOnly) {
		t.Error("all-implementer rows should not count as independent")
	}
	mixed := "| # | Command | Runner |\n|---|---|---|\n| 1 | x | implementer (Opus) |\n| 2 | y | sonnet-verifier |\n"
	if !evidenceHasIndependentRow(mixed) {
		t.Error("a row with a distinct runner should count as independent")
	}
	// "(non-implementer)" asserts independence — the substring collision with
	// "implementer" must not swallow it (live phrasing in methodology 01/12).
	nonImplementerPhrasing := "| # | Command | Runner |\n|---|---|---|\n| 1 | x | sonnet verifier (non-implementer) |\n"
	if !evidenceHasIndependentRow(nonImplementerPhrasing) {
		t.Error(`a "(non-implementer)" runner cell asserts independence and must count as an independent row`)
	}
	empty := ""
	if evidenceHasIndependentRow(empty) {
		t.Error("empty evidence has no independent row")
	}
	// Two tables in one section: the first is implementer-only, the second
	// (an "independent re-run") has a distinct runner — must still be found.
	twoTables := "Implementer run:\n\n" +
		"| # | Command | Runner |\n|---|---|---|\n| 1 | x | implementer (Opus) |\n\n" +
		"Independent re-run:\n\n" +
		"| # | Command | Runner |\n|---|---|---|\n| 1 | x | opus-verifier |\n"
	if !evidenceHasIndependentRow(twoTables) {
		t.Error("an independent row in a second table should still be found")
	}
	// The header row's own "Runner" cell must never be mistaken for content.
	headerOnly := "| # | Command | Runner |\n|---|---|---|\n"
	if evidenceHasIndependentRow(headerOnly) {
		t.Error("a header-only table (no content rows) has no independent row")
	}
	// When a header names the Runner column, a trailing extra column must not
	// shift what gets read (PR #106 review, finding 3).
	extraColumn := "| # | Command | Runner | Notes |\n|---|---|---|---|\n| 1 | x | implementer (Opus) | see log |\n"
	if evidenceHasIndependentRow(extraColumn) {
		t.Error("with a named Runner column, the implementer attribution must be read from it, not from a trailing Notes cell")
	}
	extraColumnIndependent := "| # | Command | Runner | Notes |\n|---|---|---|---|\n| 1 | x | opus-verifier | ran by implementer's teammate |\n"
	if !evidenceHasIndependentRow(extraColumnIndependent) {
		t.Error("an independent runner must count even when a trailing cell mentions the implementer")
	}
}

// TestVerifierFloorFailure pins the verifier floor's membership test at the
// unit level (methodology/19). Three properties matter:
//
//  1. Membership is CAPABILITY, not price. `glm-5.2` is inexpensive to run and
//     strong; it must clear the floor. An older list held a bare `glm` and so
//     rejected every `glm-*` verifier.
//  2. Families are matched at NAME-SEGMENT boundaries, not as substrings.
//     Substring matching silently swallows unrelated names — `gemini-*` on
//     `mini`, `elite-*` on `lite` — which is the same defect class that made
//     `glm` unsafe.
//  3. A `human:` token only clears the floor when it is the RUNNER token and
//     names a known human (assay-toolkit#280). The cases carrying that property
//     are grouped at the bottom and each is paired with its opposite direction:
//     the legitimate human stamp that must still clear, next to the forgery
//     built from it that must now be caught.
func TestVerifierFloorFailure(t *testing.T) {
	cases := []struct {
		verified string
		want     bool // true = FAILS the floor
		why      string
	}{
		// Below the floor: the named weak families.
		{"2026-07-09 deepseek-verifier", true, "deepseek is a named below-floor family"},
		{"2026-07-09 sonnet-verifier", true, "sonnet is a named below-floor family"},
		{"2026-07-09 haiku-verifier", true, "haiku is a named below-floor family"},
		{"2026-07-09 gpt-4o-mini", true, "mini is a small-model family name"},
		{"2026-07-09 gemini-2.5-flash", true, "flash is a small-model family name"},
		{"2026-07-09 gemini-2.5-flash-lite", true, "lite is a small-model family name"},
		{"2026-07-09 via-deepseek", true, "the family need not lead the token"},
		{"2026-07-09 claude-sonnet-5", true, "a versioned family name still matches"},
		{"2026-07-09 sonnet5", true, "trailing version digits are stripped"},
		{"2026-07-09 SONNET-verifier", true, "matching is case-insensitive"},
		{"2026-07-09 model:sonnet", true, "a model: prefix does not hide the family"},

		// Above the floor.
		{"2026-07-09 glm-5.2-verifier", false, "glm-5.2 is cheap on price, strong on capability"},
		{"2026-07-09 glm-verifier", false, "no glm line is below the floor"},
		{"2026-07-09 opus-verifier", false, "opus is above the floor"},
		{"2026-07-09 fable-verifier", false, "fable is above the floor"},
		{"2026-07-09 gemini-verifier", false, "gemini must not trip the mini family (segment, not substring)"},
		{"2026-07-09 elite-verifier", false, "elite must not trip the lite family (segment, not substring)"},

		// ---- assay-toolkit#280: the human-token exemption, BOTH directions ----
		//
		// ACCEPT (a genuine human-run verification must still clear the floor):
		{"2026-07-09 human:ian", false, "a known human as the RUNNER token still clears the floor"},
		{"2026-07-09 human:IAN", false, "the human-name lookup is case-insensitive"},
		{"2026-07-09 human:ian (sonnet-assisted)", false, "a known human runner clears it even when a weak model is named as an assist"},
		{"2026-07-09 human:ian,", false, "trailing punctuation on the name does not break the lookup"},
		//
		// REJECT (the forgeries built from that same accept):
		{"2026-07-09 sonnet-verifier human:ian", true,
			"#280: appending a human token must NOT suppress the floor — the RUNNER is sonnet-verifier"},
		{"2026-07-09 haiku-verifier (reviewed by human:ian)", true,
			"#280: a human named anywhere but the runner slot suppresses nothing"},
		{"2026-07-09 glm-5.2-verifier human:ian", false,
			"#280 must not over-correct: the runner is above the floor, so this still clears"},
		{"2026-07-09 human:bob", true,
			"#280: an unresolvable human token fails LOUD — it would otherwise clear silently, since belowFloorRunner(\"human:bob\") is false"},
		{"2026-07-09 human:іan", true,
			"#280/#243: a homoglyph name (Cyrillic і) resolves to no known human and must not clear"},
		{"2026-07-09 superhuman:ian", false,
			"#243: superhuman: is not a human stamp, so it is judged as an ordinary runner token — and superhuman names no weak family"},
		{"2026-07-09 superhuman:sonnet", true,
			"#243: a non-stamp runner is still judged on its model family"},

		// Shape cases the floor deliberately does not own.
		{"no-date-here sonnet-verifier", false, "an undated cell is attributionProblems' to report, not the floor's"},
		{"", false, "an empty cell is not the floor's to report"},
	}

	for _, c := range cases {
		reason, failed := verifierFloorFailure(c.verified)
		if failed != c.want {
			t.Errorf("verifierFloorFailure(%q) = %v, want %v — %s", c.verified, failed, c.want, c.why)
			continue
		}
		if failed && reason == "" {
			t.Errorf("verifierFloorFailure(%q) failed the floor but returned no reason to name in the error", c.verified)
		}
		if !failed && reason != "" {
			t.Errorf("verifierFloorFailure(%q) cleared the floor but returned reason %q", c.verified, reason)
		}
	}
}

// TestVerifierFloorHumanTokenIsNotASuppressor is the regression test for
// assay-toolkit#280 stated as the property rather than as a table row: for EVERY
// below-floor runner, appending a human token to the cell must not change the
// verdict. That is the invariant the old `hasHumanReviewer` early return broke —
// it was a single `if` whose effect was to switch the whole control off from
// anywhere in the string.
func TestVerifierFloorHumanTokenIsNotASuppressor(t *testing.T) {
	for _, runner := range []string{"sonnet-verifier", "haiku-verifier", "deepseek-verifier", "gpt-4o-mini"} {
		bare := "2026-07-31 " + runner
		if _, failed := verifierFloorFailure(bare); !failed {
			t.Fatalf("precondition: %q should fail the floor", bare)
		}
		for _, suffix := range []string{" human:ian", " (human:ian)", " human:ian human:ian", " — human:ian signed"} {
			cell := bare + suffix
			if _, failed := verifierFloorFailure(cell); !failed {
				t.Errorf("verifierFloorFailure(%q) cleared the floor — a human token outside the runner slot must not suppress the check (#280)", cell)
			}
		}
	}
}

// TestHumanRunnerName covers the token-shape half directly, including the
// assay-toolkit#243 confusables: "superhuman:"/"non-human:" are not human stamps,
// and a homoglyph name parses to the empty name rather than to something that
// looks like a login.
func TestHumanRunnerName(t *testing.T) {
	cases := []struct {
		token    string
		wantName string
		wantOk   bool
	}{
		{"human:ian", "ian", true},
		{"human:IAN", "IAN", true},
		{"human:ian,", "ian", true},
		{"human:ian(x)", "ian", true},
		{"human:", "", true},
		{"human:іan", "", true}, // Cyrillic first rune -> empty ASCII name
		{"superhuman:ian", "", false},
		{"non-human:ian", "", false},
		{"sonnet-verifier", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		name, ok := humanRunnerName(c.token)
		if ok != c.wantOk || name != c.wantName {
			t.Errorf("humanRunnerName(%q) = (%q, %v), want (%q, %v)", c.token, name, ok, c.wantName, c.wantOk)
		}
	}
}
