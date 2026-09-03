package main

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/qualgen/verifier"
)

func TestEnforceEvidence_ReclassifiesActionableWithoutPointer(t *testing.T) {
	for _, class := range []verifier.VerdictClass{verifier.ClassConfirmed, verifier.ClassNeedsHuman} {
		oc := enforceEvidence(verifier.Verdict{Class: class, EvidencePointer: "   ", Rationale: "trust me"})
		if !oc.Reclassified {
			t.Errorf("%s with empty pointer not reclassified", class)
		}
		if oc.Verdict.Class != verifier.ClassCouldNotVerify {
			t.Errorf("%s with empty pointer became %q, want could-not-verify", class, oc.Verdict.Class)
		}
	}
}

func TestEnforceEvidence_KeepsWellFormedVerdicts(t *testing.T) {
	// Actionable WITH a pointer is untouched.
	oc := enforceEvidence(verifier.Verdict{Class: verifier.ClassConfirmed, EvidencePointer: "a.go:1 x"})
	if oc.Reclassified || oc.Verdict.Class != verifier.ClassConfirmed {
		t.Errorf("well-formed confirmed altered: %+v", oc)
	}
	// A false-positive needs no pointer and is untouched.
	oc = enforceEvidence(verifier.Verdict{Class: verifier.ClassFalsePositive, Rationale: "intentional"})
	if oc.Reclassified || oc.Verdict.Class != verifier.ClassFalsePositive {
		t.Errorf("false-positive altered: %+v", oc)
	}
}

// errVerifier always fails, to prove a verifier error becomes could-not-verify.
type errVerifier struct{}

func (errVerifier) Verify(verifier.Suspect, verifier.ContextPack) (verifier.Verdict, error) {
	return verifier.Verdict{}, errors.New("agent unavailable")
}

func TestVerifySuspects_VerifierErrorIsCouldNotVerify(t *testing.T) {
	suspects := []verifier.Suspect{{Fingerprint: "fp1", Category: "dead-code", File: "a.go", LineStart: 1}}
	out, err := verifySuspects(t.TempDir(), errVerifier{}, suspects)
	if err != nil {
		t.Fatalf("verifySuspects returned error: %v", err)
	}
	if len(out) != 1 || out[0].Verdict.Class != verifier.ClassCouldNotVerify {
		t.Fatalf("verifier error not recorded as could-not-verify: %+v", out)
	}
	if out[0].Verdict.Fingerprint != "fp1" {
		t.Errorf("fingerprint not stamped on error verdict")
	}
}

func TestVerifySuspects_NoVerifierWithSuspectsErrors(t *testing.T) {
	_, err := verifySuspects(t.TempDir(), nil, []verifier.Suspect{{Fingerprint: "fp"}})
	if err == nil {
		t.Errorf("expected an error when suspects need verifying but no verifier is configured")
	}
	// No suspects + nil verifier is fine.
	if _, err := verifySuspects(t.TempDir(), nil, nil); err != nil {
		t.Errorf("nil verifier with no suspects should not error: %v", err)
	}
}
