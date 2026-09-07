---
brief: desk-containers/03
title: per-desk images (named by desk) + build matrix + publish wiring
wave: 2
depends: ["desk-containers/01", "desk-containers/02"]
unblocks: ["desk-containers/04", "desk-containers/05", "desk-containers/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
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

## Review
Gate: model (all four risk answers no — thin public layers over the brief-01 base,
credential mechanics fixed by the human-gated brief-02 contract; this brief only
implements that contract). Reviewer confirms rows 3/4/6 and that the workflow scan
gate cannot be skipped on the publish path.
