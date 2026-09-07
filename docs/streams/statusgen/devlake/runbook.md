# DevLake deployment runbook — human/platform steps (STAGED)

> **Every step below is a HUMAN / PLATFORM act.** This brief (statusgen/11) stages
> the config in this directory; it does **not** run any of these steps, does not
> run mutating `kubectl`/`helm`, and does not provision credentials. Standing
> DevLake up is a cross-repo platform follow-up. Do not read this as a record of a
> running system — there is none until a human completes it.

Deployment target: the **platform k8s cluster** (operator-named). DevLake is
self-hosted (Apache-2.0). The staged inputs are [`values.yaml`](./values.yaml) and
[`github-connection.yaml`](./github-connection.yaml).

## Preconditions (human)

- Access to the platform k8s cluster (a kubecontext with install rights in a
  dedicated `devlake` namespace).
- Helm ≥ 3, and the DevLake Helm chart repo added.
- Authority to create a GitHub App (or PAT) for the orgs to be ingested.

## Step 1 — Provision GitHub auth (human)

**Human act.** Create a GitHub **App** (preferred) or a **PAT** scoped to read the
configured orgs/repos (repo metadata, PRs, PR reviews, issues, commits). Do **not**
commit the credential. Store it as the k8s secret the values file references:

```sh
# human/platform act — run against the platform k8s cluster
kubectl -n devlake create secret generic devlake-github-auth \
  --from-literal=token='<APP_INSTALL_TOKEN_OR_PAT>'
kubectl -n devlake create secret generic devlake-encryption-key \
  --from-literal=key='<32-byte-random>'
kubectl -n devlake create secret generic devlake-mysql-auth \
  --from-literal=password='<db-password>'
```

## Step 2 — Apply the deployment (human)

**Human act.** Install DevLake into its namespace on the platform k8s cluster with
the staged values. Pin the image tags in `values.yaml` first.

```sh
# human/platform act
helm repo add devlake https://apache.github.io/incubator-devlake-helm-chart
kubectl create namespace devlake       # if absent
helm install devlake devlake/devlake -n devlake -f values.yaml
```

## Step 3 — Create the connection + scope (human)

**Human act.** In the DevLake config-UI (or via its API), create the GitHub
connection and scope from [`github-connection.yaml`](./github-connection.yaml):
App/PAT auth reading the `devlake-github-auth` secret, and the configured org/repo
scope. Add the real org/repo roster here (this public spec ships only the neutral
`medici-finance/assay` example).

## Step 4 — First ingest (human)

**Human act.** Bind the connection + scope into the `statusgen-commodity-dora`
blueprint and run the **first ingest** manually. Confirm the DORA + delivery
dashboards (the `DevLake` column of [`../metric-map-11.md`](../metric-map-11.md))
render before enabling the nightly schedule.

## Step 5 — Feed the retained roadmap pages (follow-up, human/platform)

**Human/platform act, cross-repo follow-up.** Per the settled direction (Ian,
2026-08-18), DevLake **feeds INTO** the retained internal roadmap pages — it does
not replace them. Wiring the DevLake DORA output into statusgen's roadmap DORA
tiles (`roadmapdora.go` / `--roadmap`) is a later platform task, out of scope for
this staging brief.

## What this runbook does NOT do

- It does not run `helm`/`kubectl apply`, provision any secret, or run any mutating
  `kubectl` — those are human/platform acts above.
- It does not claim a running DevLake. Until a human completes steps 1–4, the
  `DevLake` column of the metric map describes the intended home, not a live system.
