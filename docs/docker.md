> Release ownership is changing: see [release images and consumer pins](release-images.md).
> The staged umbrella workflow produces the complete image set and immutable
> references; consumer re-pins only read those published outputs.

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

- `Dockerfile` (repo root) is a multi-stage build: a `golang:1.27.3-alpine`
  builder compiles every binary `CGO_ENABLED=0` (static) with the same version
  stamps the release workflows use, and a small `alpine` final stage carries
  the binaries plus `git`, `gh`, and `ca-certificates` under a non-root user.
- `.github/workflows/release.yml` owns image publication as part of the umbrella
  release. Its single approved release job uses the prepared Linux binaries,
  builds/checks all nine images, pushes them, then publishes the tag and release
  assets with checksummed immutable image references. `dry_run` builds and checks
  without publishing. See [the release contract](release-images.md).

# Desk images

The `desk-tools` image above carries the *binaries*. Running a whole **desk**
interactively (a session that boots into a desk's skill, with the toolchains and
the assay plugin on hand) uses a separate family of images under
[`containers/`](../containers/), all built from one shared **base image**.

## The base image

`containers/base/Dockerfile` is the shared base every per-desk image builds
`FROM`. It carries:

- the **Go toolchain** (pinned to `tools/desk/go.mod`) and **Python 3 + pip**;
- **git 2.41 or later** (built from a pinned upstream release and checked at
  build time by `containers/scripts/git-floor-check.sh`), the **`gh` CLI**, and
  **CA certificates**;
- the **desk-tools suite + `statusgen`**, reused via
  `COPY --from=ghcr.io/medici-finance/assay/desk-tools` (not recompiled);
- the **assay plugin** (skills, commands, hooks) baked from `plugins/assay/` to
  `/opt/assay/plugin`;
- a first-run installer for the **interactive agent CLI** (installed into the
  persistent volume rather than baked — see `containers/README.md`);
- a **non-root `desk` user** (UID/GID pinned to 999) and a declared
  **`VOLUME /work`** for persistent working state.

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

## The five desk images

Each desk is published as its own image, named by the desk, so running a desk is
a single image reference. Every desk image is a **thin** layer over the base: it
adds only `ENV ASSAY_DESK=<desk-name>` and the shared boot entrypoint (see
`containers/<desk-name>/Dockerfile`), and is version-locked to the same base tag.

| Image | Desk | `ASSAY_DESK` |
|-------|------|--------------|
| `ghcr.io/medici-finance/assay/desk-base` | *(shared base — not run directly)* | — |
| `ghcr.io/medici-finance/assay/intake-desk` | intake-desk | `intake-desk` |
| `ghcr.io/medici-finance/assay/worker-desk` | worker-desk | `worker-desk` |
| `ghcr.io/medici-finance/assay/pr-review-desk` | pr-review-desk | `pr-review-desk` |
| `ghcr.io/medici-finance/assay/verify-desk` | verify-desk | `verify-desk` |
| `ghcr.io/medici-finance/assay/the-desk` | the-desk (coordinator) | `the-desk` |

### Tags

All six images share the same tag scheme as `desk-tools`, and the five desk
images are always built against the **same** version tag of the base:

- `:vX.Y.Z` — a released version.
- `:latest` — the most recently published version.
- `:sha-<short>` — the exact source commit an image was built from.

### What's inside

Everything in the base (Go + Python 3, git + `gh`, the desk-tools suite +
`statusgen`, the assay plugin under `/opt/assay/plugin`, a non-root `desk` user,
a persistent `/work` volume — see the base section above), plus the desk's
identity and the shared boot entrypoint. The entrypoint runs a **fail-closed
credential preflight** (it refuses to start without the mounted App PEM and a
runtime model credential — see [`containers/secrets.md`](../containers/secrets.md)),
prints the desk's identity and skill pointer, then lands in the interactive
session.

### Run a desk

A desk needs its credentials injected at runtime — the bot App PEM mounted
read-only and the model environment supplied from an operator-held env-file (the
full contract is in [`containers/secrets.md`](../containers/secrets.md)). One
example per desk (substitute the paths and tag):

