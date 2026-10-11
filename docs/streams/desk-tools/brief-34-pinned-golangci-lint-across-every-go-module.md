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
schema: brief-v2
version: 1
authored: 2026-10-11 by statusgen newbrief
sources:
  - "docs/go-engineering.md — merged library and security-tooling policy, PR #2620"
  - "https://github.com/medici-finance/assay/pull/2535 — closed unmerged; replacement scope uses new IDs rather than claiming that desk-tools/32 landed"
  - "freshness-checked 2026-10-11 @ 2c45dba9b535 — fresh main, 21 tracked go.mod files, no root .golangci.yml or golangci/gosec/govulncheck job in ci.yml"
exec-tier: strong
exec-tier-why: "Cross-artifact correctness: tool pins, module/build coverage, exit propagation, fixtures and actual workflow execution must agree."
consumers:
  - ".golangci.yml: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - "scripts/go-analysis.sh: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - "scripts/tests/go-analysis.sh: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - "scripts/tests/testdata/go-analysis/: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - "tools/go-analysis/pins.env: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - "docs/go-analysis.md: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - "docs/runsheets/go-analysis-ci-hunk.md: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
  - ".github/workflows/ci.yml: follow-up desk-tools/34 (implementation; workflow applied by a human on that branch)"
---

# Brief 34 — Pinned golangci-lint across every Go module

## Context

files: `.golangci.yml` (planned), `scripts/go-analysis.sh` (planned), `scripts/tests/go-analysis.sh` (planned),
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

## Ground rules

- Follow the dependency-risk and release-age procedure before installing tools. No floating versions.
- Preserve existing vet/tests, secret checks, workflow permissions and independent review.
- Do not push workflow-file changes with a role App that lacks workflow permission; prepare the
  runsheet for the human. Do not dispatch workflows or probe live infrastructure.
- Stop at implemented; a different identity supplies verification Evidence. No assumed clean scans,
  invented failure evidence, broad suppressions or security weakening to make a check pass.
- If main or another active worker already owns the scope, checkpoint and reconcile before editing.

## Task

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
   handling. Measure every module before fixes, record per-module/per-linter counts, fix small
   mechanical findings only. A structural finding needs linked follow-up and a narrow, dated,
   reasoned exception; never a directory-wide exclusion. All nolint directives need a named
   linter and explanation, including the three existing bare ones. Do not make baseline changes
   that weaken a security control without its human gate.
3. Implement the shared interface below. Discover tracked manifests once per invocation, use each
   containing directory as cwd and the one root config. Cover supported build configurations:
   Linux amd64 plus the existing desk Windows amd64 and Darwin arm64 surface; document any
   additional supported module/build-tag combinations found at pickup. Report every attempted
   module/configuration and final counts; unsupported analysis is blocked, never silently green.
   Test inclusion is explicit and enabled for correctness lint. Preserve nonzero results even
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
   then all-module lint. The human applies it on this implementation branch AFTER the worker's
   final commit and BEFORE merge. Re-read the combined diff and wait for that job's actual run;
   neither a runsheet nor a green local command proves deployed CI. Add the changelog fragment.

## Interface contract

`bash scripts/go-analysis.sh acquire --allow-downloads` is the explicit network acquisition
entrypoint; print targets before contact. `bash scripts/go-analysis.sh lint` uses already
verified tools and prepared module caches; it must not silently install or upgrade anything.
`bash scripts/tests/go-analysis.sh lint` exercises the harness and real negative controls.
Run from the repository root with the documented pinned toolchain/cache environment. The
script discovers the root, quotes paths and stops on a missing/empty discovery result.
Exit zero means every expected analysis completed cleanly; scan, acquisition and coverage
errors are nonzero and distinguishable in the report. The security extension reuses this
owner, pin file and discovery contract rather than adding another runner.

## Verify

Rows 1–3 fail against the authoring baseline because the runner/config do not exist. Observe
real pre/post outcomes during implementation; do not pre-fill Evidence from that prediction.
Rows 6–7 run only after the human workflow commit; row 7 is repeated on merged main.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `bash scripts/go-analysis.sh acquire --allow-downloads` | exit 0; exact admitted version/toolchain and checksum evidence, no unverified fallback. |
| 2 | check +mutation | `bash scripts/tests/go-analysis.sh lint` | exit 0 only after every discovery, checksum and failure-propagation control and real analyzer red/green pair ran; print individual cases and nonzero case count. Removing module discovery or swallowing one exit makes this row fail. |
| 3 | check +flow | `bash scripts/go-analysis.sh lint` | exit 0; all discovered modules and declared configurations complete, counts reconcile with tracked manifests and no unaccounted findings. |
| 4 | check +neighbour | `cd tools/desk && GOOS=windows GOARCH=amd64 go vet ./... && GOOS=darwin GOARCH=arm64 go vet ./... && go test -count=1 ./internal/deskkit/ ./cmd/deskdispatch/` | exit 0; existing vet and representative suites remain green. |
| 5 | check | `statusgen --root . --lint && statusgen --root . --consumers --brief desk-tools/34` | exit 0; routes match the implementation diff; update this brief's own future routes as sites land. |
| 6 | check +dereference | `git diff --check && git diff --name-only 2c45dba9b..HEAD -- .github/workflows/ci.yml && gh run list --repo medici-finance/assay --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,headSha,event,conclusion` | human workflow commit exists; record run ID at this exact head, then inspect its go-lint steps/logs. Worker-authored commits contain no workflow diff. Missing run is could-not-check, not pass. |
| 7 | gate:model +flow | `gh run list --repo medici-finance/assay --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,headSha,event,conclusion` | On merged main, record actual run ID, successful go-lint job, pin acquisition and per-module/configuration logs. Match job commands to rows 1–3; a different job or head is insufficient. |

## Pre-mortem

Skipped new module or swallowed failure → row 2; plausible but wrong pin → rows 1–2;
local-only delivery or inaccessible artifact host → rows 6–7; baseline suppression hides real
code defects → row 3 plus review of each exception. Adequacy of exceptions stays review-only.

## Evidence

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|

## Review

Gate: model; all four risk answers are no because this adds development checks and changes no
production data, customer behavior or credential scope. The workflow application still requires
the human's permitted identity. Judge real analyzer discrimination, complete coverage, narrow
exceptions, pin provenance, unchanged security controls and actual CI execution, not file presence.
