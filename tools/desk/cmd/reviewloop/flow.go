package main

import (
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

func LookupAction(action string) (rule, error) {
	r, err := reviewflow.LookupAction(action)
	if err != nil {
		return r, deskkit.Unverifiable(err.Error(), nil)
	}
	return r, nil
}
func ReadBoard(actions, prs []byte) (*Board, error) {
	b, err := reviewflow.ReadBoard(actions, prs)
	if err != nil {
		return nil, deskkit.Unverifiable(err.Error(), nil)
	}
	return b, nil
}
