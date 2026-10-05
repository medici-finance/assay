package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Every execution kit can compile during implementation or verification. Treat
// all as heavy rather than silently exempt a newly added kit. This check runs
// before forge reads, credentials, claims or worktrees; recovery retries the
// same item and relies on the existing claim protocol for deduplication.
func storageAdmission(dry bool) error {
	p, err := cellcache.FromEnv(os.Environ())
	if err != nil {
		return deskkit.Unverifiable("storage-deferred: invalid cache policy", err)
	}
	if p == nil {
		return nil
	}
	r, err := cellcache.Check(*p, dry)
	b, _ := json.Marshal(r)
	fmt.Fprintf(os.Stderr, "storage-admission %s\n", b)
	if err != nil {
		return deskkit.Unverifiable(err.Error(), err)
	}
	return nil
}
