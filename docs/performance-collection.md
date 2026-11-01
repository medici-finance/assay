# Local performance collection (perfdata)

`perfdata` records bounded performance observations into a local spool and reports on them
offline. It has no network code, no database dependency and no listener, and it does not run
as a service. It uses the standard library only. The record format, the batch and the receipt
are specified in [`spec/performance-event-v1.md`](../spec/performance-event-v1.md).

This is a separate instrument from statusgen's counts-only `telemetry-v1` payload and from the
`perf-v1`..`perf-v4` local history that `perfrec` keeps. Neither of those changes. Human
acceptance of the field allowlist, the loss envelope and the writer overhead below is still
pending. Until that acceptance exists, treat this as an implemented, unaccepted local
capability. Nothing here starts collection on any machine.

## Build

```sh
cd perfdata && GOWORK=off go build ./cmd/perfdata
```

The module has its own `go.mod` and builds with no PostgreSQL, HTTP or cell dependency.

## Turning it on

Recording is armed only when both conditions hold in one process:

1. `ASSAY_TELEMETRY` is exactly `1`; and
2. a profile is selected explicitly, either with `--profile tool|workflow` or with
   `ASSAY_PERFDATA_PROFILE`. The flag wins.

With either condition missing, `perfdata record` prints `collection off (<reason>)` on stderr,
exits 0 and creates nothing. It writes no directory, no file and no source identity. The
reasons are `telemetry-off`, `no-profile` and `unknown-profile`. An existing telemetry-v1 or
perf-v1 opt-in (`ASSAY_TELEMETRY=1` alone) never selects a profile, so it never turns this
on.

| Profile | Records |
|---|---|
| `tool` | Fixed-vocabulary measurements, build and platform, a random source identity and an unlinked invocation alias. No session, agent, attempt, role or work linkage. |
| `workflow` | Everything in `tool`, plus aliased correlation (parent invocation, session, agent, attempt, shared refresh, desk role, assigned role and brief/issue references) and the `attempt` and `usage` kinds. |

**Workflow records are pseudonymous, not anonymous.** Every work, session and agent locator is
replaced by a random alias before it is written. The alias map stays in the local store, and
the source identity links all of one source's records. Raw argv, paths, titles, bodies, raw
errors, credentials, transcripts and native harness IDs are never written. A draft that tries
to carry them is refused, and the refusal does not echo the input.

## Use

A policy file chooses the profiles and tools admitted, the age cap and the byte cap. A byte cap
is required. There is no default.

```json
{"schema":"performance-policy-v1","revision":"local-1","profiles":["tool"],
 "tools":["statusgen"],"ageCapDays":7,"byteCap":67108864}
```

Drafts are one JSON object per line: a `kind`, an optional `occurredAt`, a raw `ctx` and a
typed `payload`. The writer assigns the identity, times, build and policy revision, and it maps
raw `ctx` locators to aliases.

```sh
printf '%s\n' '{"kind":"tool.completion","ctx":{"invocation":"run-1"},"payload":{"tool":"statusgen","verb":"lint","outcome":"ok","startedAt":"2026-11-01T12:00:00.000Z","wall":{"basis":"measured","ms":120},"cpu":{"own":{"basis":"measured","ms":40},"descendant":{"basis":"unavailable"},"inclusive":{"basis":"unavailable"}},"children":{"started":0,"uninstrumented":0}}}' > drafts.jsonl

ASSAY_TELEMETRY=1 ./perfdata record --dir "$STATE" --policy policy.json --profile tool --input drafts.jsonl
./perfdata report --dir "$STATE"            # text
./perfdata report --dir "$STATE" --json     # performance-report-v1
```

`record` prints the exact stored lines, so the preview is the bytes on disk. `--json` wraps
them as `{recorded, events, refused}`. `report` accepts `--from`/`--to` (UTC days) and an
optional `--policy` to stamp the snapshot's policy revision. A report gives:

- per-source coverage;
- per-day state;
- pooled tool wall and own-CPU statistics;
- elapsed time as the union of intervals, with summed wall time beside it;
- usage per series;
- unfinished attempts;
- HTTP, service and handler totals;
- collection health counters and retirement loss.

