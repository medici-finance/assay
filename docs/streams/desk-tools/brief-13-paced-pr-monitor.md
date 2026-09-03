---
brief: desk-tools/13
title: "`pr-monitor.sh` — a paced, per-repo head-sha / draft-state PR monitor shipped in the plugin tree"
why: >-
  The review desk's event monitor — poll every open PR's head sha and state across the repo
  set, emit on change — is described in the skill and hand-written by every session that boots
  it. A 24-hour sweep of fifteen desk-role and worker session transcripts found one review
  loop write a 16-repo tight-loop poller that tripped the forge's secondary rate limit, which
  then blocked the same session's flip tool from reading its model floor; the session hand-
  wrote a paced version with a sleep between repos. The inbound (issue) monitor already ships
  as a script precisely because a hand-rolled poll keeps re-acquiring the same three bugs; the
  PR monitor has the same three plus pacing, and it should ship the same way, with the pacing
  a documented contract rather than a sleep someone remembers to add.
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
  - "freshness-checked 2026-09-02 @ 547b708 — `plugins/assay/scripts/` ships `inbound-monitor.sh` (issues) and `assay-inbox.sh` only; no PR monitor script exists; `inbound-monitor.sh`'s per-repo loop has no pacing between `gh` calls. `plugins/assay/skills/pr-review-desk/SKILL.md` § boot step 3 describes the event monitor in prose (`gh pr list --state open --limit 100` head-shas + states, keyed `<slug>#<num> <sha> <state>`, pre-seeded) and every session writes it. `tools/desk/cmd/reviewloop/` consumes `deskboard` JSON and carries no monitor; `tools/desk/cmd/scanloop/monitor.go` locates, arms and parses the inbound script."
  - "The monitor the new script mirrors line for line where the concern is shared: `plugins/assay/scripts/inbound-monitor.sh` — per-repo state files, seed-silently-on-first-sight, `MONITOR-ARMED:` / `MONITOR-DEGRADED:` lines, the truncation and collapse guards, the `.assay/repos.txt` repo resolution; and its test harness `inbound-monitor.test.sh` (fake `gh` on PATH)."
  - "The outward-write budget is NOT the control here — the monitor makes reads — but the breaker vocabulary it should echo when degraded: `tools/desk/internal/deskkit/ratelimit.go`."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
---

# Brief 13 — `pr-monitor.sh`: a paced, per-repo head-sha / draft-state PR monitor

## Dependencies
None.

## Context

files:
- `plugins/assay/scripts/pr-monitor.sh` (planned)
- `plugins/assay/scripts/pr-monitor.test.sh` (planned) — fake `gh`, same harness shape as the inbound test
- `plugins/assay/scripts/inbound-monitor.sh` (adopt the same pacing knob — one `sleep` site and the contract comment; no other change)
- `plugins/assay/skills/pr-review-desk/SKILL.md` (boot step 3 names the script instead of describing a poll to write)
- `tools/desk/README.md` (the pacing contract, in the monitors section)

facts:
- per repo, one read: `gh pr list --repo <slug> --state open --limit <N> --json
  number,headRefOid,isDraft,state,mergeStateStatus` with `--limit` explicit and mandatory
  (a bare call caps at 30 silently); a count equal to the limit is TRUNCATED → `MONITOR-DEGRADED`
  for that repo, previous baseline kept — the inbound script's exact rule.
- state per repo: `<state-dir>/<owner__name>.state`, one line per PR
  `<num> <head-sha> <draft|ready> <state> <mergeState>`; first sight SEEDS silently.
- emitted lines, one per change, machine-parsable, stable field order:
  `PR-EVENT: <slug>#<num> <kind> <old> -> <new>` where kind ∈ `opened | pushed | draft-flip |
  state | merge-state | closed`; plus `MONITOR-ARMED: <n repos>` on the first run and
  `MONITOR-DEGRADED: <slug> <why>` on any read the script cannot trust.
- **pacing contract** (the new thing): `ASSAY_MONITOR_PACE_SECONDS` (default **2**) is slept
  between consecutive repo reads; `ASSAY_MONITOR_MAX_REPOS_PER_CYCLE` (default **0** = all)
  caps a cycle and carries the cursor to the next run in the state dir; a `gh` exit carrying
  the secondary-limit signature (HTTP 403 with `secondary rate limit` in stderr, or 429) marks
  EVERY remaining repo `MONITOR-DEGRADED: <slug> rate-limited, skipped` and ends the cycle
  without further calls — one tripped limit must not be compounded by the remaining reads.
  Pace and cap are echoed on the `MONITOR-ARMED` line so a transcript shows what was in force.
