package deskkit

import (
	"strings"
	"testing"
)

// addressto_test.go — the `to:<role>` desk-inbox addressee stamp shares ONE resolver with
// `raised-by` (boundRole) and ONE three-state reader (stampOf). These tests pin that
// sharing: the vocabulary the two flags accept is identical, and the reader answers the
// same three states.

// TestAddressedToLabelSharesTheRaisedByVocabulary — `--to` accepts exactly the roles
// `--raised-by` stamps, because both go through boundRole over RaisedByRoles(). A role
// bound in the roster addresses; the SKILL-file names (`pr-review-desk`) do not, the same
// as raised-by.
func TestAddressedToLabelSharesTheRaisedByVocabulary(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, role := range RaisedByRoles() {
		got, err := AddressedToLabel(role)
		if err != nil {
			t.Fatalf("AddressedToLabel(%q): %v", role, err)
		}
		if got != "to:"+role {
			t.Errorf("AddressedToLabel(%q) = %q, want to:%s", role, got, role)
		}
	}
	// A SKILL name is not a roster role — refused, with the bound set named.
	_, err := AddressedToLabel("pr-review-desk")
	if err == nil {
		t.Fatal("AddressedToLabel accepted a SKILL name as a role — the vocabulary is the roster's")
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("unbound-role exit code = %d, want %d (refused)", ExitCodeOf(err), ExitRefused)
	}
	for _, want := range []string{"reviewer", "verifier", "worker"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must enumerate the bound roles (missing %q): %s", want, err)
		}
	}
}

// TestAddressedToLabelCaseFolds — a skill writing `Worker` still addresses the canonical
// lowercase label, same as raised-by.
func TestAddressedToLabelCaseFolds(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, in := range []string{"worker", "Worker", "  WORKER  "} {
		got, err := AddressedToLabel(in)
		if err != nil {
			t.Fatalf("AddressedToLabel(%q): %v", in, err)
		}
		if got != "to:worker" {
			t.Errorf("AddressedToLabel(%q) = %q, want to:worker", in, got)
		}
	}
}

// TestAddressedToLabelUnconfiguredRosterRefuses — fail closed. No compiled fallback
// vocabulary; an unconfigured roster addresses nothing.
func TestAddressedToLabelUnconfiguredRosterRefuses(t *testing.T) {
	plantRoster(t, "")
	if _, err := AddressedToLabel("worker"); err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("AddressedToLabel on an unconfigured roster = %v, want refused", err)
	}
}

// TestAddressedToLabelEmptyRoleRefuses — a blank addressee is not a stamp.
func TestAddressedToLabelEmptyRoleRefuses(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, in := range []string{"", "   "} {
		if _, err := AddressedToLabel(in); err == nil {
			t.Errorf("AddressedToLabel(%q) succeeded — a blank role would address `to:`", in)
		}
	}
}

// TestAddressedToOfThreeStates — the reader answers the same three states RaisedByOf
// does, over the `to:` prefix.
func TestAddressedToOfThreeStates(t *testing.T) {
	cases := []struct {
		name      string
		labels    []string
		wantRole  string
		wantState RaisedByState
	}{
		{"no labels", nil, "", RaisedByUnknown},
		{"labels but no stamp", []string{"bug", "urgent"}, "", RaisedByUnknown},
		{"one addressee", []string{"bug", "to:verifier"}, "verifier", RaisedByStamped},
		{"odd case and space", []string{" To:Verifier "}, "verifier", RaisedByStamped},
		{"duplicate identical is not a conflict", []string{"to:worker", "to:worker"}, "worker", RaisedByStamped},
		{"two different addressees", []string{"to:worker", "to:reviewer"}, "", RaisedByIndeterminate},
		{"empty role half", []string{"to:"}, "", RaisedByIndeterminate},
		// A label that merely CONTAINS the prefix, or an ordinary word starting "to", is
		// not an addressee.
		{"prefix not at the start", []string{"not-to:worker"}, "", RaisedByUnknown},
		{"todo is not an addressee", []string{"todo"}, "", RaisedByUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, state := AddressedToOf(tc.labels)
			if role != tc.wantRole || state != tc.wantState {
				t.Fatalf("AddressedToOf(%v) = (%q, %v), want (%q, %v)", tc.labels, role, state, tc.wantRole, tc.wantState)
			}
		})
	}
}

// TestAddressedToPrefixPinned — the prefix is a wire format shared with the inbox
// consumers (fanoutloop, issueboard); changing it orphans every addressed issue.
func TestAddressedToPrefixPinned(t *testing.T) {
	if AddressedToPrefix != "to:" {
		t.Fatalf("AddressedToPrefix = %q, want \"to:\"", AddressedToPrefix)
	}
}
