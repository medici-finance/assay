# Release images and consumer pins

The umbrella release is the producer of the binaries AND its Docker images.
Re-pinning repositories and cells is a separate consumer operation: it selects
a published umbrella release, verifies checksums and copies immutable references.
It never builds, pushes, or dispatches an image workflow.

The producer is staged in `ci/staged-workflows/release.yml` and
`ci/staged-workflows/docker-publish.yml`; both require maintainer promotion
before this contract is live. The currently live image workflow still uses the
legacy component-tag/manual triggers.

## Producer order

1. Run the existing version, source and test gates. Stamp the release source,
   build every binary platform, and checksum the artifacts. Capture the exact
   prepared source revision and its archive; preserve any local stamp commits
   in a Git bundle for the final tag.
2. In the release-owned image job, verify the build inputs and use those exact
   Linux statusgen and desk-tools bytes for all images. Build desk-tools first,
   desk-base from that newly built version, and the five desks from that base.
   Build assay-harness with the same binaries and prepared plugin source, and
   verify-runner from the release statusgen plus its digest-pinned Go base.
3. Check statusgen's version in all nine images; run the existing layer-secret
   gate on the shared desk base and all five desks. All checks precede the first
   push. Publication retains the approved `release` Environment.
4. Publish the versioned images and read their registry digests. Refuse an
   existing image tag, an indeterminate registry lookup, or a missing/wrong
   digest. Attach the references and their checksums to the draft GitHub release
   before its single immutable publish. A partial registry push is not atomic:
   if a later push fails, no complete image manifest or GitHub release is
   published; recover by fixing forward with the next version.

Dry runs build and check the images but push none, write no pinnable-reference
files, cut no tag and publish no GitHub release. CI artifacts may carry build
inputs between jobs; those are not published release assets.

## Published reference artifacts

`release-images.json` has schema `release-images-v1`, the umbrella `version`,
`source_revision`, release `repository`, and exactly nine `images` records:

- desk-tools, assay-harness and desk-base;
- intake-desk, worker-desk, pr-review-desk, verify-desk and the-desk;
- verify-runner.

Each record has `name`, `platform` (`linux/amd64`) and a complete
`<repository>:<umbrella-tag>@sha256:<registry-manifest-digest>` reference.
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
and move the accompanying tool-version marker in the same change.

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
