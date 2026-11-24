# Merge-base Go correctness analysis

The pinned runner inventories every tracked Go module. The gate analyses the pull
request's merge base and its head under the same pinned policy and fails only on
what the head adds: new findings, changed diagnostics and incomplete scans. Debt
that main already carries passes; a finding the head repairs resolves with no file
to edit. A passing gate means no regressions against the merge base, not zero
findings. A separate [debt report](#debt-report) keeps main's findings visible and
never gates. Existing vet, tests and security checks remain independent.

## Run locally

Python 3.12+, Bash and Git are required. The acquisition host supports Linux amd64
and Darwin arm64. Acquisition prints public download targets before contact and
needs explicit opt-in:

```sh
bash scripts/go-analysis.sh acquire --allow-downloads
bash scripts/tests/go-analysis.sh lint
GO_ANALYSIS_BASE=$(git rev-parse refs/remotes/origin/main)
export GO_ANALYSIS_BASE
bash scripts/go-analysis.sh acquire --allow-downloads --base "$GO_ANALYSIS_BASE"
bash scripts/go-analysis.sh lint
bash scripts/go-analysis.sh report
```

`lint` needs the base commit and enough history to compute `git merge-base`. It
checks the merge base out as a detached worktree under the cache, scans it,
and removes it afterwards. When the head is its own merge base (a push to main),
one scan serves as both sides. Acquisition with `--base` also prepares the merge
base's module graph, because the scan itself is offline.

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
built with **Go 1.27.1** (released 1 September 2026). Project package loading uses a
separately installed **Go 1.27.1** (`ANALYSIS_GO`). The rest of the fleet has since moved
to one Go version (1.27.4, `go 1.27.0`; #2918); these analysis pins lag it on purpose
and are listed exceptions in that single-version guard
(`tools/desk/internal/deskkit/goversion_test.go`) until a separately reviewed
reviewed policy migration moves them (see "Maintain the comparison").

The runner cannot reach GitHub's release-asset CDN. It therefore uses the
[upstream source-install route](https://golangci-lint.run/docs/welcome/install/local/),
which upstream does not recommend or guarantee. Qualification is specific:

- Official Go archive SHA-256 pins and an immutable module-proxy source archive.
- Upstream go.mod and go.sum hashes before and after the build. No dependency
  update, project graph mixing, replacement loss or disabled checksum database.
- A readonly source build in isolation; `go mod verify`; recorded binary SHA-256.
- Actual errcheck, staticcheck and unused defects produce a failing analyzer run;
  the repaired fixture passes with the same binary and Go 1.27.1 package loader.

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
source revision, 30 modules yielded 36 scans: Linux amd64 for every module, plus
Windows amd64, Darwin arm64 and Linux amd64 with the `parity` tag for both
`tools/desk` and `gitcore`. Tests are
included. The `untrustscan_broken_unicode` tag is an intentional mutation, not a
shipping configuration. Other OS constraint branches are outside the current
supported build matrix; expanding support must expand this declared matrix too.

`.golangci.yml` explicitly enables errcheck, staticcheck, unused and govet. It also
enables nolintlint to require a specific rule and explanation on suppressions;
pre-existing bare suppressions are debt, not a mass-edit requirement. Upstream
strict generated-file detection is the only generated-code exclusion. Default
exclusion presets, path exclusions and issue truncation are disabled.

The complete measurement at the last recorded migration source (named by the retained
register's `source_sha`) contains **13,487 occurrences**: 11,599 errcheck, 1,447 staticcheck,
416 unused, 13 govet and 12 nolintlint. These counts include separate configurations
of the same source, not distinct physical defects. Staticcheck "related information"
notes annotate a primary finding and are dropped before identity (a note without its
primary refuses); the primary findings remain individually recorded. The govet
observations are upstream's newer analyzers; they do not override the existing vet
gate. This measurement replaced the first one after the package loader moved to
Go 1.27.1: records whose identity and diagnostic were unchanged kept their original
first-observed date.
No shipping Go source was changed to introduce the register.
[Remediation #2654](https://github.com/medici-finance/assay/issues/2654) owns the
incremental inventory; existing deferred-Close and deprecated AST work remains
linked from that issue.

## Maintain the comparison

The gate compares two scans of the same pull request; no committed file decides
what passes:

1. A finding present at the merge base and unchanged at the head is `existing` and
   passes. A finding the head adds is `new`; a changed diagnostic at an identity
   the merge base carries is `worsened`. Either fails the gate.
2. A merge-base finding absent from a complete head scan is `resolved`. A fix
   needs no allowance edit. An incomplete scan on either side cannot pass the head
   or resolve anything.
3. The merge base is analyzed under the runner's own reviewed policy, or not at
   all. If the merge base's `.golangci.yml` or `pins.env` differs from the runner's,
   the gate refuses with `analysis policy migration requires reviewed trusted
   runner`. Tool, configuration and fingerprint changes stay a separately reviewed
   migration with before/after evidence.
4. CI never writes to main. Main moving on after a branch forked does not change
   that branch's verdict: it is compared with its own merge base until it merges
   main in.

`tools/go-analysis/baseline.json` is kept for reporting only. The gate never reads
it. The debt report uses it to name the owner and linked remediation work of
recorded debt, and the action pinned in CI before this change still reads it. It
is retired once the CI pin moves past this change and the debt report has an
owner register of its own.

### Debt report

`bash scripts/go-analysis.sh report` scans the analyzed revision, lists every
finding as `existing`, and marks which carry a record in the retained register.
It exits 0 whenever the scan is complete, whatever it found; only an incomplete
scan fails it. The scheduled workflow that publishes it is staged at
`ci/staged-workflows/go-analysis-debt-report.yml` and is not a required check.

Identity uses source content and distinguishing context, not line numbers or
counts. The anchor is scoped to the enclosing top-level declaration: fixing,
adding or removing a finding in another function never re-identifies an untouched
finding, even when the two lines are textually identical. Within one declaration,
identical lines are told apart by the smallest distinguishing neighborhood, so
editing next to such a line can conservatively produce a new identity. Blank/comment
movement preserves ordinary code anchors. Lines before the first declaration (the
package clause and the comments and directives above it) form their own scope. Ambiguous identities fail for review. See the [report contract](go-analysis-report-v1.md) for exact fields.
The comparison does not exempt failures from any already-enforced test, vet or
security gate. Findings are analyzer observations, not confirmed vulnerabilities.

## Evidence and reporting

The runner retains raw upstream JSON and stdout/stderr locally and emits
`report.json` even on analyzer or gate failure when the filesystem permits it.
Only the structured report is published. Source text and diagnostic payloads are
hashed; errors use controlled categories. Publication failure remains visible.

The report binds analyzed source SHA, event/head/base identities, the merge base
it compared against, tool receipt, policy digest, run/attempt, head and merge-base
coverage state, finding dispositions and gate result. Incomplete scans cannot resolve debt or report measured zero.
Desk-tools/35 consumes this same artifact into qualgen; ingestion, vulnerability
analysis and historical rendering are not implemented by this correctness change.

The committed harness runs 23 tests and 24 guard mutations, with real upstream
analyzer red/green controls. It proves module discovery, source movement, the
merge-base comparison (a head that adds a finding fails; one that touches none or
repairs one passes; a moved main does not change a forked branch's verdict),
duplicate refusal, policy at the merge base, checksum rejection and failure
propagation in the first/middle/last scan and in the merge-base scan. The action has its own separate wrapper
controls. Implementation evidence belongs in the PR workpad; independent verifier
Evidence and actual merged-main CI remain distinct from local results.
