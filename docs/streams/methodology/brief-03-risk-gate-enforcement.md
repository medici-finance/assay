---
brief: methodology/03
title: Risk-gate enforcement — human-gated briefs need a recorded human reviewer
wave: 1
depends: ["methodology/01"]
unblocks: ["methodology/07"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["spec §11 adopt-2 (GAIE risk routing, arXiv:2606.22484)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)"]
---

# Brief 03 — Risk-gate enforcement at the done gate

## Context
files: tools/statusgen/{brieffile.go,checks.go} (+tests)
facts:
- brief-v1 frontmatter carries `gate: model|human` derived from four risk answers; nothing enforces it at `done`.
- Enforcement rules: (a) frontmatter self-consistency — any risk answer yes with gate: model is a PROBLEM (authoring error); (b) README status `done` + gate: human requires the Reviewed column to name a human — convention: the value must contain `human:` (e.g. `2026-07-15 human:ian`); a bare model sign-off on a human-gated brief is a PROBLEM.
- This is the regulated-finance rationale: settlement/funds/identity/prod briefs answer yes somewhere → a person closes them, mechanically.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add both checks to statusgen (gate/risk self-consistency likely already in brief-01 — extend to the done/Reviewed cross-check).
2. Document the `human:<name>` Reviewed-column convention in the author-brief project wrapper (.claude/skills/author-brief/SKILL.md) and this stream's README shared conventions.
3. TDD fixtures: done+human-gate+`human:x` (pass), done+human-gate+model sign-off (fail), done+model-gate (pass), risk-yes+gate-model (fail).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestRiskGate -v` | exit 0; ≥4 subtests PASS |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint` | exit 0 (the PR gate under the single-writer model, methodology/15; `--check` is main's byte-compare and cannot pass on a status-changing branch) |
| 4 | `grep -c "human:" .claude/skills/author-brief/SKILL.md` | ≥1 (convention documented) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestRiskGate -v` | 0 | 5 subtests PASS (human-done+human ok; human-done+model-only fail; model-done exempt; risk-yes+gate-model fail; superhuman-tag fail) | 2026-07-08 | implementer (Opus 4.8) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-08 | implementer (Opus 4.8) |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | clean; no real brief is done+gate:human yet (07/09/10/11/13 all todo) | 2026-07-08 | implementer (Opus 4.8) |
| 4 | `grep -c "human:" .claude/skills/author-brief/SKILL.md` | — | 2 (human:<name> convention documented in wrapper + methodology README) | 2026-07-08 | implementer (Opus 4.8) |

Note: part (a) (risk-yes + gate:model self-consistency) was already implemented in
brief-01; this brief adds part (b) — `done` + `gate: human` requires a `human:` reviewer.
No STATUS.md commit (single-writer model, methodology/15).

Independent verification (non-implementer re-run on merged main `a4e3e04b`, post-#100):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestRiskGate -v` | 0 | 5 subtests PASS (human-done+human ok; human-done+model-only fail; model-done exempt; risk-yes+gate-model fail; `superhuman:` tag rejected) — ≥4 required | 2026-07-09 | independent (Fable, did not implement 03) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok (incl. post-#100 demotion tests); vet clean | 2026-07-09 | independent (Fable) |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | clean on merged main; no done+gate:human brief exists yet so the check is armed but silent | 2026-07-09 | independent (Fable) |
| 4 | `grep -c "human:" .claude/skills/author-brief/SKILL.md` | 0 | 2 (≥1 required; convention documented) | 2026-07-09 | independent (Fable) |

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes.
