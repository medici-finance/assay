# DevLake deployment — spec + runbook (STAGED, not applied)

> **Status: STAGED, not deployed.** This directory stages the configuration and
> the runbook for a self-hosted [Apache DevLake](https://devlake.apache.org/)
> (Apache-2.0) that will serve statusgen/11's **commodity** engineering metrics
> (the `DevLake` column of [`../metric-map-11.md`](../metric-map-11.md)). Nothing
> here is running. The `helm`/`kubectl apply`, the GitHub App/PAT provisioning,
> and the first ingest are **human/platform acts** — see [`runbook.md`](./runbook.md).
> This brief stages config + runbook only; it does not deploy DevLake, run any
> mutating `kubectl`, or claim a running instance.

## What DevLake serves (and what it does not)

DevLake ingests GitHub events (commits, pull requests, PR reviews, issues,
deployments) and computes the commodity DORA + delivery metrics off the shelf. In
the statusgen/11 hybrid it serves the **DevLake** rows of the metric map — the four
DORA core metrics, rework rate, commodity PR throughput/velocity, the
code-efficiency ratios DevLake can model, and the point-in-time open-PR/issue
queue counts.

It does **not** serve the **ours** rows (factory-floor WIP×dwell bottleneck,
verification-debt backlog curve, `‡`-run-coverage, issue-loop net-delta, gate-queue
age, the grouped-DORA roadmap-tile core). Those are methodology metrics derived from
brief/Evidence/historian semantics DevLake has no model of, and stay in statusgen.
Per the settled direction (Ian, 2026-08-18) DevLake **feeds INTO** the retained
internal roadmap pages — it does not replace them.

## Files in this spec

| File | What it is |
|---|---|
| [`values.yaml`](./values.yaml) | Helm values for the DevLake chart, targeting the platform k8s cluster (self-hosted, Apache-2.0). Namespace, images, persistence, ingress, and the config-UI/grafana toggles. |
| [`github-connection.yaml`](./github-connection.yaml) | The GitHub **connection + scope** config DevLake ingests through: App/PAT-based auth (provisioned by a human — see runbook) and the org/repo scope set to blueprint. |
| [`runbook.md`](./runbook.md) | The ordered **human/platform** steps: App/PAT provisioning, `helm`/`kubectl apply`, connection creation, blueprint + first ingest, and the hand-off to the roadmap-page feed. Each step is marked as a human/platform act; none is run by this brief. |

## Deployment target

The deployment target is the **platform k8s cluster** (operator-named, the way the
brief names it — no hostname is asserted in this public spec). DevLake is deployed
**self-hosted** into that cluster via its official Helm chart, in its own namespace.
The apply is staged here and performed platform-side as a cross-repo follow-up.

## Cross-links

- Up to the mapping: [`../metric-map-11.md`](../metric-map-11.md) — every metric →
  `DevLake` | `ours` | `dropped`; this spec stands up the `DevLake` column.
- Brief: [`../brief-11-devlake-hybrid-metrics-split.md`](../brief-11-devlake-hybrid-metrics-split.md).
