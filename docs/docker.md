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

# Desk images

The `desk-tools` image above carries the *binaries*. Running a whole **desk**
interactively (a session that boots into a desk's skill, with the toolchains and
the assay plugin on hand) uses a separate family of images under
[`containers/`](../containers/), all built from one shared **base image**.

## The base image

`containers/base/Dockerfile` is the shared base every per-desk image builds
`FROM`. It carries:

- the **Go toolchain** (pinned to `tools/desk/go.mod`) and **Python 3 + pip**;
- **git**, the **`gh` CLI**, and **CA certificates**;
- the **desk-tools suite + `statusgen`**, reused via
  `COPY --from=ghcr.io/medici-finance/assay/desk-tools` (not recompiled);
- the **assay plugin** (skills, commands, hooks) baked from `plugins/assay/` to
  `/opt/assay/plugin`;
- a first-run installer for the **interactive agent CLI** (installed into the
  persistent volume rather than baked — see `containers/README.md`);
- a **non-root `desk` user** and a declared **`VOLUME /work`** for persistent
  working state.

It is a **glibc** base (`debian:bookworm-slim`), not Alpine, because the agent
CLI and Python wheels need glibc; the reused desk-tools binaries are static and
run on it unchanged. **No credential of any kind is ever baked into the image** —
credentials arrive at runtime only.

Build it locally (from the repo root — the context is the repo root because the
image bakes `plugins/assay/`):

```sh
docker build -f containers/base/Dockerfile -t assay-desk-base:dev .
```

See [`containers/README.md`](../containers/README.md) for the full layout, the
distro and agent-CLI decisions, the `/work` layout, and the no-secrets rule.
