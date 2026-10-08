---
brief: assay:assay:statusgen:16
title: 'Hold an issue out of dispatch when a non-human removed its excluded label: both scanners hold the placeholder, the issue board holds the un-briefed row'
why: >-
  An issue labelled `needs-decision` (or any other system-state label) is kept off the board:
  the scanner writes no dispatchable placeholder for it while the label is on. The moment the
  label comes off, the scanner creates or re-activates the placeholder as `todo`, and it never
  asks who took the label off. So an agent that removes the label, by mistake or because it
  misread a relayed answer as the ruling, turns an undecided question into dispatchable work
  with no human act anywhere in the chain. The driver ruled on 2026-10-08 that when an excluded
  label was removed by a non-human actor, the placeholder is held until a comment from the
  ratifying identity exists on the issue. This brief makes both scanners apply that, and makes
  the issue board stop offering such an issue as un-briefed work, since a hold in the scanner
  alone leaves that second route to dispatch open.
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This is a trust-boundary control: the rule decides which actor counts as a human and when a
  human's ratification of an open decision is taken as given, and the change edits code under
  the gate's own security-path trigger (the forge layer in `tools/desk/internal/deskkit/`). A
  human decides how far the hold reaches (which labels, which release signal) in the Human
  decision section, and at this gate reads the choices the author made where the ruling is
  silent (Context, "choices made here"), since each of those widens or narrows what an
  automated actor can turn into dispatchable work without a human act. The four risk answers
  stay `no`; the reasoning is under Context.
decision-trigger: creation
issues: []
outcome: none
exec-tier: strong
exec-tier-why: >-
  Crosses two Go modules, a forge interface with two backends, and three admission paths in
  two scan functions plus the issue board (question b), and it is trust plumbing where a wrong
  actor, ordering or read-failure rule still passes a happy-path test (question c).
schema: brief-v2
version: 1
authored: 2026-10-08 by the-desk session (on behalf of the driver)
sources:
  - "ruled by the driver, 2026-10-08: hold a placeholder whose excluded label was removed by a non-human actor until a comment from the ratifying identity exists on that issue. No record of this ruling is linked on this tracker yet; the decision issue filed from this brief's Human decision section becomes that record"
  - "statusgen/scanissues.go — `planScan`: the create path and the two reactivate paths, none of which reads who removed a label"
  - "statusgen/transcribescan.go — `planTranscribeScan`: the same three paths for the home repo, marked KEEP IN SYNC with `planScan`"
  - "statusgen/main.go — the two entry-point calls: `runScanIssues` gets `deskread`-backed readers, `runTranscribeScan` gets `gh`-backed ones"
  - "statusgen/trustgate.go — `isBlessAuthorityID`, `authorizedAuthorSet`: the identity checks this brief reuses"
  - "tools/desk/internal/deskkit — `LabelEvent`, `ListIssueLabelEvents`: the label-event read, which carries a login and no numeric id today, and on GitLab drops an event whose label was deleted"
  - "tools/desk/cmd/issueboard/board.go — `classifyIssue`: an open issue with no placeholder and no excluded label is `CREATE-PLACEHOLDER`, the row the worker pool's un-briefed sweep draws on"
  - "freshness-checked 2026-10-08 @ 82caf63b2 (origin/main), re-checked @ a0b70eb87 with none of the code, register or skill files this brief routes changed between the two — no Go file under `statusgen/` reads a label event; `deskread` has no label-event kind; the forge op register's numbered tables end at op 58 and a later row numbered 61 already records the existing issue label-event read, so 59 and 60 are unused; open PRs that touch files this brief routes: #2377 (`tools/desk/cmd/deskread/main.go`), #2382 (the deskkit forge files and the op register, where it adds ops 62 and 63), #2266 (`statusgen/main.go`), #1688 (both skills), #2218 and #2225 (the worker-desk skill); statusgen brief 17, on main at `todo`, routes the intake-desk skill too"
consumers:
  - "tools/desk/internal/deskkit/modelstamp.go: follow-up statusgen/16 (this brief; `LabelEvent` gains the actor's numeric id, additive)"
  - "tools/desk/internal/deskkit/forge.go: follow-up statusgen/16 (this brief; one new read op for an issue's label history)"
  - "tools/desk/internal/deskkit/forge_github.go: follow-up statusgen/16 (this brief; the timeline read fills the actor id, and the new op)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: follow-up statusgen/16 (this brief; both label-event reads fill the actor id, and the new op keeps the deleted-label events the existing reads drop)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up statusgen/16 (this brief; the op register records the new op and its consumers)"
  - "tools/desk/cmd/deskread/main.go: follow-up statusgen/16 (this brief; the new per-issue `label-events` kind)"
  - "tools/desk/cmd/issueboard/board.go: follow-up statusgen/16 (this brief; an un-briefed issue with excluded-label history is `AWAIT`, not `CREATE-PLACEHOLDER`)"
  - "statusgen/forgeread.go: follow-up statusgen/16 (this brief; the reader for that kind)"
  - "statusgen/scanissues.go: follow-up statusgen/16 (this brief; the hold on the create path and both reactivate paths)"
  - "statusgen/transcribescan.go: follow-up statusgen/16 (this brief; the same hold on its three paths)"
  - "statusgen/main.go: follow-up statusgen/16 (this brief; wires the default checker into both entry points)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up statusgen/16 (this brief; one sentence where the skill describes a scanner reactivate)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up statusgen/16 (this brief; one sentence where the skill describes the un-briefed sweep)"
---

# Brief 16 — hold an issue out of dispatch when a non-human removed its excluded label

## Context

files:
- `tools/desk/internal/deskkit/modelstamp.go` — `LabelEvent` gains one field, the actor's
  numeric id.
- `tools/desk/internal/deskkit/forge.go`, `tools/desk/internal/deskkit/forge_github.go`,
  `tools/desk/internal/deskkit/forge_gitlab.go` — the existing label-event reads fill the id;
  one new read op, `IssueLabelHistory` (planned), returns an issue's label history with a
  completeness flag.
- `docs/streams/forge-gitlab/inventory.md` — the op register gains that op.
- `tools/desk/cmd/deskread/main.go` (and its forge seam beside it) — a new per-issue kind,
  `label-events` (planned).
- `tools/desk/cmd/issueboard/board.go` — one new classifier input and the read behind it.
- `statusgen/forgeread.go` — the reader for the new kind.
- `statusgen/labelremovalhold.go` (planned) — the pure decision and the default checker.
- `statusgen/scanissues.go`, `statusgen/transcribescan.go`, `statusgen/main.go` — the hold and
  its wiring.
- the test files named in Task; `plugins/assay/skills/intake-desk/SKILL.md` and
  `plugins/assay/skills/worker-desk/SKILL.md` (one sentence each).
