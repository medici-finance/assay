package main

// roleapp_test.go — the role→App binding on the desktoken mint path.
//
// The binding lets N desk roles mint with M<N Apps: `<ROLE>_APP=<app-name>` (env, then
// apps.env) names the App a role mints as, and that App-name is the stem for the PEM file,
// the App ID key and the install ID key. Absent, it is `<role>-app` — byte-identical to the
// pre-binding layout.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRoleAppBindingVersionEcho — Verify rows 1 (`-run RoleApp`), 2 and 3. The --version
// config echo prints one `bindings=` line, `role=app-name` per role: a bound role shows its
// App-name, an unbound one shows the default `<role>-app`.
func TestRoleAppBindingVersionEcho(t *testing.T) {
	t.Run("bound role shows its App-name", func(t *testing.T) {
		setupTest(t)
		t.Setenv("REVIEWER_APP", "x-act")
		rc, stdout, stderr := runCap(t, []string{"--version"})
		if rc != 0 {
			t.Fatalf("--version rc = %d, want 0 (stderr: %s)", rc, stderr)
		}
		if !strings.Contains(stdout, "bindings=") {
			t.Fatalf("--version output has no bindings= line:\n%s", stdout)
		}
		if !strings.Contains(stdout, "reviewer=x-act") {
			t.Fatalf("--version bindings did not reflect REVIEWER_APP=x-act:\n%s", stdout)
		}
	})

	t.Run("unbound role shows the default <role>-app", func(t *testing.T) {
		setupTest(t)
		// No REVIEWER_APP and an empty config home: the default binding applies.
		rc, stdout, stderr := runCap(t, []string{"--version"})
		if rc != 0 {
			t.Fatalf("--version rc = %d, want 0 (stderr: %s)", rc, stderr)
		}
		if !strings.Contains(stdout, "reviewer=reviewer-app") {
			t.Fatalf("--version default binding not shown as reviewer=reviewer-app:\n%s", stdout)
		}
	})
}

// TestRoleAppBindingResolvesBoundAppID — the binding drives the App ID lookup, not just the
// display: with REVIEWER_APP=x-act the App ID resolves off the BOUND prefix (X_ACT_APP_ID),
// and the role's own REVIEWER_APP_ID is not consulted.
func TestRoleAppBindingResolvesBoundAppID(t *testing.T) {
	setupTest(t)
	t.Setenv("REVIEWER_APP", "x-act")
	t.Setenv("X_ACT_APP_ID", "424242")
	appName := appNameFor("reviewer")
	if appName != "x-act" {
		t.Fatalf("appNameFor(reviewer) = %q, want x-act", appName)
	}
	got, err := deskkit.AppIDForApp(appName)
	if err != nil || got != "424242" {
		t.Fatalf("App ID for bound reviewer = %q err=%v, want 424242 (from X_ACT_APP_ID)", got, err)
	}
}

