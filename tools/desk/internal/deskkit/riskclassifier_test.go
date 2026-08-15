package deskkit

import (
	"strings"
	"testing"
)

// The seam's load-bearing property (Task 4).
//
// Cutting a classifier seam under a security gate creates one new way to break it
// that no existing table can see: a fail-closed precondition sliding BEHIND the
// dispatch, where a plug-in gets to answer it. The tables in riskpath_test.go and
// riskpathratified_test.go would stay green through that move — the pattern
// classifier fails closed on those inputs too, so the answers would not change
// until a second classifier arrived and answered false.
//
// So the assertion here is not about the ANSWER, it is about REACHABILITY: on a
// mechanism-side input, no classifier runs at all. The stub panics, so a dispatch
// that should not have happened is a test failure rather than a coincidence.

// panicClassifier fails the test if it is ever consulted. It deliberately does not
// return a value: reaching it at all is the defect.
type panicClassifier struct{}

func (panicClassifier) Classify(repo string, changedFiles []string) (bool, string) {
	panic("a classifier was consulted on a mechanism-side fail-closed input: " +
		"repo=" + repo + " files=" + strings.Join(changedFiles, ",") + " — " +
		"the fail-closed enumeration must short-circuit BEFORE dispatch, or a future " +
		"classifier gets a vote on whether an unknown/public repo or an unreadable " +
		"diff is risk-classed")
}

// sentinelClassifier is the positive control: it proves dispatch DOES happen on an
// input that clears the mechanism, so the panic assertions above are discriminating
// rather than being satisfied by a seam that never dispatches at all.
type sentinelClassifier struct {
	classed bool
	reason  string
	calls   *int
}

func (s sentinelClassifier) Classify(string, []string) (bool, string) {
	*s.calls++
	return s.classed, s.reason
}

// seamRoster gives one private, in-set repo (clears every precondition), one public
// repo, and one whose visibility is unstated.
func seamRoster(t *testing.T) {
	t.Helper()
	r := goldenRoster()
	r[EnvAllowedRepos] = strings.Join([]string{
		"acme/private-widget:ci:private",
		"acme/open-widget:ci:public",
		"acme/unstated-widget:ci",
	}, ",")
	withRoster(t, r)
}

// TestSeamFailClosedShortCircuitsBeforeClassifier — the four mechanism-side
// preconditions are decided without consulting a classifier.
func TestSeamFailClosedShortCircuitsBeforeClassifier(t *testing.T) {
	seamRoster(t)

	cases := []struct {
		name       string
		repo       string
		files      []string
		wantReason string
	}{
		{
			"repo outside the allowed set", "attacker/private-widget", []string{"README.md"},
			"repo is outside the fixed desk set — no policy, fail closed",
		},
		{
			"empty repo string", "", []string{"README.md"},
			"repo is outside the fixed desk set — no policy, fail closed",
		},
		{
			"public repo", "acme/open-widget", []string{"README.md"},
			"PUBLIC repo — every PR on a public repo is risk-classed",
		},
		{
			"visibility unstated", "acme/unstated-widget", []string{"README.md"},
			"repo visibility is not stated in the compiled-in policy — fail closed",
		},
		{
			"nil changed-file list", "acme/private-widget", nil,
			"no changed files could be read for this PR — fail closed",
		},
		{
			"empty changed-file list", "acme/private-widget", []string{},
			"no changed files could be read for this PR — fail closed",
		},
		{
			"blank path entry", "acme/private-widget", []string{""},
			"touches a security path for acme/private-widget",
		},
		{
			"whitespace path entry", "acme/private-widget", []string{"   "},
			"touches a security path for acme/private-widget",
		},
		{
			"blank entry among clean entries", "acme/private-widget", []string{"README.md", ""},
			"touches a security path for acme/private-widget",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The panic, if the stub is reached, propagates and fails this subtest.
			classed, reason := riskClassify(panicClassifier{}, c.repo, c.files)
			if !classed {
				t.Fatalf("riskClassify(%q, %v) = false — a mechanism-side input must fail closed", c.repo, c.files)
			}
			if reason != c.wantReason {
				t.Fatalf("riskClassify(%q, %v) reason = %q, want %q (reason wording is part of the "+
					"behavior-identical contract)", c.repo, c.files, reason, c.wantReason)
			}
		})
	}
}

