package main

import (
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The post-flip backstop's timing (#6242). Production uses deskkit's 30s bound; tests
// shorten both and replace the sleep so they run instantly.
var (
	backstopDeadline = deskkit.ReadyBackstopDeadline
	backstopInterval = deskkit.ReadyBackstopInterval
	backstopSleep    = time.Sleep
	backstopNow      = time.Now
)

// readyBackstop runs deskkit.ReadyBackstop right after `ready`'s flip landed: it reads the
// forge's merge state and, when the forge reports BLOCKED or DIRTY, returns the PR to draft
// with one comment. A could-not-verify notice goes to stderr; out.Err is non-nil when the
// flip was reverted (Refused) or the revert failed (Unverifiable). Ruling:
// https://github.com/medici-finance/assay/issues/6242#issuecomment-6812384155
func readyBackstop(client postBackend, repo string, pr int, nodeID, head string) deskkit.BackstopOutcome {
	out := deskkit.ReadyBackstop{
		Repo:    repo,
		Number:  pr,
		HeadSHA: head,
		ReadState: func() (string, error) {
			info, err := client.getPR(pr)
			if err != nil {
				return "", err
			}
			return info.MergeableState, nil
		},
		ConvertToDraft: func() error { return client.convertToDraft(nodeID) },
		Comment:        func(body string) error { return client.postComment(pr, body) },
		Deadline:       backstopDeadline,
		Interval:       backstopInterval,
		Sleep:          backstopSleep,
		Now:            backstopNow,
	}.Run()
	if out.Notice != "" {
		fmt.Fprintf(stderr, "deskpost: %s\n", out.Notice)
	}
	return out
}