```sh
# Shared flags: read-only PEM mount + model env-file + a per-desk work volume.
PEM=--mount=type=bind,src=/path/to/app.pem,dst=/run/secrets/assay/app.pem,ro
ENVF=--env-file=/path/to/desk.env

docker run -it "$PEM" "$ENVF" -v intake-work:/work \
  ghcr.io/medici-finance/assay/intake-desk:latest
docker run -it "$PEM" "$ENVF" -v worker-work:/work \
  ghcr.io/medici-finance/assay/worker-desk:latest
docker run -it "$PEM" "$ENVF" -v prreview-work:/work \
  ghcr.io/medici-finance/assay/pr-review-desk:latest
docker run -it "$PEM" "$ENVF" -v verify-work:/work \
  ghcr.io/medici-finance/assay/verify-desk:latest
docker run -it "$PEM" "$ENVF" -v thedesk-work:/work \
  ghcr.io/medici-finance/assay/the-desk:latest
```

Run without the PEM mount or without a model credential and the desk exits
non-zero, naming exactly what is missing — the fail-closed preflight.

### Run a desk from your desktop

[`containers/desk-run.sh`](../containers/desk-run.sh) composes that `docker run` for
you, so the credentials are mounted the same way every time instead of being
hand-assembled. It validates the desk name, checks that your credential files
exist, and starts the desk interactively (`-it`) with a per-desk work volume.
The script comes from a checkout of this repository
(`git clone https://github.com/medici-finance/assay.git && cd assay`); the
commands below run from the checkout root.

**Prerequisites**

