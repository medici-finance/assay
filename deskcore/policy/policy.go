// Package policy holds the types of approved policy data: which operations each role may
// request, on which targets and hosts, and which checks are mandatory.
//
// This is a skeleton. It defines the data and its structural validation only; the predicates
// that evaluate a request against a policy come later. Authority lives here and nowhere else:
// none of these settings is a tunable knob, and nothing in deskcore/config can raise them.
//
// The package is pure: it parses nothing from disk and loads nothing by itself. A policy is
// admitted only as an Approved value whose digest matches the exact bytes it was decoded from,
// and the caller is responsible for reading those bytes from an operator-approved reference
// rather than from an agent's workspace.
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

// Approved is a policy document admitted by the operator: the document, the digest of the exact
// bytes it was decoded from, and the immutable reference those bytes were read at.
type Approved struct {
	Document  Document
	Digest    domain.Digest
	Reference string
}

// NewApproved admits doc as approved when raw (the bytes doc was decoded from) hashes to want.
// It refuses a digest mismatch, an empty reference and a structurally invalid document.
func NewApproved(doc Document, raw []byte, want domain.Digest, reference string) (Approved, error) {
	if err := want.Validate(); err != nil {
		return Approved{}, fmt.Errorf("policy approval: %w", err)
	}
	if got := domain.DigestOf(raw); got != want {
		return Approved{}, fmt.Errorf("policy approval: content digest %s does not match approved %s", got, want)
	}
	if strings.TrimSpace(reference) == "" {
		return Approved{}, errors.New("policy approval: the immutable reference is empty")
	}
	if err := doc.Validate(); err != nil {
		return Approved{}, err
	}
	return Approved{Document: doc, Digest: want, Reference: reference}, nil
}

// Grant returns the grant for role, if the policy has one. A role with no grant may do nothing.
func (a Approved) Grant(role domain.Role) (RoleGrant, bool) {
	for _, g := range a.Document.Grants {
		if g.Role == role {
			return g, true
		}
	}
	return RoleGrant{}, false
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
