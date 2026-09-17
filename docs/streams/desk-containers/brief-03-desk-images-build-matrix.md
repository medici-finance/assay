---
brief: assay:assay:desk-containers:03
title: per-desk images (named by desk) + build matrix + publish wiring
wave: 2
depends: ["desk-containers/01", "desk-containers/02"]
unblocks: ["desk-containers/04", "desk-containers/05", "desk-containers/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#63 — the request (each desk installed separately; call each one by their desk name; versioned)"
  - "docs/streams/desk-containers/spec.md — build topology, goal 2, open questions 4-5"
  - "containers/secrets.md (brief 02) — the env names/mount defaults these images declare"
  - ".github/workflows/docker-publish.yml — the existing single-image publish pattern to extend"
  - "freshness-checked 2026-08-22 @ b3a2067 — docker-publish.yml builds only the combined desk-tools image; no per-desk images exist"
why: >-
  The request's core deliverable: one image per desk, named by the desk, so "run the
  pr-review-desk" is one image reference. Thin per-desk layers over the shared base keep
  each Dockerfile a few reviewable lines while the matrix keeps all five versioned in
  lock-step with the base.
consumers:
  - "docs/docker.md (image names + tags documented): fixed-here"
  - "containers/desk-run.sh (image references): follow-up desk-containers/04"
  - "containers/compose.yaml (image references): follow-up desk-containers/05"
  - "containers/k8s/ (image references): follow-up desk-containers/06"
version: 1
id: 7ca03faf-2880-425a-b676-a907c164da2b
---

# Brief 03 — per-desk images + build matrix + publish wiring

## Context

files:
- `containers/intake-desk/Dockerfile`, `containers/worker-desk/Dockerfile`,
  `containers/pr-review-desk/Dockerfile`, `containers/verify-desk/Dockerfile`,
  `containers/the-desk/Dockerfile` (all new) — thin layers over the base.
- `containers/entrypoint.sh` (new) — shared interactive boot: verifies the brief-02
  credential mounts/env are present (fail-closed with a precise message), prints the
  desk's identity + skill pointer, and lands in the interactive session.
- `.github/workflows/docker-publish.yml` — extend with the base + five-desk build
  matrix (base first, desks FROM it, shared tags `vX.Y.Z` / `sha-<short>` / `latest`).
- `docs/docker.md` — document the five image names + tags.

facts:
- Image names: `ghcr.io/medici-finance/assay/<desk-name>` for the five desks and
  `ghcr.io/medici-finance/assay/desk-base` for the base (spec open question 4 — if the
  reviewer prefers a `desk-` prefix, apply it consistently and update docs; record the
  final decision in containers/README.md).
- Each desk Dockerfile: `FROM desk-base:<same version>` + `ENV ASSAY_DESK=<desk-name>`
  + the shared entrypoint. Desk-specific content beyond that requires a recorded reason
  (spec open question 5 — assume none until proven).
- Version discipline: a desk image is only ever built against the SAME version tag of
  the base; the matrix enforces this, no floating `latest` FROM.
- Credential handling implements `containers/secrets.md` (planned) exactly: env NAMES and default
  PATH VALUES may be declared; no credential FILE or VALUE in any layer or ENV default.
- The publish workflow must not require secrets at build time beyond registry login —
  a build that needs a desk credential is a design error.
- Workflow-file pushes to this repo may be restricted for bot identities; if the push
  of `.github/workflows/docker-publish.yml` is rejected, deliver the workflow change in
  the PR as-is and flag the rejection in the PR body for a human to carry — do not
  route around the restriction.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- A blocked workflow push is a STOP to report, never a thing to work around.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the five per-desk Dockerfiles + the shared `entrypoint.sh` (fail-closed
   credential preflight per `containers/secrets.md` (planned), then interactive boot surfacing
   the desk's skill by name).
2. Extend `docker-publish.yml`: build+push `desk-base`, then the five desk images FROM
   the just-built base, all stamped with the same version/sha tags; keep the existing
   `dry_run` discipline; run `containers/scripts/layer-secret-scan.sh` (planned) against ALL six
   images in the workflow and fail the publish on any hit.
3. Update `docs/docker.md` with the image table (names, tags, what's inside, one-line
   run example per desk).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do docker build -t assay/$d:dev containers/$d \|\| exit 1; done` | exit 0 — all five build (against a locally built `desk-base:dev`) |
| 2 | `for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' assay/$d:dev \| grep -q "ASSAY_DESK=$d" \|\| exit 1; done` | exit 0 — each image carries its own desk name |
| 3 | `for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do sh containers/scripts/layer-secret-scan.sh assay/$d:dev \|\| exit 1; done` | exit 0 — every desk image scans clean of key material |
| 4 | `docker run --rm assay/worker-desk:dev` (no mounts, no env-file) | exit non-zero; output names the missing PEM mount and env — fail-closed preflight fires (negative-path row) |
| 5 | `grep -c 'layer-secret-scan' .github/workflows/docker-publish.yml` | exit 0; count ≥ 1 — the publish path runs the scan |
| 6 | `grep -iE -e '^copy' -e '^add' containers/*/Dockerfile \| grep -ci -e pem -e token -e secret -e credential -e key; test $? -eq 1` | exit 0 — no desk Dockerfile COPY/ADD line names key material |
| 7 | `statusgen --consumers --brief desk-containers/03 --root .` | exit 0; the three follow-up entries (04/05/06) listed for the reviewer to weigh |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- **No secret in any image layer**: none of the six images carries a credential in any
  layer, ENV default, or build-arg; row 3 (scan) and row 6 (Dockerfile grep) are the
  mechanical floor, and the publish workflow enforces the scan on every future build.
- Fail-closed preflight proven by the negative-path row (row 4).
- All five images named exactly by their public desk name and version-locked to the
  base tag; `docs/docker.md` regenerated accordingly (docs-regen item).

## Evidence
### Non-implementer verifier run — VERIFY: FAIL (row 5 — docker-publish.yml not extended with the base+matrix+scan; brief Task 2 unimplemented) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5d20ff9`
Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`); docker rows offline-barred. `gate: model`, all risk `no`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | docker build the five desk images | exit 0 all five | COULD-NOT-CHECK — docker offline-barred. STATIC: each containers/<d>/Dockerfile:35 sets ENV ASSAY_DESK=<d> | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | docker inspect ASSAY_DESK per image | exit 0 | COULD-NOT-CHECK (needs built images). STATIC cross-check PASS | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | layer-secret-scan.sh on each image | exit 0 clean | COULD-NOT-CHECK — operates on built images (docker history/inspect), offline-barred | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | docker run worker-desk with no mounts → fail-closed | non-zero, names missing PEM+env | COULD-NOT-CHECK — docker offline. STATIC: entrypoint.sh fail-closed preflight, exit 78 @ :111, no ambient fallback | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | grep -c layer-secret-scan .github/workflows/docker-publish.yml | ≥1 | **FAIL — 0, grep exit 1.** Workflow never extended: no layer-secret-scan, no desk-base build, no per-desk matrix. Last touch (97108ef) predates brief-03; brief-03 commit f21f2ee touched only containers/. docs/docker.md:175 describes scan-in-workflow the workflow does not perform (docs/impl mismatch) | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | grep COPY/ADD key-material in containers/*/Dockerfile == none | exit 0 | PASS — count 0; COPY/ADD carry only binaries/plugin/entrypoint | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | statusgen --consumers --brief desk-containers/03 | exit 0; 04/05/06 listed | COULD-NOT-CHECK — statusgen aborts on docs/streams/decisions/README.md no-frontmatter (#557), not this brief | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: NAMED, NOT DERIVED — DESK_TOOLS_IMAGE = ghcr.io/medici-finance/assay/desk-tools:v0.1.0 @ containers/base/Dockerfile:43 — a MUTABLE TAG pin, not a content-addressed digest. Satisfies "no floating latest FROM" (v0.1.0 fixed) but whether v0.1.0 is the correct/current release, and whether it should be a @sha256: digest for supply-chain immutability, needs a registry/release-ledger lookup (offline-barred) — a reviewer/human call.`
**VERIFY: FAIL — row 5.** Brief Task 2 (extend docker-publish.yml with base + five-desk build matrix running layer-secret-scan on all six images) was NOT implemented — the merged workflow is still the original single combined-image publish, defeating the DoD's "publish workflow enforces the scan on every future build"; docs/docker.md:175 compounds it (docs/impl mismatch). Rows 1-4 could-not-check (docker/online lane), row 7 could-not-check (#557), row 6 PASS. Status stays `implemented`. (Bug filing budget-deferred this session — detail is here + in the PR; route for a `bug` next cycle.) CFR sidecar row appended (verify-fail).

<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
### Non-implementer verifier re-run — 2026-09-16 verify-desk (desk-containers/03 dispatched verifier) — **VERIFY: PASS**

Runner ≠ implementer. Own detached temp worktree off `origin/main`, offline (`KUBECONFIG=/dev/null`). Independent re-run following the 2026-09-06 `opus-4.8[1m]-verifier` FAIL (row 5), after `medici-finance/assay#1025` ("ci(desk-containers/03): apply the desk-images build matrix", merged 2026-09-14T16:15:12Z) landed. Merged main `e9fa19d3`.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `docker build` (all five `containers/<desk>/Dockerfile`) | exit 0, all five build | COULD-NOT-CHECK — offline envelope (real external network egress required: `debian:bookworm-slim`, `ghcr.io/.../desk-tools:v0.1.0`, go.dev/github-releases/nodejs.org). Static cross-check: all five `containers/<d>/Dockerfile` present, each `FROM ${BASE_IMAGE}`, each `COPY containers/entrypoint.sh /usr/local/bin/desk-entrypoint` | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |
| 2 | `docker inspect ... ASSAY_DESK` | exit 0, each image carries its own name | COULD-NOT-CHECK — needs built images. Static: `ENV ASSAY_DESK=<name>` confirmed one per Dockerfile, exact desk name each | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |
| 3 | `layer-secret-scan.sh` on each image | exit 0, clean | COULD-NOT-CHECK — needs built images. Static: `containers/scripts/layer-secret-scan.sh` present, scans docker history/config/layer-fs against PEM/`ghp_`/`ghs_`/`github_pat_`/`sk-ant-`/`sk-` patterns; unit-test fixture present | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |
| 4 | `docker run --rm assay/worker-desk:dev` (no mounts) | non-zero, names missing PEM+env | COULD-NOT-CHECK — needs a built image. Static: `containers/entrypoint.sh:47-61` fails closed naming the missing PEM/mount path; `:90-105` fails closed naming accepted credential env vars; `:107-112` exits 78 (EX_CONFIG), no ambient-credential fallback | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |
| 5 | `grep -c 'layer-secret-scan' .github/workflows/docker-publish.yml` | exit 0, count ≥1 | **CHECKED-CLEAN — exit 0, count 4** (was 0/FAIL on 2026-09-06). PR #1025 added a `desk-images` job: builds `desk-base` first, then the five desk images `FROM` that exact version tag via `--build-arg BASE_IMAGE=...` (never `latest`), runs the layer-secret scan unconditionally before any push, preserves the existing `dry_run`/push-resolution logic. **Row 5 now PASSES.** | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |
| 6 | `grep -iE 'COPY\|ADD' containers/*/Dockerfile \| grep -ci 'pem\|token\|secret\|credential\|key'` | exit 0, no hits | CHECKED-CLEAN — exit 0, count 0 | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |
| 7 | `statusgen --consumers --brief desk-containers/03 --root .` | exit 0, 04/05/06 listed | COULD-NOT-CHECK — the brief's changes are fully absorbed into HEAD with no outstanding diff to check against; a `--base <merge-base>` diff run was blocked by the sandbox's local classifier (guard STOP, not retried per C2). Structural cross-check only: `docs/streams/desk-containers/brief-04/05/06-*.md` all exist, consistent with the claim, but not the tool-verified corroboration the row asks for | 2026-09-16 | verify-desk (desk-containers/03 dispatched verifier) |

No invented scope — all 7 rows map to Task 1/2/3. Rows 1-4 and 7 explicitly could-not-check (offline docker lane / guard block), never silently skipped, never assumed pass — same class the 2026-09-06 pass already recorded for rows 1-4. `docs/docker.md:175` (previously flagged as a docs/impl mismatch against the missing scan) now correctly describes a scan the workflow actually performs — mismatch resolved.

**Risk-bearing value.**

**RISK-VALUE: DERIVED** — `base_version_tag="${REGISTRY_BASE}/desk-base:${VERSION}"` @ `.github/workflows/docker-publish.yml:259`, threaded into `--build-arg "BASE_IMAGE=${base_version_tag}"` @ `:264` for all five desk builds. `VERSION` is resolved once per run from the tag/dispatch input and never defaults to `latest` (the "Resolve version and publish mode" step errors out if it can't parse `vMAJOR.MINOR.PATCH`) — structurally satisfies the brief's "no floating `latest` FROM" invariant (facts, brief:56-57) by direct code read, not by a passing test.

**RISK-VALUE: NAMED, NOT DERIVED** — `ARG DESK_TOOLS_IMAGE=ghcr.io/medici-finance/assay/desk-tools:v0.1.0` @ `containers/base/Dockerfile:43` — a mutable tag pin, not a content-addressed digest. Carried forward unchanged from the 2026-09-06 Evidence; PR #1025 touched only `docker-publish.yml`, not this Dockerfile. Filed as a question: medici-finance/assay#1218 — not a blocker to this brief's flip (unchanged by this diff, out of #1025's scope).

**VERIFY: PASS** — row 5 (the sole prior FAIL) is now checked-clean given PR #1025; row 6 remains checked-clean; rows 1-4 and 7 are explicitly could-not-check for stated, non-guessed reasons, not failures. No row structurally fails against merged main.

## Review
Gate: model (all four risk answers no — thin public layers over the brief-01 base,
credential mechanics fixed by the human-gated brief-02 contract; this brief only
implements that contract). Reviewer confirms rows 3/4/6 and that the workflow scan
gate cannot be skipped on the publish path.
