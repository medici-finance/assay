# Brief-flow event v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen/briefevent.go` (validation, fact store,
alias registry, historian adapter, export) and `statusgen/briefstage.go` (stage reducer).
**Schema:** [`schemas/brief-flow-event-v1.json`](../schemas/brief-flow-event-v1.json)
**Acceptance runner:** `bash tests/brief-flow/07.sh <mode>`

## 1. Scope

This document specifies `brief-flow-event/v1`: one JSON object per line, each recording one
observed fact about one brief's flow — written, a pull request opened, flipped ready, merged,
verified, done, reopened — together with how a reader validates, deduplicates, orders and
projects those facts. It exists so that every collector and every report describes the same
work in the same units: the **brief**, identified by its stable uuid, not a pull request, a
commit, a session or a board row.

It specifies:

- the event envelope and its mandatory and optional members (§3);
- brief identity and effective-dated aliases (§4);
- canonical fact ids, digests, corroboration, conflict and supersession (§5);
- source authority and precision precedence (§6);
- the deterministic stage reducer, first milestones versus episodes (§7);
- the board-historian adapter (§8);
- the portable export and its manifest (§9);
- report grouping and quantile rules (§10);
- consumer compatibility and the owner dispositions (§11, §12).

It does **not** specify wait or intervention receipts, measurement-context links, a
human-intervention record or report projection profiles. Those are later extensions of this
envelope; §3.3 is how they arrive without breaking a v1 reader. It defines no service, no
attempt lifecycle and no durable store: the export (§9) is the import/export format a durable
store reads and writes.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT",
"RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in
RFC 2119.

- **Fact** — one thing that happened once (pull request `pr:211` flipped ready). Identified by
  its fact id.
- **Observation** — one source's report of a fact. A fact may have several observations (the
  forge's event record and the acting tool's receipt); they corroborate, they never add.
- **First milestone** — the earliest qualifying fact of a user-defined boundary (§7.1). Set
  once, never moved.
- **Episode** — a repeated transition (a ready→draft regression, a pull-request reopen, a brief
  reopen), recorded beside the firsts rather than overwriting them.

## 2. Line format

A brief-flow event file is JSON Lines: one JSON object per line, UTF-8, `\n`-terminated. Blank
lines are ignored. A line that is not valid JSON, or an object with a duplicated member name,
MUST be refused; the refusal does not name the repeated member. Any valid JSON line MUST
otherwise be accepted as JSON defines it, every string escape (including `\/`) included. A
reader MAY refuse a line nested deeper than a fixed bound (64 levels in the reference
validator). Every timestamp is UTC in RFC 3339 form ending in `Z`.

## 3. The envelope

### 3.1 Mandatory members

| Member | Meaning |
|---|---|
| `schema` | The literal `brief-flow-event/v1`. Any other value MUST be refused: a later major version may change a meaning this reader would silently misread. |
| `fact_id` | The canonical fact id (§5.1). |
| `brief_uuid` | The brief's stable identity — its brief-v2 `id:` (uuid v4, lower case). |
| `alias` | The `<stream>/<NN>` alias the brief carried **when the fact occurred** (§4). |
| `milestone` | The fact kind: `written`, `alias_assigned`, `status_observed`, `pr_opened`, `pr_ready`, `pr_draft`, `pr_merged`, `pr_closed`, `pr_reopened`, `acceptance_scope`, `verified`, `done`, `reopened`, `retraction`. An unknown kind MUST be refused. |
| `occurred_at` | When the fact happened, as well as the source knows it. |
| `received_at` | When this observation reached the collector. Receipt order is not occurrence order. |
| `initiator_role`, `executor_role` | Role classification of who asked and who acted (§3.4). |
| `coverage` | `observed`, `partial` or `unknown`: whether the source read behind the observation was complete. Partial and unknown are never read as complete, and missing data is never read as zero. |

Kind-specific members are mandatory for their kind:

