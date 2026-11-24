package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/workref"
)

func refusalOf(t *testing.T, err error) Refusal {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("not a refusal: %v", err)
	}
	return *r
}

// milestone-support/09: a normalized legacy identity alone must
// never erase the revision the Evidence was requested for.
// regression: #7088
func TestPrefixedVerifySubject(t *testing.T) {
	const key = "4b7d2e10-0000-4000-8000-000000000009"
	brief := workref.Identity{Kind: workref.Brief, Cell: "example", Repo: "widgets", Stream: "build", Number: "09"}
	reg, err := workref.NewRegistry([]workref.Root{{Cell: "example", Repo: "widgets"}}, []workref.Binding{{Identity: brief, Key: key}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &workref.Context{Cell: "example", Repo: "widgets"}

	// NewWorkAt returns the existing WorkIdentity and the exact revision.
	w, subject, err := NewWorkAt("build:b-09@3", ctx, reg, "inst-example-09", "verify", 1)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := NewWork("example:widgets:build:09", "", "inst-example-09", "verify", 1)
	if err != nil {
		t.Fatal(err)
	}
	if w != legacy {
		t.Errorf("NewWorkAt identity %+v differs from the legacy identity %+v", w, legacy)
	}
	if subject.LogicalKey != key || subject.Revision != 3 {
		t.Fatalf("NewWorkAt subject = %+v, want %s revision 3", subject, key)
	}
	if _, _, err := NewWorkAt("build:m-01@1", ctx, reg, "inst-example-09", "verify", 1); err == nil {
		t.Error("a milestone reference became a work identity")
	}

	// Approved side: the brief at the judged base, revision 3.
	approved, err := workref.BriefSubject("example:widgets:build:09", EvidenceRevision("brief-v2", 3), ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSubject(approved, subject); err != nil {
		t.Errorf("the same subject was refused: %v", err)
	}
	_, older, err := NewWorkAt("build:b-09@2", ctx, reg, "inst-example-09", "verify", 1)
	if err != nil {
		t.Fatal(err)
	}
	if r := refusalOf(t, CheckSubject(approved, older)); r.Code != "work-subject" || r.State != "checked-failed" {
		t.Errorf("revision-2 Evidence for a revision-3 approval: %+v, want checked-failed work-subject", r)
	}
	_, bare, err := NewWorkAt("example:widgets:build:09", nil, reg, "inst-example-09", "verify", 1)
	if err != nil {
		t.Fatal(err)
	}
	if r := refusalOf(t, CheckSubject(approved, bare)); r.Code != "work-subject" || r.State != "could-not-check" {
		t.Errorf("legacy id with no revision: %+v, want could-not-check work-subject", r)
	}
	_, other, err := NewWorkAt("build:b-10@3", ctx, reg, "inst-example-09", "verify", 1)
	if err != nil {
		t.Fatal(err)
	}
	if r := refusalOf(t, CheckSubject(approved, other)); r.State != "checked-failed" {
		t.Errorf("another brief's Evidence: %+v, want checked-failed", r)
	}

	// An omitted version is 1 for brief-v2 and unrevisioned for brief-v1.
	_, one, err := NewWorkAt("build:b-09@1", ctx, reg, "inst-example-09", "verify", 1)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := workref.BriefSubject("example:widgets:build:09", EvidenceRevision("brief-v2", 0), ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSubject(v2, one); err != nil {
		t.Errorf("brief-v2 with no explicit version must match @1: %v", err)
	}
	if r := refusalOf(t, CheckSubject(v2, bare)); r.State != "could-not-check" {
		t.Errorf("brief-v2 with no explicit version must hold an unrevisioned reference: %+v", r)
	}
	v1, err := workref.BriefSubject("example:widgets:build:09", EvidenceRevision("brief-v1", 0), ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Revision != 0 {
		t.Errorf("brief-v1 gained a revision: %+v", v1)
	}
	if err := CheckSubject(v1, bare); err != nil {
		t.Errorf("brief-v1 stays unrevisioned and matches the legacy id: %v", err)
	}

	// A matching subject never stands in for the candidate check.
	cand := Candidate{Repository: "example/widgets", BaseCommit: strings.Repeat("a", 40), BaseTree: strings.Repeat("b", 40),
		Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40), ManifestDigest: "sha256:" + strings.Repeat("e", 64)}
	binding, err := BindAcceptance(cand, 1, "sha256:"+strings.Repeat("f", 64), []string{"coverage", "eligibility"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSubjectEvidence(binding, cand, approved, subject); err != nil {
		t.Errorf("matching subject and candidate refused: %v", err)
	}
	stale := cand
	stale.Commit = strings.Repeat("9", 40)
	if r := refusalOf(t, ValidateSubjectEvidence(binding, stale, approved, subject)); r.Code != "candidate-mismatch" {
		t.Errorf("stale candidate with a matching subject: %+v, want candidate-mismatch", r)
	}
	if r := refusalOf(t, ValidateSubjectEvidence(binding, cand, approved, older)); r.Code != "work-subject" {
		t.Errorf("substituted subject with the bound candidate: %+v, want work-subject", r)
	}
}
