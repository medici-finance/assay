package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// selectionRequest is an invocation value supplied by the policy owner, not a
// new policy register. Scope and source revision must match the collector; an
// access/configuration change requires refresh, never a silently smaller queue.
type selectionRequest struct {
	Policy            rf.SelectionPolicy
	ObservationPolicy string
	Scope             []rf.Repository
	Oldest            time.Time
	Capacity          *int
}

func planSnapshot(path, selection string, now time.Time, stdout io.Writer) error {
	var req selectionRequest
	d := json.NewDecoder(strings.NewReader(selection))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		return deskkit.Refused("reviewloop plan: --selection requires a complete JSON invocation: " + err.Error())
	}
	var extra any
	if d.Decode(&extra) != io.EOF || req.Capacity == nil || len(req.Scope) == 0 {
		return deskkit.Refused("reviewloop plan: selection requires capacity, scope and one JSON object")
	}
	b, err := readInput(path)
	if err != nil {
		return deskkit.Unverifiable("reviewloop plan: cannot read snapshot", err)
	}
	o, err := rf.DecodeObservation(b)
	if err != nil {
		return deskkit.Unverifiable("reviewloop plan: invalid snapshot", err)
	}
	if !reflect.DeepEqual(req.Scope, o.Inputs.Requested) {
		return deskkit.Unverifiable("reviewloop plan: access scope changed; refresh observation", nil)
	}
	p, err := rf.PlanCandidates(o, rf.ObservationPolicy{Now: now, Oldest: req.Oldest, Policy: req.ObservationPolicy}, req.Policy, *req.Capacity)
	if err != nil {
		return deskkit.Unverifiable("reviewloop plan: snapshot cannot support selection; refresh or hold", err)
	}
	// Complete machine artifact: no display prefix can truncate depth/idle inputs.
	if err := json.NewEncoder(stdout).Encode(p); err != nil {
		return deskkit.Unverifiable("reviewloop plan: cannot write plan", err)
	}
	if p.Idle.State == rf.IdleCouldNotCheck {
		return deskkit.Unverifiable(fmt.Sprint("reviewloop plan: ", p.Idle), nil)
	}
	return nil
}
