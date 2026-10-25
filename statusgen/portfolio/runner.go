package portfolio

import (
	"encoding/json"
	"fmt"

	"github.com/medici-finance/assay/deskcore/domain"
)

// RunnerVersion is the loop-admin runner contract this translation reads. The runner
// contract is owned by loopadmin (schemas/loop-admin-runner-v1.json); this file decodes its
// documents strictly, interprets only the members it needs and never widens that contract.
const RunnerVersion = "loop-admin-runner/v1"

// Attribution is what a runner report does not carry and an attribution source must supply:
// the stable segment identity and revision, the profile revision, the measuring source, the
// subject and, when one was measured, the active interval. Elapsed is a separately labeled
// proxy; it is never read as active time.
type Attribution struct {
	Segment         SegmentRef
	ProfileRevision string
	Source          string
	Subject         *Subject
	Active          *Interval
	Elapsed         *int64
}

// AttributionSource resolves one invocation's attribution. It reports false when it holds
// none for that invocation.
type AttributionSource func(Invocation) (Attribution, bool)

// The runner records below carry every member the runner contract defines, so the strict
// decode refuses an unknown, case-variant, duplicate or absent required member exactly as the
// contract does. A member this translation does not read stays opaque (json.RawMessage) and
// is never interpreted here. TestPortfolioRunnerStrict holds these records to the schema.

type runnerRequest struct {
	Version   string          `json:"version"`
	Caller    string          `json:"caller"`
	ID        string          `json:"id"`
	Mode      json.RawMessage `json:"mode"`
	Desk      json.RawMessage `json:"desk"`
	Work      json.RawMessage `json:"work"`
	Packet    json.RawMessage `json:"packet"`
	Profile   runnerProfile   `json:"profile"`
	Workspace json.RawMessage `json:"workspace"`
	Authority runnerAuthority `json:"authority"`
	Budget    json.RawMessage `json:"budget"`
	Resume    json.RawMessage `json:"resume"`
	Require   json.RawMessage `json:"require"`
	Ext       json.RawMessage `json:"ext"`
}

func (runnerRequest) RequiredKeys() []string {
	return []string{"version", "caller", "id", "mode", "packet", "profile", "workspace", "authority", "budget"}
}

type runnerProfile struct {
	ID    string          `json:"id"`
	Model string          `json:"model"`
	Skill json.RawMessage `json:"skill"`
	Tools json.RawMessage `json:"tools"`
}

func (runnerProfile) RequiredKeys() []string { return []string{"id", "model", "tools"} }

type runnerAuthority struct {
	Key         json.RawMessage `json:"key"`
	Generation  uint64          `json:"generation"`
	ValidatedBy json.RawMessage `json:"validated_by"`
}

func (runnerAuthority) RequiredKeys() []string { return []string{"key", "generation", "validated_by"} }

type runnerObservation struct {
	State       string        `json:"state"`
	ActualModel string        `json:"actual_model"`
	Usage       runnerUsage   `json:"usage"`
	Result      *runnerResult `json:"result"`
}

func (runnerObservation) RequiredKeys() []string { return []string{"state", "usage"} }

type runnerUsage struct {
	InputTokens  json.RawMessage `json:"input_tokens"`
	OutputTokens json.RawMessage `json:"output_tokens"`
	CostMicros   *int64          `json:"cost_micros"`
}

type runnerResult struct {
	Caller       string          `json:"caller"`
	ID           string          `json:"id"`
	Generation   uint64          `json:"generation"`
	Outcome      json.RawMessage `json:"outcome"`
	Summary      json.RawMessage `json:"summary"`
	Artifacts    json.RawMessage `json:"artifacts"`
	ToolRequests json.RawMessage `json:"tool_requests"`
}

func (runnerResult) RequiredKeys() []string { return []string{"caller", "id", "generation", "outcome"} }

// Refusals name what failed, never what the report said: a runner report is caller content.
var (
	errRunnerRequest      = fmt.Errorf("portfolio: runner request does not match the runner contract")
	errRunnerVersion      = fmt.Errorf("portfolio: runner request is not %s", RunnerVersion)
	errRunnerObservation  = fmt.Errorf("portfolio: runner observation does not match the runner contract")
	errRunnerUnattributed = fmt.Errorf("portfolio: the attribution source has no attribution for the runner invocation")
)

// FromRunner translates one runner request and its terminal observation, both as the bytes
// the runner contract defines, into a ledger segment. The attribution source is required:
// the runner contract carries no active interval, so without one the segment's active time
// stays unknown rather than being inferred from anything else.
//
// The pinned profile model is kept as the profile's model and never stands in for the
// actual model: an observation that reports none leaves the actual model unknown. A negative
// cost is unknown, as the runner contract defines.
func FromRunner(request, observation []byte, attribute AttributionSource) (Segment, error) {
	if attribute == nil {
		return Segment{}, fmt.Errorf("portfolio: runner translation needs an attribution source")
	}
	// Another version is refused by name before its payload is evaluated.
	var header struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(request, &header); err != nil {
		return Segment{}, errRunnerRequest
	}
	if header.Version != RunnerVersion {
		return Segment{}, errRunnerVersion
	}
	var req runnerRequest
	if err := domain.DecodeStrict(request, &req); err != nil || req.Version != RunnerVersion {
		return Segment{}, errRunnerRequest
	}
	var obs runnerObservation
	if err := domain.DecodeStrict(observation, &obs); err != nil {
		return Segment{}, errRunnerObservation
	}
	if !member(obs.State, []string{"finished", "failed", "stopped"}) || obs.Result == nil {
		return Segment{}, fmt.Errorf("portfolio: runner observation is not a terminal report with a result")
	}
	if obs.Result.Caller != req.Caller || obs.Result.ID != req.ID || obs.Result.Generation != req.Authority.Generation {
		return Segment{}, fmt.Errorf("portfolio: runner result echoes another invocation or generation")
	}
	inv := Invocation{Caller: req.Caller, ID: req.ID}
	a, ok := attribute(inv)
	if !ok {
		return Segment{}, errRunnerUnattributed
	}
	cost := obs.Usage.CostMicros
	if cost != nil && *cost < 0 {
		cost = nil
	}
	g := Segment{
		Invocation:  inv,
		ID:          a.Segment.ID,
		Revision:    a.Segment.Revision,
		Profile:     Profile{ID: req.Profile.ID, Revision: a.ProfileRevision, Model: req.Profile.Model},
		ActualModel: obs.ActualModel,
		Source:      a.Source,
		Subject:     a.Subject,
		Active:      a.Active,
		Elapsed:     a.Elapsed,
		Cost:        cost,
	}
	return g, nil
}
