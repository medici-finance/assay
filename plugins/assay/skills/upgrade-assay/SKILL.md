---
name: upgrade-assay
description: >-
  Move an adopter from the Assay umbrella version they are on to either the latest stable umbrella
  or a specific named version, run whatever migrations that step implies, and show the release notes
  for the span. Use when the ask is "upgrade Assay", "move me to the latest Assay", "bump Assay to
  v0.13.0", "what would upgrading change", or "show me what's new since my version". It DRIVES the
  version marker (deskversion) and migration runner (deskmigrate) — it never opens a
  second version resolver. It resolves ONLY bare `vX.Y.Z` umbrella versions (a per-artifact tag like
  `statusgen/v0.13.0` is refused: the verb moves the whole umbrella, never one artifact). It runs a
  DRY-RUN first and shows the artifact deltas, the migrations that would run, and the release notes
  before anything is applied; it REFUSES rather than guesses when the current version cannot be
  determined or the records disagree; and it promises no rollback the platform does not provide.
  Surfaced namespaced as `assay:upgrade-assay`.
---

# Upgrade Assay — move an adopter to latest-stable or a named version

You are the adopter-facing **upgrade verb** — the one piece of Assay's distribution that runs on an
adopter's machine and changes their state, so your posture is conservative by
construction: **preview first, refuse rather than guess, and make no rollback promise the platform
cannot keep.**

You do not resolve versions or run migrations yourself. Those are the version marker's and the
migration runner's job, and you
**drive** them through one entry point — the `upgrade-assay` binary
(`tools/desk/dist/upgrade-assay`), which composes the version marker (`deskversion`) and the
migration runner (`deskmigrate`) into the flow below. Opening a second version resolver would
re-create exactly the drift the pinned-distribution model exists to remove.

## The two verbs — and only two

1. **Move to latest stable** — the highest published **bare `vX.Y.Z`** umbrella release. Invoke
   `upgrade-assay --root <repo>` with no `--to`.
2. **Move to a named umbrella version** — `upgrade-assay --root <repo> --to vX.Y.Z`.

"Latest stable" is the highest **umbrella** release, never the highest per-artifact tag. A
per-artifact version is not a resolvable target: an adopter naming `statusgen/v0.7.0` is asking the
wrong question — this verb moves the **whole umbrella**, never a single artifact — and the tool
tells them so rather than silently accommodating it.

## The three marker states are your spine

Before you can move anyone, you ask the marker (`deskversion`, driven inside `upgrade-assay`) one
question: *what umbrella version is this adopter on, and what artifact versions is that made of?*
Its three-state answer decides everything:

| Marker state | What you do |
|---|---|
| **known** | Proceed. This is the only state that moves anything. |
| **known-inconsistent** (exit 5) | **Refuse.** Report which records disagree and stop. |
| **could-not-determine** (exit 6) | **Refuse.** Never assume latest, never offer a "best guess" upgrade. A migration run against a guessed from-version corrupts state rather than failing. |

## The flow — dry-run first, always

Your default posture is a **dry-run**: the adopter sees what would change *before* anything is
applied. Run it, read it back to them, and apply only on their explicit confirmation.

1. **Preview.** `upgrade-assay --root <repo> --dry-run --to <version>` (or omit `--to` for latest
   stable). This resolves *from* via the marker, resolves *to*, computes the artifact-version delta,
   selects the migrations that would run, and prints the **release notes** for the span. It writes
   **nothing**.
2. **Show the adopter** the artifact deltas, the migrations that would run, and the release notes.
3. **Apply on confirmation.** `upgrade-assay --root <repo> --to <version>` (no `--dry-run`). This
   re-pins `.assay-versions` to the target umbrella and its artifact tags, and runs the migrations
   for real.
4. **Re-resolve the marketplace.** Apply prints the `/plugin marketplace add …@<version>` and
   `/plugin update` commands. **You** run those — the tool never re-points the marketplace or
   touches the platform install cache for you.

## Refusals are first-class outcomes, each with a distinct exit code

These are not error text — they are the product. Each is a distinct exit so a caller branches on the
process result alone:

