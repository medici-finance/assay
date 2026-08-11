# Commands — the plugin's slash-command surface

This directory holds the Assay plugin's slash commands, each as `<name>.md`. When
the plugin is installed they surface **namespaced** as `assay:<name>` (same
namespacing rule as `../skills/`). `commands/` is orthogonal to `skills/` +
`hooks/` — it ships user-invoked one-shot commands, not resident skills or
lifecycle hooks.

## Commands

| Command | What it does |
|---|---|
| [`inbox.md`](./inbox.md) (`assay:inbox`) | The free tier's inbox: shells `../scripts/assay-inbox.sh` to query `gh` across your configured repos for open `needs-decision` / `question` / `help wanted` / `urgent` issues (the escalation contract), sorted urgency-then-age. Read-only, terminal-agnostic. |

**Naming:** command names follow the same kebab-case convention as skills; a
command's `description` frontmatter is what a user/agent sees before invoking it —
keep it a trigger, not a workflow summary.
