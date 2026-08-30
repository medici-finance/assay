# Commands — the plugin's slash-command surface

This directory holds the Assay plugin's slash commands, each as `<name>.md`. When
the plugin is installed they surface **namespaced** as `assay:<name>` (same
namespacing rule as `../skills/`). `commands/` is orthogonal to `skills/` +
`hooks/` — it ships user-invoked one-shot commands, not resident skills or
lifecycle hooks.

## Commands

| Command | What it does |
|---|---|
| [`inbox.md`](./inbox.md) (`assay:inbox`) | The inbox: shells `../scripts/assay-inbox.sh` to query `gh` across your configured repos for open `needs-decision` / `question` / `help wanted` / `urgent` issues (the escalation contract), sorted urgency-then-age. Three renderings of one queue: the terminal table, `--walk [--item K]` for one item in the five-part decision format, and `--html OUT.html` for the whole queue as a self-contained page. Read-only, terminal-agnostic. |

**Naming:** command names follow the same kebab-case convention as skills; a
command's `description` frontmatter is what a user/agent sees before invoking it —
keep it a trigger, not a workflow summary.

**Commands render; skills drive.** `inbox --walk` prints one decision and exits — it never
prompts and never loops. The turn-taking that surrounds it (ask one, wait, record the ruling
as a relay on the issue, move the escalation label, present the next) lives in
[`assay:ask-decision`](../skills/ask-decision/SKILL.md). Keeping the loop out of the script is
what lets an agent, a human at a prompt, and a CI job share one renderer.
