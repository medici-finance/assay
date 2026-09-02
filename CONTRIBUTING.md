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

## Inbound pull-request policy

The aim of these two guidelines is to **filter, not to deter** — they keep the review queue
honest without turning away a genuine contributor.

- **Issue-first.** A pull request from an author who is not already a maintainer should
  **link an open issue**. Open the issue, let a maintainer confirm the change fits the
  direction, then send the pull request linking it (a closing keyword such as `Fixes #123`,
  or just a `#123` reference, is enough). A pull request with no linked issue gets a friendly
  advisory comment and a `needs-issue-link` label; it is **not** closed for that reason today.
  If a grace-window auto-close is ever switched on, this page and the comment will say so
  plainly first.
- **A guideline on concurrent pull requests.** Please keep the number of pull requests you
  have open here at once **small** (the current guideline is three). Ten open pull requests
  from one author in a minute is not ten contributions; it is one prompt and a review burden.
  Going over the guideline earns an advisory comment and a `too-many-open-prs` label, not a
  closed pull request. Fewer, well-scoped changes land faster.

Both guidelines are **advisory** unless this page says otherwise: the automation comments and
labels, and a human maintainer decides. Maintainers, collaborators, and repository bots are
exempt from both.

## Practical notes

- License: Apache-2.0. By contributing you agree your contribution is licensed the same.
  The [NOTICE](NOTICE) file carries the attribution that conventionally accompanies that
  license.
- Keep changes focused. A pull request that does one thing, with a linking issue, is
  reviewable; a pull request that refactors adjacent code while doing something else forces
  the maintainer to review the unrelated change under time pressure.
