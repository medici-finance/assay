"""Validate published image references; this consumer contract never builds or pushes."""
import json
from pathlib import Path
import re
import sys

NAMES = {"desk-tools", "assay-harness", "desk-base", "intake-desk", "worker-desk",
         "pr-review-desk", "verify-desk", "the-desk", "verify-runner"}


def validate(manifest, version, source, runner, repository):
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
        if name not in NAMES or name in seen or image["platform"] != "linux/amd64":
            raise ValueError("missing, duplicate or unexpected image/platform")
        path = repository.split("/")[0] + "/verify-runner" if name == "verify-runner" else repository + "/" + name
        prefix = f"ghcr.io/{path}:{version}@sha256:"
        if not image["ref"].startswith(prefix) or not re.fullmatch(r"[0-9a-f]{64}", image["ref"][len(prefix):]):
            raise ValueError("image reference has the wrong repository/tag or lacks an immutable digest")
        seen[name] = image["ref"]
    if set(seen) != NAMES:
        raise ValueError("image manifest is incomplete")
    if runner != seen["verify-runner"] + "\n":
        raise ValueError("verify-runner-image.txt disagrees with the image manifest")
    return seen


def main():
    try:
        path, version, source, runner_path, repository = sys.argv[1:]
        validate(json.loads(Path(path).read_text()), version, source,
                 Path(runner_path).read_text(), repository)
    except (ValueError, OSError, TypeError, KeyError) as exc:
        sys.exit(f"release image contract: {exc}")


if __name__ == "__main__":
    main()
