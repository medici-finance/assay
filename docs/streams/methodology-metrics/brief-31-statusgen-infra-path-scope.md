---
brief: methodology-metrics/31
title: 'Path-scope the repo-infra lint checks — a finance-app deploy check must not red-gate an unrelated doc PR'
why: >-
  The DAR ConfigMap parity check (tools/statusgen/darsync.go, #465) is a finance-app *deploy*
  check, but it fired on every PR statusgen lints. So a doc-only PR in another product's stream
  went red for a DAR drift it never touched — assay-launch/06 (PR #788, a one-line README change)
  was blocked exactly this way. A red check blocks the desk's ready-flip, so one product's
  deploy-in-flight state stalled every other product's merges. Scope the infra checks to their own
  deploy surface so cross-product false-reds stop.
wave: 0
depends: []
unblocks: ["methodology-metrics/32"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-18 by Opus desk session (human:<name> direction, prompted by PR #788's DAR-drift red)
sources: ["[I-three-cell-split](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-14-three-cell-split-per-cell-desks-master-aggregator.md) — the Full-tier federation this is the tooling precursor to", "PR #788 (assay-launch/06) — the concrete symptom: a one-line doc change red-gated by the DAR check", "issue #465 — the DAR ConfigMap / EXPECTED_DAR_VERSION parity drift these checks flag", "tools/statusgen/darsync.go — the isolated infra check being scoped (already merge-base git-diff aware for #587)", "freshness-checked 2026-07-18 @ origin/main 1dde84b7 (darsync checks fired whole-tree; 4 DAR PROBLEMs present)"]
consumers:
  - "tools/statusgen/: fixed-here (--changed input + darInDomain gate on darsync)"
  - ".github/workflows/statusgen.yml: fixed-here (compute PR changed paths, pass --changed)"
  - ".github/workflows/status-regen.yml (main regen): out-of-scope (no --changed → checks run unconditionally; whole-tree authority preserved)"
gate-why: n/a
---

# Brief 31 — Path-scope the repo-infra lint checks

## Context
files: `../assay-toolkit/statusgen/darsync.go`, `../assay-toolkit/statusgen/main.go` (flag + `run()` thread),
.github/workflows/statusgen.yml (the PR `--lint` invocation)
facts:
- The infra checks live entirely in `../assay-toolkit/statusgen/darsync.go`: DAR ConfigMap parity + the
  `EXPECTED_DAR_VERSION` parity. Their deploy surface is `k8s/**`, `daml/**`, `daml.yaml`.
- They fired unconditionally over the whole tree, so a PR touching none of that inherited main's
  pre-existing #465 drift as a red check (the PR #788 false-red).
- darsync already computes the origin/main merge-base and git-diffs paths (for the #587
  content-drift check) — the path-scoping reuses that shape.

## Task (as built)
1. `--changed <file>` flag (main.go): a file of changed repo-relative paths (one per line). Read
   into `[]string`, threaded through `run(root, mode, budget, changed, scope)`.
2. `darInDomain(p)` + a gate at the top of `darSyncProblems(root, changed)`: when a changed-set is
   supplied and NONE of it is under `k8s/`/`daml/` or `== daml.yaml`, return nil (drift is
   pre-existing main state, not this PR's doing). Empty changed-set → run unconditionally.
3. .github/workflows/statusgen.yml: `git diff --name-only origin/main...HEAD > changed.txt`,
   pass `--changed changed.txt` to the lint.
4. `status-regen.yml` (main) left untouched — no `--changed` → whole-tree authority preserved.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; incl. TestDarSyncChangedGateSuppressesOffDomain + TestDarSyncChangedGateFiresInDomain |
| 2 | lint with `--changed` = docs-only set, DAR drift on tree | 0 PROBLEMs (off-domain suppressed) |
| 3 | lint with `--changed` including `k8s/` or `daml.yaml`, drift present | DAR PROBLEM fires |
| 4 | lint with NO `--changed`, drift present | DAR PROBLEM fires (whole-tree authority) |
| 5 | `go vet ./tools/statusgen/` | exit 0 |

## Evidence
<!-- implementer rows (2026-07-18, Opus impl session); non-implementer re-run required for `verified`. -->
| # | Command | Result |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | ok — full suite PASS incl. the two new changed-gate tests |
| 2 | `--lint --changed <this-branch diff: docs/streams + tools + .github, no k8s/daml>` | 0 PROBLEMs (this PR's own check goes green) |
| 3 | TestDarSyncChangedGateFiresInDomain ({daml.yaml},{k8s/…},{daml/…}) | PASS — drift still caught in-domain |
| 4 | `--lint` (no --changed) on drifted tree | 4 PROBLEMs (the #465 drift, authority preserved) |
| 5 | `go vet ./tools/statusgen/` | exit 0 |

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `a6286beb`, 2026-07-19)

| # | Command | Result |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0 incl. `TestDarSyncChangedGateSuppressesOffDomain` + `TestDarSyncChangedGateFiresInDomain` |
| 2 | `--lint --changed <docs-only>` | exit 0, 0 PROBLEMs — off-domain DAR drift suppressed (proven via /tmp fixture; live tree in sync) |
| 3 | `--lint --changed` incl. `k8s/`/`daml.yaml` | 4 DAR PROBLEMs fire — fixture-proven, in-domain fire |
| 4 | `--lint` (no `--changed`) | 4 PROBLEMs fire — fixture-proven, whole-tree authority preserved |
| 5 | `go vet ./tools/statusgen/` | exit 0 |

VERIFY: PASS. The `--changed` gate suppresses off-domain drift, fires in-domain, and the no-`--changed` path keeps whole-tree authority; `statusgen.yml` uses `--changed`, `status-regen.yml` does NOT. **Key caveat:** rows 3/4 assume #465 DAR drift present on main — it's NOT (cleared by `f3c34a50`/`f5953909`; `daml.yaml`=0.1.32=ConfigMaps), so rows 3/4 reproduce only via induced-drift fixture + unit tests. `gate: model`, all risks `no` → flip.

## Review
Gate: model. Reviewer confirms scope ≠ remove (fires on `k8s/`/`daml/`/`daml.yaml` and on main),
the `--changed` mechanism is generic (reused by 32), and `status-regen.yml` is unchanged.
