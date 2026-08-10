# Skills — assay loop skills v0.1.0

The Assay skills, each as `<name>/SKILL.md`. When the plugin is installed they
surface **namespaced** as `assay:<name>` — that namespacing is deliberate: it
structurally prevents a personal `~/.claude` skill of the same bare name from
shadowing the plugin one.

**Naming:** skill names follow [`docs/skill-naming.md`](../../../docs/skill-naming.md) —
kebab-case; `<role>-desk` for standing roles, `verb-noun` for actions,
`<domain>-<thing>` for project subsystems; methodology skills here are
domain-neutral, project skills carry a domain token; descriptions are triggers only.

## The five loop skills (assay-dogfood brief 02)

| Skill | Namespaced as | Role |
|-------|---------------|------|
| the-desk | `assay:the-desk` | Coordinator — arbitrates across streams |
| pr-review-desk | `assay:pr-review-desk` | Pre-merge review loop |
| verify-desk | `assay:verify-desk` | Post-merge verification |
| batch-fanout | `assay:batch-fanout` | Work dispatch — fan out Next-up to workers |
| author-brief | `assay:author-brief` | Brief authoring methodology (portable core) |

[`PARITY.md`](../PARITY.md) in the parent directory records the port from the
`~/.claude` / oit source skills, with every intentional change
justified.

## Other skills

| Skill | Namespaced as | Role |
|-------|---------------|------|
| adopt | `assay:adopt` | Install/adopt the Assay methodology into a project |
| market-intelligence | `assay:market-intelligence` | Recurring competitive / field scan |
