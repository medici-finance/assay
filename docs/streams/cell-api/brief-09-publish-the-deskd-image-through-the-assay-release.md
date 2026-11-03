---
brief: assay:assay:cell-api:09
title: Publish the deskd image through the assay release
why: "cell-api/08 builds the deskd image locally, but only the umbrella release produces an image reference anyone may pin, and its image set has no deskd. Adding deskd to that producer gives the service one approved, digest-pinned publication path, so downstream deployments can stop building or publishing deskd on their own and cell-api/05 has a real release record to pin."
wave: 1
depends: ["cell-api/08"]
unblocks: ["cell-api/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: "Release-image publishing is a supply-chain surface, so the gate was weighed rather than defaulted. All four risk answers are no for THIS brief: it changes the release producer's code, contract and docs and adds one image to the set; it cuts no release, pushes nothing, dispatches no workflow and handles no credential, and the change reverts by an ordinary PR. The act that is irreversible, a published image tag, remains the release workflow's single protected human approval, which this brief neither changes nor bypasses, and the default contract it tightens fails closed. A model gate suffices because that human approval, the pre-push checks and the layer-secret gate stand behind every publication independently of this review."
issues: []
schema: brief-v2
version: 1
id: 9272a1f6-cbd3-470d-bda5-e3f98347c19e
authored: 2026-11-02 by worker-desk
exec-tier: strong
exec-tier-why: The producer, its validation contract, a pinned pre-change consumer record and an unwritable live workflow must stay consistent together; a contract default chosen the wrong way passes every existing test while silently dropping deskd from what the release step enforces.
domain: complicated
outcome: none
sources:
  - docs/streams/cell-api/README.md
  - docs/streams/cell-api/brief-05-package-the-deskd-image-and-generic-kubernetes-service-profile.md
  - docs/streams/cell-api/brief-08-build-the-deskd-binary-and-image-from-public-source-through-the-makefile.md
  - docs/release-images.md
  - containers/k8s/README.md
  - ci/staged-workflows/README.md
  - tools/release/README.md
  - https://github.com/medici-finance/assay/issues/3669
  - freshness-checked 2026-11-02 @ f05c320919d4fc2766f0ba69ff66776748818de9
consumers:
  - "tools/release/images.py: follow-up cell-api/09"
  - "tools/release/images_contract.py: follow-up cell-api/09"
  - "tools/release/images_test.py: follow-up cell-api/09"
  - "docs/release-images.md: follow-up cell-api/09"
  - "docs/docker.md: follow-up cell-api/09"
  - "containers/k8s/check.py: follow-up cell-api/09"
  - "containers/k8s/README.md: follow-up cell-api/09"
  - "deskd/README.md: follow-up cell-api/09"
  - "changelog/deskd-release-image.md: follow-up cell-api/09"
  - "containers/k8s/release-images.json: out-of-scope (the pinned pre-change v1.0.52 release record, kept byte for byte; it is read, never edited)"
  - "containers/k8s/components/compatibility-cellctl/candidate-release-images.json: out-of-scope (the pinned pre-change v1.0.65 candidate release record, kept byte for byte; it is read, never edited)"
  - ".github/workflows/release.yml: out-of-scope (the producer reads its image set from images.py, so no functional workflow change is needed; its 'nine images' comments and dry-run message go stale and are a maintainer's edit, because no App identity writes workflows)"
  - "ci/staged-workflows/release.yml: out-of-scope (held byte-identical to the live workflow by the release tests; it changes only with the live file)"
  - "images/deskd/: follow-up cell-api/08 (the image definition is consumed unchanged)"
  - "deskd image artifact pin and service profile: follow-up cell-api/05"
  - "downstream deployment repository's deskd image pin: out-of-scope (a follow-on outside this repository re-pins it onto the first published release record that carries deskd; no brief is authored here)"
  - "the sibling console repository's own deskd release workflow: out-of-scope (a follow-on outside this repository retires it once the downstream pin has moved; no brief is authored here)"
---

# Brief 09 — Publish the deskd image through the assay release

## Context

files: `tools/release/images.py`, `tools/release/images_contract.py`, `tools/release/images_test.py`, `docs/release-images.md`, `docs/docker.md`, `containers/k8s/check.py`, `containers/k8s/README.md`, `deskd/README.md`, `changelog/deskd-release-image.md` (planned)

facts:
- pin: public main f05c32091, inspected 2026-11-02. cell-api/08 is implemented: `images/deskd/Dockerfile` and its `Dockerfile.dockerignore` exist, the build context is `deskd/`, the revision comes from `ARG REVISION`, the final stage is `FROM scratch` with `USER 65532:65532` and `ENTRYPOINT ["/deskd"]`. The image has no shell and no statusgen, and deskd has no version flag.
- producer: `tools/release/images.py` holds `NAMES` (nine: desk-tools, assay-harness, desk-base, five desks, verify-runner). `build_images` builds every image for `linux/amd64` (verify-runner also `linux/arm64`), runs `check_version` (statusgen `--version` inside each image) on all of them, runs `containers/scripts/layer-secret-scan.sh` on desk-base and the five desks, and only then, when publishing, pushes and writes `release-images.json` and `verify-runner-image.txt`. A dry run builds and checks and pushes nothing.
- contract: `tools/release/images_contract.py` `validate()` requires `set(seen) == NAMES`; its `runner_platforms` argument already carries one compatibility default (an older amd64-only runner record). The live release step calls `validate(..., runner_platforms=(RUNNER_PLATFORM,))` and passes nothing else.
- consumers: `containers/k8s/check.py` calls `validate()` at two sites, on two different pinned nine-image records. `recorded_images()` (used by `--base`, and again by `--compatibility-cellctl`) validates the v1.0.52 record `containers/k8s/release-images.json`. `candidate_images()` (used by `--compatibility-cellctl`) validates the v1.0.65 record `containers/k8s/components/compatibility-cellctl/candidate-release-images.json`, pinned by `candidate.json`'s `record_sha256`. Adding deskd to the contract's required set breaks both sites unless each says which set it accepts.
- workflows: no App identity writes `.github/workflows/` (`ci/staged-workflows/README.md`), and `images_test.py` asserts the live `release.yml` and `ci/staged-workflows/release.yml` are byte-identical. The live workflow text says "all nine images" in comments and in its dry-run message only; the set itself comes from `images.py`.

layering: No new component. The release producer already owns image publication and its contract already owns validation. This brief adds one entry to each and one explicit compatibility option, and leaves the workflow, the single approval and the publication order unchanged.

single-point-of-failure: the release workflow's single protected `release` Environment approval is the one control between a merged producer change and a published deskd tag. Behind it, on different signals: the pre-push checks and the layer-secret gate (Verify 1) stop a bad image before any registry write, and the contract's strict default (Verify 2) stops the release's own validation step from accepting a record without deskd. Neither replaces the approval; both run inside it.

design-fit:
  owner: the umbrella release producer (`tools/release/images.py`) and its consumer contract (`tools/release/images_contract.py`)
  contract: none — the `release-images-v1` record shape is unchanged; one name is added to its required set
  retires: []
  weight: verbs +0; flags +0; refusals +0; rule-text +0 (one image in the existing set, one named compatibility value beside the existing runner one; no new CLI verb or flag)
  why-add: n/a

All planned paths and named tests are implementation deliverables. The source pin proves
the starting point, not that these deliverables exist or were tested.

## Read first

`docs/release-images.md` (producer order, published reference artifacts, consumer
operation), `tools/release/README.md`, `ci/staged-workflows/README.md`,
`containers/k8s/README.md` "Image provenance", [brief 08](brief-08-build-the-deskd-binary-and-image-from-public-source-through-the-makefile.md)
(Task 1 and its interface section) and [brief 05](brief-05-package-the-deskd-image-and-generic-kubernetes-service-profile.md)
Task 1 (the pin this brief makes possible).

## Ground rules

- Work in an owned branch. No merge, ready flip, image push, registry login, release tag, workflow dispatch or cluster contact.
- Do not edit `.github/workflows/` or `ci/staged-workflows/`. A change the release needs there is NEEDS_CONTEXT, never a reason to weaken a test that holds the two copies equal.
- Consume `images/deskd/` as cell-api/08 delivered it. A needed Dockerfile change goes through 08's contract test, not a second definition here.
- Public source and fixtures are synthetic. Stop at implemented; independent verification remains distinct.

## Task

1. **Producer.** Add `deskd` to `NAMES` in `tools/release/images.py`.
   - Build it for `linux/amd64` from `images/deskd/Dockerfile` with the build context
     `deskd/` (not the repository root), passing the prepared source revision as
     `REVISION`. Build it before any push, in the same approved job, like the others.
   - deskd carries no statusgen, so `check_version` does not apply to it. Give it its own
     pre-push check instead: the image's configured user is numeric and non-root, its
     `org.opencontainers.image.revision` label equals the prepared source revision, and
     the binary runs with `--network none` and prints its usage. A mismatch stops the
     job before any registry write.
   - Run the existing layer-secret gate on deskd before the first push.
   - On publish, write its record with `platform: linux/amd64` and a registry manifest
     digest, like the other amd64 images. A dry run builds and checks deskd and pushes
     nothing.

2. **Contract.** In `tools/release/images_contract.py`, `NAMES` becomes the ten-image set
   including deskd. Add `LEGACY_NAMES`, the nine-image set before this change, and a
   `names` argument to `validate()` that accepts one or more sets.
   - The default is the current set only, so the live release step's existing call (which
     passes `runner_platforms` alone and which no App identity may edit) refuses a record
     without deskd with no workflow change.
   - A consumer of an older record opts in by name: both `validate()` calls in
     `containers/k8s/check.py` (`recorded_images()` and `candidate_images()`) pass the
     current and legacy sets. Any other reader found at
     pickup gets the same explicit choice; none silently inherits leniency.
   - deskd's reference is `ghcr.io/<repository>/deskd:<umbrella-tag>@sha256:<digest>`,
     validated like the other per-repository images.

