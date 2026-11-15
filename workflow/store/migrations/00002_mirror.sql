-- +goose Up
CREATE TABLE public.mirror_scopes (
    scope_key bytea PRIMARY KEY CHECK (octet_length(scope_key)=32),
    provider text NOT NULL CHECK (provider IN ('github','gitlab')),
    host bytea NOT NULL CHECK (octet_length(host)>0),
    repository bytea NOT NULL CHECK (octet_length(repository)>0),
    kind text NOT NULL CHECK (kind IN ('issue','change')),
    admission bytea NOT NULL CHECK (octet_length(admission)>0),
    version bigint NOT NULL DEFAULT 0 CHECK (version>=0),
    state text NOT NULL DEFAULT 'never-read' CHECK (state IN ('never-read','complete','partial','unavailable')),
    reason text NOT NULL DEFAULT '' CHECK (reason IN ('','permission','transport','rate-limited','truncated','unknown')),
    last_attempted numeric(30,0),
    last_complete numeric(30,0),
    last_inventory numeric(30,0),
    observed_up_to numeric(30,0),
    retained_from numeric(30,0) CHECK (retained_from IS NULL),
    validator_since numeric(30,0),
    validator bytea NOT NULL DEFAULT ''::bytea,
    revision bytea NOT NULL DEFAULT ''::bytea,
    validator_full boolean NOT NULL DEFAULT false,
    complete_scan boolean NOT NULL DEFAULT false,
    full_inventory boolean NOT NULL DEFAULT false,
    retention_required boolean NOT NULL DEFAULT false,
    write_xid xid8 NOT NULL DEFAULT pg_current_xact_id(),
    CHECK (state<>'complete' OR (last_complete IS NOT NULL AND reason='')),
    CHECK (state<>'unavailable' OR reason<>''),
    CHECK (last_complete IS NULL OR (last_attempted IS NOT NULL AND last_complete<=last_attempted)),
    CHECK (last_inventory IS NULL OR (last_complete IS NOT NULL AND last_inventory<=last_complete)),
    CHECK (observed_up_to IS NULL OR (last_attempted IS NOT NULL AND observed_up_to<=last_attempted)),
    CHECK (validator_since IS NULL OR (last_attempted IS NOT NULL AND validator_since<=last_attempted))
);
CREATE TABLE public.mirror_records (
    scope_key bytea NOT NULL REFERENCES public.mirror_scopes(scope_key),
    record_kind text NOT NULL CHECK (record_kind IN ('issue','pull-request','merge-request')),
    number bigint NOT NULL CHECK (number>0),
    payload bytea NOT NULL CHECK (octet_length(payload)>1),
    in_inventory boolean NOT NULL,
    updated_at numeric(30,0),
    observed_at numeric(30,0),
    PRIMARY KEY(scope_key,record_kind,number)
);

