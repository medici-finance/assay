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
67 operational desk commands each append one `perf-v1` record per completed invocation to
`~/.config/assay/perf/<YYYY-MM-DD>.jsonl`; `deskperf report` and `deskperf boot` read it back.
Day files are kept for 7 UTC days and pruned on write. With the variable unset — or set to
anything other than exactly `1` — nothing is created: no directory, no file, no install id.

It sends nothing. No file that implements it opens a network connection, and there is no
endpoint or sender for it anywhere. That is why it has **one** switch rather than two: the
two-switch rule above exists so that no inherited environment can cause a *send*, and this
log is a file on your own disk. The single switch still means consent — a user who never
opted in gets nothing new. **The two-switch rule binds any future remote sink for these
records in full**: a sender for the perf history needs its own explicit flag in addition
to `ASSAY_TELEMETRY=1`, exactly as `statusgen --telemetry` does.

Every record carries exactly these fields and no field capable of holding free text:

| Field | Type | Meaning |
|-------|------|---------|
| `schema` | string | Record schema version, `perf-v1`. Adding any field bumps it. |
| `ts` | string | UTC timestamp, second resolution. |
| `tool` | string | The desk tool's canonical key from a compiled-in roster; anything else is `other`. |
| `verb` | string | The tool's own registered subcommand, or `other`; a boot step is `step:<name>` from `deskboot`'s compiled step list (else `step:other`). Never argv text. |
| `role` | string | `$DESK_LOOP` mapped onto the compiled loop-name roster; unset or unknown is `none`. |
| `wallMs` | int | Wall-clock duration of the invocation (or boot step). |
| `cpuMs` | int, optional | Process user+system CPU; **omitted** where it cannot be measured (Windows, boot steps), so "unmeasured" never reads as zero. |
| `exit` | int | The verb's exit code. |
| `result` | string | The exit code mapped onto a fixed word (`ok`, `disabled`, `refused`, `unverifiable`); never the input. |
| `childProcs` | int | Child processes the invocation started through the shared subprocess runner (`deskkit.Run`). |
| `childWallMs` | int | The summed wall time of those counted children. |
| `forgeCalls` | int | HTTP round-trips through the forge client's counting transport (counted, never inspected). |
| `sourceSHA` | string | The binary's build stamp when it is hex, else `unpinned`. |
| `install` | string | 32 random hex characters minted once per state directory. Identifies nothing but the install. |

`childProcs` and `forgeCalls` count only what passes through those two shared seams, so
they are lower bounds, not totals. A verb that starts its own children or makes its own
HTTP calls outside them is not counted. The known cases: `deskboot` starts its boot-step
children through its own runner; `desktoken`'s token mint and `deskpost`'s posts use their
own direct HTTP client; and the repository-visibility read uses one when no client is
injected. A `0` from those tools means "not counted", not "none made".

Never recorded: argv, flag values, repo, PR or issue numbers, branch, path, title, body,
error text, session tag, environment values, hostname, user name, or tokens. A test plants a
repo slug, a PR number, a branch, an absolute path, a session tag and a token-shaped
environment value, and asserts none of them appears in the written bytes.

### statusgen writes the same local record

`statusgen` appends to the same history. With `ASSAY_TELEMETRY=1` exported, each `statusgen`
process writes one `perf-v1` record on exit into the same `~/.config/assay/perf/` day files,
with `tool` set to `statusgen`, `verb` the mode that ran (a positional sub-command such as
`verifyrun`, else the first mode flag such as `--lint`, else `regen` for a plain run; a token
outside that compiled-in roster is `other`), `exit` the process exit code and `result` its
fixed word. `role` is `none`, `sourceSHA` is the build stamp when it is hex (a release tag or
`dev` is `unpinned`), and the child and forge counters stay `0` ("not counted"), so
`deskperf report --tool statusgen` reads it like any other tool. With the variable unset, or
anything other than exactly `1`, `statusgen` creates nothing and behaves as it did before.

This is the same local file, not the network payload above: the payload still needs
`--telemetry` as well, and nothing in this section sends anything. The record is written by
one shared module (`perfrec/`) that the desk tools and `statusgen` both call, so the schema,
layout, retention and install id exist once.

The perf history is not the desk audit ledger and never reads or writes it. It also holds no
agent usage: per-invocation tokens and cost belong to the agent runner's own usage record.

## Dependency posture

The client uses the Go standard library only (`net/http`), consistent with
`statusgen`'s stdlib + `yaml.v3` rule. No third-party telemetry SDK is
introduced.

### Command coverage

Every operational command under tools/desk/cmd has exactly one optional performance wrapper.
The inventory test maintains two explicit exceptions: deskperf must not measure its own report
reads, and muhar is a developer mutation harness. The original seven commands keep their
registered verb vocabularies; added commands record the fixed `other` verb. User argv, file
paths, subjects, credentials and free-form error messages never enter these records.
On/off does not change exit codes or stdout/stderr; a failed sink cannot fail an operation.
Coverage is terminal observations, not a running-process census: killed or still-running
commands may have no record. Child and forge counters retain the documented lower bounds;
adding wrappers does not make custom runners or HTTP clients counted transports.
