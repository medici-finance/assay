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
CREATE FUNCTION public.guard_refresh_eviction() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE observed_now timestamptz := clock_timestamp();
BEGIN
 IF (OLD.state='done' AND OLD.retire_at<=observed_now
     AND COALESCE(OLD.retry_at,'-infinity')<=observed_now
     AND COALESCE(OLD.base_retry_at,'-infinity')<=observed_now
     AND COALESCE(OLD.review_retry_at,'-infinity')<=observed_now
     AND NOT EXISTS(SELECT 1 FROM public.refresh_waiters w WHERE w.fill_key=OLD.fill_key AND NOT released AND expires_at>observed_now)
     AND NOT EXISTS(SELECT 1 FROM public.refresh_attempts a WHERE a.fill_key=OLD.fill_key AND (NOT observed OR unknown_hold))) IS DISTINCT FROM true
 THEN RAISE EXCEPTION 'unqualified refresh eviction'; END IF;
 RETURN OLD;
END $$;
CREATE TRIGGER refresh_eviction_guard BEFORE DELETE ON public.refresh_fills FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_eviction();
-- +goose StatementEnd
