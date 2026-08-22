# containers/

Container images and launch surfaces for running an assay desk from a desktop.

Each assay desk (`intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`,
and the coordinator `the-desk`) is packaged as its own versioned image, all
built from one shared **base image**. This directory holds the base and, as they
land, the per-desk images and launch surfaces.

## Layout

| Path | What it is |
|------|------------|
| `base/Dockerfile` | The shared base image every per-desk image builds `FROM`. |
| `README.md` | This map. |
| `secrets.md` | Runtime credential-injection contract + layer scan (planned). |

Per-desk Dockerfiles (`<desk-name>/Dockerfile`), an interactive launch script,
a compose file, Kubernetes manifests, and a multi-desk control layer land in
later work and are added to this table as they arrive.

## The base image

The base carries everything a desk session needs that is common across desks:

- **Go toolchain**, pinned to the `go` line in `tools/desk/go.mod`.
- **Python 3 + pip**.
- **git, the `gh` CLI, and CA certificates**.
- The **desk-tools binary suite + `statusgen`**, copied from the already-published
  `ghcr.io/medici-finance/assay/desk-tools` image (`COPY --from`) rather than
  recompiled — the compiled, version-stamped static binaries are reused.
- The **assay plugin** (skills, commands, hooks) baked from `plugins/assay/` into
  `/opt/assay/plugin`, so a desk session finds its skill by public desk name.
- A first-run installer for the **interactive agent CLI** (see below).
- A **non-root `desk` user** and a declared **`VOLUME /work`** for persistent
  working state.

### Build it locally

```sh
# From the repo ROOT — the build context is the repo root because the image
# bakes plugins/assay/ (COPY needs it in context), so pass the Dockerfile with
# -f and use "." as the context:
docker build -f containers/base/Dockerfile -t assay-desk-base:dev .
```

Version stamps are optional build-args (defaults keep a plain local build
working):

```sh
docker build -f containers/base/Dockerfile \
  --build-arg VERSION=v0.0.0-dev \
  --build-arg SOURCE_SHA="$(git rev-parse --short HEAD)" \
  --build-arg BUILT_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -t assay-desk-base:dev .
```

## Decisions and their reasons

### Base distro: Debian (`bookworm-slim`), not Alpine

The root `desk-tools` image is Alpine (musl), which is fine there: it ships only
static, `CGO_ENABLED=0` Go binaries. The desk-base additionally runs the
interactive agent CLI (a Node program with native components) and installs
Python wheels (manylinux, glibc). musl breaks both native Node modules and
manylinux wheels, so the base uses a **glibc** distro (`debian:bookworm-slim`).
The reused desk-tools binaries are static and run unchanged on Debian.

### Interactive agent CLI: installed at first run, not baked

The interactive agent CLI is **not** copied into this image. It is proprietary
software whose terms do not grant redistribution rights, and this image is
published to a public registry — baking the CLI in would redistribute it.

Instead the base installs the Node runtime and ships
`/usr/local/bin/install-agent-cli`, which installs the CLI into the persistent
`/work` volume at first run (via an npm prefix under `/work`), so it persists
across container restarts without ever living in an image layer:

```sh
install-agent-cli            # installs the default CLI package @ latest
AGENT_CLI_VERSION=x.y.z install-agent-cli   # pin a version
```

If the CLI's terms later permit redistribution in a public image, this can move
to a baked install; the persistent-`/work` fallback is the conservative choice
until then.

### `/work` layout (persistent volume)

`/work` is the single persistent mount for a desk. `$HOME` is routed to `/work`
so the parts of home that must survive a restart live on the volume:

| Location | Holds |
|----------|-------|
| `/work` | `$HOME`; clones and worktrees. |
| `/work/.config/assay` | Persistent desk config (e.g. the minted-token cache). |
| `/work/.npm-global` | The first-run agent CLI install (on `PATH`). |
| `/work/go`, `/work/.cache/go-build` | Go module and build caches. |
| `/work/.cache`, `/work/.local` | XDG cache / data / state. |

Volumes are per-desk — two desks never share a writable working tree.

## No secret in any image layer

**No credential of any kind is ever baked into these images.** No App PEM, no
GitHub token, and no model-endpoint API key is `COPY`ed or `ADD`ed, passed as a
build-arg, or set as an image `ENV` default. An image layer is distributable;
credentials are not. All credentials arrive at **runtime** via mounted secrets
and runtime environment variables.

The runtime credential-injection contract, and an automated image-layer scan
that fails the build if key-shaped material appears in any layer, are defined in
[`secrets.md`](secrets.md) (planned). Until that scan lands, the base
`Dockerfile` is checkable by hand: no `COPY`/`ADD` line names key material, and
`docker history --no-trunc` on a built image carries no key-shaped strings.
