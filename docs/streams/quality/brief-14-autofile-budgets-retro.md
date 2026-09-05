---
brief: quality/14
title: closing the loop — auto-filed refactor work + quality error-budgets + RETRO output feed
wave: 5
why: >-
  Metrics that no one acts on decay into a dashboard nobody reads. This brief closes the
  loop: duplicate-block clusters and decaying hotspots above threshold file advisory,
  budgeted refactor work; per-stream churn/defect budgets raise an alarm when breached; and
  the mined trends feed a cadence retrospective. The tool never self-dispatches — it makes
  the quality signal actionable while a human or intake process stays in the loop.
depends: ["quality/03", "quality/04", "quality/07", "quality/12"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §9.5 — auto-filed refactor work (pluggable issue-filer, advisory + budgeted)"
  - "docs/streams/quality/spec.md §9.6 — quality error-budgets (alarm posture, config after ≥2 windows)"
  - "docs/streams/quality/spec.md §9.7 — retrospective inputs (generated/logged only)"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant; §10 honest-claims"
---

# Brief 14 — auto-filed refactor + error-budgets + RETRO feed

## Context

files:
- NEW `qualgen/consumers/autofile.go` (planned) (+ `autofile_test.go`) — turns
  duplicate-block clusters (quality/03) and decaying hotspots above threshold (quality/03)
  into refactor issues/briefs, through a pluggable issue-filer; advisory and budgeted;
  NEVER self-dispatching.
- NEW `qualgen/filer/filer.go` (planned) — the pluggable `IssueFiler` interface: file a
  refactor item (title, body, labels) OR dry-run (return what WOULD be filed without
  filing). Supports a budget cap the caller enforces.
- NEW `qualgen/filer/githubadapter.go` (planned) (+ `githubadapter_test.go`) — the GitHub
  Issues REFERENCE adapter shipped in-tree; other filers are configuration, not a fork.
- NEW `qualgen/consumers/budgets.go` (planned) (+ `budgets_test.go`) — per-stream churn and
  defect-inducing error-budgets in an ALARM posture (a breach is an alarm, not a dashboard
  line); budgets are config, armable only after ≥2 windows of measurement.
- NEW `qualgen/consumers/retro.go` (planned) (+ `retro_test.go`) — emits the retrospective
  input set (churn trend, gate yield, per-stage ledger, budget status) as generated/logged
  output a cadence retrospective consumes; this tool produces the inputs, it does not run
  the retro.
- CONSUMES hotspots + duplicate-block clusters (quality/03), instruction-layer brittleness
  (quality/04), M2 defect metrics (quality/07), and gate-yield (quality/12) for the retro
  feed.

facts:
- **Advisory + budgeted, never self-dispatching** (spec §9.5): the tool FILES an item for a
  human/intake process to triage — it never assigns, starts, or dispatches work. Filing is
  rate-budgeted; over budget, it degrades to dry-run/logged, not unbounded filing.
- **Pluggable issue-filer + GitHub reference adapter** (spec §9.5): the interface ships
  here with one generic reference adapter; a dry-run mode is first-class so filing can be
  proven without writing to any tracker.
- **Budgets armable only after ≥2 windows** (spec §9.6): a budget cannot be set/enforced
  until the stream has ≥2 measured windows — before that the budget is `could-not-measure`,
  not zero (three-state, spec §3.2).
- **Retro inputs are generated/logged only** (spec §9.7): retro.go EMITS the input set; it
  does not schedule, run, or narrate a retrospective.
- **Preconditions.** Wave 5: needs a SEASONED M1–M3 corpus (spec §11) — thresholds,
  clusters, and budgets are only meaningful on ≥2 windows of measurement. The code is
  testable against fixtures without a live seasoned corpus.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- The tool NEVER self-dispatches work and NEVER files outside its budget; the GitHub
  reference adapter MUST support a dry-run that files nothing. If anything is unclear or
  contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Define `IssueFiler` in `qualgen/filer/filer.go` (planned): file(item) + a dry-run mode returning
   the composed item without side effects, plus a budget cap the caller checks. Implement
   the GitHub Issues reference adapter in `githubadapter.go` behind that interface.
2. Implement `autofile.go`: from quality/03 duplicate-block clusters and
   decaying-hotspot-above-threshold input, compose ONE refactor item per distinct cluster
   / per distinct above-threshold hotspot (deduped by target path), route through the
   `IssueFiler`, enforce the budget (over budget → dry-run/logged), and mark every item
   advisory. Never dispatch.
3. Implement `budgets.go`: per-stream churn and defect-inducing budgets; refuse to
   arm/enforce a budget when the stream has <2 measured windows (emit could-not-measure);
   on a breach emit an ALARM record, not a dashboard line.
4. Implement `retro.go`: emit the four-part retrospective input set (churn trend, gate
   yield, per-stage ledger, budget status) as generated/logged output only.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./consumers/ ./filer/` | exit 0 |
| 2 | `cd qualgen && go test ./consumers/ ./filer/` | exit 0; autofile, budgets, retro, and reference-filer tests pass |
| 3 (DEREFERENCE) | `cd qualgen && go test ./consumers/ -run TestAutofile_AboveThresholdFilesExactlyOne -v` | exit 0 — fixture: a hotspot above threshold routed through the GitHub reference filer in DRY-RUN produces EXACTLY ONE item (count == 1, asserted), and its composed body references that hotspot's file path (proves the filer dereferenced the right hotspot, not merely that some item was produced) |
| 4 (budget/negative) | `cd qualgen && go test ./consumers/ -run TestAutofile_OverBudgetDegradesToDryRun -v && go test ./consumers/ -run TestBudgets_RefusesUnderTwoWindows -v` | exit 0 — over-budget filing degrades to dry-run/logged (nothing filed); a budget with <2 measured windows is refused as could-not-measure, never armed at zero |
| 5 | `cd qualgen && go test ./consumers/ -run TestRetro_EmitsFourInputSet -v && grep -c -e churn -e 'gate.yield' -e ledger -e budget consumers/retro.go` | exit 0; retro output carries all four inputs (churn trend, gate yield, per-stage ledger, budget status); count ≥ 4 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — VERIFY: PASS — 2026-09-04 opus-4.8[1m]-verifier (verify-desk dispatch), merged main 4e500df

Runner != implementer. Offline (KUBECONFIG=/dev/null). gate: model, risk {all no}, irreversible: no.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | cd qualgen && go build ./... && go vet ./consumers/ ./filer/ | 0 | clean build + vet, no output | 2026-09-04 | opus-4.8[1m]-verifier |
| 2 | cd qualgen && go test ./consumers/ ./filer/ | 0 | ok consumers 0.826s; ok filer 1.603s | 2026-09-04 | opus-4.8[1m]-verifier |
| 3 | cd qualgen && go test ./consumers/ -run TestAutofile_AboveThresholdFilesExactlyOne -v | 0 | PASS — exactly one item filed, body references the hotspot path | 2026-09-04 | opus-4.8[1m]-verifier |
| 4 | cd qualgen && go test ./consumers/ -run TestAutofile_OverBudgetDegradesToDryRun -v && go test ./consumers/ -run TestBudgets_RefusesUnderTwoWindows -v | 0 | both PASS — over-budget degrades to dry-run; under two windows refused | 2026-09-04 | opus-4.8[1m]-verifier |
| 5 | cd qualgen && go test ./consumers/ -run TestRetro_EmitsFourInputSet -v && grep -c -e churn -e gate.yield -e ledger -e budget consumers/retro.go | 0 | PASS; grep count 15 (>=4) | 2026-09-04 | opus-4.8[1m]-verifier |

**VERIFY: PASS** — all 5 Verify rows offline-clean; none unrun.

**RISK-VALUE: DERIVED** — MinWindowsToArm = 2 @ qualgen/consumers/budgets.go:19 — spec 9.6 pins "armable only after >=2 windows": a budget alarm compares an observed window against a ceiling meaningless without history, so two (a prior + current window) is the least history under which it is honest; arming at 1 or 0 false-alarms on the first measurement. Code const matches the spec value exactly.

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go consuming already-mined
artifacts; the issue-filer is pluggable with a dry-run reference adapter, filing is
advisory + budgeted, and the tool never self-dispatches work). Reviewer confirms the
advisory/budgeted/never-self-dispatch posture holds and the budget refuses to arm under 2
windows, then records verdict + date in the stream README table.
