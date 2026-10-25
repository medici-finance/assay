package index

import (
	"fmt"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// This file is the local_overrides setters/getters (the service contract, ruling
// §10.5): the operator override table in deskd's OWN SQLite. Like the read/ack
// setters (setters.go, ruling D7) it writes ONE local table and reaches no
// GitHub client — "no platform write" is a property of what internal/index can
// reach, not a promise. The record PR that mirrors an override to cells.yaml is
// a SEPARATE App write on the mcp side; this table stays authoritative until
// that PR merges (the single-point-of-failure the brief names).

// overrideReloadActor marks a row the config-reload convergence adopted from a
// merged cells.yaml, distinct from an answerer login (setby). It is how the
// adoption is attributable in the table without pretending a human set it.
const overrideReloadActor = "config-reload"

// SetOverrides upserts every SET key of ov for cell, attributing each to setBy
// at time at. Absent keys are left untouched — a POST that sets a subset of
// keys changes exactly that subset, exactly as local_attention keys per
// (escalation, actor). The value column stores the key's string form; the
// (cell, key) primary key makes each upsert a per-key replace.
func (d *DB) SetOverrides(cell string, ov domain.Overrides, setBy string, at time.Time) error {
	if cell == "" {
		return fmt.Errorf("setter refused: empty cell")
	}
	ts := at.UTC().Format(time.RFC3339Nano)
	for _, key := range ov.SetKeys() {
		value, _ := ov.StringValue(key)
		if _, err := d.sql.Exec(
			`INSERT INTO local_overrides (cell, key, value, set_by, set_at)
			 VALUES (?,?,?,?,?)
			 ON CONFLICT(cell, key) DO UPDATE SET value = excluded.value, set_by = excluded.set_by, set_at = excluded.set_at`,
			cell, key, value, setBy, ts); err != nil {
			return fmt.Errorf("override upsert (cell %q, key %q) failed: %w", cell, key, err)
		}
	}
	return nil
}

// Overrides returns the current override table for a cell, assembled from the
// per-key rows, and the table's as_of — the NEWEST set_at across its rows (nil
// when the cell has no overrides). The loops read ONLY this (never the config
// file) for the current value, which is the one-reader-path leg of the SPOF
// design: the table is the single answer while a record PR is in flight.
func (d *DB) Overrides(cell string) (domain.Overrides, *time.Time, error) {
	rows, err := d.sql.Query(`SELECT key, value, set_at FROM local_overrides WHERE cell = ?`, cell)
	if err != nil {
		return domain.Overrides{}, nil, err
	}
	defer rows.Close()
	var ov domain.Overrides
	var asOf *time.Time
	for rows.Next() {
		var key, value, setAt string
		if err := rows.Scan(&key, &value, &setAt); err != nil {
			return domain.Overrides{}, nil, err
		}
		if err := ov.SetFromString(key, value); err != nil {
			return domain.Overrides{}, nil, fmt.Errorf("cell %q: %w", cell, err)
		}
		if t := parseOptTime(setAt); t != nil && (asOf == nil || t.After(*asOf)) {
			asOf = t
		}
	}
	if err := rows.Err(); err != nil {
		return domain.Overrides{}, nil, err
	}
	return ov, asOf, nil
}

// overridesLatestSetAt returns the newest set_at across a cell's rows, or the
// zero time when the cell has no overrides.
func (d *DB) overridesLatestSetAt(cell string) (time.Time, error) {
	var raw string
	err := d.sql.QueryRow(`SELECT COALESCE(MAX(set_at), '') FROM local_overrides WHERE cell = ?`, cell).Scan(&raw)
	if err != nil {
		return time.Time{}, err
	}
	if t := parseOptTime(raw); t != nil {
		return *t, nil
	}
	return time.Time{}, nil
}

// AdoptFileOverrides is the config-reload convergence (layer b of the SPOF
// design). When a cell's cells.yaml `overrides:` block is NEWER than the local
// table — its record PR merged, so fileModTime post-dates every local set_at —
// deskd adopts the file's values, replacing the table for that cell, and
// reports adopted=true so the caller can log it. Convergence happens in a
// DIFFERENT component (config load) than the HTTP handler that wrote the table,
// which is what makes the two-writer race resolvable rather than permanent.
//
// When the file is NOT newer (a local write is more recent than the last
// merge, or the file carries no overrides), the local table wins and nothing
// is adopted — the table stays authoritative until a fresher merge lands.
func (d *DB) AdoptFileOverrides(cell string, fileOv domain.Overrides, fileModTime time.Time) (bool, error) {
	if cell == "" {
		return false, fmt.Errorf("adopt refused: empty cell")
	}
	latest, err := d.overridesLatestSetAt(cell)
	if err != nil {
		return false, err
	}
	// The file must be strictly newer than every local write to win. A file at
	// or before the newest local set_at is not evidence of a fresher merge, so
	// the table — which may hold an override set AFTER the last reload — stands.
	if !fileModTime.After(latest) {
		return false, nil
	}
	// Replace the cell's rows with the file's values wholesale: the merged file
	// is now the source, so a key the file dropped is no longer an override.
	if _, err := d.sql.Exec(`DELETE FROM local_overrides WHERE cell = ?`, cell); err != nil {
		return false, fmt.Errorf("adopt: clearing cell %q failed: %w", cell, err)
	}
	if fileOv.IsEmpty() {
		// The merged file cleared every override; the table is now empty and
		// converged. That IS an adoption — the local overrides are gone.
		return true, nil
	}
	if err := d.SetOverrides(cell, fileOv, overrideReloadActor, fileModTime); err != nil {
		return false, err
	}
	return true, nil
}
