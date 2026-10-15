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
consumers:
  - "tools/desk/internal/comms/envelope.go (the cellmsg-v1 risk signal the router would derive from): follow-up #1722 (Option 2 taken — the field does not exist upstream and the strict parser refuses one, so the wiring is tracked as issue #1722; this branch touches no consumer path)"
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
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go test ./cmd/commsloop/ -run TestRouterRiskDerivedFromEnvelope -count=1` | Option 1: exit 0, output contains "ok". Option 2: N/A (record so in Evidence, do not force a pass) |
| 2 | check | `cd tools/desk && go test ./cmd/commsloop/ -run 'TestRouter.*Risk' -count=1 -v 2>&1 \| grep -q 'PASS'` | Option 1: exit 0 (the backstop-fires and fail-closed cases both ran and passed). Option 2: N/A |
| 3 | check +mutation | `cd tools/desk && ( ! grep -q 'const risk = false' cmd/commsloop/loop.go ) \|\| grep -q 'Derivation:' cmd/commsloop/loop.go` | exit 0 — Option 1: the hardcoded literal is gone. Option 2: the literal remains, but a `Derivation:` block justifies it at the call site. Mutation proof: the same command run against `origin/main`'s `loop.go` exits 1 (no `Derivation:` block there); at this branch's head it exits 0 — the guard reddens on main and greens here |
| 4 | check | `cd tools/desk && go vet ./cmd/commsloop/` | exit 0 (both options) |
| 5 | check | `statusgen --root . --consumers --brief assay:assay:measured-status:03` | Option 2 only: exit 0; output does not contain "DISPROVED" (the follow-up wiring issue recorded as a `consumers:` `follow-up` edge is corroborated, not contradicted) |

## Evidence
Option taken (implementer record, 2026-09-27): **Option 2 — Derivation block + tracked
follow-up.** The cellmsg-v1 envelope (`tools/desk/internal/comms/envelope.go`) carries no
risk field anywhere upstream and its strict parser (`DisallowUnknownFields`) refuses one,
so no sender can produce the signal assign.yaml's risk axis reads; wiring a real envelope
signal is genuinely out of reach in this change. The `// Derivation:` block at the call
site (`tools/desk/cmd/commsloop/loop.go`, above the retained `const risk = false`)
enumerates the seven classes of message the router can see and shows each is covered
without the risk axis; the residual gap (a mis-routed risk-shaped message reaching
TierSession) is filed as the tracked wiring follow-up, issue #1722.

Implementer's local runs of the applicable rows (the verifier re-runs; rows 1 and 2 are
N/A — Option 2 taken, no new routing behaviour exists to test, no pass forced):