3. **Tests.** In `tools/release/images_test.py`, deliver `test_deskd_release_record`
   (planned) and `test_deskd_contract_compatibility` (planned), and update the existing
   fake-Docker expectations for the tenth image. Give the two named tests no docstring:
   under `unittest -v` a docstring moves `... ok` onto a second line, which rows 1 and 2
   would not match. Keep every existing test, including the
   byte-identity check of the two workflow copies, passing unchanged.

4. **Documentation.** Each file below carries the phrase Verify row 4 checks in it, and
   none of those phrases is present at the authoring pin.
   - `docs/release-images.md`: producer order, the image list naming `deskd`, records
     "exactly ten" images, the deskd check, and that deskd has no statusgen.
   - `docs/docker.md`: the "all nine images" line names `deskd` and no longer says
     "all nine images".
   - `containers/k8s/README.md` "Image provenance": the contract sentence names
     `LEGACY_NAMES`, the set its pinned records are validated with, explicitly allowed.
   - `deskd/README.md`: the local image stays local-only, and the umbrella release
     publishes deskd, whose reference that release's `release-images.json` records.
   - Add `changelog/deskd-release-image.md` (planned).

## Interface and acceptance contract

The record shape (`release-images-v1`) is unchanged; a release cut after this lands
carries one more image record. This brief publishes nothing: the first deskd reference
exists only after a maintainer cuts a release through the approved workflow.