// TestMutationRemovedBindingInCorpus — Verify row 7. The mutation corpus must include the
// removed-binding mutant (appNameFor returning the role default unconditionally), and the
// --version bindings echo (row 2's assertion) is the guard that reddens on it — demonstrated
// here so this test IS the catch the reviewer's `muhar` run confirms.
func TestMutationRemovedBindingInCorpus(t *testing.T) {
	raw, err := os.ReadFile("mutations.json")
	if err != nil {
		t.Fatalf("read mutations.json: %v", err)
	}
	var spec struct {
		Mutations []struct{ Name, File, Old, New string } `json:"mutations"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse mutations.json: %v", err)
	}
	found := false
	for _, m := range spec.Mutations {
		if strings.Contains(m.File, "desktoken.go") &&
			strings.Contains(m.Old, "deskkit.AppBinding(role)") &&
			strings.Contains(m.New, `role + "-app"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("mutations.json lacks the removed-binding mutant (old: return deskkit.AppBinding(role) → " +
			"new: return role + \"-app\") — Verify row 7 requires the corpus to carry it")
	}

	// The guard the mutant reddens: with the binding set, --version echoes the bound App.
	setupTest(t)
	t.Setenv("REVIEWER_APP", "x-act")
	rc, stdout, stderr := runCap(t, []string{"--version"})
	if rc != 0 || !strings.Contains(stdout, "reviewer=x-act") {
		t.Fatalf("row-2 assertion (the mutant's guard) did not hold: rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
}

// --- cell-issues: a seventh role, selectable only by name (desk-console/31) --------------

// TestCellIssuesRoleAcceptedByRoleCheck — Task 2a. cell-issues is a valid role: it must be
// present in validRoles and echoed by --version's bindings= line at its unbound default,
// exactly like every other role.
func TestCellIssuesRoleAcceptedByRoleCheck(t *testing.T) {
	if !validRoles["cell-issues"] {
		t.Fatal("cell-issues must be accepted by the role check — it is the write-issues App " +
			"identity, mintable by name (desk-console/31)")
	}
	setupTest(t)
	rc, stdout, stderr := runCap(t, []string{"--version"})
	if rc != 0 {
		t.Fatalf("--version rc = %d, want 0 (stderr: %s)", rc, stderr)
	}
	if !strings.Contains(stdout, "cell-issues=cell-issues-app") {
		t.Fatalf("--version bindings did not carry the unbound cell-issues default:\n%s", stdout)
	}
}

// TestCellIssuesRoleTypoRefused — Task 2a, negative half. A near-miss spelling is refused
// exactly like any other unknown role, and the refusal's valid-role list now names
// cell-issues — Verify row 4 (`desktoken cell-issue` greps 1 occurrence of "cell-issues" in
// the refusal).
func TestCellIssuesRoleTypoRefused(t *testing.T) {
	setupTest(t)
	rc, _, stderr := runCap(t, []string{"cell-issue"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("typo'd role rc = %d, want %d (refused); stderr: %s", rc, deskkit.ExitRefused, stderr)
	}
	if strings.Count(stderr, "cell-issues") != 1 {
		t.Fatalf("refusal must list cell-issues exactly once among the valid roles: %s", stderr)
	}
}

// TestCellIssuesMissingPEMNamesOnlyItsOwnKey — Task 2b / Verify row 3, the NEGATIVE-PATH
// fail-closed shape (#794): a deployment whose apps.env carries the App ID but not yet the
// PEM (this desk host's exact state, per the brief's facts) refuses at exit 6, names
// cell-issues-app.pem and the searched directories, and names NO other role's key — proving
// the resolution never falls through to a neighbouring role's credential (e.g. the
// issue-loop carrier's slot).
func TestCellIssuesMissingPEMNamesOnlyItsOwnKey(t *testing.T) {
	homeDir := setupTest(t)
	appsEnvDir := filepath.Join(homeDir, ".config", "assay")
	if err := os.MkdirAll(appsEnvDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	appsEnv := "CELL_ISSUES_APP_ID=1\nCELL_ISSUES_INSTALL_ID_EXAMPLE_ORG=1\n"
	if err := os.WriteFile(filepath.Join(appsEnvDir, "apps.env"), []byte(appsEnv), 0o600); err != nil {
		t.Fatalf("write apps.env: %v", err)
	}
	// Deliberately NOT creating cell-issues-app.pem.

	rc, _, stderr := runCap(t, []string{"cell-issues", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing PEM rc = %d, want %d (unverifiable); stderr: %s", rc, deskkit.ExitUnverifiable, stderr)
	}
	if strings.Count(stderr, "cell-issues-app.pem") == 0 {
		t.Fatalf("refusal must name cell-issues-app.pem: %s", stderr)
	}
	for _, other := range []string{
		"issue-loop-app.pem", "intake-loop-app.pem", "worker-app.pem",
		"reviewer-app.pem", "verifier-app.pem", "desk-app.pem",
	} {
		if strings.Contains(stderr, other) {
			t.Fatalf("refusal named a DIFFERENT role's key (%s) — the write App's mint must never "+
				"fall through to a neighbour's PEM: %s", other, stderr)
		}
	}
}

// TestAppEnvPrefixAndBindingDefaultsForCellIssues — Task 2c. The role→App binding is
// generic (appconfig.go), so cell-issues needs no code change to resolve, but the two
// facts the brief pins are asserted directly: AppEnvPrefix("assay-cell-issues") is the
// ASSAY_CELL_ISSUES prefix the pod ConfigMap's bound keys already use, and AppBinding's
// unbound default for the role is cell-issues-app.
func TestAppEnvPrefixAndBindingDefaultsForCellIssues(t *testing.T) {
	if got := deskkit.AppEnvPrefix("assay-cell-issues"); got != "ASSAY_CELL_ISSUES" {
		t.Fatalf(`AppEnvPrefix("assay-cell-issues") = %q, want "ASSAY_CELL_ISSUES"`, got)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	t.Setenv("CELL_ISSUES_APP", "")
	if got := deskkit.AppBinding("cell-issues"); got != "cell-issues-app" {
		t.Fatalf("AppBinding(cell-issues) unbound = %q, want cell-issues-app", got)
	}
}
