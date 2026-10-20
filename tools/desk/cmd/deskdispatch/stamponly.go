package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This is a bounded entry to the existing stamp step, never a dispatch with steps
// skipped. Its flag set cannot carry claims, allocations, hooks, or prompt output.
func executeStampOnly(o dispatchOpts) (err error) {
	defer func() {
		// Dry-run does not even append a local audit record.
		if !o.dryRun {
			audit(o, err)
		}
	}()
	if o.attestation < 0 {
		return deskkit.Refused("stamp-only: --attestation must be a positive record number")
	}
	if o.pr <= 0 || strings.TrimSpace(o.repo) == "" || strings.TrimSpace(o.model) == "" || o.tier == "" {
		return deskkit.Refused("stamp-only requires --repo, positive --pr, explicit --model and --tier")
	}
	if _, e := stampKindForKit(o.kit).Labels(o.model, o.tier); e != nil {
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
