package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantRoster writes a roster.env into a private HOME and reloads the config cache, so a
// test can exercise both the configured and the unconfigured state of the vocabulary.
func plantRoster(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
			t.Fatalf("write roster: %v", err)
		}
	}
	SetToolClass(ClassWrite)
	ReloadConfig()
	t.Cleanup(ReloadConfig)
}

const raisedByFixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,intake-loop=example-intake-loop-app:300000002,issue-loop=example-issue-loop-app:300000003,reviewer=example-reviewer-app:300000004,verifier=example-verifier-app:300000005,worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`

// TestRaisedByRolesDerivedFromRoster — the vocabulary is a PROJECTION of the roster's
// role-bindings, not a list in raisedby.go. This is the derive-or-diff property: adding a
// binding is the only way to add a stampable role.
func TestRaisedByRolesDerivedFromRoster(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	got := RaisedByRoles()
	want := []string{"desk", "intake-loop", "issue-loop", "reviewer", "verifier", "worker"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("RaisedByRoles() = %v, want %v (sorted projection of Config.RoleBots)", got, want)
	}
}

// TestRaisedByRolesTracksTheRosterRatherThanAConstant is the mutation half of the above:
// a roster with ONE binding must yield ONE role. A hardcoded six-element list would pass
// the previous test and fail this one.
func TestRaisedByRolesTracksTheRosterRatherThanAConstant(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=example-reviewer-app:300000004
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`)
	got := RaisedByRoles()
	if len(got) != 1 || got[0] != "reviewer" {
		t.Fatalf("RaisedByRoles() = %v, want [reviewer] — the vocabulary must follow the roster, "+
			"not a compiled list", got)
	}
	if _, err := RaisedByLabel("verifier"); err == nil {
		t.Fatal("RaisedByLabel(verifier) succeeded against a roster that does not bind verifier — " +
			"the write path is stamping a role nothing else in the tools recognises")
	}
}

