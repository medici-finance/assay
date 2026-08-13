package deskkit

// Idempotency store. "Done" means a prior audit entry with result ∈ {ok, noop}
// at the SAME (repo, pr, head) for this verb. A prior REFUSAL never counts — a
// refusal followed by a state change is a legitimate retry (fail-closed forbids retrying
// past a LIVE failed verification, not retrying after the world changed).
//
// ResultDryRun does not count either, and must not be added to the set (#214): a
// rehearsal performed no act, so treating it as done would let `--dry-run` SUPPRESS the
// real write it was supposed to preview — a silent skip, the failure mode this store's
// fail-closed contract exists to prevent. That is the second reason a dry run is its own
// result class rather than a `noop`.
//
// The signature keys on (repo, pr, head, verb). Verbs that need a finer key
// — review by verdict KIND AND verdict, comment by bodyDigest, pr-create by branch
// — encode that discriminator into `verb` at their own layer (e.g.
// "review:security:approve"); this store matches on whatever verb string it is given.
//
// That delegation puts a real obligation on the caller: the verb must carry EVERY
// component that distinguishes two writes the desk must be able to make at the same
// (repo, pr, head). deskpost's review verb omitted the verdict KIND until
// #220 — a correctness approve and a `Security-Review: pass` are both
// `--verdict approve`, so on a risk-classed PR (the one case that requires both at one
// head) the second was matched here as a repeat of the first and silently dropped. This
// store cannot detect that: an under-specified verb is indistinguishable, from in here,
// from a genuine duplicate. A caller adding a verb must ask what else could legitimately
// be written at the same head, and put it in the verb.
//
// The same lesson recurred one discriminator deeper (#518): kind+flag+head
// still under-specifies a REVIEW, because two parallel lanes can legitimately post the
// same verdict shape at one head with different findings. deskpost's review guard now
// bypasses AlreadyDoneIn for its own predicate (reviewAlreadyPostedIn) that additionally
// matches the entry's recorded BodyDigest — kept in the schema field rather than folded
// into the verb, so ledger history stays readable under one verb per lane.

// AlreadyDoneIn is the pure predicate over ENTRIES already loaded and verified by
// LoadEntries (under the outward-write flock). It has no IO and therefore no error
// path, so it cannot silently default on a read error — the outward-write flow uses
// this form after LoadEntries has already refused a corrupt file (exit 6).
func AlreadyDoneIn(entries []Entry, repo string, pr int, head, verb string) bool {
	for i := range entries {
		e := &entries[i]
		if e.Repo == repo &&
			e.Verb == verb &&
			e.PR != nil && *e.PR == pr &&
			e.HeadSHA != nil && *e.HeadSHA == head &&
			(e.Result == ResultOK || e.Result == ResultNoop) {
			return true
		}
	}
	return false
}

// AlreadyDone is the convenience form carrying the signature. It loads the
// audit file itself and delegates to AlreadyDoneIn.
//
// Fail-closed contract: it NEVER returns true on an unverifiable read. On a
// corrupt/unreadable audit file it returns false — a corrupt file can therefore never
// masquerade as "already done" and silently SKIP a needed write. The corrupt-file
// REFUSAL (exit 6) is raised by AllowWrite / LoadEntries, which the outward-write
// flow calls under the flock BEFORE this check; use AlreadyDoneIn there so that
// Unverifiable error is not discarded.
func AlreadyDone(repo string, pr int, head, verb string) bool {
	entries, err := LoadEntries()
	if err != nil {
		return false
	}
	return AlreadyDoneIn(entries, repo, pr, head, verb)
}
