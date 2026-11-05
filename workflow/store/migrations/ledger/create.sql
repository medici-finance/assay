CREATE TABLE public.coordination_schema (
    version_id bigint PRIMARY KEY CHECK (version_id >= 0),
    checksum text NOT NULL CHECK (checksum ~ '^[0-9a-f]{64}$'),
    is_applied boolean NOT NULL DEFAULT true CHECK (is_applied),
    applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
