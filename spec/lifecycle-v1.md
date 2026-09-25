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

A **risk-gated** brief (section 4.4) MUST NOT enter `in-progress` until it has passed
the **design-approval gate** — an approved design-decision record exists and the brief
cites it. This is a precondition on the `todo → in-progress` transition, not a new
ordered state: the five-state sequence is unchanged. See section 4.4.

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

**Date-bounded successor.** The "no execution witness" sentence
above describes every closure made **before statusgen v1.0.13**
<!-- TRACKING: v1.0.13 is the anticipated pin; confirm/correct this version string at cut-release time if the actual next tag differs. The gate itself is git-merge-base scoped and does not depend on this string. -->. From that pin,
`statusgen verifyrun` writes the witness (command, exit code, output hash, date,
runner) into the Evidence section, and a `verified`/`done` transition made on a branch
merging after the pin with no witness behind one or more Verify rows is a hard lint
PROBLEM, merge-base scoped (a closure already on `main` at the merge-base is
grandfathered to a NOTICE — the inherited corpus is not retroactively falsified). A
conforming implementation MUST NOT describe a POST-pin `verified`/`done` closure with
no witness as adequately attested.

**The evidence coverage condition (graph-execution/03).** `verified` additionally
requires that the brief's evidence **coverage** be `released`: the set of mandatory
claims — every Verify row, plus a bound workflow-pattern-v1 node's own
`evidence[].mandatory: true` entries when one applies — each resolve `pass` at the
item's revision (the merged SHA for a merged brief, the PR head for an open one). Any
mandatory claim that is `missing`, `error`, `could-not-check`, `wrong-revision`, or an
outright `fail` HOLDS coverage, and a conforming implementation MUST NOT promote
`verified` (or the `verified`→`done` flip) while coverage is not released — this is a
DEMOTION exactly like the stale-witness-version case above, read off a different
signal. A pattern node MAY declare an `observe` evidence entry — a signal watched over
a window after a change lands, filled from a named source; an unreadable source is
`could-not-check`, never `pass`. A conforming implementation MUST NOT treat a model's
own textual assessment of a row as an execution witness satisfying a mandatory claim,
and MUST invalidate a witness whose bound Verify-row text has since changed (the claim
resolves `error`, not `pass`) — a witness answers the question it was asked, not
whatever the row now asks.

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

### 4.4 Design-approval gate

Review (section 4.1) reads the finished diff; it is the LAST control before a change
lands, so a wrong *design* — the alternatives were never weighed, the consequences of
failure were never written down — is caught only after it has been built. The
design-approval gate adds one EARLIER control, on the `todo → in-progress` transition,
so a risk-gated brief records what was decided and why before implementation begins.

**Scope.** The gate binds a **risk-gated** brief only — one whose `gate` is `human`, or
any of whose four `risk` answers is `yes` (the same derivation section 4.3 uses). A
`gate: model` brief with all four risk answers `no` is NOT subject to it. The gate is
deliberately scoped: it is not a blanket new obligation on every brief.

**The design-decision record.** A risk-gated brief passes the gate by citing an approved
**design-decision record** — a typed register entry (`DR-<slug>`, schema `decision-v1`,
`registers-v1.md` §7) that records what was decided, the alternatives considered and why
each was ruled out, the consequences accepted, its ordered `consequence` severity axis,
and a dated `human:<name>` `decided-by` stamp. A brief cites it with the OPTIONAL
`design:` frontmatter key (`brief-v1.md` §3.2).

**Design-approval authority.** `decided-by` MUST be a `human:<name>` stamp — the same
human-gate authority a `gate: human` brief already carries, reusing the existing
decision-issue mechanism rather than opening a second human-gate channel. Approval is a
recorded human act; a model sign-off MUST NOT stand in for it. The record MAY carry the ruling
itself as a `ruling:` link to the human's comment on the decision issue; the corroboration
check resolves that link and verifies its author, and a `decided-by` placeholder with no
resolvable link is a problem (`registers-v1.md` §7.5).

**What the gate does and does not attest.** The gate proves an approved design-decision
record with a human approver EXISTS and dereferences. It does NOT mechanically prove the
approver is a different identity from the brief's author — that is the same
attribution-not-identity limit section 7.1.2 declares for verification, and a conforming
implementation MUST NOT claim author≠approver as a mechanical boundary.

