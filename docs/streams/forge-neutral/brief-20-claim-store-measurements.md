---
brief: assay:assay:forge-neutral:20
title: Measurements — what the reviewer writes, who reads claims, and what the file store can know about where it runs
why: >-
  The plan to move dispatch claims off the forge and drop the reviewer's repository write
  rests on things nobody has measured: that the claim is the reviewer's only repository write,
  that every claim reader is known, and that the file store can tell when it has been pointed
  at a network filesystem or started inside a container, where it is not supported. A wrong
  assumption here becomes a silent double-dispatch or a guard that never fires. This brief
  turns the spec's could-not-check rows into dated reads before code is built on them.
wave: 1
depends: []
unblocks: ["forge-neutral/21"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them"
  - "tools/desk/cmd/deskclaim/main.go:30-35 — the recorded reason dispatch claims left the machine-local directory on 2026-08-13"
  - "tools/desk/internal/deskkit/claim.go:217-323 — the shipped exclusive-create-under-lock primitive whose guarantees rows S2 and S4 are about"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): two sweeps (every reviewer-role forge write; every claim reader) whose misses are invisible unless the sweep method itself is re-derivable"
domain: complicated
version: 1
id: e90319ae-edc8-4dc2-8127-f9e609c4704c
---

# Brief 20 — Claim-store measurements

## Context
files:
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — §3.1 gains two inventory tables; §3.3 rows S2 and S4 move from could-not-check to
  a dated result.
- `changelog/<branch-slug>.md` (planned) — the per-PR fragment.

This brief writes no tool code. It is documentation backed by reads on **fixtures** the
measuring operator owns.

Withdrawn from the first draft by ruling (spec §10 D, H): the shared-container-volume
measurement (containers use the served store, permanently) and every forge-behaviour read
(the forge-ref store is being removed, so nothing is built on them).

facts:
- Reviewer-role forge calls are constructed through `ForgeFor(repo, "reviewer")` or a reviewer
  token mint. Known sites at `c67cc371`: `tools/desk/cmd/deskpost/`, `tools/desk/cmd/deskflip/`,
  `tools/desk/cmd/deskdispatch/dispatch.go:872-918` (the claim),
  `tools/desk/cmd/deskclose/superseded.go`, `tools/desk/cmd/desklabel/`.
- Known claim readers outside the claim tool (spec T8): `tools/desk/cmd/desksupervise/live.go`,
  `tools/desk/cmd/deskpost/claimliveness.go`, `tools/desk/cmd/fanoutloop/land.go`,
  `tools/desk/internal/loopengine/writescope_io.go`, `tools/desk/cmd/deskroster/` (list join),
  and skill bodies that tell the reader to `git ls-remote` the claim namespace.
- A result row is: filesystem or platform, question, exact command, observed outcome, date.
  A row that could not be measured stays **COULD-NOT-CHECK** with the reason.
- The file-store race probe is: N processes each attempt an exclusive create of the same path
  under the shipped directory lock; the row records how many reported success. Exactly one is
  the only passing count.

## Ground rules
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- This brief may start while the spec is `draft` — it is the measurement the spec's approval
  leans on. It changes no code.
- Record commands and outcomes only. No credential value enters the document.
- Do not convert a could-not-check into a result by reasoning or from memory. If the
  filesystem or platform is not available, the row stays could-not-check and says what is
  missing.

## Task
1. **Reviewer write inventory.** For every desk verb that can act as the reviewer role, list
   each forge operation and the permission it needs on each forge; mark read / pull-request
   write / issue write / repository write. Expected answer: "repository write: the dispatch
   claim only". Any other row is a finding — file it and name it in the PR body.
2. **Claim reader inventory.** Every site that reads, lists or releases a dispatch claim
   without going through the claim tool, with file:line and what it does when the namespace is
   empty. This is brief 22's work list.
3. **S2 — network filesystems.** Run the race probe on a local disk (the control) and on any
   network filesystem you can reach from a plain host process. Record filesystem type, mount
   options, N, and the success count.
4. **S4 — what the tool can know.** On each supported OS you can reach, record whether a Go
   process can (a) determine the filesystem type of a directory and (b) determine that it is
   running inside a container or pod, and by what read. "Cannot be determined on this OS" is a
   result. These decide how much of the spec's §6 filesystem and container guards can be a
   refusal rather than a notice.
5. Update the spec's §3.3 status column. Where a result contradicts the recommended default
   for the spec's open question L, say so at the top of the PR body — do not edit the default.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | `2` — both work-list rows are still present |
| 2 | check +dereference | `grep -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'of [0-9][0-9]* succeeded' -e 'determined' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | exit 0 and no line printed — each row quotes a probe count or a determination result, or says COULD-NOT-CHECK |
| 3 | gate:model +dereference | Re-run the local-disk control of Task 3 from the recorded command text | exactly one of N succeeded; a different count is checked-failed on this brief |
| 4 | check | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the reviewer write inventory |
| 5 | check | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the claim reader inventory or is the claim tool itself |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A reviewer-role write is missed and the narrowed reviewer later fails at that verb | row 4 |
| A claim reader is missed and goes blind when claims leave the forge | row 5 |
| A network-filesystem row is filled from general knowledge and presented as measured | row 2 (a measured row must carry a count) and row 3 |
| Container or filesystem detection is reported as possible on an OS where it was never tried | review-only — each S4 row names the OS and the read it used |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no; documents only). Reviewer records
verdict + date in the stream README table.