| Milestone | Also REQUIRED |
|---|---|
| `alias_assigned` | `alias_reason` (`authored`, `renumber`, `rename`, `archive`, `legacy`) |
| `status_observed` | `status_to` (and `status_from`, empty for a historian seed row) |
| `pr_opened` | `contribution` with `ref`, `kind` (`implementation` or `authoring`) and `draft` |
| other `pr_*` | `contribution` with `ref` |
| `acceptance_scope` | `acceptance` with `coverage` and `contributions` |
| `verified`, `done` | `acceptance` with `coverage` |
| `retraction` | `supersedes`, naming a fact other than itself |

### 3.2 Optional members

`occurred_after` / `occurred_before` (inclusive bounds on the occurrence instant), `precision`,
`source_authority`, `source_ref` (`kind`, `id`, `revision` of the source object read),
`brief_revision` (the brief-v2 `version:` in force at occurrence), `owner_cell` and
`owner_stream` (ownership **at occurrence**), `visibility` (`public` or `restricted`),
`contribution.replaces` (the contribution a replacement pull request supersedes),
`acceptance.revision`, `supersedes`, `import_run`, `must_understand`.

Bounds MUST be consistent: `occurred_after` ≤ `occurred_at` ≤ `occurred_before`, and
`received_at` MUST NOT precede the earliest possible occurrence. An absent `visibility` MUST be
read as `restricted`: nothing is published without an explicit `public`.

### 3.3 Unknown members: optional unless declared mandatory

A member this reader does not know is **optional**. The reader MUST keep it and MUST write it
back unchanged in any export (§9): an extension added by a later minor version round-trips
through an older reader. A producer that needs a new member to be understood lists its name in
`must_understand`; a reader that does not know a listed member MUST refuse the whole event
rather than guess. The refusal names the list position, never the value.

### 3.4 Actors are roles, never people

`initiator_role` and `executor_role` each take exactly one of `desk`, `reviewer`, `verifier`,
`worker`, `human`, `unknown`. The first five are the run-record role vocabulary
(graph-execution/06); the workflow-pattern-v1 node `role` enum is a subset of it. `unknown`
is an explicit value, never a default: a source that does not say who acted produces
`unknown`, and a reader MUST NOT fill it in. The two roles are separate because they differ —
a human may ask and a worker act.

An event MUST NOT carry a person identifier. A reader MUST refuse, before schema validation,
any event that has, at any depth:

- a member whose name, compared case-insensitively with `-` read as `_`, is a person
  identifier: exactly one of `login`, `user`, `username`, `email`, `mail`, `name`, `author`,
  `actor`, `person`, `principal`, `handle`, `display_name`, `full_name`, `user_id`,
  `committer`, `assignee`; or ending in `_login`, `_email`, `_name`, `_user`, `_username`,
  `_handle`, `_author`, `_person`, `_principal`, `_mail`; or containing `login`, `email`,
  `username` or `user_name`; or
- any member **name** or string value shaped like an email address.

The refusal MUST name only the member's position, never its value, and schema-validation
messages MUST NOT echo a refused value either. A member name is producer data too: in a
refusal's position path, a member whose name this schema does not declare is shown by its
position among its siblings (`member #3`), never by its name.

This scan is a name-and-shape heuristic, not a complete person-data filter. A bare login that
is not email-shaped, placed in a free-form string (an owner, a reference, a source id, an
extension member's value or name), is not caught. A producer MUST NOT rely on the reader to
remove person data it should never have emitted. Verifying that a `human` role is genuine stays
with the source that authenticated the act; the event records only the classification.

## 4. Identity and aliases

The accounting unit is the brief uuid. Pull requests, commits, sessions and board rows are
evidence about a brief, never units of throughput.

An alias (`example/01`) is a **locator**, effective-dated: an `alias_assigned` fact opens a
span for its brief at `occurred_at`; the span ends when the **same** brief is assigned its next
alias. Renumber, rename and archive therefore keep the uuid and add a span. An alias may later
name a different brief: `example/01` can name brief A in one span and brief B in a later one.

Resolving an alias at an instant:

| Situation | Result |
|---|---|
| Exactly one brief's span covers the instant | That brief's uuid. |
| No span covers it | `unknown` — refused, never guessed. |
| Two or more briefs' spans cover it — whether their claims start together (two `legacy` claims on one alias) or are staggered (brief B is assigned the alias while brief A's span is still open) | `ambiguous` — refused. A later assignment to another brief never closes an earlier brief's span, so it never wins by being later. |

