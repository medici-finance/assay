# Glossary

The terms that show up across the docs before any single one of them is defined.
Each entry is 1–3 sentences plus a pointer to the doc that actually specifies the term —
**this page is not the spec**; when the referenced doc and this page disagree, the
referenced doc governs. Alphabetized. Fictional examples in linked docs use `example-app` /
`human:alex`.

## bless

The single-identity authority (`ASSAY_BLESS_LOGIN`, numeric-ID pinned) that can move an
externally-authored item out of quarantine. Unset means nobody can bless and everything
external stays quarantined — closed, not open. Configured in
`roster-configuration.md` (internal, pending a public-safe rewrite).

## board

`STATUS.md` at the repo root — a **single-writer generated artifact**, regenerated from
the stream READMEs and registers by main's CI on every push that touches a source.
Branches never commit it; PR CI runs the generator in `--lint` mode instead. See
[`lifecycle.md`](./lifecycle.md).

## brief

The unit of work: a self-contained scope-and-DoD contract (schema `brief-v1`) one agent
can execute without reading the rest of the plan. Frontmatter, tables and Verify rows
carry the load-bearing facts; prose only motivates. See
[`brief-rules.md`](./brief-rules.md) and [`brief-template.md`](./brief-template.md).

## capability vocabulary

The harness-neutral set of dispatch verbs (`dispatch-worker`, `isolate-workspace`,
`message-agent`, …) that lets a brief describe *what* a harness must do without binding
to one harness's specific tool names. Owned by the `harness-portability` stream, not
yet landed publicly — a `docs/streams/harness-portability/README.md` reference until
that stream's public-safe rewrite ships.

## cell

One accountable unit — a lead plus its agent fleet — scoped as its own repo (a
"product cell") or its own directory tree, the two adoption shapes for growing the
methodology past a single repo. See [`adopting-assay.md`](./adopting-assay.md).

## claim

A brief already has an open branch against it on `origin`. Next-up drops claimed
briefs from its batch so two sessions don't pick the same work; when the claim read
itself fails (timeout, error), Next-up degrades to an unfiltered superset and says so.
See [`lifecycle.md`](./lifecycle.md).

## desk

The generic name for a role window in the dispatch pipeline — brief authoring, fan-out
dispatch, PR review, post-merge verification, and coordination. An adopting team runs
these as its own project-local skills; the public methodology bundle ships the two
portable skills (`adopt`, `author-brief`) only, not the loop-role skills themselves.
See [`adopting-assay.md`](./adopting-assay.md) (`install-desk-plugin`).

## done

The lifecycle state after `verified` that additionally carries a recorded review
verdict. A `gate: human` brief needs a review entry naming a human (`human:<name>`); a
model sign-off cannot close a risk-flagged brief. See [`lifecycle.md`](./lifecycle.md).

## drain

Working a queue (the post-merge "awaiting verification" queue, an open-PR queue)
continuously to empty rather than sampling it once. An unwatched, undrained queue is
how briefs rot at `implemented`. See [`lifecycle.md`](./lifecycle.md).

## evidence

The section of a brief filled in at implementation time: one row per Verify item with
command, exit code, output/hash, date and runner. A non-implementer re-fills it on
merged main to move a brief from `implemented` to `verified`. See
[`brief-template.md`](./brief-template.md) and [`lifecycle.md`](./lifecycle.md).

## fan-out

The dispatch role that turns a board's Next-up batch into parallel worker agents, each
implementing one brief in its own worktree and opening a draft PR. See
[`adopting-assay.md`](./adopting-assay.md) (`install-desk-plugin`).

## gate

Whether a brief closes on a model's say-so or needs a named human. Derived from risk,
not chosen: record all four risk answers (regulatory, customer, irreversible,
sensitive-data); any `yes` forces `gate: human`. See [`brief-rules.md`](./brief-rules.md).

## implemented

The lifecycle state an implementer reaches when finished and the Evidence section is
filled with its own run. Implementers stop here — verifying your own work is the
narrator grading their own exam. See [`lifecycle.md`](./lifecycle.md).

