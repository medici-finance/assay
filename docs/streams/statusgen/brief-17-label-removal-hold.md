---
brief: assay:assay:statusgen:17
title: '`--scan-issues` / `--transcribe-scan`: hold a placeholder whose excluded label was removed by a non-human actor until the ratifying identity has commented'
why: >-
  An issue labelled `needs-decision` (or any other system-state label) is kept off the board:
  the scanner writes no dispatchable placeholder for it while the label is on. The moment the
  label comes off, the scanner creates or re-activates the placeholder as `todo`, and it never
  asks who took the label off. So an agent that removes the label, by mistake or because it
  misread a relayed answer as the ruling, turns an undecided question into dispatchable work
  with no human act anywhere in the chain. The driver ruled on 2026-10-08 that when an excluded
  label was removed by a non-human actor, the placeholder is held until a comment from the
  ratifying identity exists on the issue. This brief makes both scanners apply that.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
outcome: none
exec-tier: strong
exec-tier-why: >-
  Crosses two Go modules and three admission paths in two scan functions (question b), and it
  is trust plumbing where a wrong actor or ordering rule still passes a happy-path test
  (question c).
schema: brief-v2
version: 1
authored: 2026-10-08 by the-desk session (on behalf of the driver)
sources:
  - "ruled by the driver, 2026-10-08: hold a placeholder whose excluded label was removed by a non-human actor until a comment from the ratifying identity exists on that issue"
  - "statusgen/scanissues.go — `planScan`: the create path and the two reactivate paths, none of which reads who removed a label"
  - "statusgen/transcribescan.go — `planTranscribeScan`: the same three paths for the home repo, marked KEEP IN SYNC with `planScan`"
  - "statusgen/trustgate.go — `isBlessAuthorityID`, `authorizedAuthorSet`: the identity checks this brief reuses"
  - "tools/desk/internal/deskkit — `LabelEvent`, `ListIssueLabelEvents`: the label-event read, which carries a login and no numeric id today"
  - "freshness-checked 2026-10-08 @ 403b8ec8c (origin/main) — no Go file under `statusgen/` reads a label event; `deskread` has no label-event kind; open PRs #2377 (`tools/desk/cmd/deskread/main.go`) and #2382 (the deskkit forge files) touch files this brief names"
consumers:
  - "tools/desk/internal/deskkit/modelstamp.go: follow-up statusgen/17 (this brief; `LabelEvent` gains the actor's numeric id, additive)"
  - "tools/desk/internal/deskkit/forge_github.go: follow-up statusgen/17 (this brief; the timeline read fills the actor id)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: follow-up statusgen/17 (this brief; both label-event reads fill the actor id)"
  - "tools/desk/cmd/deskread/main.go: follow-up statusgen/17 (this brief; the new per-issue `label-events` kind)"
  - "statusgen/forgeread.go: follow-up statusgen/17 (this brief; the reader for that kind)"
  - "statusgen/scanissues.go: follow-up statusgen/17 (this brief; the hold on the create path and both reactivate paths)"
  - "statusgen/transcribescan.go: follow-up statusgen/17 (this brief; the same hold on its three paths)"
  - "statusgen/main.go: follow-up statusgen/17 (this brief; wires the default checker into both entry points)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up statusgen/17 (this brief; one sentence where the skill describes a scanner reactivate)"
  - "tools/desk/cmd/issueboard/board.go: out-of-scope (the issue board has no lane for a held issue; listed under Context as open, not decided)"
---

# Brief 17 — hold a placeholder when a non-human removed its excluded label

## Context

files:
- `tools/desk/internal/deskkit/modelstamp.go` — `LabelEvent` gains one field, the actor's
  numeric id.
- `tools/desk/internal/deskkit/forge_github.go`, `tools/desk/internal/deskkit/forge_gitlab.go`
  — the label-event reads fill it.
- `tools/desk/cmd/deskread/main.go` (and its forge seam beside it) — a new per-issue kind,
  `label-events` (planned).
