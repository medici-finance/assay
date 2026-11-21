package main

import "github.com/medici-finance/assay/tools/desk/internal/deskkit"

// readWorkerRecordFn reads and validates a worker attestation record through the verb's
// resolved forge (the authoring-stamp condition, #6855). A package var only so tests can serve
// a record without a forge-side issue; production always reads through deskkit.
var readWorkerRecordFn = deskkit.ReadWorkerAttestation
