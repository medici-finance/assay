package main

// prstate_test.go — the pull-request state word the merge preconditions name their refusal by,
// pinned so a drift is red rather than only a different refusal reason.
//
//   - prStateWord: the forge's open/closed word plus its own merged flag and timestamp. A merged
//     pull request reads MERGED however the forge words its state, so the refusal names the
//     right reason.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestPRStateWord(t *testing.T) {
	cases := []struct {
		name string
		pr   deskkit.PullRequest
		want string
	}{
		{"open", deskkit.PullRequest{State: "open"}, "OPEN"},
		{"closed, not merged", deskkit.PullRequest{State: "closed"}, "CLOSED"},
		{"closed with the merged flag only", deskkit.PullRequest{State: "closed", Merged: true}, "MERGED"},
		{"closed with a merge timestamp only", deskkit.PullRequest{State: "closed", MergedAt: "2026-10-01T00:00:00Z"}, "MERGED"},
		{"padded state", deskkit.PullRequest{State: " open "}, "OPEN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr := tc.pr
			if got := prStateWord(&pr); got != tc.want {
				t.Fatalf("prStateWord = %q, want %q", got, tc.want)
			}
		})
	}
}
