---
brief: code-review-2026-07-23-assay-toolkit/02
title: DAR-drift CI gate + Job-name-bump enforcement (M3) [assay-toolkit + oit CI]
why: >-
  statusgen already HAS a darsync check that detects EXPECTED_DAR_VERSION vs daml.yaml drift, but it
  is not gating merges — which is exactly why #1117 merged with a DAR-version drift. Wiring darsync
  as a CI gate (and enforcing the Job-name-bump-on-scripts-change rule) closes a silent deploy-drift
  hole with machinery that already exists; it just isn't fatal.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-23 by Opus session (code-review-2026-07-23 authoring pass)
sources: ["2026-07-23 codebase review, finding M3", "canonical SOURCE: this repo (assay-toolkit) statusgen/darsync.go; CI wiring in oit .github/workflows"]
exec-tier: strong
exec-tier-why: (b) cross-component/cross-repo — the darsync check lives in this repo's statusgen but the gate must be wired into the oit CI workflow that consumes the release binary.
---

# Brief 02 — DAR-drift CI gate + Job-name-bump enforcement

## Context
SOURCE (two repos):
- **THIS repo (`assay-toolkit`)** `statusgen/darsync.go` — the drift check itself (already exists;
  detects a reassembled DAR with no matching `oit-v<N>-<version>` entry, issue #465).
- **`oit`** `.github/workflows/*.yml` — where the check must be
  invoked and made a hard gate (the running statusgen is the pinned assay-toolkit release binary).
facts:
- M3 root cause: darsync detects the drift but nothing FAILS the merge on it — #1117 merged with an
  `EXPECTED_DAR_VERSION` vs `daml.yaml` drift. The fix is to make the existing check gating, not to
  write a new detector.
- Deploy rule (oit CLAUDE.md): `medici-deploy-v<N>` Jobs have an immutable `spec.template` — the
  Job name must be bumped whenever the Job spec/env/init/scripts change (#465). This is not enforced
  today; M3 asks to enforce it alongside the DAR-drift gate.
- Verify darsync behaves: `cd statusgen && go test ./... -run Darsync -count=1`.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Commit only per task instructions.
- Stop at `implemented`. Report NEEDS_CONTEXT rather than guess on the exact oit CI workflow.
- This brief crosses a repo boundary: it lands a statusgen change (this repo) AND a CI-workflow
  change (oit). Note the sibling-repo SHA pair in both PRs.

## Task
1. **Gate wiring** — ensure statusgen's darsync check exits non-zero (or the `--lint`/a dedicated
   `--check-dar` mode does) when EXPECTED_DAR_VERSION vs daml.yaml / reassembled-DAR drift is
   present, and wire that exit code as a REQUIRED, merge-blocking step in the oit CI workflow that
   already runs statusgen. Confirm the drift that #1117 exhibited would now fail the job.
2. **Job-name-bump enforcement** — add a check (in statusgen, or the oit CI) that fails when a PR
   changes the medici-deploy Job spec/env/init/scripts without bumping `medici-deploy-v<N>` (the
   immutable-`spec.template` rule). Keep it advisory-with-clear-message if a hard rule is ambiguous,
   but make the drift visible in CI.
3. If any part must land in the oit repo's `.github/workflows/` and you cannot verify the exact file
   here, describe the exact required workflow edit in the PR body and report NEEDS_CONTEXT for the
   oit-side wiring.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go build ./... && go test ./... -run Dar -count=1` | exit 0 |
| 2 | Construct a drifted fixture (EXPECTED_DAR_VERSION ≠ daml.yaml) and run the darsync check | check exits NON-zero (fails) |
| 3 | Run the check on current (non-drifted) main | exit 0 |
| 4 | Inspect the oit CI workflow diff | the darsync/DAR-drift step is a required, merge-blocking job |

## Evidence
<!-- one row per Verify item, filled by a non-implementer -->

## Review
Gate: model (review loop). Reviewer records verdict + date in the stream README table.
