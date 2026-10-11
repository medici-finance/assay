---
brief: assay:assay:desk-tools:35
title: Go security analysis with gosec and govulncheck
why: >-
  Correctness lint does not cover risky security patterns or newly disclosed dependency vulnerabilities. Add complementary checks whose failures cannot disappear in CI reporting.
wave: 2
depends: ["desk-tools/34"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-11 by statusgen newbrief
sources:
  - "Maintainer direction 2026-10-11: ratchet lint/vulnerability checks over existing debt rather than rewrite old code wholesale; feed findings into qualgen reporting."
  - "docs/go-engineering.md — merged library and security-tooling policy, PR #2620"
  - "https://github.com/medici-finance/assay/pull/2535 — closed unmerged; replacement scope uses new IDs rather than claiming that desk-tools/32 landed"
  - "freshness-checked 2026-10-11 @ 2c45dba9b535 — fresh main, 21 tracked go.mod files, no root .golangci.yml or golangci/gosec/govulncheck job in ci.yml"
exec-tier: strong
exec-tier-why: "Cross-artifact correctness: tool pins, module/build coverage, exit propagation, fixtures and actual workflow execution must agree."
consumers:
  - "docs/go-engineering.md: follow-up desk-tools/35 (document the ratchet and report adoption)"
  - "qualgen/mine.go: follow-up desk-tools/35 (ingest the scan artifact with the existing miner)"
  - "qualgen/artifacts.go: follow-up desk-tools/35 (append-only scan record family)"
  - "qualgen/report.go: follow-up desk-tools/35 (render debt, delta and coverage)"
  - "qualgen/init.go: follow-up desk-tools/35 (keep adopter scaffold consistent)"
  - "docs/quality/go-analysis.jsonl: follow-up desk-tools/35 (main CI appends; branch only uses fixtures)"
  - "docs/quality/QUALITY.md: follow-up desk-tools/35 (CI-only generated output, never branch-committed)"
  - ".github/workflows/assay-qualgen.yml: follow-up desk-tools/35 (human-applied ingestion/publication wiring)"
  - "ci/staged-workflows/assay-qualgen.yml: follow-up desk-tools/35 (keep staged template aligned)"
  - "tools/go-analysis/baseline.json: follow-up desk-tools/35 (reviewed debt records)"
  - "docs/go-analysis-report-v1.md: follow-up desk-tools/35 (scan artifact contract)"
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

files: `tools/go-analysis/baseline.json` (planned), `docs/go-analysis-report-v1.md` (planned), `.golangci.yml` (planned), `scripts/go-analysis.sh` (planned), `scripts/tests/go-analysis.sh` (planned),
`scripts/tests/testdata/go-analysis/` (planned), `tools/go-analysis/pins.env` (planned), `docs/go-analysis.md` (planned),
`docs/runsheets/go-security-ci-hunk.md` (planned), `changelog/go-analysis-security.md` (planned); module manifests
and source only for small measured security fixes with version-specific dependency admission.
Also `qualgen/{mine,artifacts,report,init}.go` and their tests, `qualgen/goanalysis.go` (planned),
`qualgen/goanalysis_test.go` (planned), `qualgen/testdata/goanalysis/` (planned),
`ci/staged-workflows/assay-qualgen.yml` `docs/report-packs.md` and `docs/go-engineering.md`.
The human also edits `.github/workflows/assay-qualgen.yml` and `.github/workflows/ci.yml` on the implementation branch after the worker's
final commit. Existing findings become individually recorded debt; structural fixes are linked follow-up
work, not a demand to rewrite the repository before rollout. No blanket exemption or weakening
of an existing security gate is authorized.

facts:
- The merged Go policy selects gosec through golangci-lint and a separate govulncheck tool.
- The prerequisite delivers `scripts/go-analysis.sh` (planned) with acquire/lint interfaces, all-module
  discovery, pin verification, coverage reports and failure propagation; reuse that owner.
- Gosec finds security-oriented source patterns; govulncheck reports known vulnerable code
  using source reachability. Neither proves authorization/business-logic correctness.
- Govulncheck's structured-output modes can exit zero despite findings. Its source analysis
  depends on Go version, GOOS/GOARCH, build tags and test inclusion; one build is not all builds.
- `qualgen/report.go` reads append-only quality artifacts and guards its CI-only write path;
  `qualgen/suspects.go` covers slop categories and line-based identities, not vulnerability
  accounting. Reuse its three-state measure model, not its line-only identity for this ratchet.
- `.github/workflows/assay-qualgen.yml` is the live quality-report workflow;
  `ci/staged-workflows/assay-qualgen.yml` is a staged counterpart. Both need coverage review.
- No gosec/govulncheck job was present at the dated freshness baseline. Recheck at pickup.

layering: Extend the existing thin runner with upstream security-tool invocation and report
aggregation. Qualgen owns artifact ingestion/history/rendering, and never reruns analyzers or recomputes
a conflicting CI verdict. Do not build a vulnerability database, custom analyzer or dashboard.

Sizing: L because scanner output, the ratchet and qualgen must agree end-to-end; splitting the
consumer off would let CI land with no reporting. The upstream runner is already a prerequisite.

single-point-of-failure: Tool orchestration can hide findings; real scanner fixtures and actual
workflow-log verification check that independently. Gosec and govulncheck cover different fault
classes; neither replaces existing leak controls or independent security review.

design-fit:
  owner: repository Go CI and scripts/go-analysis.sh
  contract: none — CI execution policy has no domain semantic-owner row
  retires: []
  weight: verbs +0, flags +1 (qualgen mine input), refusals +0 (desk tools), rule-text +0 (worker prompts)
  why-add: add one scan-input seam to the existing miner and report owner instead of a new command or dashboard

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
   rules. Apply the prerequisite finding-level ratchet to an initial measured gosec debt
   baseline; no wholesale cleanup and no separate floating gosec installation. Any scoped
   suppression must name rule/path, concrete rationale, owner, linked work and review date.
   An existing unchecked-error or broad exclusion must not silently exempt security rules.
2. Admit and pin an exact govulncheck release in the existing pin file using the prerequisite's
   verified acquisition path. Ratchet known reachable vulnerabilities with explicit records
   keyed by vulnerability ID, affected module/version, symbol and build/reachability scope;
   record owner, first-seen, remediation work and review-by date. These record accepted debt,
   not false positives. Newly disclosed IDs, newly reachable symbols, affected-version upgrades,
   broadened scope, expired exceptions and reintroduced fixed issues fail, even in old code.
   Keep advisories without source reachability separate, not silently zeroed or called safe.
   Use source analysis and structured findings for the thin comparison adapter. Govulncheck
   JSON/SARIF can exit zero with findings, so scanner exit alone cannot gate this ratchet:
   validate complete output and explicitly evaluate findings against the trusted baseline.
   This is the maintainer-requested ratchet refinement of the policy's initial text-mode gate;
   prove raw text-mode failure and the structured evaluator's fail/pass behavior with real tools.
   No unconditional success on raw nonzero exits or malformed/unknown output.
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
   Prove raw text-mode failure, then structured ratchet failure/pass with real govulncheck;
   accepted existing debt passes while a new advisory against unchanged code fails.
   Test scope broadening, expiry, resolved-then-reintroduced findings and baseline tampering. A stubbed exit-code
   test alone is insufficient. Add module-load/database failure controls and an integration case
   where the first scanner/module has a scan error or new finding and later ones succeed;
   the aggregate must still fail. Accepted debt must never swallow an execution failure.
   Keep vulnerable fixture sources/manifests as templates outside shipped-module discovery and
   use temporary copies. No external vulnerable service or executable exploit is needed.
5. Document coverage, debt and deltas in the operational guide and Go engineering policy;
   replace the earlier text-mode-only deployment guidance with this verified ratchet contract.
   Fixes are incremental. Extend the shared scan report
   with vulnerability/database/reachability fields and publish it on green and red scans.
   Prepare exact workflow diff in `docs/runsheets/go-security-ci-hunk.md` (planned) for a `go-security`
   job on push/PR and daily main scans (new advisories need no source change). Keep permissions
   read-only, existing fork approval and secret isolation; no pull_request_target execution of
   PR code. The schedule must use main's reviewed config and current database. Avoid secrets or
   unrestricted credentials for tool/dependency acquisition. Human applies the hunk to this
   implementation branch after the worker's final commit and before merge. Record actual branch
   and merged-main job execution; a prepared hunk alone is not delivery. Add a changelog fragment.

6. Feed both lint and security reports into qualgen through an optional
   `qualgen mine --analysis-report <file>` input, preserving existing invocation behavior.
   Define/validate the schema and append snapshots to `docs/quality/go-analysis.jsonl` (planned)
   through the existing Store conventions. Missing, stale, malformed, incomplete or wrong-repo/
   head artifacts produce could-not-measure with reasons, never a measured zero or resolved debt.
   Deduplicate retries by run/tool/module/configuration identity; do not collapse distinct scans
   or merge different heads. Retain reported gate results alongside ingestion validity.
7. Render a Go analysis section in qualgen's existing report: total outstanding, accepted debt,
   new/worsened, resolved, overdue exceptions, age since first observation (not presumed introduction) and coverage by tool/module; dated trends
   only across comparable complete scans. Separate lint, gosec and dependency-vulnerability
   families and note overlapping diagnoses rather than summing them into an invented quality
   score. Existing unscanned repos render not configured/could-not-measure, not clean. Keep
   raw output pointers and provenance; do not relabel scanner findings as confirmed defects.
   Redact credential literals, local paths and sensitive diagnostic payloads before publishing;
   keep sensitive raw artifacts access-controlled and follow SECURITY.md for disclosure.
8. Extend the human runsheet to feed matching CI artifacts into the existing quality-report
   flow, including failed and scheduled scans. Authenticate artifact provenance to the expected
   repository, workflow/run and source SHA; never execute downloaded artifact content. PR renders
   are previews only. The main writer alone appends history and writes QUALITY.md, preserving its
   guard, serialization and permissions. No privileged execution of PR-supplied code. Update
   staged/adopter templates and trigger coverage for scanner config/pins, baseline, source and
   report schema. Demonstrate the full flow with a captured real scanner report and controlled
   debt/new/resolved/failed fixtures; fixture output is local, not a hand-edit to generated views.

## Verify

Use the prerequisite's documented pinned-tool environment. Rows 2–4 must discriminate with
observed fail/pass results; Evidence stays empty until independent verification. Row 7 requires
an actual scheduled run, so verification can remain pending until that event occurs naturally. Rows 8–9 prove scanner output reaches qualgen.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `bash scripts/go-analysis.sh acquire --allow-downloads` | exit 0; both exact tools, bundled gosec version and admitted checksum evidence recorded. |
| 2 | check +mutation | `bash scripts/tests/go-analysis.sh security` | exit 0 only after real scanner and finding-level ratchet controls, including new disclosure on unchanged code, expiry, baseline inflation and database/load/crash/aggregation failures. Print cases and nonzero count; swallowing a scanner failure makes the suite fail. |
| 3 | check +flow | `bash scripts/go-analysis.sh security --allow-downloads` | exit 0 only for complete current-database scans with no unaccepted new/worsened findings; accepted debt remains reported; evidence names database identity/time, tools, flags and any reviewed exceptions. |
| 4 | check +mutation | `bash scripts/go-analysis.sh security` | With the prepared cache, exit 0 and no network; missing database/cache produces nonzero with an explicit error, proven separately by row 2. Offline success never claims current online data. |
| 5 | check +neighbour | `bash scripts/tests/go-analysis.sh lint && bash scripts/go-analysis.sh lint && statusgen --root . --lint && statusgen --root . --consumers --brief desk-tools/35` | exit 0; correctness coverage survives, consumers match the implementation diff. |
| 6 | gate:model +flow | `gh run list --repo medici-finance/assay --workflow ci.yml --commit "$(git rev-parse HEAD)" --json databaseId,headSha,event,conclusion` | After human workflow application, record exact-head successful go-security job and logs proving both scanners, current database and all-module coverage. Repeat on merged main; no skipped job accepted. |
| 7 | gate:model +dereference | `gh run list --repo medici-finance/assay --workflow ci.yml --event schedule --branch main --limit 10 --json databaseId,headSha,createdAt,conclusion` | Record a real scheduled run after the implementation merge; inspect that run's go-security logs for both scanners and fresh database access. Earlier runs do not satisfy this row. Do not dispatch a workflow to manufacture scheduled evidence. |
| 8 | check +flow +mutation | `cd qualgen && go test -count=1 ./...` | Includes a non-vacuous scanner-report → miner ingest → Store → rendered report fixture: accepted debt passes CI but remains visible; new findings fail CI and appear in report; complete repair shrinks debt. Failed/partial/stale/wrong-head input cannot render zero or clear findings; repeated import does not double-count. Preserve single-writer and existing metric tests. |
| 9 | gate:model +flow | `gh run list --repo medici-finance/assay --workflow assay-qualgen.yml --branch main --limit 10 --json databaseId,headSha,createdAt,conclusion` | On merged main, inspect a quality-report run after implementation and its bound scanner artifact(s): source SHA/run identity and debt/delta/coverage match emitted data and the generated report. A green CI badge alone is insufficient; record red/scheduled artifact delivery evidence too. |

## Pre-mortem

Structured output silently passes a vulnerability → row 2; a later module masks an earlier red
→ row 2; stale/missing database accepted → rows 2–4; platform code skipped → row 3 coverage
plus review of declared configurations; local-only or absent schedule → rows 6–7. Suppression
adequacy and business-logic threat coverage are review-only, never claimed scanner guarantees.
Green ratchet hides old debt, wrong-head artifacts look current, or failed scans clear debt
→ rows 8–9; report delivery omits red runs → row 9.

## Evidence

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|

## Review

Gate: model; this adds development checks with synthetic fixtures, not production operations or
sensitive-data access. A discovered vulnerability follows SECURITY.md; do not disclose private
findings in public evidence. Waiving or weakening a security gate remains a separate human
ruling. Check real scanner negative controls, database behavior, platform/module completeness,
permissions, suppression reasons and actual PR/main/scheduled execution evidence.
