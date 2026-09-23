---
brief: assay:assay:desk-containers:13
title: "Argo CD install example — an adopter overlay plus an Application and AppProject that install the desks from a pinned release"
why: >-
  Teams that already run Argo CD should be able to install assay by committing a few small
  files to their own GitOps repo, not by copying and hand-maintaining our manifests. A
  worked, validated example pinned to a release tag makes that one step, keeps secrets out
  of Git, and shows the unattended mode from brief 12 switched on.
wave: 5
depends: ["desk-containers/06", "desk-containers/12"]
unblocks: ["desk-containers/14"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-09-23 by the-desk (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-09-23: examples for installing assay via Argo CD and Flux, so others can run it in their clusters"
  - "docs/streams/desk-containers/brief-06-k8s-manifests.md — the Kustomize base at containers/k8s/ this example installs; its template/overlay boundary is the pattern this example follows"
  - "docs/streams/desk-containers/brief-12-unattended-tick-mode-cronjob-component.md — the component the example overlay switches on"
  - "containers/secrets.md — the Secret names and shapes the adopter must create out of band"
  - freshness-checked 2026-09-23 @ 8ae705049 (origin/main) — no examples/gitops/ tree and no Argo CD manifest exist
domain: complicated
consumers:
  - "docs/docker.md (Kubernetes section gains a one-line pointer to docs/install-argocd.md): follow-up desk-containers/13 (this brief; flips to fixed-here when the implementation edits the path)"
---

# Brief 13 — Argo CD install example

## Context
files:
- `examples/gitops/argocd/overlay/kustomization.yaml` (planned) (new — the adopter-owned overlay: remote base at a pinned tag, brief 12's component, namespace, image pin, schedule patch)
- `examples/gitops/argocd/application.yaml` (planned) (new — `argoproj.io/v1alpha1` Application pointing at the adopter's repo path holding that overlay)
- `examples/gitops/argocd/appproject.yaml` (planned) (new — an AppProject scoping source repos and the destination namespace)
- `examples/gitops/check.sh` (planned) (new — builds every `examples/gitops/*/overlay` with its remote base swapped for the local `containers/k8s`, so CI and verifiers test the example against this tree; brief 14 reuses it without editing it)
- `docs/install-argocd.md` (planned) (new)
- `docs/docker.md` — one pointer line in the Kubernetes section
- `changelog/desk-containers-13-argocd-example.md` (planned)

facts:
- The pattern is an overlay the ADOPTER owns in their own GitOps repo. Its `resources:` names our base remotely as `https://github.com/medici-finance/assay//containers/k8s?ref=<release tag>`, and its `components:` names brief 12's component the same way. The Application points at the adopter's repo, never at ours. Our repo only hosts the copyable example.
- Pin by release tag everywhere, never `main` or `HEAD`: the remote base `ref=`, the image tags, and the doc's instructions. Use the newest published release that contains `containers/k8s/` at implementation time. If no release contains it yet, pin the placeholder the doc tells the adopter to replace, and record row 5 as could-not-check until a release exists.
- Secrets never go in Git. The doc tells the adopter to create the PEM Secret and the model-credential Secret out of band, with the names and keys from `containers/secrets.md`. It names External Secrets, Sealed Secrets and SOPS as options without depending on any.
- The Application ships `syncPolicy.automated` with `selfHeal: true` and `prune: false`, and the doc explains when to turn prune on. It uses `CreateNamespace=true` in syncOptions.
- The AppProject allows only the adopter's repo as source and only the assay namespace as destination. Its cluster-resource allow-list contains only `Namespace`.
- API facts to re-establish before writing, not from memory: Application and AppProject are `argoproj.io/v1alpha1`. Check every field against the Argo CD release the doc names; row 2 validates against that release's schema.
- The same image, secrets and component work unchanged under Flux (brief 14). Keep anything Argo-specific out of the overlay so brief 14 can reuse the same overlay shape.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands against any shared cluster. Client-side validation and a throwaway local `kind` cluster only.
- Stop at `implemented` — you do not set verified/done.
- Public repository: example values only (`example-org/gitops`, `example.com`); no real organisation, host or person names beyond the published image and repo paths.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `examples/gitops/check.sh` (planned): POSIX sh, `set -eu`. For each `examples/gitops/*/overlay`, copy it to a temp dir, rewrite the remote `containers/k8s` references to this checkout's paths, and run `kubectl kustomize --load-restrictor LoadRestrictionsNone`. Exit non-zero on the first failure and print which overlay failed.
2. Write the overlay, Application and AppProject per the facts.
3. Write `docs/install-argocd.md` (planned): prerequisites, the Secrets to create and how, copying the overlay, pinning and bumping a release, applying the AppProject and Application, checking sync status, reading a tick's result from a Job log, and uninstalling.
4. Add the pointer line to `docs/docker.md`, and write the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `sh examples/gitops/check.sh` | exit 0 — the example overlay builds against this tree's base and component |
| 2 | check +dereference | `kubeconform -strict -summary -schema-location default -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' examples/gitops/argocd/application.yaml examples/gitops/argocd/appproject.yaml` | exit 0 — both resources validate against Argo CD's published CRD schema |
| 3 | check | `grep -c -E 'ref=v[0-9]+\.[0-9]+\.[0-9]+' examples/gitops/argocd/overlay/kustomization.yaml` | ≥ 1 — the base is pinned to a release tag |
| 4 | check | `grep -c -e 'ref=main' -e 'ref=HEAD' -e 'targetRevision: main' -e 'targetRevision: HEAD' examples/gitops/argocd/overlay/kustomization.yaml examples/gitops/argocd/application.yaml; test $? -eq 1` | exit 0 — nothing tracks a moving branch |
| 5 | check +dereference | `t=$(grep -oE 'ref=v[0-9]+\.[0-9]+\.[0-9]+' examples/gitops/argocd/overlay/kustomization.yaml \| head -1 \| cut -d= -f2); git fetch -q origin tag "$t" && git cat-file -e "$t:containers/k8s/kustomization.yaml"` | exit 0 — the pinned tag exists and contains the base (could-not-check before the first release that ships it) |
| 6 | check | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' $(find examples/gitops -type f) docs/install-argocd.md; test $? -eq 1` | exit 0 — no credential-shaped value |
| 7 | check | `statusgen --root . --lint` | exit 0 — including link checks on the new doc and pointer |
| 7a | check | `statusgen --root . --consumers --brief desk-containers/13` | exit 0 — the `consumers:` routing holds against the branch diff |
| 8 | gate:model +flow | In a throwaway `kind` cluster: install Argo CD at the version the doc names, create the two Secrets with fake values, apply the AppProject and an Application pointed at a local copy of the example overlay, and wait for sync | the Application reports `Synced` and the namespace holds five CronJobs. Record `kubectl get applications` and `kubectl get cronjobs` output. No docker → could-not-check. |
| 9 | gate:model +dereference | Follow `docs/install-argocd.md` (planned) literally, as a reader, against the row-8 cluster | every command in the doc runs as written. A step that needs knowledge not in the doc is a failure, quoted in Evidence. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — example manifests with no credential values; nothing touches a shared cluster). The reviewer checks rows 2, 5 and 8, and that no example value names a real organisation.
