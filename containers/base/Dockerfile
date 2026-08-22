# syntax=docker/dockerfile:1

# =============================================================================
# assay desk-base — the shared base image every per-desk container builds FROM.
#
# WHAT'S INSIDE
#   - Go toolchain (pinned to the `go` line in tools/desk/go.mod) + Python 3 +
#     pip, so a desk session can build and run the toolkit and its scripts.
#   - The desk-tools binary suite + statusgen, COPY --from the already-published
#     desk-tools image (the Go tree is NOT re-compiled here — the compiled,
#     version-stamped binaries are reused).
#   - git, the gh CLI, and CA certificates — enough to drive the desk loops
#     against GitHub from inside the container.
#   - The assay plugin (skills, commands, hooks) baked from this repo's
#     plugins/assay/ into /opt/assay/plugin, so a desk session finds its skill
#     by public desk name.
#   - A first-run installer for the interactive agent CLI (see NODE / AGENT CLI
#     below) — the CLI itself is NOT baked into this public image.
#   - A non-root `desk` user and a declared VOLUME /work for persistent working
#     state (clones, worktrees, and the persistent parts of $HOME).
#
# WHAT'S DELIBERATELY ABSENT
#   - NO credentials of any kind. No App PEM, no GitHub token, no model-endpoint
#     API key is COPYed, ADDed, passed as a build-arg, or set as an ENV default.
#     An image layer is distributable; credentials arrive at RUNTIME only. That
#     runtime-injection contract is a separate brief; nothing here reads or
#     bakes a secret.
#
# BASE DISTRO — debian:bookworm-slim (glibc), NOT the Alpine (musl) base the
#   root desk-tools image uses. Reason: this image additionally runs the
#   interactive agent CLI (a Node program with native components) and installs
#   Python wheels (manylinux, glibc). musl breaks both; the desk-tools image can
#   stay on Alpine because it ships only static, CGO-off Go binaries with no such
#   needs. Rationale is recorded in containers/README.md.
#
# PLATFORM: linux/amd64 first (single-arch by default); the arch-parameterised
#   RUN steps also build linux/arm64 under buildx.
# =============================================================================

# ---- Reused binary suite --------------------------------------------------
# Pin the already-published desk-tools image; its static binaries (desk-tools
# suite + statusgen) are copied into the final stage rather than recompiled.
ARG DESK_TOOLS_IMAGE=ghcr.io/medici-finance/assay/desk-tools:v0.1.0
FROM ${DESK_TOOLS_IMAGE} AS desktools

# ---- Base -----------------------------------------------------------------
FROM debian:bookworm-slim

# Target arch. Defaulted so a plain `docker build` produces linux/amd64 without
# buildx; a buildx build overrides TARGETARCH per platform.
ARG TARGETARCH=amd64

# Pinned toolchain versions.
#   GO_VERSION  — matches `go 1.25.0` in tools/desk/go.mod (and statusgen/go.mod).
#   GH_VERSION  — pinned gh CLI release.
#   NODE_VERSION — Node LTS; the runtime the interactive agent CLI installs onto.
ARG GO_VERSION=1.25.0
ARG GH_VERSION=2.63.2
ARG NODE_VERSION=22.11.0

# OCI label stamps, threaded through from CI (same pattern as the root
# Dockerfile). Defaults keep a plain local `docker build` working.
ARG VERSION=dev
ARG SOURCE_SHA=unknown
ARG BUILT_AT=unknown

ENV DEBIAN_FRONTEND=noninteractive

