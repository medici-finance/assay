// Package index is deskd's SQLite store: a derived cache of GitHub, plus the
// one small table that is NOT derived from GitHub.
//
// # The pure-cache invariant
//
// Everything in the `derived_*` tables is re-derivable from GitHub. Nothing
// in them is authoritative — GitHub is. Deleting them and re-polling must
// reproduce them exactly, which is what makes a warm restart safe and what
// makes a corrupt index a five-minute problem rather than a data loss.
//
// The `local_*` tables are the exception, and they are the FIRST deskd state
// not rebuildable from GitHub (ruling D7): the read/ack timestamps a human
// leaves on this surface. They live under a separate name prefix, are
// created by a separate statement, and are untouched by [DB.DropDerived] —
// the function that implements "delete the cache". That separation is why
// the invariant is checkable rather than asserted: `DropDerived` physically
// cannot drop a local table, because the list it iterates is the derived one.
//
// This build lands the local table as SCHEMA ONLY. There is no setter, no
// endpoint and no CLI verb that writes read_at or acked_at — those are
// the service contract. The columns are here because a field is near-free at
// greenfield and costs a migration later.
package index

import (
	"database/sql"
	"fmt"
	"strings"
)

// derivedTables is the authoritative list of what "the cache" means. Adding
// a derived table without adding it here means [DB.DropDerived] leaves it
// behind, so the rebuild test (which asserts a full rebuild reproduces the
// row set) is what catches the omission.
var derivedTables = []string{
	"derived_escalations",
	"derived_options",
	"derived_repo_health",
	"derived_poll",
}

// schemaDerived is the re-derivable half.
//
// collapse_key and repeat are COLUMNS ONLY in this build (spec §1.1.3 merge
// behaviour is the service contract). collapse_key is empty for an unattributable
// filing, which is how "unattributable automations never merge" is enforced
// structurally: there is no key for a fold to match on.
//
// Every derived field that can fail to derive has a companion
// `*_confidence` column and a `field_confidence` JSON blob. A NULL value
// column beside a 'could-not-check' confidence is the storage form of the
// rule that an underivable field is never a zero.
const schemaDerived = `
CREATE TABLE IF NOT EXISTS derived_escalations (
    id              TEXT PRIMARY KEY,          -- domain EscalationID (ULID)
    cell            TEXT NOT NULL,
    repo            TEXT NOT NULL,             -- platform ref, adapter-side only
    platform_key    INTEGER NOT NULL,          -- platform ref, adapter-side only
    platform_url    TEXT NOT NULL,             -- serving-edge links.platform
    automation_cell TEXT,                      -- NULL = UNRESOLVED join
    automation_role TEXT,                      -- NULL = UNRESOLVED join
    kind            TEXT NOT NULL,
    provenance      TEXT NOT NULL,
    urgency         INTEGER NOT NULL,
    title           TEXT NOT NULL,
    situation       TEXT NOT NULL,
    rationale       TEXT NOT NULL DEFAULT '',
    raw_body        TEXT NOT NULL DEFAULT '',  -- retained when a v2 block did not parse
    options_rev     TEXT NOT NULL DEFAULT '',
    options_present INTEGER NOT NULL DEFAULT 0,
    collapse_key    TEXT NOT NULL DEFAULT '',  -- '' = unattributable, never folds
    repeat          INTEGER NOT NULL DEFAULT 1,
    filed_at        TEXT NOT NULL,
    updated_at      TEXT,                      -- derivation-set; NULL = never
    closed          INTEGER NOT NULL DEFAULT 0,
    -- The derived answer (the service contract increment 3, task 6): the outcome
    -- internal/adapter.DeriveAnswer parsed off a closed issue's polled
    -- comments. answer_at NULL is the "no derived answer" flag — a zero time
    -- would read as an answer at the epoch, exactly the fabricated-zero shape
    -- the rest of this schema already refuses (design §10).
    answer_login       TEXT,
    answer_is_bot      INTEGER NOT NULL DEFAULT 0,
    answer_option      TEXT,
    answer_options_rev TEXT NOT NULL DEFAULT '',
    answer_at          TEXT,
    -- answered-unverified (the authorization contract gap 3, item (5)): the marker
    -- could not be verified as the named human's answer; answer_login is
    -- then the comment's real author and answer_option is NULL.
    answer_unverified  INTEGER NOT NULL DEFAULT 0,
    field_confidence TEXT NOT NULL DEFAULT '{}',
    first_seen_at   TEXT NOT NULL,
    last_seen_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_esc_cell ON derived_escalations(cell);
CREATE INDEX IF NOT EXISTS idx_esc_collapse ON derived_escalations(collapse_key);

CREATE TABLE IF NOT EXISTS derived_options (
    escalation_id TEXT NOT NULL,
    position      INTEGER NOT NULL,
    option_id     TEXT NOT NULL,
    label         TEXT NOT NULL,
    consequence   TEXT NOT NULL,
    PRIMARY KEY (escalation_id, position)
);

CREATE TABLE IF NOT EXISTS derived_repo_health (
    repo        TEXT PRIMARY KEY,
    confidence  TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    probe       TEXT NOT NULL DEFAULT '{}',
    not_modified INTEGER NOT NULL DEFAULT 0,
    checked_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS derived_poll (
    k          TEXT PRIMARY KEY,
    started_at TEXT,
    ended_at   TEXT,
    outcome    TEXT
);
`

