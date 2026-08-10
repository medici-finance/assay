---
brief: desk-tools/08
title: Loop stop-flags — ALL/per-loop kill switch checked every iteration + heartbeat lease
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-tools/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [221]
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: dead-man's switch the loops check before iterating/every minute — ALL or per-loop", "docs/streams/desk-tools/scoping.md (C-6 kill switch, C-10 fail-closed)", "issue #221 (out-of-repo skill edit protocol)", "freshness-checked 2026-07-10 @ fb9223ce"]
why: >-
  The loops are standing background actors; today halting one means finding its session and
  killing it by hand, and halting ALL of them means doing that N times under pressure. A file
  flag any terminal can touch, honored within one iteration, is the minimum containment for
  autonomous loops — and a prerequisite for ever letting them run unattended. human:<name> asked for
  exactly this: ALL or individual, checked before iterating / every minute.
---

# Brief 08 — Loop stop-flags (ALL / per-loop) + heartbeat lease

## Context
files: `../assay-toolkit/tools/desk/internal/deskkit/` (extends brief 01's killswitch.go), tools/desk README
(operator section), `../assay-toolkit/tools/desk/cmd/deskboard/` (flag-state banner, coordinates with brief 02);
out-of-repo per issue #221 protocol: `~/.claude/skills/pr-review-desk/SKILL.md`,
`~/.claude/skills/verify-desk/SKILL.md`, `~/.claude/skills/batch-fanout/SKILL.md`,
`~/.claude/skills/the-desk/SKILL.md` (iteration-top check snippet + canonical name)
facts:
- Flags (all under `~/.claude/desk-tools/`, same dir as C-6's DISABLED):
  - `STOP` → ALL loops halt.
  - `STOP.<loop-name>` → that loop halts. Canonical names: `pr-review-desk`, `verify-desk`,
    `batch-fanout`, `the-desk`; an ad-hoc `/loop` or Monitor picks a name at arm time and
    states it in its description.
  - `HEARTBEAT` (dead-man lease, optional): if the file EXISTS and its mtime is older than
    24h, treat as STOP-ALL. Absent file = feature off (no behavior change until human:<name> opts in
    by touching it once). Renewal is a HUMAN act — human:<name> or an human:<name>-owned cron touches it;
    no agent/loop may touch or delete HEARTBEAT (that would defeat the dead-man semantics).
- Check semantics: at ITERATION BOUNDARIES only — top of every monitor poll cycle, on every
  scheduled wakeup, before each loop-skill cycle (all loops poll at ≤90s, satisfying the
  every-minute requirement). On hit: emit one line naming the exact flag file, exit cleanly.
  Never halt mid-action; a started outward write completes (C-5 audit intact).
- Layer 2, the teeth (tool layer): each loop session sets `DESK_LOOP=<canonical-name>` at
  boot; `deskkit.Guard()` additionally honors `STOP` and `STOP.$DESK_LOOP` → exit 3 with
  audit result=disabled, detail naming the flag. A loop that skips its own check is thereby
  DEFANGED — every outward verb refuses — which is the fail-safe direction. Precedence:
  `DISABLED` (C-6) ⊇ `STOP` ⊇ `STOP.<name>`; DISABLED halts tools for everyone, STOP scopes
  to loops.
- Relation to brief 01: additive to its killswitch contract (new file checks + one env read
  in Guard()). Brief 01 is `todo` — if unimplemented when this brief runs, implement both
  killswitch pieces together in 01's files; if implemented, extend. Either way the tests land
  under this brief.
- Restart is `rm <flag>` + re-arm the loop; flags are not self-clearing (C-10: a switch that
  un-flips itself is not a switch).
- deskboard (brief 02) banners any active STOP/stale-HEARTBEAT flag so a silently-stopped
  loop is visible on every board read, not discovered by absence.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Out-of-repo skill files follow issue #221's protocol: declare the exact paths, apply live
  edits as the LAST step before `implemented`, paste before/after diffs into the PR body.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. deskkit: extend `Guard()` — check `STOP`, then `STOP.$DESK_LOOP` (when env set), then
   HEARTBEAT staleness; each hit → the existing disabled path (exit 3) with detail naming
   the flag. Table-driven tests: each flag form alone; both; heartbeat fresh/stale/absent;
   DESK_LOOP unset (per-loop flags ignored, STOP still honored).
2. tools/desk README operator section: flag names, touch/rm usage, heartbeat opt-in and its
   human-only renewal rule, the precedence chain, and the loop-name registry.
3. Out-of-repo (per #221): add the iteration-top check snippet (3-line shell test) + the
   canonical `DESK_LOOP` boot line to the four loop skills; monitor templates in those
   skills gain the check inside their poll loops.
4. deskboard: banner active STOP flags and stale HEARTBEAT (coordinate with brief 02 — if 02
   is unimplemented, leave a typed TODO in its brief's Context via a one-line amendment).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/... -count=1` | exit 0; includes every flag/heartbeat case in Task 1 |
| 2 | `go test ./tools/desk/internal/deskkit -run TestGuardStopFlags -count=1 -v 2>&1 \| grep -c "STOP.pr-review-desk"` | ≥1 (per-loop flag exercised via unit-test; Guard tests are temp-dir-isolated by design — real flag-files have no effect) |
| 3 | `grep -c "STOP\." tools/desk/README.md` | ≥1 (operator doc present) |
| 4 | PR body contains before/after diffs of all four out-of-repo skill edits (#221 protocol) | present |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verify — VERIFY: PASS — glm-5.2-verifier, merged main `9221a1b2`, 2026-07-24

All 5 Verify rows green on `origin/main` HEAD `9221a1b2`. `Guard()` stop-flag + heartbeat tests
pass (15 packages ok); per-loop `STOP.<name>` exercised via `TestGuardStopFlags`; fail-closed path
asserted (unreadable flag dir → exit 6 / IsUnverifiable); deskboard STOP/stale-HEARTBEAT banner
present. Row 4 (out-of-repo #221 skill diffs) satisfied via merged PRs #694 + #1029 — all four
in-repo skill bodies carry the DESK_LOOP export + STOP-check snippet.

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/... -count=1` | 0 | ok — all 15 packages (deskkit, deskboard, deskpost, deskpr, deskwt, deskreply, deskroster, deskpushguard, desktoken, issueboard, verifyloop, writeguard, damlcigate, loopengine, bodycheck) | 2026-07-24 | glm-5.2-verifier |
| 2 | `go test ./tools/desk/internal/deskkit -run TestGuardStopFlags -count=1 -v \| grep -c "STOP.pr-review-desk"` | 0 | 1 (≥1; 15 subtests PASS — STOP alone/empty, STOP.<loop> match/unset/different, both, HEARTBEAT fresh/stale/absent/boundary, unreadable→exit 6, DISABLED>STOP, STOP>HEARTBEAT, audit-names) | 2026-07-24 | glm-5.2-verifier |
| 3 | `grep -c "STOP\." tools/desk/README.md` | 0 | 3 (≥1; operator doc: touch/rm, heartbeat opt-in + human-only renewal, precedence chain, loop-name registry) | 2026-07-24 | glm-5.2-verifier |
| 4 | PR body contains before/after diffs of all 4 out-of-repo skill edits (#221) | n/a | present — #694 (pr-review-desk) + #1029 (verify-desk/batch-fanout/the-desk, cross-refs #694); grep on in-repo skill bodies: prrd 4, vd 5, bf 9, td 5 | 2026-07-24 | glm-5.2-verifier |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only; no errors) | 2026-07-24 | glm-5.2-verifier |

**VERIFY: PASS.** Status flipped `implemented → verified` by the verify desk (gate: model, no risk).

## Review
Gate: model. Reviewer confirms (a) HEARTBEAT is human-renewal-only with no agent touch path,
(b) flags are checked at iteration boundaries and never mid-action, (c) the Guard() extension
fails closed (unreadable flag dir → exit 6 per C-10, not silent pass), (d) cutover (06) now
depends on this brief so loops never go zero-prompt without their kill switch.
