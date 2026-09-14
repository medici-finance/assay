# `changelog/` — one fragment per change

A notable change records itself here, in **one small file per pull request**,
instead of editing a shared `## Unreleased` section in `CHANGELOG.md`. When many
PRs are open at once they no longer all touch the same lines of the same file, so
the standing merge-conflict class that section generated is gone. At release
time the release workflow **aggregates** every fragment into the new version's
`CHANGELOG.md` section and the release notes, then **clears** this directory.

## Adding a fragment

Create one file named for your branch or PR:

```
changelog/<slug>.md          e.g. changelog/deskpost-verdict-labels.md
```

Put one — or a few — human-legible highlight bullets in it, the same
"is-this-notable?" bar as before:

```markdown
### Added
- `deskpost` attaches mechanical verdict-time triage labels to agent PRs …
```

- **Buckets are optional.** A `### Added`, `### Fixed`, or `### Changed` heading
  classifies the bullets beneath it (Keep a Changelog). Bullets with no heading
  default to **Changed**. A fragment may carry more than one bucket.
- **At least one real bullet.** A fragment must carry at least one `- …`
  highlight line — an empty, whitespace-only, or bullet-less file records nothing
  and is **rejected** by `changelog-check` (the gate is not satisfiable by
  `touch changelog/x.md`).
- **Keep it to highlights**, not a commit log — descriptive lines a reader
  understands without the diff.
- **Slug uniqueness** is what keeps two PRs from colliding: name the file after
  your branch/PR so no two open PRs write the same path.

## Not notable?

A genuinely non-notable PR (a typo, a comment-only diff, a pure refactor) carries
the **`changelog:skip`** label instead of a fragment. The label is for changes
that are not worth recording — **not** for a notable change whose PR simply
cannot carry a fragment; that case has its own path, "Fragment by proxy" below. The `changelog-check` CI
leg greens on *either* a fragment *or* that label, and prints the skip in its log
so it is never silent.

## Fragment by proxy (fork PRs)

A pull request from a **fork** cannot always receive a fragment: maintainers
often cannot commit to the contributor's branch, and the contributor may be gone
by the time the omission is noticed. Labelling such a PR `changelog:skip` would
green it by dropping a notable change out of the release notes — the wrong
trade. Land the fragment on the **base branch** instead, under a name reserved
for that one PR:

```
changelog/pr-<N>-<slug>.md   e.g. changelog/pr-1234-widget-frame-drop.md
```

`<N>` is the fork PR's number. `changelog-check` reads the base branch for a
fragment matching that PR's number and greens the PR on its strength — the same
content bar applies, so the proxy must carry at least one real `- …` highlight
bullet. The proxy is read from the **live tip of the base branch**, not from the
base commit recorded when the PR was opened, so it works for a fragment merged
*after* the fork PR opened — which is the usual case — and nothing has to be
updated on the contributor's branch. Resolving that live tip needs the base
branch's name; the check reads it from `GITHUB_BASE_REF`, the env var GitHub
Actions sets by default on every `pull_request` run, so this requires **no
workflow change** to start working (an explicit `BASE_REF` in the workflow
overrides it, but is not required).

**You need not credit the contributor by hand.** At release time the aggregator
resolves each fragment back to the pull request it arrived on and appends
` — thanks @<login>` to that fragment's bullets when the author is somebody the
operator's roster does not already list — so a proxy fragment carries the credit
whether or not whoever wrote it remembered to. Writing the credit into the
bullet as well is harmless but redundant:

```markdown
### Fixed
- `widget` no longer drops the last frame. (#1234)
```

## Credit in the release notes

A change that arrived from outside is thanked in the changelog section and the
release body, by the login on its pull request. The name comes from git and the
pull request, never from the fragment's text, so nobody has to remember to write
it. Three rules bound it:

- **Only somebody the project does not already list.** A maintainer, a mapped
  human or a role automation account is never thanked — the credit exists to
  name an outside contributor. The identity question is the operator's existing
  roster, asked once.
- **Opt-out.** A contributor who would rather not be named puts this marker on a
  line of its own in the **pull-request body**:

  ```
  <!-- changelog-credit: no -->
  ```

  The fragment then aggregates exactly as before, credited to nobody. Anyone who
  can edit the body — the contributor or a maintainer acting on their request —
  can set it, and it takes effect at the next cut.
- **A missing credit never fails a cut, and a published section is never
  rewritten.** If the pull request, the author or the identity cannot be
  resolved, the release proceeds uncredited and says so in its log. Credit is
  forward-only: a `## vX.Y.Z` section already in `CHANGELOG.md`, and a release
  already published, are left exactly as they are.

The steps:

1. A maintainer or the review desk opens a **fragment-only PR** on the base
   branch adding `changelog/pr-<N>-<slug>.md` with the credited bullet, and
   merges it. (That PR records itself: the fragment it adds is its own.)
2. On the fork PR, **add or remove any label**. `changelog-check` re-runs on
   `labeled`/`unlabeled`, re-reads the base branch, finds the proxy, and greens.
   There is nothing to push to the fork branch.

**If the fork PR closes unmerged, remove the proxy** by a follow-up PR before the
next release cut — otherwise the notes advertise a change that never landed. The
proxy is an ordinary fragment in every other respect: it aggregates, and the
release clears it like any other.

A proxy is bound to exactly one PR by its `<N>`; it never greens a different PR,
and a `pr-<N>-…` file added by the PR itself is just an ordinary fragment
satisfying the ordinary rule.

## Do NOT edit `CHANGELOG.md`'s `## Unreleased` section

That path is **retired**. `changelog-check` refuses a PR that adds a highlight
bullet under `## Unreleased` — record it as a fragment here instead. `CHANGELOG.md`
is written only by the release workflow, which aggregates these fragments at cut
time.

## What release does

At each `vX.Y.Z` cut the release workflow:

1. **Aggregates** every `changelog/*.md` (this README excluded), sorted and
   de-duplicated within each bucket, into a dated `## vX.Y.Z — <date>` section in
   `CHANGELOG.md` and into the release-body **Highlights**.
2. **Refuses** to cut when there is nothing to aggregate — no fragments and no
   residual content — exactly as the old convention refused an empty
   `## Unreleased`.
3. **Clears** `changelog/` (deletes the aggregated fragments) in the same release
   commit, so the next cycle starts empty.

The engine and its offline tests live under `tools/changelog/`
(`aggregate.py`, `check.sh`, and their `*_test.sh`).
