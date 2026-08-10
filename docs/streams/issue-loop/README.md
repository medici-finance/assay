---
stream: issue-loop
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# Issue-Loop Stream → Intake-desk (generic front door)

> **Reconception (human:<name> 2026-07-17, [F-issue-desk-intake](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-17-issue-desk-becomes-generic-intake-desk.md)):**
> this stream/desk is the **generic intake-desk** — the front-of-pipeline automation loop that ingests
> ALL inbound (GitHub issues + the `I-` intake register + any incoming) and converts each into one of
> five exits: a **spec/brief**, a **bug/issue**, a **finding**, a **decision-needed**, or a reasoned
> **rejection/watch**. It is the first of the four loops `the-desk` coordinates:
> `intake-desk → worker-desk → pr-review-desk → verify-desk`. Design + the resolved one-vs-two-desk
> fork (→ one desk, two lanes): [intake-desk-scoping.md](./intake-desk-scoping.md). The rename
> `issue-loop → intake-desk` landed in brief 13; the stream directory keeps its name for history.

Make open GitHub issues a **first-class part of the work model** instead of a backlog that
rots outside it. Today ~30 issues are open and only the ones the desk hand-converts to
briefs enter Next-up; the rest (the R-01 "~22 app-layer bugs unmapped" finding) are
invisible to dispatch. This stream builds the inbound loop I-25 designed: a scanner drops a
thin placeholder brief for every unhandled issue, placeholders flow through Next-up like any
brief, and the GitHub issue itself is the worker↔human conversation channel.

Origin: INTAKE I-25 (the inbound half of the verify-gate brainstorm).

> **Design revised 2026-07-16 ([F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md)):
> the inbound loop now has its OWN standing desk + window** (`../oit/.claude/skills/intake-desk/SKILL.md`,
> brief-11), owning both lanes. The original principle below — *"no fourth standing window,"* the loop
> distributed across pr-review-desk's monitor (scanner) and the-desk's cadence (intake triage) — is
> superseded. The mechanics are unchanged (two lanes, single decision queue, routing test); only the
> host window moved. Briefs **04** (scanner host) and **09** (intake host) carry stale host-window
> wording until re-picked-up.

Original design principle (human:<name>, 2026-07-12, **superseded** — kept for provenance): *no fourth
standing window — the loop is distributed across machinery that already exists (scanner on
pr-review-desk's monitor cadence, dispatch on Next-up, await/unblock on statusgen). Maintenance
owner: the process desk.*

Briefs 07–09 add the **sibling front-door loop** (INTAKE I-intake-desk): the same
disease/cure shape applied to the intake register — untriaged `disposition: new` entries are
invisible to scoping the way unhandled issues were invisible to dispatch. Sensor (07) +
triage verbs/decision queue (08) + wiring (09, now into the **intake-desk** window per F-41,
not the-desk); same machinery family, same statusgen surface.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Placeholder-brief schema + statusgen recognizes `issue-<NN>` briefs](./brief-01-placeholder-schema.md) | 0 | M | done | 2026-07-11 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 02 | [`statusgen --scan-issues` — emit placeholders for unhandled open issues](./brief-02-issue-scanner.md) | 1 | M | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 03 | [Await/unblock state — worker questions park on the issue, comments resume](./brief-03-await-unblock-state.md) | 1 | M | done | 2026-07-16 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 04 | [Wire the scanner into pr-review-desk + close-out semantics](./brief-04-wire-and-closeout.md) | 2 | S | done | 2026-07-16 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 05 | [Reviewer desk-flags become issues — non-blocking review residuals feed the loop](./brief-05-reviewer-deskflags.md) | 2 | M | done | 2026-07-15 reviewer-app[bot] | 2026-07-15 reviewer-app[bot] |
| 06 | [Human-decision issues — any human gate surfaces as a labeled decision issue (situation + pros/cons)](./brief-06-human-decision-issues.md) | 2 | M | done | 2026-07-16 glm-5.2-verifier | 2026-07-15 reviewer-app[bot] |
| 07 | [Intake untriaged-age alarm — NOTICE past 3 days + intake-debt board line](./brief-07-intake-untriaged-alarm.md) | 3 | S | done | 2026-07-16 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 08 | [Intake triage verbs + decision queue — four exits; decision-needed routes into the single needs-decision queue](./brief-08-intake-triage-verbs-decision-queue.md) | 3 | M | done | 2026-07-16 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 09 | [Wire intake triage into the-desk — triage at boot + on the alarm; no fourth window](./brief-09-intake-triage-wiring.md) | 4 | S | verified | 2026-07-20 glm-5.2-verifier | — |
| 10 | [Reviewed issue-close via `bugs/` carrier + daily `bugs-gc` prune](./brief-10-reviewed-issue-close.md) | 2 | M | done | 2026-07-17 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 11 | [Issue-loop desk skill + dedicated window — the inbound twin of pr-review-desk](./brief-11-issue-loop-desk-skill.md) | 4 | M | done | 2026-07-17 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 12 | [Issue desk self-dispatch — fans out its own issue-placeholders (claims-locked); batch-fanout skips them](./brief-12-issue-desk-self-dispatch.md) | 5 | S | done | 2026-07-17 glm-5.2-verifier | 2026-07-16 reviewer-app[bot] |
| 13 | [Reconceive the issue-desk as the generic intake-desk — rename + five-exit front door](./brief-13-generic-intake-desk-rename.md) | 5 | M | done | 2026-07-20 glm-5.2-verifier | 2026-07-20 assay-reviewer-app[bot] |
| 14 | [Desk emits briefs, not PRs — dispatch returns to batch-fanout; closed-issue briefs archive to per-stream done/](./brief-14-desk-emits-briefs-done-archive.md) | 6 | L | todo | — | — |
| 15 | [Intake directory split by disposition — `ls` is the triage board; a move is a transition, id is identity](./brief-15-intake-directory-split.md) | 6 | M | todo | — | — |

