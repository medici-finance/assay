---
brief: assay:assay:desk-containers:14
title: "Flux install example — a GitRepository and Kustomization that install the desks from a pinned release, with an adopter overlay"
why: >-
  Flux is the other common GitOps engine. Its users should get the same one-step install as
  Argo CD users: a few files committed to their own GitOps repo, pinned to a release, secrets
  kept out of Git, and the unattended mode switched on. It reuses the overlay shape and the
  check script from brief 13, so the two examples cannot drift apart.
wave: 6
depends: ["desk-containers/06", "desk-containers/12", "desk-containers/13"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-09-23 by the-desk (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-09-23: examples for installing assay via Argo CD and Flux, so others can run it in their clusters"
  - "docs/streams/desk-containers/brief-13-argo-cd-install-example.md — the overlay shape and examples/gitops/check.sh this brief reuses unchanged"
  - "docs/streams/desk-containers/brief-12-unattended-tick-mode-cronjob-component.md — the component the example switches on"
  - "containers/secrets.md — the Secret names and shapes the adopter must create out of band"
  - freshness-checked 2026-09-23 @ 8ae705049 (origin/main) — no Flux manifest exists in the tree
domain: complicated
consumers:
  - "docs/docker.md (Kubernetes section gains a one-line pointer to docs/install-flux.md, next to brief 13's Argo CD pointer): follow-up desk-containers/14 (this brief; flips to fixed-here when the implementation edits the path)"
---

# Brief 14 — Flux install example

## Context
files:
- `examples/gitops/flux/overlay/kustomization.yaml` (planned) (new — the same adopter-owned overlay shape as brief 13: remote base and component at a pinned tag, namespace, image pin, schedule patch)
- `examples/gitops/flux/gitrepository.yaml` (planned) (new — a `source.toolkit.fluxcd.io` GitRepository for the ADOPTER's GitOps repo)
- `examples/gitops/flux/kustomization.yaml` (planned) (new — a `kustomize.toolkit.fluxcd.io` Kustomization that applies the overlay path from that GitRepository)
- `docs/install-flux.md` (planned) (new)
- `docs/docker.md` — one pointer line beside brief 13's
- `changelog/desk-containers-14-flux-example.md` (planned)

facts:
- Same pattern as brief 13: the adopter owns the overlay in their own repo, and it names our base and brief 12's component remotely at a release tag. Flux's GitRepository points at the adopter's repo, never at ours.
- `examples/gitops/check.sh` (planned) from brief 13 already builds every `examples/gitops/*/overlay`. Adding `examples/gitops/flux/overlay/` is enough for row 1 to cover it; do not edit the script. If the script cannot handle this overlay, that is a finding against brief 13, filed and reported, not patched here.
- Pin by release tag everywhere, never a branch: the overlay's remote `ref=`, the image tags, and the doc's instructions.
- The Flux Kustomization sets `prune: true`, `wait: true`, a `timeout`, a `targetNamespace`, and an `interval`. The doc explains that `prune: true` deletes what is removed from Git, the opposite default to brief 13's Argo example, and why.
- Secrets never go in Git. The doc tells the adopter to create the two Secrets out of band per `containers/secrets.md`, and names Flux's SOPS decryption and External Secrets as options without depending on either.
- API facts to re-establish before writing, not from memory: the current `source.toolkit.fluxcd.io` and `kustomize.toolkit.fluxcd.io` API versions and fields for the Flux release the doc names. Row 2 validates against that release's schema, and row 3 builds with the real Flux CLI.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands against any shared cluster. Client-side validation and a throwaway local `kind` cluster only.
- Stop at `implemented` — you do not set verified/done.
- Public repository: example values only (`example-org/gitops`, `example.com`); no real organisation, host or person names beyond the published image and repo paths.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the overlay, GitRepository and Kustomization per the facts.
2. Write `docs/install-flux.md` (planned): prerequisites, the Secrets to create and how, copying the overlay, pinning and bumping a release, applying the two Flux objects, checking reconciliation, reading a tick's result from a Job log, and uninstalling.
3. Add the pointer line to `docs/docker.md`, and write the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `sh examples/gitops/check.sh` | exit 0 — both the Argo CD and the Flux overlays build against this tree |
| 2 | check +dereference | `kubeconform -strict -summary -schema-location default -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' examples/gitops/flux/gitrepository.yaml examples/gitops/flux/kustomization.yaml` | exit 0 — both resources validate against Flux's published CRD schema |
| 3 | check +dereference | `flux build kustomization assay --path examples/gitops/flux/overlay --kustomization-file examples/gitops/flux/kustomization.yaml --dry-run > /dev/null` | exit 0 — the Flux CLI itself accepts the Kustomization. The remote base is fetched, so the pinned tag must exist; before the first release that ships the base, could-not-check. |
| 4 | check | `grep -c -e 'ref=main' -e 'ref=HEAD' -e 'branch: main' examples/gitops/flux/overlay/kustomization.yaml examples/gitops/flux/gitrepository.yaml; test $? -eq 1` | exit 0 — the upstream base never tracks a moving branch. The adopter's own repo MAY track a branch, and the doc says so. |
| 5 | check | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' $(find examples/gitops/flux -type f) docs/install-flux.md; test $? -eq 1` | exit 0 — no credential-shaped value |
| 6 | check +neighbour | `grep -h 'containers/k8s?ref=' examples/gitops/argocd/overlay/kustomization.yaml examples/gitops/flux/overlay/kustomization.yaml \| sort -u \| wc -l` | 1 — both examples name the identical remote base line, so the two install paths cannot drift |
| 7 | check | `statusgen --root . --lint` | exit 0 — including link checks on the new doc and pointer |
| 7a | check | `statusgen --root . --consumers --brief desk-containers/14` | exit 0 — the `consumers:` routing holds against the branch diff |
| 8 | gate:model +flow | In a throwaway `kind` cluster: `flux install` at the version the doc names, create the two Secrets with fake values, point a GitRepository at a local git server or a public test repo holding the example overlay, apply the Kustomization, wait for reconciliation | `flux get kustomizations` reports Ready=True and the namespace holds five CronJobs. Record both outputs. No docker → could-not-check. |
| 9 | gate:model +dereference | Follow `docs/install-flux.md` (planned) literally, as a reader, against the row-8 cluster | every command in the doc runs as written. A step that needs knowledge not in the doc is a failure, quoted in Evidence. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — example manifests with no credential values; nothing touches a shared cluster). The reviewer checks rows 2, 3, 6 and 8.
