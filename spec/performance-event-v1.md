# performance-event-v1 — the local performance record, batch and receipt

**Version:** v1.0-draft
**Status:** DRAFT, proposed. Published for review. v1.0-draft is unstable: breaking changes MAY
be made without a major-version bump. Human acceptance of the field allowlist, the local loss
envelope and the measured writer overhead is pending; nothing here activates collection or a
remote destination.
**Describes reference implementation:** the `perfdata/` module (`perfdata`, `perfdata/local`,
`perfdata/report`, `perfdata/cmd/perfdata`).
**Machine-readable schemas:** [`schemas/performance-event-v1.json`](../schemas/performance-event-v1.json)
and [`schemas/performance-batch-v1.json`](../schemas/performance-batch-v1.json) (the receipt is
`$defs/receipt` there). The schemas are structural. Where this document or the Go validator
(`perfdata.DecodeEvent`) is stricter, the stricter rule wins. `TestSchemaFiles` keeps the
schema key sets equal to the Go types.

This contract describes one bounded performance observation, how a local store keeps it, the
batch a future sender transmits and the receipt a future collector answers with. It is separate
from `telemetry-v1` (statusgen's counts-only payload) and from the `perf-v1`..`perf-v4` local
history kept by `perfrec`. Neither of those changes meaning, switch or retention because this
contract exists.

The key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 1. Rules every reader keeps

1. **Missing is not zero.** A measurement has an explicit basis. `unavailable` carries no value.
   Every other basis carries one. A reader MUST NOT turn an unavailable value, an absent token
   count or an unrecorded day into 0.
2. **Nothing free-form crosses the boundary.** Every string in an event is an enum, a bounded
   token (`^[a-z][a-z0-9-]{0,31}$`), a revision token (`^[a-z0-9][a-z0-9._-]{0,31}$`), a random
   identity, a SHA-256 digest or a canonical UTC timestamp. A refusal names a class, a fixed
   field path and a fixed rule sentence. It MUST NOT quote the refused input.
3. **Nothing is counted twice.** Request counters are own-process only. Inclusive CPU is never
   added to a child's own CPU. Overlapping wall intervals are never summed as elapsed time.
   A cumulative usage snapshot replaces the previous snapshot of its series and is never added
   to it.
4. **Observations confer no authority.** No performance record grants work, acceptance, budget
   or effect authority. With collection off, every operational record (audit, budget accounting,
   dispatch records) works unchanged.

## 2. Consent and profiles

Local writing is armed only when both hold in the same process:

- `ASSAY_TELEMETRY` is exactly `1`; and
- a profile is explicitly selected, either by the command's `--profile` flag or by
  `ASSAY_PERFDATA_PROFILE`. The flag wins. No profile is ever implied, and an unknown name
  disarms. It is never widened to another profile.

The profile selector is a separate variable on purpose: an existing telemetry-v1 or perf-v1
opt-in never becomes consent to this record class. With either switch missing, nothing is
created: no directory, no file, no source identity.

| Profile | May carry | Never carries |
|---|---|---|
| `tool` | Fixed-vocabulary measurements, build/platform, the random source identity and an unlinked invocation alias | Parent invocation, session, agent, attempt, shared refresh, desk role, assigned role, work references |
| `workflow` | Everything in `tool`, plus aliased correlation context (`ctx` below) and the workflow-only kinds `attempt` and `usage` | Raw locators, native harness IDs, argv, paths, titles, bodies, raw errors, credentials, transcripts |

A `tool`-profile event with any linked context field is refused (`privacy`), and so is a
draft carrying one. Records under either profile are **pseudonymous, not anonymous**: the
source identity links every record of one source.

## 3. The event

One event is one line of canonical JSON: `json.Marshal` of the Go `Event` struct, no trailing
whitespace, at most 8192 bytes without the newline. A reader MUST refuse unknown fields,
duplicate keys, reordered fields, trailing data and any other non-canonical byte form.

| Field | Rule |
|---|---|
| `schema` | `performance-event-v1`. Any other value is refused `unsupported`. |
| `id` | `<source>-e<epoch>-s<seq>`. MUST equal that derivation. Immutable. |
| `source` | `src-` + 26 base32 characters (128 random bits), minted locally. |
| `epoch`, `seq` | Integers starting at 1. A source never reuses a readable `(epoch, seq)`. |
| `profile` | `tool` or `workflow`. |
| `policy` | Revision token of the collection policy in force when the event was recorded. |
| `instrument` | `{"name":"perfdata","version":"1"}`. Any other value is refused `unsupported`. |
| `build` | `{version, os, arch}` as bounded tokens. |
| `occurredAt`, `recordedAt` | `YYYY-MM-DDTHH:MM:SS.mmmZ`, UTC. A draft without `occurredAt` takes `recordedAt`. A collector's received-at time is extra provenance and never replaces either. |
| `kind` | The closed v1 set in §4. An unknown kind is refused `unsupported`. |
| `ctx` | Optional correlation context (§3.1). Required (with `invocation`) for every kind except `health` and `correction`. |
| `payload` | The typed payload of `kind`, itself in canonical form. |

### 3.1 Correlation context

Every identity in `ctx` is a scoped random alias, `pa-` + 26 base32 characters. The local
alias mapper mints it from a qualified locator and keeps the mapping private (§6.3). A raw ID,
a qualified work locator or any free text fails the alias pattern and is refused `privacy`.

`invocation`, `parentInvocation`, `session`, `agent`, `parentAgent`, `attempt`,
`sharedRefresh`: aliases. An invocation cannot be its own parent, and neither can an agent.
`deskRole` (originating desk, attribution only): `the-desk`, `worker-desk`, `pr-review-desk`,
`verify-desk`, `intake-desk` or `unavailable`. `role` (assigned function): `coordinate`,
`implement`, `review`, `verify`, `triage`, `investigate`, `other` or `unavailable`. Under the
workflow profile both are required, and unknown producer input becomes `unavailable`, never
copied text. `work`: at most 8 `{kind: brief|issue, ref: alias, primary?}` entries with no
duplicate ref and at most one primary. An issue locator is qualified by forge instance and
repository, so identically numbered issues in different repositories never share an alias.
An `attempt` event requires `ctx.attempt`.

### 3.2 Measurements and distributions

A **measurement** is `{basis, ms?, method?}`, where `basis` is one of `measured`, `proxy`,
`estimated` or `unavailable`. `measured` carries `ms` and no method. `proxy` and `estimated`
carry `ms` and a method token. `unavailable` carries neither. `ms` is between 0 and 31 days.

A **distribution** (`log2ms-v1`) is `{v, count, sumMs, maxMs, bins[32], samples?}`. Bin 0 holds
0 ms, bin 1 holds 1 ms, bin *i* (2..30) holds [2^(i-1), 2^i) ms and bin 31 holds 2^30 ms and
above. These are the perf-v3 request-duration histogram edges, pinned by
`TestDistributionEdges`. Bins MUST sum to `count`, and `maxMs` MUST fall in the highest
non-empty bin. `samples`, when present, holds every observation exactly (at most 64 on one
event, 4096 in a daily rollup). Distributions merge by adding counts, sums and bins. A
percentile is read once from the merged distribution: exact nearest-rank when every sample is
retained, otherwise a bin estimate that the report labels as such. A reader MUST NOT average
percentiles.

## 4. Kinds

| Kind | Profiles | Payload (all fields bounded; see the schema) |
|---|---|---|
| `tool.completion` | tool, workflow | `tool` and optional `verb` (tokens; the tool must be in the policy's vocabulary), `outcome` (`ok`, `error`, `refused`, `cancelled`, `unknown`), `startedAt`, `wall`, `cpu` `{own, descendant, inclusive}`, `children` `{started, uninstrumented}`. With all three CPU values measured, inclusive equals own plus descendant. |
| `phase` | tool, workflow | `phase` from the closed set `boot`, `auth`, `audit`, `collect`, `page`, `enrich`, `classify`, `plan`, `workspace`, `source`, `packet`, `claim`, `recheck`, `child`, plus `startedAt`, `wall`, `cpuOwn`. |
| `attempt` | workflow | `state` (`started`, `finished`, `abandoned`); finished and abandoned carry an `outcome`. A started attempt with no terminal observation is unfinished. It is not counted as a success or as a confirmed death. |
| `usage` | workflow | `mode` (`delta` or `cumulative`), `model` (revision token or `unavailable`), nullable token counts (`inputTokens`, `outputTokens`, `cacheReadTokens`, `cacheWriteTokens`), optional `cost` `{microUnits, currency, pricingRevision, basis: measured|estimated, method?}`. A null count is unknown. |
| `http` | tool, workflow | Actual HTTP attempts by this process at one `forge` / endpoint `class` / `origin`; `scope` is always `own`. `attempts`, `retries`, `notModified`, `failed` (none exceeds `attempts`) and a `duration` distribution. `graphql` attempts MUST carry `points`, with every attempt in exactly one of measured, estimated, unknown, unsupported or failed. A sum is present only when its samples exist. Other classes carry no points. |
| `service.call` | tool, workflow | Client view of a query service: `operation`, `logicalCalls`, `transportAttempts`, `retries`, `outcomes` (summing to `logicalCalls`), `duration`. |
| `service.handler` | tool, workflow | Server view: `operation`, `requests`, `cache` (summing to `requests`), `coalescing`, `upstream` (`observed` or `unobserved`; only `observed` lets zero provider attempts read as a measured zero), `duration`. |
| `health` | tool, workflow | Writer health: `queueFull`, `writeFailed`, `flushTimeout`, `refused` counters and a `writeLatency` distribution. |
| `correction` | tool, workflow | `corrects` (an earlier event ID of the same source), its `digest`, `reason` (`wrong-value`, `wrong-basis`, `wrong-attribution`, `retracted`). The old record is kept but excluded from aggregates. A replacement value is a new ordinary event. |

A client service call, its handler and the provider attempts behind it are three different
kinds. A reader MUST NOT combine the two sides of one call, and MUST NOT copy one provider
observation onto every waiting work item. `refresh` and `poll` origins form a shared,
unallocated bucket.

## 5. Collection policy

`performance-policy-v1` is an operator-selected JSON document. Its revision is stamped on
every event.

| Field | Rule |
|---|---|
| `revision` | Revision token. Required. |
| `profiles` | Non-empty subset of `tool`, `workflow`. An event under another profile is refused `privacy`. |
| `tools` | 1 to 256 tokens. A `tool.completion` naming any other tool is refused `privacy`. |
| `ageCapDays` | 1 to 7 UTC days. |
| `byteCap` | Required, no default, at most 1 GiB. A policy without one is refused. |
| `segmentBytes` | Default 1 MiB. At least four maximum events, at most `byteCap`. |
| `queue` | Sink queue bound. Default 256. |
| `flushBudgetMs` | How long a sink close waits. Default 250 ms, at most 10 s. |
| `distributionDays` | 0 to 90. When above 0, expired raw tool completions are first folded into daily rollups kept this many days. |

## 6. Local store

### 6.1 Layout

The store is `<stateDir>/performance-events/`. It is distinct from `audit.jsonl`,
`dispatch-records.jsonl` and the `perf/` history. Directories are 0700, files 0600, and
metadata is replaced atomically by rename.

```text
manifest.json        performance-store-v1 layout and schema versions
source.json          source identity, epoch, last reserved sequence
aliases-e<N>.json    the private alias map of epoch N
coverage.json        retirements, failed-write ranges, unclean ends, epoch resets
lock                 writer lock (bounded wait)
run/                 one live marker per open writer
segments/            seg-<firstSeq>-e<epoch>-<YYYYMMDD>.jsonl
distributions/       day-<YYYY-MM-DD>.json rollups (performance-rollup-v1)
```

### 6.2 Identity and recovery

Appends from several processes are serialized by the lock, so no two records interleave or
share an identity. A new segment's range is reserved in `source.json` before its first append.
Appends inside an open segment advance `source.json` lazily, and the next writer catches it
up from the segment tail. A crash cannot silently reset a sequence:

- a lost `source.json` is rebuilt from the newest segment with a **new epoch**
  (`source-metadata-lost`);
- metadata found behind where the newest segment began (a restored or rewritten file) starts
  a new epoch (`source-metadata-behind`);
- a lost alias map starts a new epoch (`alias-map-lost`), so old and new aliases can never be
  joined.

One sequence number can be reused. When a crash tears the last line of a segment and the lazy
metadata had not advanced past it, the next writer opens a new segment and may give the torn
record's sequence to a new event. The torn line never decodes, so no two *readable* records
share an identity. The reader still counts the torn line as a partial tail.

### 6.3 Aliases

Each epoch has its own random secret. An alias is minted once per qualified locator, and the
map is keyed by HMAC(secret, locator), so raw locators are never written. The map file is
excluded from events, previews, batches and reports. It grows with every distinct locator in
an epoch, and it is not counted against `byteCap`.

### 6.4 Best-effort recording

Producers hand drafts to a bounded sink and never wait on storage. A full queue drops the
draft and counts `queueFull`. A failed write (a full disk) counts `writeFailed` and is never
returned to the caller. A close waits at most `flushBudgetMs`. A clean close writes one
`health` event and removes its live marker. A close that runs out of budget leaves the marker
behind. The next reader then reports an unclean termination and partial coverage.

**Crash-loss envelope.** Appends do not fsync on the work path. A crash or kill can lose queued
drafts and the tail of the last write. What the reader can see is a partial tail, an
unexplained sequence gap, or an orphaned live marker. A full disk can also prevent the loss
record itself from being written, and that loss then stays unexplained. The reader never
reports any of these as complete coverage.

### 6.5 Retention

Raw segments older than `ageCapDays` UTC days are retired. When the store exceeds `byteCap`,
the oldest segments are retired as well. The byte cap is checked when a new segment starts,
so the store can exceed the cap by about one segment. Nothing in this contract sends data, so
every retirement is recorded as **unsent** coverage loss with its sequence range, day window,
event count and bytes. With `distributionDays` above 0, expired tool completions are folded
into daily rollups before their segment is removed. A rollup older than its window is retired
too, and its day reads `retired`, never zero. The retirement record is saved before the rollup
and before the removal, and a reader ignores segments already recorded as retired. This order
prevents double counting after a crash.

A rollup reflects the corrections applied before it was written. A correction recorded after
its target's day was rolled up does not change that rollup.

## 7. Reader coverage

A reader returns, per source, a `SourceCoverage` manifest: state, epochs, event count,
first/last recorded times, last sequence, corrupt lines, corrupt metadata, partial tails,
explained and unexplained gaps, retirements, drops, unclean terminations, live writers and
epoch resets. The states are:

| State | Meaning |
|---|---|
| `missing` | No store. Unknown, never zero. |
| `empty-complete` | A clean store with no events in the window. |
| `complete` | Events present, no unexplained loss. |
| `partial` | Events present with gaps, drops, partial tails or unclean ends. |
| `corrupt` | Undecodable records or metadata. |

Identical re-reads of one identity count as duplicates. One identity carrying two different
records counts as a conflict, and only the first record is kept.

## 8. Report snapshot

`performance-snapshot-v1` freezes one report input: cutoff, UTC timezone, day window, policy
revision, instrument, the per-source coverage manifest, and which of the three operating
reports it can feed. A local spool feeds `tool-performance`. `completed-briefs` and
`stream-progress` are marked `unavailable`, with a reason, because their lifecycle and flow
sources are not collected here. Re-running a report over the same snapshot gives the same
output.

The local report (`performance-report-v1`) applies the rules of §1:

- elapsed time is the union of invocation intervals, and summed wall time is shown beside it;
- only own CPU is summed;
- each cumulative usage series contributes its latest snapshot, while deltas add;
- unknown values are counted as unknown;
- each day reads `measured`, `rolled-up`, `retired` or `unknown`;
- pooled tool distributions are merged across raw days and rollups before any mean or
  percentile is read.

## 9. Batch and receipt

A **batch** (`performance-batch-v1`) is `{schema, source, consentRevision, profile, events}`.
`events` holds 1 to 500 stored events, byte-for-byte as spooled, and the whole batch is at most
1 MiB. A **receipt** (`performance-receipt-v1`) is `{schema, batchDigest, committed, status,
reason?, results[]}`, with one result `{index, id?, status, class?, digest?}` per event.

A collector applies these rules in order (`perfdata.Admit` is the reference):

1. An oversized or malformed batch, a wrong schema, or an event count outside 1..500 is rejected
   whole (`bounds` or `schema`).
2. The batch `source` MUST equal the source bound to the caller's credential. Otherwise the
   batch is rejected `source-not-authenticated`.
3. `consentRevision` MUST equal the collector's current consent revision. Otherwise the batch is
   rejected `consent-revision`.
4. The batch profile MUST be allowed for that source. Otherwise the batch is rejected
   `profile-not-allowed`.
5. Each event is decoded with the full event rules. A refused event gets `rejected` and its
   class. An event whose source or profile differs from the batch gets `rejected` (`privacy`).
6. An event recorded before `now - replayHorizon` is `outside-horizon`. It is never resurrected
   after expiry.
7. An identity already committed, or already admitted earlier in the same batch, is
   `duplicate` when its digest matches and `conflicting` when it differs.
8. Every other event is `acknowledged`. All of them are committed together.

An acknowledgment means **committed to the configured store**. If the commit fails, the receipt
is `retry` (`commit-failed`), `committed` is false and every would-be acknowledgment is `retry`.
If nothing is acknowledged or duplicate, the receipt is `rejected` (`none-admitted`).
Otherwise it is `committed`, or `partial` when some events were refused, conflicting or outside
the horizon. Delivery is at least once inside the replay horizon, not exactly once. Retries keep
identity and payload unchanged, and lost acknowledgments and concurrent redelivery are normal
cases.

## 10. Conformance

`perfdata/testdata/api/` is the conformance suite for any database, server or sender built
against this contract. Each case is `<case>.batch.json` (the exact bytes sent) and
`<case>.case.json` (the collector context and the expected receipt). The cases are
`committed`, `identical-duplicate`, `replay-all-duplicate`, `conflicting-identity`,
`conflict-within-batch`, `partial-refused-event`, `outside-replay-horizon`, `none-admitted`,
`commit-failed`, `source-not-authenticated`, `consent-revision-mismatch`, `profile-not-allowed`
and `unknown-field`. An implementation conforms when it returns the same receipt for every case.
The fixtures are digest-checked byte for byte, so they are stored with `-text` line-ending
handling. `TestAPIContractFixtures` checks them against hand-written expectations.

## 11. Versioning

A new meaning gets a new identifier. It never reinterprets an existing one. A reader refuses any
`schema`, `instrument` version or distribution version it does not know (`unsupported`) and
never guesses.
