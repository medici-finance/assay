-- +goose Up
CREATE TABLE public.refresh_budgets (
 source_id bytea PRIMARY KEY CHECK(octet_length(source_id)>0),
 profile bytea NOT NULL CHECK(octet_length(profile)>0),
 known boolean NOT NULL,
 write_xid xid8,
 window_at timestamptz NOT NULL,
 window_ns bigint NOT NULL CHECK(window_ns>0),
 http_used bigint NOT NULL CHECK(http_used>=0),
 http_limit bigint NOT NULL CHECK(http_limit>0 AND http_used<=http_limit),
 inflight_limit bigint NOT NULL CHECK(inflight_limit>0),
 reservation_limit bigint NOT NULL CHECK(reservation_limit>0),
 retry_at timestamptz,
 unknown_hold boolean NOT NULL DEFAULT false
);
CREATE TABLE public.refresh_admissions (
 admission bytea PRIMARY KEY CHECK(octet_length(admission)>0),
 source_id bytea NOT NULL REFERENCES public.refresh_budgets(source_id),
 profile bytea NOT NULL CHECK(octet_length(profile)>0),
 known boolean NOT NULL,
 write_xid xid8,
 window_at timestamptz NOT NULL,
 window_ns bigint NOT NULL CHECK(window_ns>0),
 fills_used bigint NOT NULL CHECK(fills_used>=0),
 fill_limit bigint NOT NULL CHECK(fill_limit>0 AND fills_used<=fill_limit),
 http_used bigint NOT NULL CHECK(http_used>=0),
 http_limit bigint NOT NULL CHECK(http_limit>0 AND http_used<=http_limit),
 active_limit bigint NOT NULL CHECK(active_limit>0),
 waiter_limit bigint NOT NULL CHECK(waiter_limit>0),
 failure_limit bigint NOT NULL CHECK(failure_limit>0)
);
CREATE TABLE public.refresh_callers (
 admission bytea NOT NULL REFERENCES public.refresh_admissions(admission),
 caller_id bytea NOT NULL CHECK(octet_length(caller_id)>0),
 known boolean NOT NULL,
 write_xid xid8,
 window_at timestamptz NOT NULL,
 fills_used bigint NOT NULL CHECK(fills_used>=0),
 fill_limit bigint NOT NULL CHECK(fill_limit>0 AND fills_used<=fill_limit),
 http_used bigint NOT NULL CHECK(http_used>=0),
 http_limit bigint NOT NULL CHECK(http_limit>0 AND http_used<=http_limit),
 active_limit bigint NOT NULL CHECK(active_limit>0),
 waiter_limit bigint NOT NULL CHECK(waiter_limit>0),
 PRIMARY KEY(admission,caller_id)
);
CREATE TABLE public.refresh_fills (
 fill_key bytea PRIMARY KEY CHECK(octet_length(fill_key)=32),
 admission bytea NOT NULL REFERENCES public.refresh_admissions(admission),
 identity bytea NOT NULL CHECK(octet_length(identity)>0),
 scope_key bytea NOT NULL CHECK(octet_length(scope_key)=32),
 generation bytea NOT NULL CHECK(octet_length(generation)>0),
 owner_id bytea NOT NULL CHECK(octet_length(owner_id)>0),
 owner_reviews boolean NOT NULL,
 expires_at timestamptz NOT NULL,
 retire_at timestamptz NOT NULL,
 state text NOT NULL CHECK(state IN ('pending','done','held')),
 reason text NOT NULL DEFAULT '' CHECK(reason IN ('','unknown','permission','transport','rate-limited','truncated')),
 retry_at timestamptz,
 snapshot_version bigint NOT NULL DEFAULT 0 CHECK(snapshot_version>=0),
 base_retry_at timestamptz,
 review_retry_at timestamptz,
 attempts_used bigint NOT NULL DEFAULT 0 CHECK(attempts_used>=0),
 attempt_limit bigint NOT NULL CHECK(attempt_limit>0 AND attempts_used<=attempt_limit),
 attempt_held boolean NOT NULL DEFAULT false,
 review_only_partial boolean NOT NULL DEFAULT false CHECK(NOT review_only_partial OR state='done' AND owner_reviews),
 completion_xid xid8
);
CREATE TABLE public.refresh_waiters (
 ticket_id bytea PRIMARY KEY CHECK(octet_length(ticket_id)>0),
 admission bytea NOT NULL REFERENCES public.refresh_admissions(admission),
 fill_key bytea NOT NULL CHECK(octet_length(fill_key)=32),
 generation bytea NOT NULL CHECK(octet_length(generation)>0),
 caller_id bytea NOT NULL CHECK(octet_length(caller_id)>0),
 reviews boolean NOT NULL,
 expires_at timestamptz NOT NULL,
 released boolean NOT NULL DEFAULT false
);
CREATE TABLE public.refresh_attempts (
 reservation_id bytea PRIMARY KEY CHECK(octet_length(reservation_id)>0),
 admission bytea NOT NULL REFERENCES public.refresh_admissions(admission),
 source_id bytea NOT NULL REFERENCES public.refresh_budgets(source_id),
 fill_key bytea NOT NULL CHECK(octet_length(fill_key)=32),
 generation bytea NOT NULL CHECK(octet_length(generation)>0),
 caller_id bytea NOT NULL CHECK(octet_length(caller_id)>0),
 grant_id bytea NOT NULL CHECK(octet_length(grant_id)>0),
 expires_at timestamptz NOT NULL,
 observed boolean NOT NULL DEFAULT false,
 unknown_hold boolean NOT NULL DEFAULT false,
 http_count bigint NOT NULL DEFAULT 0 CHECK(http_count>=0),
 http_known boolean NOT NULL DEFAULT false,
 decoded_bytes bigint NOT NULL DEFAULT 0 CHECK(decoded_bytes>=0),
 decoded_known boolean NOT NULL DEFAULT false,
 response_complete boolean NOT NULL DEFAULT false,
 status integer NOT NULL DEFAULT 0 CHECK(status BETWEEN 0 AND 599),
 retry_knowledge text NOT NULL DEFAULT 'unknown' CHECK(retry_knowledge IN ('unknown','not-before','no-observed-constraint','qualified-no-response')),
 retry_at timestamptz,
 observed_at timestamptz,
 source_observed_at numeric(30,0),
 reported_retry_at numeric(30,0),
 retry_delay_ns bigint CHECK(retry_delay_ns>=0),
 delay_known boolean NOT NULL DEFAULT false
);
CREATE INDEX refresh_waiters_fill ON public.refresh_waiters(fill_key,generation);
CREATE INDEX refresh_attempts_source ON public.refresh_attempts(source_id,observed);
-- +goose StatementBegin
CREATE FUNCTION public.guard_refresh_budget() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'refresh budget deletion refused'; END IF;
 IF NEW.source_id<>OLD.source_id OR NEW.profile<>OLD.profile OR NEW.window_ns<>OLD.window_ns OR NEW.http_limit<>OLD.http_limit OR NEW.inflight_limit<>OLD.inflight_limit OR NEW.reservation_limit<>OLD.reservation_limit OR (NOT OLD.known AND NEW.known) THEN RAISE EXCEPTION 'refresh budget identity refused'; END IF;
 IF OLD.retry_at IS NOT NULL AND (NEW.retry_at IS NULL OR NEW.retry_at<OLD.retry_at) THEN RAISE EXCEPTION 'retry regression'; END IF;
 IF OLD.unknown_hold AND NOT NEW.unknown_hold THEN RAISE EXCEPTION 'unknown budget reset'; END IF;
 IF NEW.window_at<OLD.window_at OR NEW.window_at>clock_timestamp() THEN RAISE EXCEPTION 'window clock refused'; END IF;
 IF NEW.window_at>OLD.window_at THEN
  IF NEW.http_used<>0 OR clock_timestamp()<OLD.window_at+((OLD.window_ns::numeric/1000000000)*interval '1 second') OR EXISTS(SELECT 1 FROM public.refresh_attempts WHERE source_id=OLD.source_id AND (NOT observed OR unknown_hold)) THEN RAISE EXCEPTION 'unqualified budget reset'; END IF;
 ELSIF NEW.http_used<OLD.http_used THEN RAISE EXCEPTION 'unqualified window move'; END IF;
 IF NEW.http_used>OLD.http_used THEN IF NEW.http_used<>OLD.http_used+1 THEN RAISE EXCEPTION 'budget increment refused'; END IF; NEW.write_xid:=pg_current_xact_id(); ELSE NEW.write_xid:=OLD.write_xid; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER refresh_budget_guard BEFORE UPDATE OR DELETE ON public.refresh_budgets FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_budget();
