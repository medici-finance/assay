# Runtime credential contract (normative)

Status: **normative**. Every image, launch script, compose file, and Kubernetes
manifest under `containers/` implements exactly this contract for how the bot
GitHub App signing key and the model-endpoint credentials reach a running desk.
This document is the single source of truth for the mount path and the
environment-variable names; the launch surfaces do not invent their own.

The one hard constraint this contract exists to guarantee:

> **No secret in any image layer — ever.**

The bot App PEM, any GitHub token, and any model-endpoint API key are **never**
baked into an image. They arrive only at container **runtime**, from a mounted
secret file (the PEM) and from runtime environment variables (the model
credentials). An image layer is distributable; the App PEM is App-signing
custody and the model keys are billable credentials — neither may travel inside
something a `docker pull` hands out.

This rule is backed by three independent controls, not one:

1. **This contract**, reviewed by a human (App-PEM custody is sensitive-data).
2. **`scripts/layer-secret-scan.sh`** (added here), which fails a build if
   key-shaped material appears in any layer or in the image config.
3. **The repository leak-sweep** already gating `main`.

A single control is a single point of failure; a mis-designed contract, a scan
with a pattern gap, and a sweep with a blind spot fail in different ways, so all
three stand behind the rule together. Weakening any of them — retiring a scan
pattern, relaxing a mount to read-write, permitting an image default — is a
human decision, never a model self-clear.

---

## 1. The credential inventory