An ambiguous legacy mapping needs a reviewed, permanent identity record; until then every
observation that names it is refused and counted in the export's `refused` coverage. A reader
MUST NOT deduplicate by alias or title, and MUST NOT mint a new identity per import.

## 5. Facts, digests and corrections

### 5.1 Fact ids

A producer SHOULD use the source's own immutable event id. A source without one uses the
deterministic legacy derivation:

```
legacy-v1:<first 24 hex of sha256("legacy-v1" \0 namespace \0 object \0 revision \0 kind \0 subject)>
```

Live capture and a later backfill of the same fact MUST derive the same id. For the board
historian (§8) the inputs are namespace `historian`, object `<owner>/<repo>:<alias>`,
revision = the row's commit sha, kind `status_observed`, subject = the row's `to` status.

### 5.2 Digest

The digest is `sha256:` + hex sha256 of the event's canonical JSON (sorted keys, no
insignificant whitespace) **excluding the observation members**: `received_at`,
`occurred_at`, `occurred_after`, `occurred_before`, `precision`, `source_authority`,
`source_ref`, `coverage`, `import_run`, `status_from`. The digest is the fact's meaning;
the excluded members describe one source's view of it.

### 5.3 Adding an observation

| Incoming observation | Result |
|---|---|
| New fact id | `added`. |
| Known id, byte-identical canonical form | `duplicate` — no-op. |
| Known id, same digest, different observation members | `corroborated` — kept as another observation of the same fact; never a second transition. |
| Known id, different digest | **held conflict** — refused, recorded with both digests, and the held fact stays in force. |

### 5.4 Supersession

A fact with `supersedes: <id>` withdraws the named fact; a `retraction` withdraws it without
replacement and is itself never projected. Supersession is applied once over the whole fact
set before projection. A target that does not exist, or that belongs to another brief, is a
reported problem, never a silent no-op.

## 6. Source precedence

When one fact has several observations, the reader projects the **preferred** one:

