# syntax=docker/dockerfile:1

# =============================================================================
# assay house CI-runner tools — one combined Linux runtime image (linux/amd64).
#
# Historically "desk-tools + statusgen"; this image is the house's dedicated
# CI-runner-tools bundle: the desk loop binaries plus the standalone
# single-purpose Go tools the CI loops shell out to. It is DISTINCT from
# images/ci-tools (gh + crane, a COPY --from provisioning layer) — the tools
# here are the house's own compiled Go binaries.
#
# WHAT'S INSIDE
#   - The desk-tools suite: every binary under tools/desk/cmd/* (deskboard,
#     deskpost, deskpr, verifyloop, writeguard, … — the tools that drive the
#     desk loops and their guard rails).
#   - statusgen: the board linter/generator. It is a separate Go module but
#     rides in this same image under the recognised "desk-tools" suite name,
#     a first-class citizen on PATH alongside the desk binaries.
#   - bugs-gc: prunes bugs/<N>.md carriers whose GitHub issue has closed
#     (its own stdlib-only Go module; generic over --root).
#   - freshness: repo-agnostic staleness checker (its own Go module).
#
# All binaries land in /usr/local/bin (on PATH) and are VERSION-STAMPED so a
# running container can report exactly what it is:
#   - desk-tools binaries carry SourceSHA + BuiltAt (deskkit.Version()) —
#     `deskboard --version` prints the source SHA / build time.
#   - statusgen carries its version string — `statusgen --version` prints it.
# The stamps arrive as build-args, so an image tagged :vX self-reports vX and a
# pin can be checked from inside the container.
#
# PLATFORM: linux/amd64. Built with plain `docker build` (single-arch); a
# container is Linux, and native macOS / Windows binaries are a SEPARATE
# artifact, NOT built here.
#
# This image is for RUNNING the desk loops in-container, so it bakes in git,
# the gh CLI and ca-certificates alongside the compiled binaries. The Go
# toolchain and the source tree are intentionally absent from the final image —
# the compiled, stamped binaries are the product; the go-run fallback is not.
# =============================================================================

# ---- Builder --------------------------------------------------------------
# golang:1.25 matches the `go 1.25.0` line in both go.mod files (tools/desk and
# statusgen). Alpine base, CGO off -> fully static Linux binaries.
FROM golang:1.25-alpine AS builder

# Target GOOS/GOARCH. Defaulted so a plain `docker build` produces linux/amd64
# without buildx; a buildx build would override these per platform.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Version stamps, threaded through from CI (.github/workflows/docker-publish.yml).
# The defaults keep a plain `docker build` working: an unstamped desk binary
# self-reports "unpinned", which is the correct signal for a local/dev build.
ARG VERSION=dev
ARG SOURCE_SHA=unknown
ARG BUILT_AT=unknown

WORKDIR /src

# Warm the module cache first so source edits don't re-download deps. The desk
# module is stdlib-only (no go.sum); statusgen pulls one dependency (yaml.v3).
COPY statusgen/go.mod statusgen/go.sum ./statusgen/
COPY tools/desk/go.mod ./tools/desk/
RUN cd statusgen && go mod download

# Sources.
COPY statusgen/ ./statusgen/
COPY tools/desk/ ./tools/desk/

ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

# desk-tools: build every cmd under tools/desk/cmd/* with the SourceSHA / BuiltAt
# stamps release-desk uses (same -X targets as tools/desk/internal/deskkit).
RUN set -eux; \
    DESK_PKG="github.com/medici-finance/assay/tools/desk/internal/deskkit"; \
    LDFLAGS="-X ${DESK_PKG}.SourceSHA=${SOURCE_SHA} -X ${DESK_PKG}.BuiltAt=${BUILT_AT}"; \
    mkdir -p /out; \
    cd tools/desk; \
    for cmd in ./cmd/*/; do \
        name="$(basename "$cmd")"; \
        echo "building desk-tools/$name"; \
        go build -trimpath -ldflags "$LDFLAGS" -o "/out/$name" "$cmd"; \
    done

# statusgen: build with the -X main.statusgenVersion stamp release-statusgen
# uses. In this COMBINED image statusgen is stamped with the image VERSION so it
# reports the same version the image is tagged with.
RUN set -eux; \
    cd statusgen; \
    go build -trimpath -ldflags "-X main.statusgenVersion=${VERSION}" -o /out/statusgen .

# bugs-gc: its own stdlib-only module (de-housed from the toolkit). No stamp —
# the tool carries no version var; the module source is the pin.
COPY tools/bugs-gc/ ./tools/bugs-gc/
RUN set -eux; \
    cd tools/bugs-gc; \
    go build -trimpath -o /out/bugs-gc .

# freshness: repo-agnostic staleness checker, its own module (deps: yaml.v3).
COPY tools/freshness/ ./tools/freshness/
RUN set -eux; \
    cd tools/freshness; \
    go mod download; \
    go build -trimpath -o /out/freshness .

# leaksweep joins this image once its human-gated security-review clears and its
# source lands in assay; metrics-harvest and loopresolve likewise join once their
# publication-boundary rulings clear and they land. Not built here until then.

# ---- Final ----------------------------------------------------------------
# Small runtime: Alpine + git + gh CLI + ca-certificates. github-cli lives in
# the Alpine community repo, so gh installs cleanly with apk — no third-party
# package repo needed.
FROM alpine:3.21

RUN apk add --no-cache git github-cli ca-certificates \
    && addgroup -S desk \
    && adduser -S -G desk -h /home/desk desk

# Binaries onto PATH (desk-tools suite + statusgen + freshness + bugs-gc).
COPY --from=builder /out/ /usr/local/bin/

# Re-declare in this stage so the OCI labels can read the build-args (ARG values
# do not cross stage boundaries).
ARG VERSION=dev
ARG SOURCE_SHA=unknown
ARG BUILT_AT=unknown
LABEL org.opencontainers.image.title="assay desk-tools" \
      org.opencontainers.image.description="house CI-runner tools: desk-tools suite + statusgen + freshness + bugs-gc, static Linux binaries on PATH" \
      org.opencontainers.image.source="https://github.com/medici-finance/assay" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${SOURCE_SHA}" \
      org.opencontainers.image.created="${BUILT_AT}" \
      org.opencontainers.image.licenses="Apache-2.0"

USER desk
WORKDIR /home/desk

# No ENTRYPOINT on purpose: every binary is on PATH, so a tool is invoked
# directly, e.g. `docker run --rm <image> statusgen --version` or
# `docker run --rm <image> deskboard --version`. A bare `docker run <image>`
# falls through to this CMD, which prints what's inside and how to run it.
CMD ["/bin/sh", "-c", "echo 'assay house CI-runner tools (desk-tools + statusgen + freshness + bugs-gc)'; echo; echo 'Tools on PATH (/usr/local/bin):'; ls /usr/local/bin; echo; echo 'Run one directly, e.g.: docker run --rm <image> statusgen --version'"]
