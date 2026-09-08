---
brief: forge-neutral/12
title: deskboard non-board reads onto the seam
why: >-
  forge-neutral/06 migrated deskboard's two hand-authored GraphQL reads (the bulk open-PR read
  and the PR/issue trust queries) onto typed Forge ops, but left deskboard's PERIPHERAL read
  surface on `ghRun` behind ONE narrowed permit row. Those reads — PR search, commit-history
  listing, single-commit reads, the combined-status probe and the workflow-directory listing —
  are the last `gh` in `cmd/deskboard`, and until they reach the interface the board cannot run
  on a non-GitHub forge and the ratchet cannot fall below 13. This brief puts them on the seam
  and retires deskboard's narrowed row (ceiling 13 → 12, or → 9 once 04b has also landed).
wave: 4
depends: ["forge-neutral/06"]
unblocks: ["forge-neutral/10"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-07 by forge-neutral/06's implementer (mid-flight, per the 06 ruling)
sources:
  - "docs/streams/forge-neutral/brief-06-read-verbs-on-the-seam.md — the read established the two-op board.go migration and NARROWED, rather than removed, the deskboard permit row; this brief is that row's declared exit"
  - "tools/desk/internal/forgeban/allowlist.go — the `cmd/deskboard/board.go::ghRun::gh` row whose reason names these five categories and cites this brief as its exit condition"
  - "tools/desk/internal/deskkit/forge.go — the frozen Forge interface each new op is added to under the §6 freeze rule (a method lands with its consuming call site)"
  - "docs/streams/forge-gitlab/inventory.md — where each new op and its GitLab mapping (or could-not-check gap) is tabulated"
exec-tier: strong
exec-tier-why: "several of these reads have a partial or non-1:1 GitLab mapping (code search, a raw combined-status total, a workflow-directory listing), and the RIGHT shape for each op — and whether it belongs on the frozen interface at all — is a cross-component surface decision the Review gate must weigh, not a mechanical transport swap."
domain: complicated
consumers:
  - "tools/desk/cmd/deskboard: fixed-here (its remaining ghRun call sites)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go: fixed-here (the new read ops on both backends)"
  - "tools/desk/internal/forgeban/allowlist.go: fixed-here (the deskboard permit row removed once its last ghRun caller is gone; ceiling lowered)"
  - "docs/streams/forge-gitlab/inventory.md: fixed-here (the new ops tabulated)"
---

# Brief 12 — deskboard non-board reads onto the seam

## Context

files:
- `tools/desk/cmd/deskboard/scope.go` — the owner-wide PR search (`ownersOf`).
- `tools/desk/cmd/deskboard/health.go` — the commit-history listing (`fetchRecentCommits`) and
  the per-commit check-runs read (`fetchCheckRuns`).
- `tools/desk/cmd/deskboard/stalled.go` — the single-commit read (`fetchHeadCommit`), the PR
  conversation-comments read (`fetchLastAuthorComment`), and the behind-main compare
  (`fetchBehindMain`).
- `tools/desk/cmd/deskboard/zeroci.go` — the combined-status total (`fetchCombinedStatusTotal`),
  the workflow-directory listing (`listWorkflowFiles`), and a workflow file read
  (`fetchWorkflowContent`).
- `tools/desk/cmd/deskboard/board.go` — the remaining REST reads that ride `ghRun`
  (`fetchReviews`, `fetchChangedFiles`, `changedFilesBetween`, `fetchLabelEvents`, the queue's
  issues-by-label walk, `cmdDiff`'s raw PR diff, `cmdFiles`' contents read, the policy-drift
  repo-metadata reads) and the `ghRun` choke point + the `cmd/deskboard/board.go::ghRun::gh`
  permit row itself.
- `tools/desk/cmd/deskboard/prstate.go` — the tombstone PR-state read (`fetchPRState`).
- `tools/desk/internal/deskkit/forge.go` and both backends — the new read ops.
- `docs/streams/forge-gitlab/inventory.md` — every newly enumerated op and its GitLab mapping.

**Why the risk answers are all `no`.** These are reads. No credential, permission or trust
decision changes; the custody binding was settled under the human gate in `forge-neutral/01`,
and the identity these reads authenticate as was settled in `forge-neutral/06` (deskboard's
minted session-role App token via `deskkit.ForgeFor`, never an ambient CLI identity).

single-point-of-failure: the same one `forge-neutral/06` established — the resolver decides
which forge a read hits, and a wrong answer yields a board of the wrong place. The independent
second layer is the three-state contract itself: deskboard already reports a repository its
installation cannot resolve as per-repo could-not-check rather than failing the whole sweep, and
every op this brief adds preserves that (an unsupported-forge read surfaces as could-not-check on
the board, never as a shorter list).

facts:
- After `forge-neutral/06`, `cmd/deskboard` holds EXACTLY ONE `gh` literal — `board.go`'s
  `ghRun` choke point — and exactly one permit row for it, whose `reason:` names the five
  categories below and cites THIS brief as its exit condition. Verify row 7b of 06 pins that
  count at 1.
- Some of these reads DO have an enumerated `Forge` op already and stay on `ghRun` only for
  historical reasons — `fetchReviews` ↔ `ReviewsAtHead`, `fetchChangedFiles` ↔
  `ListChangedFiles`, `changedFilesBetween` ↔ (a compare op this brief adds or `ListChangedFiles`
  reconciled), `fetchLabelEvents` ↔ `ListLabelEvents`, `cmdFiles` ↔ `ReadFile`. Those can move
  to the EXISTING op with no interface addition; the freeze rule is not engaged for them.
- The five categories that have NO enumerated op are the surface decision this brief owns.
- The freeze rule requires any added op to land with its consuming call site in the same change
  (`tools/desk/internal/deskkit/forge.go`), typed inputs and outputs only — NO passthrough.
- Code search has no 1:1 GitLab mapping (GitLab's search API is instance- and scope-shaped
  differently); a workflow-directory listing is GitHub-Actions-specific; a raw combined-status
  total overlaps `ChecksAtHead` but is read here as a bare count. Each such gap is a
  could-not-check-with-gap on GitLab, never an approximation.

**The ten call sites this brief resolves** (the six that need a NEW op are marked ★; the rest
move to an EXISTING op):

| Call site | Read | New op? | GitLab mapping note |
|---|---|---|---|
| `scope.go::ownersOf` | ★ PR search (`search prs --owner --state open`) | yes | GitLab search is scoped per group/project and paginated differently — could-not-check-with-gap where the owner-wide GitHub search has no analog |
| `health.go::fetchRecentCommits` | ★ commit-history listing (`repos/…/commits?per_page`) | yes | `GET /projects/:id/repository/commits` — largely 1:1; the empty-repo signal maps to a 404/empty list |
| `stalled.go::fetchHeadCommit` | ★ single-commit read (`repos/…/commits/{sha}`) | yes | `GET /projects/:id/repository/commits/:sha` — author + committed-date; 1:1 |
| `zeroci.go::fetchCombinedStatusTotal` | ★ combined-status total (`repos/…/commits/{sha}/status`) | yes (or fold into `ChecksAtHead`) | GitLab commit `status` → combined state; the bare TOTAL count is the gap to decide |
| `zeroci.go::listWorkflowFiles` | ★ workflow-directory listing (`contents/.github/workflows`) | yes | GitHub-Actions-specific; on GitLab CI config is a single `.gitlab-ci.yml` — could-not-check-with-gap |
| `zeroci.go::fetchWorkflowContent` | ★ single workflow-file read (`contents/.github/workflows/{f}`) | reuse `ReadFile` | `ReadFile` already serves file-at-ref on both backends |
| `health.go::fetchCheckRuns` | check-runs at a commit | reuse `ChecksAtHead` | already enumerated on both backends |
| `board.go::fetchReviews` | reviews | reuse `ReviewsAtHead` | already enumerated |
| `board.go::fetchChangedFiles` / `changedFilesBetween` | PR files / compare | reuse `ListChangedFiles` (+ reconcile) | already enumerated; the compare form is the residual gap |
| `board.go::cmdQueue` / `cmdDiff` / `fetchLabelEvents` / `prstate.go::fetchPRState` / policy-drift | issues-by-label, raw PR diff, label events, PR state, repo metadata | mixed (some reuse `ListLabelEvents`/`GetPullRequest`/`RepoVisibility`; raw diff + issues-by-label are gaps) | decide per read in Review |

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Every verb's existing test suite stays green; retool a transport shim only where a read moves,
  and never weaken an assertion to make the migration pass.
- Do not add an operation you do not convert a call site for in this same change (freeze rule).
- Removing deskboard's permit row is the SANCTIONED ratchet step here, NOT a security-gate
  removal; any OTHER security/access-control control removal → STOP + BLOCKED-ON-HUMAN.

## Task
1. Enumerate the FIVE read-op categories deskboard needs that the interface does not yet have —
   **PR search**, **commit-history listing**, **single-commit read**, **combined-status total**,
   and **workflow-directory listing** — and for each: decide (in the Review gate) whether it
   belongs on the frozen `Forge` interface as a typed op or whether the consuming read should be
   re-shaped to an EXISTING op; add each chosen op WITH its consuming call site, implement on
   BOTH backends (typed inputs/outputs, no passthrough), and record each in
   `docs/streams/forge-gitlab/inventory.md` with its consumers. Where the GitLab mapping is not
   1:1 (code search, the workflow-directory listing, the bare combined-status total), the op
   returns could-not-check-with-gap naming the gap rather than approximating.
2. Move the reads that ALREADY have an enumerated op (`fetchReviews`, `fetchChangedFiles`,
   `fetchCheckRuns`, `fetchWorkflowContent`, `fetchLabelEvents`, `cmdFiles`, `fetchPRState`,
   the policy-drift metadata reads) onto those existing ops. No interface addition for these.
3. **Preserve the three-state reads.** deskboard's existing per-repo could-not-check behavior
   must survive, and an unsupported-forge read must surface as could-not-check on the board
   rather than as a shorter list.
4. Delete `ghRun` (and its `ownerFromArgs`/token helpers if they become unused) once its last
   caller is gone, remove the `cmd/deskboard/board.go::ghRun::gh` permit row, and lower
   `allowedInvocationCeiling` accordingly (13 → 12, or → the value the surviving rows demand at
   the time this lands, given 04b may also have retired its three rows). Leave every surviving
   permit row intact with its reason.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskboard/... -count=1` | exit 0 |
| 3a | `cd tools/desk && go test -list '.*' ./cmd/deskboard/... \| grep -c '^Test'` | ≥ the baseline measured on this brief's base — the migration retools transports, it does not delete coverage |
| 3b | reviewed diff: every `*_test.go` hunk is a transport-shim retool (gh shim → the forge seam), no assertion weakened, no expected value changed, no negative-path test dropped. Reviewer signs 3b. |
| 4 | `grep -c '"gh"' tools/desk/cmd/deskboard/*.go \| grep -v ':0' \| grep -v '_test.go'` (or `grep -r '"gh"' tools/desk/cmd/deskboard --include='*.go' \| grep -v _test.go \| wc -l`) | prints `0` — NO `gh` literal remains in `cmd/deskboard`; the board reaches every forge through the interface |
| 5 | `cd tools/desk && go test ./internal/forgeban/... -count=1 -v` | exit 0 — the ratchet passes at its new (lower) value, the `cmd/deskboard/board.go::ghRun::gh` row is gone, and every surviving permit row still matches a live call site |
| 6 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows the new value (13 minus deskboard's row, minus any 04b rows already retired) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestNoForgeCLIShellout\|TestForgeNoPassthrough\|TestForgeGitlabCoverage' -count=1 -v` | exit 0 — the surface stays closed, no new passthrough, every new op tabulated in the committed inventory |
| 8 | `cd tools/desk && go test ./internal/deskkit/ -run TestReadOpsBothBackends -count=1 -v` | exit 0 — each newly enumerated read op runs the same scenario name against both backends (GitHub returns the typed result; GitLab returns could-not-check-with-gap where not 1:1) |
| 9 | `cd tools/desk && go test ./cmd/deskboard/... -run 'TestOutOfInstallationRepoIsCouldNotCheck\|TestOtherReadErrorsStillFailTheRunClosed' -count=1 -v` | **negative path**: a repo the resolved forge cannot read is an explicit could-not-check row, not an omitted or empty one; a non-out-of-installation error still fails the run closed |
| 10 | `statusgen --root . --consumers --brief forge-neutral/12` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A migrated read swallows a failure and returns an empty result, so a downstream sweep reads an idle board | rows 9 + the three-state assertions the existing deskboard tests already carry |
| A read op is added speculatively with no call site, breaking the freeze rule | row 7 (`TestForgeNoPassthrough` + inventory) plus the inventory entry naming consumers |
| The old shell helper stays reachable behind a variable | rows 4 + 5 (two instruments: a literal grep and the ban's structural scan) |
| The ratchet is lowered by DELETING the row rather than by migrating its callers | row 5, whose ratchet test fails when a permit row matches no live call site |
| A GitLab mapping is approximated because the semantics nearly match | row 8 runs identical scenario names on both backends; a divergence is a named failing scenario |
| A read that HAS an enumerated op grows a NEW redundant op instead of reusing it | Review gate reads task 2's list; a new op whose read an existing op already serves is a finding |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter; all four risk answers are `no`). The reviewer records verdict
+ date in the stream README table, confirms every surviving permit row still carries its reason,
and — the load-bearing judgement for THIS brief — rules on WHICH of the five categories earns a
place on the frozen interface versus a re-shape onto an existing op, and whether each new op's
GitLab could-not-check-with-gap is honest (a genuine non-1:1) rather than an unimplemented 1:1.
