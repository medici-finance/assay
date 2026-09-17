---
brief: assay:assay:forge-neutral:31
title: Remaining roles' write audit — what the desk, worker, verifier and loop roles actually write, measured after the reviewer change is live
why: >-
  Every desk role is granted the same three writes, and only the reviewer's has been examined.
  It was ruled that every other role is audited and narrowed to what it really uses — after
  the reviewer change is live, so that fix does not wait. Narrowing a role on a guess either
  breaks it mid-pass or leaves the surplus in place; this brief is the measurement the later
  per-role duties briefs are authored from.
wave: 8
depends: ["forge-neutral/30"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the ruling of 2026-09-17 on the spec's §10 I, and the spec's §12 — every remaining role is audited and narrowed as a follow-on wave behind brief 30; the per-role duties briefs are authored later, from this brief's findings"
  - "docs/streams/forge-neutral/brief-20-claim-store-measurements.md — the inventory method reused here (Task 1 and Verify row 4)"
  - "tools/desk/internal/deskkit/preflight.go — `dutiesFor(role, store)` as left by brief 25: already role-keyed, so a narrowed role is a row change"
  - "routed here from assay:assay:forge-neutral:25 (duties of the roles other than the reviewer)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): a sweep across every verb that can act as each of five roles; a missed call site is a role that breaks after it is narrowed"
domain: complicated
version: 1
id: 2799ba50-f6ef-4ee6-beb2-6ca378f909d3
---

# Brief 31 — Remaining roles' write audit

## Context
files:
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — §12 gains one inventory table per role and a findings list.
- `changelog/<branch-slug>.md` (planned)

**Placeholder by design.** This brief is authored now so the follow-on wave has a head; its
method is fixed, its findings are not knowable yet. It writes no tool code and proposes no
duty change. The per-role duties briefs are authored **after** it, one per finding.

facts:
- Roles in scope: desk, worker, verifier, issue-loop, intake-loop. The reviewer is out of
  scope — briefs 20–30 cover it.
- A role's forge calls are findable by the role argument of the forge resolver and of the
  token mint, as in brief 20.
- For each role the question is per permission: `pull_requests`, `issues`, repository write —
  which operations need it, on which forge, and is there any the role never performs.
- Depends on brief 30 by ruling: it starts once the reviewer change is live, so that the
  claim-step mint it would otherwise have to account for is already gone.

## Ground rules
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved` and brief 30 is done.** Row 1
  checks the first.
- Documents only. Do not change a duty, a grant or a test. A role that looks narrowable is a
  finding to file, not a change to make.
- Do not convert a could-not-check into a result by reasoning or from memory.

## Task
1. For each role in scope, inventory every forge operation it performs through a desk verb,
   with file:line, the permission it needs on each forge, and whether that permission is read
   or write.
2. For each role, state which of the three granted writes it uses, and any it does not.
3. File one finding per role whose grant exceeds its use, naming the surplus. Those findings
   are what the per-role duties briefs are authored from.
4. Record the tables and the findings list in the spec's §12.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check | `cd tools/desk && grep -rln -e 'ForgeFor(' -e 'RoleTokenForRepo(' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in at least one role's inventory table, or is listed as reviewer-only |
| 3 | gate:model +dereference | Pick one operation per role from the tables and open its file:line | the operation, the role argument and the permission are as the table says |
| 4 | check | `grep -c -e '^### Role: ' docs/streams/forge-neutral/reviewer-write-boundary.md` | `5` — one table per role in scope |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A role's call site is missed and the role later breaks when narrowed | row 2 |
| A table row is written from the verb's documentation rather than its code | row 3 |
| The audit quietly narrows something | Ground rules; review — the diff contains documents only |
| A surplus is noticed and not filed, so no duties brief is ever authored for it | review-only — the findings list in §12 is read against the tables |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no; documents only). Reviewer records
verdict + date in the stream README table.
