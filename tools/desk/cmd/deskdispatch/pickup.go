package main

// pickup.go — the context pickup admission (statusgen/21), taken BEFORE the claim.
//
// An implementer dispatch may carry the context receipt its brief was authored against
// (--context-receipt, a context-handoff record) and the current context snapshot
// (--context-snapshot, the directory `statusgen context` reads). deskdispatch then runs the
// shipped statusgen/20 recheck in process, through the same statusgen/contexthandoff package
// the `statusgen context recheck` verb calls, and:
//
//   - admits a receipt that is still current, or whose changes are unrelated edits, ignored
//     draft rivals or product changes to assess, and carries the recheck into the worker
//     kit's prompt so the worker's Architecture pickup assessment starts from it;
//   - holds a receipt the recheck says must be reconciled (a material conflict) or that
//     cannot be confirmed (an unavailable input or partial coverage), before any claim,
//     worktree or child process exists;
//   - refuses a receipt whose schema version this build cannot validate, naming the manual
//     bootstrap. It is never admitted unchecked.
//
// With neither flag, nothing changes: the kit's Architecture pickup section is the manual
// bootstrap (plugins/assay/references/implementation-pickup.md).

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// pickupHeldMarker opens the refusal a pickup hold returns.
const pickupHeldMarker = "dispatch HELD on context pickup"

// pickupMaxReceipt bounds the receipt read, like `statusgen context`'s own input bound.
const pickupMaxReceipt = 8 << 20

// pickupSchemas are the context-handoff versions this build validates.
var pickupSchemas = []string{contexthandoff.HandoffSchema, contexthandoff.HandoffSchemaV2, contexthandoff.HandoffSchemaV3}

// pickupResult is an admitted recheck, as the prompt reports it.
type pickupResult struct {
	prior, merged, schema, coverage, scope string
	findings                               []contexthandoff.Finding
}

// pickupAdmit rechecks the dispatch's context receipt against its snapshot. It returns nil
// when no receipt was given.
func pickupAdmit(o dispatchOpts) (*pickupResult, error) {
	snap, rcpt := strings.TrimSpace(o.contextSnapshot), strings.TrimSpace(o.contextReceipt)
	if snap == "" && rcpt == "" {
		return nil, nil
	}
	if snap == "" || rcpt == "" {
		return nil, refusePickup("--context-snapshot and --context-receipt are given together or not at all")
	}
	if !workerKit(o.kit) {
		return nil, refusePickup(fmt.Sprintf("context pickup admission applies to the implementer kits, not --kit %s", o.kit))
	}
	raw, err := readPickupFile(rcpt)
	if err != nil {
		return nil, holdPickup("the context receipt could not be read (%v): could-not-check", err)
	}
	prior, err := contexthandoff.DecodeHandoff(raw)
	if err != nil {
		if contexthandoff.RuleOf(err) == contexthandoff.RuleSchemaVersion {
			return nil, refusePickup(fmt.Sprintf("the context receipt's schema is not one this deskdispatch validates (%s): %v. "+
				"It is not admitted unchecked. Dispatch without --context-receipt and follow the manual bootstrap "+
				"(implementation-pickup.md), or upgrade to an Assay release that ships that schema.",
				strings.Join(pickupSchemas, ", "), err))
		}
		return nil, refusePickup(fmt.Sprintf("the context receipt does not validate: %v", err))
	}
	root, err := os.OpenRoot(snap)
	if err != nil {
		return nil, holdPickup("the context snapshot could not be opened (%v): could-not-check", err)
	}
	defer root.Close()
	s, err := contexthandoff.LoadSnapshot(root.FS(), contexthandoff.DefaultLimits)
	if err != nil {
		return nil, holdPickup("the context snapshot could not be read (%v): could-not-check", err)
	}
	rc, err := contexthandoff.Recheck(prior, s)
	if err != nil {
		return nil, holdPickup("the recheck could not run (%v): could-not-check", err)
	}
	switch {
	case rc.Reconcile():
		return nil, holdPickup("receipt %s must be reconciled before implementation: %s", prior.Digest(), pickupFindings(rc.Findings, contexthandoff.FindingReconcile))
	case rc.Hold():
		return nil, holdPickup("receipt %s cannot be confirmed current: %s", prior.Digest(), pickupFindings(rc.Findings, contexthandoff.FindingHold))
	}
	r := &pickupResult{prior: prior.Digest(), merged: rc.Receipt.Digest(), schema: rc.Receipt.Schema,
		coverage: rc.Receipt.Discovery.Coverage.State, scope: contexthandoff.ArchitectureOnly, findings: rc.Findings}
	if rc.Receipt.Product != nil {
		r.scope = contexthandoff.ProductAssessed
	}
	return r, nil
}

func readPickupFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, pickupMaxReceipt+1))
	if err != nil {
		return nil, err
	}
	if len(b) > pickupMaxReceipt {
		return nil, fmt.Errorf("larger than %d bytes", pickupMaxReceipt)
	}
	return b, nil
}

func pickupFindings(fs []contexthandoff.Finding, class string) string {
	var out []string
	for _, f := range fs {
		if f.Class == class {
			out = append(out, f.Scope+": "+f.Reason)
		}
	}
	return strings.Join(out, "; ")
}

func refusePickup(why string) error {
	return deskkit.Refused(fmt.Sprintf("step %s: %s. Nothing was claimed and no worktree was created.", stepClaimAcquire, why))
}

func holdPickup(format string, a ...any) error {
	return deskkit.Refused(fmt.Sprintf("%s — step %s: %s. Nothing was claimed and no worktree was created; this is a delay, "+
		"not a verdict. Reconcile or restore the context, then dispatch again.", pickupHeldMarker, stepClaimAcquire, fmt.Sprintf(format, a...)))
}

// writePickupReceipt carries an admitted recheck into an implementer prompt.
func writePickupReceipt(b *strings.Builder, o dispatchOpts, p *pickupResult) {
	if p == nil || !workerKit(o.kit) {
		return
	}
	b.WriteString("\n## Context pickup receipt\n\n")
	fmt.Fprintf(b, "deskdispatch rechecked context receipt `%s` against the current snapshot before the claim "+
		"(statusgen context recheck semantics) and admitted it. Start your Architecture pickup assessment from it; "+
		"do not create a second register.\n\n", p.prior)
	fmt.Fprintf(b, "- **Merged receipt:** `%s` (%s)\n", p.merged, p.schema)
	fmt.Fprintf(b, "- **Coverage:** %s; scope: %s\n", p.coverage, p.scope)
	if len(p.findings) == 0 {
		b.WriteString("- **Findings:** none: the receipt is current\n")
	}
	for _, f := range p.findings {
		fmt.Fprintf(b, "- **%s:** %s: %s\n", f.Class, f.Scope, f.Reason)
	}
	b.WriteString("\nRecheck it with `statusgen context recheck` on resume and after a material scope or target change.\n")
}
