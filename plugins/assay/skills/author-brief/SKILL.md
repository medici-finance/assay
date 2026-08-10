---
name: author-brief
description: >-
  Author and structure a phase plan as self-contained briefs organized into dependency
  waves, with a critical path. Use this WHENEVER the user wants to plan multi-step work and split it
  up: "break this into briefs and waves", "create a phase plan", "do for phase N what we did for
  phase 1", "this brief is too big, split it", "turn this into tasks with dependencies", or when
  scoping a body of work that several people/agents will pick up. Encodes a brief format
  (Wave / Depends-on / Unblocks / Effort / Deliverables / DoD), a phase-README status-table +
  critical-path + dependency-wave layout, and the lesson that you must verify the REAL blocker at the
  head of the critical path rather than assume it. Prefer this over inventing an ad-hoc plan shape.
---

# Author Brief

## Model-tier gate (check BEFORE anything else)

Brief authoring and work decomposition are **design-tier work**: this is where risk gates are
derived, dependencies mapped, and the critical-path head verified — errors made here COMPOUND
through every downstream implementer and review cycle, unlike implementation errors, which gates
catch. Therefore:

- **If you are a fast/cheap-tier model** (a haiku-class or equivalent economy tier): **STOP.**
  Report which model you are and tell the user to switch to a strong-tier session or escalate
  the authoring to one. **Do not author anyway** — not for "just a small brief set", not as a
  "draft for review", not because the user seems in a hurry. A cheap draft anchors the strong
  model that reviews it; refusing is the cheaper path.
- **If you are dispatching this work to a subagent**: use the most capable model available for
  authoring/decomposition — never an economy tier. (Implementation is the inverse: cheap
  implementers behind strong gates.)

Planning multi-step work follows a consistent shape: a phase README that holds a **status table**, a
**critical path**, and **dependency waves**, plus one **self-contained brief** per unit of work. The
point of the structure is that any single agent can pick up a brief and execute it without re-reading
the whole plan, and the reader can see at a glance what blocks what. **Match the phase docs already in
the repo — don't invent a new format.** If a project-level `author-brief` skill exists in the current
repo (`.claude/skills/author-brief/SKILL.md`), read it after this skill and apply its specifics —
paths, docs-regen commands, and local examples layer on top of this methodology.

## The most important judgment call: find the REAL head of the critical path

