package celllaunch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
)

// SpecRef is an opaque basename under a separately trusted private store root.
// Digest pins the exact bytes approved by the launcher. Neither field supplies
// the trusted root or grants permission to execute those bytes.
type SpecRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func (r SpecRef) Validate() error {
	if !namePattern.MatchString(r.ID) || !fullID.MatchString(r.Digest) {
		return errors.New("invalid opaque spec reference")
	}
	// These basenames address DOS devices even when used under a private root.
	name := strings.ToUpper(r.ID)
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" ||
		len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9' {
		return errors.New("reserved opaque spec reference")
	}
	return nil
}

// BindLaunch checks bytes AFTER a trusted store has established custody. A
// digest supplied by the same untrusted source is not a custody attestation.
func BindLaunch(data []byte, ref SpecRef, permit Permit) (LaunchSpec, error) {
	var zero LaunchSpec
	if err := ref.Validate(); err != nil {
		return zero, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != ref.Digest {
		return zero, errors.New("launch bytes differ from approved reference")
	}
	s, err := ParseLaunch(data)
	if err != nil {
		return zero, err
	}
	if s.LaunchID != ref.ID {
		return zero, errors.New("launch reference identity mismatch")
	}
	if err = permit.Check(s); err != nil {
		return zero, err
	}
	return s, nil
}

type ConsoleIdentity struct {
	Backend   string `json:"backend"`
	Workspace string `json:"workspace"`
	Handle    string `json:"handle"`
	LaunchID  string `json:"launch_id"`
}

type ProcessIdentity struct {
	PID        int    `json:"pid"`
	Created    string `json:"created"` // opaque OS start identity, not a formatted wall clock
	Executable string `json:"executable"`
	Owner      string `json:"owner"` // UID/SID as observed by the platform adapter
	Tree       string `json:"tree"`  // owned process group/job identity, not a process-name match
}

type ContainerIdentity struct {
	Endpoint     string `json:"endpoint"` // validated/frozen local endpoint from the engine adapter
	Engine       string `json:"engine"`   // daemon identity; endpoint alone can name a restarted/replaced daemon
	ID           string `json:"id"`       // full immutable ID, never a name or prefix
	Cell         string `json:"cell"`
	Role         string `json:"role"`
	PolicyDigest string `json:"policy_digest"` // digest of expected image/mounts/isolation plan
}

// A pending/exited record can preserve partial handles for diagnostics and
// component matching. Only ready records support aggregate Match. No record
// authorizes an act.
type SessionRecord struct {
	Schema    string             `json:"schema"`
	LaunchID  string             `json:"launch_id"`
	Cell      string             `json:"cell"`
	Role      string             `json:"role"`
	OS        string             `json:"os"`
	Intent    string             `json:"intent"`
	State     string             `json:"state"` // pending, ready or exited
	Spec      SpecRef            `json:"spec"`
	Console   *ConsoleIdentity   `json:"console,omitempty"`
	Process   *ProcessIdentity   `json:"process,omitempty"`
	Container *ContainerIdentity `json:"container,omitempty"`
}

func (s SessionRecord) Validate() error {
	if s.Schema != SessionSchema || !namePattern.MatchString(s.LaunchID) || !namePattern.MatchString(s.Cell) || !namePattern.MatchString(s.Role) || !validOS(s.OS) {
		return errors.New("invalid session schema or identity")
	}
	if err := s.Spec.Validate(); err != nil {
		return err
	}
	if s.Spec.ID != s.LaunchID {
		return errors.New("session launch reference mismatch")
	}
	if s.Intent != "host" && s.Intent != "linux-container" {
		return errors.New("invalid session intent")
	}
	if s.State != "pending" && s.State != "ready" && s.State != "exited" {
		return errors.New("invalid session state")
	}
	if s.Intent == "host" && s.Container != nil || s.Intent == "linux-container" && s.Process != nil {
		return errors.New("workload identity conflicts with intent")
	}
	if c := s.Console; c != nil {
		if !validCockpit(c.Backend, s.OS) || c.Workspace == "" || c.Handle == "" || c.LaunchID != s.LaunchID || badControl(c.Workspace) || badControl(c.Handle) {
			return errors.New("invalid console identity")
		}
	}
	if p := s.Process; p != nil {
		if p.PID <= 0 || p.Created == "" || p.Owner == "" || p.Tree == "" || !absolute(p.Executable, s.OS) || badControl(p.Created+p.Owner+p.Tree+p.Executable) {
			return errors.New("incomplete process identity")
		}
	}
	if c := s.Container; c != nil {
		if c.Endpoint == "" || c.Engine == "" || badControl(c.Endpoint+c.Engine) || !fullID.MatchString(c.ID) || !fullID.MatchString(c.PolicyDigest) || c.Cell != s.Cell || c.Role != s.Role {
			return errors.New("incomplete container identity")
		}
	}
	if s.State == "ready" && (s.Console == nil || s.Process == nil && s.Container == nil) {
		return errors.New("ready session lacks ownership handles")
	}
	return nil
}

func ParseSession(data []byte) (SessionRecord, error) {
	var s SessionRecord
	if err := decode(data, &s); err != nil {
		return s, err
	}
	return s, s.Validate()
}

type ObservationState string

const (
	Owned   ObservationState = "owned"
	Missing ObservationState = "missing"
	Foreign ObservationState = "foreign"
	Unknown ObservationState = "unknown"
)

// Observation is an aggregate assembled by the coordinator AFTER independent
// resource observations. Individual adapters cannot reconstruct a whole record.
type Observation struct {
	State  ObservationState
	Record SessionRecord
}

func (s SessionRecord) Match(observed Observation) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.State != "ready" || observed.State != Owned {
		return errors.New("session ownership is not established")
	}
	if err := observed.Record.Validate(); err != nil {
		return err
	}
	if !reflect.DeepEqual(s, observed.Record) {
		return errors.New("observed session identity differs")
	}
	return nil
}

