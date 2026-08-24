package main

import (
	"os"
	"strings"
	"testing"
)

// verifySectionProbs copies the dedicated verifysection fixture tree into a temp
// root, loads the streams, and returns the Verify-table structure problems
// (methodology/19 item 1). It exercises verifySectionProblems in isolation from
// checkBriefFiles so the presence lint has its own controlled fixture set.
func verifySectionProbs(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/verifysection")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return verifySectionProblems(streams)
}

// TestVerifySection covers methodology/19 item 1: a brief-v1 file must carry a
// `## Verify` section with at least one table row whose Command and Expect
// cells are non-empty. Presence/structure only — the lint never asserts quality
// and its error text must not imply it does.
func TestVerifySection(t *testing.T) {
	problems := verifySectionProbs(t)

	t.Run("present Verify table raises no problem", func(t *testing.T) {
		if hasProblem(problems, "brief-01-present.md") {
			t.Errorf("a present Verify table must pass; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("missing Verify section is flagged", func(t *testing.T) {
		if !hasProblem(problems, "brief-02-missing.md", "Verify") {
			t.Errorf("a missing Verify section must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("empty Verify section (no table row) is flagged", func(t *testing.T) {
		if !hasProblem(problems, "brief-03-empty.md", "Verify") {
			t.Errorf("a Verify section with no Command/Expect row must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("legacy brief (no frontmatter) is exempt", func(t *testing.T) {
		if hasProblem(problems, "brief-04-legacy.md") {
			t.Errorf("a legacy brief must be exempt from the Verify lint; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("error text names Verify only, never quality", func(t *testing.T) {
		for _, p := range problems {
			low := strings.ToLower(p)
			for _, banned := range []string{"quality", "good", "adequate", "sufficient", "meaningful"} {
				if strings.Contains(low, banned) {
					t.Errorf("Verify presence lint must not imply quality; found %q in %q", banned, p)
				}
			}
		}
	})
}

// TestVerifierFloor covers methodology/19 item 2: a risk-flagged brief
// (gate: human OR any risk answer yes) marked verified/done must be verified by
// a human or a runner ABOVE the floor. Irreversible briefs defer to the
// human-at-verified rule (via the Reviewed cell) and are exempt here.
//
// The floor's membership test is CAPABILITY, not price (belowFloorModels in
// attribution.go). These cases pin both directions of that: a genuinely weak
// family is rejected, and a strong-but-inexpensive family is admitted.
func TestVerifierFloor(t *testing.T) {
	problems := briefSchemaProblems(t)

	t.Run("below-floor runner on a risk-flagged brief is rejected", func(t *testing.T) {
		if !hasProblem(problems, "brief-40-floor-cheap-reject.md", "verifier floor") {
			t.Errorf("a below-floor verifier on a risk-flagged brief must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("a human verifier clears the floor", func(t *testing.T) {
		if hasProblem(problems, "brief-41-floor-human-accept.md", "verifier floor") {
			t.Errorf("a human verifier must clear the floor; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("a strong-tier runner clears the floor", func(t *testing.T) {
		if hasProblem(problems, "brief-42-floor-noncheap-accept.md", "verifier floor") {
			t.Errorf("an above-floor runner must clear the floor; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("irreversible briefs are exempt (deferred to the stricter rule)", func(t *testing.T) {
		if hasProblem(problems, "brief-43-floor-irreversible-exempt.md", "verifier floor") {
			t.Errorf("an irreversible brief must be exempt from the floor; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("risk-clear briefs may use a below-floor runner", func(t *testing.T) {
		if hasProblem(problems, "brief-44-floor-notflagged-accept.md", "verifier floor") {
			t.Errorf("a risk-clear brief must not be subject to the floor; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	// glm-5.2 is cheap on PRICE and strong on CAPABILITY. The floor is a
	// capability gate, so it must admit this runner — an earlier substring list
	// matched the bare string `glm` and produced false rejections of real
	// verifications.
	t.Run("a glm-5.2 runner clears the floor (cheap on price, not on capability)", func(t *testing.T) {
		if hasProblem(problems, "brief-46-floor-glm-accept.md", "verifier floor") {
			t.Errorf("a glm-5.2 verifier must clear the floor; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	// The gate must not become a no-op in the other direction: the two families
	// human:<name> named as genuinely below the floor, neither of which the old list held.
	t.Run("a deepseek runner is rejected", func(t *testing.T) {
		if !hasProblem(problems, "brief-47-floor-deepseek-reject.md", "verifier floor") {
			t.Errorf("a deepseek verifier on a risk-flagged brief must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("a sonnet runner is rejected", func(t *testing.T) {
		if !hasProblem(problems, "brief-48-floor-sonnet-reject.md", "verifier floor") {
			t.Errorf("a sonnet verifier on a risk-flagged brief must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	// ---- human-token suppression, END TO END through the lint ----
	//
	// The unit tests pin verifierFloorFailure; these pin that the LINT actually
	// changes verdict, which is the thing this guards. The pair is deliberate:
	// brief-41 above (a genuine `human:alex` runner) must still clear, or this fix
	// would just be a new false rejection wearing a security argument.
	t.Run("a human token appended to a below-floor runner does not suppress the floor", func(t *testing.T) {
		if !hasProblem(problems, "brief-78-floor-human-suffix-reject.md", "verifier floor") {
			t.Errorf("Verified cell \"2026-07-09 sonnet-verifier human:alex\" must NOT clear the floor — "+
				"appending a human token was the silent off-switch; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("a malformed human runner token (names no login) fails loud", func(t *testing.T) {
		if !hasProblem(problems, "brief-79-floor-unknown-human-reject.md", "verifier floor") {
			t.Errorf("Verified cell \"2026-07-09 human:\" names no login at all and must not clear the floor "+
				"— it would otherwise pass silently, since \"human:\" matches no model family; got:\n%s",
				strings.Join(problems, "\n"))
		}
	})
	// The leaver principle, END TO END: a well-formed human login that is NOT in
	// today's map must still CLEAR the floor. brief-80's Verified cell is
	// `2026-07-09 human:bob`; `bob` is not in the fixture map (only `alex` is), yet
	// a historical stamp must not be red-lined by a later roster change. This is the
	// headline case the fix exists for; if it regressed to a PROBLEM the fleet-wide
	// red is back.
	t.Run("a historical human runner not in today's map still clears the floor", func(t *testing.T) {
		if hasProblem(problems, "brief-80-floor-historical-human-accept.md", "verifier floor") {
			t.Errorf("Verified cell \"2026-07-09 human:bob\" names a well-formed human login not in the current "+
				"map — by the leaver principle it must CLEAR the floor, not red-line a historical board; got:\n%s",
				strings.Join(problems, "\n"))
		}
	})

	// ---- the Evidence read: the floor no longer stops at the Verified cell ----
	//
	// The gap this closes: a Verified cell naming an
	// above-floor runner cleared the cell-only floor even when the Evidence
	// section — the record of who actually ran the row — showed the row run at a
	// below-floor tier with no strong-tier re-run curing it. brief-49's cell is
	// `k3-verifier` (above floor, so the cell check passes), and its Evidence
	// records the row run only by `haiku-verifier`. This is the previously-passing
	// bad case; it must now FAIL.
	t.Run("an above-floor cell does not clear the floor when Evidence records a below-floor run", func(t *testing.T) {
		if !hasProblem(problems, "brief-49-floor-evidence-cheap-reject.md", "verifier floor") {
			t.Errorf("Verified cell \"2026-07-09 k3-verifier\" clears the cell check, but ## Evidence records "+
				"the row run only by \"haiku-verifier\" — reading Evidence, the floor must reject this; got:\n%s",
				strings.Join(problems, "\n"))
		}
	})
	// The converse must still pass: a row run cheap then genuinely re-run strong
	// (two Evidence tables, unioned per row) is CURED. Keeping this a pass is what
	// stops the Evidence read from becoming a new false rejection.
	t.Run("a below-floor Evidence run cured by a strong-tier re-run clears the floor", func(t *testing.T) {
		if hasProblem(problems, "brief-53-floor-evidence-cured-accept.md", "verifier floor") {
			t.Errorf("row 1 was re-run at strong tier (opus-verifier) in a second Evidence table — the floor is "+
				"cured per row and must not fire; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("error text names the cell, the token, and the rule", func(t *testing.T) {
		var got string
		for _, p := range problems {
			if strings.Contains(p, "brief-40-floor-cheap-reject.md") && strings.Contains(p, "verifier floor") {
				got = p
			}
		}
		if got == "" {
			t.Fatalf("no floor problem to inspect; got:\n%s", strings.Join(problems, "\n"))
		}
		for _, want := range []string{"Verified", "haiku-verifier", "methodology/19"} {
			if !strings.Contains(got, want) {
				t.Errorf("floor error text missing %q; got %q", want, got)
			}
		}
	})
	// The error must not teach the price framing that caused this bug.
	t.Run("error text does not frame the floor as a price tier", func(t *testing.T) {
		for _, p := range problems {
			if !strings.Contains(p, "verifier floor") {
				continue
			}
			if strings.Contains(strings.ToLower(p), "cheap tier") {
				t.Errorf("floor error must not describe itself as a cheap TIER (price framing); got %q", p)
			}
		}
	})
}

// TestReviewedCell covers methodology/19 item 3: at `done`, the Reviewed cell
// must be a dated runner ("YYYY-MM-DD <runner>"), the same shape the Verified
// cell already requires. The existing gate:human human:<name> rule is unchanged.
func TestReviewedCell(t *testing.T) {
	problems := briefSchemaProblems(t)

	t.Run("undated Reviewed cell at done is flagged", func(t *testing.T) {
		if !hasProblem(problems, "brief-45-reviewed-undated.md", "dated Reviewed") {
			t.Errorf("an undated Reviewed cell at done must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("a dated Reviewed cell at done passes", func(t *testing.T) {
		if hasProblem(problems, "brief-32-model-done.md", "dated Reviewed") {
			t.Errorf("a dated Reviewed cell must pass; got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("verified (not done) is out of scope for the Reviewed shape lint", func(t *testing.T) {
		if hasProblem(problems, "brief-41-floor-human-accept.md", "dated Reviewed") {
			t.Errorf("a verified row must not be subject to the done Reviewed-shape lint; got:\n%s", strings.Join(problems, "\n"))
		}
	})
}
