package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/writepolicy"
)

// regression: #3752
func TestHookOutcomeMapping(t *testing.T) {
	t.Run("invalid-observation", func(t *testing.T) {
		claim := writepolicy.ClaimObservation{Source: writepolicy.ClaimSource("invalid-fixture")}
		cfg := Config{SharedRoot: "/shared", ProjectDir: "/owned", Cwd: "/owned", CwdTop: "/owned", claim: &claim}
		v, out, _ := cfg.evaluate("Write", "/shared/new")
		if out.Decision != writepolicy.Unknown || out.Reason != writepolicy.InvalidObservation {
			t.Fatalf("fixture did not reach the terminal invalid observation: %+v", out)
		}
		if !v.Block || !strings.Contains(v.Reason, string(out.Reason)) {
			t.Fatalf("terminal unknown was allowed or lost its reason: %+v / %+v", v, out)
		}
	})
	t.Run("unsupported", func(t *testing.T) {
		cl := writepolicy.Classify(writepolicy.Request{Tool: "Write", Target: "/shared/new"},
			writepolicy.LexicalContext{Platform: writepolicy.POSIX, Cwd: "/owned"})
		out := writepolicy.Evaluate(writepolicy.Evaluation{Classified: cl, ClassificationID: cl.ID(),
			Policy: writepolicy.Policy{Profile: writepolicy.Profile("unsupported-fixture")}})
		if out.Decision != writepolicy.Unknown || out.Reason != writepolicy.Unsupported {
			t.Fatalf("fixture did not reach unsupported outcome: %+v", out)
		}
		v := (Config{}).renderOutcome(out, "")
		if !v.Block || !strings.Contains(v.Reason, string(out.Reason)) {
			t.Fatalf("terminal unknown was allowed or lost its reason: %+v / %+v", v, out)
		}
	})
	for _, reason := range []writepolicy.ReasonCode{writepolicy.MissingObservation, writepolicy.ManagedHold} {
		t.Run(string(reason), func(t *testing.T) {
			out := writepolicy.Outcome{Decision: writepolicy.Unknown, Reason: reason}
			v := (Config{}).renderOutcome(out, "")
			if !v.Block || !strings.Contains(v.Reason, string(reason)) {
				t.Fatalf("terminal unknown was allowed or lost its reason: %+v", v)
			}
		})
	}
	t.Run("explicit-allow-and-callout-deny", func(t *testing.T) {
		if v := (Config{}).renderOutcome(writepolicy.Outcome{Decision: writepolicy.Allow, Reason: writepolicy.NoObjection}, ""); v.Block {
			t.Fatalf("explicit allow changed: %+v", v)
		}
		v := (Config{}).renderOutcome(writepolicy.Outcome{Decision: writepolicy.Deny, Reason: writepolicy.CalloutDenied}, "fixture refusal")
		if !v.Block || v.Reason != "fixture refusal" {
			t.Fatalf("callout denial changed: %+v", v)
		}
	})
}
