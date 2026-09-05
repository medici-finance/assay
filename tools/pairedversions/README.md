# `pairedversions` — the front-door consistency guard

`assay:install` resolves the statusgen (and desk-tools) binary an adopter gets
**from `plugins/assay/paired-versions.yaml`**, on behalf of the plugin version
recorded in `plugins/assay/.claude-plugin/plugin.json`:

> plugin version (`plugin.json`) → paired statusgen tag (`paired-versions.yaml`)
> → pinned, sha256-verified download from the release home.

Those two records are edited by different changes at different times, and until
this checker existed nothing made them disagree *loudly*. A plugin bump that
skipped the re-pin left every clean adopter installing the tool the **previous**
plugin was paired with — many minors behind the skills shipping alongside it,
and behind the pins the desk skills expect. The manifest's own header already
carried the rule ("keep `plugin` in step with plugin.json's version; on a plugin
bump, re-pin"); a rule with nothing that reddens when it is broken is a comment.
This is the thing that reddens.

## What it asserts

| | Assertion | Why it matters |
|---|---|---|
| **a** | `plugin.json` `version` == `paired-versions.yaml` `plugin` | Otherwise the shipped pairing is for a plugin version nobody is running, and the installer resolves the wrong tag for the skills it ships with. |
| **b** | each section's `tag` is a **published** release of the `release_home` it names | A tag that was never cut, or a draft, gives the adopter's `gh release download` nothing to install. |
| **c** | every per-platform `sha256` equals that release's own `checksums.txt` entry | The installer REFUSES on a hash mismatch. A locally built binary lacks the release `-ldflags` stamp and hashes differently, so a hash that did not come from the published `checksums.txt` bricks the install. |

Three supporting assertions fall out of the same read, because each is a way for
(b) or (c) to be true of nothing:

- a pin line must have the channel-E `<artifact> <tag> <sha256>` shape — an
  unreadable pin is a pin that has **not** been checked;
- a pin line's own tag must equal its section's tag — `assay:install` reads the
  tag from the *line*, so a half-applied re-pin would download a different
  release than the one whose checksums were verified;
- a pinned artifact must actually be published by that release — the manifest
  deliberately leaves an unavailable platform **out** rather than inventing a
  hash for it, and this catches inventing one anyway.

A **pre-release** pin is reported as a note, not a failure: a prerelease is
published and installable, so the honest treatment is to make it visible rather
than to fail or to stay silent.

## Three-state, fail-closed

`checked-clean` is exit 0. A checked disagreement **and** a could-not-check are
both exit 1, and each is reported as itself — a release home that could not be
read has cleared nothing, and is never rounded up to a pass
([`docs/three-state-instrument-rule.md`](../../docs/three-state-instrument-rule.md)).
There is no flag that skips the network half: a checker that quietly passes when
its evidence is unavailable is the fail-open shape this guard exists to remove.

The run does not short-circuit. One invocation reports every disagreement it can
see, so a re-pin is a single edit rather than a sequence of one-failure-per-round
CI trips.

## Running it

```
make paired-versions                       # from the repo root
cd tools/pairedversions && go run . --root ../..
```

Exit codes: `0` consistent · `1` a problem or a could-not-check · `2` usage error.

It reads the GitHub REST API (`api.github.com`, or `$GITHUB_API_URL`) for (b) and
(c), using `$GH_TOKEN` / `$GITHUB_TOKEN` when one is present. It speaks to the
API directly rather than shelling out to `gh`, because not every self-hosted
runner ships the CLI and a checker that skips when its tool is missing fails open.

## Tests

```
cd tools/pairedversions && go test ./... -count=1
```

Every failure mode has a fixture under `testdata/` (or a fake release state in
`check_test.go`), and each case asserts both that the guard goes red **and** that
the report names the right reason — a guard that reddens for the wrong reason is
not the guard you think you have. `github_test.go` drives the real transport
against an `httptest` server, including the 404-is-a-checked-answer mapping and
the `checksums.txt`-against-its-own-digest cross-check.

## CI

The workflow half is staged at [`activation/plugin-drift.yml`](activation/plugin-drift.yml)
— see [`activation/README.md`](activation/README.md) for the copy command and
what a human still has to land.