## Dependency waves

```
Wave 0: [01]
Wave 1: [02, 03] ← 01
Wave 2: [04 ← 02, 03; 05 ← 02; 06 ← 01; 10 ← methodology-metrics/22 (cross-stream)]
Wave 3: [07; 08 ← 06]
Wave 4: [09 ← 07, 08; 11 ← 02, 07]
Wave 5: [12 ← 11]
Wave 6: [14 ← 10, 12; 15 ← 08 (retrofit: the disposition-subdir layout lands under 08's verbs; cross-repo — statusgen change releases from assay-toolkit first)]
```

Wave 6 (brief-14, human:<name> 2026-07-20) restructures the desk's OUTPUT: it emits briefs only —
self-dispatch (12) is superseded (dispatch returns to batch-fanout), the `bugs/` close-PR (10)
is authored by pipeline workers off a `close-candidate` row, and RETIRE moves closed-issue
placeholders into a per-stream `done/` archive. See
F-desk-emits-briefs; the brief-13
rename is orthogonal.

10 is independent of the inbound-loop chain; its only dependency is cross-stream
(methodology-metrics/22, the daily-harvest collector it wires the `bugs-gc` prune into).
It can be picked up any time mm/22 is at/near merge — see brief-10 Ground rules for the
"mm/22 not yet merged" staging path.

Critical path: **01 → 06 → 08 → 09** — **essentially landed**: 01–04, 06–08 and 10–12 are verified/done
(05 withdrawn); only **09** remains `implemented`, awaiting verification of its F-41-re-targeted Verify
table. (The design constraint still holds for provenance: 06's `needs-decision` queue had to exist
before 08 could route `decision-needed` into a SINGLE queue — it does now.) The **F-41 re-targets of
brief-04 and brief-09 are DONE** (2026-07-17): both now point their host-window wording at the
issue-loop desk skill (`../oit/.claude/skills/intake-desk/SKILL.md`) instead of pr-review-desk/the-desk, and
brief-09's Verify row 2 re-targets to the in-repo skill that carries the triage lane — un-orphaning the
step its earlier VERIFY: FAIL flagged. **brief-10** was built by PR #567 (`feat/issue-loop-10-bugs-gc`:
the `bugs/` carrier + `tools/bugs-gc` + its wire-in to `daily-harvest.yml` + the CLAUDE.md close-PR
flow); this PR's earlier `todo` → `implemented` correction has since been superseded by its
verification to `done` (2026-07-17, merged from main). The only remaining item is the future read-only
`issueboard.go` board tool (un-numbered). 07 had no dependencies and ran in parallel.

## One inbound loop, two lanes (human:<name> + desk, 2026-07-12)

The issue lane (01–06) and the intake lane (07–09) are ONE loop family — both are inbound
triage: unstructured inbound thing → classified → enters the work model. They share the
sensor host (statusgen alarms), the owner (**the intake-desk + window**, brief-11 / F-41 —
was "the coordinate loop; no new window" before the 2026-07-16 revision), the decision
queue (a `decision-needed` intake entry surfaces through 06's `needs-decision` issues —
never a second queue), and this stream. They stay two LANES, not one mechanism, because:

1. **Content type (load-bearing):** an issue is WORK-shaped — the body is the spec, a worker
   can take it as-is, so the exit is mechanical (scanner → placeholder → dispatch). An
   intake entry is IDEA-shaped — it needs judgment before it is work (scoping, strong-tier
   authoring, or a strategy decision), so the exit is triage. You cannot automate the second
   the way you automate the first.
