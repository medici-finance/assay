package cellop

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

// confirmPlan is the one-action drain plan of testPlan under a fixed id, so its digest and its
// confirm word are the same on every run.
func confirmPlan(t *testing.T, id, value string) Plan {
	t.Helper()
	p, err := NewPlan(id, "drain", []string{"example"}, []Action{{
		ID: "drain-example", Cell: "example", Summary: "drain example",
		Preconditions: []Precondition{{Key: "example/state", Value: value}},
	}}, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The confirm code is one lowercase word from the id lists, the same for the same plan, never a
// word of the plan's own id, and a plan carrying any other word fails its check (spec/cell-op-v1.md).
// regression: #6605
func TestConfirmIsOneWord(t *testing.T) {
	words := append(append([]string{}, idAdjectives[:]...), idNouns[:]...)
	ids := []string{"amber-falcon", "calm-heron", "brisk-otter", "steady-wren", "4f1c2a9e7b3d5c8e0a6f2b1d"}
	for range 60 {
		ids = append(ids, newID(t))
	}
	for _, id := range ids {
		p := confirmPlan(t, id, "running")
		if !slices.Contains(words, p.ConfirmCode) {
			t.Fatalf("plan %s: confirm %q is not one word of the compiled lists", id, p.ConfirmCode)
		}
		if slices.Contains(strings.Split(id, "-"), p.ConfirmCode) {
			t.Fatalf("plan %s: confirm %q is a word of its own id", id, p.ConfirmCode)
		}
		if again := confirmPlan(t, id, "running"); again.Digest != p.Digest || again.ConfirmCode != p.ConfirmCode {
			t.Fatalf("plan %s: the same plan sealed twice gave %q then %q", id, p.ConfirmCode, again.ConfirmCode)
		}
		q := p
		q.ConfirmCode = words[(slices.Index(words, p.ConfirmCode)+1)%len(words)]
		if err := q.Verify(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("plan %s with confirm %q verified: %v", id, q.ConfirmCode, err)
		}
	}
	// A changed digest selects a different word for these pinned plans.
	a, b := confirmPlan(t, "amber-falcon", "running"), confirmPlan(t, "amber-falcon", "paused")
	if a.Digest == b.Digest || a.ConfirmCode == b.ConfirmCode {
		t.Fatalf("a changed plan kept its confirm: %s %q, %s %q", a.Digest, a.ConfirmCode, b.Digest, b.ConfirmCode)
	}
}

// A plan stored with the retired 8-hex confirm is refused plan-invalid, with a reason that names
// the format change, and nothing runs (spec/cell-op-v1.md section 3).
// regression: #6605
func TestRetiredHexConfirm(t *testing.T) {
	s, w, c := StoreFor(t.TempDir()), newWorld(), &clock{now: t0}
	p := testPlan(t, "example", "running")
	p.ConfirmCode = p.Digest[:8]
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path(p.ID), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(p.ID); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "retired 8-hex") {
		t.Fatalf("a retired hex confirm loaded: %v", err)
	}
	w.set("example/state", "running")
	r := session(s, w, c, "example", "drain").Apply(p.ID)
	expect(t, r, Refused, ReasonPlanInvalid)
	if r.Detail == nil || !strings.Contains(*r.Detail, "retired 8-hex") {
		t.Fatalf("the refusal does not name the format change: %v", r.Detail)
	}
	if consumed(t, s, p.ID) || w.runs("drain-example") != 0 {
		t.Fatal("a refused retired plan was consumed or ran")
	}
}

// The typed word is compared without case or surrounding space; any other word is a mismatch.
// regression: #6605
func TestApplyConfirmWordCase(t *testing.T) {
	s, w, c := StoreFor(t.TempDir()), newWorld(), &clock{now: t0}
	w.set("example/state", "running")
	p := confirmPlan(t, "amber-falcon", "running")
	save(t, s, p)
	sess := session(s, w, c, "example", "drain")
	sess.Confirm = func(p Plan, _ []Plan) (string, error) { return "falcon", nil }
	expect(t, sess.Apply(p.ID), Refused, ReasonConfirmMismatch)
	sess.Confirm = func(p Plan, _ []Plan) (string, error) { return " " + strings.ToUpper(p.ConfirmCode) + " ", nil }
	expect(t, sess.Apply(p.ID), OK, "")
}
