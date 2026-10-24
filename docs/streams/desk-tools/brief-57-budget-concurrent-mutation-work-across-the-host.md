---
brief: assay:assay:desk-tools:57
title: Budget concurrent mutation work across the host
why: "Independent verify and review runs each size their mutation workers as if they own the machine, causing avoidable contention and misleading timeouts. A shared lease budget will bound total mutation work while preserving each run's complete mutation set."
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-24 by maintainer-directed mutation review
sources:
  - medici-finance/assay#4359
  - tools/desk/cmd/muhar/main.go
  - freshness-checked 2026-10-24 @ 61eb62a4a (origin/main)
exec-tier: strong
exec-tier-why: "Cross-process locking, cancellation and launcher propagation require concurrency and lifecycle reasoning across components."
domain: complex
consumers:
  - "tools/desk/cmd/muhar/README.md: follow-up desk-tools/57 (this brief; flips to fixed-here when the lease contract is documented)"
  - "tools/desk/cmd/cellctl/env.go: follow-up desk-tools/57 (this brief; flips to fixed-here when launcher propagation lands)"
  - "docs/cellctl.md: follow-up desk-tools/57 (this brief; flips to fixed-here when operator configuration is documented)"
---

# Brief 57 — Budget concurrent mutation work across the host

## Context
files: tools/desk/cmd/muhar/main.go, tools/desk/cmd/muhar/budget.go, tools/desk/cmd/muhar/budget_test.go, tools/desk/cmd/muhar/README.md, tools/desk/cmd/cellctl/env.go, tools/desk/cmd/cellctl/budget_test.go, tools/desk/cmd/cellctl/envkeys_test.go, docs/cellctl.md, changelog/muhar-host-budget.md
facts:
- On 2026-10-24 three independent muhar processes and a full Go review suite overlapped on one 16-CPU host; per-process -j limits did not bound their sum.
- Existing -j 0 caps one process at two workers; it is not a host-wide budget.
- The lease must bound mutation count, not change which mutations run. Every run still executes its baseline, control and complete spec.
- Containerised runs are covered only when they explicitly share the lease directory and locking semantics; otherwise the configuration is could-not-check, not a claim of host coverage.

layering: Add one small lease owner used by muhar, with pure slot-count policy separated from OS lock effects. cellctl only propagates reviewed configuration; it does not become a second lock manager or scheduler.
design-fit:
  owner: muhar worker admission and cellctl's existing environment propagation
  contract: none — local lease files stay inside the existing tool boundary
  retires: [independent unbounded mutation worker pools on one host]
  weight: "Two opt-in environment values and invalid-config refusals; no new command or daemon."
  why-add: "A shared lease directory and total slot count are the minimum cross-process inputs; per-run flags alone cannot express a machine-wide limit."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not evade a held lease by starting additional workers, using a second identity, or silently selecting a different lease directory.

## Task
1. Add opt-in `MUHAR_LEASE_DIR` and `MUHAR_TOTAL_SLOTS`. When both are set, each run acquires one OS-backed slot lease per resolved worker before baseline/control work and holds it for that worker's lifetime. Unset configuration preserves current behaviour; partial, non-positive, or unreadable configuration refuses loudly.
2. Bound the sum of workers across independent processes, including -j 0 and explicit -j N. Waiting for a lease is observable in stderr and distinct from build/test duration. Never shard, skip, or reorder mutations to fit the budget.
3. Use the existing cross-platform exclusive-lock primitives and automatic close-on-exit release. Kill process groups and release every lease on normal return, harness failure, SIGINT and SIGTERM. Refuse multiple owners of one slot; stale files alone must not prove a live holder.
4. Extend cellctl with reviewed per-cell propagation of the two values. Document how independent roles share one host lease directory and total, and the explicit limitation for containers without a shared lock filesystem. Do not set a new default for existing cells.
5. Add `TestBudgetBoundsTotal`, `TestBudgetReleases`, and `TestBudgetBadConfig` using two independent processes and a fixture that records peak concurrent work. Add `TestMuharBudgetEnv` proving cellctl propagates only explicitly configured values.
6. Document queue-wait measurement and operator recovery. Change all three consumer entries to `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestBudget' -count=1 -v > "${TMPDIR:-/tmp}/dt57-budget.log" 2>&1 && grep -F -- '--- PASS: TestBudgetBoundsTotal ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetReleases ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetBadConfig ' "${TMPDIR:-/tmp}/dt57-budget.log"` | exit 0; all three named PASS lines; two processes never exceed the configured total and cancellation releases every slot | check:ci +mutation +flow |
| 2 | `cd tools/desk && go test ./cmd/cellctl -run '^TestMuharBudgetEnv$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-cell.log" 2>&1 && grep -F -- '--- PASS: TestMuharBudgetEnv ' "${TMPDIR:-/tmp}/dt57-cell.log"` | exit 0; named PASS; explicit values propagate and unset configuration stays unset | check:ci +flow |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt57-anchors.log"` | exit 0; existing specs and mutation sets remain unchanged | check:ci +neighbour |
| 4 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/57 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; runner, launcher and operator documentation are all in the delivering diff | check +flow +dereference |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map
- Two runs each exceed the host total: TestBudgetBoundsTotal records the excess and fails.
- Cancellation leaves capacity wedged: TestBudgetReleases fails.
- A bad or partial configuration silently disables budgeting: TestBudgetBadConfig fails.
- cellctl invents a default or drops the explicit setting: TestMuharBudgetEnv fails.
- The runner works but launcher/docs are omitted: row 4 fails.

## Review
Gate: model. Check lock ownership, process-group cancellation and actual cross-process coverage; per-process -j tests alone are insufficient.
