package main

// workerattestation.go — --attest-worker, the opt-in pre-work attestation for a FRESH worker
// dispatch (#3056). See deskkit/workerattestation.go for the record itself.
//
// Without the flag nothing here runs: a fresh worker dispatch still reports its model stamp
// PENDING and the coordinator stamps the change after it opens (STAMP AFTER OPEN). With it, step 5
// creates, stamps, closes and reads back a dispatcher-authored record BEFORE the prompt is emitted;
// any failure releases the claim and emits no prompt (exit 6). The later --stamp-only call reads
// the record named on the PR (or by --attestation) and refuses a mismatched selection or branch.

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// validateWorkerAttestation is --attest-worker's caller precondition, run pre-claim from
// validateCallerPreconditions: everything the caller controls is decided before durable state.
func validateWorkerAttestation(o dispatchOpts) error {
	if !o.attestWorker {
		return nil
	}
	if role, ok := kitRole(o.kit); !ok || role != "worker" {
		return deskkit.Refused(fmt.Sprintf("step %s: --attest-worker attests a worker-kit dispatch only, not --kit %s", stepModelStamp, o.kit))
	}
	if o.pr != 0 {
		return deskkit.Refused(fmt.Sprintf("step %s: --attest-worker is the pre-work attestation for a dispatch with no change yet; "+
			"with --pr the open change itself is stamped", stepModelStamp))
	}
	if strings.TrimSpace(o.model) == "" {
		return deskkit.Refused(fmt.Sprintf("step %s: --attest-worker requires an explicit --model (and --tier); unknown is never a default model", stepModelStamp))
	}
	if _, isIssue := issueNumFromItem(o.item); o.brief == "" && !isIssue {
		return deskkit.Refused(fmt.Sprintf("step %s: --attest-worker binds the run's source: pass --brief, or dispatch an issue-<N> item", stepModelStamp))
	}
	role, _, err := deskkit.SessionTokenRole(toolName)
	if err != nil {
		return err
	}
	if role != deskkit.DispatcherRole && role != "worker" {
		return deskkit.Refused("--attest-worker requires a dispatching desk or worker-desk session")
	}
	return deskkit.WorkerAttestationAuthorityReady()
}

// workerAttestationSource is the source the record binds: the brief the prompt names, or the
// issue of an issue-<N> item when no brief was given.
func workerAttestationSource(o dispatchOpts) (brief string, issue int) {
	if o.brief != "" {
		return o.brief, 0
	}
	n, _ := issueNumFromItem(o.item)
	return "", n
}

// attestWorkerDispatchFn is the step-5 seam: tests drive the failure branches through it.
var attestWorkerDispatchFn = attestWorkerDispatch

// workerAttestationForgeFn resolves the forge the record is written through: the worker kit's
// stamp role (the dispatcher), the same custody a --pr dispatch stamps its change under.
var workerAttestationForgeFn = func(repo string, kit string) (deskkit.Forge, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, err
	}
	f, _, err := deskkit.ResolveForge(fr, stampRoleForKit(kit))
	return f, err
}

func attestWorkerDispatch(o dispatchOpts, plan dispatchPlan, repo, home string) (deskkit.WorkerReceipt, error) {
	f, err := workerAttestationForgeFn(repo, o.kit)
	if err != nil {
		return deskkit.WorkerReceipt{}, deskkit.Unverifiable("cannot resolve the dispatcher forge for the worker attestation", err)
	}
	brief, issue := workerAttestationSource(o)
	b, err := deskkit.PrepareWorkerBinding(f, deskkit.WorkerBindingInput{
		Repo: repo, Item: plan.claimKey, Kit: o.kit, Branch: plan.branch, Home: home,
		BriefPath: brief, SourceIssue: issue, Model: o.model, Tier: o.tier,
	})
	if err != nil {
		return deskkit.WorkerReceipt{}, err
	}
	return deskkit.IssueWorkerAttestation(f, b)
}

// workerStampLine is step 5's report for an admitted run. It starts with OK, so the dispatch
// record states the stamp applied; the change's own stamp is still the coordinator's, after open.
func workerStampLine(o dispatchOpts, repo string, r deskkit.WorkerReceipt) string {
	return fmt.Sprintf("OK: pre-work worker attestation applied and verified (%s); %s. The change stamp stays "+
		"post-open: coordinator runs deskdispatch --stamp-only --repo %s --pr <N> --model %s --tier %s --kit %s --attestation %d",
		r.URL, r.Receipt(), repo, o.model, o.tier, o.kit, r.Issue)
}

