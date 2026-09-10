---
brief: assay:assay:desk-containers:01
title: base image — toolchains, desk-tools, assay skills, persistent-volume layout
wave: 1
depends: []
unblocks: ["desk-containers/03"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#63 — the request (base dockerfile; go, python, skills, persistent volume)"
  - "docs/streams/desk-containers/spec.md — goals, build topology, open questions 1-3"
  - "Dockerfile (repo root) + docs/docker.md — the existing published desk-tools image this base reuses"
  - "freshness-checked 2026-08-22 @ b3a2067 — no containers/ dir exists; root Dockerfile is the only image on file"
why: >-
  Every desk image builds FROM this base. Without one shared, versioned base, five desk
  images drift apart (five toolchain copies, five skill snapshots) and every fix lands
  five times. One base makes the per-desk layer thin enough to be reviewable at a glance.
version: 1
id: b8bccc67-ae4b-4c59-898c-cc146b013ea3
---

# Brief 01 — base image

## Context

files:
- `containers/base/Dockerfile` (new) — the shared base image.
- `containers/README.md` (new) — one-page map of the containers/ tree (grows with later
  briefs).
- `docs/docker.md` — extend with a "desk images" section pointing at the new base.

facts:
- Reuse, don't rebuild: desk-tools binaries + statusgen come from the already-published
  image via `COPY --from=ghcr.io/medici-finance/assay/desk-tools:<pinned-tag>` — the Go
  source tree is NOT re-compiled here.
- Must additionally install: Go toolchain (pinned to the `go` line in `tools/desk/go.mod`),
  Python 3 + pip, git, gh, ca-certificates, and the interactive agent CLI (spec open
  question 1 — if its licence forbids baking into a public image, install-on-first-run
  into /work and record that decision in containers/README.md).
- Assay plugin baked from this repo: `plugins/assay/` → `/opt/assay/plugin` (skills,
  commands, hooks), so a desk session finds its skill by public desk name.
- Non-root user `desk`; `VOLUME /work` declared for persistent state (clones, worktrees,
  persistent $HOME config routed under /work — layout is this brief's design decision).
- Base distro is this brief's decision (Alpine continuity vs Debian-slim glibc for the
  agent CLI + Python wheels); record the choice + reason in containers/README.md.
- NO credentials of any kind in this image: no COPY/ADD of key material, no build-arg
  carrying a secret, no ENV defaulting to a real credential. Runtime injection is
  brief 02's contract.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Do NOT touch `.github/workflows/*` in this brief — publish wiring is brief 03.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `containers/base/Dockerfile`: pinned distro base; Go + Python 3 + git + gh +
   CA certs; `COPY --from` the pinned desk-tools image for the binary suite; bake
   `plugins/assay/` to `/opt/assay/plugin`; install/pin the agent CLI per the licence
   decision; create user `desk`; declare `VOLUME /work` and route persistent $HOME
   state under it; OCI labels matching the root Dockerfile's pattern (title, source,
   version/revision/created via build-args).
2. Write `containers/README.md` (planned): what the base carries, the distro + agent-CLI
   decisions with reasons, the /work layout, and the no-secrets rule with a pointer to
   `containers/secrets.md` (planned) — brief 02; use a forward reference if 02 has not landed.
3. Extend `docs/docker.md` with the desk-base section (image name, what's inside, local
   build command).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `docker build -t assay-desk-base:dev containers/base` | exit 0 |
| 2 | `docker run --rm assay-desk-base:dev sh -c 'go version && python3 --version && git --version && gh --version && statusgen --version && deskboard --version'` | exit 0; each tool prints a version |
| 3 | `docker run --rm assay-desk-base:dev sh -c 'ls /opt/assay/plugin/skills'` | exit 0; output contains `the-desk`, `worker-desk`, `intake-desk`, `pr-review-desk`, `verify-desk` |
| 4 | `docker inspect --format '{{json .Config.Volumes}}' assay-desk-base:dev` | output contains `/work` |
| 5 | `docker inspect --format '{{.Config.User}}' assay-desk-base:dev` | output is `desk` (non-root) |
| 6 | `grep -iE -e '^copy' -e '^add' containers/base/Dockerfile \| grep -ci -e pem -e token -e secret -e credential -e key; test $? -eq 1` | exit 0 — no COPY/ADD line names key material |
| 7 | `docker history --no-trunc assay-desk-base:dev \| grep -ci -e 'PRIVATE KEY' -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-'; test $? -eq 1` | exit 0 — no layer command carries key-shaped material (once brief 02 lands, its `layer-secret-scan.sh` supersedes this row) |

