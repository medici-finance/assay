---
brief: assay-product/02
title: Market analysis — the AI-native PM / agent-orchestration landscape
wave: 0
depends: []
unblocks: ["assay-product/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable session (human:<name>'s assay-product direction)
sources: ["[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md) (2026-07-08 prior-art sweep: spec-driven dev, BMAD, agentic-SDLC, AI-PM tooling — findings in the design spec §11)", "../reconciler/docs/market-analysis.md (the D2 shape to mirror)", "freshness-checked 2026-07-10 @ 78200803"]
---

# Brief 02 — Market analysis: the AI-native PM / agent-orchestration landscape

**CROSS-REPO:** the deliverable lands in `../assay-toolkit/docs/market-analysis.md`
(commit there, SHA in Evidence).

## Context
files: ../assay-toolkit/docs/market-analysis.md (new)
facts:
- Mirror the Plumb D2 shape (`../reconciler/docs/market-analysis.md`): segments, named
  competitors/adjacent tools, where the wedge is, what would make this obsolete, honest
  "why might nobody pay" section.
- Starting corpus (do NOT restart from zero): the 2026-07-08 deep-research sweep behind
  [I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md) — spec-driven dev, BMAD, agentic-SDLC frameworks, AI-PM tooling, commercial
  offerings; verified findings + adoption list live in
  `../oit/docs/superpowers/specs/2026-07-08-initiative-streams-design.md` §11. Refresh it with a
  2026-07 web pass (the field moves monthly): agent-orchestration boards, AI-native
  issue-trackers, eval/verification tooling for agent output.
- The differentiator hypothesis to test against the landscape: generated-board +
  registers + risk-gated lifecycle + evidence enforcement as ONE system (individually
  common, jointly unfound in the 07-08 sweep).
- Segments to cover at minimum: solo-operator agent fleets (today's regime), SME
  mixed human/agent teams ([I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md)), platform/marketplace angle ([I-27](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-admin-pod-as-deliberate-chokepoint-evidence-receipts-mischie.md) dogfood).
- [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) constraint applies to any numbers quoted from our own operation.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-toolkit commits local; push is human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Re-run the landscape pass (web research) against the §11 baseline; note what changed
   since 2026-07-08.
2. Write `../assay-toolkit/docs/market-analysis.md` per the D2 shape: segments, named
   players per segment, wedge analysis, pricing observations, obsolescence risks, and an
   honest adverse case.
3. Update the stream-README row.

## Verify (executable — presence gates; prose quality owned by the review gate)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-toolkit/docs/market-analysis.md && wc -w < ../assay-toolkit/docs/market-analysis.md` | ≥1500 |
| 2 | `grep -ciE "obsolet|adverse|why might nobody" ../assay-toolkit/docs/market-analysis.md` | ≥1 (adverse case present) |
| 3 | `grep -c "2026-07" ../assay-toolkit/docs/market-analysis.md` | ≥1 (dated refresh recorded) |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `c15dd509`, assay-toolkit origin/main `703b262`, 2026-07-18)

Deliverable confirmed on `medici-finance/assay-toolkit` origin/main `703b26228` at `../assay-toolkit/docs/market-analysis.md`
(3034 words) — isolated `/private/tmp` clone (shared checkout untouched).

| # | Command | Exit | Output |
|---|---------|------|--------|
| 1 | `test -f …/docs/market-analysis.md && wc -w` | 0 | 3034 words (≥1500) |
| 2 | `grep -ciE "obsolet\|adverse\|why might nobody"` | 0 | 13 (≥1) |
| 3 | `grep -c "2026-07"` | 0 | 3 (≥1) |
| 4 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

All 4 rows meet thresholds. `gate: model`, all four risks `no` → model flip permitted → `implemented → verified`.

## Review
Gate: model. Reviewer checks named-competitor claims carry sources/links and the adverse
case is real analysis, not a strawman.
