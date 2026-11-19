package deskkit

import (
	"fmt"
)

// AuthoringStampCondition is the ready-flip condition that holds an attested worker change
// until its authoring stamp stands (#6855). A change whose description names a
// `Worker-Attestation:` record must carry the dispatcher-applied authored-model / authored-tier
// pair that record selected before it may leave draft.
//
// WHY THE FLIP. `deskdispatch --stamp-only` refuses a merged change by design, so an authoring
// stamp that has not landed by merge can never land. Merge is human; ready is the last machine
// gate in front of it. Holding ready on the stamp makes "stamped before merge" hold even when
// the coordinator's sweep and the worker's hand-off both missed the change.
//
// It is ONE shared implementation for both ready-flip verbs (deskflip and `deskpost ready`):
// the two perform the identical mutation and must clear the identical floor, or the ungated one
// becomes the route around it.
const AuthoringStampCondition = "authoring-stamp"

// WorkerRecordReader reads and validates a worker attestation record. Production passes
// ReadWorkerAttestation bound to the verb's resolved forge.
type WorkerRecordReader func(repo ForgeRepo, n int) (WorkerReceipt, error)

// StampOnlyFixLine is the exact command that lands an attested worker change's authoring stamp.
// Every value comes from the validated record, so the operator never types a model or tier by
// hand.
func StampOnlyFixLine(r WorkerReceipt, pr int) string {
	return fmt.Sprintf("deskdispatch --stamp-only --repo %s --pr %d --attestation %d --model %s --tier %s --kit %s",
		r.Binding.Repo, pr, r.Issue, r.Binding.Model, r.Binding.Tier, r.Binding.Kit)
}

// AuthoringStampGate decides the authoring-stamp condition for change pr in repo. body is the
// change description as the verb just read it; tl is the change's label timeline (present labels
// plus attributed events). It returns the line the verb reports when the condition holds, or a
// Refused / Unverifiable error naming the condition.
//
//   - No `Worker-Attestation:` line: not an attested worker change, so the condition does not
//     apply and the flip proceeds.
//   - A malformed line, or a line naming a record in another repository, refuses: the binding
//     cannot be established and the stamp cannot be landed from here.
//   - The record must read and validate (ReadWorkerAttestation). A refusal keeps its class; a
//     read failure is could-not-check, never a pass.
//   - The record's run must be the line's run.
//   - The change's authored-* pair, attributed to a stamp authority, must be complete and equal
//     to the record's Model and Tier. Missing (unstamped), indeterminate, or different
//     (mismatched) each refuse and print the exact `--stamp-only` line that fixes it.
func AuthoringStampGate(repo ForgeRepo, pr int, body string, tl StampTimeline, read WorkerRecordReader) (string, error) {
	cond := AuthoringStampCondition
	ref, found, err := ParseWorkerAttestationLine(body)
	if err != nil {
		return "", Refused(fmt.Sprintf("condition %s: PR #%d: %s; the attestation binding cannot be established",
			cond, pr, err.Error()))
	}
	if !found {
		return fmt.Sprintf("no %s line in the change description: not an attested worker change",
			WorkerAttestationLinePrefix), nil
	}
	if ref.Repo != repo.Slug() {
		return "", Refused(fmt.Sprintf("condition %s: PR #%d names a worker attestation in %s, not %s; "+
			"its authoring stamp cannot be landed or checked from this repository", cond, pr, ref.Repo, repo.Slug()))
	}
	rec, err := read(repo, ref.Issue)
	if err != nil {
		msg := fmt.Sprintf("condition %s: PR #%d's worker attestation %s#%d did not validate: %s",
			cond, pr, ref.Repo, ref.Issue, err.Error())
		if ExitCodeOf(err) == ExitRefused {
			return "", Refused(msg)
		}
		return "", Unverifiable(msg+" (could-not-check is never a standing stamp)", err)
	}
	if rec.Binding.Run != ref.Run {
		return "", Refused(fmt.Sprintf("condition %s: PR #%d's line names run %s, but attestation %s#%d is run %s",
			cond, pr, ref.Run, ref.Repo, ref.Issue, rec.Binding.Run))
	}
	fix := StampOnlyFixLine(rec, pr)
	stamp, state := StampAuthored.AttestedStampOf(tl, IsStampAuthorityLogin)
	switch {
	case state == ModelUnknown:
		return "", Refused(fmt.Sprintf("condition %s: PR #%d is UNSTAMPED: it names worker attestation %s#%d "+
			"(model %s, tier %s) but carries no %s / %s pair. Land it before the flip: %s",
			cond, pr, ref.Repo, ref.Issue, rec.Binding.Model, rec.Binding.Tier, AuthoredModelPrefix, AuthoredTierPrefix, fix))
	case state != ModelStamped:
		return "", Refused(fmt.Sprintf("condition %s: PR #%d's authoring stamp is %s (incomplete, conflicting, or "+
			"applied by an identity that is not a stamp authority); attestation %s#%d selected model %s, tier %s. "+
			"Re-stamp: %s", cond, pr, state, ref.Repo, ref.Issue, rec.Binding.Model, rec.Binding.Tier, fix))
	case stamp.Model != rec.Binding.Model || stamp.Tier != rec.Binding.Tier:
		return "", Refused(fmt.Sprintf("condition %s: PR #%d's authoring stamp is MISMATCHED: it carries model %s, "+
			"tier %s, but attestation %s#%d selected model %s, tier %s. Re-stamp: %s",
			cond, pr, stamp.Model, stamp.Tier, ref.Repo, ref.Issue, rec.Binding.Model, rec.Binding.Tier, fix))
	}
	return fmt.Sprintf("authoring stamp %s/%s matches worker attestation %s#%d", stamp.Model, stamp.Tier,
		ref.Repo, ref.Issue), nil
}

// WorkerStampGap is the read-only listing's cheap test (#6855): from a change's description
// and PRESENT label set alone, does it name a worker attestation without a complete authored-*
// pair? It makes no forge read and attributes nothing, so it can only NOMINATE a change for the
// stamp; the ready-flip gate (AuthoringStampGate) and `--stamp-only`'s own read-back are the
// authority-aware checks. A malformed line nominates too: the gate will refuse it.
func WorkerStampGap(body string, labels []string) (WorkerAttestationRef, bool) {
	ref, found, err := ParseWorkerAttestationLine(body)
	if err != nil {
		return ref, true
	}
	if !found {
		return ref, false
	}
	_, state := StampAuthored.StampOf(labels)
	return ref, state != ModelStamped
}
