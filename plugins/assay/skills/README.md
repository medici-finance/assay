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
| adopt | `assay:adopt` | Install/adopt the Assay methodology into a project |
| author-brief | `assay:author-brief` | Brief authoring methodology (portable core) |

These are the portable, domain-neutral methodology skills every Assay bundle ships. A project
authoring its own project-local skills follows the same naming convention above and keeps them in
the repo's own `.claude/skills/`.

## The desk-role skills

| Skill | Namespaced as | Role |
|-------|---------------|------|
| the-desk | `assay:the-desk` | Coordinator desk — cadence, register discipline, cross-desk state |
| intake-desk | `assay:intake-desk` | Front door — converts inbound issues/ideas into one of five tracked exits |
| batch-fanout | `assay:batch-fanout` | Dispatches workers against the Next-up batch |
| pr-review-desk | `assay:pr-review-desk` | Reviews PRs leaving the system and flips them ready |
| verify-desk | `assay:verify-desk` | Drains `implemented → verified` with independent re-verification |

These five carry the current house methodology for running the five-desk pipeline
(`intake-desk → batch-fanout → pr-review-desk → verify-desk`, coordinated by `the-desk`) — see
[`../PARITY.md`](../PARITY.md) and [`../SOURCES.yaml`](../SOURCES.yaml) for provenance. Unlike the
two methodology skills above, they are **not yet parameterised**: `intake-desk` (education/04) is
scrubbed of house repo slugs, issue numbers and names, but the other four are still a straight
re-port that carries house-specific prose (tracked as the deferred `assay-selfcontain/09`
parameterisation pass — read `../PARITY.md`'s "Known-behind" section before relying on their exact
wording in another project).
