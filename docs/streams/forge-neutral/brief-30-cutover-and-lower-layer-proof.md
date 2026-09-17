---
brief: assay:assay:forge-neutral:30
title: Cutover and removal — release N proves the narrowed reviewer, the operator narrows the grant, release N+1 deletes the forge store
why: >-
  A grant narrowed before the tools accept it stops the review desk; a store switched while
  another dispatcher is live double-dispatches; a grant never narrowed leaves the work without
  effect; and a legacy store left in place forever keeps a reason for the reviewer to hold
  repository write. The cutover needs one written plan across two releases: prove and narrow
  in the first, delete the old store in the second.
wave: 7
depends: ["forge-neutral/22", "forge-neutral/23", "forge-neutral/24", "forge-neutral/25", "forge-neutral/29"]
unblocks: ["forge-neutral/31"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them (removal schedule: one release window — N ships `file` and the serve mode with an unset key still resolving to the forge store under a NOTICE naming the removal release; N+1 deletes the store and refuses an unset key)"
  - "the spec's §7 (two releases; readers before writers) and §11 (order of operations)"
  - "tools/desk/internal/deskkit/preflight.go:835-838 — after a permission change the cached token carries the old grant; re-mint fresh"
  - "tools/desk/cmd/deskclaim-ref/gogit.go and claim.go:32 — the forge store implementation and its namespace, the code slice 2 deletes"
  - "docs/UPGRADING.txt and docs/release-notes/ — where a release-bound operator step is announced"
  - "routed here from assay:assay:forge-neutral:21 (the forge store and the unset-key resolution), assay:assay:forge-neutral:23 (the forge-side mixed-store read) and assay:assay:forge-neutral:25 (the legacy duty row) — the deletions slice 2 performs"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "questions (b) and (c): the proof spans the resolver, the readers, the boot check, the forge and the adopter guide at one pinned release; the deletion removes a mutual-exclusion store while installs may still depend on it"
gate-why: >-
  This brief is where a role identity's grant actually changes on real installations and
  where a claim store is deleted. It is one brief of effort L landing as two slices because
  the ruling makes the removal one plan across two releases. The grant change is a human act
  and is NOT performed by this brief or any tool. The human confirms the fixture proof is
  genuine (a credential that truly holds no repository write; a forge-side refusal), that
  the window has run and installs were warned on every boot, and only then approves the
  deletion.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/cmd/deskclaim-ref (the forge store and its transport): follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/claimstore.go (legacy resolution, forge-side mixed-store read): follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/preflight.go (the legacy duty row): follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskdispatch/dispatch.go (the claim-step credential mint): follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/UPGRADING.txt and the release-N+1 notes: follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/dispatch-claim.sh in consumer repositories: out-of-scope (it lives in each consumer repository; after N+1 nothing in this tree invokes it, and the release notes tell consumers to delete it)"
  - "each adopter's reviewer App permission set or GitLab settings: out-of-scope (a human, admin-side act per installation; no tool in this tree performs it)"
version: 1
id: 183da2db-7fc2-4834-ac56-11d3c576a431
---

# Brief 30 — Cutover and removal

## Context
files:
- Slice 1 (during the release-N window): `docs/streams/forge-neutral/reviewer-write-boundary.md` — §11 gains the dated proof table.
- Slice 2 (for release N+1): `tools/desk/cmd/deskclaim-ref/gogit.go`, `claim.go`, `main.go`;
  `tools/desk/internal/deskkit/claimstore.go` (planned) — brief 21's file, present by then;
  `tools/desk/internal/deskkit/preflight.go`; `tools/desk/cmd/deskdispatch/dispatch.go`;
  `tools/desk/README.md`; `docs/UPGRADING.txt`; `docs/release-notes/<release N+1>.md` (planned).
- `changelog/<branch-slug>.md` (planned), one per slice.

This brief lands as **two slices under one brief**, as the stream has done before for a brief
that spans a release boundary. Slice 1 changes documents only. Neither slice changes a grant.

single-point-of-failure: the written order (readers before writers; drain before switch; grant
change after the switch; deletion after the window) — behind it, the tools themselves: a store
switched beside a live dispatcher is refused by the mixed-store check (brief 23); a reviewer
narrowed while still on the legacy store is refused at boot naming both remedies (brief 25) and
again at dispatch (brief 21); and after N+1 an install that never set the key is refused at
boot with the two valid values in front of it. Every wrong order produces a loud stop, not a
silent gap.

facts:
- **Release N** contains briefs 21–25, 28 and 29: both stores, readers on the seam, store-aware
  duties, scaffolds, docs. An unset key still resolves to the forge store under the removal
  NOTICE. Nothing forces a switch in N.
- **During the window**, per cell: drain, set the store key on every dispatching process,
  resume. Then the fixture proof (slice 1). Then **the operator narrows the reviewer to
  repository read** — a human act per installation, after the cell has switched. After any
  permission change, re-mint fresh before booting.
- **Release N+1** (slice 2) deletes: the forge store implementation and its git transport use
  for claims; the legacy resolution of an unset key (now a refusal printing `file` and
  `service`); the forge-side mixed-store read and the reverse check; the legacy reviewer duty
  row and its store line; the claim-step credential mint in `deskdispatch`; every README
  mention. The conformance table keeps running for the two remaining backends.
- Slice 2 does not depend on any operator having narrowed a grant. It depends on the window
  having run: release N published, and the removal NOTICE having named this release.
- What is lost is stated in the release notes in the spec's words (§4.3): machines that share
  nothing but the forge and cannot reach one service have no supported store.
- Rollback: during the window, restore the grant and reverse the drain. After N+1, re-pin N.
- The proof is run by someone who did not implement briefs 21, 22, 23 or 25.

## Human decision
The tools have been changed so the reviewing identity no longer needs write access to the
repository once a cell keeps its dispatch claims in a folder or with the claim service, and a
release containing that change is installed. Two things remain. First, on each real
installation a person with administrator rights reduces the reviewing identity to read-only
access, by hand, after that cell has switched its claim storage; before anyone does, a test
installation is used to show that a read-only reviewing identity starts correctly, can dispatch
reviews, and is refused by the platform if it tries to push. Second, one release later, the old
way of keeping claims on the platform is deleted from the tools; from then on an installation
that never chose a storage setting stops at start-up and is told the two valid settings. The
decision is whether the evidence is sufficient to narrow, and whether the waiting period has
run so the old storage may be deleted.

Options:
1. **Narrow now; delete the old storage in the next release** — as ruled.
2. **Narrow now; extend the waiting period by one more release** — for installs known not to
   have switched.
3. **Do not narrow yet** — the tools support both states during the waiting period.

Default if no answer: option 3 for narrowing; the deletion does not land without an answer.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- NEVER change a permission, ruleset or protection on any real installation. The proof runs on a throwaway fixture; the real change is the human's.
- Slice 1 does not start until a release containing briefs 21–25, 28 and 29 exists and the fixture is pinned to it; row 2 checks.
- Slice 2 does not start until release N is published and its removal NOTICE names the release slice 2 targets; row 8 checks.
- Slice 2 deletes a store. Do not delete or loosen any guard, refusal or test that belongs to the two remaining stores; if a deletion reddens one of their tests, STOP and report.

## Task
**Slice 1 — prove, in the window**
1. Build the fixture at the pinned release N.
2. Run rows 2–8; record command, outcome and date in the spec's §11 proof table.

**Slice 2 — remove, for release N+1**
3. Delete the forge store, the legacy resolution, the forge-side mixed-store read, the legacy
   duty row and store line, and the claim-step mint. An unset key refuses, printing the two
   valid values.
4. `docs/UPGRADING.txt` and the N+1 release notes: what was removed, what an unset key now
   does, what is no longer supported, and that consumer repositories may delete their legacy
   claim script.
5. Run rows 9–13.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check | Slice 1. On the fixture: `deskpreflight --version` and `deskclaim-ref --version` | both print a `releaseTag` that is the published release N, not `dev` |
| 3 | check +flow +dereference | **FIXTURE ONLY.** Cell on `file`, reviewer App at repository read, fresh re-mint; boot the reviewer role | `app-scopes-vs-duties` checked-clean; detail carries `claim store: file` and the declared-not-verified notice |
| 4 | check +flow | **FIXTURE ONLY.** Same cell: dispatch a review of an open change; then `deskclaim-ref show fixture--issue-1` and `desksupervise status` | claim-acquire OK with `store file`; both readers show the holder; `git ls-remote` of the fixture shows no claim ref |
| 5 | check +flow | **FIXTURE ONLY.** A second cell on `service` (serve mode on loopback), reviewer at repository read: boot and dispatch | clean boot with `claim store: service`; dispatch succeeds; no claim ref on the fixture |
| 6 | check +flow | **FIXTURE ONLY.** Same reviewer credential, plain git: `git push origin HEAD:refs/heads/probe-cutover` | REFUSED by the forge for lack of write access, quoting the forge's text (spec V11) |
| 7 | check | **FIXTURE ONLY.** While a live forge claim exists for the repo, set the store key to `file` and dispatch | refused before any worktree, naming the live forge claim and the drain — the half-done migration is loud |
| 8 | check | **FIXTURE ONLY.** Cell with no store key, reviewer at repository read, boot | refused at boot naming both remedies, and the removal NOTICE names a release — the wrong-order case is loud. Record the named release: slice 2 targets exactly that one |
| 9 | check:ci | Slice 2. `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStore' -count=1 -timeout 120s -v` | exit 0; unset key → refused, message contains both `file` and `service`; no subtest resolves a forge store (spec V8, N+1 half) |
| 10 | check:ci | `cd tools/desk && ! grep -rn -e 'refs/dispatch' -e 'forge-ref' --include='*.go' cmd/deskclaim-ref cmd/deskdispatch internal/deskkit` | exit 0 and no line printed — no forge store code or name remains at those sites |
| 11 | check:ci +neighbour | `cd tools/desk && go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s` | exit 0 — the two remaining backends still pass the whole table |
| 12 | check:ci +neighbour | `cd tools/desk && go test ./cmd/deskclaim-ref/ ./cmd/deskdispatch/ ./cmd/desksupervise/ -count=1 -timeout 900s` | exit 0 — nothing that belongs to the remaining stores went with the deletion |
| 13 | check | `grep -n -i -e 'unset' -e 'no longer supported' docs/UPGRADING.txt` | the N+1 entry states the refusal and the unsupported topology |
| 14 | check | `(cd statusgen && go build -o /tmp/statusgen-fn30 .) && /tmp/statusgen-fn30 --root . --consumers --brief forge-neutral/30` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The proof runs on a dev build | row 2 |
| Row 6's refusal comes from a desk guard | row 6 requires plain git and the forge's text |
| A cell is switched while another dispatcher is live | row 7 |
| An operator narrows a cell that has not switched | row 8 shows a loud refusal; the docs' order (brief 29) |
| Slice 2 lands before installs were warned for a whole window | row 8 records the release the NOTICE names; the ground rule; the human gate |
| The deletion takes a guard of the remaining stores with it | rows 11, 12 |
| A forge-store remnant keeps a reason for the reviewer to hold write | row 10 |
| This brief's implementer narrows a real installation "to finish the job" | Ground rules; review — slice 1's diff contains documents only |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; the cutover of a role identity's grant and the deletion of a claim store). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a mis-ordered cutover and a dark review desk or a double dispatch? (The written order; beneath it the mixed-store refusal and the boot and dispatch refusals that name the remedy.)
2. Does any row prove the lower layer with the upper bypassed? (Row 6: no desk tool in the path. Rows 7 and 8: the order deliberately violated. Rows 11–12: the deletion checked against the stores that remain.)
