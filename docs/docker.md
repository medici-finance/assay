# Container image

The desk-tools suite and `statusgen` are published together as one combined
container image, so you can run the desk loops without installing a Go
toolchain or the binaries locally.

## Image

```
ghcr.io/medici-finance/assay/desk-tools
```

`desk-tools` is the recognised suite name; `statusgen` rides in the same image.

Tags:

- `:vX.Y.Z` — a released version.
- `:latest` — the most recently published version.
- `:sha-<short>` — the exact source commit an image was built from.

## What's inside

- Every desk-tools binary (`deskboard`, `deskpost`, `deskpr`, `verifyloop`,
  `writeguard`, … — the tools under `tools/desk/cmd/*`).
- `statusgen`, the board linter/generator.
- `git`, the `gh` CLI, and CA certificates — enough to run the desk loops
  against GitHub from inside the container.

All binaries are on `PATH` (`/usr/local/bin`) and are version-stamped, so a
running container can report exactly what it is:

```sh
docker run --rm ghcr.io/medici-finance/assay/desk-tools deskboard --version
docker run --rm ghcr.io/medici-finance/assay/desk-tools statusgen --version
```

A bare `docker run` prints the tool list and a usage hint.

## Pull and run

```sh
docker pull ghcr.io/medici-finance/assay/desk-tools:latest

# Run a specific tool (each binary is on PATH):
docker run --rm ghcr.io/medici-finance/assay/desk-tools statusgen --version

# Lint/generate a board for a repo mounted into the container:
docker run --rm -v "$PWD:/work" -w /work \
  ghcr.io/medici-finance/assay/desk-tools statusgen --root .
```

## Supported platforms

Linux only: **`linux/amd64`**. A container is Linux, so there is no macOS or
Windows image here.

Native macOS and Windows binaries are a **separate artifact**, not part of this
image.

## How it's built and published

- `Dockerfile` (repo root) is a multi-stage build: a `golang:1.25-alpine`
  builder compiles every binary `CGO_ENABLED=0` (static) with the same version
  stamps the release workflows use, and a small `alpine` final stage carries
  the binaries plus `git`, `gh`, and `ca-certificates` under a non-root user.
- `.github/workflows/docker-publish.yml` builds the image and pushes to GHCR.
  It runs on a `desk-tools/v*` tag push, or via `workflow_dispatch` (with a
  `dry_run` option that builds without publishing).
