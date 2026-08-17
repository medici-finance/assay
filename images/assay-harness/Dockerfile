# assay-harness — the free-tier headless-agent harness image.
#
# This image lives in `medici-finance/assay`, the release home, because the
# container-channel work is OSS-release infrastructure and belongs where the
# binaries and images are cut. Ian's ruling 2026-08-17.
#
# WHY THIS IMAGE EXISTS, and how it differs from the combined desk-tools image.
# assay already ships ONE combined image (../../Dockerfile + docker-publish.yml):
# the desk-tools suite + statusgen on PATH, for RUNNING the desk loops. THIS
# image is different: it additionally bakes the `assay` PLUGIN, pre-seeded via
# CLAUDE_CODE_PLUGIN_SEED_DIR, so a headless agent in a pod starts with the
# methodology installed and NO runtime clone / no interactive `/plugin install`.
# The per-image narrative for this repo's images is assay `docs/docker.md`.
#
# BUILD CONTEXT IS THE REPOSITORY ROOT, not this directory:
#     docker build -f images/assay-harness/Dockerfile \
#        --build-arg ASSAY_VERSION=v0.9.1 \
#        -t assay-harness:local .
# because the image builds `statusgen` (./statusgen) and the `desk-tools`
# commands (./tools/desk/cmd/*) from this repo's sources and seeds the plugin
# from `.claude-plugin/` + `plugins/`. NOTE: in assay the umbrella version is a
# plain `vX.Y.Z` tag, so ASSAY_VERSION is `v0.9.1`, not `assay/v0.9.1`.
#
# THE BINARIES ARE STAMPED, NOT A SOURCE BUILD. `ASSAY_VERSION` is REQUIRED and
# the build refuses an empty one (below): an unstamped `statusgen` self-reports
# `dev`, and an image whose tools cannot say which release they are is the stale
# install the version stamp exists to end (statusgen/main.go, statusgenVersion).
# This is the container-channel analogue of the release workflow's ldflag stamp.
#
# PUBLICATION IS GATED: this Dockerfile BUILDS an image; whether it is pushed to
# a public registry is a human decision the release workflow gates behind a flag
# — nothing here pushes anything.
#
# WHY EVERY BASE/TOOLCHAIN VERSION IS PINNED BY DIGEST: a base image that drifts
# silently is a fleet-wide toolchain change nobody reviewed. Base images are
# pinned by DIGEST (a tag is mutable at a registry). assay's sibling
# `images/ci-tools/Dockerfile` pins by tag, not digest, so these are STANDALONE
# digest pins with no sibling to keep them in lockstep; bump them deliberately
# (they are the linux/amd64 golang:1.25.0-bookworm and debian:bookworm-slim
# artifacts as of this image's addition). See the PR body's base-digest note.
#
# TARGET PLATFORM is linux/amd64 (the ARC runner convention).

# ---------------------------------------------------------------------------
# Stage 1 — build this repo's toolkit binaries, stamped.
# ---------------------------------------------------------------------------
# --platform=$BUILDPLATFORM keeps the Go toolchain on the builder's arch and
# cross-compiles to linux/amd64 rather than running under qemu emulation.
FROM --platform=$BUILDPLATFORM golang:1.25.0-bookworm@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a AS gobuild

# GOWORK=off is load-bearing: the repo root carries no go.work, and statusgen/
# and tools/desk are self-contained modules (only external dep is
# gopkg.in/yaml.v3, no `replace` directives), so each builds from its own dir with
# GOWORK off. GOARCH is fixed to amd64 to match the runtime stage's platform.
ENV GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# The version stamps. ASSAY_VERSION is the umbrella tag and is REQUIRED; the two
# desk stamps are optional (empty -> the binary self-reports UNPINNED, a warning,
# not a build failure). STATUSGEN_TAG defaults to the umbrella version.
ARG ASSAY_VERSION=
ARG STATUSGEN_TAG=
ARG DESK_SOURCE_SHA=
ARG DESK_BUILT_AT=

# FAIL CLOSED: an image with unstamped tools is a source build wearing a release
# tag. Refuse it here rather than ship a `statusgen --version` that prints `dev`.
RUN if [ -z "${ASSAY_VERSION}" ]; then \
      echo "ASSAY_VERSION build-arg is required — refusing to build a harness whose binaries self-report 'dev' (see the Dockerfile header)" >&2; \
      exit 1; \
    fi

