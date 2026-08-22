---
brief: desk-containers/06
title: Kubernetes manifests for the five desks
wave: 3
depends: ["desk-containers/02", "desk-containers/03"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
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
---

# Brief 06 — Kubernetes manifests

## Context

files:
- `containers/k8s/` (new): `namespace.yaml`, one `<desk-name>.yaml` per desk
  (StatefulSet, replicas: 1, + its PVC via volumeClaimTemplates),
  `secret.example.yaml` (placeholder values only), `kustomization.yaml`.
- `docs/docker.md` — add the k8s section.

facts:
- One StatefulSet per desk, named exactly by desk name, image
  `ghcr.io/medici-finance/assay/<desk-name>:<pinned tag>` (kustomize `images:` is the
  version knob); `stdin: true` + `tty: true` so `kubectl attach -it` gives the
  interactive session the desks run as today.
- PVC per desk mounted at `/work` (volumeClaimTemplates; storage class left to the
  overlay).
- Credentials per `containers/secrets.md` (planned): PEM from a k8s Secret mounted read-only at
  `/run/secrets/assay/app.pem` (defaultMode 0400); model credentials via `envFrom:
  secretRef`. `secret.example.yaml` documents the shape with obviously-fake values;
  real Secrets are created out-of-band by the operator, never committed.
- Baseline pod hardening: runAsNonRoot, no privilege escalation, resources
  requests/limits set; the desks need outbound network only (GitHub + model
  endpoints) — no Service/Ingress.
- These manifests are a deployment TEMPLATE shipped in a public repo — cluster-specific
  values (storage class, real secret names, node placement) belong in a downstream
  kustomize overlay, not here.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands (no kubectl
  against any live cluster — validation is client-side only). Feature branch + draft
  PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Placeholder values only in `secret.example.yaml` — never a real credential.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the manifests per the facts: namespace, five StatefulSets + PVCs, example
   Secret, kustomization tying them together with the image-pin knob.
2. Document the k8s flow in `docs/docker.md`: create the real Secrets out-of-band,
   `kubectl apply -k`, then `kubectl attach -it <desk-name>-0` to work interactively;
   note the template/overlay boundary.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `kubectl kustomize containers/k8s/ > /dev/null` | exit 0 — the set builds |
| 2 | `kubectl kustomize containers/k8s/ \| grep -c 'kind: StatefulSet'` | 5 |
| 3 | `kubectl kustomize containers/k8s/ \| grep -c '/run/secrets/assay'` | count ≥ 5 — every desk mounts the PEM Secret |
| 4 | `kubectl kustomize containers/k8s/ \| grep -c 'volumeClaimTemplates'` | count ≥ 5 — persistent /work per desk |
| 5 | `kubectl kustomize containers/k8s/ \| grep -c 'runAsNonRoot: true'` | count ≥ 5 |
| 6 | `kubectl apply --dry-run=client -k containers/k8s/` | exit 0 — client-side validation passes (no live cluster is touched) |
| 7 | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' $(find containers/k8s -name '*.yaml'); test $? -eq 1` | exit 0 — no credential-shaped value in any committed manifest |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- Credentials arrive only from k8s Secrets at runtime per the brief-02 contract;
  **no secret in any image layer** and none in any committed manifest (row 7 — the
  example Secret is placeholder-only).
- A cluster operator can deploy from `docs/docker.md` alone: Secrets out-of-band,
  apply, attach; cluster-specific values stay in a downstream overlay.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a deployment template with placeholder
secrets, implementing the human-gated brief-02 contract; no live cluster is touched).
Reviewer confirms rows 3/5/7 and the template/overlay boundary.
