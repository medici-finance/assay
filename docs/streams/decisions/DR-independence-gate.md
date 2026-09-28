---
id: DR-independence-gate
date: "2026-09-16"
title: "Verifier independence and dispatch authority are enforced from a derived, verifiable signal — a same-identity author/verifier pair is a hard reject, not a NOTICE, and dispatch authority is read from a signed stamp, not from a trusting reader's guess about a label actor's login"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep the committer-identity cross-check a NOTICE and rely on the token-level checks (the free-text `implementer` token, the `selfVerificationReason` string match) as the only independence signal — ruled out: the token checks are themselves self-attested prose a session writes about its own work, so an author who simply avoids the free-text token gets a green gate. A NOTICE that never gates is the self-attestation weakness this stream exists to close, restated one layer down."
  - "Make the model-floor honor a `dispatched-*` label applied by any trusted LOGIN (not only a trusted dispatcher slug) — ruled out as the primary control: it widens what counts as authority to the ambient set of trusted humans, and a label actor's login is exactly the guessed, un-derived signal #336 shows failing. A first-class re-stamp/migration verb that lets a dispatcher re-apply the label under its own slug keeps the floor's derivation intact and is the sanctioned durable path; honoring a login is at most an interim operator action a human authorizes, recorded, not a code default."
  - "Prove author≠verifier fully mechanically at the design-approval gate as well — ruled out for now as infeasible without a brief-author→human mapping the briefs do not carry (the boundary `designgate.go` already declares and `lifecycle-v1.md` §7.1.2 names). This record enforces independence at the point where a machine-readable signal DOES exist (the committer identity of the authoring vs Evidence-adding commit, and the dispatch stamp), and states the design-gate mechanical proof as an out-of-scope boundary rather than claiming it."
  - "Hard-reject on the committer-identity signal unconditionally, including when only one git identity exists in the repo — ruled out: a single-identity repo cannot distinguish author from verifier by commit metadata at all, so a hard reject there would red every brief in a solo or single-App checkout. The three-state read is preserved: one-identity is a could-not-check (inconclusive, stays a NOTICE), MULTIPLE identities with a same-identity author/verifier pair is the discriminating case that becomes the hard PROBLEM."
accepted:
  - "The `verified`/`done` gate becomes strictly harder to satisfy: in a multi-identity repo, a brief whose authoring commit and most-recent (Evidence-adding) commit share one git identity is a hard PROBLEM, not a NOTICE. A legitimate same-identity re-run (e.g. the author lands a post-verify typo fix as the last commit) will red until a non-implementer commit or an explicit recorded override cures it. The mitigation is that the cure is a real independent act, which is the property the gate exists to force."
  - "The model-floor refuses more loudly and derives authority from the dispatch stamp: an authority signal that cannot be derived reads as could-not-verify and is refused (fail-closed), never rounded up to trust. The accepted cost is that a genuinely-dispatched item whose stamp path was misconfigured is blocked until the stamp is applied through the sanctioned path, rather than admitted on a login guess."
  - "Independence is enforced only where a machine-readable signal exists; the full author≠approver proof at the design gate remains a declared boundary, not a silent claim. A reader must be able to see WHERE the property is enforced and where it is only best-effort."
---

**PROPOSED — status is split by half; this record covers two decisions and only one of them
has a ruling recorded.** This record decides two things: (1) the committer-identity
cross-check becomes a hard PROBLEM (not a NOTICE) in a multi-identity repo — the
**attribution** half, cited by brief-05 — and (2) dispatch authority is read from a derived
signal rather than a reader's login guess, and the widening that signal may accept — the
**dispatch-authority** half, cited by brief-06. Only the dispatch-authority half has a ruling
recorded, transcribed below. **The attribution half is OPEN — no ruling is recorded for it.**
Brief-05's own Human decision section states this explicitly ("Default if no answer: none —
blocks until answered"); nothing in this record or in #336 rules on it, and this PR does not
ask for that ruling — the desk is raising the open attribution half with the driver
separately, outside this PR. `decided-by:` stays the register's placeholder form until both
halves are ruled.

