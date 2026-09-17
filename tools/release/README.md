# Staged release-workflow gate — activation prerequisite (assay#1192)

`spec/brief-v1.md`'s `Describes reference implementation:` line names a
`statusgen` version by hand. `#302` (2026-09-02) did a one-time freshen but
added no mechanical floor, so the header drifted silently again — it still
read `v0.22.0` while the released umbrella tag had moved dozens of releases
ahead by the time `#1192` was filed. This is the still-missing second half of
brief-13's Task 4
(`docs/streams/desk-tools/brief-13-schema-first-conformance.md`, house repo
`medici-finance/assay-toolkit`): "a release-time check ... that the spec
header matches the version being cut."

## What's live on this branch, no activation needed

- `spec/brief-v1.md` — the header freshened to the actual current release
  (verified against `gh release list`, not assumed from the issue text).
- `tools/release/check-spec-header-version.sh` — the check itself: compares
  the header's `statusgen` version token to the tag being cut, three-state
  (0 match / 1 mismatch / 2 could-not-check on an unreadable file, missing
  line, or unparseable token).
- `tools/release/check-spec-header-version.test.sh` — its offline unit test
  (8 cases: match, mismatch, header-ahead-of-tag, missing header line,
  unparseable token, missing file, malformed tag argument, default file-path
  resolution). Run it directly: `bash tools/release/check-spec-header-version.test.sh`.

## What's staged here, awaiting activation

One new step in `.github/workflows/release.yml`'s `guard` job — "Release gate
— refuse a stale spec/brief-v1.md reference-implementation version" — is
**staged, not committed live**. The identity that authored it (the worker-desk
App) **cannot push under `.github/workflows/`**: GitHub rejected the push with

```
! [remote rejected]   feat/issue-1192 -> feat/issue-1192 (refusing to allow a
GitHub App to create or update workflow `.github/workflows/release.yml`
without `workflows` permission)
```

This is the **established** staging path in this repo — `tools/changelog/`
carries the same shape (`release.yml.patch` + `release.yml.proposed` +
activation instructions in its own README) for the same server-side
workflows-scope block, and is reused here rather than inventing a second
convention.

Per the house no-evasion rule, a workflows-scope push block is a STOP signal,
not an obstacle to route around — this directory is that stop, made
reviewable.

## Activating the gate

A **workflows-capable identity** (a human, or an App/token carrying the
`workflows` permission) applies the staged change with either:

```
git apply tools/release/release.yml.patch
```

or by copying `tools/release/release.yml.proposed` verbatim over
`.github/workflows/release.yml`, dropping its leading `⚠️ STAGED …` banner
block down to the `# ---` rule (the `.proposed` file is the full intended
content; the `.patch` is the exact delta from the currently-live file — both
were verified byte-identical against each other before this PR was opened).

The change adds exactly one step to the `guard` job, next to the existing
`plugins/assay` manifest-version gate:

```yaml
      - name: Release gate — refuse a stale spec/brief-v1.md reference-implementation version
        env:
          TAG: ${{ needs.resolve.outputs.tag }}
        run: bash tools/release/check-spec-header-version.sh "$TAG"
```

No other line of `release.yml` changes — the tag-cut, guard-against-moving-a-
tag, build, test-matrix, publish and changelog-roll logic are untouched.

## Verifying the fail-first / pass evidence without activation

The check script and its test are independently runnable — see the PR body's
`## Fail-first` section for the actual pre-fix (stale header, exit 1) and
post-fix (freshened header, exit 0) runs against the real
`spec/brief-v1.md`, plus the full 8-case test suite.
