---
brief: statusgen/01
title: 30-day statusgen check-firing audit — retire cold --lint rules
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-20 (authored clean for the statusgen board)
sources:
  - "statusgen main.go run() — the check chain this audit tallies per-rule firings over"
  - "An intake note observing statusgen's run() had accreted ~14 check functions with no periodic audit of whether each still catches live problems"
  - "A critical-thinking review that flagged the lint surface as monotonically growing"
why: >-
  statusgen's run() chains ~14 check functions, each added to stop a specific incident, and
  the accretion is visible — but nothing periodically asks whether every rule still catches
  live problems or whether some have gone cold. A rule that has fired zero times in 30 days
  and gates no regression test is a candidate for retirement; without a firing audit the
  lint surface only ever grows.
---

# Brief 01 — 30-day statusgen check-firing audit

## Context

files:
- `statusgen/main.go` — `run()` chains the check functions (`check`, `checkBriefFiles`,
  `checkPlaceholderFiles`, `freshnessCheckNotices`, `registerIntegrityProblems`,
  `darSyncProblems`, `attributionProblems`, `verifySectionProblems`, `unfailableRowNotices`,
  `linkProblems`, `registerRefProblems`, `qualityNotices`, `standingAlarmNotices`,
  `debtNotice`, intake-alarm checks)
- a NEW audit subcommand file, e.g. `statusgen/lintaudit.go` (planned) + `statusgen/lintaudit_test.go` (planned)
- `docs/streams/statusgen/README.md` — one convention line

facts:
- Each check returns `[]string` PROBLEMs and/or NOTICEs prefixed with a rule tag (e.g. the
  emitted strings already name their owning rule). The audit does NOT re-run 30 days of
  history against 30 days of tree state — that is unbounded. Instead: `--lint-audit` replays
  `git log --since=30.days` over `docs/streams/**` and `statusgen/**` touching commits,
  running `--lint` at each and tallying **per-rule PROBLEM/NOTICE firing counts** (keyed by a
  stable rule tag each check function is given). Cheap enough for a scheduled run (one lint per
  sampled commit; sample daily, not per-commit, to bound cost).
- Output: a compact table `rule | 30-day firings | gates-a-test?` sorted ascending by firings.
  A rule with 0 firings AND no referencing `_test.go` assertion is flagged
  `COLD — retirement candidate`. The audit only REPORTS — it never retires a rule itself
  (retirement is a human judgment; a cold rule may still be a correctly-quiet guardrail).
- To make per-rule tallying tractable, each check function's emitted strings must carry a
  stable, greppable rule tag (many already name their rule). Introduce a small `ruleTag`
  registry if needed; do NOT rewrite the checks' logic — only ensure their output is
  attributable.
- Runs read-only against a worktree/`git log`; no network, no ledger. Wave 1 (it reads git
  history via plain `git log`, available today).
- Out of scope: actually deleting any check (a separate, human-reviewed follow-up per COLD
  finding); wiring the audit into CI as a gate (it is advisory; a NOTICE at most).

## Ground rules
- NEVER git push to main / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: a fixture repo (or injected `git log`/lint-runner seam) with
   commits where a known rule fires N times and another fires zero; assert the audit tallies
   per-rule counts correctly and flags the zero-firing, no-test rule `COLD`.
2. Implement `--lint-audit` (daily-sampled 30-day window, per-rule firing tally, gates-a-test
   detection via grep over `_test.go`, ascending table, COLD flag). Add stable rule tags where
   a check's output is not already attributable.
3. README: one line under conventions — `--lint-audit` reports 30-day per-rule firing counts;
   COLD (0-firing, un-tested) rules are retirement candidates, retirement stays a human call.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run LintAudit -v` | exit 0; `TestLintAudit*` covers the per-rule tally + COLD-flag detection |
| 2 | `go test ./statusgen/ && go vet ./statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint-audit` | exit 0; prints a `rule \| firings \| gates-a-test?` table sorted ascending |
| 4 | `statusgen --root . --lint; echo $?` | 0 (the audit subcommand does not perturb ordinary lint) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling; a read-only advisory audit,
no rule is retired by this brief). Reviewer records verdict + date in the stream README table.
