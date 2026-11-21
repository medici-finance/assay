package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readWorkerRecord reads and validates a worker attestation record for the authoring-stamp
// ready condition (#6855) through the reviewer App's resolved forge — the identity this verb
// already acts under. A package var only so tests can serve a record without a forge-side issue.
var readWorkerRecord deskkit.WorkerRecordReader = func(fr deskkit.ForgeRepo, n int) (deskkit.WorkerReceipt, error) {
	fg, err := forgeForReviewer(fr)
	if err != nil {
		return deskkit.WorkerReceipt{Issue: n}, deskkit.Unverifiable(fmt.Sprintf("cannot resolve the forge to read worker attestation %s#%d", fr.Slug(), n), err)
	}
	return deskkit.ReadWorkerAttestation(fg, fr, n)
}

// onceWorkerRecord memoizes readWorkerRecord for one invocation, so the TOCTOU re-check decides
// on the record the first check read and costs no second read.
func onceWorkerRecord() deskkit.WorkerRecordReader {
	type key struct {
		slug string
		n    int
	}
	type val struct {
		r   deskkit.WorkerReceipt
		err error
	}
	seen := map[key]val{}
	return func(fr deskkit.ForgeRepo, n int) (deskkit.WorkerReceipt, error) {
		k := key{fr.Slug(), n}
		if v, ok := seen[k]; ok {
			return v.r, v.err
		}
		r, err := readWorkerRecord(fr, n)
		seen[k] = val{r, err}
		return r, err
	}
}

// authoringStamp runs the shared authoring-stamp condition (#6855) for `ready`. stop is true
// when the verb must return res (a refusal, or could-not-check); otherwise the decision line is
// reported on stderr and the flip proceeds.
func authoringStamp(repo string, pr int, head string, fr deskkit.ForgeRepo, body string,
	tl deskkit.StampTimeline, read deskkit.WorkerRecordReader) (writeResult, bool) {
	msg, err := deskkit.AuthoringStampGate(fr, pr, body, tl, read)
	if err != nil {
		if deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
			return refused("ready", repo, pr, head, err.Error()), true
		}
		return unverifiableNoWrite("ready", repo, pr, head, err.Error(), err), true
	}
	fmt.Fprintf(stderr, "deskpost: %s OK: %s\n", deskkit.AuthoringStampCondition, msg)
	return writeResult{}, false
}
