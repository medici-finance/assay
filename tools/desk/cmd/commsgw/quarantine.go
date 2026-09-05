package main

import (
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

// quarantine.go — thin aliases onto internal/commsqueue's filing convention,
// SHARED with ../commsloop (both quarantine through the same held-mailbox +
// deskfile-issue shape — see commsqueue's package doc).

// ErrQuarantined is always part of Quarantine's returned error chain when the
// held-mailbox write succeeded.
var ErrQuarantined = commsqueue.ErrQuarantined

// IssueFiler raises a filed issue naming a quarantined message and why.
type IssueFiler = commsqueue.IssueFiler

// RaisedByRole is the desk role commsgw's own filings are attributed to.
const RaisedByRole = commsqueue.RaisedByRole

// DeskfileIssueFiler is the concrete IssueFiler: it shells out to `deskfile`.
type DeskfileIssueFiler = commsqueue.DeskfileIssueFiler

// Quarantine durably holds env and files an issue naming it — never a silent
// drop. See commsqueue.Quarantine for the full contract.
func Quarantine(root string, env comms.Envelope, reason string, now time.Time, filer IssueFiler) error {
	return commsqueue.Quarantine(root, env, reason, now, filer)
}
