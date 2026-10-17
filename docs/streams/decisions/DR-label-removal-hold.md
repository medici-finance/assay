---
id: DR-label-removal-hold
date: "2026-10-09"
title: "Option 1 of #2414 is taken for the label-removal hold: hold on any non-human removal across all five excluded labels; a ratifying-identity comment made after the label was applied releases"
consequence: major  # the transcriber's classification, not part of the ruling; see "The consequence level"
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2414#issuecomment-6079818525"
alternatives:
  - "Option 2 of #2414 — 'same hold; only a comment after the removal releases' (#2414 `### Fork test`, second `option:` line). Its stated consequence: 'the person comments a second time whenever an agent removes a label after the ruling'. Ruled out: the recorded answer is 1; no rationale was recorded."
  - "Option 3 of #2414 — 'hold only for the two decision labels, with option 1's release' (#2414 `### Fork test`, third `option:` line). Its stated consequence: 'a mistaken automated removal of the other three system labels is not caught'. Ruled out: the recorded answer is 1; no rationale was recorded."
accepted:
  - "The consequence #2414 states for option 1, quoted whole: 'a comment from that person that is not a ruling also releases the hold' (#2414 `### Fork test`, first `option:` line)."
  - "The limit #2414 states for all three options: 'the hold is checked only when a work item would be created or brought back. A work item that already exists when the label goes on, and whose label comes off again before the next scan, stays in the queue and is never checked' (#2414 `## Why it needs an answer`)."
---

