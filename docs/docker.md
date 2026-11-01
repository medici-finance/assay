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
