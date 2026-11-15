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

A session below strong tier does not group. An economy-tier session does mechanical work
only, as the intake skill's tier rule says. A judging session that is below strong tier
records only direct repairs, duplicates of existing work it can cite, and external waits,
and leaves every other item owed. Authoring a
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

What the match is decides what happens next:

- **A brief, a stream or an approved spec owns the mechanism.** Each member is recorded
  `duplicate` to that owner, with the owner's brief or issue reference as the successor. The
  owner's author is then asked, on the owner's own tracker item, to add the member numbers to
  its `issues:`. Intake does not edit another brief. The group is finished; steps 2 to 4 do
  not run for it. A production-down or security member is the exception: it is not recorded
  `duplicate`, because a `duplicate` is never scheduled as a repair and the class decision
  keeps such a symptom `todo`. Release it as its own direct repair (step 4). Ask the owner's
  author only to reference it, never to list it in `issues:`, because delivering the owner
  would then close it while its own repair is still open.
- **An open `error-class` issue owns the mechanism.** The class issue wins over a brief that
  also matches, because it holds the count. The group is not finished. Attach one instance
  block per member with `deskfile attach`, as the class decision defines the block. Recording
  a member `duplicate` to a class issue before attaching would leave the class count where it
  was. What happens next depends on the class issue's state:
  - **Not yet `design-owed`.** Go to step 2 and count the class again with the new blocks
    included.
  - **Already `design-owed`, and not parked under `needs-decision`.** The class already has
    its deliverable authored or queued, so the group parks and steps 2 and 3 do not run for
    it. This is the class decision's handling of a later symptom of a `design-owed` class:
    the symptom is parked on the class and the design deliverable's `Closes` closes it.
    Record each member that is not a production-down or security symptom `duplicate` to that
    deliverable: the class brief (`stream/NN`) when one exists on the main branch, because the
    writer resolves a brief only from the working tree, otherwise the class issue itself
    (`owner/repo#N`, which the writer accepts while it is open and admitted). Release a
    production-down or security member as its own direct repair (step 4). Never author a
    second class brief or a second investigation for a class that has one.
  - **Already `design-owed`, but still parked under `needs-decision`.** This is a
    `worker-desk` rule-4 risk-bearing class that is waiting for the driver (step 3, first
    bullet). Nothing is authored or queued, and the ruling may be a decline, which sets the
    class's symptoms `todo`. A `duplicate` here would discharge the member with no wake
    condition, so a decline would strand it. Attach the blocks, run neither step 2 nor
    step 3, and record each member that is not a production-down or security symptom
    `external-prerequisite` exactly as step 3's first bullet does for the members that
    found the class waiting: the driver as the external role, the ruling as the action and
    its arrival as the wake condition. Release a production-down or security member as its
    own direct repair (step 4).

## Step 2: apply the trigger

With no owner, or with an open class issue as the owner, count exactly as the class decision
counts it. For a class issue, the count is its existing counted instances plus this group's
new blocks. Only `confirmed-defect` and `false-positive` instances count, one per
`incident-group`. The class is `design-owed` at 3 counted instances, at the class's second merged fix, or at the
first instance in a module carrying a `brittle` mark. Nothing here lowers or amends that
trigger. A group below it goes to step 4; a group at it goes to step 3.

**A backlog group that meets the threshold is `design-owed` on this pass.** The backlog
already holds the instances; there is no later arrival to wait for.

## Step 3: author the class brief

When the trigger is met, the class decision's handling at the trigger applies unchanged.
Three parts of it change what this step does:

- **A risk-bearing surface waits for the driver.** A class on a `worker-desk` rule-4
  risk-bearing surface (public-repo copy, security, production deploy, irreversible actions)
  stays parked under `needs-decision` until the driver rules. No class brief is authored on
  this pass. Its members stay parked on the class issue as the class decision says, and each
  is recorded `external-prerequisite` with the driver as the external role, the ruling as
  the action and the ruling's arrival as the wake condition. Authoring waits for the ruling.
  A member that arrives in a later batch, while the class is still waiting, is recorded the
  same way (step 1, the second `design-owed` case).
- **A `brittle` first is an investigation, and intake authors nothing.** When the trigger is
  met by the first instance in a module carrying a `brittle` mark, the deliverable is the
  investigation per build-less-brittle/09. That brief has a dispatched strong-tier session
  produce it under `docs/investigations/`; the intake session does not write it, and does not
  author a class brief in its place. Open the class issue if none exists (with one instance
  block per member), label it `design-owed`, and unpark its placeholder with the deliverable
  line `the investigation per build-less-brittle/09`, as the class decision defines.
  Queueing that placeholder is how the investigation is dispatched. An investigation file is
  not a successor the writer accepts, so record each member that is not a production-down or
  security symptom `duplicate` to the class issue (`owner/repo#N`). The rest of this step
  does not run for it.
- **Production-down and security members are not folded into the brief.** Each is released
  as its own direct repair (step 4) and stays dispatchable. The brief does not list it in
  `issues:`, because delivering the brief would close it while its own repair is still open;
  the brief names it in its body as related work. It is not recorded `duplicate`, and it is
  not parked.

Otherwise author one class brief with `author-brief`, in the **same
reviewed change** as the members' judgments. The brief states:

- the mechanism, in one line;
- the owning module;
- every member except a production-down or security symptom, in `issues:`;
- every site the mechanism can recur at, not only the reported ones;
- the existing or planned work it was matched against in step 1, and the result;
- what it retires;
- a class-guard Verify row that fails on a planted second instance.

