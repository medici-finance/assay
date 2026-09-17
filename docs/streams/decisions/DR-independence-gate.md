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

**PROPOSED — no ruling is recorded.** This record captures the design as authored so the
design-approval gate has something to dereference; `decided-by:` is a placeholder until a
human rules on the decision issue of the brief(s) that cite it, and the ruling is recorded
here in the same motion.

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