# statusgen — its own module, stamped with main.statusgenVersion.
WORKDIR /src/statusgen
COPY statusgen/go.mod statusgen/go.sum ./
RUN go mod download
COPY statusgen/ ./
RUN SG_TAG="${STATUSGEN_TAG:-${ASSAY_VERSION}}"; \
    go build -trimpath -ldflags="-s -w -X main.statusgenVersion=${SG_TAG}" \
      -o /out/statusgen .

# desk-tools — its own module; every command under cmd/*, stamped with the
# deskkit SourceSHA/BuiltAt pair IsPinned() requires (matches release.yml).
WORKDIR /src/desk
COPY tools/desk/go.mod tools/desk/go.sum ./
RUN go mod download
COPY tools/desk/ ./
RUN DESK_LD="-s -w"; \
    if [ -n "${DESK_SOURCE_SHA}" ]; then DESK_LD="${DESK_LD} -X github.com/medici-finance/assay/tools/desk/internal/deskkit.SourceSHA=${DESK_SOURCE_SHA}"; fi; \
    if [ -n "${DESK_BUILT_AT}" ];  then DESK_LD="${DESK_LD} -X github.com/medici-finance/assay/tools/desk/internal/deskkit.BuiltAt=${DESK_BUILT_AT}"; fi; \
    built=0; \
    for cmd in ./cmd/*/; do \
      [ -d "$cmd" ] || continue; \
      name="$(basename "$cmd")"; \
      go build -trimpath -ldflags="${DESK_LD}" -o "/out/${name}" "./cmd/${name}"; \
      built=$((built + 1)); \
    done; \
    if [ "$built" -eq 0 ]; then echo "tools/desk/cmd/*/ matched no packages — refusing to build an empty harness" >&2; exit 1; fi

# ---------------------------------------------------------------------------
# Stage 2 — the runtime image.
# ---------------------------------------------------------------------------
FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

# Small, stable OS utilities a headless agent needs: TLS roots, git for the
# clone a worker Job does into HOME, tini to reap child processes, a shell.
# apt packages are deliberately NOT version-pinned — Debian's archive serves one
# version per suite, so an `=`-pin breaks the build on every point release; the
# base-image digest above freezes the starting filesystem instead.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      git \
      tini \
 && rm -rf /var/lib/apt/lists/*

# The toolkit binaries, from stage 1. statusgen + every desk-tools command land
# on PATH at /usr/local/bin — this is the container-channel equivalent of the
# binary install the CLIs ship through, so `statusgen --version` answers here.
COPY --from=gobuild /out/ /usr/local/bin/

# The assay plugin, pre-seeded at BUILD time. CLAUDE_CODE_PLUGIN_SEED_DIR
# pre-populates marketplaces+plugins read-only with no runtime clone — the
# headless install path. Loose skill copies do not install in a container; the
# plugin is what does.
COPY .claude-plugin/ /opt/assay/plugin-seed/assay/.claude-plugin/
COPY plugins/        /opt/assay/plugin-seed/assay/plugins/
ENV CLAUDE_CODE_PLUGIN_SEED_DIR=/opt/assay/plugin-seed

# Provenance labels. The repo URL comes from a build ARG the workflow fills from
# ${{ github.server_url }}/${{ github.repository }} rather than being baked here —
# a slug written into a Dockerfile ships wherever the file does.
ARG ASSAY_VERSION=
ARG SOURCE_URL=
ARG SOURCE_REVISION=
LABEL org.opencontainers.image.title="assay-harness" \
      org.opencontainers.image.description="Headless-agent harness: assay toolkit binaries + the assay plugin pre-seeded via CLAUDE_CODE_PLUGIN_SEED_DIR" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.revision="${SOURCE_REVISION}" \
      org.opencontainers.image.version="${ASSAY_VERSION}"

# Non-root by construction: a worker Job is ephemeral and clones into HOME; the
# uid is fixed so a securityContext can pin runAsUser without reading the image.
RUN useradd --create-home --uid 10001 --shell /usr/sbin/nologin harness
USER 10001
ENV HOME=/home/harness \
    GIT_TERMINAL_PROMPT=0
WORKDIR /home/harness

# tini reaps the agent runtime's children; the default command prints the stamp
# so `docker run <image>` with no args is a self-identifying smoke check.
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["statusgen", "--version"]
