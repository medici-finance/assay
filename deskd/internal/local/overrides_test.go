package local_test

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/local"
)

func intp(n int) *int { return &n }

// TestOverridesConfigReloadAdoptsMergedFile proves layer (b) of the SPOF
// design: a cells.yaml `overrides:` block NEWER than the local table (its
// record PR merged) is adopted on reload and the table converges on it, while
// an OLDER file never overwrites a fresher local write. Without the AdoptFile
// convergence a merged file would never win — the table would stay authoritative
// forever, the exact "file merges; table still wins" failure the pre-mortem maps
// to this row.
func TestOverridesConfigReloadAdoptsMergedFile(t *testing.T) {
	db := openIndex(t)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	st := &local.OverrideStore{DB: db, Now: func() time.Time { return now }}

	// A local answerer sets dispatch_width=4 at `now`.
	if _, _, err := st.Set("assay", local.Overrides{DispatchWidth: intp(4)}, "operator"); err != nil {
		t.Fatalf("initial set: %v", err)
	}

	// An OLDER file (a merge that predates the local write) must NOT win.
	fileOv := local.Overrides{DispatchWidth: intp(8)}
	adopted, err := st.AdoptFile("assay", fileOv, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("adopt (older): %v", err)
	}
	if adopted {
		t.Fatalf("an older file was adopted — a merge predating the local write must not overwrite it")
	}
	got, _, err := st.Get("assay")
	if err != nil {
		t.Fatalf("get after older-file adopt: %v", err)
	}
	if got.DispatchWidth == nil || *got.DispatchWidth != 4 {
		t.Fatalf("table should still hold the local write 4, got %v", got.DispatchWidth)
	}

	// A NEWER file (its record PR merged) wins and the table converges on it.
	adopted, err = st.AdoptFile("assay", fileOv, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("adopt (newer): %v", err)
	}
	if !adopted {
		t.Fatalf("a file newer than the table was NOT adopted — layer (b) convergence did not fire")
	}
	got, _, err = st.Get("assay")
	if err != nil {
		t.Fatalf("get after newer-file adopt: %v", err)
	}
	if got.DispatchWidth == nil || *got.DispatchWidth != 8 {
		t.Fatalf("table should have adopted the merged file's 8, got %v", got.DispatchWidth)
	}
}

// TestOverridesLocalTableIsAuthoritative proves the one-reader-path leg: a POST
// merges keys into the table and Get returns exactly what was set, across two
// partial writes (the HTTP write and the MCP read read ONE table).
func TestOverridesLocalTableIsAuthoritative(t *testing.T) {
	db := openIndex(t)
	st := local.NewOverrideStore(db)

	hold := true
	if _, _, err := st.Set("assay", local.Overrides{DispatchWidth: intp(1), HoldMerges: &hold}, "operator"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, asOf, err := st.Get("assay")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DispatchWidth == nil || *got.DispatchWidth != 1 || got.HoldMerges == nil || !*got.HoldMerges {
		t.Fatalf("table did not echo the write: %+v", got)
	}
	if asOf == nil {
		t.Fatalf("a non-empty table must carry an as_of")
	}

	// A partial POST changes only the key it carries; the rest stay.
	pause := true
	if _, _, err := st.Set("assay", local.Overrides{PauseVerify: &pause}, "operator"); err != nil {
		t.Fatalf("partial set: %v", err)
	}
	got, _, _ = st.Get("assay")
	if got.DispatchWidth == nil || *got.DispatchWidth != 1 || got.PauseVerify == nil || !*got.PauseVerify {
		t.Fatalf("partial set should have preserved dispatch_width and added pause_verify: %+v", got)
	}
}
