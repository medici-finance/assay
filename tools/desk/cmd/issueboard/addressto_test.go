package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// addressto_test.go — the desk-inbox (`to:<role>`) lane on `issueboard issues`.
//
// These tests run through the SAME recorded fake Forge seam issueboard_test.go
// introduced (installForge → the forgeFor override + fixture roster): issues arrive as
// deskkit.IssueSummary, and the addressee's comment history arrives as a
// deskkit.TrustPayload from IssueTrustEvents — the one bounded events read the board
// pays per addressed (or decision-owed) item (board.go:689, fetchIssueEvents). An
// addressed item that survives the --to filter therefore MUST carry a trust fixture, or
// the events read fails closed (exit 6); every fixture below seeds one.
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
	// A wide SLA (--sla-days 3650) keeps the addressed item ADDRESSED (not ESCALATE) so the
	// filter — not the clock — is what this row proves. #30 is the only item that survives
	// the --to worker filter, so it is the only one that pays an events read: seed its trust
	// fixture (complete, no events) so that bounded read succeeds.
	installForge(t, map[string]*repoFixture{
		homeRepo: {
			issues: []deskkit.IssueSummary{
				{Number: 30, Title: "for the worker desk", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"to:worker"}, CreatedAt: "2026-09-01T00:00:00Z"},
				{Number: 31, Title: "for the reviewer desk", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"to:reviewer"}, CreatedAt: "2026-09-01T00:00:00Z"},
				{Number: 32, Title: "a plain unaddressed issue", Author: deskkit.Account{Login: "shared-agent"}},
			},
			trust: map[int]*deskkit.TrustPayload{
				30: {Complete: true},
			},
		},
	})

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
	// Unflagged run: both items are scanned. #40 (to:worker) pays an events read — seed its
	// trust fixture; #41 (plain) pays none.
	installForge(t, map[string]*repoFixture{
		homeRepo: {
			issues: []deskkit.IssueSummary{
				{Number: 40, Title: "addressed to worker", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"to:worker"}, CreatedAt: "2026-09-01T00:00:00Z"},
				{Number: 41, Title: "plain open issue", Author: deskkit.Account{Login: "shared-agent"}},
			},
			trust: map[int]*deskkit.TrustPayload{
				40: {Complete: true},
			},
		},
	})

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
	// The worker App login the fixture roster binds, in its `<slug>[bot]` rendering — the
	// same identity the board folds through deskkit.SameActor when it asks whether the
	// addressee responded. Seed the addressee's comment with this exact login (mirroring
	// issueboard_test.go's own bot-comment fixtures, which carry the `[bot]` form).
	workerApp, ok := deskkit.RoleAppLogin("worker")
	if !ok {
		t.Fatal("fixture roster does not bind the worker role — cannot test the addressee-App signal")
	}

	t.Run("aged, no App response -> ESCALATE", func(t *testing.T) {
		// Only a non-addressee (desk App) comment: the worker App has not responded, and a
		// bot comment never moves the escalation clock, so age is measured from the 2026-01-01
		// createdAt — well past the default SLA.
		installForge(t, map[string]*repoFixture{
			homeRepo: {
				issues: []deskkit.IssueSummary{
					{Number: 50, Title: "unread worker inbox item", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"to:worker"}, CreatedAt: "2026-01-01T00:00:00Z"},
				},
				trust: map[int]*deskkit.TrustPayload{
					50: {Complete: true, Events: []deskkit.ContentEvent{comment("assay-desk-app[bot]", 300000001, "2026-01-02T00:00:00Z")}},
				},
			},
		})

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
		// The worker App HAS commented — the addressee responded, so no escalation even aged.
		installForge(t, map[string]*repoFixture{
			homeRepo: {
				issues: []deskkit.IssueSummary{
					{Number: 51, Title: "answered worker inbox item", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"to:worker"}, CreatedAt: "2026-01-01T00:00:00Z"},
				},
				trust: map[int]*deskkit.TrustPayload{
					51: {Complete: true, Events: []deskkit.ContentEvent{comment(workerApp, 300000006, "2026-01-02T00:00:00Z")}},
				},
			},
		})

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
	installForge(t, nil)
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "--to", "worker", "intake"}, &out, &errb); code != deskkit.ExitRefused {
		t.Fatalf("run(intake --to worker) = exit %d, want 5 (refused); stderr=%s", code, errb.String())
	}
}
