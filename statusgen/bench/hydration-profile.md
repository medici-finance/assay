# Hydration profile (statusgen/44)

commit: 1db3085ddbdc8314ab46cdcd8a0ac4e256580d68
host class: Apple M4 Max, darwin/arm64, 16 CPUs, 128 GB; load average about 16 during the runs (a busy workstation, so absolute times are pessimistic and noisy; ratios and counts are the signal)
tree: this repository at the commit above: 38 streams plus 2 archived, 623 brief files, 40 of them waiting on a recorded verifier FAIL

dominant: newestFailFix (statusgen/stalefail.go), reached from waitingBriefNotice (statusgen/brieffile.go) inside checkBriefFiles, which loadHydratedStreams (statusgen/load.go) calls only for its side effect of populating Brief frontmatter and whose notices it throws away

## Wall times, in-process, best of three (`TestHydrateRealTree`)

| Load | Before | After |
|---|---|---|
| bare `loadStreams` | 6.3 ms | 5.0 ms |
| hydrated `loadHydratedStreams` | 4585 ms (730x bare) | 148 ms (30x bare) |
| git history queries during hydration | 40 | 0 |
| brief parses during hydration | 623 | 623 |

Compiled CLI, one process, same tree (`--gate-scores`): 15.4 s and 15.9 s before; 0.39 s and 0.46 s after. Standard output of `--gate-scores` and `--launch` is byte-identical before and after.

## Top ten by cumulative time, before the change (3.54 s of samples)

1. `statusgen.checkBriefFilesMode` (`checkBriefFiles`): 3.19 s, 90%
2. `statusgen.waitingBriefNotice`: 3.03 s, 86%
3. `statusgen.newestFailFix`: 2.97 s, 84%
4. `statusgen.ordinaryGitCore` (opens the repository, reads its config and replacement refs per query): 1.64 s, 46%
5. `go-git (*Repository).References` / `dotgit.walkReferencesTree` (the ref directory walk under that): 1.28 s, 36%
6. `gitcore.(*Repo).NativeReplacementReadReason`: 0.95 s, 27%
7. `statusgen.gitCoreFor` / `observeHistoryProfile` / `referenceGitCore`: 0.89 s, 25%
8. `gitcore.(*Repo).NativeDiscoveryReason`: 0.85 s, 24%
9. `go-git (*ConfigStorage).Config`: 0.83 s, 23%
10. `os.ReadDir` under go-git `osfs.readDir`: 0.72 s, 20%

The terms that remain after the change are the per-brief open, stat, read and YAML parse (`readFileMemo`, `statStamp`, `parseBriefFile`), about 0.15 to 0.2 s for 623 files, almost all of it syscalls. Each is below the brief's 20% rule only because the wall time collapsed; they are the floor of hydration, which has to read every brief.

## Why the synthetic corpus missed it

The old workload corpus is not a git repository, so `newestFailFix` returns before it queries anything, and none of its briefs carry a FAIL. The `real-shape` profile added to the workload (`testdata/perfworkload/sizes.json`) is a git repository with an `origin/main` ref, 24 first-parent merges and implemented briefs with a recorded FAIL. On it `--gate-scores` takes 2.5 s in-process (1.6 s compiled) before the change and 11 ms (9 ms compiled) after.

## Second pass: the brief's 10x restored (issue #6900)

base: 9c845402c8c0, the merged first pass, with the budgets loosened to 20x (synthetic) and 100x (real tree)
host class: the same Apple M4 Max, 16 CPUs; load average 19 to 35 during the runs
tree: this repository at that commit, 642 brief files

With the history queries gone, a serial profile of hydration put 46% of its samples in the `open` system call itself: about 120 to 160 µs per brief file on this host, against about 3 µs for a `stat`. Each cold read also made two path stats around the open. Placeholder discovery listed every stream directory twice more with `filepath.Glob`. Three changes, none touching what is parsed or reported:

1. `parseBriefFilesAhead` (`brieffile.go`) reads and parses every brief file of every stream concurrently (`GOMAXPROCS` workers) before `checkBriefFilesMode` walks the streams in their usual order. The walk consumes the results in place of its per-file `parseBriefFile` call, so notices, holds and could-not-checks come out in the same order as before.
2. `readFileMemo` (`readmemo.go`) path-stats only when it holds an entry to validate. A read that goes to disk stamps the open descriptor (`fstat` before and after the read), so a cold read costs one path lookup instead of three. The stamp, the settle window and the before/after comparison are unchanged.
3. `placeholderFilePaths` (`placeholder.go`) matches both placeholder globs against one `readDirMemo` listing, which `briefFilePaths` already reads, instead of two `filepath.Glob` directory scans per stream.

Hydrated over bare, in-process (`TestHydrateBudget`: best of nine; `TestHydrateRealTree`: best of nine after this change, best of three before it):

| Row | Before | After |
|---|---|---|
| synthetic corpus, 32 briefs | 12.4x, 12.5x | 4.9x to 7.3x |
| real tree, 642 briefs | 24.1x, 33.4x | 7.2x to 8.2x |

Worker count matters on this host: one worker gave 28x to 32x on the real tree, four gave 11x to 16x, and sixteen (`GOMAXPROCS`) gave the best results. Concurrent opens scale only about fourfold here, so the per-open cost, not the parse, is still the largest single term. It is the floor: hydration has to read every brief.
