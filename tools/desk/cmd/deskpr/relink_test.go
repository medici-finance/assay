package main

// relink_test.go — `deskpr edit --relink-from/--relink-to/--relink-reason` (#5020, option A).
//
// FAIL-FIRST. Against the base (no relink flags), every `run` here that passes a --relink-*
// flag stops at flag parsing with "refused: bad flags", exit 5. The positive tests
// (TestRelinkIssueToFixes, TestRelinkToNewNumber, TestRelinkBlessedPasses) therefore go RED
// there, and every refusal test asserts on its OWN message, so it cannot pass vacuously on the
// bad-flags refusal. TestEditNoFlagUnchanged is the pin that the default path is unchanged; it
// is GREEN on the base by design.

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// relinkSetup stages a PR whose current body links curLink, and a replacement body linking
// newLink.
func relinkSetup(t *testing.T, curLink, newLink string) string {
	t.Helper()
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\n"+curLink+"\n")
	return writeTempFile(t, "the corrected body\n"+newLink+"\n")
}

// assertNoWrites fails when the fake saw any PR edit or comment.
func assertNoWrites(t *testing.T) {
	t.Helper()
	if curForge.edited != nil || len(curForge.commentNums) != 0 {
		t.Fatalf("a refusal still wrote: edit %v, comments %v", curForge.edited, curForge.commentNums)
	}
}

