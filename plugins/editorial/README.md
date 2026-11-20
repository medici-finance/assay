# Editorial

Install `editorial@assay` to write and revise technical articles. The plugin loads Article Craft, Prose Cadence, and their references. It has no hooks, agents, commands, MCP servers, or setup step.

## Claude Code

Run these commands in your terminal, then start a fresh session in your writing project.

```sh
claude plugin marketplace add medici-finance/assay
claude plugin install editorial@assay
```

Try `/editorial:article-craft` with a draft and its source notes. Once you have checked the claims, use `/editorial:prose-cadence` on the paragraph you want to revise.

## Codex

```sh
codex plugin marketplace add medici-finance/assay
codex plugin add editorial@assay
```

Start a fresh session in the writing project. Select `$editorial:article-craft` or `$editorial:prose-cadence` and name your draft in the prompt.

## Cursor

On Teams or Enterprise, import `https://github.com/medici-finance/assay` through **Dashboard → Plugins & MCPs → Team Marketplaces → Add Marketplace → Import from Repo**. Track `main`, add **editorial**, and grant the team access. Members can enable **editorial** under **Customize** and ask for either skill by name in a new Agent chat. If the marketplace is already connected, refresh it to discover the new plugin.

## Try it

Give Article Craft a source note and a draft claim. For example, a benchmark measured 20 warm requests from a replay of 80, but the draft says performance improved for every request. Ask which words the evidence supports. Then give the verified paragraph to Prose Cadence and ask it to make the sampling limit easier to hear while preserving the measurements.

## Updating this package

Article Craft and Prose Cadence are shared with the methodology plugin. Edit their canonical directories under `plugins/assay/skills/`, then copy the complete changed directories here, including references. When the published payload changes, increment the version in all three editorial plugin manifests. The umbrella release gate checks for a version increase when the editorial package changes between releases. The `tools/harnessgen` tests compare the complete skill trees, notices, manifests, and marketplace entries.
