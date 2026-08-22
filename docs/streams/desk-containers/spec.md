# desk-containers — spec

**Requests**: medici-finance/assay#63 — "docker / k8s pod"; medici-finance/assay#64 —
"tmux and equivalents".

Package each assay desk (`intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`,
and the coordinator `the-desk`) as its own versioned container image, all built from one
shared base image, so a desk session can be fired up interactively from a desktop with a
single command — and, secondarily, launched via docker-compose or Kubernetes. On top of
the per-desk launch sits a cheap multi-desk **control layer** (tmux/tmuxinator or a
cross-platform equivalent) that fires up and attaches to all the desks together, on
macOS and win32.

## Goals

1. **Base image** (`desk-base`): one Dockerfile every desk image builds `FROM`. Carries
   the Go toolchain, Python 3, git + `gh` + CA certificates, the desk-tools binary suite
   and `statusgen`, the assay plugin (skills, commands, hooks) baked from this repo's
   `plugins/assay/`, the interactive agent CLI, a non-root `desk` user, and a declared
   persistent-volume mount point for working state.
2. **Per-desk images**, one per desk, **named by the desk**:
   `ghcr.io/medici-finance/assay/intake-desk`, `…/worker-desk`, `…/pr-review-desk`,
   `…/verify-desk`, `…/the-desk`. Each is a thin layer over the base that pins the desk
   identity (`ASSAY_DESK=<name>`) and the desk's boot entrypoint. Versioned tags
   (`vX.Y.Z`, `sha-<short>`, `latest`) matching the repo's existing image-tag scheme.
3. **Interactive desktop launch**: a shell script (`desk-run.sh <desk-name>`) that runs
   the chosen desk container interactively (`-it`) the way the desks are run today —
   with the persistent volume attached, the bot App PEM and model-endpoint credentials
   injected **at runtime**, and the container named after the desk.
4. **Secondary launch targets**: a `docker compose` file defining all five desks as
   services, and Kubernetes manifests (one workload per desk + PVC + Secret references)
   that deploy the same images unchanged.
