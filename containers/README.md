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
| `intake-desk/Dockerfile` | Thin per-desk image: `FROM desk-base` + `ASSAY_DESK=intake-desk` + the shared entrypoint. |
| `worker-desk/Dockerfile` | Thin per-desk image for `worker-desk`. |
| `pr-review-desk/Dockerfile` | Thin per-desk image for `pr-review-desk`. |
| `verify-desk/Dockerfile` | Thin per-desk image for `verify-desk`. |
| `the-desk/Dockerfile` | Thin per-desk image for the coordinator `the-desk`. |
| `entrypoint.sh` | Shared interactive boot baked into all five desk images: fail-closed credential preflight, then desk identity + skill pointer, then the interactive session. |
| `scripts/layer-secret-scan.sh` | Fails a built image that carries key-shaped material in any layer/config/history. |
| `secrets.md` | Runtime credential-injection contract (normative). |
| `README.md` | This map. |

An interactive launch script, a compose file, Kubernetes manifests, and a
multi-desk control layer land in later work and are added to this table as they
arrive.

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

## The per-desk images

Each desk is its own image, so "run the pr-review-desk" is a single image
reference. The five images are **thin**: over the shared base they add only

- `ENV ASSAY_DESK=<desk-name>` — the desk's public name, and
- the shared **`entrypoint.sh`**, installed as `/usr/local/bin/desk-entrypoint`.

Nothing else differs between them. The entrypoint reads `ASSAY_DESK`, runs the
fail-closed credential preflight (see [`secrets.md`](secrets.md) §5), prints the
desk's identity and a by-name pointer to its skill under
`/opt/assay/plugin/skills/<desk-name>`, and then execs into the interactive
session (an interactive shell by default) from which the operator invokes
`/<desk-name>`.

### Build them locally

The build context is the **repository root** (as for the base image), because
the shared entrypoint lives at `containers/entrypoint.sh`, one level above each
desk directory. Build the base first under the tag the desk `BASE_IMAGE`
build-arg defaults to, then each desk against it:

```sh
# 1. the base, tagged for local desk builds:
docker build -f containers/base/Dockerfile -t desk-base:dev .

# 2. each desk (BASE_IMAGE defaults to desk-base:dev):
for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do
  docker build -f "containers/$d/Dockerfile" -t "assay/$d:dev" .
done
```

`BASE_IMAGE` is a build-arg so CI can version-lock each desk to the exact base
tag it just built (`--build-arg BASE_IMAGE=ghcr.io/medici-finance/assay/desk-base:vX.Y.Z`);
a desk image is **never** built against a floating `latest` base.

## Decisions and their reasons

### Image names: `ghcr.io/medici-finance/assay/<desk-name>`, base `…/desk-base`

The five desk images are published as `ghcr.io/medici-finance/assay/<desk-name>`
— `…/intake-desk`, `…/worker-desk`, `…/pr-review-desk`, `…/verify-desk`,
`…/the-desk` — and the base as `ghcr.io/medici-finance/assay/desk-base`.

Decision on the `desk-` prefix (spec open question 4): **no extra prefix on the
five desk images.** Their names already read as desk names — four of the five
literally end in `-desk`, and `the-desk` is the coordinator's own name — so a
`desk-` prefix would produce awkward, redundant names (`desk-the-desk`,
`desk-worker-desk`). The base keeps the descriptive `desk-base` name it was
given in brief 01. The result is that a desk's image reference is exactly its
public desk name, which is the whole point of the request (#63): "call each one
by their desk name."

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

The runtime credential-injection contract, and the automated image-layer scan
that fails the build if key-shaped material appears in any layer, are defined in
[`secrets.md`](secrets.md) and implemented by
[`scripts/layer-secret-scan.sh`](scripts/layer-secret-scan.sh). The publish
workflow runs that scan against the base **and all five desk images** and fails
the publish on any hit, so the rule is enforced mechanically on every build —
not just by review. The per-desk layers add no `COPY`/`ADD` of key material and
no credential `ENV` default; the credential preflight in `entrypoint.sh` reads
the runtime mount/env only.
