package verifier

import "testing"

func TestFixture_VerifyByMatchKeyAndCountsCalls(t *testing.T) {
	fx := NewFixture(map[string]Verdict{
		"dead-code|a.go": {Class: ClassConfirmed, EvidencePointer: "a.go:1"},
	})
	s := Suspect{Fingerprint: "fp1", Category: "dead-code", File: "a.go", LineStart: 1}

	v, err := fx.Verify(s, ContextPack{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Class != ClassConfirmed {
		t.Errorf("class = %q, want confirmed", v.Class)
	}
	// The fingerprint is stamped from the suspect, not carried by the script.
	if v.Fingerprint != "fp1" {
		t.Errorf("fingerprint = %q, want fp1", v.Fingerprint)
	}
	if fx.Calls() != 1 {
		t.Errorf("calls = %d, want 1", fx.Calls())
	}
}

func TestFixture_MissingScriptErrors(t *testing.T) {
	fx := NewFixture(map[string]Verdict{})
	if _, err := fx.Verify(Suspect{Category: "dead-code", File: "x.go"}, ContextPack{}); err == nil {
		t.Errorf("expected an error for a suspect with no scripted verdict")
	}
	// A missing script is still a Verify call — it counts.
	if fx.Calls() != 1 {
		t.Errorf("calls = %d, want 1", fx.Calls())
	}
}

func TestMatchKey(t *testing.T) {
	got := MatchKey(Suspect{Category: "dead-code", File: "pkg/a.go"})
	if got != "dead-code|pkg/a.go" {
		t.Errorf("MatchKey = %q", got)
	}
}

func TestVerdictClass_Actionable(t *testing.T) {
	if !ClassConfirmed.Actionable() || !ClassNeedsHuman.Actionable() {
		t.Errorf("confirmed/needs-human must be actionable")
	}
	if ClassFalsePositive.Actionable() || ClassCouldNotVerify.Actionable() {
		t.Errorf("false-positive/could-not-verify must not be actionable")
	}
}
