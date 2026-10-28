---
brief: assay:assay:desk-tools:34
title: Pinned golangci-lint across every Go module
why: >-
  Unchecked errors, dead code and common correctness faults should be caught consistently across every Go module before review. A shared pinned runner makes new modules join the checks automatically.
wave: 1
depends: []
unblocks: ["desk-tools/35"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
delivery:
  - {in: "assay#2655", covers: partial, note: "analyzer action, runner, baseline and local controls only; CI acquisition and the human-applied workflow with an actual job run (Task 1 and 5) remain owed"}
schema: brief-v2
version: 3
authored: 2026-10-11 by statusgen newbrief
sources:
  - "Maintainer direction 2026-10-11: ratchet lint/vulnerability checks over existing debt rather than rewrite old code wholesale; feed findings into qualgen reporting."
  - "docs/go-engineering.md — merged library and security-tooling policy, PR #2620"
  - "https://github.com/medici-finance/assay/pull/2535 — closed unmerged; replacement scope uses new IDs rather than claiming that desk-tools/32 landed"
  - "freshness-checked 2026-10-11 @ 2c45dba9b535 — fresh main, 21 tracked go.mod files, no root .golangci.yml or golangci/gosec/govulncheck job in ci.yml"
exec-tier: strong
exec-tier-why: "Cross-artifact correctness: tool pins, module/build coverage, exit propagation, fixtures and actual workflow execution must agree."
consumers:
  - "actions/go-analysis/: fixed-here (reusable GitHub Action; direct maintainer direction during implementation)"
  - "tools/go-analysis/runner.py: fixed-here (stdlib adapter for upstream JSON and finding ratchet)"
  - "tools/go-analysis/baseline.json: fixed-here (reviewed debt records)"
  - "docs/go-analysis-report-v1.md: fixed-here (scan artifact contract)"
  - ".golangci.yml: fixed-here (implementation; workflow applied by a human on that branch)"
  - "scripts/go-analysis.sh: fixed-here (implementation; workflow applied by a human on that branch)"
  - "scripts/tests/go-analysis.sh: fixed-here (implementation; workflow applied by a human on that branch)"
  - "scripts/tests/testdata/go-analysis/: fixed-here (implementation; workflow applied by a human on that branch)"
  - "tools/go-analysis/pins.env: fixed-here (implementation; workflow applied by a human on that branch)"
  - "docs/go-analysis.md: fixed-here (implementation; workflow applied by a human on that branch)"
  - "docs/runsheets/go-analysis-ci-hunk.md: fixed-here (implementation; workflow applied by a human on that branch)"
  - ".github/workflows/ci.yml: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
---

# Brief 34 — Pinned golangci-lint across every Go module

## Context

files: `tools/go-analysis/baseline.json` (planned), `docs/go-analysis-report-v1.md` (planned), `.golangci.yml` (planned), `scripts/go-analysis.sh` (planned), `scripts/tests/go-analysis.sh` (planned),
`scripts/tests/testdata/go-analysis/` (planned), `tools/go-analysis/pins.env` (planned), `docs/go-analysis.md` (planned),
`docs/runsheets/go-analysis-ci-hunk.md` (planned), `changelog/go-analysis-lint.md` (planned); Go source files only
where the measured lint finding requires a small mechanical fix. The human also edits
`.github/workflows/ci.yml` on the implementation branch after the worker's final commit.

facts:
- Main's module discovery is `git ls-files '*go.mod'`; it currently finds 21, not the 20 in
  the closed authoring PR. A hard-coded count or module roster is not the coverage contract.
- `.github/workflows/ci.yml` already has push/PR triggers, read-only contents permission,
  a Go toolchain install, a module discovery loop, and cross-platform desk vet.
- That workflow records that the runner cannot reach GitHub's release-asset CDN; Go's
  download site and module proxy are available. Do not assume a release-asset URL works.
- `docs/go-engineering.md` selects errcheck, staticcheck, unused and govet as the correctness
  baseline. Gosec/govulncheck are added by the dependent brief; this runner must support reuse.
- Current bare gosec suppression comments include three lines in
  `tools/desk/internal/weight/weight_test.go`; remeasure their locations at pickup.

layering: A thin repository script owns module enumeration, pinned-tool invocation and exit
aggregation; upstream analyzers own lint semantics. No new Go module, desk verb, framework or
analyzer. Test the script with isolated fixture repos and real pinned-tool controls.

single-point-of-failure: A runner can silently skip work; independent fixture coverage and
inspection of the actual merged workflow back its result. Existing vet/tests remain independent.

design-fit:
  owner: repository Go CI and its shared script, not a desk runtime module
  contract: none — CI execution policy has no domain semantic-owner row
  retires: []
  weight: verbs +0, flags +0 (desk tools), refusals +0 (desk tools), rule-text +0 (worker prompts)
  why-add: new CI script/config only; reuse upstream analyzers and one discovery loop rather than a new analysis framework

## Read first

- [Go engineering policy](../../go-engineering.md), especially analysis and security tooling.
- `.github/workflows/ci.yml`, `CONTRIBUTING.md`, `changelog/README.md` and the root Makefile.
- [golangci-lint configuration](https://golangci-lint.run/docs/configuration/) and
  [installation guidance](https://golangci-lint.run/docs/welcome/install/) for the pinned release.

The policy's historical #2535 / desk-tools/32 delivery pointers are superseded by these
two briefs; desk-tools/35 owns updating those pointers alongside the analysis policy.

## Ground rules

- Follow the dependency-risk and release-age procedure before installing tools. No floating versions.
- Preserve existing vet/tests, secret checks, workflow permissions and independent review.
- Do not push workflow-file changes with a role App that lacks workflow permission; prepare the
  runsheet for the human. Do not dispatch workflows or probe live infrastructure.
- Stop at implemented; a different identity supplies verification Evidence. No assumed clean scans,
  invented failure evidence, broad suppressions or security weakening to make a check pass.
- If main or another active worker already owns the scope, checkpoint and reconcile before editing.

## Task

Maintainer implementation direction: expose these checks as a reusable GitHub Action
under `actions/go-analysis/`, versioned with Assay and consumed by immutable commit
SHA. The script remains its shared tested engine; the human workflow hunk is a thin
caller. Desk-tools/35 extends the same action for security and qualgen reporting.

1. Admit one exact golangci-lint v2 release compatible with the project's Go toolchain. Record
   acquisition URL/module, version, release date and checksums in `tools/go-analysis/pins.env` (planned).
   Prefer an immutable binary with an independently recorded SHA-256 when its host is available.
   If the runner requires building from the module proxy, qualify the upstream-documented source-build
   path, which upstream explicitly does NOT guarantee or recommend: isolate it from project
   dependencies, preserve upstream replacements, pin toolchain and complete module inputs,
   retain checksum-database verification, and record source checksums plus binary SHA-256.
   Prove the real analyzer controls with that build; if qualification fails, report blocked
   pending a reachable approved binary distribution route rather than silently substituting. Do not disable sum verification or
   fall back to an unverified download. Prove acquisition in CI before declaring this delivered.
2. Add `.golangci.yml` (planned) in the selected v2 schema: explicit errcheck, staticcheck, unused and
   govet; no enable-all. Turn off broad default exclusions except documented generated-code
   handling. Measure every module at a fixed main SHA before fixes and commit a reviewed
   finding-level debt baseline, not a cleanup of all legacy findings. Require an explanation for
   new or changed nolint directives; inventory pre-existing bare directives as debt rather than
   mass-editing them. Use the ratchet contract below; small fixes are optional, not a rollout gate.
   Baseline admission never overrides an already enforced vet/test/security control.
3. Implement the shared interface below. Discover tracked manifests once per invocation, use each
   containing directory as cwd and the one root config. Cover supported build configurations:
   Linux amd64 plus the existing desk Windows amd64 and Darwin arm64 surface; document any
   additional supported module/build-tag combinations found at pickup. Report every attempted
   module/configuration and final counts; unsupported analysis is blocked, never silently green.
   Test inclusion is explicit and enabled for correctness lint. Preserve scan errors and ratchet regressions even
   when later modules pass; no `|| true` or success-only reporting pipeline.
4. Add harness controls: zero modules, newly added tracked module, module path with spaces,
   bad pin/checksum, missing analyzer, and failing first/middle/last module all fail correctly.
   Include a real pinned-analyzer fixture containing an unchecked error, staticcheck defect and
   unused symbol, then its repaired form. Assert expected diagnostics and observed red/green
   outcomes. Fixtures are copied into temporary repos; deliberately bad programs never enter
   normal shipping-module discovery. Use source templates without tracked `go.mod` fixtures.
5. Document commands, versions, coverage, exclusions and evidence in `docs/go-analysis.md` (planned).
   Prepare the exact CI diff and apply command in `docs/runsheets/go-analysis-ci-hunk.md` (planned):
   job `go-lint`, existing push/PR events and permissions, verified acquisition, runner tests,
   then all-module ratcheted lint and publication of its JSON report even on a failed gate. The human applies it on this implementation branch AFTER the worker's
   final commit and BEFORE merge. Re-read the combined diff and wait for that job's actual run;
   neither a runsheet nor a green local command proves deployed CI. Add the changelog fragment.

## Interface contract

`bash scripts/go-analysis.sh acquire --allow-downloads` is the explicit network acquisition
entrypoint; print targets before contact. `bash scripts/go-analysis.sh lint` uses already
verified tools and prepared module caches; it must not silently install or upgrade anything.
`bash scripts/tests/go-analysis.sh lint` exercises the harness and real negative controls.
Run from the repository root with the documented pinned toolchain/cache environment. The
script discovers the root, quotes paths and stops on a missing/empty discovery result.
Exit zero means complete coverage with no new/worsened findings beyond accepted debt; it does
not mean zero findings. Scan, acquisition, malformed-output and coverage errors remain nonzero. The security extension reuses this
owner, pin file and discovery contract rather than adding another runner.

## Ratchet and report contract

Establish `tools/go-analysis/baseline.json` (planned) from a full scan of one recorded main
commit with pinned tools/configuration. Review the initial debt once. Each record identifies
an actual finding: analyzer/rule, module/build configuration, relative path and stable symbol
or normalized source anchor, plus first-seen, owner and linked remediation work. Do not match
by counts, line number alone, directory, linter name alone or a broad diagnostic regex.

Normal PRs may remove debt records and may fix one old issue without touching unrelated debt.
New findings, new occurrences, worsened findings and reintroductions of a removed finding fail.
Line-only movement must not create false new debt; ambiguous identity changes fail for review
rather than silently match. Adding a defect while fixing another must fail even if total counts
fall. Removing a baseline entry is permanent for ordinary PRs: absence after a complete scan
requires deleting its allowance in that fix, preventing later reintroduction under a stale entry.
Compare against the trusted target-branch baseline as well as the PR's baseline: no self-approved
expansion or automatic rebaseline in CI. A separate reviewed migration can admit a changed tool,
rule set, fingerprint schema or exception; record before/after counts and reasons and never
silently reset history. Existing security gates are unaffected.

Full scans remain mandatory, not just changed-line lint. Preserve raw tool output and emit a
versioned machine-readable report (schema in `docs/go-analysis-report-v1.md` (planned)) with
repo/commit/run identity, scan time, tool versions/config hash, baseline digest/revision,
module/configuration coverage and state, findings and their identities, new/worsened/existing/
resolved dispositions, and gate result. Include exception/review metadata and reasons for
unavailable coverage. Resolved is valid only after a complete comparable scan. A failed scan
cannot clear debt or be represented as zero findings. Report emission must run on analyzer
failure and retain the original failing exit; failed publication is visible, not success.

This is qualgen's input, not another dashboard or scanner. The dependent brief owns ingestion,
trend storage and rendering. Keep the comparison/adapter thin and reuse upstream output
formats; do not build an analyzer or a generic policy engine.

## Verify

Rows 1–3 fail against the authoring baseline because the runner/config do not exist. Observe
real pre/post outcomes during implementation; do not pre-fill Evidence from that prediction.
Rows 6–7 run only after the human workflow commit; row 7 is repeated on merged main.
For row 6, refresh origin/main, record its resolved SHA as VERIFY_BASE, and run before
merge. Inspect every listed commit's workflow patch and verify its forge attribution;
author metadata alone is not proof of the authenticated pusher. A main-only workflow
change must produce no branch commit and cannot satisfy this row. Empty history or
unverifiable attribution is could-not-check, not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `bash scripts/go-analysis.sh acquire --allow-downloads` | exit 0; exact admitted version/toolchain and checksum evidence, no unverified fallback. |
| 2 | check +mutation | `bash scripts/tests/go-analysis.sh lint` | exit 0 only after every discovery, checksum and failure-propagation control and real analyzer red/green pair ran; print individual cases and nonzero case count. Also prove accepted legacy debt passes; new/worsened/reintroduced findings fail; fixing one cannot hide another; line shifts preserve identity; baseline expansion, malformed reports and incomplete scans fail; a resolved item cannot retain its allowance. |
| 3 | check +flow | `bash scripts/go-analysis.sh lint` | exit 0; all discovered modules and declared configurations complete, counts reconcile with tracked manifests; existing debt remains visible and only new/worsened findings or scan/ratchet failures block. JSON totals and dispositions match the gate. |
| 4 | check +neighbour | `cd tools/desk && GOOS=windows GOARCH=amd64 go vet ./... && GOOS=darwin GOARCH=arm64 go vet ./... && go test -count=1 -timeout 300s ./internal/deskkit/ ./cmd/deskdispatch/` | exit 0; existing vet and representative suites remain green. |
| 5 | check | `statusgen --root . --lint && statusgen --root . --consumers --brief desk-tools/34` | exit 0; routes match the implementation diff; update this brief's own future routes as sites land. |
| 6 | check +dereference | `git diff --check && test -n "${VERIFY_BASE:?set to the recorded current main SHA}" && git log --format='%H %an %ae' "$VERIFY_BASE..HEAD" -- .github/workflows/ci.yml && gh run list --repo medici-finance/assay --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,headSha,event,conclusion` | Branch-only history identifies the human-applied workflow commit(s), their authors and patches; no worker-authored workflow changes. Confirm forge attribution, then record run ID at this exact head and inspect its go-lint steps/logs. Missing run is could-not-check, not pass. |
| 7 | gate:model +flow | `gh run list --repo medici-finance/assay --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,headSha,event,conclusion` | On merged main, record actual run ID, successful go-lint job, pin acquisition and per-module/configuration logs. Match job commands to rows 1–3; a different job or head is insufficient. |

## Pre-mortem

Count-only ratchet or baseline inflation → row 2; skipped new module or swallowed failure → row 2; plausible but wrong pin → rows 1–2;
local-only delivery or inaccessible artifact host → rows 6–7; baseline suppression hides real
code defects → row 3 plus review of each exception. Adequacy of exceptions stays review-only.

## Evidence

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|

## Review

Gate: model; all four risk answers are no because this adds development checks and changes no
production data, customer behavior or credential scope. The workflow application still requires
the human's permitted identity. Judge real analyzer discrimination, complete coverage, narrow
ratchet identity/monotonicity, pin provenance, unchanged security controls and actual CI execution, not file presence.
