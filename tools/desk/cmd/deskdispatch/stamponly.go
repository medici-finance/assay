package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This is a bounded entry to the existing stamp step, never a dispatch with steps
// skipped. Its flag set cannot carry claims, allocations, hooks, or prompt output.
func cmdStampOnly(args []string) (err error) {
	fs := flag.NewFlagSet("deskdispatch --stamp-only", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	o := dispatchOpts{stampOnly: true}
	fs.StringVar(&o.repo, "repo", "", "target owner/repo (required)")
	fs.IntVar(&o.pr, "pr", 0, "already-open change number (required)")
	fs.StringVar(&o.model, "model", "", "exact model selected by the dispatcher (required)")
	fs.StringVar(&o.tier, "tier", "", "selected execution tier (required)")
	fs.StringVar(&o.kit, "kit", "worker", "original dispatch kit")
	fs.BoolVar(&o.dryRun, "dry-run", false, "read and report without mutation")
	defer func() {
		// Dry-run does not even append a local audit record.
		if !o.dryRun {
			audit(o, err)
		}
	}()
	if e := fs.Parse(args); e != nil {
		return deskkit.Refused("stamp-only: " + e.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("stamp-only accepts flags only, no item key")
	}
	if o.pr <= 0 || strings.TrimSpace(o.repo) == "" || strings.TrimSpace(o.model) == "" || o.tier == "" {
		return deskkit.Refused("stamp-only requires --repo, positive --pr, explicit --model and --tier")
	}
	if _, e := deskkit.ModelStampLabels(o.model, o.tier); e != nil {
		return deskkit.Refused(e.Error())
	}
	if _, e := kitText(o.kit); e != nil {
		return e
	}
	if _, e := forgeRepoOf(o.repo); e != nil {
		return deskkit.Refused(e.Error())
	}
	if !deskkit.IsAllowedRepo(o.repo) {
		return deskkit.Refused("stamp-only: repo is outside the desk repo set")
	}
	role, _, e := deskkit.SessionTokenRole(toolName)
	if e != nil {
		return deskkit.Refused("stamp-only requires the original dispatcher's session: " + e.Error())
	}
	if role != stampRoleForKit(o.kit) {
		return deskkit.Refused(fmt.Sprintf("stamp-only: session role %q is not the dispatcher role %q for kit %q; the dispatched worker cannot attest itself", role, stampRoleForKit(o.kit), o.kit))
	}
	if _, bound := deskkit.RoleAppLogin(role); !bound {
		return deskkit.Refused("stamp-only: dispatcher role has no roster binding")
	}
	line, e := stepStamp(o, o.repo)
	if e != nil {
		return e
	}
	o.stampReceipt = line
	fmt.Fprintln(os.Stdout, "deskdispatch: stamp-only "+line)
	return nil
}

// Both dispatch entry points use the same reader as the capability floor. A 2xx
// label write is not evidence that the requested, attributable pair is standing.
func verifyStamp(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, want []string) error {
	expected, _ := deskkit.ModelStampOf(want)
	change, err := fg.GetPullRequest(fr, pr)
	if err != nil {
		return deskkit.Unverifiable("model-stamp: post-write change read failed; stamp is NOT verified", err)
	}
	if change == nil {
		return deskkit.Unverifiable("model-stamp: post-write change missing; stamp is NOT verified", nil)
	}
	events, err := fg.ListLabelEvents(fr, pr)
	if err != nil {
		return deskkit.Unverifiable("model-stamp: post-write timeline read failed; stamp is NOT verified", err)
	}
	stamp, state := deskkit.AttestedModelStampOf(deskkit.StampTimeline{Present: change.Labels, Events: events}, deskkit.IsStampAuthorityLogin)
	if state != deskkit.ModelStamped || stamp != expected {
		return deskkit.Unverifiable(fmt.Sprintf("model-stamp: post-write stamp is NOT verified (state=%s); requested model=%s tier=%s", state, expected.Model, expected.Tier), nil)
	}
	return nil
}