// TestSeamDispatchesPastTheMechanism is the control for the test above: an input
// that clears all four preconditions REACHES the classifier, and the classifier's
// answer — both halves of it — is what the seam returns.
func TestSeamDispatchesPastTheMechanism(t *testing.T) {
	seamRoster(t)

	for _, want := range []struct {
		classed bool
		reason  string
	}{
		{true, "stub says classed"},
		{false, notRiskClassed},
	} {
		calls := 0
		classed, reason := riskClassify(
			sentinelClassifier{classed: want.classed, reason: want.reason, calls: &calls},
			"acme/private-widget", []string{"README.md"})
		if calls != 1 {
			t.Fatalf("classifier was consulted %d times, want exactly 1 — the seam does not dispatch, "+
				"so the fail-closed panic assertions prove nothing", calls)
		}
		if classed != want.classed || reason != want.reason {
			t.Fatalf("riskClassify carried (%v, %q) out of the classifier, want (%v, %q)",
				classed, reason, want.classed, want.reason)
		}
	}
}

// TestExportedAccessorsShareOneClassifyCall pins the structural half of the
// RiskPathTriggered/RiskClassReason coupling: both are readings of the SAME
// riskClassify result for the SAME default classifier, not two walks that happen to
// agree. Re-deriving either one independently is how the two drifted apart before.
func TestExportedAccessorsShareOneClassifyCall(t *testing.T) {
	seamRoster(t)

	repos := []string{"acme/private-widget", "acme/open-widget", "acme/unstated-widget", "attacker/x", ""}
	for _, repo := range repos {
		for _, files := range riskReasonFileSets {
			wantClassed, wantReason := riskClassify(defaultRiskClassifier, repo, files)
			if got := RiskPathTriggered(repo, files); got != wantClassed {
				t.Fatalf("RiskPathTriggered(%q,%v) = %v but the seam decided %v", repo, files, got, wantClassed)
			}
			if got := RiskClassReason(repo, files); got != wantReason {
				t.Fatalf("RiskClassReason(%q,%v) = %q but the seam said %q", repo, files, got, wantReason)
			}
		}
	}
}

// TestSeamDefaultClassifier — the seam ships as the union of the compiled pattern
// classifier and the adopter-executable callout, and nothing else
// It is a source-level check rather than a behavioral
// one on purpose: a behavioral test cannot distinguish "the union has exactly
// these two members" from "the union has these two members plus a third that
// happens not to fire in this test's fixtures".
func TestSeamDefaultClassifier(t *testing.T) {
	u, ok := defaultRiskClassifier.(unionClassifier)
	if !ok {
		t.Fatalf("defaultRiskClassifier is %T, want unionClassifier — the seam must not change "+
			"which classification ships until a brief says so", defaultRiskClassifier)
	}
	if len(u.members) != 2 {
		t.Fatalf("defaultRiskClassifier union has %d members, want exactly 2 (pattern + callout)", len(u.members))
	}
	if _, ok := u.members[0].(patternClassifier); !ok {
		t.Fatalf("union member 0 is %T, want patternClassifier", u.members[0])
	}
	if _, ok := u.members[1].(calloutClassifier); !ok {
		t.Fatalf("union member 1 is %T, want calloutClassifier", u.members[1])
	}
}

// TestPatternClassifierReasonContract — an implementation that answers false MUST
// say exactly "not risk-classed", because RiskClassReason's contract is textual.
func TestPatternClassifierReasonContract(t *testing.T) {
	seamRoster(t)

	classed, reason := patternClassifier{}.Classify("acme/private-widget", []string{"README.md"})
	if classed {
		t.Fatal("fixture no longer exercises the false arm: a clean file classified")
	}
	if reason != notRiskClassed {
		t.Fatalf("patternClassifier reason on a clean diff = %q, want %q", reason, notRiskClassed)
	}

	classed, reason = patternClassifier{}.Classify("acme/private-widget", []string{compiledTrigger})
	if !classed {
		t.Fatal("fixture no longer exercises the true arm: a compiled trigger did not classify")
	}
	if reason == notRiskClassed {
		t.Fatalf("patternClassifier answered classed with reason %q — the one string that means waived", reason)
	}
}
