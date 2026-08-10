---
brief: desk-tools/11
title: deskboard orders actionable PRs by gate score (statusgen --gate-scores)
wave: 1
depends: ["desk-tools/02"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [885]
schema: brief-v1
authored: 2026-07-20 by Opus 4.8 authoring session (intake Tier-2, #885)
sources: ["[I-deskboard-gateorder](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-17-gate-scores-deskboard.md)", "methodology-metrics/11 (the --gate-scores JSON emitter + PR→brief mapping contract)", "docs/streams/desk-tools/brief-02-deskboard.md (deskboard actions)", "#885 (tracking)"]
why: >-
  methodology-metrics/11 shipped the `--gate-scores` JSON emitter and ordered the Awaiting board
  by score, but desk-tools/02 (deskboard) was already `done` when brief 11 landed, so the
  PR-ordering half was deferred. deskboard's actionable-PR listing still surfaces PRs in arrival
  order — the reviewer drains oldest-first instead of highest-priority-first, exactly the
  arrival-order bias mm/11 removed on the brief side.
---

# Brief 11 — deskboard orders actionable PRs by gate score

## Context
files:
- `../assay-toolkit/tools/desk/cmd/deskboard/` — the `actions` subcommand (NEEDS-REVIEW / RE-REVIEW / … rows);
  the JSON + `--table` renderers
- `../assay-toolkit/tools/desk/cmd/deskboard/*_test.go` — the classifier tests (extend, do not rewrite)
- `tools/statusgen` — the `--gate-scores` JSON emitter (mm/11; the source of truth for scores)
- `docs/streams/desk-tools/README.md` — table row + one convention line

facts:
- mm/11's Context specifies the full contract, reuse it verbatim: **PR→brief mapping via the
  brief row (branch-as-claim)** — a PR's head branch (`fix/issue-NN`, `brief/<stream>-NN`, etc.)
  or body maps to its owning brief; **default weight for unowned PRs** (register PRs, docs) —
  they take a neutral default score; **oldest-first tie-break** among equal scores.
- Design: `deskboard actions` invokes `statusgen --gate-scores` (JSON) once per run, builds the
  brief→score map, maps each actionable PR to its owning brief via branch-as-claim, and orders
  the actionable rows by score descending, oldest-first on ties. Both the JSON output and the
  `--table` render carry the resolved `score` + `owningBrief` (or `unowned`) per PR so the
  ordering is inspectable, never opaque.
- Read-only end-to-end (C-7/C-9): invoking `statusgen --gate-scores` is a local read; no new
  mutating surface. C-10 fail-closed carries over — if `--gate-scores` fails or emits
  unparseable JSON, exit 6 naming the failure (never silently fall back to arrival order, which
  would hide that scoring is broken).
- The score is GUIDANCE, not a hard gate (mm/11's rule): a human request to review a specific PR
  legitimately jumps the queue; ordering only sets the default drain order. FLIP/security-review
  semantics from brief 02 are unchanged — this brief reorders rows, it does not change which
  action a PR gets.
- Depends on desk-tools/02 (deskboard exists) and consumes mm/11's emitter (already `verified`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Extend `deskboard actions` to invoke `statusgen --gate-scores`, build the brief→score map, map
   each actionable PR to its owning brief (branch-as-claim, per mm/11), and order actionable rows
   by score desc / oldest-first tie-break. Surface `score` + `owningBrief`/`unowned` per PR in
   both JSON and `--table`.
2. Fail closed (C-10): a `--gate-scores` failure or parse error → exit 6 naming the failure, never
   a silent arrival-order fallback.
3. Tests (fake gh + a stubbed/fixture `--gate-scores` output): owned PRs order by score desc;
   equal-score PRs order oldest-first; an unowned PR takes the default weight and sorts among the
   rest; a `--gate-scores` failure → exit 6. FLIP/security-review classifier tests stay green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskboard/... -count=1` | exit 0; includes score-desc ordering, oldest-first tie-break, unowned-default, and gate-scores-failure→exit-6 subtests; existing classifier tests still pass |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `go run ./tools/desk/cmd/deskboard actions 2>&1 \| python3 -m json.tool > /dev/null; echo $?` | 0 (valid JSON against live repos; rows carry `score` + `owningBrief`; read-only run) |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verified 2026-07-31 by glm-5.2-verifier (non-implementer) against merged main `07b7bbc7`.

**Cross-repo note (assay-selfcontain/02):** the source `tools/desk/cmd/deskboard/` moved out of oit to the **assay-toolkit** sibling. At oit `07b7bbc7` the `tools/desk/` tree is gone, so the literal `go run ./tools/...` paths can't run in oit. The implementation commit `83e1c985` IS an ancestor of oit main (confirmed) and was ported to assay-toolkit as `0efe655` (PR #211). Go rows (1, 2) ran against assay-toolkit `origin/main` `5ae9bf1` where the source lives; functional rows (3, 4) used the installed binaries.

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskboard/... -count=1` (in assay-toolkit `5ae9bf1`) | 0 | `ok …/deskboard 5.461s`. All 4 brief-11 subtests PASS: `TestGateScores_ScoreDescOrdering`, `_OldestFirstTieBreak`, `_UnownedDefault`, `_FailureExit6`; existing classifier tests green (`TestClassify` 19 subtests, `TestSecurityReviewRequired_216`, `TestActions_PartialFailure_Exit6`, `TestMergeNow_*`). (Unrelated `cmd/damlbreakguard` VCS-stamping failure in the broader suite is a detached-worktree artifact, different package.) |
| 2 | `go vet ./tools/desk/...` (assay-toolkit) | 0 | clean — no output for `./cmd/deskboard/...` and full `tools/desk`. |
| 3 | `deskboard actions \| python3 -m json.tool > /dev/null` (installed binary, read-only gh GETs) | 0 | 24005 bytes valid JSON; 48 actionable rows, **every row carries `score` + `owningBrief`**. Live ordering proven: owned PR `oit#1438` (`owningBrief='assay-dogfood/01'`, `score=2200`) sorts first; 47 unowned (`score=0`) below; scores non-increasing; oldest-first tie-break (equal score → PR# asc) holds. |
| 4 | `statusgen --root . --lint` (pinned `/Users/iholsman/.local/bin/statusgen`) | 0 | `LINT: PASS` (in-repo `go run ./tools/statusgen` is the frozen #465 false-positive). |

`RISK-VALUE: DERIVED — defaultGateScore = 0` @ `tools/desk/cmd/deskboard/board.go:694` (assay-toolkit) — the only production literal this brief adds; grounded in mm/11's gate-score floor (min P2 = 1000) via the code comment @ :693 ("Zero sorts below every real score (min P2=1000)"), so 0 < 1000 guarantees unowned PRs sort below every owned PR. A value > 1000 would invert priority; 0 avoids it. Reversible heuristic weight.

## Review
Gate: model (all four risk answers no — read-only cross-repo board tooling; reorders rows,
introduces no mutating surface). Reviewer confirms (a) ordering uses mm/11's PR→brief mapping and
oldest-first tie-break, (b) a `--gate-scores` failure fails closed (exit 6), never a silent
arrival-order fallback, and records verdict + date in the stream README table.
