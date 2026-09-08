package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// addressto_test.go — the desk-inbox (`to:<role>`) lane on `issueboard issues`.
//
// Verify row 4's three tests:
//   - TestIssuesToRoleFilter        — `--to <role>` shows only that role's items.
//   - TestForeignAddressedExcluded  — an unflagged run renders a to:* item ADDRESSED→<role>
//                                     and keeps it OUT of the un-briefed (CREATE-PLACEHOLDER) work.
//   - TestAddressedEscalate         — an aged to:<role> item with no comment from that role's
//                                     App classifies ESCALATE; the same item WITH the App's
//                                     comment does not.

// TestIssuesToRoleFilter — `issues --to worker` returns only issues carrying to:worker; a
// to:reviewer item and a plain item are excluded from the inbox view.
func TestIssuesToRoleFilter(t *testing.T) {
	installFakeGH(t)
	t.Setenv("ISSUEBOARD_GH_ISSUE_REPO", homeRepo)
	t.Setenv("ISSUEBOARD_GH_ISSUES_JSON",
		`[{"number":30,"title":"for the worker desk","author":{"login":"shared-agent"},"labels":[{"name":"to:worker"}],"createdAt":"2026-09-01T00:00:00Z"},`+
			`{"number":31,"title":"for the reviewer desk","author":{"login":"shared-agent"},"labels":[{"name":"to:reviewer"}],"createdAt":"2026-09-01T00:00:00Z"},`+
			`{"number":32,"title":"a plain unaddressed issue","author":{"login":"shared-agent"},"labels":[]}]`)
	// A wide SLA keeps the addressed item ADDRESSED (not ESCALATE) so the filter — not the
	// clock — is what this row proves.
	t.Setenv("ISSUEBOARD_GH_GRAPHQL_JSON", gqlIssuePayload(""))

	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "--sla-days", "3650", "--to", "worker", "issues"}, &out, &errb); code != 0 {
		t.Fatalf("run(issues --to worker) = exit %d, stderr=%s", code, errb.String())
	}
	board := out.String()
	if !strings.Contains(board, "for the worker desk") || !strings.Contains(board, "ADDRESSED→worker") {
		t.Errorf("the worker inbox must list the to:worker item as ADDRESSED→worker; got:\n%s", board)
	}
	if strings.Contains(board, "for the reviewer desk") {
		t.Errorf("a to:reviewer item leaked into the worker inbox; got:\n%s", board)
	}
	if strings.Contains(board, "a plain unaddressed issue") {
		t.Errorf("a plain (unaddressed) item leaked into the worker inbox; got:\n%s", board)
	}
}

// TestForeignAddressedExcluded — the unflagged `issues` run renders a to:worker item
// ADDRESSED→worker and does NOT count it as CREATE-PLACEHOLDER work, while a plain open
// issue is still CREATE-PLACEHOLDER.
func TestForeignAddressedExcluded(t *testing.T) {
	installFakeGH(t)
	t.Setenv("ISSUEBOARD_GH_ISSUE_REPO", homeRepo)
	t.Setenv("ISSUEBOARD_GH_ISSUES_JSON",
		`[{"number":40,"title":"addressed to worker","author":{"login":"shared-agent"},"labels":[{"name":"to:worker"}],"createdAt":"2026-09-01T00:00:00Z"},`+
			`{"number":41,"title":"plain open issue","author":{"login":"shared-agent"},"labels":[]}]`)
	t.Setenv("ISSUEBOARD_GH_GRAPHQL_JSON", gqlIssuePayload(""))

	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "--sla-days", "3650", "issues"}, &out, &errb); code != 0 {
		t.Fatalf("run(issues) = exit %d, stderr=%s", code, errb.String())
	}
	board := out.String()

	if !strings.Contains(board, "ADDRESSED→worker") || !strings.Contains(board, "addressed to worker") {
		t.Errorf("the to:worker item must render ADDRESSED→worker; got:\n%s", board)
	}
	// The addressed item must NOT be classified as un-briefed CREATE-PLACEHOLDER work.
	// Split the lane at the addressed row and confirm #40 never wears a CREATE action.
	for _, ln := range strings.Split(board, "\n") {
		if strings.Contains(ln, "#40") && strings.Contains(ln, actCreatePlaceholder) {
			t.Errorf("the addressed item #40 was classified CREATE-PLACEHOLDER (un-briefed work): %q", ln)
		}
	}
	// The plain item is still CREATE-PLACEHOLDER.
	if !strings.Contains(board, actCreatePlaceholder) || !strings.Contains(board, "plain open issue") {
		t.Errorf("the plain item must still be CREATE-PLACEHOLDER; got:\n%s", board)
	}
}

