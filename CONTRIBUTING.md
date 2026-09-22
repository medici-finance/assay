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

## The trust bar

The guidelines above are the half of the bar you see as rules. The other half is what the
project's automation does with a submission from an account it does not recognise, and that is
written down here too — so the bar is a policy you can read and meet, not something that
simply happens to you. Nothing in this section changes anything above: the guidelines stay
advisory, and a human maintainer still decides everything that matters.

- **A submission from an unrecognised account may be measured.** There is tooling that gathers
  a fixed list of mechanical facts about such a submission and posts them as a short
  **provenance card** — a comment on the pull request. The facts are about the submission's
  shape and timing, never about you as a person: how old the account is against when it first
  did anything, how long the fork existed before the pull request was opened, how many
  pull requests the author opened across repositories in a day, what proportion of the
  author's earlier submissions were merged, whether the commits are signed, and whether the
  diff touches the files that control what the build executes. The card states each fact with
  the ordinary innocent
  explanation beside it (a new account is how everybody starts; a fast fork-to-pull-request is
  what a prepared patch looks like), renders no score and no verdict, and closes with the same
  statement every time: it is not a judgement of the change, and the change is reviewed on its
  merits. A fact that could not be gathered is shown as *could not check*, never as silence.
  What the card measures, and what it deliberately never measures — no profile text, no
  follower counts, no employer, no location — is published in full in
  [docs/contributor-provenance.md](docs/contributor-provenance.md). The tooling never fetches,
  builds or executes your branch; every fact comes from public metadata.
- **Trust tiers exist, and what they unlock is published.** An account is not re-assessed as a
  stranger forever. There are four tiers — `unknown`, `blessed-once`, `contributor`,
  `maintainer` — and what each one unlocks (a deeper claims-versus-diff review, continuous
  integration without a manual approve, automation acting on your items at all, and the fork
  changelog arrangement below) is published in [docs/contributor-trust.md](docs/contributor-trust.md).
  A tier gates how much automation runs without a human present; it never merges anything and
  never stands in for a maintainer's judgement. **Who holds which tier is not published** —
  the model is public, the records are not, because a public list of trust judgements about
  named people would be a reputation register that serves nobody.
- **How an item is admitted.** As above, automation ignores items from outside contributors
  until a maintainer engages. A maintainer's comment on your item admits that one item — and
  if new content is added after that comment, the admission lapses until a maintainer looks
  again, so an edit does not ride in on an earlier approval. Continuous integration on a pull
  request from a first-time fork also needs a maintainer's approve-and-run click. Moving an
  account between tiers is a recorded human act with a reason: no amount of merged changes
  promotes anybody automatically, and a good record is deliberately not allowed to predict
  anything on its own.

The aim of all of it is the one stated above: to filter, not to deter. And the one thing that
is never filed in public is a security problem — see [SECURITY.md](SECURITY.md).

## Pull-request template

Every pull request opens against a short template with two prompts: which factual claims the
description makes and how you checked each one, and whether the change was produced with the
help of an AI coding tool or agent. Neither is checkable by any tool here — the point is that
an honest answer costs you nothing and a false one is a specific statement a reviewer can
point at. Fill it in; it is deliberately short.

## Changelog for fork pull requests

Most pull requests record a notable change with one fragment file under `changelog/` (see
`changelog/README.md`), and a required check fails a pull request that is missing one. If your
pull request comes from a fork, maintainers cannot push a fragment to your branch to fix a red
check the way they could on a branch in this repository — so you don't have to. When that
applies, a maintainer lands the fragment on the base branch on your pull request's behalf
instead, and the check picks it up from there. You are welcome to include your own fragment in
the pull request if you'd like to suggest the wording, but a missing one on a fork pull request
is not something you need to fix yourself.

## Practical notes

- License: Apache-2.0. By contributing you agree your contribution is licensed the same.
  The [NOTICE](NOTICE) file carries the attribution that conventionally accompanies that
  license.
- Keep changes focused. A pull request that does one thing, with a linking issue, is
  reviewable; a pull request that refactors adjacent code while doing something else forces
  the maintainer to review the unrelated change under time pressure.
