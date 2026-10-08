# The Awaiting board: four owned queues and the desk's queue

`statusgen` renders the `## Awaiting verification / review` section of `STATUS.md`
from every brief at `implemented` or `verified`. Each brief goes in exactly one
bucket. Every bucket has one owner, and every row names the next act that owner
takes. Four of the buckets are **owned queues**: someone other than the
verify-desk moves them. The fifth, **Desk-actionable**, holds only the rows that
need judgement, so the desk's count is work the desk can actually do.

The bucketing is one pure function, `bucketAwaiting` in
`statusgen/awaiting_bucket.go`. The only disk reads are in `awaitInputs`
beside it, and they read data the renderer already has: the README row, the
brief file (its `risk` frontmatter, Verify table and Evidence section), and the
per-file verify-outcome records under `docs/streams/verify-outcomes/`.

## The headline

```
## Awaiting verification / review (<D> for the desk · <H> for the driver · <W> for workers · <E> for an operator · <R> runner-pending — of <M> total[; <C> could-not-check])
```

`M` counts every awaiting brief, including could-not-check rows and the paused
and parked streams. The could-not-check count appears only when it is non-zero.
The headline does not split `implemented` from `verified` any more: the owner
counts replace that split, and each row still shows its own status.

## The buckets

The first matching row wins. A **recorded blocker** is the brief's latest
verify-outcome record when its outcome is a fail (`verify-fail`, `fail`) or a
hold (`blocked`, `needs-context`, `needs_context`) and the Evidence's live
verdict has not since passed. A live PASS means the fix landed and the brief was
re-verified, so an older record decides nothing.

| Order | Condition | Bucket | Owner | Next act |
|---|---|---|---|---|
| 1 | `gate: human` or `irreversible: yes`, and the Evidence's live verdict is PASS (any emphasis); or `gate: human`, status `verified`, and no live FAIL | Awaiting human gate | driver | close the sign-off card; where no card is raised, `sign off the PASS verdict (no sign-off card is raised for this brief)` |
| 2a | a recorded blocker of kind `implementation` or `check-definition` whose `blocker_ref` names an issue | Awaiting implementer rework | worker | `fix, cite <ref>` |
| 2b | a recorded blocker of kind `human-action` | Awaiting human gate | driver | `human action, cite <ref>` |
| 3 | status `verified`, `gate: model`, Reviewed cell empty | Runner-pending | CI auto-flip | none; stuck after one main run → file |
| 4 | an unrun Verify row is `check:cluster`, a billed or live probe, or recorded `could-not-check` with an exact command; or a recorded blocker of kind `environment`; or the brief carries `blocked-by: env` | Environment-blocked | operator | the row's command, verbatim; for a record alone `environment blocker, cite <ref>` |
| 5 | `gate: model`, at least one unrun row, every unrun row runnable by the offline runner on Linux, no FAIL in the Evidence, no recorded blocker | Runner-pending | verify runner | none |
| 6 | a judgement row (see below) | Desk-actionable | verify-desk | dispatch one judge |
| 7 | anything else | Desk-actionable | verify-desk | triage, then re-bucket |

A blocker owner follows `tools/desk/internal/deskkit/verifywake.go`: an
`implementation` blocker is the worker's, `check-definition` the brief author's
(it renders in the worker queue, where the fix is cited), `human-action` the
human's, and `environment` the operator's. A hold with no kind, an unknown kind,
or an implementation kind with no issue to cite falls through to rows 6 and 7.

The sections render in a fixed order: Awaiting human gate, Awaiting implementer
rework, Environment-blocked, Runner-pending, Desk-actionable. All five headings
always render, with `_None._` under an empty one, so an empty queue reads
differently from a missing one.

### Who moves each queue

- **Awaiting human gate: the driver.** Either the model verification passed and
  the driver reads the sign-off card and closes it, or a verifier recorded that
  the blocker is a human action and cites it. The desk does not dispatch
  anything. A sign-off card is raised for a verified `gate: human` brief, or for
  an implemented one whose verdict line is the strict bold `**VERIFY: PASS**`
  with no held row; any other row here says so in its next act.
