-- loopadmin execution journal, version 2 (DR-graph-exec-41, option 1:
-- enforced grants). Forward-only: there is no Down section, and the
-- migration runner refuses one. Version 1 is not edited.

-- +goose Up
-- The two roles of the grant model. Both are NOLOGIN group roles: a login is
-- made a member of one of them by an operator act outside this migration,
-- which configures no login, password or membership. The migration owner owns
-- the schema and every table and makes every schema change; the runtime role
-- owns nothing.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'loopadmin_owner') THEN
        CREATE ROLE loopadmin_owner NOLOGIN;
    END IF;
EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
END
$$;
-- +goose StatementEnd
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'loopadmin_runtime') THEN
        CREATE ROLE loopadmin_runtime NOLOGIN;
    END IF;
EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
END
$$;
-- +goose StatementEnd

-- A profile's launch budget: at most max_launches recorded launches in any
-- window of window_seconds. Operator-written; the runtime only reads it.
CREATE TABLE loopadmin.profile_budget (
    profile        text        PRIMARY KEY CHECK (profile <> ''),
    window_seconds bigint      NOT NULL CHECK (window_seconds > 0),
    max_launches   bigint      NOT NULL CHECK (max_launches >= 0),
    recorded_at    timestamptz NOT NULL DEFAULT now()
);

-- Admitted immutable limits of a profile. Operator-written; the runtime only
-- reads them.
CREATE TABLE loopadmin.profile_limit (
    profile     text        NOT NULL CHECK (profile <> ''),
    name        text        NOT NULL CHECK (name <> ''),
    value       bigint      NOT NULL CHECK (value >= 0),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (profile, name)
);

-- The append-only launch ledger: one row per admitted launch, the shared
-- profile reservation. Admission derives the window total from these rows.
-- The runtime may only insert and read them.
CREATE TABLE loopadmin.launch (
    seq              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profile          text        NOT NULL CHECK (profile <> ''),
    caller_namespace text        NOT NULL,
    invocation_id    text        NOT NULL,
    owner_generation bigint      NOT NULL,
    cell_id          text        NOT NULL CHECK (cell_id <> ''),
    incarnation_id   text        NOT NULL CHECK (incarnation_id <> ''),
    launched_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (caller_namespace, invocation_id, owner_generation),
    FOREIGN KEY (caller_namespace, invocation_id, owner_generation)
        REFERENCES loopadmin.invocation (caller_namespace, invocation_id, owner_generation)
);
CREATE INDEX launch_profile_time ON loopadmin.launch (profile, launched_at);

-- Usage reports for an invocation, each saying whether it is complete.
CREATE TABLE loopadmin.usage_report (
    seq              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    caller_namespace text        NOT NULL,
    invocation_id    text        NOT NULL,
    owner_generation bigint      NOT NULL,
    complete         boolean     NOT NULL,
    launches         bigint      NOT NULL CHECK (launches >= 0),
    recorded_at      timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (caller_namespace, invocation_id, owner_generation)
        REFERENCES loopadmin.invocation (caller_namespace, invocation_id, owner_generation)
);

-- Checkpoints of an invocation, by reference and digest.
CREATE TABLE loopadmin.checkpoint (
    caller_namespace text        NOT NULL,
    invocation_id    text        NOT NULL,
    owner_generation bigint      NOT NULL,
    ref              text        NOT NULL CHECK (ref <> ''),
    digest           text        NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    recorded_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (caller_namespace, invocation_id, owner_generation, ref),
    FOREIGN KEY (caller_namespace, invocation_id, owner_generation)
        REFERENCES loopadmin.invocation (caller_namespace, invocation_id, owner_generation)
);

-- Artifacts an invocation references outside the database, by reference and
-- digest. A backup's inventory is read from these rows.
CREATE TABLE loopadmin.artifact (
    caller_namespace text        NOT NULL,
    invocation_id    text        NOT NULL,
    owner_generation bigint      NOT NULL,
    ref              text        NOT NULL CHECK (ref <> ''),
    digest           text        NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    recorded_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (caller_namespace, invocation_id, owner_generation, ref),
    FOREIGN KEY (caller_namespace, invocation_id, owner_generation)
        REFERENCES loopadmin.invocation (caller_namespace, invocation_id, owner_generation)
);

-- Ownership: every object belongs to the migration owner.
ALTER SCHEMA loopadmin OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.journal_schema OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.activation OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.invocation OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.receipt OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.event OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.profile_budget OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.profile_limit OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.launch OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.usage_report OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.checkpoint OWNER TO loopadmin_owner;
ALTER TABLE loopadmin.artifact OWNER TO loopadmin_owner;

-- The runtime role: schema use without CREATE, so no DDL; insert and read on
-- the ledger and the typed records; read only on budgets and limits; the
-- version-1 runtime writes (invocation state, the activation pause) and
-- nothing else.
REVOKE ALL ON SCHEMA loopadmin FROM PUBLIC;
GRANT USAGE ON SCHEMA loopadmin TO loopadmin_runtime;
GRANT SELECT ON loopadmin.journal_schema TO loopadmin_runtime;
GRANT SELECT, UPDATE (paused, recorded_at) ON loopadmin.activation TO loopadmin_runtime;
GRANT SELECT, INSERT, UPDATE (state, updated_at) ON loopadmin.invocation TO loopadmin_runtime;
GRANT SELECT, INSERT ON loopadmin.receipt TO loopadmin_runtime;
GRANT SELECT, INSERT ON loopadmin.event TO loopadmin_runtime;
GRANT SELECT ON loopadmin.profile_budget TO loopadmin_runtime;
GRANT SELECT ON loopadmin.profile_limit TO loopadmin_runtime;
GRANT SELECT, INSERT ON loopadmin.launch TO loopadmin_runtime;
GRANT SELECT, INSERT ON loopadmin.usage_report TO loopadmin_runtime;
GRANT SELECT, INSERT ON loopadmin.checkpoint TO loopadmin_runtime;
GRANT SELECT, INSERT ON loopadmin.artifact TO loopadmin_runtime;
