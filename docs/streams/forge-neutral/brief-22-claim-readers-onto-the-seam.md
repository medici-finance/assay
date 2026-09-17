---
brief: assay:assay:forge-neutral:22
title: Claim readers onto the seam — the supervisor, the verdict stamp, the fan-out release and the roster read the resolved store
why: >-
  Several tools read or release dispatch claims by looking at the forge directly. The moment a
  cell keeps its claims anywhere else, those tools see an empty namespace: the supervisor
  reports every slot free, the verdict stamp ages out live work, and a release deletes
  nothing. No non-forge store can be selected safely until every reader asks the same resolver
  the dispatcher asks.
wave: 3
depends: ["forge-neutral/21"]
unblocks: ["forge-neutral/28", "forge-neutral/30"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "tools/desk/cmd/desksupervise/live.go:35 — live claim enumeration listed straight from the forge"
  - "tools/desk/cmd/deskpost/claimliveness.go:41-61 — the verdict stamp's claim-liveness read (`RefExists`)"
  - "tools/desk/cmd/fanoutloop/land.go:81-154 — release through `Forge.DeleteRef`"
  - "tools/desk/internal/loopengine/writescope_io.go:31-65 — local `for-each-ref` over the claim namespace"
  - "routed here from assay:assay:forge-neutral:21 (claim readers outside the claim tool)"
  - "the claim reader inventory brief 20 adds to the spec's §3.1 — the authoritative work list; the four sites above are the ones known at authoring"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): a sweep across every claim reader, where a missed site fails silently as 'no claims' rather than as an error"
gate-why: >-
  Readers decide when a claim is dead and may be reclaimed, and when a verdict stamp has
  aged out — claim custody. A reader left on the forge after claims move reports slots free
  that are held. The human confirms every reader resolves the same store as the dispatcher
  and that an unreadable store is could-not-check, never 'empty'.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/cmd/desksupervise: follow-up forge-neutral/22 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskpost/claimliveness.go: follow-up forge-neutral/22 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/fanoutloop/land.go: follow-up forge-neutral/22 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/loopengine/writescope_io.go: follow-up forge-neutral/22 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskroster (list join): follow-up forge-neutral/22 (this brief; flips to fixed-here when the implementation edits the path)"
  - "skill bodies that read the claim namespace with git: follow-up forge-neutral/29"
version: 1
id: 5babe9aa-27f1-4ccf-988c-97eab1bc4368
---

# Brief 22 — Claim readers onto the seam

## Context
files:
- `tools/desk/cmd/desksupervise/live.go`, `actions.go`
- `tools/desk/cmd/deskpost/claimliveness.go`, `forgeclient.go`
- `tools/desk/cmd/fanoutloop/land.go`
- `tools/desk/internal/loopengine/writescope_io.go`
- `tools/desk/cmd/deskroster/` (the list join)
- `tools/desk/internal/deskkit/mutations.json`; `changelog/<branch-slug>.md` (planned)

single-point-of-failure: each reader's use of the resolver — behind it, a source-level test
that fails when any non-test file outside the store implementations names the forge claim
namespace (a missed or newly added direct reader is caught at build time, in a different
component from the readers themselves).

facts:
- Only `forge-ref` is selectable when this brief lands (brief 21), so every change here is
  behaviour-preserving and provable against today's suites.
- The three-state rule holds per reader: a store that cannot be read is could-not-check. It is
  never rendered as "no claims", "free", or "released".
- The verdict stamp's liveness mapping is unchanged: only a positive ABSENT ages a stamp out
  (`deskpost/claimliveness.go:36-40`).
- Release stays compare-and-delete where the reader has the holder it expects
  (`desksupervise/actions.go:28`).

## Human decision
Several background tools look directly at the code-hosting platform to see which work items
are claimed, to free claims that have gone stale, and to decide whether a review verdict is
still current. If a cell later keeps its claims somewhere else, those tools would see nothing
and treat held work as free. The proposal makes every one of them ask the same single decision
point the dispatcher uses, and adds a build-time check that no tool reads claims any other way.
Behaviour today does not change. The decision is whether this is the accepted way to keep
claim readers and claim writers in agreement.

Options:
1. **Approve as specified.**
2. **Approve without the build-time check** — smaller, but a reader added later can silently
   bypass the decision point.
3. **Reject** — claims must stay on the platform.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- Work from brief 20's reader inventory. A reader found that is not in it is a finding: add it to the inventory in the same change.

## Task
1. Route each reader's list / show / release through `ResolveClaimStore`.
2. Add `TestNoDirectClaimNamespaceReads`: fails naming the file when a non-test Go file
   outside the store implementations references the claim namespace constants or literals.
3. Mutation entry: make one reader treat a store read error as an empty list.
4. Keep every reader's output format unchanged.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestNoDirectClaimNamespaceReads' -count=1 -timeout 120s` | exit 0 |
| 3 | check:ci | `cd tools/desk && go test ./cmd/desksupervise/ ./cmd/fanoutloop/ ./cmd/deskroster/ -count=1 -timeout 600s` | exit 0 — existing suites pass unchanged |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskpost/ -run 'TestClaimLiveness' -count=1 -timeout 120s -v` | exit 0; an unreadable store yields Unknown, never Absent |
| 5 | check:ci +mutation | the `mutations.json` entry named `claimreader-error-reads-as-empty` | the supervisor's could-not-check test goes RED |
| 6 | check:ci +flow | `cd tools/desk && go test ./cmd/desksupervise/ -run 'TestSupervisorSeesClaimFromResolvedStore' -count=1 -timeout 120s` | exit 0 — a claim placed through an in-memory non-forge store double is listed by the supervisor and is NOT reported free (spec V5, reader half) |
| 7 | check | `(cd statusgen && go build -o /tmp/statusgen-fn22 .) && /tmp/statusgen-fn22 --root . --consumers --brief forge-neutral/22` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A reader is missed and reports held slots free once claims move | rows 2 and 6 |
| A read error is flattened to "no claims" | rows 4, 5 |
| A reader's output changes and breaks a parser | row 3 |
| A new direct reader is added after this lands | row 2, permanently |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; stale-claim reclaim and verdict-stamp ageing). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a relocated claim and a reader that calls it free? (Each reader's use of the resolver; beneath it the source-level test.)
2. Does any row prove the lower layer with the upper bypassed? (Row 5 breaks one reader's error handling and shows the suite catches it; row 2 catches a reader that skips the resolver entirely.)
