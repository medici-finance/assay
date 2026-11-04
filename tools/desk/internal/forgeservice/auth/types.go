// Package auth implements the local launcher caller-authority protocol.
// Its operator controller belongs outside query and untrusted-role processes.
package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

const Schema = "forge-local-authority-v1"
const Operation = "mirror-query"
const Profile = "linux-named-volume-v1"

// Absolute protocol allocation ceilings; actual budgets remain explicit inputs.
const maxRegistryEntries = 1024
const maxChannelConnections = 64

var (
	ErrConfig   = errors.New("local authority: invalid trusted configuration")
	ErrHeld     = errors.New("local authority: current authority unavailable")
	ErrConflict = errors.New("local authority: control version conflict")
)

// ProducerBinding is independently observed by the protected launcher. An
// endpoint, record or binding value alone grants no signing/current authority.
type ProducerBinding struct {
	Engine, ContainerID, PlanDigest, Cell         string
	PrivateVolume, CurrentVolume, BootstrapVolume string
}

func (b ProducerBinding) valid() bool {
	return text(b.Engine) && hexID(b.ContainerID) && hexID(b.PlanDigest) && text(b.Cell) && name(b.PrivateVolume) && name(b.CurrentVolume) && name(b.BootstrapVolume) && b.PrivateVolume != b.CurrentVolume && b.PrivateVolume != b.BootstrapVolume && b.CurrentVolume != b.BootstrapVolume
}

// GrantScope is trusted operator policy. The query never selects its role,
// represented actor, source, epoch, policy limits or admitted installation.
type GrantScope struct {
	Scope                                      mirror.Scope
	Caller                                     refresh.Caller
	Actor, Role, Cell, SourceID, CompositionID string
}

func (s GrantScope) valid() bool {
	return s.Scope.Validate() == nil && s.Caller.Limits.Validate() == nil && s.Caller.AdmissionID == s.Scope.AdmissionID && text(s.Caller.ID) && text(s.Caller.AccessEpoch) && text(s.Caller.ConfigEpoch) && text(s.Caller.PolicyClass) && text(s.Caller.PolicyEpoch) && text(s.Actor) && text(s.Role) && text(s.Cell) && text(s.SourceID) && text(s.CompositionID)
}

// LaunchPolicy comes from protected launcher policy, not a decoded query.
// Expected describes independently approved intent, never live ownership.
type LaunchPolicy struct {
	ID, GrantVolume string
	Scope           GrantScope
	Launch          celllaunch.AuthorityLaunch
}

type ProducerConfig struct {
	Schema, Profile, Issuer, Audience                                         string
	MaxGrantLease, ClockSkew, CurrentReadLease, MaxRevocationBound, IOTimeout time.Duration
	MaxFrameBytes, MaxGrants, MaxConnections                                  int
	Launches                                                                  []LaunchPolicy
}

// CheckConfig is pure and runs before key, listener, filesystem or engine I/O.
func CheckConfig(c ProducerConfig) error {
	if c.Schema != Schema || c.Profile != Profile || !text(c.Issuer) || !text(c.Audience) || c.MaxGrantLease <= 0 || c.ClockSkew < 0 || c.ClockSkew > c.MaxGrantLease || c.CurrentReadLease <= 0 || c.MaxRevocationBound < c.CurrentReadLease || c.IOTimeout <= 0 || c.MaxFrameBytes <= 0 || c.MaxFrameBytes > celllaunch.MaxRecordBytes || c.MaxGrants <= 0 || c.MaxGrants > maxRegistryEntries || c.MaxConnections <= 0 || c.MaxConnections > maxChannelConnections || len(c.Launches) == 0 || len(c.Launches) > c.MaxGrants {
		return ErrConfig
	}
	seen, volumes := map[string]bool{}, map[string]bool{}
	for _, p := range c.Launches {
		if !name(p.ID) || !name(p.GrantVolume) || seen[p.ID] || volumes[p.GrantVolume] || !p.Scope.valid() || p.Launch.Validate() != nil || p.Launch.Kind != celllaunch.AuthorityCaller || p.Launch.LaunchID != p.ID || p.Launch.Cell != p.Scope.Cell || p.Launch.Role != p.Scope.Role {
			return ErrConfig
		}
		seen[p.ID], volumes[p.GrantVolume] = true, true
	}
	return nil
}

