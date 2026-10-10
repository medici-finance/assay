package main

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// actions.go — what a non-dry-run tick actually DOES for RECLAIM-ELIGIBLE and
// BLOCKED-TIMEOUT. Both are seams (reclaimFunc / fileBlockedTimeoutFunc) so sweep's dry-run
// suppression is testable without a real forge or a real deskfile binary; production wires
// the two functions at the bottom of this file.
//
// reclaimFunc releases one claim so it becomes re-dispatchable. Production deletes the
// forge ref; a test injects a recording fake.
type reclaimFunc func(claim claimRecord) error

// fileBlockedTimeoutFunc files (or, on a repeat, no-ops against deskfile's own dedupe) a
// help-wanted issue naming a blocked-timeout claim. Production shells to deskfile; a test
// injects a recording fake.
type fileBlockedTimeoutFunc func(claim claimRecord) error

// armRunStopFunc arms the per-run stop flag for a claim before its reclaim, so a still-live
// wedged worker is halted by deskkit.Guard on its next desk verb even after its claim is
// freed for re-dispatch (Layer A of the two-layer per-run stop). Production writes the state-dir
// flag through deskkit.ArmRunStop; a test injects a recording fake — arming is a real
// filesystem write with real consequences (it can STOP a production run), so it is a seam
// for the same reason reclaim and fileBlockedTimeout are.
type armRunStopFunc func(claim claimRecord, reason string) error

// doArmRunStop is the production armRunStopFunc: write the STOP.run.<key> flag through the
// single deskkit writer so its format has one author.
func doArmRunStop(claim claimRecord, reason string) error {
	return deskkit.ArmRunStop(claim.Key, reason)
}