**Dispatch-authority ruling recorded (2026-09-02): both.** The driver (`human:<name>`) recorded
the ruling on [issue #336](https://github.com/medici-finance/assay/issues/336) — "both." at
[comment 5512151770](https://github.com/medici-finance/assay/issues/336#issuecomment-5512151770)
followed by "ratified" at
[comment 5512153662](https://github.com/medici-finance/assay/issues/336#issuecomment-5512153662)
(2026-09-02T15:36:58Z / 15:37:06Z) — approving both remedy paths the relay put to the driver
for the DISPATCH-AUTHORITY half only:
(1) an operational, one-time re-stamp of the affected legacy backlog under a trusted App slug,
recorded per-PR; and (2) a durable, tool-level fix under which the model-floor's actor check
also honours a `dispatched-*` label applied by a trusted LOGIN, not only a trusted dispatcher
slug, with a positive-control test that an untrusted actor's label still refuses. This record
transcribes that ruling into the register; it does not mint a new one — the human act was the
driver's comment on #336, not this file. The ruling as put to the driver (desk relay, comment
[5512151713](https://github.com/medici-finance/assay/issues/336#issuecomment-5512151713)) reads,
quoted: "the model-floor's actor check honours `dispatched-*` labels applied by a trusted LOGIN
as well as by a trusted dispatcher slug (a trusted login is a stronger signal, not a weaker
one), with a positive-control test that an untrusted actor's label still returns indeterminate
→ refuse; and a first-class re-stamp verb so a future migration never needs hand work." It does
not itself specify an allowance-key mechanism; see the amendment below.

**Amendment (2026-09-27, at implementation of brief-06 — narrows how alternative 2 above
reads, does not reopen it, and does not extend the ruling to the attribution half above).**
The second `alternatives:` bullet rules out, as the *primary, ambient* control, honouring a
`dispatched-*` label from any trusted human login. #336's ratified durable fix, quoted above,
authorizes exactly that — honouring a trusted LOGIN generally, with a positive control — and
does not itself mandate an allowance-key mechanism. The *explicit, roster-configured
allowance* form this brief ships (`ASSAY_STAMP_TRUSTED_LOGINS`, itself a strict subset of the
trusted-human set — unset or unconfigured vouches for nobody, an entry outside the
trusted-human set or bot-shaped refuses the whole allowance, and every floor consumer still
runs the one predicate) is **brief-06 Task 1's own engineering choice** — "if the ruling widens
accepted authority, gate that behind an explicit, roster-configured allowance — never a silent
default" — built to satisfy the ruling narrowly, not a mechanism #336 itself ratified. The
alternative bullet's rejection of an *ambient* "any trusted login vouches" default stands
unchanged; this amendment records that the narrower, explicit, opt-in, fail-closed allowance
form brief-06 ships satisfies the ruling without reopening the rejected ambient shape, so the
record and the code no longer disagree — for the dispatch-authority half only. The attribution
half (brief-05) is untouched by this amendment and remains OPEN, per the status paragraph
above.

**Second amendment (2026-09-27, `deskrestamp`'s OWN re-stamp bar — narrower than the
allowance above, a distinct decision).** The amendment directly above describes the
model-floor's READER: what a widened set of appliers the floor will *recognise* as already
attesting. It says nothing about `deskrestamp`'s WRITER — the verb that *mints* a fresh
dispatcher-attested stamp by removing a foreign-applied label and re-applying it — and a
correctness/security review of this PR (SEC-1b, round 3) found the two conflated: an
earlier round of `deskrestamp` re-attested any login the roster trusted at all
(`deskkit.IsTrustedHumanLogin`), which is wider than even the allowance above and does not
require the applier to be IN the allowance. The driver ruled on this directly, on PR #1727
([comment 5860170351](https://github.com/medici-finance/assay/pull/1727#issuecomment-5860170351),
2026-09-27T21:54:19Z), quoted (tool-facing sentence only — the comment's second sentence is
a deployment-configuration instruction out of this record's scope): "deskrestamp may vouch
only for dispatched-* labels applied by the driver's own login (the roster bless login)
before 2026-09-27T00:00:00Z (the #336 legacy backlog); every other applier is refused."
`deskrestamp`'s provenance bar is
therefore now STRICTER than the floor's own allowance, deliberately: minting a fresh
dispatcher attestation over content someone else applied is a bigger act than a reader
merely recognising an already-widened stamp, and the ruling narrows it to the roster's
single blessing authority (`deskkit.IsRestampDriverLogin`, read dynamically — never any
other trusted login, allowance member or not) and a closed, dated backlog window
(`deskkit.RestampDriverCutoff`, a compiled constant, not a roster key — the backlog is a
fixed historical set, not an ongoing knob). This does not reopen or widen the allowance
amendment above; it is a second, independent, narrower decision about one verb's own
repair bar, and it does not touch `ASSAY_TRUSTED_LOGINS` or its semantics anywhere.

The decision is what makes the author≠verifier property TRUE rather than ASSERTED. The
constraint behind it is the self-attestation error class: everything a session writes about
its own work is prose it authored, so a gate keyed on a self-declared token or a login a
reader chooses to trust inherits that prose's unchecked-ness. The design therefore moves each
independence/authority decision onto a signal that fails on a DIFFERENT input than the thing
being attested:

- The committer-identity cross-check already reads a second, independent signal — the git
  identity of the authoring commit versus the commit that most recently touched the brief.
  Today that signal only ever produces a NOTICE. This record decides it produces a hard
  PROBLEM in the one case where the signal is discriminating: a multi-identity repo with a
  same-identity author/verifier pair. The one-identity (inconclusive) case stays a NOTICE, and
  the "multiple identities, last commit is a benign author re-touch" case is what the record's
  accepted-cost line owns.
- Dispatch authority is read from the dispatch stamp the dispatcher applies as its own bound
  identity, not from a reader's judgement about whether a label actor's login is trusted. An
  authority that cannot be derived from the stamp reads could-not-verify and is refused.

**What this record does not decide.** It does not fix the exact wording of the new PROBLEM
messages (that is each brief's Task and the review gate's judgement), it does not decide the
one-time operator action for the legacy backlog #336 names (that is a separate recorded human
authorization, not a code default), and it does not license a mechanical author≠approver proof
at the design gate — that remains a declared boundary until a brief-author→human mapping
exists to make it sound.