1. `source_authority`, in order: `forge` (the forge's own event record), `producer` (the acting
   tool's receipt written after the act succeeded), `git` (main-branch history), `historian`
   (board-historian rows), `backfill` (replayed historian rows); absent ranks last.
2. `precision`, in order: `event` (the source's own event time), `commit` (a commit-time
   proxy), `poll` (observed between two polls), `day` (date only); absent ranks last.
3. Earliest `received_at`.
4. Lexical order of canonical bytes — a total order, so the choice never depends on input order.

Precedence picks which observation's times and source a projection reports; it never changes
the fact's meaning, which the digest already fixed.

## 7. Stage reducer

Facts are replayed per brief uuid in `occurred_at` order, then fact id, then canonical bytes.
The result is independent of input order.

### 7.1 First milestones

| First milestone | Set by the earliest | Never set by |
|---|---|---|
| written | `written` fact (one birth per uuid) | a historian seed row, an alias, an authored date |
| coded | `pr_opened` of an `implementation` contribution | an `authoring` pull request |
| reviewed | `pr_ready` of an implementation contribution, or a `pr_opened` that was ready at creation (recorded as `ready_at_creation`, not an invented flip) | an approval |
| merged | `pr_merged` of an implementation contribution whose `executor_role` is `human` | an app merge (`desk`, `reviewer`, `verifier` or `worker`) — counted separately as `other_merges`; a merge whose executor is `unknown` is neither, and is counted as `unknown_merges` |
| verified, done | `verified` / `done` whose `acceptance.coverage` is `complete` | anything less — counted as `unqualified` |

Each first carries what was true **at occurrence**: fact id, time, precision, source authority,
both roles, owner cell and stream, contribution and brief revision. A later change of owner
does not move an earlier milestone's owner.

### 7.2 Episodes

| Episode | Fact |
|---|---|
| `ready_to_draft` | `pr_draft` on a contribution that was ready |
| `pr_reopen` | `pr_reopened` on a closed contribution (restores its prior state) |
| `brief_reopen` | `reopened` — clears current verified/done and the acceptance scope; earlier contributions belong to the prior round |

### 7.3 Current stage

The current whole-brief stage is computed from the live contribution set and the acceptance
owner's scope — **never from the furthest-ahead pull request**:

1. A current complete `done` → `done`; else a current complete `verified` → `verified`.
2. A pull-request fact whose contribution was never seen opened → `unknown` (a coverage gap).
3. Contributions that were replaced, belong to a prior round, or are authoring are set aside;
   a closed one is **abandoned**.
4. No live contribution: abandoned ones → `unknown`; a reopened brief → `reopened`; a written
   brief → `written`; otherwise `unknown`.
5. Live and abandoned contributions together → `mixed`.
6. With an acceptance scope: scope coverage not `complete` → `unknown`; scope ≠ live set →
   `mixed`; every live contribution merged → `merged`; all draft → `coded`; all ready →
   `reviewed`; otherwise `mixed`.
7. Without a scope: more than one live contribution → `mixed`; one → `coded` (draft),
   `reviewed` (ready) or `merged`.

So three pull requests never make three completions, an authoring pull request never makes
`coded`, and one merged contribution of three never makes the brief `merged`.

## 8. Board-historian adapter

The board historian's rows (`{ts, brief, from, to, sha, source}`) are read as
`status_observed` facts. The historian's format and its single writer are unchanged: the
adapter only reads, and nothing in this contract writes the historian log.

| Row | Authority | Precision | Bounds |
|---|---|---|---|
| live (`source` absent) | `historian` | `poll` | `occurred_before` = `ts`: the row is when the board was regenerated, an upper bound, not the transition instant |
| `source: backfill` | `backfill` | `commit` | — |
| any other `source` | refused | | |

The row's alias is resolved at `ts` (§4); an unknown or ambiguous alias refuses that row. The
fact id is the §5.1 legacy derivation over the row's `{brief, to, sha}` — the same key the
backfill replayer deduplicates on — so a live row and a replayed row of one transition are one
fact (`status_from` is an observation member, so the seed row and the replayed row
corroborate). Roles are `unknown`. A row whose `from` is empty is a **seed**: it records that
the brief was in a state when observation began. A seed is never a birth.

## 9. Portable export

An export is a directory holding:

- `events.jsonl` — every stored observation, one canonical event per line, sorted as §7;
- `manifest.json` — `schema` (`brief-flow-export/v1`), `event_schema`
  (`brief-flow-event/v1`), `events` (line count), `digest` (`sha256:` over the exact bytes of
  `events.jsonl`) and `coverage`: counts by source authority and by coverage value, the
  earliest and latest occurrence, and the number of held conflicts and refused inputs.

A writer MUST re-validate every event before writing (an event that bypassed validation cannot
carry a refused member out), MUST write each file atomically, and MUST refuse a destination
that is inside a git work tree: an export is operator data, never a committed artifact, and
never reaches the board historian. The work-tree test MUST resolve symbolic links on the
nearest existing ancestor of the destination before looking for a `.git` entry, so a symlinked
destination, or a symlinked parent followed by `..`, is judged where the files would actually
land. The reference writer creates the export directory owner-only (`0700`) and writes each
file through a fresh owner-only (`0600`) temporary file in the same directory, synced and then
renamed. **Limit:** the test looks for a `.git` entry on the destination's ancestors, so a work
tree whose git directory lives elsewhere (a separate `--git-dir` with `--work-tree`, or
`core.worktree`) and holds no `.git` entry is not detected; an operator using such a layout
chooses the destination accordingly. A reader MUST refuse an unknown manifest schema, a digest or
count mismatch, and any line that fails §3.

## 10. Report grouping and quantiles

These rules bind every report that consumes these facts.

- **Grouping.** Group by `owner_cell` / `owner_stream` **at occurrence** — stage durations by
  the owner at entry, completions by the owner at completion. Absent ownership stays an explicit
  unknown group. A roll-up across groups counts the **distinct union** of brief uuids, so a
  brief that moved between groups is counted once.
- **Cohorts.** A duration belongs to the window that contains its **exit**, and it is the full
  duration even when the entry precedes the window. Open intervals are excluded from completed
  durations and reported as aging instead. First-pass intervals and repeat episodes are separate
  series.
- **Pool, never average averages.** Mean, median and p85 are computed over the pooled
  intervals of the window, never over per-day summaries.
- **Median** is the sorted middle value, the mean of the two central values when n is even.
- **p85** is the nearest rank: the value at rank ⌈0.85·n⌉ (1-based).
- **Sample size.** Every statistic carries n; no mean or percentile is reported for n = 0, and
  small samples carry a low-sample label. Unknown or bounded durations are reported in
  coverage, never as zero.

Worked example: intervals of 2h and 4h exiting on one day and 24h exiting on another give, for
a window containing both days, mean **10h**, median **4h**, p85 **24h** (rank ⌈2.55⌉ = 3),
n = 3. Averaging the two daily means ((3h + 24h) / 2) gives 13.5h, which is wrong.

**Compatibility note.** The existing `statusgen` brief-flow lead-time percentile (`pctlDays`)
indexes the sorted list at ⌊n·q⌋ (0-based). That equals nearest rank ⌈n·q⌉ (1-based) whenever
n·q is not an integer, and is one rank higher when it is: at n = 20, ⌊17⌋ = index 17 is the 18th
value, while nearest rank ⌈17⌉ is the 17th. The existing metric keeps its documented
definition; a report built on this contract uses nearest rank and labels it.

## 11. Consumer compatibility

| Consumer | Status under this contract |
|---|---|
| Board historian (`statusgen/history.go`) | Format, encoder and single writer unchanged; read through §8 only. |
| Historian replayer (`statusgen/backfill.go`) | Unchanged; its `{brief, to, sha}` key is the §8 fact-id input, so replayed and live rows meet as one fact. |
| Brief-flow metrics (`statusgen/briefflow.go`) | Unchanged; they keep reading the historian directly (see the §10 note). |
| Run records (graph-execution/06) | Not landed. The role vocabulary agrees with its role list today; binding facts to run records waits for that owner. |
| Workflow instances (graph-execution/09) | Not landed. Instance references are a later optional member, added through §3.3. |

### 11.1 Retired assumptions

A report built on this contract no longer assumes that:

- a historian row's `ts` is the transition instant (it is an observation upper bound, §8);
- the first historian row of a brief is its birth (a seed is not a birth, §8);
- an alias, a number or a title identifies a brief (the uuid does, §4);
- a merged pull request means the brief is merged, verified or done (§7.3);
- an app merge is a human merge, or a merge by an unknown executor is an app merge (§7.1);
- a mean of daily means is a mean (§10).

## 12. Owner dispositions

| Owner | Disposition |
|---|---|
| Board historian and replayer (`statusgen`) | **Extended**: read-only adapter, no format change. |
| Brief-flow metrics (`statusgen`) | **Extended** by contract text (§10); code unchanged. |
| Run-record contract (graph-execution/06) | **Wait**: not landed; role vocabulary aligned and checked by the acceptance runner. |
| Instance contract (graph-execution/09) | **Wait**: not landed; no substitute instance or run schema is defined here. |
| Workflow pattern (`schemas/workflow-pattern-v1.json`) | Aligned: its node role enum is a subset of §3.4's and checked by the acceptance runner. |

## 13. Conformance

A conforming reader MUST implement §2–§6 and §8–§9 as written and MUST produce the §7 projection
for the published fixtures. The reference implementation's acceptance runner,
`bash tests/brief-flow/07.sh all`, checks identity, stage, compatibility, flow, role identity
and mutation sensitivity against synthetic fixtures under
`statusgen/testdata/brief-flow/contract/`, whose expected values are hand-written from this
document, not generated by the code under test. Fixture success is not deployment evidence.