// doReclaim deletes only the immutable claim version whose holder and activity
// were assessed. Both terminal reconciliation and liveness reclaim use this path.
func doReclaim(claim claimRecord) error {
	opts, err := claimListOpts(claim.Repo)
	if err != nil {
		return err
	}
	return reclaimVersion(claim, opts, gitcore.PushRefUpdate)
}
func reclaimVersion(claim claimRecord, opts gitcore.ListOpts, push func(context.Context, gitcore.RefUpdate) (gitcore.RefUpdateResult, error)) error {
	expected := deskkit.DispatchClaimActiveRefsPrefix + claim.Key
	if claim.Key == "" || strings.Contains(claim.Key, "/") || claim.Ref != expected || len(claim.Version) != 40 || !plumbing.IsHash(claim.Version) || plumbing.NewHash(claim.Version) == plumbing.ZeroHash {
		return fmt.Errorf("reclaim %s: missing or invalid assessed claim version; held", claim.Key)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := push(ctx, gitcore.RefUpdate{URL: opts.URL, Auth: opts.Auth, Ref: plumbing.ReferenceName(claim.Ref), Old: plumbing.NewHash(claim.Version), New: plumbing.ZeroHash})
	if err != nil {
		return fmt.Errorf("reclaim %s: could not conditionally release: %w", claim.Key, err)
	}
	if result != gitcore.RefUpdateApplied {
		return fmt.Errorf("reclaim %s: claim changed or release refused; held for reassessment", claim.Key)
	}
	return nil
}

// doFileBlockedTimeout is the production fileBlockedTimeoutFunc: `deskfile new` with a
// deterministic title keyed on the claim's key, so a repeat tick's filing collides with
// deskfile's OWN dedupe-before-file search and comes back exit 5 ("already claimed") —
// that IS the "idempotent by marker" the Task asks for; desksupervise adds no marker file
// of its own. deskfile is invoked as a sibling binary (the same shell-to-desk-verb pattern
// deskdispatch uses for the consumer's own claim script), never re-implemented here.
func doFileBlockedTimeout(claim claimRecord) error {
	title := fmt.Sprintf("desksupervise: %s blocked-timeout (wall-cap exceeded)", claim.Key)
	body := fmt.Sprintf(
		"desksupervise's liveness observer classified dispatch claim `%s` OVER-WALL-CAP "+
			"(start-to-close expiry) for item `%s` on branch `%s`, dispatched at %s.\n\n"+
			"The worker is presumed still running (a wall-cap expiry is a pure clock check, "+
			"independent of heartbeat) but has exceeded its tier's start-to-close budget. The "+
			"claim was landed blocked-timeout rather than reclaimed, so it is NOT re-dispatched "+
			"automatically — a human should look at the run before deciding whether to retry it.\n",
		claim.Key, claim.Item, claim.Branch, claim.DispatchedAt)

	f, err := os.CreateTemp("", "desksupervise-blocked-timeout-*.md")
	if err != nil {
		return fmt.Errorf("file blocked-timeout for %s: cannot create a body file: %w", claim.Key, err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return fmt.Errorf("file blocked-timeout for %s: cannot write the body file: %w", claim.Key, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("file blocked-timeout for %s: cannot close the body file: %w", claim.Key, err)
	}

	cmd := exec.Command("deskfile", "new", "-R", claim.Repo, "--title", title,
		"--body-file", f.Name(), "--label", "help wanted", "--raised-by", "verifier")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == deskkit.ExitRefused {
		// deskfile's own dedupe found an existing filing for this key — that IS the
		// idempotent no-op this action wants, not a failure.
		return nil
	}
	return fmt.Errorf("file blocked-timeout for %s: deskfile new failed: %w (%s)", claim.Key, err, strings.TrimSpace(string(out)))
}

// runAction executes (or, under dryRun, only PRINTS) the action a Disposition implies, and
// journals the decision when it actually ran. It is the single place sweep calls into for
// side effects, so the dry-run suppression lives in exactly one function.
func runAction(claim claimRecord, disp loopengine.Disposition, dryRun bool, reclaim reclaimFunc, fileBT fileBlockedTimeoutFunc, arm armRunStopFunc, runTag string) (action string, err error) {
	switch disp {
	case loopengine.ReclaimNeverStarted, loopengine.ReclaimHeartbeat:
		action = "RECLAIM-ELIGIBLE"
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] would arm STOP.run.%s then release dispatch claim %s (%s)\n", claim.Key, claim.Key, disp)
			fireAfterRun(claim, true)
			return action, nil
		}
		// Arm the per-run stop BEFORE releasing the claim. A NEVER-STARTED / HEARTBEAT-EXPIRED
		// classification is the OBSERVER's evidence that the run is dead, not proof the worker
		// PROCESS is gone — so Layer A (Guard refuses the worker's next desk verb) must already
		// be armed when the claim is freed, or a still-live wedged worker races the fresh
		// dispatch onto the same item. Order is the property row 6 pins.
		if aerr := arm(claim, fmt.Sprintf("desksupervise reclaim: %s %s (%s)", disp, claim.Key, runTag)); aerr != nil {
			return action, aerr
		}
		if rerr := reclaim(claim); rerr != nil {
			return action, rerr
		}
		loopengine.JournalObserverDecision("desksupervise", loopengine.EventReclaim, claim.Item, claim.Tier, "", disp.String(), runTag)
		fireAfterRun(claim, false)
		return action, nil
	case loopengine.BlockedStartToClose:
		action = "BLOCKED-TIMEOUT"
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] would file a help-wanted issue for %s (blocked-timeout)\n", claim.Key)
			fireAfterRun(claim, true)
			return action, nil
		}
		if ferr := fileBT(claim); ferr != nil {
			return action, ferr
		}
		loopengine.JournalObserverDecision("desksupervise", loopengine.EventLand, claim.Item, claim.Tier, "blocked-timeout", disp.String(), runTag)
		fireAfterRun(claim, false)
		return action, nil
	default:
		return "none", nil
	}
}

// fireAfterRun runs the after_run lifecycle hook at the end of an observer action that
// released or landed a claim (an "attempt end" in Symphony's model). LOGGED failure class:
// an after_run failure is reported to stderr but never changes the action's outcome — the
// claim was already released/landed. Under dryRun it only reports the hook plan and touches
// nothing.
func fireAfterRun(claim claimRecord, dryRun bool) {
	if dryRun {
		if line, err := deskkit.HookDryRunLine(deskkit.HookAfterRun); err != nil {
			fmt.Fprintf(os.Stderr, "[dry-run] after_run hook plan unreadable for %s: %v\n", claim.Key, err)
		} else {
			fmt.Fprintf(os.Stderr, "[dry-run] %s (%s)\n", line, claim.Key)
		}
		return
	}
	if _, err := deskkit.RunHook(deskkit.HookAfterRun, deskkit.HookEnv{
		RunKey: claim.Key, Repo: claim.Repo,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "desksupervise: after_run hook failed for %s (action already completed): %v\n", claim.Key, err)
	}
}
