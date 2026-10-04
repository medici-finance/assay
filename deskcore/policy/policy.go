// Package policy holds the types of approved policy data: which operations each role may
// request, on which targets and hosts, and which checks are mandatory.
//
// This is a skeleton. It defines the data and its structural validation only; the predicates
// that evaluate a request against a policy come later. Authority lives here and nowhere else:
// none of these settings is a tunable knob, and nothing in deskcore/config can raise them.
//
// The package is pure: it reads nothing from disk and loads nothing by itself. A policy is
// admitted only as an Approved value, which NewApproved builds by decoding the very bytes whose
// digest it checked; no other way to build one exists outside this package. The caller is
// responsible for reading those bytes from an operator-approved reference rather than from an
// agent's workspace.
package policy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/deskcore/domain"
)

// SchemaV1 is the schema value a policy document carries.
const SchemaV1 = "desk-policy-v1"

// Operation names one operation a role may request, for example "change.open".
type Operation string

// TargetPattern selects the targets a grant applies to. An empty field is not a wildcard:
// every field must be set, and "*" is the explicit wildcard for ObjectKind only.
type TargetPattern struct {
	ForgeInstanceID string `json:"forge_instance_id"`
	RepositoryID    string `json:"repository_id"`
	ObjectKind      string `json:"object_kind"`
}

// RoleGrant is what one role may do.
type RoleGrant struct {
	Role           domain.Role     `json:"role"`
	Operations     []Operation     `json:"operations"`
	Targets        []TargetPattern `json:"targets"`
	Hosts          []string        `json:"hosts"`
	RequiredChecks []string        `json:"required_checks"`
}

// Document is one policy document.
type Document struct {
	Schema   string          `json:"schema"`
	Revision domain.Revision `json:"revision"`
	Grants   []RoleGrant     `json:"grants"`
}

// RequiredKeys lists the keys a policy document's JSON object must carry (see
// domain.DecodeStrict).
func (Document) RequiredKeys() []string { return []string{"schema", "revision", "grants"} }

// RequiredKeys lists the keys a grant's JSON object must carry.
func (RoleGrant) RequiredKeys() []string {
	return []string{"role", "operations", "targets", "hosts", "required_checks"}
}

// RequiredKeys lists the keys a target pattern's JSON object must carry.
func (TargetPattern) RequiredKeys() []string {
	return []string{"forge_instance_id", "repository_id", "object_kind"}
}

// Approved is a policy document admitted by the operator: the document decoded from the exact
// bytes whose digest was approved, that digest, and the immutable reference the bytes were read
// at. Its fields are unexported, so NewApproved is the only way to build one; the zero value
// grants nothing.
type Approved struct {
	doc       Document
	digest    domain.Digest
	reference string
}

// NewApproved admits the policy document in raw when raw hashes to want. It decodes raw itself,
// strictly (domain.DecodeStrict), so the admitted document is always the one the approved bytes
// hold. It refuses a malformed digest, a digest mismatch, an empty reference, bytes that do not
// decode strictly, and a structurally invalid document.
func NewApproved(raw []byte, want domain.Digest, reference string) (Approved, error) {
	if err := want.Validate(); err != nil {
		return Approved{}, fmt.Errorf("policy approval: %w", err)
	}
	if got := domain.DigestOf(raw); got != want {
		return Approved{}, fmt.Errorf("policy approval: content digest %s does not match approved %s", got, want)
	}
	if strings.TrimSpace(reference) == "" {
		return Approved{}, errors.New("policy approval: the immutable reference is empty")
	}
	var doc Document
	if err := domain.DecodeStrict(raw, &doc); err != nil {
		return Approved{}, fmt.Errorf("policy approval: %w", err)
	}
	if err := doc.Validate(); err != nil {
		return Approved{}, err
	}
	return Approved{doc: doc, digest: want, reference: reference}, nil
}

// Document returns a copy of the approved document; changing the copy changes nothing here.
func (a Approved) Document() Document {
	d := a.doc
	d.Grants = make([]RoleGrant, len(a.doc.Grants))
	for i, g := range a.doc.Grants {
		d.Grants[i] = g.clone()
	}
	if a.doc.Grants == nil {
		d.Grants = nil
	}
	return d
}

// Digest returns the digest of the approved bytes.
func (a Approved) Digest() domain.Digest { return a.digest }

// Reference returns the immutable reference the approved bytes were read at.
func (a Approved) Reference() string { return a.reference }

// Grant returns a copy of the grant for role, if the policy has one. A role with no grant may do
// nothing.
func (a Approved) Grant(role domain.Role) (RoleGrant, bool) {
	for _, g := range a.doc.Grants {
		if g.Role == role {
			return g.clone(), true
		}
	}
	return RoleGrant{}, false
}

func (g RoleGrant) clone() RoleGrant {
	g.Operations = append([]Operation(nil), g.Operations...)
	g.Targets = append([]TargetPattern(nil), g.Targets...)
	g.Hosts = append([]string(nil), g.Hosts...)
	g.RequiredChecks = append([]string(nil), g.RequiredChecks...)
	return g
}

// Validate reports every structural defect in d.
func (d Document) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf("policy: "+format, a...)) }
	if d.Schema != SchemaV1 {
		add("schema %q is not %q", d.Schema, SchemaV1)
	}
	if d.Revision == "" {
		add("revision is empty")
	}
	seen := map[domain.Role]bool{}
	for i, g := range d.Grants {
		if err := g.Role.Validate(); err != nil {
			add("grants[%d]: %v", i, err)
		}
		if seen[g.Role] {
			add("grants[%d]: role %q is granted twice", i, g.Role)
		}
		seen[g.Role] = true
		if len(g.Operations) == 0 {
			add("role %q: no operations; omit the grant instead", g.Role)
		}
		ops := map[Operation]bool{}
		for _, op := range g.Operations {
			if op == "" || strings.ContainsAny(string(op), " *") {
				add("role %q: operation %q must be a literal name", g.Role, op)
			}
			if ops[op] {
				add("role %q: operation %q is listed twice", g.Role, op)
			}
			ops[op] = true
		}
		if len(g.Targets) == 0 {
			add("role %q: no targets", g.Role)
		}
		for _, tp := range g.Targets {
			if tp.ForgeInstanceID == "" || tp.RepositoryID == "" || tp.ObjectKind == "" {
				add("role %q: target %+v must set every field", g.Role, tp)
			}
			if tp.ForgeInstanceID == "*" || tp.RepositoryID == "*" {
				add("role %q: target %+v: only object_kind may be the wildcard", g.Role, tp)
			}
		}
		for _, h := range g.Hosts {
			if h == "" || strings.ContainsAny(h, "/*") {
				add("role %q: host %q must be a literal host name", g.Role, h)
			}
		}
	}
	return errors.Join(errs...)
}
