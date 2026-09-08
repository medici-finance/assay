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