- this brief's own `consumers:` block (Task 14).
- `changelog/<branch-slug>.md` — the per-PR fragment this repo enforces.

single-point-of-failure: custody of the label. Today the only thing between an undecided issue
and dispatchable work is that nobody removes the excluded label early, and every identity with
label-write on the repo can. This brief adds one layer behind it, applied at both routes to
dispatch: before the scanner admits a placeholder, and before the issue board offers the issue
as un-briefed work, the forge's label-event history is read. That layer is exactly as strong
as what the read RETURNS. The actor who removed the label cannot edit the events, but the read
can stop at a page bound, the forge can stop naming a deleted label, and a rename or a
transfer may leave no trace (facts, below). Every shape the read can report must hold; the
shapes it cannot report are named under "what this cannot see". A third layer exists for only
part of the work: a placeholder whose derived gate is human still needs a human sign-off after
dispatch. A placeholder derived `gate: model` has no layer after this one.

facts (all read on main @ 82caf63b2, 2026-10-08):
- **The excluded set.** `scanExcludedLabelSet()` in `statusgen/scanissues.go` is the
  topology's system-state labels: `verify-gate`, `live-verify`, `needs-decision`,
  `review-request`, `needs-human`. `hasExcludedLabel` matches an issue's current labels
  against it. The issue board's own `hasExcludedLabel` reads the same set
  (`TestIssueboardExclusionMatchesScannerSource`).
- **Three admission paths, none of which reads an actor.** In `planScan`: (1) CREATE: an open
  issue with an excluded label is skipped and gets no placeholder, so when the label comes off
  the next scan creates one as `todo`; (2) ROOT REACTIVATE: a placeholder retired to
  `status: done` whose issue is open and no longer excluded gets a `closeOutPlan` with action
  `reactivate`; (3) ARCHIVE REACTIVATE: the same for an archived placeholder. Each decides from
  the issue's CURRENT label list only.
- **A second scanner has the same three paths.** `planTranscribeScan` in
  `statusgen/transcribescan.go` creates and reactivates for the home repo, and its close-out
  loop is marked KEEP IN SYNC with `planScan`'s. Its refusals are recorded as `clauseSkip`
  rows (a `Clause` and a `Reason`); `planScan`'s are NOTICE strings.
- **The two scanners reach the forge by different routes.** `--scan-issues` reads through the
  `deskread` binary: `statusgen/main.go` passes `runScanIssues` the three `defaultScan…`
  readers, and `TestScanIssuesMainWiresNativeReads` pins that argument list as an exact string.
  `--transcribe-scan` does not: the same file passes `runTranscribeScan` the `gh`-backed
  `ghIssueLister`, `issueCommentLister`, `ghAuthorResolver`, `ghIssueBlessChecker` and
  `ghCommentResolver`. Moving those reads onto `deskread` kinds is owned by forge-neutral
  brief 18, not by this brief. That lane is inert until its enactment gate is armed, and no
  workflow in this repo runs it; whether the runtime it will run in has `deskread` on its PATH
  was not established at authoring (could-not-check).
- **`deskreadReader` fails closed on every unread shape.** In `statusgen/forgeread.go`,
  `readItem` turns an issue listed in `partial`, a missing item, a malformed envelope and a
  failed run into an error, never a zero value. It has `IssueTrust` and `IssueComments`
  (login, id and creation time per comment). `deskread` has no label-event kind. The two
  modules share no code: statusgen declares its own copy of the envelope fields it reads.
- **The label-event read exists but is not enough as it stands.** `deskkit.LabelEvent` is
  `Name`, `AppliedBy` (a login), `Removed`, `CreatedAt`. `ListIssueLabelEvents` reads the
  GitHub issue timeline and GitLab's `resource_label_events`; both return an error past their
  page bound. Two gaps: the GitHub wire struct decodes `actor.login` only, though both forges
  return the actor's id on the same event; and both GitLab reads DROP an event whose label has
  since been deleted, which that code documents as coming back with an empty label name. Six
  desk commands and deskkit's own stamp reducer read `LabelEvent` today, and the drop is
  deliberate for them (an unnamed stamp attests to nothing), so the existing reads must keep
  it.
