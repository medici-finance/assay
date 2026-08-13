# syntax=docker/dockerfile:1
#
# ci-tools — a carry-only image of pinned, public CI binaries.
#
# WHY THIS IMAGE EXISTS
#   A build environment with no egress to the GitHub release-asset CDN
#   (objects.githubusercontent.com) cannot `curl` a prebuilt tool straight from
#   a GitHub release. This image is built once and bakes those tools at stable
#   absolute paths so every downstream build pulls them with a registry
#   `COPY --from` instead of a network fetch:
#
#     COPY --from=ghcr.io/medici-finance/ci-tools:latest /usr/local/bin/gh    /usr/local/bin/gh
#     COPY --from=ghcr.io/medici-finance/ci-tools:latest /usr/local/bin/crane /usr/local/bin/crane
#
#   A registry pull is reachable from such a build; the GitHub CDN is not.
#
# HOW IT IS PINNED — `go install`, NOT a release-tarball download
#   Both tools are BUILT FROM SOURCE with `go install <module>@vX.Y.Z`. The Go
#   toolchain fetches the module source through GOPROXY (proxy.golang.org) and
#   verifies every module against the checksum database / go.sum
#   (GOSUMDB=sum.golang.org). That module+checksum verification IS the pin —
#   there is no release-CDN fetch and no tarball sha256 to carry. GOPROXY and
#   GOSUMDB are plain public HTTPS; the blocked GitHub release CDN is never
#   touched. The build is fully reproducible on any machine with Docker and
#   Go-proxy access (`docker build images/ci-tools`).
#
# PUBLIC-CLEAN BY DESIGN
#   Only pinned, public tools built from public source; no secrets, no private
#   config. The published package is a PUBLIC GHCR package so downstream builds
#   pull it anonymously (no registry auth / imagePullSecret needed).
#
# PINS (bump = edit the module version below; go.sum verification gates the
#       fetch and fails the build on any tampered module source):
#   gh    — github.com/cli/cli
#   crane — github.com/google/go-containerregistry
#
# ── Stage 1: build both tools from source via `go install` ────────────────
# Pinned Go toolchain. `GOTOOLCHAIN=local` forbids an on-the-fly toolchain
# download, so the build is deterministic on THIS Go version (bump the base
# tag if a tool's go.mod ever requires a newer toolchain).
FROM golang:1.26-bookworm AS build

ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOPROXY=https://proxy.golang.org,direct \
    GOSUMDB=sum.golang.org

ARG GH_VERSION=2.97.0
ARG CRANE_VERSION=0.21.9

# gh: set the build-info version vars so `gh --version` reports the pinned
# tag exactly (cosmetic — the module version is the real pin). The var paths
# are gh's own `internal/build` package.
RUN go install \
      -ldflags "-X github.com/cli/cli/v2/internal/build.Version=${GH_VERSION} -X github.com/cli/cli/v2/internal/build.Date=source-build" \
      "github.com/cli/cli/v2/cmd/gh@v${GH_VERSION}"

# crane: the go-containerregistry release stamps its version via this var.
RUN go install \
      -ldflags "-X github.com/google/go-containerregistry/cmd/crane/cmd.Version=v${CRANE_VERSION}" \
      "github.com/google/go-containerregistry/cmd/crane@v${CRANE_VERSION}"

# ── Stage 2: the tiny carry image ─────────────────────────────────────────
# ubuntu:24.04 — a minimal glibc base that carries only the two binaries plus a
# base rootfs. Consumers reference it only via `COPY --from`, so its own size is
# not on any runtime hot path; a shell here lets the smoke test below actually
# run. (The binaries are CGO_ENABLED=0 static, so they also run on a
# scratch/distroless base if that is ever preferred.)
FROM ubuntu:24.04

# Re-declare the version ARGs so they are in scope for the LABEL below (ARGs are
# reset at each FROM). Values default to the same pins as the build stage; the
# build passes no overrides, so they stay in lockstep.
ARG GH_VERSION=2.97.0
ARG CRANE_VERSION=0.21.9

# Recipe commit that produced this image (threaded from CI as a build-arg;
# defaults to "unknown" for a plain local `docker build`).
ARG SOURCE_SHA=unknown

COPY --from=build /go/bin/gh    /usr/local/bin/gh
COPY --from=build /go/bin/crane /usr/local/bin/crane

# Fail the build if either binary is missing or won't execute at its pinned path.
RUN set -eux; \
    /usr/local/bin/gh --version; \
    /usr/local/bin/crane version

# OCI metadata for a public package. `image.source` points at the public repo
# that holds this buildable recipe (per the OCI spec, image.source is the URL
# to the source code FOR BUILDING the image — i.e. this Dockerfile). The image
# is built privately on self-hosted runners; the recipe here is what makes it
# reproducible. `image.licenses` reflects the BUNDLED tools (gh is MIT, crane is
# Apache-2.0). image.version tracks the tool pins so it does not silently
# inherit the "24.04" version label from the ubuntu base.
LABEL org.opencontainers.image.title="ci-tools" \
      org.opencontainers.image.description="Pinned CI binaries (gh, crane) built from source via go install, for COPY --from in downstream image builds, so builds on egress-restricted runners need no GitHub release-CDN fetch." \
      org.opencontainers.image.source="https://github.com/medici-finance/assay" \
      org.opencontainers.image.revision="${SOURCE_SHA}" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0" \
      org.opencontainers.image.version="gh-${GH_VERSION}-crane-${CRANE_VERSION}"
