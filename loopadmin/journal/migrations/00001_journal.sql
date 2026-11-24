-- loopadmin execution journal, version 1. Forward-only: there is no Down
-- section, and the migration runner refuses one.

-- +goose Up
-- The passive activation record: the incarnation the operator last activated,
-- its key reference, the paused state and the stop-confirmation receipt.
-- Restoring this row never makes it current; the current incarnation is read
-- from protected operator state outside this database.
CREATE TABLE loopadmin.activation (
    cell_id        text        PRIMARY KEY CHECK (cell_id <> ''),
    incarnation_id text        NOT NULL CHECK (incarnation_id <> ''),
    key_ref        text        NOT NULL CHECK (key_ref <> ''),
    paused         boolean     NOT NULL,
    stop_receipt   text        NOT NULL CHECK (stop_receipt <> ''),
    recorded_at    timestamptz NOT NULL DEFAULT now()
);

-- One row per launch intent, keyed by caller namespace, invocation ID and
-- owner generation. The caller's stable request ID is unique per namespace so
-- a duplicate request returns the existing row instead of launching twice.
CREATE TABLE loopadmin.invocation (
    caller_namespace text        NOT NULL CHECK (caller_namespace <> ''),
    invocation_id    text        NOT NULL CHECK (invocation_id <> ''),
    owner_generation bigint      NOT NULL CHECK (owner_generation >= 0),
    request_id       text        NOT NULL CHECK (request_id <> ''),
    cell_id          text        NOT NULL CHECK (cell_id <> ''),
    profile          text        NOT NULL,
    intent_digest    text        NOT NULL,
    incarnation_id   text        NOT NULL,
    state            text        NOT NULL CHECK (state IN ('intended', 'launched', 'unknown', 'completed', 'failed', 'cancelled')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (caller_namespace, invocation_id, owner_generation),
    UNIQUE (caller_namespace, request_id)
);

-- The journal receipt handed to the caller. It commits in the same
-- transaction as its invocation row.
CREATE TABLE loopadmin.receipt (
    receipt_id       text        PRIMARY KEY CHECK (receipt_id <> ''),
    caller_namespace text        NOT NULL,
    invocation_id    text        NOT NULL,
    owner_generation bigint      NOT NULL,
    issued_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (caller_namespace, invocation_id, owner_generation),
    FOREIGN KEY (caller_namespace, invocation_id, owner_generation)
        REFERENCES loopadmin.invocation (caller_namespace, invocation_id, owner_generation)
);

-- Process facts recorded after the intent: lifecycle, cancellation, limits,
-- usage and checkpoints. Append-only.
CREATE TABLE loopadmin.event (
    seq              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    caller_namespace text        NOT NULL,
    invocation_id    text        NOT NULL,
    owner_generation bigint      NOT NULL,
    kind             text        NOT NULL CHECK (kind IN ('lifecycle', 'cancellation', 'limit', 'usage', 'checkpoint')),
    detail           jsonb       NOT NULL,
    recorded_at      timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (caller_namespace, invocation_id, owner_generation)
        REFERENCES loopadmin.invocation (caller_namespace, invocation_id, owner_generation)
);