These are the existing `author-brief` rules (9(d), 14) and `design-fit`. The class brief is
a draft pull request like any brief. When no class issue exists yet, open it in the same
change as the class decision defines it (title `class: <mechanism>`, label `error-class`),
with one instance block per member, so later arrivals of the mechanism match on it and the
brief reuses the class record. Label the class issue `design-owed` and unpark it as the class
decision defines. Record each member that is not a production-down or security symptom as
`duplicate` to the brief. The writer checks that the successor brief exists in the working tree, so the brief is
committed before the companions are recorded.

## Step 4: release the rest

A group below the trigger is not a class brief. Open or update the class issue with one
instance block per member, as the class decision defines the block (an existing class issue
already received its blocks in step 1), and record each member as its own direct `repair`:
route `repair`, successor the issue itself. A production-down or security member from step 1
or step 3 is released the same way.

## What grouping never does

It never merges evidence, never marks a duplicate on the tracker, never closes an issue,
never dispatches a worker and never takes a claim. A `duplicate` judgment records routing and
closes nothing. The duplicate rule is unchanged: the evidence merges first, a strong worker
marks the duplicate, and an independent pass closes it.

## Partial results

A class brief without its members' judgments is incomplete, and the members stay owed. A
judgment written but not published leaves the placeholder classification-needed and out of
dispatch, and the next pass does not show it as owed: the local history already counts it as
settled, so nothing reports it. A refused or unreadable write is could-not-check, never an
empty queue. Record the member judgments with `scanloop land --handoff`, then publish them
and read them back in the same session, as the intake skill's "Accountable handoff" passage
and `intake-handoff.md` sections 6 and 7 describe. A class pass records many judgments at
once, so read back every member, not only the first.

## Examples

One worked batch from a placeholder repository, `example-org/example`, covering every
outcome. Every value is a placeholder. A test (`TestClassPassExamples`) reads each
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

### Group E: a class issue on a risk-bearing surface

An open, parked class issue already counts one instance. Issues 141 and 142 report the same
mechanism on a risk-bearing surface. Step 1 matches the class issue and attaches one block
per member, so the count reaches 3 and the trigger is met. Because the surface is
risk-bearing, the class issue stays parked under `needs-decision` and no class brief is
authored on this pass. Each member waits on the driver's ruling.

outcome: parked

expect: external-wait external

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#141",
  "opened": "2026-01-08T08:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "external-prerequisite",
  "route": "external",
  "external_role": "human",
  "required_action": "Rule on the parked class issue: proceed to a design brief or decline.",
  "wake_condition": "The driver's ruling is recorded on the class issue.",
  "evidence": "example-org/example#141"
}
```

expect: external-wait external

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#142",
  "opened": "2026-01-08T08:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "external-prerequisite",
  "route": "external",
  "external_role": "human",
  "required_action": "Rule on the parked class issue: proceed to a design brief or decline.",
  "wake_condition": "The driver's ruling is recorded on the class issue.",
  "evidence": "example-org/example#142"
}
```

### Group F: a class that already has its deliverable

The open class issue `example-org/example#150` is already `design-owed`, and its class brief
`example-class/02` was authored by an earlier pass. Issue 151 reports the same mechanism in
a later batch. Step 1 matches the class issue and attaches one block. Because the class is
already `design-owed`, the group parks: steps 2 and 3 do not run, and no second class brief
is authored. The member is `duplicate` to the existing brief.

outcome: owned

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#151",
  "opened": "2026-01-09T08:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-class/02",
  "evidence": "example-org/example#151"
}
```

### Group G: a `brittle` first

Issue 161 is the first confirmed instance of a mechanism in a module carrying a `brittle`
mark. The trigger is met at the first instance, and the deliverable is the investigation
per build-less-brittle/09. Intake authors neither the investigation nor a class brief. It
opens the class issue `example-org/example#160` with one block, labels it `design-owed` and
unparks its placeholder with the investigation deliverable line, so a dispatched session
produces the investigation. The investigation file is not a successor the writer accepts, so
the member is `duplicate` to the class issue.

outcome: investigation

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#161",
  "opened": "2026-01-09T09:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "duplicate",
  "route": "duplicate",
  "successor": "example-org/example#160",
  "evidence": "example-org/example#161"
}
```

### Group H: a later member of a class still waiting for the driver

The class issue from Group E is `design-owed` and still parked
under `needs-decision`: the driver has not ruled. Issue 171 reports the same mechanism in a
later batch. Step 1 matches the class issue and attaches one block. No deliverable is
authored or queued, and a decline would set the symptoms `todo`, so the member is not
`duplicate`. It is `external-prerequisite`, waking on the driver's ruling like the members
in Group E.

outcome: waiting

expect: external-wait external

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#171",
  "opened": "2026-01-10T08:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "external-prerequisite",
  "route": "external",
  "external_role": "human",
  "required_action": "Rule on the parked class issue: proceed to a design brief or decline.",
  "wake_condition": "The driver's ruling is recorded on the class issue.",
  "evidence": "example-org/example#171"
}
```

### Group D: a quarantined item

Issue 131 was written by an account the trust gate does not admit, and no blessing covers
it. It is not read into any group and is not counted. It has no record: not a repair, not a
duplicate.

outcome: quarantined

expect: none
