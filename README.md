# Assay

**The agent-native delivery methodology — briefs, registers, and a derived status board behind machine-checkable gates, plus the tools that enforce them.**

📖 [assay.guide](https://assay.guide/) · 🎥 [Video walkthroughs](https://www.youtube.com/watch?v=zlIsDjFjqT4&list=PLQ-vGHp06goU)

---

Assay is an operating model for running **fleet-of-agents software work behind machine-checkable gates**. When a swarm of AI agents does the work, the hard part isn't generating code — it's *knowing what actually happened*. Assay makes each unit of work a self-contained, verifiable artifact, lints the whole tree, and derives the status board from those artifacts rather than from anyone's say-so.

## What it is

Four load-bearing parts:

- **Briefs** — self-contained units of scope + definition-of-done, each with typed dependencies, a **risk-derived review gate**, and an executable **Verify** table. One brief is one reviewable, checkable piece of work. See [`docs/brief-template.md`](docs/brief-template.md) and [`docs/brief-rules.md`](docs/brief-rules.md).
- **Registers** — append-only `FINDINGS` / `INTAKE` / `RETRO` logs. You *tombstone*, never delete; `statusgen` enforces contiguous sequencing so nothing quietly disappears.
- **Lifecycle & board** — briefs live in *streams* and move through a defined lifecycle. The board (`STATUS.md`) is **generated**, never hand-edited.
- **The tools that enforce them** — `statusgen` lints the tree and generates the board; a reviewer identity **the author cannot post as** makes reviews attributable; a trust gate names who the tooling obeys.

### The honest framing — why Assay is different

Assay's board is **derived from agent-authored artifacts with linting and independent re-verification — *not* measured from ground truth.** It deliberately claims the weaker, true thing.

The value isn't a dashboard that shows everything green. It's a set of gates that make it **expensive to fake green and cheap to catch when someone did** — attributable reviews, append-only registers, contiguous sequencing, risk-derived gating, and a linter that fails the build on drift.

## What's in this repo

| Path | What it is |
|------|-----------|
| [`docs/`](docs/) | The methodology — brief schema, the [adoption runbook](docs/adopting-assay.md), enforcement model, roster/trust configuration |
| [`statusgen/`](statusgen/) | The generator + linter. Run `cd statusgen && go run . --root .. --lint` |
| [`plugins/assay/`](plugins/assay/) | The **Assay plugin for Claude Code** — skills for authoring briefs, running the review desk, and fanning out agents (`the-desk`, `author-brief`, `pr-review-desk`, `verify-desk`, `batch-fanout`, `adopt`, …) |
| [`examples/adopter-scaffold/`](examples/adopter-scaffold/) | A worked, populated streams + registers tree to copy from |
| [`tools/freshness/`](tools/freshness/) | Freshness checks for tracked artifacts |

## Adopting Assay

The install guide, [`docs/adopting-assay.md`](docs/adopting-assay.md), is an **agent-executable runbook** — written for an Opus-class coding agent to follow step by step (human-readable second). It covers three scenarios — **green-field**, **existing suite**, and **carve-out** — composing named primitives (`install-statusgen`, `scaffold-streams`, `add-statusgen-ci`, `install-desk-plugin`, `configure-roster`, …).

Some steps are **human-gated on purpose and never autonomous**: creating the reviewer GitHub App, choosing the trust-roster values, repo admin grants, and merging to `main`. Agents open **draft** PRs only — a human flips and merges.

## Learn more

- **Site & guide:** [assay.guide](https://assay.guide/)
- **Video walkthroughs:** [YouTube playlist](https://www.youtube.com/watch?v=zlIsDjFjqT4&list=PLQ-vGHp06goU)

## License & contributing

Licensed under [Apache 2.0](LICENSE) (see also [`NOTICE`](NOTICE)). Contributions are welcome — please read [`CONTRIBUTING.md`](CONTRIBUTING.md), the [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md), and the security policy in [`SECURITY.md`](SECURITY.md).
