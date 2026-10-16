# verify-runner — the sandbox image a scheduled verifier runs Verify rows inside.
#
# A runner executes each row in a container with no network, no bind mount and
# no host namespace, so everything a row needs must already be in the image: the
# Go toolchain, `unshare`, `curl`, `getent` (all from the golang bookworm base)
# and the released linux `statusgen` on PATH. Consumers pin the image by registry
# digest (`<repo>:<tag>@sha256:<digest>`), never by tag or local image id.
#
# BUILD CONTEXT IS A DIRECTORY HOLDING ONE FILE, `statusgen`: the umbrella
# release's own `statusgen-linux-amd64` asset, sha256-checked against that
# release's checksums.txt before the build. The image builds nothing from
# source, so the binary inside is byte-for-byte the one consumers pin in
# .assay-versions. The release workflow's `verify-runner` job does this:
#     d=$(mktemp -d)
#     cp statusgen-linux-amd64 "$d/statusgen" && chmod 755 "$d/statusgen"
#     docker build --platform linux/amd64 -f images/verify-runner/Dockerfile \
#        -t verify-runner:local "$d"
#
# THE BASE IS PINNED BY DIGEST, to the fleet's single Go toolchain: the same
# golang:1.27.2-bookworm digest images/assay-harness/Dockerfile builds on, and
# the version deskkit's goversion test holds every Dockerfile to. A drifting base
# is a toolchain change nobody reviewed; it moves only with a fleet Go bump.
FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61

COPY statusgen /usr/local/bin/statusgen