Unknown values are counted as unknown, never as zero.

## Where data lives

Everything is under `<dir>/performance-events/`: `manifest.json`, `source.json`,
`aliases-e<N>.json`, `coverage.json`, `lock`, `run/`, `segments/` and `distributions/`.
Directories are 0700 and files 0600. Metadata is replaced atomically. This tree is separate
from `audit.jsonl`, `dispatch-records.jsonl` and `perf/`, and turning collection off never
affects those.

## Loss and failure envelope

- **Never blocks the caller.** The library sink has a bounded queue (default 256). A full
  queue drops the draft and counts `queueFull`. A failed write, such as a full disk, counts
  `writeFailed` and never reaches the caller's result. Recording cannot change an operation's
  exit code or output.
- **Flush budget.** Closing the sink waits at most `flushBudgetMs` (default 250 ms, at most
  10 s). Drafts still queued after that are lost and counted as `flushTimeout`. Their live
  marker stays behind, so the next report shows an unclean termination and partial coverage.
- **Crash-loss envelope.** Appends do not fsync on the work path. A crash or kill can lose
  queued drafts and the tail of the last write. A reader shows that loss as a partial tail, an
  unexplained sequence gap or an orphaned live marker, never as complete coverage. If the disk
  is full, the record of the loss may not be written either. That loss then stays unexplained
  but visible as a gap.
- **Torn tails.** A torn last line is skipped and counted. After a crash, the next writer
  starts a new segment. If the lazily advanced source metadata had not yet passed the torn
  record, that record's sequence number can be given to a new event. The torn line never
  decodes, so no two readable records share an identity.
- **Identity across crashes.** If the source metadata is lost, or if it is found behind the
  newest segment, a new epoch starts. A lost alias map also starts a new epoch, so old and new
  aliases can never be joined. Each of these resets is recorded and reported.

## Retention

- Raw segments are retired after `ageCapDays` UTC days (1–7, initially 7).
- When the store exceeds `byteCap` (at most 1 GiB), the oldest segments are retired. The cap
  is checked when a new segment starts (default segment size 1 MiB), so the store can exceed
  the cap by about one segment.
- The alias map (`aliases-e<N>.json`) grows with each distinct locator in an epoch and is
  not counted against `byteCap`. A long-lived workflow source should expect it to grow. A new
  epoch starts a new map. Old maps remain until the store is removed.
- Nothing in this tool sends data, so **every retirement is unsent loss**. The coverage
  record keeps its sequence range, day window, event count and bytes.
- With `distributionDays` (0–90) set, expired raw tool completions are first folded into
  daily `log2ms-v1` rollups. A rollup keeps up to 4096 exact samples. Past that, P95 is a
  labelled bin estimate. Pooled average and P95 come from the merged distribution, never from
  averaging daily figures. A day whose rollup has also expired reads `retired`, never zero.

## Writer cost

Measured by `TestLocalCostAndBounds` on darwin/arm64, with a 729-byte tool-completion event
and 300 runs:

| Measure | Median | p95 | Max |
|---|---|---|---|
| Locked append (open-segment write under the lock) | 430.75 µs | 557 µs | 988.6 µs |
| Producer cost, collection off | 42 ns | — | — |
| Producer cost, collection on (enqueue) | 958 ns | 1.625 µs | 47.6 µs |

Under a deliberately slow writer, recording returned in a total of 192 µs, and the close
finished in about 51 ms, inside its budget. The excess was dropped and counted (QueueFull 184,
FlushTimeout 8). These numbers come from one reference machine and are evidence for the
pending human decision. They are not a portability guarantee.

## Fixtures

`perfdata/testdata/api/` holds the batch and receipt conformance cases. `perfdata/testdata/events/`
holds valid stored events. The files are compared byte for byte by digest, so
`.gitattributes` marks them `-text`, and checkouts must not convert their line endings.

## What this does not do

It does not upload data, listen for connections, write to a database, change the
telemetry-v1 endpoint or switches, or change perfrec's history. A future sender, collector and
optional database (the later briefs of this stream) consume the batch and receipt contract
and the conformance fixtures. They do not change the meaning of what is already recorded.