CREATE FUNCTION public.guard_refresh_usage() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE duration bigint;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'refresh usage deletion refused'; END IF;
 IF NEW.admission<>OLD.admission OR (NOT OLD.known AND NEW.known) OR NEW.fill_limit<>OLD.fill_limit OR NEW.http_limit<>OLD.http_limit THEN RAISE EXCEPTION 'refresh usage identity refused'; END IF;
 IF TG_TABLE_NAME='refresh_callers' THEN
  IF NEW.caller_id<>OLD.caller_id OR NEW.active_limit<>OLD.active_limit OR NEW.waiter_limit<>OLD.waiter_limit THEN RAISE EXCEPTION 'caller identity refused'; END IF;
  SELECT window_ns INTO duration FROM public.refresh_admissions WHERE admission=OLD.admission;
 ELSE
  IF NEW.source_id<>OLD.source_id OR NEW.profile<>OLD.profile OR NEW.window_ns<>OLD.window_ns OR NEW.active_limit<>OLD.active_limit OR NEW.waiter_limit<>OLD.waiter_limit OR NEW.failure_limit<>OLD.failure_limit THEN RAISE EXCEPTION 'admission profile refused'; END IF;
  duration:=OLD.window_ns;
 END IF;
 IF NEW.window_at<OLD.window_at OR NEW.window_at>clock_timestamp() THEN RAISE EXCEPTION 'usage clock refused'; END IF;
 IF NEW.window_at>OLD.window_at THEN
  IF NEW.fills_used<>0 OR NEW.http_used<>0 OR clock_timestamp()<OLD.window_at+((duration::numeric/1000000000)*interval '1 second') OR EXISTS(SELECT 1 FROM public.refresh_attempts WHERE admission=OLD.admission AND (NOT observed OR unknown_hold)) THEN RAISE EXCEPTION 'unqualified usage reset'; END IF;
 ELSIF NEW.fills_used<OLD.fills_used OR NEW.http_used<OLD.http_used THEN RAISE EXCEPTION 'unqualified usage window'; END IF;
 IF NEW.http_used>OLD.http_used OR NEW.fills_used>OLD.fills_used THEN IF NEW.http_used>OLD.http_used+1 OR NEW.fills_used>OLD.fills_used+1 THEN RAISE EXCEPTION 'usage increment refused'; END IF; NEW.write_xid:=pg_current_xact_id(); ELSE NEW.write_xid:=OLD.write_xid; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER refresh_admission_guard BEFORE UPDATE OR DELETE ON public.refresh_admissions FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_usage();