- **Awaiting implementer rework: a worker.** A verifier recorded a fail whose
  blocker is the implementation or the check itself, and named the issue that
  carries the fix. The worker fixes it and cites that issue. A brief whose
  Evidence was re-verified to PASS after the record is not here.
- **Environment-blocked: an operator.** A row needs something no offline
  verifier has: a cluster, live credentials, or a billed service, or a verifier
  recorded an environment blocker. The next act is the command to run, or the
  cited blocker.
- **Runner-pending: CI or the verify runner.** For a `verified` model-gated
  brief with an empty Reviewed cell, CI's auto-flip moves it on the next main
  run. If it is still stuck after one main run, file it. For an `implemented`
  model-gated brief whose remaining rows are all offline-runnable, the verify
  runner runs them.
- **Desk-actionable: the verify-desk.** A judgement row is a row only a judge can
  settle: a risk value named in the Evidence, a Verify section that gates
  presence, an unrun `gate:model` or `gate:human` row, or a recorded fail
  with no blocker class. The desk dispatches one judge. Rows that match nothing
  above are triaged and then re-bucketed.

## Could-not-check

A row whose inputs cannot be read gets **no bucket**. It renders under
`### Could-not-check` with the reason as its next act, owned by the verify-desk.
This happens when:

- the brief file exists but cannot be read;
- its Evidence section holds an unterminated HTML comment, which hides every row
  after it;
- the verify-outcome records cannot be read;
- a record for the brief is dated beyond the clock-skew tolerance, so it never
  wins "latest" and the brief's real state is unknown;
- the latest record's `outcome` is none of `verified`, `verify-fail`, `fail`,
  `blocked`, `needs-context` or `needs_context`; or
- the Evidence's last verdict is FAIL but no outcome record names the brief, so
  the blocker class is unrecorded.

Could-not-check rows count toward the verification-debt measure with the
desk-actionable rows, so an unreadable input cannot lower the count and switch
the drain-before-instrument gate off. When any are counted, the debt notice
says how many.

The segment renders only when it is non-empty.

## Paused and parked streams

Briefs in a `paused` or `parked` stream are not bucketed. They render after the
five buckets under `### Paused stream` and `### Parked stream`, each only when
non-empty, and they count only in the headline total. The Parked segment is
unchanged from earlier releases.

## Defaults the table leaves open

Each of these is a reversible choice, stated in `statusgen/awaiting_bucket.go`:

- "Blocker issue open" cannot be read offline. The board treats a `blocker_ref`
  that names an issue (`#N`, `owner/repo#N`, `alias#N`, or an `/issues/N` URL)
  as open until a newer outcome record supersedes it. A ref of `none …` does
  not match.
- Runner-pending (row 5) requires at least one unrun row (an all-settled brief
  leaves the runner nothing to run) and no recorded blocker or Evidence FAIL. A
  recorded fail or hold means the runner already ran, so running it again is not
  the next act.
- A recorded hold (`blocked`, `needs-context`) is routed exactly like a recorded
  fail, by its kind.
- Row 1 reads the Evidence's live verdict whatever its emphasis. The card is a
  separate, stricter test (see "Who moves each queue"), and the next act says
  which applies.
- The billed-probe and presence-gate tests are textual, because statusgen has no
  row marker for them yet.
- A `cmd` or `pwsh` shell row does not count as runnable on Linux.
- Outcome values `verify-fail` and `fail` both count as a fail, and `blocked`,
  `needs-context` and `needs_context` as a hold, the vocabulary of
  `IsFailedOrBlocked` in deskkit. Any other value is could-not-check.
- A `blocker_ref` that names an issue is read as open, because the board renders
  offline. A closed issue stays in the rework queue until a newer record
  supersedes it.
