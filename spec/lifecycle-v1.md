# Lifecycle v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen/v0.8.0`

## 1. Scope

This document specifies the brief lifecycle, the status-board generation model,
the Next-up work-queue semantics, and what the board does
and does not measure. A conforming implementation MUST satisfy every MUST in this
document.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## 2. Brief lifecycle states

A brief MUST progress through five states, in this order:

```
todo → in-progress → implemented → verified → done
```

### 2.0 `blocked` — the off-path state

`blocked` is a sixth valid status value and is NOT a position in the ordered sequence
above. A brief MAY enter `blocked` from any state before `done` when it cannot proceed
for a reason outside the implementer's control, and MUST re-enter the sequence at a
state no later than the one it left.

A conforming implementation MUST accept `blocked` as a valid status, MUST NOT offer a
`blocked` brief in the Next-up batch (section 5), and MUST NOT treat `blocked` as
implying progress. Board reporting that orders the lifecycle (bottleneck, roadmap, and
trend views) SHOULD render `blocked` as a terminal-for-now bucket alongside the five
ordered states rather than between any two of them.

A status value that is neither one of the five ordered states nor `blocked` MUST be
rejected.

### 2.1 `todo`

Authored, unclaimed, dependencies known. The brief has been written, its frontmatter
is complete, and its Verify table is defined.

### 2.2 `in-progress`

A session owns the brief and is implementing it. Only one session MUST own a given
brief at a time.

### 2.3 `implemented`

The implementer has finished and filled the Evidence section with their own run.
Implementers MUST stop at this state. An implementer MUST NOT set `verified` or
`done` — the implementer verifying their own work is the narrator grading their
own exam.

### 2.4 `verified`

A NON-implementer has re-run the Verify table on merged main and filled the
Evidence section (dated, with runner identity). Independent re-execution is the
check that "works on my machine" and "green in isolation" claims survive contact
with main. Merging to main does NOT set this state; verified is a distinct, owned
step.

**What `verified` does and does not attest.** `verified` records that an Evidence
section exists, is dated, and *attributes* an independent runner. It does NOT attest
that the Verify commands were executed, nor that they exited as reported: there is no
execution witness — no recorded command, exit code, or output hash — so a green Verify
table is **self-reported**. A conforming implementation MUST NOT describe `verified` as
proof of execution. See `spec/README.md` § "Known divergences from the reference
implementation".

### 2.5 `done`

The brief additionally carries the recorded review verdict. A `gate: human` brief
MUST have a review entry naming a human; a model sign-off MUST NOT close a
risk-flagged brief. A brief whose `risk.irreversible` is `yes` MUST carry a
human-named review entry before it may be marked `verified`, not only before `done`:
a model runner MAY execute the Verify table, but MUST NOT close a change that cannot
be walked back.

### 2.6 Cell format

Verified and Reviewed cells MUST be dated, attributed entries of the form
`YYYY-MM-DD <runner-identity>`. The date prefix is REQUIRED in every case.

A human reviewer is named by a `human:<name>` token **inside** the runner-identity
part — `2026-08-01 human:ada`, never a bare `human:ada`. `human:<name>` is therefore
an additional constraint on the identity token for the cases section 2.5 names, not
an alternative cell form: a cell carrying `human:<name>` without a date MUST be
rejected on the same rule as any other undated cell. A conforming implementation MUST
match the `human:` prefix on a whitespace-separated token, so that an identity merely
containing the substring (`superhuman:x`) does not satisfy it.

A bare checkmark MUST NOT be accepted as a valid cell entry.

## 3. STATUS.md — single-writer generated artifact

`STATUS.md` at the repo root is a generated artifact. It MUST NOT be hand-edited.

### 3.1 Single writer

STATUS.md has exactly one writer: main's CI. It regenerates and commits STATUS.md
on every push that touches a source document (stream READMEs, register files).

### 3.2 Branch prohibition

Branches MUST NOT commit STATUS.md. CI on branches MUST run in lint-only mode
(source checks, no STATUS.md read or write). A CI gate MUST fail any PR whose
diff touches STATUS.md.

**Limit.** "Fail" means the check reports failure; whether a failing check can *block a
merge* depends on the host's branch-protection facilities, which may be unavailable —
on the reference implementation's host they are plan-locked for private repositories,
making the merge gate advisory. A conforming implementation MUST NOT describe a CI
gate as preventing a merge unless it has verified that enforcement is actually
available and enabled.

### 3.3 Merge conflicts

On a local STATUS.md merge conflict, either side MAY be taken and the generator
re-run. STATUS.md MUST NOT be hand-merged.

## 4. Review gates

### 4.1 Verify vs Review

The Verify table proves function ("works?"). The review proves quality
("well-built?"). Neither substitutes for the other. A change MAY pass every check
and still be badly built, and vice versa.

### 4.2 Pre-PR and open-PR review

A working diff (pre-PR) MAY receive a review pass. Open PRs MUST receive a review
pass. The verdict MUST be recorded in the stream README table.

### 4.3 Gate derivation

`gate` MUST be derived from the four `risk` answers — not chosen independently:

- If ANY risk answer is `yes`: `gate` MUST be `human`.
- If all four risk answers are `no`: `gate` MAY be `model`.

A `gate: human` brief at `done` MUST carry a review entry naming a human.

## 5. Next-up semantics

### 5.1 Generation

A conforming board generator MUST compute a Next-up batch — the briefs to pick next
— so a session does not default to "the next brief in my stream." The generator MUST
apply all five of the following:

