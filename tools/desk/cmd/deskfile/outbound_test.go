package main

import (
	"os"
	"path/filepath"
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

// --- desktools-v2/11: the house callout through the whole verb -------------------

// writeHouseCallout installs an invented executable that answers `block` and plants it in
// the fixture roster. It returns the executable's directory (the stub logs beside itself).
func writeHouseCallout(t *testing.T) string {
	t.Helper()
	return writeHouseCalloutSaying(t, "block example-house-rule")
}

// writeHouseCalloutSaying is writeHouseCallout with the answer line chosen by the test.
func writeHouseCalloutSaying(t *testing.T, answer string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "callout.sh")
	script := "#!/bin/sh\necho run >> \"${0%/*}/ran.log\"\ncat > /dev/null\necho '" + answer + "'\necho example-house-diagnostic >&2\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	rp := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
	b, err := os.ReadFile(rp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, append(b, []byte("\n"+deskkit.EnvOutboundCallout+"="+p+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return dir
}

// TestHouseCalloutBlocksIssueFiling — a body the compiled layer passes, a callout that
// blocks: the issue is NOT filed, the exit is the refusal code, the reason is on stderr and
// not in the audit log, and --force-scan-override does not take it through.
func TestHouseCalloutBlocksIssueFiling(t *testing.T) {
	withEnv(t)
	dir := writeHouseCallout(t)
	var notices strings.Builder
	t.Cleanup(deskkit.SetOutboundNoticeWriter(&notices))

	args := []string{"new", "-R", "example-org/example-k8s",
		"--title", "red check needs an owner", "--body-file", bodyFileWith(t, "The check has been red since the last merge and needs an owner.")}
	rc, out := runCapture(args)
	if curForge.filed != nil {
		t.Fatalf("the issue was FILED although the house callout blocked it — rc=%d\n%s", rc, out)
	}
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)\n%s", rc, deskkit.ExitRefused, out)
	}
	if !strings.Contains(out, deskkit.RuleHouseCallout) {
		t.Fatalf("the refusal does not name %q:\n%s", deskkit.RuleHouseCallout, out)
	}
	if !strings.Contains(notices.String(), "example-house-rule") {
		t.Fatalf("the callout's reason is not on stderr:\n%s", notices.String())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "ran.log")); !strings.Contains(string(b), "run") {
		t.Fatal("the callout never ran")
	}
	if strings.Contains(out, "example-house-rule") {
		t.Fatalf("the reason reached the verb's own output (which verbs also log):\n%s", out)
	}
	audit, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
	if !strings.Contains(string(audit), `"result":"refused"`) {
		t.Fatalf("the verb wrote no refused audit row:\n%s", audit)
	}
	if strings.Contains(string(audit), "example-house-rule") {
		t.Fatalf("the reason reached the audit log:\n%s", audit)
	}

	rc, out = runCapture(append(args, "--"+deskkit.ScanOverrideFlag, "the operator believes this is fine"))
	if curForge.filed != nil || rc != deskkit.ExitRefused {
		t.Fatalf("the override took a house.callout block through: rc=%d filed=%v\n%s", rc, curForge.filed != nil, out)
	}
}

// TestHouseCalloutOffVocabularyNeverLogged — the whole verb, with a callout whose answer is
// outside the vocabulary but carries its own words (`block:` with a colon, another verb). The
// write is refused and NOT filed, and the callout's words appear on stderr only: not in the
// verb's own output (which carries the returned error) and not in the audit log the verb
// writes that error into.
func TestHouseCalloutOffVocabularyNeverLogged(t *testing.T) {
	for _, answer := range []string{"block: example-house-rule", "deny example-house-rule", "maybe example-house-rule"} {
		t.Run(answer, func(t *testing.T) {
			withEnv(t)
			writeHouseCalloutSaying(t, answer)
			var notices strings.Builder
			t.Cleanup(deskkit.SetOutboundNoticeWriter(&notices))
			rc, out := runCapture([]string{"new", "-R", "example-org/example-k8s",
				"--title", "red check needs an owner", "--body-file", bodyFileWith(t, "The check has been red since the last merge and needs an owner.")})
			if curForge.filed != nil || rc != deskkit.ExitRefused {
				t.Fatalf("answer %q: want refused and not filed, got rc=%d filed=%v\n%s", answer, rc, curForge.filed != nil, out)
			}
			if !strings.Contains(out, deskkit.RuleHouseCallout) || !strings.Contains(out, "neither `allow` nor `block`") {
				t.Fatalf("answer %q: the refusal does not name the failure:\n%s", answer, out)
			}
			if !strings.Contains(notices.String(), "example-house-rule") || !strings.Contains(notices.String(), "example-house-diagnostic") {
				t.Errorf("answer %q: the callout's answer and diagnostic are not on stderr:\n%s", answer, notices.String())
			}
			audit, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
			if !strings.Contains(string(audit), `"result":"refused"`) {
				t.Fatalf("answer %q: the verb wrote no refused audit row:\n%s", answer, audit)
			}
			for where, text := range map[string]string{"the verb's output": out, "the audit log": string(audit)} {
				for _, leak := range []string{"example-house-rule", "example-house-diagnostic"} {
					if strings.Contains(text, leak) {
						t.Errorf("answer %q: %q reached %s:\n%s", answer, leak, where, text)
					}
				}
			}
		})
	}
}
