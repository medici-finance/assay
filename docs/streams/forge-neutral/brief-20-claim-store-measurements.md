---
brief: assay:assay:forge-neutral:20
title: Measurements — what the reviewer writes, who reads claims, what a shared directory guarantees, and what each forge refuses
why: >-
  The plan to move dispatch claims off the forge and drop the reviewer's repository write
  rests on things nobody has measured: that the claim is the reviewer's only repository write,
  that every claim reader is known, that a shared directory really gives mutual exclusion
  where adopters would put it, and that each forge refuses what its documentation implies. A
  wrong assumption here becomes a silent double-dispatch or a promised boundary that is not
  there. This brief turns the spec's could-not-check rows into dated reads before code is
  built on them.
wave: 1
depends: []
unblocks: ["forge-neutral/21", "forge-neutral/26", "forge-neutral/27"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "docs/streams/forge-neutral/claim-shape.md §1 — the live-read table format this brief reuses, and its open row L10, which this brief closes"
  - "tools/desk/cmd/deskclaim/main.go:30-35 — the recorded reason dispatch claims left the machine-local directory on 2026-08-13"
  - "tools/desk/internal/deskkit/claim.go:217-323 — the shipped exclusive-create-under-lock primitive whose guarantees rows S1–S4 are about"
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
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — §3.1 gains two inventory tables; §3.3 rows S2–S4 and §3.4 rows F4–F8, F11–F13 move
  from could-not-check to a dated result.
- `docs/streams/forge-neutral/claim-shape.md` — row L10 gains its result.
- `changelog/<branch-slug>.md` (planned) — the per-PR fragment.

This brief writes no tool code. It is documentation backed by reads on **fixtures** the
measuring operator owns.

facts:
- Reviewer-role forge calls are constructed through `ForgeFor(repo, "reviewer")` or a reviewer
  token mint. Known sites at `c67cc371`: `tools/desk/cmd/deskpost/`, `tools/desk/cmd/deskflip/`,
  `tools/desk/cmd/deskdispatch/dispatch.go:872-918` (the claim),
  `tools/desk/cmd/deskclose/superseded.go`, `tools/desk/cmd/desklabel/`.
- Known claim readers outside the claim tool (spec T8): `tools/desk/cmd/desksupervise/live.go`,
  `tools/desk/cmd/deskpost/claimliveness.go`, `tools/desk/cmd/fanoutloop/land.go`,
  `tools/desk/internal/loopengine/writescope_io.go`, `tools/desk/cmd/deskroster/` (list join),
  and two skill bodies that tell the reader to `git ls-remote` the claim namespace.
- A result row is: deployment or filesystem, question, exact request or command, status code /
  refusal text / observed count, date. A row that could not be measured stays
  **COULD-NOT-CHECK** with the reason.
- The file-store race probe is: N processes each attempt an exclusive create of the same path
  under the shipped directory lock; the row records how many reported success. Exactly one is
  the only passing count.

## Ground rules
- NEVER push to, or change a setting on, any repository other than a throwaway fixture you
  created for this brief. Never point a row at this repository.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- This brief may start while the spec is `draft` — it is the measurement the spec's approval
  leans on. It changes no code.
- Record requests and outcomes only. No credential value enters the document.
- Do not convert a could-not-check into a result by reasoning or from memory. If the fixture,
  filesystem or credential is not available, the row stays could-not-check and says what is
  missing.

## Task
1. **Reviewer write inventory.** For every desk verb that can act as the reviewer role, list
   each forge operation and the permission it needs on each forge; mark read / pull-request
   write / issue write / repository write. Expected answer: "repository write: the dispatch
   claim only". Any other row is a finding — file it and name it in the PR body.
2. **Claim reader inventory.** Every site that reads, lists or releases a dispatch claim
   without going through the claim tool, with file:line and what it does when the namespace is
   empty. This is brief 22's work list.
3. **Store reads (S2–S4).** Run the race probe on: a local disk; a Docker named volume mounted
   into two containers on one host; and any network filesystem or cluster read-write-many
   volume you can reach. Record filesystem type, mount options, N, and the success count.
   Record whether the directory's filesystem type is detectable from Go on each OS tried.
4. **Forge reads (F4–F8, F11–F13, L10)** — relevant to the `forge-ref` store only. GitHub
   fixture with the restrict-creations/updates/deletions pair and the measuring App outside
   bypass: a branch create, a branch update by git push, a contents-endpoint write, an
   update-branch call, a create under each claim prefix, an update to a fork-hosted change
   head. GitLab fixture: branch creation against a wildcard protection; a push under
   `refs/dispatch/`; a git push with a PAT of scope `api` only. Record plan, edition, tier.
5. Update the spec's status columns and `claim-shape.md` L10. Where a result contradicts a
   recommended default in the spec's §10, say so at the top of the PR body — do not edit the
   default.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -e '^. S2 . ' -e '^. S3 . ' -e '^. S4 . ' -e '^. F4 . ' -e '^. F5 . ' -e '^. F7 . ' -e '^. F8 . ' -e '^. F11 . ' -e '^. F12 . ' -e '^. F13 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | `10` — every work-list row is still present, none silently dropped |
| 2 | check +dereference | `grep -e '^. S2 . ' -e '^. S3 . ' -e '^. F4 . ' -e '^. F5 . ' -e '^. F8 . ' -e '^. F11 . ' -e '^. F12 . ' -e '^. F13 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'HTTP [0-9][0-9][0-9]' -e 'remote: ' -e 'of [0-9][0-9]* succeeded' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | exit 0 and no line printed — every measured row quotes a status code, the forge's refusal text or a probe count, or says COULD-NOT-CHECK |
| 3 | gate:model +dereference | Re-run ONE store row and ONE forge row from the recorded command text against your own fixture | the recorded outcome reproduces; a mismatch is checked-failed on this brief |
| 4 | check | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the reviewer write inventory |
| 5 | check | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the claim reader inventory or is the claim tool itself |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A reviewer-role write is missed and the narrowed reviewer later fails at that verb | row 4 |
| A claim reader is missed and goes blind when claims leave the forge | row 5 |
| A network-filesystem row is filled from general knowledge and presented as measured | row 2 (a measured row must carry a count) and row 3 |
| A fixture's plan or tier differs from an adopter's | review-only — the row records plan and tier so a reader can tell |
| The fork-hosted head case is skipped because it is awkward | row 2 — F8 must carry a result or an explicit COULD-NOT-CHECK |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no; documents only, on fixtures the
operator owns). Reviewer records verdict + date in the stream README table.
