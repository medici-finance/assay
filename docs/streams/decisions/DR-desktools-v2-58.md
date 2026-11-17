---
id: DR-desktools-v2-58
date: "2026-11-05"
title: "Role credential provider (desktools-v2/58): option 1, accept the demonstrated implementation, is the driver's chosen option; no comment names this record, so it is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 2 — hold or narrow the change: identify the unsupported boundary and revise it before acceptance (the brief's `## Human decision` and decision issue #3734). The driver's comment on #3734 chose option 1 over it."
accepted:
  - "Proposed record of the chosen option. Under option 1, the bounded change is accepted with its stated compatibility, and it grants no new credential, role, endpoint or deployment authority (option 1 as worded in the brief and on #3734). No additional credential holder or universal broker is allowed (the brief's `## Human decision`)."
  - "Proposed record of the chosen option. The acceptance packet posted on #3734 on 2026-11-03 records a process-boundary change that option 1 must consciously accept: a consumer that mints now reads the App signing key and holds the installation token in its own process, under the existing OS user, where previously the signing key was handled inside a short-lived desktoken child."
  - "Proposed record of the chosen option. The same packet records that not every minter subprocess was retired: eleven direct CLI callers were explicitly deferred and, as of that 2026-11-03 packet, still used the same provider through the CLI adapter, and the release/preflight executable checks and shell or non-Go consumers keep their intentional boundaries (the brief's `consumers:` list marks the preflight probe, the release minter, cellctl and shell consumers and statusgen out of scope). Since then ten of the eleven have moved in process by later changes: `repohardenguard` under #1573, and the nine write verbs under #5457, each tracked on its own issue. Only the boot mint check still starts the CLI, as a probe of the installed binary. The `tools/desk/README.md` callsite inventory records the current state."
  - "Under either option, approval grants no new identity or deployment (the brief's `## Review`), and independent verification of the brief's Verify table stays a separate act; this record supplies neither."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** Its decision
issue, [issue #3734](https://github.com/medici-finance/assay/issues/3734), carries the
decision-gate marker for `assay:assay:desktools-v2:58`. When this record was first written
(2026-11-05) the issue held only two worker-App comments, neither a ruling. Since then the driver
has chosen an option: an unedited comment from the driver's own account on
[issue #3734](https://github.com/medici-finance/assay/issues/3734#issuecomment-6923693852),
posted 2026-11-16, names `desktools-v2/58` and option 1, accepting the implementation delivered
in #4408 and its stated compatibility, with no new credential, role, endpoint or deployment
authority. That choice covers the implementation in #4408 only. The later in-process moves of
the remaining direct callers (`repohardenguard` under #1573, the nine write verbs under #5457)
were separate changes tracked on their own issues, and the #3734 comment does not rule on them.
The brief's `ruling:` key cites that comment. The comment does not name
`DR-desktools-v2-58`, and `statusgen --corroborate` requires the comment's own text to name the
record, so this record stays `placeholder-unratified` until the driver posts a comment that
names `DR-desktools-v2-58`.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record carries no `ruling:` link. The record attests only
that the alternatives recorded in the brief and on the decision issue were weighed. The
`ruling:` key is added by a later push, only after an unedited driver comment naming
`DR-desktools-v2-58` exists.

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

It does not itself choose an option; the driver's comment on #3734 does. It does not record a verification result: the brief's Evidence is
empty, and the Verify table stays the independent verifier's. It does not move the brief's board
row. It does not decide desktools-v2/59, desktools-v2/75 or any other dependent. Whether the
implementation matches whichever option the driver chooses is the review gate's judgement and
the brief's Verify table's.