The critical path is the chain that, if it slips, slips everything. The recurring failure is putting
the wrong item at the head — sequencing work behind a step that is itself blocked. (Concrete shape of
the failure: a plan listed Brief B → Brief A as the unblock, but both were dead-on-arrival until an
upstream bug — tracked elsewhere — was fixed; that bug was the true head and wasn't even in the path.)
Before you finalize the order: for each "smallest unblocking move," **verify it actually works** —
check the code/config/bug tracker, don't assume. If a proposed first step depends on something
unproven, that something is the real head. Say so explicitly.

## Brief structure (one file per unit of work)

`docs/<phase>/brief-<NN>-<slug>.md` (or whatever your repo's convention is — match existing phase docs).
Conceptually every brief carries the same handful of things: **Wave, Depends on, Unblocks, Effort**
(the scheduling metadata), **Context** (why + the exact facts needed), **Read first** (required
reading), **Deliverables** (concrete outputs), the **Interface contract** (the seam other briefs rely
on), and a **Definition of Done**.

Older brief sets narrated all of this in prose — a header line plus paragraphs. That drifts in two
specific ways once a plan has more than a few briefs: dependencies written as free-text arrows that
don't survive a rename or a re-numbering, and DoD items with no runnable check attached, so "done" is
whatever the implementer felt like asserting. **The template below is the current standard — use it
for every new brief.** It carries the same concepts as data (YAML frontmatter fields, tables) instead
of only prose, so a script — or the next agent, or you in six months — can rely on the structure
without re-reading the whole plan to extract it.

### Brief file template (data-first)

```markdown
---
brief: <stream>/<NN>                # typed ID, matches stream README table row
title: <one line>
why: <prose>                        # REQUIRED for every NEW brief — one to three lines a non-engineer
                                    # could read and justify the work from. Not the WHAT (that's title
                                    # + Task), but WHY it matters: what problem, whose pain, what the
                                    # outcome unlocks. Quality bar: "all 22 pages are eagerly imported
                                    # into a single JS chunk, so every user pays a multi-MB first load;
                                    # splitting cuts the initial payload ~5×." (frontend/05 exemplar).
                                    # Omit only on legacy briefs being backfilled later — statusgen
                                    # NOTICEs a missing why: (non-fatal this phase; hard error once
                                    # backfill lands — same pattern as gate-why, methodology/27).
wave: <int>
depends: []                        # typed IDs only: ["<stream>/<NN>", ...] — NEVER prose arrows
unblocks: []                       # typed IDs
effort: S | M | L
gate: model | human                # from the four risk questions below
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []                         # GH issue numbers this brief closes
schema: brief-v1
authored: <YYYY-MM-DD> by <who/session>
sources: []                        # provenance: scoping doc, finding IDs, intake IDs this derives from.
                                    # F-<slug>/I-<slug> references should be links: "[F-ws-token-expiry](../findings/2026-07-09-websocket-token-expiry.md)"
gate-why: <prose>                  # REQUIRED when gate: human OR any risk answer is yes — what about THIS
                                   # brief makes it risky and what the human is confirming (a line or two).
                                   # Omit only when gate: model and all four risk answers are no.
exec-tier: any | strong             # OPTIONAL (absent = any) — minimum execution-model tier.
                                    # DERIVED from three complexity questions (see rule 9); any yes → strong.
exec-tier-why: <one line>           # Recommended when exec-tier: strong — which question(s) it answered yes.
consumers: []                       # OPTIONAL — list of consumer sites when this brief changes a shared
                                    # value. Each entry: "<path>: fixed-here | follow-up <stream/NN> |
                                    # out-of-scope (<why>)". Absent = no shared value changed (the default).
                                    # See rule 6. Authoring convention — no lint enforces it yet
                                    # (methodology-metrics/21; assay-toolkit#159).
---

# Brief <NN> — <title>

## Context
files: <exact paths the implementer touches>
out-of-repo files: <exact paths outside the repo (e.g. ~/.claude/skills/...), if any —
see rule 7; omit the line entirely when none>
facts: <the 3-5 project facts needed — key: value, no narrative. The implementer
must never need to explore the repo.>

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per
  the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
<what to build/change — explicit steps>

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | <literal command> | exit 0; output contains "<literal>" |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: <model|human> (from frontmatter). Reviewer records verdict + date in the
stream README table. Human gate is MANDATORY when any risk answer is yes.
```

Fill every frontmatter field — an empty `sources: []` or a missing `risk:` answer is not a shortcut,
it's a gap the reviewer will bounce back. `gate` is derived, not chosen: if any of the four risk
questions is `yes`, `gate` must be `human`; only when all four are `no` may `gate` be `model`.

### Rules behind the template

1. **Load-bearing facts live in islands, not prose.** Frontmatter, tables, and Verify rows are the
   record; prose explains and motivates but never solely carries a fact a script or a reviewer needs.
2. **References are typed IDs** (`stream/NN`, `F-<slug>`, `I-<slug>`, legacy `F-NN`, `I-NN`, …) — no
   fuzzy names ("the pricing brief"), no arrow notation (`←12a`). A typed ID survives renumbering
   and greps cleanly. **Register IDs are letter-prefixed slugs (methodology/35):** `F-<slug>` for
   findings, `I-<slug>` for intake — the slug is 10–20 chars, `[a-z0-9-]`, derived from the title
   (example: `F-ws-token-expiry`, `I-model-mix-tiers`). Older entries use numeric IDs (`F-01`, `I-01`)
   — those are frozen legacy and remain valid; new entries must use the slug form. **Going
   forward, register references SHOULD be clickable links pointing at the per-entry
   file: `[F-ws-token-expiry](../findings/2026-07-09-websocket-token-expiry.md)`. The typed ID
   remains the primary key (link text); the link is for readers. The mapping is each entry file's
   frontmatter `id:` — never guess slugs. Bare (unlinked) IDs stay legal but are not the preferred
   form for new briefs.
3. **Verify rows must be runnable by someone who didn't do the work.** A DoD/Verify item with no
   literal command and no expected exit/output is not a DoD item — it's a hope.
4. **`gate: human` is mandatory when any risk answer is `yes`.** Record all four risk answers
   (regulatory, customer, irreversible, sensitive-data), not just the resulting gate — the answers are
   what a reviewer audits, the gate is just their conclusion. **A risk-gated brief (`gate: human` OR
   any risk answer `yes`) MUST carry a `gate-why`** — a line or two naming what about *this* brief
   trips the wire and what the human is actually confirming at the sign-off. A risk boolean with no
   rationale is an unfalsifiable assertion; the verify-gate card surfaces the `gate-why` verbatim.
   (statusgen NOTICEs a missing `gate-why` today; it becomes a hard lint error once the backfill lands.)
5. **Provenance is required.** A brief with an empty `sources:` — no scoping doc, no finding ID, no
   intake ID it derives from — is untraceable: no one can tell why the work exists or whether it's
   still needed.
6. **A brief that changes a SHARED VALUE must enumerate its consumers, and verify the FLOW — not just
   the site.** (Cost that forced this rule: a party-identity change was verified green at its own site
   — "the agent submits as its own party" — while a frontend fallback that assumed the *old* coupling
   silently broke the whole end-to-end flow; no `CollateralVault` was ever minted. The change was
   correct locally and broken across components.) A shared value is anything another component reads:
   a party/identity, an env-var NAME, a ConfigMap/Secret key, a field's MEANING, a wire/JSON format,
   or a default. The trigger is cross-component: when the brief's Context `files:` references multiple
   top-level directories (k8s/, daml/, frontend/, services/, docs/, docs-site/, scripts/, .github/,
   tools/) or its Task touches a shared-surface keyword (party, identity, env-var, ConfigMap, Secret,
   wire format, JSON format, a default).
   - **Enumerate consumers as a structured frontmatter field**:
     `consumers:` — an optional YAML list in the brief-v1 frontmatter (absent = the brief asserts it
     changes no shared value). Each entry is `<path-or-component>: <routing>` where routing is one of
     `fixed-here` / `follow-up <stream>/<NN>` / `out-of-scope (<why>)`. An unlisted consumer is how
     you strand an assumption. Project wrappers (this repo's `.claude/skills/author-brief/SKILL.md`)
     carry the full format spec. **No lint enforces this yet** — it is an authoring convention;
     the statusgen check is tracked as assay-toolkit#159 and reaches a consuming repo on an
     `.assay-versions` pin bump. Conform now so the gate lands on a clean backlog.
   - **Never reclassify an entry to make it fit the grammar.** The routing token is a commitment
     record — `follow-up` says work is still owed, `out-of-scope` says it isn't — so swapping one
     for the other to satisfy a regex destroys the fact the field exists to hold. Two cases have
     no legal encoding yet (open on assay-toolkit#159): a `fixed-here` wanting a `(<why>)`
     parenthetical, and a `follow-up` whose target is an intake entry or an unauthored brief, with
     no `<NN>` to cite. Until they settle, write the entry truthfully and leave it non-conforming —
     nothing enforces this yet, so a non-conforming entry costs nothing and a rewritten one costs
     the record. Same reason existing briefs are not backfilled ahead of the settled grammar.
   - **Flow Verify row**: a shared-value brief's Verify table must carry at least one row exercising
     the cross-component flow end-to-end (e.g. *frontend split → operator executes → CollateralVault
     appears*), not just the changed site. Site-green does not imply flow-green. This one stays a
     judgement call even after #159 ships — prose Verify rows cannot be perfectly classified, so the
     planned check is an advisory NOTICE, never a hard gate.
   - **Cross-repo pairing (#272)**: when a change crosses a repo boundary in addition to a component
     boundary, it carries BOTH a `consumers:` list (this rule) AND a sibling-repo SHA (#272) — note
     the pair in both places.
   This applies to **issue-driven work too** — a change to a shared value that skips a brief skips
   this analysis entirely, which is the highest-risk path in the system. Promote such a change to a
   brief, or at minimum run the consumer grep and record it.
7. **Out-of-repo files must be declared — the declaration is the claim.** A brief whose scope
   touches files outside the repo (user-level skills/settings, e.g. `~/.claude/**`) MUST list the
   exact paths in Context under `out-of-repo files:`. These files have no worktree isolation and no
   branch-as-claim, and an edit takes effect in every running session the moment it's written —
   review is post-hoc. So: the dispatcher **serializes** on the declaration (at most ONE such brief
   in flight at a time, across all streams), and the implementer stages the edit as a diff in the
   PR, applying it to the live file only as the LAST step before `implemented` — never
   incrementally. Where the out-of-repo surface is itself a git repo (e.g. the `~/.claude` stopgap
   repo), commit the applied edit there too. A brief that touches this surface without the
   declaration is scope drift — bounce it.
8. **Freshness check — before authoring a fix/change brief, verify the site isn't already
   fixed.** Read the target site on fresh `origin/main` (`git fetch origin && git log
   origin/main --oneline -- <path>` + read the code) and record the check in the brief's
   `sources:` as `freshness-checked <YYYY-MM-DD> @ <short-sha>`. If the deliverable is already
   satisfied, the brief is NOT authored — resolve or re-scope the finding/issue instead. The
   check is light; the waste it prevents (re-writing a fix that already landed, claiming a vuln
   closed that was never open) is heavy.
9. **exec-tier: complexity signals a minimum execution-model tier.** `exec-tier` is DERIVED,
   not chosen (mirror of the gate/risk rule). The author answers three complexity questions;
   any yes → `exec-tier: strong` (absent/any-no → `any`, the default):
   (a) Does the Task require design decisions the facts do not fully pre-specify?
   (b) Does correctness depend on cross-component/cross-artifact reasoning (shared values,
       end-to-end flows, sweeping a pattern across sites)?
   (c) Is it code where a subtle implementation error survives the brief's own tests (auth,
       funds, concurrency, safety plumbing)?
   `strong` SHOULD carry a one-line `exec-tier-why` naming which question(s). `statusgen --lint`
   PROBLEMs an unrecognized value, NOTICEs a missing `exec-tier-why`. **Honest limitation:**
   statusgen never verifies which model actually ran — pickup-side compliance is honor-system
   self-report; the enforced surface is desk dispatch + the review gate (methodology/29).

Keep a brief self-contained: if executing it requires knowledge from another brief, either link it
under "Read first" / `facts:` or state the dependency in `depends:` — never assume the reader has the
whole plan in context.

## Phase README structure

`docs/<phase>/README.md` ties the briefs together:

1. **Status table** — `| # | Brief | Wave | Status | What's deployed/landed |`. Status uses
   ✅ / ⚠️ / ❌ with a terse, honest "what's actually true" note (not aspirational).
2. **Critical path** — an ASCII chain showing the blocking order, with a one-line "smallest unblocking
   move" and a warning if any tempting-but-wrong first step is a dead end.
3. **Dependency waves** — `Wave 0: [..]  Wave 1: [..]←0  …`, plus the one-line critical path
   (`0→8→1→2→5`).
4. (Optional) "Before starting" prereqs and "Shared conventions" the briefs inherit.

## Working method

1. **Inventory the work** — list the discrete units. Each becomes a brief; keep them roughly equal-
   sized (split anything too big — e.g. one brief doing two distinct roles becomes two).
2. **Map dependencies** — for each unit, what must exist first (`Depends on`) and what it enables
   (`Unblocks`). This gives the waves (Wave 0 = no deps; Wave N = depends only on <N).
3. **Derive + verify the critical path** — the longest dependency chain. Verify its head per the
   judgment-call section above.
4. **Write the README first** (table + path + waves), then the briefs. Cross-link both directions.
5. **Convert relative dates to absolute** (e.g. "by next week" → the actual date) — these docs
   outlive the conversation.
6. **Be honest in status** — "⚠️ Workaround" / "Blocked by #X" beats a green checkmark that isn't
   true; the table is the thing people trust.
7. **Handoff to execution**: a brief is a scope-and-DoD contract, not a step-by-step plan — don't
   write one here. When a brief (especially Effort: L) is picked up for execution, consider running
   `superpowers:writing-plans` against that brief's Deliverables/DoD to produce a bite-sized TDD plan
   before touching code. That's a separate step, done at pickup time by whoever executes it — not
   part of authoring the brief.

## Conventions to inherit

- Briefs are reference docs, not narration — terse, imperative, link to the authoritative source
  rather than duplicating it.
- When a brief's work lands, fill its Evidence section (one row per Verify item, filled by someone who
  did NOT implement) and update the README status row + critical path in the same change, so the plan
  never drifts from reality.
- **Docs regeneration**: if this project has auto-generated docs or a docs-site that must be
  regenerated when the public-facing architecture, API surface, user flow, or operational tooling
  changes, every such brief MUST carry a docs-regeneration item in its DoD naming the specific
  pages/areas to update. State the project's concrete regen command in the project-level skill (or
  the repo's instructions file).
- If executing a brief surfaces a NEW non-obvious gotcha, fold it into the repo's instructions file
  (CLAUDE.md / AGENTS.md / etc.) so the next person doesn't rediscover it.
