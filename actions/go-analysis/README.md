# Ratcheted Go analysis action

A composite action for the shared Go lint runner. It acquires pinned tools,
checks every tracked Go module, preserves individually reviewed legacy debt,
fails regressions, and publishes the same structured findings used by the gate.
It also exposes counts and scan state as outputs and writes a job summary.

Use a full reviewed Assay commit SHA. The action versions with this repository;
there is no separate action release or Marketplace publication. Its implementation
follows the subdirectory packaging chosen for Muhar in #2642.

```yaml
permissions:
  contents: read
steps:
  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
    with:
      ref: ${{ github.event.pull_request.head.sha || github.sha }}
      fetch-depth: 0
      persist-credentials: false
  - id: analysis
    uses: medici-finance/assay/actions/go-analysis@REVIEWED_ASSAY_COMMIT_SHA
    with:
      root: .
      base-ref: main
      allow-downloads: 'true'
```

Before adopting, prepare the reviewed finding-level baseline and matching analyzer
config/pins described in [the operational guide](../../docs/go-analysis.md).
The action consumes those policy files; an empty baseline is not a shortcut for
an unmeasured repository. For the first admission only, provide an immutable
`bootstrap-revision` containing the reviewed baseline in the calling repository.
Once the base has a baseline, bootstrap is ignored so resolved allowances can
continue to shrink. Partial installations fail rather than silently reverting to
an older baseline.

The action executes code from its own pinned ref. Config/pins in the analyzed tree
must match that version. Updating policy or tools requires an explicitly reviewed
action-pin and baseline migration; a PR cannot silently make its checker permissive.
The current action deliberately refuses post-bootstrap contract migrations.
Desk-tools/35 must add an explicit human-reviewed migration lane before enabling
gosec; changing the pin alone will fail against the old trusted baseline.
Do not pass a PR-controlled ref as `base-ref`. The caller's workflow permissions,
fork approval, runner isolation and merge protection still govern execution;
a composite action cannot protect a workflow that a caller permits a PR to replace.

## Inputs and runtime

| Input | Default | Purpose |
|---|---|---|
| `root` | `.` | Git checkout to analyze. |
| `base-ref` | `main` | Trusted baseline ref in the calling repository; an explicit recorded SHA is also accepted. |
| `bootstrap-revision` | empty | Full human-reviewed SHA for initial admission, only while the base has no installation. |
| `allow-downloads` | `false` | Set explicitly to `true` to acquire pinned tools and dependencies from printed public sources. |

Requires Bash, Git, Python 3.12+, and a supported Linux/macOS host from the pin file.
Toolchains are installed into an isolated task directory, never globally. The
analysis toolchain remains distinct from the tool's build toolchain. No analyzed project test binaries or
generators are executed, and no production endpoints are contacted. The harness uses synthetic local
fixtures and the real pinned analyzers.

## Outputs and failure behavior

`report-path`, `state`, `gate`, `existing`, `new`, `worsened`, and `resolved` are
outputs. Counts are unset for an incomplete scan: unknown must not look like zero.
A scan returning nonzero still fails the action; summary and upload steps use
`always()` and do not convert it to success. Missing or malformed reports fail.

Only sanitized `report.json` is uploaded as
`go-analysis-lint-<source-sha>-<run-id>-<attempt>`, retained for 14 days. No raw source
snippets or diagnostics are uploaded. The [report contract](../../docs/go-analysis-report-v1.md)
defines provenance and valid comparisons. Security scanning and qualgen ingestion
are the dependent desk-tools/35 implementation, not claimed as delivered here.

## Development

Run `python3 -m unittest discover -s actions/go-analysis -p 'test_*.py'` for the
wrapper's offline controls. The underlying real-analyzer and ratchet tests remain
`bash scripts/tests/go-analysis.sh lint` with the acquired cache set through
`GO_ANALYSIS_TEST_CACHE`. The human-applied [CI runsheet](../../docs/runsheets/go-analysis-ci-hunk.md)
contains the exact thin consumer workflow patch.
