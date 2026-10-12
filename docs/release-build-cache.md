# Release build cache: measurement and implementation plan

This is a dispatch plan. No cache or build scheduling change has been implemented yet.

## Observed baseline

The v1.0.41 dry run [38313808379](https://github.com/medici-finance/assay/actions/runs/38313808379), at source ca0dad5c4162a7203167340c49d4c241e14060bb, recorded:

| Release job step | Elapsed |
| --- | --- |
| Install Go | 3 seconds |
| Build statusgen binaries | 59 seconds |
| Build qualgen binaries | 22 seconds |
| Build and package desk-tools binaries | 7 minutes 54 seconds |
| Entire release job | 10 minutes 10 seconds |

The desk-tools recipe invokes go build separately for each of 68 command directories on each of five target platforms. These invocations run in nested serial loops. Shared packages can reuse Go's package cache within the job; these are not 340 independent full dependency compilations. The release job log contains 71 module download messages. The workflow declares no explicit persistent build-cache restore. The step timings do not separate compilation, linking and compression.

## Authorized scope

Persist reusable Go module downloads and compiled package outputs for the release build, while keeping mutation-test caches separate. Preserve the existing source, toolchain, version-stamping, asset, checksum, release authorization and publication contracts. Inspect the runner configuration available in source; do not contact live infrastructure. No new shared-library packaging or command architecture is required.

Measure the same source and target with cold and warm owned caches, recording toolchain, target, flags, elapsed time, cache sizes and executable inventory. Measure changed source and release stamp behavior as well as repeated identical builds. Do not describe a local measurement as release-runner evidence.

Use these measurements to decide whether batched command builds or bounded platform parallelism warrants an additional change. Preserve output names and every currently shipped platform. Record each alternative and its measured outcome. Enumerate the active and staged workflow consumers and keep their intended relationship consistent.

## Delivery checks

- Cache absence or a cache-service failure still permits a correct cold build.
- Cached outputs remain scoped to the appropriate build inputs and trusted release path; unrelated pull-request or mutation runs cannot populate the release cache through this change.
- Source and version changes produce correctly rebuilt and stamped binaries.
- Repeated builds have complete artifact inventories, including Windows executable suffixes.
- Existing release gates and permission boundaries remain intact.
- Record before/after evidence and remaining could-not-check items here, and update the PR description to describe the final implementation.

This follows the release-duration work in issue #138; that issue's earlier test-matrix change is already delivered and is not being reimplemented here.
