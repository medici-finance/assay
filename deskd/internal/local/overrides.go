package local

import (
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// This file is the operator override table's LOCAL-STATE surface
// (the service contract, ruling §10.5), the sibling of write.go's read/ack
// surface (ruling D7). Like read/ack it writes deskd's OWN SQLite and NOTHING
// on the platform: the package imports internal/index and internal/domain and
// no transport, so "no platform write" is a property of what this code can
// reach. The record PR that mirrors an override to cells.yaml is a separate App
// write on the mcp side — the local table here stays authoritative until it
// merges.
//
// The typed override shape is [domain.Overrides] (the pure leaf), shared with
// the HTTP wire, the MCP read and the record PR's YAML so a rename in one is a
// compile error in the others (the exec-tier-(b) hazard this brief was raised
// on). This alias is the name the local surface uses for it.
type Overrides = domain.Overrides

// OverrideStore is the local override table over the index. It is a thin,
// typed seam so the HTTP write path (internal/serve) and the config-reload
// convergence share one entry point rather than each reaching into the DB.
type OverrideStore struct {
	DB *index.DB
	// Now is injectable so a test can pin set timestamps.
	Now func() time.Time
}

// NewOverrideStore builds a store with a real clock.
func NewOverrideStore(db *index.DB) *OverrideStore {
	return &OverrideStore{DB: db, Now: func() time.Time { return time.Now().UTC() }}
}

func (s *OverrideStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// Set upserts the SET keys of ov for cell, attributed to setBy (the answerer
// login). Absent keys are untouched — a partial POST changes only the keys it
// carries. It returns the cell's full table AFTER the write, so a caller echoes
// the merged result rather than only what it just set.
func (s *OverrideStore) Set(cell string, ov Overrides, setBy string) (Overrides, *time.Time, error) {
	if err := s.DB.SetOverrides(cell, ov, setBy, s.now()); err != nil {
		return Overrides{}, nil, err
	}
	return s.DB.Overrides(cell)
}

// Get returns the cell's current override table and its as_of (nil when the
// cell has no overrides). The loops read this — and only this — for the current
// value each tick; they never read the config file for it.
func (s *OverrideStore) Get(cell string) (Overrides, *time.Time, error) {
	return s.DB.Overrides(cell)
}

// AdoptFile is the config-reload convergence (layer b): when the cell's
// cells.yaml `overrides:` block is newer than the local table (its record PR
// merged), the file's values are adopted and the table converges on them. It
// returns whether an adoption happened, so the caller can log it.
func (s *OverrideStore) AdoptFile(cell string, fileOv Overrides, fileModTime time.Time) (bool, error) {
	return s.DB.AdoptFileOverrides(cell, fileOv, fileModTime)
}
