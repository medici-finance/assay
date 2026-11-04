-- +goose Up
CREATE TABLE public.refresh_initializations (
 kind text NOT NULL CHECK(kind IN ('source','admission','caller')),
 identity_hash bytea NOT NULL CHECK(octet_length(identity_hash)=32),
 identity bytea NOT NULL CHECK(octet_length(identity)>0),
 baseline bytea NOT NULL CHECK(octet_length(baseline)>0),
 initialized_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(kind,identity_hash)
);
-- +goose StatementBegin
CREATE FUNCTION public.guard_refresh_initialization() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'initialization provenance is immutable';
END $$;
CREATE TRIGGER refresh_initialization_guard BEFORE UPDATE OR DELETE ON public.refresh_initializations FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_initialization();
-- +goose StatementEnd
