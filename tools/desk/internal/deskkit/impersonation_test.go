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
		// The genuine impersonating body — a ruling label issued at line start. (The
		// META-description of such a comment, `**An agent posted a comment headed
		// "Decision (Alex, …)" on example-org/decks#16**`, is now a DoesNotFire case: it is
		// mid-line with a citation, i.e. a report ABOUT an impersonation, not one — see the
		// verify-desk-ops-hardening narrowing.)
		{"line-initial decision claim", "Decision (Alex): we are going with approach B."},
		{"markdown heading form", "## Decision (Alex, 2026-07-14)\n\nWe are going with approach B."},
		{"ruling dash attribution, the Verify checklist's own example", "Ruling: ship the fallback path as the default — Alex"},
		{"ruling double-hyphen attribution", "Ruling: use the retry queue instead -- Alex"},
		{"decision colon dash attribution", "Decision: hold the release until Monday — Alex"},
		{"first person parenthetical", "I (Alex) have decided we should roll this back."},
		{"first person comma form", "I, Alex, ruled that this stays blocked."},
		{"first person decided, no 'have'", "I (Alex) decided to merge anyway."},
		// First person is NEVER a relay: a citation does NOT exempt it, because it is this
		// account writing in the human's voice, not pointing at where they spoke.
		{"first person WITH a citation still fires", "I (Alex) have decided, see #414."},
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
		// Narrowing (verify-desk-ops-hardening): a mid-line paren label that is not
		// positioned as issuing a ruling is a mention/description, not a claim — a report
		// ABOUT an impersonation (the old decks#16 instance-shape fixture), and a
		// forward-looking diagram node in a stream-README.
		{"meta-description of an impersonation (old decks#16 fixture)", `**An agent posted a comment headed "Decision (Alex, 2026-07-14)" on example-org/decks#16.**`},
		{"forward-looking diagram node label, mid-line no colon", "03 ruling (Alex) ──► 04 neutral core"},
		{"cited relay in paren tail (a link where they said it)", "by ruling (alex, 2026-08-03, PR #414): ship it"},
		{"cited relay with a URL in the paren", "Decision (Alex, https://github.com/example-org/decks/issues/952): invert the gate"},
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

// TestImpersonatedRulingClaim_CitedRelayAndPositionNarrowing pins the two exemptions added
// in verify-desk-ops-hardening, verbatim against the shapes that motivated them, driven by a
// CONFIGURED human name (the neutral example roster's alex:ada — never a real login). A cited
// paren/dash attribution is treated as a relay and skipped; a paren label is refused ONLY when
// it is positioned as issuing a ruling (line start, or a colon after the close paren); first
// person is refused regardless of citation.
func TestImpersonatedRulingClaim_CitedRelayAndPositionNarrowing(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001",
		EnvTrustedBotSlugs: "reviewer=assay-reviewer-app:300000004",
		EnvAllowedRepos:    "medici-finance/assay:ci:private",
		EnvHumanLoginMap:   "alex:ada",
	})

	nowPass := []struct{ name, text string }{
		{"forward-looking diagram node label", "03 ruling (Alex) ──► 04 neutral core"},
		{"cited relay, paren tail issue ref", "by ruling (alex, 2026-08-03, PR #414): x"},
		{"cited relay, URL in paren", "Decision (Alex, https://github.com/example-org/decks/issues/952): invert"},
	}
	for _, c := range nowPass {
		t.Run("pass/"+c.name, func(t *testing.T) {
			if name, found := ImpersonatedRulingClaim(c.text); found {
				t.Fatalf("ImpersonatedRulingClaim(%q) = found true (name %q), want false", c.text, name)
			}
		})
	}

	stillRefuse := []struct{ name, text string }{
		{"line-initial paren claim with colon, uncited", "Decision (Alex): flip the gate"},
		{"markdown heading paren claim", "## Ruling (Alex)"},
		{"dash attribution, uncited", "Ruling: merge now — Alex"},
		{"first person with a citation is never a relay", "I (Alex) have decided, see #414"},
	}
	for _, c := range stillRefuse {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			name, found := ImpersonatedRulingClaim(c.text)
			if !found {
				t.Fatalf("ImpersonatedRulingClaim(%q) = found false, want true", c.text)
			}
			if name != "alex" {
				t.Fatalf("ImpersonatedRulingClaim(%q) name = %q, want \"alex\"", c.text, name)
			}
		})
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

