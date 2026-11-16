---
id: DR-handoff-publish
date: "2026-11-15"
title: "The scan publishes recorded intake judgments beside their placeholders, and under any root not proven private it publishes only judgments whose every field has a closed shape (the brief's option 2); proposed, not ruled"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Publish the judgment's fields as recorded (the brief's option 1), relying on the reviewed intake change and the repository's outbound checks to catch prose before merge. Not chosen as the default. A judgment is pushed to a draft pull request branch before anyone reads it, and in a public repository text that has been pushed cannot be taken back. It stays open as the other side of the fork on #6364."
  - "Have `scanloop` write the companion file and the two placeholder keys itself, after `land --handoff`. Not chosen: the scanner renders, strips and archives the placeholder, so a second writer could disagree with it on the same file. The scanner stays the only writer and `scanloop` only supplies inputs."
  - "Under a public root, drop the free-text fields and publish the rest. Not chosen: the board refuses to schedule on a judgment whose evidence is empty, so dropping evidence publishes a judgment that admits nothing, and dropping it silently hides that the judgment was held back."
  - "A denylist that strips or refuses the named free-text fields (`evidence`, `required_action`, `wake_condition`). Not chosen, and replaced during review: any string field the list does not name can carry prose, so a denylist publishes by default whatever it forgot."
  - "A closed-shape allowlist with no check that it covers the record. Not chosen: a string field added to the record later is never inspected and is published as written, while the existing tests stay green (reproduced in review as 6362-c7)."
  - "Decide visibility from the issue's source repository. Not chosen: one scan writes placeholders for every repository in its scan list into one root, so the source repository's marking says nothing about where the companion is committed."
  - "Decide visibility from the roster's home repository. Not chosen: the home repository and the scan's commit target can differ, and a home repository marked private must not make a public commit target private."
  - "Trust the declared `--scan-target` name on its own. Not chosen: the carrier pushes to the repository the root checkout's origin names, and an explicit target is accepted on its shape alone, so the declared name and the push destination can disagree."
  - "Add a visibility flag, an environment variable or a second roster parse for the scan. Not chosen: `statusgen` already decides visibility fail-closed in one reader, and a second input for the same decision is a second path that can disagree with it."
accepted:
  - "Proposed, not ruled. `statusgen --scan-issues --judgments <log> --scan-target <owner/repo>` publishes, for each active placeholder it owns, the latest revision of the latest event the log holds for that issue, as `<placeholder stem>.handoff.json`, and sets `handoff:` and `handoff-opened:`. The pointer is written first and the companion second; an unchanged judgment produces no diff; reactivation and archive behave as before. `scanloop`'s scan lane passes the log and the drain's resolved target, never a visibility value."
  - "Proposed, not ruled. A root counts as private only when three things hold together: a target was supplied, the root checkout's origin push destination (`git remote get-url --push --all origin`, exactly one value) names that same repository, and the roster marks it private through the existing fail-closed `onBehalfOfPublicForm` reader. No target, an unreadable or unparseable origin, an origin naming another repository, and an unlisted or unmarked repository all count as public. A scan run by hand, in CI or with no roster publishes no free text."
  - "Proposed, not ruled. Under a public root a judgment is published only when every string field has a closed shape: the evidence is a typed reference of at most 96 characters, `required_action` and `wake_condition` are empty, each enumerated field holds an allowlisted value, identifiers in the verification block are opaque ids or slugs, its rows are capped at 64, and the event time parses as RFC 3339. Anything else is left out, named by field on stderr and counted as unpublished; the text stays in the local log."
  - "Proposed, not ruled. The allowlist carries an explicit list of the JSON paths it covers and compares that list with the record's type. If the shared record gains a field the list does not name, every judgment under a public root is held back, naming that field, until the field is listed and checked. A field added in the shared intake library therefore stops public publication until this package is changed."
  - "A judgment about work waiting on someone outside the pipeline is never published to a public repository, because its required action and wake condition are free text. That holds back nothing a worker could start: such a judgment does not make an issue schedulable. Intake must write evidence as typed references in public repositories."
  - "The issue wire carries no event time, so the scan cannot match a judgment against the issue's live event; it publishes the log's latest event and cannot detect a stale record. This substitution for the brief's event-time match is declared in the pull request's Desk-decided section."
  - "`scanloop land --exit bug` or `--exit placeholder` without `--handoff` is refused with exit 5, naming `--handoff` and `--handoff-reader`, before any record is written. `finding`, `needs-decision` and `rejected-watching` are unchanged. This is the scoping document's D2, ruled when the plan merged; it is recorded here because it ships in the same change, not because it is reopened."
  - "The judgment log is read without the audit lock, so an append in flight can leave a partial last line; it is skipped, named and counted, and picked up on the next pass. The scan-carrier lane discards the scan's unpublished-judgment notices (advisory 6362-a4, deferred to a follow-up)."
