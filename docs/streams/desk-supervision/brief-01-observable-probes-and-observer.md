---
brief: desk-supervision/01
title: Observable probes + the `desksupervise` observer — liveness that bites
why: >-
  The engine's liveness taxonomy (schedule-to-start, heartbeat gap, per-tier wall cap)
  is fully coded and completely inert: no consumer supplies an ObservableProbe, so a
  worker that wedges holds its claim until the 120-minute stale-claim backstop, and a
  human notices first. Wedged reviewers and verifiers sitting 30–40 minutes with a frozen
  transcript are a recorded failure class. Supplying the probes and one read-mostly
  observer loop turns "wedged" into a logged, minutes-scale reclaim with no human in it.
wave: 0
depends: []
unblocks: ["desk-supervision/02", "desk-supervision/03", "desk-supervision/04", "desk-supervision/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §8.5 (active-run reconciliation: stall detection per tick) and §8.4 (retry + backoff) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/desk/internal/loopengine/liveness.go — LivenessPolicy, DefaultLivenessPolicy (10m / 20–90m / 20m), the three-state ObservableProbe contract, conservative reclaim through deskkit.ReleaseMatching"
  - "tools/desk/internal/loopengine/engine.go ~line 220 — `Observe *ObservableProbes` is consumer-supplied; 'with Observe nil, liveness is therefore inert'"
  - "measured 2026-09-02: `grep -rln ObservableProbe tools/desk/cmd` returns zero files — no consumer supplies probes; fanoutloop and verifyloop expose only `plan`"
  - "freshness-checked 2026-09-02 @ 30c9934 — no observer verb exists under tools/desk/cmd"
exec-tier: strong
exec-tier-why: >-
  (b) and (c): correctness is cross-artifact (audit stream, forge refs, PR state read
  against claim records) and a probe that reads could-not-check as no-life double-dispatches
  live work — the exact failure the conservative-reclaim rules exist to prevent.
consumers:
  - "tools/desk/internal/loopengine/engine.go Config.Observe: fixed-here (the probes this brief ships are the value a future driver cutover plugs into Config.Observe unchanged — no engine edit)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Sources of work row 9 (queue suppressors): follow-up desk-supervision/07 (the observer's snapshot becomes the instrument that reads expired claims; the skill row is rewritten when the snapshot verb lands)"
---

# Brief 01 — Observable probes + the `desksupervise` observer

## Context

files:
- `tools/desk/internal/loopengine/probes.go` (new) — the three house probes, each
  implementing the existing `ObservableProbe` contract in `liveness.go`.
- `tools/desk/internal/loopengine/probes_test.go` (new) — three-state tests per probe.
- `tools/desk/cmd/desksupervise/main.go` (new) — the observer verb: `tick`, `run
  --interval`, `--dry-run`, and the fixture flags the Verify rows use.
- `tools/desk/cmd/desksupervise/testdata/` (new) — claim + observation fixtures.
- `tools/desk/README.md` — add the `desksupervise` row to the tool reference table.

single-point-of-failure: the claim record (`refs/dispatch/*`, tagger date stamped by the
forge) is the one thing every decision hangs off — the layers behind it are the three
independent probes (audit stream, branch SHA movement, PR activity), any one of which alone
proves life, and the lock-guarded single-winner release that makes a wrong reclaim a
re-claim race rather than a second dispatch.

facts:
- **The policy exists; the probes do not** (measured 2026-09-02 @ `30c9934`).
  `liveness.go` defines `ObservableProbe` as three-state: `(obs, nil)` checked with `obs.At`
  the latest sign of life or zero for none; `(_, err)` could-not-check, NEVER read as no
  life. `DefaultLivenessPolicy()` is ScheduleToStart 10m, StartToClose local 20m / cheap
  90m / session 90m, HeartbeatGap 20m. `engine.go` `Config.Observe` is nil in every
  consumer, so the whole taxonomy is inert today.
- **Three observable sources, all already produced by workers** (`liveness.go` header):
  audit lines attributable to the dispatch (`~/.config/assay/audit.jsonl`, deskkit
  `Entry` fields `ts`, `tool`, `verb`, `repo`, `pr`, `headSHA`, `sessionTag`); the claim's
  recorded branch gaining commits (`git ls-remote` SHA movement); PR creation/updates on
  the recorded PR.
- **Claims are forge refs with two states**: `state=claimed` (TTL 20m) and
  `state=dispatched` (TTL 120m), enumerated with `git ls-remote origin 'refs/dispatch/*'`;
  the message carries owner, state and branch. The observer reads them; it never steals.
- **Reclaim is release-only.** Expiry frees the claim through `deskkit.ReleaseMatching`
  (compare-and-delete under the claims lock); re-dispatch still races through the ordinary
  claim path. Wall-cap expiry does NOT free the claim — it lands the item blocked-timeout
  (file-and-continue), per `liveness.go`.
- **Loop-wide kill switch is separate.** `killswitch.go`'s `HEARTBEAT` dead-man lease
  (24h, stop-all) is about the desk-tools installation, not a run; do not reuse it.
- **Roster beacons are the wrong signal** for a run: per-session, self-declared at
  `deskroster set`, fresh for 60 minutes. A wedged worker under a live desk reads alive.
- Verb contract: deskkit kill switch first, one audit line per invocation, exit 0 · 3 ·
  5 · 6, `deskkit.SetToolClass(deskkit.ClassForTool(false))`, `EchoEffectiveConfig`.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Probes** (`probes.go`). Implement `AuditProbe`, `BranchProbe`, `PRProbe`, each
   satisfying the `ObservableProbe` signature in `liveness.go` and each three-state:
   - `AuditProbe`: newest audit line since `since` whose `sessionTag` / `repo`+`pr` /
     `headSHA` attributes it to the dispatch. Unreadable file ⇒ error (could-not-check).
   - `BranchProbe`: `git ls-remote` on the claim's recorded branch; SHA changed since the
     last tick ⇒ observation now. Network failure ⇒ error.
   - `PRProbe`: the recorded PR's `updatedAt` via the forge read path in deskkit. 404 on a
     PR that was recorded ⇒ error, never no-life.
   Export `HouseProbes() *ObservableProbes` composing the three, so a future driver sets
   `Config.Observe = HouseProbes()` and nothing else changes.
2. **Observer verb** (`desksupervise`).
   - `desksupervise tick [--root DIR] [--repo OWNER/NAME] [--dry-run] [--now RFC3339]
     [--claims-fixture FILE] [--observations-fixture FILE]`: enumerate `state=dispatched`
     claims, run the probes, evaluate `DefaultLivenessPolicy()` (or the policy in
     `<StateDir>/liveness.json` when present), print one line per claim:
     `<key>  <ALIVE|NEVER-STARTED|HEARTBEAT-EXPIRED|OVER-WALL-CAP|COULD-NOT-CHECK>
     last=<ts|none> via=<source|->  action=<none|RECLAIM-ELIGIBLE|BLOCKED-TIMEOUT|BLIND>`.
   - Actions, when not `--dry-run`: `RECLAIM-ELIGIBLE` ⇒ `deskkit.ReleaseMatching` +
     journal event; `BLOCKED-TIMEOUT` ⇒ journal event + `deskfile` a `help wanted` issue
     naming the run (once per key, idempotent by marker); `BLIND` ⇒ no action, exit 6 for
     the tick, the line names the unreachable source.
   - `desksupervise run --interval 5m`: loop forever honouring the kill switch between
     ticks, exit 0 on SIGTERM (mirror `deskwt prune --interval`).
   - Fixture flags bypass the forge and audit file so the Verify rows run offline.
3. **Tests**: per-probe three-state tests; observer classification table test; the
   negative control that a could-not-check on one probe with a clean sign of life on
   another yields `ALIVE`; the negative control that all-probes-could-not-check yields
   `BLIND` and releases nothing.
4. **README row** in `tools/desk/README.md` (read-mostly; writes: claim release, journal,
   one filed issue).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/loopengine/ -run 'Probe' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise --help` | exit 0; output contains `tick` and `run --interval` |
| 3 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/dead-worker.json --observations-fixture cmd/desksupervise/testdata/dead-worker-obs.json` | exit 0; output contains `HEARTBEAT-EXPIRED` and `action=RECLAIM-ELIGIBLE` |
| 4 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/alive-worker.json --observations-fixture cmd/desksupervise/testdata/alive-worker-obs.json` | exit 0; output contains `ALIVE` and `action=none`; output does not contain `RECLAIM` |
| 5 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/dead-worker.json --observations-fixture cmd/desksupervise/testdata/blind-obs.json; echo rc=$?` | output contains `COULD-NOT-CHECK` and `action=BLIND` and `rc=6`; output does not contain `RECLAIM` |
| 6 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/never-started.json --observations-fixture cmd/desksupervise/testdata/none-obs.json` | exit 0; output contains `NEVER-STARTED` |
| 7 | `cd tools/desk && ./desksupervise tick --dry-run --now 2026-09-02T14:00:00Z --claims-fixture cmd/desksupervise/testdata/long-runner.json --observations-fixture cmd/desksupervise/testdata/long-runner-obs.json` | exit 0; output contains `OVER-WALL-CAP` and `action=BLOCKED-TIMEOUT`; output does not contain `RECLAIM` |
| 8 | `grep -rln ObservableProbe tools/desk/cmd tools/desk/internal/loopengine/probes.go \| wc -l` | output is `2` or more (positive control for the measured zero) |
| 9 | `grep -c 'desksupervise' tools/desk/README.md` | output is `1` or more |
| 10 | `statusgen --root . --consumers --brief desk-supervision/01` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "reclaims a live worker whose only activity was git pushes" → row 4
(branch observation alone reads ALIVE); "treats an unreachable forge as a dead worker" →
row 5; "frees a claim on wall-cap expiry and re-dispatches the same budget-blower" → row 7;
"probe code lands but no verb reads it" → rows 2, 8. Review-only: whether the shipped
default policy numbers suit this house — a knob, not a defect.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Exit | Output | Date | Runner |
|---|------|--------|------|--------|
| 1 | 0 | `ok  	github.com/medici-finance/assay/tools/desk/internal/loopengine	0.223s` | 2026-09-02 | implementing worker |
| 2 | 0 | `USAGE:` / `desksupervise tick [...]` / `desksupervise run --interval DUR [...]` (both `tick` and `run --interval` present) | 2026-09-02 | implementing worker |
| 3 | 0 | `example-stream--dead-01  HEARTBEAT-EXPIRED last=2026-09-02T11:00:00Z via=branch sha moved action=RECLAIM-ELIGIBLE` | 2026-09-02 | implementing worker |
| 4 | 0 | `example-stream--alive-01  ALIVE last=2026-09-02T11:58:00Z via=audit line action=none` (no `RECLAIM` substring) | 2026-09-02 | implementing worker |
| 5 | 6 | `example-stream--dead-01  COULD-NOT-CHECK last=none via=- action=BLIND` then `1 of 1 claim(s) were COULD-NOT-CHECK (action=BLIND) — the tick ran, the reading is incomplete`; `rc=6` (no `RECLAIM` substring) | 2026-09-02 | implementing worker |
| 6 | 0 | `example-stream--never-01  NEVER-STARTED last=none via=- action=RECLAIM-ELIGIBLE` | 2026-09-02 | implementing worker |
| 7 | 0 | `example-stream--long-01  OVER-WALL-CAP last=2026-09-02T12:00:00Z via=branch sha moved action=BLOCKED-TIMEOUT` (no `RECLAIM` substring) | 2026-09-02 | implementing worker |
| 8 | 0 | `5` (`tools/desk/internal/loopengine/probes.go` plus four files under `tools/desk/cmd/desksupervise/` — `main.go`, `run.go`, `live.go`, `fixtures.go` — match `ObservableProbe`) | 2026-09-02 | implementing worker |
| 9 | 0 | `1` (`tools/desk/README.md`'s new `desksupervise` tool-reference row) | 2026-09-02 | implementing worker |
| 10 | 0 | `consumers corroboration — ... 1 brief(s)` / `desk-supervision/01` / both `consumers:` claims read `UNCHECKED` (each "unchanged since the merge-base", correct: `engine.go` is deliberately untouched — "no engine edit" — and the SKILL.md row is an explicit `desk-supervision/07` follow-up) / `summary: 0 corroborated, 0 disproved, 2 unchecked` — no `DISPROVED` anywhere | 2026-09-02 | implementing worker |

Also run (not a Verify row, full-suite regression check): `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1` — every package `ok` (2026-09-02); `tools/desk/internal/deskkit`'s `TestNoForgeCLIShellout` required registering `tools/desk/cmd/desksupervise/live.go`'s runtime-resolved exec of the consumer repo's own dispatch-claim script in `tools/desk/internal/forgeban/allowlist.go`'s `UnresolvedArgv` ledger (same shape as the existing deskboard/deskpreflight runtime-resolved-binary entries) — fixed in a follow-up commit on this branch.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
