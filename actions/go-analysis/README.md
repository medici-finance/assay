# Merge-base Go analysis action

A composite action for the shared Go lint runner. It acquires pinned tools,
checks every tracked Go module at the pull request's merge base and at its head,
fails only on findings the head adds, and publishes the same structured findings
used by the gate. In `mode: report` it instead lists every finding at the
analyzed revision and never fails on findings.
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

Gate mode needs full history (`fetch-depth: 0`) so the merge base can be computed,
and matching analyzer config/pins at the trusted base, described in
[the operational guide](../../docs/go-analysis.md). No committed baseline is
needed: the merge base's own scan decides what is already debt. The retired
`bootstrap-revision` input is refused if set, so a stale caller is noticed.

The action executes code from its own pinned ref. Config/pins in the analyzed tree
and at the merge base must match that version. Updating policy or tools requires
an explicitly reviewed action-pin migration; a PR cannot silently make its checker
permissive. Desk-tools/35 must add an explicit human-reviewed migration lane before
enabling gosec.
Do not pass a PR-controlled ref as `base-ref`. The caller's workflow permissions,
fork approval, runner isolation and merge protection still govern execution;
a composite action cannot protect a workflow that a caller permits a PR to replace.

## Inputs and runtime

| Input | Default | Purpose |
|---|---|---|
| `root` | `.` | Git checkout to analyze. |
| `base-ref` | `main` | Trusted base ref in the calling repository; the head is compared with its merge base against this ref. An explicit recorded SHA is also accepted. Unused in report mode. |
| `mode` | `gate` | `gate` compares merge base and head; `report` lists every finding and never fails on findings. |
| `bootstrap-revision` | empty | Retired; a non-empty value is refused. |
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
`always()` and do not convert it to success. Missing or malformed reports fail, and so does a report whose mode differs from
the requested `mode`: a debt report can never stand in for the gate.

Only sanitized `report.json` is uploaded as
`go-analysis-lint-<source-sha>-<run-id>-<attempt>`, retained for 14 days. No raw source
snippets or diagnostics are uploaded. The [report contract](../../docs/go-analysis-report-v1.md)
defines provenance and valid comparisons. Security scanning and qualgen ingestion
are the dependent desk-tools/35 implementation, not claimed as delivered here.

## Development

Run `python3 -m unittest discover -s actions/go-analysis -p 'test_*.py'` for the
wrapper's offline controls. The underlying real-analyzer and merge-base comparison tests remain
`bash scripts/tests/go-analysis.sh lint` with the acquired cache set through
`GO_ANALYSIS_TEST_CACHE`. The human-applied [CI runsheet](../../docs/runsheets/go-analysis-ci-hunk.md)
contains the exact thin consumer workflow patch.
