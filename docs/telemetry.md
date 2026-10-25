# statusgen telemetry — opt-in, anonymized, off by default

`statusgen` can emit an anonymized, counts-only telemetry payload describing how
a fleet of briefs drifts over time — which categories of lint failure recur, how
briefs move through the lifecycle, how many streams and briefs a repo carries.
It exists to turn aggregate patterns into better defaults and Console features.

**It is off by default and collects nothing unless you turn it on twice.** A
default `statusgen` run makes no network call at all. This document is the
contract: what a payload can contain, how you opt in, how long data is kept, and
the promise underneath all of it.

## The promise

- **Off by default.** No flag, no environment variable, no ping. Installing or
  running `statusgen` sends nothing.
- **Counts only, never content.** A payload is integer tallies keyed by a fixed
  vocabulary. It never contains a repo name, a stream name, a brief number or
  title, a file path, register text, a commit SHA, or any identity.
- **You can always see exactly what would be sent.** Every run with telemetry
  armed prints the full payload before anything leaves the machine, and
  `--telemetry-dry-run` prints it and sends nothing.
- **Two independent switches.** Turning it on requires both a command-line flag
  and an environment variable, so no CI vendor default, wrapper, or inherited
  environment can flip it on silently.

## How to opt in

Telemetry is armed only when **both** of these are true in the same run:

1. `--telemetry` is passed on the command line, and
2. `ASSAY_TELEMETRY=1` is set in the environment.

Either one alone leaves telemetry off. When `--telemetry` is passed without
`ASSAY_TELEMETRY=1`, `statusgen` prints a one-line note on stderr saying it
stayed off and that both switches are required.

```sh
# Off (default) — nothing collected, nothing sent:
statusgen --root . --lint

# Preview the payload locally, send nothing:
ASSAY_TELEMETRY=1 statusgen --root . --telemetry --telemetry-dry-run

# Armed on a normal run: prints the payload; sends it only if a receiver is
# configured in the build (none is today — see "No receiver yet" below):
ASSAY_TELEMETRY=1 statusgen --root . --lint --telemetry
```

To turn it off, remove either switch.

## What a payload contains

The payload is versioned by the `schema` field (currently `telemetry-v1`). Every
field is an integer count or a map from a fixed-vocabulary key to an integer
count. There is no field capable of carrying free text.