**Grandfathering — a forward-only obligation.** Because this gate is a lifecycle change
that reaches every consumer on a pin bump and cannot be quietly reverted, a conforming
implementation MUST NOT retroactively fail a brief already in flight. The reference
implementation grandfathers by authoring date: the gate binds only a brief `authored:`
STRICTLY AFTER a named cutover constant, so every brief already in any corpus is exempt
and the obligation attaches only to risk-gated briefs authored after the rule lands. An
implementation MAY choose a different grandfathering mechanism, but it MUST provide one,
and it MUST document the boundary it chose.

**Three-state.** Where the design-decision register cannot be read, a conforming
implementation MUST report the gate as `could-not-check` (an unverified dereference),
never as a clean pass and never as a blanket failure of every citing brief.

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
claim the strong form ("measured ground truth", "tamper-proof", "cannot lie").

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

   A conforming implementation MAY additionally run a genuine identity check on top
   of the attribution check above: comparing the git identity that actually
   committed a brief's Evidence lines against a roster-bound verifier role, and
   rejecting a mismatch. Unlike clause 2's attribution check, this one does not read
   authored text — it reads commit metadata a single authoring session cannot also
   author as someone else, without forging a commit under a role it does not hold.
   Its scope is exactly two-fold and MUST be stated wherever it is claimed:
   (a) it binds Evidence commits made under the identity-check cutover the
   implementation records (an earlier landing predates the check and is
   grandfathered, never silently accepted as checked); and (b) it is evaluated
   per-transition, not per-brief — a brief already standing at `verified`/`done` at
   merge-base(HEAD, origin/main) is pre-existing and is reported as a NOTICE
   (visible, not blocking), while a brief a branch newly closes to `verified`/`done`
   is the transition the check gates, and a mismatch there MUST be a PROBLEM. A
   checkout that cannot resolve its own history (a shallow/grafted clone, or an
   unresolvable merge-base) MUST render as could-not-check, never as a pass and
   never as the failure this check exists to catch. Outside this check's scope —
   the Verified cell always, and any Evidence commit outside (a) or (b) — clause 2's
   attribution-on-text check is the only one in force, exactly as stated above.
3. MUST require dated, attributed Verified/Reviewed cells.
4. MUST derive `gate` from `risk` answers exclusively.
5. MUST require a human-named review for `gate: human` briefs at `done`.
6. MUST NOT allow a risk-gated brief (section 4.4) subject to the design-approval gate
   to sit at `in-progress` or later without citing an approved design-decision record,
   and MUST grandfather briefs outside the gate's scope so a pin bump reds nothing
   already in flight.

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
8. Flag any risk-gated brief in the design-approval gate's scope (section 4.4) that is
   at `in-progress` or later and either cites no design-decision record or cites one
   that does not dereference. It MUST report `could-not-check` where the register is
   unreadable, and MUST NOT flag a brief the gate grandfathers.
9. Flag any design-decision record (`registers-v1.md` §7) missing its ordered
   `consequence` axis, its `human:<name>` `decided-by` stamp, or its enumerated
   alternatives.

## 8. Spec and scoping-doc lifecycle

Sections 2 through 7 govern the brief. This section governs the artifact *upstream* of
the brief: the specification or scoping document from which briefs are authored. An idea
becomes work only by becoming a stream — a scoping document plus authored briefs — and
until this section, that edge lived only in prose and human memory. An approved document
carried no machine-readable state, so nothing could watch it, and a document approved as
the plan of record could sit indefinitely with no brief ever authored against it.

A conforming implementation MUST give spec-shaped and scoping documents a
machine-readable three-state header, defined below, so that the approved-but-unrouted
condition is detectable rather than remembered.

### 8.1 The `Status:` header grammar

A spec-shaped document opts in to this lifecycle by carrying, in its header block, a line
of the form `**Status:** <state>`, where `<state>` is exactly one of `draft`, `approved`,
or `routed`, OPTIONALLY followed
by ` — <free prose>` (a space, an em dash, a space, then any explanatory text). The state
token MUST be the first token after the label; explanatory prose MUST move behind the
` — ` delimiter, so the line stays machine-parseable while preserving its human meaning.

A `**Status:**` line whose first token is not one of the three states leaves the document
**unclassified** (legacy). A conforming detector MUST ignore an unclassified document and
MAY warn that it carries an unparseable state.

