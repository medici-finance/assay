package runner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Version is the protocol version string a v1 request carries.
const Version = "loop-admin-runner/v1"

// Mode is the execution mode of a launch request.
type Mode string

const (
	// ModeStandingDesk launches a role session for an existing desk or queue.
	// It needs a desk binding and no graph reference.
	ModeStandingDesk Mode = "standing-desk"
	// ModeWorkflowStage launches one bounded stage of a workflow instance. It
	// needs the canonical work, node and attempt references.
	ModeWorkflowStage Mode = "workflow-stage"
)

// Trust records where a piece of content came from. The contract preserves it;
// it never raises it.
type Trust string

const (
	// TrustOperator is content from an operator-approved profile or packet.
	TrustOperator Trust = "operator"
	// TrustCaller is content the caller assembled from its own records.
	TrustCaller Trust = "caller"
	// TrustUntrusted is model output, or content from outside the caller. It is
	// untrusted until a caller's own check accepts it.
	TrustUntrusted Trust = "untrusted"
)

// Capability names an optional adapter facility a request may make mandatory.
type Capability string

const (
	CapResume        Capability = "resume"
	CapCancelAck     Capability = "cancel-ack"
	CapBudgetLaunch  Capability = "budget-launch"
	CapBudgetRequest Capability = "budget-request"
	CapTelemetry     Capability = "telemetry"
	CapSnapshot      Capability = "snapshot"
	CapLaunchDedupe  Capability = "launch-dedupe"
)

var knownCapabilities = map[Capability]bool{
	CapResume: true, CapCancelAck: true, CapBudgetLaunch: true,
	CapBudgetRequest: true, CapTelemetry: true, CapSnapshot: true,
	CapLaunchDedupe: true,
}

// BudgetScope says whether a reservation bounds the whole launch or each
// request inside it. An adapter declares which it can enforce.
type BudgetScope string

const (
	BudgetLaunch  BudgetScope = "launch"
	BudgetRequest BudgetScope = "request"
)

// RolePacket is the role's instructions as a reference plus the trust of its
// source. It never carries credentials.
type RolePacket struct {
	Role  string `json:"role"`
	Ref   string `json:"ref"`
	Hash  string `json:"hash,omitempty"`
	Trust Trust  `json:"trust"`
}

// Profile is the operator-approved, pinned model/skill/tool profile. Tools is
// the complete list of tools the invocation may request; anything else is
// refused whatever the model says.
type Profile struct {
	ID    string   `json:"id"`
	Model string   `json:"model"`
	Skill string   `json:"skill,omitempty"`
	Tools []string `json:"tools"`
}

// Authority is the externally validated claim or ownership generation the
// launch runs under. The contract does not mint it and does not validate the
// claim itself; ValidatedBy names the component that did. Key identifies what
// the generation fences (a desk claim key, or a workflow attempt key).
type Authority struct {
	Key         string `json:"key"`
	Generation  uint64 `json:"generation"`
	ValidatedBy string `json:"validated_by"`
}

// Reservation is a reference to a budget reservation made by the caller. The
// contract carries it and the adapter enforces it at the declared scope.
type Reservation struct {
	ID            string      `json:"id"`
	Scope         BudgetScope `json:"scope"`
	MaxTokens     int64       `json:"max_tokens,omitempty"`
	MaxCostMicros int64       `json:"max_cost_micros,omitempty"`
}

// DeskRef is the standing-desk reference.
type DeskRef struct {
	BindingID string `json:"binding_id"`
}

// WorkRef holds the canonical workflow references. They are references to
// existing identities, never replacements for them.
type WorkRef struct {
	WorkID    string `json:"work_id"`
	NodeID    string `json:"node_id"`
	AttemptID string `json:"attempt_id"`
}

// SessionRef names a session to resume. It is pinned to a role and profile.
type SessionRef struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	ProfileID string `json:"profile_id"`
}

// LaunchRequest starts one invocation. (Caller, ID) is its stable identity: the
// same pair always means the same request.
type LaunchRequest struct {
	Version    string                     `json:"version"`
	Caller     string                     `json:"caller"`
	ID         string                     `json:"id"`
	Mode       Mode                       `json:"mode"`
	Desk       *DeskRef                   `json:"desk,omitempty"`
	Work       *WorkRef                   `json:"work,omitempty"`
	Packet     RolePacket                 `json:"packet"`
	Profile    Profile                    `json:"profile"`
	Workspace  string                     `json:"workspace"`
	Authority  Authority                  `json:"authority"`
	Budget     Reservation                `json:"budget"`
	Resume     *SessionRef                `json:"resume,omitempty"`
	Require    []string                   `json:"require,omitempty"`
	Extensions map[string]json.RawMessage `json:"ext,omitempty"`
}

// Ref is the stable handle for an invocation, derived from its request.
type Ref struct {
	Caller     string `json:"caller"`
	ID         string `json:"id"`
	Generation uint64 `json:"generation"`
}

// Ref returns the invocation's handle.
func (r LaunchRequest) Ref() Ref {
	return Ref{Caller: r.Caller, ID: r.ID, Generation: r.Authority.Generation}
}

func (r Ref) key() string { return r.Caller + "\x00" + r.ID }