2. **Substrate (chosen deliberately):** issues are GitHub platform objects — labels,
   conversation-native (worker questions park there), anyone can file. Intake entries are
   in-git register files — lint-checked, typed-linked, tombstoned, offline-readable, and
   portable with the repo (the methodology self-containment goal: an adopter's front door
   must not require our GitHub). Merging the registers would re-open the issues-vs-registers
   analysis settled at methodology/23 and lose exactly the properties registers were picked
   for.
3. **Flow direction:** intake routes INTO the issue lane (`disposition: issue`), never the
   reverse — they are sequential stages of one funnel (ideas → work), not duplicates.

**Routing test:** *if you could hand it to a worker as-is, it's an issue; if it needs
judgment first, it's intake.*

For the loops-reference doc and any exec surface, present the pair as a SINGLE "inbound
loop" with two lanes — one story, one triage cadence, one decision queue (I-loops-reference
carries this framing for the eventual loops.md).

**Loop-owner conventions:** intake-triage loop = sensor (07) + verbs (08) + wiring (09) — the
front-door drain, eighth loop (I-loops-reference). **The intake-desk owns the front door**
(brief-11 / F-41, revised 2026-07-16 — was "the-desk" before the inbound loop got its own window):
it triages the `disposition: new` set at boot and whenever the intake-debt alarm fires.
Triage decides ROUTE only; authoring is queued, never done inline at cheap tier.

## Shared conventions (inherited by every brief)

- **The GitHub issue body IS the spec** — a placeholder never duplicates it; it points
  (`issue: <NN>`, `repo: <owner/name>`) and carries only the scheduling metadata Next-up
  needs. The issue is also the conversation channel (brief 03).
- **System-emitted labels are excluded** from scanning: `verify-gate`, `live-verify`, and `needs-decision`
  (I-22) issues are closeable *states*, not work — a placeholder for them would be noise.
- Offline discipline: the scanner is desk/CI-adjacent (needs `gh`); it MUST NOT become a
  hard dependency of the offline `--lint` gate (the registers-in-git principle).
- **Intake sensor** (brief-07): the untriaged-age alarm is offline/deterministic — keys on
  `disposition: new` only (parsed from YAML frontmatter, not body-prose grep), computes
  age from the in-git `date:` field + the system clock, and never touches the network.
  `scoped`/`rejected`/`watching`/`adopted`/`decision-needed` — and any other explicit
  non-`new` disposition — are triaged states and do NOT count.
- **Decision-issue template** (brief-06): every `needs-decision` issue is **self-contained**, readable without opening the repo:
  - **Situation** (prose): what the brief is doing, what fork was hit, why
    it can't proceed on a default.
  - **Options** (2-4), each with pros/cons at the mm/12 trade-off bar
    ("why we'd want it / what it limits or costs"), plus a recommendation.
  - **What happens on each answer**: which brief rows/PRs move, what gets
    unblocked.
  - **Links**: brief typed-ID, PR if any, related findings/issues.
  - The decider is the human; closing the issue with the chosen option
    stated is the decision record. Only a verified human account is
    honored (#237).

## Triage verbs (issue-loop/08)

Every intake entry leaves the `disposition: new` queue as exactly ONE of these four exits. The
verb is spelled in the entry's frontmatter `disposition:` field (and rendered in the
`INTAKE.md` view generated by statusgen):

1. **`scoped → <stream>`** — becomes a brief via the author-brief flow. **Tier gate: triage
   only QUEUES authoring** — brief authoring is design-tier work (author-brief model-tier
   gate); a cheap-tier triage session never authors inline.
2. **`scoped → issue #NN`** — operational/bug-shaped work routed to a GitHub issue (label
   `bug` when bug-shaped). The entry records the issue number.
3. **`decision-needed`** — a human call that routes into the **single decision queue**
   (issue-loop/06's `needs-decision` label + template). Flipping to `decision-needed`
   REQUIRES filing (or already having) a `needs-decision` issue and recording it in
   the entry's frontmatter as `decision-issue: <NN>`. The intake `INTAKE.md` view
   renders these entries in a distinct "Decision queue — waiting on a human" section,
   but that section is a **pointer** into the 06 needs-decision queue — never a second
   queue. One place human:<name> answers decisions.
4. **`rejected — <why>` / `watching`** — existing vocabulary, explicit reason required for
   `rejected`.

**Single-queue rule**: `decision-needed` is the ONLY triage exit that routes to the human
decision queue. That queue IS the `needs-decision` label + template from issue-loop/06. The
intake view's "Decision queue" section points to those issues; it renders no independent
decision state. If you find yourself creating two places human:<name> needs to answer, you've split
the queue — go back and route through 06.
