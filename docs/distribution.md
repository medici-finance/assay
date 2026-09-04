# Assay distribution — install and upgrade are one story

Assay is distributed as a versioned Claude Code plugin plus a set of version-pinned tool binaries.
There is exactly **one mechanism** underneath both installing and upgrading: a repo records which
release it runs in a `.assay-versions` pin file, and a single umbrella version names the composition
of per-artifact tags that pin should hold. Install writes that pin for the first time; upgrade moves
it. This document is the contract for the pin file and the umbrella, and the map from install to
upgrade.

The step-by-step install runbook lives in [`adopting-assay.md`](adopting-assay.md); this file is the
distribution model those steps implement. The two are one story with one mechanism, not two
implementations.

Installing changes nothing about what leaves your machine: `statusgen` collects **no telemetry by
default** — an anonymized, counts-only ping exists but stays off unless you opt in twice
(`--telemetry` plus `ASSAY_TELEMETRY=1`); see [`telemetry.md`](telemetry.md).

## The umbrella version

The release home `medici-finance/assay` cuts a **bare `vX.Y.Z`** umbrella tag (e.g. `v0.13.0`) —
never a component-prefixed `assay/vX.Y.Z`. One umbrella version names a **composition**: the exact
per-artifact tag each shipped component was built at, recorded in a composition manifest
(`releases/<umbrella>.yaml`). "Latest stable" means the highest **umbrella** release — never the
highest per-artifact tag.

Per-artifact tags (`statusgen/v0.8.2`, `desk-tools/v0.2.6`) are how the source repo cuts individual
tools; they are **not** upgrade targets. An adopter moves the whole umbrella, never a single
artifact.

**A plugin bump is not cut until the pairing is re-pinned.** `assay:install` resolves the binary an
adopter gets from `plugins/assay/paired-versions.yaml`, so bumping
`plugins/assay/.claude-plugin/plugin.json` without re-pinning that manifest — same plugin version,
the paired release tag, and every per-platform `sha256` refreshed from that release's published
`checksums.txt` — ships the *previous* plugin's tool to every clean install. `make paired-versions`
(`tools/pairedversions`) asserts all three and fails closed; it is intended as a required check, so
a bump that skips the re-pin is red before it lands.

## The `.assay-versions` pin file

The pin file lives at the consumer repo root and is the single record of which release a consumer
runs. It has two kinds of line:

- **Per-artifact lines** — `<artifact> <tag> <sha256>`, one per installed tool/platform. The tag is
  a bare `vX.Y.Z` (or a legacy `<component>/vX.Y.Z`); the third field is the sha256 of the published
  release asset (or, for a `-source` line, a 40-hex commit sha). Selection is a **trailing-space
  prefix match**: `statusgen ` never matches `statusgen-linux-amd64 `.
- **The optional umbrella line** — `assay <vX.Y.Z>`. It names a suite composition, not a
  downloadable asset, so it is the one line with no sha256. Its absence is a valid, expected state
  (the per-artifact lines stay authoritative); a repo simply is not recorded against a suite
  version.

A consumer that cannot read its pin cannot claim to be pinned: a missing or malformed pin file is
**fail-closed** (could-not-check), never silently defaulted. `deskpins --check` validates a pin file
against this contract.

## The version marker (`deskversion`)

`deskversion --root <repo>` answers, three-state, which umbrella version a repo is on and which
artifact versions that is made of. It assembles the answer from the pin file cross-checked against
the composition manifest — it invents no fourth source of truth — and reports one of:

- **known** (exit 0) — one umbrella version, consistent composition.
- **known-inconsistent** (exit 5) — records disagree; the report names the pair.
- **could-not-determine** (exit 6) — no/unreadable pin, no umbrella line, or an unreadable
  composition. Never "assume latest".

## The migration runner (`deskmigrate`)

A migration carries an adopter's repository across one version step. Migration files are
human-and-agent readable: YAML frontmatter (`id`, `from`, `to`, idempotent `apply:` steps) plus a
markdown "what changed" body that is the release note. `deskmigrate` selects the migrations whose
span lies within a requested `[from,to]` and applies them idempotently, or previews them under
`--dry-run`. Most releases ship **no** migration; the common upgrade path is empty and silent.

## Upgrading — the `assay:upgrade-assay` skill

Upgrading is a single verb: **`assay:upgrade-assay`** (the `upgrade-assay` skill), which drives the
marker and the runner above through the `upgrade-assay` binary. It moves an adopter to latest stable
or a named umbrella version, previews the change dry-run-first, runs the migrations the step
implies, and shows the release notes — then prints the `/plugin` re-resolve command for the adopter
to run. It is the **only** supported upgrade path: hand-editing pins and reading a release page is
exactly the drift the pin/umbrella model removes.

`upgrade-assay` refuses rather than guesses — on an undetermined version, on inconsistent records,
on a per-artifact tag where an umbrella version was required, and on a target that names no
published release (never a nearest-match guess).

### There is no rollback

The platform has no downgrade verb: `/plugin` can update but cannot downgrade, and cached prior
versions are pruned after about 14 days. Moving to an older named version is a re-point and
re-resolve — it is **not a rollback**, and `upgrade-assay` never calls it one. An artifact older
than roughly two weeks may be a fresh fetch rather than a cache hit and is not available if the
release home no longer serves it; the verb refuses cleanly rather than pretending otherwise.

## From install to upgrade — one line moves

1. **Install** writes `.assay-versions` (per-artifact pins, and optionally the umbrella line) and
   acquires the sha256-verified binaries — see [`adopting-assay.md`](adopting-assay.md).
2. **Upgrade** (`assay:upgrade-assay`) moves the umbrella line and re-pins the artifact tags to the
   target composition, runs migrations, and shows release notes.
3. **Verify** with `deskversion --root <repo>` (known at the new umbrella) and `deskpins --check`
   (the pin file still conforms).
