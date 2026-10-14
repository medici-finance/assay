---
brief: assay:assay:desk-containers:17
title: "PostgreSQL on Kubernetes with CloudNativePG — a Kustomize component with one Cluster, a database and role per cell, out-of-band Secrets, object-store backups, and the explicit local-to-CNPG migration procedure"
why: >-
  The non-CI target runs each cell's operational state in PostgreSQL in local Docker first,
  then on CloudNativePG. Brief 16 ships the local profile. This brief ships the cluster profile:
  an opt-in Kustomize component beside brief 06's base, with a CNPG `Cluster`, one `Database`
  and one `DatabaseRole` per cell, Secrets created out of band, scheduled backups with WAL
  archiving to an S3-compatible object store, and the explicit migration from the local server
  that the target names: pin compatible versions, quiesce admission, migrate at a declared
  recovery point, validate, switch endpoints, reconcile, and only then reopen admission.
wave: 4
depends: ["desk-containers/06", "desk-containers/16"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by the-desk (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-13: CloudNativePG on Kubernetes — a CNPG Cluster manifest beside brief 06's Kustomize manifests, per-cell databases and roles, Secrets out of band, backups to an example S3-compatible endpoint, and the explicit local-to-CNPG migration procedure"
  - "docs/proposals/non-ci-architecture/14-storage-forge-graph.md lines 11, 13, 15, 44 and 56 — HA needs failure domains and replication does not replace backups; per-cell database and restricted role; the migration steps and the network-volume prohibition; no zero-loss claim; restore must leave admission paused"
  - "docs/proposals/non-ci-architecture/15-regression-from-day-one.md line 62 — local Docker state moves to CNPG with a compatible schema/extension set, a validated recovery point, a paused handover and no admission before reconciliation"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md lines 200 and 204 — local Docker then CloudNativePG; per-cell databases and roles; the administrator stays trusted"
  - "docs/streams/desk-containers/brief-06-k8s-manifests.md (the `containers/k8s/` base and its overlay boundary), brief-12-unattended-tick-mode-cronjob-component.md (the `components/` + `overlays/` shape), brief-13-argo-cd-install-example.md (kubeconform against the CRDs catalog; the throwaway `kind` flow row)"
  - "docs/streams/desk-containers/brief-16-local-docker-postgres.md — the source server, the name mapping, `backup.sh`, `check-isolation.sh`, `provision-cell.sh --disable-login`, and the `CELL_DB_DSN` / `CELL_DB_PASSFILE` keys this brief's migration switches"
  - "CloudNativePG v1.30.1 (2026-09-23) release manifest `cnpg-1.30.1.yaml` and docs/src/release_notes/v1.30.md, fetched 2026-10-13: default operand `18.6-system-trixie`; PostgreSQL 14–18 supported; Kubernetes 1.34–1.36; new standalone `DatabaseRole` CRD (passwordSecret, login, createdb, createrole, superuser, replication, bypassrls, connectionLimit, inRoles, ensure, databaseRoleReclaimPolicy retain|delete, immutable cluster reference); `Database` CRD (cluster, name, owner, ensure, databaseReclaimPolicy, schemas, extensions); in-tree `.spec.backup.barmanObjectStore` deprecated and to be removed in 1.31.0; `bootstrap.initdb.import` microservice and monolith types"
  - "Barman Cloud Plugin v0.15.1 (2026-09-30) release manifest, fetched 2026-10-13: `ObjectStore` (barmancloud.cnpg.io/v1) with endpointURL, s3Credentials and retentionPolicy; needs CNPG ≥ 1.26 and cert-manager; wired with `spec.plugins` (`barman-cloud.cloudnative-pg.io`, `isWALArchiver: true`, `barmanObjectName`) and `method: plugin` backups"
  - freshness-checked 2026-10-14 @ bcc02252e (origin/main) — no `containers/k8s/` tree (brief 06 todo), no CNPG or PostgreSQL manifest anywhere, no `docs/postgres.md` (planned)
domain: complicated
outcome: none
consumers:
  - "docs/postgres.md (planned) (gains the Kubernetes profile, backup and recovery, and the local-to-CNPG migration runbook): follow-up desk-containers/17 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/docker.md (Kubernetes section gains a one-line pointer to the PostgreSQL component): follow-up desk-containers/17 (this brief; flips to fixed-here when the implementation edits the path)"
---

# Brief 17 — PostgreSQL on Kubernetes with CloudNativePG

## Context
files:
- `containers/k8s/components/postgres/kustomization.yaml` (planned) (new — `kind: Component`; namespaced resources only)
- `containers/k8s/components/postgres/cluster.yaml` (planned) (new — the CNPG `Cluster`)
- `containers/k8s/components/postgres/objectstore.yaml` (planned) (new — the Barman Cloud Plugin `ObjectStore`)
- `containers/k8s/components/postgres/scheduledbackup.yaml` (planned) (new)
- `containers/k8s/components/postgres/cell-example-cell.yaml` (planned) (new — one `Database` and one `DatabaseRole`: the per-cell pattern an overlay copies)
- `containers/k8s/components/postgres/secret.example.yaml` (planned) (new — placeholder shapes only; not listed in the component's `resources:`)
- `containers/k8s/overlays/postgres/kustomization.yaml` (planned) (new — the reference overlay: brief 06's base plus this component; what the Verify rows build)
- `containers/k8s/overlays/postgres-kind/kustomization.yaml` (planned) (new — patches the reference overlay to one instance and the throwaway object store, for the kind drill only)
- `containers/postgres/migrate-cell.sh` (planned) (new — one cell, local server to CNPG: preflight, final dump, restore, validate)
- `containers/postgres/kind-drill.sh` (planned) (new — the throwaway kind flow)
- `docs/postgres.md` (planned) (brief 16 creates it; this brief adds the Kubernetes sections)
- `docs/docker.md` — one pointer line in brief 06's Kubernetes section
- `changelog/desk-containers-17-cnpg-postgres.md` (planned)

facts:
- **Pinned versions, re-established at implementation.** Pin CloudNativePG 1.30.1, the Barman Cloud Plugin v0.15.1 and the operand image `ghcr.io/cloudnative-pg/postgresql:18.6-system-trixie`, pinned by digest in `spec.imageName`. 18.6 is the same PostgreSQL version as brief 16's local image, so the migration is same-major and same-minor. Before writing, check each pin and every field below against that release's CRDs and docs, not against this brief or memory. If a newer patch release is out, use it and record it in the PR body. The plugin requires cert-manager. Installing the operator, the plugin and cert-manager is cluster-scoped, so the component does not do it. `docs/postgres.md` (planned) names the versions and lists those installs as prerequisites, each to be done by the adopter's own GitOps or by hand.
- **Shape: an opt-in component, as in brief 12.** Brief 06's base stays untouched. `containers/k8s/components/postgres/` is a `kind: Component` holding namespaced resources only. The reference overlay `containers/k8s/overlays/postgres/` lists the base plus the component. Storage class, storage size, instance count, anti-affinity, bucket, endpoint and the list of cells are overlay values, by brief 06's template/overlay boundary. Nothing Argo-specific or Flux-specific goes in the component, so briefs 13 and 14's adopter overlays can add it with one `components:` line.
- **Cluster.** Name `assay-pg`. `instances: 3` by default. The doc states that three pods on one node or one zone do not survive losing it (doc 14 line 11): for real HA the overlay sets required pod anti-affinity on a topology key with real failure domains. `enableSuperuserAccess: false`; no application ever uses the superuser. Storage comes from a PVC template whose class is an overlay value. `spec.plugins` names `barman-cloud.cloudnative-pg.io` with `isWALArchiver: true` and `barmanObjectName: assay-pg-store`. TLS is CNPG's default server TLS. A client sets `sslmode=verify-full` and `sslrootcert` to the CA from the cluster's CA Secret; the doc shows how to extract it. Exposing the `-rw` Service to host cells outside the cluster (port-forward, a LoadBalancer through `managed.services`, or running the cell in-cluster) is an operator choice. The doc lists the options and ships none as a default.
- **Per-cell database and role: standalone `Database` and `DatabaseRole` objects.** One pair per cell, named from brief 16's mapping (`example-cell` maps to `cell_example_cell`). The `DatabaseRole` sets `login: true`, `superuser: false`, `createdb: false`, `createrole: false`, `replication: false`, `bypassrls: false`, a `connectionLimit`, `passwordSecret` naming a `kubernetes.io/basic-auth` Secret, and `databaseRoleReclaimPolicy: retain`. The `Database` sets `owner` to that role, `databaseReclaimPolicy: retain`, and any `extensions:` the consumer needs. Removing a cell from Git therefore leaves its role and data in place for the operator to retire by hand.
  - Alternatives set aside: `Cluster.spec.managed.roles` puts every cell's role in the one shared `Cluster` object, so each cell add or remove edits the cluster definition, and a role has no reclaim policy of its own there. An init Job is imperative and nothing reconciles it.
- **Per-cell isolation needs one SQL step.** The 1.30.1 `Database` CRD has no field for database-level privileges, re-established from the CRD at implementation. PostgreSQL grants `CONNECT` and `TEMPORARY` to `PUBLIC` on a new database. The migration script and the doc's add-a-cell procedure therefore run `REVOKE CONNECT, TEMPORARY ON DATABASE <db> FROM PUBLIC` through `kubectl exec` into the current primary (from the Cluster's `status.currentPrimary`) as the local `postgres` user. That uses the in-pod socket and needs no superuser network access. Brief 16's `check-isolation.sh` gains a `--kubectl <namespace>/<cluster>` mode and detects a cell added without the step. Object ownership is the second layer, as in brief 16. An alternative to re-establish, not adopt blind: `spec.postgresql.pg_hba` with `samerole` rules. Whether user rules can deny before CNPG's own final rule depends on the release's rule ordering. <!-- graph: encoded desk-containers/16→desk-containers/17 -->
- **Secrets out of band.** Each cell's role Secret (`kubernetes.io/basic-auth`, `username` equal to the role name) and the object-store credential Secret are created by the operator, by hand or through External Secrets, Sealed Secrets or SOPS; the doc names these options without depending on any. `secret.example.yaml` shows the shapes with placeholder values only, and it is not in `resources:`, so the build never contains it. Example values: endpoint `https://s3.example.com`, bucket `s3://example-bucket/assay`.
- **Backups.** `ObjectStore` `assay-pg-store` with `endpointURL: https://s3.example.com`, `destinationPath: s3://example-bucket/assay`, `s3Credentials` naming the out-of-band Secret's keys, and a `retentionPolicy` (default `30d`, an overlay value). WAL archiving is continuous, so recovery is point-in-time. One `ScheduledBackup` runs daily with `method: plugin` and `pluginConfiguration.name: barman-cloud.cloudnative-pg.io`; CNPG's schedule takes six fields, seconds first. The recoverable point is the last archived WAL segment, and the doc states CNPG's archive timeout as the bound on loss for a quiet database (re-established from the release docs). It claims no zero loss and says replicas do not replace backups (doc 14 lines 11 and 44).
  - Set aside: the in-tree `.spec.backup.barmanObjectStore`, deprecated in 1.30 and to be removed in 1.31.0; and `volumeSnapshot` backups, which need a CSI snapshot class the example cannot assume.
- **Recovery leaves cells unable to connect.** The doc's restore procedure creates a new `Cluster` that bootstraps by recovery from the object store, through `externalClusters` with the plugin, `barmanObjectName` and `serverName`. Before applying it, the operator sets `login: false` on every cell's `DatabaseRole` in Git. Recovery restores role attributes as of the backup, and the CNPG operator then re-applies Git's `false`. So no cell connects to the restored cluster until the operator has completed the consumer's reconcile procedure (graph-execution/16 operator incarnation; doc 14 line 56) and flipped `login` back. That is the local `NOLOGIN` gate from brief 16, carried over. The consumer's paused-on-mismatched-incarnation check (graph-execution/19) is the independent second layer.
- **The local-to-CNPG migration (doc 14 line 15; doc 15 line 62)** is a runbook in `docs/postgres.md` (planned) plus `migrate-cell.sh`, which handles one cell at a time:
  1. **Pin compatible versions.** The script refuses unless the source and target server major versions are equal and the target minor is at least the source minor. Every `pg_extension` row in the source database must be declared in the target `Database`'s `extensions:` and be available on the target image.
  2. **Quiesce admission.** `cellctl down <cell>`, then brief 16's `provision-cell.sh --cell <cell> --disable-login`. The script refuses while `pg_stat_activity` shows any session for the role.
  3. **Declare the recovery point.** One final `backup.sh` run after quiescing. Its manifest (UTC start time and sha256) is the declared recovery point and is written to the migration log. Rebuild path: if the source cannot be dumped, the consumer rebuilds from canonical sources (graph-execution/19's reconstruction; doc 14 line 46) with admission paused. The script does not attempt that rebuild.
  4. **Migrate.** The target `Database` and `DatabaseRole` exist with `login: false`. The script streams the dump into the primary with `kubectl exec -i … pg_restore -U postgres -d <db> --exit-on-error --single-transaction` and then applies the per-cell `REVOKE`.
  5. **Validate.** The sorted data-only text of the dump must equal that of a dump taken from the target, as in brief 16's drill. Extension lists must match. The consumer's schema-migration version, if its table exists, must be equal. Any mismatch stops the run before the switch.
  6. **Switch endpoints.** `cellctl set <cell> CELL_DB_DSN=<target DSN>`, with TLS settings, and `CELL_DB_PASSFILE=<new pgpass file>`. The source role stays `NOLOGIN`, so a cell still pointed at the old server fails closed rather than writing to the stale copy. The source database is kept; retiring it after a retention window is a separate, manual operator step.
  7. **Reconcile outstanding effects.** This is the consumer's procedure (graph-execution/16 and /19), run before any reopen. If the cell has no consumer schema yet, the log records "nothing to reconcile: no consumer schema".
  8. **Reopen admission.** Set `login: true` on the cell's `DatabaseRole` in Git, then `cellctl desk` / `cellctl up`.
  - Never move a live local data directory onto a network volume and call that a migration (doc 14 line 15).
  - Set aside as the primary path: `bootstrap.initdb.import` (microservice/monolith). It runs only when a `Cluster` is created, so it cannot add a cell to a running cluster. It also needs network reach from the cluster to the host's loopback-only server. The doc mentions it as an option for a first, all-cells cutover into a new cluster.
- Not a core-system brief. The single control between a migrating cell and a split write is the source role's `NOLOGIN`. The target role's `login: false` until reopen is the second, independent layer; it trips on a different server.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands against any shared cluster or checkout. Cluster-side checks run only in a throwaway `kind` cluster that the drill creates and deletes; `kind-drill.sh` refuses to run unless the current kube context is the `kind-` context it created.
- Stop at `implemented` — you do not set verified/done.
- Public repository: example values only (`example-cell`, `example-org`, `example.com`, `s3.example.com`); no real organisation, host, bucket or person names, and no credential value in any committed file.
- Do not edit `containers/secrets.md` or brief 06's base files. If the base needs a change, report NEEDS_CONTEXT.
- If a field this brief names does not exist in the pinned release, or behaves differently, follow the release and record the difference in the PR body. If no design meets a fact above, report NEEDS_CONTEXT.

## Task
1. Re-establish the pins and the CRD fields from the release manifests and docs; record what you checked and where in the PR body.
2. Write the component, the reference overlay, the kind overlay and `secret.example.yaml` per the facts.
3. Write `migrate-cell.sh` (steps 1–6 of the procedure, with refusals; it prints steps 7–8 as the operator's next actions and performs neither) and `kind-drill.sh`. Add the `--kubectl` mode to brief 16's `check-isolation.sh`.
4. Add the Kubernetes sections to `docs/postgres.md` (planned): prerequisites and versions, Secrets, exposing the endpoint, HA limits, backups and the recovery bound, point-in-time restore with the `login: false` gate, adding a cell, and the migration runbook. Add the pointer line in `docs/docker.md` and the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `kubectl kustomize containers/k8s/overlays/postgres > /dev/null` | exit 0 — the reference overlay builds (needs brief 06's base) |
| 2 | check | `kubectl kustomize containers/k8s/overlays/postgres \| grep -c -e '^kind: Cluster$' -e '^kind: ObjectStore$' -e '^kind: ScheduledBackup$' -e '^kind: Database$' -e '^kind: DatabaseRole$'` | `5` — one of each in the reference overlay |
| 3 | check | `kubectl kustomize containers/k8s/overlays/postgres \| grep -c 'barmanObjectStore'; test $? -eq 1` | exit 0 — the in-tree backup stanza is absent |
| 4 | check | `kubectl kustomize containers/k8s/overlays/postgres \| grep -E 'imageName: ghcr\.io/cloudnative-pg/postgresql:18\.[0-9]+-[a-z-]+@sha256:[0-9a-f]{64}$' \| wc -l` | `1` — the operand is pinned by digest |
| 5 | check +dereference | `kubectl kustomize containers/k8s/overlays/postgres \| kubeconform -strict -summary -schema-location default -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' -` | exit 0, `Skipped: 0`. If the catalog lacks a kind (for example the plugin's `ObjectStore`), add a schema location generated from the pinned release's CRD manifest and record how; never `-ignore-missing-schemas`. Offline → could-not-check. |
| 6 | check | `kubectl kustomize containers/k8s/overlays/postgres \| grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' -e 'kind: Secret'; test $? -eq 1` | exit 0 — no Secret and no credential-shaped value in the build |
| 7 | check | `kubectl kustomize containers/k8s/overlays/postgres \| grep -c -e '^kind: Namespace$' -e '^kind: CustomResourceDefinition$' -e '^kind: ClusterRole$' -e '^kind: ClusterRoleBinding$'` | `1` (brief 06's `Namespace`) — the component adds no cluster-scoped resource |
| 8 | check | `shellcheck containers/postgres/migrate-cell.sh containers/postgres/kind-drill.sh containers/postgres/check-isolation.sh` | exit 0 |
| 9 | gate:model +flow +mutation | `containers/postgres/kind-drill.sh` (planned) — in a throwaway `kind` cluster: install cert-manager, CNPG and the plugin at the pinned versions, a throwaway MinIO as the object store, the Secrets with fake values, and `containers/k8s/overlays/postgres-kind`; start a throwaway brief-16 server with one provisioned cell holding test rows; run `migrate-cell.sh` for that cell; take an on-demand backup; recover a second Cluster from it with `login: false`; delete the kind cluster and the local project | exit 0. The output shows: `Cluster` healthy; migration validated (data, extensions); source role `NOLOGIN`; target role cannot log in before the flip and can after; `check-isolation.sh --kubectl` passes; the recovered cluster holds the rows and its cell role cannot log in; WAL archiving reports success. Mutation: skip step 2's `--disable-login` — `migrate-cell.sh` must refuse. No docker or kind → could-not-check. |
| 10 | check | `statusgen --root . --lint` | exit 0 |
| 10a | check | `statusgen --root . --consumers --brief desk-containers/17` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no. The component ships example manifests with no credential values; Secrets are created out of band. Every cluster-side check runs in a throwaway `kind` cluster that the drill creates and deletes. Both reclaim policies are `retain`, the migration never drops the source database, and recovery creates a new `Cluster`, so nothing here is irreversible. The reviewer checks:
- rows 3, 6, 7 and 9, including row 9's mutation;
- that `kind-drill.sh` refuses any kube context it did not create;
- that `migrate-cell.sh` has no path that reopens admission or sets `login: true`;
- that each CRD field used is cited in the PR body to the pinned release's CRD or docs.
