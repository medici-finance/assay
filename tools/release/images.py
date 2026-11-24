"""Release-owned Linux artifacts and container images. No consumer builds images."""
import datetime
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import time

DESKS = ("intake-desk", "worker-desk", "pr-review-desk", "verify-desk", "the-desk")
# deskd is the service image cell-api/08 defines (images/deskd/, context deskd/).
# It carries no statusgen, so check_deskd replaces check_version for it.
NAMES = ("desk-tools", "assay-harness", "desk-base", *DESKS, "deskd", "verify-runner")
# verify-runner's platforms (#4323); the other images are linux/amd64. These
# equal images_contract's values, which the release step validates against.
RUNNER_PLATFORMS = ("linux/amd64", "linux/arm64")
RUNNER_PLATFORM = ",".join(RUNNER_PLATFORMS)


def run(*args, capture=False):
    print("+ " + " ".join(map(str, args)), flush=True)
    return subprocess.run(list(map(str, args)), check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


# Release builds run on a non-ephemeral runner, and two kinds of BuildKit state
# outlive the build that wrote them: cache mounts (RUN --mount=type=cache),
# shared by target path across every build on the daemon, and the instruction
# cache, whose key for a RUN step does not cover the contents of its cache
# mounts. Either could carry an earlier build's output (another ref, a dry run,
# a build outside this module) into a published image. Every release image
# build goes through docker_build, which clears all exec cache mounts first
# (stopping the release if that fails) and then builds with --no-cache, so no
# cached layer is reused and every step runs against empty mounts. Tests refuse
# any other build call.
PRUNE_CACHE_MOUNTS = ("docker", "builder", "prune", "--all", "--force",
                      "--filter", "type=exec.cachemount")
NO_CACHE = "--no-cache"


def docker_build(*args):
    run(*PRUNE_CACHE_MOUNTS)
    run("docker", "build", NO_CACHE, *args)


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
        # Read every tag before the build, so unreadable registry state stops
        # the release early. A tag already present is not refused here: no
        # pushed digest exists yet to compare it with. attach_version_tags
        # compares it after every push and refuses any other digest (#7619).
        for ref in targets:
            present = tag_digest(ref)
            if present is not None:
                print(f"Image tag already present: {ref}@{present}; "
                      "it must name the digest this run pushes", flush=True)
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
    # The one non-executable member admitted is the root MANIFEST.sha256 the
    # package step writes (#6604): it is read, never copied into an image, and
    # every other member's bytes must match it before any build runs.
    digests = {}
    manifest = None
    with tarfile.open(artifacts / "desk-tools-linux-amd64.tar.gz", "r:gz") as archive:
        for member in archive.getmembers():
            name = member.name.removeprefix("./")
            if member.isdir():
                continue
            if not member.isfile() or name.startswith("/") or ".." in Path(name).parts:
                raise ValueError("unsafe Linux suite archive member")
            if name in digests or (name == SUITE_MANIFEST and manifest is not None):
                raise ValueError(f"duplicate Linux suite archive member: {name}")
            if name == SUITE_MANIFEST:
                if member.size > SUITE_MANIFEST_MAX:
                    raise ValueError("Linux suite MANIFEST.sha256 is too large")
                with archive.extractfile(member) as src:
                    manifest = src.read()
                continue
            if "/" in name:
                if name != "hooks/pre-push":
                    raise ValueError("unexpected nested Linux suite archive member")
                with archive.extractfile(member) as src:
                    digests[name] = stream_sha256(src)
                continue
            if not member.mode & 0o111:
                raise ValueError("non-executable Linux suite archive member")
            with archive.extractfile(member) as src, (bins / name).open("wb") as dst:
                shutil.copyfileobj(src, dst)
            (bins / name).chmod(0o755)
            digests[name] = file_sha256(bins / name)
    check_suite_manifest(manifest, digests)
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
    docker_build("--platform", "linux/amd64", *stamps,
        "--build-arg", "BINARY_STAGE=released", "-t", refs["desk-tools"],
        "-f", root / "Dockerfile", root)
    docker_build("--platform", "linux/amd64", "--build-arg", "BINARY_STAGE=released",
        "--build-arg", f"ASSAY_VERSION={version}", "--build-arg", f"SOURCE_REVISION={source}",
        "--build-arg", "SOURCE_URL=https://github.com/" + prefix.removeprefix("ghcr.io/"),
        "-t", refs["assay-harness"], "-f", root / "images/assay-harness/Dockerfile", root)
    docker_build("--platform", "linux/amd64", *stamps,
        "--build-arg", f"DESK_TOOLS_IMAGE={refs['desk-tools']}",
        "-t", refs["desk-base"], "-f", root / "containers/base/Dockerfile", root)
    for name in DESKS:
        docker_build("--platform", "linux/amd64", "--build-arg",
            f"BASE_IMAGE={refs['desk-base']}", "-t", refs[name],
            "-f", root / f"containers/{name}/Dockerfile", root)
    # deskd's build context is its own public module, never the repository root.
    docker_build("--platform", "linux/amd64", "--build-arg", f"REVISION={source}",
        "-t", refs["deskd"], "-f", root / "images/deskd/Dockerfile", root / "deskd")
    # ARM64 is built and checked on a native runner. Import its exact checked
    # image; this publisher must never register an emulator or execute ARM64.
    import_native_runner(native, version, source, runner_refs["linux/arm64"])
    platform = "linux/amd64"
    ref = runner_refs[platform]
    build_runner(root, platform, ref)
    checks = [(name, ref, "linux/amd64") for name, ref in refs.items()
              if name not in ("verify-runner", "deskd")]
    checks.append(("verify-runner " + platform, ref, platform))
    for name, ref, platform in checks:
        check_version(name, ref, platform, version)
    check_runner(root, runner_refs["linux/amd64"], "linux/amd64")
    check_deskd(refs["deskd"], source)
    for name in ("desk-base", *DESKS, "deskd"):
        run("sh", root / "containers/scripts/layer-secret-scan.sh", refs[name])
    if not publish:
        print("Dry build complete: no image pushed and no pinnable-reference artifacts written.")
        return
    # Digest first (#7619): every image is pushed under a staging tag and its
    # digest recorded; no version tag moves until every push has succeeded.
    images, wanted = [], {}
    try:
        for name, ref in refs.items():
            if name == "verify-runner":
                digest, platforms = publish_runner_index(ref, runner_refs)
                wanted.update((runner_refs[p], d) for p, d in platforms.items())
                images.append({"name": name, "platform": RUNNER_PLATFORM, "ref": ref + "@" + digest})
            else:
                digest = push_digest(stage_tag(ref))
                images.append({"name": name, "platform": "linux/amd64", "ref": ref + "@" + digest})
            wanted[ref] = digest
    except Exception:
        # Plan the next cut from the job log alone: what the registry now holds.
        print("Release images pushed before the failure, with no version tag attached: "
              + (" ".join(image["ref"] for image in images) or "none"), file=sys.stderr, flush=True)
        raise
    attach_version_tags(wanted)
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


def check_deskd(ref, source):
    """deskd has no statusgen: check its user, revision label and usage instead."""
    config = json.loads(run("docker", "image", "inspect", "--format", "{{json .Config}}",
                            ref, capture=True))
    user = config.get("User") or ""
    if not re.fullmatch(r"[0-9]+(?::[0-9]+)?", user) or any(int(p) == 0 for p in user.split(":")):
        raise ValueError(f"deskd image user {user!r} is not a numeric non-root user")
    revision = (config.get("Labels") or {}).get("org.opencontainers.image.revision")
    if revision != source:
        raise ValueError(f"deskd image revision label {revision!r} != {source}")
    # Go's flag package prints usage to stderr and exits 0 for --help.
    args = ["docker", "run", "--rm", "--pull", "never", "--network", "none",
            "--platform", "linux/amd64", "--read-only", "--cap-drop", "ALL",
            "--security-opt", "no-new-privileges", ref, "--help"]
    print("+ " + " ".join(args), flush=True)
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if result.returncode != 0 or "Usage of" not in result.stdout or "-addr" not in result.stdout:
        # The output is the binary's own text from a --network none run; keep
        # a bounded tail so the release log alone can diagnose the failure.
        tail = result.stdout[-2000:]
        raise ValueError(f"deskd image did not print its usage (exit {result.returncode}); "
                         f"output tail: {tail!r}")


def build_runner(root, platform, ref):
    # Each platform must push as ONE manifest, not a provenance/SBOM index.
    docker_build("--platform", platform,
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


SUITE_MANIFEST = "MANIFEST.sha256"
SUITE_MANIFEST_MAX = 1024 * 1024
SUITE_MANIFEST_LINE = re.compile(r"([0-9a-f]{64})  ([^\r\n]+)")


def check_suite_manifest(data, digests):
    """Refuse unless data is a MANIFEST.sha256 listing exactly the suite's
    members with their digests: the format tools/release/desk_manifest.py
    writes, one '<sha256>  <path>' line each, newline-terminated. It is parsed
    here rather than imported because the image tests load images.py alone.
    The manifest is no trust root (checksums.txt and the attestation are); this
    is a second, post-tar reading of the bytes the package step described."""
    if data is None:
        raise ValueError("Linux suite archive has no MANIFEST.sha256")
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        raise ValueError("Linux suite MANIFEST.sha256 is malformed: not UTF-8") from None
    lines = text.split("\n")
    if lines.pop() != "":
        raise ValueError("Linux suite MANIFEST.sha256 is malformed: no final newline")
    problems, listed = [], {}
    for number, line in enumerate(lines, 1):
        match = SUITE_MANIFEST_LINE.fullmatch(line)
        if not match:
            problems.append(f"line {number} malformed")
            continue
        digest, name = match.groups()
        if name in listed:
            problems.append(f"{name}: listed twice")
        listed[name] = digest
    for name, digest in sorted(listed.items()):
        if name not in digests:
            problems.append(f"{name}: listed but not present")
        elif digests[name] != digest:
            problems.append(f"{name}: sha256 does not match the manifest")
    problems += [f"{name}: present but not listed" for name in sorted(set(digests) - set(listed))]
    if problems:
        raise ValueError("Linux suite MANIFEST.sha256 disagrees with the archive: " + "; ".join(problems))


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


# A registry can fail a push it accepts moments later: GHCR answered "unknown
# blob" straight after a cross-repository blob mount and burned a release
# (#7619). Only such registry-side transients are retried, a bounded number of
# times with a growing wait: a missing blob (both spellings), a 5xx status, a
# 429 rate limit, and a dropped or timed-out connection. An auth or permission
# refusal (401/403 included), an existing-tag or invalid-manifest refusal, and
# every other failure raise on the first attempt; a refusal word anywhere in
# the message wins over a transient marker beside it. A retried push re-sends
# the same local image to its staging tag, and a retried tag attach names the
# same digest, so a retry never moves a version tag to other bytes.
PUSH_ATTEMPTS = 3
PUSH_BACKOFF_SECONDS = 15
PUSH_REFUSAL = re.compile(r"denied|unauthori[sz]ed|forbidden|authenticat|already exists"
                          r"|\b401\b|\b403\b", re.I)
PUSH_TRANSIENT = re.compile(r"unknown blob|blob unknown"
                            r"|\bhttp(?: status)?:? 5[0-9]{2}\b|internal server error"
                            r"|bad gateway|service unavailable|gateway timeout"
                            r"|\bhttp(?: status)?:? 429\b|toomanyrequests|too many requests"
                            r"|connection reset|i/o timeout|tls handshake timeout", re.I)


def registry_transient(stderr):
    """True only for a registry-side transient that carries no refusal."""
    return bool(not PUSH_REFUSAL.search(stderr) and PUSH_TRANSIENT.search(stderr))


def registry_push(*args, capture=False, sleep=time.sleep):
    """Run one docker registry write, retrying registry transients only.

    Every docker push, manifest push and imagetools tag write goes through
    here. On exhaustion, or on any other failure, the last attempt's
    CalledProcessError is raised."""
    attempt = 1
    while True:
        print("+ " + " ".join(map(str, args)), flush=True)
        result = subprocess.run(list(map(str, args)), text=True, stderr=subprocess.PIPE,
                                stdout=subprocess.PIPE if capture else None)
        sys.stderr.write(result.stderr or "")
        sys.stderr.flush()
        if result.returncode == 0:
            return result.stdout
        if attempt >= PUSH_ATTEMPTS or not registry_transient(result.stderr):
            result.check_returncode()
        wait = PUSH_BACKOFF_SECONDS * 2 ** (attempt - 1)
        print(f"registry transient on attempt {attempt} of {PUSH_ATTEMPTS}; "
              f"retrying in {wait}s", flush=True)
        sleep(wait)
        attempt += 1


def push_digest(ref, *, sleep=time.sleep):
    """Push one tagged image and return the registry manifest digest it got."""
    registry_push("docker", "push", ref, sleep=sleep)
    repo = ref.rsplit(":", 1)[0]
    digests = run("docker", "image", "inspect", "--format",
                  "{{range .RepoDigests}}{{println .}}{{end}}", ref, capture=True).splitlines()
    candidates = {s for s in digests if re.fullmatch(re.escape(repo) + r"@sha256:[0-9a-f]{64}", s)}
    if len(candidates) != 1:
        raise ValueError(f"push did not yield one immutable digest for {ref}")
    return candidates.pop().split("@", 1)[1]


# A push needs a tag, so each image is pushed under a staging tag in its own
# repository: repo:staging-<version tag>. Pushing it uploads the image and
# yields its digest without touching the version tag (#7619). A staging tag
# is mutable working state that a re-run overwrites; nothing pins it.
STAGING_PREFIX = "staging-"


def staging_ref(ref):
    repo, tag = ref.rsplit(":", 1)
    return f"{repo}:{STAGING_PREFIX}{tag}"


def stage_tag(ref):
    """Give the local image ref its staging tag and return that reference."""
    staged = staging_ref(ref)
    run("docker", "tag", ref, staged)
    return staged


def tag_digest(ref):
    """The digest a registry tag names now, or None when the tag is absent.

    Only the registry's explicit unknown-manifest answer is absence; any other
    lookup failure, or a present tag whose digest cannot be read, refuses."""
    result = subprocess.run(["docker", "manifest", "inspect", ref], text=True, capture_output=True)
    if result.returncode != 0:
        if "manifest unknown" in result.stderr.lower():
            return None
        raise ValueError(f"could not establish image-tag absence: {ref}: {result.stderr}")
    found = json.loads(run("docker", "buildx", "imagetools", "inspect", "--format",
                           "{{json .Manifest}}", ref, capture=True))
    digest = found.get("digest") if isinstance(found, dict) else None
    if not isinstance(digest, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError(f"could not read the digest image tag {ref} names")
    return digest


def attach_tag(ref, digest, *, sleep=time.sleep):
    """Point the version tag ref at the pushed digest and read it back."""
    repo = ref.rsplit(":", 1)[0]
    # --prefer-index=false copies the one manifest as it is. Without it buildx
    # wraps a single image in a new index, which has another digest.
    registry_push("docker", "buildx", "imagetools", "create", "--prefer-index=false",
                  "--tag", ref, repo + "@" + digest, sleep=sleep)
    got = tag_digest(ref)
    if got != digest:
        raise ValueError(f"{ref} names {got} after tagging, not the pushed {digest}")
    return digest


def attach_version_tags(wanted):
    """Attach each version tag to its pushed digest, once every push succeeded.

    wanted maps every version tag to the digest this run pushed for it. A tag
    already naming that digest is left as it is, so a re-run completes. A tag
    naming any other digest refuses the release before any tag moves."""
    attach, clash = {}, []
    for ref, digest in wanted.items():
        got = tag_digest(ref)
        if got is None:
            attach[ref] = digest
        elif got == digest:
            print(f"Image tag already names its pushed digest: {ref}@{digest}", flush=True)
        else:
            clash.append(f"{ref} names {got}, this run pushed {digest}")
    if clash:
        raise ValueError("image tag already exists; fix forward: " + "; ".join(clash))
    tagged = []
    try:
        for ref, digest in attach.items():
            attach_tag(ref, digest)
            tagged.append(ref)
    except Exception:
        print("Version tags attached before the failure: " + (" ".join(tagged) or "none"),
              file=sys.stderr, flush=True)
        raise


INDEX_TYPES = ("application/vnd.docker.distribution.manifest.list.v2+json",
               "application/vnd.oci.image.index.v1+json")


def publish_runner_index(ref, runner_refs):
    """Push each platform image, then the index over exactly those digests.

    Returns the index digest and the pushed platform digests. Everything is
    pushed under staging tags; the version tags are attached later. The
    reference pins the index, not one platform: the index bytes name every
    platform manifest by digest, so one immutable pin selects one exact image
    per platform. The index is read back by that digest and must list exactly
    the pushed platform digests, nothing else.
    """
    repo = ref.rsplit(":", 1)[0]
    pushed = {platform: push_digest(stage_tag(platform_ref))
              for platform, platform_ref in runner_refs.items()}
    staged = staging_ref(ref)
    run("docker", "manifest", "create", staged, *(repo + "@" + d for d in pushed.values()))
    out = registry_push("docker", "manifest", "push", "--purge", staged, capture=True).split()
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
    return digest, pushed