// ProducerResources are already-bound, disjoint protected roots. The launcher
// qualifies their namespace/mount views; root permissions alone are not custody.
type ProducerResources struct {
	Private, Current, Bootstrap *os.Root
	Grants                      map[string]*os.Root
	Binding                     ProducerBinding
}

type Bootstrap struct {
	Schema                        string
	Binding                       ProducerBinding
	Instance, Incarnation, KeyRef string
	PublicKey                     []byte
}

func (b Bootstrap) valid() bool {
	return b.Schema == Schema && b.Binding.valid() && hexID(b.Instance) && hexID(b.Incarnation) && hexID(b.KeyRef) && len(b.PublicKey) == ed25519.PublicKeySize
}

type GrantClaims struct {
	Schema, Operation, Issuer, Audience                            string
	Binding                                                        ProducerBinding
	Instance, Incarnation, KeyRef, GrantID, LaunchID, LaunchDigest string
	Generation                                                     uint64
	Scope                                                          GrantScope
	IssuedAt, ExpiresAt                                            time.Time
}

// SignedGrant carries authentication bytes; never log it or expose it in an
// observation, cache key, telemetry export or ordinary transport error.
type SignedGrant struct{ Payload, Signature []byte }

type CurrentProbe struct {
	Schema    string
	Instance  string
	Challenge []byte
	Grant     SignedGrant
}

type CurrentProof struct {
	Schema, Instance, GrantDigest string
	Challenge                     []byte
	Claims                        GrantClaims
	Version                       uint64
	Remaining                     time.Duration
	CapturedAt                    time.Time
}

type SignedProof struct{ Payload, Signature []byte }

type ControlOperation string

const (
	Initialize ControlOperation = "initialize-fresh"
	Register   ControlOperation = "register-launch"
	Revoke     ControlOperation = "revoke-grant"
	Advance    ControlOperation = "advance-epochs"
	Rotate     ControlOperation = "rotate-fresh"
	Stop       ControlOperation = "stop"
)

// LaunchBinding records independently permitted intent and a fresh actual
// resource observation. It is not caller authentication by itself.
type LaunchBinding struct {
	ID          string
	Permit      celllaunch.AuthorityPermit
	Observation celllaunch.ContainerObservation
}

// ControlCommand is operator-only and is never accepted on either data channel.
type ControlCommand struct {
	Operation       ControlOperation
	ExpectedVersion uint64
	Launch          *LaunchBinding
	GrantID         string
	Lease           time.Duration
	Policy          *GrantScope
}
type ControlReceipt struct {
	Version     uint64
	Ready       bool
	TupleDigest string
}
type GrantChannel struct{ LaunchID string }
type CurrentChannel struct{}

type CurrentConfig struct {
	Path          string
	Bootstrap     Bootstrap
	MaxFrameBytes int
	IOTimeout     time.Duration
}
type VerifierConfig struct {
	Schema, Profile, Issuer, Audience                              string
	Bootstrap                                                      Bootstrap
	Policy                                                         GrantScope
	Launch                                                         celllaunch.AuthorityLaunch
	MaxGrantLease, ClockSkew, CurrentReadLease, MaxRevocationBound time.Duration
	MaxFrameBytes                                                  int
}

type readLease struct {
	original context.Context
	until    time.Time // process-local time.Now value; never serialized/UTC-stripped
	expires  time.Time
	used     atomic.Bool
}

// VerifiedRead is detached identity plus a single-release request lease. Its
// private fields prevent an untrusted wire record from manufacturing authority.
type VerifiedRead struct {
	owner  *Verifier
	policy GrantScope
	lease  *readLease
}

func (r VerifiedRead) Caller() refresh.Caller { return r.policy.Caller }
func (r VerifiedRead) Actor() string          { return r.policy.Actor }
func (r VerifiedRead) Role() string           { return r.policy.Role }

func text(s string) bool {
	if s == "" || len(s) > 512 || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func name(s string) bool {
	if !text(s) || len(s) > 96 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.", r)) {
			return false
		}
	}
	return s != "." && s != ".."
}
func hexID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'f' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func same(a, b any) bool { return reflect.DeepEqual(a, b) }

func clockPresent(c refresh.Clock) bool {
	if c == nil {
		return false
	}
	v := reflect.ValueOf(c)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	}
	return true
}
