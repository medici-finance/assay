# Contributing

Thanks for your interest. A few expectations up front, so nobody is surprised:

- **Issues are the contribution surface.** Open an issue before investing in a change. The
  maintainers decide what enters the repository; an unrequested pull request may be closed
  undiscussed if it is not aligned with the direction. A linked issue is how that alignment
  is shown.
- **The maintainer merges.** Pull requests are reviewed at maintainer discretion; there is
  no SLA. Small, well-scoped changes with a clear problem statement (and a linking issue)
  fare best. Reviews may be assisted by automation, but a human maintainer is the merge
  authority.
- **Automation acts only on maintainer-approved items.** The maintainers run automated
  tooling over this repository. That tooling deliberately ignores issues, pull requests, and
  comments from outside contributors until a maintainer has engaged with them. If your item
  has not been picked up, it is waiting for a human maintainer — commenting more will not
  summon the bots.
- **CI for pull requests from forks requires maintainer approval** before it runs. This is
  intentional, and matches the fork-PR approval policy recorded for this repository.
- **Consumers pin versions; this repo does not promise a moving main.** This repository is
  consumed by tag or commit SHA. `main` is the development tip and may shift; a release tag
  is the supported artifact. Anything you build against this work should pin a tag, and
  re-pin deliberately when you want a change.
- **Security problems** must never be a public issue. See [SECURITY.md](SECURITY.md).

## Practical notes

- License: Apache-2.0. By contributing you agree your contribution is licensed the same.
  The [NOTICE](NOTICE) file carries the attribution that conventionally accompanies that
  license.
- Keep changes focused. A pull request that does one thing, with a linking issue, is
  reviewable; a pull request that refactors adjacent code while doing something else forces
  the maintainer to review the unrelated change under time pressure.
