# Staged CHANGELOG-discipline workflows — activation prerequisite

The CHANGELOG-discipline change (driver direction, 2026-08-31: releases carry
DESCRIPTIVE highlights, not commit lists) has two halves that live under
`.github/workflows/`. The identity that authored this PR **cannot push under
`.github/workflows/`** (a server-side workflows-scope block), so those two halves
are staged here instead of committed live. Everything else in the PR — the root
`CHANGELOG.md` and the `worker-desk` skill line — is live on the branch.

A **workflows-capable identity** activates the two staged halves with the two
copies below. Nothing here runs while it sits under `tools/`; the copy is the
whole activation.

## Workflows-scope activation prerequisite

1. **New PR-gating leg — `changelog-check`.** Copy
   `tools/changelog/changelog-check.yml` to
   `.github/workflows/changelog-check.yml` (dropping the leading `⚠️ STAGED …`
   banner block, down to the `# ---` rule). It fails a PR that adds no
   `## Unreleased` highlight line to `CHANGELOG.md` unless the PR carries the
   `changelog:skip` label; the label-gated skip is printed in the check log. Also
   create the repository label **`changelog:skip`** so the escape hatch exists.

2. **`release.yml` change — lift + refuse + roll.** Apply
   `tools/changelog/release.yml.patch` to the live
   `.github/workflows/release.yml`:

   ```
   git apply tools/changelog/release.yml.patch
   ```

   (Or copy `tools/changelog/release.yml.proposed` over
   `.github/workflows/release.yml`, dropping its leading `⚠️ STAGED …` banner
   block down to the `# ---` rule — the `.proposed` file is the full intended
   content; the `.patch` is the exact delta from the currently-live file.) The
   change adds three "CHANGELOG discipline"-marked steps to the `release` job:
   - **extract + refuse** — reads `## Unreleased` from `CHANGELOG.md` and REFUSES
     to cut (clear error) when it is empty; runs on the `dry_run` path too;
   - **lift** — the release body LEADS with the extracted highlights, above the
     existing asset-inventory boilerplate;
   - **roll** — in the same tagged flow, moves the highlights under a dated
     `## vX.Y.Z — <date>` heading and leaves `## Unreleased` empty for the next
     cycle.

No other files change under `.github/workflows/`.

## Why staged rather than committed

Per the house no-evasion rule, a workflows-scope push block is a STOP signal, not
an obstacle to route around. The complete intended content is authored here so a
workflows-capable identity can activate it in one reviewable step, with the exact
delta preserved in `release.yml.patch`.
