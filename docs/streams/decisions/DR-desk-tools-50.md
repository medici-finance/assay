---
id: DR-desk-tools-50
date: "2026-10-29"
title: "Bind verifier prerequisites to the local run admission with declared trust limits"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4393#issuecomment-6505589048"
alternatives:
  - "Boot preflight alone or a prose environment banner — the proposal rejects this because it does not bind the checked environment to later row/Evidence admission and leaves stale preflight reusable."
  - "A fresh live probe or separate environment service per row — the proposal rejects this because it duplicates custody and policy boundaries and introduces external access outside this brief."
  - "Require forge-anchored world metadata before accepting the design — the proposal offers this stronger tamper boundary as a separate stamp-format/compatibility amendment; the ruling instead approves the described local-record design."
accepted:
  - "The ruling explicitly includes the documented local-record tamper boundary and compatibility for earlier unmarked runs. World and WorldRequired are local-only, not forge-anchored environment evidence."
  - "Independent implementation verification and lifecycle acceptance remain pending, exactly as the ruling states. Design approval does not supply either."
---

The [human ruling](https://github.com/medici-finance/assay/issues/4393#issuecomment-6505589048)
was created on 29 October 2026 at 18:53:05 UTC and has the same creation and update
time. It names this record and approves the design described in PR #4517, including
its documented local-record tamper boundary and compatibility for earlier unmarked
runs. It expressly leaves independent implementation verification and lifecycle
acceptance pending. This record transcribes that act; it does not create a ruling.

## Approved basis and revision limits

The PR head at approval was `653a458`, whose immutable
[proposal](https://github.com/medici-finance/assay/blob/653a458dc4600373d1da251953edb612e6f36169/docs/streams/desk-tools/design-50-run-prerequisites.md)
is the approved basis. The comment names the PR rather than a commit; the revision
association is a reconstruction from its history, not a hash the human wrote.
Later edits must not inherit approval merely by retaining this record's name.

The design keeps prerequisite admission in existing deskkit/preflight and verifier
attestation owners, deriving the run's source, declared siblings, pinned instrument
bytes, tool/runtime and capability evidence through the common brief derivation.
It distinguishes hold from could-not-check and successful empty scans, and rechecks
bound records before rows/Evidence. No new service, production probe, credential
scope, per-row credential mint or workflow activation is approved.

## Security clarifications after the approval-time proposal

The [current design note](../desk-tools/design-50-run-prerequisites.md) expands the
consequences omitted from the original proposal, addressing review finding S1.
These are disclosures of the delivered implementation, not claims that the human
explicitly reviewed every consequence in that later wording:

- A writer of the verifier home's local record can strip both `WorldRequired`
  and `World`. Admission then treats the run as an earlier unmarked run and skips
  tool, sibling, instrument and runtime prerequisite rechecks. Forge-anchored
  source and dispatcher-stamp checks remain. Clearing `World` also defeats the
  local different-digest rebinding refusal. The compatibility allowance and the
  rebind refusal share this same local tamper boundary.
- Delivery security advisory [S3](https://github.com/medici-finance/assay/pull/4392#pullrequestreview-5645899147)
  identifies a latent native verifyloop path that does not mark a run before
  recovery; failed world binding leaves a closed, unmarked, unbound home that
  later admission can accept without prerequisite rechecks. No shipped verb
  reaches that path. Wiring native dispatch requires the mark before recovery
  and independent refusal verification; this record claims no explicit human
  acceptance of that omitted advisory and grants no activation.

The same delivery review records advisory S2 for the local-only record. These
residuals remain visible for independent review and the pending implementation/
lifecycle acceptance; the ruling does not close them as verified defects.

## Severity and boundaries

`major` retains the proposal's severity as a reversible desk default declared in
this PR: a wrong prerequisite decision can suppress verification or admit stale
evidence, with a route back through a fresh admitted run. The ruling states no
severity, so this field is not attributed to the human.

The brief cites this record as `design:`. It gains no implementation `ruling:`;
its gate, UUID, version, Task, Verify, Evidence and board status are unchanged.
The original delivery is PR #4392; this PR neither reimplements it nor independently
verifies it. No ready flip, merge, release, pin change or live infrastructure
access follows from recording the design approval.
