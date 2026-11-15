"""Validate published image references; this consumer contract never builds or pushes."""
import re

# The image set before deskd joined the release. Only a consumer of an older
# release's record passes it, by name; the default accepts the current set only.
LEGACY_NAMES = frozenset({"desk-tools", "assay-harness", "desk-base", "intake-desk", "worker-desk",
                          "pr-review-desk", "verify-desk", "the-desk", "verify-runner"})
NAMES = LEGACY_NAMES | {"deskd"}
# verify-runner is published as one image index over these platforms (#4323); its
# reference pins the index digest, which names each platform's manifest digest.
RUNNER_PLATFORMS = ("linux/amd64", "linux/arm64")
RUNNER_PLATFORM = ",".join(RUNNER_PLATFORMS)
# Releases before #4323 recorded an amd64-only runner. A consumer that keeps one
# of those records still validates it; the producer passes only RUNNER_PLATFORM.
LEGACY_RUNNER_PLATFORM = "linux/amd64"


def validate(manifest, version, source, runner, repository,
             runner_platforms=(RUNNER_PLATFORM, LEGACY_RUNNER_PLATFORM), names=(NAMES,)):
    if isinstance(runner_platforms, str) or not runner_platforms:
        raise ValueError("runner_platforms must be a non-empty collection of platform values")
    # names is a collection of complete image sets; the record must equal one.
    # A bare set is refused rather than read as a set of one-name sets.
    if isinstance(names, (str, set, frozenset)) or not names or any(
            not isinstance(s, (set, frozenset)) or not s or not all(isinstance(n, str) for n in s)
            for s in names):
        raise ValueError("names must be a non-empty collection of image-name sets")
    name_sets = {frozenset(s) for s in names}
    allowed = frozenset().union(*name_sets)
    if set(manifest) != {"schema", "version", "source_revision", "repository", "images"}:
        raise ValueError("unexpected image manifest fields")
    if manifest["schema"] != "release-images-v1" or manifest["version"] != version:
        raise ValueError("image manifest disagrees with the selected release")
    if manifest["source_revision"] != source or not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("image manifest disagrees with the prepared source revision")
    if manifest["repository"] != repository or not re.fullmatch(r"[a-z0-9._-]+/[a-z0-9._-]+", repository):
        raise ValueError("image manifest disagrees with the release repository")
    seen = {}
    for image in manifest["images"]:
        if set(image) != {"name", "platform", "ref"}:
            raise ValueError("unexpected image fields")
        name = image["name"]
        platforms = runner_platforms if name == "verify-runner" else ("linux/amd64",)
        if name not in allowed or name in seen or image["platform"] not in platforms:
            raise ValueError("missing, duplicate or unexpected image/platform")
        path = repository.split("/")[0] + "/verify-runner" if name == "verify-runner" else repository + "/" + name
        prefix = f"ghcr.io/{path}:{version}@sha256:"
        if not image["ref"].startswith(prefix) or not re.fullmatch(r"[0-9a-f]{64}", image["ref"][len(prefix):]):
            raise ValueError("image reference has the wrong repository/tag or lacks an immutable digest")
        seen[name] = image["ref"]
    if frozenset(seen) not in name_sets:
        raise ValueError("image manifest is incomplete")
    if runner != seen["verify-runner"] + "\n":
        raise ValueError("verify-runner-image.txt disagrees with the image manifest")
    return seen
