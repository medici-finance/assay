# Activation — `plugin-drift.yml`

`plugin-drift.yml` in this directory is the **workflow half** of the
`pairedversions` guard. It is **staged, not active**: GitHub only runs workflows
that live under `.github/workflows/`, so nothing here executes until a human
copies it into place.

It is staged rather than landed because the identity that authored this change
does not hold the `workflows` permission — it cannot write `.github/workflows/*`
at all. That is a deliberate scope boundary, not an oversight, so the file is
delivered where it can be reviewed in the same diff as the tool it runs.

## Landing it — a human act

```sh
cp tools/pairedversions/activation/plugin-drift.yml .github/workflows/plugin-drift.yml
git add .github/workflows/plugin-drift.yml
git commit -m "ci: add the plugin↔statusgen front-door drift gate"
```

Copy it **verbatim**. It is written against this repository's existing CI
conventions — the `medici-builder-public` runner label, the pinned
`actions/checkout` sha, and the by-hand Go install from `go.dev/dl` that
`ci.yml` documents (that runner cannot reach the release-asset CDN
`actions/setup-go` downloads from).

## What it does once landed

Runs on **`pull_request`** and on **`push` to `main`**, with no `paths:` filter,
and fails when any of these is false:

- `plugins/assay/.claude-plugin/plugin.json` `version` equals
  `plugins/assay/paired-versions.yaml` `plugin`;
- each section's paired `tag` is a **published** release of the release home it
  names;
- every per-platform `sha256` equals that release's own `checksums.txt` entry.

Both triggers are load-bearing. `pull_request` catches a plugin bump that skipped
the re-pin *before* it lands, which is the only moment the fix is cheap.
`push: main` catches the drift that needs no diff at all — a release re-tagged or
removed upstream — which a pull-request-only gate would never look at again.

## Intended as a required check

The point of the gate is that the front door **cannot** be shipped inconsistent.
That means it should be a **required status check** on `main`'s branch protection
ruleset, under the check name **`paired-versions`** (the job id). Adding it to
branch protection is a repo-admin act and is not something this change can do.

Until it is required it is advisory: it will go red on the PR, but nothing stops
a merge over it.

## Verifying it after landing

```sh
# green on a consistent front door
cd tools/pairedversions && go run . --root ../..; echo "exit: $?"    # -> 0

# red on an inconsistent one — mutate a COPY of the manifest, never the tracked file
```

`../README.md` describes what each assertion asserts and how to run the unit
tests that prove the guard can go red for each reason independently.
