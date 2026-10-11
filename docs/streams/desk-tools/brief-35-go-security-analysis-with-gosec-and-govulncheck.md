---
brief: assay:assay:desk-tools:35
title: Go security analysis with gosec and govulncheck
why: >-
  Correctness lint does not cover risky security patterns or newly disclosed dependency vulnerabilities. Add complementary checks whose failures cannot disappear in CI reporting.
wave: 2
depends: ["desk-tools/34"]
unblocks: []
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
  - ".golangci.yml: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - "scripts/go-analysis.sh: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - "scripts/tests/go-analysis.sh: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - "scripts/tests/testdata/go-analysis/: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - "tools/go-analysis/pins.env: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - "docs/go-analysis.md: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - "docs/runsheets/go-security-ci-hunk.md: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
  - ".github/workflows/ci.yml: follow-up desk-tools/35 (implementation; workflow applied by a human on that branch)"
---

# Brief 35 — Go security analysis with gosec and govulncheck

## Context

files: `.golangci.yml` (planned), `scripts/go-analysis.sh` (planned), `scripts/tests/go-analysis.sh` (planned),
`scripts/tests/testdata/go-analysis/` (planned), `tools/go-analysis/pins.env` (planned), `docs/go-analysis.md` (planned),
`docs/runsheets/go-security-ci-hunk.md` (planned), `changelog/go-analysis-security.md` (planned); module manifests
and source only for small measured security fixes with version-specific dependency admission.
The human also edits `.github/workflows/ci.yml` on the implementation branch after the worker's
final commit. Structural fixes become linked work and block acceptance unless explicitly waived
through the existing human security gate; no blanket grandfathering of vulnerable dependencies.

facts:
- The merged Go policy selects gosec through golangci-lint and a separate govulncheck tool.
- The prerequisite delivers `scripts/go-analysis.sh` (planned) with acquire/lint interfaces, all-module
  discovery, pin verification, coverage reports and failure propagation; reuse that owner.
- Gosec finds security-oriented source patterns; govulncheck reports known vulnerable code
  using source reachability. Neither proves authorization/business-logic correctness.
- Govulncheck's structured-output modes can exit zero despite findings. Its source analysis
  depends on Go version, GOOS/GOARCH, build tags and test inclusion; one build is not all builds.
- No gosec/govulncheck job was present at the dated freshness baseline. Recheck at pickup.

layering: Extend the existing thin runner with upstream security-tool invocation and report
aggregation. Do not build a vulnerability database, custom static analyzer or new desk command.

single-point-of-failure: Tool orchestration can hide findings; real scanner fixtures and actual
workflow-log verification check that independently. Gosec and govulncheck cover different fault
classes; neither replaces existing leak controls or independent security review.

design-fit:
  owner: repository Go CI and scripts/go-analysis.sh
  contract: none — CI execution policy has no domain semantic-owner row
  retires: []
  weight: verbs +0, flags +0 (desk tools), refusals +0 (desk tools), rule-text +0 (worker prompts)
  why-add: reuse the prerequisite runner and upstream checks; no second orchestration framework

## Read first

- [Go engineering policy](../../go-engineering.md), `docs/go-analysis.md` (planned), `.golangci.yml` (planned),
  `tools/go-analysis/pins.env` (planned), `scripts/go-analysis.sh` (planned), `.github/workflows/ci.yml`, `SECURITY.md`.