| Field | Type | Meaning |
|-------|------|---------|
| `schema` | string | Payload schema version, e.g. `telemetry-v1`. Bumped when fields change. |
| `statusgen_version` | string | The `statusgen` build version (the tool's own version, not your data). |
| `stream_count` | int | Number of streams discovered. |
| `brief_count` | int | Number of briefs discovered across all streams. |
| `brief_status_counts` | map(status → int) | How many briefs sit in each lifecycle status. Keys are the fixed status vocabulary only (`todo`, `in-progress`, `implemented`, `verified`, `done`, `blocked`, plus `none` for unset and `other` for anything outside the vocabulary). |
| `lint_failure_categories` | map(category → int) | How many lint failures fell into each **category**. Keys are a fixed set of category labels defined in `statusgen` (e.g. `frontmatter`, `brief-status`, `finding-reference`, `word-budget`, `link-check`, `uncategorized`). A failure whose shape isn't recognized is counted under `uncategorized` — never quoted. |
| `lifecycle_transitions` | map("from->to" → int) | How many recorded lifecycle transitions moved between each pair of statuses, e.g. `implemented->verified`. Both ends are drawn from the fixed status vocabulary above. |

### Why this cannot leak identifiers

Lint messages and history records inside `statusgen` do contain stream names,
brief numbers, and paths. Those never reach a payload: each lint message is
mapped to a category **label** and each transition to a **status pair**, both
drawn from a fixed vocabulary defined in the tool. The mapping only ever returns
one of its own constants — it never returns any part of the message it was
handed — so a message that embeds a name or a path is counted, never quoted. An
unrecognized lint shape becomes an `uncategorized` count; an unrecognized status
becomes `other`. This is enforced by tests that feed the builder records full of
sentinel identifiers and assert none survive into the serialized payload.

## Retention

The receiver and its retention policy are **not yet stood up** (see below). When
one is, this section will state the retention window and deletion policy before
any release carries a live endpoint, and no payload will be transmitted to it
until that policy is published here. Because payloads are counts-only and carry
no identity, there is no per-user record to request or delete; there is only the
aggregate corpus.

## No receiver yet (v-next)

This release ships the telemetry **client** only, behind an empty endpoint. No
receiver URL is compiled into the binary, so even an armed run sends nothing — it
prints the payload and reports that no endpoint is configured. Standing up the
receiver, publishing its retention policy, and enabling transmission is
Console-side work tracked separately. Until then, `--telemetry-dry-run` is the
only meaningful mode, and it is purely local.

## Desk-tools: the local perf history

The desk tools (`tools/desk/`) keep a separate, **local-only** performance log under the
same promise and the same environment variable. With `ASSAY_TELEMETRY=1` exported, the
67 operational desk commands each append one `perf-v2` record per completed invocation to
`~/.config/assay/perf/<YYYY-MM-DD>.jsonl`, and so does `statusgen`, with one phase per pipeline
stage; `deskboot` boot steps and `cellctl desk` launch steps still append the legacy `perf-v1`
record. `deskperf report` and
`deskperf boot` read both back (see [Two schemas](#two-schemas-perf-v1-and-perf-v2)).
Day files are kept for 7 UTC days and pruned on write. With the variable unset — or set to
anything other than exactly `1` — nothing is created: no directory, no file, no install id.

It sends nothing. No file that implements it opens a network connection, and there is no
endpoint or sender for it anywhere. That is why it has **one** switch rather than two: the
two-switch rule above exists so that no inherited environment can cause a *send*, and this
log is a file on your own disk. The single switch still means consent — a user who never
opted in gets nothing new. **The two-switch rule binds any future remote sink for these
records in full**: a sender for the perf history needs its own explicit flag in addition
to `ASSAY_TELEMETRY=1`, exactly as `statusgen --telemetry` does.

A `perf-v1` record — the legacy schema, frozen: its fields and their meanings never change, and
history already on disk is never reinterpreted — carries exactly these fields and no field
capable of holding free text:

| Field | Type | Meaning |
|-------|------|---------|
| `schema` | string | Record schema version, `perf-v1`. Adding any field bumps it (perf-v2 is that bump). |
| `ts` | string | UTC timestamp, second resolution. |
| `tool` | string | The desk tool's canonical key from a compiled-in roster; anything else is `other`. |
| `verb` | string | The tool's own registered subcommand, or `other`; a boot step is `step:<name>` from `deskboot`'s compiled step list, and a `cellctl desk` launch phase is `step:launch-<phase>` from `cellctl`'s compiled launch-step list (else `step:other`). Never argv text. |
| `role` | string | `$DESK_LOOP` mapped onto the compiled loop-name roster; unset or unknown is `none`. |
| `wallMs` | int | Wall-clock duration of the invocation (from the wrapper's own start) or of the boot/launch step. |
| `cpuMs` | int, optional | Process user+system CPU; **omitted** where it cannot be measured (Windows, boot steps), so "unmeasured" never reads as zero. |
| `exit` | int | The verb's exit code. |
| `result` | string | The exit code mapped onto a fixed word (`ok`, `disabled`, `refused`, `unverifiable`); never the input. |
| `childProcs` | int | Child process attempts through `deskkit.Run`, `RunProcess` or `OutputProcess`, including failed starts. |
| `childWallMs` | int | The summed wall time of those counted attempts; overlapping children are not elapsed workflow time. |
| `forgeCalls` | int | HTTP attempts reaching the forge client's leaf counting transport, including failures, retries and 304 responses; local refusals are excluded. Requests are not GraphQL quota points. |
| `sourceSHA` | string | The binary's build stamp when it is hex, else `unpinned`. |
| `install` | string | 32 random hex characters minted once per state directory. Identifies nothing but the install. |

`childProcs` and `forgeCalls` count only what passes through those two shared seams, so
they are lower bounds, not totals. A verb that starts its own children or makes its own
HTTP calls outside them is not counted. The known cases: `deskboot` starts its boot-step
children through its own runner; `desktoken`'s token mint and `deskpost`'s posts use their
own direct HTTP client; and other command-specific direct transports remain outside this inventory. A `0` from those tools means "not counted", not "none made".

### Review workflow attribution and coverage

`deskboard` records the parsed operation, preserving canonical aliases (`todo`, `next`
and `next-up` map to `dispatch`). Output flags are not operations. `issueboard` records
`board`, `issues` or `intake` from its selected handler, including the default board.
The deskboard Cobra adapter keeps help/version and parse refusals outside its
recording callback (no sample); the other adapters retain allowlisted spellings or
`other` for early rejection. A failed operation still carries its selected operation
and original exit. No parser is duplicated to obtain attribution.

`deskdispatch` records `dispatch:worker`, `dispatch:worker-objective`, `dispatch:review`
or `dispatch:verifier`, and the corresponding `stamp-only:<kit>` constants. Kit comes
from the existing parser (default `worker`), with its existing case/space normalization;
it never comes from session role or an item key. Unknown kit/arguments remain `other`.
These are additive values of the existing `verb` string, not new perf-v1 fields. Existing
`deskperf` readers decode and display them unchanged; legacy rows are not reclassified.
The current dispatch CLI port must keep these handler calls when it changes adapters.

| Three-tool inventory | Coverage and boundary |
|---|---|
| `deskboard` statusgen version, gate-score and next-up launches | `OutputProcess` observes all four source sites without trimming bytes, replacing stderr, changing environment or wrapping the original error. |
| `deskdispatch` claim, worktree, roster and Git children | Existing `Run` delegates to `RunProcess` once; local argv/trace/redaction and error causes stay with the existing runner. |
| Shared role minter, verifier Git, run-key/dispatch-ref Git, lifecycle hooks and optional content/risk/advisor callouts | Raw process observers retain each owner's cancellation, process-group cleanup, streams, environment and refusals. All callers of these shared helpers gain counters. |
| `issueboard` direct children | None in the command package; shared helpers above still apply. |
| Resolved GitHub REST/GraphQL, GitHub diff, GitLab SDK | Existing counter; the GitHub conditional wrapper counts at its delegated leaf. A request refused locally by the board quota floor costs zero HTTP attempts. A validated 304 still counts one attempt. |
| Shared account-liveness and visibility reads | Copy the existing client with a counting transport, preserving its redirect/timeout policy and caller-owned client. |
| External claim scripts, statusgen, token minter and hooks' own descendants/HTTP | Not recursively instrumented by the parent. Their child counts/CPU/requests remain unknown unless independently measured. |
| Other command-specific runners, preflight/migration/lock-holder inspection, direct mint/post HTTP | Outside these three tools' direct inventory; existing lower-bound semantics remain. This change is not fleet-wide complete coverage. |

`cpuMs` measures only the current process; descendant and inclusive CPU are unavailable
in perf-v1. Do not add child `cpuMs` to a parent's inclusive measurement. Do not add
`childWallMs` to `wallMs`, or sum overlapping invocation wall times as elapsed time.
The record carries neither endpoint/retry/304 breakdown nor GraphQL quota points;
those distinctions remain unknown in this dataset, even though every delegated attempt
is counted. The qualification fixtures can observe these distinctions independently.
Performance-collection/01–02 own richer coverage, correlation and quota events. No new
collection consent, sender, spool, sink or listener is enabled here; the one schema expansion
is the versioned `perf-v2` record below, and it adds no consent or sender either.

### Two schemas: perf-v1 and perf-v2

Since desktools-v2/61, `deskkit.PerfRun` — the wrapper around every operational desk
command — writes `perf-v2`, and `statusgen` writes it too: its phases are its pipeline
stages (`parse`, `lint` and the heaviest lint checks `briefs`, `ordering` and `evidence`,
`links`, `claims`, `view`, `render`, `write`, `corroborate`), and its `http` counter counts
each direct HTTP attempt at its forge seams (a redirect the HTTP client follows inside one
attempt is not counted again); its `gh`/`git` child processes have no shared
seam, so `subprocess` is listed unavailable. Its phases are listed in roster order, not
first-entry order (the view build is entered before the claim read but listed after it), and
they do not sum to `wallMs`: process start and the exit after the last stage fall inside
`wallMs` but outside every phase. Boot and launch step records keep writing
`perf-v1`. Both live in the same day files; a reader groups by `schema` and never merges the
two, because their `wallMs` is measured over different boundaries. A `perf-v2` record carries
exactly these fields:

| Field | Type | Meaning |
|-------|------|---------|
| `schema` | string | `perf-v2`. |
| `ts`, `tool`, `verb`, `role`, `exit`, `result`, `sourceSHA`, `install` | | As in perf-v1, and every label (`tool`, `verb`, `role`) also passes perfrec's label gate — lower-case, at most 64 characters, roster-shaped — or is recorded as `other`. |
| `boundary` | string | Where the measurement starts: `process-init` (the default: a mark taken while the shared `perfrec` package initialises, before any importer's initialisers and before `main`) or `caller` (a mark a caller took itself). |
| `wallMs` | int | Wall time from the boundary mark to the end of the measured work. Executable load, Go runtime start-up and the standard-library initialisers `perfrec` imports precede a `process-init` mark and are NOT included; writing the record (encoding, install-id read, prune, append) follows the end and is NOT included. A verb's configuration echo and activation checks in `main` ARE included. Whole-executable elapsed time is a separate benchmark, never this field. |
| `cpu` | string | CPU coverage state: `measured` (`cpuMs` present), `unsupported` (the platform has no reader, e.g. Windows), `failed` (a read failed, or the clock went backwards), `missing` (the switch was off when the process started, so no start sample exists). Only `measured` carries a number. |
| `cpuMs` | int, only when `cpu` is `measured` | This process's user+system CPU over the SAME interval as `wallMs`: the start sample is read with the boundary's wall reading and the end sample right after the end wall reading. Own process only; descendants are never included. |
| `counters` | object | Each counter that was measured, with its count (a measured zero is `0`). |
| `unavailable` | array | Each counter that was NOT measured, in roster order. `counters` and `unavailable` together list every roster counter exactly once, so "not observed" never reads as zero. |
| `phases` | array of `{name, n, ms}` | Timed phases from the tool's own fixed roster (at most 16 names; any other name is `other`): instance count and summed ms per name. Empty when the tool times none. |
| `phaseOverlap` | bool | A phase started while another was open: the phase ms then overlap and must not be summed against `wallMs`. |

The counter roster is closed: `subprocess` and `subprocessMs` (child process attempts through
the shared runners, and their summed inclusive wall), `http` (HTTP attempts reaching the
counting transport), `cacheLookup`, `cacheHit`, `cacheMiss`, `cacheRefresh` (a shared cache's
outcomes), and `waitShared`, `waitFailed`, `waitAbandon` (a coalesced waiter that received a
shared result, received the leader's failure, or gave up). `PerfRun` measures `subprocess`,
`subprocessMs` and `http` through the same seams and with the same lower-bound meaning as
perf-v1's `childProcs`, `childWallMs` and `forgeCalls`; until desktools-v2/62 places provider
hooks, every cache and waiter counter is `unavailable`. Measurement adapters only observe an
effect that already happened; none can start a credential, provider, network or process call.

Rules for any reader, enforced by `deskperf`:

- Never add `subprocessMs` or phase `ms` to `wallMs`: both are inclusive, already inside it.
- Never add a step record (`verb` `step:<name>`) to an invocation total: the step is inside a
  `deskboot` or `cellctl` invocation that has its own record. Steps appear only in
  `deskperf boot`.
- A sum of `wallMs` over many records is summed invocation time, not elapsed time: concurrent
  invocations overlap. `deskperf report` labels its perf-v2 total that way.
- A CPU or counter distribution is over the records that MEASURED it; `unavailable`,
  `unsupported`, `failed` and `missing` are coverage to report, never zeros to average in.
- `perf-v1` totals and `perf-v2` totals are not comparable and are reported as separate
  groups; the detailed legacy group retains its original fields and meaning.

`deskperf report` leads with a **command performance** overview (JSON `commands`):
one row per tool, command and timing cohort, largest accumulated invocation costs first.
This exposes expensive `statusgen verifyrun` calls beside `statusgen --lint`, and
`issueboard issues` beside its cheaper projections, without letting frequent cheap commands
dilute the expensive command's median. Rows show count, summed wall, nearest-rank p50/p90/max,
exact outcome counts, own measured CPU and direct HTTP attempts with measured/missing counts.
The overview uses readable durations; JSON retains exact milliseconds. Text durations of
a minute or more omit subsecond detail.
A measured zero differs from unavailable. Legacy CPU is unavailable in this view because
its process-wide total is not a delta over its wrapper interval.

Each timing cohort retains the source record family and declared boundary. Its share uses
only that cohort's total; separate rows do not pool caller intervals, process-initialization
intervals or legacy wrapper intervals. Summed invocation work includes overlapping parent,
child and concurrent runs, so it is neither unique elapsed time nor human time saved. Phases,
subprocess/request durations and non-process service observations are never added to wall.
Workflow filters exclude unattributed old records from this overview; default output carries
no workflow identifiers. V3 identity selection precedes tool filtering, so a conflicting copy
under another tool cannot disappear from the selection. `--top 0` still suppresses the
individual slowest list; the overview is independent of that list. `--by-day` retains its
existing tool-by-day view. The committed command-summary fail-first map is
`tools/desk/cmd/deskperf/command-mutations.json`; from `tools/desk`, run
`go run ./cmd/muhar -j 1 -spec cmd/deskperf/command-mutations.json`. It uses a distinct
count-conservation control and checks verb separation, whole-window identity selection,
exclusion of nested service observations, unfiltered conflict-only detail retention and
filtered unattributed counts. This is local qualification, not a claim
that the map is wired into the truth-suite CI matrix.

The detailed legacy perf-v1 group follows (unchanged lines and JSON `summary`), then a perf-v2 group (JSON `perfV2`) with its boundary and CPU-state counts,
phase-overlap count, per-tool wall and measured-CPU distributions, per-counter
measured/unavailable counts with totals over measured records only, and per-phase
distributions. The `perfrec` module's README documents the recorder API for other
standalone tools.

A forge-calls section follows (JSON `forge`), per tool and per release build (`sourceSHA`):
runs, unavailable runs, total forge calls, calls per run (p50 / p90 / max, nearest rank), each
row's share of all forge calls, and the rate-limit cost — the busiest UTC clock hour's calls
and that hour and the p90 run as a percentage of the 5000-requests-per-hour App budget floor. Unlike
wall time, the forge count is pooled across the two schemas: perf-v1 `forgeCalls` and the
perf-v2 `http` counter are the same seam with the same lower-bound meaning, and a call spends
the budget whichever schema recorded it. Its rules:

- A perf-v2 record whose `http` counter is `unavailable` is an unavailable run: it adds no
  calls and sits in no distribution, and a row with no measured run prints `-`, never `0`.
- Older `statusgen` builds wrote perf-v1 with `forgeCalls` always `0`, meaning "not
  counted", so a `statusgen` perf-v1 record is an unavailable run too; perf-v1 has no
  unavailable state of its own. Current `statusgen` writes perf-v2, whose `http` counter is
  read like any other perf-v2 record.
- Steps are never read (their calls belong to the invocation), and perf-v3 records stay in
  their own section.
- The figures are HTTP attempts, not GraphQL quota points. 5000 requests per hour is the floor
  of an App installation's REST limit (a larger installation gets more), while the counted
  attempts also include GraphQL calls (their own points budget), conditional 304s and other
  forges, and the busiest hour pools every tool whichever installation it ran as. A
  percentage therefore gauges pressure; it is neither a bound on nor a measure of any one
  installation's draw.
- A row is keyed on the record's tool label and build stamp only when they have the roster
  and hex shapes (`perfrec.Label`, `perfrec.SourceSHA`); anything else groups under `other`
  or `unpinned`, so a malformed or planted perf-v1/perf-v2 record cannot put a path, URL or
  control byte into this section. (The older tables on the same report still print the raw
  `tool`.) A negative count reads as unavailable, and sums hold at the int64 maximum instead
  of wrapping.

Never recorded: argv, flag values, repo, PR or issue numbers, branch, path, title, body,
error text, session tag, environment values, hostname, user name, or tokens. A test plants a
repo slug, a PR number, a branch, an absolute path, a session tag and a token-shaped
environment value, and asserts none of them appears in the written bytes.

### statusgen writes the same local record

`statusgen` appends to the same history. With `ASSAY_TELEMETRY=1` exported, each `statusgen`
process writes one `perf-v2` record on exit into the same `~/.config/assay/perf/` day files,
with `tool` set to `statusgen`, `verb` the mode that ran (a positional sub-command such as
`verifyrun`, else the first mode flag such as `--lint`, else `regen` for a plain run; a token
outside that compiled-in roster is `other`), `exit` the process exit code and `result` its
fixed word. `role` is `none` and `sourceSHA` is the build stamp when it is hex (a release tag
or `dev` is `unpinned`). Its phases are its pipeline stages, its `http` counter is measured
(every direct HTTP attempt at its forge seams, so `0` means none was made), and `subprocess`
is listed unavailable; see [Two schemas](#two-schemas-perf-v1-and-perf-v2) for the phase
roster and how to read it. `deskperf report --tool statusgen` reads it like any other tool.
With the variable unset, or anything other than exactly `1`, `statusgen` creates nothing and
behaves as it did before.

This is the same local file, not the network payload above: the payload still needs
`--telemetry` as well, and nothing in this section sends anything. The record is written by
one shared module (`perfrec/`) that the desk tools and `statusgen` both call, so the schema,
layout, retention and install id exist once.

The perf history is not the desk audit ledger and never reads or writes it. It also holds no
agent usage: per-invocation tokens and cost belong to the agent runner's own usage record.

### statusgen phases in perf-v3

With `ASSAY_TELEMETRY=1` **and** a selected perf-v3 profile (`ASSAY_PERF_PROFILE` or
`--perf-profile`, `tool` or `workflow`), `statusgen` writes one `perf-v3` process-summary record
through the shared recorder instead of the `perf-v2` record. With the switch on and no profile
selected, the `perf-v2` record above is unchanged; with the switch off nothing is created,
whatever profile context is set.

- **Profile and context.** `statusgen` binds the same eight `--perf-*` flags the desk tools take
  on its own flag set (it cannot import the desk adapter). A flag overrides its environment
  variable and an empty flag clears it; the values reach the shared resolver exactly as the flag
  parser accepted them, never re-read from argv. A malformed value becomes `invalid` coverage —
  it never changes output or exit. The tool profile records no session, agent, role or work
  field, even when the environment carries them.
- **Phase roster.** `discover`, `parse`, `evaluate`, `render`, `write` and `deskread`. Phases are
  exclusive: entering a nested phase pauses the enclosing one, so phase milliseconds never
  double count (`phaseOverlap` stays false) and `n` counts segments, not calls. `evaluate` is the
  residue between flag parse and exit not claimed by another phase.
- **Boundary and rosters.** A v3 record starts at the same process-init mark as the `perf-v2`
  record and ends at the same exit. Its phase roster is not the v2 stage roster, and a reader
  never merges the two schemas, so compare like with like.
- **deskread child.** The only online route is the `deskread` child that `--forge` starts. Its
  start-to-exit wait is the `deskread` phase; the one-time `--help` check below runs before
  that phase opens, so it is not part of the child's wait. The context reaches the child as `--perf-*` flags
  on its own command line, after the read kind. Under the workflow profile they carry the
  session, agent, role and work with this invocation as the parent. Under the tool profile they
  carry the profile alone. With no profile selected they carry `--perf-profile=off`, so a parent
  that cleared its profile never hands on one it did not use. The flags are passed only when the
  resolved `deskread` lists all of them in its `--help`. That check runs once per run, with an
  empty environment and a time bound. An older `deskread` that would refuse an unknown flag gets
  none, so output and exit never change. Because the context travels on the child's command
  line, another local user who can list processes can read it while the child runs. It holds
  only validated values: random opaque ids, the role, work references and the profile, never a
  credential. The child's provider requests, credentials and cost
  are its own observations: the parent records no request observation (`requestCoverage` is
  `missing`), and child time is never added to the parent's.
- **Reading.** `deskperf report --tool statusgen` reads these records through the same shared
  decoder and v3 reducer as every desk tool; `--perf-profile workflow --group-by session` (or
  `work`, `agent`, `role`) groups them with the desk tools' records and the `deskread` child.
- **Boundary kept.** `statusgen`'s dependency closure still has no desk, deskkit, adapter or
  custody package, the recorder reads no credential, and a recording run starts exactly the
  processes a switch-off run starts. The one addition is the offline `deskread --help` check,
  made only when a `deskread` child is started.

A future context resolver (statusgen/20) may consume explicit, source-bound, immutable input
snapshots that a run already holds; it may not weaken independent membership enumeration — a
new file outside an earlier read set still invalidates completeness, and HEAD equality alone
never makes a dirty working-tree parse reusable. The snapshot-reuse inventory and the
repeatable workload bench for these phases are follow-up work under statusgen/26.

## Dependency posture

The client uses the Go standard library only (`net/http`), consistent with
`statusgen`'s stdlib + `yaml.v3` rule. No third-party telemetry SDK is
introduced.

### Command coverage

Every operational command under tools/desk/cmd has exactly one optional performance wrapper.
The inventory test maintains two explicit exceptions: deskperf must not measure its own report
reads, and muhar is a developer mutation harness. The review workflow commands select their closed vocabulary from the actual parsed
operation; other commands retain their existing registered vocabularies and fallbacks. User argv, file
paths, subjects, credentials and free-form error messages never enter these records.
On/off does not change exit codes or stdout/stderr; a failed sink cannot fail an operation.
Coverage is terminal observations, not a running-process census: killed or still-running
commands may have no record. Child and forge counters retain the documented lower bounds;
adding wrappers does not make custom runners or HTTP clients counted transports.

## Local perf-v3 profiles — 19 October 2026

The [attributed performance specification](performance-attribution-spec.md) is delivered incrementally by desktools-v2/63–65. The local schema, typed resolver,
writer and mixed reader are implemented in perfrec; CLI/launcher and provider adoption are
separate integrations. The [frozen API and wire contract](../perfrec/V3.md) describes exact
fields, bounds, coverage and examples. Human acceptance remains separate. The preceding field prohibitions remain binding on perf-v1/perf-v2; this section
scopes their future exception to a new schema and explicit workflow profile only.

Existing `ASSAY_TELEMETRY=1` without a selected new profile keeps prior producer behavior.
Explicit tool profile adds bounded v3 measurements without work/session/agent correlation;
workflow profile permits typed qualified **local** brief/issue references, random logical
session/agent IDs, fresh tool invocation and parent IDs, desk role and assigned execution
role. Local workflow data is identifiable, not anonymous. Unknown or invalid context remains
coverage, never guessed identity. No raw argv/path/title/body/error/credential or arbitrary
environment capture is permitted. File permissions and the seven-UTC-day diagnostic window
remain; a writer failure cannot change the tool result. There is no new sender or listener.

V3 also carries own-process forge attempt/timing observations and actual/estimated/unknown
GraphQL points with explicit units and measured sample coverage. Children emit their own
observations; parent inclusive time and child/request time are never summed into elapsed
workflow time. Future perfdata import applies its own explicit workflow consent and
source/epoch aliases before preview or upload; it never forwards raw local work locators or
native harness IDs. Existing counts-only network telemetry and its two switches are unchanged.

The current [provider adapter coverage](provider-performance.md) records actual sends at
the counted leaf, forwards response bodies without an additional read, and separates
desktoken authentication HTTP. Qualified public-provider queries may select bounded cost
metadata in the same operation. Other schemas retain unknown cost without a probe or
fallback request. Hot-path composition remains explicit; this is not fleet-wide coverage.

The planned v3 contract includes query/cache-service calls as a separate observation boundary:
client logical/transport counts and latency, handler/cache/coalescing work, and actual upstream
provider attempts/cost. Optional opaque diagnostic links can use only a separately admitted
service protocol/transport with workflow consent at both ends; a selected profile alone never
adds headers or forwards raw work/session/agent locators. Shared refresh cost is counted once,
missing server measurements remain unknown, and process CPU cannot be duplicated per handler.
