# Statusgen synthetic workload

Source: `6dcacca23f8262c9049c3ca609542466b7fc1091`. Environment: `go1.27.2`, `darwin/arm64`. Samples per row: 20.

Cold means cleared invocation-local read/parse memos; warm is the immediately following replay over the same inputs in the same process. OS caches are uncontrolled. No persistent cache or deployment speedup is measured. The measured boundary includes the operational owner, coverage projection, output capture and generated-file comparison. CPU is process user+system, without child CPU. File/directory counts cover only readFileMemo/readDirMemo disk attempts (including errors); other file reads are unmeasured. Parses count canonical brief parses. Every measured replay matches a memo-cleared reference byte-for-byte for stdout, stderr, exit, generated files and coverage. Each operational mode and coverage projection must exit zero, and cold parsing must cover every declared brief while warm parsing performs zero new parses.

| Corpus | Mode | State | Wall ms p50 / p95 | CPU ms p50 / p95 | Memo file reads p50 / p95 | Memo dir reads p50 / p95 | Brief parses p50 / p95 |
|---|---|---|---|---|---|---|---|
| small (2 × 4) | lint | cold | 43.480 / 44.954 | 14.000 / 15.000 | 11.000 / 11.000 | 2.000 / 2.000 | 8.000 / 8.000 |
| small (2 × 4) | lint | warm | 42.397 / 45.157 | 13.000 / 14.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| small (2 × 4) | regen | cold | 52.386 / 59.199 | 15.000 / 17.000 | 11.000 / 11.000 | 2.000 / 2.000 | 8.000 / 8.000 |
| small (2 × 4) | regen | warm | 49.817 / 65.851 | 14.000 / 16.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| small (2 × 4) | gate-scores | cold | 10.320 / 12.058 | 3.000 / 4.000 | 8.000 / 8.000 | 2.000 / 2.000 | 8.000 / 8.000 |
| small (2 × 4) | gate-scores | warm | 9.643 / 12.081 | 3.000 / 7.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| small (2 × 4) | next-up | cold | 28.818 / 38.929 | 7.000 / 10.000 | 8.000 / 8.000 | 2.000 / 2.000 | 8.000 / 8.000 |
| small (2 × 4) | next-up | warm | 26.994 / 37.784 | 6.000 / 9.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| large (12 × 20) | lint | cold | 210.224 / 365.886 | 132.000 / 211.000 | 253.000 / 253.000 | 12.000 / 12.000 | 240.000 / 240.000 |
| large (12 × 20) | lint | warm | 195.824 / 220.002 | 113.000 / 141.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| large (12 × 20) | regen | cold | 214.145 / 224.284 | 129.000 / 137.000 | 253.000 / 253.000 | 12.000 / 12.000 | 240.000 / 240.000 |
| large (12 × 20) | regen | warm | 194.578 / 201.882 | 111.000 / 115.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| large (12 × 20) | gate-scores | cold | 36.622 / 39.320 | 32.000 / 35.000 | 240.000 / 240.000 | 12.000 / 12.000 | 240.000 / 240.000 |
| large (12 × 20) | gate-scores | warm | 18.957 / 21.220 | 16.000 / 18.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| large (12 × 20) | next-up | cold | 126.454 / 131.171 | 53.000 / 54.000 | 240.000 / 240.000 | 12.000 / 12.000 | 240.000 / 240.000 |
| large (12 × 20) | next-up | warm | 100.893 / 104.980 | 36.000 / 39.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |

The compiled CLI rows below are separate fresh processes using the shared selected-profile record. Every sample has a recording-off reference and generated-file parity check. They measure the process-init boundary, including the real flag parser and exit recorder, after Go runtime startup. Warm OS state is uncontrolled; invocation memos cannot cross this boundary. Read/parse counts for these process records are unavailable; the owner-local replay rows above describe those counters separately.

| Corpus | Compiled CLI mode | Process wall ms p50 / p95 | Process CPU ms p50 / p95 | Actual deskread starts |
|---|---|---|---|---|
| small | lint | 57.000 / 59.000 | 18.000 / 20.000 | 0 |
| small | regen | 66.000 / 104.000 | 18.000 / 23.000 | 0 |
| small | gate-scores | 5.000 / 6.000 | 6.000 / 8.000 | 0 |
| small | next-up | 34.000 / 128.000 | 10.000 / 17.000 | 0 |
| large | lint | 249.000 / 260.000 | 126.000 / 130.000 | 0 |
| large | regen | 255.000 / 264.000 | 126.000 / 130.000 | 0 |
| large | gate-scores | 27.000 / 30.000 | 30.000 / 32.000 | 0 |
| large | next-up | 159.000 / 164.000 | 49.000 / 51.000 | 0 |

Compiled fake-deskread parent (20 recorded samples; each checked against a recording-off run): process wall ms p50/p95 66.000 / 71.000; actual child starts per run 1 / 1. The fake child returns unavailable (exit 6), preserved as unavailable; it reports no request costs. Help probes are excluded from actual child starts and child phases.
Parent process CPU ms p50/p95 14.000 / 18.000.

Coverage limits: synthetic local corpora, no providers or live permissions, no controlled OS-cold disk, no installed fleet baseline. Context discovery does not run inside these four modes: no context index reuse or savings is claimed.
