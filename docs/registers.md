# Registers — FINDINGS, INTAKE, REQUIREMENTS, RETRO

> **SUPERSEDED — the normative source is the versioned registers-v1 specification.**
> This page predates the per-entry-file register model and describes the retired
> single-file, numeric-counter dialect. Where the two disagree, the spec governs. The
> shared-conventions bullets below have been corrected in place; the format sections
> further down still show the legacy monolith shape and its `F-NN` headings. The full
> rewrite is tracked separately, as is the scaffold that
> still emits the legacy dialect.

Four append-only logs sit at `docs/streams/` alongside the stream directories. They are
the system's memory: what invalidated a plan (FINDINGS), what raw ideas are queued
(INTAKE), what the product was asked to do (REQUIREMENTS), and what each cadence retro
decided (RETRO — never implemented; there is no RETRO parser or entry directory). Each
rule below is stated with its reason.

For who may write these registers, how an alteration would be detected, and how long
they and the other artifacts in this repo are kept, see
[`docs/records-and-retention.md`](records-and-retention.md).

## Shared conventions

- **Append-only.** A new entry is a new file under `docs/streams/findings/` or
  `docs/streams/intake/`; existing entry files are never edited away. Reason: these logs
  are the audit trail — a silent edit erases the record of why a decision was made.
- **Slug IDs — contiguity is NOT enforced.** New entry IDs are slugs (`F-<slug>`,
  `I-<slug>`, 10–20 chars of `[a-z0-9-]`); `--lint` rejects a *new* numeric-counter entry
  and grandfathers landed ones. Slugs deliberately carry **no** contiguity guarantee —
  there is no sequence to have a gap in — because a counter reintroduces the write
  contention per-entry files exist to remove. Do not claim gap detection.
- **Deletion is caught by a tombstone check against history, not by a gap.** An entry file
  that exists in trunk history but is absent from the working tree is flagged, and its
  disappearance is visible in the change-set diff. That check is a history comparison, so
  it silently returns nothing outside a repository checkout or on a history-read error —
  treat a clean result as "no deletion observed".
- **Withdrawal is a tombstone, never a deletion.** To retract an entry, keep its file,
  flip its disposition/resolution, and let the body explain the withdrawal. Reason:
  deleting the file both loses the record and trips the tombstone check above.
- **Typed IDs.** Entries reference briefs and each other by typed ID (`stream/NN`,
  `F-<slug>`, `I-<slug>`, `REQ-<slug>`), never prose names.

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

## REQUIREMENTS — what the product was asked to do

What someone wanted the product to do, in that person's terms, with the criteria that
would settle whether the ask was met. It is the first link of the chain ask → work →
evidence → release; without it, acceptance criteria exist only inside one brief's Verify
table, where nothing can rank or roll them up. A requirement is not a brief: no wave, no
dependency edges, no DoD.

Entries are per-entry files under `docs/streams/requirements/<slug>.md` with slug IDs
(`REQ-<slug>`). The normative format, the ordered `impact` axis, the `proposed → accepted
→ satisfied → withdrawn` lifecycle and the reserved `satisfies:` citation on a brief are
specified in [`../spec/registers-v1.md`](../spec/registers-v1.md) §6 — unlike the two
sections below, this one has never had a legacy single-file dialect, so the spec is the
only description of it and this page does not restate the fields.

Two properties are worth stating here because they are the ones most easily misread:

- **The register records claims, not observations.** It says what was asked for and what
  was claimed to satisfy it. It does not establish that the product meets the ask; that
  lives in the cited brief's Verify rows and Evidence, recorded by someone who did not
  implement it. Do not present the register as coverage.
- **The brief citation is reserved, not gating.** A brief may carry
  `satisfies: ["REQ-<slug>"]`. The key is parsed, its refs are shape-checked, and the
  linter says out loud that it is reserved — but an absent citation is never flagged, a
  citation naming a requirement that does not exist is not an error, and nothing about
  traceability changes an exit code. Reason: the enforcing half costs a linter release and
  a re-pin in every consumer, and that is not spent before the schema has been used once
  in anger.

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