// TestRaisedByRolesUnboundSlugIsNotAVocabularyEntry — a bot slug with no `role=` prefix
// binds no role, so it contributes nothing to stamp from.
func TestRaisedByRolesUnboundSlugIsNotAVocabularyEntry(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=example-unbound-app:300000009,reviewer=example-reviewer-app:300000004
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`)
	if got := RaisedByRoles(); len(got) != 1 || got[0] != "reviewer" {
		t.Fatalf("RaisedByRoles() = %v, want [reviewer] — an unbound slug is not a role", got)
	}
}

// TestRaisedByLabelFormatAndCaseFolding — the ONE spelling, and a role is matched
// case-insensitively so a skill writing `Reviewer` still stamps the canonical label.
func TestRaisedByLabelFormatAndCaseFolding(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, in := range []string{"reviewer", "Reviewer", "  REVIEWER  "} {
		got, err := RaisedByLabel(in)
		if err != nil {
			t.Fatalf("RaisedByLabel(%q): %v", in, err)
		}
		if got != "raised-by:reviewer" {
			t.Errorf("RaisedByLabel(%q) = %q, want raised-by:reviewer", in, got)
		}
	}
}

// TestRaisedByLabelUnknownRoleRefusesAndEnumerates — a refusal a caller cannot act on is
// half a refusal, so the bound set has to be in the message.
func TestRaisedByLabelUnknownRoleRefusesAndEnumerates(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	_, err := RaisedByLabel("pr-review-desk")
	if err == nil {
		t.Fatal("RaisedByLabel accepted a SKILL name as a role — the vocabulary is the roster's, " +
			"and a skill-named label would group by a role nothing emits")
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("exit code = %d, want %d (refused)", ExitCodeOf(err), ExitRefused)
	}
	for _, want := range []string{"reviewer", "verifier", "worker", "desk", "issue-loop", "intake-loop"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not enumerate the bound role %q: %s", want, err)
		}
	}
}

// TestRaisedByLabelUnconfiguredRosterRefuses — fail closed. No compiled fallback
// vocabulary exists, so an unconfigured deployment stamps nothing rather than stamping a
// role it does not have.
func TestRaisedByLabelUnconfiguredRosterRefuses(t *testing.T) {
	plantRoster(t, "")
	if got := RaisedByRoles(); got != nil {
		t.Errorf("RaisedByRoles() = %v on an unconfigured roster, want nil — a fallback vocabulary "+
			"is the second copy derive-or-diff exists to prevent", got)
	}
	_, err := RaisedByLabel("reviewer")
	if err == nil {
		t.Fatal("RaisedByLabel succeeded with no roster configured")
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("exit code = %d, want %d", ExitCodeOf(err), ExitRefused)
	}
}

// TestRaisedByLabelEmptyRoleRefuses — an empty --raised-by is not "unknown", it is a
// caller passing a blank. Omission is the way to say unknown.
func TestRaisedByLabelEmptyRoleRefuses(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, in := range []string{"", "   "} {
		if _, err := RaisedByLabel(in); err == nil {
			t.Errorf("RaisedByLabel(%q) succeeded — a blank role would stamp `raised-by:`", in)
		}
	}
}

// TestRaisedByZeroValueIsUnknown pins the fail-safe default. A consumer that declares a
// RaisedByState and forgets to set it must hold the NON-ANSWER, never a bucket.
func TestRaisedByZeroValueIsUnknown(t *testing.T) {
	var zero RaisedByState
	if zero != RaisedByUnknown {
		t.Fatalf("the zero RaisedByState is %v, want RaisedByUnknown — the default must be the "+
			"non-answer, or a consumer that forgets the state gets a confident wrong number", zero)
	}
	if zero.Answered() {
		t.Fatal("the zero state reports Answered() — unknown is not an answer")
	}
	if RaisedByIndeterminate.Answered() {
		t.Fatal("indeterminate reports Answered() — conflicting provenance is a could-not-check")
	}
	if !RaisedByStamped.Answered() {
		t.Fatal("stamped does not report Answered()")
	}
}

// TestRaisedByOfThreeStates is the READER contract mm/30 computes against.
func TestRaisedByOfThreeStates(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	cases := []struct {
		name      string
		labels    []string
		wantRole  string
		wantState RaisedByState
	}{
		{"no labels at all", nil, "", RaisedByUnknown},
		{"labels but no stamp", []string{"bug", "urgent"}, "", RaisedByUnknown},
		{"one stamp", []string{"bug", "raised-by:verifier"}, "verifier", RaisedByStamped},
		{"stamp with odd case and space", []string{" Raised-By:Verifier "}, "verifier", RaisedByStamped},
		{"duplicate identical stamps are not a conflict",
			[]string{"raised-by:worker", "raised-by:worker"}, "worker", RaisedByStamped},
		{"two different stamps", []string{"raised-by:worker", "raised-by:reviewer"}, "", RaisedByIndeterminate},
		{"empty role half", []string{"raised-by:"}, "", RaisedByIndeterminate},
		{"empty role half alongside a real one", []string{"raised-by:", "raised-by:desk"}, "", RaisedByIndeterminate},
		// A stamp naming a role the CURRENT roster does not bind is still a stamp: it is
		// a historical fact about that filing, and re-validating at read time rewrites
		// history as the roster changes.
		{"role no longer bound", []string{"raised-by:retired-loop"}, "retired-loop", RaisedByStamped},
		// A label that merely CONTAINS the prefix is not a stamp.
		{"prefix not at the start", []string{"not-raised-by:worker"}, "", RaisedByUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, state := RaisedByOf(tc.labels)
			if role != tc.wantRole || state != tc.wantState {
				t.Fatalf("RaisedByOf(%v) = (%q, %v), want (%q, %v)",
					tc.labels, role, state, tc.wantRole, tc.wantState)
			}
			if state != RaisedByStamped && role != "" {
				t.Fatalf("a non-answer returned a role %q — a consumer would read it as attribution", role)
			}
		})
	}
}

// TestRaisedByOfNeverInventsHumanRaised is the invariant stated as a test: NOTHING this
// package returns says "a human raised it". The only positive claim it can make is a role
// name that was literally on the issue.
func TestRaisedByOfNeverInventsHumanRaised(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, labels := range [][]string{nil, {}, {"bug"}, {"question", "needs-decision"}} {
		role, state := RaisedByOf(labels)
		if state != RaisedByUnknown {
			t.Fatalf("RaisedByOf(%v) = %v, want unknown", labels, state)
		}
		if role != "" {
			t.Fatalf("RaisedByOf(%v) attributed %q to an unstamped issue", labels, role)
		}
	}
}

// TestRaisedByPrefixPinned — the label prefix is a wire format shared with a consumer in
// another module (mm/28, mm/30). Changing it silently orphans every stamp already applied.
func TestRaisedByPrefixPinned(t *testing.T) {
	if RaisedByPrefix != "raised-by:" {
		t.Fatalf("RaisedByPrefix = %q, want \"raised-by:\" — the prefix is the wire format the "+
			"by-desk metric groups on; changing it orphans every stamp already on an issue",
			RaisedByPrefix)
	}
}

// TestRaisedByLabelCarriesNoPersonalIdentifier — a naming trap that has already fired in
// this tree: a decision class had to be renamed away from a person's handle because the
// tree-sweep blocks personal identifiers on a push. A label is published on every issue it
// touches, so the vocabulary must be role-shaped and nothing else.
func TestRaisedByLabelCarriesNoPersonalIdentifier(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, role := range RaisedByRoles() {
		label, err := RaisedByLabel(role)
		if err != nil {
			t.Fatalf("RaisedByLabel(%q): %v", role, err)
		}
		// Roles are function names. Every character has to fit the lowercase-and-dash
		// shape a role name takes; anything else is a name, an id, or a handle.
		for _, r := range strings.TrimPrefix(label, RaisedByPrefix) {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				t.Errorf("label %q carries %q — role labels are lowercase/dash only, so a human "+
					"handle or given name cannot reach a published label", label, string(r))
			}
		}
	}
}
