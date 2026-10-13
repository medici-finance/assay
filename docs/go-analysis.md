# Ratcheted Go correctness analysis

The pinned runner inventories every tracked Go module. Existing debt is recorded as
individual findings; CI rejects additions, changed diagnostics, stale allowances
and incomplete scans. A passing gate means no regressions beyond accepted debt,
not zero findings. Existing vet, tests and security checks remain independent.

## Run locally

Python 3.12+, Bash and Git are required. The acquisition host supports Linux amd64
and Darwin arm64. Acquisition prints public download targets before contact and
needs explicit opt-in:

```sh
bash scripts/go-analysis.sh acquire --allow-downloads
bash scripts/tests/go-analysis.sh lint
GO_ANALYSIS_BASE=$(git rev-parse refs/remotes/origin/main)
export GO_ANALYSIS_BASE
bash scripts/go-analysis.sh lint
```

Use `--root PATH` to analyze a different checkout and `--cache PATH` for a dedicated
cache outside that checkout. Run the trusted harness with
`GO_ANALYSIS_TEST_CACHE=PATH` pointing at that prepared cache. A CI cache must be a
fresh directory outside the candidate checkout, not candidate-controlled content.
The [GitHub Action](../actions/go-analysis/README.md) owns consumer integration;
the [human workflow runsheet](runsheets/go-analysis-ci-hunk.md) applies the local job.

The lint command is offline: `GOPROXY=off`, `GOVCS=*:off`, `GOTOOLCHAIN=local`,
`GOENV=off`, `GOWORK=off`, cgo disabled and no inherited Go flags. Missing modules,
packages, tools or valid output fail. It never installs a missing tool or silently
changes its version. Checksum signatures remain required: the offline sumdb URL is
a cache-owned file URL, so a checksum cache miss fails locally instead of Go
falling back to a remote database. A local proxy-trap fixture proves zero attempted
transport on that miss. Acquisition uses the normal authenticated sumdb.
Acquisition prepares the module cache separately and does not
run project tests or generators.

## Pins and source-build qualification

[Admission #2649](https://github.com/medici-finance/assay/issues/2649) records the
risk assessment. The scanner is golangci-lint **2.14.0**, released 24 September 2026,
built with **Go 1.27.1** (released 1 September 2026). Project package loading uses
**Go 1.25.0**, matching the existing CI target. These are separate toolchains;
no module directive or existing CI job is upgraded.

The runner cannot reach GitHub's release-asset CDN. It therefore uses the
[upstream source-install route](https://golangci-lint.run/docs/welcome/install/local/),
which upstream does not recommend or guarantee. Qualification is specific:

- Official Go archive SHA-256 pins and an immutable module-proxy source archive.
- Upstream go.mod and go.sum hashes before and after the build. No dependency
  update, project graph mixing, replacement loss or disabled checksum database.
- A readonly source build in isolation; `go mod verify`; recorded binary SHA-256.
- Actual errcheck, staticcheck and unused defects produce a failing analyzer run;
  the repaired fixture passes with the same binary and Go 1.25.0 package loader.

The observed Darwin arm64 qualified build SHA-256 was
`6da8712d00a514c310b6bd8e6994e37265d204591b94c68cfe23cac6352e472c`.
Each platform's acquisition records its own receipt and checks it before scanning.
This digest does not certify a Linux build that has not run. Actual CI acquisition
and controls remain mandatory before deployment can be claimed.

The initial attempt using `go mod download all` added transitive test checksums
and correctly failed the immutable-lock check. The final command downloads the
selected module graph and builds readonly; both upstream lockfiles remain exact.
Build output uses a fixed upstream release timestamp to retain the version stamp.
No floating source or binary fallback is provided.

## Coverage and policy

Discovery uses tracked `*go.mod` paths, including paths with spaces. At the migration
source revision, 21 modules yielded 24 scans: Linux amd64 for every module, plus
desk Windows amd64, Darwin arm64 and Linux amd64 with the `parity` tag. Tests are
included. The `untrustscan_broken_unicode` tag is an intentional mutation, not a
shipping configuration. Other OS constraint branches are outside the current
supported build matrix; expanding support must expand this declared matrix too.

`.golangci.yml` explicitly enables errcheck, staticcheck, unused and govet. It also
enables nolintlint to require a specific rule and explanation on suppressions;
pre-existing bare suppressions are debt, not a mass-edit requirement. Upstream
strict generated-file detection is the only generated-code exclusion. Default
exclusion presets, path exclusions and issue truncation are disabled.

The complete measurement at the current migration source (which includes the
latest `main` sync, listed in the PR body) contains **8,873 occurrences**:
7,531 errcheck, 1,073 staticcheck, 249 unused, 12 nolintlint and 8 govet. These counts
include separate configurations of the same source, not distinct physical defects.
The eight govet observations are upstream's newer inline analyzer on two sites
across four desk configurations; they do not override the existing Go 1.25 vet gate.
No shipping Go source was changed to introduce the baseline.
[Remediation #2654](https://github.com/medici-finance/assay/issues/2654) owns the
incremental inventory; existing deferred-Close and deprecated AST work remains
linked from that issue.

## Maintain the ratchet

Each baseline entry names analyzer/rule, module/configuration, relative source
path, stable source anchor, diagnostic hash, first-observed date, owner and linked
remediation work. Review the initial inventory once. Subsequently:

1. Fix an individual finding and remove its exact allowance in the same PR.
2. Run a complete comparable scan. An old allowance whose finding disappeared
   fails until removed; a removed allowance cannot silently re-admit a regression.
3. New occurrences and changed diagnostics fail, even if another fix lowers total
   counts. Baseline entries must be byte-equivalent records from trusted main;
   ordinary PRs cannot add or rewrite them.
4. Tool, configuration and fingerprint changes require a separately reviewed
   migration with before/after evidence. CI never automatically rebaselines.

Identity uses source content and distinguishing context, not line numbers or
counts. The anchor is scoped to the enclosing top-level declaration: fixing,
adding or removing a finding in another function never re-identifies an untouched
finding, even when the two lines are textually identical. Within one declaration,
identical lines are told apart by the smallest distinguishing neighborhood, so
editing next to such a line can conservatively produce a new identity. Blank/comment
movement preserves ordinary code anchors. Ambiguous identities fail for review. See the [report contract](go-analysis-report-v1.md) for exact fields.
The baseline does not exempt failures from any already-enforced test, vet or
security gate. Findings are analyzer observations, not confirmed vulnerabilities.

## Evidence and reporting

The runner retains raw upstream JSON and stdout/stderr locally and emits
`report.json` even on analyzer/ratchet failure when the filesystem permits it.
Only the structured report is published. Source text and diagnostic payloads are
hashed; errors use controlled categories. Publication failure remains visible.

The report binds analyzed source SHA, event/head/base identities, tool receipt,
policy and baseline digests, run/attempt, coverage state, finding dispositions and
gate result. Incomplete scans cannot resolve debt or report measured zero.
Desk-tools/35 consumes this same artifact into qualgen; ingestion, vulnerability
analysis and historical rendering are not implemented by this correctness change.

The committed harness runs 13 tests and 13 guard mutations, with real upstream
analyzer red/green controls. It proves module discovery, source movement, baseline
monotonicity, duplicate refusal, policy changes, checksum rejection and failure
propagation in the first/middle/last scan. The action has its own separate wrapper
controls. Implementation evidence belongs in the PR workpad; independent verifier
Evidence and actual merged-main CI remain distinct from local results.