// DecodeRequest parses a request and refuses any field this version does not
// define: an unknown field might be mandatory, and a v1 consumer cannot tell.
// Optional additions travel under "ext" instead.
func DecodeRequest(b []byte) (LaunchRequest, error) {
	var r LaunchRequest
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return LaunchRequest{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if dec.More() {
		return LaunchRequest{}, fmt.Errorf("%w: trailing data", ErrInvalidRequest)
	}
	return r, nil
}

// Validate checks everything about a request that does not depend on an
// adapter: version, identity, mode references, pinned profile, authority,
// budget and credential exclusion.
func (r LaunchRequest) Validate() error {
	if r.Version != Version {
		return fmt.Errorf("%w: %q", ErrUnsupportedVersion, r.Version)
	}
	for name, v := range map[string]string{
		"caller": r.Caller, "id": r.ID, "packet.role": r.Packet.Role,
		"packet.ref": r.Packet.Ref, "profile.id": r.Profile.ID,
		"profile.model": r.Profile.Model, "workspace": r.Workspace,
		"authority.key": r.Authority.Key, "authority.validated_by": r.Authority.ValidatedBy,
		"budget.id": r.Budget.ID,
	} {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%w: %s is required", ErrInvalidRequest, name)
		}
	}
	if r.Authority.Generation == 0 {
		return fmt.Errorf("%w: authority.generation is required", ErrInvalidRequest)
	}
	switch r.Packet.Trust {
	case TrustOperator, TrustCaller, TrustUntrusted:
	default:
		return fmt.Errorf("%w: packet.trust %q", ErrInvalidRequest, r.Packet.Trust)
	}
	if r.Profile.Tools == nil {
		return fmt.Errorf("%w: profile.tools is required (an empty list pins no tools)", ErrInvalidRequest)
	}
	switch r.Budget.Scope {
	case BudgetLaunch, BudgetRequest:
	default:
		return fmt.Errorf("%w: budget.scope %q", ErrInvalidRequest, r.Budget.Scope)
	}
	if r.Budget.MaxTokens <= 0 && r.Budget.MaxCostMicros <= 0 {
		return fmt.Errorf("%w: budget carries no limit", ErrInvalidRequest)
	}
	if err := r.validateMode(); err != nil {
		return err
	}
	if r.Resume != nil {
		if r.Resume.SessionID == "" || r.Resume.Role != r.Packet.Role || r.Resume.ProfileID != r.Profile.ID {
			return fmt.Errorf("%w: resume must name a session pinned to this role and profile", ErrInvalidRequest)
		}
	}
	return r.rejectCredentials()
}

func (r LaunchRequest) validateMode() error {
	switch r.Mode {
	case ModeStandingDesk:
		if r.Desk == nil || strings.TrimSpace(r.Desk.BindingID) == "" {
			return fmt.Errorf("%w: standing-desk mode requires desk.binding_id", ErrModeFields)
		}
		if r.Work != nil {
			return fmt.Errorf("%w: standing-desk mode takes no workflow references", ErrModeFields)
		}
	case ModeWorkflowStage:
		w := r.Work
		if w == nil || w.WorkID == "" || w.NodeID == "" || w.AttemptID == "" {
			return fmt.Errorf("%w: workflow-stage mode requires work_id, node_id and attempt_id", ErrModeFields)
		}
		if r.Desk != nil {
			return fmt.Errorf("%w: workflow-stage mode takes no desk binding", ErrModeFields)
		}
	default:
		return fmt.Errorf("%w: mode %q", ErrInvalidRequest, r.Mode)
	}
	return nil
}

func (r LaunchRequest) rejectCredentials() error {
	fields := []string{r.Caller, r.ID, r.Packet.Role, r.Packet.Ref, r.Packet.Hash,
		r.Profile.ID, r.Profile.Model, r.Profile.Skill, r.Workspace,
		r.Authority.Key, r.Authority.ValidatedBy, r.Budget.ID}
	fields = append(fields, r.Profile.Tools...)
	fields = append(fields, r.Require...)
	if r.Desk != nil {
		fields = append(fields, r.Desk.BindingID)
	}
	if r.Work != nil {
		fields = append(fields, r.Work.WorkID, r.Work.NodeID, r.Work.AttemptID)
	}
	if r.Resume != nil {
		fields = append(fields, r.Resume.SessionID)
	}
	for _, f := range fields {
		if LooksLikeCredential(f) {
			return ErrCredential
		}
	}
	for k, v := range r.Extensions {
		if CredentialKey(k) || LooksLikeCredential(k) || LooksLikeCredential(string(v)) {
			return fmt.Errorf("%w: extension %q", ErrCredential, k)
		}
	}
	return nil
}

// CheckCapabilities refuses the request when it marks anything mandatory that
// the adapter does not provide, or that this contract does not define. A
// requested resume the adapter cannot do is refused too, never started fresh.
func (r LaunchRequest) CheckCapabilities(c Capabilities) error {
	for _, name := range r.Require {
		if ext, ok := strings.CutPrefix(name, "ext:"); ok {
			if !c.hasExtension(ext) {
				return fmt.Errorf("%w: extension %q", ErrUnsupportedMandatory, ext)
			}
			continue
		}
		cap := Capability(name)
		if !knownCapabilities[cap] {
			return fmt.Errorf("%w: %q is not defined by %s", ErrUnsupportedMandatory, name, Version)
		}
		if !c.Has(cap) {
			return fmt.Errorf("%w: adapter does not declare %q", ErrUnsupportedMandatory, name)
		}
	}
	if r.Resume != nil && !c.Resume {
		return ErrResumeUnsupported
	}
	if !c.supportsBudget(r.Budget.Scope) {
		return fmt.Errorf("%w: adapter cannot enforce a %s-scoped budget", ErrUnsupportedMandatory, r.Budget.Scope)
	}
	return nil
}
