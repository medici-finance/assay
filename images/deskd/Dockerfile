# syntax=docker/dockerfile:1.27.1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
#
# deskd service image, built LOCALLY from the public deskd/ module.
#
# Build it with `make deskd-image` (context: deskd/ alone, so nothing outside the
# public module can be copied; images/deskd/Dockerfile.dockerignore keeps local
# build output and any SQLite index out of that context). The result is a local
# image named assay-deskd:<source SHA>. It is never an artifact pin: publishing
# it, and pinning the published reference, is the release's separately approved
# act.
#
# INSIDE the final image:
#   /deskd                               the static deskd binary (CGO off, -trimpath)
#   /etc/ssl/certs/ca-certificates.crt   CA trust bundle from the pinned build stage
#   /etc/passwd, /etc/group              one numeric non-root user, 65532:65532
#
# Deliberately ABSENT: a shell, a package manager, a container CLI and every
# terminal backend executable (the backends resolve them from PATH at run time;
# routing them is not this image's job). No CMD and no baked flag: the config,
# index, token and key paths and the listen address are the deployment profile's,
# so deskd keeps its loopback default bind. No credential, no secret mount, no
# token-named ENV or ARG.
#
# Pins: the frontend above and the Go builder below are pinned by tag AND
# sha256 digest, reusing the repo's existing references (containers/base,
# images/verify-runner). The final stage is FROM scratch, which pulls nothing.
# Modules are fetched against deskd/go.sum with checksum verification left on.
#
# tools/desk/internal/cellapi/deskdimage_test.go holds every property above.

FROM golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 AS build

ENV GOWORK=off \
    CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly

WORKDIR /src

# Modules first, so the download layer is reused across source edits. go mod
# verify re-checks the downloaded tree against go.sum.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /out/deskd ./cmd/deskd

RUN printf 'deskd:x:65532:65532:deskd:/:/nonexistent\n' > /out/passwd && \
    printf 'deskd:x:65532:\n' > /out/group

FROM scratch

ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/medici-finance/assay" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.title="deskd"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/passwd /etc/passwd
COPY --from=build /out/group /etc/group
COPY --from=build /out/deskd /deskd

USER 65532:65532
ENTRYPOINT ["/deskd"]
