<!-- GENERATED from plugins/assay/skills/trace-path-to-live/references/progress.md by deskinstall --harness cursor. Do not edit. -->
# Progress measures

These views sit on the page defined in [Report presentation](report.md). They separate
recorded progress, evidence that is ahead of the record, and activity. None of them changes a
recorded status, a chart line or a path weight. Collect their inputs during SKILL.md step 4,
from the same captured revisions as the rest of the page.

## Stall statement — above the chart, only when it applies

Count **recorded stage changes**: forward changes of recorded status among the scoped briefs
across the seven daily readings, from the same history reader as the chart. Count **merged
changes**: pull or merge requests merged into the scoped roots' default branches inside the
same seven days. Take the count from the forge's merged list. Where that cannot be read, count
first-parent commits and leave out every generated commit: status regenerations, reconcile
and rollup commits, and Evidence or outcome-record landings. A generated commit is activity
nobody merged. Say on the page which method was used and how many commits were left out. When
recorded stage changes are two or fewer and merged changes are twenty or more,
open the page with a short stall statement above the monthly chart. The chart stays the first
chart. State both thresholds on the page, and say so if you change them.

The statement gives both counts and their ratio as **merged changes per recorded stage
change**. With no recorded stage change, write **No recorded stage change** and do not divide.
It then names up to three standing obstacles (see **Shared obstacles**) with the briefs each
holds and its age, or says **No shared obstacle identified** when the grouping found none.
Merged changes, Evidence publications and repeated runs are activity. Never draw them on the
progress chart, add them to a progress count or describe them as movement toward a target.
If either count is unknown, write **Stall not established** in the coverage block and show no
statement. A quiet week with few merged changes is not a stall under this rule; the movement
boards already show it.

## Banked, not recorded

Show this view directly after the six-line chart. It counts scoped briefs whose evidence is ahead of
their recorded status. It never changes a recorded status, a chart line, a remaining count or
a path weight. Each brief appears in at most one group, chosen by its recorded stage.

| Group | Recorded stage | Evidence required | Source |
|-------|----------------|-------------------|--------|
| **Delivered; board row held** | `todo` or `in-progress` | the board reconcile lists the row as held and names the merged delivering change | the body of the latest reconcile commit on the default branch, or on the reconcile job's own open change when it passes the check below |
| **Passed; record or sign-off held** | `implemented` | the latest retained non-implementer outcome record passed every Verify row, and its recorded outcome is not `verified` | the brief's newest record in the `docs/streams/verify-outcomes/` register, newest by the record's own `ts` and read through the register's reader, which also unions any legacy `verify-outcomes*.jsonl` lines |
| **Verified; completion refused** | `verified` | the completion reader lists the brief as refused or could-not-check | `statusgen --root <dir> --auto-flip-model --dry-run` in a scratch clone pinned to the captured revision, or the output of the latest default-branch run of the same reader |

Rules for the three groups:

- Use the newest record or run only. A later failing or blocked-with-failing-rows result
  removes a brief from the second group; an older pass does not count.
- A reconcile commit on an open change is an observation at that change's head, not merged
  state. Read it only when the change's head branch is the job's fixed branch in the same
  repository, never a fork, and its head commit is authored by the job's own identity. A
  change that fails either test is an unreadable source: the group is **Unknown**. Name the
  head revision and say which of the two the list came from. Only the default-branch commit
  feeds **Shared obstacles**.
- The job commits only when a board row changes, so the latest reconcile commit can be older
  than the captured revision. Give its date beside the captured revision; when it is older,
  the held list is a lower bound. With no such commit to read, `statusgen reconcile --root <dir> --repo <owner/name>` without `--apply`
  reports witnessed rows that differ from the board, but not why a write is held: label
  those **Delivered; row differs, hold reason not read**.
- The completion reader needs repository history, which an archive tree lacks. Use a scratch
  clone at the captured revision; never run it in a shared checkout and never without
  `--dry-run`. It considers `gate: model` briefs only. A `verified` `gate: human` brief that
  awaits sign-off never appears in the third group; it stays under **What is holding
  progress**. Say so beside the count.
- Group each list by the stated reason and keep the reason text and the cited issue. These
  reasons feed **Shared obstacles**.
- Link every count to its source revision, record or run. A source that could not be read
  makes its group **Unknown**, never zero. Do not add the three groups into one figure.
- A banked brief leaves the view only when its recorded status changes. Evidence that was
  superseded, or a hold that was lifted without a status change, is shown as such.

Show today's counts. Show an earlier reading only where the same source was read for that
reading; otherwise leave it out and say why.

## Work burned and scope added

The six-line chart follows today's scope, so newly added prerequisites can hide completed
work. This view separates the two. It uses the node weights from SKILL.md step 5 and labels
every figure a relative work estimate.

- **Baseline.** Take the earliest comparable saved capture for the same confirmed targets and
  reader coverage. Record its file, capture time, root revisions and brief membership. With no
  earlier capture, today's capture is the baseline: write **First measured capture** and show
  no burn.
- **Fixed-scope remaining.** Sum the weights over the baseline's brief set only, at the
  baseline and at each chart reading on or after it, using each brief's recorded status at
  that reading. **Burned** is the baseline sum minus today's sum over that same set. Give the
  matching brief counts by stage beside it.
- **Scope added.** List the briefs in today's scope that are absent from the baseline set,
  with the date each first appeared in scope and the weight each carries today. Say whether a
  brief was newly authored or was pulled in by a later dependency edge when the history shows
  it; otherwise **Not established**. List briefs that left scope separately.
- **Never net the two.** Show burned and added as two labelled values or two separate bars
  with their own scales explained. Do not publish their difference as progress, and do not
  fold added weight into the burn figure. Total remaining stays on the six-line chart.
- A brief whose status is unknown at a reading makes the fixed-scope sum unknown at that
  reading. If a brief's effort or gate changed since the baseline, use today's values at
  every reading and say so.

## Shared obstacles

Group the concrete obligations in **What is holding progress**, the outcome records' blocker
references, the reconcile hold reasons and the completion refusals by the issue or reference
each one cites. One cited reference is one obstacle, however many briefs and categories it
touches. For each obstacle show:

- the scoped briefs it holds, deduplicated, and how many of them lie on a critical path;
- its age: the earliest date a scoped brief's record cited it, with that record as the source;
- the owning role, read from the issue's routing label. An assignee is an account, not a
  role: name the account beside the role, and with an assignee but no routing label give the
  account and write **Role not recorded**. With neither, write **No owner recorded**;
- the count of Evidence changes citing it since that earliest date, merged and open counted
  separately. This is activity spent against the obstacle, not progress;
- the reference's current state, any recorded ruling on it, and whether a hold citing it has
  been released since. A ruling or a closed issue is not proof that the obstacle cleared: say
  **Effect not observed** until a held brief's recorded status changes.

An obstacle that holds three or more scoped briefs and is seven or more days old is a
**standing obstacle**. Render it in the graph as its own node kind, with a distinct shape and
text label and an edge to each brief it holds. It is not a brief: leave it out of total work,
remaining work, path weights and every chart. In **What to do next**, list standing obstacles
before critical-path brief moves, ordered by scoped briefs held on a critical path, then
briefs held, then age. The move's owning role is the obstacle's recorded role; with no role
recorded, the move is the desk's: assign one. Place each brief move that would meet a
standing obstacle under that obstacle, because repeating it adds activity and releases
nothing. Holds that cite no reference, or one that could not be read, stay ungrouped under
**Unknown**; do not merge them by similar wording.
