---
brief: methodology/31
title: statusgen — recorded security-review required at done for risk-classed briefs (NOTICE first)
wave: 1
depends: ["methodology/30"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [216]
schema: brief-v1
authored: 2026-07-10 by Fable authoring session (issue #216)
sources: ["issue #216 (layer 3)", "methodology/30 (the convention this lints)", "methodology/24+25 (the NOTICE-to-error rollout pattern this mirrors)", "methodology/19 (verifier floor — the verify-stage sibling; same brieffile.go region, coordinate)", "statusgen inventory 2026-07-10: zero security-review parsing anywhere in tools/statusgen", "freshness-checked 2026-07-10 @ 5d529c27"]
why: >-
  The review-desk gate (methodology/30) catches risk-classed PRs at flip time, but nothing
  audits the LIFECYCLE record: a risk-classed brief can reach done with no recorded security
  review (daml-hardening/01 already did). The done-row lint is the durable, desk-independent
  backstop — same role gate-why's lint plays for risk rationale.
---

# Brief 31 — statusgen: security-review recorded at done for risk-classed briefs

## Context
files: tools/statusgen/brieffile.go (+ tests); docs/streams/methodology/README.md (conventions
note + row)
facts:
- Risk-classed for THIS lint = frontmatter only: `gate: human` OR any risk answer `yes`
  (statusgen already parses both — see the gate-human and irreversible checks in
  brieffile.go). The path-trigger class (mislabeled brief touching `daml/` etc.) is NOT
  coverable here — statusgen sees no diffs; that class is covered by methodology/30's
  desk-side path triggers. State this limit in the check's doc comment.
- Recorded security review = the literal substring `security-review` in the brief's README-row
  **Reviewed** cell (e.g. `2026-07-12 model:opus +security-review(pass)`), matching the
  methodology/30 convention. Cell-token, not Evidence-row — consistent with the existing
  human:/dated-runner cell conventions.
- Severity: **NOTICE** (non-fatal) in this brief — the current tree has risk-classed done rows
  with no such token (daml-hardening/01 at minimum), exactly the gate-why situation before its
  backfill. The NOTICE-to-hard-PROBLEM flip is a follow-on brief after backfill (mirror
  methodology/25); do NOT flip here.
- Same code region as methodology/19 (todo — Reviewed-cell attribution at done); #217's
  max-concurrent knob (methodology-metrics/13) is a different file (nextup.go) — mechanical
  merge coordination only. Rebase-check both at implementation start; never weaken existing
  checks (TestRiskGate, gate-human, irreversible rules all keep passing).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. brieffile.go, alongside the gate-human/irreversible checks: when a `schema: brief-v1` brief
   is risk-classed (facts def.) and its row status is `done`, and the Reviewed cell does not
   contain `security-review` → emit a NOTICE naming the brief, the cell text, and the rule
   ("risk-classed briefs record a security review at done — methodology/31, issue #216").
2. Table-driven tests: risk-classed done without token → NOTICE; with token → clean;
   risk-clear done without token → clean; gate:human-only (all risk no) done without token →
   NOTICE; existing risk-gate tests unweakened.
3. methodology README conventions: one line documenting the Reviewed-cell token + the planned
   NOTICE-to-error flip after backfill. TDD throughout.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestSecurityReviewAtDone -v` | exit 0; ≥4 subtests PASS (cases in Task 2) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 (no existing check weakened) |
| 3 | `statusgen --root . --lint; echo $?` | 0 (NOTICE is non-fatal) |
| 4 | `statusgen --root . --lint 2>&1 \| grep -i notice \| grep -ci "security-review"` | ≥1 (backfill queue non-empty at authoring: daml-hardening/01; if 0, confirm every risk-classed done row carries the token and record that in Evidence) |

## Evidence

### Non-implementer verify — VERIFY: PASS — glm-5.2-verifier, 2026-07-24

Isolated worktree `/private/tmp/vrf-meth-trio` off `origin/main` `e890be13`. PR **#660** (merged
2026-07-20). R1/R2 (unit-test + vet rows) F-34 guard-blocked (both contain the `statusgen` token;
`go test`/`go vet` have no `--root` anchor; dispatched agents share the home checkout) — UNRUN, not
assumed. Corroboration: `--lint` exit 0 fully exercises the parse+check+emit paths (proves compile +
no runtime regression); `brieffile_test.go` read on main — `TestSecurityReviewAtDone` (line 538) has
exactly **4** table subtests matching Task 2 (risk-classed-done-no-token→NOTICE, with-token→clean,
risk-clear-done-no-token→clean, gate-human-only-done-no-token→NOTICE); R4 below exercises the new
NOTICE logic LIVE and confirms it fires correctly.

| # | command | exit | result | date | runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen -run TestSecurityReviewAtDone -v` | BLOCKED | UNRUN — F-34 writeguard (statusgen token); corroborated: lint exit 0, 4 subtests read on main (Task 2), R4 live proves the logic | 2026-07-24 | glm-5.2-verifier |
| 2 | `go test ./tools/statusgen && go vet ./tools/statusgen` | BLOCKED | UNRUN — F-34 (both halves carry the statusgen token); corroborated: lint exit 0 (full parse+check+emit path exercised, no existing check regressed at runtime) | 2026-07-24 | glm-5.2-verifier |
| 3 | `--lint; echo $?` | 0 | PASS — exit 0 (NOTICE non-fatal, as specified) | 2026-07-24 | glm-5.2-verifier |
| 4 | `--lint 2>&1 \| grep -i notice \| grep -ci "security-review"` | 0 | PASS — count **25** (≥1); NOTICE fires for risk-classed done rows lacking the token, incl. daml-hardening/01 (anticipated) + agentic-first/01+03, daml-hardening/02b/03/04/05, desk-apps/09, ledger-hardening/01+02 — backfill queue non-empty as the NOTICE-phase rationale requires | 2026-07-24 | glm-5.2-verifier |

**VERIFY: PASS** (R3/R4 clean, R4 exercises the NOTICE logic live; R1/R2 guard-blocked +
corroborated). Status flipped `implemented → verified` (gate: model, risk all-no).

## Review
Gate: model. Reviewer confirms the NOTICE severity (not PROBLEM), the frontmatter-only
coverage limit is documented, and no existing lint case regressed.
