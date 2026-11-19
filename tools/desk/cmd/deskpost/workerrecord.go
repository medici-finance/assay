package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readWorkerRecord reads and validates a worker attestation record for the authoring-stamp
// ready condition (#6855) through the reviewer App's resolved forge — the identity this verb
// already acts under. A package var only so tests can serve a record without a forge-side issue.
var readWorkerRecord = func(fr deskkit.ForgeRepo, n int) (deskkit.WorkerReceipt, error) {
	fg, err := forgeForReviewer(fr)
	if err != nil {
		return deskkit.WorkerReceipt{Issue: n}, deskkit.Unverifiable(fmt.Sprintf("cannot resolve the forge to read worker attestation %s#%d", fr.Slug(), n), err)
	}
	return deskkit.ReadWorkerAttestation(fg, fr, n)
}