- `statusgen/forgeread.go` — the reader for that kind.
- `statusgen/labelremovalhold.go` (planned) — the pure decision and the default checker.
- `statusgen/scanissues.go`, `statusgen/transcribescan.go`, `statusgen/main.go` — the hold and
  its wiring.
- the test files named in Task, and `plugins/assay/skills/intake-desk/SKILL.md` (one sentence).
- `changelog/<branch-slug>.md` — the per-PR fragment this repo enforces.

single-point-of-failure: custody of the label. Today the only thing between an undecided issue
and a dispatchable placeholder is that nobody removes the excluded label early, and every
identity with label-write on the repo can. This brief adds one independent layer behind it:
the scanner reads the forge's own append-only label-event log and comment thread, a record the
actor who removed the label cannot rewrite. A third layer stays out of band and is unchanged:
a `gate: human` placeholder still needs its human sign-off after dispatch.

facts (all read on main @ 403b8ec8c, 2026-10-08):
- **The excluded set.** `scanExcludedLabelSet()` in `statusgen/scanissues.go` is the
  topology's system-state labels: `verify-gate`, `live-verify`, `needs-decision`,
  `review-request`, `needs-human`. `hasExcludedLabel` matches an issue's current labels
  against it.
- **Three admission paths, none of which reads an actor.** In `planScan`: (1) CREATE: an open
  issue with an excluded label is skipped and gets no placeholder, so when the label comes off
  the next scan creates one as `todo`; (2) ROOT REACTIVATE: a placeholder retired to
  `status: done` whose issue is open and no longer excluded gets a `closeOutPlan` with action
  `reactivate`; (3) ARCHIVE REACTIVATE: the same for an archived placeholder. Each decides from
  the issue's CURRENT label list only.
- **A second scanner has the same three paths.** `planTranscribeScan` in
  `statusgen/transcribescan.go` creates and reactivates for the home repo, and its close-out
  loop is marked KEEP IN SYNC with `planScan`'s. Its refusals are recorded as `clauseSkip`
  rows; `planScan`'s are NOTICE strings.
- **The label-event read exists but carries no numeric id.** `deskkit.LabelEvent` is
  `Name`, `AppliedBy` (a login), `Removed`, `CreatedAt`. `ListIssueLabelEvents` reads the
  GitHub issue timeline (paged, an error past the page bound) and GitLab's
  `resource_label_events`. The GitHub wire struct decodes `actor.login` only. Both forges
  return the actor's id on the same event object. Six desk commands and deskkit's own
  stamp reducer read `LabelEvent` today, so the change to it must be additive.
- **statusgen reaches the forge only through `deskread`.** `deskreadReader` in
  `statusgen/forgeread.go` runs the `deskread` binary and decodes its envelope; every unread
  shape is an error, never a zero value. It has `IssueTrust` and `IssueComments`. `deskread`
  has no label-event kind. The two modules share no code: statusgen declares its own copy of
  the envelope fields it reads.
- **The ratifying identity is already defined with an id.** It is the configured blessing
  authority (`ASSAY_BLESS_LOGIN`, a login and a numeric id). `isBlessAuthorityID(login, id)`
  is true only for that login with that id. `authorizedAuthorSet()` returns the rostered
  human authors as login to mandatory id, seeded with the blessing authority; with nothing
  rostered it is the blessing authority alone.
- **The `[bot]` suffix is not a stable signal.** One App is rendered `slug[bot]` by the REST
  API, bare `slug` by GraphQL (the trust reader re-suffixes it for that reason), and
  `app/slug` by some clients. GitLab's label event carries no account type at all, and a
  machine user is an ordinary user account on both forges.

the rule (what "held" means, exactly). It is evaluated for an OPEN issue whose current labels
carry no excluded label, immediately before a create plan or a reactivate plan would be
produced, and it is stateless: it reads the issue's whole label-event history each time.
1. For each excluded label L that has at least one REMOVAL event on the issue: let R be the
   last removal of L, and A the time of L's latest application before R (unrecorded if the
   log shows none).