// regression: #5020
func TestRelinkPartialRefuses(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"from only", []string{"--relink-from", "77"}, "--relink-to N"},
		{"to only", []string{"--relink-to", "78"}, "--relink-from N"},
		{"reason only", []string{"--relink-reason", "wrong issue"}, "--relink-from N"},
		{"no reason", []string{"--relink-from", "77", "--relink-to", "78"}, "--relink-reason TEXT"},
		{"no to", []string{"--relink-from", "77", "--relink-reason", "x"}, "--relink-to N"},
		{"no from", []string{"--relink-to", "78", "--relink-reason", "x"}, "--relink-from N"},
		{"blank reason", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "  "}, "--relink-reason TEXT"},
		{"negative from", []string{"--relink-from", "-1", "--relink-to", "78", "--relink-reason", "x"}, "--relink-from N"},
		{"multi-line reason", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\nb"}, "must be one line"},
		{"long reason", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", strings.Repeat("r", 501)}, "keep it to 500"},
		{"blank reason alone", []string{"--relink-reason", "  "}, "--relink-from N"},
		{"carriage return", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\rb"}, "must be one line"},
		{"comment opener", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a <!-- b"}, "an HTML comment opener"},
		{"backtick", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a ` b"}, "a backtick"},
		{"line separator", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u2028b"}, "U+2028"},
		{"next line", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u0085b"}, "U+0085"},
		{"paragraph separator", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u2029b"}, "U+2029"},
		{"bidi override", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u202eb"}, "format character U+202E"},
		{"bidi isolate", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u2066b"}, "format character U+2066"},
		{"zero-width space", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u200bb"}, "format character U+200B"},
		{"zero-width joiner", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\u200db"}, "format character U+200D"},
		{"byte order mark", []string{"--relink-from", "77", "--relink-to", "78", "--relink-reason", "a\ufeffb"}, "format character U+FEFF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
			err := cmdEdit(append([]string{"--body-file", bodyPath}, tc.args...))
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("rc = %d, want %d; err %v", deskkit.ExitCodeOf(err), deskkit.ExitRefused, err)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal does not name %q: %v", tc.want, err)
			}
			if got := ghCalls(nil); len(got) != 0 {
				t.Fatalf("refused with %d forge call(s); want zero: %v", len(got), got)
			}
		})
	}
}

// regression: #5020
func TestRelinkIssueToFixes(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Fixes: #77")
	rc := run([]string{"edit", "--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "77", "--relink-reason", "this PR resolves the issue"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if want := []string{"comment", "edit", "comment"}; !slices.Equal(curForge.ops, want) {
		t.Fatalf("write order = %v, want %v (audit comment, edit, re-review notice)", curForge.ops, want)
	}
	audit := curForge.comments[0]
	for _, want := range []string{"`Issue: #77` → `Fixes: #77`", "Reason: `this PR resolves the issue`", "BEFORE the body edit"} {
		if !strings.Contains(audit, want) {
			t.Fatalf("audit comment lacks %q:\n%s", want, audit)
		}
	}
	if !slices.Equal(curForge.issueNums, []int{77}) {
		t.Fatalf("new issue reads = %v, want [77]", curForge.issueNums)
	}
}

// regression: #5020
func TestRelinkToNewNumber(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Fixes: #78")
	rc := run([]string{"edit", "--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "typo in the issue number"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !slices.Equal(curForge.issueNums, []int{78}) {
		t.Fatalf("the gate read issue %v, want the NEW issue [78]", curForge.issueNums)
	}
	// The audit comment (and the notice) go to the PR, never to the --relink-to issue.
	if len(curForge.commentNums) != 2 || curForge.commentNums[0] != curForge.editedNum ||
		curForge.commentNums[1] != curForge.editedNum {
		t.Fatalf("comments went to %v, want both on the edited PR #%d", curForge.commentNums, curForge.editedNum)
	}
	if curForge.edited == nil || !strings.Contains(curForge.edited.Body, "Fixes: #78") {
		t.Fatalf("the edit did not land the new link: %+v", curForge.edited)
	}
}

// regression: #5020
func TestRelinkOldMismatch(t *testing.T) {
	for _, tc := range []struct{ name, cur, want string }{
		{"other number", "Issue: #76", "--relink-from #77 does not match the PR's current trailer `Issue: #76`"},
		{"brief link", "Brief: fixture/01", "--relink-from #77 does not match the PR's current trailer (Brief: fixture/01)"},
		{"no link", "", "--relink-from #77 does not match the PR's current trailer (no link trailer)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodyPath := relinkSetup(t, tc.cur, "Issue: #78")
			err := cmdEdit([]string{"--body-file", bodyPath,
				"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want exit 5 naming %q", err, tc.want)
			}
			if len(curForge.issueNums) != 0 {
				t.Fatalf("the new issue was read before the old number matched: %v", curForge.issueNums)
			}
			assertNoWrites(t)
		})
	}
}

// regression: #5020
func TestRelinkBodyMustNameTo(t *testing.T) {
	for _, tc := range []struct{ name, newLink, want string }{
		{"other number", "Fixes: #79", "--relink-to is #78 but the replacement body's trailer is `Fixes: #79`"},
		{"brief", "Brief: fixture/01", "the replacement body's trailer is not one"},
		{"cross repo", "Fixes: example-org/agents#78", "the replacement body's trailer is not one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodyPath := relinkSetup(t, "Issue: #77", tc.newLink)
			err := cmdEdit([]string{"--body-file", bodyPath,
				"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want exit 5 naming %q", err, tc.want)
			}
			if got := ghCalls(nil); len(got) != 0 {
				t.Fatalf("refused with %d forge call(s); want zero: %v", len(got), got)
			}
		})
	}
}

// regression: #5020
func TestRelinkUntrustedRefuses(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_ISSUE_AUTHOR", "mallory:9")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "trust gate") {
		t.Fatalf("err = %v, want exit 5 from the trust gate", err)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkBlessedPasses(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_ISSUE_AUTHOR", "mallory:9")
	t.Setenv("FAKEGH_ISSUE_BLESSED", "1")
	if rc := run([]string{"edit", "--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 for a blessed issue", rc)
	}
}

// regression: #5020
func TestRelinkTrustUnreadable(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_ISSUE_AUTHOR", "mallory:9")
	t.Setenv("FAKEGH_ISSUE_TRUST_FAIL", "1")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "trust gate") {
		t.Fatalf("err = %v, want exit 6 from the trust read", err)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkGatedRefuses(t *testing.T) {
	for _, tc := range []struct{ name, key, val, want string }{
		{"needs-decision", "FAKEGH_ISSUE_LABELS", "bug,needs-decision", "gated (label needs-decision)"},
		{"gate label", "FAKEGH_ISSUE_LABELS", "gate:human", "gated (label gate:human)"},
		{"human-only", "FAKEGH_ISSUE_LABELS", "human-only", "gated (label human-only)"},
		{"security label", "FAKEGH_ISSUE_LABELS", "security", "gated (label security)"},
		{"title marker", "FAKEGH_ISSUE_TITLE", "[HUMAN GATE] rotate", "gated (title marker"},
		{"gate line", "FAKEGH_ISSUE_BODY", "text\nGate: human\n", "gated (body line"},
		{"decision marker", "FAKEGH_ISSUE_BODY", "<!-- decision-gate: s/01 -->", "gated (a decision-gate marker)"},
		{"missing", "FAKEGH_ISSUE_MISSING", "1", "does not exist"},
		{"pull request", "FAKEGH_ISSUE_PR", "1", "is a pull request"},
		{"closed", "FAKEGH_ISSUE_STATE", "closed", "is closed, not open"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
			t.Setenv(tc.key, tc.val)
			err := cmdEdit([]string{"--body-file", bodyPath,
				"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want exit 5 naming %q", err, tc.want)
			}
			assertNoWrites(t)
		})
	}
}

// regression: #5020
func TestRelinkSameNumGated(t *testing.T) {
	for _, label := range []string{"needs-decision", "human-only", "security", "gate:human"} {
		t.Run(label, func(t *testing.T) {
			bodyPath := relinkSetup(t, "Issue: #77", "Fixes: #77")
			t.Setenv("FAKEGH_ISSUE_LABELS", label)
			err := cmdEdit([]string{"--body-file", bodyPath,
				"--relink-from", "77", "--relink-to", "77", "--relink-reason", "x"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "gated (label "+label+")") {
				t.Fatalf("err = %v, want exit 5: Issue: -> Fixes: on the same gated number passes the same gate", err)
			}
			assertNoWrites(t)
		})
	}
}

// regression: #5020
func TestRelinkAuditFailNoEdit(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_COMMENT_FAIL", "1")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "NOT edited") {
		t.Fatalf("err = %v, want exit 6 saying the body was not edited", err)
	}
	if curForge.edited != nil {
		t.Fatalf("the body was edited although its audit comment failed: %+v", curForge.edited)
	}
}

// regression: #5020
func TestRelinkReasonScanned(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "ghp_" + strings.Repeat("c", 30)})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "relink reason") {
		t.Fatalf("err = %v, want the secret scan to refuse the relink reason", err)
	}
	if got := ghCalls(nil); len(got) != 0 {
		t.Fatalf("refused with %d forge call(s); want zero: %v", len(got), got)
	}
}