---

**Status:** draft

**Approval state: placeholder-unratified. This is not an approved design record.**
`decided-by` keeps the public placeholder and the record carries no `ruling:` link, because no
human has ruled. Approving it is the driver's act. Until a ruling comment that names
`DR-handoff-publish` exists and the record links it, read this as a proposal: the brief that
cites it (`intake-class-routing/02`, via `design: DR-handoff-publish`) does not pass the
design-approval gate (`spec/lifecycle-v1.md` §4.4). `statusgen --corroborate` is designed to
report `placeholder-unratified` on a pull request that edits this file
([register README](README.md)), but the CI step returns early on a pull request that touches no
`docs/streams/findings/` entry, so on such a pull request it does not run (the open gap
medici-finance/assay#6077, also medici-finance/assay#4853), and `statusgen --lint` checks only
that a cited record exists and is well formed, not that it is approved
(medici-finance/assay#6565). No check confirms this record's approval today.

## The decision, and the constraint behind it

[`DR-handoff-intake`](DR-handoff-intake.md) made an accountable handoff a precondition of
repair scheduling: the board admits a placeholder only through its `handoff:` and
`handoff-opened:` keys and the companion file they name. Nothing wrote them. A judgment
recorded with `scanloop land --handoff` stayed in a local log, and publishing it was a hand
edit nobody made. This record covers the step that design left open: who carries a recorded
judgment from the log into the committed tree, and what that copy may contain.

The carrier is the scanner, because it is already the only writer of the placeholder file.
`scanloop` supplies the log and the scan target to the scan it already runs and writes no
placeholder itself.

The constraint is that a committed companion is published wherever the root is pushed. A
judgment's evidence, required action and wake condition are free text, and in a public
repository pushed text cannot be taken back. So the scan decides, fail-closed, whether the
repository the companion is committed to is public. It treats every root as public unless it
can prove otherwise, and under a public root it publishes only judgments whose fields cannot
carry prose. The proof of privacy needs three agreeing facts (supplied target, matching push
destination, private marking) because each one alone was shown to be wrong in some
configuration: the source repository and the home repository are the wrong repository, and a
declared target is not where the bytes go.

The boundary is an allowlist, not a denylist, and it checks its own coverage. Review round 3
(6362-c7) showed that an allowlist which only inspects the fields it names publishes, as
written, any field the record gains later. The coverage comparison closes that: the list of
covered JSON paths is compared with the record's type, so an unlisted field holds back every
public-root judgment until someone lists and checks it. A positive control plants an unlisted
field at the top level and one inside the verification block and requires both to be named.

## Two layers

The change has two independent layers, so either can fail without disabling the other. The
scan publishes what was recorded (in `statusgen`), and the land verb refuses a judged exit
that has no judgment behind it (in `scanloop`). The refusal is the scoping document's D2.

## What this record does NOT decide

It does not decide the scrub boundary itself. Option 2 of the brief (typed references only,
option A on the decision issue) is implemented as the interim default because it is the
narrower behaviour and can be widened later. Whether the driver keeps it or chooses to publish
the fields as recorded is the open fork on #6364, and this record does not resolve it. If the
ruling chooses the other option, this record is amended or superseded, and the implementation
follows the ruling.

It does not add an event time to the issue wire or an exact event-time match; that is a
follow-up brief if the driver wants it. It does not change `spec/intake-handoff-v1.md`,
`spec/intake-exit-v1.md` or the board's admission rule. It does not claim the allowlist is
proof that no judgment carries sensitive content: a typed reference or an enumerated value can
still be the wrong one, which is the review gate's judgement.
