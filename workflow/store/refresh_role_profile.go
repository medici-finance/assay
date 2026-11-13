package store

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/medici-finance/assay/forgeread/refresh"
)

// RefreshRoleProfile names preexisting operator/runtime roles on an explicitly
// selected database. Rendering it executes no SQL and creates no role or grant.
// Both roles must already be nonadministrative LOGIN NOINHERIT identities without
// memberships or object ownership. Migration authority is a separate identity.
type RefreshRoleProfile struct {
	Database, Operator, Runtime string
}

type refreshTableRole struct {
	name, columns                         string
	runtimeInsert, runtimeUpdate          string
	operatorInsert, operatorUpdate        string
	operatorSelect, runtimeSelect, delete bool
}

// This is the one declared privilege profile, consumed by rendering and effective
// catalog checks. Independent fixture negatives pin its semantic containment.
var refreshRoleTables = []refreshTableRole{
	{name: "coordination_schema", columns: "version_id checksum is_applied applied_at", operatorSelect: true, runtimeSelect: true},
	{name: "mirror_scopes", columns: "scope_key provider host repository kind admission version state reason last_attempted last_complete last_inventory observed_up_to retained_from validator_since validator revision validator_full complete_scan full_inventory retention_required write_xid", runtimeSelect: true, runtimeInsert: "scope_key provider host repository kind admission", runtimeUpdate: "version state reason last_attempted last_complete last_inventory observed_up_to retained_from validator_since validator revision validator_full complete_scan full_inventory retention_required"},
	{name: "mirror_records", columns: "scope_key record_kind number payload in_inventory updated_at observed_at", runtimeSelect: true, runtimeInsert: "scope_key record_kind number payload in_inventory updated_at observed_at", runtimeUpdate: "payload in_inventory updated_at observed_at"},
	{name: "refresh_budgets", columns: "source_id profile known write_xid window_at window_ns http_used http_limit inflight_limit reservation_limit retry_at unknown_hold", operatorSelect: true, runtimeSelect: true, operatorInsert: "source_id profile known window_at window_ns http_used http_limit inflight_limit reservation_limit retry_at", operatorUpdate: "known", runtimeUpdate: "window_at http_used retry_at unknown_hold"},
	{name: "refresh_admissions", columns: "admission source_id profile known write_xid window_at window_ns fills_used fill_limit http_used http_limit active_limit waiter_limit failure_limit", operatorSelect: true, runtimeSelect: true, operatorInsert: "admission source_id profile known window_at window_ns fills_used fill_limit http_used http_limit active_limit waiter_limit failure_limit", operatorUpdate: "known", runtimeUpdate: "window_at fills_used http_used"},
	{name: "refresh_callers", columns: "admission caller_id known write_xid window_at fills_used fill_limit http_used http_limit active_limit waiter_limit", operatorSelect: true, runtimeSelect: true, operatorInsert: "admission caller_id known window_at fills_used fill_limit http_used http_limit active_limit waiter_limit", operatorUpdate: "known", runtimeUpdate: "window_at fills_used http_used"},
	{name: "refresh_fills", columns: "fill_key admission identity scope_key generation owner_id owner_reviews expires_at retire_at state reason retry_at snapshot_version base_retry_at review_retry_at attempts_used attempt_limit attempt_held review_only_partial completion_xid", runtimeSelect: true, runtimeInsert: "fill_key admission identity scope_key generation owner_id expires_at state attempt_limit retire_at owner_reviews", runtimeUpdate: "generation owner_id expires_at state reason retry_at attempt_held review_only_partial owner_reviews attempts_used completion_xid snapshot_version retire_at base_retry_at review_retry_at", delete: true},
	{name: "refresh_waiters", columns: "ticket_id admission fill_key generation caller_id reviews expires_at released", runtimeSelect: true, runtimeInsert: "ticket_id admission fill_key generation caller_id reviews expires_at", runtimeUpdate: "released"},
	{name: "refresh_attempts", columns: "reservation_id admission source_id fill_key generation caller_id grant_id expires_at observed unknown_hold http_count http_known decoded_bytes decoded_known response_complete status retry_knowledge retry_at observed_at source_observed_at reported_retry_at retry_delay_ns delay_known", operatorSelect: true, runtimeSelect: true, runtimeInsert: "reservation_id admission source_id fill_key generation caller_id grant_id expires_at", runtimeUpdate: "observed unknown_hold http_count decoded_bytes response_complete status retry_knowledge retry_at observed_at source_observed_at reported_retry_at retry_delay_ns delay_known http_known decoded_known"},
	{name: "refresh_initializations", columns: "kind identity_hash identity baseline initialized_at", operatorSelect: true, operatorInsert: "kind identity_hash identity baseline"},
}

