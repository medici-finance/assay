package deskkit

// C8 — the adopter half of risk-path classification (Task 5).
//
// The shipped design rule (riskpath.go) is that every change may only ever WIDEN
// what is risk-classed, and that no env var or flag reads into the trigger sets.
// Parameterising for an adopter had to keep that rule rather than trade it away,
// so the config is ADDITIVE-ONLY: it unions on top of the compiled sets and there
// is no value syntax that drops a compiled entry.
//
// Both properties below are named tests rather than prose, because an implementer
// who lets the variable REPLACE the compiled set, or who narrows a configured set
// without changing what any run echoes, must FAIL something.

import (
	"bytes"
	"strings"
	"testing"
)

// compiledTrigger is a path covered by the UNIVERSAL compiled trigger list. It
// must classify in every configuration state — including states where the adopter
// variable names something else entirely.
const compiledTrigger = "secrets/prod/token.yaml"

// TestRiskPathTriggersConfigIsAdditiveOnly: the compiled triggers ALWAYS apply.
// Unset, set, and set to arbitrary adopter paths — a compiled trigger is
// risk-classed in all three states, and the adopter paths classify when
// configured. There is no value that removes a compiled entry.
func TestRiskPathTriggersConfigIsAdditiveOnly(t *testing.T) {
	repo := "one/private-repo"
	rosterWith := func(extra string) map[string]string {
		r := mediciRoster()
		r[EnvAllowedRepos] = repo + ":ci:private"
		if extra != "" {
			r[EnvRiskPathTriggersExtra] = extra
		}
		return r
	}

	states := []struct {
		name  string
		extra string
	}{
		{"unset", ""},
		{"set to adopter paths", "adopter/infra/,adopter/secrets/"},
		{
			// The shape an implementer reaching for "replace" would produce, and the
			// shape an admin might try in order to NARROW: naming a single unrelated
			// path. Under additive semantics it adds; it cannot subtract.
			"set to something unrelated", "totally/unrelated/",
		},
	}
	for _, st := range states {
		t.Run(st.name, func(t *testing.T) {
			withRoster(t, rosterWith(st.extra))

			if !RiskPathTriggered(repo, []string{compiledTrigger}) {
				t.Errorf("a COMPILED trigger (%s) stopped being risk-classed with %s=%q — "+
					"adopter config must be additive-only; a value that drops a compiled entry is a "+
					"waiver, and a narrowing knob on a security gate is a waiver waiting to be set",
					compiledTrigger, EnvRiskPathTriggersExtra, st.extra)
			}
			// The base list is still fully present in the reported set.
			got := strings.Join(RiskPathTriggersFor(repo), ",")
			for _, want := range baseRiskPathTriggers {
				if !strings.Contains(got, want) {
					t.Errorf("compiled base trigger %q is missing from the effective set with %s=%q: %s",
						want, EnvRiskPathTriggersExtra, st.extra, got)
				}
			}
			// A clean file is still NOT risk-classed — otherwise "always true" would
			// satisfy every assertion above and this test could not fail.
			if RiskPathTriggered(repo, []string{"README.md"}) {
				t.Errorf("a clean file was risk-classed with %s=%q — the positive control is "+
					"inverted, so the additive assertions above prove nothing",
					EnvRiskPathTriggersExtra, st.extra)
			}
		})
	}

	// The adopter paths classify when configured, and do NOT when unset — the
	// other direction of the same property.
	withRoster(t, rosterWith("adopter/infra/"))
	if !RiskPathTriggered(repo, []string{"adopter/infra/config.yaml"}) {
		t.Error("a configured adopter trigger did not classify — the variable adds nothing, so it is inert")
	}
	withRoster(t, rosterWith(""))
	if RiskPathTriggered(repo, []string{"adopter/infra/config.yaml"}) {
		t.Error("an adopter path classified with the variable UNSET — the test cannot distinguish " +
			"configured from unconfigured")
	}
}

// TestRiskPathTriggersNarrowingIsVisible is P3 applied to the configured EXTRA
// set. Removing an entry from a non-empty configured set must change the echoed
// effective-set line: config can only widen the COMPILED set, but a configured
// set is still narrowable, and a narrowing that leaves no trace in run output is
// the reviewability this brief must not trade away.
func TestRiskPathTriggersNarrowingIsVisible(t *testing.T) {
	r := mediciRoster()
	r[EnvRiskPathTriggersExtra] = "adopter/infra/,adopter/secrets/"
	withRoster(t, r)
	var wide bytes.Buffer
	EchoEffectiveConfig(&wide)
	if !strings.Contains(wide.String(), "adopter/secrets/") {
		t.Fatalf("the run echo does not carry the configured trigger set:\n%s", wide.String())
	}

	r2 := mediciRoster()
	r2[EnvRiskPathTriggersExtra] = "adopter/infra/"
	withRoster(t, r2)
	var narrow bytes.Buffer
	EchoEffectiveConfig(&narrow)

	if narrow.String() == wide.String() {
		t.Fatal("removing an entry from a non-empty ASSAY_RISK_PATH_TRIGGERS_EXTRA did not change the " +
			"run echo — a repository-settings narrowing would leave no diff, no PR and no trace")
	}
	if strings.Contains(narrow.String(), "adopter/secrets/") {
		t.Error("the echo still names the removed trigger — it is not the EFFECTIVE set")
	}
	// And the narrowing must be REAL, not merely echoed: the dropped path stops
	// classifying. An echo that changes while behaviour does not would be theatre.
	repo := "one/private-repo"
	r3 := mediciRoster()
	r3[EnvAllowedRepos] = repo + ":ci:private"
	r3[EnvRiskPathTriggersExtra] = "adopter/infra/"
	withRoster(t, r3)
	if RiskPathTriggered(repo, []string{"adopter/secrets/token.yaml"}) {
		t.Error("a trigger removed from the configured set still classifies — the echo and the " +
			"behaviour disagree")
	}
}
