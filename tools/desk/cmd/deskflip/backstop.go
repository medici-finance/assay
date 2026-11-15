package main

import (
	"fmt"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The post-flip backstop's bound and re-read interval. Package vars ONLY so in-package tests
// run instantly; production uses the ruled 30s bound (deskkit.ReadyBackstopDeadline).
var (
	backstopDeadline = deskkit.ReadyBackstopDeadline
	backstopInterval = deskkit.ReadyBackstopInterval
	backstopSleep    = time.Sleep
	backstopNow      = time.Now
)

// readyBackstop runs deskkit.ReadyBackstop right after a successful ready mutation: ready
// means mergeable per the forge. A DIRTY merge state, or a BLOCKED one with a merge requirement
// unmet or unreadable (medici-finance/assay#6508), returns the change to draft and comes back
// as the Refused error the verb exits with; a failed revert comes back as
// Unverifiable; a verdict that could not be read leaves the change ready and prints the
// could-not-verify notice. Ruling:
// https://github.com/medici-finance/assay/issues/6242#issuecomment-6812384155
func readyBackstop(fg deskkit.Forge, fr deskkit.ForgeRepo, number int, change *deskkit.PullRequest, head string) error {
	out := deskkit.ReadyBackstop{
		Repo: fr.Slug(), Number: number, HeadSHA: head,
		// The draft flag travels with the merge state: GitHub reports a draft as BLOCKED, so a
		// read that still shows the change as a draft is settling, never a revert.
		ReadState: func() (string, bool, error) {
			p, err := fg.GetPullRequest(fr, number)
			if err != nil {
				return "", false, err
			}
			return p.MergeStateStatus, p.Draft, nil
		},
		// The opaque id is the one the change read returned, as for the flip itself.
		ConvertToDraft: func() error { return fg.ConvertToDraft(change.NodeID) },
		Comment: func(body string) error {
			// Typed (#6264): the backstop comments on the change it just flipped.
			_, err := fg.PostCommentTyped(fr, number, deskkit.TargetChange, body)
			return err
		},
		// A BLOCKED read is judged by the requirements every reader sees alike (#6508).
		Requirements: func() (*deskkit.MergeRequirements, error) { return fg.MergeRequirements(fr, number) },
		Deadline:     backstopDeadline, Interval: backstopInterval, Sleep: backstopSleep, Now: backstopNow,
	}.Run()
	if out.Notice != "" {
		fmt.Fprintf(os.Stderr, "deskflip: %s\n", out.Notice)
	}
	return out.Err
}
