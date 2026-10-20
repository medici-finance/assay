---
id: DR-desk-tools-45
date: "2026-10-20"
title: "Expose the canonical verification queue: `statusgen --gate-scores` rows carry an additive, versioned queue classification; `deskboard awaiting` and `throughput` read it; `verifyloop plan` builds one plan value with `--json` and `--base-queue`, and the canonical base can only demote a dispatch"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option B on the decision issue, reject or amend the design before it moves (\"a follow-up commit can change the class set, the base precedence or the --json shape\") — not chosen for the contract: the driver answered A (https://github.com/medici-finance/assay/issues/3382#issuecomment-6258267531). No rationale was recorded, and none is supplied here."
  - "Hold until the reviewer has reviewed the pull request — offered as option C in the relay comment that put the question (posted by the worker role's App, not the desk's), not in the issue body's fork test; not chosen for the contract, which the driver accepted on that issue. No rationale was recorded."
  - "A second, independent classifier in each consumer (deskboard and verifyloop each inferring the queue from status fields) — ruled out by the brief's own design-fit: \"One canonical queue meaning, additive JSON fields and explicit unknown coverage; no new independent classifier or subprocess per row.\" The delivered `queueFor` calls the existing `bucketAwaiting` once and maps its answer onto the class vocabulary."
  - "Consumers importing statusgen's classifier as a Go package — ruled out by the brief's facts: \"The existing supported cross-module seam is the statusgen command output. Do not promise importing its package main\". The delivered seam is the `--gate-scores` JSON output."
  - "A canonical base that may also promote an item an overlay holds, or widen a dispatch — not delivered: `applyBase` lets the overlays decide first and lets the base only demote a dispatch to `queue-held` or could-not-check. The security review's advisory SEC-2 relies on exactly this bound."
accepted:
  - "An unreadable root or a could-not-check row makes the reading partial, not zero: `deskboard` reports a coverage gap and refuses to advise widening any stage. In `verifyloop plan`, a skipped or unreadable root marks the plan incomplete and makes the exit code 6. A could-not-check item is held in its bucket and changes neither the exit code nor the plan's completeness. (The exit status for an unsupported or partial canonical base is not accepted here; see Open / not decided.)"
  - "Only `implemented` rows classed `judgment-dispatch` count toward verify depth. Runnable-offline, continuation and human-action rows no longer size a model verifier pool, so a board reading counts fewer verify items than before for the same tree."
---

**This record transcribes a recorded human ruling and states the delivered design. It is
NOT approved.** The ruling below accepts brief desk-tools/45's queue contract and says, in its
own words, that the design-approval record is a separate follow-up. This file is that
follow-up's draft. The design approval is pending on its own decision issue: https://github.com/medici-finance/assay/issues/3577
Until the driver's comment approving this record exists there, nothing here claims the design
was approved.

## The ruling recorded

The decision issue is medici-finance/assay#3382, "Decision needed: brief 45 queue contract may
move to implemented". Its fork test offers option A, "accept the delivered queue contract and
record the ruling and a design-decision record (or rule the design gate not applicable)", and
option B, "reject or amend the design before it moves", with default B.

The driver's own comment (type User, 2026-10-19T02:19:33Z, not edited), verbatim: https://github.com/medici-finance/assay/issues/3382#issuecomment-6258267531

> Ruling: A. Accept the delivered brief 45 queue contract (additive queue classification on statusgen --gate-scores rows; deskboard awaiting/throughput reading it; verifyloop plan with --json and --base-queue, where the canonical base can only demote a dispatch to queue-held). The design-approval record is a separate follow-up.

The issue's only other comment is the relay comment, posted by the worker role's App sixteen seconds earlier. It says of
itself that it "is a relay RECORD, not the ruling", and that "The design-approval record
(§4.4) is a separate follow-up question and is not decided here." No later comment on the
issue rules on the design (read 2026-10-20).

**Option labels.** The ruling and the decision issue call the accepted option "A". The brief's
own Human decision lists numbered options instead: "1. Approve the concrete implementation
after independent verification", "2. Request corrections and keep the change pending" and
"3. Hold the change", with "Default if no answer: hold." The ruling answers the decision
issue's A, not the brief's option 1, and the independent verification that option 1 names has
not run.

**Why this record carries no `ruling:` key yet.** `decided-by` is the public placeholder
`human:<name>`. Under `spec/registers-v1.md` §7.5's placeholder rule, a placeholder names
nobody: it is never looked up in a login map and never corroborated by a pull-request
approval. Its only route is a `ruling:` link to an unedited human comment whose own text names
this record by its id, posted on a decision issue that carries this record's decision-gate
marker. The ruling comment above fails that test: it names no record (condition 5), and it
accepts the contract rather than the design, so a `ruling:` pointing at it would fail closed.
The decision issue for this record's approval carries the marker and asks the driver for a new
comment naming the record. The `ruling:` key is added to this record, by a later push, only
after that unedited comment exists. Until then this record is not approved, and the
corroboration check correctly reports `decided-by` as `placeholder-unratified`.

## The design stated for approval

The brief is `docs/streams/desk-tools/brief-45-expose-the-canonical-verification-queue-to-its-consumers.md`.
Its delivering pull request is medici-finance/assay#3376 (draft, head `2caa578a`). What it
delivers, read from that diff:

1. **One classification, exported additively.** `statusgen --gate-scores` rows keep their
   legacy fields (`brief`, `score`, `blockedCount`, `stream`, `status`, `repo`) and add
   `queueVersion`, `queueClass`, `queueBucket`, `queueOwner`, `queueNextAct`, `queueGate`,
   `queueRisk`, `queueIrreversible`, `queueValid` and `queueWhy`. `queueFor`
   (`statusgen/awaiting_bucket.go`) calls the existing `bucketAwaiting` once and maps its
   answer onto contract version 1's classes: `judgment-dispatch` (the only class that is
   model-verify work), `runnable-offline`, `continuation`, `human-action`, `unreadable`
   (`queueValid=false`, unknown, never zero), and the `paused` / `parked` placements. A row
   with an unknown or missing `queueVersion` is a coverage gap for every consumer.
