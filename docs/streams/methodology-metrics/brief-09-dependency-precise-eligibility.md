---
brief: methodology-metrics/09
title: Dependency-precise Next-up eligibility — gate on typed depends, not whole-wave completion
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by Fable session (assay-review-1)
sources: ["docs/assay-review-1/README.md (B-05)", "tools/statusgen/nextup.go eligibility: a todo brief enters Next-up only when EVERY lower-wave brief in its stream is done/verified; the typed depends: list (validated by brieffile.go checkRef) is never consulted", "ledger-hardening/01 case: depends [ledger-hardening/03], wave 1, gate human — the sweep's top-severity open item, unschedulable behind five untouched wave-0 sibling todos", "[I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md) (scoring-v2 — composes with this; this brief changes ELIGIBILITY only, no score inputs)"]
---

# Brief 09 — Dependency-precise Next-up eligibility

## Context
files: tools/statusgen/nextup.go (+ model wiring as needed) + tests; docs/streams/methodology-metrics/README.md
facts:
- Today's rule (nextup.go `eligible`): `todo` is eligible only when every lower-wave brief in the
  SAME STREAM is `done` or `verified`. brief-v1's `depends:` is parsed, typed, and cross-checked at
  lint time — then ignored by the scheduler.
- Concrete cost: ledger-hardening/01 ("Intent strike = ratio × spot" — the needs-fixing sweep's
  headline, still open two days later) declares `depends: ["ledger-hardening/03"]`. The wave rule
  instead demands all ELEVEN wave-0 siblings reach done/verified — five of them untouched todos
  that nothing depends on. Even after 03 verifies, 01 stays unschedulable indefinitely. The board's
  own top-severity item cannot surface in Next-up; a human hand-pick is the only path, which is the
  failure mode Next-up exists to prevent.
- The bar itself is right and KEEPS: dependencies must be `done` or `verified` (never build on
  merely-implemented, unverified work). What changes is the SCOPE of the gate — this brief's own
  deps, not the whole wave.
- Legacy briefs (no brief-v1 frontmatter) have no typed deps — they keep the wave rule unchanged
  (checker changes never weaken; the brief-01 opt-in pattern).
- Boundary ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) / [I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md)): NO new score inputs here. Scoring-v2 (dependency priority-inheritance,
  type boosts, per-brief staleness) is [I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md)'s brief; this one only fixes which briefs are eligible.
  Cross-stream deps resolve exactly as checkRef does (`<stream>/<NN>`).

## Ground rules
- NEVER push to main / trigger workflows / run mutating kubectl. Feature-branch push + draft PR per
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop is the sanctioned flow; leave other commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. In `eligible`: for a `todo` brief that is brief-v1, gate on its OWN `depends:` — eligible iff
   every referenced brief (cross-stream) is `done` or `verified`; an empty `depends: []` means
   eligible now (that is what the author declared). Non-brief-v1 briefs keep the wave rule.
2. `in-progress` handling, stale-knowledge exclusion, caps, and scoring stay byte-identical.
   Note the interaction in a comment: mm/08 (claim-aware) filters in-flight; this changes todo
   gating only.
3. Tests: a fixture shaped like the ledger-hardening/01 case (wave-1 brief, dep verified,
   unrelated wave-0 todos present → eligible; same fixture with dep merely `implemented` →
   ineligible; legacy stream fixture unchanged).
4. Document the semantics change in this README ("wave = organization/rendering for brief-v1;
   `depends:` is the scheduler's gate") and in the methodology README tiering/conventions block if
   it references wave gating.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestEligib -v` | exit 0; subtests cover dep-verified/eligible, dep-implemented/ineligible, empty-depends/eligible, legacy-wave-rule-unchanged |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint` | exit 0 |
| 4 | regenerate locally (`statusgen --root .`, do NOT commit STATUS.md): | Next-up on the real tree includes ≥1 brief-v1 wave-1 brief whose typed deps are satisfied, where the old rule excluded it (record which, in Evidence) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestEligib -v` | 0 | PASS — dep-verified→eligible, dep-implemented→ineligible | 2026-07-10 | opus-verifier |
| 2 | `go test ./tools/statusgen/ && go vet` | 0 | ok | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | clean | 2026-07-10 | opus-verifier |
| 4 | regen — dep-satisfied wave≥1 brief now eligible | 0 | ledger-hardening/16 (wave 2, deps 04+05 both done) in Next-up despite 15 non-done siblings; also daml-hardening/02 | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — eligibility is per-dependency, not whole-wave; dep-satisfied later-wave briefs surface.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer checks the [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)/[I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md) boundary held: zero scoring changes in the diff.
