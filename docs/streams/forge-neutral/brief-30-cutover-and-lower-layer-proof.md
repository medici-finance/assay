---
brief: assay:assay:forge-neutral:30
title: Release-N cutover — ship, prove the narrowed reviewer on a live cell, then the operator narrows the grant
why: >-
  A grant narrowed before the tools accept it stops the review desk; a store switched while
  another dispatcher is live double-dispatches; and a grant never narrowed leaves the work
  without effect. The cutover needs a written order and a proof that it holds at a published
  release, run before anyone touches a real installation's grant. Deleting the old store is a
  separate, later brief (32), so the step that cannot be half-done is signed off on a release
  of evidence.
wave: 7
depends: ["forge-neutral/22", "forge-neutral/23", "forge-neutral/24", "forge-neutral/25", "forge-neutral/29"]
unblocks: ["forge-neutral/31", "forge-neutral/32"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them (removal schedule: one release window — N ships `file` and the serve mode with an unset key still resolving to the forge store under a NOTICE naming the removal release; N+1 deletes the store and refuses an unset key. Shape of this brief: split — this brief is the release-N cutover only; the release-N+1 deletion is brief 32, human-gated, depending on this one)"
  - "the spec's §7 (two releases; readers before writers) and §11 (order of operations)"
  - "tools/desk/internal/deskkit/preflight.go:835-838 — after a permission change the cached token carries the old grant; re-mint fresh"
  - "docs/streams/forge-neutral/brief-32-forge-store-deletion.md — the release-N+1 half, split out of this brief by ruling; row 8 here records the release it targets"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): the proof spans the resolver, the readers, the boot check, the forge and the adopter guide at one pinned release, on a live cell rather than a unit test"
gate-why: >-
  This brief is the evidence a human acts on when a role identity's grant actually changes on
  real installations. The grant change is a human act and is NOT performed by this brief or
  any tool. The human confirms the proof is genuine — run at the published release, on a live
  cell, with a credential that truly holds no repository write, and refused by the forge
  itself — before any operator narrows a real reviewer. Nothing is deleted here; the deletion
  has its own brief and its own human gate (32).
decision-trigger: start
domain: complicated
consumers:
  - "the release-N+1 deletion of the forge store, which starts from this brief's recorded proof and the release its row 8 records: follow-up forge-neutral/32"
  - "each adopter's reviewer App permission set or GitLab settings: out-of-scope (a human, admin-side act per installation; no tool in this tree performs it)"
version: 1
id: 183da2db-7fc2-4834-ac56-11d3c576a431
---

# Brief 30 — Release-N cutover

## Context
files:
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — §11 gains the dated proof table.
- `changelog/<branch-slug>.md` (planned)

