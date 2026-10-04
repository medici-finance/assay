package policy

import (
	"testing"

	"github.com/medici-finance/assay/deskcore/domain"
)

func validDoc() Document {
	return Document{
		Schema:   SchemaV1,
		Revision: "rev-1",
		Grants: []RoleGrant{{
			Role:       "worker",
			Operations: []Operation{"change.open", "change.push"},
			Targets:    []TargetPattern{{ForgeInstanceID: "forge-1", RepositoryID: "1001", ObjectKind: "*"}},
			Hosts:      []string{"forge.example"},
		}},
	}
}

func TestDocumentValidate(t *testing.T) {
	if err := validDoc().Validate(); err != nil {
		t.Fatalf("valid document refused: %v", err)
	}
	cases := map[string]func(*Document){
		"schema":          func(d *Document) { d.Schema = "desk-policy-v2" },
		"revision":        func(d *Document) { d.Revision = "" },
		"bad role":        func(d *Document) { d.Grants[0].Role = "Worker" },
		"dup role":        func(d *Document) { d.Grants = append(d.Grants, d.Grants[0]) },
		"no operations":   func(d *Document) { d.Grants[0].Operations = nil },
		"wildcard op":     func(d *Document) { d.Grants[0].Operations = []Operation{"*"} },
		"dup op":          func(d *Document) { d.Grants[0].Operations = []Operation{"a", "a"} },
		"no targets":      func(d *Document) { d.Grants[0].Targets = nil },
		"empty target":    func(d *Document) { d.Grants[0].Targets[0].RepositoryID = "" },
		"wildcard repo":   func(d *Document) { d.Grants[0].Targets[0].RepositoryID = "*" },
		"wildcard host":   func(d *Document) { d.Grants[0].Hosts = []string{"*.example"} },
		"host with path":  func(d *Document) { d.Grants[0].Hosts = []string{"forge.example/x"} },
		"wildcard forge":  func(d *Document) { d.Grants[0].Targets[0].ForgeInstanceID = "*" },
		"empty host name": func(d *Document) { d.Grants[0].Hosts = []string{""} },
	}
	for name, mutate := range cases {
		d := validDoc()
		d.Grants = append([]RoleGrant(nil), d.Grants...)
		d.Grants[0].Targets = append([]TargetPattern(nil), d.Grants[0].Targets...)
		mutate(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: mutated document validated", name)
		}
	}
}

func TestNewApprovedBindsDigest(t *testing.T) {
	raw := []byte(`{"schema":"desk-policy-v1"}`)
	want := domain.DigestOf(raw)
	a, err := NewApproved(validDoc(), raw, want, "refs/policy@abc")
	if err != nil {
		t.Fatalf("NewApproved: %v", err)
	}
	if _, ok := a.Grant("worker"); !ok {
		t.Error("granted role not found")
	}
	if _, ok := a.Grant("reviewer"); ok {
		t.Error("an ungranted role was found")
	}
	if _, err := NewApproved(validDoc(), append(raw, ' '), want, "refs/policy@abc"); err == nil {
		t.Error("a digest mismatch was admitted")
	}
	if _, err := NewApproved(validDoc(), raw, want, " "); err == nil {
		t.Error("an empty reference was admitted")
	}
	if _, err := NewApproved(validDoc(), raw, "sha256:bad", "refs/policy@abc"); err == nil {
		t.Error("a malformed digest was admitted")
	}
	bad := validDoc()
	bad.Schema = ""
	if _, err := NewApproved(bad, raw, want, "refs/policy@abc"); err == nil {
		t.Error("an invalid document was admitted")
	}
}
