---
stream: intake-class-routing
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: []
board: generated
spec: docs/streams/intake-class-routing/README.md
---

# intake-class-routing: judged issues reach an owner, and classes leave as one brief

**Status:** routed — authored 2026-11-12 from a read-only review. Approval is the maintainer's own act: the merge of the pull request that lands this document stamps it (`spec/lifecycle-v1.md` §8.4). That same pull request lands briefs 01–05, whose `sources:` cite this path, so the document lands `routed`. No earlier approval is asserted.

**Routes-to:** docs/streams/intake-class-routing/

**Owner:** the methodology track (intake procedure and the intake lane's tools).

**Pinned reads:** `origin/main` @ `0665bfb77` (2026-11-12). Tracker counts were read the same day.

Intake judges every issue and the result stops at a comment and three labels. Since
2026-10-26 a placeholder is schedulable only after an accountable `repair` judgment is
recorded and published, and none has been, so issue work no longer reaches workers. This
stream makes intake record and publish that judgment, and work the retained backlog as
groups: matched to existing work, authored as one class brief, or released as direct repairs.

This README is the stream's scoping document. Scope, evidence, alternatives, the dependency
review and the decisions the landing merge rules follow the board sections, from §1 on.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Record and publish a handoff for every judged issue](brief-01-record-and-publish-a-handoff-for-every-judged-issue.md) | 0 | M | todo | — | — |
| 02 | [The scan publishes recorded handoffs and land stops a judged exit without one](brief-02-the-scan-publishes-recorded-handoffs-and-land-stops-a-judged-exit-without-one.md) | 0 | M | todo | — | — |
| 03 | [Give intake replay its own bound](brief-03-give-intake-replay-its-own-bound.md) | 0 | S | todo | — | — |
| 04 | [Work the owed batch by class](brief-04-work-the-owed-batch-by-class.md) | 0 | M | todo | — | — |
| 05 | [New cells pin the intake role to a strong tier](brief-05-new-cells-pin-the-intake-role-to-a-strong-tier.md) | 0 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

```
01 ──► m-01 (one judged issue reaches the dispatch board)
04 ──► m-02 (one class brief closes its members)
02, 03 harden and widen both; neither blocks them
05 makes a new cell able to do the judgment work at all; independent of the rest
```

