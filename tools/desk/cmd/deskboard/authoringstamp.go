package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readWorkerRecordFn reads and validates a worker attestation record for the authoring-stamp
// listing (#6855). A package var only so tests can serve a record without a forge-side issue.
var readWorkerRecordFn = deskkit.ReadWorkerAttestation

// authoringStampListing is the `prs` row's stamp-sweep listing (#6855): the coordinator's
// pull-side backstop for a dropped worker hand-off. It is READ-ONLY: it nominates a change from
// its description and present labels (deskkit.WorkerStampGap), and only for a nominated change
// reads the record, so the fix line it prints carries the record's own values. It never applies
// a label; `deskdispatch --stamp-only` does that and reads its own result back.
//
// A record that does not validate is reported on the row, never fatal to the board: the other
// rows are still true, and the row says plainly that this one has no fix line.
func authoringStampListing(repo string, p prBase) (state, fix string) {
	ref, gap := deskkit.WorkerStampGap(p.Body, p.labelNames())
	if !gap {
		return "", ""
	}
	if _, _, err := deskkit.ParseWorkerAttestationLine(p.Body); err != nil {
		return "refused: " + err.Error(), ""
	}
	if ref.Repo != repo {
		return "refused: the Worker-Attestation line names a record in " + ref.Repo + ", not " + repo, ""
	}
	f, fr, err := forgeFor(repo)
	if err != nil {
		return "could-not-check: " + oneLine(err.Error()), ""
	}
	rec, err := readWorkerRecordFn(f, fr, ref.Issue)
	if err != nil {
		if deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
			return "refused: " + oneLine(err.Error()), ""
		}
		return "could-not-check: " + oneLine(err.Error()), ""
	}
	if rec.Binding.Run != ref.Run {
		return "refused: the line names run " + ref.Run + " but the record is run " + rec.Binding.Run, ""
	}
	return "missing", deskkit.StampOnlyFixLine(rec, p.Number)
}