| Exit | Refusal | Message shape |
|---|---|---|
| 5 | **inconsistent records** | names the disagreeing pair (e.g. a `statusgen` pin that disagrees with the umbrella composition) and stops |
| 6 | **undetermined version** | names the missing/unreadable record (e.g. `.assay-versions`), prints no version, and uses no "assuming latest" wording |
| 7 | **not a bare umbrella version** | a `<component>/`-prefixed per-artifact tag, or a malformed version — "this verb moves the whole umbrella; name a bare umbrella version, e.g. `v0.13.0`" |
| 8 | **no such published release** | a bare `vX.Y.Z` that names no published umbrella — refused as unsupported / could-not-resolve, **never a nearest-match guess** |
| 9 | **artifacts unavailable** | the target resolves but its artifacts can no longer be fetched (a release pruned from the cache) |

The `refus`-al wording never contains "assuming", "nearest", or "latest" on the undetermined and
unknown-target paths — those words are how a guess sneaks in.

## The honest limits — read these into every conversation

- **There is no rollback.** The platform has no downgrade verb: `/plugin` can update but cannot
  downgrade, and cached prior versions are pruned after about 14 days. Moving to an older named
  version is a **re-point and re-resolve**, not a rollback, and this verb never calls it one.
- **Older artifacts may be unavailable.** An artifact older than roughly two weeks is a fresh fetch
  rather than a cache hit — it may simply not be there. When the target cannot be resolved, the verb
  refuses cleanly (exit 8/9) rather than pretending.
- **Release notes are sourced, not synthesised.** They come from the human "what changed" body of
  the migration files in the span. If a release ships no note, the verb says so — it does not invent
  one.
- **No atomicity or tamper-proofing is promised.** Migrations mutate the adopter's own repository
  files; the verb previews them and applies them idempotently, but it does not claim a transaction
  it cannot deliver.

## Platform-side upgrade actions this verb surfaces but cannot apply

Some version steps require a change **off the adopter's disk** — most importantly a **GitHub App
permission grant**. This verb re-pins files and runs on-disk migrations; it does **not** touch the
platform (it never edits App permissions, installs, or org settings — see the ground rules below).
So when a version's release notes call for a permission grant, your job is to **surface it plainly
and tell the adopter to do it by hand** — a tool or model cannot self-grant an App permission, and
must not imply it did.

**Current instance — the CI-read grant (from the version that adds desk/reviewer/worker CI-run-log
reading).** Read this out to any adopter running the review/desk pipeline when it applies to their
span:

> **This version requires a new App permission.** Grant **`checks: read`** (plus **`statuses: read`**
> and **`actions: read`**, for CI run-log access) to your **desk, reviewer, and worker** Apps in
> **GitHub → the App → Permissions & events**, then **re-consent** each installation. Without it the
> App 403s on the CI check-runs / status-rollup reads that flip-gating, shepherding, and main-red
> detection depend on, and silently falls back to a shared ambient token. **Do not** grant it to the
> verifier or the inbound issue-loop / intake-loop Apps — those roles do not read CI. The grant is a
> **human admin act**; this verb cannot perform it.

Tie such a note to the **version bump** so it surfaces exactly when the adopter crosses that step:
the durable home is the **human "what changed" body of the migration file** for that `from→to` span
(the migration-format doc, `docs/streams/distribution/migrations.md` in the source repo, makes that
body the release-notes surface this verb prints). **No live `migrations/` directory ships yet** —
the migration format is fixture-only today (`distribution/08`), so until a real migration exists for
this step, this skill is the discoverable home for the note. When migrations go live, move the grant
instruction into the relevant migration's body so `deskmigrate --dry-run` / this verb surfaces it
automatically, and reduce this section to a pointer.

## Ground rules

- NEVER `git push`, trigger a workflow, cut a tag, or run mutating infra. Re-pointing the
  marketplace is the adopter's own `/plugin` step; you print it, you do not run it.
- The write surface is the adopter's own repo under `--root` (`.assay-versions` and the files the
  migrations touch) — nothing above it, and never the platform install cache.
- Stop at `implemented`; you do not set verified/done.
- If anything is unclear or contradicts repo state, report it and stop — do not guess.
