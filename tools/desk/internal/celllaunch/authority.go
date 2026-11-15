package celllaunch

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
)

// AuthorityKind is a closed component kind, not a new authenticated App role.
type AuthorityKind string

const (
	AuthorityProducer AuthorityKind = "producer"
	AuthorityQuery    AuthorityKind = "query"
	AuthorityCaller   AuthorityKind = "caller"
	AuthoritySibling  AuthorityKind = "sibling"
)

// AuthorityLaunch describes a noninteractive component under an independently
// approved closed container profile. It is intent/consistency, not authentication.
// Container.Role denotes the fixed component kind; Role is existing caller policy.
type AuthorityLaunch struct {
	Schema, LaunchID, Cell, Role string
	Kind                         AuthorityKind
	Container                    ContainerIdentity
}

func (s AuthorityLaunch) Validate() error {
	c := s.Container
	if s.Schema != "cell-authority-launch-v1" || !namePattern.MatchString(s.LaunchID) || !namePattern.MatchString(s.Cell) || (s.Kind != AuthorityProducer && s.Kind != AuthorityQuery && s.Kind != AuthorityCaller && s.Kind != AuthoritySibling) || (s.Kind == AuthorityProducer && s.Role != "") || (s.Kind != AuthorityProducer && !namePattern.MatchString(s.Role)) || !strings.HasPrefix(c.Endpoint, "unix:///") || !filepath.IsAbs(strings.TrimPrefix(c.Endpoint, "unix://")) || badControl(c.Endpoint+c.Engine) || c.Engine == "" || !fullID.MatchString(c.ID) || !fullID.MatchString(c.PolicyDigest) || c.Cell != s.Cell || c.Role != string(s.Kind) {
		return errors.New("invalid closed authority component intent")
	}
	return nil
}

type AuthorityPermit struct{ approved string }

func NewAuthorityPermit(expected AuthorityLaunch) (AuthorityPermit, error) {
	if expected.Validate() != nil {
		return AuthorityPermit{}, errors.New("invalid authority permit")
	}
	b, e := json.Marshal(expected)
	if e != nil {
		return AuthorityPermit{}, errors.New("invalid authority permit")
	}
	return AuthorityPermit{approved: string(b)}, nil
}
func (p AuthorityPermit) Check(s AuthorityLaunch) error {
	if s.Validate() != nil {
		return errors.New("invalid authority intent")
	}
	b, e := json.Marshal(s)
	if e != nil || p.approved == "" || string(b) != p.approved {
		return errors.New("authority component differs from independent permit")
	}
	return nil
}

// Match compares a fresh lower-adapter observation; a decoded record or Permit
// alone is insufficient. The lower adapter must actually inspect the owned engine.
func (s AuthorityLaunch) Match(o ContainerObservation) error {
	if s.Validate() != nil || o.State != Owned || o.Scope != (Scope{Cell: s.Cell, Role: string(s.Kind), LaunchID: s.LaunchID, OS: "linux"}) || !reflect.DeepEqual(s.Container, o.Identity) {
		return errors.New("current authority component ownership is not established")
	}
	return nil
}