// writeWorkerAttestationSection is the prompt section an attested worker run receives in place of
// the plain pending-stamp handoff. A nil receipt (a dry run) renders placeholders.
func writeWorkerAttestationSection(b *strings.Builder, o dispatchOpts, repo string, r *deskkit.WorkerReceipt) {
	line, num := deskkit.WorkerAttestationLinePrefix+" <created at step "+stepModelStamp+">", "<attestation>"
	if r != nil {
		line, num = r.PRBinding(), fmt.Sprint(r.Issue)
	}
	fmt.Fprintf(b, "\n## Pre-work worker attestation\n\nThis run was admitted under a dispatcher-authored attestation record, stamped and read back before this prompt was emitted. "+
		"Put this exact line, on its own line, in the draft PR's description:\n\n```\n%s\n```\n\n"+
		"After opening the draft PR, return its number to the dispatching worker-desk. That desk sends the PR and this exact dispatch model/tier selection to the coordinator desk (the-desk), which runs:\n\n"+
		"```\ndeskdispatch --stamp-only --repo %s --pr <N> --model %s --tier %s --kit %s --attestation %s\n```\n\n"+
		"That call refuses when the PR's branch, repository, kit or the stated model/tier disagree with the attestation; it never takes the model from the record. "+
		"Both worker-desk and its child worker are refused by this verb; a shared DESK_SESSION is claim custody, not stamp authority. Do not run this command as the worker or change your session identity to apply it. "+
		"It writes the authoring pair (authored-model:, authored-tier:), never the dispatched-* pair a later review dispatch writes.\n",
		line, repo, o.model, o.tier, o.kit, num)
}

// readWorkerAttestationFn is the stamp-only read seam; the reader itself is tested in deskkit.
var readWorkerAttestationFn = deskkit.ReadWorkerAttestation

// checkStampOnlyAttestation binds a post-open stamp to the worker attestation the change names
// (its PR-body line) or the coordinator names (--attestation). Neither present: the plain STAMP
// AFTER OPEN path, unchanged. A named record that is unreadable is could-not-check (6); a record
// that disagrees, or two names that disagree, refuses (5). Nothing is written by this check.
func checkStampOnlyAttestation(o dispatchOpts, fg deskkit.Forge, fr deskkit.ForgeRepo, change *deskkit.PullRequest) (string, error) {
	if stampKindForKit(o.kit) != deskkit.StampAuthored {
		// A review or verifier stamp is the session's own attestation; the change's authoring
		// attestation neither binds nor admits it.
		if o.attestation > 0 {
			return "", deskkit.Refused(fmt.Sprintf("stamp-only: --attestation binds a worker-kit authoring stamp, not --kit %s", o.kit))
		}
		return "", nil
	}
	ref, onChange, err := deskkit.ParseWorkerAttestationLine(change.Body)
	if err != nil {
		return "", err
	}
	if onChange && ref.Repo != o.repo {
		return "", deskkit.Refused(fmt.Sprintf("stamp-only: the change names a worker attestation in %s, not %s; nothing written", ref.Repo, o.repo))
	}
	n := o.attestation
	if onChange {
		if n > 0 && n != ref.Issue {
			return "", deskkit.Refused(fmt.Sprintf("stamp-only: --attestation %d disagrees with the change's %s#%d; nothing written", n, ref.Repo, ref.Issue))
		}
		n = ref.Issue
	}
	if n <= 0 {
		return "", nil
	}
	r, err := readWorkerAttestationFn(fg, fr, n)
	if err != nil {
		return "", err
	}
	if onChange && ref.Run != r.Binding.Run {
		return "", deskkit.Refused(fmt.Sprintf("stamp-only: the change's run %s is not attestation %s#%d's run; nothing written", ref.Run, o.repo, n))
	}
	if err := deskkit.CheckWorkerChangeBinding(r, o.repo, o.kit, o.model, o.tier, change.HeadRef); err != nil {
		return "", err
	}
	return fmt.Sprintf(" (bound to worker attestation %s#%d run=%s)", o.repo, n, r.Binding.Run), nil
}
