---
brief: assay:assay:desk-containers:12
title: "Unattended mode for the cluster manifests — a Kustomize component that runs each desk as a tick-mode CronJob"
why: >-
  Brief 06 ships the desks as interactive StatefulSets that a person attaches to. A GitOps
  install (Argo CD, Flux) is unattended: nobody attaches, so those pods would sit idle. Brief
  08 already gave every desk a one-pass tick mode, but no manifest runs it. One opt-in
  component that turns each desk into a CronJob running a single bounded pass per schedule
  gives adopters a real "install and it works" path. Briefs 13 and 14 build on it.
wave: 4
depends: ["desk-containers/06", "desk-containers/08"]
unblocks: ["desk-containers/13", "desk-containers/14"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-09-23 by the-desk (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-09-23: examples for installing assay via Argo CD and Flux, aimed at helping adopters run it in their own clusters"
  - "docs/streams/desk-containers/brief-06-k8s-manifests.md — the Kustomize base this component patches (interactive StatefulSets, PVC per desk, secret mounts)"
  - "docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md and plugins/assay/references/tick-contract.md — the one-pass mode, `--tick` / `ASSAY_TICK=1`, `ASSAY_TICK_DEADLINE`, the exit reserve and the summary line"
  - "containers/entrypoint.sh — preflights credentials, then `exec \"$@\"`, so a CronJob can pass the tick invocation as container args with no image change"
  - "containers/secrets.md — the PEM mount path and model-credential env names this component reuses unchanged"
  - freshness-checked 2026-09-23 @ 8ae705049 (origin/main) — no `containers/k8s/` tree and no CronJob manifest exist yet
exec-tier: strong
exec-tier-why: "(a) design decisions the facts do not fully pin — schedule defaults, PVC hand-off from StatefulSet to CronJob, deadline arithmetic against the tick contract's exit reserve"
domain: complicated
layering: "Flat config, no code: a Kustomize Component over brief 06's base. It adds nothing an adopter must adopt — the base stays interactive until an overlay lists the component."
---

# Brief 12 — Unattended mode: a tick-mode CronJob component

## Context
files:
- `containers/k8s/components/unattended/kustomization.yaml` (planned) (new, `kind: Component`)
- `containers/k8s/components/unattended/*.yaml` (new: one CronJob + one PVC per desk, or a generator the implementer picks)
- `containers/k8s/overlays/unattended/kustomization.yaml` (planned) (new reference overlay: base + this component — the thing the Verify rows build)
- `docs/docker.md` — an "Unattended mode" subsection inside brief 06's Kubernetes section
- `changelog/desk-containers-12-unattended-component.md` (planned)

facts:
- Brief 06's base (`containers/k8s/`) defines five StatefulSets named by desk (`intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk`), each with a `/work` PVC from `volumeClaimTemplates`, the PEM Secret mounted read-only at `/run/secrets/assay/app.pem`, and model credentials via `envFrom: secretRef`. Read the landed base before writing — if its names differ, follow the base.
- The component must leave the base untouched and do all of its work through `components:` in an overlay. It removes the five StatefulSets (`$patch: delete`) and adds five CronJobs with the same image and the same credential wiring.
- `containers/entrypoint.sh` preflights credentials, then `exec "$@"`. The CronJob sets the container args to the harness's one-shot invocation of the desk skill with `--tick`, and sets env `ASSAY_TICK=1` and `ASSAY_TICK_DEADLINE`. Take the exact invocation from `plugins/assay/references/tick-contract.md` and the harness reference it cites. Do not invent flags.
- `ASSAY_TICK` is compared EXACTLY to `1` by the contract, so the manifest writes the string `"1"`.
- Each CronJob sets `concurrencyPolicy: Forbid`, so one tick per desk at a time and one writer on its PVC. It sets `backoffLimit: 0`: a failed tick reports through its summary line, and the next schedule is the retry. `activeDeadlineSeconds` must exceed `ASSAY_TICK_DEADLINE` plus the contract's exit reserve, so Kubernetes never kills a pass the contract would have let finish. Job history limits stay small.
- CronJob pods cannot use `volumeClaimTemplates`. The component adds one standalone PVC per desk, named so an adopter switching from the StatefulSet can see the old claim is a different object and migrate data by hand. Document that; don't automate it.
- Schedules are overlay values. Ship staggered defaults so five desks don't start in the same minute, and document how to patch them.
- No credential, real or example, is added by this brief. Brief 06's `secret.example.yaml` stays the only example Secret.
- Single-point-of-failure note: not a core-system brief. The one control is `concurrencyPolicy: Forbid` keeping a single writer per PVC. The ReadWriteOnce access mode is the second, independent layer, because a second pod cannot mount the claim.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands against any shared cluster. Client-side validation and a throwaway local `kind` cluster only.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- Public repository: example values only (`example-org`, `example.com`); no organisation, host or person names beyond the published image paths.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Read brief 06's landed base and brief 08's tick contract. Confirm the desk names, secret wiring and the exact tick invocation from the tree, not from this brief.
2. Write the component and the reference overlay per the facts.
3. Add the "Unattended mode" subsection to `docs/docker.md`: what it is, how to include the component, schedule patching, the deadline relationship, the PVC hand-off note, and how to read a tick's result from the Job's logs (the summary line).
4. Write the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `kubectl kustomize containers/k8s/overlays/unattended > /dev/null` | exit 0 |
| 2 | check | `kubectl kustomize containers/k8s/overlays/unattended \| grep -c '^kind: CronJob'` | 5 |
| 3 | check | `kubectl kustomize containers/k8s/overlays/unattended \| grep -c '^kind: StatefulSet'; test $? -eq 1` | exit 0 — no StatefulSet survives the component |
| 4 | check | `kubectl kustomize containers/k8s/overlays/unattended \| grep -c 'concurrencyPolicy: Forbid'` | 5 |
| 5 | check | `kubectl kustomize containers/k8s/overlays/unattended \| grep -c '/run/secrets/assay'` | ≥ 5 — every CronJob keeps the PEM mount |
| 6 | check +neighbour | `kubectl kustomize containers/k8s/ \| grep -c '^kind: StatefulSet'` | 5 — the base without the component is unchanged |
| 7 | check | `kubectl apply --dry-run=client -k containers/k8s/overlays/unattended` | exit 0 |
| 8 | check | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' $(find containers/k8s/components containers/k8s/overlays -name '*.yaml'); test $? -eq 1` | exit 0 — no credential-shaped value committed |
| 9 | gate:model +flow | In a throwaway `kind` cluster: create the namespace and Secrets with obviously fake values, `kubectl apply -k containers/k8s/overlays/unattended`, then `kubectl -n <ns> create job --from=cronjob/worker-desk t1` and read the pod log | the pod runs the entrypoint, reaches its credential preflight and exits non-zero with the preflight's refusal naming the bad credential. This proves the mount, env and args wiring end to end without a real key. Record the log line. No docker → could-not-check, never pass. |
| 10 | gate:model +dereference | Compare the CronJob's `activeDeadlineSeconds` and `ASSAY_TICK_DEADLINE` against the exit-reserve arithmetic in `plugins/assay/references/tick-contract.md` | deadline > tick deadline + reserve, with the numbers and the contract line quoted in Evidence |

Row 9 is the negative-path row: fake credentials must be refused by the entrypoint, not silently accepted.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — deployment templates with no credential values; nothing touches a shared cluster). The reviewer checks rows 3, 6 and 9 and that the base is unchanged when the component is not listed.