// schemaLocal is the half that is NOT derived from GitHub (ruling D7).
//
// It also holds the platform-ref ↔ domain-id map. That map is technically
// re-derivable (a cold rebuild may re-mint ids, which the spec accepts,
// "because no domain id appears on the wire"), but keeping it local means a
// cache drop does not churn every id in a running Console's open tabs. It is
// a comfort, not a correctness requirement, and it is recorded as such.
const schemaLocal = `
CREATE TABLE IF NOT EXISTS local_attention (
    escalation_id TEXT NOT NULL,
    actor         TEXT NOT NULL,   -- per-audience read (spec §1.1.2)
    read_at       TEXT,            -- nullable; no setter until the service contract
    acked_at      TEXT,            -- nullable; no setter until the service contract
    PRIMARY KEY (escalation_id, actor)
);

CREATE TABLE IF NOT EXISTS local_id_map (
    repo         TEXT NOT NULL,
    platform_key INTEGER NOT NULL,
    escalation_id TEXT NOT NULL UNIQUE,
    minted_at    TEXT NOT NULL,
    PRIMARY KEY (repo, platform_key)
);

CREATE TABLE IF NOT EXISTS local_drafts (
    escalation_id TEXT PRIMARY KEY,   -- one in-progress answer per escalation
    option        TEXT NOT NULL DEFAULT '',
    text          TEXT NOT NULL DEFAULT '',
    saved_at      TEXT NOT NULL
);

-- local_overrides is the operator override table (the service contract): the five
-- Drive-tab knobs a human turns to slow a cell down, in deskd's OWN SQLite
-- beside the D7 read/ack markers and untouched by DropDerived. It is service-
-- local state mirrored to git by a record PR, NOT derived from GitHub — the
-- SECOND deskd state not rebuildable from the platform (ruling §10.5). One row
-- per (cell, key) so a POST that sets a subset of keys upserts exactly those,
-- exactly as local_attention keys per (escalation, actor).
CREATE TABLE IF NOT EXISTS local_overrides (
    cell    TEXT NOT NULL,
    key     TEXT NOT NULL,   -- one of the closed override key vocabulary
    value   TEXT NOT NULL,   -- the value's string form (domain.Overrides.StringValue)
    set_by  TEXT NOT NULL,   -- the answerer login that set it (or a reload marker)
    set_at  TEXT NOT NULL,   -- RFC3339Nano; the newest is the table's as_of
    PRIMARY KEY (cell, key)
);
`

// applySchema creates both halves.
func applySchema(db *sql.DB) error {
	if err := dropStaleDerived(db); err != nil {
		return err
	}
	for name, stmt := range map[string]string{"derived": schemaDerived, "local": schemaLocal} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("apply %s schema: %w", name, err)
		}
	}
	return nil
}

// requiredDerivedColumn is the newest column the derived schema added. A
// derived_escalations table WITHOUT it was written by an older build that
// could not express answered-unverified — its stored answers were derived
// with no poll-side verification (the authorization contract gap 3, item (5)) — so
// the whole derived cache is dropped and re-derived on the next poll rather
// than migrated in place. That is the pure-cache invariant doing its job: a
// stale cached "answer" must never outlive the build that checks it, and
// nothing in a derived table is authoritative.
const requiredDerivedColumn = "answer_unverified"

// dropStaleDerived drops every derived table when derived_escalations exists
// but lacks [requiredDerivedColumn]. A fresh database (no table) and a current
// one are untouched. The local_* tables are never touched (ruling D7).
func dropStaleDerived(db *sql.DB) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info('derived_escalations')`)
	if err != nil {
		return fmt.Errorf("inspect derived schema: %w", err)
	}
	present, current := false, false
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("inspect derived schema: %w", err)
		}
		present = true
		if name == requiredDerivedColumn {
			current = true
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("inspect derived schema: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect derived schema: %w", err)
	}
	if !present || current {
		return nil
	}
	for _, t := range derivedTables {
		if strings.HasPrefix(t, "local_") {
			return fmt.Errorf("refusing to drop %q: a schema upgrade may not touch local state (ruling D7)", t)
		}
		if _, err := db.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return fmt.Errorf("drop stale derived table %s: %w", t, err)
		}
	}
	return nil
}
