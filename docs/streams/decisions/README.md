# DECISIONS register — design-decision records

This directory is the DECISIONS register ([`../../../spec/registers-v1.md`](../../../spec/registers-v1.md)
§7): one design-decision record per file, `DR-<slug>.md`. It is a register, not a stream —
stream discovery skips it.

A **design-decision record** is what the lifecycle's design-approval gate
([`../../../spec/lifecycle-v1.md`](../../../spec/lifecycle-v1.md) §4.4) dereferences. A
risk-gated brief — `gate: human`, or any `risk` answer `yes` — may not move to
`in-progress` until it cites an approved record here with its `design:` frontmatter key.
The record captures the design at authoring time, so a wrong design is caught before it is
built rather than only when the finished diff reaches the review gate.

## What a record must carry

- `id` — a slug-form `DR-<slug>` (10–20 chars of `[a-z0-9-]`, starting and ending
  alphanumeric).
- `date` — ISO-8601 `YYYY-MM-DD`.
- `title` — one line: what was decided.
- `consequence` — the ordered severity axis (`minor` < `major` < `critical`), ranking the
  consequence if this design is wrong.
- `decided-by` — a `human:<name>` stamp. This is the design-approval authority; a model
  sign-off does not stand in for it. It names a real human (a name the human-login map
  resolves), or is the literal placeholder `human:<name>` when a real name cannot be
  written (a public repository) — and then the record needs a `ruling:` link.
- `ruling` — optional: the URL of the human's ruling comment on the record's decision
  issue, exactly `https://github.com/<owner>/<repo>/issues/<N>#issuecomment-<ID>`.
- `alternatives` — the paths not taken, each with why it was ruled out. A record with no
  alternatives records an outcome, not a decision.
- `accepted` — the consequences accepted by taking this path.

It is append-only and tombstoned (`registers-v1.md` §3.1, §3.3): to reverse a decision,
keep the file and explain the reversal in the body — never delete it.

## Template

Copy this into `docs/streams/decisions/DR-<slug>.md`:

```
---
id: DR-<slug>
date: "YYYY-MM-DD"
title: "<one line: what was decided>"
consequence: minor | major | critical
decided-by: "human:<name>"
ruling: "https://github.com/<owner>/<repo>/issues/<N>#issuecomment-<ID>"
alternatives:
  - "<a path not taken> — <why it was ruled out>"
  - "<another path> — <why it was ruled out>"
accepted:
  - "<a consequence accepted by taking this path>"
---

<body: the decision, the constraint behind it, and what is explicitly NOT decided>
```

## How the approval is checked

`statusgen --corroborate` gates every record a pull request adds or edits
([`registers-v1.md`](../../../spec/registers-v1.md) §7.5):

- **With a `ruling:` link**, it reads the comment through the forge API. It passes only when
  the comment exists, sits on the linked issue in this repository and was never edited, its
  author is not a bot, the author's login maps (through the human-login map) to a human —
  the one `decided-by` names, if it names one — the comment's own text names the record's
  `DR-<slug>` id, and the issue is this record's decision issue: its body carries
  `<!-- decision-gate: DR-<slug> -->` for this record, or the decision-gate marker of a
  brief whose `design:` cites it. Anything else fails with a named reason:
  `malformed-link`, `unresolvable-link`, `deleted-comment`, `edited-comment`, `bot-author`,
  `wrong-author`, `record-not-named`, `unrelated-issue`, `record-unreadable`. A ruling
  corroborates only the human who wrote it: any other real name in the same `decided-by`
  fails (`wrong-author`), so a record approved by several humans carries no `ruling:`
  link. To correct a ruling, the human posts a new comment and the record links that one.
- **Without one**, a real `decided-by` name is corroborated as any other `human:<name>`
  stamp (that human's approval on the pull request, or the closed decision issue), and the
  placeholder is a problem: `placeholder-unratified`.

So the order is: the human rules on the decision issue first, then the record lands with
its `ruling:` link. A record still awaiting its ruling fails the check on the pull request
that adds it — until the ruling exists it is not an approved record. Records already
merged are not re-checked until a pull request edits them, and `statusgen --lint` checks
only the link's shape.

## What a record does not establish

It records that the alternatives were weighed and names a human approver. It does not by
itself prove the approver differs from the brief's author (an attributed stamp, not a
checked identity boundary — `registers-v1.md` §7.4), nor that the chosen design was
correct — that is the review gate's judgement and then the change's own **validation**
([`../../validation.md`](../../validation.md)) after it lands. A checked `ruling:` link
proves who ruled and where, not what the comment said: whether the record matches the
ruling is also the review gate's call.