type Scope struct{ Cell, Role, LaunchID, OS string }

func (s SessionRecord) Scope() Scope { return Scope{s.Cell, s.Role, s.LaunchID, s.OS} }

// Each adapter observes only its own resource. Scope must be established from
// trusted private ownership state plus the resource's actual launch/ownership
// tag; copying a requested scope onto a foreign handle is not an observation.
type ConsoleObservation struct {
	State    ObservationState
	Scope    Scope
	Identity ConsoleIdentity
}
type ProcessObservation struct {
	State    ObservationState
	Scope    Scope
	Identity ProcessIdentity
}
type ContainerObservation struct {
	State    ObservationState
	Scope    Scope
	Identity ContainerIdentity
}

// Component matches also support partial-start rollback and container recovery
// after console loss. They do not require the other component to be alive or the
// transaction to have reached ready. Absence/uncertainty never means ownership.
func (s SessionRecord) MatchConsole(o ConsoleObservation) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if o.State != Owned || o.Scope != s.Scope() || s.Console == nil || *s.Console != o.Identity {
		return errors.New("console ownership is not established")
	}
	return nil
}
func (s SessionRecord) MatchProcess(o ProcessObservation) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if o.State != Owned || o.Scope != s.Scope() || s.Process == nil || *s.Process != o.Identity {
		return errors.New("process ownership is not established")
	}
	return nil
}
func (s SessionRecord) MatchContainer(o ContainerObservation) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if o.State != Owned || o.Scope != s.Scope() || s.Container == nil || *s.Container != o.Identity {
		return errors.New("container ownership is not established")
	}
	return nil
}

// RunnerRequest is the ONLY payload a console adapter receives. Paths/args/env
// from LaunchSpec are not interpolated into console command text. Runner is a
// separately permitted absolute executable and Ref is resolved by its store.
type RunnerRequest struct {
	Runner string
	Ref    SpecRef
	Cell   string
	Role   string
	OS     string
}

func (r RunnerRequest) Validate() error {
	if err := r.Ref.Validate(); err != nil {
		return err
	}
	if !validOS(r.OS) || !absolute(r.Runner, r.OS) || badControl(r.Runner) || !namePattern.MatchString(r.Cell) || !namePattern.MatchString(r.Role) {
		return errors.New("invalid runner request")
	}
	return nil
}

// Store is constructed with a trusted cell root, not a path from RunnerRequest.
// Read must establish owner-only custody, reject links/reparse aliases and bind
// the opened file identity before returning bytes. Publish atomically creates a
// new immutable reference (never overwrites an existing generation) and syncs
// before exposing it. Filesystem implementations belong to downstream briefs.
type Store interface {
	Read(context.Context, SpecRef) ([]byte, error)
	Publish(context.Context, LaunchSpec, Permit) (SpecRef, error)
}

// Console methods must validate expected ownership AGAIN inside Attach/Close,
// rather than treating an earlier Inspect response as a capability. Close affects
// only the exact terminal, never all terminals in its worktree/workspace.
type Console interface {
	Create(context.Context, RunnerRequest) (ConsoleIdentity, error)
	Inspect(context.Context, Scope, ConsoleIdentity) (ConsoleObservation, error)
	Attach(context.Context, Scope, ConsoleIdentity) error
	Close(context.Context, Scope, ConsoleIdentity) error
}

// Process owns host workloads, not Docker container lifetime. Implementations
// resolve credentials only after independent Permit and private-store checks.
// Inspect must read the OS. Interrupt/Terminate revalidate expected identity and
// tree membership at use; Wait returns the child exit code, not an exec wrapper's.
// No platform implementation or production runner is supplied by this package.
type Process interface {
	Start(context.Context, LaunchSpec, Permit) (ProcessIdentity, error)
	Inspect(context.Context, Scope, ProcessIdentity) (ProcessObservation, error)
	Interrupt(context.Context, Scope, ProcessIdentity) error
	Wait(context.Context, Scope, ProcessIdentity) (int, error)
	Terminate(context.Context, Scope, ProcessIdentity) error
}
