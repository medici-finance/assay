"""Release-owned Linux artifacts and container images. No consumer builds images."""
import datetime
import json
from pathlib import Path
import re
import shutil
import subprocess
import tarfile

DESKS = ("intake-desk", "worker-desk", "pr-review-desk", "verify-desk", "the-desk")
NAMES = ("desk-tools", "assay-harness", "desk-base", *DESKS, "verify-runner")


def run(*args, capture=False):
    print("+ " + " ".join(map(str, args)), flush=True)
    return subprocess.run(list(map(str, args)), check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def build_images(root, version, source, prefix, verify_image, artifacts, out, publish):
    if not re.fullmatch(r"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", version):
        raise ValueError("a bare umbrella vMAJOR.MINOR.PATCH is required")
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("source revision must be a full commit SHA")
    for name in (prefix, verify_image):
        if not re.fullmatch(r"ghcr\.io/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*", name):
            raise ValueError("registry names must be untagged GHCR repository paths")
    root = Path(root).resolve()
    plugin = json.loads((root / "plugins/assay/.claude-plugin/plugin.json").read_text())
    if plugin.get("version") != version.removeprefix("v"):
        raise ValueError("prepared plugin version disagrees with the release")
    out = Path(out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    if any((out / name).exists() for name in
           ("release-images.json", "verify-runner-image.txt")):
        raise ValueError("refusing to replace existing image-reference artifacts")
    refs = {name: f"{prefix}/{name}:{version}" for name in NAMES}
    refs["verify-runner"] = f"{verify_image}:{version}"
    print("Release image targets: " + ", ".join(refs.values()), flush=True)
    if publish:
        # A failed lookup is absence only for the registry's explicit unknown
        # manifest response. Auth/transport failures never permit an overwrite.
        for ref in refs.values():
            result = subprocess.run(["docker", "manifest", "inspect", ref],
                                    text=True, capture_output=True)
            if result.returncode == 0:
                raise ValueError(f"image tag already exists; fix forward: {ref}")
            if "manifest unknown" not in result.stderr.lower():
                raise ValueError(f"could not establish image-tag absence: {ref}: {result.stderr}")
    stage = root / ".release-images"
    if stage.exists():
        raise ValueError(".release-images already exists; use a clean release checkout")
    bins = stage / "harness"
    bins.mkdir(parents=True)
    built_at = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    stamps = ["--build-arg", f"VERSION={version}", "--build-arg", f"SOURCE_SHA={source}",
              "--build-arg", f"BUILT_AT={built_at}"]
    artifacts = Path(artifacts).resolve()
    # The build job supplies the exact binaries that the final release publishes.
    # Read regular executable members only; never extract paths or links from tar.
    with tarfile.open(artifacts / "desk-tools-linux-amd64.tar.gz", "r:gz") as archive:
        for member in archive.getmembers():
            name = member.name.removeprefix("./")
            if member.isdir():
                continue
            if not member.isfile() or name.startswith("/") or ".." in Path(name).parts:
                raise ValueError("unsafe Linux suite archive member")
            if "/" in name:
                if name != "hooks/pre-push":
                    raise ValueError("unexpected nested Linux suite archive member")
                continue
            if not member.mode & 0o111:
                raise ValueError("non-executable Linux suite archive member")
            with archive.extractfile(member) as src, (bins / name).open("wb") as dst:
                shutil.copyfileobj(src, dst)
            (bins / name).chmod(0o755)
    for artifact, name in (("statusgen-linux-amd64", "statusgen"),
                           ("image-bugs-gc-linux-amd64", "bugs-gc")):
        shutil.copyfile(artifacts / artifact, bins / name)
        (bins / name).chmod(0o755)
    for name in ("statusgen", "deskboard", "cellctl", "bugs-gc"):
        if not (bins / name).is_file():
            raise ValueError(f"release build did not produce {name}")
    # The harness has the same suite/statusgen; bugs-gc belongs to desk-tools.
    (stage / "desk-tools").mkdir()
    for path in bins.iterdir():
        shutil.copyfile(path, stage / "desk-tools" / path.name)
        (stage / "desk-tools" / path.name).chmod(path.stat().st_mode)
    (bins / "bugs-gc").unlink()
    # Explicit, digest-pinned scanner acquisition precedes image builds. The
    # scanner itself runs offline and never pulls tools on a scan invocation.
    run("sh", root / "containers/scripts/layer-secret-scan.sh", "--prepare")
    run("docker", "build", "--platform", "linux/amd64", *stamps,
        "--build-arg", "BINARY_STAGE=released", "-t", refs["desk-tools"],
        "-f", root / "Dockerfile", root)
    run("docker", "build", "--platform", "linux/amd64", "--build-arg", "BINARY_STAGE=released",
        "--build-arg", f"ASSAY_VERSION={version}", "--build-arg", f"SOURCE_REVISION={source}",
        "--build-arg", "SOURCE_URL=https://github.com/" + prefix.removeprefix("ghcr.io/"),
        "-t", refs["assay-harness"], "-f", root / "images/assay-harness/Dockerfile", root)
    run("docker", "build", "--platform", "linux/amd64", *stamps,
        "--build-arg", f"DESK_TOOLS_IMAGE={refs['desk-tools']}",
        "-t", refs["desk-base"], "-f", root / "containers/base/Dockerfile", root)
    for name in DESKS:
        run("docker", "build", "--platform", "linux/amd64", "--build-arg",
            f"BASE_IMAGE={refs['desk-base']}", "-t", refs[name],
            "-f", root / f"containers/{name}/Dockerfile", root)
    run("docker", "build", "--platform", "linux/amd64", "-t", refs["verify-runner"],
        "-f", root / "images/verify-runner/Dockerfile", root)
    for name, ref in refs.items():
        got = run("docker", "run", "--rm", "--network", "none", "--platform", "linux/amd64",
                  "--entrypoint", "statusgen", ref, "--version", capture=True).strip()
        if got != version:
            raise ValueError(f"{name} statusgen version {got!r} != {version}")
    for name in ("desk-base", *DESKS):
        run("sh", root / "containers/scripts/layer-secret-scan.sh", refs[name])
    if not publish:
        print("Dry build complete: no image pushed and no pinnable-reference artifacts written.")
        return
    images = []
    for name, ref in refs.items():
        run("docker", "push", ref)
        repo = ref.rsplit(":", 1)[0]
        digests = run("docker", "image", "inspect", "--format",
                      "{{range .RepoDigests}}{{println .}}{{end}}", ref, capture=True).splitlines()
        candidates = {s for s in digests if re.fullmatch(re.escape(repo) + r"@sha256:[0-9a-f]{64}", s)}
        if len(candidates) != 1:
            raise ValueError(f"push did not yield one immutable digest for {ref}")
        pinnable = ref + "@" + candidates.pop().split("@", 1)[1]
        images.append({"name": name, "platform": "linux/amd64", "ref": pinnable})
    manifest = {"schema": "release-images-v1", "version": version,
                "source_revision": source, "repository": prefix.removeprefix("ghcr.io/"), "images": images}
    (out / "release-images.json").write_text(json.dumps(manifest, indent=2) + "\n")
    runner = next(image["ref"] for image in images if image["name"] == "verify-runner")
    (out / "verify-runner-image.txt").write_text(runner + "\n")
    print("Published all release images; immutable references are release artifacts.")
