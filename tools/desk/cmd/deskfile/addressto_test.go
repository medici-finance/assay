package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// addressto_test.go — `deskfile new --to <role>` stamps the `to:<role>` desk-inbox
// addressee label. It reuses the raised-by resolver (one resolver, two flags) and
// degrades EXACTLY as the raised-by stamp does when the label is missing — with the one
// deliberate difference that omitting --to is silent (addressing a filing is the rare
// case; not addressing one is the default).

// TestToRoleUnbound is Verify row 2, the negative-path row: `--to nosuchrole` is refused
// (exit 5) BEFORE any write, and the message names the bound role set so the caller can
// act on it.
func TestToRoleUnbound(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "a filing addressed to a role nobody bound")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an invented addressee reaches the label", "--body-file", body,
		"--to", "nosuchrole"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("unbound --to rc = %d, want 5; out=%s", rc, out)
	}
	assertNoMutatingGH(t, *calls)
	if anyCall(ghCalls(*calls), "search") {
		t.Fatalf("the addressee role was validated AFTER the dedupe search — a caller error "+
			"should not spend an API call; gh calls: %v", ghCalls(*calls))
	}
	for _, want := range []string{"reviewer", "verifier", "worker"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must enumerate the bound roles (missing %q); out=%s", want, out)
		}
	}
}

// TestToStampsWhenLabelExists — the happy path: the to:<role> label rides alongside the
// caller's own labels, and the audit records the addressee.
func TestToStampsWhenLabelExists(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug", "to:verifier"))
	body := bodyFileWith(t, "a message for the verify desk")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "please re-baseline the stale verify artifact", "--body-file", body,
		"--label", "bug", "--to", "verifier"})
	if rc != deskkit.ExitOK {
		t.Fatalf("addressed new rc = %d, want 0; out=%s", rc, out)
	}
	got := labelArgs(createArgv(*calls))
	if !hasString(got, "to:verifier") {
		t.Fatalf("create argv carries no to: label: %v", got)
	}
	if !hasString(got, "bug") {
		t.Fatalf("the addressee stamp displaced the caller's own label: %v", got)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "to=verifier") {
		t.Fatalf("audit detail = %q, want it to carry to=verifier", d)
	}
	assertNoForbiddenGH(t, *calls)
}

// TestToMissingLabelFilesUnaddressedWithRemedy — the mergedstatus.go precedent: the
// to:<role> labels do not exist on any repo yet, so a missing one must NOT stop the
// filing. The issue files UNADDRESSED and the NOTICE carries the one-off create command.
func TestToMissingLabelFilesUnaddressedWithRemedy(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug", "question")) // no to:* at all
	body := bodyFileWith(t, "a message the repo cannot yet route")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "route this to the worker desk", "--body-file", body,
		"--to", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — a missing to: label must never stop a filing; out=%s", rc, out)
	}
	if argv := createArgv(*calls); argv == nil {
		t.Fatalf("the issue was not created; gh calls: %v", ghCalls(*calls))
	} else if hasString(labelArgs(argv), "to:worker") {
		t.Fatalf("a non-existent label was passed to `gh issue create`, which would have failed "+
			"the whole filing: %v", labelArgs(argv))
	}
	if !strings.Contains(out, "gh label create to:worker") {
		t.Fatalf("the NOTICE must carry the exact remedy command; out=%s", out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "to=UNADDRESSED:label-missing") {
		t.Fatalf("audit detail = %q, want to=UNADDRESSED:label-missing", d)
	}
}

// TestToOmittedIsSilentAndUnaddressed — the deliberate difference from raised-by: omitting
// --to is the common case, so it is SILENT (no NOTICE) and stamps nothing, but the audit
// still records that the filing was unaddressed.
func TestToOmittedIsSilentAndUnaddressed(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "an ordinary filing addressed to no desk")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an ordinary unaddressed observation", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	for _, l := range labelArgs(createArgv(*calls)) {
		if strings.HasPrefix(strings.ToLower(l), deskkit.AddressedToPrefix) {
			t.Fatalf("a to: stamp %q was applied with no --to flag", l)
		}
	}
	if strings.Contains(out, "UNADDRESSED") {
		t.Fatalf("omitting --to must be SILENT (no NOTICE) — it is the common case; out=%s", out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "to=UNADDRESSED:not-requested") {
		t.Fatalf("audit detail = %q, want to=UNADDRESSED:not-requested", d)
	}
}

// TestToProbeOutageIsCouldNotCheck — an unanswered label probe is could-not-check, not
// "absent": both drop the stamp, but a caller told to create a label during an outage
// creates one that already exists.
func TestToProbeOutageIsCouldNotCheck(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABEL_FAIL", "1")
	body := bodyFileWith(t, "addressed during a label-api outage")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a message filed during a label outage", "--body-file", body,
		"--to", "desk"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — a probe outage must not block a filing; out=%s", rc, out)
	}
	if !strings.Contains(out, "could not check") {
		t.Fatalf("the NOTICE must name the could-not-check state; out=%s", out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "to=UNADDRESSED:could-not-check") {
		t.Fatalf("audit detail = %q, want to=UNADDRESSED:could-not-check", d)
	}
}

// TestToAndRaisedByCoexist — the two flags are independent: an issue can be RAISED BY one
// desk and addressed TO another, and both stamps land.
func TestToAndRaisedByCoexist(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "raised-by:worker", "to:reviewer"))
	body := bodyFileWith(t, "the worker desk asks the reviewer to look")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "worker asks reviewer to re-check the head", "--body-file", body,
		"--raised-by", "worker", "--to", "reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	got := labelArgs(createArgv(*calls))
	if !hasString(got, "raised-by:worker") || !hasString(got, "to:reviewer") {
		t.Fatalf("both stamps must land: %v", got)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=worker") || !strings.Contains(d, "to=reviewer") {
		t.Fatalf("audit detail = %q, want both raised-by=worker and to=reviewer", d)
	}
}
