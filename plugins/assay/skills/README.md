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

These are the portable, domain-neutral methodology skills the public bundle
ships. A project authoring its own project-local skills follows the same naming
convention above and keeps them in the repo's own `.claude/skills/`.