**This record transcribes a recorded human ruling. It is not an agent asserting sign-off.**
The human act is the maintainer's (`human:<name>`) comment on issue
[#2414](https://github.com/medici-finance/assay/issues/2414), posted under the maintainer's
own login and never a role App. This file copies that ruling into the register so that brief
statusgen/16 can cite it through `design:`. It mints no decision and supplies no rationale;
the ruling gave none.

## The recorded ruling

**The issue.** #2414, "statusgen/16: how far the label-removal hold reaches and what releases
it (Human decision)", was opened by the desk App on 2026-10-08T22:14:29Z. It was still open,
and still labelled `needs-decision`, when this record was written and last read for it, on
2026-10-11. The forge reports no edit history for its body.

**What was asked.** Brief statusgen/16 adds a hold: when an excluded system label is taken
off an issue by anything other than a rostered human author, no dispatchable work item is
created or brought back for that issue until the ratifying identity has commented on it. The
brief's Human decision section leaves one setting to the driver: how far the hold reaches and
what releases it. The issue states no default. Its three options under `### Fork test`,
verbatim:

> option: 1 — hold on any non-human removal across five system labels; a ratifying-identity
> comment after the label was applied releases | works-because: it is the brief as authored
> and reviewed, and it needs no second act when the person has already answered |
> consequence: a comment from that person that is not a ruling also releases the hold

> option: 2 — same hold; only a comment after the removal releases | works-because: the
> release test compares the comment time to the removal event, which the forge records |
> consequence: the person comments a second time whenever an agent removes a label after the
> ruling

> option: 3 — hold only for the two decision labels, with option 1's release |
> works-because: it narrows the label set in one place | consequence: a mistaken automated
> removal of the other three system labels is not caught

The same block says `default: none — the brief states that this must not be defaulted`. The
issue's `## Reply shape` asks for "one comment by the driver's own login naming the option and
the record: `<1|2|3> — DR-label-removal-hold`".

**The answer.** One comment by the maintainer, account type User, never edited (its
last-updated time equals its creation time):

- [comment 6079818525](https://github.com/medici-finance/assay/issues/2414#issuecomment-6079818525),
  2026-10-09T11:18:43Z. Its whole text is `1 — DR-label-removal-hold`.

It is the only comment on the issue. The record's `ruling:` link cites it, and `date:` is its
date (UTC).

## Reading, stated so it can be declined

The maintainer's comment says `1 — DR-label-removal-hold` and nothing else. This record reads
it as follows.

1. `1` is option 1 of #2414 as the issue wrote it, and as the brief's Human decision section
   wrote it: hold on any non-human removal of any of the five excluded labels (`verify-gate`,
   `live-verify`, `needs-decision`, `review-request`, `needs-human`); a ratifying-identity
   comment created strictly after the label's latest application releases.
2. Options 2 and 3 were offered in the same block and not taken.
3. The brief's rule, steps 1 to 6 of its Context, is written as option 1, and its Ground
   rules say what options 2 and 3 would have changed. Under option 1 nothing there changes.

What remains a reading is small: that a one-digit reply names the option carrying that number
in the issue body, in full and as written.

If any part of this reading is wrong, the remedy is to decline the pull request that adds this
record. Nothing here lands until the maintainer merges it.

## What the brief builds from it

Brief statusgen/16 (`docs/streams/statusgen/brief-16-label-removal-hold.md`) is the work. The
brief is the authority for the design. The ruling settles only the setting the Human decision
section asked about. The brief's Context lists the choices its author made where the ruling is
silent ("choices made here"); none of them is part of this ruling, and each can be declined at
the brief's gate:

- "non-human" is read as "not a declared human": an actor whose login and non-zero numeric id
  are in the authorized-author set; never classified by login shape or account type;
- an unnamed label event, a last-event application with the label absent, and a removal with
  no recorded application are each completed toward hold;
- a could-not-check hold is a NOTICE with the exit code unchanged;
- the issue board gets a superset test (`AWAIT`), not a second copy of the rule, and the forge
  gets a new read op.

## The consequence level

`consequence: major` is the transcriber's classification. It is not part of the ruling, which
states no level. The reason: the setting decides when a human decision is taken as given, so
if the design is wrong an automated actor can turn an undecided question into dispatchable work
with no human act, which is the act the hold exists to stop. It is not `critical` because the
change makes no forge write and only ever withholds work, and because a placeholder whose
derived gate is human still needs a human sign-off after dispatch. The maintainer can change
the level by declining this record or asking for the edit.

## The ruling link and the register's check

The register's online check (`README.md`, "How the approval is checked") passes a `ruling:`
link only when every one of its conditions holds. Each was read by hand against the forge on
2026-10-11:

- the link is in the required form and points into this repository — holds;
- the comment exists and sits on #2414 — holds;
- the comment was never edited (its last-updated time equals its creation time) — holds;
- its author is a User account, not a bot — holds;
- its author's login maps, through the repository's human-login map, to a human — holds. This
  record's `decided-by:` is the placeholder, so no particular name has to match;
- the comment's own text names this record's id, `DR-label-removal-hold`, as a whole token —
  holds;
- #2414 is this record's decision issue: its body carries a decision-gate marker for this
  record (`<!-- decision-gate: DR-label-removal-hold -->`) or for a brief whose `design:` cites
  it — **does not hold when last read.** The body carries no decision-gate marker. Until one is
  added, the check refuses this link as `unrelated-issue`. Adding it is an edit to the issue
  body, which is the maintainer's or the desk's to make, not this record's.

The online check itself (`statusgen --corroborate`) was not run for this record.

## What this record does NOT decide

- **Anything the brief lists as out of scope**: ageing or escalating a held board row, a
  durable record of holds, re-examining placeholders already on the board, the signed
  scan-delta create path, moving `--transcribe-scan`'s other reads off `gh`, and the un-block
  lane's own bot detection.
- **The excluded-label set** and **who may set or remove those labels.** Both are unchanged.
- **A later move to option 2 or 3.** Each was offered and not taken. Taking one later needs its
  own ruling.
- **Closing #2414 or removing its `needs-decision` label.** Separate acts.
- **The review and merge of the implementing change.** Those are separate acts.

And, per the register's own limits (`README.md`, "What a record does not establish"): a
checked `ruling:` link proves who ruled and where, not what the comment said. Whether this
record matches the ruling is the review gate's call on the pull request that adds it.