- **The forge interface is frozen except with a consumer.** Adding a method needs a consuming
  tool in the same change, a row in the op register (`docs/streams/forge-gitlab/inventory.md`;
  its numbered tables end at op 58 on main, and a separate later row numbered 61 records
  `ListIssueLabelEvents`, so the last table's end is not the next free number), an entry in
  the surface list
  `TestForgeSurfaceUnchangedByDeskread` checks, a class in the map
  `TestOutboundForgeWrapsEveryWriteMethod` checks, and golden fixtures on both backends. The
  rule binds methods, not fields. Forge-neutral brief 33 added ops 55 to 58 this way, with
  `deskread` kinds as their consumers.
- **The issue board is a second route to dispatch.** `classifyIssue` in
  `tools/desk/cmd/issueboard/board.go` is pure; an open issue with no placeholder, no excluded
  label, not decision-owed and not addressed is `CREATE-PLACEHOLDER`. The worker-desk skill
  (§"Un-briefed issues") has the pool sweep `issueboard issues` and take such an issue when
  four criteria hold; the only label test among them is that the issue is not CARRYING
  `question`, `needs-decision` or `help wanted` now. An issue the scanner holds has no
  placeholder, so with a scanner-only hold it is exactly the row that sweep offers. The board
  already reaches the forge through `forgeFor`, and its per-issue read `fetchIssueEvents` is
  the precedent for an incomplete read: that one row degrades and the rest of the board
  renders, while a read error fails the whole board with exit 6. The board's repo set equals
  the scanner's (`TestOwnedReposEqualsStatusgenScanRepos`).
- **The ratifying identity is already defined with an id.** It is the configured blessing
  authority (`ASSAY_BLESS_LOGIN`, a login and a numeric id). `isBlessAuthorityID(login, id)`
  is true only for that login with that id. `authorizedAuthorSet()` returns the rostered
  human authors as login to mandatory id, seeded with the blessing authority; with nothing
  rostered it is the blessing authority alone.
- **The `[bot]` suffix is not a stable signal.** One App is rendered `slug[bot]` by the REST
  API, bare `slug` by GraphQL (the trust reader re-suffixes it for that reason), and
  `app/slug` by some clients. GitLab's label event carries no account type at all, and a
  machine user is an ordinary user account on both forges.
- **What the forge does to label history on a delete, a rename or a transfer is only partly
  established.** Established, from the code above: GitLab keeps the event of a deleted label
  and stops naming it. Not established at authoring (it needs a write on a scratch issue, which
  an authoring session does not make): whether GitHub keeps, renames or removes the events of a
  deleted or renamed label, and what either forge carries across an issue transfer. A read of
  the 100 oldest and the 100 most recently updated issues on this tracker showed no event
  naming a label that no longer exists, which does not tell the cases apart. The rule below is
  written to give a stated outcome under each possible behaviour; Task 2 has the implementer
  establish the behaviour and record it.

the rule (what "held" means, exactly). It is evaluated for an OPEN issue whose current labels
carry no excluded label, immediately before a create plan or a reactivate plan would be
produced. It is stateless: it reads the issue's whole label history each time. A declared
human is an actor whose login AND non-zero numeric id are in `authorizedAuthorSet()`. Nothing
else is a human for this rule: not a trusted desk identity, not an account the forge types as
a user, not a login without an id. A ratifying comment is a comment whose login and id satisfy
`isBlessAuthorityID`; its text is not read. "Strictly after" compares creation times: a
comment with the same time as the event it must follow does not release.
1. **History gap.** The label history could not be read, was read in part, or the read
   reports itself incomplete (the page bound was reached): HELD, reported as could-not-check.
   A ratifying comment does not release this; a complete read does.
2. **Unnamed events.** An event whose label the forge no longer names may have been an
   excluded label. If the history has any, the issue is clear of them only when a ratifying
   comment exists created strictly after the latest unnamed event. Otherwise HELD.
3. **For each excluded label L with at least one named event**, let E be L's last event:
   - E is an application, and L is not on the issue: the history and the label list disagree,
     and who took L off is unknown. HELD. It is released when a declared human applies and
     removes L, which makes E a human removal.
   - E is a removal by a declared human: L is clear.
   - E is a removal by anyone else, and the history shows an application of L before E; let A
     be the LATEST such application: L is clear only when a ratifying comment exists created
     strictly after A.
   - E is a removal by anyone else, and the history shows no application of L: L is clear only
     when a ratifying comment exists created strictly after E.
4. **Comments unreadable** when step 2 or 3 needs them: HELD, reported as could-not-check.
5. No excluded label has any event and there are no unnamed events, or every one is clear: the
   issue is admitted exactly as today. Anything else: HELD.
6. HELD means no create plan and no reactivate plan this scan. The placeholder stays absent,
   or stays `status: done` where it lies. `planScan` adds a NOTICE; `planTranscribeScan` adds a
   `clauseSkip` whose `Clause` is `label-removal-hold`, or `label-removal-hold
   (could-not-check)` for steps 1 and 4. Each names the issue, the label, the removing actor
   as the forge rendered it, and what releases it. No new status value. No new exit code: this
   follows the trust gate's could-not-check branch in `planScan`, which is a NOTICE and a skip
   with the exit code unchanged, and NOT the unread-repo branch, which exits 2. The consequence
   is named under risks.

how the rule answers each forge behaviour in the last fact:
- a deleted label's events are kept and still named: step 3, either a removal by the deleting
  actor or a last-event application. Held unless released.
- kept and no longer named (GitLab): step 2.
- a renamed label's events keep the old name while the excluded set still lists the old name:
  step 3, a last-event application with the label absent. Held.
- events removed, or renamed retroactively, or not carried across a transfer: no event for L
  remains, so the rule admits. This is the part it cannot see.

the second route (the issue board). The board does NOT get a copy of the rule. It gets a
superset test, so there is one decision function and no second place to keep in step: an open
issue that would classify `CREATE-PLACEHOLDER` is `AWAIT` instead when its label history is
incomplete, or holds ANY event for an excluded label (applied or removed, by anyone), or holds
any unnamed event. The row carries a bracketed reason the way the escalation could-not-check
row does. The superset costs a correctly released issue one scan interval at most: the scanner
admits it and writes its placeholder at the next scan, and an issue with a placeholder is no
longer an un-briefed row. The read is paid only for rows that would otherwise be
`CREATE-PLACEHOLDER`. An incomplete read degrades that one row; a read error fails the board
with exit 6, as its other per-issue reads do. This design is the fixing author's choice among
three; the two rejected are a verdict-carrying `deskread` kind (a `deskread` kind is transport
only and carries no verdict) and a marker placeholder for a held issue (the board does not
read the archive, and a window stays open between the removal and the next scan).

the second lane (`--transcribe-scan`). The default checker reads label events AND the
ratifying comments through `deskread`, on both lanes. For `--transcribe-scan` that is a new
runtime dependency, named here on purpose: its other reads still go through `gh`. Where that
lane runs with no `deskread` on its PATH, or with one that predates the new kind, every create
and reactivate there is held as could-not-check. That is the safe direction, and it means the
lane creates nothing until `deskread` is present; Verify row 11 pins both.

choices made here, where the ruling is silent. The ruling names a removal by a non-human actor
and a comment from the ratifying identity. It does not say how either is told, or what happens
when the history cannot answer. Each choice below is one line to change, and each says who
settles it. An author's choice is not a ruling: a human who wants it otherwise says so at this
brief's gate.
- **"Non-human" is read as "not a declared human"**, which fails closed. The alternative,
  classifying by the forge's account type or by a login suffix, admits a machine user and
  breaks on the suffix mismatch above. Cost of this reading: a person who is not rostered and
  removes the label also causes a hold. Author's choice. The Human decision section takes it
  as given and does not ask it.
- **"A comment exists" is read as "created after the label's latest application"** (step 3).
  The literal reading, any ratifying comment ever, would let an issue that is labelled again
  for a second question be released by the answer to the first. Whether the comment must also
  follow the removal is the human's call: Human decision options 1 and 2.
- **Three cases the ruling does not cover are completed toward hold.** An unnamed event is
  released only by a ratifying comment after the latest one (step 2). A last-event application
  with the label absent is released by no comment, only by a declared human applying and
  removing the label (step 3). A removal with no recorded application is released only by a
  ratifying comment after the removal (step 3). Author's choices, asked in no option.
- **A could-not-check hold is a NOTICE with the exit code unchanged** (step 6). The
  alternative is the unread-repo branch's exit 2, which would make every scan report
  could-not-check for as long as one issue's history stays unreadable. Author's choice; its
  cost is the second named risk.
- **The board gets a superset test, and the forge gets a new op**, in place of a second copy
  of the rule and of a change to the existing label-event reads. Author's choices, reasoned
  under "the second route" above and in design-fit.

named risks:
- **The release signal is weaker than a ruling.** A ratifying comment made after the label
  was applied and BEFORE a non-human removed it releases the hold, whatever it says. A reply
  that asks for more detail counts. This is the cost of the second choice above; Human decision
  option 2 closes it at the price of a second comment.
- **A history that stays unreadable looks like a clean scan.** Steps 1 and 4 hold with a
  NOTICE and the exit code unchanged, so an issue whose history can never be read (a timeline
  past the page bound) is held on every scan with nothing but that NOTICE to show it. A
  durable record of holds is out of scope below.
- **Bot-login rendering.** The same App reaches this code as `slug`, `slug[bot]` or
  `app/slug` depending on the API that produced the event. The rule never looks at the login's
  shape, and Verify row 3 pins all three renderings as held.
- **Tool version skew.** A `deskread` without the new kind makes every label-history read an
  error, so every create and reactivate is held as could-not-check until both binaries are
  current.
- **Reach, partly measured.** The rule looks at the whole history, so a label removed by
  automation long ago holds at the issue's NEXT create or reactivate unless a ratifying
  comment followed its application. Measured on this tracker on 2026-10-08, over the 100 most
  recently updated of 491 open issues: 8 carry no excluded label now and have a removal as the
  last event of one; all 8 are `needs-decision`, and 7 of the 8 removals were by an App
  account. Not measured: how many of those have a ratifying comment after the application,
  the other 391 open issues, and how often automation removes the other four labels on issues
  that are later reopened. Placeholders that are already live produce no plan and are not
  re-examined.
- **Unnamed-event reach, not measured.** On a forge that keeps a deleted label's events and
  stops naming them (GitLab, per the label-event fact above), step 2 holds every issue that
  ever carried ANY label that was later deleted, excluded or not, until a ratifying comment
  follows the latest such event, and the board shows the same issues as `AWAIT`. How many
  issues that is on a given tracker was not measured; deleting one widely used label there
  holds all of them at their next create or reactivate.
- **The board without the scanner.** Where the board runs and the scanner does not, an issue
  with any excluded-label history stays `AWAIT` until someone writes its placeholder or brief
  by hand. `AWAIT` rows of this kind do not age toward `ESCALATE`.
- **Cost.** One label-history read per would-be create or reactivate, one comment read only
  when step 2 or 3 needs it, repeated every scan for an issue that stays held; and one
  label-history read per would-be `CREATE-PLACEHOLDER` row on every board run.
- **What this cannot see.** Automation acting under a declared human's own credential is that
  human to the forge. That includes the release: a comment posted by automation under the
  ratifying identity's credential releases the hold, even one carrying the automation marker
  the un-block lane's `isBotComment` refuses, because this rule reads no comment text. A label
  rename, delete or issue transfer that leaves no event behind is invisible; a rename already
  releases every issue carrying the label today, with or without this brief. And a label
  applied and removed between two scans is never examined when the issue's placeholder was
  already live: `planScan` retires a live placeholder only when a scan sees the label ON, so
  that placeholder is never retired, no create or reactivate plan is produced for it, and the
  rule does not run. That is the act the ruling is about, on a path this brief does not reach
  (Out of scope item 3).

the `risk-files-crossread` lint NOTICE fires here and is answered, not ignored. What is edited
under its trigger: `tools/desk/internal/deskkit/`, where one field is added to `LabelEvent`
and filled at three read sites, and one read-only forge op is added. The four risk answers
stay `no` for stated reasons: the change makes no forge write and adds no credential path
(sensitive-data, irreversible); it only ever withholds a placeholder or a board row, which a
ratifying comment or a later scan undoes (irreversible); it touches no customer-facing or
regulated surface (customer, regulatory). The gate is human on a different ground, stated in
`gate-why`: this is a trust boundary, and a wrong rule here admits work no human released.

sizing: `effort: L`, kept whole on purpose. The new forge op cannot land without its consumer
(the freeze rule), and the scanner hold must not land ahead of the board change, because on
its own it ships the open second route described above. If the reviewer prefers two units,
the seam is: (a) the actor id, the forge op, the `deskread` kind and the board change; then
(b) the scanner hold, depending on (a).

Out of scope — open, NOT decided by this brief or by the ruling behind it:
1. Ageing or escalating a held row on the issue board. The row is visible as `AWAIT` with its
   reason; nothing times it.
2. A durable record of holds beyond the scan's NOTICE and skip output and the board row.
3. Re-examining or rewriting placeholders already on the board.
4. The signed scan-delta create path, `planScanDelta` in `statusgen/transcribescan.go`. It
   creates placeholders from the entries of a signed delta and reads no label of the entry's
   issue at all, excluded or otherwise. Whether its producer applies the excluded-label rule,
   and so whether a hold belongs there, was not established at authoring.
5. Moving `--transcribe-scan`'s other reads off `gh` (forge-neutral brief 18).
6. The un-block lane's own bot detection (`isBotComment`), which does read the login suffix.
7. Recording this hold in the contract register as a reader that is not an acceptance anchor
   (see design-fit).

Also unchanged: the excluded-label set, the trust gate on issue authors, `retire-label` and
`sweep` close-outs, the never-overwrite rule, placeholder gate derivation, every existing
field of `LabelEvent`, and what `ListLabelEvents` and `ListIssueLabelEvents` return to their
current readers.

design-fit:
  owner: `statusgen/scanissues.go` — the scanner's admission decision (`planScan`), which `statusgen/transcribescan.go` mirrors; the issue board's classifier (`tools/desk/cmd/issueboard/board.go`) owns only what it offers as un-briefed work
  contract: S-decision-acceptance — this hold is NOT one of that contract's anchors and does not decide whether a decision is ratified. The anchors need a closure by the blessed human, a per-record marker, or a resolving ruling link; the hold accepts a weaker signal (a comment's author and time, text unread) and decides only whether an issue may be dispatched. It reuses the contract's id-pinned identity check and nothing else
  retires: []
  weight: verbs 0, flags 0, forge ops +1, deskread kinds +1, `LabelEvent` fields +1, scanner refusals +1 (one hold, reported two ways), issue-board classifier inputs +1, rule-text lines +2 (one sentence in each of two skills)
  why-add: >-
    The hold goes INTO the owner's admission paths as one shared check, called from both scan
    functions, not a rule per path, and the board gets a superset test instead of a second
    copy. A new forge op is needed because the existing label-event reads drop the unnamed
    events and report no completeness, and changing them would change what seven current
    readers see. A new `deskread` kind is needed because no existing kind returns label
    events; `--scan-issues` has no other route to the forge, and `--transcribe-scan`, which
    still shells out to `gh`, is deliberately given the same route so the rule has one reader.
    The actor id is a new field because a login alone cannot be pinned to an identity. Removal
    considered: the create and reactivate paths could not be merged to pay for this, since one
    writes a file and the other rewrites one. Reusing the trust read's blessing events instead
    of a label-history read was rejected: it answers "is the content blessed", not "who
    removed the label".

