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
the **`changelog:skip`** label instead of a fragment. The `changelog-check` CI
leg greens on *either* a fragment *or* that label, and prints the skip in its log
so it is never silent.

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