// TestEditNoFlagUnchanged pins the default: without --relink-*, a link change refuses with
// the same message as before #5020, reads no issue, and a plain edit writes edit-then-notice.
func TestEditNoFlagUnchanged(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Fixes: #77")
	err := cmdEdit([]string{"--body-file", bodyPath})
	const want = "refused: the replacement body changes the PR's link trailer (Issue: #77 → Fixes: #77) — a work item " +
		"link is not editable after the fact. Keep `Issue: #77` in the body and re-run; a PR that genuinely delivers " +
		"different work is a different PR."
	if err == nil || err.Error() != want {
		t.Fatalf("no-flag refusal changed:\n got %v\nwant %s", err, want)
	}
	if len(curForge.issueNums) != 0 || len(curForge.trustNums) != 0 {
		t.Fatalf("the no-flag path read an issue: %v %v", curForge.issueNums, curForge.trustNums)
	}
	assertNoWrites(t)

	bodyPath = relinkSetup(t, "Issue: #77", "Issue: #77\nmore text")
	if rc := run([]string{"edit", "--body-file", bodyPath}); rc != deskkit.ExitOK {
		t.Fatalf("plain edit rc = %d, want 0", rc)
	}
	if want := []string{"edit", "comment"}; !slices.Equal(curForge.ops, want) {
		t.Fatalf("plain edit write order = %v, want %v", curForge.ops, want)
	}
	if len(curForge.issueNums) != 0 {
		t.Fatalf("the plain edit read an issue: %v", curForge.issueNums)
	}
}