2. **The board reads it.** `deskboard awaiting` and `throughput` count only `implemented`
   rows classed `judgment-dispatch` toward verify depth. An unreadable root, a statusgen older
   than the contract, or could-not-check rows make the depth a lower bound; the advice then
   widens no stage.
3. **The planner builds one value.** `verifyloop plan` builds one plan value and renders it as
   text or, with `--json`, as a versioned JSON document. Diagnostics stay off JSON stdout.
   Each root is reported as healthy, skipped or unreadable, and a skipped or unreadable root
   makes the plan incomplete and the exit code 6. Evidence-only human work carries a different
   authority from a flip: its Evidence rows may be written, its status never.
4. **The base can only demote.** With `--base-queue` (opt-in; off, the plan is the overlay
   plan it always was), `applyBase` (in verifyloop's plan.go, new in the delivering pull request) composes the
   canonical class under the existing overlays, with one documented precedence. The overlays
   decide first, and an item they hold stays held. The base is applied last and may only demote
   a dispatch: a dispatch item not classed `judgment-dispatch` is held as `queue-held`, an
   unreadable or missing class is could-not-check, and Evidence-only work never gains flip
   authority from the base. The delivering pull request's `base_never_promotes` and
   `landing_guard_holds_with_planner_bypassed` subtests pin these properties.

The brief and its delivering diff are the authority. This summary does not amend either.

## Consequence level — transcriber's classification

`consequence: major` is not part of the ruling, which states no level. The transcriber took
it from the brief's risk answers (`customer: yes`, the other three `no`, including
`irreversible: no`) and its `gate-why`: the change alters "verification scheduling/landing
behavior relied on by adopters". A wrong queue can hide verification work or hold it back,
and a route back exists. The driver can correct the level before the record's approval is linked.

## Open / not decided

- **The design approval.** It is the driver's, and it is pending on https://github.com/medici-finance/assay/issues/3577
  It counts only as the driver's own unedited comment there naming `DR-desk-tools-45`, linked
  from this record's `ruling:` key. A pull-request approval, a desk relay or a reviewer verdict
  does not stand in for it.
- **SEC-2, the unpinned statusgen in the base read.** The security review of the delivering
  pull request (review 5579323310, advisory) found that `baseQueueRows`
  (verifyloop's plan.go, lines 156-167 at that head) runs `STATUSGEN_BIN` or the statusgen on
  PATH with no version pin or skew check. It is an open advisory: no ruling has accepted or
  mitigated it, and this record accepts neither. The demote-only rule bounds what the base
  can do to the plan's output: a substituted or stale binary can hold work back but not advance
  it. It does not bound what that binary does when it runs, with the caller's full
  environment. Whether to add the pin and skew check, and when, is not decided here.
- **The exit status for an unsupported or partial canonical base.** At the delivering pull
  request's head, `verifyloop plan --base-queue` over an unsupported or partial base (for
  example an older statusgen that predates the contract) marks the plan incomplete
  (`complete: false`, with each gap named on stderr) but exits 0. A change so that an
  incomplete plan does not exit 0 has been requested on that pull request. This record does
  not accept the exit-0 behaviour; which exit status ships is open until that change lands
  or is declined.
- **Independent verification.** The brief's Verify table, including the 12-entry consumer
  corroboration (row 4) and the negative control (row 5), runs after merge by a
  non-implementer. This record attests none of it.
- **Adoption and rollout.** The brief's failure map says an old consumer must not claim
  rollout complete, and that adoption needs published paired versions and a local acceptance
  receipt. That is later briefs' work, not this record's.
- **The brief's human sign-off.** desk-tools/45 is `gate: human`. Its `verified` and `done`
  flips need the human sign-off on a Verify run of the merged implementation. This record is
  a design record only.

## What happens after merge

The delivering pull request merges with desk-tools/45 not yet at `implemented`. After this
record merges, a new follow-up pull request flips desk-tools/45 to `implemented` and adds
`design: DR-desk-tools-45` to the brief's frontmatter, then re-runs `lint`.