5. **Multi-desk control layer** (#64): a terminal-multiplexer configuration that lays
   out all five desks as panes/windows in one session — each pane running the
   interactive launch path from goal 3 — so the whole desk fleet is fired up, watched,
   and re-attached as one surface. tmux + tmuxinator are the candidate baseline;
   **cross-platform is a requirement**: the chosen setup must work on macOS AND win32,
   or a named win32 alternative (Windows Terminal panes, wezterm, zellij) ships
   alongside. The control layer only orchestrates the launch script — it holds no
   credentials and adds no new privilege.

## Non-goals

- **No secret in any image layer — ever.** The bot App PEM, any GitHub token, and any
  model-endpoint API key are **never** `COPY`ed/`ADD`ed into a Dockerfile, never passed
  as build-args, and never appear in any image layer or image env default. An image
  layer is distributable; the App PEM is App-signing custody. All credentials arrive at
  **runtime** via mounted secrets (bind-mounted file / docker secret / k8s Secret) and
  runtime environment variables. This is a hard design constraint, restated as an
  explicit Definition-of-Done line in every image-producing brief.
- Not a replacement for local (`/opt/desk-tools`) installs — the containers are an
  additional way to run a desk, not the new canonical one.
- No change to desk behaviour, gates, or write budgets — the desks inside a container
  operate exactly as documented; packaging confers no new authority.
- No unattended/autonomous operation change: k8s/compose launch standing *interactive*
  sessions (tty kept open); turning desks into fully headless loops is out of scope.
- No Windows/macOS container variants; images are Linux (`linux/amd64` first).

## Components

| # | Component | Path (proposed) | Brief |
|---|-----------|-----------------|-------|
| 1 | Base image Dockerfile | `containers/base/Dockerfile` | 01 |
| 2 | Credential/secret runtime-injection contract + layer-scan check | `containers/secrets.md` (planned), `containers/scripts/layer-secret-scan.sh` (planned) | 02 |
| 3 | Per-desk Dockerfiles + build matrix + publish wiring | `containers/<desk-name>/Dockerfile` | 03 |
| 4 | Interactive launch script | `containers/desk-run.sh` (planned) | 04 |
| 5 | docker-compose definition | `containers/compose.yaml` (planned) | 05 |
| 6 | Kubernetes manifests | `containers/k8s/` | 06 |
| 7 | Multi-desk control layer (tmux/equivalent, macOS + win32) | `containers/control/` (planned) | 07 |

## Build topology

```
containers/base/Dockerfile
  ├── FROM a pinned language base (Go + Python installed; distro decision in brief 01)
  ├── COPY --from=ghcr.io/medici-finance/assay/desk-tools:<pin>  (desk binaries + statusgen,
  │     reusing the existing published image rather than re-building the Go tree)
  ├── plugins/assay/  →  baked to /opt/assay/plugin  (skills, commands, hooks)
  ├── agent CLI installed + pinned (see open questions)
  ├── non-root user `desk`; VOLUME /work  (persistent working state: clones, worktrees)
  └── NO credentials of any kind

containers/<desk>/Dockerfile   (×5: intake-desk, worker-desk, pr-review-desk,
  ├── FROM desk-base:<same version>          verify-desk, the-desk)
  ├── ENV ASSAY_DESK=<desk-name>
  └── ENTRYPOINT: interactive shell that surfaces the desk's boot instructions
      (the skill of the same name); no secrets, no baked config

CI: one build matrix — base first, then the five desk images FROM it, tagged together
(vX.Y.Z / sha-<short> / latest), published to GHCR alongside the existing desk-tools
image by the same trigger discipline (tag push / workflow_dispatch with dry-run).
```

## Runtime credential contract (summary — normative text lands with brief 02)

- The App PEM is mounted **read-only** at `/run/secrets/assay/app.pem`; the container
  reads its path from `ASSAY_APP_PEM_FILE` (default set to that mount point, the file
  itself never present in the image).
- Model-endpoint credentials and endpoints (e.g. `ANTHROPIC_API_KEY`, base-URL
  overrides) arrive as runtime environment variables via `--env-file` /
  compose `env_file` / k8s `secretRef` — never as image `ENV` defaults.
- The launch paths (script, compose, k8s) all implement this one contract; brief 02 also
  ships an automated **image layer scan** that fails the build if key-shaped material
  appears in any layer, so the "no secret in a layer" rule is enforced by a check, not
  only by review.
- Fail-closed: a desk container that starts without its PEM/credentials reports exactly
  what is missing and exits; it does not fall back to ambient host credentials.

## Persistent volume

One named volume (or k8s PVC) per desk, mounted at `/work`, holding clones, worktrees,
and session state so a desk survives container restarts. `$HOME` config that must
persist (e.g. minted-token cache) lives under the same volume via symlink or
`XDG`-style env, decided in brief 01. Volumes are per-desk — two desks never share a
writable working tree.

## Interactive launch (primary aim)

```
containers/desk-run.sh <desk-name> [--version vX.Y.Z] [--dry-run]
```

- Validates the desk name against the fixed five; refuses anything else.
- Refuses to start when the PEM file or env-file is missing (fail-closed), printing the
  expected locations.
- Runs: `docker run -it --name <desk-name> -v <desk-name>-work:/work
  --mount type=bind,src=<pem>,dst=/run/secrets/assay/app.pem,ro
  --env-file ~/.config/assay/desk.env ghcr.io/medici-finance/assay/<desk-name>:<tag>`
  (exact flags land with brief 04; `--dry-run` prints the command instead of running).
- Re-attach path for an existing container (`docker start -ai <desk-name>`).

## Multi-desk control layer (#64)

The launch script starts ONE desk; the control layer starts and holds ALL of them:

- One session, five panes/windows, each named by its desk and running
  `desk-run.sh <desk-name>` (or the compose/k8s attach equivalent) — detach and
  re-attach the whole fleet at once.
- Baseline candidate: **tmux** with a **tmuxinator** project definition (declarative
  YAML naming the five desks). Evaluated against cross-platform equivalents —
  **zellij** (layout files, no Ruby dependency), **wezterm** (Lua config, first-class
  Windows support), **Windows Terminal** (`wt` pane splitting) — because tmux does not
  run natively on win32 (WSL2 changes the answer; the evaluation must say for whom).
- Deliverable shape: a short written evaluation with ONE recommendation, plus working
  config for macOS and the win32 answer (native alternative config or a documented
  WSL2 recipe) — not five half-configs.
- The control layer never touches credentials: it only invokes the launch script,
  which owns the brief-02 injection contract.

## Secondary launch targets

- **compose**: five services named by desk, shared base config via YAML anchors,
  per-desk named volumes, `secrets:` for the PEM, `env_file` for model credentials,
  `stdin_open: true` + `tty: true` so `docker compose run <desk>` is interactive.
- **k8s**: one StatefulSet (replicas: 1) per desk + PVC, PEM as a k8s `Secret` volume,
  model credentials as a `Secret` envFrom; `kubectl attach`/`exec` is the interactive
  path. Manifests ship with placeholder secrets (`example` values only) and
  `kustomization.yaml` so a cluster overlays its real secret names.

## Open questions

1. **Agent CLI choice + pinning**: which interactive agent CLI the base image installs,
   how it is version-pinned, and how its licence/terms interact with redistribution in
   a public GHCR image. May need the CLI to be installed at first-run into `/work`
   instead of baked, if redistribution is not allowed.
2. **Base distro**: extend the existing Alpine pattern vs a Debian-slim base (glibc
   compatibility for the agent CLI and Python wheels). Decided in brief 01.
3. **arm64**: `linux/amd64` first; whether to add `linux/arm64` to the matrix (Apple
   Silicon desktops run amd64 images under emulation, slowly).
4. **Registry naming**: per-desk images as `ghcr.io/medici-finance/assay/<desk-name>`
   vs a `desk-` prefix; collision risk with future non-container artifacts.
5. **the-desk extras**: whether the coordinator image needs anything beyond the base
   (it dispatches to other sessions today); assumed "no" until proven otherwise.
6. **Token minting in-container**: `desktoken` reads the PEM from config-dir paths;
   confirm the mounted-PEM path + env override covers it without host config leaking in.
7. **Skills freshness**: skills are baked at image-build time, so a skills fix implies
   an image rebuild; is that acceptable, or should `/work` allow a repo-checkout overlay?
8. **Control-layer host span**: does the win32 story assume WSL2 + docker desktop (tmux
   works there unchanged) or native win32 (needs Windows Terminal / wezterm / zellij)?
   Brief 07's evaluation answers this with a recommendation rather than leaving both open.

## Sequencing

Seven briefs in four waves; see [README.md](README.md) for the status table, dependency
waves, and critical path. The pacing item is the **human security review of the
credential contract (brief 02)** — every image and launch surface builds against that
contract, and it is deliberately human-gated because it handles App-signing custody.
