package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// actions.go — what a non-dry-run tick actually DOES for RECLAIM-ELIGIBLE and
// BLOCKED-TIMEOUT. Both are seams (reclaimFunc / fileBlockedTimeoutFunc) so sweep's dry-run
// suppression is testable without a real forge or a real deskfile binary; production wires
// the two functions at the bottom of this file.
//
// # Why DeleteRef, not deskkit.ReleaseMatching
//
// The brief's own fact section says the reclaim "frees the claim through
// deskkit.ReleaseMatching (compare-and-delete under the claims lock)" — that IS what
// liveness.go's own internal reclaimForLiveness does, but reclaimForLiveness reclaims
// against cfg.ClaimsDir, the LOCAL flock-backed claims directory (internal/deskkit/claim.go).
// A dispatch claim has not lived there since 2026-08-13 (cmd/deskclaim/main.go's own SCOPE
// NOTE: "the dispatch kind no longer belongs here... now the GitHub ref
// refs/dispatch/<id>"), and cmd/fanoutloop/land.go's forgeDispatchSink — the one other place
// in this tree that releases a dispatch claim — releases it through
// deskkit.Forge.DeleteRef(repo, "dispatch/<key>"), never through ReleaseMatching. Since the
// claims desksupervise's Task exists to reclaim ARE dispatch claims (the brief's own
// motivating scenario is a wedged WORKER's dispatch), this file follows land.go's
// established, currently-correct path rather than the brief's fact citation, which
// describes a DIFFERENT (pre-2026-08-13 / non-dispatch-kind) claim mechanism. See the PR
// body's Context section for this note quoted in full — clause 7 ("verify before you apply
// a correction") binds this disagreement, not silence.

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

// doReclaim is the production reclaimFunc: obtain the forge that serves the claim's repo
// through deskkit.ForgeFor (the single construction site, which resolves the forge and reads
// the resolved role's already-minted token from custody) and delete the `dispatch/<key>` ref
// through deskkit.Forge, exactly as cmd/fanoutloop/land.go's forgeDispatchSink does. A ref
// already gone (404/422 — refAlreadyGone's condition in land.go) is a no-op, not a failure:
// the claim is not held either way.
func doReclaim(claim claimRecord) error {
	owner, name, ok := strings.Cut(claim.Repo, "/")
	if !ok || owner == "" || name == "" {
		return fmt.Errorf("reclaim %s: repo %q is not owner/name", claim.Key, claim.Repo)
	}
	role, _, rerr := deskkit.SessionTokenRole("desksupervise")
	if rerr != nil {
		return fmt.Errorf("reclaim %s: cannot resolve a token role: %w", claim.Key, rerr)
	}
	repo := deskkit.ForgeRepo{Owner: owner, Name: name}
	forge, ferr := deskkit.ForgeFor(repo, role)
	if ferr != nil {
		return fmt.Errorf("reclaim %s: cannot resolve a forge for %s: %w", claim.Key, claim.Repo, ferr)
	}
	ref := "dispatch/" + claim.Key
	if err := forge.DeleteRef(repo, ref); err != nil {
		if deskkit.IsForgeNotFound(err) {
			return nil
		}
		return fmt.Errorf("reclaim %s: DeleteRef refs/%s in %s: %w", claim.Key, ref, claim.Repo, err)
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
		return action, nil
	case loopengine.BlockedStartToClose:
		action = "BLOCKED-TIMEOUT"
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] would file a help-wanted issue for %s (blocked-timeout)\n", claim.Key)
			return action, nil
		}
		if ferr := fileBT(claim); ferr != nil {
			return action, ferr
		}
		loopengine.JournalObserverDecision("desksupervise", loopengine.EventLand, claim.Item, claim.Tier, "blocked-timeout", disp.String(), runTag)
		return action, nil
	default:
		return "none", nil
	}
}