| # | Result |
|---|--------|
| 1 | N/A — Option 2 taken |
| 2 | N/A — Option 2 taken |
| 3 | PASS — `Derivation:` present at the call site, literal `const risk = false` retained. Mutation proof re-run against `origin/main` (no `Derivation:` block there): exit 1. Re-run at this branch's head: exit 0 |
| 4 | PASS — `go vet ./cmd/commsloop/` exit 0 |
| 5 | PASS — exit 0, no DISPROVED (the follow-up edge reports UNCHECKED: it names issue #1722, not a stream/NN brief — corroborated as not-contradicted) |

Review follow-up (2026-09-27): both reviewer findings from the first review round are
addressed in this push. The Derivation block's closing sentence, which named the class-6
consult-failure default as the layer behind the retained literal, is corrected — class 6
only fires when the consult itself fails, not when a successful consult mis-routes, so it
cannot be that backstop. The block now names the controls that actually apply: Dispatch's
own `Native` zero-value (no session fired, mailbox delivery only) today, and, if `Native` is
ever enabled, the kill switch and role-fenced session. The Verify table above gained a
`Class` column with row 3 marked `+mutation`, with the red-then-green proof recorded in
Evidence. The hand-edited board row in the stream README (row 03's Status cell) is reverted
to `todo` — the generated table is statusgen's to write, and the PR body's claim of a row
flip is dropped in the same push.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/commsloop/ -run TestRouterRiskDerivedFromEnvelope -count=1` | pass exit=0 | sha256:96cda6b08f51 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/commsloop/ -run 'TestRouter.*Risk' -count=1 -v 2>&1 \| grep -q 'PASS'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && ( ! grep -q 'const risk = false' cmd/commsloop/loop.go ) \|\| grep -q 'Derivation:' cmd/commsloop/loop.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go vet ./cmd/commsloop/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:03` | fail exit=2 | sha256:16391eade42f | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |

### Independent verification — 2026-10-02 (non-implementer, merged main 6e419774479f)

Runner for every row below: assay-verifier-app[bot] @ 6e419774479f (claude-opus-5-5, on-behalf-of human:ian). The table above is the execution witness from the same run; this block is the hand reading of each row. Main carries Option 2 (literal kept, `Derivation:` block at the call site, follow-up issue #1722 recorded as a `consumers:` edge).

| Verify row | Command | Exit | Observed | Reading |
|---|---|---|---|---|
| 1 | `cd tools/desk && go test ./cmd/commsloop/ -run TestRouterRiskDerivedFromEnvelope -count=1` | 0 | `ok … [no tests to run]` | N/A under Option 2, as the row's Expect states. The exit 0 is vacuous (no matching test exists) and is not counted as a pass. |
| 2 | `cd tools/desk && go test ./cmd/commsloop/ -run 'TestRouter.*Risk' -count=1 -v 2>&1 \| grep -q 'PASS'` | 0 in a plain shell, 141 under pipefail | `testing: warning: no tests to run` then `PASS` | N/A under Option 2. The witness scores it fail (exit 141: `grep -q` closes the pipe early under pipefail); the row is not option-aware. |
| 3 | `cd tools/desk && ( ! grep -q 'const risk = false' cmd/commsloop/loop.go ) \|\| grep -q 'Derivation:' cmd/commsloop/loop.go` | 0 | literal at `loop.go` line 302, `Derivation:` block at lines 244–301, directly above the `Assign` call at 303 | PASS. Mutation check: the same command against `loop.go` from the parent of the implementing commit exits 1. |
| 4 | `cd tools/desk && go vet ./cmd/commsloop/` | 0 | no output | PASS (non-discriminating: it passes without the work). |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:03` | 2 | the tool reports the brief `is not in the diff against 6e41977…, so this run carries no evidence about its claims` | FAIL as written. On merged main the default base is main itself, so the row has no diff to read. Diagnostic only, not the row: with `--base` set to the parent of the implementing commit it exits 0 with `0 corroborated, 0 disproved, 1 unchecked`. |

Result: 2 of 3 applicable rows pass (rows 1 and 2 are N/A under Option 2); row 5 fails as written. Status stays `implemented`. The row-5 shape is the class tracked in #1915; rows 1, 2 and 5 need a Verify-table re-baseline before the witness can read green.

Derivation block checked by hand against the tree: the envelope carries no risk field and the parser refuses unknown fields (`envelope.go` line 149); every `risk: "yes"` row of `assign.yaml` is tier human; `Native` is declared and read in `loop.go` but assigned nowhere outside tests.

RISK-VALUE: DERIVED — `risk = false` at `tools/desk/cmd/commsloop/loop.go:302`. No sender can express a risk signal today, so `false` is what an envelope-derived read would yield; the assign table's risk axis only narrows. Conditional: this stops holding if `Native` is enabled for the router before #1722 lands.
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at c690023a864b (Option 2 holds; rows 3–4 pass, rows 1–2 N/A, row 5 could-not-check as written)

Verification-Attestation: medici-finance/assay#2931 run=116f1b6e90378ea372ea249a7d5a189605778ed553ebd0bdb7e6ad84c919f335 source=c690023a864b1c1cfdd55c0a28e19d22e572048d brief=docs/streams/measured-status/brief-03-commsloop-risk-derivation.md model=claude-opus-5-5 tier=strong

Fresh non-implementer pass on merged main c690023a864b, admitted by the dispatcher check (exit 0, binding above). Main still carries Option 2: the literal `const risk = false` stays at tools/desk/cmd/commsloop/loop.go line 302, the `Derivation:` block covers lines 244 to 301, and the follow-up wiring issue #1722 is still open and recorded as a `consumers:` edge. loop.go, assign.yaml and the comms envelope source are unchanged from the implementing commit 0842782a4b09 to c690023a864b. That commit changed only comment lines in loop.go. Checked by hand at c690023a864b: the cellmsg-v1 Envelope struct has no risk field, and ParseEnvelope refuses unknown fields (envelope.go line 149). All 14 `risk: "yes"` rows in assign.yaml are `tier: human`. The commsloop `Loop.Native` field (loop.go line 71) is not set to true anywhere outside tests. Rows 1 and 2 are N/A under Option 2, as the brief's Expect cells say. Their exit 0 is vacuous (no test matches), so it does not count as a pass. Row 5 as written runs against merged main with no `--base`. The tool reports COULD-NOT-CHECK, and that is not a pass. This is the check-definition class tracked in #1915. As a diagnostic only, not the row: with `--base` set to the parent of the implementing commit, it exits 0 with `0 corroborated, 0 disproved, 1 unchecked`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/commsloop/ -run TestRouterRiskDerivedFromEnvelope -count=1` | N/A — Option 2 taken (exit=0, vacuous) | `ok github.com/medici-finance/assay/tools/desk/cmd/commsloop 0.177s [no tests to run]` — no test named Test Router Risk Derived From Envelope exists; not counted as a pass | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./cmd/commsloop/ -run 'TestRouter.*Risk' -count=1 -v 2>&1 \| grep -q 'PASS'` | N/A — Option 2 taken (exit=0 plain shell; exit=141 under pipefail) | the `-v` run prints `testing: warning: no tests to run` then `PASS`; under `bash -o pipefail` the pipeline exits 141 because `grep -q` closes the pipe early (same shape as #2241) | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && ( ! grep -q 'const risk = false' cmd/commsloop/loop.go ) \|\| grep -q 'Derivation:' cmd/commsloop/loop.go` | pass exit=0 | literal at loop.go line 302 and `Derivation:` block at lines 244 to 301, directly above the `Assign` call at line 303. Mutation: the same predicate against loop.go from 0842782a4b09^ (literal at line 253, no `Derivation:`) exits 1, so the check fails on the parent and passes at this commit | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && go vet ./cmd/commsloop/` | pass exit=0 | no output | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:03` | could-not-check exit=2 | statusgen v1.0.42: `--consumers: COULD-NOT-CHECK: assay:assay:measured-status:03 is not in the diff against c690023a864b…, so this run carries no evidence about its claims`. Diagnostic only, not the row: `--base 9509504ef56b` (parent of 0842782a4b09) exits 0 with `0 corroborated, 0 disproved, 1 unchecked` (the follow-up #1722 edge names no stream/NN target). Class tracked in #1915 | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |

Enumeration (risk-bearing values). The implementing commit 0842782a4b09 changed only comment lines in loop.go, so it introduced or changed no literal. The values the item's Deliverables name are: the `risk` constant (loop.go line 302), the `risk: "yes"` to `tier: human` rows of assign.yaml (14 rows, lines 38 to 75), and the `Native` zero value that bounds the residual gap (loop.go line 71: a declared bool, assigned nowhere outside tests). The Derivation block depends on that zero value. Ranked by blast radius: the `risk` constant first, the assign.yaml rows second. All of them are reversible by an edit and a redeploy, and the brief's frontmatter has `irreversible: no`.

RISK-VALUE: DERIVED — risk = false @ tools/desk/cmd/commsloop/loop.go:302 — no sender can express a risk signal today. The cellmsg-v1 Envelope struct has no risk field, and ParseEnvelope's DisallowUnknownFields (envelope.go:149) refuses one, so false is exactly what an envelope-derived read would return. assign.yaml's risk axis only narrows (every risk:"yes" row is tier human), so false cannot grant more autonomy than the router's action choice already gives. This is conditional: it stops holding if commsloop's Loop.Native is enabled before #1722 lands. Native is still false in all non-test code at c690023a864b.

RISK-VALUE: DERIVED — risk:"yes" rows tier = human @ tools/desk/cmd/commsloop/assign.yaml:38 (all 14 rows, lines 38 to 75) — this is the standing convention that any risk:yes goes to a human gate, as stated in the table's own header (lines 17 to 20). This item left the rows unchanged, and grep confirms no risk:"yes" row maps to any other tier.

**VERIFY: BLOCKED** — rows 3 and 4 pass and rows 1 and 2 are N/A (Option 2). Row 5 is could-not-check as written: post-merge `--consumers --brief` with no `--base` (check-definition class #1915). The diagnostic with `--base` shows 0 disproved. The Verify table needs a re-baseline before row 5 can pass. Status stays `implemented`.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
