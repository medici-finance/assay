---
brief: methodology/37
title: CLAUDE.md word budget becomes a lint gate — statusgen --budget, wired into PR CI
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [280]
schema: brief-v1
authored: 2026-07-10 by Fable session (issue #280; methodology/14 implementer closing its own gap)
sources: ["issue #280 (verify FAIL: 2907 > 2850 on merged main)", "methodology/14 (the diet + the ≤2850 cap; its Verify row 1 is the invariant this mechanizes)", "the re-diet landing with this brief (2907 → 2836)", "freshness-checked 2026-07-10 @ 53012778"]
why: >-
  methodology/14 dieted CLAUDE.md under 2850 and the cap drifted back over within hours
  (brief-23's register docs re-grew it to 2907; second drift of the day). A one-shot brief
  cannot own a standing invariant — without a gate, every future PR that touches CLAUDE.md
  can silently breach the cap and the breach is only found at the next verify wave.
---

# Brief 37 — CLAUDE.md word budget as a lint gate

## Context
files: tools/statusgen/main.go (flag) + a small budget check (new file or checks.go) +
tests; .github/workflows/ (the PR lint invocation gains the budget arg — find the workflow
that runs `statusgen --lint` on PRs); docs/streams/methodology/README.md (row)
facts:
- statusgen is REPO-AGNOSTIC (stream convention: no medici paths, stdlib+yaml.v3). A
  hardcoded CLAUDE.md cap would violate that — so the knob is a repeatable flag:
  `--budget <path>:<maxwords>` (e.g. `--budget CLAUDE.md:2850`), checked only when passed.
  Word count = `len(strings.Fields(content))` (matches `wc -w`).
- Exceeded budget = hard PROBLEM naming the file, the count, the cap, and the rule owner
  ("methodology/14 cap — diet before merging; methodology/37, issue #280").
- Missing budgeted file = PROBLEM too (a deleted CLAUDE.md must not pass silently).
- Wiring: the PR CI lint step adds `--budget CLAUDE.md:2850`. The plain local `--lint`
  without the flag stays unchanged (agnostic default). Find the exact workflow file by
  grepping `.github/workflows/` for `--lint` — do not assume its name.
- Baseline at authoring: CLAUDE.md = 2836 words (re-dieted from 2907 in the same PR as
  this brief). Cap stays 2850 (methodology/14's number); headroom is the diet's job, not
  the gate's.
- Never weaken existing checks; new check is flag-opt-in (mirrors the schema-marker
  opt-in precedent from methodology/01).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD: table-driven tests — file under budget → clean; over budget → PROBLEM (message
   carries file, count, cap); budgeted file missing → PROBLEM; flag absent → no check;
   malformed flag value (`CLAUDE.md`, `CLAUDE.md:abc`) → usage error.
2. Implement the repeatable `--budget path:maxwords` flag + check.
3. Add `--budget CLAUDE.md:2850` to the PR CI lint invocation (workflow edit).
4. README row; lint green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Budget' -v` | exit 0; covers the five cases in Task 1 |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint --budget CLAUDE.md:2850; echo $?` | 0 (current file under cap) |
| 4 | `statusgen --root . --lint --budget CLAUDE.md:100 2>&1 \| grep -c PROBLEM` | ≥1 (gate provably fires) |
| 5 | `grep -c "budget CLAUDE.md:2850" .github/workflows/*.yml` | ≥1 (CI wired) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->
Verifier run (independent, non-implementer — opus-verifier, merged main `0174b912`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Budget' -v` | 0 | `TestParseBudgetSpec` (valid/path-with-dirs/missing-colon/non-integer) + `TestCheckBudget` (under/at-exactly/over→PROBLEM w/ count+cap/missing→PROBLEM/absent→no-check/malformed→usage-error) — all 5 Task-1 cases | 2026-07-12 | opus-verifier |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | `ok ...statusgen 1.606s`; vet clean | 2026-07-12 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint --budget CLAUDE.md:2850; echo $?` | 0 | NOTICEs only, no PROBLEM — CLAUDE.md under the 2850 cap | 2026-07-12 | opus-verifier |
| 4 | `... --budget CLAUDE.md:100 \| grep -c PROBLEM` | — | 1 (≥1) — gate fires when the cap is breached | 2026-07-12 | opus-verifier |
| 5 | `grep -c "budget CLAUDE.md:2850" .github/workflows/*.yml` | 0 | `statusgen.yml:1` (≥1) — CI wired in the PR-lint workflow | 2026-07-12 | opus-verifier |

**VERIFY: PASS** — `statusgen --budget` parses/enforces a word-cap (fires PROBLEM over-cap, silent under), and the 2850 CLAUDE.md budget is wired into CI.

## Review
Gate: model. Reviewer confirms the flag stays repo-agnostic (no hardcoded path in Go),
the CI wiring targets the PR-lint workflow actually in use, and no existing check weakened.
