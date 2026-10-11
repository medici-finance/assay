# ci-check-v1 — CI check history

Status: implemented (`cicheckhistory.go`, beside this file). Local operational state for later
analysis. A record
never grants, refuses or changes anything: no desk decision reads it, and the read that produced
it returns exactly what it would have returned without it.

## Why

The desk tools read every pull request's CI results many times a day to decide what to flip,
review or report, then reduce them to an in-memory verdict and throw them away. Without a record,
"which checks flake, how often, and how long do they take?" or "did a PR that went red twice also
fail verify?" cannot be answered. This record keeps one small line per finished check run or
commit status, taken from reads the tools already make — it costs no extra forge request.

## Where it lives

- **File:** `<state dir>/ci-checks.jsonl`, where the state dir is the desk tools' own
  (`~/.config/assay/`, the directory the audit log and kill switch live in). Directory mode
  `0700`, file mode `0600`. Writers serialize on their own lock file, `ci-checks.lock`, beside it
  — never the audit lock.
- **Local operational state, never committed.** It is not a repository artifact, is not pushed,
  and is not read by any gate.
- **Who writes it:** the single Forge decorator every desk tool reads through
  (`OutboundChecked`, the only construction site of a forge backend). Its `ChecksAtHead`,
  `ListOpenChanges` and `ReviewQueueSnapshot` reads hand their already-fetched results to the
  recorder after the read succeeds. The recorder makes no forge call.
- **Best-effort.** A write failure, a lock held longer than 2 s, or a panic inside the recorder
  drops that batch with one stderr line per process. After one lock timeout the process records
  nothing more, so a stuck lock holder costs one 2 s wait per process, not one per read. The
  warning names files by their base name only, never the state directory's path. The read's
  result and error are returned unchanged either way.
- **The live file is a regular file, mode `0600`.** A symlink or any other kind of file at its
  path is refused (the batch is dropped, nothing is written through it), and a wider mode found
  on it is tightened to `0600` before the append. If the file does not end in a newline (a
  partial write cut the last batch short), the next batch starts with one, so the torn fragment
  stays a single skipped line.

## Fields (`schema: "ci-check-v1"`)

One JSON object per line. These fields, and no others:

| Field | Type | Meaning |
|---|---|---|
| `schema` | string | always `"ci-check-v1"` |
| `observed_at` | string | RFC3339 UTC: when the tool read the result |
| `tool` | string | the running desk tool's canonical key (`CanonicalToolKeyOr` of the binary name) |
| `repo` | string | `owner/name` |
| `head_sha` | string | the commit the result is for |
| `pr` | int | the change number, when the read carried one (the rollup reads); omitted otherwise (`ChecksAtHead` reads a commit, not a change) |
| `kind` | string | `check-run` or `status` (a commit status — any CI that posts one rather than a check run, e.g. a required external scanner or a GitLab pipeline) |
| `name` | string | the check run's name, or the status context |
| `attempt` | string | which execution this is — see "Attempt key" |
| `conclusion` | string | the forge's terminal value, lowercased (`success`, `failure`, `cancelled`, `timed_out`, `neutral`, `skipped`, `action_required`, … for a check run; `success`, `failure`, `error` for a status) |
| `started_at` | string | RFC3339 as the forge reported it; check runs only; omitted when absent |
| `completed_at` | string | RFC3339 as the forge reported it; check runs only; omitted when absent |
| `duration_s` | int | whole seconds from `started_at` to `completed_at`; check runs only; omitted when either stamp is missing or unordered |

### Attempt key

- **check-run:** the forge's run id (`CheckRun.ID` from the REST check-runs read, `RollupNode.ID`
  from the GraphQL rollup's `databaseId` — both rendered by the same function, so the two reads of
  one run agree). A re-run of the same named check is a new id, and so a new attempt. When the
  forge served no id: `t:<started_at>`.
- **status:** `t:<created_at>`. The stamp identifies the forge's status OBJECT, not one execution.
  On GitHub a new state is posted as a new status with its own stamp, so the two coincide. The
  GitLab backend publishes the head pipeline as one status named `pipeline` whose stamp is the
  pipeline's own, and a job retried inside that pipeline does not change it: the pipeline's
  `failure` and its later `success` share one attempt. They are still two records, because the
  conclusion is part of the dedupe key (below). The individual GitLab jobs, read through
  `ChecksAtHead`, are check runs with their own ids, so each retried job is a new attempt.
- An entry with no usable key (no id and no stamp) is **not recorded**.

### Dedupe key

`(repo, head_sha, kind, name, attempt, conclusion)`. Each distinct terminal conclusion of one
attempt is kept once, first occurrence wins: an attempt whose result changed (a retried GitLab
pipeline, a run whose conclusion the forge rewrote under the same id) carries one record per
conclusion, in the order first seen. The limit: a result that returns to a conclusion already
recorded for that attempt (`failure`, `success`, `failure`) is not recorded a second time.
Deduplication happens twice:

1. **On write.** The writer skips a key this process already wrote, and a key already present in
   the live file (read under the lock — another process may have written it).
2. **On read.** `LoadCIChecks` reads every rotated segment and the live file and keeps the first
   occurrence of each key, so a duplicate written across a day rotation or by two racing
   processes never reaches an analysis.

### Only terminal results

A check run is recorded only when its status is `completed`; a status only when its state is
`success`, `failure` or `error`. Queued, in-progress, pending and expected entries are skipped:
recording one would claim the key before the real result arrived and dedupe the result away. The
GitLab backend's could-not-check rollup sentinel is skipped too.

### Over-long lines

The writer never appends a line longer than 64 KiB; a record that would exceed it (only an
absurd check name can) is not written. The reader skips any line longer than 1 MiB, like any
other unparseable line, so one corrupted line never stops recording or fails `LoadCIChecks`.

## Retention

The live file rotates daily, by the audit log's rule: when its last append fell on an earlier UTC
day, it is renamed to `ci-checks.jsonl.<YYYY-MM-DD>` (with a `.N` suffix when that name is taken)
before the next write. **The tools never delete a segment.** How long to keep them is the
adopter's choice; removing old segments by hand is safe, because no desk decision reads them.
The segment name cannot match the audit log's segment pattern, so audit rotation and audit reads
never sweep a CI-check segment in, and the reverse holds too.

## Never collected

The record type has no field for any of the following, so none of it can be written:

- check output, title, summary, annotations or log text;
- details URLs, target URLs or any other link;
- the actor or app that posted the check or status;
- any session transcript.

There is no per-person field at all. Names and conclusions only.

**A name is unauthenticated.** A check name or status context is whatever the poster of that
check run or status supplied, on any open pull-request head the tools read, contributors'
heads included, and the record carries no author or trust field. A consumer must not treat a
name as proof of which check produced a result, and must sanitize a name before printing it:
control characters are escaped on disk by the JSON encoding and come back raw when decoded.

## Joining

The record joins other desk records (review rounds, verify outcomes) by `repo` + `head_sha`
(+ `pr` when present). A dispatch reference is not known at a CI read, so the record does not
carry one.
