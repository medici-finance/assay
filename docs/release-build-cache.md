# Release Go cache and batched command builds

The release job can reuse Go module downloads and compiled package outputs from
an immutable artifact produced by an earlier successful release job. Each platform
now builds all desk commands in one Go invocation. Platforms remain serial.

## Trust and failure boundary

This does not use the Actions cache service, a cache key prefix, a runner mount,
or a shared test cache. A separate cache artifact uses the already pinned
`actions/upload-artifact` action. Restore accepts all of these together:

- The same repository's successful, completed `release.yml` main-branch manual
  dispatch, from the latest five successful dispatches; never the current run.
- An ancestor source commit whose release workflow and cache adapter are byte
  identical to the current producer contract. A contract change causes a cold
  build until a new producer succeeds.
- Exactly one successful `release` job from the recorded run attempt, including
  successful upload and receipt steps. The job's log records the artifact ID and
  SHA-256 digest returned by the pinned upload action.
- That exact unexpired artifact ID, run/source metadata and digest, followed by
  verification of the downloaded bytes and toolchain/host manifest.

Artifact names alone are insufficient: another job can choose the same name.
The producing job's log binds the ID and digest independently of the archive.
[The pinned action's contract](https://github.com/actions/upload-artifact/blob/b7c566a772e6b6bfb58ed0dc250532a479d7789f/README.md)
states that archives are immutable and replacement creates a new ID. A replacement
therefore cannot match the receipt. This is a forge-mediated producer check,
not a cryptographic provenance attestation.

The release job retains its environment gate, existing contents/PR permissions,
source and toolchain checks, stamps, artifact names, checksums and publication
ordering. It adds **`actions: read`**, solely to read prior artifacts and job logs;
no new credential or write privilege is introduced. Anonymous access to a public
job-log endpoint returned HTTP 403 during qualification. The
[API documents the read permission](https://docs.github.com/en/rest/actions/workflow-jobs#download-job-logs-for-a-workflow-run).

Restore creates isolated job-local `GOCACHE` and `GOMODCACHE` directories before
release Go commands. Test and mutation jobs never use them. Missing, expired,
unreadable, incompatible or corrupt cache data is a cold miss, and so is any
transport or decoding failure, including a truncated response. The one refusal
is a cache directory that already exists on the runner before restore: it is
never adopted unvalidated, and the step fails instead. Reads have a
60-second total deadline, individual network timeouts, five-candidate limit,
2 GiB archive limit and 8 GiB expanded limit. Extraction rejects traversal,
links, duplicate members and unexpected roots, then exposes the result atomically.

Save is optional, bounded to two minutes per packaging/upload step, and happens
only for main dispatches that are **not dry runs**. It uploads a seven-day artifact,
never a release asset, tag or package image. Readers additionally require the
whole producer run to have succeeded. Cache availability is not a release gate.
Dry runs can restore but upload no new cache. A cache reset, toolchain change or
source change still uses Go's normal input-sensitive rebuild rules.

## Measurements

Release baseline: [run 38313808379](https://github.com/medici-finance/assay/actions/runs/38313808379),
source `ca0dad5c4162a7203167340c49d4c241e14060bb`, Go 1.25.0 on Linux amd64.
Install Go took 3 seconds, statusgen 59 seconds, qualgen 22 seconds, desk-tools
474 seconds and the entire release job 610 seconds. There were 71 module download
messages. The old recipe made 68 command invocations per platform, 340 total;
shared packages could already reuse the within-job Go cache.

Local qualification used tracked source `f6f81ecb0cd3803168eda781de988debb8d64f4b`,
Go **1.27.1**, Darwin arm64, `GOMAXPROCS=4`, `CGO_ENABLED=0`, `GOWORK=off` and
`GOFLAGS=-mod=readonly`. These are local measurements, **not release-runner speedup
or Go 1.25 parity evidence**. Source/toolchain paths, CPU and linking behavior differ.
The release workflow does not change its CGO policy or pinned Go version.

| 68-command case | Seconds |
| --- | ---: |
| Native serial, empty build and module caches | 41.012 |
| Native serial, same-source warm caches | 26.406 |
| Native batched, same-source warm caches | 4.468 |
| Linux amd64 batched, target cold / warm | 11.037 / 4.731 |
| Darwin amd64 batched, target cold / warm | 11.223 / 4.316 |
| Windows amd64 batched, target cold / warm | 11.392 / 4.175 |
| Windows arm64 batched, target cold / warm | 11.427 / 4.116 |

Later target-cold cases reused module downloads from the native case, but had no
compiled packages for their target. Fresh output directories forced complete
inventories. All 68 native output hashes matched across serial cold, serial warm
and batched warm builds. Each other platform's cold/warm hashes also matched,
including all Windows `.exe` names. A separate execution of the exact new
`./cmd/...` workflow pattern preserved all 340 target binary hashes. A changed release tag rebuilt `deskpr` and
`cellctl` with the new value; a changed source return value appeared in a rebuilt
`deskpr` using the same warm cache. Cross-compiled binaries were not executed.

The five-platform cache occupied 1,856,409,600 bytes as tar and 588,443,676 bytes
as a level-1 zip. Local packaging took 7.020 seconds, compression 11.119 seconds,
and digest verification plus transactional restore 6.021 seconds. **Network
transfer was not measured.** These costs can outweigh compilation savings;
artifact persistence is not a claim that every release becomes faster. The next
human-authorized releases must compare restore/save overhead and total build time.
The first run is necessarily cold; no workflow was dispatched for this change.

[Retained measurement data](qualification/release-cache/measurements.json) records
flags, inventories, per-binary hashes and observed version output. Each case's
`inventory_sha256` list holds the SHA-256 of the binary at the same position
in its `inventory` list. The native
commands were equivalent to the workflow's original per-command loop and the
new `go build -ldflags "$LDFLAGS" -o "$stage/" ./cmd/...` batch, with explicit
`GOOS`/`GOARCH` per target and fresh owned caches/output directories.

Batching has direct local evidence and changes neither command architecture nor
artifact contracts. Additional platform parallelism was not added: the batch
already schedules Go work concurrently, and local results do not qualify extra
runner memory/CPU contention. A prepared builder image or separate storage
service would add a publishing/permission boundary; neither is needed here.

## Consumers and checks

- `.github/workflows/release.yml`: active producer/consumer, five platforms for
  statusgen, qualgen and desk-tools; Cursor bundle and validation pack unchanged.
- `tools/release/release.yml.proposed`: same cache and batching delta, preserving
  its independent spec-header proposal.
- `ci/staged-workflows/release.yml`: same delta, preserving its independent
  release-authorizer proposal.
- `tools/changelog/release.yml.proposed`: obsolete historical snapshot; its banner
  explicitly forbids overwriting the newer live workflow.
- Test/mutation workflows and container builds: unchanged, no release-cache use.

Run `python3 -B -m unittest discover -s tools/release -p go_cache_test.py -v`
and `python3 -B tools/release/go_cache_mutation.py` from the repository root.
The existing unconditional `build-test` job also runs this suite through the desk
regression package. It covers producer metadata and receipts, real loopback HTTP,
cache failures (a truncated response included), extraction and all three
workflow consumers. Seven planted
mutations removing workflow/job/ID/expiry/digest/path/source-contract checks each
made the relevant fixture fail. CLI inventory, trigger coverage and existing
release-stamp checks remain enforced.

Live runner restoration, storage egress and measured end-to-end release benefit
remain **could-not-check** until a human-authorized release executes the change.
This implements the release-performance follow-up in #138; its previous test
matrix work is not repeated.
