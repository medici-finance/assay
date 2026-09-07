package main

// workflowrun_test.go — the successor to TestReadPR_UnreadableWorkflowRun_CanStillFlip and to
// TestFlipPRGraphQLBalanced (querybalance_test.go, now retired with the constant it guarded).
//
// WHAT THE PREDECESSORS PROTECTED. deskflip's PR-state read used to be a hand-authored GraphQL
// document, kept hand-authored precisely because gh's built-in `statusCheckRollup` field pulls
// in `checkSuite { workflowRun … }` — a LINK to the Actions run, which needs `actions:read`.
// Under the reviewer App's `checks:read`-only identity that sub-field 403s and the whole read
// fails, so the desk could not flip ANY private PR. Two tests guarded that: one drove a fake
// `gh` that refused any read mentioning workflowRun/checkSuite, and one checked the query
// constant's braces balanced (a 13-open/14-close typo had taken the fleet down, and no
// gh-stubbing test executed the constant).
//
// WHAT REPLACES THEM. There is no query constant any more: the state read is
// GetPullRequest (REST) and the rollups are ChecksAtHead (REST), so there is no document to
// mis-brace and no way to select an `actions:read` field by accident. The property itself is
// still asserted rather than assumed — a forge that 403s anything Actions-shaped must not
// disturb this verb — and it is asserted against the RECORDER, which sees the requests that
// were actually emitted rather than the words a CLI was handed.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestPRStateReadNeedsNoActionsRead(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0 — the desk must be able to flip under a checks:read-only "+
			"identity", rc)
	}

	// Nothing the verb emitted may name an Actions-run surface: not in a path, not in a query,
	// not in a request body (which is where a GraphQL selection would hide).
	for _, r := range s.requests {
		blob := r.Path + "?" + r.Query + " " + r.Body
		for _, forbidden := range []string{"workflowRun", "checkSuite", "/actions/"} {
			if strings.Contains(blob, forbidden) {
				t.Errorf("the verb requested %q, which needs actions:read: %s", forbidden, r)
			}
		}
	}

	// And the reads it DOES make are the two `checks:read`/`statuses:read` rollups, so the
	// assertion above is not vacuously satisfied by a verb that read nothing.
	if !s.saw(http.MethodGet, "/check-runs") || !s.saw(http.MethodGet, "/status") {
		t.Errorf("the checks-green gate did not read both rollups: %v", s.requests)
	}
}