| Credential | Kind | How it arrives at runtime | Image default allowed? |
|---|---|---|---|
| Bot App signing key (PEM) | Mounted **file**, read-only | Bind-mount / docker secret / k8s Secret volume at `/run/secrets/assay/app.pem` | Path **value** only (see §2). The **file**: never. |
| Model API key / auth token | Runtime **env** | `--env-file` / compose `env_file` / k8s `envFrom` a Secret | Never. |
| Model endpoint + model selection | Runtime **env** | same as above | Never (non-secret, but still runtime-supplied — see §3). |
| GitHub token (if a pre-minted one is used at all) | Runtime **env** | same as above | Never. |
| Project role secret (a `service:<name>` Verify row's credential) | Mounted **file**, read-only, **worker and verifier containers only** | Native container cell (`cellctl`) bind at `/run/secrets/role-<service>`; `ASSAY_ROLE_SECRET_<SERVICE>_FILE` names the path (§4.4) | Never: neither the file nor the path variable. |

The desks mint their own short-lived, per-role GitHub tokens **from the mounted
PEM** at runtime (via `desktoken`); the PEM is therefore the root GitHub secret,
and a long-lived `GH_TOKEN`/`GITHUB_TOKEN` is not baked and not required in the
image. If a caller injects one anyway it follows the model-env rule: runtime
env, never a layer.

---

## 2. The App PEM — mounted, read-only, path-by-env

- The PEM is mounted **read-only** at the canonical path
  **`/run/secrets/assay/app.pem`**.
- The container reads that path from the environment variable
  **`ASSAY_APP_PEM_FILE`**.
- The image **may** default `ASSAY_APP_PEM_FILE` to the mount-point *path value*
  (`ASSAY_APP_PEM_FILE=/run/secrets/assay/app.pem`) as a convenience — a path
  string is not a secret. The image **must never** contain the file that path
  points to: no `COPY`/`ADD` of a PEM, no PEM written by a `RUN` step, no PEM
  embedded in a build-arg. In a correctly built image the path is defaulted and
  the file is absent until a runtime mount supplies it.
- Read-only is required: a desk never rewrites its own signing key, and a
  writable mount is an exfiltration/rotation hazard.

Injection recipes are in §4.

---

## 3. Model credentials and endpoints — runtime env only

Model access is configured entirely through runtime environment variables. None
of these may be set as an image `ENV` default, because (a) the secret-bearing
ones are credentials and (b) even the non-secret ones (endpoint URL, model
names) baked into a public image would pin a default endpoint into a
distributable artifact. The launch surfaces pass them from an operator-held
env-file or Secret.

**The full passthrough set** (enumerated so a consumer surface needs nothing
beyond this document):

**Secret-bearing — must come from a runtime Secret / env, never a layer:**

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Primary API key for the model endpoint. |
| `ANTHROPIC_AUTH_TOKEN` | Bearer / OAuth token, the SDK's second-choice credential. |
| `ANTHROPIC_IDENTITY_TOKEN` | Workload-identity-federation JWT (inline form). |
| `ANTHROPIC_IDENTITY_TOKEN_FILE` | Path to a **mounted** WIF JWT file (the file is a mounted secret, like the PEM; only the path is an env value). |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_BEARER_TOKEN_BEDROCK` | Gateway credentials when the endpoint is reached via Amazon Bedrock. |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to a **mounted** GCP service-account key file, when reaching the endpoint via Vertex AI. |

**Non-secret configuration — still runtime-supplied, never an image default:**

| Variable | Purpose |
|---|---|
| `ANTHROPIC_BASE_URL` | Endpoint / gateway base-URL override. |
| `ANTHROPIC_MODEL` | Default model id. |
| `ANTHROPIC_DEFAULT_OPUS_MODEL`, `ANTHROPIC_DEFAULT_SONNET_MODEL`, `ANTHROPIC_DEFAULT_HAIKU_MODEL`, `ANTHROPIC_SMALL_FAST_MODEL` | Per-tier model overrides. |
| `ANTHROPIC_CUSTOM_HEADERS` | Extra headers for the endpoint. |
| `ANTHROPIC_PROFILE` | Selects a named on-disk credential profile (see §5 — a mounted profile, never a host one). |
| `ANTHROPIC_FEDERATION_RULE_ID`, `ANTHROPIC_ORGANIZATION_ID`, `ANTHROPIC_SERVICE_ACCOUNT_ID`, `ANTHROPIC_WORKSPACE_ID` | Workload-identity-federation non-secret selectors. |
| `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX` | Route the agent CLI through Bedrock / Vertex. |
| `AWS_REGION`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION` | Gateway region / project selectors. |
| `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` | Endpoint-reachability proxy configuration. |

An operator supplies whichever subset their endpoint needs (typically just
`ANTHROPIC_API_KEY`, plus `ANTHROPIC_BASE_URL`/`ANTHROPIC_MODEL` for a gateway).
The container treats the whole set as runtime input.

---

## 4. Per-surface injection recipes

All four surfaces implement §2 and §3 identically; only the syntax differs.
Project role secrets (§4.4) are bound by native container cells only.

### 4.1 `docker run` (interactive launch)

```
docker run -it --name <desk-name> \
  -v <desk-name>-work:/work \
  --mount type=bind,src=<host-path-to>/app.pem,dst=/run/secrets/assay/app.pem,ro \
  --env-file <operator-path>/desk.env \
  ghcr.io/medici-finance/assay/<desk-name>:<tag>
```

`--mount …,ro` gives the read-only PEM at the canonical path; `--env-file`
carries the §3 variables. No credential appears on the command line.

### 4.2 docker compose

```
services:
  <desk-name>:
    image: ghcr.io/medici-finance/assay/<desk-name>:<tag>
    stdin_open: true
    tty: true
    env_file:
      - <operator-path>/desk.env        # the §3 variables
    secrets:
      - source: assay_app_pem
        target: /run/secrets/assay/app.pem
        mode: 0400
    volumes:
      - <desk-name>-work:/work

secrets:
  assay_app_pem:
    file: <host-path-to>/app.pem         # supplied by the operator, never committed
```

### 4.3 Kubernetes

```
# PEM as a read-only Secret volume; model env from a Secret via envFrom.
containers:
  - name: <desk-name>
    image: ghcr.io/medici-finance/assay/<desk-name>:<tag>
    env:
      - name: GODEBUG
        value: multipathtcp=0              # CVE-2026-98432 stopgap; patched node kernels are the fix
    envFrom:
      - secretRef:
          name: assay-model-credentials   # the §3 variables
    volumeMounts:
      - name: assay-app-pem
        mountPath: /run/secrets/assay      # app.pem lands read-only inside
        readOnly: true
volumes:
  - name: assay-app-pem
    secret:
      secretName: assay-app-pem
      defaultMode: 0400
      items:
        - key: app.pem
          path: app.pem
```

Manifests ship with **placeholder** Secret names / `example` values and a
`kustomization.yaml` so a cluster overlays its real secret names — the manifests
themselves carry no credential.

### 4.4 Project role secrets (native container cells)

A project secret that a worker or verifier needs for a `service:<name>` Verify
row reaches that role's container as a **path**, never a value. The scope unit
is (project repository, role, service name). Only `worker-desk` and
`verify-desk` may hold one; the reviewer, intake and the coordinator never do.

This release builds the **container surface only**. The operator keeps each
secret in an owner-only file on the host and names it in the cell's own
`cell.env` (never `defaults.env`, never the launching shell):

```
CELL_KIND=container
CELL_CONTAINER_CONFIG=<operator-path>/containers.json
CELL_ROLE_SECRET_SURFACE=container
CELL_ROLE_SECRETS=<owner>/<repo>:worker-desk:<service>=<host-path-to>/<service>.secret
```

`cellctl desk` / `up` then plans, for that role's container only:

```
--mount type=bind,src=<host-path-to>/<service>.secret,dst=/run/secrets/role-<service>,readonly
-e ASSAY_ROLE_SECRET_<SERVICE>_FILE=/run/secrets/role-<service>
```

(`.` and `-` in the service name are spelled `_` in the variable.) The
reviewer, intake and coordinator containers get no such mount and no such
variable; a binding to any of those roles refuses every launch rather than
being dropped.

`cellctl check`, `desk` and the container's own run verify, at **every**
launch and without ever opening the file, that:

- the file exists, is a regular file (not a symlink), is mode `0600` or
  stricter, is owned by the identity `cellctl` runs as, and carries no
  access-control list; its directory is owned by that identity, writable by
  nobody else, and carries no list either;
- the file lies inside no git checkout or worktree;
- the binding's project is the container cell's repository;
- no other cell under the same cells root runs host windows (they run as the
  file's owner and could open it by its path), runs on an external launcher,
  mounts the file, binds the same file, or shares a writable volume with this
  cell's containers. An unreadable cell refuses the binding.

Any failure is a `MISS` in `check` and a refusal in `desk`. Every other
surface binds nothing and refuses: a host kind (including the one-OS-user
desktop cell), a Kubernetes compatibility cell, an external
`CELL_CONTAINER_LAUNCHER`, and `CELL_ROLE_SECRET_SURFACE=host`, whose per-role
OS identity launch is a follow-up. On those surfaces, rows that need a secret
stay in the Environment-blocked queue. A harness read-deny rule is client-side
and advisory and is never enough on its own.

Inside a holding window, `deskdispatch` refuses, before any claim, every item
it cannot show to be trusted from forge data: a review-kit dispatch, an item
with no forge item behind it, an untrusted unblessed issue or change, and a
change whose head commits are not all attributed to roster logins. A window is a
holding window when its environment has any variable whose name starts with
`ASSAY_ROLE_SECRET_` (case-insensitive, set or empty) or when any file matches
`/run/secrets/role-*`, so unsetting the variable does not lift the checks. The
head check reads commit attribution, not the pusher: the forge links an
account by the email recorded in the commit, which a pusher chooses, and it
compares commits by login only. It also strips every `ASSAY_ROLE_SECRET_*`
variable from every child process it starts.

**Revocation and audit.** Replacing or deleting the host file stops future
launches only: a removed file makes the next launch for that role refuse. It
does not invalidate a copy that has already leaked; revoking a leaked secret
means rotating it at the issuing service, then replacing the file. Each launch
appends one JSON line per bound service to `<cell>/state/role-secrets.receipts`
(directory `0700`, file `0600`): launch time, cell, role, project, service,
path and the file's modification time — never the value, and never a hash of
it (a hash of a short password is a guessing oracle).

Residual exposure the surface does not remove: any other process on the host
running as the file's owner, outside every cell, can still read the file; and
a trusted worker's own dependency can exfiltrate the secret it legitimately
holds; a writer on the base repository can forge the commit attribution the
head check reads; and an admitted agent still reads comments, review bodies and
linked content from accounts outside the roster, so prompt-injection
exfiltration through that content is not covered. Keep role secrets to non-production, per-role, per-service credentials,
and rotate on any doubt.

---

## 5. Fail-closed behaviour

A desk container that starts **without** its PEM or without a usable model
credential **reports exactly what is missing and exits non-zero**. It does not
start in a degraded or anonymous mode, and — critically — **it never falls back
to ambient host credentials.**

This last point is specific, not rhetorical. The model SDK/CLI resolves
credentials through a fallback chain: `ANTHROPIC_API_KEY`, then
`ANTHROPIC_AUTH_TOKEN`, then an on-disk OAuth profile, then workload-identity
env vars, then a default profile on disk. Inside a container that chain must
terminate at the **runtime-injected** credential and nowhere else:

- The image ships **no** baked credential profile, and `$HOME`/config does not
  resolve to any host-mounted profile directory the operator did not
  deliberately supply as a runtime secret.
- If the intended runtime credential is absent, the desk **fails** — it must not
  silently succeed by resolving to whatever profile or host credential happens
  to be reachable. "Started successfully on an unknown credential" is the exact
  failure this rule forbids.

The boot sequence therefore checks, before doing any signing or model call:

1. `ASSAY_APP_PEM_FILE` names a file that exists and is readable; else print the
   expected path (`/run/secrets/assay/app.pem`) and exit non-zero.
2. A model credential is present in the runtime environment (or an explicitly
   mounted, operator-supplied profile/WIF file is configured); else print which
   of the §3 variables are missing and exit non-zero.

Precise exit codes and message text are the launch script's to define
(`desk-run.sh`), but the contract is: **missing credential ⇒ named error ⇒
non-zero exit ⇒ no ambient fallback.**

---

## 6. No secret in any image layer — what it forbids

The rule **no secret in any image layer** is enforced mechanically by
`scripts/layer-secret-scan.sh`, which fails a build carrying key-shaped
material. Concretely, an image-producing change must not:

- `COPY` or `ADD` a PEM, token, key file, or env-file into any layer.
- Pass a credential as a `--build-arg` (build-args are echoed into layer
  history even when the final stage does not keep the file).
- Set a credential as an `ENV` **value** default (the path *value* of
  `ASSAY_APP_PEM_FILE` is allowed; a key value is not).
- Leave a credential in layer history via a `RUN` step that wrote then deleted
  it (the earlier layer still contains it).
- Write a credential in one layer and overwrite that path in a later layer,
  for example by a second `COPY` or by replacing the file with a directory.
  The earlier layer still contains the credential.

`scripts/layer-secret-scan.sh <image>` walks `docker history`, the image config
environment, and every layer's filesystem for PEM blocks, GitHub token prefixes
(`ghp_`, `ghs_`, `github_pat_`), and model API-key shapes (`sk-ant-`, `sk-`),
exiting non-zero on any hit. It extracts and scans each layer on its own, never
a merged view of the image, so whatever a later layer does to a path cannot
hide what an earlier layer wrote there. `scripts/layer-secret-scan.test.sh` is
its mutation proof: it bakes an obviously-fake key into throwaway fixture
images, including keys that a later layer overwrites or replaces, and asserts
the scan goes red, so the scan is known to fire and is not a control that only
ever passes. The scan is fail-closed: an image it cannot read in full exits 2,
never clean, and the test holds it to that with fixtures it must refuse.
`scripts/layer-secret-scan.mutate.sh` runs that test against mutated copies of
the scan, a merged-view scan and one per fail-closed step, and requires each
fixture to fail against its own mutant.
