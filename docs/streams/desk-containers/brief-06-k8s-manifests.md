---
brief: assay:assay:desk-containers:06
title: Kubernetes manifests for the five desks
wave: 3
depends: ["desk-containers/02", "desk-containers/03"]
unblocks: ["desk-containers/12", "desk-containers/13", "desk-containers/14", "desk-containers/27", "desk-containers/28"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#63 — the request (secondary aim: launched via k8s)"
  - "docs/streams/desk-containers/spec.md — secondary launch targets"
  - "containers/secrets.md (brief 02) — Secret volume + envFrom recipe"
  - "freshness-checked 2026-08-22 @ b3a2067 — no k8s manifests exist in the tree"
why: >-
  The second half of the request's secondary aim: run the same desk images on a
  cluster, with per-desk persistent volumes and credentials from k8s Secrets, so a desk
  can live on shared infrastructure instead of one person's desktop.
version: 5
id: 9ff1192a-c441-4dd3-b574-85b8cf85954e
exec-tier: strong
exec-tier-why: Per-role persistence, credential profiles and offline validation cross artifacts.
consumers:
  - "containers/k8s/: follow-up desk-containers/06"
  - "docs/docker.md: follow-up desk-containers/06"
  - "docs/streams/desktools-v2/cli-migration.json: fixed-here"
outcome: none

---

# Brief 06 — Kubernetes manifests for separately interactive desks

## Context

files: `containers/k8s/` (planned namespace, five StatefulSets/PVCs, Kustomize base, placeholder-only secret example and `check.sh`), `docs/docker.md`, `docs/streams/desktools-v2/cli-migration.json` (exact test-class routes for the source-only checker and YAML adapter), `changelog/kubernetes-desk-manifests.md` (planned).

facts: Assay main d8687cab6 inspected 20 October 2026; the five desk images exist but
containers/k8s does not. Each configured desk gets its own Pod/container and PVC at /work,
replicas 1, stdin/tty enabled, pinned image digest and explicit namespace/container name.
Storage class and deployment values belong to an adopter overlay. A Running Pod is not an
admitted role owner; the managed launch owner is graph-execution/26–27 plus agent-execution/09.

layering: Kustomize packages existing role images; offline structured render/schema checks
validate packaging. Runtime ownership and credential policy remain in their existing owners.
No controller or process supervisor is introduced by a manifest.

design-fit:
  owner: existing image/profile owners; Kustomize is packaging
  contract: containers/secrets.md compatibility contract; managed profile follows agent-execution/09
  retires: []
  weight: verbs +0, flags +0 runtime, refusals +0 runtime policy, packaging files +7 plus check fixtures
  why-add: Kubernetes needs declarative workload packaging; one shared base and opt-in profile overlays avoid separate launch engines.

## Read first

- [Current target and migration routing](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).
- [Credential contract](../../../containers/secrets.md) and [historical spec](spec.md).

## Ground rules

Branch/draft PR only; no workflow, deployment or live API contact. Render with pinned local
Kustomize and kubeconform plus cached schemas. Never run kubectl apply, including a claimed
client-only dry run, as offline validation. Stop at implemented; independent verifier owns
Evidence. No credential value in manifests, layers or examples.

## Task

1. Package namespace and one StatefulSet/PVC per configured current desk, stdin/tty true,
   pinned digest, declared resources, non-root, no privilege escalation, dropped capabilities
   and explicit service-account token automount disabled for agent workloads. Require
   default-deny ingress and no Service/Ingress exposing desk terminals. Compatibility
   egress is explicitly allowed only to its qualified forge/model endpoints and DNS;
   managed /09 owns its stricter API/metadata/CNPG/direct-forge denial. Preserve role
   names/image compatibility and independently retained work volumes. No hostPath, privileged
   container, Docker socket or hidden dependency on a desktop multiplexer.
2. Document the **compatibility** profile's runtime Secret references and model configuration
   under the existing human-gated /02 contract. Secret examples contain placeholders only and
   are excluded from Kustomize resources; a build produces no credential Secret. Explicitly
   disclose that App-PEM-bearing legacy images do not satisfy the managed target's isolation.
   /09 and /28 own managed boot/overlay changes; do not pretend a manifest removes that custody.
   Keep each independent role's App-PEM and model credential Secret references distinct;
   the checker rejects a shared credential Secret between role workloads. Compatibility
   terminal access and container session logs are transitive key custody and cannot use
   the /28 managed operator. Document equivalent custody restrictions for `pods/log`,
   node log files/shipping and downstream log readers; shared logging cannot widen access.
3. Supply offline render/schema and structured checks plus hostile manifest fixtures. Reject
   one workload hosting all roles, absent/duplicate role, missing PVC/tty, mutable image,
   privileged/host mounts, agent service-account credential or example Secret in the build.
   Checks must fail on each planted defect and make zero API/discovery requests. Extend
   the same checker with `--source-secrets` to scan every committed manifest/example,
   including unrendered Secret examples, using the existing pinned credential scanner.
   Keep a synthetic positive control that is excluded from rendering yet still rejected.
   Reject terminal Service/Ingress exposure, absent ingress deny and shared per-role
   credential Secret references. The repository-wide credential scan remains independent.
4. Update docs/docker.md with the operator-installed apply/attach workflow, explicit Pod and
   container selection, storage/Secret/profile limits and disconnect behavior. Distinguish
   attaching a retained session from creating a process. Link the future in-cluster operator
   /28; no Windows, compose, CronJob or workflow-controller prerequisite for this package.

## Verify

The script and fixtures below are implementation deliverables; checks are entirely offline.

For the consumer row, the independent verifier obtains the actual implementation delivery
from the merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or this authoring PR. Both commits must be available in the verification
checkout; absent/unknown provenance fails closed. Retain exact consumer result counts and
resolve unchecked required consumers separately; zero disproofs alone is not complete flow
coverage. These inputs are future implementation-verification evidence, not claims made here.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `sh containers/k8s/check.sh --base` | exit 0; pinned render/cached-schema validation and structured assertions prove five distinct interactive role workloads/PVCs, image pins and explicit identity; no API calls or credential Secret in rendered output |
| 2 | check +mutation | `sh containers/k8s/check.sh --base --self-test` | exit 0; checker reports every hostile fixture rejected; removing a role, tty, PVC, image digest or token isolation fails; fixture containing an example Secret fails |
| 3 | check +neighbour | `sh containers/scripts/layer-secret-scan.test.sh` | exit 0; current image credential-layer protections remain; coordinate any merged scan replacement rather than restoring a retired scanner |
| 4 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers remain a review obligation |
| 5 | check +mutation | `sh containers/k8s/check.sh --source-secrets --self-test` | exit 0; all committed manifests and unrendered examples are scanned; excluded-example synthetic credential is rejected independently of rendering |

## Definition of Done

The package and docs implement the declared compatibility scope, all offline rows are
independently witnessed and managed isolation remains explicitly unqualified until /09,/28,/29.
Human acceptance of /02 is not replaced by this model-gated packaging review.

## Evidence

<!-- Independent verifier records commands, exact versions, profile and observations. -->

Verified against merged main 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (detached verifier worktree), darwin/arm64, python 3.15.0, go 1.27.2, statusgen v1.0.54; the checker itself asserted kustomize v5.8.3, kubeconform v0.8.0 and gitleaks 8.30.1 before rendering. KUBECONFIG=/dev/null for every row; no cluster contact. Delivery: merge commit 099de97604b0075229ff96b697534ccee07f0477 (PR 3729), first parent 2fe90aa47808766df70f0b75161a0920de50d209; no later main commit touches the k8s package or docs/docker.md.
Verification-Attestation: medici-finance/assay#3844 run=987389ee03deca992750fa0122cc04d3485b9ab3fe97781e5bdd31996f0735c4 source=3cea6aa621ec0364fb7a6cbd1196141829ba6c4c brief=docs/streams/desk-containers/brief-06-k8s-manifests.md model=claude-opus-5-5 tier=strong

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `sh containers/k8s/check.sh --base` | exit 0; five distinct interactive role workloads/PVCs, pins, identity; no API calls or credential Secret | exit 0; "checked-clean: 9 schemas; five distinct interactive role workloads/PVCs" | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `sh containers/k8s/check.sh --base --self-test` | exit 0; every hostile fixture rejected (role, tty, PVC, digest, token isolation, example Secret) | exit 0; 51 "rejected:" lines incl. missing-role, duplicate-role, no-tty, no-pvc, mutable-image, agent-sa-token, missing-token-isolation, example-secret-rendered, one-workload-all-roles, terminal-service, absent-ingress-deny, shared-pem-secret; final "checked-clean: 51 hostile fixtures rejected" | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `sh containers/scripts/layer-secret-scan.test.sh` | exit 0; image credential-layer protections remain | exit 0; 53 PASS lines, 0 FAIL lines; last "PASS labels: separate file after benign match" | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` with ASSAY_DELIVERY_SHA=099de97604b0075229ff96b697534ccee07f0477, ASSAY_DELIVERY_BASE_SHA=2fe90aa47808766df70f0b75161a0920de50d209 | exit 0; exact counts, 0 disproved | exit 0; "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" (containers/k8s/, docs/docker.md, cli-migration.json all CORROBORATED); omitted-consumer review: the delivery also changed the changelog fragment, proposal 28 and brief 29, which are documentation of this item, not consumers | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `sh containers/k8s/check.sh --source-secrets --self-test` | exit 0; all manifests and unrendered examples scanned; excluded-example synthetic credential rejected | exit 0; "checked-clean: 25 package sources scanned, including unrendered examples"; "rejected: excluded-example-credential"; "rejected: excluded-example-base64" | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Rows: 5 of 5 passed; every row executed.

Risk-bearing values (gate model; risk answers all "no", not irreversible). Enumerated over the delivered package: replicas 1, persistentVolumeClaimRetentionPolicy whenDeleted/whenScaled Retain, runAsUser/runAsGroup/fsGroup 999, image tag v1.0.52 plus sha256 digest per role, secret defaultMode 0440, requests cpu 250m / memory 512Mi, limits cpu 2 / memory 4Gi, PVC storage 20Gi (each at the same line in all five role manifests: lines 7, 11-12, 30-32, 38, 72, 57-61, 83); DNS egress port 53 in the base network policy line 33; checker pins kustomize v5.8.3, kubeconform v0.8.0, gitleaks 8.30.1 (check.py lines 18-19), schema kubernetes-version 1.34.1 (check.py line 135), subprocess timeout 90 (check.py line 38). Ranked: retention policy and secret mode first (work-volume loss and credential readability are not undone by a redeploy), then image digest and UID, then the resource/storage/timeout knobs (reversible, no derivation needed).

- RISK-VALUE: DERIVED — whenDeleted = Retain (and whenScaled = Retain) @ containers/k8s/base/verify-desk.yaml:11-12 (same lines in all five) — the brief requires independently retained work volumes; Delete would destroy a desk's /work PVC when its StatefulSet is removed or scaled down, which no redeploy restores, so Retain is the only value that meets the contract.
- RISK-VALUE: DERIVED — defaultMode = 0440 @ containers/k8s/base/verify-desk.yaml:72 (same line in all five) — the PEM is read by the runAsUser 999 process with fsGroup 999; 0440 gives owner and group read only, no write or execute and no other-read, the narrowest mode that still lets the desk process read the key.
- RISK-VALUE: DERIVED — replicas = 1 @ containers/k8s/base/verify-desk.yaml:7 (same line in all five) — the brief fixes one interactive owner per role with its own RWO work volume; a second replica would be a second, unadmitted owner of the same role.
- RISK-VALUE: NAMED, NOT DERIVED — runAsUser/runAsGroup/fsGroup = 999 @ containers/k8s/base/verify-desk.yaml:30-32 (same in all five) — the base image Dockerfile creates the desk user with useradd --system and no fixed UID, so 999 depends on allocation at build time; proving it for the pinned v1.0.52 digests needs those images, which are not in the local image store and would need a registry pull. Supplemental probe only (no acceptance credit): a locally cached v1.0.46 verify-desk image run with no network reports uid=999(desk) gid=999(desk). A fixed UID in the Dockerfile would remove this dependency.
- RISK-VALUE: NAMED, NOT DERIVED — image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ containers/k8s/base/verify-desk.yaml:38 (and the four other role digests at line 38) — no record in the tree binds these digests to the v1.0.52 release build; confirming them needs a registry manifest lookup, which is outside this offline envelope.
- RISK-VALUE question filed: #3857 (desk UID/GID and role image digest provenance; the brief advances, the derivations stay open there).

**VERIFY: PASS** — 5/5 rows passed against 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c; UID and image-digest values named, not derived.

### Non-implementer verifier re-run with verifyrun witness — 2026-10-23 claude-opus-5-5

This block supersedes the earlier hand-run block above (2026-10-21, at 3cea6aa621ec) as the execution record for this brief. That block stays as written. This run is at the current merged main 4b7568de01248b514f48493efea7734ce0149823, which includes the post-delivery change #3886.

Verification-Attestation: medici-finance/assay#4188 run=3ab69b3eb3ccf4a256a3874b358a205b828c8d9aecc54805484492b7ffc0922e source=4b7568de01248b514f48493efea7734ce0149823 brief=docs/streams/desk-containers/brief-06-k8s-manifests.md model=claude-opus-5-5 tier=strong

Execution witness: installed `statusgen verifyrun --brief docs/streams/desk-containers/brief-06-k8s-manifests.md --dry-run` (statusgen v1.0.56, not built from the tree), run from the admitted detached home at 4b7568de0124 with ASSAY_DELIVERY_SHA=099de97604b0075229ff96b697534ccee07f0477 and ASSAY_DELIVERY_BASE_SHA=2fe90aa47808766df70f0b75161a0920de50d209 exported and KUBECONFIG=/dev/null. verifyrun exited 0 and printed:

```
row 1: pass (exit=0, sha256:d0087bf9e391)
row 2: pass (exit=0, sha256:5d885fb8e488)
row 3: pass (exit=0, sha256:effc577a7503)
row 4: pass (exit=0, sha256:1ea32c54c5f9)
row 5: pass (exit=0, sha256:f1dc09e9c94e)
```

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sh containers/k8s/check.sh --base` | pass exit=0 | sha256:d0087bf9e391 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sh containers/k8s/check.sh --base --self-test` | pass exit=0 | sha256:5d885fb8e488 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh containers/scripts/layer-secret-scan.test.sh` | pass exit=0 | sha256:effc577a7503 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 4 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | pass exit=0 | sha256:1ea32c54c5f9 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh containers/k8s/check.sh --source-secrets --self-test` | pass exit=0 | sha256:f1dc09e9c94e | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |

Observed output, from a by-hand run of each row right after the witness, in the same home at the same sha with the same environment. Each by-hand output log hashes to the same sha256 prefix as its witness row above, so the lines below are the output the witness hashed.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `sh containers/k8s/check.sh --base` | exit 0; five distinct interactive role workloads/PVCs, pins, identity; no API calls or credential Secret | exit 0; "checked-clean: 9 schemas; five distinct interactive role workloads/PVCs" (the checker asserted kustomize v5.8.3, kubeconform v0.8.0 and gitleaks 8.30.1 before rendering) | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de01248b514f48493efea7734ce0149823 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `sh containers/k8s/check.sh --base --self-test` | exit 0; every hostile fixture rejected (role, tty, PVC, digest, token isolation, example Secret) | exit 0; 71 "rejected:" lines, last four include image-provenance-the-desk-fabricated-pair, image-provenance-verify-desk-fabricated-pair, image-provenance-worker-desk-fabricated-pair and schema-wrong-type; final "checked-clean: 71 hostile fixtures rejected" | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de01248b514f48493efea7734ce0149823 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `sh containers/scripts/layer-secret-scan.test.sh` | exit 0; image credential-layer protections remain | exit 0; 53 PASS lines; every mutation fixture reports the expected scan exit (PEM-in-layer, key-in-ENV and overwrite fixtures exit 1, clean fixture exit 0, grep walk and blob error fixtures exit 2 and are reported CLOSED); last "PASS labels: separate file after benign match" | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de01248b514f48493efea7734ce0149823 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0; exact counts, 0 disproved | exit 0; base 2fe90aa47808..099de97604b0 (immutable delivered snapshot); CORROBORATED containers/k8s/, docs/docker.md and docs/streams/desktools-v2/cli-migration.json; "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de01248b514f48493efea7734ce0149823 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `sh containers/k8s/check.sh --source-secrets --self-test` | exit 0; all manifests and unrendered examples scanned; excluded-example synthetic credential rejected | exit 0; "checked-clean: 26 package sources scanned, including unrendered examples"; "rejected: excluded-example-credential"; "rejected: excluded-example-base64" | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de01248b514f48493efea7734ce0149823 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Rows: 5 of 5 passed in the witness and by hand; every row executed; 0 failed, 0 could-not-run. `deskpathguard rederive` reports 5 rows from base, 0 author-added, so the witness covers the full original inventory.

Runner environment: darwin/arm64 host, python 3.15.0, git 2.56.0, go 1.27.2, installed statusgen v1.0.56, KUBECONFIG=/dev/null for every row. No cluster or API contact; the checker's own offline wrapper also sets GOPROXY=off. Dispatcher admission (#4188) passed before the witness and again after the by-hand runs. The runner is not the implementer.

Delivery provenance: #3729 merged as 099de97604b0075229ff96b697534ccee07f0477, first parent 2fe90aa47808766df70f0b75161a0920de50d209, an ancestor of 4b7568de0124. Since that delivery, exactly two commits on main touch containers/k8s, docs/docker.md or containers/base/Dockerfile: c9941458dbb5 and e930c9096d87, both from #3886 (follow-up to the RISK-VALUE question #3857). This run is at a main that includes them. That is why the fixture count (71, was 51), the source count (26, was 25) and the line numbers below differ from the earlier block. Omitted-consumer review: the delivery also changed its changelog fragment, proposal 28 and brief 29. Those document this item and are not consumers.

Disclosed attempts. The witness was a `--dry-run`: verifyrun printed the table and wrote nothing into the brief, because this pass produces its Evidence outside the admitted home. While the witness ran, row 2's import of the checker's test fixtures left an ignored bytecode cache (containers/k8s/testdata/__pycache__) in the home. That made the admission re-check refuse with "unattested worktree file". The cache was removed. The by-hand runs then used PYTHONDONTWRITEBYTECODE=1, and admission re-passed on a clean home. The witness was not re-run: none of the five rows reads that cache, and the by-hand output hashes match the witness hashes. Supplemental probes (no acceptance credit): `go test ./internal/deskkit/ -run TestDeskAccountID -count=1` in tools/desk passed (TestDeskAccountIDIsPinned, TestDeskAccountIDMutants); the v1.0.52 release assets checksums.txt and release-images.json were downloaded read-only from the forge for the digest derivation below.

Risk-bearing values. The gate is model; the risk answers are present and all "no"; the item is not irreversible, so the trigger does not fire. The enumeration was done anyway over the delivered package as it stands at 4b7568de0124. Each value sits at the same line in all five role manifests, cited here for verify-desk.yaml: replicas = 1 @ containers/k8s/base/verify-desk.yaml:7; whenDeleted = Retain, whenScaled = Retain @ :11-12; runAsUser/runAsGroup/fsGroup = 999 @ :30-32; image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ :39 (the four other role digests are also at line 39); requests cpu 250m / memory 512Mi @ :58-59; limits cpu '2' / memory 4Gi @ :61-62; defaultMode = 0440 @ :73; storage = 20Gi @ :84. Also: DNS egress port 53 UDP/TCP @ containers/k8s/base/network.yaml:33-34; RECORD_SHA256 = ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64 @ containers/k8s/check.py:89; tool pins kustomize v5.8.3, kubeconform v0.8.0, gitleaks 8.30.1 @ containers/k8s/check.py:18-19; subprocess timeout = 90 @ containers/k8s/check.py:38; kubernetes-version 1.34.1 @ containers/k8s/check.py:171; desk account groupadd --gid 999 / useradd --uid 999 @ containers/base/Dockerfile:244-246. Ranked by irreversibility: retention policy and secret mode first, because work-volume loss and credential readability are not undone by a redeploy. Image digest and record pin come next, then the account ID. Resource, storage, timeout and tool pins are reversible knobs and need no derivation.

- RISK-VALUE: DERIVED — whenDeleted = Retain (and whenScaled = Retain) @ containers/k8s/base/verify-desk.yaml:11-12 (same lines in all five) — the brief requires independently retained work volumes; Delete would destroy a desk's /work PVC when its StatefulSet is removed or scaled down, which no redeploy restores, so Retain is the only value that meets the contract.
- RISK-VALUE: DERIVED — defaultMode = 0440 @ containers/k8s/base/verify-desk.yaml:73 (same line in all five) — the PEM is read by the UID 999 process with fsGroup 999; 0440 grants owner and group read only, with no write, execute or other-read, the narrowest mode that still lets the desk process read its key.
- RISK-VALUE: DERIVED — replicas = 1 @ containers/k8s/base/verify-desk.yaml:7 (same line in all five) — the brief fixes one interactive owner per role with its own RWO work volume; a second replica would be a second, unadmitted owner of the same role.
- RISK-VALUE: DERIVED — image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ containers/k8s/base/verify-desk.yaml:39, with RECORD_SHA256 = ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64 @ containers/k8s/check.py:89 — the right digest for each role is the one the v1.0.52 release build recorded for it. All five manifest image references equal the per-role refs in containers/k8s/release-images.json. That file is byte-identical to the v1.0.52 release asset, whose sha256 in the release's own checksums.txt is ff9237178e5b...afdd64, the value pinned in check.py and stated in the k8s README. The record's source_revision 2718fc978b74 is the commit tag v1.0.52 resolves to. Row 1 enforces the record binding and row 2's image-provenance fixtures prove it discriminates.
- RISK-VALUE: DERIVED — runAsUser/runAsGroup/fsGroup = 999 @ containers/k8s/base/verify-desk.yaml:30-32 (same in all five), with groupadd --gid 999 / useradd --uid 999 @ containers/base/Dockerfile:244-246 — 999 is the image's desk account. The Dockerfile now pins that account and fails the build unless desk resolves to 999:999. check.py:238 requires 999 for all three fields, and the deskuid probe above holds the Dockerfile and manifests to the same value. The pinned v1.0.52 images predate the pin: they were built with plain useradd/groupadd --system on debian bookworm-slim, which allocates system IDs from SYS_UID_MAX/SYS_GID_MAX = 999 downward. No earlier step in that Dockerfile creates a system account, so the first one gets 999. The k8s README records a 2026-10-20 local inspection of all five v1.0.52 digests that found desk at 999:999. This pass did not pull those images to repeat that inspection, which would mean leaving the offline envelope.
- Both values that the earlier block left NAMED, NOT DERIVED now have in-tree derivations through #3886. The question #3857 can be dispositioned against #3886.

**VERIFY: PASS** — 5/5 rows pass under the statusgen verifyrun witness and by hand at merged main 4b7568de01248b514f48493efea7734ce0149823; all risk values DERIVED.

Lifecycle: the row stays `implemented`. On this repo, a `verified` flip needs a Linux verifyrun execution witness, and this run was on a darwin host. Because this is an Evidence-only landing, it writes no `verified` outcome record.

### Non-implementer verifier re-run: container bracket plus host witness — 2026-10-25 claude-opus-5-5

This block supersedes the 2026-10-23 block above as the execution record for this brief. Both earlier blocks stay as written. The 2026-10-23 block's witness rows named the source in its 12-character form and its by-hand rows in the full 40-character form, so the lifecycle flip could not read a single source from its rows. This run is at merged main 0c3b558efec39c54781c270ef639fba38376ba8f. Since 4b7568de0124, no commit changes containers/k8s or docs/docker.md. Four commits (aaf2852ef, 1ccd9d907, 39e5f6687, bb482aecb) change only the desk-tools image pin and one comment in containers/base/Dockerfile, not the desk account lines.

Rows 1 and 2 ran inside the closed admission bracket. The host opened it; the rows ran in a network-none container with the default seccomp profile, the home mounted read-only and the receipt read-only, from the published verify-runner v1.0.60 image (digest sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3, which carries statusgen v1.0.60, kustomize v5.8.3, kubeconform v0.8.0, gitleaks 8.30.1, go 1.27.2, python 3.11.2 and git 2.56.0); the host closed it. The witness below is the closed file's bytes. Rows 3, 4 and 5 ran on the host with statusgen verifyrun --rows 3,4,5, because each needs something the container lacks: row 3 builds and loads fixture images with docker, row 4 reads the delivery's git history, and row 5 lists package sources with git ls-files (the linked worktree's git directory is not mounted in the container). The two selections are disjoint and together cover rows 1 to 5.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sh containers/k8s/check.sh --base` | pass exit=0 sandbox=container-netns | sha256:d0087bf9e391 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `sh containers/k8s/check.sh --base --self-test` | pass exit=0 sandbox=container-netns | sha256:5d885fb8e488 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3+dirty (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:2ffcb8b667f316d7dd935900442a725e46864bf2ad980e4ba6247712c59f77df
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4444 run=153302019be8c396741211b1c0c04e40a1e88b16fb1b33fff741455b099638c9 source=0c3b558efec39c54781c270ef639fba38376ba8f brief=docs/streams/desk-containers/brief-06-k8s-manifests.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=153302019be8c396741211b1c0c04e40a1e88b16fb1b33fff741455b099638c9 source=0c3b558efec39c54781c270ef639fba38376ba8f tree=sha256:2ffcb8b667f316d7dd935900442a725e46864bf2ad980e4ba6247712c59f77df witness=sha256:4b1db8bd4f1b38c135817eb1cf00a01d068ba82255a11b3375bb32b9b1f363a6 opened=2026-10-25T19:43:49Z closed=2026-10-25T19:44:37Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `sh containers/scripts/layer-secret-scan.test.sh` | pass exit=0 | sha256:effc577a7503 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | pass exit=0 | sha256:1ea32c54c5f9 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh containers/k8s/check.sh --source-secrets --self-test` | pass exit=0 | sha256:f1dc09e9c94e | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4444 run=153302019be8c396741211b1c0c04e40a1e88b16fb1b33fff741455b099638c9 source=0c3b558efec39c54781c270ef639fba38376ba8f brief=docs/streams/desk-containers/brief-06-k8s-manifests.md model=claude-opus-5-5 tier=strong

Row 4 ran with ASSAY_DELIVERY_SHA=099de97604b0075229ff96b697534ccee07f0477 (the merge of PR 3729, which delivered this item) and ASSAY_DELIVERY_BASE_SHA=2fe90aa47808766df70f0b75161a0920de50d209 (that merge's first parent), both ancestors of the verified source. Every row ran with KUBECONFIG=/dev/null; the checker's own wrapper also sets GOPROXY=off. There was no cluster or API contact. On the host, PYTHONDONTWRITEBYTECODE=1 kept row 2 and row 5 from leaving a bytecode cache in the home, and the home held only this Evidence edit throughout. Host: darwin/arm64, statusgen v1.0.60, git 2.56.0, python 3.15.0, go 1.27.2, with docker for row 3's fixtures. The runner is not the implementer.

Observed output. These are the key lines from a by-hand run of each row on the host right after the witnesses, at the same source with the same environment. Each by-hand log hashes to the same sha256 prefix as that row's witness cell above. They are supplemental, with no acceptance credit of their own.

- Row 1: exit 0; "checked-clean: 9 schemas; five distinct interactive role workloads/PVCs".
- Row 2: exit 0; 71 "rejected:" lines; final "checked-clean: 71 hostile fixtures rejected".
- Row 3: exit 0; 53 PASS lines, 0 FAIL lines; last "PASS labels: separate file after benign match".
- Row 4: exit 0; CORROBORATED containers/k8s/, docs/docker.md and docs/streams/desktools-v2/cli-migration.json; "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". Omitted-consumer review: the delivery also changed its changelog fragment, proposal 28 and brief 29. Those document this item and are not consumers.
- Row 5: exit 0; "checked-clean: 26 package sources scanned, including unrendered examples"; "rejected: excluded-example-credential"; "rejected: excluded-example-base64".

Rows: 5 of 5 passed; every row executed; 0 failed; 0 could-not-run. Rows 1 and 2 have a container witness inside a closed admission bracket, and rows 3 to 5 have a host witness. statusgen verifyrun --check reports 5 pass, 0 fail, 0 could-not-run/missing, and --bracket-digest reports verdict=match for the bracket.

Disclosed attempt: a first container bracket for rows 1 and 2 was opened and closed (verdict=match, same output hashes) before this block's prose was in the home, so its Runner cells named the clean source while the later host rows carried the +dirty tree marker. That witness was never landed. It was removed and the bracket re-run on the same edited tree, so all five Runner cells read alike.

Risk-bearing values. The gate is model; the risk answers are present and all "no"; the item is not irreversible, so the trigger does not fire. The enumeration was done anyway over the delivered package at 0c3b558efec3. Each value sits at the same line in all five role manifests, cited for verify-desk.yaml: replicas = 1 @ containers/k8s/base/verify-desk.yaml:7; whenDeleted = Retain, whenScaled = Retain @ :11-12; runAsUser/runAsGroup/fsGroup = 999 @ :30-32; image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ :39 (the four other role digests are also at line 39); requests cpu 250m / memory 512Mi @ :58-59; limits cpu '2' / memory 4Gi @ :61-62; defaultMode = 0440 @ :73; storage = 20Gi @ :84. Also: DNS egress port 53 UDP/TCP @ containers/k8s/base/network.yaml:33-34; RECORD_SHA256 = ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64 @ containers/k8s/check.py:91; tool pins kustomize v5.8.3, kubeconform v0.8.0, gitleaks 8.30.1 @ containers/k8s/check.py:18-19; subprocess timeout = 90 @ containers/k8s/check.py:38; kubernetes-version 1.34.1 @ containers/k8s/check.py:171; desk account groupadd --gid 999 / useradd --uid 999 @ containers/base/Dockerfile:245-246, asserted at :247. Ranked by irreversibility: retention policy and secret mode first, because work-volume loss and credential readability are not undone by a redeploy. Image digest and record pin come next, then the account ID. Resource, storage, timeout and tool pins are reversible knobs and need no derivation.

- RISK-VALUE: DERIVED — whenDeleted = Retain (and whenScaled = Retain) @ containers/k8s/base/verify-desk.yaml:11-12 (same lines in all five) — the brief requires independently retained work volumes; Delete would destroy a desk's /work PVC when its StatefulSet is removed or scaled down, which no redeploy restores, so Retain is the only value that meets the contract.
- RISK-VALUE: DERIVED — defaultMode = 0440 @ containers/k8s/base/verify-desk.yaml:73 (same line in all five) — the PEM is read by the UID 999 process with fsGroup 999; 0440 grants owner and group read only, with no write, execute or other-read, the narrowest mode that still lets the desk process read its key.
- RISK-VALUE: DERIVED — replicas = 1 @ containers/k8s/base/verify-desk.yaml:7 (same line in all five) — the brief fixes one interactive owner per role with its own RWO work volume; a second replica would be a second, unadmitted owner of the same role.
- RISK-VALUE: DERIVED — image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ containers/k8s/base/verify-desk.yaml:39, with RECORD_SHA256 = ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64 @ containers/k8s/check.py:91 — the right digest for each role is the one the v1.0.52 release build recorded for it. All five manifest image references equal the per-role refs in containers/k8s/release-images.json. This pass downloaded the v1.0.52 release assets checksums.txt and release-images.json read-only. The release's checksums.txt lists release-images.json at ff9237178e5b...afdd64, the downloaded asset is byte-identical to the in-tree record, and that value is the one check.py pins. The record's source_revision 2718fc978b74 is the commit tag v1.0.52 resolves to. Row 1 enforces the record binding, and row 2's image-provenance fixtures prove it discriminates.
- RISK-VALUE: DERIVED — runAsUser/runAsGroup/fsGroup = 999 @ containers/k8s/base/verify-desk.yaml:30-32 (same in all five), with groupadd --gid 999 / useradd --uid 999 @ containers/base/Dockerfile:245-246 — 999 is the image's desk account. The Dockerfile pins it and fails the build at :247 unless desk resolves to 999:999, and check.py:238 requires 999 for all three fields. The pinned v1.0.52 images predate that pin. They were built with plain useradd/groupadd --system on debian bookworm-slim, which allocates from SYS_UID_MAX/SYS_GID_MAX = 999 downward, and no earlier step creates a system account. The k8s README records an inspection of all five v1.0.52 digests that found desk at 999:999. This pass did not pull those images to repeat it, which would leave the offline envelope.

**VERIFY: PASS** — 5/5 rows pass at merged main 0c3b558efec39c54781c270ef639fba38376ba8f: rows 1-2 under a closed container admission bracket, rows 3-5 under the host witness; all top-ranked risk values DERIVED.
### Non-implementer verifier re-run: container bracket plus host witness — 2026-10-27 claude-opus-5-5

This block supersedes the 2026-10-25 block above as the execution record for this brief; every earlier block stays as written. This run is at merged main 47ef38f961aa1006ff296034d118630efdc177cb. Since 0c3b558efec3 no commit changes containers/k8s or containers/scripts. Three paths this item touches or depends on did change: docs/docker.md (one line, the golang builder tag moved 1.27.2 to 1.27.3), containers/base/Dockerfile (only the GO_VERSION and Go tarball digest ARGs; the desk account lines 245-247 are unchanged) and docs/streams/desktools-v2/cli-migration.json (other tools' rows). The brief's own Verify table is unchanged.

Rows 1 and 2 ran inside the closed admission bracket. The host opened it; the rows ran with `--rows 1,2` in a network-none container with the default seccomp profile, the home mounted read-only and the receipt read-only, from the published verify-runner v1.0.63 image (digest sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8, which carries statusgen v1.0.63, kustomize v5.8.3, kubeconform v0.8.0, gitleaks 8.30.1, go 1.27.2, python 3.11.2 and git 2.56.0); the host closed it. The witness below is the closed file's bytes. Rows 3, 4 and 5 ran on the host with `statusgen verifyrun --rows 3,4,5`, because each needs something the container lacks: row 3 builds and loads fixture images with docker, row 4 reads the delivery's git history, and row 5 lists package sources with git ls-files (the linked worktree's git directory is not mounted in the container). The two selections are disjoint and together cover all five rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sh containers/k8s/check.sh --base` | pass exit=0 sandbox=container-netns | sha256:d0087bf9e391 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 2 | `sh containers/k8s/check.sh --base --self-test` | pass exit=0 sandbox=container-netns | sha256:5d885fb8e488 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a284f4ec3df000d33368cb25837ba8237ac42a3d4307c251726ecf208aa795fd
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4575 run=7652926bea27aa48b2f2cf3b6fd7586610a8d57141ca2d7f9c83cff7a5632ed5 source=47ef38f961aa1006ff296034d118630efdc177cb brief=docs/streams/desk-containers/brief-06-k8s-manifests.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=7652926bea27aa48b2f2cf3b6fd7586610a8d57141ca2d7f9c83cff7a5632ed5 source=47ef38f961aa1006ff296034d118630efdc177cb tree=sha256:a284f4ec3df000d33368cb25837ba8237ac42a3d4307c251726ecf208aa795fd witness=sha256:6c520fbb53a1f182c6c2f8e4d35b3c0c0d5ac0b38940ce490723c82bc2d560d7 opened=2026-10-27T01:28:00Z closed=2026-10-27T01:30:15Z before=receipt after=check-verifier
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `sh containers/scripts/layer-secret-scan.test.sh` | pass exit=0 | sha256:effc577a7503 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 4 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/06 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | pass exit=0 | sha256:44dfb83a1691 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh containers/k8s/check.sh --source-secrets --self-test` | pass exit=0 | sha256:f1dc09e9c94e | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4575 run=7652926bea27aa48b2f2cf3b6fd7586610a8d57141ca2d7f9c83cff7a5632ed5 source=47ef38f961aa1006ff296034d118630efdc177cb brief=docs/streams/desk-containers/brief-06-k8s-manifests.md model=claude-opus-5-5 tier=strong

Row 4 ran with ASSAY_DELIVERY_SHA=099de97604b0075229ff96b697534ccee07f0477 (the merge of #3729, which delivered this item, read from the forge) and ASSAY_DELIVERY_BASE_SHA=2fe90aa47808766df70f0b75161a0920de50d209 (that merge's first parent), both ancestors of the verified source. Every row ran with KUBECONFIG=/dev/null; the checker's own wrapper also sets GOPROXY=off. There was no cluster or API contact. On the host, PYTHONDONTWRITEBYTECODE=1 kept rows 2 and 5 from leaving a bytecode cache in the home, and the home held no edit and no extra file throughout. Host: darwin/arm64, statusgen v1.0.63, git 2.56.0, python 3.15.0, go 1.27.2, with docker for row 3's fixtures. The runner is not the implementer.

Observed output. These are the key lines from a by-hand run of each row on the host right after the witnesses, at the same source with the same environment. Each by-hand log hashes to the same sha256 prefix as that row's witness cell above. They are supplemental, with no acceptance credit of their own.

- Row 1: exit 0; "checked-clean: 9 schemas; five distinct interactive role workloads/PVCs".
- Row 2: exit 0; 71 "rejected:" lines, among them missing-role, duplicate-role, no-tty, no-pvc, mutable-image, agent-sa-token, missing-token-isolation, example-secret-rendered, one-workload-all-roles, terminal-service, absent-ingress-deny, shared-pem-secret and the image-identity and image-provenance fixtures for all five roles; final "checked-clean: 71 hostile fixtures rejected".
- Row 3: exit 0; 53 PASS lines, 0 FAIL lines; last "PASS labels: separate file after benign match".
- Row 4: exit 0; base 2fe90aa47808..099de97604b0 (immutable delivered snapshot); CORROBORATED containers/k8s/, docs/docker.md and docs/streams/desktools-v2/cli-migration.json; "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". The row's output hash differs from the 2026-10-25 block because the installed statusgen now prints its configuration echo ahead of the result; the corroboration lines are the same. Omitted-consumer review: the delivery also changed its changelog fragment, proposal 28 and brief 29. Those document this item and are not consumers.
- Row 5: exit 0; "checked-clean: 26 package sources scanned, including unrendered examples"; "rejected: excluded-example-credential"; "rejected: excluded-example-base64".

Rows: 5 of 5 passed; every row executed; 0 failed; 0 could-not-run. Rows 1 and 2 have a container witness inside a closed admission bracket, and rows 3 to 5 have a host witness. No row carries a risk-bearing, live, mutating or end-to-end tag and the brief's risk answers are all "no", so no row owes a fail-first witness.

Disclosed attempts: the first two container launches for rows 1 and 2 were refused before any row ran ("the mounted root ... digests to sha256:e3b0c442..., not the sha256:a284f4ec... the host recorded at the open step"). The cause was a shell quoting error in the bind-mount argument that mounted an empty directory instead of the home. With the mount corrected, the same command under the same receipt admitted and ran; no home edit happened between the open and close steps.

Risk-bearing values. The gate is model; the risk answers are present and all "no"; the item is not irreversible, so the trigger does not fire. The enumeration was done anyway over the delivered package at 47ef38f961aa. Each value sits at the same line in all five role manifests, cited for verify-desk.yaml: replicas = 1 @ containers/k8s/base/verify-desk.yaml:7; whenDeleted = Retain, whenScaled = Retain @ :11-12; runAsUser/runAsGroup/fsGroup = 999 @ :30-32; image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ :39 (the four other role digests are also at line 39); requests cpu 250m / memory 512Mi @ :58-59; limits cpu '2' / memory 4Gi @ :61-62; defaultMode = 0440 @ :73; storage = 20Gi @ :84. Also: DNS egress port 53 UDP/TCP @ containers/k8s/base/network.yaml:33-34; RECORD_SHA256 = ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64 @ containers/k8s/check.py:91; kubernetes-version 1.34.1 @ containers/k8s/check.py:171; subprocess timeout = 90 @ containers/k8s/check.py:38. Ranked: retention policy and secret mode first (work-volume loss and credential readability are not undone by a redeploy), then image digest and desk UID, then replicas; resources, storage size, the DNS port, the schema version and the timeout are reversible operational knobs and need no derivation.

- RISK-VALUE: DERIVED — whenDeleted = Retain (and whenScaled = Retain) @ containers/k8s/base/verify-desk.yaml:11-12 (same lines in all five) — the brief requires independently retained work volumes; Delete would destroy a desk's /work PVC when its StatefulSet is removed or scaled down, which no redeploy restores, so Retain is the only value that meets the contract.
- RISK-VALUE: DERIVED — defaultMode = 0440 @ containers/k8s/base/verify-desk.yaml:73 (same line in all five) — the PEM is read by the UID 999 process with fsGroup 999; 0440 grants owner and group read only, with no write, execute or other-read, the narrowest mode that still lets the desk process read its key.
- RISK-VALUE: DERIVED — image = verify-desk:v1.0.52@sha256:906d3487ae40858ca527af023b1f7443c6fc52d3c3afa771f9c6ba3c2e15423c @ containers/k8s/base/verify-desk.yaml:39, with RECORD_SHA256 = ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64 @ containers/k8s/check.py:91 — the right digest for each role is the one the v1.0.52 release build recorded for it. This pass found all five manifest image references present in containers/k8s/release-images.json, whose sha256 is ff9237178e5b...afdd64. A read-only fetch of the v1.0.52 release's checksums.txt lists release-images.json at that same value, and tag v1.0.52 resolves to 2718fc978b7454f0f2aa422e1236fb6ad543e0d2, the record's source_revision. Row 1 enforces the record binding, and row 2's image-provenance fixtures (digest, swapped, tag and fabricated-pair for each role) show the check discriminates.
- RISK-VALUE: DERIVED — runAsUser/runAsGroup/fsGroup = 999 @ containers/k8s/base/verify-desk.yaml:30-32 (same in all five), with groupadd --gid 999 / useradd --uid 999 @ containers/base/Dockerfile:245-246 — 999 is the image's desk account. The Dockerfile pins it and fails the build at :247 unless desk resolves to 999:999, and check.py:238 requires 999 for all three fields; row 2's image-identity fixtures reject any other value for each role. The pinned v1.0.52 images predate that pin. They were built with plain useradd/groupadd --system on debian bookworm-slim, which allocates from SYS_UID_MAX/SYS_GID_MAX = 999 downward, and no earlier step creates a system account. The k8s README records an inspection of all five v1.0.52 digests that found desk at 999:999. This pass did not pull those images to repeat it, which would leave the offline envelope.
- RISK-VALUE: DERIVED — replicas = 1 @ containers/k8s/base/verify-desk.yaml:7 (same line in all five) — the brief fixes one interactive owner per role with its own RWO work volume; a second replica would be a second, unadmitted owner of the same role.

**VERIFY: PASS** — 5/5 rows pass at merged main 47ef38f961aa1006ff296034d118630efdc177cb: rows 1-2 under a closed container admission bracket, rows 3-5 under the host witness; all top-ranked risk values DERIVED.

## Review

Gate: model, unchanged risk answers: generic offline templates only, no custody redesign.
Review all structured negative controls and the honest compatibility/managed distinction.
