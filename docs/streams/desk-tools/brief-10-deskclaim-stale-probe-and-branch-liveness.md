---
brief: desk-tools/10
title: "`deskclaim stale` + branch-liveness on `acquire` — reclaim a dead session's claim through the tool, not by hand"
why: >-
  A 24-hour sweep of fifteen desk-role and worker session transcripts found one sweep hand-
  stealing a claim five times — "age 1206m, TTL 120m — stealing it" — by deleting the claim
  file and re-creating it. The library under `deskclaim` already holds fail-closed stale logic
  (never reclaim an unreadable claim; cannot-prove-inactive means do not steal). The reason the
  operator still stole by hand is in the CLI: it passes no branch-liveness probe, so every claim
  recorded with `--branch` is un-reclaimable through the tool at ANY age, and the only exit is
  the hand-delete that bypasses every one of those guards — including the flock that closes
  double-dispatch. This brief gives the CLI the probe, a read-only `stale` verb that states the
  verdict with its age and TTL, and an audit line that says "reclaimed" when it did.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/deskclaim/claim.go` § cmdAcquire builds `ClaimConfig{ClaimsDir, StaleClaim}` with NO `BranchActive`; `tools/desk/internal/deskkit/claim.go` § isStale returns not-stale whenever a branch is recorded and `BranchActive == nil` (\"cannot prove inactive => conservative, do not steal\"); verbs are `acquire`, `release`, `list` only — no probe verb; the acquire audit line is identical for a fresh acquire and a reclaim."
  - "The engine that DOES wire the probe, and the comment warning that Branch alone makes claims un-reclaimable: `tools/desk/internal/loopengine/engine.go` (Config.BranchActive) and `tools/desk/internal/loopengine/claim.go`."
  - "The liveness evidence `deskwt` already reads for the same question: `tools/desk/cmd/deskwt/lockreclaim.go` § readSessionBeacon / judgeLock (a session's roster beacon), and `branchcollision.go` § branchHolders (a branch checked out in a registered worktree)."
  - "The library API this stays on top of: `deskkit.Acquire`, `deskkit.IsStale`, `deskkit.ReleaseMatching` (`claim.go`), which already serialise the whole create-or-reclaim decision under one flock."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
exec-tier: strong
exec-tier-why: "(c) concurrency and safety plumbing — a liveness probe that answers \"inactive\"
  for a branch that is merely not checked out HERE steals a live claim on another machine, and
  every single-machine test passes."
---

# Brief 10 — `deskclaim stale` + branch-liveness on `acquire`

## Dependencies
None. The stale logic, the flock, and both liveness signals exist on main; this brief wires
them into the CLI.

## Context

single-point-of-failure: **the liveness probe's verdict** — a probe that says "inactive" is what
licenses the reclaim. Behind it stand two independent layers, both already present: the **age
floor** (no claim younger than its TTL is examined at all, whatever the probe says) and the
**in-place flock'd rewrite** in `deskkit.Acquire` (two reclaimers cannot both win; an
unreadable claim is never stolen). Independence: the probe fails on a wrong liveness signal,
the age floor on a wrong clock, the flock on a filesystem without locking — different reasons,
different components. This brief adds the probe and must not touch the other two.

files:
- `tools/desk/cmd/deskclaim/claim.go` (`cmdAcquire` wires `BranchActive`; new `cmdStale`)
- `tools/desk/cmd/deskclaim/liveness.go` (planned) — the probe, composed from the two signals
- `tools/desk/cmd/deskclaim/main.go` (usage, `stale` dispatch)
- `tools/desk/cmd/deskclaim/*_test.go`
- `tools/desk/README.md` (contract)

facts:
- claims are machine-local JSON files under the config home's `claims/` dir (`deskclaim`
  header); kinds `route|file|close|verify` gate a single session's own actions; the `dispatch`
  kind's cross-machine form lives on the forge as `refs/dispatch/*` and is OUT OF SCOPE here
  (its reclaim is a forge-side decision with no TTL in this library).
- TTL: `deskkit.DefaultStaleClaim` = 120 minutes; the CLI has no `--ttl` and this brief adds
  none — a caller-chosen TTL is how a live claim gets stolen by a caller in a hurry.
- `isStale` (`tools/desk/internal/deskkit/claim.go`): age ≤ TTL → not stale; unreadable → Unverifiable; branch
  recorded and `BranchActive == nil` → not stale; branch recorded and `BranchActive(branch)` →
  not stale; else stale. The CLI today always lands on the third arm for any `--branch` claim.
- the probe (`liveness.go`), **fail-closed composition — a branch is INACTIVE only when every
  signal it can read says so, and unreadable means ACTIVE**:
  1. the branch is checked out in a registered worktree of `--repo` (`git worktree list
     --porcelain`, the `branchHolders` reading) → ACTIVE;
  2. the claim's `owner` session has a live roster beacon (the `readSessionBeacon` reading from
     `tools/desk/cmd/deskwt/lockreclaim.go`, same beacon dir) → ACTIVE;
  3. `--repo` absent or not a git repo, or the beacon dir unreadable → ACTIVE (cannot prove).
  It never contacts the forge: a branch that exists only on the remote is invisible here, and
  that is accepted because a claim recorded with a branch is protected by the beacon signal too.
- `deskclaim stale --item I [--repo R]`: **read-only**; prints one line
  `item=<I> age=<m>m ttl=<m>m branch=<B or ->  holder=<owner> verdict=<stale|live|unreadable>
  because=<age-under-ttl|branch-checked-out:<path>|beacon-live|no-repo-cannot-prove|old-no-live-signal>`;
  exit **0 stale**, **5 live**, **6 unreadable / missing** (a missing claim is not stale — there
  is nothing to reclaim — and is reported as such). It calls `deskkit.IsStale` with the probe
  wired; it never acquires, releases or rewrites.
- `acquire` audit line distinguishes the two successes: `fresh` vs `reclaimed age=<m>m
  ttl=<m>m prior-owner=<o> because=<…>`; stdout says `reclaimed` when it did. `deskkit.Acquire`
  returns only `acquired bool` today — extend it with a reclaim report (a small struct or a
  second return) WITHOUT changing the create-or-reclaim decision or its locking.
- tests: temp claims dir + temp git repo with a worktree; beacon dir pointed at a temp dir;
  clock via file mtimes (`os.Chtimes`), never sleeps.

## Ground rules
- Never weaken `isStale`'s three fail-closed arms; the probe only supplies the signal the
  fourth arm needs.
- No forge contact from any test or Verify row.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **`liveness.go`**: `branchActiveProbe(repo, beaconDir, owner string) func(branch string)
   bool` composed per the facts, fail-closed; a `because` string alongside for the audit line.
2. **`acquire`**: accept `--repo <path>` (default: the current directory if it is a git repo,
   else no repo → probe answers ACTIVE); wire the probe into `ClaimConfig.BranchActive`;
   extend `deskkit.Acquire` to report fresh-vs-reclaimed with the age; audit + stdout per the
   facts.
3. **`stale` verb**: the read-only probe with the 0/5/6 contract and the one-line report.
4. **Tests**: `--branch` claim older than TTL with the branch checked out → `stale` exits 5,
   `acquire` refuses; same with the worktree removed and no beacon → `stale` exits 0,
   `acquire` reclaims and the audit line says `reclaimed`; younger than TTL → live regardless
   of signals; unreadable claim file (mode 000, skipped as root) → exit 6 from both verbs;
   missing claim → `stale` exits 6 with "no claim"; `--repo` pointing at a non-repo → live;
   a beacon for the owner session present → live even with no worktree; and the NEGATIVE
   control that a concurrent reclaim still yields exactly one winner (the existing
   `Acquire` race test, re-run with the probe wired).
5. **README**: the verdict table, the `because` vocabulary, and the sentence that a hand-delete
   of a claim file bypasses the flock and is never the remedy.
6. **Nothing else.** No `--ttl`, no `--force`, no forge-side reclaim.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskclaim/ -run '^TestStaleVerdictOldBranchCheckedOut$' -count=1` | exit 0 — age > TTL, branch held by a worktree: `stale` exits 5 with `because=branch-checked-out:`; `acquire` refuses |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskclaim/ -run '^TestAcquireReclaimsOldUnheldBranchClaim$' -count=1` | exit 0 — worktree gone, no beacon: `stale` exits 0; `acquire` succeeds and its audit line carries `reclaimed age=` and `prior-owner=` |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskclaim/ -run '^TestYoungClaimIsLiveWhateverTheSignals$' -count=1` | exit 0 — the age floor holds with every liveness signal absent |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskclaim/ -run '^TestProbeFailsClosedWithoutRepoOrBeaconDir$' -count=1` | exit 0 — no `--repo`, or an unreadable beacon dir, answers ACTIVE (exit 5), never stale |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskclaim/ -run '^TestBeaconKeepsClaimLive$' -count=1` | exit 0 — a live beacon for the owner session is sufficient on its own |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskclaim/ -run '^TestStaleMissingAndUnreadableAreSix$' -count=1` | exit 0 — both exit 6; neither is reported stale |
| 8 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestAcquire' -count=1` | exit 0 — the library's existing acquire/race/fail-closed tests are unchanged and green with the extended return |
| 9 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 |
| 10 | check:ci | `gofmt -l tools/desk/cmd/deskclaim tools/desk/internal/deskkit > /tmp/dc-fmt.out; test ! -s /tmp/dc-fmt.out` | exit 0 |
| 11 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Probe answers "inactive" when it merely could not look | row 5 |
| Probe ignores the beacon and steals a claim whose worktree is on another machine | row 6 |
| Age floor bypassed by a probe that runs first | row 4 |
| `stale` verb mutates (rewrites mtime, acquires) | row 7 + a row-2 assertion that the claim file's bytes and mtime are unchanged after `stale` |
| Extending `Acquire`'s return breaks the race guarantee | row 8 |
| Audit line says `fresh` for a reclaim | row 3 |
| Forge-side `refs/dispatch/*` claims assumed covered | review-only — the README states the scope boundary in one sentence |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer records verdict + date in the stream README
table and answers: (1) the single control is the probe's verdict; rows 4 and 8 prove the age
floor and the flock catch a wrong verdict independently; (2) row 5 is the negative-path row —
the probe with its inputs removed must answer LIVE, and a green there is what makes a green on
row 3 meaningful.