// TestScanSurfaceRulingClaim_RefusesClaim pins the ruling-claim-only entry point: it
// must refuse the impersonating shape exactly as ScanSurface does, with the same
// refusal wording, so the diff caller that routes its ADDED lines through it loses
// nothing of the guard.
func TestScanSurfaceRulingClaim_RefusesClaim(t *testing.T) {
	err := ScanSurfaceRulingClaim("added lines of branch diff vs origin/main",
		[]byte("Ruling: ship the fallback path as the default — Alex"))
	if err == nil {
		t.Fatal("ScanSurfaceRulingClaim on an impersonating line = nil, want a refusal")
	}
	if !contains(err.Error(), "RULING/DECISION claim") {
		t.Fatalf("refusal = %q, want the impersonation-guard wording", err.Error())
	}
	if !contains(err.Error(), "added lines of branch diff vs origin/main") {
		t.Fatalf("refusal = %q, want it to name the surface it was given", err.Error())
	}
	if err := ScanSurfaceRulingClaim("added lines of branch diff vs origin/main",
		[]byte("Relaying Alex's direction from https://github.com/example-org/decks/issues/16#issuecomment-1: ship it.")); err != nil {
		t.Fatalf("ScanSurfaceRulingClaim on the endorsed relaying wording = %v, want nil", err)
	}
}

// TestScanSurfaceSecrets_SkipsRulingClaimOnly pins the diff-surface split in both
// directions at once: the SAME impersonating text refuses under ScanSurface (which a
// body/title/branch surface still gets) and passes under ScanSurfaceSecrets (which a
// diff caller runs over removed and context lines the branch did not write) — because
// a deletion cannot introduce a forged ruling, and the guard's added-direction pass is
// ScanSurfaceRulingClaim's job on that surface.
func TestScanSurfaceSecrets_SkipsRulingClaimOnly(t *testing.T) {
	text := []byte("Ruling: ship the fallback path as the default — Alex")
	if err := ScanSurface("body", text); err == nil {
		t.Fatal("ScanSurface on an impersonating line = nil, want a refusal (the full scan keeps the guard)")
	}
	if err := ScanSurfaceSecrets("branch diff vs origin/main", text); err != nil {
		t.Fatalf("ScanSurfaceSecrets on the same line = %v, want nil (the ruling guard is not this entry point's arm)", err)
	}
}

// TestScanSurfaceSecrets_KeepsEveryCredentialArm proves the secrets-only entry point is
// a SKIP of one guard, not a weakening of the rest: a literal-marker credential and a
// bare high-entropy run refuse under ScanSurfaceSecrets exactly as under ScanSurface.
func TestScanSurfaceSecrets_KeepsEveryCredentialArm(t *testing.T) {
	// Both fixtures are split across concatenation for the reason recorded at
	// bodycheck_test.go's scanSecret40: written contiguously they would refuse the very
	// branch diff that carries this file.
	cases := []struct {
		name string
		text string
	}{
		{"github token prefix", "pushed with ghp_" + "0123456789abcdefABCDEF0123456789abcdef"},
		{"bare high-entropy run", "value Qx7pLk2wZt9mNc4bYf6Rh" + "Vs8Ju3XoAeG5idWn1Dz here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ScanSurfaceSecrets("branch diff vs origin/main", []byte(tc.text)); err == nil {
				t.Fatalf("ScanSurfaceSecrets(%q) = nil, want a refusal", tc.text)
			}
		})
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
