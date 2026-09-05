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

   **Steps 1 and 2 are ACTIVATED** — both halves are live under
   `.github/workflows/`, so `release.yml.patch` / `release.yml.proposed` are kept
   only as the historical delta. Step 3 below is the one still awaiting
   activation.

3. **`release.yml` change — move the roll into its own job, and retry the
   push.** Apply `tools/changelog/release-roll-retry.yml.patch` to the live
   `.github/workflows/release.yml`:

   ```
   git apply tools/changelog/release-roll-retry.yml.patch
   ```

   It applies cleanly to the currently-live file and changes nothing outside the
   roll. Two changes, both tracked on #312:

   - **The roll becomes its own job, `changelog-roll` (`needs: [resolve,
     release]`), instead of two trailing steps of `release`.** As a trailing
     step, a failed roll turned the whole run's conclusion to `failure` even
     though the tag, every asset and the published notes were complete — the
     misleading signal #312 reports. It was also unrecoverable: `release`'s
     upload step hard-refuses an asset name that already exists, so "Re-run
     failed jobs" could never reach the roll again and every failed roll had to
     be hand-filed as a PR (v0.23.0, v0.24.0, v0.26.0). As its own job the roll
     is independently re-runnable and `release`'s conclusion means "the release
     published" again.

     `continue-on-error: true` on the step was considered and rejected: it fixes
     the conclusion and nothing else — the failure would render as a neutral,
     near-invisible annotation inside a green job, and it would still be
     unreachable by a re-run. The split keeps the failure loud and red where it
     belongs while leaving the release green. Visibility and recoverability, not
     suppression.

   - **The push gets a bounded retry (5 attempts) that re-syncs and
     re-aggregates.** The previous shape did a single bare `git push` and lost
     the race against a moving default branch:

     ```
     ! [rejected]        HEAD -> main (fetch first)
     error: failed to push some refs to 'https://github.com/medici-finance/assay'
     ```

     This is a plain non-fast-forward rejection, **not** a ruleset refusal — a
     ruleset refusal is the `GH013: Repository rule violations found` message
     that v0.23.0/v0.24.0 hit before the board-writer App token was wired in.
     The App's bypass works; the race was simply never handled.

     The window is much wider than it looks, and **no checkout tweak can close
     it**. `actions/checkout` resolves `github.sha`, which for a
     `workflow_dispatch` is the head the ref carried *when the run was created* —
     not the tip when the job starts. And `release` sits behind the `release`
     environment's human-approval gate, so dispatch→job-start is however long the
     approver takes. The staleness window is therefore dispatch→push. Run
     `33989081398` (2026-09-05, the v0.26.0 cut) is the worked example: `head_sha`
     `3bb33ad5` pinned at 20:05:57Z, the release job not started until 20:12:51Z,
     the push at 20:19:15Z — by which time the default branch had taken three
     board-writer commits (20:06:55Z, 20:07:08Z, 20:08:40Z) that the checkout
     could never have contained. Re-syncing *at push time* is the only shape that
     closes this.

     The recovery mirrors `assay-statusgen.yml`'s `regen` and `model-autoflip`
     jobs exactly — the two other places in this repo that write generated files
     straight to the default branch: on rejection, re-sync to the new head and
     re-derive against it. Never `git pull --rebase` and never `--force`; the
     rolled section is a generated artifact, so a rebase conflict in it is
     meaningless and re-running `aggregate.py` against the new head *is* the
     resolution. A retry that finds nothing left to aggregate (`aggregate.py`
     rc 2) means a concurrent roll already landed the section, and exits 0.

     One accepted divergence: a fragment merged between the tag and a retry is
     aggregated into the `CHANGELOG.md` section but is absent from the
     already-published release notes. Over-inclusion is the safe direction — the
     alternative, pinning the fragment set to the tag, would drop that fragment
     from the CHANGELOG entirely, since the roll deletes it either way on the
     next cycle.

   The identity is unchanged: the roll still commits as the board-writer App,
   whose ruleset bypass covers this class of generated-file write, and still
   uses per-command header auth with `persist-credentials: false`. Opening a PR
   instead was considered and rejected — it would be the only generated-file
   write in this repo that does not land directly, it needs a human review on a
   mechanical commit, and it leaves `changelog/` un-cleared (so the *next*
   release aggregates the same fragments again) for as long as the PR sits.

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

## The cutover PR carries `changelog:skip` — on purpose

The PR that introduces this convention records its own change as a fragment
(dogfooding the new gate), so it adds no `## Unreleased` bullet. The *currently
live* `changelog` CI leg is still the pre-fragment one: it greps `CHANGELOG.md`
for an added bullet and cannot see fragments, so it would red this PR. The
`changelog:skip` label is that leg's sanctioned escape hatch, applied here as the
honest bridge for exactly one PR — the one that retires the leg. Once
`changelog-check.yml` is promoted (it greens on the fragment directly), the label
is moot and can be removed.
