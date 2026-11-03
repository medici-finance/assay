# Workflow persistence foundation

`store` is the canonical PostgreSQL connection/migration substrate. It uses pgx v5.12.0
and Goose v3.29.0, selected by `docs/go-engineering.md`; version-specific assessment and
the exact driver age exception are recorded in [issue 5396](https://github.com/medici-finance/assay/issues/5396).
No controller, execution claim, outbox, activation, mirror persistence or server is present.
The schema contract is [coordination-store-v1](../spec/coordination-store-v1.md).

Call `store.Migrate(ctx, explicitConfig)` for an explicitly authorized migration operation.
Call `store.Open(ctx, explicitConfig)` to validate an already compatible schema; Close the
returned handle. Open performs no DDL. Production calls never read a DSN or select an
ambient connection/credential. Nonempty PostgreSQL environment settings are refused.
No default endpoint, alternate backend, migration-down/reset API or secret-bearing error.
The database role owns migration DDL only when that operation is explicitly authorized;
provisioning, restricted application roles and activation remain operator-owned.

The foundation has been qualified only against PostgreSQL 18.6. Durability assumes normal
PostgreSQL durable commits and a functioning volume; backup rollback, whole-database loss,
operator incarnation and domain recovery remain full graph-execution/19 obligations.
The optional Goose CLI and its all-dialect drivers are not linked by this module.
Transitive checksum entries for upstream test dependencies are not runtime backends.

## Disposable container profile

Tests need a selected local Docker context whose endpoint the operator verified as local,
plus an explicit disposable PostgreSQL DSN. They never start Docker, select a context,
download a binary, inspect kubeconfig or contact an ambient database by default. The example
Compose file pins PostgreSQL 18.6; run it only with an explicit chosen local context. Bind
its host port to loopback, use a unique project name, inspect the mapped port after any
restart and supply the resulting synthetic-credential URI to the test command. Teardown
only the resources that exact project created; no shared-volume prune or daemon-wide cleanup.
Do not paste an operational DSN into this profile.

```sh
# From workflow/. Choose the context explicitly; inspect is local metadata only.
: "${ASSAY_TEST_DOCKER_CONTEXT:?select the verified local Docker context}"
endpoint=$(docker context inspect "$ASSAY_TEST_DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')
case "$endpoint" in unix://*) test -S "${endpoint#unix://}" ;; *) exit 1 ;; esac
printf 'Disposable fixture target: context=%s endpoint=%s\n' "$ASSAY_TEST_DOCKER_CONTEXT" "$endpoint"
ASSAY_TEST_PROJECT="assay-pg-$(python3 -c 'import uuid;print(uuid.uuid4().hex)')"
# Compose creates only this unique project's network and volume, with project labels.
docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml up -d --wait
published=$(docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml port postgres 5432)
case "$published" in 127.0.0.1:*) port=${published##*:} ;; *) exit 1 ;; esac
export ASSAY_TEST_POSTGRES_DSN="postgresql://fixture_admin:fixture_only@127.0.0.1:${port}/postgres?sslmode=disable"
```

After a container restart, repeat the `port` inspection and DSN assignment: some local
engines change a randomly published port when restarting. Keep the selected context and
unique project identity for teardown. Inspect Compose's project labels before deleting;
then the exact project cleanup is:

```sh
docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml ps
docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml down --volumes
```

Do not reuse a project name or apply this cleanup to another invocation's project.
No global `prune`, shared volume removal or default context is part of the profile.

```sh
# ASSAY_TEST_POSTGRES_DSN must already name the explicitly provisioned container.
test -n "$ASSAY_TEST_POSTGRES_DSN"
GOWORK=off go test -count=1 -timeout=120s ./store/...
GOWORK=off go vet ./...
```

The harness accepts a `postgresql://user:synthetic-password@127.0.0.1:port/postgres?sslmode=disable`
URI (an explicit IPv6 loopback is also supported). Missing input fails rather than skipping.
Every test uses a unique database with an owner marker; it does not reset the admin database.
The standalone CI job supplies its own disposable container and executes the complete suite.
No provider, cluster or production contact is part of these tests.

Schema/migration changes are forward-only additions to `store/migrations`; do not modify
recorded SQL checksums. Add future mirror/domain tables there, never in a competing ledger.
`TestPGFoundationSchema` dereferences the actual database catalog in both directions.
`TestPGFoundationConcurrent` launches two independent test processes against the same
explicit fixture; local goroutines alone do not qualify cross-process migration locking.
