# Assay

Assay is a methodology for running software delivery across a fleet of AI agents, with machine-checkable gates so you can tell what actually happened.

[assay.guide](https://assay.guide/) · [Video walkthroughs](https://www.youtube.com/watch?v=zlIsDjFjqT4&list=PLQ-vGHp06goU)

## Overview

When agents do the work, generating code is the easy part; knowing whether it was done correctly is not. Assay makes each unit of work a self-contained, checkable artifact, and derives the project's status board from those artifacts rather than from a human's assertion.

It has four parts:

- **Briefs.** A self-contained unit of scope and definition-of-done, with typed dependencies, a risk-derived review gate, and a Verify table that can actually be run. One brief is one reviewable piece of work. See [docs/brief-template.md](docs/brief-template.md) and [docs/brief-rules.md](docs/brief-rules.md).
- **Registers.** Append-only `FINDINGS`, `INTAKE`, and `RETRO` logs. Entry IDs are slugs, not a counter, so they carry no contiguity guarantee — there is no sequence to have a gap in. What keeps nothing going missing quietly is what statusgen actually enforces: an entry that has ever existed on main but is absent from the working tree is a lint failure (a tombstone check against history), and duplicate IDs are rejected. Entries are tombstoned rather than deleted.
- **Streams and the board.** Briefs live in streams and move through a defined lifecycle. The board, `STATUS.md`, is generated, never hand-edited.
- **Tools that enforce the above.** `statusgen` lints the tree and generates the board. A separate reviewer identity, which the author cannot post as, keeps reviews attributable to someone other than the author. A trust gate names who the tooling will obey.

The board is derived from agent-authored artifacts, checked by the linter and re-verified independently. It is not measured from ground truth, and Assay does not claim it is. The aim is not a green dashboard but a set of gates that make a passing state hard to fake and easy to catch when the artifacts and the checks disagree.

## Contents

| Path | What it is |
|------|-----------|
| [docs/](docs/) | The methodology: the [adoption runbook](docs/adopting-assay.md), the brief schema ([rules](docs/brief-rules.md), [template](docs/brief-template.md)), the [lifecycle](docs/lifecycle.md), the [registers](docs/registers.md), the [evidence bundle](docs/evidence-bundle.md), and the [telemetry posture](docs/telemetry.md) (opt-in, off by default). |
| [statusgen/](statusgen/) | The generator and linter. Run `cd statusgen && go run . --root ../examples/adopter-scaffold --lint`. |
| [plugins/assay/](plugins/assay/) | The Assay plugin for Claude Code: methodology skills for adopting Assay and authoring briefs (`adopt`, `author-brief`). |
| [examples/adopter-scaffold/](examples/adopter-scaffold/) | A populated streams and registers tree to copy from. |
| [tools/freshness/](tools/freshness/) | Freshness checks for tracked artifacts. |
| [Dockerfile](Dockerfile) | Builds one combined Linux image with the desk-tools suite + `statusgen` on PATH, published to GHCR. See [docs/docker.md](docs/docker.md). |

## Container image

The desk-tools suite and `statusgen` are published together as a combined
Linux image at `ghcr.io/medici-finance/assay/desk-tools` (`linux/amd64`).
Native macOS/Windows binaries are a separate artifact. See
[docs/docker.md](docs/docker.md) for how to pull, run, and what's inside.

## Adopting Assay

The install guide is [docs/adopting-assay.md](docs/adopting-assay.md). It reads as a runbook for a coding agent to follow step by step, and covers three cases: a green-field repo, an existing suite of repos, and carving a unit out of a larger project.

Some steps are deliberately human-gated and never run autonomously: creating the reviewer GitHub App, choosing the trust-roster values, granting repo admin, and merging to `main`. Agents open draft PRs; a human reviews and merges.

## Links

- Site and guide: [assay.guide](https://assay.guide/)
- Video walkthroughs: [YouTube playlist](https://www.youtube.com/watch?v=zlIsDjFjqT4&list=PLQ-vGHp06goU)

## License

Apache 2.0; see [LICENSE](LICENSE) and [NOTICE](NOTICE). Before contributing, read [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md); report security issues per [SECURITY.md](SECURITY.md).