This brief unblocks, and does not implement:

- cell-api/05's artifact pin, which keeps a published release record that carries deskd
  and accepts only its deskd reference;
- a follow-on outside this repository that re-pins the downstream deployment repository
  onto the assay-published deskd image, from that published record;
- a follow-on outside this repository that then retires the sibling console
  repository's own deskd release workflow, so that one producer remains.

No brief is authored here for either follow-on. The live workflow's stale "nine images"
comment and dry-run text is a maintainer's edit to both workflow copies together.

## Verify

Rows 1–4 are offline: they run on a clean checkout of merged main with
`KUBECONFIG=/dev/null`, fake Docker and no registry, cluster or forge contact. Each named
test must be delivered and run, with a recorded failing control before the restored
pass. Zero selected tests or a skipped test is failure.

Row 5 is a host row. It needs the tools `containers/k8s/README.md` lists (Kustomize,
kubeconform, gitleaks, the cached Go YAML module). Without them it is could-not-check,
never pass.

For row 6, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed.

The whole-tree `statusgen --root . --lint` is the repository's CI gate and can be
reddened by an unrelated brief, so it is not a row here. No row proves a real
publication: that needs an approved release, and the downstream follow-on dereferences
its record.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `o=$(mktemp) && python3 -B -m unittest discover -s tools/release -p images_test.py -v > "$o" 2>&1 && grep -E '^test_deskd_release_record .*\.\.\. ok$' "$o"` | exit 0; the existing suite passes and `test_deskd_release_record` is ok. With fake Docker, a dry run builds deskd from `images/deskd/Dockerfile` with the `deskd/` context and the source revision, runs its user/label/usage check and the layer-secret gate, and pushes nothing. A publish writes a deskd `linux/amd64` record with a registry manifest digest only after every check. Each of these stops before any push or metadata write: a root or non-numeric user, a revision label that differs from the source, a binary that does not print usage, and a deskd layer-secret finding. Removing deskd from `NAMES` in a scratch copy fails the test. At the pre-delivery base the test does not exist and the grep fails. |
| 2 | check:ci +mutation | `o=$(mktemp) && python3 -B -m unittest discover -s tools/release -p images_test.py -v > "$o" 2>&1 && grep -E '^test_deskd_contract_compatibility .*\.\.\. ok$' "$o"` | exit 0; `test_deskd_contract_compatibility` is ok. With the default arguments, `validate()` refuses a ten-image record missing deskd, a nine-image record, a deskd reference with a bare tag, the wrong repository or a non-amd64 platform, and accepts a complete ten-image record. Called with the current and legacy sets, it accepts the real bytes of both pinned nine-image records, `containers/k8s/release-images.json` (v1.0.52) and `containers/k8s/components/compatibility-cellctl/candidate-release-images.json` (v1.0.65), and still refuses a duplicate or unknown name. Making the default lenient in a scratch copy fails the test. |
| 3 | check:ci +neighbour | `o=$(mktemp) && python3 -B -m unittest tools.release.images_test tools.release.ci_step_test -v > "$o" 2>&1 && grep -E '^test_release_has_one_approval_before_all_publication .*\.\.\. ok$' "$o" && tail -n 3 "$o" \| grep -E '^OK'` | exit 0; both release suites pass, `docs/release-images.md`'s validation command unchanged. The one-approval test, which also asserts the live and staged `release.yml` are byte-identical and that no separate publish workflow exists, is ok. |
| 4 | check +dereference | `python3 -B -c "import sys; sys.path.insert(0, 'tools/release'); import images, images_contract as c; n = set(images.NAMES); assert 'deskd' in n and n == set(c.NAMES) and len(n) == 10, sorted(n); assert set(c.LEGACY_NAMES) == n - {'deskd'}, sorted(c.LEGACY_NAMES); print('names ok')" && grep -F -- 'exactly ten' docs/release-images.md && grep -F -- 'deskd' docs/release-images.md && grep -F -- 'deskd' docs/docker.md && ! grep -F -- 'all nine images' docs/docker.md && grep -F -- 'LEGACY_NAMES' containers/k8s/README.md && grep -F -- 'release-images.json' deskd/README.md` | exit 0; prints `names ok`. The producer and contract name the same ten images, and the legacy set is exactly those without deskd. Each documentation probe reads one file and checks the phrase Task 4 prescribes for it, so a delivery that skips any one file fails the row. At the pre-delivery base the first assertion fails, and so does each of the six file probes on its own. |
| 5 | check +neighbour | `sh containers/k8s/check.sh --base && sh containers/k8s/check.sh --base --self-test && o=$(mktemp) && { sh containers/k8s/check.sh --compatibility-cellctl --self-test > "$o" 2>&1; true; } && grep -Fx -- 'checked-clean: compatibility mutation fixtures; actual candidate remains a separate gate' "$o" && ! grep -F -- 'release-images-v1' "$o" && python3 -B -c "import sys; sys.path.insert(0, 'containers/k8s'); import check; print('candidate record accepted:', len(check.candidate_images()), 'role refs')"` | exit 0 with the listed tools installed; prints `candidate record accepted: 9 role refs`. Both `validate()` sites in `containers/k8s/check.py` still accept their pinned nine-image records through the release contract: `--base` covers `recorded_images()`, and the direct `candidate_images()` call covers the v1.0.65 candidate record (it also re-reads the v1.0.52 record). The image-provenance and compatibility mutation fixtures still fire, and neither contract refusal (both name `release-images-v1`) appears in the compatibility self-test's output. That self-test's own exit is not read: at the authoring pin it ends `checked-failed: prepared launch identity missing`, because the template's adopter-owned preparation values are unresolved, which no release-contract change affects. Control: in a scratch copy whose contract requires deskd while `check.py` passes no legacy set, both `recorded_images()` and `candidate_images()` raise. A missing tool is could-not-check, never pass. |
| 6 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && git diff --quiet "$ASSAY_DELIVERY_BASE_SHA" "$ASSAY_DELIVERY_SHA" -- .github/workflows ci/staged-workflows containers/k8s/release-images.json containers/k8s/components/compatibility-cellctl/candidate-release-images.json images/deskd && o=$(mktemp) && statusgen --root . --consumers --brief cell-api/09 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA" > "$o" 2>&1 && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved' "$o"` | exit 0 only for the named landed implementation delivery and its first parent. The delivery changed no workflow copy, either pinned record or the deskd image definition. Record exact corroborated, disproved and unchecked counts, with 0 disproved. Unchecked consumers stay held, and the non-path entries (cell-api/05's pin and the two follow-ons outside this repository) are review obligations. Unset inputs exit 1. |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| deskd is pushed before its checks or its layer-secret gate, or with a credential in a layer | row 1; the release's protected approval stays a separate human layer |
| A deskd image built from the wrong context, revision or user is published | row 1 (context, revision label and user check before any push) |
| The release's own validation step accepts a record without deskd because the contract default is lenient | row 2's default-arguments refusals and its scratch-copy mutation |
| The contract change breaks either of the Kubernetes desk package's pinned pre-change records (`recorded_images()` on v1.0.52 or `candidate_images()` on v1.0.65) | row 2 (both records' real bytes) and row 5 (`--base`, the compatibility self-test and a direct `candidate_images()` call) |
| The producer, contract and docs disagree on the image set | row 4 |
| A workflow copy, either pinned record or the deskd Dockerfile is edited by this delivery | rows 3 and 6 |
| A downstream deployment pins a local or out-of-band deskd image instead of the release record | No row here; cell-api/05's pin and the downstream follow-on dereference the published record |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or publication result is asserted. -->

## Review

Gate: model (from frontmatter; all four risk answers no). Confirm that the producer
builds deskd from cell-api/08's definition without a second Dockerfile, that every deskd
check precedes the first push, that the contract's default is strict and only named
consumers opt into the legacy set, and that no workflow copy changed. Test success
publishes nothing and grants no release, pin or deployment.
