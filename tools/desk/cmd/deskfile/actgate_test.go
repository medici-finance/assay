package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// actgate_test.go — the human-only act gate on `deskfile new`.
//
// A human-only hand-off (label `human-only`, or a body whose first line is BLOCKED-ON-HUMAN)
// hands the driver an ACT, so its body must carry that act in runnable form: a fenced `sh`
// block, or a fenced `url` block for a browser step (the ask-decision skill's Act block).
// Without one the filing is REFUSED (exit 5); `--force-new --reason` is the only bypass, and
// it is audited. `attach` is a separate verb and is unaffected.

const fence = "```"

const actProseOnly = "Please rotate the deploy key on the release repo and re-run the publish job."

const actShBody = "The publish job needs a key only the driver can rotate.\n\n" +
	fence + "sh\n" +
	"# 1. rotate the deploy key (dry run first)\n" +
	"DRY_RUN=1 rotate-deploy-key --repo example/release  # fill: the new key's label\n" +
	fence + "\n"

const actURLBody = "The repo setting has no API; it is one browser step.\n\n" +
	fence + "url\n" +
	"https://example.com/settings/actions\n" +
	"Workflow permissions: Read repository contents\n" +
	fence + "\n"

const blockedProse = "BLOCKED-ON-HUMAN — the workflow dispatch is outside this App's scope.\n\n" + actProseOnly

const blockedSh = "BLOCKED-ON-HUMAN — the workflow dispatch is outside this App's scope.\n\n" + actShBody

// fileNew runs `deskfile new` against the fake forge with the given body and labels.
func fileNew(t *testing.T, body string, labels []string, extra ...string) (int, string) {
	t.Helper()
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	if len(labels) > 0 {
		t.Setenv("FAKEGH_LABELS", labelsJSON(t, labels...))
	}
	args := []string{"new", "-R", allowedRepo,
		"--title", "rotate the release deploy key", "--body-file", bodyFileWith(t, body)}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	return runCapture(append(args, extra...))
}

// TestActGateRefusesNoFence — a human-only hand-off with no act fence exits 5 and files
// nothing, on either trigger, whatever the prose looks like.
func TestActGateRefusesNoFence(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		labels []string
	}{
		{"human-only label, prose only", actProseOnly, []string{humanOnlyLabel}},
		{"BLOCKED-ON-HUMAN first line, no label", blockedProse, nil},
		{"decorated marker after a blank line", "\n\n**BLOCKED-ON-HUMAN** — scope.\n\n" + actProseOnly, nil},
		{"heading marker", "## blocked-on-human\n\n" + actProseOnly, nil},
		{"wrong fence language", actProseOnly + "\n\n" + fence + "text\nrotate-deploy-key\n" + fence + "\n", []string{humanOnlyLabel}},
		{"unclosed sh fence", actProseOnly + "\n\n" + fence + "sh\nrotate-deploy-key\n", []string{humanOnlyLabel}},
		{"empty sh fence", actProseOnly + "\n\n" + fence + "sh\n\n" + fence + "\n", []string{humanOnlyLabel}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls := withEnv(t)
			rc, out := fileNew(t, c.body, c.labels)
			if rc != deskkit.ExitRefused {
				t.Fatalf("rc = %d, want %d (exit 5); out=%s", rc, deskkit.ExitRefused, out)
			}
			if !strings.Contains(out, "human-only hand-off") || !strings.Contains(out, fence+"sh") {
				t.Fatalf("refusal does not name the act gate and the ```sh block; out=%s", out)
			}
			if strings.Contains(out, deskkit.DedupeRefusalPrefix) {
				t.Fatalf("act-gate refusal carries the dedupe prefix (reads as 'already filed'); out=%s", out)
			}
			if curForge.filed != nil || createArgv(*calls) != nil {
				t.Fatal("an issue was filed despite the missing act block")
			}
		})
	}
}

// TestActGateShFencePasses — a ```sh act block satisfies the gate on either trigger.
func TestActGateShFencePasses(t *testing.T) {
	for name, c := range map[string]struct {
		body   string
		labels []string
	}{
		"human-only label":       {actShBody, []string{humanOnlyLabel}},
		"BLOCKED-ON-HUMAN line":  {blockedSh, nil},
		"label and marker, both": {blockedSh, []string{humanOnlyLabel}},
	} {
		t.Run(name, func(t *testing.T) {
			withEnv(t)
			rc, out := fileNew(t, c.body, c.labels)
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0; out=%s", rc, out)
			}
			if curForge.filed == nil {
				t.Fatal("the hand-off carrying a ```sh block was not filed")
			}
		})
	}
}

// TestActGateURLFencePasses — a ```url block (a browser step) satisfies the gate.
func TestActGateURLFencePasses(t *testing.T) {
	withEnv(t)
	rc, out := fileNew(t, actURLBody, []string{humanOnlyLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the hand-off carrying a ```url block was not filed")
	}
}

// TestActGateForceNewIsAudited — --force-new --reason is the bypass: the fence-less
// hand-off files, and its audit line records the bypass and the reason. A force-new filing
// that is NOT a hand-off carries no bypass note, so the note means what it says.
func TestActGateForceNewIsAudited(t *testing.T) {
	withEnv(t)
	const why = "the act is a phone call to the registrar"
	rc, out := fileNew(t, blockedProse, []string{humanOnlyLabel}, "--force-new", "--reason", why)
	if rc != deskkit.ExitOK {
		t.Fatalf("--force-new rc = %d, want 0 (bypass); out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the bypassed hand-off was not filed")
	}
	var ok *deskkit.Entry
	for _, e := range readAudit(t) {
		if e.Verb == "new" && e.Result == deskkit.ResultOK {
			e := e
			ok = &e
		}
	}
	if ok == nil {
		t.Fatal("no ok `new` audit line for the bypassed filing")
	}
	for _, want := range []string{actGateBypassNote, "force-new: " + why} {
		if !strings.Contains(ok.Detail, want) {
			t.Fatalf("audit detail missing %q; got %q", want, ok.Detail)
		}
	}

	withEnv(t)
	rc, out = fileNew(t, "an ordinary unique filing", nil, "--force-new", "--reason", "search is down")
	if rc != deskkit.ExitOK {
		t.Fatalf("plain --force-new rc = %d, want 0; out=%s", rc, out)
	}
	for _, e := range readAudit(t) {
		if e.Verb == "new" && strings.Contains(e.Detail, actGateBypassNote) {
			t.Fatalf("a non-hand-off filing carries the act-gate bypass note: %q", e.Detail)
		}
	}
}

// TestActGateAttachUnaffected — the gate binds `new` only: a BLOCKED-ON-HUMAN observation
// posted with `attach` is not refused.
func TestActGateAttachUnaffected(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_ISSUE_STATE", "OPEN")
	rc, out := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11",
		"--body-file", bodyFileWith(t, blockedProse)})
	if rc != deskkit.ExitOK {
		t.Fatalf("attach rc = %d, want 0 (attach is unaffected); out=%s", rc, out)
	}
	if len(curForge.comments) == 0 {
		t.Fatal("the attach observation was not posted")
	}
}