- **A score** over eligible briefs, per section 5.2.
- **A per-stream cap**: no single stream floods the batch. The cap is a tunable, not
  a fixed value of this specification; an implementation MUST expose the value it
  uses. The reference implementation's default is **4**, raised from 2 on
  2026-07-16 on the reasoning that agents rather than humans work the queue and that
  waves and dependencies already constrain eligibility. An implementation MAY also
  let a stream declare a tighter cap for itself; it MUST NOT let a stream declare one
  wider than the global cap.
- **Findings exclusion**: a brief with an unresolved finding against it MUST be held
  out until the finding resolves.
- **Claim exclusion**: a brief that is already claimed — an open branch, worktree, or
  pull request against it — MUST be dropped from the batch, so two sessions do not
  converge on the same pick. Because this filter depends on state the generator reads
  from outside its own documents, an implementation that cannot read that state MUST
  render the batch as **degraded/unfiltered** rather than silently emitting an
  unfiltered batch (section 6, and the three-state invariant in
  `brief-v1.md` §8).
- **A span-of-control cap**: the batch MUST be truncated to a maximum size, and when
  more briefs are eligible than are shown the generator MUST render an overflow
  indicator rather than silently truncating. The reference implementation caps at
  **20** and treats the same number as the overflow threshold; both are configurable.

Briefs in the `blocked` state (section 2.0) MUST NOT be offered.

### 5.2 Scoring

The score is a weighted sum. A conforming generator MUST document the terms it uses
and MUST state that they are tunable heuristic weights, not measurements. The
reference implementation's shipped formula has four terms:

```
score = priorityWeight(stream)
      + staleness_days × stalenessPerDay
      + valueWeight(brief.value)
      + unblocksWeight × blockedCount(brief)
```

where `staleness_days` is capped, `valueWeight` maps the optional `value:` field
(`low`/`med`/`high`) to a coarse three-way term, and `blockedCount` is the number of
not-`done` briefs this one transitively holds up.

**Known limitation.** The staleness term rewards neglect regardless of *why* a stream
aged, and any source-document touch resets a rival stream's staleness. The value term
is a coarse three-way knob asserted by the author, not a measurement. `blockedCount`
carries no effort term, so a brief blocking several trivial briefs can outrank one
blocking a single expensive brief. The board is a heuristic scheduler, not an oracle.
A conforming implementation MUST document this limitation.

## 6. What the board measures

THE BOARD IS DERIVED FROM AUTHORED ARTIFACTS WITH CONSISTENCY LINTING, NOT
MEASURED FROM GROUND TRUTH.

The generator parses status tables, frontmatter, Verified/Reviewed cells, and
Evidence blocks — all documents written by the same identities whose work it
reports. It checks the internal consistency of those documents (duplicate ids,
missing evidence, unresolved findings, malformed gates) and is backstopped by
adversarial spot-verification. It does NOT independently observe that the code
does what a brief claims.

### 6.1 Defensible statement

The defensible statement is:

> Status is derived from authored artifacts with consistency linting and
> independent re-verification.

It MUST NOT be:

> Status is measured, never self-reported.

The strong form is false: the sensors are writable by the same identities whose
work they report. Optional hardening (machine-attributable lifecycle transitions,
non-self-writable gate cells, a deny-hook layer that mechanically blocks an
implementer from writing its own gate cells) narrows the gap but does not close
it while a single identity can author both the work and its record.

A conforming implementation MUST claim the weaker, true statement. It MUST NOT
claim the strong form ("measured ground truth", "un-forgeable", "cannot lie").

## 7. Conformance

### 7.1 Lifecycle conformance

A conforming implementation:

1. MUST implement the five-state ordered lifecycle plus the off-path `blocked` state
   (section 2.0), and MUST hold an implementer at `implemented`.
2. MUST reject a `verified` or `done` brief whose Evidence attributes the
   implementer. **This is an attribution check on authored text, not an identity
   check**: it reads who the Evidence *says* ran the verification. Nothing in this
   specification prevents a single actor from authoring both the work and an Evidence
   row naming someone else — role separation is a convention, and a conforming
   implementation MUST NOT claim it as a boundary.
3. MUST require dated, attributed Verified/Reviewed cells.
4. MUST derive `gate` from `risk` answers exclusively.
5. MUST require a human-named review for `gate: human` briefs at `done`.

### 7.2 Board generator conformance

A conforming board generator:

1. MUST be a single writer (regenerates and commits on main, never on branches).
2. MUST NOT accept hand-edits to the generated board.
3. MUST compute Next-up with the rules in section 5, and MUST publish the values of
   its per-stream cap and span-of-control cap.
4. MUST exclude briefs with unresolved findings, claimed briefs, and `blocked` briefs
   from Next-up.
5. MUST render the batch as degraded when claim filtering could not run, and MUST
   render an overflow indicator when eligible briefs exceed the span cap.
6. MUST document the heuristic limitation (section 5.2).
7. MUST claim the statement in section 6.1, not the strong form.

### 7.3 Linter conformance

A conforming linter MUST:

1. Flag any brief whose status cell claims `verified` or `done` without an Evidence
   section attributing a non-implementer runner (subject to the attribution-not-identity
   limit in section 7.1.2).
2. Flag any `gate: human` brief at `done` without a human-named review entry.
3. Flag any bare-checkmark Verified or Reviewed cell, and any Verified or Reviewed
   cell lacking the `YYYY-MM-DD ` date prefix required by section 2.6.
4. Flag any `gate` value inconsistent with `risk` answers.
5. Flag any attempt to commit STATUS.md on a branch.
6. Flag any status value that is neither one of section 2's five ordered states nor
   `blocked`.
7. Flag any `risk.irreversible: yes` brief marked `verified` or `done` without a
   human-named review entry (section 2.5).