-- +goose StatementBegin
CREATE FUNCTION public.guard_mirror_scope() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE zero_stamp constant numeric := -62135596800000000000;
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION 'mirror deletion has no retention authority'; END IF;
    IF TG_OP='INSERT' THEN
        IF NEW.version<>0 OR NEW.state<>'never-read' OR NEW.reason<>'' OR NEW.last_attempted IS NOT NULL OR NEW.last_complete IS NOT NULL OR NEW.last_inventory IS NOT NULL OR NEW.observed_up_to IS NOT NULL OR NEW.validator_since IS NOT NULL OR NEW.complete_scan OR NEW.full_inventory OR NEW.retention_required THEN
            RAISE EXCEPTION 'mirror scope must start never-read';
        END IF;
    ELSE
        IF NEW.scope_key IS DISTINCT FROM OLD.scope_key OR NEW.provider IS DISTINCT FROM OLD.provider OR NEW.host IS DISTINCT FROM OLD.host OR NEW.repository IS DISTINCT FROM OLD.repository OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.admission IS DISTINCT FROM OLD.admission THEN RAISE EXCEPTION 'mirror scope identity changed'; END IF;
        IF NEW.version<>OLD.version+1 OR NEW.last_attempted IS NULL OR NEW.state='never-read' THEN RAISE EXCEPTION 'invalid mirror transition'; END IF;
        IF coalesce(NEW.last_attempted,zero_stamp)<coalesce(OLD.last_attempted,zero_stamp) OR coalesce(NEW.last_complete,zero_stamp)<coalesce(OLD.last_complete,zero_stamp) OR coalesce(NEW.last_inventory,zero_stamp)<coalesce(OLD.last_inventory,zero_stamp) OR coalesce(NEW.observed_up_to,zero_stamp)<coalesce(OLD.observed_up_to,zero_stamp) THEN RAISE EXCEPTION 'mirror watermark regression'; END IF;
        IF NEW.state='complete' AND NOT NEW.complete_scan THEN RAISE EXCEPTION 'incomplete scan claims complete'; END IF;
        IF coalesce(NEW.last_complete,zero_stamp)>coalesce(OLD.last_complete,zero_stamp) AND (NOT NEW.complete_scan OR NEW.state<>'complete') THEN RAISE EXCEPTION 'incomplete completeness advance'; END IF;
        IF coalesce(NEW.observed_up_to,zero_stamp)>coalesce(OLD.observed_up_to,zero_stamp) AND (NOT NEW.complete_scan OR NEW.state<>'complete') THEN RAISE EXCEPTION 'incomplete observed watermark advance'; END IF;
        IF coalesce(NEW.last_inventory,zero_stamp)>coalesce(OLD.last_inventory,zero_stamp) AND (NOT NEW.complete_scan OR NOT NEW.full_inventory OR NEW.state<>'complete') THEN RAISE EXCEPTION 'incomplete inventory advance'; END IF;
        IF OLD.reason='permission' AND NOT coalesce((NEW.complete_scan AND NEW.full_inventory AND NEW.state='complete' AND NEW.last_complete>OLD.last_attempted AND NEW.last_inventory>OLD.last_attempted),false) THEN
            IF NEW.state<>'unavailable' OR NEW.reason<>'permission' OR NOT NEW.retention_required THEN RAISE EXCEPTION 'permission recovery needs fresh full inventory'; END IF;
        END IF;
        IF NEW.reason='permission' AND (NEW.state<>'unavailable' OR NOT NEW.retention_required) THEN RAISE EXCEPTION 'permission decision signal absent'; END IF;
        IF NEW.retention_required AND (NEW.state<>'unavailable' OR NEW.reason<>'permission') THEN RAISE EXCEPTION 'invalid retention hold'; END IF;
    END IF;
    NEW.write_xid:=pg_current_xact_id();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER mirror_scope_guard BEFORE INSERT OR UPDATE OR DELETE ON public.mirror_scopes FOR EACH ROW EXECUTE FUNCTION public.guard_mirror_scope();

-- +goose StatementBegin
CREATE FUNCTION public.guard_mirror_record() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s public.mirror_scopes; zero_stamp constant numeric := -62135596800000000000;
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION 'mirror deletion has no retention authority'; END IF;
    SELECT * INTO STRICT s FROM public.mirror_scopes WHERE scope_key=NEW.scope_key;
    IF s.write_xid<>pg_current_xact_id() OR s.retention_required OR s.version=0 THEN RAISE EXCEPTION 'record write lacks current scope transition'; END IF;
    IF TG_OP='INSERT' AND NOT NEW.in_inventory THEN RAISE EXCEPTION 'observed insert cannot claim absence'; END IF;
    IF (s.kind='issue' AND NEW.record_kind<>'issue') OR (s.kind='change' AND ((s.provider='github' AND NEW.record_kind<>'pull-request') OR (s.provider='gitlab' AND NEW.record_kind<>'merge-request'))) THEN RAISE EXCEPTION 'foreign record kind'; END IF;
    IF NEW.updated_at>s.last_attempted OR NEW.observed_at>s.last_attempted THEN RAISE EXCEPTION 'future record stamp'; END IF;
    IF TG_OP='UPDATE' THEN
        IF NEW.scope_key IS DISTINCT FROM OLD.scope_key OR NEW.record_kind IS DISTINCT FROM OLD.record_kind OR NEW.number IS DISTINCT FROM OLD.number THEN RAISE EXCEPTION 'record identity changed'; END IF;
        IF coalesce(NEW.updated_at,zero_stamp)<coalesce(OLD.updated_at,zero_stamp) OR coalesce(NEW.observed_at,zero_stamp)<coalesce(OLD.observed_at,zero_stamp) THEN RAISE EXCEPTION 'record timestamp regression'; END IF;
        IF OLD.in_inventory AND NOT NEW.in_inventory AND (NOT s.complete_scan OR NOT s.full_inventory OR s.state<>'complete') THEN RAISE EXCEPTION 'incomplete absence'; END IF;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER mirror_record_guard BEFORE INSERT OR UPDATE OR DELETE ON public.mirror_records FOR EACH ROW EXECUTE FUNCTION public.guard_mirror_record();
