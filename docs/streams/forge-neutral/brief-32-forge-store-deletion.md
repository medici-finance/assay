---
brief: assay:assay:forge-neutral:32
title: Release-N+1 deletion — the forge claim store is removed and an unset store key is refused
why: >-
  A legacy store left in place forever keeps a reason for the reviewer to hold repository
  write, and keeps the unresolved namespace split and the GitLab release problem alive with
  it. Deleting it is the step that cannot be half-done: an install that still depends on it
  stops at boot. So it happens one release after the cutover, on a release of evidence, and
  behind its own human sign-off.
wave: 8
depends: ["forge-neutral/30"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them (removal schedule: one release window, N+1 deletes the store and refuses an unset key; shape: the deletion is its own brief, human-gated, depending on brief 30, with its own Verify table)"
  - "the spec's §4.3 (what is lost), §5 step 3 (the refusal), §7 (release N+1) and §11 (order of operations)"
  - "docs/streams/forge-neutral/brief-30-cutover-and-lower-layer-proof.md — the release-N cutover; its row 8 records the release the removal NOTICE names, which is the release this brief targets"
  - "tools/desk/cmd/deskclaim-ref/gogit.go and claim.go:32 — the forge store implementation and its namespace, the code this brief deletes"
  - "tools/desk/cmd/desksupervise/live.go:35, tools/desk/cmd/deskpost/claimliveness.go:41-61, tools/desk/cmd/fanoutloop/land.go:81-154 — the claim readers brief 22 moved onto the seam; this brief must leave them working"
  - "docs/UPGRADING.txt and docs/release-notes/ — where a release-bound operator step is announced"
  - "routed here from assay:assay:forge-neutral:21 (the forge store and the unset-key resolution), assay:assay:forge-neutral:23 (the forge-side mixed-store read), assay:assay:forge-neutral:25 (the legacy duty row) and assay:assay:forge-neutral:30 (the proof and the recorded release this deletion starts from)"
  - "freshness-checked 2026-09-17 @ 65e7f4b1 (origin/main)"
exec-tier: strong
exec-tier-why: "question (c): the deletion removes a mutual-exclusion store while installs may still depend on it, and shares files with the guards of the two stores that remain — a guard removed with it survives every test that was deleted alongside"
gate-why: >-
  This brief deletes a claim store from the tools. From the release that carries it, an
  installation that never set a store key stops at boot, and machines that share nothing but
  the forge have no supported store. The risk answer `irreversible` is `no` because the
  change can be walked back — re-pin release N, or revert the deletion — but it is the half
  of the removal that cannot be half-done, so it is signed off on its own. The human confirms
  that release N was published and its removal NOTICE named this release, that a whole
  release window has run, that brief 30's proof is recorded, and that the diff removes
  nothing belonging to the two remaining stores.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/cmd/deskclaim-ref (the forge store and its transport): follow-up forge-neutral/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/claimstore.go (legacy resolution, forge-side mixed-store read): follow-up forge-neutral/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/preflight.go (the legacy duty row): follow-up forge-neutral/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskdispatch/dispatch.go (the claim-step credential mint): follow-up forge-neutral/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/UPGRADING.txt and the release-N+1 notes: follow-up forge-neutral/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/dispatch-claim.sh in consumer repositories: out-of-scope (it lives in each consumer repository; after N+1 nothing in this tree invokes it, and the release notes tell consumers to delete it)"
version: 1
id: f7d785b6-1a22-4da6-a4a0-afe5d7018d4a
---

# Brief 32 — Release-N+1 deletion of the forge claim store

## Context
files:
- `tools/desk/cmd/deskclaim-ref/gogit.go`, `claim.go`, `main.go`
- `tools/desk/internal/deskkit/claimstore.go` (planned) — brief 21's file, present by then.
- `tools/desk/internal/deskkit/preflight.go`; `tools/desk/cmd/deskdispatch/dispatch.go`
- `tools/desk/README.md`; `docs/UPGRADING.txt`; `docs/release-notes/<release N+1>.md` (planned)
- `changelog/<branch-slug>.md` (planned)

Split out of brief 30 by ruling (2026-09-17; spec §10, the row on brief 30's shape). Brief 30
is the release-N cutover; this brief is the release-N+1 deletion. It changes no grant.

single-point-of-failure: the human gate on this brief, exercised on recorded evidence — the
published release N, the release its removal NOTICE names (brief 30 row 8), a window that has
run. Behind it, in a different component and at a different time: an install that never set
the key is refused **at boot**, with the two valid values in front of it (row 3) — it stops
loudly and does not double-dispatch. And behind that, under every store, the supervisor's
staleness reclaim and branch-as-claim are untouched by this brief.

facts:
- Deleted: the forge store implementation and its git transport use for claims; the legacy
  resolution of an unset key; the forge-side mixed-store read and the reverse check (brief 23's
  precondition 5); the legacy reviewer duty row and its `claim store: forge-ref (legacy …)`
  line (brief 25); the claim-step credential mint in `deskdispatch`; every README mention.
- After this brief an unset `ASSAY_CLAIM_STORE` is a refusal, exit 6, before any worktree,
  printing the two valid values `file` and `service` (spec §5 step 3). `forge-ref` as an
  explicit value was already refused in release N and stays refused.
- **Claim readers are unaffected.** The supervisor's live enumeration, the verdict stamp's
  liveness read and the fan-out release have read the resolved store through the seam since
  brief 22. They read `file` and `service` claims after this brief exactly as before it. The
  conformance table keeps running for the two remaining backends.
- This brief does not depend on any operator having narrowed a grant. It depends on the
  window having run: release N published, its removal NOTICE having named the release this
  brief targets, and brief 30 done.
- What is lost is stated in the release notes in the spec's words (§4.3): machines that share
  nothing but the forge and cannot reach one service have no supported store.
- Rollback after this brief ships is re-pinning release N.
- Effort M: the deletions are spread over five code sites that also hold the remaining stores'
  guards, plus the upgrade note and release notes; no new behaviour is designed here.

## Human decision
One release ago the tools began keeping dispatch claims in a folder or with the claim service,
and warned — at every start — any installation still keeping claims on the code-hosting
platform that this old way would be removed, naming the release. This change is that removal.
From the release that carries it, the old way no longer exists in the tools: an installation
that never chose a storage setting stops at start-up and is told the two valid settings, and
a group of computers that share nothing but the platform and cannot reach one claim service
has no supported way to coordinate. Going back means reinstalling the previous release. The
decision is whether the waiting period has run and the evidence from the first release is
recorded, so the old storage may now be deleted.

Options:
1. **Delete in this release** — as ruled: the named release has arrived and the window ran.
2. **Extend the waiting period by one more release** — for installations known not to have
   switched. The start-up notice must then be changed to name the later release, as its own
   small change, before this brief starts.
3. **Do not delete** — keep the old storage; the reviewing identity keeps a reason to hold
   write access wherever a cell has not switched.

Default if no answer: none — blocks until answered. The deletion does not land without one.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- Do not start until release N is published, brief 30 is done, and the release recorded by brief 30's row 8 is the release this work targets; row 2 checks.
- This brief deletes a store. Do not delete or loosen any guard, refusal or test that belongs to the two remaining stores; if a deletion reddens one of their tests, STOP and report.
- NEVER change a permission, ruleset or protection on any installation.

## Task
1. Delete the forge store, the legacy resolution, the forge-side mixed-store read and the
   reverse check, the legacy duty row and store line, and the claim-step mint. An unset key
   refuses, printing the two valid values.
2. Mutation entry `claimstore-unset-key-resolves`: make an unset key resolve to `file` instead
   of refusing.
3. `docs/UPGRADING.txt` and the N+1 release notes: what was removed, what an unset key now
   does, what is no longer supported (in the spec's §4.3 words), and that consumer
   repositories may delete their legacy claim script.
4. Run rows 2–11.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check +dereference | Read the release recorded in the spec's §11 proof table (brief 30 row 8), then `grep -n -F '<that release>' docs/UPGRADING.txt` and compare with the version this change is cut for | the proof table names a release; it is the release this change targets; a mismatch or an empty proof table means STOP — the window has not been shown to have run |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestResolveClaimStore' -count=1 -timeout 120s -v` | exit 0; unset key → refused, exit 6, message contains both `file` and `service`; `forge-ref` as a value → refused with the same two values; no subtest resolves a forge store (spec V8, N+1 half) |
| 4 | check:ci +mutation | the `mutations.json` entry named `claimstore-unset-key-resolves` | row 3 goes RED — an unset key that quietly resolves is the silent case the refusal exists to prevent |
| 5 | check:ci | `cd tools/desk && ! grep -rn -e 'refs/dispatch' -e 'forge-ref' --include='*.go' cmd/deskclaim-ref cmd/deskdispatch internal/deskkit` | exit 0 and no line printed — no forge store code path or name remains at those sites |
| 6 | check:ci +neighbour | `cd tools/desk && go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s` | exit 0 — the two remaining backends still pass the whole table |
| 7 | check:ci +neighbour | `cd tools/desk && go test ./cmd/desksupervise/ ./cmd/deskpost/ ./cmd/fanoutloop/ -count=1 -timeout 900s` | exit 0 — the claim readers (supervisor enumeration, verdict-stamp liveness, fan-out release) pass unchanged; this change's diff touches none of their test files |
| 8 | check +flow | With `ASSAY_CLAIM_STORE=file` and the declaration set, against a fixture repository: run a review dispatch, then `deskclaim-ref show fixture--issue-1` and `desksupervise status`; repeat with `service` on loopback | under both stores the dispatch acquires and both readers show the holder — claim readers are unaffected by the deletion |
| 9 | check:ci +neighbour | `cd tools/desk && go test ./cmd/deskclaim-ref/ ./cmd/deskdispatch/ -count=1 -timeout 900s` | exit 0 — nothing that belongs to the remaining stores went with the deletion |
| 10 | check | `grep -n -i -e 'unset' -e 'no longer supported' docs/UPGRADING.txt` | the N+1 entry states the refusal and the unsupported topology |
| 11 | check | `(cd statusgen && go build -o /tmp/statusgen-fn32 .) && /tmp/statusgen-fn32 --root . --consumers --brief forge-neutral/32` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The deletion lands before installs were warned for a whole window, or targets a release the NOTICE never named | row 2; the ground rule; the human gate |
| An unset key quietly resolves to a remaining store instead of refusing | rows 3, 4 |
| A forge-store remnant keeps a reason for the reviewer to hold write | row 5 |
| The deletion takes a guard of the remaining stores with it | rows 6, 9 |
| A claim reader still reaches for the forge namespace and goes blind or errors | rows 7, 8 |
| An install that ignored the NOTICE double-dispatches instead of stopping | row 3 — the refusal is before any worktree |
| The release notes understate what is lost | row 10; review reads the entry against the spec's §4.3 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; the deletion of a claim store). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a premature deletion and installs that stop without warning? (The human gate on recorded evidence; beneath it the boot refusal that names the two valid values, so the failure is a loud stop and never a double dispatch.) Is that acceptable?
2. Does any row prove the lower layer with the upper bypassed? (Row 4 removes the refusal and shows row 3 catches it. Rows 7 and 8 run the claim readers with the forge store gone. Rows 6 and 9 check the deletion against the stores that remain.)
