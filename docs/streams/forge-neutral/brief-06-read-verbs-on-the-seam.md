---
brief: forge-neutral/06
title: Read verbs — deskboard, issueboard, scanloop and the loop planners on the seam
why: >-
  A desk that cannot read its board on a forge cannot run a loop there, and a read that returns
  an empty list because the CLI was not present is worse than one that fails — an absence reads
  like an answer. These four are the fleet's eyes: the PR/issue board, the issue board, the
  inbound scan and the loop planners that consume them. Routing them through the resolver makes
  the board readable on any configured forge and makes an unreadable one say so.
wave: 3
depends: ["forge-neutral/01", "forge-neutral/03"]
unblocks: ["forge-neutral/10"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver and the could-not-check refusal these reads inherit"
  - "docs/streams/forge-neutral/brief-03-write-verbs-comment-and-flip.md — the wiring shape established on the write side, reused here"
  - "tools/desk/internal/forgeban/allowlist.go:68,172,188,227 — the four permit rows this brief retires, and the (no-op) blocker each names"
  - "freshness-checked 2026-09-02 @ deae247 — deskboard shells gh at board.go:143, issueboard at board.go:147, scanloop at trust.go:159 and lane.go:275; reviewloop consumes deskboard JSON on stdin (main.go:69-70) and reaches no forge itself"
exec-tier: strong
exec-tier-why: "correctness depends on cross-component reasoning: a read that degrades to an empty result rather than a refusal silently changes what every downstream loop believes about the board (question b)."
domain: complicated
consumers:
  - "tools/desk/cmd/deskboard: fixed-here"
  - "tools/desk/cmd/issueboard: fixed-here"
  - "tools/desk/cmd/scanloop: fixed-here"
  - "tools/desk/internal/forgeban/allowlist.go: fixed-here (four rows removed, ceiling lowered to 10)"
  - "tools/desk/cmd/reviewloop, verifyloop, commsloop: out-of-scope (they consume board JSON on stdin and reach no forge of their own; nothing in them changes when the board's transport does)"
  - "tools/desk/cmd/deskroster, deskpushguard, repohardenguard, deskadvisory, deskdigest, deskdisposition, deskmerge, deskdispatch: out-of-scope (each remaining permit row is either an ambient-credential custody decision or an operation with no enumerated Forge method and no settled GitLab mapping; each is its own follow-up brief, named in the stream README)"
---

# Brief 06 — Read verbs on the seam

## Context
files:
- `tools/desk/cmd/deskboard/board.go` — the board's whole read surface.
- `tools/desk/cmd/issueboard/board.go` — the issue board's read.
- `tools/desk/cmd/scanloop/trust.go`, `tools/desk/cmd/scanloop/lane.go` — the trust probe and
  the lane's one write, plus its unresolved-argv launch site.
- `tools/desk/internal/forgeban/allowlist.go` — four permit rows removed, ceiling lowered.
- `docs/streams/forge-gitlab/inventory.md` — any newly enumerated read op and its consumers.

**Why the risk answers are all `no`.** These are reads. The one write in scope is
`scanloop`'s `gh pr edit` label call, which rides the label operation `forge-neutral/03`
already added under its own gate. No credential, permission or trust decision changes here;
the custody binding was settled under the human gate in `forge-neutral/01`.

single-point-of-failure: the resolver decides which forge is read; a wrong answer yields a
board of the wrong place. The independent second layer is the three-state contract itself —
`deskboard` already reports a repository its installation cannot resolve as per-repo
could-not-check rather than failing the whole sweep, so a read that goes wrong on one repo is
visible on the board rather than absorbed into a shorter list.

facts:
- `deskboard` is self-declared read-only and shells `gh` through one helper
  (`tools/desk/cmd/deskboard/board.go:143`); the permit row names its whole read surface —
  *"pr list, pr diff, repo contents, the compare endpoint, two GraphQL queries and the
  issues-by-label walk"*, of which four have enumerated `Forge` methods and the rest do not
  (`tools/desk/internal/forgeban/allowlist.go:68-72`).
- `issueboard` shells `gh` at `tools/desk/cmd/issueboard/board.go:147` and treats any login
  with a `[bot]` suffix as a bot (`board.go:102`) — a GitHub-only rendering that
  `forge-neutral/02`'s per-forge rendering set replaces.
- `scanloop` holds two rows: the trust probe (`tools/desk/cmd/scanloop/trust.go:159`) and an
  unresolved-argv launch site (`tools/desk/cmd/scanloop/lane.go`, registered at
  `allowlist.go:227`). Its one forge write is `gh pr edit` at
  `tools/desk/cmd/scanloop/lane.go:275`.
- `reviewloop` reaches no forge: it consumes `deskboard actions`/`prs` JSON on stdin or flags
  and emits a plan (`tools/desk/cmd/reviewloop/main.go:69-70`). It is listed here so its
  absence from the task list is a recorded finding rather than an oversight.
- `Forge` already carries `GetPullRequest`, `GetIssue`, `ReviewsAtHead`, `ListChangedFiles`,
  `ChecksAtHead`, `IssueReactions` and `RepoVisibility`
  (`tools/desk/internal/deskkit/forge.go:177-191`). Listing changes/issues by state or label,
  and the compare endpoint, are NOT enumerated.
- The freeze rule requires any added op to land with its consuming call site in the same
  change (`tools/desk/internal/deskkit/forge.go:169-172`).
- An empty result and a failed read must be distinguishable: this is the three-state
  instrument rule the desk tools already follow, and it is the property most easily lost in a
  transport swap.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Every verb's existing test suite stays green **unmodified**.
- Do not add an operation you do not convert a call site for in this same change.

## Task
1. Enumerate the read operations these three need that the interface does not yet have —
   change listing by state, issue listing by label, and the compare/diff read — add each to
   `Forge` WITH its consuming call site, implement on both backends, and record each in
   `docs/streams/forge-gitlab/inventory.md` with its consumers. Where a GitLab mapping is not
   1:1, the operation returns could-not-check naming the gap rather than approximating.
2. Route `deskboard`, `issueboard` and `scanloop`'s trust probe through `deskkit.ForgeFor`;
   route `scanloop`'s label write through the label operation `forge-neutral/03` added. Delete
   the shell helpers.
3. Resolve `scanloop`'s unresolved-argv launch site (`allowlist.go:227`) — either by making
   its `argv[0]` a compile-time constant so the ban can resolve it, or by removing the launch.
   An unresolvable launch site is carried as could-not-check by the ban and is never rounded
   to clean; leaving it is leaving a hole the control itself reports.
4. **Preserve the three-state reads.** Every migrated read distinguishes empty from
   unreadable. `deskboard`'s existing per-repo could-not-check behavior must survive, and an
   unsupported-forge read must surface as could-not-check on the board rather than as a
   shorter list.
5. Lower `allowedInvocationCeiling` from 14 to **10** and remove the four retired rows. Leave
   the remaining ten rows in place with their reasons intact — they are named in the stream
   README as declared follow-ups, and silently deleting one would break the ratchet's meaning.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskboard/... ./cmd/issueboard/... ./cmd/scanloop/... -count=1` | exit 0 — all three suites green |
| 3 | `git diff --stat origin/main -- tools/desk/cmd/deskboard tools/desk/cmd/issueboard tools/desk/cmd/scanloop \| grep -c '_test.go' \|\| true` | prints `0` — no verb's own tests were edited to make the migration pass |
| 4 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows `= 10` |
| 5 | `cd tools/desk && go test ./internal/forgeban/... -count=1 -v` | exit 0 — the ratchet passes at 10, and the ten surviving rows each still match a live call site |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1 -v` | exit 0 |
| 7 | `grep -rn -e '"gh"' tools/desk/cmd/deskboard tools/desk/cmd/issueboard tools/desk/cmd/scanloop --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — independent cross-check of row 6 by a different instrument |
| 8 | `cd tools/desk && go test ./internal/forgeban/... -run TestUnresolvedArgvLedger -count=1 -v` | exit 0; the `scanloop/lane.go::RealExec` unresolved-argv row is gone from the ledger, not merely permitted |
| 9 | `cd tools/desk && go test ./cmd/deskboard/... -run TestUnreadableRepoIsCouldNotCheck -count=1 -v` | **negative path**: a repo the resolved forge cannot read appears as an explicit could-not-check row, NOT as an omitted row and NOT as an empty result — asserted on the JSON output, so a shorter list fails the row |
| 10 | `cd tools/desk && go test ./cmd/issueboard/... -run TestEmptyVersusUnreadable -count=1 -v` | **negative path**: a genuinely empty issue list and an unreadable one produce different exit codes and different output; the test fails if they are indistinguishable |
| 11 | `cd tools/desk && go test ./internal/deskkit/ -run TestReadOpsBothBackends -count=1 -v` | exit 0 — the newly enumerated read ops run the same scenario names against both backends' recorded fixtures |
| 12 | `statusgen --root . --consumers --brief forge-neutral/06` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A migrated read swallows a failure and returns an empty list, so every downstream loop reads an idle board | rows 9 + 10, both asserting on output rather than on exit status alone |
| The board's existing per-repo could-not-check behavior is lost in the transport swap | row 9 |
| A read op is added speculatively with no call site, breaking the freeze rule | row 11 plus the inventory entry naming consumers |
| The old shell helper stays reachable — the `#274` shape | rows 6 + 7 (two instruments) |
| The unresolved-argv site is left, so the ban keeps a hole it reports and nobody closes | row 8 |
| The ratchet is lowered by DELETING surviving rows rather than by migrating call sites | row 5, whose ratchet test fails when a permit row matches no live call site |
| `reviewloop` is assumed to need work and is churned for nothing, or is assumed not to and quietly does | recorded in `facts:` with its citation and routed `out-of-scope` in `consumers:`; row 12 corroborates the routing against the diff |
| A GitLab list-by-label mapping is approximated because the semantics nearly match | row 11 runs identical scenario names on both backends; a divergence is a named failing scenario |
| The migration is correct but slower, and a board sweep starts timing out | **no row** — review-only. Latency is an adequacy judgement; the Review gate reads the pagination shape of each new op |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — see the note in
`## Context`). Reviewer records verdict + date in the stream README table, and confirms the
ten surviving permit rows still carry their original reasons.
