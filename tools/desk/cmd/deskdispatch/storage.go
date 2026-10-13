package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Every execution kit can compile during implementation or verification. Treat
// all as heavy rather than silently exempt a newly added kit. This check runs
// before forge reads, credentials, claims or worktrees; recovery retries the
// same item and relies on the existing claim protocol for deduplication.
func storageAdmission(dry bool, root string) error {
	p, err := cellcache.FromEnv(os.Environ())
	if err != nil {
		return deskkit.Unverifiable("storage-deferred: invalid cache policy", err)
	}
	floor := uint64(0)
	if p != nil {
		floor = p.Floor
	}
	if value, set := os.LookupEnv("DESK_MIN_FREE_BYTES"); set {
		floor, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return deskkit.Unverifiable("storage-deferred: DESK_MIN_FREE_BYTES must be a nonnegative byte count", err)
		}
	}
	if p == nil {
		return storageVolumes(root, floor, dry, availableDiskBytes)
	}
	r, err := cellcache.Check(*p, dry)
	b, _ := json.Marshal(r)
	fmt.Fprintf(os.Stderr, "storage-admission %s\n", b)
	if err != nil {
		return deskkit.Unverifiable(err.Error(), err)
	}
	return storageVolumes(root, floor, dry, availableDiskBytes)
}

// Admission is a measurement, not a reservation against concurrent writers.
func storageVolumes(root string, floor uint64, dry bool, measure func(string) (uint64, error)) error {
	if floor == 0 {
		return nil
	}
	if root == "" {
		root = "."
	}
	for _, path := range []string{root, os.TempDir()} {
		free, err := measure(path)
		report := struct {
			Path      string  `json:"path"`
			Minimum   uint64  `json:"minimum_free_bytes"`
			Available *uint64 `json:"available_bytes"`
			DryRun    bool    `json:"dry_run"`
			Outcome   string  `json:"outcome"`
		}{Path: path, Minimum: floor, DryRun: dry, Outcome: "admitted"}
		if err == nil {
			report.Available = &free
		}
		if err != nil || free < floor {
			report.Outcome = "storage-deferred"
		}
		b, _ := json.Marshal(report)
		fmt.Fprintf(os.Stderr, "storage-volume %s\n", b)
		if err != nil {
			return deskkit.Unverifiable("storage-deferred: cannot measure available space at "+path, err)
		}
		if free < floor {
			return deskkit.Unverifiable(fmt.Sprintf("storage-deferred: %s has %d available bytes, requires %d; free space and retry the same item (nothing claimed)", path, free, floor), nil)
		}
	}
	return nil
}
