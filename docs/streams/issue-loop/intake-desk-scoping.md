# Intake-desk — scoping (the generic front door)

**Date:** 2026-07-17 · **Origin:** human:<name> 2026-07-17 (reconceive the issue-desk as a generic intake-desk);
builds on [F-issue-desk-intake](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-17-issue-desk-becomes-generic-intake-desk.md) and
[F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md). **Decision owner:** human:<name>.

## Problem

The inbound loop already has its own desk and window (F-41; briefs 11/12, implemented) — but it is
framed and named around **GitHub issues**, with intake as a bolt-on second lane, and the intake half
(the `I-` register → tracked work) is still unbuilt (briefs 08/09 `todo`). human:<name>'s 2026-07-17 direction:
make it a **generic intake-desk** — the one front door that takes in *every* kind of inbound and
converts it into the right tracked artifact, and name it as the front-of-pipeline loop.

## What the intake-desk is

The **front-of-pipeline automation loop**, first of the four the coordinator `the-desk` orchestrates:

```
intake-desk  →  worker-desk  →  pr-review-desk  →  verify-desk
(triage inbound   (implement)     (review)          (verify)
 → tracked work)
                     ↑ the-desk coordinates all four
```

**Ingests (the front door):**
- **GitHub issues** — external + internal, work-shaped (the body IS the spec).
- **The intake register** — `I-` entries at `disposition: new` (idea-shaped, need judgment).
- **Any incoming request/idea** a human or loop drops at the front door.

**Converts each into exactly one tracked artifact (the five exits):**
1. **spec / brief** — `scoped → <stream>`; queued for author-brief (strong-tier authoring — the
   intake session QUEUES it, never cheap-authors it).
2. **bug / issue** — operational/defect-shaped → GitHub issue, label `bug` (per the bug-tracking rule);
   an intake entry records the number and flips `scoped → issue #NN`.
3. **finding** — knowledge that invalidates a brief → `F-NN` in the findings register.
4. **decision-needed** — a call that is human:<name>'s → the single `needs-decision` queue (issue-loop/06),
   never a second queue.
5. **rejected — <why>** / **watching** — explicit, with a reason (tombstone rules unchanged).

**Routing test** (unchanged, F-41): *hand it to a worker as-is → it's an issue (mechanical lane);
needs judgment first → it's intake (triage lane).* The two lanes are sequential stages of one funnel
(ideas → work), sharing the sensor, the owner window, and the decision queue.

## The fork human:<name> raised — one desk or two? → ONE generic desk, two lanes

human:<name> floated splitting intake into a *separate* desk (an `issue-desk` **and** an `intake-desk`). This
scoping keeps **one desk, two lanes**:

- **For one desk:** both lanes are inbound triage feeding the same funnel; they share the statusgen
  sensor/alarms, the owner window, and the single `needs-decision` queue; intake routes *into* the
  issue lane (`disposition: issue`), so they are stages, not peers. Two windows would duplicate the
  sensor and risk minting a second decision queue (the design forbids that).
- **When to revisit (a split is warranted only if):** the intake-triage judgment load — the
  strong-tier brief authoring it must *queue* — grows enough to starve the mechanical issue lane, or
  the two lanes diverge onto different cadences/owners. Track intake-debt drain rate (brief-07 alarm);
  if the intake lane consistently backs up while the issue lane keeps pace, split then.

Naming (per `../assay-toolkit/docs/skill-naming.md`): the one desk is **`intake-desk`** (function = the
inbound/triage phase), matching the loop taxonomy. `issue-loop` → `intake-desk` is a rename.

## Brief map

| Brief | State | Under the reconception |
|-------|-------|------------------------|
| 01 placeholder schema · 02 issue scanner | done | issue lane (mechanical) — unchanged |
| 03 await/unblock · 05 reviewer-deskflags | impl/done | issue lane — unchanged |
| 04 wire scanner + close-out | todo | issue lane; host window = intake-desk (F-41) |
| 06 human-decision issues | impl | the single decision queue (exit 4) — unchanged |
| 07 intake untriaged-age alarm | impl | the intake sensor — unchanged |
| 08 intake triage verbs + decision queue | todo | **intake lane** — the five-exit vocabulary |
| 09 wire intake triage into the window | todo | intake lane; host = intake-desk, not the-desk (F-41) |
| 11 desk skill + window | impl | **broadens** to the generic intake-desk (brief 13) |
| 12 self-dispatch | impl | issue lane self-dispatch — unchanged |
| **13 (new)** reconceive as generic intake-desk + rename | todo | this scoping's deliverable |

**Critical path for the reconception:** `08 (five-exit triage vocabulary)` → `13 (rename + broaden the
skill to the generic front door, wire the intake lane)`. Brief 13 also carries the `issue-loop →
intake-desk` rename (skill dir + stream + by-name refs), sequenced after in-flight PRs on the
issue-loop path land.

## Out of scope

- A second standing window (deferred per the fork decision; revisit criteria above).
- Merging the issue and intake registers (settled at methodology/23 — different substrates on purpose).
- Cheap-tier brief authoring inside triage (authoring stays strong-tier; triage only queues it).
