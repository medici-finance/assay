<!-- GENERATED from plugins/assay/references/intake-handoff.md by deskinstall --harness cursor. Do not edit. -->
# Record and publish an intake handoff

<!-- assay:harnesslint non-matrix-reference — harness-neutral intake procedure shared by the intake-desk skill, not a per-harness capability binding; it defines no harness capability or per-skill degradation cells -->

Intake judges an issue, comments on it and labels it. That is not the end of the work: a
placeholder is schedulable only after an accountable judgment about it is recorded and
published, and a judgment that exists only in a local log assigns nothing. This reference is the
step that records and publishes the judgment. The record's meaning belongs to
`spec/intake-handoff-v1.md`; this file consumes that spec and changes none of it.

Every record below is an example a test decodes and assesses (`TestHandoffProcedureExamples` in
`tools/desk/cmd/scanloop`). An example that disagrees with the shipped assessment fails CI, so
copy one and change its values rather than writing a record from memory. All names in the
examples are placeholders.

## 1. When a judged issue needs a companion

Every issue you route by judgment needs one companion, recorded and published, before it is
routed. A mechanical exit, scan PR, label, routing comment or closed issue is not accepted
successor work.

Two cases need none:

- An accountable `rejected-watching` exit that you recorded by judgment, with that event's
  `--opened`. A rejection has no successor to read back, so the exit record is the discharge.
- An item parked on a `needs-decision` issue. The decision issue is its owner.

Every other judged issue is one row of the table in section 2.

## 2. The judgment table

| Judgment | `disposition` | `route` | `successor` | Also |
|---|---|---|---|---|
| A worker can take the issue as written | `repair` | `repair` | the issue itself | |
| Existing work already covers it | `duplicate` | `duplicate` | that brief or issue | |
| It belongs to a class owed a design | `duplicate` | `duplicate` | the class brief | |
| It needs scoping first | `needs-scoping` | `authoring` | a brief that declares strong execution | `author_tier: strong`, and a strong session |
| Someone outside the pipeline must act first | `external-prerequisite` | `external` | none | `external_role`, `required_action`, `wake_condition` |
| The source is already delivered | `repair` | `qualification` | the qualifying work | `delivered: true` |

Successor rules. Scoping queues a linked author-spec/author-brief task for a strong author. A
sufficiently scoped repair may use the existing issue; delivered source goes to
installation/qualification. An external prerequisite names the role, exact action and wake
condition. Reuse existing owner work, including when a source issue is closed as a duplicate.

A routed successor owns its own admission and claim. Only the first row, a direct repair, makes
the source placeholder itself schedulable; every other row records where the work went, and the
source placeholder stays off the dispatch queue.

Record rules the writer enforces:

- `item` is the issue exactly as `--item` gives it, `opened` is the event time exactly as
  `--opened` gives it, and `deciding_role` is the loop you run as. Any difference refuses.
- `revision` starts at 1. A new judgment for the same event is the next revision; the writer
  appends and never edits.
- `evidence` is never empty. `successor` is a typed reference: a brief as `<stream>/<NN>` or an
  issue as `owner/repo#N` or `#N`. The writer reads it back through the forge or the brief file.
- Leave out `verification`; it belongs to the verifier-blocker lane, not to a judged issue.

## 3. One example per row

Each example is preceded by a line `expect: <state> <route>`, with a trailing ` direct` when the
record is a direct repair. The state and route are what the shipped assessment returns for the
record.

A worker can take the issue as written (a direct repair, successor is the issue itself):

expect: observed repair direct

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#12",
  "opened": "2026-11-01T09:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "delivered": false,
  "route": "repair",
  "successor": "example-org/example#12",
  "author_tier": "",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "Issue body names the failing check and the file; a worker can take it as written."
}
```

Existing work already covers it:

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#13",
  "opened": "2026-11-01T10:05:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "delivered": false,
  "route": "duplicate",
  "successor": "example-stream/03",
  "author_tier": "",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "Brief example-stream/03 already names this mechanism and lists the symptom."
}
```

It belongs to a class owed a design:

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#14",
  "opened": "2026-11-01T10:40:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "delivered": false,
  "route": "duplicate",
  "successor": "example-class/01",
  "author_tier": "",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "Third counted instance of the mechanism that class brief example-class/01 closes."
}
```

It needs scoping first (strong session only):

expect: observed authoring

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#15",
  "opened": "2026-11-01T11:15:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "needs-scoping",
  "delivered": false,
  "route": "authoring",
  "successor": "example-stream/04",
  "author_tier": "strong",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "The issue asks for a new capability with no design; brief example-stream/04 scopes it."
}
```

Someone outside the pipeline must act first:

expect: external-wait external

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#16",
  "opened": "2026-11-01T11:50:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "external-prerequisite",
  "delivered": false,
  "route": "external",
  "successor": "",
  "author_tier": "",
  "external_role": "operator",
  "required_action": "Rotate the example credential named in the issue.",
  "wake_condition": "The issue reporter confirms the rotation on the issue.",
  "evidence": "The fix needs an account change that only an operator can make."
}
```

The source is already delivered:

expect: observed qualification

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#17",
  "opened": "2026-11-01T12:25:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "delivered": true,
  "route": "qualification",
  "successor": "example-stream/05",
  "author_tier": "",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "The change this issue asks for merged earlier; brief example-stream/05 qualifies it."
}
```