**The head, checked.** The stated plan rests on one edge: a recorded and published direct
repair makes a placeholder schedulable on the shipped build. I verified the tree half of that
by reading `statusgen/nextup.go:594`: "`if b.Blocked != "" ||
!issueflow.RepairDispositionSettled(b.IntakeAssessment)`"; it showed that a placeholder is
held out of Next-up until its assessment is settled, and `statusgen/placeholder.go:364` shows
the assessment is read from the placeholder's `handoff:` key and the sibling JSON file. The
live half is unproven: §1.1 below says "Committed handoff companions, and `handoff:`
pointers | 0 and 0", which does not establish that the writer and the board agree on a real
installation, and issue 6199 reports the disposition store unreadable there. One live
published judgment is therefore the real head: brief 01 settles it in its host-witnessed
Verify row 5, and it is milestone m-01's acceptance.

**Smallest unblocking move:** record and hand-publish one direct repair on one installation
with the shipped tools, then read the dispatch queue uncapped. It needs no code, and it can be done
before brief 01 merges.

**A tempting wrong first step:** building the class pass (04) before any judgment has been
published. A class brief whose members' judgments never reach the board changes nothing a
worker can see.

## Dependency waves

```
Wave 0: [01, 02, 03, 04, 05]
```

Critical path: `01`, then `04`. No brief in this stream depends on another brief, here or in
any other stream; §6 below records each candidate that was examined and why it is not an
edge. Briefs 01 and 04 both edit `plugins/assay/skills/intake-desk/SKILL.md`, in different
sections; whichever merges second merges main first.

## Delivery milestones

Proposed in §7 below as authored prose. No operational milestone record is declared here.

## Before starting

- Installed Assay tools at v1.0.62 or later (the first release carrying the handoff reader,
  its writer and the admission rule).
- The decisions in §9 below are ruled by the merge that lands this stream.

## Shared conventions

- Every example record a brief adds to a reference file is decoded and assessed by a test. A
  procedure example that no test reads is not accepted.
- No brief here changes `spec/intake-handoff-v1.md` or `spec/intake-exit-v1.md`.
- Each brief's pull request carries `changelog/<slug>.md`.

---

## 1. The problem

Open issues on this repository keep growing, and almost none of them reach a worker. The
cause is not missing design. Since 2026-10-26 an issue placeholder is schedulable only after
intake records an accountable `repair` judgment for it (`spec/intake-handoff-v1.md`), and no
intake pass observed for this review has recorded one. Intake classifies every issue, comments
on it and labels it, and the result stops there. The class procedure that would turn repeated
symptoms into one design brief (`build-less-brittle/04`) is likewise written down and not run.

### 1.1 What the evidence shows

Tracker, `medici-finance/assay`, read 2026-11-12:

| Measure | Value |
|---|---|
| Open issues | 1,007 (370 older than 30 days) |
| Opened 2026-11-05..11 | 431, of which 241 are dispatcher attestation records that open and close on their own |
| Opened, attestation records excluded | 187 (175 the week before) |
| Closed, attestation records excluded | 61 (43 the week before) |
| Closed with a linked pull request | 27 (16 the week before) |
| Pull requests merged the same week | 261 |
| Open issues carrying an impact label | 154 |
| `error-class` issues, all time | 11, none opened after 2026-10-24; 2 are `design-owed` |

One adopting house's stream root and intake window, read the same day. This is one
installation, not a fleet census:

| Measure | Value |
|---|---|
| Issue placeholders at `todo` | about 2,700 |
| Committed handoff companions, and `handoff:` pointers | 0 and 0 |
| Placeholder rows in the dispatch queue | 0. Read uncapped (`--next-up --span 100000`) at each of the installation's four roots on their main: 40, 53, 0 and 1 eligible rows, none in the `issue-loop` stream. The root that holds the placeholders reported no row held by a stream cap |
| Judgment exits in the local `intake-exit-v1` log | 563 |
| Local `intake-handoffs-v1.jsonl` | absent |
| Owed judgments presented by the last intake pass | 147, at the replay bound in 5 of 7 repositories |
| Symptom placeholders parked on a class | 0 |

Source, at the pinned revision:

| Fact | Where |
|---|---|
| A placeholder is eligible only when `RepairDispositionSettled` holds | `statusgen/nextup.go:594` |
| That means: assessed `observed`, route `repair`, successor is the source issue | `deskcore/issueflow/intake.go:146` |
| The gate merged 2026-10-26 (#4413) and first shipped in v1.0.62 | `git log -S`, `git tag --contains` |
| The writer exists: `scanloop land --handoff FILE --handoff-reader intake-handoff-v1` | `tools/desk/cmd/scanloop/landverb.go:66` |
| The writer appends to a local log only; nothing writes the companion file or the placeholder's `handoff:` and `handoff-opened:` keys | `tools/desk/cmd/scanloop/handoffrecord.go`, `statusgen/placeholder.go:364` |
| The board reads only the committed companion beside the placeholder | `statusgen/intakehandoffs.go:17` |
| Replay presents at most 25 retained events per repository per pass, sharing the poller's burst knob | `tools/desk/cmd/scanloop/observations.go:65` |

### 1.2 What this does and does not establish

Observed: the admission gate is live; the judgment that satisfies it is not being recorded;
publication of a recorded judgment has no tool; the class procedure leaves no record. The
intake session asked for this review stated that the handoff step "is simply not in my loop"
and that it had not tried the flag, and that it was running below the strong tier its skill
reserves judgment for. That is one session's report, consistent with the committed state.

Not established: that the handoff writer works end to end on a live installation. Nobody has
run it there, and [issue 6199](https://github.com/medici-finance/assay/issues/6199) reports
the disposition store unreadable under a held audit lock. Whether recording handoffs raises the
close rate is a hypothesis this stream tests; the counts above are a baseline, not a proof of
cause. No throughput target is promised.

## 2. Outcome and scope

Whose problem: the driver, who reads a tracker that only grows, and every reporter whose issue
is judged and then goes nowhere.

**Outcome.** Every issue intake judges leaves with a published, accountable successor, and
issues that share a mechanism leave as one class brief a worker can pick up.

In scope:

1. Intake records and publishes an `intake-handoff-v1` judgment for each issue it judges.
2. The tool publishes the companion and stops a judged exit that has none.
3. Intake works the retained backlog as groups, not single items: it matches a group to work
   that already exists, or authors one class brief for it, or releases the members as direct
   repairs.
4. The replay bound can be raised for a drain without raising the poller's burst bound.
5. A new installation starts its intake role on a strong tier, in both the seeded provider
   defaults and the scaffolded cell file, so the judgment work above can run from the first
   boot.

Out of scope, with the reason:

- **The 241 attestation records a week.** They are dispatcher run records, already excluded
  from intake and work scans. Moving them off the tracker is a separate change to the
  dispatcher.
- **Reducing what the other desks file.** About nine in ten open issues were filed by a desk
  role. Having those filings attach to an open class at source is worth doing and is not
  needed for this outcome. `build-less-brittle/05` owns the worker's half.
- **A scoring term that lifts class briefs on the dispatch board.** Deferred, see D5.
- **The verifier-blocker path** (verifier to intake to repair to independent recheck). It stays
  with `verification-handoff/02`, `/04`, `/05` and `/06`.
- **Any change to `intake-handoff-v1` or `intake-exit-v1`.** Both are consumed as shipped.

Terms. A **judged issue** is one intake has routed by judgment, not one the scanner emitted
mechanically. A **direct repair** is a `repair` judgment whose successor is the source issue
itself. A **class** is a mechanism shared by more than one symptom, recorded on an
`error-class` issue. The **owed batch** is what one intake pass presents for judgment.

## 3. Existing obligations

| Meaning | Owner | This stream |
|---|---|---|
| Handoff record, its assessment and the admission rule | `spec/intake-handoff-v1.md`, `deskcore/issueflow` | consumes; changes nothing |
| The five intake exits | `spec/intake-exit-v1.md` | consumes; changes nothing |
| Class record, instance block, counting and the `design-owed` trigger | `build-less-brittle/04` and the intake skill's class decision | reuses unchanged (D3) |
| Brittle investigation and refactor oracle | `build-less-brittle/09`, `/12` | a class brief that is a redesign follows them |
| Design-fit on every new brief | `build-less-brittle/02`, `spec/brief-v1.md` §4.1 | class briefs carry it like any brief |
| Duplicate handling: evidence merged first, a strong worker marks, an independent pass closes | the intake skill, step 5 | unchanged; a `duplicate` judgment records routing, it closes nothing |
| Trust gate and quarantine | the intake skill and `scanloop` | unchanged; quarantined text is never grouped or routed |
| Judgment work runs on a strong tier | the intake skill's model requirement | scaffolded for new cells (brief 05); an operator act for existing ones, §8 |

The handoff spec says deployment "remains a separate act" and that a companion writer must not
be required "until the participating readers support this version". The reader shipped in
v1.0.62 and already requires the record for scheduling, so the condition is met. D1 asks the
maintainer to confirm that reading.

## 4. Required behaviour

**4.1 Every judged issue gets a handoff.** For each issue in the owed batch intake records one
of the five dispositions the handoff spec defines. Two cases need no companion and are
unchanged: an accountable `rejected-watching` exit, and an item parked on a `needs-decision`
issue. Everything else is one of:

| Judgment | Disposition, route | Successor |
|---|---|---|
| A worker can take the issue as written | `repair`, `repair` | the issue itself |
| Existing work already covers it | `duplicate`, `duplicate` | that brief or issue |
| It belongs to a class owed a design | `duplicate`, `duplicate` | the class brief |
| It needs scoping first | `needs-scoping`, `authoring`, `author_tier: strong` | a brief that declares strong execution |
| Someone outside the pipeline must act first | `external-prerequisite`, `external` | none; owner, action and wake condition |
| The source is already delivered | `repair`, `qualification`, `delivered: true` | the qualifying work |

**4.2 A recorded judgment is published.** The companion file and the placeholder's two pointer
keys ride the same reviewed intake change as the placeholder. A judgment that exists only in a
local log assigns nothing; the handoff spec already says so.

**4.3 The owed batch is grouped before it is judged.** Intake groups the batch by mechanism,
using the module, the labels and the open `error-class` issues. For each group, in this order:

1. **Match existing work.** Look for an open class issue, an existing brief or stream, and an
   approved spec that already covers the mechanism. If one does, the members are `duplicate`
   to it, and the owning brief's `issues:` gains their numbers through its own author.
2. **Apply the class trigger.** With no owner, count the group as the class procedure already
   counts: confirmed defects and false positives, one per incident group. At the existing
   threshold the class is `design-owed` now, because the backlog already holds the instances.
3. **Author the class brief** when the trigger is met and the session is strong tier. The
   brief names the mechanism, the owning module, every member in `issues:`, what it retires,
   and the planned or in-flight work it must fit. Members are `duplicate` to it.
4. **Release the rest.** A group below the trigger gets a class issue with its instance blocks
   and each member gets its own direct `repair` judgment.

A single unrelated item skips to its own judgment. Grouping never merges evidence, never marks
a duplicate on the tracker and never closes anything.

**4.4 The class brief generalizes.** It is authored with `author-brief`, at strong tier, and it
must state: the mechanism in one line; every site the mechanism can recur at, not only the
reported ones; the existing or planned work it was matched against and the result; what it
removes; and a Verify row that fails on a planted second instance. These are existing
`author-brief` rules (9d, 14) and `design-fit`; this stream adds no new brief schema.

**4.5 Failure and partial success.** A refused or unreadable handoff write leaves the item
owed and is reported as could-not-check, never as an empty queue. A judgment recorded but not
published leaves the placeholder `classification-needed`; the next pass shows it still owed. A
class brief authored without its members' judgments is incomplete, and the members stay owed.
An intake session below strong tier records `repair`, `duplicate` and external judgments only
and queues the rest; the writer already refuses a `needs-scoping` judgment from such a session.

**4.6 Authority.** Nothing here grants a claim, a merge, an issue close or a dispatch. Intake
still never fans out workers. A class brief is a draft pull request like any brief.

## 5. Design and alternatives

The required outcome is "judged issues reach an owner". The mechanism proposed is the shipped
handoff record, used as designed. Alternatives considered:

| Alternative | Why not |
|---|---|
| Remove the admission gate so placeholders dispatch again | It restores the state the gate was ruled to end ([issue 4307](https://github.com/medici-finance/assay/issues/4307)): routing comments with no accepted successor. |
| Bulk-mark the legacy backlog as direct repairs | The handoff spec forbids bulk edits of legacy records, and a third of the backlog is over 30 days old and unjudged; marking it ready asserts a judgment nobody made. |
| A new sweep verb or a new desk role | The retained-observation replay already presents the backlog oldest first. The gap is what is done with the batch, which is procedure, plus one missing publish step. |
| More text in the intake skill | The step is already in the skill and was not run. Brief 01 shortens the skill text and moves the procedure to a reference with executable examples; brief 02 makes the tool carry the step. |
| Lower the class trigger for the backlog | Not needed: a backlog group of three counted members meets the existing trigger on the first pass (D3). |

What this retires: hand publication of companions (after brief 02), and the prose-only
"Accountable handoff" passage in the intake skill (brief 01). What it adds and why it is
justified: one publish behaviour and one refusal on an existing verb, one knob, and two
reference files. No new verb, record schema, label or gate.

## 6. Dependencies examined

The request was that this stream should not wait on other briefs. Each candidate was read:

| Candidate | State | Part this stream needs | Disposition |
|---|---|---|---|
| `verification-handoff/03` | implemented; merged 2026-10-26; in v1.0.62 and later | the record, its writer, the admission rule | Consumed as a shipped capability at v1.0.62 or later. No brief edge: its verification flip is independent of this work. |
| `verification-handoff/04` | todo, wave 6 | none | It covers successors addressed to a role and the fanout board. A direct repair is already admitted by `nextup.go`; a class brief is an ordinary brief. No edge. |
| `verification-handoff/06` | todo, wave 8, behind four other briefs | only a proof that the issue-lane path works end to end | Split. That one proof becomes this stream's first milestone acceptance (D1). The verifier-blocker replay and the remaining guidance stay in 06, which this stream does not edit. |
| `verification-handoff/02`, `/05` | todo | none | No edge. |
| `build-less-brittle/04`, `/09` | verified | class record, trigger, investigation template | Consumed as delivered. |
| `build-less-brittle/12`, `/13` | implemented, todo | none, unless a class brief is a brittle redesign | Such a brief follows those rules when it is authored. No edge from this stream. |
| Context pickup receipt (`statusgen context`) | shipped, version-gated | an optional aid when matching a group to existing work | The manual bootstrap in `plugins/assay/references/implementation-pickup.md` is the fallback. No edge. |

Result: all five briefs are wave 0 with empty `depends:`. Brief 05 touches only the cell
scaffold and has no edge to the other four. Briefs 01 and 04 each edit the
intake skill, in different sections, and each keeps its procedure in its own reference file so
that neither needs the other merged first.

## 7. Proposed delivery milestones

Proposals only. Reader support for operational milestone records was not established in this
review, so these stay in authored prose (`plugins/assay/references/milestone-authoring.md`).

- **`assay:assay:intake-class-routing:m-01` — judged issues reach the dispatch board.**
  Acceptance: on one installation, a committed companion makes its placeholder appear in the
  dispatch queue read uncapped; the pass that recorded it no longer shows that issue as owed; and a companion
  with a wrong event time leaves the placeholder held. Contributors: 01 alone (hand
  publication); 02 removes the hand step. This is the issue-lane proof split from
  `verification-handoff/06`.
- **`assay:assay:intake-class-routing:m-02` — the backlog leaves by class.** Acceptance: one
  class brief merged whose pull request closes at least three member issues; the per-repository
  deferred count from the intake pass falls over two consecutive weeks on that installation;
  the weekly count of issues closed with a linked pull request is reported beside the §1.1
  baseline. Contributors: 03, 04. Evidence owner: the intake desk's pass reports and a tracker
  read by someone other than the intake session.

No milestone promises a close rate. The measures are reported, not targeted.

## 8. Operator acts and external prerequisites

Not briefs; each is an act only an operator or maintainer can perform.

- **Move the intake role to a strong tier on existing installations.** Where a
  provider-defaults file or model policy exists it decides the model and a per-role pin is
  ignored, so the tier is changed there, and that cell's running windows are restarted. One
  cell can be moved without touching the shared file: a `providers.json` in the cell's own
  directory holding only the intake tier is merged over it. Where neither exists, the per-role
  pin applies. The skill already requires a strong tier for judgment
  work; the session observed was not on one. Brief 05 changes the seeded and scaffolded
  defaults for new installations only; it rewrites no existing file.
- **Clear what blocks the intake carrier** on the installation being drained. Two open bugs sit
  on that path here: [issue 6199](https://github.com/medici-finance/assay/issues/6199)
  (disposition store unreadable under a held lock) and
  [issue 6212](https://github.com/medici-finance/assay/issues/6212) (intermittent unreadable
  carrier). Both are ordinary issue-lane repairs and are themselves waiting on a judgment.
- **Raise the replay bound for the drain** once brief 03 ships. Until then the shared burst
  knob is the only lever, and raising it also raises the poller's bound.

## 9. Decisions for the approver

Merging the landing pull request rules each of these on its recommended default. A different
ruling is a comment on that pull request before merge.

| # | Decision | Options | Recommended |
|---|---|---|---|
| D1 | Does intake recording issue-lane handoffs count as the "live adoption" that `verification-handoff/06` must qualify first? | (a) No: the reader gate is already live, and milestone m-01 is the issue-lane qualification. (b) Yes: this stream waits behind 06 and four briefs before it. | (a). Under (b) issue work stays undispatchable for the length of that chain. |
| D2 | When `scanloop land` records a judged `bug` or `placeholder` exit for an issue with no handoff, what happens? | (a) Refuse, naming the flag. (b) Record it and print a notice counted in the pass summary. | (a). The prose-only form of this rule is what failed. `needs-decision`, `finding` and `rejected-watching` exits are exempt under both. |
| D3 | Class trigger for backlog groups | (a) The existing trigger, unchanged. (b) A lower one for the backlog. | (a). No amendment to `build-less-brittle` is needed. |
| D4 | Who authors the class brief? | (a) The intake session, when strong tier, in the same reviewed change as the members' judgments. (b) Always a separate authoring task. | (a), with (b) as the path for a session below strong tier. |
| D5 | A dispatch-score term for class briefs by member count | (a) Defer. (b) Add now. | (a). Revisit if a class brief sits unclaimed for 7 days while worker slots are idle. |

One decision is not ruled here. Brief 02 is `gate: human` because the scan would copy a
judgment's free-text fields into a committed file. Where the scrub boundary sits for that
copy is asked in brief 02's own decision issue when the brief is first picked up.

## 10. Proof

What can be checked now, from the tree: the admission rule, the writer and its refusals, the
replay bound (§1.1 source table). What is checked after implementation, and by whom:

| Condition | Check | Could falsify |
|---|---|---|
| The procedure's example records are valid | a test decodes every JSON example in the reference file and asserts its assessment (brief 01, 04) | a wrong-but-plausible example in the procedure |
| A published direct repair is admitted and a stale one is not | scanner-to-board test over a fixture root, both directions (brief 02) | publication that the board cannot read |
| A judged exit with no handoff is stopped | negative-path test on `land` (brief 02) | the enforcement silently not firing |
| The replay bound moves alone | test sets the new knob and asserts the poller's burst bound is unchanged (brief 03) | the two bounds still coupled |
| Grouping yields the stated record set | fixture batch with one matched group, one triggered class, one below-trigger pair, one quarantined item (brief 04) | a member left owed, or quarantined text routed |
| A new installation's intake role resolves to a strong tier | seed test over every provider and scaffold test over each cell kind, each with a neighbouring role left unchanged (brief 05) | the seed left on mid and overriding the pin, or a scaffold site missed |
| The path works on a live installation | milestone m-01 acceptance, witnessed by a non-implementer | everything the fixtures do not model |

No check here needs a database or an external service. The live-installation check runs
against one adopting installation's own stream root and tracker, reached with that
installation's intake role credential; it is a witnessed check, never hermetic CI.

## 11. Next step

Dispatch briefs 01–05; all five can start at once. The head of the path is verified in the Critical path section above. Nothing in this document starts implementation by itself.
