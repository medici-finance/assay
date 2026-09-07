---
brief: desk-supervision/02
title: Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal
why: >-
  Nothing can stop ONE run today. The kill switch knows DISABLED, STOP and STOP.<loop>, all
  loop-wide, so the only way to halt a single wedged or superseded worker is a human finding
  its task and stopping it by hand. The observer (brief 01) can now say which run is dead;
  it needs a primitive that halts that run and nothing else, and the primitive has to reach
  the worker through two independent paths because one of them (the cooperative flag) is
  exactly what a wedged worker never reads.
wave: 1
depends: ["desk-supervision/01"]
unblocks: ["desk-supervision/03"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §8.5 ('terminate the worker and queue a retry'), §10.6 (timeouts and error mapping) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/desk/internal/deskkit/killswitch.go — flag precedence DISABLED > STOP > STOP.<loop>; an unrecognised loop name never masks a STOP; flags live in the state directory only (dirOverride is a test hook, deliberately not an env var)"
  - "tools/desk/cmd/deskdispatch/main.go step 2 — the worktree is created by `deskwt add` in the item's own repo root; the claim key is derived by a fixed rule so every desk computes the same key"
  - "plugins/assay/references/claude-code.md — capability:durable-monitor and capability:session-notifications are the desk window's wake signals; skill bodies use capability vocabulary, never harness tool names"
  - "freshness-checked 2026-09-02 @ 30c9934 — no per-run stop exists in killswitch.go"
exec-tier: strong
exec-tier-why: >-
  (c): safety plumbing in the kill-switch path. A stop that a mis-derived key can mask, or a
  precedence slip that lets a per-run flag override a loop-wide STOP, weakens the one control
  every desk verb runs first.
consumers:
  - "tools/desk/internal/deskkit/killswitch.go Guard() precedence: fixed-here (every desk verb runs Guard first, so the new layer reaches all of them with no per-verb edit)"
  - "tools/desk/cmd/deskdispatch/main.go step 2 (worktree-create): fixed-here (records the run key worktree-locally so verbs resolve it from cwd)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Cadence and wake: fixed-here (the desk window's sweep reads `desksupervise status --stops` and issues the harness-side stop in capability vocabulary)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md and verify-desk/SKILL.md cadence sections: follow-up desk-supervision/07 (the same sweep line lands with the snapshot verb for all three windows)"
  - "docs/streams/desk-containers (process-level kill in container mode): out-of-scope (a container desk's process kill is that stream's launch/control layer; this brief's two layers do not depend on it)"
---

# Brief 02 — Per-run stop signal

## Context

files:
- `tools/desk/internal/deskkit/killswitch.go` — add the `STOP.run.<key>` layer below
  `STOP.<loop>` in `Guard()`; the run key resolves from the worktree, never from an
  argument.
- `tools/desk/internal/deskkit/killswitch_test.go` — precedence and masking tests.
- `tools/desk/cmd/deskdispatch/main.go` — step 2 writes the run key worktree-locally
  (`git config --worktree assay.runKey <claim-key>`) right after `deskwt add`.
- `tools/desk/cmd/desksupervise/main.go` (planned) — `stop <key> --reason "..."` arms the flag and
  audits; `tick` arms it automatically for `HEARTBEAT-EXPIRED` / `NEVER-STARTED` before
  releasing the claim; `status --stops` lists armed stops (the desk window's read).
- `plugins/assay/skills/worker-desk/SKILL.md` §Cadence and wake — one added sentence:
  the sweep reads armed stops and issues the harness-side stop for each, in capability
  vocabulary.

single-point-of-failure: NONE claimed — two layers by design. Layer A: the flag, enforced
by `Guard()` in every desk verb the worker runs next (fails when the worker never runs
another verb). Layer B: the desk window's cadence sweep reading `status --stops` and
stopping the dispatched agent through the harness (fails when the desk window itself is
dead — which the existing loop-wide heartbeat lease and the cadence backstop already
cover). They fail for different reasons in different components.

facts:
- `Guard()` precedence today (`killswitch.go`): `DISABLED` > `STOP` > `STOP.<loop>`, flags
  are files in the state directory (`~/.config/assay`), a mis-spelled `DESK_LOOP` never
  masks a `STOP`. The new layer sits strictly BELOW these: a per-run flag can only add a
  refusal, never lift one.
- The run key is the claim key deskdispatch already derives (`<repo>--<stream>--<NN>` or
  `<repo>--issue-<NN>`), sanitised to `[A-Za-z0-9._-]` for the file name.
- Verbs run inside the item's worktree (that is the isolate-first rule), so `git config
  --worktree assay.runKey` is readable by every verb from cwd with no agent cooperation. A
  verb run outside any worktree has no run key and skips the layer (no false refusals).
- Exit code for an armed stop is 3 (disabled) with a reason line naming the key and the
  recorded `--reason`, matching `STOP.<loop>` behaviour.
- Skill bodies name mechanisms by capability (`capability:dispatch-worker` etc.); the
  harness-side stop is described as "stop the dispatched worker" and bound per harness in
  `plugins/assay/references/<harness>.md` — a `harnesslint` closure check exists for those
  files.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `killswitch.go`: after the `STOP.<loop>` check, resolve the run key from
   `git config --worktree assay.runKey` in cwd (absent ⇒ skip); if
   `<StateDir>/STOP.run.<key>` exists, refuse with exit 3 and the file's first line as
   reason. Add `ArmRunStop(key, reason)` / `ListRunStops()` helpers (state-dir only).
2. `deskdispatch` step 2: set `assay.runKey` in the new worktree; `--dry-run` prints the
   key it would record.
3. `desksupervise stop <key> --reason R` (audited); `tick` arms before release for the
   two reclaim classes; `status --stops` prints `key  armed_at  reason` lines.
4. Worker-desk skill: the cadence sweep's one added step, capability vocabulary only;
   add the binding row to `plugins/assay/references/claude-code.md` (and the degradation
   cell in `codex.md`) if `harnesslint bindings` requires it.
5. Tests: precedence (`STOP` still wins with a run flag present; a run flag for key A does
   not touch key B; no worktree ⇒ no refusal); arm/list round trip; deskdispatch records
   the key.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'RunStop\|Killswitch\|StopFlag' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise stop --help` | exit 0; output contains `--reason` |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestRunStopRefusesOnlyItsOwnKey -v -count=1` | exit 0; output contains `--- PASS: TestRunStopRefusesOnlyItsOwnKey` |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestRunStopNeverMasksStopAll -v -count=1` | exit 0; output contains `--- PASS: TestRunStopNeverMasksStopAll` |
| 5 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run TestDispatchRecordsRunKey -v -count=1` | exit 0; output contains `--- PASS: TestDispatchRecordsRunKey` |
| 6 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestTickArmsStopBeforeRelease -v -count=1` | exit 0; output contains `--- PASS: TestTickArmsStopBeforeRelease` |
| 7 | `grep -c 'status --stops' plugins/assay/skills/worker-desk/SKILL.md` | output is `1` or more |
| 8 | `cd tools/harnesslint 2>/dev/null && GOWORK=off go run . bodies ../../plugins/assay/skills && GOWORK=off go run . bindings ../../plugins/assay/references \|\| echo SKIP-no-harnesslint` | exit 0; output does not contain `FAIL` |
| 9 | `statusgen --root . --consumers --brief desk-supervision/02` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "a per-run flag masks the loop-wide STOP" → row 4; "the key
derivation differs between dispatch and observer so the flag never matches" → rows 5, 6
share one fixture key; "worker never runs another verb so the flag is dead" → layer B is
the skill sweep, row 7 proves it is written (its live behaviour is review-only until a
harness smoke exists); "skill edit names a harness tool" → row 8.

## Evidence
### Non-implementer verifier run — VERIFY: PASS (9/9 rows; row 9 re-run clean with a current-source statusgen after #557 closed as stale-oracle) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `67abbac`

Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`); desk rows module-scoped from `tools/desk`. Frontmatter: `gate: model`, all risk `no`, `irreversible: no`. Implementation: commit `5c28224` (per-run stop signal) + `39e3c7c`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | go test ./internal/deskkit -run run-stop/killswitch/stop-flag | exit 0, ok | exit 0, ok (weak `-run` glob matched none; substantive cases covered by rows 3-4) | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | go build ./cmd/desksupervise && desksupervise stop --help | exit 0; --reason | exit 0 — help shows "--reason is REQUIRED" | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | go test -run run-stop-refuses-only-its-own-key -v | exit 0 PASS | exit 0 — PASS (3 subtests) | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | go test -run run-stop-never-masks-stop-all -v | exit 0 PASS | exit 0 — PASS (ordering: run-stop resolved AFTER loop-wide flags) | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | go test ./cmd/deskdispatch -run dispatch-records-run-key -v | exit 0 PASS | exit 0 — PASS (records assay.runKey) | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | go test ./cmd/desksupervise -run tick-arms-stop-before-release -v | exit 0 PASS | exit 0 — PASS | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | grep -c 'status --stops' worker-desk/SKILL.md | ≥1 | exit 0 — 2 | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | harnesslint (or SKIP-no-harnesslint) | exit 0, no FAIL | harnesslint absent tree-wide → designed fallback SKIP-no-harnesslint, exit 0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 9 | statusgen --root . --consumers --brief desk-supervision/02 | exit 0; no DISPROVED | PASS — exit 0, no DISPROVED; consumers UNCHECKED by construction (this branch did not make those claims). Re-run with a statusgen built from current public main `5d20ff9`; the earlier exit-2 abort on docs/streams/decisions/README.md was a STALE-ORACLE (pinned v0.27.0 lacked the complete reservedRegisterNames skip that b730bd8 added; current source skips the decisions register — #557 closed stale-oracle) | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — ExitDisabled = 3 @ tools/desk/internal/deskkit/exitcodes.go:20 — the exit an armed STOP.run.<key> returns (killswitch.go:485); REUSED not introduced — a per-run stop must be indistinguishable from a loop-wide STOP to every desk verb, so existing exit-3 handling applies. Flag/dir modes 0o600/0o700 (the only new numeric literals) are owner-only least-privilege for control-plane flags. Reversible.`
`RISK-VALUE: NAMED, NOT DERIVED — runStopPrefix = "STOP.run." @ tools/desk/internal/deskkit/killswitch.go:22 — a string, not a numeric bound; collision-safety with STOP / STOP.<loop> is enforced by the exact-match guard (n != runStopPrefix @ :282) and proven by row 4. The brief's safety property (a per-run flag can never mask a loop-wide DISABLED/STOP) is an ORDERING (runStopState resolved at guard() step 3, after loop-wide flags @ :476-486), not a literal — proven by row 4 PASS.`

**VERIFY: PASS** — all 9 rows PASS (per-run stop signal is ordered after the loop-wide flags and cannot mask them; --reason required; run-key recorded). Row 9's earlier could-not-check was a STALE-ORACLE (pinned v0.27.0 statusgen predating b730bd8's complete `reservedRegisterNames` skip); re-run with a statusgen built from current public main `5d20ff9` exits 0 with no DISPROVED (consumers UNCHECKED by construction). #557 closed stale-oracle. `gate: model`, all risk `no` — advances `implemented → verified`.

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
