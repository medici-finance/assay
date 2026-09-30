package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// outbound_test.go — the issue-filing incident fixture (desktools-v2/10, Verify rows 3-4).
//
// Before the outbound-write check, `deskfile new` ran the credential scan and the
// impersonation guard only: an issue body naming a configured withheld identifier was FILED
// on a public repository. The same body on a private target is the control — the public
// layers key on the target's configured visibility, so it must still file.

const obWithheld = "example-withheld-slug"

func obBody(t *testing.T) string {
	t.Helper()
	return bodyFileWith(t, "The failing check lives in the "+obWithheld+
		" stream; it has been red since the last merge and needs an owner.")
}

func TestNewRefusesWithheldIdentifierOnPublicTarget(t *testing.T) {
	withEnv(t)
	t.Setenv(deskkit.EnvWithheldIdentifiers, obWithheld)

	rc, out := runCapture([]string{"new", "-R", "example-org/example-k8s",
		"--title", "red check needs an owner", "--body-file", obBody(t)})
	if curForge.filed != nil {
		t.Fatalf("the issue was FILED on a public target with a withheld identifier in its body "+
			"(title %q) — rc=%d\n%s", curForge.filed.Title, rc, out)
	}
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)\n%s", rc, deskkit.ExitRefused, out)
	}
	if !strings.Contains(out, deskkit.RuleWithheldIdentifier) {
		t.Fatalf("refusal does not name rule %q:\n%s", deskkit.RuleWithheldIdentifier, out)
	}

	// The layered-override ruling: a written reason does NOT publish a withheld identifier.
	rc, out = runCapture([]string{"new", "-R", "example-org/example-k8s",
		"--title", "red check needs an owner", "--body-file", obBody(t),
		"--" + deskkit.ScanOverrideFlag, "the operator believes this is fine"})
	if curForge.filed != nil || rc != deskkit.ExitRefused {
		t.Fatalf("override took a withheld-identifier refusal through: rc=%d filed=%v\n%s", rc, curForge.filed != nil, out)
	}
}

func TestNewPassesSameBodyOnPrivateTarget(t *testing.T) {
	withEnv(t)
	t.Setenv(deskkit.EnvWithheldIdentifiers, obWithheld)

	rc, out := runCapture([]string{"new", "-R", "example-org/tracker",
		"--title", "red check needs an owner", "--body-file", obBody(t)})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — a private target does not run the public layers\n%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("no FileIssue recorded for the private target")
	}
}