// regression: #5020
func TestRelinkNoChangeRefuses(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #77")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "77", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "nothing for --relink-* to correct") {
		t.Fatalf("err = %v, want exit 5: the flags name no change", err)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkEditFailAfterAudit(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_EDIT_FAIL", "1")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable ||
		!strings.Contains(err.Error(), "may still carry Issue: #77; check the PR body") {
		t.Fatalf("err = %v, want exit 6 telling the caller to check which link the PR carries", err)
	}
	if want := []string{"comment", "edit"}; !slices.Equal(curForge.ops, want) {
		t.Fatalf("write order = %v, want %v (the audit, then the failed edit, no notice)", curForge.ops, want)
	}
}

// regression: #5020
func TestRelinkReasonSelfContain(t *testing.T) {
	work := newPublicFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\nIssue: #77\n")
	bodyPath := writeTempFile(t, "the corrected body\nIssue: #78\n")
	err := cmdEdit([]string{"--body-file", bodyPath, "--relink-from", "77", "--relink-to", "78",
		"--relink-reason", "session fbf250ed-be0d-4832-9ebb-2bff546eebbf found it"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "relink reason") {
		t.Fatalf("err = %v, want the self-containment scan to refuse the relink reason", err)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkTrustIncomplete(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_ISSUE_AUTHOR", "mallory:9")
	t.Setenv("FAKEGH_ISSUE_BLESSED", "1")
	t.Setenv("FAKEGH_ISSUE_TRUST_INCOMPLETE", "1")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "trust gate") {
		t.Fatalf("err = %v, want exit 5: a blessing on an incomplete record does not count", err)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkIssueUnreadable(t *testing.T) {
	for _, key := range []string{"FAKEGH_ISSUE_READ_FAIL", "FAKEGH_ISSUE_NIL"} {
		t.Run(key, func(t *testing.T) {
			bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
			t.Setenv(key, "1")
			err := cmdEdit([]string{"--body-file", bodyPath,
				"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "#78") {
				t.Fatalf("err = %v, want exit 6 for an unreadable new issue", err)
			}
			assertNoWrites(t)
		})
	}
}

// regression: #5020
func TestRelinkPRNumberRefuses(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_ISSUE_PR", "1")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "#78 is a pull request") ||
		strings.Contains(err.Error(), "--kind") {
		t.Fatalf("err = %v, want exit 5 naming the PR, with no --kind hint deskpr edit cannot take", err)
	}
	want := []deskkit.TargetKind{deskkit.TargetIssue, deskkit.TargetChange}
	if !slices.Equal(curForge.issueKinds, want) {
		t.Fatalf("reads = %v, want %v (the issue read, then one change read to classify it)", curForge.issueKinds, want)
	}
	assertNoWrites(t)
}

// TestRelinkGitHubKindPin runs the gate against the REAL GitHub backend, served from a local
// httptest stub that answers a pull-request record at the number. The fake forge copies the
// backend's kind-mismatch wording; this pins that the backend's own answer is still the one
// relinkKindMismatch recognises, so a change to that wording goes red here instead of
// silently turning the documented exit-5 refusal into exit 6.
//
// regression: #5020
func TestRelinkGitHubKindPin(t *testing.T) {
	var (
		mu    sync.Mutex
		paths []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.URL.Path != "/repos/example-org/tracker/issues/78" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":78,"title":"t","state":"open","body":"",` +
			`"user":{"login":"ada","id":2001,"type":"User"},"pull_request":{}}`))
	}))
	defer srv.Close()
	fg := &deskkit.GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}
	err := relinkIssueGate(fg, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, 78)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "#78 is a pull request") {
		t.Fatalf("err = %v, want exit 5: the GitHub backend's own kind-mismatch answer must classify the number", err)
	}
	want := []string{"GET /repos/example-org/tracker/issues/78", "GET /repos/example-org/tracker/issues/78"}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(paths, want) {
		t.Fatalf("requests = %v, want %v (the issue read, then one change read)", paths, want)
	}
}