CREATE TRIGGER refresh_caller_guard BEFORE UPDATE OR DELETE ON public.refresh_callers FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_usage();
CREATE FUNCTION public.guard_refresh_fill() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_version bigint; current_xid xid8; current_state text; current_complete boolean;
BEGIN
 IF TG_OP='INSERT' OR NEW.generation<>OLD.generation THEN
  IF NOT EXISTS(SELECT 1 FROM public.refresh_admissions WHERE admission=NEW.admission AND known AND write_xid=pg_current_xact_id()) OR NOT EXISTS(SELECT 1 FROM public.refresh_callers WHERE admission=NEW.admission AND caller_id=NEW.owner_id AND known AND write_xid=pg_current_xact_id()) THEN RAISE EXCEPTION 'fill usage not reserved'; END IF;
 END IF;
 IF TG_OP='INSERT' OR NEW.generation<>OLD.generation THEN
  IF EXISTS(SELECT 1 FROM public.refresh_admissions d WHERE admission=NEW.admission AND ((SELECT count(*) FROM public.refresh_fills WHERE admission=NEW.admission AND state='pending' AND expires_at>clock_timestamp() AND fill_key<>NEW.fill_key)>=d.active_limit OR TG_OP='INSERT' AND (SELECT count(*) FROM public.refresh_fills WHERE admission=NEW.admission)>=d.failure_limit)) OR EXISTS(SELECT 1 FROM public.refresh_callers c WHERE admission=NEW.admission AND caller_id=NEW.owner_id AND (SELECT count(*) FROM public.refresh_fills WHERE admission=NEW.admission AND owner_id=NEW.owner_id AND state='pending' AND expires_at>clock_timestamp() AND fill_key<>NEW.fill_key)>=c.active_limit) THEN RAISE EXCEPTION 'fill capacity refused'; END IF;
 END IF;
 IF TG_OP='INSERT' THEN RETURN NEW; END IF;
 IF NEW.fill_key<>OLD.fill_key OR NEW.admission<>OLD.admission OR NEW.identity<>OLD.identity OR NEW.scope_key<>OLD.scope_key THEN RAISE EXCEPTION 'fill identity refused'; END IF;
 IF NEW.generation<>OLD.generation THEN
  IF OLD.state='pending' AND OLD.expires_at>clock_timestamp() OR EXISTS(SELECT 1 FROM public.refresh_attempts WHERE fill_key=OLD.fill_key AND (NOT observed OR unknown_hold)) THEN RAISE EXCEPTION 'fill takeover refused'; END IF;
  IF NEW.state<>'pending' OR NEW.attempts_used<>0 THEN RAISE EXCEPTION 'fill reset refused'; END IF;
 ELSE
  IF NEW.owner_id<>OLD.owner_id OR NEW.owner_reviews<>OLD.owner_reviews OR NEW.expires_at<>OLD.expires_at OR NEW.attempt_limit<>OLD.attempt_limit OR NEW.attempts_used<OLD.attempts_used THEN RAISE EXCEPTION 'fill owner refused'; END IF;
  IF OLD.state<>'pending' AND (NEW.state<>OLD.state OR NEW.review_only_partial<>OLD.review_only_partial) THEN RAISE EXCEPTION 'completion replay refused'; END IF;
  IF NEW.state='done' AND OLD.state='pending' THEN
   IF OLD.expires_at<=clock_timestamp() OR NEW.completion_xid IS DISTINCT FROM pg_current_xact_id() THEN RAISE EXCEPTION 'unfenced completion'; END IF;
   SELECT version,write_xid,state,complete_scan INTO current_version,current_xid,current_state,current_complete FROM public.mirror_scopes WHERE scope_key=NEW.scope_key;
   IF COALESCE(current_version,0)<>NEW.snapshot_version OR (NEW.snapshot_version<>OLD.snapshot_version AND current_xid IS DISTINCT FROM pg_current_xact_id()) THEN RAISE EXCEPTION 'completion outside observation transaction'; END IF;
   IF NEW.review_only_partial AND (NEW.reason<>'' OR current_state IS DISTINCT FROM 'partial' OR current_complete OR current_xid IS DISTINCT FROM pg_current_xact_id()) THEN RAISE EXCEPTION 'unqualified review-only partial'; END IF;
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER refresh_fill_guard BEFORE INSERT OR UPDATE ON public.refresh_fills FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_fill();
CREATE FUNCTION public.guard_refresh_waiter() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.refresh_fills WHERE fill_key=NEW.fill_key AND admission=NEW.admission AND generation=NEW.generation AND state='pending' AND expires_at>clock_timestamp()) OR NOT EXISTS(SELECT 1 FROM public.refresh_admissions d WHERE admission=NEW.admission AND known AND (SELECT count(*) FROM public.refresh_waiters WHERE admission=NEW.admission AND NOT released AND expires_at>clock_timestamp())<d.waiter_limit) OR NOT EXISTS(SELECT 1 FROM public.refresh_callers c WHERE admission=NEW.admission AND caller_id=NEW.caller_id AND known AND (SELECT count(*) FROM public.refresh_waiters WHERE admission=NEW.admission AND caller_id=NEW.caller_id AND NOT released AND expires_at>clock_timestamp())<c.waiter_limit) OR NEW.released OR NEW.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'waiter capacity refused'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER refresh_waiter_guard BEFORE INSERT ON public.refresh_waiters FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_waiter();
CREATE FUNCTION public.guard_refresh_attempt_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.refresh_budgets WHERE source_id=NEW.source_id AND known AND NOT unknown_hold AND write_xid=pg_current_xact_id() AND (retry_at IS NULL OR retry_at<=clock_timestamp()) AND (SELECT count(*) FROM public.refresh_attempts WHERE source_id=NEW.source_id AND NOT observed)<inflight_limit AND (SELECT count(*) FROM public.refresh_attempts WHERE source_id=NEW.source_id)<reservation_limit) OR NOT EXISTS(SELECT 1 FROM public.refresh_admissions WHERE admission=NEW.admission AND source_id=NEW.source_id AND known AND write_xid=pg_current_xact_id()) OR NOT EXISTS(SELECT 1 FROM public.refresh_callers WHERE admission=NEW.admission AND caller_id=NEW.caller_id AND known AND write_xid=pg_current_xact_id()) OR NOT EXISTS(SELECT 1 FROM public.refresh_fills WHERE fill_key=NEW.fill_key AND admission=NEW.admission AND generation=NEW.generation AND owner_id=NEW.caller_id AND NEW.grant_id=NEW.generation AND expires_at=NEW.expires_at AND state='pending' AND expires_at>clock_timestamp() AND attempts_used=(SELECT count(*)+1 FROM public.refresh_attempts WHERE fill_key=NEW.fill_key AND generation=NEW.generation)) THEN RAISE EXCEPTION 'attempt not reserved'; END IF;
 IF NEW.observed OR NEW.unknown_hold OR NEW.http_count<>0 OR NEW.http_known OR NEW.decoded_bytes<>0 OR NEW.decoded_known OR NEW.observed_at IS NOT NULL THEN RAISE EXCEPTION 'pre-observed attempt refused'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER refresh_attempt_insert_guard BEFORE INSERT ON public.refresh_attempts FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_attempt_insert();
