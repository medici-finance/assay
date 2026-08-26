package deskkit

import "testing"

// TestSopsMarkerNeedsStructuralEnvelope pins the fifth #781 shape: PROSE that merely NAMES
// the sops AES256-GCM marker while describing the scanner is not ciphertext and must not
// trip, while a real lone encrypted VALUE — the one that carries the envelope's mandatory
// `data:` field — must still refuse exactly as before.
//
// The refusal arm keys on reSopsEncVal, which is anchored on `data:` (bodycheck.go). Both
// directions are exercised through the public BodyCheck entry point, and the refuse rows use
// a 24-character payload — under the 32-char high-entropy-run threshold — so the sops arm is
// the ONLY thing that can refuse them. If the narrowing were undone (bare-substring match) a
// pass row would flip; if it went too far (full-envelope match, or dropping the arm) a refuse
// row would flip. This lives in its own file, and every marker is split across concatenation
// or kept short, because deskpr secret-scans the branch diff before it pushes and a
// contiguous marker or a 32+ char run on a scanned line would refuse the very PR that adds
// the test (#380).
//
// This is the both-directions regression pair the security floor requires for the narrowing:
// (a) the legitimate description now passes, and (b) a real lone sops value of a nearby shape
// is STILL refused.
func TestSopsMarkerNeedsStructuralEnvelope(t *testing.T) {
	// The bare marker, with NO `data:` field after it — the shape a description carries.
	marker := "ENC" + "[AES256_GCM"

	cases := []struct {
		name       string
		body       string
		wantRefuse bool
	}{
		// --- pass: prose that merely NAMES the marker (no envelope field after it) ---
		{"prose naming the marker while describing the scanner",
			"the scanner keys on an " + marker + " marker at the start of a sops value", false},
		{"code comment mentioning the marker",
			"// detects " + marker + " prefixes on encrypted values", false},
		{"marker with a closing bracket but no envelope field",
			"a bare " + marker + "] with nothing inside it", false},
		{"marker named in a sentence about ciphertext structure",
			"structural ciphertext begins with the " + marker + " token, then data/iv/tag/type", false},

		// --- refuse: a real lone encrypted VALUE still carries `data:` and still refuses.
		// 24-char payload keeps every base64 run under the entropy threshold, so ONLY the
		// sops arm can be catching these — which is what makes them a clean guard for the
		// narrowing rather than an accidental catch by the high-entropy loop. ---
		{"lone complete sops encrypted value (with data:) still refuses",
			"value: " + encVal("Zm9vYmFyZm9vYmFyZm9vYmFy"), true},
		{"partial envelope carrying only the data: field still refuses",
			"note: " + encOpen + "data:Zm9vYmFyZm9vYmFyZm9vYmFy]", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := BodyCheck([]byte(c.body))
			if c.wantRefuse {
				if !IsRefused(err) {
					t.Fatalf("BodyCheck(%q) = %v, want Refused (exit 5)", c.body, err)
				}
				if ExitCodeOf(err) != ExitRefused {
					t.Fatalf("ExitCodeOf = %d, want %d", ExitCodeOf(err), ExitRefused)
				}
			} else if err != nil {
				t.Fatalf("BodyCheck(%q) = %v, want nil (clean body)", c.body, err)
			}
		})
	}

	// Positive control for the length-preserving neutralisation change: a fully
	// sops-encrypted manifest (the sanctioned commit-at-rest artifact #778 exists to admit)
	// must STILL pass. neutraliseSopsMarkers now anchors on `data:` and carries that field in
	// its replacement, keeping the surface the same length; this asserts the sanctioned
	// document is still recognised and admitted end to end.
	if err := BodyCheck([]byte(encryptedSecretFixture)); err != nil {
		t.Fatalf("a fully sops-encrypted manifest was refused after the neutralisation "+
			"change — the sanctioned encrypted-at-rest artifact must pass: %v", err)
	}
}
