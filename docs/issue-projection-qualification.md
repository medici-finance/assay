# Local issue-projection qualification

Source qualification uses the real issueboard CLI, parser, typed GitHub adapter and socket-free fake HTTP provider. The matched legacy arm retains the unchanged full/default and role-filter forms; the subject selects work-ready, intake and addressed-inbox before enrichment. Each complete serial cycle runs all three roles, with the old broad worker read disabled in the subject. Base source: `ecb8c74`; initial subject source: `4a11262`. Both use the same synthetic data, serial bounds, no cache and prebuilt command test binaries. This is a diagnostic cohort, not an accepted CPU/latency tradeoff or a fleet SLO.

| Issues | Arm | Samples | HTTP attempts per cycle | Wall median (ms) | Direct-child own CPU median (ms) |
|---|---|---|---|---|---|
| 8 | legacy | 3 | 55 | 59.870 | 51.719 |
| 8 | subject | 3 | 51 | 63.403 | 51.805 |
| 80 | legacy | 3 | 172 | 66.313 | 60.335 |
| 80 | subject | 3 | 132 | 66.348 | 57.713 |

Counts are actual attempts at the production HTTP transport, including the required full intake read and the already-narrow legacy addressed inbox. The 14-repository fixture includes empty repositories. At 8 issues, attempts fall from 55 to 51; at 80, from 172 to 132. CPU and wall vary in both directions, so these samples support a request-work reduction only. No concurrency increase is justified. The first legacy sample includes an OS/runtime cold outlier; medians retain it rather than deleting it.

Build and fixture setup are excluded. CPU is the OS user+system time of each direct command test process, including its test harness; descendant, inclusive and model CPU are unknown. Provider pages are synthetic and complete in this cost arm; separate fault qualification exercises permission failure, incomplete histories and positive closure. No real GitHub quota/GraphQL point cost is observed, and no synthetic value is labeled billable cost. Independent concurrent windows and a live deployment are unqualified.

The human-inbox shell journey separately proves a compatible producer removes all four per-label CLI reads, preserves its rendered population, refuses incompatible producers before discovery, retains partial exit 2 without fallback, and supports explicit legacy rollback. Its screen, body/comment detail reads, cards and flow stay on the existing path. A production deskinbox adapter test records one actual local HTTP listing with no detail request; the shell compatibility test uses a local producer shim, so it is not a measurement of GitHub quota.

Reproduce with the named tests in the brief plus `GOWORK=off go test -count=1 -timeout 90s -v -run '^TestProjectionCost$' ./internal/reviewtest` from `tools/desk`. `ASSAY_ISSUE_REPORT` optionally saves local numeric samples. No report or telemetry is uploaded. The shared perf-v3 producer/context/request owners and desk-tools/38 verification remain completion holds.