## Human decision
When an issue carries a label saying a person still has to decide it (or one of four other
system labels), no dispatchable work item is created for it. Today, the moment such a label is
taken off, the work becomes dispatchable, and nothing checks who took it off, so an automated
agent that removes the label by mistake turns an undecided question into work with no person
involved. This change adds a hold: when the label was removed by anything other than a person
on the roster of human authors, the work stays out of the queue until the person who ratifies
decisions has commented on the issue. Measured on this tracker on 2026-10-08, among the 100
most recently updated open issues, 8 have had the decision label removed and carry no system
label now; 7 of those removals were by an automated account. Three settings of the rule trade
how much an agent's mistake can still slip through against how often a person must act twice.
Under all three the hold is checked only when a work item would be created or brought back: a
work item that already exists when the label goes on, and whose label is taken off again before
the next scan runs, stays in the queue and is never checked.

Options:
1. **Hold on any non-human removal; a comment made after the label was applied releases
   (recommended)** — covers all five system labels. Consequence: when the ratifying person has
   already answered and an agent then removes the label, the work proceeds with no second
   step; but a comment from that person that was not a ruling, such as a request for more
   detail, also releases, because the wording of the comment is not read.
2. **The same hold; only a comment made after the removal releases** — Consequence: closes
   the gap in option 1, and every time an agent removes a label after the person has ruled,
   that person must comment once more before the work proceeds.
