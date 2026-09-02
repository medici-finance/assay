package main

import (
	"strings"
	"testing"
)

// TestEnforcementStatusThreeValueSet — the generated block must carry all three
// enforcement statuses. Two, not three, is the honesty failure the spec's
// non-coverage rule (D6) exists to prevent: "not enforced" is a real state and
// eliding it tells authors a convention is a gate or a gate is a convention.
func TestEnforcementStatusThreeValueSet(t *testing.T) {
	block, err := renderEnforcementBlock(lintRuleRegistry)
	if err != nil {
		t.Fatalf("renderEnforcementBlock: %v", err)
	}
	for _, want := range []string{"| fatal |", "| advisory |", "| not enforced |"} {
		if !strings.Contains(block, want) {
			t.Errorf("generated block does not carry a %q row — the three-value status set is not fully represented", want)
		}
	}
	// And the header must NAME the boundary (task step 7): the block reports what
	// the lint enforces, not what the methodology requires.
	if !strings.Contains(block, "what the methodology") {
		t.Errorf("generated block header does not state the coverage boundary")
	}
}

// TestEnforcementStatusStableOrder — the render is a pure function (same input,
// same bytes) and its rows are sorted by tag. An unstable order would turn every
// unrelated registry change into a diff and defeat the byte-diff gate.
func TestEnforcementStatusStableOrder(t *testing.T) {
	a, err := renderEnforcementBlock(lintRuleRegistry)
	if err != nil {
		t.Fatalf("renderEnforcementBlock: %v", err)
	}
	b, err := renderEnforcementBlock(lintRuleRegistry)
	if err != nil {
		t.Fatalf("renderEnforcementBlock: %v", err)
	}
	if a != b {
		t.Fatalf("render is not deterministic: two calls produced different bytes")
	}

	// Rows sorted by tag, ascending.
	var tags []string
	for _, ln := range strings.Split(a, "\n") {
		if strings.HasPrefix(ln, "| `") {
			tag := ln[strings.Index(ln, "`")+1:]
			tag = tag[:strings.Index(tag, "`")]
			tags = append(tags, tag)
		}
	}
	if len(tags) != len(lintRuleRegistry) {
		t.Fatalf("rendered %d rule rows, registry has %d", len(tags), len(lintRuleRegistry))
	}
	for i := 1; i < len(tags); i++ {
		if tags[i-1] >= tags[i] {
			t.Errorf("rows not in ascending tag order: %q before %q", tags[i-1], tags[i])
		}
	}
}

// TestEnforcementStatusTracksTheLint — THE derivation proof. Flipping one rule's
// enforcement status in the SOURCE (the registry) changes the generated block. A
// block that did not change when the source did would be merely PRESENT, not
// derived — exactly the failure this brief closes.
func TestEnforcementStatusTracksTheLint(t *testing.T) {
	base, err := renderEnforcementBlock(lintRuleRegistry)
	if err != nil {
		t.Fatalf("renderEnforcementBlock: %v", err)
	}

	// Copy the registry and flip the one fatal rule to advisory.
	flipped := make([]LintRule, len(lintRuleRegistry))
	copy(flipped, lintRuleRegistry)
	var target string
	for i := range flipped {
		if flipped[i].Status == StatusFatal {
			target = flipped[i].Tag
			flipped[i].Status = StatusAdvisory
			break
		}
	}
	if target == "" {
		t.Fatal("registry has no fatal rule to flip — cannot prove the block tracks status")
	}

	after, err := renderEnforcementBlock(flipped)
	if err != nil {
		t.Fatalf("renderEnforcementBlock(flipped): %v", err)
	}
	if base == after {
		t.Fatal("flipping a rule's status did not change the generated block: the block is not derived from the registry")
	}
	// And the change is exactly the flipped rule's row losing its fatal status.
	if strings.Contains(after, "| `"+target+"` |") && strings.Contains(lineFor(after, target), "| fatal |") {
		t.Errorf("rule %q still reads fatal after being flipped to advisory", target)
	}
}

func lineFor(block, tag string) string {
	for _, ln := range strings.Split(block, "\n") {
		if strings.Contains(ln, "| `"+tag+"` |") {
			return ln
		}
	}
	return ""
}

// TestEnforcementStatusRegistryCoversVerifyRowRules — the visibility guard. Every
// stable rule-tag the Verify-row lint declares must be registered, so a tagged
// rule added without registration reddens the suite rather than reading as
// silently absent from the guidance (task step 1).
func TestEnforcementStatusRegistryCoversVerifyRowRules(t *testing.T) {
	registered := map[string]bool{}
	for _, r := range lintRuleRegistry {
		registered[r.Tag] = true
	}
	for _, tag := range []string{
		ruleERELiteralPipe, ruleGrepZeroCount, ruleExitSwallowed, ruleRE2LiteralPipe,
		ruleMetavar, ruleGoRunExit, ruleBREAlternation, ruleShreddedCell,
		ruleMovingRef, rulePortability,
	} {
		if !registered[tag] {
			t.Errorf("Verify-row rule %q is not in lintRuleRegistry — it would be invisible in the generated block", tag)
		}
	}
}

// TestEnforcementStatusRejectsInvalidStatus — a malformed registry (a status
// outside the three-value set, an empty tag, a duplicate) is a programming error
// and must red the render, never ship a false claim.
func TestEnforcementStatusRejectsInvalidStatus(t *testing.T) {
	cases := [][]LintRule{
		{{Tag: "x", Checks: "c", Status: EnforcementStatus("maybe")}},
		{{Tag: "", Checks: "c", Status: StatusAdvisory}},
		{{Tag: "dup", Checks: "c", Status: StatusAdvisory}, {Tag: "dup", Checks: "c", Status: StatusFatal}},
	}
	for i, rules := range cases {
		if _, err := renderEnforcementBlock(rules); err == nil {
			t.Errorf("case %d: renderEnforcementBlock accepted a malformed registry", i)
		}
	}
}
