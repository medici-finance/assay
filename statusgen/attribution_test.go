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

// TestAttributionEvidenceEscapedPipeAllImplementer pins an
// Evidence table whose every Runner cell is implementer-attributed must still
// be flagged even when one of its rows contains an escaped pipe (`\|`) in an
// earlier cell (e.g. a `grep -ciE "arm64\|amd64"` Command). Before the fix,
// evidenceHasIndependentRow split rows with the non-escape-aware splitRow,
// so the escaped pipe shifted every later row's Runner read off its named
// column and onto a cell that never says "implementer" — the all-implementer
// table read as independently backed and the gate went green.
func TestAttributionEvidenceEscapedPipeAllImplementer(t *testing.T) {
	problems := attrProblems(t)
	if !hasProblem(problems, "attr/brief-08", "independent (non-implementer) Evidence row") {
		t.Errorf("an all-implementer Evidence table must be flagged even with an escaped pipe in an earlier row; got:\n%s", strings.Join(problems, "\n"))
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
	// shift what gets read.
	extraColumn := "| # | Command | Runner | Notes |\n|---|---|---|---|\n| 1 | x | implementer (Opus) | see log |\n"
	if evidenceHasIndependentRow(extraColumn) {
		t.Error("with a named Runner column, the implementer attribution must be read from it, not from a trailing Notes cell")
	}
	extraColumnIndependent := "| # | Command | Runner | Notes |\n|---|---|---|---|\n| 1 | x | opus-verifier | ran by implementer's teammate |\n"
	if !evidenceHasIndependentRow(extraColumnIndependent) {
		t.Error("an independent runner must count even when a trailing cell mentions the implementer")
	}
	// a Command cell containing an escaped pipe (`\|`) must
	// not shift the Runner column read on that row or any row after it. Before
	// the fix, splitRow (not escape-aware) split ON the escaped pipe too, so a
	// header-named Runner column read the WRONG cell for this row — one that
	// never says "implementer" — and an all-implementer table read as
	// independently backed.
	escapedPipeAllImplementer := "| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go test ./...` | 0 | ok | 2026-07-08 | implementer (k3) |\n" +
		"| 2 | `grep -ciE \"arm64\\|amd64\"` | 0 | 0 | 2026-07-08 | implementer (k3) |\n" +
		"| 3 | `go vet ./...` | 0 | clean | 2026-07-08 | implementer (k3) |\n"
	if evidenceHasIndependentRow(escapedPipeAllImplementer) {
		t.Error("an escaped pipe in a Command cell must not make an all-implementer table read as independent")
	}
	escapedPipeWithIndependentRow := "| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `grep -ciE \"arm64\\|amd64\"` | 0 | 0 | 2026-07-08 | implementer (k3) |\n" +
		"| 2 | `go vet ./...` | 0 | clean | 2026-07-08 | opus-verifier |\n"
	if !evidenceHasIndependentRow(escapedPipeWithIndependentRow) {
		t.Error("a genuinely independent row after an escaped-pipe row must still be found")
	}
	// An UNESCAPED pipe in a Command cell is the more
	// common author mistake (forgetting to escape) and is the sibling of the
	// escaped-pipe bug — splitRowEscaped correctly treats it as a real delimiter, so
	// it still shifts every cell after it and would mis-locate the Runner
	// column by index. Before the cell-count/row-alignment check, this row
	// read cell 5 ("2026-07-08", the Date) as Runner — no "implementer"
	// substring — so the all-implementer table went green as independently
	// backed. It must now be skipped (ragged row => not independent), not
	// misread as independent.
	unescapedPipeAllImplementer := "| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `grep foo | bar` | 0 | ok | 2026-07-08 | implementer (k3) |\n"
	if evidenceHasIndependentRow(unescapedPipeAllImplementer) {
		t.Error("an unescaped pipe in a Command cell must not make an all-implementer table read as independent (ragged row must fail closed)")
	}
	unescapedPipeWithIndependentRow := "| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `grep foo | bar` | 0 | ok | 2026-07-08 | implementer (k3) |\n" +
		"| 2 | `go vet ./...` | 0 | clean | 2026-07-08 | opus-verifier |\n"
	if !evidenceHasIndependentRow(unescapedPipeWithIndependentRow) {
		t.Error("a genuinely independent row after an unescaped-pipe (ragged) row must still be found")
	}
}

// TestVerifierFloorFailure pins the verifier floor's membership test at the
// unit level. Three properties matter:
//
//  1. Membership is CAPABILITY, not price. `glm-5.2` is inexpensive to run and
//     strong; it must clear the floor. An older list held a bare `glm` and so
//     rejected every `glm-*` verifier.
//  2. Families are matched at NAME-SEGMENT boundaries, not as substrings.
//     Substring matching silently swallows unrelated names — `gemini-*` on
//     `mini`, `elite-*` on `lite` — which is the same defect class that made
//     `glm` unsafe.
//  3. A `human:` token clears the floor only when it is the RUNNER token and
//     names a WELL-FORMED human login — by the leaver principle, whether or not
//     that login is in today's map (a historical stamp is not invalidated by a
//     later roster change; live-identity enforcement is --corroborate's job). The
//     cases carrying that property are grouped at the bottom and each is paired
//     with its opposite direction: the human stamp that must still clear (mapped
//     OR an unmapped leaver), next to the forgery — a below-floor model wearing a
//     human suffix, or a human token that names no real login — that must be caught.
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

		// ---- the human-token exemption, BOTH directions ----
		//
		// ACCEPT (a genuine human-run verification must still clear the floor):
		{"2026-07-09 human:alex", false, "a known human as the RUNNER token still clears the floor"},
		{"2026-07-09 human:ALEX", false, "the human-name lookup is case-insensitive"},
		{"2026-07-09 human:alex (sonnet-assisted)", false, "a known human runner clears it even when a weak model is named as an assist"},
		{"2026-07-09 human:alex,", false, "trailing punctuation on the name does not break the lookup"},
		//
		// REJECT (the forgeries built from that same accept):
		{"2026-07-09 sonnet-verifier human:alex", true,
			"appending a human token must NOT suppress the floor — the RUNNER is sonnet-verifier"},
		{"2026-07-09 haiku-verifier (reviewed by human:alex)", true,
			"a human named anywhere but the runner slot suppresses nothing"},
		{"2026-07-09 glm-5.2-verifier human:alex", false,
			"must not over-correct: the runner is above the floor, so this still clears"},
		//
		// THE LEAVER PRINCIPLE — a human confirmed HISTORICALLY clears the floor even
		// when no longer in TODAY's map. `bob` and `former_lead` are NOT in the current
		// map (only `alex` is) but ARE in the fixture's ASSAY_FORMER_HUMAN_LOGIN_MAP, so
		// they CLEAR: a Verified cell records who signed off THEN, and dropping a human
		// from the current roster must not retroactively red every board they ran. The
		// floor is a model-capability gate; live-identity enforcement is --corroborate's
		// and the register's job, both of which consult the CURRENT map only.
		{"2026-07-09 human:bob", false,
			"a human in the FORMER-humans map (departed) still clears — the leaver principle: a historical stamp is not invalidated by a later roster change"},
		{"2026-07-09 human:former_lead", false,
			"another departed human recorded in the former-humans map clears for the same reason"},
		//
		// NEVER-CONFIRMED NOW FAILS — a well-formed login SHAPE that was NEVER a confirmed
		// human (in neither the current nor the former map) must FAIL. This is the
		// forgery rejection #104's shape-only form dropped and this rework restores:
		// "any plausible login shape" is not proof a human acted.
		{"2026-07-09 human:carol", true,
			"a well-formed login that was never a confirmed human (not in the current map, not in the former-humans map) must FAIL — shape alone is not confirmation"},
		{"2026-07-09 human:nobody", true,
			"another plausible-but-never-confirmed login fails for the same reason"},
		//
		// FORGERY STILL CAUGHT — a human token that names NO real login fails loud, with a
		// distinct malformed-token reason. These plus the never-confirmed names above are
		// the human-token shapes the floor rejects.
		{"2026-07-09 human:іan", true,
			"a homoglyph name (Cyrillic і) yields an EMPTY login and must not clear"},
		{"2026-07-09 human:", true,
			"a human token with no name at all names no login and must not clear"},
		{"2026-07-09 human:@@@", true,
			"a human token whose name is all punctuation yields an EMPTY login and must not clear"},
		{"2026-07-09 superhuman:alex", false,
			"superhuman: is not a human stamp, so it is judged as an ordinary runner token — and superhuman names no weak family"},
		{"2026-07-09 superhuman:sonnet", true,
			"a non-stamp runner is still judged on its model family"},

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

// TestEvidenceFloorFailure pins the Evidence read that completes the floor: the
// Verified cell is one line an agent can edit, so the floor must ALSO read the
// ## Evidence section — the per-row record of who actually ran the check. A row
// whose completed Evidence runners are all below the floor poisons the gate; a
// row cured by an above-floor (or human) re-run does not.
func TestEvidenceFloorFailure(t *testing.T) {
	cases := []struct {
		name     string
		evidence string
		want     bool // true = FAILS the floor
	}{
		{
			name: "a single row run only below the floor fails",
			evidence: "| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-09 | haiku-verifier |",
			want: true,
		},
		{
			name: "an above-floor run clears",
			evidence: "| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-09 | opus-verifier |",
			want: false,
		},
		{
			name: "a below-floor run cured by a strong re-run (second table) clears",
			evidence: "| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-09 | haiku-verifier |\n\n" +
				"| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-10 | opus-verifier |",
			want: false,
		},
		{
			name: "a resolvable human runner clears the row",
			evidence: "| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-09 | human:alex |",
			want: false,
		},
		{
			name: "one uncured below-floor row among several cleared ones still fails",
			evidence: "| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-09 | opus-verifier |\n" +
				"| 2 | 2026-07-09 | sonnet-verifier |",
			want: true,
		},
		{
			name: "a below-floor row explicitly marked UNRUN is not a completed cheap run",
			evidence: "| # | Date | Runner | Result |\n|---|------|--------|--------|\n" +
				"| 1 | 2026-07-09 | haiku-verifier | UNRUN |",
			want: false,
		},
		{
			name:     "an empty Evidence section is not the floor's to fail on",
			evidence: "",
			want:     false,
		},
		{
			// False-REJECT direction: a table with no `Runner` column makes the
			// last cell (free-text output) the "runner"; a family word anywhere
			// in it must NOT segment-match and fail an otherwise-strong run.
			name: "a free-text output cell in a table with no Runner column does not poison the floor",
			evidence: "| # | Command | Exit | Key output |\n|---|---------|------|------------|\n" +
				"| 1 | `go test ./...` | 0 | all green; drafted by the sonnet worker earlier |",
			want: false,
		},
		{
			// False-ACCEPT (laundering) direction: a below-floor row in a
			// column-named table must NOT be cured by a later table that has no
			// `Runner` column, whose last cell happens to read as a strong token.
			name: "a below-floor row is not laundered clear by a later table with no Runner column",
			evidence: "| # | Date | Runner |\n|---|------|--------|\n" +
				"| 1 | 2026-07-09 | haiku-verifier |\n\n" +
				"| # | Command | Exit | Result |\n|---|---------|------|--------|\n" +
				"| 1 | `go test` | 0 | opus-verifier |",
			want: true,
		},
	}
	for _, c := range cases {
		reason, failed := evidenceFloorFailure(c.evidence)
		if failed != c.want {
			t.Errorf("%s: evidenceFloorFailure() = %v, want %v", c.name, failed, c.want)
			continue
		}
		if failed && reason == "" {
			t.Errorf("%s: failed the floor but returned no reason to name in the error", c.name)
		}
	}
}

// TestVerifierFloorHumanTokenIsNotASuppressor is the regression test for
// The exemption is stated as the property rather than as a table row: for EVERY
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
		for _, suffix := range []string{" human:alex", " (human:alex)", " human:alex human:alex", " — human:alex signed"} {
			cell := bare + suffix
			if _, failed := verifierFloorFailure(cell); !failed {
				t.Errorf("verifierFloorFailure(%q) cleared the floor — a human token outside the runner slot must not suppress the check", cell)
			}
		}
	}
}

// TestHumanRunnerName covers the token-shape half directly, including the
// Confusables: "superhuman:"/"non-human:" are not human stamps,
// and a homoglyph name parses to the empty name rather than to something that
// looks like a login.
func TestHumanRunnerName(t *testing.T) {
	cases := []struct {
		token    string
		wantName string
		wantOk   bool
	}{
		{"human:alex", "alex", true},
		{"human:ALEX", "ALEX", true},
		{"human:alex,", "alex", true},
		{"human:alex(x)", "alex", true},
		{"human:", "", true},
		{"human:іan", "", true}, // Cyrillic first rune -> empty ASCII name
		{"superhuman:alex", "", false},
		{"non-human:alex", "", false},
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
