---
brief: methodology-metrics/32
title: 'Product-scoped statusgen — reuse serves: + --scope <product> so each product lints only its own streams'
why: >-
  Minimal (31) path-scopes the infra checks, but the general coupling is broader: every per-stream
  and per-register check runs across all ~22 streams spanning three products (Assay, Reconciler,
  Medici-loan), so any one product's problem can red-gate another's PRs. Scoping the lint by
  product fixes that generally — and it is the exact seam the future per-cell split
  (I-three-cell-split) divides along. Doing it in-repo now makes federation additive, not a rewrite.
wave: 1
depends: ["methodology-metrics/31"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-18 by Opus desk session (human:<name> direction, product-scoping thread)
sources: ["[I-three-cell-split](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-14-three-cell-split-per-cell-desks-master-aggregator.md) — the Full-tier federation whose 3 cells this taxonomy is pinned to", "[I-per-product-apps](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-14-per-product-github-apps-product-cells.md) — the actor-side twin (per-product Apps); same 3-product axis", "methodology-metrics/31 — the --changed mechanism this reuses to derive scope from a PR", "PR #788 — the cross-product false-red this generalizes the fix for", "the existing serves: field (mm/23, parse.go/model.go/roadmap.go) — REUSED rather than adding a parallel product: key", "freshness-checked 2026-07-18 @ origin/main 1dde84b7 (serves: parsed but every stream untagged; no --scope flag existed)"]
exec-tier: strong
exec-tier-why: 'correctness depends on sweeping one tag (serves) across ALL stream READMEs consistently and wiring CI to derive scope — cross-component reasoning (question b)'
consumers:
  - "tools/statusgen/: fixed-here (--scope flag, deriveScope, filterStreamsByServes, servesCoverageNotices)"
  - ".github/workflows/statusgen.yml: fixed-here (--changed drives auto-derivation; no per-product loop needed)"
  - "docs/streams/*/README.md: fixed-here (all 22 streams tagged serves:)"
  - "dora.go / roadmap.go (existing serves: consumers): out-of-scope but IMPROVED (backfill lights up --dora --by goal + the roadmap deck, previously all-untagged)"
  - ".github/workflows/status-regen.yml (main regen): out-of-scope (no --scope/--changed → whole-house authority; STATUS.md stays house-wide)"
  - "I-three-cell-split federation: follow-up (Full tier reuses this taxonomy as the split seam)"
gate-why: n/a
---

# Brief 32 — Product-scoped statusgen (reuse serves:)

## Context
files: `../assay-toolkit/statusgen/scope.go` (new), `../assay-toolkit/statusgen/main.go` (flag + `run()` filter),
all `docs/streams/*/README.md` (backfill `serves:`), .github/workflows/statusgen.yml
facts:
- **Reuse the existing `serves:` field** (`lending-app | reconciler | assay | platform`, mm/23) —
  already parsed (`parse.go` → `Stream.Serves`), already consumed by `--dora --by goal` and the
  roadmap deck. No new `product:` key. `lending-app` = the Medici-loan cell; `platform` = the
  shared/cross-cutting bucket, ALWAYS included in a scoped run.
- **`--scope <product>`**: filters the per-stream/per-register checks to `serves == scope ||
  serves == platform`. Applied to the CHECK set only — the whole-house `streams` slice still
  drives the view build, so `--lint` stays a true superset of the post-merge regen.
- **Auto-derivation (`deriveScope`) from 31's `--changed`**: scope is inferred only when the change
  is unambiguously one non-shared product's streams; a tooling/`k8s/`/register change, an untagged
  stream, a platform-only change, or a multi-product change → unscoped (whole house). CI passes only
  `--changed`; no per-product bash loop.
- **`servesCoverageNotices`**: NOTICE any stream missing `serves:` so the taxonomy can't silently
  degrade. Advisory, never a hard gate.
- **Taxonomy (desk-confirmed 2026-07-18 with human:<name>):** assay = methodology, methodology-metrics,
  assay-dogfood, assay-launch, assay-product, desk-apps, desk-tools, issue-loop · lending-app =
  daml-hardening, ledger-hardening, frontend, privacy-hardening, compliance, deploy-hardening,
  agentic-first, agent-interop, model-risk · reconciler = reconciler-spinout, midnight-poc ·
  platform = infra-split, diagnostics, observability. **Rule for `platform`:** a stream that
  genuinely spans products (diagnostics audits both the reconciler's rights and the lending-app's
  Canton readAs; observability is house-wide monitoring) is tagged `platform` — it is included in
  EVERY scoped run, so it is never wrongly scoped out of a product it serves. When a stream "could
  be product X or Y and you're not sure", `platform` is the answer.
- Medium is the prerequisite for the cell-split: the `serves:` tags + the scope machinery are what
  per-cell statusgen instances consume; `--scope assay` in-repo becomes "the assay repo lints its
  own streams" at federation with no re-tagging.

## Task (as built)
1. `../assay-toolkit/statusgen/scope.go`: `deriveScope`, `filterStreamsByServes`, `servesCoverageNotices`,
   `validServes`.
2. `main.go`: `--scope` flag (validated against `validServes`); `run()` computes the effective
   scope (explicit `--scope` wins, else `deriveScope(streams, changed)`) and feeds a filtered
   `checkStreams` to every per-stream check; view build keeps the full `streams`.
3. Backfill `serves:` on all 22 stream READMEs (taxonomy above).
4. .github/workflows/statusgen.yml: `--changed` (from 31) drives auto-derivation.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; incl. TestDeriveScope, TestFilterStreamsByServes, TestServesCoverageNotices |
| 2 | `statusgen --root . --lint` | no serves-coverage NOTICE (all 22 tagged) |
| 3 | `--lint --scope assay` on a tree with a reconciler-stream problem | reconciler problem NOT emitted |
| 4 | `--lint --scope reconciler` on the same | reconciler problem IS emitted; assay ones not |
| 5 (flow) | `--changed` = only an assay stream, reconciler drift present → derive scope=assay | reconciler drift does not red-gate (changed→scope→assay-only) |
| 6 | `--lint` with NO scope/changed, multi-product problems | ALL emitted (whole-house authority) |
| 7 | `go vet ./tools/statusgen/` | exit 0 |

## Evidence
<!-- implementer rows (2026-07-18, Opus impl session); non-implementer re-run required for `verified`. -->
| # | Command | Result |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | ok — PASS incl. TestDeriveScope (12 cases), TestFilterStreamsByServes, TestServesCoverageNotices |
| 2 | `--lint` after backfill | 0 serves-coverage NOTICEs (22/22 tagged, verified `serves=` on every README) |
| 3–4 | TestDeriveScope + TestFilterStreamsByServes cover scope-in / scope-out per product | PASS |
| 5 | `--lint --changed <assay-only set>` on drifted tree | 0 PROBLEMs (auto-scope + DAR gate) |
| 6 | `--lint` (no scope/changed) | 4 PROBLEMs (the #465 drift — whole-house authority preserved) |
| 7 | `go vet ./tools/statusgen/` | exit 0 |

### Non-implementer re-verify — VERIFY: PASS — opus-verifier, merged main `60e3b637`, 2026-07-20

The #860 fix (findings' `affects:` validated against the full universe, not the scoped set) resolves
the prior glm-5.2 FAIL below. All 7 rows PASS, including a live injected-problem test of the scope
boundary (fixture reverted, tree left clean).

| # | Command | Exit | Result | Runner |
|---|---------|------|--------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok`; incl. TestDeriveScope, TestFilterStreamsByServes, TestServesCoverageNotices, TestCheckScopedFindingsAffectsFullUniverse (#860 regression) | opus-verifier (non-implementer) |
| 2 | `go run ./tools/statusgen --root . --lint` | 0 | no serves-coverage NOTICE (22/22 tagged) | opus-verifier (non-implementer) |
| 3 | `--lint --scope assay` with an injected invalid-status problem in reconciler-spinout | 0 | reconciler-spinout problem NOT emitted, no false "unknown stream" reds | opus-verifier (non-implementer) |
| 4 | `--lint --scope reconciler`, same injected problem | 1 | `PROBLEM: reconciler-spinout/brief-01: invalid status "bogusstatus"` emitted; assay checks scoped out | opus-verifier (non-implementer) |
| 5 | `--lint --changed <methodology-metrics/README.md>` with reconciler drift present | 0 | deriveScope→assay; reconciler problem not emitted; control `--changed <reconciler-spinout/README.md>` → problem IS emitted | opus-verifier (non-implementer) |
| 6 | `--lint` with no scope/changed, injected problem | 1 | `PROBLEM: reconciler-spinout/brief-01: invalid status "bogusstatus"` emitted (whole-house authority) | opus-verifier (non-implementer) |
| 7 | `go vet ./tools/statusgen/` | 0 | clean | opus-verifier (non-implementer) |

**VERIFY: PASS (7/7 rows).**

### Non-implementer verifier run — VERIFY: FAIL (stays `implemented`) — glm-5.2-verifier, 2026-07-19

Rows 1, 2, 6, 7 green; Rows 3, 4, 5 fail at the `--scope` boundary.

| # | Check | Exit | Result |
|---|---|---|---|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | PASS — incl. `TestDeriveScope`/`TestFilterStreamsByServes`/`TestServesCoverageNotices` |
| 2 | `--lint` (whole-house) | 0 | PASS — no serves-coverage NOTICE (all 22 streams tagged) |
| 3 | `--lint --scope assay` (reconciler problem should NOT emit) | 1 | **FAIL** — `check()`'s findings-integrity half validates each finding's `affects:` against the SCOPED set → false "Affects references unknown stream" PROBLEMs for every out-of-scope stream |
| 4 | `--lint --scope reconciler` (assay should NOT emit) | 1 | **FAIL** — same manufactured noise on assay-side findings |
| 5 | `--changed` = assay stream, reconciler drift → scope=assay, no red-gate | 1 | **FAIL** — reconciler drift DOES red-gate via the same cross-product (precisely the failure the brief exists to kill) |
| 6 | `--lint` no scope/changed, multi-product | 0 | PASS — ALL emitted (whole-house authority) |
| 7 | `go vet ./tools/statusgen/` | 0 | PASS |

**VERIFY: FAIL — stays `implemented`.** Root cause: `main.go:69` passes the **scoped** stream set into `check()`, whose findings-integrity half validates `affects:` against that scoped set → false "unknown stream" PROBLEMs under `--scope` (the cross-product red-gate the brief exists to kill). The unit tests pass only because they never exercise `check()` under a scoped set. Fix is narrow: validate findings' `affects:` against the FULL roster (split `check()` into a scoped per-stream half + an unscoped findings half). Bug filed as **[#860](https://github.com/example-org/oit/issues/860)**. No README flip.

## Review
Gate: model. Reviewer confirms (a) `serves:` was reused (no parallel key); (b) the enum maps to
I-three-cell-split's cells (lending-app ≡ Medici-loan); (c) `platform` is included in every scope;
(d) main (`status-regen.yml`) stays unscoped; (e) the ⚠ taxonomy calls (diagnostics, observability,
infra-split) are confirmed.