## in-progress

The lifecycle state once a session owns a brief and is actively implementing it. See
[`lifecycle.md`](./lifecycle.md).

## intake

The append-only register of raw ideas (`I-<slug>` entries) that are neither rejected
nor yet scoped into a brief. An idea becomes work only by flipping its disposition to
`scoped → <stream>`. See [`registers.md`](./registers.md).

## next-up

The generated batch of briefs to pick next, weighted by priority + staleness, capped
per stream, and excluding briefs with an unresolved finding or an open claim. When the
claim read is unavailable the batch is an unfiltered superset and says so. See
[`lifecycle.md`](./lifecycle.md).

## pin

A `.assay-versions` line naming the exact released tag (and sha256) of a tool a repo
depends on, so a reviewer can see a tool-source change in the diff instead of it
happening silently underneath a workflow. See `distribution.md` (internal, pending a
public-safe rewrite).

## positive control

Proof that a check actually fails when the thing it guards is broken — break the
guarded thing, run the check, confirm it goes red — required before trusting any clean
report from it. See
[`three-state-instrument-rule.md`](./three-state-instrument-rule.md).

## register

One of the append-only logs (FINDINGS, INTAKE; RETRO is never-implemented) that sit at
`docs/streams/` as the system's memory. Entries are per-file and slug-identified;
withdrawal is a tombstone (flip disposition, keep the file), never a deletion. See
[`registers.md`](./registers.md).

## review

The pre-merge quality pass on a working diff or open PR; the verdict is recorded in the
stream README table. Proves "well-built?" — a distinct question from the Verify
table's "works?", and neither substitutes for the other. See
[`lifecycle.md`](./lifecycle.md).

## roster

The adopter configuration that decides who is trusted, which repos they may write to,
and which paths force a security review — Actions variables or a config-home file,
never a file in the work tree a PR could edit to admit itself. Defined in
`roster-configuration.md` (internal, pending a public-safe rewrite).

## stream

A named body of related briefs with its own README (frontmatter, brief table, critical
path) under `docs/streams/<stream>/`. An idea becomes a stream by scoping doc plus
brief authoring. See [`registers.md`](./registers.md) and
[`brief-rules.md`](./brief-rules.md) (typed IDs, `stream/NN`).

## the-desk

The standing coordinator role: watches the board, dispatches and adversarially
verifies work, synthesizes reviews, and keeps the registers honest — as opposed to the
fan-out, review, or verify role windows, each of which runs its own loop over one slice
of the pipeline. See [`adopting-assay.md`](./adopting-assay.md) (`install-desk-plugin`).

## three-state instrument

Every desk instrument (program, script, query, check) must report in three states,
never two: `checked-clean`, `checked-failed`, or `could-not-check` — never rendering
"never ran" as either pass or fail. See
[`three-state-instrument-rule.md`](./three-state-instrument-rule.md).

## todo

The lifecycle state a brief starts in: authored, unclaimed, dependencies known. See
[`lifecycle.md`](./lifecycle.md).

## verified

The lifecycle state a **non-implementer** reaches by re-running the Verify table on
merged main and filling the Evidence section — independent re-execution, the check
that "works on my machine" survives contact with main. See
[`lifecycle.md`](./lifecycle.md).

## verify

The post-merge role that drains the "awaiting verification" queue: a non-implementer
runs each merged brief's Verify table, fills Evidence, and advances
`implemented → verified → done`. Merging is not completion. See
[`lifecycle.md`](./lifecycle.md) and [`adopting-assay.md`](./adopting-assay.md)
(`install-desk-plugin`).

## wave

A brief's position in the dependency graph: Wave 0 has no dependencies; Wave N depends
only on briefs in strictly earlier waves. Waves are a projection of the dependency
graph, not a hand-picked grouping — a same-wave dependency is a lint violation. See
[`brief-rules.md`](./brief-rules.md).
