package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestFetchOpenPRs_UnreadableWorkflowRun_DoesNotBlindBoard is the assay-toolkit#2024
// regression: the bulk open-PR read must not depend on the `checkSuite.workflowRun` link,
// which requires `actions:read`. gh's built-in `pr list --json statusCheckRollup` requests
// that sub-field; under an App holding only `checks:read` (the reviewer App) it 403s, and on
// a repo with many Actions check suites `gh pr list` returns those errors with NO salvageable
// partial stdout — it exits non-zero and empty, and fetchOpenPRs wrapped that Unverifiable
// (exit 6), blinding the whole cross-repo board on the first repo alphabetically.
//
// This models that identity: a fake gh that FAILS (fatally, no partial) any bulk read still
// requesting the built-in statusCheckRollup via `pr list`, or any read that requests the
// workflowRun/checkSuite sub-field at all, and answers only a read that omits it. Before the
// fix fetchOpenPRs issued `gh pr list --json …statusCheckRollup`, so this fake fails it and
// the test is RED; after the fix it issues a hand-authored `gh api graphql` that never asks
// for workflowRun, the fake answers, and the PR (with its rollup) parses — GREEN.
func TestFetchOpenPRs_UnreadableWorkflowRun_DoesNotBlindBoard(t *testing.T) {
	var gotArgs []string
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		gotArgs = args

		// A read that requests the Actions run link at all is refused, the way the
		// checks:read-only App is refused it — whether it arrives via gh's built-in
		// statusCheckRollup field or an explicit sub-selection.
		if strings.Contains(joined, "workflowRun") || strings.Contains(joined, "checkSuite") {
			return nil, errors.New("gh: HTTP 403: Resource not accessible by integration " +
				"(checkSuite.workflowRun) — token lacks actions:read")
		}
		// gh's `pr list --json statusCheckRollup` pulls in workflowRun implicitly, and on a
		// PR-heavy repo returns hundreds of those 403s as a fatal, empty-stdout failure.
		if strings.Contains(joined, "pr list") && strings.Contains(joined, "statusCheckRollup") {
			return nil, errors.New("gh: pr list statusCheckRollup: 403 on checkSuite.workflowRun " +
				"(no partial stdout)")
		}

		// A read that omits the Actions link is served — checks:read covers every
		// conclusion (CheckRun.conclusion, StatusContext.state). The bytes are the SAME
		// flat shape gh api graphql's --jq emits in production.
		return []byte(`[{"number":7,"title":"t","state":"OPEN","isDraft":true,` +
			`"author":{"login":"assay-worker-app[bot]"},"createdAt":"2026-01-01T00:00:00Z",` +
			`"headRefOid":"abc123","headRefName":"feat/x","baseRefName":"main",` +
			`"mergeStateStatus":"BLOCKED","statusCheckRollup":[` +
			`{"__typename":"CheckRun","name":"ci","status":"COMPLETED","conclusion":"SUCCESS"},` +
			`{"__typename":"StatusContext","context":"legacy","state":"SUCCESS"}]}]`), nil
	})

	prs, _, err := fetchOpenPRs("example-org/tracker")
	if err != nil {
		t.Fatalf("fetchOpenPRs must not fail when checkSuite.workflowRun is unreadable; "+
			"the board is blinded otherwise. got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if len(prs) != 1 || prs[0].Number != 7 {
		t.Fatalf("expected the one open PR to be read; got %+v", prs)
	}
	// The rollup the board classifies on must survive — both node shapes, from checks:read.
	if got := len(prs[0].StatusCheckRollup); got != 2 {
		t.Fatalf("expected 2 rollup contexts (CheckRun + StatusContext); got %d", got)
	}
	pass, pending, fail, unknown := ciState(prs[0])
	if pass != 2 || pending != 0 || fail != 0 || unknown != 0 {
		t.Errorf("rollup must classify as 2 pass; got pass=%d pending=%d fail=%d unknown=%d",
			pass, pending, fail, unknown)
	}

	// Belt-and-braces: the read we actually issued must never request the actions:read field.
	joined := strings.Join(gotArgs, " ")
	if strings.Contains(joined, "workflowRun") || strings.Contains(joined, "checkSuite") {
		t.Errorf("the open-PR read must not request checkSuite.workflowRun (needs actions:read); "+
			"args were: %s", joined)
	}
}
