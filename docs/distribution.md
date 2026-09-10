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

### One tag, one tree — and the exemption marker

`statusgen --lint` enforces **one tag, one tree**: the artifacts share a release tag so a single
tree is never read by tools cut from different releases. Every per-artifact line participates in
that comparison (the umbrella line does not — it names a composition, not an asset); if two
artifacts carry different tags the lint PROBLEMs.

Two legitimate states need to opt one line out of the comparison without hiding it:

- a **guard binary frozen on an earlier tag** of the same tool family by a recorded maintainer
  ruling — still a `desk-tools`-family artifact, just an older tag; and
- a **separate-repository artifact on its own release cadence** that the umbrella release never
  ships.

Declare the exemption **per line** with a trailing comment:

```
reconciler v2.4.1 <sha256>  # same-tag: exempt — separate release cadence
desk-tools-guard v0.13.0 <sha256>  # same-tag: exempt — frozen by maintainer ruling
```

The marker is the token `same-tag: exempt` in the line's trailing comment; the text after it is a
free-form reason kept for the human record. An exempt line stays a fully valid, lint-visible pin
(`deskpins --check` still validates its shape and sha256) — it is only removed from the same-tag
grouping. Every **non-exempt** artifact must still share one tag: a genuine, undeclared mixed-tag
state still PROBLEMs, and the exempt line's off-tag never appears in that message.

## Report packs

Some released tools are **report packs** — periodic reporting instruments (the board view, the
quality trend view, and more to come) that an adopter installs exactly like the board
generator: pin a `<artifact>-<platform>` line, run one `<tool> init`, and obtain a committed
report without building anything from source. A report pack is a member of the umbrella release
(criterion 1 above), emits its own generated workflow and config via `init`, keeps its committed
output single-writer (a pull request renders and discards; only push-to-main writes, behind a
writer env var the tool enforces), and loads every operator value from configuration. The one
exception the contract admits is the **producing repository**, which self-hosts the tool from its
own source because a released binary would lag the pull request changing it. The normative
contract — the four criteria, the producing-repo seam, and how the conformance sweep reads a pack
— is [`report-packs.md`](report-packs.md); `qualgen` (the quality view) is the reference pack.

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
