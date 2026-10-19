package main

import (
	"errors"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// Legacy command adapters preserve the existing wire and exit-code contract.
type Disposition = reviewflow.Disposition
type BoardScope = reviewflow.BoardScope
type PRPopulation = reviewflow.PRPopulation
type ActionRow = reviewflow.ActionRow
type ActionsReport = reviewflow.ActionsReport
type PRRow = reviewflow.PRRow
type PRsReport = reviewflow.PRsReport
type Board = reviewflow.Board
type Row = reviewflow.Row
type Key = reviewflow.Key
type PlannedVerb = reviewflow.PlannedVerb
type IdleState = reviewflow.IdleState
type IdleVerdict = reviewflow.IdleVerdict
type rule = reviewflow.Rule

const DispositionUnknown = reviewflow.DispositionUnknown
const DispositionDispatch = reviewflow.DispositionDispatch
const DispositionFlip = reviewflow.DispositionFlip
const DispositionSurface = reviewflow.DispositionSurface
const DispositionWait = reviewflow.DispositionWait
const DispositionNoOp = reviewflow.DispositionNoOp
const HeadUnresolved = reviewflow.HeadUnresolved
const IdleCouldNotCheck = reviewflow.IdleCouldNotCheck
const IdleNo = reviewflow.IdleNo
const IdleYes = reviewflow.IdleYes
const MaxBoardAge = reviewflow.MaxBoardAge

var KnownActions = reviewflow.KnownActions
var Coalesce = reviewflow.Coalesce
var Dispatched = reviewflow.Dispatched
var Idle = reviewflow.Idle

// LookupAction and ReadBoard keep the command's historical error contract: the original
// message text with no kind prefix, exit 6, and the underlying decode or parse error still
// reachable through the Unwrap chain (so errors.As finds a *json.SyntaxError or
// *time.ParseError). The typed reviewflow.InputError stops here; its Detail and Cause are
// what the command always reported.
func LookupAction(action string) (rule, error) {
	r, err := reviewflow.LookupAction(action)
	if err != nil {
		return r, wireError(err)
	}
	return r, nil
}
func ReadBoard(actions, prs []byte) (*Board, error) {
	b, err := reviewflow.ReadBoard(actions, prs)
	if err != nil {
		return nil, wireError(err)
	}
	return b, nil
}

// wireError maps a reviewflow input error to the command's deskkit.Unverifiable, carrying
// the original detail text and the original cause. An error that is not an InputError is
// kept whole as the cause rather than flattened to text.
func wireError(err error) error {
	var ie *reviewflow.InputError
	if errors.As(err, &ie) {
		return deskkit.Unverifiable(ie.Detail, ie.Cause)
	}
	return deskkit.Unverifiable("reviewloop: "+err.Error(), err)
}
