# Staged CHANGELOG-discipline workflows — activation prerequisite

The CHANGELOG discipline has moved from hand-edits of `CHANGELOG.md`'s
`## Unreleased` section to **per-PR fragment files under `changelog/`** — one file
per change, so concurrent PRs no longer collide on a shared section (the standing
merge-conflict class that section generated). Two halves of that rework live
under `.github/workflows/`. The identity that authored this PR **cannot push
under `.github/workflows/`** (a server-side workflows-scope block), so those two
halves are staged here instead of committed live.

Everything else in the rework is **live on the branch** and needs no activation:

- `changelog/` — the fragment directory, its `changelog/README.md` convention
  doc, and the seed fragments carrying the entries that were pending in
  `## Unreleased` at cutover;
- `CHANGELOG.md` — header prose rewritten for the fragment convention, and the
  `## Unreleased` section emptied to its pointer note (no bullets — see "Cutover"
  below);
- `tools/changelog/aggregate.py` + `tools/changelog/check.sh` — the engines the
  two workflows call, with offline unit tests (`aggregate_test.sh`,
  `check_test.sh`) and their fail-first stubs under `testdata/`;
- `plugins/assay/skills/worker-desk/SKILL.md` — the worker clause, now naming the
  `changelog/<slug>.md` fragment path.

A **workflows-capable identity** activates the two staged halves with the two
copies below. Nothing here runs while it sits under `tools/`; the copy is the
whole activation.

## Workflows-scope activation prerequisite

1. **PR-gating leg — `changelog-check`.** Copy
   `tools/changelog/changelog-check.yml` to
   `.github/workflows/changelog-check.yml` (dropping the leading `⚠️ STAGED …`
   banner block, down to the `# ---` rule). It greens a PR that adds a fragment
   under `changelog/` **carrying at least one highlight bullet** **or** carries
   the `changelog:skip` label, and **refuses** a PR that adds a highlight bullet
   under `## Unreleased` in `CHANGELOG.md` (the deprecation guard). An empty,
   whitespace-only, or bullet-less fragment is rejected — the gate is not
   satisfiable by `touch changelog/x.md`. The whole decision is
   `tools/changelog/check.sh`; the workflow only feeds it the PR base/head SHAs
   and the label boolean. The
   `changelog:skip` label already exists from the prior activation; no new label
   is needed.

2. **`release.yml` change — aggregate + refuse + clear.** Apply
   `tools/changelog/release.yml.patch` to the live
   `.github/workflows/release.yml`:

   ```
   git apply tools/changelog/release.yml.patch
   ```

   (Or copy `tools/changelog/release.yml.proposed` over
   `.github/workflows/release.yml`, dropping its leading `⚠️ STAGED …` banner
   block down to the `# ---` rule — the `.proposed` file is the full intended
   content; the `.patch` is the exact delta from the currently-live file.) The
   change reworks the two "CHANGELOG discipline"-marked steps in the `release`
   job:
   - **aggregate + refuse** — `aggregate.py highlights changelog CHANGELOG.md`
     collects every fragment (sorted, deduped, bucketed) into the release
     highlights and REFUSES to cut (clear error) when there is nothing to
     aggregate; runs on the `dry_run` path too. This is the direct heir of the
     retired empty-`## Unreleased` refusal.
   - **lift** — the release body still LEADS with those highlights, above the
     asset-inventory boilerplate (this step's code is unchanged; it reads the
     aggregated notes file).
   - **write + clear** — `aggregate.py roll …` writes the highlights under a dated
     `## vX.Y.Z — <date>` heading, empties `## Unreleased` to its pointer note,
     and the step `git rm`s the aggregated fragments from `changelog/` (keeping
     `README.md`) — all in ONE release commit.

   None of the tag-cut, signing, asset-upload, or immutable-release logic is
   touched; only the two changelog steps change.

No other files change under `.github/workflows/`.

## Cutover — the pending entries are not lost

Every entry that was pending in `## Unreleased` at cutover — including the ones
that landed on `main` while this PR was open — is seeded as a fragment under
`changelog/`, so they aggregate into the next release exactly as any fragment
would. This PR **dogfoods its own convention**: its own changelog entry is the
fragment `changelog/changelog-fragments-convention.md`, not a `## Unreleased`
edit, so the PR passes its own new gate. `## Unreleased` in `CHANGELOG.md` is
therefore fully empty (its pointer note only, no bullets). `aggregate.py` still
folds any *residual* `## Unreleased` bullet into the first aggregated release as a
safety net, but after this cutover there is nothing to fold — the fragments are
the only source.

**Promote before the next release cut.** The seed fragments are only aggregated
by the *new* `release.yml` (step 2). If a release is cut while the live,
pre-fragment `release.yml` is still in place, it reads only `## Unreleased` (now
just the pointer note) and does not aggregate the fragments; the seed fragment
files are left untouched on disk (the old workflow never reads `changelog/`) and
are picked up by the first release after promotion — deferred, not lost. Promote
both halves before cutting so the fragments land in that release.

## Why staged rather than committed — and where

Per the house no-evasion rule, a workflows-scope push block is a STOP signal, not
an obstacle to route around. The complete intended content is authored here so a
workflows-capable identity can activate it in one reviewable step, with the exact
delta preserved in `release.yml.patch`. This `tools/changelog/` location is the
**established** staged path in this repo (the current live workflows were
activated from here); it is used again rather than introducing a second staging
directory.