# System packages: CA certs, curl (to fetch pinned toolchains), git, Python 3 +
# pip + venv, and xz-utils (to unpack the Node tarball). No third-party apt repo
# and no key material is added.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        python3 \
        python3-pip \
        python3-venv \
        xz-utils; \
    rm -rf /var/lib/apt/lists/*

# Go toolchain, pinned, from the official distribution tarball.
RUN set -eux; \
    url="https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz"; \
    curl -fsSL "$url" -o /tmp/go.tgz; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm /tmp/go.tgz; \
    /usr/local/go/bin/go version

# gh CLI, pinned, from the official release tarball.
RUN set -eux; \
    dir="gh_${GH_VERSION}_linux_${TARGETARCH}"; \
    url="https://github.com/cli/cli/releases/download/v${GH_VERSION}/${dir}.tar.gz"; \
    curl -fsSL "$url" -o /tmp/gh.tgz; \
    tar -C /tmp -xzf /tmp/gh.tgz; \
    install -m 0755 "/tmp/${dir}/bin/gh" /usr/local/bin/gh; \
    rm -rf /tmp/gh.tgz "/tmp/${dir}"; \
    gh --version

# Node.js LTS, pinned, from the official distribution tarball. This is the
# runtime for the interactive agent CLI; the CLI itself is installed at first
# run (see /usr/local/bin/install-agent-cli), NOT baked here.
RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) narch=x64 ;; \
        arm64) narch=arm64 ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    dir="node-v${NODE_VERSION}-linux-${narch}"; \
    url="https://nodejs.org/dist/v${NODE_VERSION}/${dir}.tar.xz"; \
    curl -fsSL "$url" -o /tmp/node.txz; \
    tar -C /usr/local --strip-components=1 -xJf /tmp/node.txz; \
    rm /tmp/node.txz; \
    node --version; \
    npm --version

# desk-tools suite + statusgen: the compiled, version-stamped static binaries
# from the pinned desk-tools image, onto PATH. No source, no re-compile.
COPY --from=desktools /usr/local/bin/ /usr/local/bin/

# The assay plugin baked from this repo: skills, commands, hooks. A desk session
# finds its skill by public desk name under /opt/assay/plugin/skills.
COPY plugins/assay/ /opt/assay/plugin/

# Non-root user. Home is routed onto the persistent volume (see ENV below) so
# the parts of $HOME that must survive a container restart live on /work.
RUN set -eux; \
    groupadd --system desk; \
    useradd --system --gid desk --create-home --home-dir /home/desk --shell /bin/bash desk

# First-run installer for the interactive agent CLI. The CLI is proprietary and
# is NOT redistributed inside this public image; this script installs it into
# the persistent /work volume (via the npm prefix set below) so it persists
# across restarts. See containers/README.md for the licence decision.
RUN set -eux; \
    printf '%s\n' \
        '#!/bin/sh' \
        '# Install the interactive agent CLI into the persistent /work volume.' \
        '# The CLI is NOT baked into the public image (licence); it is installed' \
        '# here at first run and persists via the npm prefix under /work.' \
        'set -eu' \
        ': "${AGENT_CLI_PACKAGE:=@anthropic-ai/claude-code}"' \
        ': "${AGENT_CLI_VERSION:=latest}"' \
        'echo "Installing ${AGENT_CLI_PACKAGE}@${AGENT_CLI_VERSION} into ${NPM_CONFIG_PREFIX} ..."' \
        'npm install -g "${AGENT_CLI_PACKAGE}@${AGENT_CLI_VERSION}"' \
        'echo "Installed. ${NPM_CONFIG_PREFIX}/bin is already on PATH."' \
        > /usr/local/bin/install-agent-cli; \
    chmod 0755 /usr/local/bin/install-agent-cli

# Persistent working state lives on /work. Route $HOME and the XDG / toolchain
# caches under it so clones, worktrees, the go build cache, the npm-global
# prefix, and the persistent bits of desk config all survive a restart.
#   HOME=/work            — e.g. ~/.config/assay (minted-token cache) -> /work/.config/assay
#   NPM_CONFIG_PREFIX     — the agent CLI installs under /work and stays there
#   GOPATH / GOCACHE      — go module + build caches on the volume
ENV HOME=/work \
    XDG_CONFIG_HOME=/work/.config \
    XDG_CACHE_HOME=/work/.cache \
    XDG_DATA_HOME=/work/.local/share \
    XDG_STATE_HOME=/work/.local/state \
    GOPATH=/work/go \
    GOCACHE=/work/.cache/go-build \
    NPM_CONFIG_PREFIX=/work/.npm-global \
    ASSAY_PLUGIN_DIR=/opt/assay/plugin \
    PATH=/usr/local/go/bin:/work/.npm-global/bin:/work/go/bin:/usr/local/bin:/usr/bin:/bin

# Create /work owned by desk BEFORE declaring the volume, so a fresh named
# volume inherits desk ownership. VOLUME marks it as persistent working state.
RUN set -eux; \
    mkdir -p /work; \
    chown desk:desk /work
VOLUME /work

# OCI labels — same pattern as the root Dockerfile.
LABEL org.opencontainers.image.title="assay desk-base" \
      org.opencontainers.image.description="Shared base for the assay desk container images: Go + Python 3, git + gh, the desk-tools suite + statusgen, and the assay plugin, on a non-root user with a persistent /work volume. No credentials in any layer." \
      org.opencontainers.image.source="https://github.com/medici-finance/assay" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${SOURCE_SHA}" \
      org.opencontainers.image.created="${BUILT_AT}" \
      org.opencontainers.image.licenses="Apache-2.0"

USER desk
WORKDIR /work

# No ENTRYPOINT: this is a base. Per-desk images set ASSAY_DESK and the desk
# boot entrypoint on top. A bare `docker run` of the base drops into a shell.
CMD ["/bin/bash"]
