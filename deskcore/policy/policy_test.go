package policy

import (
	"reflect"
	"strings"
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

const approvedDoc = `{"schema":"desk-policy-v1","revision":"rev-1","grants":[` +
	`{"role":"worker","operations":["change.open","change.push"],` +
	`"targets":[{"forge_instance_id":"forge-1","repository_id":"1001","object_kind":"*"}],` +
	`"hosts":["forge.example"],"required_checks":[]}]}`

func TestNewApprovedBindsDigest(t *testing.T) {
	raw := []byte(approvedDoc)
	want := domain.DigestOf(raw)
	a, err := NewApproved(raw, want, "refs/policy@abc")
	if err != nil {
		t.Fatalf("NewApproved: %v", err)
	}
	if a.Digest() != want || a.Reference() != "refs/policy@abc" || a.Document().Revision != "rev-1" {
		t.Errorf("approved value does not carry its inputs: %s %s %+v", a.Digest(), a.Reference(), a.Document())
	}
	if _, ok := a.Grant("worker"); !ok {
		t.Error("granted role not found")
	}
	if _, ok := a.Grant("reviewer"); ok {
		t.Error("an ungranted role was found")
	}
	if _, err := NewApproved(append(raw, ' '), want, "refs/policy@abc"); err == nil {
		t.Error("a digest mismatch was admitted")
	}
	if _, err := NewApproved(raw, want, " "); err == nil {
		t.Error("an empty reference was admitted")
	}
	if _, err := NewApproved(raw, "sha256:bad", "refs/policy@abc"); err == nil {
		t.Error("a malformed digest was admitted")
	}
	bad := []byte(strings.Replace(approvedDoc, `"schema":"desk-policy-v1"`, `"schema":""`, 1))
	if _, err := NewApproved(bad, domain.DigestOf(bad), "refs/policy@abc"); err == nil {
		t.Error("an invalid document was admitted")
	}
}

// The admitted document is always the one the approved bytes hold: bytes that grant nothing
// grant nothing, whatever else the caller has to hand.
func TestApprovedBytesAreWhatGrants(t *testing.T) {
	raw := []byte(`{"schema":"desk-policy-v1","revision":"rev-2","grants":[]}`)
	a, err := NewApproved(raw, domain.DigestOf(raw), "refs/policy@def")
	if err != nil {
		t.Fatalf("NewApproved: %v", err)
	}
	if g, ok := a.Grant("worker"); ok {
		t.Errorf("approved bytes grant nothing, yet worker was granted %+v", g)
	}
	var zero Approved
	if _, ok := zero.Grant("worker"); ok {
		t.Error("the zero Approved granted a role")
	}
}

// Approved has no exported field, so outside this package NewApproved is the only way to build
// one, and the returned document is a copy.
func TestApprovedIsUnforgeable(t *testing.T) {
	rt := reflect.TypeFor[Approved]()
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).IsExported() {
			t.Errorf("Approved.%s is exported", rt.Field(i).Name)
		}
	}
	raw := []byte(approvedDoc)
	a, err := NewApproved(raw, domain.DigestOf(raw), "refs/policy@abc")
	if err != nil {
		t.Fatalf("NewApproved: %v", err)
	}
	d := a.Document()
	d.Grants[0].Role = "reviewer"
	d.Grants[0].Operations[0] = "change.merge"
	g, _ := a.Grant("worker")
	g.Operations[1] = "change.merge"
	again, ok := a.Grant("worker")
	if !ok || again.Operations[0] != "change.open" || again.Operations[1] != "change.push" {
		t.Errorf("changing a returned copy changed the approval: %+v", again)
	}
}

// A policy document is decoded strictly: a key that differs only in case, a key given twice, or
// a missing required key is refused even when the digest matches.
func TestNewApprovedDecodesStrictly(t *testing.T) {
	cases := map[string]string{
		"case variant": strings.Replace(approvedDoc, `"grants":[`, `"Grants":[`, 1),
		"duplicate":    strings.Replace(approvedDoc, `"role":"worker"`, `"role":"reader","role":"worker"`, 1),
		"missing key":  strings.Replace(approvedDoc, `,"required_checks":[]`, ``, 1),
		"unknown key":  strings.Replace(approvedDoc, `"revision":"rev-1"`, `"revision":"rev-1","admin":true`, 1),
	}
	for name, doc := range cases {
		if doc == approvedDoc {
			t.Fatalf("%s: the fixture edit did not apply", name)
		}
		raw := []byte(doc)
		if _, err := NewApproved(raw, domain.DigestOf(raw), "refs/policy@abc"); err == nil {
			t.Errorf("%s: NewApproved admitted %s", name, doc)
		}
	}
}
