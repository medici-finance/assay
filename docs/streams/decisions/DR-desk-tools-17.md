---
id: DR-desk-tools-17
date: "2026-09-11"
title: "One trust bar for public-repo authors: `deskboard` classifies on the same predicate `deskpost` gates on"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep the two-tier bar and instead TIGHTEN `deskpost`'s gate to match `deskboard`'s stricter public-repo predicate — ruled out: `deskpost`'s plain `TrustedAuthorID` gate is the reference behaviour this brief aligns TO (the brief's own `consumers:` marks it out-of-scope, read and asserted against, unchanged), and review-trust is not merge-authority — there is no defect in `deskpost`'s existing bar to fix."
  - "Leave the divergence in place and instead teach `deskboard` operators to route around it by hand (a manual blessing comment on every affected PR) — ruled out: it repeats, per PR, a decision that is really scoped to the AUTHOR ('this login is a reviewable identity'), the exact anti-pattern brief-18's sibling ruling already rejected for the repo-scoped write question; the queue would keep silently draining to zero until a human notices, which is the whole failure mode this brief exists to close."
  - "Widen `TrustedHumanAuthor` or `trustedContentAuthor` alongside `TrustedAuthor` to fully collapse every trust axis into one — ruled out (brief's own Ground rules): those two answer a DIFFERENT question (accountable-human / strict-id-aware content trust) and are explicitly out of scope; a diff touching their logic is not this brief."
accepted:
  - "After this change the ONE control standing between an untrusted author and a desk-posted verdict on a public repo is the trusted-logins roster itself — accepted because three independent layers stay behind it unchanged: the outward-write gate (a widened author set still cannot reach a repo the operator never listed), the unconditional public-repo `Security-Review: pass` requirement at head, and branch protection with a human merge (a posted verdict is not a merge)."
  - "A trusted SHARED automation login (configured in the trusted-logins set but neither a role App nor a mapped human) becomes reviewable on public repos, where before only role Apps and mapped humans were — accepted because review-trust and merge-authority are different questions, and an unlisted/fork author is still quarantined unless blessed (the negative control, Verify row 4)."
---

**Ruling recorded (2026-09-11): APPROVED — approve as briefed.** The driver (`human:<name>`)
recorded the ruling on the brief's decision-gate
[issue #808](https://github.com/medici-finance/assay/issues/808) — the
[ruling comment](https://github.com/medici-finance/assay/issues/808#issuecomment-5636289412)
(2026-09-11T14:51:38Z) — as **"executed as recorded - approve as briefed"**, approving the
design as briefed: `deskboard`'s PR classifier drops its stricter public-repo branch so the
author bar is `deskkit.TrustedAuthor(login)` on every repo, matching `deskpost`'s existing
gate; an unlisted author stays quarantined unless blessed. This record transcribes that
ruling into the register; it does not mint a new one — the human act was the driver's comment
on #808, not this file.

**The decision.** `docs/streams/desk-tools/brief-17-one-trust-bar-public-authors.md` removes
the `VisibilityRiskClassed` branch from `classifyPR` (`tools/desk/cmd/deskboard/board.go`) that
swapped in the narrower `TrustedPublicAuthor` predicate on any public/internal/unknown repo.
The brief's own `sources:` records the original maintainer ruling (2026-09-06) that this
design implements; issue #808 is the decision-gate's formal closure of that same ruling for
the lifecycle's design-approval-gate requirement (brief-17 was authored after the
design-approval-gate cutover and needs a cited `DR-<slug>`, which did not exist until this
record).

**The constraint behind it.** After this change the ONE control standing between an untrusted
author and a desk-posted verdict on a public repository is the configured trusted-logins
roster (brief's own Context, `single-point-of-failure`). Three layers behind it, each failing
on a different signal in a different component: (1) the outward-write gate — an outward write
on a public repo is refused unless the repo carries an explicit `:public` allowed-repos entry,
so a widened AUTHOR set still cannot reach a repo the operator never listed; (2) unconditional
risk-classing of public repos — every PR on a public repo needs a `Security-Review: pass` at
head before any flip, a content check independent of author identity; (3) branch protection and
human merge — a posted verdict is not a merge, and no desk tool merges.

**What this record does not decide.** It does not attest the design is correct — that the
alternatives were weighed is recorded here; whether the gate's shape is right is the review
gate's judgement (brief-17's own Review section, which asks the reviewer to confirm the layers
named above are actually independent and that the negative control — an unlisted author is
still quarantined — is exercised), then the change's own Verify table after it lands. It does
not touch `TrustedHumanAuthor` or `trustedContentAuthor` — different controls on different
questions, left as-is by the brief.