### 8.2 State meanings

- **`draft`** — not yet ruled. The document is a working proposal; it is the plan of
  record for nothing, and no downstream control watches it.
- **`approved`** — ruled and merged as the plan of record. Brief-authoring against this
  document is **owed**: the edge from approval to authored briefs is open.
- **`routed`** — at least one brief now exists whose `sources:` frontmatter dereferences
  this document's path. The authoring edge is closed.

### 8.3 `Routes-to:`

A document in state `approved` or `routed` MUST also carry, in its header block, a line of
the form `**Routes-to:** <destination>`, naming the stream directory the work routes into,
as a repo-relative path (for example, a
`docs/streams/<stream>/` directory). A document in state `draft` MUST NOT be required to
carry `Routes-to:`. A conforming linter MUST flag an `approved` or `routed` document that
lacks a `Routes-to:` line.

### 8.4 Flip rules

The two forward flips ride the pull request that makes each true, never a standalone edit:

- **`draft → approved`** rides the PR that lands the ruling. Approval is stamped at merge:
  the same merge that records the decision sets the state.
- **`approved → routed`** rides the PR that lands the citing briefs. The state follows the
  provenance — once a brief's `sources:` dereferences the document, the document is routed.

### 8.5 The citation rule

A specification is **cited** when at least one brief's `sources:` frontmatter contains the
specification's repo-relative path. This is a dereference against authored provenance, not
an assertion: a prose mention of the document's title does NOT count, and neither does a
citation that names the title without its path.

From the citation rule follows the owed condition a conforming detector MUST compute:

> An `approved` document that no brief cites has brief-authoring **owed**.

A conforming detector MUST treat only `approved` documents as candidates for the owed
condition — a `draft` watches nothing, and a `routed` document is by definition already
cited.

### 8.6 Safe default

When a legacy document's correct state cannot be determined, a conforming backfill MUST
default it to `draft`. A wrongly-`draft` document is silent status quo; a wrongly-`approved`
document makes owed-detection emit noise. The failure that costs least MUST be preferred.

## 9. Deploy — a transition beyond the brief lifecycle

Sections 2 through 8 govern a brief's own five-state sequence and the document upstream of
it, ending at `done`. `done` records that a change was reviewed and merged; it does NOT
record that the change reached a place where users are. A conforming implementation MUST
NOT conflate the two: "done" and "deployed" answer different questions, and a brief may be
`done` for infrastructure a deploy never touches at all.

### 9.1 Deploy is a record, not a sixth lifecycle state

A conforming implementation that models deployment MUST implement it as a transition on a
**separate typed record** (a DEPLOY entry, normatively specified in
[`../docs/deploy-model.md`](../docs/deploy-model.md) § "The deploy transition"), never as an
additional position in section 2's ordered sequence. The five-state sequence plus the
off-path `blocked` state (section 2.0) is unchanged by this section; deploy is incorporated
by reference, not folded in as a new brief status.

### 9.2 Precondition and reused markers

The deploy transition's precondition set, the per-environment authority, and the rollback
obligation are specified normatively in `docs/deploy-model.md`; this section does not
duplicate them. Two points are stated here because they bind the lifecycle directly:

- A DEPLOY record's transition MUST NOT be considered to have fired while the brief it
  carries is at `implemented` or earlier — the same implementer-self-report boundary
  section 2.3 already draws, extended to a deploy authority rather than restated as a new
  rule.
- A DEPLOY record waiting on an environment that does not yet exist reuses the existing
  `blocked-by: env` marker (brief-v1 frontmatter) rather than a second marker minted for
  deploys specifically — see `docs/deploy-model.md` § "Board handling".

### 9.3 Conformance

A conforming implementation that models deployment:

1. MUST NOT add deployment as a sixth position in the section 2 sequence.
2. MUST refuse a deploy transition whose carried brief is not `verified` or `done`.
3. MUST require a named human deploy authority per environment; a model-gated identity
   MUST NOT hold it.
4. MUST require every deploy record to state a rollback obligation — a stated reverse path,
   or an explicitly accepted absence naming an approver — never an omission.
5. MUST NOT describe this section as changing `docs/distribution.md`'s release-rollback
   statement: a release rollback and a deployment rollback are different operations
   (`docs/deploy-model.md` § "Rollback"), and this section reconciles the vocabulary rather
   than reversing either claim.