- [gosec integration](https://golangci-lint.run/docs/linters/configuration/#gosec),
  [govulncheck command](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) and
  [Go vulnerability database](https://go.dev/security/vuln/database).

## Ground rules

- Follow the dependency-risk and release-age procedure before installing tools. No floating versions.
- Preserve existing vet/tests, secret checks, workflow permissions and independent review.
- Do not push workflow-file changes with a role App that lacks workflow permission; prepare the
  runsheet for the human. Do not dispatch workflows or probe live infrastructure.
- Stop at implemented; a different identity supplies verification Evidence. No assumed clean scans,
  invented failure evidence, broad suppressions or security weakening to make a check pass.
- If main or another active worker already owns the scope, checkpoint and reconcile before editing.

## Task

1. Add gosec to the admitted golangci-lint config and record its bundled version and enabled
   rules. Measure findings before fixes; no separate floating gosec installation. Any scoped
   suppression must name rule/path, concrete rationale, owner, linked work and review date.
   An existing unchecked-error or broad exclusion must not silently exempt security rules.
2. Admit and pin an exact govulncheck release in the existing pin file using the prerequisite's
   verified acquisition path. Use source/text mode for the failing gate; optional JSON/SARIF
   publication is supplementary. For each module/configuration, preserve failures even if later
   scans pass. Reports distinguish reachable findings, scan errors and dependency advisories;
   a zero reachable-findings result is not a claim that all dependencies are vulnerability-free.
3. Extend the interface with `bash scripts/go-analysis.sh security`. Reuse the same module
   discovery and declared platform/tag coverage; enable test inclusion explicitly and record it.
   Pinning a scanner does not freeze advisories: explicitly configure the database source and
   record retrieval/snapshot time and content identity. The default local invocation uses prepared
   module/database caches without network: explicit file-database URL, module-proxy access off
   and automatic toolchain download disabled. Do not rely on scanner defaults. `security --allow-downloads` permits fetching current
   data from printed configured targets in CI; it does not allow production/credential access.
   Failed, malformed or unavailable database reads, missing tools, package-load errors, empty
   coverage and analyzer crashes fail closed. Preserve previous reports only as historical data,
   never as a substitute for the current scan. Do not persistently pin an old database to get green.
4. Add real-tool controls to `bash scripts/tests/go-analysis.sh security`: insecure source that
   triggers a selected gosec rule and repaired source; a synthetic temporary Go module calling a
   vulnerable symbol described in a local fixture database and its fixed-version counterpart.
   Prove the text-mode gate fails then passes with the real pinned govulncheck. A stubbed exit-code
   test alone is insufficient. Add module-load/database failure controls and an integration case
   where the first scanner/module fails but later ones succeed; the aggregate must still fail.
   Keep vulnerable fixture sources/manifests as templates outside shipped-module discovery and
   use temporary copies. No external vulnerable service or executable exploit is needed.
5. Document coverage and findings; fix small admitted issues, route substantial fixes as above.
   Prepare exact workflow diff in `docs/runsheets/go-security-ci-hunk.md` (planned) for a `go-security`
   job on push/PR and daily main scans (new advisories need no source change). Keep permissions
   read-only, existing fork approval and secret isolation; no pull_request_target execution of
   PR code. The schedule must use main's reviewed config and current database. Avoid secrets or
   unrestricted credentials for tool/dependency acquisition. Human applies the hunk to this
   implementation branch after the worker's final commit and before merge. Record actual branch
   and merged-main job execution; a prepared hunk alone is not delivery. Add a changelog fragment.

## Verify

Use the prerequisite's documented pinned-tool environment. Rows 2–4 must discriminate with
observed fail/pass results; Evidence stays empty until independent verification. Row 7 requires
an actual scheduled run, so verification can remain pending until that event occurs naturally.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `bash scripts/go-analysis.sh acquire --allow-downloads` | exit 0; both exact tools, bundled gosec version and admitted checksum evidence recorded. |
| 2 | check +mutation | `bash scripts/tests/go-analysis.sh security` | exit 0 only after real gosec and govulncheck fail/pass controls and database/load/crash/aggregation controls ran. Print cases and nonzero count; swallowing a scanner failure makes the suite fail. |
| 3 | check +flow | `bash scripts/go-analysis.sh security --allow-downloads` | exit 0 only for complete clean current-database scans of every declared module/configuration; evidence names database identity/time, tools, flags and any reviewed exceptions. |
| 4 | check +mutation | `bash scripts/go-analysis.sh security` | With the prepared cache, exit 0 and no network; missing database/cache produces nonzero with an explicit error, proven separately by row 2. Offline success never claims current online data. |
| 5 | check +neighbour | `bash scripts/tests/go-analysis.sh lint && bash scripts/go-analysis.sh lint && statusgen --root . --lint && statusgen --root . --consumers --brief desk-tools/35` | exit 0; correctness coverage survives, consumers match the implementation diff. |
| 6 | gate:model +flow | `gh run list --repo medici-finance/assay --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,headSha,event,conclusion` | After human workflow application, record exact-head successful go-security job and logs proving both scanners, current database and all-module coverage. Repeat on merged main; no skipped job accepted. |
| 7 | gate:model +dereference | `gh run list --repo medici-finance/assay --workflow ci.yml --event schedule --branch main --limit 10 --json databaseId,headSha,createdAt,conclusion` | Record a real scheduled run after the implementation merge; inspect that run's go-security logs for both scanners and fresh database access. Earlier runs do not satisfy this row. Do not dispatch a workflow to manufacture scheduled evidence. |

## Pre-mortem

Structured output silently passes a vulnerability → row 2; a later module masks an earlier red
→ row 2; stale/missing database accepted → rows 2–4; platform code skipped → row 3 coverage
plus review of declared configurations; local-only or absent schedule → rows 6–7. Suppression
adequacy and business-logic threat coverage are review-only, never claimed scanner guarantees.

## Evidence

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|

## Review

Gate: model; this adds development checks with synthetic fixtures, not production operations or
sensitive-data access. A discovered vulnerability follows SECURITY.md; do not disclose private
findings in public evidence. Waiving or weakening a security gate remains a separate human
ruling. Check real scanner negative controls, database behavior, platform/module completeness,
permissions, suppression reasons and actual PR/main/scheduled execution evidence.