## Definition of Done
- Verify rows 1-7 green, recorded in Evidence by a non-implementer.
- **No secret in any image layer**: no credential (App PEM, token, model API key) is
  COPYed, ADDed, build-arg'd, or ENV-defaulted into the image (rows 6-7 are the
  mechanical floor; review confirms intent, not just pattern absence).
- Distro + agent-CLI licence decisions recorded with reasons in `containers/README.md` (planned).
- `docs/docker.md` updated (docs-regen: the "Container image" page gains the desk-base
  section).

## Evidence
### Non-implementer verifier run — VERIFY: HELD (row 6 PASS; rows 1-5,7 could-not-check — docker/online lane) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5d20ff9`
Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`); docker rows offline-barred. `gate: model`, all risk `no`. Impl commit `46a684f` (containers/base).

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | docker build -t assay-desk-base:dev containers/base | exit 0 | COULD-NOT-CHECK — docker offline (online lane). STATIC DEFECT: the row's build context `containers/base` fails Dockerfile:125 `COPY plugins/assay/` (resolves against context root); docs/docker.md:102 uses the correct root-context form `-f containers/base/Dockerfile … .`. Re-baseline row 1 to that form | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | docker run … tool versions | exit 0; each prints a version | COULD-NOT-CHECK — docker offline (online lane) | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | docker run … ls /opt/assay/plugin/skills has five desks | exit 0 | COULD-NOT-CHECK — docker offline. STATIC: plugins/assay/skills carries all five desks | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | docker inspect Volumes contains /work | contains /work | COULD-NOT-CHECK — needs built image. STATIC: Dockerfile:174 VOLUME /work | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | docker inspect User is desk | desk | COULD-NOT-CHECK — needs built image. STATIC: Dockerfile:185 USER desk (non-root) | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | grep COPY/ADD key-material in containers/base/Dockerfile == none | exit 0 | PASS — count 0; only COPY --from=desktools /usr/local/bin/ and plugins/assay/; no ADD | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | docker history no key-shaped material | exit 0 | COULD-NOT-CHECK — needs built image (online lane) | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: NAMED, NOT DERIVED — DESK_TOOLS_IMAGE = ghcr.io/medici-finance/assay/desk-tools:v0.1.0 @ containers/base/Dockerfile:43 — the COPY --from source, deciding which desk-tools/statusgen binaries the base carries. The repo's binary suite is pinned v0.26.0 (plugins/assay/paired-versions.yaml:57), so a v0.1.0 image pin is a suspected stale/placeholder tag; a registry probe to confirm the valid current image tag is offline-barred. Route to human. Companion GO_VERSION=1.25.0 @ :57 is DERIVED (matches tools/desk/go.mod:3 + statusgen/go.mod:3); AGENT_CLI_VERSION:=latest @ :145 is a floating tag worth an implementer note.`
**VERIFY: HELD** — row 6 (Dockerfile grep) PASS; rows 1-5,7 are could-not-check (docker build/run/inspect/history — offline/online lane, re-run by a docker-capable verifier). Two carry-to-human items: row-1 build-context command defect (re-baseline to the root-context form docs already use), and the DESK_TOOLS_IMAGE v0.1.0-vs-v0.26.0 pin. Status stays `implemented`.

<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
- 2026-08-22 (board-row worker, post-46a684f): implementation was already on main via
  commit `46a684f` (containers/base/Dockerfile, containers/README.md, docs/docker.md §Desk
  images) with the board row left `todo` — phantom-row class. This PR records the row flip
  `todo → implemented`. Verify rows 1–7 were not re-run in this pass; the review lane should
  confirm them against the built image per the DoD.

## Review
Gate: model (all four risk answers no — the image bakes only public repo content and
public toolchains; credential handling is explicitly excluded and human-gated in
brief 02). Reviewer confirms rows 6-7 and that no build-arg or ENV smuggles a
credential past the grep patterns.