- the script never writes to the forge and holds no credential of its own: it inherits the
  identity the calling shell already has, exactly as the inbound script does.
- test harness: a fake `gh` on PATH that serves canned JSON per repo, records call
  timestamps, and can return the 403 signature on demand; the test asserts events, pacing
  (≥ pace seconds between the recorded calls), and the stop-on-limit rule.
- bash 3.2 compatible (no `mapfile`, no associative arrays) — the inbound script's stated floor.

## Ground rules
- No test or Verify row calls the real `gh`; the fake on PATH is the only `gh`.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **`pr-monitor.sh`** per the facts, structured as the inbound script is (resolve repos,
   validate slugs, state dir, seed, diff, emit), plus the pacing block.
2. **Pacing knob in `inbound-monitor.sh`**: the same `ASSAY_MONITOR_PACE_SECONDS` sleep between
   repo reads and the same stop-on-limit rule; nothing else in it changes, and its existing
   test stays green.
3. **`pr-monitor.test.sh`**: opened / pushed / draft-flip / state / closed each produce exactly
   one `PR-EVENT` line with the right old/new; a seed run emits no events; truncation degrades
   the repo and keeps its baseline; the 403 signature on repo 2 of 4 degrades repos 2–4 and
   makes no further `gh` call (the fake's call log proves it); consecutive call timestamps are
   ≥ the pace; `ASSAY_MONITOR_PACE_SECONDS=0` is honoured for the test's own speed.
4. **Skill text**: boot step 3 points at the script and the two knobs; the described contract
   (`--limit` explicit, pre-seeded, never a disowned loop) stays.
5. **README**: the pacing contract, in one table.
6. **Nothing else.** No Go arming code in `reviewloop` (the skill arms the script through the
   session's durable monitor, as it does the inbound one); no change to `deskboard`.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `bash -n plugins/assay/scripts/pr-monitor.sh && bash -n plugins/assay/scripts/inbound-monitor.sh` | exit 0 |
| 2 | check:ci | `bash plugins/assay/scripts/pr-monitor.test.sh` | exit 0 — every case above passes; the output names each case |
| 3 | check:ci | `bash plugins/assay/scripts/inbound-monitor.test.sh` | exit 0 — the inbound script's existing suite is unchanged and green with the pacing knob |
| 4 | check:ci | `bash plugins/assay/scripts/pr-monitor.test.sh --case rate-limit-stops-cycle` | exit 0 — the NEGATIVE control: after the 403 signature the fake `gh` records NO further call, and every remaining repo carries a `MONITOR-DEGRADED` line |
| 5 | check:ci | `bash plugins/assay/scripts/pr-monitor.test.sh --case pacing-honoured` | exit 0 — with pace 1 and three repos, the fake's call log spans ≥ 2 seconds |
| 6 | check:ci +flow | `test -x plugins/assay/scripts/pr-monitor.sh && grep -q 'pr-monitor.sh' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'ASSAY_MONITOR_PACE_SECONDS' tools/desk/README.md && grep -q 'ASSAY_MONITOR_PACE_SECONDS' plugins/assay/scripts/pr-monitor.sh && grep -q 'ASSAY_MONITOR_PACE_SECONDS' plugins/assay/scripts/inbound-monitor.sh` | exit 0 — the cross-component path: the skill names a script that exists and is executable, and the knob the README documents is the knob BOTH scripts read |
| 7 | check:ci +dereference | `d=$(grep -o 'ASSAY_MONITOR_PACE_SECONDS:-[0-9]*' plugins/assay/scripts/pr-monitor.sh \| head -1 \| sed 's/.*-//'); [ -n "$d" ] && grep -q "ASSAY_MONITOR_PACE_SECONDS.*default.*$d" tools/desk/README.md` | exit 0 — the default the README states is dereferenced against the default the script implements, not restated |
| 8 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Pace implemented as a sleep AFTER the last repo only | row 5 |
| A tripped limit followed by the remaining 14 reads | row 4 |
| A truncated page read as "those PRs closed" | row 2 (truncation case) |
| First run floods every open PR as `opened` | row 2 (seed case) |
| Draft flip and push on the same PR collapse to one event | row 2 (the pushed+draft-flip fixture expects two lines) |
| The inbound script's behaviour changes beyond the knob | row 3 |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer confirms the script holds no credential,
makes no write, and that row 4 is genuinely negative — the fake `gh` must be armed to fail and
the call log must be asserted empty after it.
