---
id: DR-desk-containers-26
date: "2026-11-05"
title: "Local PostgreSQL for host cells with database isolation and restricted restore"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2940#issuecomment-6668052413"
alternatives:
  - "Put PostgreSQL in the desks' Compose project. The brief keeps it separate because the database must outlive desk restarts and serve host cells independently of desk container support."
  - "Load a cell-controlled archive from a superuser session with SET ROLE. The brief rejects this because RESET ROLE could restore superuser authority during data load."
  - "Approve with changes or hold the brief. Decision issue #2940 offered these alternatives. The driver selected option 1, approve as briefed."
accepted:
  - "Run a digest-pinned, loopback-only PostgreSQL service in a separate Compose project with its own named volume. Inject the administrator credential from a runtime file."
  - "Give each cell a database and a restricted role. Use a password-free DSN and a protected libpq password file. The database administrator is trusted. Host cells sharing an operating-system user can reach each other's credential files and the administrator's authority. Use a separate server or stronger host isolation where that boundary is insufficient."
  - "Use scheduled logical backups and retain the previous database during restore. Hold the cell role at NOLOGIN until consumer reconciliation and explicit re-enable. Load cell-controlled data in a one-shot non-superuser session and fence that session through ordered teardown and expiry."
  - "Keep database schemas and reconstruction with their existing consumers. Desk-containers/27 owns the Kubernetes profile and migration. Independent verification and final human acceptance remain separate lifecycle steps."
---

## Approval and binding

This is a retrospective record of the local profile in
[desk-containers/26](../desk-containers/brief-26-local-docker-postgres.md). The driver selected
option 1 on the decision issue on October 27, 2026, in an initial selection and a clarification.

- [Initial selection](https://github.com/medici-finance/assay/issues/2940#issuecomment-6454685177)
- [Clarification approving option 1](https://github.com/medici-finance/assay/issues/2940#issuecomment-6455035865)

The November 5 comment names `DR-desk-containers-26` and `desk-containers/26`, refers to
option 1 as briefed, and leaves independent verification and final acceptance pending. It binds
this record to the October approval under
[registers-v1 §7.5](../../../spec/registers-v1.md#75-the-ruling-link-and-its-corroboration).
The record and brief cite the same comment. [Named ruling](https://github.com/medici-finance/assay/issues/2940#issuecomment-6668052413)

## Delivery and ownership

[PR #3001](https://github.com/medici-finance/assay/pull/3001) delivered the brief on October 29,
2026. Its merge commit `bfcfe9a6f0f56673b7590ee2577775a480be0b2d` is the delivery witness.
The table locates the shipped files for each original task.

| Task | Delivery in #3001 |
| --- | --- |
| 1. Compose and init | `containers/postgres/compose.yaml` pins PostgreSQL, binds loopback, mounts the named volume and reads the runtime secret. `containers/postgres/initdb/00-harden.sql` restricts the default databases. |
| 2. Provision, back up, restore and check isolation | `containers/postgres/provision-cell.sh`, `backup.sh`, `restore.sh`, `check-isolation.sh` and `drill.sh` own those operator operations. `restore.sh` authenticates data load as the one-shot role and keeps the cell role disabled through reconciliation. |
| 3. Cellctl wiring | `tools/desk/cmd/cellctl/envkeys.go`, `launch.go`, `desk.go`, `env.go` and `codex_environment.go` carry the DSN and passfile. `celldb.go` and `check.go` check the pointer and file custody. `celldb_test.go` covers the cellctl contract. |
| 4. Operator guidance and release note | `docs/postgres.md` documents operations, recovery and the shared-host limit. `docs/cellctl.md`, `docs/docker.md`, `containers/README.md` and `changelog/desk-containers-26-local-postgres.md` carry the linked guidance. |

The local profile owns database provisioning and the credential and backup boundary described in
[the container credential contract](../../../containers/secrets.md) and [storage proposal 14](../../proposals/non-ci-architecture/14-storage-forge-graph.md).
The coordination store and operator incarnation stay with their existing graph-execution owners.
The dependency on this profile in desk-containers/27 remains in place. After the binding lands,
`statusgen reconcile` can assess the existing delivery. The brief's Verify table, Evidence and
final acceptance remain with their designated owners.
