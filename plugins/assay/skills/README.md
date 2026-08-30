# Skills — assay methodology skills v0.1.0

The Assay methodology skills, each as `<name>/SKILL.md`. When the plugin is
installed they surface **namespaced** as `assay:<name>` — that namespacing is
deliberate: it structurally prevents a personal `~/.claude` skill of the same
bare name from shadowing the plugin one.

**Naming:** skill names follow [`docs/skill-naming.md`](../../../docs/skill-naming.md) —
kebab-case; `<role>-desk` for standing roles, `verb-noun` for actions,
`<domain>-<thing>` for project subsystems; methodology skills here are
domain-neutral, project skills carry a domain token; descriptions are triggers only.

## The methodology skills

| Skill | Namespaced as | Role |
|-------|---------------|------|
| install | `assay:install` | Turnkey installer — invoke → self-installs the whole project setup (Unix-first) |
| adopt | `assay:adopt` | Install/adopt runbook — scenario routing + PRIMITIVEs the turnkey installer wraps |
| author-brief | `assay:author-brief` | Brief authoring methodology (portable core) |
| ask-decision | `assay:ask-decision` | Puts the pending human decisions to the driver one at a time — context, options with a recommended default, reply shape, verification — and relays each ruling back onto its issue |

These are the portable, domain-neutral methodology skills every Assay bundle ships. A project
authoring its own project-local skills follows the same naming convention above and keeps them in
the repo's own `.claude/skills/`.

`ask-decision` is the one any of the five desk roles below may invoke: it is how a desk that has
accumulated human gates hands them over, one at a time, instead of narrating them. It carries the
presentation and relay contract only — the escalation-label vocabulary and the close authority stay
where they already live, in `intake-desk` and `the-desk`. It shares its rendering with the
[`assay:inbox`](../commands/inbox.md) command's `--walk` and `--html` modes, so the skill, the
terminal and the page cannot state the queue differently.

## The desk-role skills

| Skill | Namespaced as | Role |
|-------|---------------|------|
| the-desk | `assay:the-desk` | Coordinator desk — cadence, register discipline, cross-desk state |
| intake-desk | `assay:intake-desk` | Front door — converts inbound issues/ideas into one of five tracked exits |
| worker-desk | `assay:worker-desk` | Dispatches workers against the Next-up batch |
| pr-review-desk | `assay:pr-review-desk` | Reviews PRs leaving the system and flips them ready |
| verify-desk | `assay:verify-desk` | Drains `implemented → verified` with independent re-verification |

These five carry the current house methodology for running the five-desk pipeline
(`intake-desk → worker-desk → pr-review-desk → verify-desk`, coordinated by `the-desk`). All five
are present and scrubbed for the public plugin — house repo slugs, issue numbers and names removed.
Unlike the two portable methodology skills above they still carry house-specific operating prose, so
read a skill's own `SKILL.md` before relying on its exact wording in another project.
