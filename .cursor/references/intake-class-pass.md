<!-- GENERATED from plugins/assay/references/intake-class-pass.md by deskinstall --harness cursor. Do not edit. -->
# Intake class pass: work the owed batch by mechanism

<!-- assay:harnesslint non-matrix-reference — harness-neutral procedure read by the intake-desk skill, not a per-harness capability binding; it names no harness capability and carries no per-skill degradation cells -->

Each intake pass presents the oldest retained, unjudged issues as `intake:` lines under
`EMITTED`, up to a per-repository bound. That set is **the owed batch**. The class decision
in the intake skill (item 1 of "The judgment half") judges one new issue at a time, so
symptoms of one mechanism arrive as unrelated items and are filed, labelled and fixed again
and again. This procedure groups the batch before it is judged. It reuses the class record,
the counting and the `design-owed` trigger unchanged, and the `intake-handoff-v1` record as
shipped (`spec/intake-handoff-v1.md`). It adds no verb, flag, label or record field.

## When

Run it before judging the owed batch item by item, on a **strong-tier session**.

A session below strong tier does not group. It records only direct repairs, duplicates of
existing work it can cite, and external waits, and leaves every other item owed. Authoring a
class brief is strong-tier work (`author-brief` rule 9(d)), and the writer refuses a
`needs-scoping` judgment from a session below strong tier.

## Group

Group the batch **by mechanism**: the shared cause, not the shared symptom. Use three
inputs: the owning module each issue names, the issue's labels, and the open `error-class`
issues (title `class: <mechanism>`).

- Quarantined or unblessed text is never read into a group. An item the trust gate holds
  back or retired as `quarantined` gets no group, no count and no record.
- A single unrelated item goes straight to its own judgment.
- Grouping reads; it records nothing. Every outcome below is recorded only in steps 1 to 4.

## Step 1: match existing work

For each group, look for work that already owns the mechanism, in this order:

1. Open `error-class` issues, including a parked one.
2. Briefs and streams whose title, `why:` or `files:` name the mechanism or the module.
3. Approved or routed specs that cover it.
4. Where the installed tools support it, the context discovery that
   [`implementation-pickup.md`](implementation-pickup.md) describes
   (`statusgen context`); otherwise its manual bootstrap.

A match means **each member is recorded `duplicate` to that owner**, with the owner's brief
or issue reference as the successor. The owner's author is then asked, on the owner's own
tracker item, to add the member numbers to its `issues:`. Intake does not edit another
brief. The group is finished; steps 2 to 4 do not run for it.

## Step 2: apply the trigger

With no owner, count the group exactly as the class decision counts it. Only
`confirmed-defect` and `false-positive` instances count, one per `incident-group`. The
class is `design-owed` at 3 counted instances, at the class's second merged fix, or at the
first instance in a module carrying a `brittle` mark. Nothing here lowers or amends that
trigger.

**A backlog group that meets the threshold is `design-owed` on this pass.** The backlog
already holds the instances; there is no later arrival to wait for.

## Step 3: author the class brief

When the trigger is met, author one class brief with `author-brief`, in the **same
reviewed change** as the members' judgments. The brief states:

- the mechanism, in one line;
- the owning module;
- every member, in `issues:`;
- every site the mechanism can recur at, not only the reported ones;
- the existing or planned work it was matched against in step 1, and the result;
- what it retires;
- a class-guard Verify row that fails on a planted second instance.

These are the existing `author-brief` rules (9(d), 14) and `design-fit`. The class brief is
a draft pull request like any brief. Record each member as `duplicate` to the brief. The
writer checks that the successor brief exists in the working tree, so the brief is
committed before the companions are recorded.

## Step 4: release the rest

A group below the trigger is not a class brief. Open or update the class issue with one
instance block per member, as the class decision defines the block, and record each member
as its own direct `repair`: route `repair`, successor the issue itself.

## What grouping never does

It never merges evidence, never marks a duplicate on the tracker, never closes an issue,
never dispatches a worker and never takes a claim. A `duplicate` judgment records routing and
closes nothing. The duplicate rule is unchanged: the evidence merges first, a strong worker
marks the duplicate, and an independent pass closes it.

## Partial results

A class brief without its members' judgments is incomplete, and the members stay owed. A
judgment written but not published leaves the placeholder classification-needed, and the
next pass shows it still owed. A refused or unreadable write is could-not-check, never an
empty queue. Record the member judgments with `scanloop land --handoff`, and publish them
as the intake skill's "Accountable handoff" passage describes.

## Examples

One worked batch from a placeholder repository, `example-org/example`, covering all four
outcomes. Every value is a placeholder. A test (`TestClassPassExamples`) reads each
`outcome:` line, each `expect: <state> <route>` line (a trailing ` direct` marks a direct
repair) and the record after it, decodes the record, assesses it as the shipped code does,
and fails when the result differs. `expect: none` means the item has no record.

### Group A: matched to an existing brief

Issues 101 and 102 both report that the example fetcher gives up after one failed request.
Step 1 finds the brief `example-stream/07`, whose `why:` names that mechanism. Both members
are `duplicate` to it, and its author is asked to add 101 and 102 to its `issues:`.

outcome: matched

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#101",
  "opened": "2026-01-05T09:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-stream/07",
  "evidence": "example-org/example#101"
}
```

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#102",
  "opened": "2026-01-05T09:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-stream/07",
  "evidence": "example-org/example#102"
}
```

### Group B: meets the trigger

Issues 111, 112 and 113 each report that the example parser accepts a path it never
validated, in three separate incident groups, all `confirmed-defect`. No owner matches in
step 1. Three counted instances meet the trigger, so the class is `design-owed` on this
pass. One class brief, `example-class/01`, is authored in the same change; it lists 111,
112 and 113 in `issues:`, names every call site that reaches the parser, and carries a
class-guard Verify row that fails on a planted second unvalidated path. Each member is
`duplicate` to the brief.

outcome: class

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#111",
  "opened": "2026-01-06T10:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-class/01",
  "evidence": "example-org/example#111"
}
```

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#112",
  "opened": "2026-01-06T10:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-class/01",
  "evidence": "example-org/example#112"
}
```

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#113",
  "opened": "2026-01-06T11:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-class/01",
  "evidence": "example-org/example#113"
}
```

### Group C: a pair below the trigger

Issues 121 and 122 share a mechanism, two counted instances, no owner and no `brittle`
mark. The class issue is opened with one instance block per member and stays parked. Each
member is its own direct repair, successor the issue itself.

outcome: below

expect: observed repair direct

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#121",
  "opened": "2026-01-07T08:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "route": "repair",
  "successor": "example-org/example#121",
  "evidence": "example-org/example#121"
}
```

expect: observed repair direct

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#122",
  "opened": "2026-01-07T08:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "route": "repair",
  "successor": "example-org/example#122",
  "evidence": "example-org/example#122"
}
```

### Group D: a quarantined item

Issue 131 was written by an account the trust gate does not admit, and no blessing covers
it. It is not read into any group and is not counted. It has no record: not a repair, not a
duplicate.

outcome: quarantined

expect: none
