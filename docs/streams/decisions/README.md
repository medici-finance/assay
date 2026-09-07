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
  sign-off does not stand in for it.
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
alternatives:
  - "<a path not taken> — <why it was ruled out>"
  - "<another path> — <why it was ruled out>"
accepted:
  - "<a consequence accepted by taking this path>"
---

<body: the decision, the constraint behind it, and what is explicitly NOT decided>
```

## What a record does not establish

It records that the alternatives were weighed and names a human approver. It does not by
itself prove the approver differs from the brief's author (an attributed stamp, not a
checked identity boundary — `registers-v1.md` §7.4), nor that the chosen design was
correct — that is the review gate's judgement and then the change's own **validation**
([`../../validation.md`](../../validation.md)) after it lands.
