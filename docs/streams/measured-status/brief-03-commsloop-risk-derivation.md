---
brief: assay:assay:measured-status:03
title: "Derive the commsloop router's risk field from the envelope instead of hardcoding false, or record why false is sound — restore the risk:yes->tier:human backstop"
why: >-
  The inbound prose router pins `risk = false` on every message it assigns, so the mechanical
  `risk:yes`->`tier:human` rule in assign.yaml can never fire — the router's own prose-advisor
  action choice becomes the sole layer between a risk-shaped message and an unattended
  session-tier action. A safety backstop that is disabled by a hardcoded literal is the
  derive-not-assert seam at its most consequential: the value should come from the message, or
  its unconditional falseness should be derived and recorded.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1065]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
sources:
  - "#1065 — hardcoded risk=false in commsloop router, derivation needed"
  - "tools/desk/cmd/commsloop/loop.go — `const risk = false` passed into Assign(action, class, risk) for every routed item"
  - "tools/desk/cmd/commsloop assign.yaml — the dispatch-class rows whose tier splits on the risk axis (risk:yes forces tier:human)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the adjacent comment names this a follow-up wiring gap distinct from acp-guardrails/05's contract but derives no sufficiency and files no NEEDS_CONTEXT"
exec-tier: strong
exec-tier-why: re-arms a fail-closed routing backstop on live inbound; an error here either strands safe messages at tier:human or lets a risk-shaped message reach a session-tier action unattended
domain: complicated
value: high
version: 1
id: 3134b1cd-e373-43bd-a44e-dc48da4cb05b
---

# Brief 03 — Derive the commsloop router risk field

## Context
files:
- `tools/desk/cmd/commsloop/loop.go` — the `const risk = false` call site feeding `Assign`.
- `tools/desk/cmd/commsloop/assign.yaml` — the risk-axis dispatch rows (read to confirm the contract).
- `tools/desk/cmd/commsloop` tests — add coverage for a risk-shaped message routing to `TierHuman`.
facts:
- `assign.yaml`'s risk axis only ever NARROWS (a `risk:yes` row forces `tier:human`); it never
  grants more autonomy, so deriving a real risk signal can only make routing safer, never looser.
- the risk signal is "carried on the envelope as an INPUT to model resolution" per assign.yaml —
  i.e. the envelope is the intended source; today nothing populates it and the router pins false.
- the router already routes anything it cannot classify to escalate-human-issue/quarantine
  (fail-closed), so the derivation must preserve that fail-closed default when the envelope
  carries no risk signal.
- `tools/desk` is its own Go module; commsloop tests run from `tools/desk/`.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Replace `const risk = false` with a value DERIVED from the item's envelope: read the risk
   signal the envelope is specified to carry and pass it into `Assign`. When the envelope
   carries no risk signal, default to the fail-closed value the router already uses for the
   unclassifiable case (route risk-shaped-when-unknown toward tier:human / quarantine), not a
   silent `false`.
2. If wiring a real envelope signal is genuinely out of reach in this repo (the field does not
   yet exist anywhere upstream), then instead record a `// Derivation:` block at the call site
   proving why `false` is sound here — enumerate the classes of message the router sees and show
   each is covered by the router's own action choice — AND file the wiring as a tracked
   follow-up (`consumers:` `follow-up`), rather than leaving an underived literal. Under this
   option the literal `const risk = false` STAYS — the Derivation block is what justifies it,
   not a replacement for it — so mark this brief `implemented` with the follow-up issue linked
   in `## Evidence`, not as if the envelope wiring landed.
3. Which option applies determines which rows below are in scope; record the choice in
   `## Evidence` before filling the table.
   - **Option 1 (envelope wiring landed):** add a test asserting a risk-shaped envelope routes
     to `TierHuman` (the backstop fires) and an unknown-risk envelope fails closed rather than
     routing to a session tier. Rows 1, 2 and 4 apply; row 3 must show the literal gone.
   - **Option 2 (Derivation block + follow-up):** no new routing behaviour exists to test, so
     rows 1 and 2 are not applicable (record "N/A — Option 2 taken" against them, do not invent
     a test to force a pass). Row 3 must show the Derivation block present at the call site
     with the literal still `false`. Row 5 applies instead of rows 1/2.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./cmd/commsloop/ -run TestRouterRiskDerivedFromEnvelope -count=1` | Option 1: exit 0, output contains "ok". Option 2: N/A (record so in Evidence, do not force a pass) |
| 2 | `cd tools/desk && go test ./cmd/commsloop/ -run 'TestRouter.*Risk' -count=1 -v 2>&1 \| grep -q 'PASS'` | Option 1: exit 0 (the backstop-fires and fail-closed cases both ran and passed). Option 2: N/A |
| 3 | `cd tools/desk && ( ! grep -q 'const risk = false' cmd/commsloop/loop.go ) \|\| grep -q 'Derivation:' cmd/commsloop/loop.go` | exit 0 — Option 1: the hardcoded literal is gone. Option 2: the literal remains, but a `Derivation:` block justifies it at the call site |
| 4 | `cd tools/desk && go vet ./cmd/commsloop/` | exit 0 (both options) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:03` | Option 2 only: exit 0; output does not contain "DISPROVED" (the follow-up wiring issue recorded as a `consumers:` `follow-up` edge is corroborated, not contradicted) |

## Evidence
<!-- appended at implementation time by a non-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
