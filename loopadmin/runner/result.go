package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// State is the lifecycle state of an invocation as the contract tracks it.
type State string

const (
	// StateUnknown: the outcome is not known (launch acknowledgment lost, or
	// the adapter could not say). It holds the authority until reconciled.
	StateUnknown State = "unknown"
	// StateRunning: the adapter reports the invocation running.
	StateRunning State = "running"
	// StateFinished: the adapter reports a terminal result. This is NOT
	// acceptance; see Client.AcceptResult.
	StateFinished State = "finished"
	// StateFailed: the launch definitely did not start, or definitely ended
	// without a result.
	StateFailed State = "failed"
	// StateCancelRequested: a cancel was requested. It is not a confirmed stop.
	StateCancelRequested State = "cancel-requested"
	// StateStopped: the adapter observed the invocation stopped.
	StateStopped State = "stopped"
	// StateAbsent is adapter-only: reconcile found no record of the identity.
	StateAbsent State = "absent"
)

// holds reports whether an invocation in this state may still be running and
// so still owns its authority: no replacement may start under it.
func (s State) holds() bool {
	return s == StateUnknown || s == StateRunning || s == StateCancelRequested
}

// Outcome is what the adapter says the invocation concluded with.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
)

// Artifact is a produced output, by reference and hash.
type Artifact struct {
	Name  string `json:"name"`
	Ref   string `json:"ref"`
	Hash  string `json:"hash"`
	Trust Trust  `json:"trust"`
}

// ToolRequest is a tool the invocation asked to use. Note is the model's own
// text and carries no authority.
type ToolRequest struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

// Result is the adapter's terminal report. Generation is the authority
// generation the attempt ran under, echoed back by the adapter.
type Result struct {
	Generation   uint64        `json:"generation"`
	Outcome      Outcome       `json:"outcome"`
	Summary      string        `json:"summary,omitempty"`
	Artifacts    []Artifact    `json:"artifacts,omitempty"`
	ToolRequests []ToolRequest `json:"tool_requests,omitempty"`
}

// Observation is one adapter report of an invocation's state.
type Observation struct {
	State       State   `json:"state"`
	ActualModel string  `json:"actual_model,omitempty"`
	Usage       Usage   `json:"usage"`
	Result      *Result `json:"result,omitempty"`
}

// Receipt is the adapter's launch acknowledgment.
type Receipt struct {
	Ref         Ref    `json:"ref"`
	SessionID   string `json:"session_id,omitempty"`
	ActualModel string `json:"actual_model,omitempty"`
}

// CancelAck is the adapter's cancel acknowledgment. Acknowledged means the
// adapter accepted the request; it does not mean the invocation stopped.
type CancelAck struct {
	Acknowledged bool `json:"acknowledged"`
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Validate checks a result's shape and credential exclusion.
func (r Result) Validate() error {
	if r.Generation == 0 {
		return fmt.Errorf("%w: generation is required", ErrMalformedResult)
	}
	switch r.Outcome {
	case OutcomeSuccess, OutcomeFailure:
	default:
		return fmt.Errorf("%w: outcome %q", ErrMalformedResult, r.Outcome)
	}
	for i, a := range r.Artifacts {
		if strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Ref) == "" {
			return fmt.Errorf("%w: artifact %d needs a name and a ref", ErrMalformedResult, i)
		}
		if !sha256Hex.MatchString(a.Hash) {
			return fmt.Errorf("%w: artifact %q hash is not a sha256", ErrMalformedResult, a.Name)
		}
		// Model output is untrusted until a caller's own check accepts it; an
		// adapter cannot raise it.
		if a.Trust != TrustUntrusted {
			return fmt.Errorf("%w: artifact %q claims trust %q", ErrMalformedResult, a.Name, a.Trust)
		}
	}
	for i, t := range r.ToolRequests {
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("%w: tool request %d has no name", ErrMalformedResult, i)
		}
	}
	return r.rejectCredentials()
}

func (r Result) rejectCredentials() error {
	fields := []string{r.Summary}
	for _, a := range r.Artifacts {
		fields = append(fields, a.Name, a.Ref)
	}
	for _, t := range r.ToolRequests {
		fields = append(fields, t.Name, t.Note)
	}
	for _, f := range fields {
		if LooksLikeCredential(f) {
			return ErrCredential
		}
	}
	return nil
}