// TestAddressedEscalate — the second, cooperation-free layer: a to:worker item aged past
// the SLA with NO comment from the worker App is ESCALATE; the same item WITH a worker-App
// comment is held ADDRESSED, not escalated.
func TestAddressedEscalate(t *testing.T) {
	// The worker App login the fixture roster binds, in its `<slug>[bot]` rendering. The
	// GraphQL Bot actor carries the BARE slug (renderedLogin re-appends `[bot]`), so the
	// fixture comment is seeded with the slug alone.
	workerApp, ok := deskkit.RoleAppLogin("worker")
	if !ok {
		t.Fatal("fixture roster does not bind the worker role — cannot test the addressee-App signal")
	}
	workerSlug := strings.TrimSuffix(workerApp, "[bot]")

	t.Run("aged, no App response -> ESCALATE", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("ISSUEBOARD_GH_ISSUE_REPO", homeRepo)
		t.Setenv("ISSUEBOARD_GH_ISSUES_JSON",
			`[{"number":50,"title":"unread worker inbox item","author":{"login":"shared-agent"},"labels":[{"name":"to:worker"}],"createdAt":"2026-01-01T00:00:00Z"}]`)
		// Only a non-addressee comment: the worker App has not responded.
		t.Setenv("ISSUEBOARD_GH_GRAPHQL_JSON",
			gqlIssuePayload("", gqlComment("assay-desk-app[bot]", "Bot", 300000001, "2026-01-02T00:00:00Z")))

		root := t.TempDir()
		var out, errb bytes.Buffer
		if code := run([]string{"--root", root, "issues"}, &out, &errb); code != 0 {
			t.Fatalf("run(issues) = exit %d, stderr=%s", code, errb.String())
		}
		board := out.String()
		if !strings.Contains(board, "ESCALATE") || !strings.Contains(board, "unread worker inbox item") {
			t.Errorf("an aged to:worker item with no worker-App response must ESCALATE; got:\n%s", board)
		}
	})

	t.Run("aged, App responded -> not escalated", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("ISSUEBOARD_GH_ISSUE_REPO", homeRepo)
		t.Setenv("ISSUEBOARD_GH_ISSUES_JSON",
			`[{"number":51,"title":"answered worker inbox item","author":{"login":"shared-agent"},"labels":[{"name":"to:worker"}],"createdAt":"2026-01-01T00:00:00Z"}]`)
		// The worker App HAS commented — the addressee responded, so no escalation even aged.
		t.Setenv("ISSUEBOARD_GH_GRAPHQL_JSON",
			gqlIssuePayload("", gqlComment(workerSlug, "Bot", 300000006, "2026-01-02T00:00:00Z")))

		root := t.TempDir()
		var out, errb bytes.Buffer
		if code := run([]string{"--root", root, "issues"}, &out, &errb); code != 0 {
			t.Fatalf("run(issues) = exit %d, stderr=%s", code, errb.String())
		}
		board := out.String()
		if strings.Contains(board, "ESCALATE") {
			t.Errorf("a to:worker item the worker App answered must NOT escalate; got:\n%s", board)
		}
		if !strings.Contains(board, "ADDRESSED→worker") {
			t.Errorf("the answered item should still render ADDRESSED→worker; got:\n%s", board)
		}
	})
}

// TestToFilterOnWrongLaneRefused — --to is the issues-lane filter; on board/intake it is a
// refused caller error, not a silent no-op that reads as an empty inbox.
func TestToFilterOnWrongLaneRefused(t *testing.T) {
	installFakeGH(t)
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "--to", "worker", "intake"}, &out, &errb); code != deskkit.ExitRefused {
		t.Fatalf("run(intake --to worker) = exit %d, want 5 (refused); stderr=%s", code, errb.String())
	}
}
