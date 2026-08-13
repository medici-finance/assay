package deskkit

import "testing"

// TestImpersonatedRulingClaim_Catches pins the shapes named directly by
// this guard's instance and the Verify checklist: a "Decision (Alex, ...)"
// heading (the literal example-org/decks#16 shape), a "Ruling: ... — Alex" dash
// attribution (the Verify checklist's own example), and the first-person "I (Alex) have
// decided" form the issue also names. All fire against the fixture roster's
// ASSAY_HUMAN_LOGIN_MAP=alex:ada (rosterfixture_test.go) with NO ada account action
// anywhere near the text — proving the guard catches an agent-authored claim that
// isn't backed by any real ada comment/commit, exactly the Verify requirement.
func TestImpersonatedRulingClaim_Catches(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"literal decks#16 instance shape", `**An agent posted a comment headed "Decision (Alex, 2026-07-14)" on example-org/decks#16.**`},
		{"markdown heading form", "## Decision (Alex, 2026-07-14)\n\nWe are going with approach B."},
		{"ruling dash attribution, the Verify checklist's own example", "Ruling: ship the fallback path as the default — Alex"},
		{"ruling double-hyphen attribution", "Ruling: use the retry queue instead -- Alex"},
		{"decision colon dash attribution", "Decision: hold the release until Monday — Alex"},
		{"first person parenthetical", "I (Alex) have decided we should roll this back."},
		{"first person comma form", "I, Alex, ruled that this stays blocked."},
		{"first person decided, no 'have'", "I (Alex) decided to merge anyway."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, found := ImpersonatedRulingClaim(tc.text)
			if !found {
				t.Fatalf("ImpersonatedRulingClaim(%q) = found false, want true", tc.text)
			}
			if name != "alex" {
				t.Fatalf("ImpersonatedRulingClaim(%q) name = %q, want \"alex\"", tc.text, name)
			}
		})
	}
}

// TestImpersonatedRulingClaim_DoesNotFire pins the must-not-fire side: ordinary
// prose that mentions Alex by name, discusses a past decision, or uses the
// dispatch template's ENDORSED relaying wording must never be refused — the
// guard would be unusable (and would train workers to route around it) if a
// desk tool could no longer say "Alex" or "decision" in a body at all.
func TestImpersonatedRulingClaim_DoesNotFire(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"plain mention, no ruling label", "Alex asked us to look at this before the release."},
		{"decision word far from the name", "This decision affects three repos. Alex, could you weigh in when you have a minute?"},
		{"the endorsed relaying wording (control 2)", "Relaying Alex's direction from https://github.com/example-org/decks/issues/16#issuecomment-1: ship the fallback path as default."},
		{"decision label with no name at all", "Decision: ship the fallback path as the default."},
		{"name with no ruling/decision label nearby", "I checked with Alex and they agreed the approach looks right."},
		{"past-tense narration, not a claim", "Alex's ruling on the custody split is still pending; see #44."},
		{"empty body", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if name, found := ImpersonatedRulingClaim(tc.text); found {
				t.Fatalf("ImpersonatedRulingClaim(%q) = found true (name %q), want false", tc.text, name)
			}
		})
	}
}

// TestImpersonatedRulingClaim_UnconfiguredRosterCannotDetect pins the documented
// residual: with no roster (P1), there is no configured human name to check
// against, so even the literal instance text is NOT detected. This is the same
// fail-shape as every other roster-gated check in this package — "closed" means
// "cannot detect", not "cannot be fooled" — and must not be mistaken for a
// pass-through elsewhere in the guard.
func TestImpersonatedRulingClaim_UnconfiguredRosterCannotDetect(t *testing.T) {
	withNoRoster(t)
	if name, found := ImpersonatedRulingClaim(`Decision (Alex, 2026-07-14): shipping approach B.`); found {
		t.Fatalf("unconfigured roster: ImpersonatedRulingClaim = found true (name %q), want false (nothing configured to detect against)", name)
	}
}

// TestImpersonatedRulingClaim_NameDriven proves the guard is driven by the
// CONFIGURED human map, not a hardcoded "alex" literal: installing a roster that
// maps a different name to a different login still catches that name's claim
// and does not catch "sam" (which is not configured).
func TestImpersonatedRulingClaim_NameDriven(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001",
		EnvTrustedBotSlugs: "reviewer=assay-reviewer-app:300000004",
		EnvAllowedRepos:    "medici-finance/assay:ci:private",
		EnvHumanLoginMap:   "alex:ada",
	})
	if name, found := ImpersonatedRulingClaim(`## Decision (Alex, 2026-08-12)`); !found || name != "alex" {
		t.Fatalf("ImpersonatedRulingClaim(Decision (Alex, ...)) = %q, %v, want \"alex\", true", name, found)
	}
	if _, found := ImpersonatedRulingClaim(`## Decision (Sam, 2026-08-12)`); found {
		t.Fatal("ImpersonatedRulingClaim(Decision (Sam, ...)) = found true, but \"sam\" is not configured in this roster")
	}
}

// TestScanSurface_RefusesImpersonatedRuling is the write-gate integration test:
// ScanSurface (BodyCheck's engine, shared by deskpost/deskpr/deskfile/
// deskevidence/deskreply) must REFUSE a body carrying the impersonating shape,
// and the refusal message must name issue #45 and the offending attribution so
// a refused caller can see why without re-deriving it.
func TestScanSurface_RefusesImpersonatedRuling(t *testing.T) {
	body := []byte("## Decision (Alex, 2026-07-14)\n\nWe are going with approach B; see the discussion above.")
	err := BodyCheck(body)
	if err == nil {
		t.Fatal("BodyCheck on an impersonated-ruling body = nil error, want a refusal")
	}
	de, ok := err.(*DeskError)
	if !ok {
		t.Fatalf("BodyCheck error type = %T, want *DeskError", err)
	}
	if de.ExitCode() != ExitRefused {
		t.Fatalf("BodyCheck impersonation refusal exit = %d, want %d (ExitRefused)", de.ExitCode(), ExitRefused)
	}
	if !contains(err.Error(), "#45") || !contains(err.Error(), "ruling") && !contains(err.Error(), "RULING") {
		t.Fatalf("refusal message = %q, want it to name issue #45 and the ruling/decision claim", err.Error())
	}
}

// TestScanSurface_PassesEndorsedRelayingWording proves the guard does not brick
// the dispatch template's own prescribed escape hatch — a body that RELAYS a
// real decision with a link, rather than writing in the human's voice, must
// pass unchanged.
func TestScanSurface_PassesEndorsedRelayingWording(t *testing.T) {
	body := []byte("Relaying Alex's direction from https://github.com/example-org/decks/issues/16#issuecomment-1: ship the fallback path as default.")
	if err := BodyCheck(body); err != nil {
		t.Fatalf("BodyCheck on the endorsed relaying wording = %v, want nil", err)
	}
}

// contains is a tiny case-sensitive substring helper kept local to this test
// file to avoid importing strings twice for one call site.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
