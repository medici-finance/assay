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
