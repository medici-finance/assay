# Release-time spec header stamp — staged workflow change (assay#1192)

`spec/brief-v1.md`'s `Describes reference implementation:` line names a
`statusgen` version. Edited by hand, it drifted: `#302` freshened it once, and
it drifted again (`v0.22.0`, then `v1.0.12`) while releases moved on. This is
the second half of brief-13's Task 4: "a release-time check ... that the spec
header matches the version being cut."

**Ruling (option a, `stamp`).** The maintainer chose on `#1192` that the
release writes the tag being cut into the header, the same way it already
stamps the plugin manifest, and `check-spec-header-version.sh` stays as the
proof that the stamp ran. The header therefore means "this spec shipped with
release X". It does not mean someone re-read the spec against X; the spec text
is kept honest by `TestBriefV1SchemaCoverage` and `statusgen conform`.

## What is in this directory

- `stamp-spec-header-version.sh <vX.Y.Z> [spec-file]` — the stamp. Rewrites
  the first `vX.Y.Z` token on the first `Describes reference implementation:`
  line to the tag, leaves every other byte alone, and reads the result back
  through the check. Idempotent (a header already at the tag is not
  rewritten). Fails closed with exit 2, writing nothing, on a malformed tag,
  an unreadable file, a missing header line, or a header line with no token.
- `check-spec-header-version.sh <vX.Y.Z> [spec-file]` — the proof. Exit 0 on a
  match, 1 on a mismatch, 2 when it cannot check. It is an adapter onto
  `deskchangelog check-spec-header` (`tools/desk/cmd/deskchangelog`), run from
  `PATH` or built from `tools/desk`; without either it exits 127.
- `stamp-spec-header-version.test.sh` and `check-spec-header-version.test.sh` —
  offline suites in bash + grep + awk; they drive the check through its adapter,
  so they also need `deskchangelog` on `PATH` or a Go toolchain. Run them directly:

  ```
  bash tools/release/stamp-spec-header-version.test.sh
  bash tools/release/check-spec-header-version.test.sh
  ```

- `bundles.py` / `bundles_test.py` — the provenance-bundle lane of the release
  job (#3094), unrelated to the spec stamp. The release attests every published
  asset in one step; `bundles.py` refuses unless the bundle has the shape
  `deskinstall` verifies and names the sha256 of every listed asset (and every
  `checksums.txt` file is listed with its recorded bytes), writes it beside each
  asset as `<asset>.sigstore.json`, and refuses to publish a release on which
  any asset or bundle is missing. It verifies no signature; `deskinstall` and
  `gh attestation verify` do. Offline fixtures:
  `python3 -B -m unittest discover -s tools/release -p bundles_test.py -v`.
- `sbom.py` / `sbom_test.py` — the release SBOM (#4716). The
  artifacts job writes `sbom.spdx.json`, an SPDX 2.3 JSON document built from
  the Go build information `go version -m` reads out of each statusgen and
  qualgen binary and each desk-tools tarball. It holds one package per asset,
  carrying the asset's sha256, and one package per linked Go module plus the
  Go toolchain (`pkg:golang/stdlib`), each with a `pkg:golang` purl and joined
  to its asset by `DEPENDS_ON`. The output is deterministic: the creation time
  is the source commit's time. The job refuses, writing nothing, when an asset
  is missing or carries no Go build information. The SBOM is checksummed,
  uploaded and attested like every other asset. Offline fixtures (one builds a
  real binary when `go` is on PATH):
  `python3 -B -m unittest discover -s tools/release -p sbom_test.py -v`.
- `release.yml.patch` — the exact delta to `.github/workflows/release.yml`.
- `release.yml.proposed` — the whole intended `release.yml` (the patch applied
  to the live file as it stood at staging, plus a banner). Kept for review;
  apply the patch, not this file.

Both scripts default `spec-file` to `spec/brief-v1.md` relative to the working
directory; the workflow runs them from the repo root.

## What the staged change does

It mirrors the plugin manifest stamp step by step, in three places:

1. **`guard` job, tag-push path only.** A pushed tag can never be re-pointed,
   so the tagged tree must already name the tag. The check refuses a stale
   header here, before the human approval, next to the plugin manifest gate.
2. **`release` job, right after "Stamp the plugin manifest version to the
   release tag".** On dispatch: stamp the header, run the check, and commit
   the change locally as `github-actions[bot]`, so the tag lands on that
   commit. On a tag push: the check alone. On a dry run: stamp and commit
   locally, push nothing — the same as the manifest stamp.
3. **`changelog-roll` job, beside the post-cut plugin manifest bump.** Leaves
   the default branch's header on the next patch version (the same floor rule:
   a header a human already moved above the cut is left alone), in the same
   commit. A dispatch of exactly that version then finds the header already
   stamped and commits nothing, so in the steady state the tag lands on the
   default branch's own commit.

Nothing else in `release.yml` changes.

## Maintainer apply step

No App identity edits `.github/workflows/` (the server refuses a workflow
write without the `workflows` permission), so a maintainer applies the change
by hand, from the repository root on an up-to-date `main`:

```
git apply --check tools/release/release.yml.patch   # must print nothing
git apply tools/release/release.yml.patch
bash tools/release/stamp-spec-header-version.test.sh
bash tools/release/check-spec-header-version.test.sh
```

Commit `.github/workflows/release.yml` by PR. If `git apply --check` refuses,
the live file has moved since staging: apply the three additions by hand from
`release.yml.patch` (each is self-contained and anchored next to the matching
plugin manifest step), not by copying `release.yml.proposed` over the live
file, which would revert whatever moved.

The header on `main` already names the latest published release, so the first
dispatched release after the apply commits a one-line stamp to the tag being
cut, and the roll moves `main` onto the next patch from then on.

## After the next release

The release's tag names its own version in `spec/brief-v1.md`:

```
git show <tag>:spec/brief-v1.md | grep 'Describes reference implementation'
```

Once that holds, `#1192` can close, citing the ruling.

## Release Go cache

`go_cache.py` is an internal, standard-library workflow adapter. The live release
workflow imports it for restore and pack; `go_cache_test.py` runs through the
existing desk regression floor. See [the design and measurements](../../docs/release-build-cache.md).

The cache and batching delta is mirrored in `release.yml.proposed` and
`ci/staged-workflows/release.yml`, preserving each proposal's other changes.
The old changelog proposal is a historical snapshot, not a promotion source.
No whole-file proposal should replace the current live workflow.
