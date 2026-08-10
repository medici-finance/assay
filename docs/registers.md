# Registers — FINDINGS, INTAKE, RETRO

Three append-only logs sit at `docs/streams/` alongside the stream directories. They are
the system's memory: what invalidated a plan (FINDINGS), what raw ideas are queued
(INTAKE), and what each cadence retro decided (RETRO). Each rule below is stated with its
reason.

## Shared conventions

- **Append-only.** New entries go at the bottom; existing entries are never edited away.
  Reason: these logs are the audit trail — a silent edit erases the record of why a
  decision was made.
- **Sequence-contiguity is enforced.** `## F-NN`, `## I-NN`, `## R-NN` numbering must run
  without gaps or duplicates or `--lint` fails. Reason: a missing number is the visible
  signature of a deleted entry — the check makes deletion machine-visible instead of
  luck-visible. (This rule exists because a register deletion was once caught only by luck:
  a parallel implementation happened to carry a regression test.)
- **Withdrawal is a tombstone, never a deletion.** To retract an entry, keep its number,
  flip its disposition/resolution, and let the body explain the withdrawal. Reason:
  deleting a heading to make a number disappear both loses the record and trips the
  sequence-gap check.
- **Typed IDs.** Entries reference briefs and each other by typed ID (`stream/NN`, `F-NN`,
  `I-NN`), never prose names.

## FINDINGS — knowledge that invalidates a brief

New knowledge that makes an existing brief wrong or stale. The validator flags every
affected brief ⚠ stale-knowledge and excludes it from Next-up until the finding resolves.

Format:

```
## F-NN — YYYY-MM-DD — title
<body paragraph: what was found, the evidence, the fix direction>
Affects: <stream>/brief-<NN>[, ...]
Ack: YYYY-MM-DD <who>          # OPTIONAL — the desk's on-record judgment that the finding is real
Resolved: yes | no
```

- Resolve a finding by updating the affected brief/README to reflect it, then setting
  `Resolved: yes`. Reason: an unresolved finding is a standing alarm; leaving it open keeps
  the brief out of the work queue.
- **The `Ack:` line is a denial-of-service mitigation.** Filing a finding is unverified
  input — anyone can write a paragraph. Without an Ack, an unresolved finding never
  hard-errors an in-flight brief: it flags and excludes from Next-up plus a non-fatal
  notice. WITH a desk Ack against an `in-progress`/`implemented` brief, it becomes a hard
  error telling the operator to demote the brief (re-gate) or resolve the finding. Reason:
  an ungated demotion rule lets anyone drop a rival brief to `todo` by filing a paragraph.
- A malformed `Ack:` line is a hard error, not a silent skip. Reason: a typo'd gate is a
  disabled gate.

## INTAKE — the raw-idea front door

Brainstorms, "we should…", strategy fragments — while they are neither rejected nor scoped
into work. Intake entries are NOT briefs: no waves, no DoD. Brainstorming sessions end by
writing an intake entry, never by creating a new ad-hoc docs/ directory.

Format:

```
## I-NN — YYYY-MM-DD — title
<one paragraph>
Disposition: new | watching | scoped → <stream> | rejected — <why>
```

- An idea becomes work only by becoming a stream (scoping doc + brief authoring) or a brief
  in an existing stream; flip its disposition to `scoped → <stream>` then. Reason: the
  disposition is the single place that says whether an idea is live, parked, or dead — no
  guessing from surrounding prose.
- Withdrawal: keep the number, set `Disposition: rejected — <why>`, body explains. (Tombstone.)

## RETRO — cadence retrospective

An append-only log (weekly to start) whose inputs are **generated/logged only** — no prose
status. Reason: a retro that reads its own narrative measures the narrator, not the system.

Inputs walked each retro: STATUS totals delta since last entry; streams untouched since
last retro (the staleness list = the rabbit-hole report); gate yield (what reviewers
caught — recurring classes graduate to standing rules or FINDINGS); FINDINGS age (anything
stale-flagged > 1 retro gets scheduled or explicitly parked); INTAKE entries still `new`;
open tracker bugs; branch/worktree hygiene; knob tuning (Next-up weights/caps, priorities,
pause/unpause).

Format:

```
## R-NN — YYYY-MM-DD
<the checklist walked, with numbers>
<the one process change (or "none"), links>
```

- **One process change max per retro**, recorded as an intake/finding/brief and never
  enacted inline. A change must displace the current worst pain to get in; otherwise it
  waits. Reason: this is the anti-bloat guard — without a hard cap, every retro accretes
  rules until the process outweighs the work. (Watch the loophole: a self-granted
  "correction-class doesn't count against the budget" exemption is how the cap gets routed
  around — bound what qualifies as correction-class, or state the guard is advisory.)
