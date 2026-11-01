"""Release-owned Linux artifacts and container images. No consumer builds images."""
import datetime
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tarfile

DESKS = ("intake-desk", "worker-desk", "pr-review-desk", "verify-desk", "the-desk")
NAMES = ("desk-tools", "assay-harness", "desk-base", *DESKS, "verify-runner")
# verify-runner's platforms (#4323); the other images are linux/amd64. These
# equal images_contract's values, which the release step validates against.
RUNNER_PLATFORMS = ("linux/amd64", "linux/arm64")
RUNNER_PLATFORM = ",".join(RUNNER_PLATFORMS)


def run(*args, capture=False):
    print("+ " + " ".join(map(str, args)), flush=True)
    return subprocess.run(list(map(str, args)), check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def build_images(root, version, source, prefix, verify_image, artifacts, out, publish, *, native):
    if not re.fullmatch(r"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", version):
        raise ValueError("a bare umbrella vMAJOR.MINOR.PATCH is required")
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("source revision must be a full commit SHA")
    for name in (prefix, verify_image):
        if not re.fullmatch(r"ghcr\.io/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*", name):
            raise ValueError("registry names must be untagged GHCR repository paths")
    native = native_transport(native)
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
    # verify-runner is one image index over RUNNER_PLATFORMS (#4323). Each
    # platform image is built, checked and pushed under its own tag first; the
    # versioned tag then names the index over exactly those pushed digests.
    runner_refs = {platform: refs["verify-runner"] + "-" + platform.replace("/", "-")
                   for platform in RUNNER_PLATFORMS}
    targets = [*refs.values(), *runner_refs.values()]
    print("Release image targets: " + ", ".join(targets), flush=True)
    if publish:
        # A failed lookup is absence only for the registry's explicit unknown
        # manifest response. Auth/transport failures never permit an overwrite.
        for ref in targets:
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
    # The runner's statusgen per platform, named as the release asset it is.
    # images/verify-runner/Dockerfile copies statusgen-linux-${TARGETARCH}, so a
    # platform with no staged binary fails its build.
    runner_bins = stage / "verify-runner"
    runner_bins.mkdir()
    for platform in RUNNER_PLATFORMS:
        asset = "statusgen-" + platform.replace("/", "-")
        shutil.copyfile(artifacts / asset, runner_bins / asset)
        (runner_bins / asset).chmod(0o755)
    for name in ("statusgen", "deskboard", "cellctl", "bugs-gc"):
        if not (bins / name).is_file():
            raise ValueError(f"release build did not produce {name}")
    # The harness has the same suite/statusgen; bugs-gc belongs to desk-tools.
    (stage / "desk-tools").mkdir()
    for path in bins.iterdir():
        shutil.copyfile(path, stage / "desk-tools" / path.name)
        (stage / "desk-tools" / path.name).chmod(path.stat().st_mode)
    (bins / "bugs-gc").unlink()
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
    # ARM64 is built and checked on a native runner. Import its exact checked
    # image; this publisher must never register an emulator or execute ARM64.
    import_native_runner(native, version, source, runner_refs["linux/arm64"])
    platform = "linux/amd64"
    ref = runner_refs[platform]
    build_runner(root, platform, ref)
    checks = [(name, ref, "linux/amd64") for name, ref in refs.items() if name != "verify-runner"]
    checks.append(("verify-runner " + platform, ref, platform))
    for name, ref, platform in checks:
        check_version(name, ref, platform, version)
    check_runner(root, runner_refs["linux/amd64"], "linux/amd64")
    for name in ("desk-base", *DESKS):
        run("sh", root / "containers/scripts/layer-secret-scan.sh", refs[name])
    if not publish:
        print("Dry build complete: no image pushed and no pinnable-reference artifacts written.")
        return
    images = []
    for name, ref in refs.items():
        if name == "verify-runner":
            digest = publish_runner_index(ref, runner_refs)
            images.append({"name": name, "platform": RUNNER_PLATFORM, "ref": ref + "@" + digest})
            continue
        images.append({"name": name, "platform": "linux/amd64",
                       "ref": ref + "@" + push_digest(ref)})
    manifest = {"schema": "release-images-v1", "version": version,
                "source_revision": source, "repository": prefix.removeprefix("ghcr.io/"), "images": images}
    (out / "release-images.json").write_text(json.dumps(manifest, indent=2) + "\n")
    runner = next(image["ref"] for image in images if image["name"] == "verify-runner")
    (out / "verify-runner-image.txt").write_text(runner + "\n")
    print("Published all release images; immutable references are release artifacts.")


NATIVE_ARCHIVE = "verify-runner-linux-arm64.tar"
NATIVE_RECEIPT = "verify-runner-linux-arm64.json"


def check_version(name, ref, platform, version):
    got = run("docker", "run", "--rm", "--network", "none", "--platform", platform,
              "--entrypoint", "statusgen", ref, "--version", capture=True).strip()
    if got != version:
        raise ValueError(f"{name} statusgen version {got!r} != {version}")


def build_runner(root, platform, ref):
    # Each platform must push as ONE manifest, not a provenance/SBOM index.
    run("docker", "build", "--platform", platform,
        "--provenance=false", "--sbom=false", "-t", ref,
        "-f", root / "images/verify-runner/Dockerfile", root)
    check_runner_platform(ref, platform)


def check_runner_platform(ref, platform):
    got = run("docker", "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}",
              ref, capture=True).strip()
    if got != platform:
        raise ValueError(f"verify-runner build for {platform} produced {got!r}")


def check_runner(root, ref, platform):
    # The same native capability proof on both architectures. No emulator,
    # mounts, installs, credentials or network are added to this fence.
    run("docker", "run", "--rm", "--pull", "never", "--network", "none",
        "--platform", platform, "--read-only", "--cap-drop", "ALL",
        "--security-opt", "no-new-privileges", "--user", "1000:1000",
        "--pids-limit", "64", "--memory", "256m", "--cpus", "1",
        "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m",
        "--entrypoint", "python3", ref, "-B", "-c",
        (root / "images/verify-runner/check.py").read_text())


def stream_sha256(stream):
    digest = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        digest.update(chunk)
    return digest.hexdigest()


def file_sha256(path):
    with path.open("rb") as stream:
        return stream_sha256(stream)


def build_native_runner(root, version, source, verify_image, artifacts, out, platform="linux/arm64"):
    """Build/check ARM64 natively and hand its exact image to the publisher.

    This job has no registry login, push or repository write. The receipt is
    same-run CI transport evidence, not a public attestation or a runtime pin.
    """
    if platform != "linux/arm64":
        raise ValueError("the native image job builds linux/arm64 only")
    if not re.fullmatch(r"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", version):
        raise ValueError("a bare umbrella vMAJOR.MINOR.PATCH is required")
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("source revision must be a full commit SHA")
    if not re.fullmatch(r"ghcr\.io/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*", verify_image):
        raise ValueError("registry names must be untagged GHCR repository paths")
    root, artifacts, out = Path(root).resolve(), Path(artifacts).resolve(), Path(out).resolve()
    plugin = json.loads((root / "plugins/assay/.claude-plugin/plugin.json").read_text())
    if plugin.get("version") != version.removeprefix("v"):
        raise ValueError("prepared plugin version disagrees with the release")
    host = run("docker", "info", "--format", "{{.OSType}}/{{.Architecture}}", capture=True).strip()
    if host not in ("linux/arm64", "linux/aarch64"):
        raise ValueError(f"native ARM64 build requires an ARM64 Docker daemon, got {host!r}")
    out.mkdir(parents=True, exist_ok=True)
    if any((out / name).exists() for name in (NATIVE_ARCHIVE, NATIVE_RECEIPT)):
        raise ValueError("refusing to replace native image transport artifacts")
    stage = root / ".release-images"
    if stage.exists():
        raise ValueError(".release-images already exists; use a clean release checkout")
    bins = stage / "verify-runner"
    bins.mkdir(parents=True)
    asset = "statusgen-linux-arm64"
    shutil.copyfile(artifacts / asset, bins / asset)
    (bins / asset).chmod(0o755)
    ref = f"{verify_image}:{version}-linux-arm64"
    build_runner(root, platform, ref)
    check_version("verify-runner " + platform, ref, platform, version)
    check_runner(root, ref, platform)
    image_id = run("docker", "image", "inspect", "--format", "{{.Id}}", ref, capture=True).strip()
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", image_id):
        raise ValueError("native build did not yield one image ID")
    archive = out / NATIVE_ARCHIVE
    run("docker", "save", "--output", archive, ref)
    receipt = {"schema": "native-runner-v1", "version": version,
               "source_revision": source, "platform": platform, "ref": ref,
               "image_id": image_id, "archive_sha256": file_sha256(archive)}
    (out / NATIVE_RECEIPT).write_text(json.dumps(receipt, indent=2) + "\n")


def native_transport(native):
    """Admit the native ARM64 transport directory, or raise. It is produced by a
    job outside the approval gate, so it lives in a directory of its own and
    nothing else may sit beside the archive and its receipt. A download into the
    verified release inputs could replace one of those."""
    if Path(native).is_symlink() or not Path(native).is_dir():
        raise ValueError("native ARM64 transport directory is missing")
    native = Path(native).resolve()
    if sorted(entry.name for entry in native.iterdir()) != sorted((NATIVE_ARCHIVE, NATIVE_RECEIPT)):
        raise ValueError("native ARM64 transport must hold exactly the archive and its receipt")
    for name in (NATIVE_ARCHIVE, NATIVE_RECEIPT):
        if (native / name).is_symlink() or not (native / name).is_file():
            raise ValueError("native ARM64 transport files must be regular files")
    return native


def import_native_runner(native, version, source, ref):
    native = native_transport(native)
    receipt = json.loads((native / NATIVE_RECEIPT).read_text())
    expected = {"schema": "native-runner-v1", "version": version,
                "source_revision": source, "platform": "linux/arm64", "ref": ref}
    if not isinstance(receipt, dict) or any(receipt.get(k) != v for k, v in expected.items()):
        raise ValueError("native ARM64 receipt disagrees with this release")
    image_id = receipt.get("image_id", "")
    digest = receipt.get("archive_sha256", "")
    if not isinstance(image_id, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", image_id):
        raise ValueError("native ARM64 receipt has no exact image ID")
    archive = native / NATIVE_ARCHIVE
    if not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest) or file_sha256(archive) != digest:
        raise ValueError("native ARM64 image archive checksum mismatch")
    # Docker load mutates the local image store. Admit exactly the transported
    # tag/config before loading, so a receipt cannot import unrelated tags.
    with tarfile.open(archive, "r:*") as saved:
        manifests = [m for m in saved.getmembers() if m.name == "manifest.json"]
        if len(manifests) != 1 or not manifests[0].isfile() or manifests[0].size > 1024 * 1024:
            raise ValueError("native ARM64 archive has no single Docker save manifest")
        with saved.extractfile(manifests[0]) as stream:
            manifest = json.load(stream)
        if (not isinstance(manifest, list) or len(manifest) != 1
                or not isinstance(manifest[0], dict) or manifest[0].get("RepoTags") != [ref]):
            raise ValueError("native ARM64 archive must contain only its expected image tag")
        config = manifest[0].get("Config")
        if not isinstance(config, str):
            raise ValueError("native ARM64 archive has no image config")
        configs = [m for m in saved.getmembers() if m.name == config]
        if len(configs) != 1 or not configs[0].isfile() or configs[0].size > 16 * 1024 * 1024:
            raise ValueError("native ARM64 archive has no single image config")
        with saved.extractfile(configs[0]) as stream:
            config_id = "sha256:" + stream_sha256(stream)
        if config_id != image_id:
            raise ValueError("native ARM64 archive config differs from the checked image")
    run("docker", "load", "--input", archive)
    check_runner_platform(ref, "linux/arm64")
    loaded = run("docker", "image", "inspect", "--format", "{{.Id}}", ref, capture=True).strip()
    if loaded != image_id:
        raise ValueError("loaded ARM64 image differs from the native checked image")


def push_digest(ref):
    """Push one tagged image and return the registry manifest digest it got."""
    run("docker", "push", ref)
    repo = ref.rsplit(":", 1)[0]
    digests = run("docker", "image", "inspect", "--format",
                  "{{range .RepoDigests}}{{println .}}{{end}}", ref, capture=True).splitlines()
    candidates = {s for s in digests if re.fullmatch(re.escape(repo) + r"@sha256:[0-9a-f]{64}", s)}
    if len(candidates) != 1:
        raise ValueError(f"push did not yield one immutable digest for {ref}")
    return candidates.pop().split("@", 1)[1]


INDEX_TYPES = ("application/vnd.docker.distribution.manifest.list.v2+json",
               "application/vnd.oci.image.index.v1+json")


def publish_runner_index(ref, runner_refs):
    """Push each platform image, then the index over exactly those digests.

    Returns the index digest. The reference pins the index, not one platform:
    the index bytes name every platform manifest by digest, so one immutable
    pin selects one exact image per platform. The index is read back by that
    digest and must list exactly the pushed platform digests, nothing else.
    """
    repo = ref.rsplit(":", 1)[0]
    pushed = {platform: push_digest(platform_ref) for platform, platform_ref in runner_refs.items()}
    run("docker", "manifest", "create", ref, *(repo + "@" + d for d in pushed.values()))
    out = run("docker", "manifest", "push", "--purge", ref, capture=True).split()
    if not out or not re.fullmatch(r"sha256:[0-9a-f]{64}", out[-1]):
        raise ValueError(f"index push did not yield an immutable digest for {ref}")
    digest = out[-1]
    index = json.loads(run("docker", "manifest", "inspect", repo + "@" + digest, capture=True))
    if index.get("mediaType") not in INDEX_TYPES:
        raise ValueError(f"{ref}@{digest} is not an image index")
    listed = {}
    for entry in index.get("manifests", []):
        platform = entry.get("platform", {})
        key = f"{platform.get('os')}/{platform.get('architecture')}"
        if key in listed:
            raise ValueError(f"{ref}@{digest} lists platform {key} twice")
        listed[key] = entry.get("digest")
    if listed != pushed:
        raise ValueError(f"{ref}@{digest} does not list exactly the pushed platform images")
    return digest
