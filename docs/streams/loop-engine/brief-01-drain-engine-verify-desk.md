---
brief: loop-engine/01
title: Drain engine + verify-desk reference consumer — the outer loop leaves the model's attention
wave: 0
depends: []
unblocks: ["loop-engine/02", "loop-engine/03", "loop-engine/04", "loop-engine/05", "loop-engine/06"]
effort: L
gate: human
gate-why: the cutover — a code conductor becoming the driver of a standing desk that writes Evidence and status straight to main — is human:<name>'s trade to sign (mirrors desk-tools C-1, enabling automation is a human act); implementation dispatches normally, the gate binds the cutover + sign-off.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [541]
schema: brief-v1
authored: 2026-07-19 by Fable design session (human:<name>'s fix-the-verify-loop direction)
sources: ["docs/loop-engine-architecture.md (§4 contract, §7 skeleton, §9 open questions — freshness-checked 2026-07-19)", ".claude/skills/verify-desk/SKILL.md (the loop being mechanized; tier + irreversible carve-out rules)", "tools/desk/internal/deskkit (Guard/audit/idempotent/claims conventions)", "#541 (inline-verify incident: Agent calls 0, artifacts 0)", "F-verify-self-attest (2026-07-17)", "human:<name> 2026-07-15: verify dispatches local model only, never opus/external"]
exec-tier: strong
exec-tier-why: contract-setting concurrency design; the engine's shape binds four downstream briefs — an economy-tier contract mistake propagates to every consumer.
why: >-
  The verify loop boots but does not sustain: the outer control loop (queue → fan out N →
  land → refill → re-poll) runs as prose in the operator model's attention, and under load
  the model sheds bookkeeping by collapsing to single-threaded — a human then stands in for
  the missing scheduler. Code cannot get confused about pool occupancy. Same move closes the
  #541 class structurally (no inline path exists) and makes Evidence attribution structural
  (land() writes from the dispatched runner's typed return, not operator free text).
---

# Brief 01 — Drain engine + verify-desk reference consumer

## Context
files: `../assay-toolkit/tools/desk/internal/loopengine/` (new: engine.go, claim.go, tests),
`../assay-toolkit/tools/desk/cmd/verifyloop/` (new: verify-desk adapter — SelectQueue off the statusgen
Awaiting filter, TierPolicy = local-model-always + risk routing, Land = Evidence + flip +
push-race loop), `../oit/.claude/skills/verify-desk/SKILL.md` (boot section repointed at the engine;
judgment/irreducible sections stay)
facts:
- The contract is architecture doc §4 verbatim: `SelectQueue/Claim/TierPolicy/Dispatch/Land/
  is_done/OnIdle`; main loop = guard → fill-to-N → land-as-returned → refill → idle-poll;
  NEVER exits on empty queue; stop-flags are the only exit (deskkit.Guard precedence
  DISABLED > STOP > STOP.verify-desk).
- Dispatch mode: resolve Open Question 9.1 FIRST — if a harness primitive lets code call the
  Agent tool, `Run()` is a real process; otherwise ship the interim mode (engine step tool
  prints exact dispatches; the model executes them verbatim and feeds structured results
  back — zero scheduler state in attention). Ship the interim mode either way (debug surface).
- TierPolicy encodes TODAY'S rules unchanged: local session model always; risk-flagged
  (`gate: human` or any risk yes) → TierHuman (checkpoint-PR / labeled-issue path, drain
  continues); `irreversible: yes` → Evidence written, NO status flip, checkpoint PR for human:<name>.
  The F-16 middle rung (arch doc §9.2) is human:<name>'s open decision — build it as a one-line
  change, do not enable it.
- Author≠runner is a typed refusal: the engine compares the brief's implementer identity to
  the dispatch target and refuses with a distinct exit code, never a prompt-carried rule.
- Land() consumes only the structured Result (command → exit → key output rows, runner ID as
  dispatched); Evidence rows are rendered from it, dated + runner-attributed; push-race
  retry (`commit → pull --rebase → push`) lives in code. A Land that cannot succeed files a
  bug issue and continues (FILE don't ask, DRAIN don't wait) — the drain never wedges.
- Honest bound (arch doc §1.1): this is attribution + audit + structural separation, NOT
  un-forgeability; do not claim otherwise in code comments, README, or PR body.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- gate: human binds APPROVAL, not implementation: build everything, but the CUTOVER — the
  standing verify window actually booting the engine as its driver — is human:<name>'s act; stop at
  the documented stop-point (engine built, drill green, skill edit staged) and report
  BLOCKED-ON-IAN on the PR.
- **Cutover checklist — reconcile the prompt template against the skill BEFORE the flip
  (#1267).** The dispatched-verifier requirement list has two consumers: `.claude/skills/
  verify-desk/SKILL.md` loop step 2 ("Its prompt MUST carry:") and `renderDispatchPrompt` in
  `../assay-toolkit/tools/desk/cmd/verifyloop/dispatch.go`. They already diverged once — SKILL.md gained the
  F-28 name-and-derive-the-risk-bearing-value step (PR #1266) and the Go template did not — and
  at cutover a mechanized template silently drops whatever the manual desk gained. At the gate:
  run `go test ./tools/desk/cmd/verifyloop/ -run Dispatch -count=1` (the `dispatchRequirements`
  registry is bound to the skill file in both directions, 1:1 — an entry may claim exactly one
  bullet and a bullet exactly one entry) and clear every `notYetCarried` entry in that registry.
  **This is now a live CI gate (#1374)**: `../oit/scripts/go-check-workspace.sh` runs `go test -count=1
  ./...` over every workspace module, `/tools/desk` included, so the check is readable off the PR
  page. (It was not when this brief was written — #1262 was open and `tools/` ran nowhere in CI.)
- Contract freeze: if the verify-desk adapter needs a hook §4 doesn't have, STOP and file a
  design issue (contract-erosion risk, §8) — do not add the hook.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `loopengine`: implement the §7 skeleton for real — Config/Item/Result/Loop/Handle,
   `Run()`, `Claim()` (noclobber, stale-claim 120m+no-branch policy), author≠runner refusal,
   Guard() at every iteration boundary, land-as-returned (never wave-end batching).
   Table-driven tests incl. the negative set: claim collision, author==runner, stop-flag
   mid-drain (started land completes, no new dispatch), land-failure files-and-continues,
   pool refill after completion, is_done→OnIdle (never exit).
2. `cmd/verifyloop` adapter: SelectQueue = Awaiting table (statusgen filter, tier-1
   empty-Evidence before tier-2 free-closes, oldest-first within class); TierPolicy per
   facts; Dispatch prompt template carrying the brief's Verify table + target SHA + isolation
   requirements (own /private/tmp worktree, F-35: no shared-checkout path in the prompt);
   Land per facts.
3. Drain drill against a FIXTURE queue (no live briefs): N≥3 concurrent fixture items, one
   deliberate FAIL, one deliberate un-landable → drill proves sustain-at-N, land-as-returned,
   file-and-continue, drain-to-empty, idle-poll, stop-flag exit.
4. Stage (do not apply as cutover) the verify-desk SKILL.md edit: boot section points at the
   engine; irreducibles (§6) stay in the skill; diff in the PR body for human:<name>'s cutover call.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0; includes every negative case in Task 1 |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `go test ./tools/desk/internal/loopengine -run 'Drain' -count=1 -v 2>&1 \| grep -cE -e 'landed' -e 'filed-and-continued'` | ≥4 (fixture drill: concurrent lands + the file-and-continue path both exercised) |
| 4 | `DESK_LOOP=drilltest go test ./tools/desk/internal/loopengine -run 'StopFlag' -count=1` | exit 0 (mid-drain stop: started land completes, no new dispatch) |
| 5 | PR body contains the staged verify-desk SKILL.md before/after diff + the BLOCKED-ON-IAN cutover stop-point | present |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

### Non-implementer verifier run — VERIFY: PASS (stays `implemented`, `gate: human`) — glm-5.2-verifier, 2026-07-22

Isolated worktree off current `origin/main` `4809aecc` (`.claude/worktrees/agent-a1365354151c67166`); shared checkout not touched. Loopengine code present on main (`../assay-toolkit/tools/desk/internal/loopengine/{engine,claim,doc}.go` + 4 test files; `../assay-toolkit/tools/desk/cmd/verifyloop/` adapter, 9 files; impl commit `242e47e2`, merged via **PR #867** `84155d4f`).

| # | Command | Exit | Key output | Result |
|---|---|---|---|---|
| 1 | `go test ./tools/desk/... -count=1` | 0 | `ok …/internal/loopengine 1.913s` (+ all 14 packages) | PASS |
| 2 | `go vet ./tools/desk/...` | 0 | (clean) | PASS |
| 3 | `go test … -run 'Drain' -v \| grep -cE -e 'landed' -e 'filed-and-continued'` | 0 | count **6** (drill-ok ×3, drill-fail, filed-and-continued drill-stuck, in-flight-1) — ≥4 | PASS |
| 4 | `DESK_LOOP=drilltest go test … -run 'StopFlag' -count=1` | 0 | `ok …/internal/loopengine 0.214s` | PASS |
| 5 | PR #867 body: staged SKILL.md diff + BLOCKED-ON-IAN | — | `## BLOCKED-ON-IAN (cutover)` present; `## Staged verify-desk SKILL.md edit (NOT applied)` carries the before/after boot-section diff | PASS |
| 6 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | `STATUSGEN_EXIT=0` (NOTICEs only, no PROBLEM) | PASS |

**VERIFY: PASS — all 6 rows green.** Negative paths genuinely exercised: `landed: drill-fail (FAIL)`, `filed-and-continued: drill-stuck (push race unresolved after retries)`, stop-flag refusal `stop: refused: stop flag active (result=disabled)`. The Drain-drill count (6) comfortably exceeds the ≥4 bar (`-run 'Drain'` also matches `TestStopFlagMidDrain`, contributing one `landed:` line either way).

**Row-6 note:** PR #867's own Verify table recorded row 6 as `exit 1 — PRE-EXISTING` (CLAUDE.md 2897 > 2850, #280); that no longer reproduces — CLAUDE.md has since been trimmed below 2850 on main, so lint exits 0 cleanly. No lint problem is attributable to this diff.

**Status: stays `implemented`.** `gate: human` (risk answers all `no`) — a model verifier records Evidence but **cannot** flip to `verified`; the cutover (a code conductor driving the standing verify desk) is human:<name>'s act, captured in PR #867's `BLOCKED-ON-IAN` + staged (NOT applied) SKILL.md edit. The flip is human:<name>'s via that cutover.

## Review
Gate: human (the cutover — a code conductor driving a standing desk — is human:<name>'s trade to
sign). Reviewer confirms (a) no inline-verify path exists anywhere in the adapter (the #541
class is unrepresentable, not discouraged), (b) TierPolicy encodes the current collapse
exactly (risk-flagged ⇒ human; irreversible ⇒ Evidence-no-flip + checkpoint PR) with the
F-16 rung left OFF behind a one-line change, (c) author≠runner is a typed refusal with a
distinct exit code, (d) Land consumes only structured Results — no free-text verdict reaches
a board cell, (e) the honest bound is stated (attribution, not un-forgeability), (f) the
engine calls deskkit guards rather than re-implementing them.