CREATE FUNCTION public.guard_refresh_attempt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'attempt deletion refused'; END IF;
 IF NEW.reservation_id<>OLD.reservation_id OR NEW.admission<>OLD.admission OR NEW.source_id<>OLD.source_id OR NEW.fill_key<>OLD.fill_key OR NEW.generation<>OLD.generation OR NEW.caller_id<>OLD.caller_id OR NEW.grant_id<>OLD.grant_id OR NEW.expires_at<>OLD.expires_at OR OLD.observed THEN RAISE EXCEPTION 'attempt report identity refused'; END IF;
 IF NOT NEW.observed THEN RAISE EXCEPTION 'attempt report required'; END IF;
 NEW.observed_at:=clock_timestamp();
 IF NOT NEW.unknown_hold THEN
  IF NOT NEW.http_known OR NEW.source_observed_at IS NULL THEN RAISE EXCEPTION 'unqualified observation'; END IF;
  IF NEW.http_count=0 THEN
   IF NEW.status<>0 OR NEW.response_complete OR NEW.decoded_bytes<>0 OR NOT NEW.decoded_known OR NEW.retry_knowledge<>'no-observed-constraint' OR NEW.reported_retry_at IS NOT NULL OR NEW.delay_known OR NEW.retry_delay_ns<>0 THEN RAISE EXCEPTION 'unqualified zero-entry observation'; END IF;
  ELSIF NEW.http_count<>1 OR NEW.retry_knowledge='unknown' OR NEW.retry_knowledge='qualified-no-response' AND (NEW.status<>0 OR NEW.response_complete OR NEW.reported_retry_at IS NOT NULL OR NEW.delay_known) OR NEW.retry_knowledge='no-observed-constraint' AND (NEW.status<100 OR NEW.reported_retry_at IS NOT NULL OR NEW.delay_known) OR NEW.retry_knowledge='not-before' AND (NEW.status<100 OR NEW.reported_retry_at IS NULL) THEN RAISE EXCEPTION 'unqualified observation'; END IF;
 END IF;

 NEW.retry_at:=NULL;
 IF NEW.reported_retry_at IS NOT NULL THEN
  NEW.retry_at:=to_timestamp(floor(NEW.reported_retry_at/1000000000)::double precision)+ceil((NEW.reported_retry_at-floor(NEW.reported_retry_at/1000000000)*1000000000)/1000)::double precision*interval '1 microsecond';
 END IF;
 IF NEW.delay_known AND NEW.retry_delay_ns>0 THEN
  NEW.retry_at:=greatest(NEW.retry_at,NEW.observed_at+ceil(NEW.retry_delay_ns::numeric/1000)::double precision*interval '1 microsecond');
 END IF;
 UPDATE public.refresh_budgets SET unknown_hold=unknown_hold OR NEW.unknown_hold,retry_at=greatest(retry_at,NEW.retry_at) WHERE source_id=NEW.source_id;
 RETURN NEW;
END $$;
CREATE TRIGGER refresh_attempt_guard BEFORE UPDATE OR DELETE ON public.refresh_attempts FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_attempt();
-- +goose StatementEnd