3. **Hold only for the two decision labels (`needs-decision`, `needs-human`), with option 1's
   release** — Consequence: automation that removes the other three system labels as part of
   normal flow never causes a hold, and a mistaken removal of one of those three is not
   caught.

Default if no answer: none — blocks until answered (this decides when a human decision is taken as given, and must not be defaulted).

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Do not start before the Human decision is answered. Nothing but this line holds you:
  dispatch does not wait for the answer. A human-gated brief dispatches normally, and the
  dispatch step files the decision issue and then launches the worker. If the decision is
  unanswered when you are launched, stop and report NEEDS_CONTEXT.
- The rule above is written as option 1. Option 2 changes one comparison in step 3 (after E,
  not after A), the release wording of Task 13's intake-desk sentence ("since the label was
  removed", not "since the label was applied") and the same wording in the changelog fragment.
  Option 3 changes the set step 3 iterates and the board test reads, and Task 13's two
  sentences and the changelog fragment name the two decision labels where they now say the
  excluded label. Change the matching table cases with it.
- Before starting, merge main and check which of the open PRs listed in `sources:` have
  landed (#2377, #2382, #2266, #1688, #2218, #2225) and whether statusgen brief 17 has been
  implemented; build on what is there and do not re-do their edits. Take the next FREE op
  number by reading every numbered row of the register, not the end of its last table: 61 is
  taken on main and #2382 adds 62 and 63.
- ONE decision function, pure, called by both scan functions through one injected checker. Do
  not write the rule twice. The issue board gets the superset test only, never the release
  rule.
- Classify the actor by login AND numeric id against `authorizedAuthorSet()` only. Never by
  `[bot]` suffix, `app/` prefix, account type, or the trusted-bot roster.
- Fail closed everywhere: an unreadable, partial or incomplete label history, an unreadable
  comment read, an event with no actor or id 0, an unparsable timestamp, a nil checker. None
  of these admits.
- No scan function and no entry point defaults a missing checker to "clear". The
  clear-everything checker the existing tests need, `labelHoldClearAll` (planned), is defined
  in a `_test.go` file, so no production build can reference it.
- `LabelEvent` changes by ADDING a field. Do not rename or re-type an existing one. The
  existing `ListLabelEvents` and `ListIssueLabelEvents` keep returning exactly what they
  return today, including the dropped deleted-label events; the unnamed events and the
  completeness flag exist only on the new op.
- Do not add a status value, an exit code, a flag, a label, or a board action. Do not change
  the excluded set.
- Do not edit the assertions of the existing reactivate and trust-gate tests; they gain the
  new checker argument and nothing else. ONE exception, because it pins the argument list as
  an exact string: `TestScanIssuesMainWiresNativeReads` is updated to the new argument list.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Fail-first: write the tests in steps 8–10 before steps 6–7, run Verify row 4, and paste the
   red output (a `reactivate` plan produced for a bot-removed label) into the PR body under
   `## Fail-first`.
2. Establish what each forge returns in an issue's label history after (a) the label is
   deleted, (b) the label is renamed, (c) the issue is transferred. Use a scratch issue or the
   forge's own documentation, and record the result in the PR body under `## Forge behaviour`,
   naming the source for each of the six cells and writing could-not-check for any you could
   not establish. Once step 5 exists, `deskread label-events --issue <owner>/<repo>#<n>` is
   the command that re-establishes it. If a behaviour falls outside the four the Context lists,
   stop and report NEEDS_CONTEXT.
3. deskkit, the actor id: add the actor's numeric id to `LabelEvent` and fill it in the GitHub
   timeline read and both GitLab label-event reads. Test
   `TestLabelEventsCarryActorID` (planned) in `tools/desk/internal/deskkit/`: for each forge,
   a labelled and an unlabelled event decode with the actor's login and id, and an event with
   no actor decodes with id 0.
4. deskkit, the new op: `IssueLabelHistory` (planned) on the `Forge` interface and both
   backends. It returns every label add and remove event of one issue, INCLUDING an event
   whose label the forge no longer names (empty name, action, time and actor kept), and a
   completeness flag that is false when the page bound was reached. A transport or permission
   failure is an error. Register it: the op register row with its two consumers (step 5's
   kind and step 11's board read), the surface list, the outbound class (`read`), and golden
   fixtures on both backends. Test `TestIssueLabelHistory` (planned): on each forge, named
   events decode with login and id; on GitLab a deleted-label event is returned with an empty
   name where `ListIssueLabelEvents` over the same fixture drops it; a history past the page
   bound returns incomplete, not an error and not a short complete list; a failed request is
   an error.
5. deskread: add the per-issue kind `label-events` (planned), addressed like `comments`, on
   the new op. Each event carries the label name (empty when unnamed), whether it is a
   removal, its time, and the actor's login and id; the item carries the completeness flag.
   The kind is transport: it carries no verdict. A read error for one issue lands in `partial`
   like the other per-issue kinds. Tests `TestDeskreadLabelEvents` (planned) and
   `TestDeskreadLabelEventsPartial` (planned) in `tools/desk/cmd/deskread/`.
6. statusgen reader: `IssueLabelHistory` (planned) on `deskreadReader`, through `readItem`.
   It returns the events and the completeness flag; a partial, empty or malformed envelope,
   and an item with no completeness field, is an error. Test
   `TestLabelHistoryUnreadShapes` (planned), with a stub `deskread` the way
   `statusgen/scanissues_nativeread_test.go` does: a good envelope decodes; an envelope with the issue in `partial`, one with no item, one with
   no completeness field, a non-zero exit, and an envelope from a `deskread` that rejects the
   kind each return an error; an item marked incomplete decodes as incomplete.
7. The decision, in `statusgen/labelremovalhold.go` (planned): a pure function over the
   excluded set, the events, the completeness flag and the comments that implements rule steps
   1–5 and returns clear, held (with the label and actor), or held could-not-check. The
   default checker, `defaultScanLabelHoldChecker` (planned), reads the label history first and
   the comments only when step 2 or 3 needs them, both through `deskreadReader`. Add the
   checker as an injected parameter of `planScan`, `planTranscribeScan`, `runScanIssues` and
   `runTranscribeScan`, beside the bless checker, and call it on all three paths in each scan
   function: on the create path AFTER the trust gate passes, so a quarantined issue costs no
   extra read. A nil checker holds as could-not-check. Emit the NOTICE or `clauseSkip` of rule
   step 6. Wire the default into BOTH entry-point calls in `statusgen/main.go`.
8. `TestLabelHoldDecision` (planned), a table over the pure function. Each case names the
   events, the completeness flag, the comments and the expected result:
   - removed by an App, no comment → held
   - removed by the ratifying identity → clear; removed by another rostered human → clear
   - removed by an App, ratifying comment after the application → clear, whether the comment
     came before or after the removal
   - removed by an App, the only ratifying comment predates a later re-application → held
   - removed by an App, comment by the ratifying login with a DIFFERENT id → held; comment by
     another rostered human → held
   - the removing actor is a trusted desk identity, its login rendered `app/<slug>` (a
     rostered rendering that does not end in `[bot]`) → held
   - the removing actor has the ratifying login and id 0, or a different id → held
   - a ratifying comment with the SAME creation time as the event it must follow (the latest
     application; the removal, where no application is recorded; the latest unnamed event) →
     held in each of the three
   - removed by an App with NO application in the history: ratifying comment before the
     removal → held; after it → clear
   - the last event for the label is an application and the label is absent → held, with a
     ratifying comment after it → still held; a later removal by a rostered human → clear
   - an unnamed event, no comment → held; a ratifying comment before it → held; after it →
     clear; two unnamed events with the comment between them → held
   - history marked incomplete, otherwise clear, with a ratifying comment → held could-not-check
   - no event for any excluded label and no unnamed event → clear, with no comment read
   - two excluded labels, one clear and one not → held, naming the one that is not
   - an unparsable event or comment time → held
9. `TestLabelHoldBotRenderings` (planned): one App id rendered `slug`, `slug[bot]` and
   `app/slug` as the removing actor, no comment → held three times, and the hold's message
   carries the login exactly as rendered; the same three with the id of a rostered human whose
   login differs → held.
10. Scan-level tests, each on a temp root with `fixtureLister` and a fixture checker:
    - `TestScanLabelHold` (planned), through `planScan`, for each of the three paths: held →
      no create plan and no `reactivate` close-out, and one NOTICE naming the issue and
      label; clear → the same plan as today.
    - `TestScanLabelHoldFailsClosed` (planned): the checker returns an error on each path, and
      then is nil on each path → no plan, and a could-not-check NOTICE.
    - `TestScanLabelHoldKeepsDone` (planned): a root placeholder at `status: done`, a held
      issue; plan, apply every plan produced, re-parse the stream — the placeholder still
      reads `done`. Then the checker returns clear, and the same sequence leaves it `todo`.
    - `TestTranscribeLabelHold` (planned): the three paths through `planTranscribeScan`, held,
      clear, error and nil, asserting the `clauseSkip` and its `Clause` value on each hold.
    - `TestLabelHoldDefaultChecker` (planned): the DEFAULT checker against a stub `deskread`.
      A label-history read that errors, one that lists the issue in `partial`, one marked
      incomplete, one from a `deskread` that rejects the kind, and a good history followed by
      an unreadable comment read each return held could-not-check. A history with no
      excluded-label event returns clear and the stub records no comment read.
    - `TestScanIssuesLabelHoldE2E` (planned): `runScanIssues` called with the same four
      defaults `statusgen/main.go` passes, a stub `deskread` and a failing stub `gh`, the way
      `TestScanIssuesTrustAndUnblockReadsUseNativeForgeNotGH` does. An issue whose
      `needs-decision` was removed by an App with no ratifying comment → no placeholder file
      is written and the output carries the hold NOTICE; the same issue removed by a rostered
      human → the placeholder is written; a stub that rejects the kind → no placeholder and a
      could-not-check NOTICE.
    - `TestTranscribeLabelHoldE2E` (planned): `runTranscribeScan` on an armed fixture, the way
      the armed case in `statusgen/transcribescan_test.go` does, with the default checker. With
      a stub `deskread`: a bot-removed label holds and a human-removed one creates. With NO
      `deskread` on PATH: nothing is created or reactivated and each skip is the
      could-not-check clause.
    - `TestLabelHoldMainWiring` (planned): reads `statusgen/main.go` and asserts the
      `runTranscribeScan` call passes `defaultScanLabelHoldChecker`; update
      `TestScanIssuesMainWiresNativeReads` to the new exact argument list of the
      `runScanIssues` call.
11. Issue board: add one input to `issueClassifyInput`, set when the issue's label history is
    incomplete, holds any event for an excluded label, or holds any unnamed event. In
    `classifyIssue` that input turns what would be `CREATE-PLACEHOLDER` into `AWAIT`, and
    changes no other outcome. Read the history through `forgeFor` and the new op, only for an
    issue that would otherwise be `CREATE-PLACEHOLDER`. Mark the row with a bracketed reason,
    and with a could-not-check reason when the read was incomplete. A read error fails the
    board as `fetchIssueEvents`'s does. Tests in `tools/desk/cmd/issueboard/`:
    `TestClassifyLabelHistoryHold` (planned), over the pure classifier: the input set → `AWAIT`
    where it was `CREATE-PLACEHOLDER`, and every other row of the existing classifier table
    unchanged with the input set. `TestBoardLabelHistoryHold` (planned), end to end with the
    package's fake forge: an un-briefed issue whose `needs-decision` was removed renders
    `AWAIT` with the reason; one with no label history renders `CREATE-PLACEHOLDER`; one with
    an incomplete history renders `AWAIT` with the could-not-check reason while the other rows
    still render; a failing history read exits 6; and the call log shows no history read for
    an issue that has a placeholder.
12. Pass the clear-everything checker to every existing `planScan`, `planTranscribeScan`,
    `runScanIssues` and `runTranscribeScan` call in the tests. Their assertions stay as they
    are, apart from the one exception in Ground rules.
13. Skills, ONE sentence each, generated blocks left alone. In the intake-desk skill, where it
    says a scanner `reactivate` sets a parked row `todo`: when the excluded label was removed
    by anything other than a rostered human, the scanner holds the item until the ratifying
    identity has commented on the issue since the label was applied. In the worker-desk skill,
    §"Un-briefed issues": an issue the board shows as `AWAIT` for its label history is not
    un-briefed work, whatever its current labels.
14. In this brief's `consumers:` block, replace every `follow-up statusgen/16 (this brief; …)`
    routing with `fixed-here`, in the same change. All thirteen entries are routed; none stays
    `follow-up`.
15. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)
Every row that runs a named test anchors its selector, writes the output to a file and asserts
that test's `--- PASS:` line, so a missing or renamed test fails the row.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./... && cd ../tools/desk && go build ./... && go vet ./...` | exit 0 in both modules, so the added `LabelEvent` field and the added forge method broke none of their current readers or implementers | check:ci |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelHoldDecision$' -v . > "${TMPDIR:-/tmp}/sg16-r2.out" 2>&1 && grep -F -e '--- PASS: TestLabelHoldDecision' "${TMPDIR:-/tmp}/sg16-r2.out"` | exit 0; every case of Task 8 holds. Mutations, each of which must turn this row red: the declared-human check replaced by "login does not end in `[bot]`" (red on the trusted-desk-identity case, whose login is rendered `app/<slug>`, and on the id-0 case); the after-application comparison dropped (red on the re-application case); "strictly after" relaxed to "at or after" (red on the same-time cases); an incomplete history treated as an empty one (red on the incomplete case); unnamed events skipped (red on the unnamed cases); only labels with a removal iterated (red on the last-event-application case); "any ratifying comment" accepted when no application is recorded (red on the comment-before-removal case) | check:ci +mutation |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelHoldBotRenderings$' -v . > "${TMPDIR:-/tmp}/sg16-r3.out" 2>&1 && grep -F -e '--- PASS: TestLabelHoldBotRenderings' "${TMPDIR:-/tmp}/sg16-r3.out"` | exit 0; the same App rendered as the bare slug, with the `[bot]` suffix and with the `app/` prefix is held in all three forms, and each hold message carries the login as rendered. With the actor classified by login shape, at least one of the three is admitted and the row exits 1 | check:ci |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanLabelHold$' -v . > "${TMPDIR:-/tmp}/sg16-r4.out" 2>&1 && grep -F -e '--- PASS: TestScanLabelHold' "${TMPDIR:-/tmp}/sg16-r4.out"` | exit 0; through `planScan`, a held issue gets no create plan on the create path and no `reactivate` close-out on the root and archive paths, with one NOTICE each; a clear issue gets the plan it gets today. Mutation: with the checker call removed from any one of the three paths the row exits 1 on that path | check:ci +mutation |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanLabelHoldFailsClosed$' -v . > "${TMPDIR:-/tmp}/sg16-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanLabelHoldFailsClosed' "${TMPDIR:-/tmp}/sg16-r5.out"` | exit 0; when the checker errors, and when it is nil, no path produces a plan and each reports could-not-check. This is the negative path: the lower layer refuses with nothing above it deciding. Mutation: a nil or erroring checker treated as clear turns the row red | check:ci +mutation |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanLabelHoldKeepsDone$' -v . > "${TMPDIR:-/tmp}/sg16-r6.out" 2>&1 && grep -F -e '--- PASS: TestScanLabelHoldKeepsDone' "${TMPDIR:-/tmp}/sg16-r6.out"` | exit 0; plan, apply and re-parse: the held issue's placeholder still reads `done` and is not dispatchable, and once the check clears the same sequence leaves it `todo` | check:ci +flow |
| 7 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestTranscribeLabelHold$' -v . > "${TMPDIR:-/tmp}/sg16-r7.out" 2>&1 && grep -F -e '--- PASS: TestTranscribeLabelHold' "${TMPDIR:-/tmp}/sg16-r7.out"` | exit 0; the second scanner holds, admits and fails closed (error and nil) on the same three paths, and records a `clauseSkip` with the `label-removal-hold` clause for each hold | check:ci |
| 8 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelHistoryUnreadShapes$' -v . > "${TMPDIR:-/tmp}/sg16-r8.out" 2>&1 && grep -F -e '--- PASS: TestLabelHistoryUnreadShapes' "${TMPDIR:-/tmp}/sg16-r8.out"` | exit 0; a good envelope decodes with login, id and the completeness flag; a partial envelope, an empty one, one with no completeness field, a failed run and a `deskread` that rejects the kind are each an error, never an empty event list | check:ci |
| 9 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelHoldDefaultChecker$' -v . > "${TMPDIR:-/tmp}/sg16-r9.out" 2>&1 && grep -F -e '--- PASS: TestLabelHoldDefaultChecker' "${TMPDIR:-/tmp}/sg16-r9.out"` | exit 0; the default checker holds as could-not-check on an erroring, a partial, an incomplete and a rejected label-history read and on an unreadable comment read, and reads no comment for an issue with no excluded-label event. Mutations, each red: the default checker returns clear on a read error; it treats an error as an empty history; it reads comments before the history | check:ci +mutation |
| 10 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanIssuesLabelHoldE2E$' -v . > "${TMPDIR:-/tmp}/sg16-r10.out" 2>&1 && grep -F -e '--- PASS: TestScanIssuesLabelHoldE2E' "${TMPDIR:-/tmp}/sg16-r10.out"` | exit 0; through the real entry point on the default wiring: a non-human removal with no ratifying comment writes no placeholder and prints the hold NOTICE; a rostered-human removal writes it; a `deskread` that rejects the kind writes none and prints could-not-check. Mutation: with the default checker's body replaced by "clear", the first and third cases write a placeholder and the row exits 1 | check:ci +flow +mutation |
| 11 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestTranscribeLabelHoldE2E$' -v . > "${TMPDIR:-/tmp}/sg16-r11.out" 2>&1 && grep -F -e '--- PASS: TestTranscribeLabelHoldE2E' "${TMPDIR:-/tmp}/sg16-r11.out"` | exit 0; through `runTranscribeScan`, armed, with the default checker: a bot-removed label holds and a human-removed one creates; with no `deskread` on PATH nothing is created or reactivated and every skip is the could-not-check clause. A default checker that falls back to "clear" when `deskread` is absent turns the row red | check:ci +flow |
| 12 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelHoldMainWiring$' -v . > "${TMPDIR:-/tmp}/sg16-r12a.out" 2>&1 && grep -F -e '--- PASS: TestLabelHoldMainWiring' "${TMPDIR:-/tmp}/sg16-r12a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanIssuesMainWiresNativeReads$' -v . > "${TMPDIR:-/tmp}/sg16-r12b.out" 2>&1 && grep -F -e '--- PASS: TestScanIssuesMainWiresNativeReads' "${TMPDIR:-/tmp}/sg16-r12b.out" && grep -l -r -e 'func labelHoldClearAll' --include='*_test.go' . && test -z "$(grep -l -r -e 'func labelHoldClearAll' --include='*.go' --exclude='*_test.go' .)"` | exit 0; both entry-point calls in `statusgen/main.go` pass `defaultScanLabelHoldChecker`, and the clear-everything checker (`labelHoldClearAll`, planned) is defined in a test file and in no non-test file, so a rename or a move into production code fails the row. Mutation: with either call's checker argument replaced by an inline always-clear function, the matching pin exits 1 | check:ci +mutation |
| 13 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestCloseOutReactivateAfterLabelRemoved$' -v . > "${TMPDIR:-/tmp}/sg16-r13a.out" 2>&1 && grep -F -e '--- PASS: TestCloseOutReactivateAfterLabelRemoved' "${TMPDIR:-/tmp}/sg16-r13a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestCloseOutReactivateFromArchive$' -v . > "${TMPDIR:-/tmp}/sg16-r13b.out" 2>&1 && grep -F -e '--- PASS: TestCloseOutReactivateFromArchive' "${TMPDIR:-/tmp}/sg16-r13b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanTrustGateUnverifiableFailsClosed$' -v . > "${TMPDIR:-/tmp}/sg16-r13c.out" 2>&1 && grep -F -e '--- PASS: TestScanTrustGateUnverifiableFailsClosed' "${TMPDIR:-/tmp}/sg16-r13c.out"` | exit 0; three tests that exist today still pass: with the check clear, a removed label still reactivates from the root and from the archive, and the trust gate still fails closed on its own. The row cannot see whether their assertions were edited; that is the reviewer's read of the diff of those three functions, which should show the added argument and nothing else | check:ci |
| 14 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | exit 0; the whole package passes | check:ci |
| 15 | `cd tools/desk && go test -count=1 -timeout 300s -run '^TestLabelEventsCarryActorID$' -v ./internal/deskkit/ > "${TMPDIR:-/tmp}/sg16-r15.out" 2>&1 && grep -F -e '--- PASS: TestLabelEventsCarryActorID' "${TMPDIR:-/tmp}/sg16-r15.out"` | exit 0; on both forges a labelled and an unlabelled event carry the actor's login and numeric id, and an event with no actor carries id 0 | check:ci |
| 16 | `cd tools/desk && go test -count=1 -timeout 300s -run '^TestIssueLabelHistory$' -v ./internal/deskkit/ > "${TMPDIR:-/tmp}/sg16-r16.out" 2>&1 && grep -F -e '--- PASS: TestIssueLabelHistory' "${TMPDIR:-/tmp}/sg16-r16.out" && grep -n -F -e 'IssueLabelHistory' ../../docs/streams/forge-gitlab/inventory.md` | exit 0; the new op returns named events with login and id, returns a deleted-label event with an empty name where the existing read drops it, reports a history past the page bound as incomplete, and errors on a failed request; the op register names the op. Mutations, each red: unnamed events dropped as the existing reads do; the page bound returned as a complete list | check:ci +mutation |
| 17 | `cd tools/desk && go test -count=1 -timeout 300s -run '^TestDeskreadLabelEvents$' -v ./cmd/deskread/ > "${TMPDIR:-/tmp}/sg16-r17a.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadLabelEvents' "${TMPDIR:-/tmp}/sg16-r17a.out" && go test -count=1 -timeout 300s -run '^TestDeskreadLabelEventsPartial$' -v ./cmd/deskread/ > "${TMPDIR:-/tmp}/sg16-r17b.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadLabelEventsPartial' "${TMPDIR:-/tmp}/sg16-r17b.out"` | exit 0; the new kind returns each event's label, removal flag, time, actor login and actor id and the item's completeness flag, and an issue whose history cannot be read is reported in `partial`, not as an empty list | check:ci |
| 18 | `cd tools/desk && go test -count=1 -timeout 300s -run '^TestClassifyLabelHistoryHold$' -v ./cmd/issueboard/ > "${TMPDIR:-/tmp}/sg16-r18a.out" 2>&1 && grep -F -e '--- PASS: TestClassifyLabelHistoryHold' "${TMPDIR:-/tmp}/sg16-r18a.out" && go test -count=1 -timeout 300s -run '^TestBoardLabelHistoryHold$' -v ./cmd/issueboard/ > "${TMPDIR:-/tmp}/sg16-r18b.out" 2>&1 && grep -F -e '--- PASS: TestBoardLabelHistoryHold' "${TMPDIR:-/tmp}/sg16-r18b.out"` | exit 0; on the board an un-briefed issue with a removed excluded label is `AWAIT` with its reason and not `CREATE-PLACEHOLDER`; one with no label history is still `CREATE-PLACEHOLDER`; an incomplete history degrades that one row; a failed read exits 6; an issue with a placeholder costs no history read. Mutation: with the new input ignored by the classifier, or never set by the board, the row exits 1. This is the second route, tested with the scanner absent | check:ci +flow +mutation |
| 19 | `cd tools/desk && go test -count=1 -timeout 900s ./cmd/deskread/ ./cmd/issueboard/ ./internal/deskkit/` | exit 0; all three packages pass, so the existing kinds, the existing board rows, the existing label-event readers, the forge surface list, the outbound class map and both golden corpora agree with the change | check:ci |
| 20 | `grep -n -F -e 'ratifying identity' plugins/assay/skills/intake-desk/SKILL.md && grep -n -F -e 'label history' plugins/assay/skills/worker-desk/SKILL.md && cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg16-skl" . && "${TMPDIR:-/tmp}/sg16-skl" --root ../..` | exit 0; the lines printed are the reactivate sentence stating the hold and the un-briefed sentence stating that an `AWAIT` row for label history is not un-briefed work, and both skills still pass every skill check. The greps match under any of the three Human decision options: that the reactivate sentence states the release of the option the human chose is the reviewer's read of the line printed | check:ci +dereference |
| 21 | `cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg16c" . && cd .. && "${TMPDIR:-/tmp}/sg16c" --root . --consumers --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg16-r21.out" && test "$(grep -c -E -e 'CORROBORATED +[^ ]+: fixed-here' "${TMPDIR:-/tmp}/sg16-r21.out")" -eq 13 && grep -F -e ', 0 disproved,' "${TMPDIR:-/tmp}/sg16-r21.out"` | exit 0; run on the implementing branch before merge, after Task 14: thirteen entries are reported corroborated as `fixed-here` by the branch's diff and none is disproved. A branch that leaves the routings as `follow-up` fails on the count (they are corroborated, but not as `fixed-here`); a branch that flips one without touching its path is disproved, which fails both the checker's exit code and the summary line. On merged main the merge-base is the head, the diff holds no brief file, the checker prints that there is nothing to corroborate and the count is 0, so the row fails there: its Evidence is the run at the implementing branch's head before the merge | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter). Human gate is MANDATORY — this changes a trust boundary: it decides when a human's ratification of an open decision is taken as given. Reviewer records verdict + date in the stream README table.
