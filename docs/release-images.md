# Release images and consumer pins

The umbrella release is the producer of the binaries AND its Docker images.
Re-pinning repositories and cells is a separate consumer operation: it selects
a published umbrella release, verifies checksums and copies immutable references.
It never builds, pushes, or dispatches an image workflow.

The producer is `.github/workflows/release.yml`; its staged twin is kept
identical. After preparation with read-only repository permissions, one
`release` job on the dedicated
Docker-capable release runner owns image, tag and GitHub release publication.
Its single protected `release` Environment approval authorizes the whole
sequence. There is no separate image-publishing workflow or second approval.
Only that job has both `contents: write` and `packages: write`; the changelog
roll remains a separate downstream job under its own existing identity.

## Producer order

1. Run the existing version, source and test gates. Stamp the release source,
   build every binary platform, and checksum the artifacts. Capture the exact
   prepared source revision and its archive; preserve any local stamp commits
   in a Git bundle for the final tag.
2. In the approved release job, verify the build inputs and use those exact
   Linux statusgen and desk-tools bytes for all images. Build desk-tools first,
   desk-base from that newly built version, and the five desks from that base.
   Build assay-harness with the same binaries and prepared plugin source, and
   verify-runner from the release statusgen plus its digest-pinned Go base and
   the Debian packages (Python libraries and row tools) pinned in
   `images/verify-runner/Dockerfile`.
   verify-runner is multi-arch: it is built once for `linux/amd64` and once for
   `linux/arm64`, each from the matching `statusgen-linux-<arch>` release
   binary, and each build's OS/architecture is read back before any check. The
   arm64 build runs under emulation, so before any image is built the release
   job registers it with the runner's Docker engine from a digest-pinned
   `tonistiigi/binfmt` installer, then proves a `linux/arm64` container of the
   runner's own base image reports `aarch64`, and stops with an error if not
   ([issue 4784](https://github.com/medici-finance/assay/issues/4784)). That
   privileged installer runs before the registry login and carries no
   credential. The other eight images stay `linux/amd64`.
3. Check statusgen's version in all nine images, and in both runner
   platforms. Execute the release-owned
   `images/verify-runner/check.py` inside each built runner platform: import PyYAML and
   jsonschema, parse YAML, validate a Draft 2020-12 schema, and prove that a
   valid instance passes and an invalid one raises a validation error. It also
   checks the cluster backend's baked entrypoints (`stage-source.sh`,
   `stage-cache.sh`, `witness.sh` in `/usr/local/bin`): `/usr/local/bin` is a
   `root:root` directory that neither its group nor others can write, and each
   script is a `root:root` regular file, executable, with no set-user-id or
   set-group-id bit, that neither its group nor others can write, and,
   executed directly with no argument, prints its usage and exits 2. It runs
   the row tools `jq`, `shellcheck` and `tmux` from `PATH`, so a release whose
   runner lacks one stops before any push. The probe
   prints Python and distribution versions into the release build log. It uses
   no pulls, network, mounts or credentials; it runs as user 1000:1000 with a
   read-only root, all capabilities dropped, no new privileges, default seccomp
   and bounded memory, CPU, process count and temporary storage. Run the existing layer-secret
   gate on the shared desk base and all five desks. All checks precede the first
   push. A failure stops the same job before any tag or GitHub release write.
4. Serialize the publication job on the resolved umbrella version before registry
   absence checks, including dispatches with or without a leading `v`.
   Publish the versioned images and read their registry digests. Refuse an
   existing image tag, an indeterminate registry lookup, or a missing/wrong
   digest. verify-runner is pushed as two per-platform tags
   (`<umbrella-tag>-linux-amd64`, `<umbrella-tag>-linux-arm64`, both
   absence-checked with the rest), then joined under the umbrella tag as one
   image index naming those two pushed digests. The index is read back by its
   digest and must list exactly those two platforms and digests. Attach the references and their checksums to the draft GitHub release
   before its single immutable publish. A partial registry push is not atomic:
   if a later push fails, no complete image manifest or GitHub release is
   published; recover by fixing forward with the next version.

Dry runs use the same one-approval job: they build and check the images but
push none, write no pinnable-reference files, cut no tag and publish no GitHub
release. CI artifacts may carry build inputs between jobs; those are not
published release assets. The publisher extracts a fresh prepared-source build
context separately from its binary downloads and Git metadata, and removes that
context and its temporary Docker credential directory on shell exit.

A retry reruns the complete publication job. Once an image tag or release tag
has been written, the existing no-overwrite checks may refuse that version;
fix forward rather than bypassing those checks. Changelog rolling remains
independently retryable after a successful publication. This change affects
future workflow runs; an already-running release keeps its original job graph.

## Published reference artifacts

`release-images.json` has schema `release-images-v1`, the umbrella `version`,
`source_revision`, release `repository`, and exactly nine `images` records:

- desk-tools, assay-harness and desk-base;
- intake-desk, worker-desk, pr-review-desk, verify-desk and the-desk;
- verify-runner.

Each record has `name`, `platform` and a complete
`<repository>:<umbrella-tag>@sha256:<registry-manifest-digest>` reference.
`platform` is `linux/amd64` for eight images; verify-runner records
`linux/amd64,linux/arm64` and its digest is the image index (manifest list)
digest, not a per-platform one. One reference then serves both host
architectures: the engine selects the native platform, and the index bytes the
digest covers name each per-platform manifest digest, so the pin stays
immutable for both. Records from releases before the multi-arch runner carry
`linux/amd64` for verify-runner; the contract still accepts them when a consumer
validates an older release, while a new release must record both platforms.
`verify-runner-image.txt` contains exactly its matching reference plus a newline.
Both files are listed in `checksums.txt`. `tools/release/images_contract.py`
rejects missing/duplicate images, wrong tags, repositories, platforms or source
revisions, bare mutable tags, and a disagreeing runner reference.

The reference digest is a registry manifest digest, never a local image config
id. These checks establish integrity and version alignment, not signed build
provenance or an atomic transaction across GitHub and GHCR.

## Consumer operation

Download the selected release's checksums and reference artifacts, verify the
artifact bytes, validate the manifest against the selected version, source
revision and release repository, then update repository/cell pins together.
For a verify-runner consumer, copy `verify-runner-image.txt` into its image lock
and move the accompanying tool-version marker in the same change. Do not force
`--platform linux/amd64` when running a multi-arch runner reference: let the
engine pick the host's platform, or pass the host's own platform, so an arm64
host runs the native image instead of an emulated one.
The Kubernetes desk package keeps the selected release's `release-images.json`
byte for byte beside its manifests; its offline check pins that record's sha256,
validates it with this contract and requires every role reference to equal the recorded one
(`containers/k8s/README.md`, "Image provenance").

If a required image/reference is absent, report the producer release incomplete
for that consumer. Do not repair it by building an image in the re-pin process.
Releases published before this producer lands cannot be enriched in place: use
the established legacy references for those releases or select a later complete
release. Local root-owned tool installation remains a separate operator act.

## Validation

```sh
python3 -B -m unittest discover -s tools/release -p images_test.py -v
sh containers/scripts/layer-secret-scan.test.sh
sh containers/scripts/layer-secret-scan.mutate.sh
```

The image-producer tests use fake Docker and check real metadata and artifact
bytes without registry access. The scan suites separately prove real-key and
hidden-layer detection, fail-closed errors and exact-label precision. The
complete release workflow must still be exercised after maintainer promotion.

The changelog-roll job uses the same board-writer App through the token action's
`client-id` input. Configure its public client ID as the `BOARD_APP_CLIENT_ID`
variable in the protected `board-writer` environment; the existing
`BOARD_APP_PRIVATE_KEY` secret remains the signing key. The client ID is not the
numeric App ID.

## Runner Debian dependencies

The Dockerfile owns the direct Debian bookworm package pins. Their risk assessment
is recorded on [issue 3469](https://github.com/medici-finance/assay/issues/3469#issuecomment-6272973965).
The row tools `jq`, `shellcheck` and `tmux` are separate direct Debian pins, added because
rows run offline and cannot fetch a tool they call
([issue 4684](https://github.com/medici-finance/assay/issues/4684),
[issue 4318](https://github.com/medici-finance/assay/issues/4318)). Issue 3469 does not
cover them; their risk summary (publisher, exact versions and release dates, permissions,
persistence, transitive packages) is recorded on #4771. The `jq` pin is a
`bookworm-security` version, which the archive drops once superseded, so that build fails
closed and needs a reviewed pin update sooner than the others.
Apt resolves transitive dependencies from the base suite during the approved build;
`/usr/local/share/verify-runner/packages.txt` records every installed Debian package
and version in the immutable image, including transitive dependencies and Python.
The capability check logs Python distribution versions separately. This is a record
of the built image, not a claim of bit-for-bit package-resolution reproducibility.
An unavailable direct pin fails the build and requires a reviewed update; runtime
package installation is never a recovery path. The release manifest remains the
existing versioned image-reference contract, without a second dependency manifest
for consumers to reconcile.

The source-side fake-Docker tests run in pull-request CI. They prove the capability
check uses the release-owned source payload, keeps its declared resource bounds,
precedes all pushes, and stops publication on a simulated import-error exit. The
two error modes cover the same producer path; they do not independently execute
either library. The real probe rejects the deficient v1.0.50 image. A corrected-image
pass still needs the approved release build; source tests do not qualify a runner or replace fresh
released-image pinning and independent qualification.

## Runner git

The admitted runner calls `/usr/bin/git` and needs `git --attr-source`, which
arrived in git 2.41; the Go base image's Debian git is 2.39.5. The runner image
therefore builds git in its own `gitbuild` stage from the same pinned upstream
tarball and sha256 as `containers/base/Dockerfile`, purges the distro git, and
installs the built one over `/usr`. The source-built git is not a Debian
package, so it does not appear in `packages.txt`. The last layer runs
`containers/scripts/git-floor-check.sh` under the runner's fixed `PATH`, so an
image whose git is below the floor fails the build. The image stays offline at
runtime: the tarball is fetched only during the build. A git bump changes the
pair in both Dockerfiles in one change.

## Runner Go module cache

Rows run with no network, a read-only root and `GOPROXY=off`, so the image bakes
the Go modules they resolve: its Dockerfile runs `go mod download` for
`tools/desk` and `statusgen` from the release's own `go.mod` and `go.sum`, and
records those roots, one per line, in
`/usr/local/share/verify-runner/go-module-roots.txt`. The cache is pinned by the
image digest like everything else in the image; nothing is mounted or fetched at
row time.

The cache matches the source the release was built from, not a tree verified
later. When main moves a requirement after a release, a row that builds that
module in an older image stops on module resolution before its check runs
(#4683). A container-netns `statusgen verifyrun` reads the roots file, compares
each root's `go.mod` in the verified tree with the cache before any row, and
prints the gap once. A row that exits non-zero naming a missing module is
recorded could-not-run with `image module cache predates go.sum (<module> <want>
vs <have>)` rather than as a failure; rows that do not touch a missing module
keep their own verdict. The remedy is to run in the image of a release whose
`go.mod` and `go.sum` for those roots match the verified tree, normally the
first release that contains the verified commit. A requirement bump on main
that no release contains yet cannot be verified offline until the next release.