- docker, with the daemon running.
- The bot App PEM for your desk's App, as a file on your machine. Each desk has
  its own default location, `~/.config/assay/<desk-name>.pem` (for example
  `~/.config/assay/worker-desk.pem`), so two desks started on defaults never
  mount the same file, and a missing file refuses rather than borrowing another
  desk's. Put each desk's own App key at its own path. The launcher mounts the
  file you point it at and does not check which App that file belongs to.
  `ASSAY_DESK_PEM_FILE` is one value for every desk started from that shell, so
  set it per desk window or pass `--pem <file>` per desk.
  The supported minimum is three distinct App identities: the implementer
  (worker-desk), the reviewer (pr-review-desk) and the verifier (verify-desk)
  each need their own App key, and none of those three may be pointed at
  another's file
  ([#463](https://github.com/medici-finance/assay/issues/463)). Pointing
  intake-desk or the-desk at a shared file is a choice you make, not a default;
  pointing two of the three at one file is not supported.
- A model env-file: `KEY=value` lines for the model endpoint (for example
  `ANTHROPIC_API_KEY`, and `ANTHROPIC_BASE_URL` or `ANTHROPIC_MODEL` when you
  use a gateway). The model variables the container accepts are listed in
  [`containers/secrets.md`](../containers/secrets.md) section 3. That list
  covers model access only; what `desktoken` reads to mint a forge token is
  under "What a launched desk needs before it can mint and post" below. The
  default location is `~/.config/assay/desk.env`, and every desk started on
  defaults reads that one file, so keep it to model settings: no forge token
  and no App key or App ID setting.

`~/.config` is `$XDG_CONFIG_HOME` when you set it. To keep the files elsewhere,
pass `--pem <file>` and `--env-file <file>`, or export `ASSAY_DESK_PEM_FILE` and
`ASSAY_DESK_ENV_FILE`. A flag wins over the environment variable, which wins over
the default. `--attach` takes no credential flag and does not read these two
variables. An override you give but leave empty (for example
`--pem "$MY_PEM"` with `MY_PEM` unset, or an exported empty variable) is
refused, never skipped, so a desk cannot start on a different credential than
the one you named. Docker Desktop shares your files with the container for you. On a
Linux host the container's `desk` user (UID 999) must be able to read the PEM, so
give it a mode and group that allow that; the container reports an unreadable PEM
at start.

**Which App a launched desk acts as**

The launcher only mounts the PEM you name, read-only, at
`/run/secrets/assay/app.pem`. It does not decide which App the desk then acts as,
and this launcher gives no guarantee about that. Inside the container, the
entrypoint checks that the mounted file exists and is readable. The `desktoken`
tool looks for a key through `<ROLE>_PEM` or an `<app-name>.pem` file on its
search path, and neither the image nor the launcher sets either of those to the
mount (the next section says what to set). Which key a desk mints its tokens
from is therefore settled by the container's own configuration, not by this
launcher or by the name of the host file. Treat the per-desk host default as a
way to keep the desks' key files apart on your machine, not as an identity
guarantee. This was read from the code and not run against a real container.

**What a launched desk needs before it can mint and post**

On a fresh work volume the desk starts and gives you a shell, but its first
`desktoken` mint fails: the mount alone is not enough. Supply these three for
the role the desk mints as (for example `worker` for worker-desk); the first
two are needed to mint at all, the third before a desk tool can post.
`<ROLE>` below is that role name in capitals with `-` as `_`, for example
`WORKER`. A deployment that binds the role to a differently named App (a
`<ROLE>_APP=<app-name>` setting) uses that App's prefix instead.

| What | Where it goes | Why |
|---|---|---|
| The key pointer: `<ROLE>_PEM=/run/secrets/assay/app.pem` | An environment variable in that desk's container: a per-desk env-file passed with `--env-file` (for example `~/.config/assay/worker-desk.env`), or an `export` in the desk's shell | `desktoken` reads the key from `<ROLE>_PEM` or from `<app-name>.pem` under `~/.config/assay`. It does not read `ASSAY_APP_PEM_FILE`, the variable the entrypoint checks. |
| The App ID: `<ROLE>_APP_ID=<id>` | A line in `/work/.config/assay/apps.env`, on that desk's own work volume, or the same per-desk env-file | Without it the mint refuses before it reads the key. |
| The trust roster | `/work/.config/assay/roster.env` on that desk's own work volume, owned by the `desk` user, with neither the file nor its directory writable by group or others (`0600` for the file) | Write-class desk tools read the roster only from this file and never from the environment, so they refuse to post or open a pull request without it. `desktoken` reports it missing and carries on with the mint. |

Inside the container `HOME` is `/work`, so `~/.config/assay` is
`/work/.config/assay` on the desk's own `<desk-name>-work` volume, and
nothing there is shared with another desk. Keep these settings per desk, in
that volume or in a per-desk env-file, never in the shared default `desk.env`:
that one file reaches every desk started on defaults, so a forge token put
there acts as one identity in all five desks, the collapse the per-desk PEM
default exists to prevent. The App ID must belong to the App whose key that
desk mounts. The launcher passes one env-file, so a per-desk env-file also
carries that desk's model settings.

This is from reading `desktoken` and the image, plus a host run of `desktoken`
built from this tree with an empty config home and a test key. With only
`ASSAY_APP_PEM_FILE` set, it exited 6 for a missing App ID. With `<ROLE>_APP_ID`
added, it exited 6 because no key was found, naming `<ROLE>_PEM`. With both
set, it passed every local check and stopped at the network request for the
installation. It was not run in a real container or against GitHub. Wiring the
mount to `desktoken` inside the image would change the credential contract in
[`containers/secrets.md`](../containers/secrets.md), which is human-gated, so
this launcher leaves it to you.

**First run**

```sh
containers/desk-run.sh worker-desk
```

The desk is one of `intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`
or `the-desk`. The script runs `ghcr.io/medici-finance/assay/<desk-name>:latest`
in a container named after the desk, with the work volume `<desk-name>-work` on
`/work`. Pass `--version vX.Y.Z` (or `sha-<short>`) to run a pinned tag.

To see the exact command without running it, add `--dry-run`. The output names the
credential paths and never their contents:

```sh
containers/desk-run.sh pr-review-desk --dry-run
```

**Re-attach**

The container keeps its name and its work volume, and its filesystem, when you
detach or restart your desktop. Resume it with `--attach`, which runs
`docker start -ai`. That starts the container again: its entrypoint runs into a
NEW shell, so `/work` and the container filesystem are kept and earlier
processes are not.

```sh
containers/desk-run.sh worker-desk --attach
```

Running the first-run command again while the container still exists is refused
with this same hint. To start from scratch, remove the container with
`docker rm <desk-name>`; the work volume stays until you remove it too. An
existing container keeps the image version it was created with, so `--attach`
does not take `--version`, `--pem` or `--env-file`: its image and mounts are
fixed, so those flags are refused with `--attach` rather than ignored. To change
a mount or an image, remove the container and start it again.

**When it refuses**

The script refuses before it starts docker, and names what it expected:

| Exit | Meaning |
|------|---------|
| 2 | Unknown desk name or option (the five valid desks are listed), an invalid `--version`, or `--pem` / `--env-file` / `--version` together with `--attach`. The message gives the argument's position and length, not its text, because stderr often lands in shell logs. |
| 3 | The PEM or env-file is missing, is not a regular file, or is not readable; an override (`--pem`, `--env-file`, `ASSAY_DESK_PEM_FILE`, `ASSAY_DESK_ENV_FILE`) was given empty; or the PEM path holds a comma, a double quote or a newline. The message names the override and how to fix it. It prints the path only for a default location; a path you supplied is named by its flag or variable, never printed back, because it may be a credential pasted into the wrong slot. |
| 4 | docker is not installed or its daemon cannot be reached, or the container is in the wrong state for the request. |

There is no fallback to other credentials on your machine: it uses the files named
above or nothing. A model credential missing from inside the env-file is caught by
the container's own start-up check.

### Run the desks with docker compose

[`containers/compose.yaml`](../containers/compose.yaml) defines the same five desks as one
compose project, so a machine hosting several desks manages them as a stack. It is the
declarative form of the `docker run` recipe in
[`containers/secrets.md`](../containers/secrets.md) section 4.2, and
`docker compose run <desk-name>` gives the same interactive session, work volume and
credential surface as `containers/desk-run.sh <desk-name>`. The commands below run from the
checkout root.

Each service is named exactly by its desk (`intake-desk`, `worker-desk`, `pr-review-desk`,
`verify-desk`, `the-desk`) and runs
`ghcr.io/medici-finance/assay/<desk-name>:${DESK_VERSION:-latest}`, with `stdin_open` and
`tty` set. Shared configuration lives in one YAML anchor (`x-desk-common`).

- **Work volume.** One named volume per desk on `/work`, named `<desk-name>-work`: the volume
  `desk-run.sh` uses, so a desk started by either surface sees the same files. No volume is
  shared across desks.
- **App PEM.** A top-level `secrets:` entry per desk binds a host file to
  `/run/secrets/assay/app.pem`, read-only, owned by the container's `desk` user (UID/GID 999)
  with mode `0400`. Each desk has its own entry and its own file, never one key for all five
  (see the prerequisites above). The files default to `~/.config/assay/<desk-name>.pem`, the
  same per-desk defaults as the launcher; set `ASSAY_DESK_PEM_FILE_DIR` to read them from
  another directory.
- **Model environment.** `env_file` reads `~/.config/assay/desk.env`, the launcher's default;
  set `ASSAY_DESK_ENV_FILE` to use another file.
  [`containers/desk.env.example`](../containers/desk.env.example) lists every model variable
  from [`containers/secrets.md`](../containers/secrets.md) section 3 with placeholder values
  only. Copy it, fill in your copy, and keep it out of version control.
- **Version.** `DESK_VERSION=vX.Y.Z` (or `sha-<short>`) pins every desk to one tag.

No credential value is in `compose.yaml` or in the example env-file, and none is in an image
layer.

```sh
# One interactive desk, removed when you exit. The work volume stays.
docker compose -f containers/compose.yaml run --rm worker-desk

# A standing session: start it detached, attach, and re-attach later.
docker compose -f containers/compose.yaml up -d worker-desk
docker compose -f containers/compose.yaml attach worker-desk

# Pin a released version.
DESK_VERSION=vX.Y.Z docker compose -f containers/compose.yaml run --rm worker-desk
```

Three things differ from `desk-run.sh`. Compose does not repeat the launcher's own checks
(exit codes 2, 3 and 4 above); a missing model env-file is reported by compose itself, and
the container's start-up check still refuses a desk with no usable PEM or model credential.
`docker compose config` resolves the file without checking that each PEM file exists, so it
is not a credential check. And it prints the resolved env-file values in clear, so do not
paste its output anywhere if your env-file holds a real credential.

### How they're built and published

The approved job in `.github/workflows/release.yml` builds the base first, then
the five desk images `FROM` that just-built base (all stamped with the same
umbrella version), and runs `containers/scripts/layer-secret-scan.sh` against all
six images before any push. A hit stops image and GitHub release publication.
Build them locally with the commands in
[`containers/README.md`](../containers/README.md).

## Kubernetes compatibility profile

[`containers/k8s/`](../containers/k8s/README.md) packages the same five desk
images as separate interactive StatefulSets. Each role has its own Pod, named
container, App-PEM/model Secret references and retained `/work` PVC. Images are
pinned by digest, each checked against the releasing build's own
`release-images.json` kept in the package, agent service-account token
mounting is disabled, ingress is denied, and no Service or Ingress exposes a terminal. Storage class and deployment
values belong in an adopter overlay. The package requires no Windows host,
compose stack, CronJob, workflow controller or desktop multiplexer.

This is the **compatibility** credential profile under
[`containers/secrets.md`](../containers/secrets.md), whose /02 human acceptance
remains separate. Terminal users can read the mounted App PEM and model
credentials: terminal access is transitive key custody. Each desk's container log
is also its session transcript: the TTY output stream includes echoed input and
anything commands print, and Kubernetes captures it in the runtime log. The
[Kubernetes logging architecture](https://kubernetes.io/docs/concepts/cluster-administration/logging/)
exposes container logs through `pods/log` and node log collection. Restrict
`pods/log` alongside `pods/attach`, `pods/exec` and `pods/ephemeralcontainers` on
`assay-desks` to principals authorized for that role's key custody. Protect node
log files and their readers accordingly. Exclude the namespace from node log
shipping, or treat every downstream collector, log store, export/backup and reader
as part of the same custody boundary, with appropriate retention and access
controls. Do not send desk transcripts to a generic shared logging service whose
readers lack that custody authority. The installation operator must qualify these
paths before admitting the profile; neither NetworkPolicy nor offline rendering
proves log isolation. Legacy images do not
satisfy the managed target's isolation. Agent-execution/09 owns managed boot;
[desk-containers/28](streams/desk-containers/brief-28-package-in-cluster-cellctl-and-bind-the-interactive-desk-profile.md)
will provide the separate in-cluster `cellctl` operator and must refuse this
compatibility binding. A Running Pod is not an admitted role owner.

An installation operator first provisions ten **distinct** role credential
Secrets out of band, selects persistent storage and capacity, pre-provisions the
approved agent CLI/work inputs, and reviews an overlay. Model credentials,
endpoint and model selection come from each role's runtime model Secret. The
empty Secret skeletons in `secrets.example.yaml` are excluded from Kustomize,
are not deployable and must never be applied. Required key names are comments;
provision their values out of band and never add them to the repository. See the package README for
non-root volume permissions and file-based credential profile limits.

The base allows only selected DNS egress; it deliberately cannot reach the
Internet until the operator qualifies and adds exact forge/model endpoints.
NetworkPolicy addresses are not hostname filters. The adopter must maintain
endpoint IPs or qualify a restrictive egress gateway, check the DNS selector,
and verify its CNI enforces the policy. There is no blanket Internet exception
for installing tools. These choices and effective isolation remain operator
qualification work, not a claim made by a successful offline render.

The following is an **operator-installed workflow**, not a command the offline
checker runs. Substitute the operator's explicit kubeconfig/context and reviewed
local overlay. Inspect the complete render before the live installation step:

```sh
# Offline, from the pinned repository checkout, using the documented tool versions:
sh containers/k8s/check.sh --base --self-test
sh containers/k8s/check.sh --source-secrets --self-test
kustomize build /operator/desk-overlay > /operator/desks.rendered.yaml
kubeconform -strict -summary -kubernetes-version 1.34.1 \
  -schema-location 'containers/k8s/schemas/{{.ResourceKind}}{{.KindSuffix}}.json' \
  /operator/desks.rendered.yaml

# Live installation: the authorized installation operator performs these steps.
kubectl --kubeconfig /operator/cluster.conf --context approved-cluster \
  apply -f /operator/desks.rendered.yaml
kubectl --kubeconfig /operator/cluster.conf --context approved-cluster \
  -n assay-desks get pod worker-desk-0 -o jsonpath='{.metadata.uid}'
kubectl --kubeconfig /operator/cluster.conf --context approved-cluster \
  -n assay-desks attach pod/worker-desk-0 -c worker-desk -i -t
```

Apply uses the installer's existing approved privileges; this package creates no
operator RoleBinding. Select the exact namespace, Pod and container on each
attachment and verify the current Pod UID. For the other roles, substitute
`intake-desk-0`/`intake-desk`, `pr-review-desk-0`/`pr-review-desk`,
`verify-desk-0`/`verify-desk` or `the-desk-0`/`the-desk`. This direct compatibility
access must remain with principals already authorized for key custody.

`attach` connects to the existing shell/session. It does not create a process or
start a model request; starting an agent from that shell is a separate explicit
act. `exec` would create another process and is not a reconnect substitute.
`stdin: true`, `stdinOnce: false` and `tty: true` retain interactive input across
attachments. Disconnect the client without sending `exit`, EOF or an interrupt
to the session. A connection loss does not request a restart; sending a signal or
ending the shell can terminate it. The manifest alone does not prove every CLI's
disconnect behavior, which remains a real-profile acceptance check.

A Pod restart/reschedule starts a new process and execution identity. The PVC
retains files, not a running process or guaranteed conversation continuity.
Confirm the previous execution stopped before admitting a replacement role;
/09 and the existing supervisor own that lifecycle. StatefulSet deletion/scale-down
retains claims, but namespace/PVC deletion and storage-provider policy still
require separate backup/recovery planning. Neither offline schema validation nor
an attach connection establishes safe recovery or a qualified managed cell.


### Opt-in cellctl compatibility agent launcher

The historical Kubernetes base still consumes its recorded shell-only v1.0.52
images. The separate `containers/k8s/components/compatibility-cellctl` template
selects the new image-owned bounded launcher and installs all roles paused. Its
candidate record now pins the five published v1.0.65 role manifest references and
the unchanged release image record. All five role images are `linux/amd64`, matching
the inherited node selectors; the separate multi-platform verify-runner does not
provide ARM64 desk images. See the [candidate provenance and preparation handoff](../containers/k8s/components/compatibility-cellctl/README.md).
Actual launch/init identities, existing role-custodian groups and operator/agent
profile inputs remain unresolved. The actual offline profile checker stays failed
until those inputs are genuinely prepared; synthetic fixtures cannot satisfy it.
Deployment, credential custody, activation and final qualification remain held.
The [custodian preparation guide](kubernetes-compatibility-preparation.md) lists
the required approved inputs and uses the existing launcher to prepare and read
back each role's genuine identities before binding its profile and workloads.

The existing image credential preflight remains the entrypoint. The launcher
uses only the already-prepared pinned agent, role instructions and retained work
inputs. It sends one pinned initial role prompt and keeps attach/reconnect
separate from starting the process. Cold preparation, dirty-history holds,
complete process-namespace stop and exact API termination acknowledgement are
specified in the trusted compatibility section of `docs/cellctl.md`.

Run the real lower-layer fixtures with an explicitly selected local test daemon
and cached `python:3.13-slim`; no image pull or container network is allowed:

```sh
ASSAY_TEST_DOCKER_HOST=unix:///var/run/docker.sock \
  sh containers/k8s/compatibility/desk-session.test.sh --self-test
```

This command uses synthetic agent/credential-free fixtures, not the real model or
cluster. Missing local Docker/image capability is could-not-check. The fixture
image identity is test-environment evidence and supplies no candidate provenance.

## Local PostgreSQL for host cells

A separate compose project, `containers/postgres/`, runs one loopback-only PostgreSQL server
with one database and restricted role per cell — see [`docs/postgres.md`](postgres.md).