## 4. The command

With readers supporting `intake-handoff-v1` (installed tools at v1.0.62 or later), record the
source event, deciding role, evidence and disposition for one item with `scanloop land ...
--handoff <json-file> --handoff-reader intake-handoff-v1`. The original exit is retained and the
call appends a new judgment revision.

```
scanloop land --item example-org/example#12 --opened 2026-11-01T09:30:00Z --kind new-issue \
  --exit placeholder --artifact example-org/example#12 --tier strong \
  --handoff issue-12.handoff.json --handoff-reader intake-handoff-v1
```

- Run it from the intake window, so `$DESK_LOOP` names the running loop; that loop is the
  record's role, and the companion's `deciding_role` must be the same name.
- `--opened` and `--kind` are the values the drain's `EMITTED` line printed for the event.
- `--exit` and `--artifact` are the exit and typed reference the item takes, exactly as the
  skill's exit-record passage describes. The companion refines that exit and never changes it; a
  call that names a different exit for an event that already left keeps the first exit and says
  the new one was not recorded.
- `--tier strong` only from a session on a strong tier. Never claim a tier the session is not on.
- The file after `--handoff` is the file you publish in section 5.

The write is idempotent for an exact retry. If the exit write succeeded and the companion write
failed, run the same command again; it still attempts the companion.

## 5. Publish it by hand

The local `intake-handoffs-v1.jsonl` history alone is not an assignment authority, and the call
above writes nothing into the stream root. The board reads only a JSON file beside the
placeholder, so publication is a separate step.

1. Write the record as `<placeholder stem>.handoff.json` in the placeholder's directory, and land
   that file in section 4. The stem is `issue-<N>` for the repository the scan runs in and
   `<repository name>-issue-<N>` for any other, so the companion for `issue-12.md` is
   `issue-12.handoff.json`.
2. In the placeholder's frontmatter set `handoff:` to that bare file name and `handoff-opened:`
   to the event time, quoted, exactly as `opened` reads in the record. Unquoted, the time is read
   as a date rather than text, the pointer is ignored and the issue stays held.
3. Commit both in the same reviewed intake change as the placeholder, through the normal
   reviewed change. Never leave either one only in a working tree.

Before committing a companion to a public repository, scrub its free-text `evidence`,
`required_action` and `wake_condition` fields; they publish as written. A repository whose
visibility the session cannot read is treated as public. The handoff spec
(`spec/intake-handoff-v1.md`) states the same rule.

## 6. Read it back

- The next pass report. An issue still owed prints an `intake:` line under `EMITTED` with its
  assessment, and a settled one no longer queues. If the line is still there, the judgment is
  not discharged.
- The dispatch queue, for a direct repair only. Read it uncapped, in the root that holds the
  placeholder:

  ```
  statusgen --root . --next-up --span 100000 | grep -c '"stream": *"issue-loop"'
  ```

  It prints 1 or more once a direct repair is recorded and published. The default dispatch view
  is capped at 20 rows and 4 per stream, so it can omit an eligible placeholder; the filter keys
  on the `issue-loop` stream because a placeholder's id has two shapes. A 0 means one of: the
  judgment is not a direct repair, the pointer or time is wrong, or something held the row. The
  same output reports `heldByStreamCap` and `heldByDriveWorkerCap` (absent means 0), which count
  held rows for the whole root: both at 0 rules a hold out, and either above 0 leaves the read
  could-not-check, never a pass.
- The successor. Read it back through its owner before treating the handoff as discharged; a
  stopped scan carrier leaves successor-owed work.
- The register view. `statusgen --intake-handoffs` reports the same meaning without changing
  `--intake-exits`. It reads only the committed intake register, never an issue's local
  companion, so it cannot confirm an issue.

Legacy or unreadable companions remain classification-needed.

## 7. When the step fails

- A refused or unreadable handoff write leaves the item owed. Report it as could-not-check,
  never as an empty queue. Do not edit `intake-handoffs-v1.jsonl` or `intake-exits.jsonl` by
  hand, and never invent a new source event or a different `--opened` to get past a conflict;
  file an issue against the tooling naming the item and the message.
- A judgment recorded but not published leaves the placeholder `classification-needed`. The next
  pass shows the item still owed; publish it then.
- A class brief authored without its members' judgments is incomplete, and the members stay
  owed.
- A session below strong tier records `repair`, `duplicate` and external judgments only, and
  queues the rest. It never records a `needs-scoping` judgment: the writer refuses it, and a
  strong session picks the item up.
- A successor the writer cannot read, or that is not open, refuses the write. The writer does
  not create another issue to supply one.
- Do not promote raw placeholders or open a new issue merely to acknowledge an observation.

## 8. Replay and the register

The drain replays durable observations after restart and still applies current trust; use the
displayed judgment to reconcile existing work, not to repeat it.

Replay of an issue stops only when its latest event has a recorded companion assessed `observed`
or `external-wait`, or an exit you recorded by judgment as `rejected-watching` with that event's
`--opened`. A mechanical exit never stops it. A newer source event on the issue presents it
again.

A `RETIRED` line records a terminal `gone` or `quarantined` disposition for one event. A
`DEFERRED` line names retained events held past this pass's replay bound. Neither line is an
empty queue.
