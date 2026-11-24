package contexthandoff

import (
	"bytes"
	"os"
	"testing"

	"github.com/medici-finance/assay/statusgen/workref"
)

// Recorded handoff digests, pinned before the subject bridge: the adapter adds
// no record field, so neither the canonical bytes nor these digests may move.
var recordedHandoffDigests = map[string]string{
	pairedHandoffPath: "sha256:e7b24a59179679baf3a2e9751a264466a9c224ce7272e2f4bc24ed51b5d69f0e",
	draftHandoffPath:  "sha256:b83be3c6142c064a3d6059bf74b4f2bc8be0fd4c0d904cda706bba6adbb71583",
}

func subjectRegistry(t *testing.T, key string) workref.Registry {
	t.Helper()
	brief := workref.Identity{Kind: workref.Brief, Cell: "example", Repo: "tools", Stream: "build", Number: "09"}
	renumbered := workref.Identity{Kind: workref.Brief, Cell: "example", Repo: "tools", Stream: "build", Number: "07"}
	reg, err := workref.NewRegistry(
		[]workref.Root{{Cell: "example", Repo: "tools", Aliases: []string{"tl"}}},
		[]workref.Binding{{Identity: brief, Key: key, Aliases: []workref.Identity{renumbered}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// regression: milestone-support/09 — a typed reference must reach the legacy
// fullId boundary with its exact revision retained beside it, never folded in.
func TestPrefixedHandoffSubject(t *testing.T) {
	// Old records keep their canonical bytes and digests.
	for path, want := range recordedHandoffDigests {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		h, err := DecodeHandoff(raw)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if got := h.Digest(); got != want {
			t.Errorf("%s: recorded digest moved: %s, want %s", path, got, want)
		}
		again, err := DecodeHandoff(h.Canonical())
		if err != nil || !bytes.Equal(again.Canonical(), h.Canonical()) {
			t.Errorf("%s: canonical form is not stable: %v", path, err)
		}
	}

	// The strict legacy grammar is not broadened.
	declaring := &QID{Cell: "example", Repo: "tools", Stream: "build", NN: "01"}
	for _, ref := range []string{"build:b-09", "build:b-09@2", "example:tools:build:b-09@2", "build:09@2", "example:tools:build:m-01@1"} {
		if got, err := ResolveQualifiedID(ref, declaring); err == nil {
			t.Errorf("legacy grammar accepted %q as %s", ref, got)
		}
	}

	const key = "6f0a3c1e-0000-4000-8000-000000000009"
	reg := subjectRegistry(t, key)
	ctx := &workref.Context{Cell: "example", Repo: "tools"}

	q, subject, err := ResolveWorkSubject("build:b-09@2", ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	if q.String() != "example:tools:build:09" || subject.LogicalKey != key || subject.Revision != 2 {
		t.Fatalf("ResolveWorkSubject(build:b-09@2) = %s, %+v", q, subject)
	}
	// An alias root and a renumbered alias resolve to the same subject.
	for _, ref := range []string{"example:tl:build:b-09@2", "build:b-07@2"} {
		_, alias, err := ResolveWorkSubject(ref, ctx, reg)
		if err != nil || !workref.SameSubject(subject, alias) {
			t.Errorf("%s: %+v, %v — want the same subject as build:b-09@2", ref, alias, err)
		}
	}

	// A milestone has no fullId alias and is refused.
	if _, _, err := ResolveWorkSubject("build:m-01@1", ctx, reg); err == nil {
		t.Error("a milestone reference reached the brief fullId boundary")
	}

	// Wrong-subject inputs fail at the comparison itself, with the adapter bypassed.
	_, other, err := ResolveWorkSubject("build:b-10@2", ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	_, older, err := ResolveWorkSubject("build:b-09@1", ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	_, unrevisioned, err := ResolveWorkSubject("build:b-09", ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	rebound := subjectRegistry(t, "6f0a3c1e-0000-4000-8000-00000000000a")
	_, reboundSubject, err := ResolveWorkSubject("build:b-09@2", ctx, rebound)
	if err != nil {
		t.Fatal(err)
	}
	milestone, err := workref.Resolve("build:m-09@2", ctx, reg, workref.TypedMilestone)
	if err != nil {
		t.Fatal(err)
	}
	for name, presented := range map[string]workref.Subject{
		"other brief":                 other,
		"other revision":              older,
		"no revision":                 unrevisioned,
		"same id re-bound to new key": reboundSubject,
		"milestone of the same id":    workref.SubjectOf(milestone),
		"zero value":                  {},
	} {
		if workref.SameSubject(subject, presented) {
			t.Errorf("%s: SameSubject accepted %+v for %+v", name, presented, subject)
		}
	}
	if workref.SameSubject(workref.Subject{}, workref.Subject{}) {
		t.Error("two empty subjects compared equal")
	}
}
