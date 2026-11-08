---
id: DR-desktools-v2-58
date: "2026-11-05"
title: "Role credential provider (desktools-v2/58): option 1, accept the demonstrated implementation, is the proposed design; no driver ruling is recorded, so this record is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 2 — hold or narrow the change: identify the unsupported boundary and revise it before acceptance (the brief's `## Human decision` and decision issue #3734). Not ruled out: no driver ruling chooses between the two options, so this record does not say which path is taken."
accepted:
  - "Proposed, not ruled. Under option 1, the bounded change is accepted with its stated compatibility, and it grants no new credential, role, endpoint or deployment authority (option 1 as worded in the brief and on #3734). No additional credential holder or universal broker is allowed (the brief's `## Human decision`)."
  - "Proposed, not ruled. The acceptance packet posted on #3734 on 2026-11-03 records a process-boundary change that option 1 must consciously accept: a consumer that mints now reads the App signing key and holds the installation token in its own process, under the existing OS user, where previously the signing key was handled inside a short-lived desktoken child."
  - "Proposed, not ruled. The same packet records that not every minter subprocess was retired: eleven direct CLI callers were explicitly deferred and still use the same provider through the CLI adapter, and the release/preflight executable checks and shell or non-Go consumers keep their intentional boundaries (the brief's `consumers:` list marks the preflight probe, the release minter, cellctl and shell consumers and statusgen out of scope)."
  - "Under either option, approval grants no new identity or deployment (the brief's `## Review`), and independent verification of the brief's Verify table stays a separate act; this record supplies neither."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** No recorded
driver ruling exists for desktools-v2/58. Its decision issue,
[issue #3734](https://github.com/medici-finance/assay/issues/3734), carries the decision-gate
marker for `assay:assay:desktools-v2:58` and is open. As of 2026-11-05 it holds two comments, both
posted by the worker App: a 2026-10-21 note that pickup was held before implementation, and a
2026-11-03 acceptance packet that states it "does not supply a ruling". Neither is a ruling, and
no comment from the driver's own login is on the issue.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record asserts no decider and carries no `ruling:` link,
because there is no driver comment to link. `statusgen --corroborate` is expected to report it
`placeholder-unratified` until the driver rules. The record attests only that the alternatives
recorded in the brief and on the decision issue were weighed. It awaits ratification by the
driver, in the driver's own comment on #3734 naming `desktools-v2/58` and `DR-desktools-v2-58`
with the chosen option. The `ruling:` key is added by a later push, only after that unedited
comment exists.

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, never that it is approved,
so citing this record now would release the design-approval hold on desktools-v2/58 before the
record is ratified. The brief gains `decision-issue: 3734` only, which records the decision issue
it already had. The `design:` key is added in the same push that adds this record's `ruling:`
link.

**Written after implementation.** desktools-v2/58 was delivered by
[#4408](https://github.com/medici-finance/assay/pull/4408), merged 2026-10-26, before this record
existed. This record was written afterwards to back-fill the design-approval gate for a delivered
brief that other work depends on: desktools-v2/59 and desktools-v2/75 each declare
`depends: ["desktools-v2/58"]` and are `todo`. It did not precede the implementation. It is
written on demand under the ruling on #5145, which limits back-filled records to held briefs
that block other work.

This record does not mint a decision. The decision text, the options and their recorded costs
are taken from the brief and the decision issue; nothing here adds rationale beyond what those
two record.

## The decision put to the driver

`docs/streams/desktools-v2/brief-58-extract-the-role-credential-provider-and-retire-internal-desktoken-callouts.md`
extracts GitHub cache, install-ID resolution and token exchange, and GitLab custody and rotation,
into one internal credential provider. The desktoken CLI becomes a path-only adapter over it, and
the shared Go resolver calls it in process instead of starting a desktoken child. The brief's
`## Human decision` asks the driver to confirm that the provider and CLI preserve the same role,
installation, host, repository scope, file protection, refresh, stop and audit behavior, after
reviewing the direct-call evidence and the GitLab read-versus-rotation results. The two options
are:

1. **Accept the demonstrated implementation.** Accept the bounded change and its stated
   compatibility; grant no new credential, role, endpoint or deployment authority.
2. **Hold or narrow the change.** Identify the unsupported boundary and revise it before
   acceptance.

Default if no answer: none. The brief blocks until answered.

## What this record does not decide

It does not choose an option. It does not record a verification result: the brief's Evidence is
empty, and the Verify table stays the independent verifier's. It does not move the brief's board
row. It does not decide desktools-v2/59, desktools-v2/75 or any other dependent. Whether the
implementation matches whichever option the driver chooses is the review gate's judgement and
the brief's Verify table's.
