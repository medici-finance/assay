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

## Dependency posture

The client uses the Go standard library only (`net/http`), consistent with
`statusgen`'s stdlib + `yaml.v3` rule. No third-party telemetry SDK is
introduced.
