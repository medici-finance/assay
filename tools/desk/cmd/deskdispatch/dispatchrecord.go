package main

// dispatchrecord.go — deskdispatch's half of the dispatch record: mint the
// per-run dispatch_ref after claim-acquire, record it in the agent worktree next to assay.runKey,
// and write one `dispatched` line after the model-stamp step. Every one of these is BEST-EFFORT:
// a failure is a WARNING and never fails a dispatch whose claim and worktree already stand — the
// record is for later analysis, and losing one line is cheaper than wedging or aborting a run.

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mintDispatchRef mints the run's dispatch_ref from the plan's clock and entropy source and
// stores it on o.run for the audit line. "" (with a WARNING) when the mint fails.
func mintDispatchRef(o dispatchOpts, plan dispatchPlan) string {
	now := plan.now
	if now == nil {
		now = dispatchClock
	}
	ref, err := deskkit.MintDispatchRef(plan.claimKey, now(), plan.entropy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskdispatch: WARNING: could not mint dispatch_ref for claim key %s: %v — "+
			"this run's dispatch_ref is null (no weaker source is used)\n", plan.claimKey, err)
		return ""
	}
	if o.run != nil {
		o.run.dispatchRef = ref
	}
	return ref
}

// recordDispatchRefInWorktree writes `git config --worktree assay.dispatchRef <ref>` in the
// agent's worktree — where deskclaim-ref reads it back at release, and where a verifier reads it
// from its own dispatched worktree. A failure is a WARNING, the same class as the run key.
func recordDispatchRefInWorktree(o dispatchOpts, home, ref string) {
	if ref == "" {
		return
	}
	if r := runCmd(home, "git", "config", "--worktree", "assay.dispatchRef", ref); r.err != nil {
		o.say("%s WARNING: could not record assay.dispatchRef in %s (%s) — the release line for this run "+
			"will carry a null dispatch_ref and pairs by claim key instead", stepWorktreeCreate, home, r.run.Said())
		return
	}
	o.say("%s OK: recorded dispatch ref (assay.dispatchRef) in %s", stepWorktreeCreate, home)
}

// modelStampOutcome maps the model-stamp step's report onto the record's closed vocabulary.
func modelStampOutcome(stamp string) string {
	switch {
	case strings.HasPrefix(stamp, "SKIPPED"):
		return deskkit.ModelStampSkipped
	case strings.HasPrefix(stamp, "PENDING"):
		return deskkit.ModelStampPending
	default:
		return deskkit.ModelStampApplied
	}
}

// writeDispatchRecord appends the run's `dispatched` line. Never fatal: a write failure prints
// the WARNING and the dispatch carries on to its queue label and prompt.
func writeDispatchRecord(o dispatchOpts, plan dispatchPlan, ref, stamp string) {
	rec := buildDispatchRecord(o, plan, ref, stamp)
	if err := deskkit.AppendDispatchRecord(&rec); err != nil {
		fmt.Fprintf(os.Stderr, "deskdispatch: WARNING: could not write dispatch record: %v\n", err)
	}
}

func buildDispatchRecord(o dispatchOpts, plan dispatchPlan, ref, stamp string) deskkit.DispatchRecord {
	opt := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	item := o.item
	if o.itemAlias != "" {
		item = o.itemAlias + ":" + o.item
	}
	var branch *string
	if !plan.detached {
		branch = opt(plan.branch)
	}
	var pr *int
	if o.pr > 0 {
		n := o.pr
		pr = &n
	}
	bf := readBriefRecordFields(o.root, o.brief)
	// A frontmatter value outside the record's vocabulary is dropped to null, never guessed and
	// never allowed to refuse the whole record.
	execTier := bf.execTier
	if execTier != nil && !isDispatchTierValue(*execTier) {
		execTier = nil
	}
	effort := bf.effort
	if effort != nil && *effort != "S" && *effort != "M" && *effort != "L" {
		effort = nil
	}
	briefID := bf.id
	if briefID != nil && !deskkit.ValidDispatchRecordString(*briefID) {
		briefID = nil
	}
	// --tier is accepted in any case and with surrounding space (validTier); the record carries
	// the canonical token, from the same normalisation the stamp label uses.
	tier := o.tier
	if c, ok := deskkit.CanonicalDispatchTier(o.tier); ok {
		tier = c
	}
	ms := modelStampOutcome(stamp)
	return deskkit.DispatchRecord{
		Event:       deskkit.DispatchEventDispatched,
		DispatchRef: opt(ref),
		ClaimKey:    plan.claimKey,
		Repo:        plan.repo,
		Item:        opt(item),
		Brief:       briefID,
		Kit:         opt(o.kit),
		Branch:      branch,
		PR:          pr,
		SessionTag:  deskkit.SessionTag(),
		Tier:        opt(tier),
		BriefExec:   execTier,
		BriefEffort: effort,
		ModelStamp:  &ms,
	}
}

func isDispatchTierValue(v string) bool {
	for _, t := range deskkit.DispatchTiers() {
		if v == t {
			return true
		}
	}
	return false
}