2. If the log's last event for L is an application, the log and the label list disagree: HELD.
3. If R's actor is a declared human, L is clear. A declared human is an actor whose login AND
   non-zero numeric id are in `authorizedAuthorSet()`. Nothing else is a human for this rule:
   not a trusted desk identity, not an account the forge types as a user, not a login without
   an id.
4. Otherwise L is clear only when the issue has a comment authored by the ratifying identity
   (`isBlessAuthorityID` on the comment's login and id) created strictly after A. When A is
   unrecorded any such comment counts. The comment's text is not read.
5. Every such L clear, or no excluded label ever removed: the issue is admitted exactly as
   today. Any L not clear: HELD. The label events or the comments could not be read, in whole
   or in part: HELD, reported as could-not-check.
6. HELD means no create plan and no reactivate plan this scan. The placeholder stays absent,
   or stays `status: done` where it lies. `planScan` adds a NOTICE and `planTranscribeScan` a
   `clauseSkip`, each naming the issue, the label, the removing actor, and that a comment from
   the ratifying identity on the issue releases it at the next scan. No new status value, no
   new exit code.

two readings of the ruling made here — each is a reviewer's call, and each is one line to
change:
- **"Non-human" is read as "not a declared human"** (rule step 3), which fails closed. The
  alternative, classifying by the forge's account type or by a login suffix, admits a machine
  user and breaks on the suffix mismatch above. Cost of this reading: a person who is not
  rostered and removes the label also causes a hold.
- **"A comment exists" is read as "created after the label's latest application"** (step 4).
  The literal reading, any comment by the ratifying identity ever, would let an issue that is
  labelled again for a second question be released by the answer to the first.

named risks:
- **Bot-login rendering.** The same App reaches this code as `slug`, `slug[bot]` or
  `app/slug` depending on the API that produced the event. Step 3 never looks at the login's
  shape, and Verify row 3 pins all three renderings as held.
- **Tool version skew.** A `deskread` without the new kind makes every label-event read an
  error, so every create and reactivate is held as could-not-check. That is the safe
  direction, and it stops the board from growing until both binaries are current.
- **Whole-history rule.** A label removed by automation long ago, with no later comment from
  the ratifying identity, holds at the issue's NEXT create or reactivate. Placeholders that
  are already live produce no plan and are not re-examined.
- **A very long timeline.** Past the forge read's page bound the read is an error and the
  issue stays held; the NOTICE says so.
- **Cost.** One label-event read per would-be create or reactivate, and one comment read only
  when step 4 is reached, repeated every scan for an issue that stays held.
- **What this cannot see.** Automation acting under a declared human's own credential is that
  human to the forge. No rule over actors can catch it.

Out of scope — open, NOT decided by this brief or by the ruling behind it:
1. A lane on the issue board for held issues. Until one exists the hold is visible only in
   the scan's NOTICE and skip output.
2. A durable record of holds beyond that output.
3. Narrowing the hold from the whole excluded set to the decision-owed labels only.
4. Re-examining or rewriting placeholders already on the board.
5. The un-block lane's own bot detection (`isBotComment`), which does read the login suffix.

Also unchanged: the excluded-label set, the trust gate on issue authors, `retire-label` and
`sweep` close-outs, the never-overwrite rule, placeholder gate derivation, and every existing
field of `LabelEvent`.

design-fit:
  owner: `statusgen/scanissues.go` — the scanner's admission decision (`planScan`), which `statusgen/transcribescan.go` mirrors
  contract: S-decision-acceptance — this is a second reader of "did the ratifying identity act on this issue"; it reuses that identity's existing id-pinned check and adds no new definition of it
  retires: []
  weight: verbs 0, flags 0, deskread kinds +1, `LabelEvent` fields +1, scanner refusals +1 (one hold, reported two ways), rule-text lines +1 (one sentence in the intake-desk skill)
  why-add: >-
    The hold goes INTO the owner's admission paths as one shared check, called from both scan
    functions, not a rule per path. The read needs a new `deskread` kind because statusgen has
    no other route to the forge and no existing kind returns label events. The actor id is a
    new field because a login alone cannot be pinned to an identity. Removal considered: the
    create and reactivate paths could not be merged to pay for this, since one writes a file
    and the other rewrites one. Reusing the trust read's blessing events instead of a
    label-event read was rejected: it answers "is the content blessed", not "who removed the
    label".

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Before starting, merge main and check whether #2377 or #2382 has landed; build on what is
  there, and do not re-do their edits.
- ONE decision function, pure, called by both scan functions through one injected checker. Do
  not write the rule twice.
- Classify the actor by login AND numeric id against `authorizedAuthorSet()` only. Never by
  `[bot]` suffix, `app/` prefix, account type, or the trusted-bot roster.
- Fail closed everywhere: an unreadable or partial label-event read, an unreadable comment
  read, an event with no actor or id 0, an unparsable timestamp. None of these admits.
- `LabelEvent` changes by ADDING a field. Do not rename or re-type an existing one, and do not
  change what any of its current readers sees.
- Do not add a status value, an exit code, a flag, or a label. Do not change the excluded set.
- Do not edit the assertions of the existing reactivate and trust-gate tests; they gain the
  new checker argument and nothing else.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Fail-first: write the tests in steps 6–8 before steps 4–5, run Verify row 4, and paste the
   red output (a `reactivate` plan produced for a bot-removed label) into the PR body under
   `## Fail-first`.
2. deskkit: add the actor's numeric id to `LabelEvent` and fill it in the GitHub timeline read
   and both GitLab label-event reads. Test `TestLabelEventsCarryActorID` (planned) in
   `tools/desk/internal/deskkit/`: for each forge, a labelled and an unlabelled event decode
   with the actor's login and id, and an event with no actor decodes with id 0.
3. deskread: add the per-issue kind `label-events` (planned), addressed like `comments`, using
   `ListIssueLabelEvents`. Each event carries the label name, whether it is a removal, its
   time, and the actor's login and id. A read error for one issue lands in `partial` like the
   other per-issue kinds. Tests `TestDeskreadLabelEventsRoundTrip` (planned) and
   `TestDeskreadLabelEventsUnreadable` (planned) in `tools/desk/cmd/deskread/`.
4. statusgen reader: `IssueLabelEvents` (planned) on `deskreadReader`, through `readItem`, so
   a partial, empty or malformed envelope is an error. Test
   `TestIssueLabelEventsUnreadShapes` (planned), with a stub `deskread` the way
   `statusgen/scanissues_nativeread_test.go` does: a good envelope decodes; an envelope with
   the issue in `partial`, one with no item, a non-zero exit, and an envelope from a
   `deskread` that rejects the kind each return an error.
5. The decision, in `statusgen/labelremovalhold.go` (planned): a pure function over the
   excluded set, the label events and the comments that implements rule steps 1–5 and returns
   clear, or held with the label and actor. The default checker reads label events first and
   comments only when step 4 is reached. Add the checker as an injected parameter of
   `planScan` and `planTranscribeScan`, beside the bless checker, and call it on all three
   paths in each: on the create path AFTER the trust gate passes, so a quarantined issue costs
   no extra read. Emit the NOTICE or `clauseSkip` of rule step 6. Wire the default in
   `statusgen/main.go`.
6. `TestLabelRemovalHoldDecision` (planned), a table over the pure function. Each case names
   the label events, the comments and the expected result:
   - removed by an App, no comment → held
   - removed by the ratifying identity → clear; removed by another rostered human → clear
   - removed by an App, ratifying-identity comment after the application → clear, whether the
     comment came before or after the removal
   - removed by an App, the only ratifying-identity comment predates a later re-application →
     held
   - removed by an App, comment by the ratifying login with a DIFFERENT id → held; comment by
     another rostered human → held
   - the removing actor is a trusted desk identity → held
   - the removing actor has the ratifying login and id 0, or a different id → held
   - label never removed, and label never present → clear
   - two excluded labels, one clear and one not → held, naming the one that is not
   - the log's last event for the label is an application → held
   - an unparsable event or comment time → held
7. `TestLabelRemovalHoldBotRenderings` (planned): one App id rendered `slug`, `slug[bot]` and
   `app/slug` as the removing actor, no comment → held three times; the same three with the
   id of a rostered human whose login differs → held.
8. Scan-level tests, each on a temp root with `fixtureLister` and a fixture checker:
   - `TestScanLabelRemovalHold` (planned), through `planScan`, for each of the three paths:
     held → no create plan and no `reactivate` close-out, and one NOTICE naming the issue and
     label; clear → the same plan as today.
   - `TestScanLabelRemovalHoldFailsClosed` (planned): the checker returns an error on each
     path → no plan, and a could-not-check NOTICE.
   - `TestScanLabelRemovalHoldLeavesPlaceholderDone` (planned): a root placeholder at
     `status: done`, a held issue; plan, apply every plan produced, re-parse the stream — the
     placeholder still reads `done`. Then the checker returns clear, and the same sequence
     leaves it `todo`.
   - `TestTranscribeScanLabelRemovalHold` (planned): the three paths through
     `planTranscribeScan`, held, clear and error, asserting the `clauseSkip` on held and
     error.
9. Pass a clear-everything checker to every existing `planScan` and `planTranscribeScan` call
   in the tests. Their assertions stay as they are.
10. In the intake-desk skill, where it says a scanner `reactivate` sets a parked row `todo`,
    add ONE sentence: when the excluded label was removed by anything other than a rostered
    human, the scanner holds the item until the ratifying identity has commented on the issue
    since the label was applied. Leave every generated block alone.
11. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)
Every row that runs a named test anchors its selector, writes the output to a file and asserts
that test's `--- PASS:` line, so a missing or renamed test fails the row.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./... && cd ../tools/desk && go build ./... && go vet ./...` | exit 0 in both modules, so the added `LabelEvent` field broke none of its current readers | check:ci |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelRemovalHoldDecision$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestLabelRemovalHoldDecision' "${TMPDIR:-/tmp}/sg17-r2.out"` | exit 0; an App removal with no ratifying-identity comment is held; removal by the ratifying identity or a rostered human is clear; an App removal with a ratifying-identity comment after the application is clear; a comment that predates a re-application, a comment under the ratifying login with another id, and a removal by a trusted desk identity are all held. Mutation: with the declared-human check replaced by "login does not end in `[bot]`" the row exits 1 on the trusted-desk-identity and id-0 cases; with the after-application comparison dropped it exits 1 on the re-application case | check:ci +mutation |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestLabelRemovalHoldBotRenderings$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestLabelRemovalHoldBotRenderings' "${TMPDIR:-/tmp}/sg17-r3.out"` | exit 0; the same App rendered as the bare slug, with the `[bot]` suffix and with the `app/` prefix is held in all three forms | check:ci |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanLabelRemovalHold$' -v . > "${TMPDIR:-/tmp}/sg17-r4.out" 2>&1 && grep -F -e '--- PASS: TestScanLabelRemovalHold' "${TMPDIR:-/tmp}/sg17-r4.out"` | exit 0; through `planScan`, a held issue gets no create plan on the create path and no `reactivate` close-out on the root and archive paths, with one NOTICE each; a clear issue gets the plan it gets today. Mutation: with the checker call removed from any one of the three paths the row exits 1 on that path | check:ci +mutation |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanLabelRemovalHoldFailsClosed$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanLabelRemovalHoldFailsClosed' "${TMPDIR:-/tmp}/sg17-r5.out"` | exit 0; when the label-removal check cannot be read, no path produces a plan and each reports could-not-check. This is the negative path: the lower layer refuses with nothing above it deciding | check:ci |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanLabelRemovalHoldLeavesPlaceholderDone$' -v . > "${TMPDIR:-/tmp}/sg17-r6.out" 2>&1 && grep -F -e '--- PASS: TestScanLabelRemovalHoldLeavesPlaceholderDone' "${TMPDIR:-/tmp}/sg17-r6.out"` | exit 0; plan, apply and re-parse: the held issue's placeholder still reads `done` and is not dispatchable, and once the check clears the same sequence leaves it `todo` | check:ci +flow |
| 7 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestTranscribeScanLabelRemovalHold$' -v . > "${TMPDIR:-/tmp}/sg17-r7.out" 2>&1 && grep -F -e '--- PASS: TestTranscribeScanLabelRemovalHold' "${TMPDIR:-/tmp}/sg17-r7.out"` | exit 0; the second scanner holds, admits and fails closed on the same three paths, and records a `clauseSkip` for each hold | check:ci |
| 8 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestIssueLabelEventsUnreadShapes$' -v . > "${TMPDIR:-/tmp}/sg17-r8.out" 2>&1 && grep -F -e '--- PASS: TestIssueLabelEventsUnreadShapes' "${TMPDIR:-/tmp}/sg17-r8.out"` | exit 0; a good envelope decodes with login and id; a partial envelope, an empty one, a failed run and a `deskread` that rejects the kind are each an error, never an empty event list | check:ci |
| 9 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestCloseOutReactivateAfterLabelRemoved$' -v . > "${TMPDIR:-/tmp}/sg17-r9a.out" 2>&1 && grep -F -e '--- PASS: TestCloseOutReactivateAfterLabelRemoved' "${TMPDIR:-/tmp}/sg17-r9a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestCloseOutReactivateFromArchive$' -v . > "${TMPDIR:-/tmp}/sg17-r9b.out" 2>&1 && grep -F -e '--- PASS: TestCloseOutReactivateFromArchive' "${TMPDIR:-/tmp}/sg17-r9b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanTrustGateUnverifiableFailsClosed$' -v . > "${TMPDIR:-/tmp}/sg17-r9c.out" 2>&1 && grep -F -e '--- PASS: TestScanTrustGateUnverifiableFailsClosed' "${TMPDIR:-/tmp}/sg17-r9c.out"` | exit 0; three tests that exist today pass with their assertions unedited: with the check clear, a removed label still reactivates from the root and from the archive, and the trust gate still fails closed on its own | check:ci |
| 10 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | exit 0; the whole package passes | check:ci |
| 11 | `cd tools/desk && go test -count=1 -timeout 300s -run '^TestLabelEventsCarryActorID$' -v ./internal/deskkit/ > "${TMPDIR:-/tmp}/sg17-r11.out" 2>&1 && grep -F -e '--- PASS: TestLabelEventsCarryActorID' "${TMPDIR:-/tmp}/sg17-r11.out"` | exit 0; on both forges a labelled and an unlabelled event carry the actor's login and numeric id, and an event with no actor carries id 0 | check:ci |
| 12 | `cd tools/desk && go test -count=1 -timeout 300s -run '^TestDeskreadLabelEventsRoundTrip$' -v ./cmd/deskread/ > "${TMPDIR:-/tmp}/sg17-r12a.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadLabelEventsRoundTrip' "${TMPDIR:-/tmp}/sg17-r12a.out" && go test -count=1 -timeout 300s -run '^TestDeskreadLabelEventsUnreadable$' -v ./cmd/deskread/ > "${TMPDIR:-/tmp}/sg17-r12b.out" 2>&1 && grep -F -e '--- PASS: TestDeskreadLabelEventsUnreadable' "${TMPDIR:-/tmp}/sg17-r12b.out"` | exit 0; the new kind returns each event's label, removal flag, time, actor login and actor id, and an issue whose events cannot be read is reported in `partial`, not as an empty list | check:ci |
| 13 | `cd tools/desk && go test -count=1 -timeout 600s ./cmd/deskread/ ./internal/deskkit/` | exit 0; both packages pass, so the existing kinds and the existing label-event readers are unchanged | check:ci |
| 14 | `grep -n -F -e 'ratifying identity' plugins/assay/skills/intake-desk/SKILL.md && cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | exit 0; the line printed is the reactivate sentence stating the hold, and the skill still passes every skill check | check:ci +dereference |
| 15 | `cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --base "$(git merge-base refs/remotes/origin/main HEAD)"` | exit 0; run on the implementing branch before merge, the nine `follow-up` routings above are corroborated by its diff; the one `out-of-scope` entry (the issue board) is reported unchecked and is the reviewer's call: the board has no held-issue lane for this change to touch | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
