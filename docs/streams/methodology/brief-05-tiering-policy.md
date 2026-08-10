---
brief: methodology/05
title: Tiering as per-stream policy — overridable implement/verify model tiers
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["spec §11 adopt-5 (arXiv:2505.20182 — no universally optimal strong/weak strategy)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "[F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) (scope note)"]
---

# Brief 05 — Tiering as per-stream policy

## Context
files: tools/statusgen/{parse.go,model.go,emit.go} (+tests); .claude/skills/author-brief/SKILL.md; docs/streams/methodology/README.md
facts:
- Default policy: cheap-model implements, strong model/human verifies+reviews. **Refined by methodology/19 (risk-keyed verifier floor, [F-16](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verifier-tier-practice-contradicts-the-tiering-default-and-t.md)):** a **risk-clear** brief (all four risk answers `no`, `gate: model`) may be verified by a cheap/local-tier verifier — that is the majority path; a **risk-flagged** brief (`gate: human` OR any risk answer `yes`) must be verified by a human (or a runner outside the cheap-tier floor), enforced in `../assay-toolkit/statusgen/brieffile.go` and reflected in the verify-desk skill. "Strong model/human verifies" is the risk-flagged rung, not a blanket default.
- New optional stream frontmatter field `tiering:` — free-text policy override, e.g. `tiering: implement=sonnet verify=fable` or `tiering: implement=any` — statusgen parses, validates non-empty when present, and renders it in the roll-up Notes column so dispatchers see it.
- statusgen does NOT enforce which model ran (unknowable from artifacts today); this field is dispatcher guidance made visible, not a gate. Keep it that honest in the docs.
- Research basis: strategy dominance flips with budget; hard-coding one global rule is wrong.
- NOTED 2026-07-09 ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)): the red-team's Next-up scoring critique (`score = priority + staleness` has no value/effort/risk term) is deliberately NOT fixed here — scoring knobs are retro-owned, and the value/effort term sits on the R-01 agenda (methodology/08). Boundary this note pins: `tiering:` is dispatcher guidance rendered in Notes, never a Next-up score input — don't let the field grow a scoring role by drift.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `tiering` to the frontmatter struct + Stream model; surface in the roll-up Notes column (alongside external pointer, `·`-separated when both).
2. Document the field + default policy in the author-brief project wrapper and this stream README's conventions — AND (amended 2026-07-08, route-2, from the methodology/02-ran-at-Opus observation) the EXECUTION-side default: downleveling only happens via subagent dispatch (a session cannot change its own tier), so the rule is effort-keyed — S = inline at session tier OK; M/L = plan at your tier, dispatch implementation to cheap subagents behind gates. Brief-12 gates the up direction; this documents the down direction — INCLUDING the role→tier table (decided at the process desk 2026-07-08): **strong tier where decisions compound** (brief authoring, retros, disposition adjudication, whole-branch review — see also methodology/12's gate); **mid tier where judgment is item-wise and reviewable** (per-issue triage drafting, task-scoped review, independent verification); **cheap tier where the spec is complete** (transcription-grade implementation, data collection, mechanical sweeps). Rationale anchor: a wrong compounding decision costs a week; a wrong item-wise call costs one item.
3. TDD: stream with tiering renders in Notes; absent field renders nothing; empty-string value is a PROBLEM.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestTiering -v` | exit 0; ≥3 subtests PASS |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --check` | exit 0 |
| 4 | `grep -c "tiering" .claude/skills/author-brief/SKILL.md` | ≥1 |
| 5 | `grep -ci "effort-keyed\|inline at" .claude/skills/author-brief/SKILL.md` | ≥1 (execution-side rule documented; row added by 2026-07-08 amendment) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestTiering -v` | 0 | 7 `TestTiering/*` subtests PASS (frontmatter round-trip present, frontmatter round-trip absent→nil, renders in Notes when present, renders nothing when absent, · -separated from external pointer, empty string is a PROBLEM, empty string parses without error) | 2026-07-09 | implementer (sonnet) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-09 | implementer (sonnet) |
| 3 | `go run ./tools/statusgen --root . --lint` (substituted for `--check`, per brief-02/brief-04 precedent — `--check` cannot pass on a status-changing branch; STATUS.md has a single writer, methodology/15) | 0 | sources valid, view builds; STATUS.md never read or written | 2026-07-09 | implementer (sonnet) |
| 4 | `grep -c "tiering" .claude/skills/author-brief/SKILL.md` | 0 | `5` (≥1 required) | 2026-07-09 | implementer (sonnet) |
| 5 | `grep -ci "effort-keyed\|inline at" .claude/skills/author-brief/SKILL.md` | 0 | `3` (≥1 required) | 2026-07-09 | implementer (sonnet) |

Independent verification (non-implementer opus re-run on merged main 37c0eab2, 2026-07-09):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestTiering -v` | 0 | 8 `TestTiering/*` subtests PASS (implementer's 7 + the review-response whitespace-only case) | 2026-07-09 | independent (opus-verifier) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-09 | independent (opus-verifier) |
| 3 | `go run ./tools/statusgen --root . --check` | 0 | STATUS.md byte-matches sources on merged main — the row ran exactly as written (no `--lint` fallback needed off-branch) | 2026-07-09 | independent (opus-verifier) |
| 4 | `grep -c "tiering" .claude/skills/author-brief/SKILL.md` | 0 | `5` (≥1 required) | 2026-07-09 | independent (opus-verifier) |
| 5 | `grep -ci "effort-keyed\|inline at" .claude/skills/author-brief/SKILL.md` | 0 | `3` (≥1 required) | 2026-07-09 | independent (opus-verifier) |

Substance (source-level, beyond presence): [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) boundary holds — `tiering` is never
referenced in tools/statusgen/nextup.go (never a Next-up score input); checks.go flags
whitespace-only values as PROBLEMs; emit.go renders the value trimmed and `·`-separated
after any external pointer.

Review verdict (model:opus, 2026-07-09): **PASS — closed to `done`.** Confirmed gate:model
with all-no risk in frontmatter (closeable by a model). Reviewed the implementation against
the contract: `tiering` is a `*string` in parse.go/model.go (nil when absent, non-nil incl.
`""` when present — the tri-state the tests exercise); checks.go L49–50 flags a whitespace-
only value as a PROBLEM (task 3 "empty is a PROBLEM"); emit.go L64–68 renders it trimmed and
`·`-separated after any external pointer in the Notes column (task 1). The **[F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) boundary
holds** — `grep tiering tools/statusgen/nextup.go` = 0, so the field is never a Next-up score
input, exactly as the scope note requires. author-brief SKILL.md carries the field docs +
the role→tier table + the effort-keyed execution rule (5 hits). `TestTiering` green. No
defect; the field is honestly "dispatcher guidance made visible," not an enforced gate.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes.