func roleIdentifier(s string) bool {
	return s != "" && len(s) <= 63 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func roleColumns(columns string) string {
	items := strings.Fields(columns)
	for i, item := range items {
		items[i] = pgx.Identifier{item}.Sanitize()
	}
	return strings.Join(items, ",")
}

func roleLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// SQL returns one bounded, transactional manifest for a separately authorized
// operator act. It includes catalog assertions; widened effective privileges,
// ownership/membership and callable user SECURITY DEFINER paths refuse the whole
// transaction. It is not an installer, connection API or automatic startup path.
func (p RefreshRoleProfile) SQL() (string, error) {
	if !roleIdentifier(p.Database) || !roleIdentifier(p.Operator) || !roleIdentifier(p.Runtime) || p.Operator == p.Runtime {
		return "", refresh.ErrPolicy
	}
	database := pgx.Identifier{p.Database}.Sanitize()
	operator, runtime := pgx.Identifier{p.Operator}.Sanitize(), pgx.Identifier{p.Runtime}.Sanitize()
	var b strings.Builder
	b.WriteString("BEGIN;\nSET LOCAL standard_conforming_strings=on;\nSET LOCAL search_path=pg_catalog,public;\n")
	// Caller data stays outside dollar-quoted procedural source, including when
	// a valid quoted database identifier itself contains that delimiter.
	fmt.Fprintf(&b, "SELECT pg_catalog.set_config('assay.refresh_profile_database',%s,true);\n", roleLiteral(p.Database))
	b.WriteString("DO $assay_database$ BEGIN IF current_database()<>current_setting('assay.refresh_profile_database') THEN RAISE EXCEPTION 'refresh role database mismatch'; END IF; END $assay_database$;\n")
	fmt.Fprintf(&b, "REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s,%s;\nREVOKE CREATE,TEMPORARY ON DATABASE %s FROM PUBLIC;\nGRANT CONNECT ON DATABASE %s TO %s,%s;\n", database, operator, runtime, database, database, operator, runtime)
	fmt.Fprintf(&b, "REVOKE ALL PRIVILEGES ON SCHEMA public FROM %s,%s;\nREVOKE CREATE ON SCHEMA public FROM PUBLIC;\nGRANT USAGE ON SCHEMA public TO %s,%s;\n", operator, runtime, operator, runtime)
	for _, table := range refreshRoleTables {
		name := pgx.Identifier{"public", table.name}.Sanitize()
		columns := roleColumns(table.columns)
		fmt.Fprintf(&b, "REVOKE ALL PRIVILEGES ON TABLE %s FROM PUBLIC,%s,%s;\n", name, operator, runtime)
		// Table revocation does not revoke prior column grants.
		fmt.Fprintf(&b, "REVOKE SELECT(%s),INSERT(%s),UPDATE(%s),REFERENCES(%s) ON TABLE %s FROM PUBLIC,%s,%s;\n", columns, columns, columns, columns, name, operator, runtime)
		for _, role := range []struct {
			name, insert, update string
			selectAll, delete    bool
		}{{operator, table.operatorInsert, table.operatorUpdate, table.operatorSelect, false}, {runtime, table.runtimeInsert, table.runtimeUpdate, table.runtimeSelect, table.delete}} {
			for _, grant := range []struct{ operation, columns string }{{"SELECT", func() string {
				if role.selectAll {
					return table.columns
				}
				return ""
			}()}, {"INSERT", role.insert}, {"UPDATE", role.update}} {
				if grant.columns != "" {
					fmt.Fprintf(&b, "GRANT %s(%s) ON TABLE %s TO %s;\n", grant.operation, roleColumns(grant.columns), name, role.name)
				}
			}
			if role.delete {
				fmt.Fprintf(&b, "GRANT DELETE ON TABLE %s TO %s;\n", name, role.name)
			}
		}
	}
	for i, role := range []string{operator, runtime} {
		fmt.Fprintf(&b, "SET LOCAL ROLE %s;\n", role)
		b.WriteString(refreshRoleChecks(i == 1))
		b.WriteString("RESET ROLE;\n")
	}
	b.WriteString("COMMIT;\n")
	if b.Len() > 64<<10 {
		return "", refresh.ErrPolicy
	}
	return b.String(), nil
}

func refreshRoleChecks(runtime bool) string {
	var b strings.Builder
	b.WriteString(`DO $assay_role$
DECLARE actor oid := (SELECT oid FROM pg_roles WHERE rolname=current_user);
        target record; attribute record; privilege text; expected boolean;
BEGIN
 IF (SELECT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls FROM pg_roles WHERE oid=actor) IS DISTINCT FROM true
    OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=actor)
    OR EXISTS(SELECT 1 FROM pg_shdepend WHERE refclassid='pg_authid'::regclass AND refobjid=actor AND deptype='o' AND dbid IN (0,(SELECT oid FROM pg_database WHERE datname=current_database())))
    OR has_database_privilege(current_database(),'CREATE') OR has_database_privilege(current_database(),'TEMPORARY')
    OR has_database_privilege(current_database(),'CONNECT WITH GRANT OPTION')
    OR EXISTS(SELECT 1 FROM pg_namespace WHERE nspname !~ '^pg_' AND nspname<>'information_schema' AND has_schema_privilege(oid,'CREATE'))
    OR EXISTS(SELECT 1 FROM pg_namespace WHERE nspname !~ '^pg_' AND nspname<>'information_schema' AND has_schema_privilege(oid,'USAGE WITH GRANT OPTION'))
    OR EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prosecdef AND n.nspname !~ '^pg_' AND n.nspname<>'information_schema' AND has_schema_privilege(n.oid,'USAGE') AND has_function_privilege(p.oid,'EXECUTE'))
 THEN RAISE EXCEPTION 'unsafe effective refresh role'; END IF;
`)
	knownTables := make([]string, len(refreshRoleTables))
	for i, table := range refreshRoleTables {
		knownTables[i] = "public." + table.name
	}
	fmt.Fprintf(&b, " IF EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f','S') AND (n.nspname||'.'||c.relname)<>ALL(%s)) THEN RAISE EXCEPTION 'undeclared refresh relation'; END IF;\n", roleArray(strings.Join(knownTables, " ")))
	b.WriteString(" FOR target IN SELECT * FROM (VALUES\n")
	for i, table := range refreshRoleTables {
		insert, update, selectAll, delete := table.operatorInsert, table.operatorUpdate, table.operatorSelect, false
		if runtime {
			insert, update, selectAll, delete = table.runtimeInsert, table.runtimeUpdate, table.runtimeSelect, table.delete
		}
		if i != 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, "(%s,%s,%s,%s,%t,%t)", roleLiteral("public."+table.name), roleArray(table.columns), roleArray(insert), roleArray(update), selectAll, delete)
	}
	b.WriteString(`
 ) AS profile(name,columns,inserts,updates,reads,deletes) LOOP
  IF ARRAY(SELECT attname::text FROM pg_attribute WHERE attrelid=to_regclass(target.name) AND attnum>0 AND NOT attisdropped ORDER BY attnum) IS DISTINCT FROM target.columns
  THEN RAISE EXCEPTION 'refresh privilege schema mismatch'; END IF;
  FOR attribute IN SELECT attname::text AS name FROM pg_attribute WHERE attrelid=to_regclass(target.name) AND attnum>0 AND NOT attisdropped LOOP
   FOREACH privilege IN ARRAY ARRAY['SELECT','INSERT','UPDATE','REFERENCES'] LOOP
    expected := CASE privilege WHEN 'SELECT' THEN target.reads WHEN 'INSERT' THEN attribute.name=ANY(target.inserts) WHEN 'UPDATE' THEN attribute.name=ANY(target.updates) ELSE false END;
    IF has_column_privilege(target.name,attribute.name,privilege) IS DISTINCT FROM expected OR has_column_privilege(target.name,attribute.name,privilege||' WITH GRANT OPTION')
    THEN RAISE EXCEPTION 'refresh column privilege mismatch'; END IF;
   END LOOP;
  END LOOP;
  FOREACH privilege IN ARRAY ARRAY['SELECT','INSERT','UPDATE','REFERENCES','DELETE','TRUNCATE','TRIGGER','MAINTAIN'] LOOP
   expected := privilege='DELETE' AND target.deletes;
   IF has_table_privilege(target.name,privilege) IS DISTINCT FROM expected OR has_table_privilege(target.name,privilege||' WITH GRANT OPTION')
   THEN RAISE EXCEPTION 'refresh table privilege mismatch'; END IF;
  END LOOP;
 END LOOP;
END $assay_role$;
`)
	return b.String()
}

func roleArray(columns string) string {
	items := strings.Fields(columns)
	for i, item := range items {
		items[i] = roleLiteral(item)
	}
	return "ARRAY[" + strings.Join(items, ",") + "]::text[]"
}
