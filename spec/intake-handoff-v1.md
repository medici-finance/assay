# intake-handoff-v1 — accountable intake judgments

This companion is separate from [intake-exit-v1](intake-exit-v1.md). The two
existing exit writers retain every v1 key and the five exits. A companion records
an accountable judgment about one source event; it does not grant a claim,
permission, issue closure, installation or independent verification acceptance.

`deskcore/issueflow` owns strict decoding and assessment. Readers advertise exact
support with `SupportsIntakeHandoff`. Unknown versions, absent records and
incomplete reads are classification-needed. Do not require a companion writer
until the participating readers support this version; deployment remains a
separate act.

## Record

| Key | Meaning |
|---|---|
| schema | Exactly `intake-handoff-v1`. |
| item | Existing issue reference or register ID; no new case identity. |
| opened | Source event time, RFC3339; distinct events retain distinct keys. |
| revision | Positive per-event judgment revision, starting at 1. |
| deciding_role | Existing closed desk-role vocabulary from intake-exit-v1. |
| disposition | unresolved, needs-scoping, repair, duplicate, external-prerequisite. |
| route | authoring, repair, qualification, duplicate or external. |
| delivered | Source implementation is already delivered; requires qualification. |
| successor | Existing typed brief or issue/PR reference; no routing comment alone. |
| author_tier | strong for an authoring handoff. |
| external_role | human, operator, brief-author or verifier, for external prerequisites. |
| required_action | Exact next action for the external owner. |
| wake_condition | Observable condition for reconsidering the external prerequisite. |
| evidence | Evidence reference supporting the judgment, retained with history. |
| verification | Optional original verification-handoff-v1 source-bound observation. |

Unknown and duplicate JSON keys are refused by the shared strict decoder.
A needs-scoping judgment requires a linked strong-author task. A repair judgment
routes either to implementation or to qualification of delivered source. A
duplicate points to existing work. An external prerequisite requires an owner,
action and wake condition, with no conflicting successor. The result `observed`
means the judgment is recorded, not that the successor accepted or completed it.

Verification observations retain the /01 exact-source and freshness checks.
An offline reader with no independent source observation must report
classification-needed rather than corroborating a record against itself.

`deciding_role` is self-asserted by the writer. It binds the record to the
acting lane's command, not to a proven identity; accountability rests on the
reviewed change that publishes the companion. The free-text `evidence`,
`required_action` and `wake_condition` values are published as written, so scrub
them before committing a companion to a public repository.

## Existing writers

The issue lane accepts `scanloop land ... --handoff <json-file>
--handoff-reader intake-handoff-v1`. It binds the companion to the command's item,
source event and acting role. It checks a successor through the existing typed
forge adapter or a declared brief file. Strong authoring requires strong execution
on both the deciding session and linked brief. An unread or missing successor
refuses; it does not create another issue automatically. A declared brief file
is checked in the writer's working tree as a convenience only; the reviewed
intake change must commit that successor, and readers check it there. When the
companion carries a verification observation, the writer checks that
observation's consistency only. That check is not a source observation, and no
reader may reuse it as corroboration.

The original exit remains immutable. A companion can refine a mechanical exit
without changing it. Local `intake-handoffs-v1.jsonl` is append-only under the
existing audit lock: exact retries are idempotent, conflicting revisions refuse,
and a new judgment increments the per-event revision. If the exit write succeeds
and the companion fails, a retry still attempts the companion. A retry that
names a different exit for the same event keeps the first exit and reports that
the new exit was not recorded. Never invent a new source event to bypass a
conflict.

The committed register and placeholders may point at a sibling JSON file using
`handoff: <filename.json>` and the independently retained `handoff-opened:` event
time. The register also retains its existing triage stamp. `statusgen
--intake-handoffs` exports records with assessment; `--intake-exits` is unchanged.
Sibling reads use a confined filesystem root, so traversal and escaping symlinks
cannot read arbitrary local files. Publish the companion and source pointer in
the existing reviewed intake change; a local log alone is not fleet assignment.

## Observation retention and scheduling

The production scanloop composition retains minimal issue/event observations in
its monitor state directory before advancing a polling baseline. It also records
the resulting baseline and delta. This reuses the existing full population read,
without a second forge list or a new GitHub issue. A restart replays retained
observations, including issues subsequently absent from the open list. Initial
arming still seeds silently. The next pass presents the seed for judgment.

History retains every event; one latest event per issue enters the existing claim
and trust path. Replay goes to judgment, never another mechanical scan carrier.
Read/write errors are could-not-check, not an empty queue. A dry-run uses only the
existing throwaway state copy. Observation retention does not infer discharge
from labels, closure, a scan PR or a v1 exit. Current companion assessments are
shown beside the routing work; the owning records remain authoritative.

A retained issue leaves replay only on its latest event. A recorded companion
whose current assessment is `observed` or `external-wait` discharges it. An
owed, held or unreadable judgment does not. A newer source event on the same
issue is a new key, so the issue is presented again.

The trust gate records a terminal disposition on a retained event in two cases.
The first is `gone`: the forge gave a definite not-found (404 or 410) for an
issue in a repository that the same pass read cleanly. The second is
`quarantined`: the event was read cleanly and is untrusted and unblessed. The
retired event stays in the history and is not read again. A newer event is read
and gated again, so a later blessing or edit is not lost. The pass that first
reads a gone item still reports could-not-check. Transient failures, other
errors and reads in a degraded repository are never retired.

Replayed events share the poller's per-repository burst bound
(`INBOUND_MONITOR_BURST_CAP`, default 25). One pass presents at most that many
retained events per repository, oldest first. The pass names the rest as
deferred, and they stay retained for later passes. The poll's own delta is
already bounded by the poller and is not counted again.

Raw placeholders remain representations. Ordinary scheduling requires a recorded
repair disposition naming that same issue as the repair target, plus existing
trust, history, decision, addressee and claim checks. A different successor has its
own admission and claim; its source placeholder must not schedule a second repair. Scoping, qualification and external prerequisites stay out of ordinary
repair scheduling. Legacy records remain classification-needed without bulk edits. Closure or an
excluded label cannot archive an unclassified source. Classified archive moves
preserve the companion; reactivation removes the old pointer so a new source
event cannot reuse a prior judgment.

## Validation

`TestIntakeHandoffDisposition` exercises production composition, restart, current
trust, role/event binding, successor existence, partial writes and retries,
settled discharge, the replay bound and terminal retirement.
`TestScanHandoffAdmission` exercises scanner output, placeholder scheduling and
both committed-register exports. Executable negative controls live in
`statusgen/testdata/verification-handoff/03-mutations.json` and use the existing
bounded offline mutation runner.