This brief changes documents only, and changes no grant. It was authored as a two-release
brief landing in two slices; by ruling (2026-09-17; spec §10, the row on this brief's shape) it is **split**: this brief is
the release-N cutover, and [brief 32](brief-32-forge-store-deletion.md) is the release-N+1
deletion, with its own human gate and its own Verify table. One brief stays one pull request.

single-point-of-failure: the written order (readers before writers; drain before switch; grant
change after the switch) — behind it, the tools themselves: a store switched beside a live
dispatcher is refused by the mixed-store check (brief 23); a reviewer narrowed while still on
the legacy store is refused at boot naming both remedies (brief 25) and again at dispatch
(brief 21). Every wrong order produces a loud stop, not a silent gap.

facts:
- **Release N** contains briefs 21–25, 28 and 29: both stores, readers on the seam, store-aware
  duties, scaffolds, docs. An unset key still resolves to the forge store under the removal
  NOTICE. Nothing forces a switch in N. Cutting and publishing it is the existing human-gated
  release process, not a step of this brief.
- **The sequence this brief sits in: ship, prove, then narrow.** Release N is published. Per
  cell: drain, set the store key on every dispatching process, resume. Then the proof below,
  on a live cell pinned to release N. Then **the operator narrows the reviewer to repository
  read** — a human act per installation, after the cell has switched; still a human act, not
  a step of this or any brief, and no tool performs or prompts it. After any permission
  change, re-mint fresh before booting.
- "Live cell" means a running cell at the published release with real role Apps against a
  throwaway repository — not a unit test and not a dev build. It is never a production
  installation: the grant this brief's runner sets is the throwaway cell's own.
- Row 8 records the release the removal NOTICE names. Brief 32 targets exactly that release.
- Rollback during the window: restore the grant and reverse the drain.
- The proof is run by someone who did not implement briefs 21, 22, 23 or 25.

## Human decision
The tools have been changed so the reviewing identity no longer needs write access to the
repository once a cell keeps its dispatch claims in a folder or with the claim service, and a
release containing that change is installed. What remains is for a person with administrator
rights, on each real installation, to reduce the reviewing identity to read-only access, by
hand, after that cell has switched its claim storage. Before anyone does, a throwaway cell
running the published release is used to show that a read-only reviewing identity starts
correctly, can dispatch reviews, and is refused by the platform if it tries to push. The
decision is whether that evidence is sufficient to narrow real installations. Deleting the old
way of keeping claims is not decided here; it is a separate decision one release later.

Options:
1. **Narrow now** — the evidence is sufficient; operators may reduce the reviewing identity.
2. **Do not narrow yet** — say what is missing. The tools support both states for the whole
   waiting period, so nothing breaks while the answer is pending.

Default if no answer: option 2.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- NEVER change a permission, ruleset or protection on any real installation. The proof runs on a throwaway fixture; the real change is the human's.
- Do not start until a release containing briefs 21–25, 28 and 29 is published and the fixture cell is pinned to it; row 2 checks.
- Delete nothing. The forge store, the legacy resolution and the mixed-store read all stay; their removal is brief 32.

## Task
1. Build the fixture cell at the published release N.
2. Run rows 2–8; record command, outcome and date in the spec's §11 proof table, including the
   release named by the removal NOTICE (row 8).

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check | On the fixture cell: `deskpreflight --version` and `deskclaim-ref --version` | both print a `releaseTag` that is the published release N, not `dev` |
| 3 | check +flow +dereference | **FIXTURE ONLY.** Cell on `file`, reviewer App at repository read, fresh re-mint; boot the reviewer role | `app-scopes-vs-duties` checked-clean; detail carries `claim store: file` and the declared-not-verified notice |
| 4 | check +flow | **FIXTURE ONLY.** Same cell: dispatch a review of an open change; then `deskclaim-ref show fixture--issue-1` and `desksupervise status` | claim-acquire OK with `store file`; both readers show the holder; `git ls-remote` of the fixture shows no claim ref |
| 5 | check +flow | **FIXTURE ONLY.** A second cell on `service` (serve mode on loopback), reviewer at repository read: boot and dispatch | clean boot with `claim store: service`; dispatch succeeds; no claim ref on the fixture |
| 6 | check +flow | **FIXTURE ONLY.** Same reviewer credential, plain git: `git push origin HEAD:refs/heads/probe-cutover` | REFUSED by the forge for lack of write access, quoting the forge's text (spec V11) |
| 7 | check | **FIXTURE ONLY.** While a live forge claim exists for the repo, set the store key to `file` and dispatch | refused before any worktree, naming the live forge claim and the drain — the half-done migration is loud |
| 8 | check | **FIXTURE ONLY.** Cell with no store key, reviewer at repository read, boot | refused at boot naming both remedies, and the removal NOTICE names a release — the wrong-order case is loud. Record the named release: brief 32 targets exactly that one |
| 9 | check | `(cd statusgen && go build -o /tmp/statusgen-fn30 .) && /tmp/statusgen-fn30 --root . --consumers --brief forge-neutral/30` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The proof runs on a dev build | row 2 |
| Row 6's refusal comes from a desk guard | row 6 requires plain git and the forge's text |
| A cell is switched while another dispatcher is live | row 7 |
| An operator narrows a cell that has not switched | row 8 shows a loud refusal; the docs' order (brief 29) |
| The deletion brief later targets the wrong release, or starts before installs were warned | row 8 records the release the NOTICE names; brief 32 reads it and has its own human gate |
| This brief's implementer narrows a real installation "to finish the job", or deletes the old store early | Ground rules; review — this brief's diff contains documents only |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; the evidence on which a role identity's grant is narrowed). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a mis-ordered cutover and a dark review desk or a double dispatch? (The written order; beneath it the mixed-store refusal and the boot and dispatch refusals that name the remedy.)
2. Does any row prove the lower layer with the upper bypassed? (Row 6: no desk tool in the path. Rows 7 and 8: the order deliberately violated.)
