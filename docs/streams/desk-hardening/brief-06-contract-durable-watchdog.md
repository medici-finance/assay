---
brief: desk-hardening/06
title: Durable desk watchdog — contract & interface
kind: contract
schema: contract-v1
---

# Durable Desk Watchdog — Contract & Interface

This document is the **contract** for the durable-liveness half of brief
desk-hardening/06. It specifies *what* the watchdog must do, the interface
between the desks and the watchdog, and what makes a dead monitor LOUD
(detectable) rather than silently absent. The **service build** belongs to the
observability stream (oit #627/#651) — this contract pairs with that build; it
does NOT reimplement the watchdog as a local bash loop.

## 1. Why a contract

The #79 incident (2026-07-17) was a dead monitor producing a silent all-clear:
the pr-review-desk background bash loop went blind under macOS App Nap and
the desk reported "nothing in flight" while 19 actionable PRs piled up. Root
cause: a monitor that disappears without a trace is indistinguishable from a
monitor that sees nothing. A durable watchdog MUST fail LOUD — its absence is
a detectable event, not an absence of events.

## 2. Where the watchdog runs

- **Home:** the always-on committed observability service in
  `oit` under `docs/streams/observability/`
  (oit #627, #651 — the watchdog-exporter + Pushgateway pattern, or an
  equivalent always-on receiver).
- **NOT:** a local bash loop (`& disown`), a laptop cron job, or any
  construct whose liveness depends on a single operator machine state.
  Those are the exact failure mode #86 diagnosed; the "retired stopgap"
  in pr-review-desk boot sequence is the proof that class is known-broken.

## 3. Contract requirements

### 3.1 Heartbeat — a dead monitor MUST be detectable

The watchdog emits a **heartbeat** on a fixed interval of **5 minutes**.
The heartbeat is written to a durable, queryable store: Pushgateway (the
oit #627/#651 pattern). The heartbeat payload carries sufficient context
that a desk reading it can judge not only liveness but sweep quality.

**Heartbeat payload schema:**

| Field | Type | Description |
|-------|------|-------------|
| `watchdog_id` | string | Identity of this watchdog instance |
| `heartbeat_ts` | RFC3339 | When this heartbeat was written |
| `last_sweep_ts` | RFC3339 | Timestamp of the most recent completed sweep |
| `last_sweep_state` | enum | `checked-clean`, `checked-failed`, or `could-not-check` |
| `repos_covered` | int | Number of repos the last sweep examined (must equal `len(AllowedRepos())`) |
| `actionable_count` | int | Number of actionable items found (NEEDS-REVIEW + RE-REVIEW); zero for idle |
| `exit_code` | int | The deskboard-equivalent exit code for the last sweep (see §3.3) |

**Detection rule (three-state, per desk-hardening/01):**

- **fresh:** `now - heartbeat_ts <= 10 min` (2x the 5-min interval) AND the
  heartbeat payload is readable and valid. The watchdog is alive.
- **stale (DEAD):** `now - heartbeat_ts > 10 min` — the watchdog has stopped
  emitting. The desk is blind, not idle, and MUST re-sweep the board itself
  before making any claim about the queue.
- **could-not-check:** the heartbeat store is unreachable, the payload is
  malformed, or `heartbeat_ts` is missing/illegible. The desk could not
  determine whether the watchdog is alive — treat as blind, not idle.

An idle claim requires ALL of:
- fresh heartbeat (§3.1 detection rule)
- `last_sweep_state == checked-clean` (sweep completed successfully)
- `repos_covered == len(AllowedRepos())` (full coverage)
- `actionable_count == 0` (no work found)

Anything less means the desk is either blind (`could-not-check`, stale) or
busy (`checked-failed`) — never idle. A desk that cannot satisfy all four
conditions must re-sweep the board itself before making any claim about the queue.

### 3.2 Full board coverage — single source, not a prose list

The watchdog MUST cover the **full board-bearing repo set** — every repo
`deskboard.go` reads. Coverage drift between the watchdog and the board is a
defect; the contract requires them to stay in sync.

**Single source:** the compiled-in `allowedRepos` table in
tools/desk/internal/deskkit/config.go (function `AllowedRepos()`). F-30 was
exactly a parallel list going stale — adding a prose list here would be a
third parallel list and the same defect. The watchdog MUST read the repo set
programmatically from `AllowedRepos()` or from a build-time derivation of that
table. A check MUST assert the watchdog set equals the compiled-in set;
a mismatch is `could-not-check`.

The compiled-in set (9 repos at time of writing; the table in `config.go` is
authoritative, not this count):

- `oit`
- `example-org/agent-runtime`
- `example-org/medici-examples`
- `medici-finance/assay-toolkit`
- `example-org/canton-k8s`
- `example-org/reconciler`
- `example-org/org-slides`
- `example-org/proposals`
- any private review channel configured by the operator

A private review channel carries findings that must not surface publicly — a
watchdog blind to it misses that queue entirely. This is precisely the "latent
blind spot" this section defines as a defect. The channel is deliberately not
named in this repo, so the watchdog reads its identity from operator
configuration rather than from a compiled-in or documented slug.

### 3.3 Three-state reporting + exit-code mapping

Every sweep reports **checked-clean / checked-failed / could-not-check** —
never two-state clean/dirty (desk-hardening/01). A sweep that could not reach
a repo is `could-not-check`, not `clean`. A partial network partition must
not read as an all-clear.

The sweep maps to the fleet compiled-in exit-code contract
(tools/desk/internal/deskkit/exitcodes.go):

| Sweep state | Exit code | Meaning |
|-------------|-----------|---------|
| `checked-clean` | 0, `actionable_count == 0` | All repos swept successfully, no work found |
| `checked-failed` | 0, `actionable_count > 0` | Sweep completed but found actionable items — NOT idle, but NOT blind |
| `could-not-check` | non-zero (3/4/5/6) | Sweep did not complete — kill switch (3), rate limit (4), refusal (5), or unverifiable error (6) |

Exit 0 means "the sweep succeeded" — it says nothing about whether the queue
is empty. Actionable count is an output field, not an exit code; `deskboard`
exits 0 on a full board with 19 actionable PRs. `checked-clean` therefore
requires BOTH exit 0 AND `actionable_count == 0`. Exits 3/4/5 mean the sweep
never looked (the merged `docs/three-state-instrument-rule.md` defines
`could-not-check` for exactly that class). Exit 4 is a rate-limit refusal;
`deskboard` is GET-only end to end and will not hit the outward-write budget,
but the watchdog service may carry its own rate limiter — any non-zero exit
is `could-not-check`.

The desk-side reader of the heartbeat MUST be a deskkit-conformant command
that maps these exit codes, so `could-not-check` has a machine representation
rather than living only in prose. `ExitCodeOf` (failing closed to 6 on any
unrecognised error) is the reference implementation.

The heartbeat read itself (§3.1) is three-state — `could-not-check`
covers both "heartbeat store unreachable" and "payload malformed."
A heartbeat read that returns `could-not-check` is treated the same as a
stale heartbeat: the desk is blind, not idle.

### 3.4 Positive control (mutation-test)

Per PR #255 (three-state-instrument-rule), a brief adding a liveness check
MUST prove the instrument goes RED when the guarded thing is broken.
For this contract: freeze or rewind the heartbeat past 10 min and confirm
the desk REFUSES the idle claim. Without it, §3.1 detection rule is an
untested green lamp.

## 4. Desk-side interface

Every desk that reports on the queue uses the watchdog heartbeat as a
**liveness precondition:**

### 4.1 Idle precondition — heartbeat freshness + payload + backstop armed

Before any idle claim, the desk:

1. **Reads the heartbeat.** A stale or `could-not-check` heartbeat means the
   watchdog is dead or unreachable — the desk is **blind, not idle**, and must
   re-sweep the board itself before saying anything about the queue.
2. **Inspects the heartbeat payload.** A fresh heartbeat whose
   `last_sweep_state != checked-clean` or `repos_covered < len(AllowedRepos())`
   or `actionable_count > 0` means the watchdog is alive but either its
   sweep pipeline is failing (`could-not-check`) or the queue has work
   (`checked-failed`) — never idle.
3. **Confirms its own fixed-cadence backstop Monitor is alive.** See §4.2.
   A desk whose backstop has died is blind regardless of the watchdog state.

All three conditions must hold for an idle claim. The desk must be able to
answer "what am I waiting on and why" at any moment.

### 4.2 Desk-side backstop — cadenced Monitor with `persistent: true`

The desk-side fallback is a fixed-cadence `Monitor` sweep (the pr-review-desk
boot step 4 pattern, already live): an independent timer that runs
`deskboard.go` on a fixed interval (~5 min) regardless of whether the
event-monitor fired. This is the liveness backstop — a dead event-monitor can
never produce a silent all-clear because a fresh sweep still lands on its own
cadence.

The backstop MUST be armed with **`persistent: true`** — the attribute that
survives across turns and re-invokes the desk reliably. Without it, the
backstop dies when the session ends or the desk never arms step 4, which is
the same failure mode §2 rejects in the `& disown` loop.

The desk confirms its backstop is alive as part of the idle precondition
(§4.1 item 3). The SKILL boot step 3 `TaskList` (already used for the
never-arm-twice check) is the instrument. A desk whose backstop Monitor has
died is blind.

The staleness signal already exists: `deskboard.go` prints `swept <ISO8601>`
on every run, and the pr-review-desk SKILL already calls it "the liveness
heartbeat … treat that as blind, not idle." The backstop reuses this signal —
it does not mint a second, competing freshness signal.

### 4.3 Heartbeat and board sweep age are independent signals

The heartbeat (§3.1) is *"is the watchdog alive?"* — the board sweep age
(`swept <ISO8601>` from `deskboard.go`) is *"how stale is the last sweep?"*.
Both must be fresh for an idle claim. A live watchdog with a stale board sweep
means the sweep pipeline is clogged — not idle.

## 5. What this contract does NOT build

This contract specifies the **interface**. The **service build** — the
watchdog-exporter, the Pushgateway/metrics endpoint, the always-on receiver —
lives in the observability stream (oit #627/#651). That stream owns the
implementation; this contract owns the requirement. The two are paired: this
file is referenced from the observability briefs as the desk-side spec they
must satisfy.

### 5.1 Relationship to existing liveness beacons

`tools/desk/cmd/deskroster` already stores per-session beacons at
`~/.claude/desk-tools/roster/<session>.json` with an RFC3339 `updated` field
and an `open_work` list (`roster.go`). This is a liveness beacon of the same
shape as §3.1 heartbeat, but it is **machine-local** — it lives in the
operator home directory and is invisible to another desk. It is NOT a
substitute for the always-on watchdog service: the watchdog must be readable
from any session, not only from the machine that wrote the heartbeat.
The deskroster beacon may be reused, extended, or explicitly rejected by the
observability build, but this contract does not duplicate its schema.

## 6. Verification

These are presence gates — the service build is external; the contract
verifies the spec is complete. Per `docs/brief-rules.md` rule 7, each row
names a command or states the presence-gate caveat. All greps exclude §6
(the verify table itself) so a row cannot satisfy itself on its own text;
commands use `-E` (ERE) for alternation.

| # | Check | Command / gate |
|---|-------|----------------|
| 1 | Three-state heartbeat detection (fresh / stale / could-not-check) specified | `grep -cE 'could-not-check\|three-state\|fresh.*stale' <(sed '/^## 6\. Verification/,$ d' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md)` -> >= 3 |
| 2 | Contract repo list matches compiled-in `AllowedRepos()` count | `[ $(grep -cE $'^- \x60[a-z0-9-]+/[a-z0-9-]+\x60$' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md) -eq $(grep -cE '"[a-z0-9-]+/[a-z0-9-]+"' tools/desk/internal/deskkit/config.go) ]` -> exit 0 |
| 3 | Payload schema fields verified by anchored per-field check on schema table | `test $(sed -n '/^\| Field.*Type.*Description/,/^$/p' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md \| grep -cE 'last_sweep_ts\|last_sweep_state\|repos_covered\|actionable_count\|exit_code') -ge 5 \&\& echo PASS` -> exit 0 |
| 4 | Idle precondition gates on payload + `actionable_count` + backstop armed | `grep -cE 'heartbeat payload\|backstop.*armed\|backstop.*alive\|actionable_count' <(sed '/^## 6\. Verification/,$ d' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md)` -> >= 3 |
| 5 | Backstop `persistent: true` + reuses `swept <ISO8601>` staleness signal | `grep -cE 'persistent.*true\|swept.*ISO8601' <(sed '/^## 6\. Verification/,$ d' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md)` -> >= 2 |
| 6 | Exit codes mapped to deskkit contract (0+actionable_count / non-zero / ExitUnverifiable) | `grep -cE 'ExitUnverifiable\|exitcodes.go\|exit.code\|exit 0.*actionable' <(sed '/^## 6\. Verification/,$ d' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md)` -> >= 3 |
| 7 | Always-on committed service, NOT local `& disown` loop | `grep -cE 'always-on\|disown\|oit.*627\|oit.*651' <(sed '/^## 6\. Verification/,$ d' docs/streams/desk-hardening/brief-06-contract-durable-watchdog.md)` -> >= 3 |
| M | Mutation test: prove desk REFUSES idle claim when heartbeat > 10 min stale | Presence gate — the observability build must demonstrate this; the contract §3.4 requires it |
