# Statusgen performance workload

Run the opt-in, credential-free workload from the repository root:

```sh
statusgen/bench/perfworkload.sh /path/to/report.md 20
```

The script runs one bounded named Go test with module downloads disabled and
`GOWORK=off`. It creates synthetic small (2 streams × 4 briefs) and large
(12 × 20) repositories, isolated homes, and fake process adapters. It never
queries a provider or uploads records. `sizes.json` and `brief.md` in
`testdata/perfworkload/` declare the corpus. The source revision and environment
are included in the report; run from a committed source tree for reproducibility.
The report path is an explicit local output, not a new statusgen option.

Two boundaries are deliberately reported separately:

- **Owner replay:** cold clears the existing read and brief-parse memos; warm
  replays the same inputs in the same process. Every pair is compared against a
  memo-cleared reference using the same canonical parser/evaluator. The comparison
  includes stdout, stderr, exit, generated files and the coverage JSON, including
  held and unavailable results. Wall and process CPU include capture/comparison
  overhead. Only disk attempts through `readFileMemo` and `readDirMemo` are counted;
  other file reads remain unmeasured. Brief parses use `briefParseCount`.
- **Compiled CLI:** separate fresh processes exercise the real flag parser and
  `exitProcess` recorder in lint, regen, gate-scores and next-up. Each selected-profile
  run must match recording-off output, exit and generated files. The shared
  recorder supplies process wall and CPU; these rows have no cross-process warm
  semantic cache and no complete file-read counter. A compiled fake-deskread run
  separately checks actual child starts, phase count and unavailable results.

CPU is the parent's user+system CPU; it excludes child CPU. Parent inclusive wall
and child wall must never be summed as savings. The fake child returns unavailable
and supplies no request observations, so no request-cost zero is inferred.
The offline help probe is excluded from actual read-child counts.

No OS page-cache reset is attempted. These measurements are local synthetic
observations, not a cold-storage, installed-binary, fleet or deployment claim.
There is no numerical performance target and no new cache to benchmark as an
optimization. Warm replay can legitimately show no useful reduction.

## Repeated-input inventory

Inspected at main `8663d6ae0`, after the phase producer in #3759 and the offline
context resolver in #3839. The runtime reuse below already existed at pickup.
This follow-up qualifies it rather than introducing another cache or parser.

| Mode / owner | Repeated work and existing reuse | Boundary retained |
|---|---|---|
| lint / `run` | Independent checks enumerate `briefFilePaths` and call `parseBriefFile` repeatedly. The directory/file memo validates size, mtime, ctime, inode, device and a settle interval; the parser keys results on path plus the hash of the bytes actually read. | Changes and errors cause fresh reads; no HEAD-only decision. Directory membership is enumerated independently of README rows. |
| regen / `run` write mode | Same checks and parse owner as lint, followed by canonical rendering and generated writes. | Output/exit/coverage parity includes generated file bytes. Writes do not authorize reusing a previous read set. |
| gate-scores / `runGateScores` | `loadHydratedStreams` supplies the canonical brief hydration; queue/risk/coverage readers may request those briefs again. Existing parse results serve identical content. | Scores and classification remain with their owners. No aggregate evaluation-result cache is added. |
| next-up / `runNextUp` | Same hydration; eligibility, coverage and dispatch views consume it through their existing adapters. | Claims and unavailable evidence retain their current semantics; results are reevaluated rather than cached by HEAD. |
| README / findings / other direct reads | Existing readers may read these again outside the memo seam. | These are unmeasured by the owner-local file counter; no new read cache or savings is claimed. |
| context / `contexthandoff` | Not invoked by these four modes. `LoadSnapshot` captures the explicitly supplied sources, and `newIndex` independently enumerates all available source files for each discover/recheck/check operation. | No index-reuse API is shipped. Snapshot maps are caller-supplied input, not proof of immutability. A later consumer may reuse only source-bound immutable bytes/results under compatible parser/evaluator versions and must still enumerate new membership. |

`TestInvocationSnapshotReuseParity` exercises the production mode owners against
fresh references, including same-HEAD dirty contents with restored mtime, added
unlisted/untracked consumers, deletion and a new stream. A direct directory-owner
check catches a new consumer outside the prior parse set. Go overlays plant a
path-only parse key and a prior-readset lookup into the actual production owners;
the named test must fail for each, and compilation failure does not count.
`TestPerfReadCountCoverage` pins the file/directory attempt split, including read
errors, and kills omitted-increment overlays. No production guard is weakened.

## Architecture pickup

Disposition: **continue** within the remaining statusgen/26 obligations. The
brief is version 3 at `8663d6ae0`; #3759 is recorded as partial delivery.
`docs/library-first.md`, `docs/go-engineering.md`, `docs/contracts.md` and the
performance-attribution spec were inspected at that source revision. Existing
`perfrec`, `parseBriefFileBytes`, `readmemo`, `loadHydratedStreams`, eligibility
and coverage remain the semantic owners. The later forge-library transition is
separately gated under desktools-v2/75–77; this change imports no credential
provider, desk internals or authenticated adapter and retains the deskread process
boundary. The source-query service document is a draft, not a shipped reuse API.

The existing context resolver and its new-consumer/partial-source controls remain
unchanged. No active owner is replaced, no scope or claim authority is moved, and
no lifecycle or independent Evidence is authored here. Standard-library test and
reporting helpers suffice; no dependency is installed, upgraded or promoted.