// regression: #5020
func TestRelinkSplitReadFail(t *testing.T) {
	for _, mode := range []string{"503", "403", "503-echo", "transport"} {
		t.Run(mode, func(t *testing.T) {
			bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
			t.Setenv("FAKEGH_ISSUE_SPLIT_FAIL", mode)
			err := cmdEdit([]string{"--body-file", bodyPath,
				"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable ||
				strings.Contains(err.Error(), "refused: --relink-to #78 is a pull request") {
				t.Fatalf("err = %v, want exit 6: a failed issue read is could-not-check, and a change "+
					"at the same number in a separate sequence does not make #78 a pull request", err)
			}
			want := []deskkit.TargetKind{deskkit.TargetIssue}
			if !slices.Equal(curForge.issueKinds, want) {
				t.Fatalf("reads = %v, want %v (no change read after a plain read failure)", curForge.issueKinds, want)
			}
			assertNoWrites(t)
		})
	}
}

// regression: #5020
func TestRelinkAuditRowDetail(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Fixes: #78")
	if rc := run([]string{"edit", "--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	audit := string(auditBytes(t))
	if !strings.Contains(audit, "(link Issue: #77 -") || !strings.Contains(audit, " Fixes: #78)") {
		t.Fatalf("the audit row does not record the link change:\n%s", audit)
	}
}

// regression: #5020
func TestRelinkRerunAfterLanding(t *testing.T) {
	bodyPath := relinkSetup(t, "Fixes: #78", "Fixes: #78")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "the correction has landed") {
		t.Fatalf("err = %v, want exit 5 saying the correction already landed", err)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkOtherCheckout(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "checkout", "-b", "desk-window", "refs/remotes/origin/main")
	withEnv(t, work)
	t.Setenv("FAKEGH_PR_OID", "1111111111111111111111111111111111111111")
	t.Setenv("FAKEGH_PR_HEAD", "feature/held-elsewhere")
	t.Setenv("FAKEGH_PR_BODY", "someone else's PR\nIssue: #77\n")
	bodyPath := writeTempFile(t, "the corrected body\nIssue: #78\n")
	err := cmdEdit([]string{"--pr", "42", "--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "only from the PR's own checkout") {
		t.Fatalf("err = %v, want exit 5: a relink runs only from the PR's own checkout", err)
	}
	if len(curForge.issueNums) != 0 {
		t.Fatalf("the new issue was read before the checkout was admitted: %v", curForge.issueNums)
	}
	assertNoWrites(t)
}

// regression: #5020
func TestRelinkUntypedPRRecord(t *testing.T) {
	bodyPath := relinkSetup(t, "Issue: #77", "Issue: #78")
	t.Setenv("FAKEGH_ISSUE_PR", "1")
	t.Setenv("FAKEGH_ISSUE_UNTYPED", "1")
	err := cmdEdit([]string{"--body-file", bodyPath,
		"--relink-from", "77", "--relink-to", "78", "--relink-reason", "x"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "#78 is a pull request") {
		t.Fatalf("err = %v, want exit 5 when the issue read returns a pull-request record", err)
	}
	assertNoWrites(t)
}
