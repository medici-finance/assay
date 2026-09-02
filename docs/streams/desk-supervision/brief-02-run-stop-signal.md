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
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
