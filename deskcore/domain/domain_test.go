package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestOutcomeKindsAreExactlySix(t *testing.T) {
	want := []string{"applied", "denied", "conflict", "pending_external", "could_not_check", "outcome_unknown"}
	got := OutcomeKinds()
	if len(got) != len(want) {
		t.Fatalf("got %d outcome kinds, want %d", len(got), len(want))
	}
	for i, k := range got {
		if string(k) != want[i] {
			t.Errorf("kind %d = %q, want %q", i, k, want[i])
		}
		if p, err := ParseOutcomeKind(want[i]); err != nil || p != k {
			t.Errorf("ParseOutcomeKind(%q) = %q, %v", want[i], p, err)
		}
	}
}

func TestUnknownOutcomeKindRefused(t *testing.T) {
	for _, s := range []string{"", "ok", "failed", "Applied", "unknown"} {
		if _, err := ParseOutcomeKind(s); err == nil {
			t.Errorf("ParseOutcomeKind(%q) accepted an unknown kind", s)
		}
	}
	var r Result
	if err := json.Unmarshal([]byte(`{"kind":"failed"}`), &r); err == nil {
		t.Fatal("decoding a Result with kind \"failed\" succeeded; want a refusal")
	}
}

func TestResultRequiresItsKindField(t *testing.T) {
	bare := map[OutcomeKind]Result{}
	for _, k := range OutcomeKinds() {
		bare[k] = Result{Kind: k}
	}
	for k, r := range bare {
		if err := r.Validate(); err == nil {
			t.Errorf("a bare %s result validated; it must carry its required field", k)
		}
	}
	full := []Result{
		{Kind: Applied, Receipt: &Receipt{OperationID: "op-1"}},
		{Kind: Denied, Reason: "target outside assignment"},
		{Kind: Conflict, CurrentRevision: "abc123"},
		{Kind: PendingExternal, Reference: "check-run/42"},
		{Kind: CouldNotCheck, Cause: "listing incomplete"},
		{Kind: OutcomeUnknown, OperationID: "op-2"},
	}
	for _, r := range full {
		if err := r.Validate(); err != nil {
			t.Errorf("%s: %v", r.Kind, err)
		}
	}
	if err := (Result{Kind: "nope"}).Validate(); err == nil {
		t.Error("a result of an unknown kind validated")
	}
}

func TestDigest(t *testing.T) {
	d := DigestOf([]byte("x"))
	if err := d.Validate(); err != nil {
		t.Fatalf("DigestOf output does not validate: %v", err)
	}
	for _, bad := range []Digest{"", "sha256:", "sha1:" + Digest(strings.Repeat("a", 64)), "sha256:" + Digest(strings.Repeat("A", 64))} {
		if bad.Validate() == nil {
			t.Errorf("digest %q validated", bad)
		}
	}
}

func TestRole(t *testing.T) {
	for _, ok := range []Role{"worker", "pr-review", "verify-2"} {
		if err := ok.Validate(); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []Role{"", "Worker", "worker_desk", "-worker", "worker-", "a--b", Role(strings.Repeat("a", 64))} {
		if bad.Validate() == nil {
			t.Errorf("role %q validated", bad)
		}
	}
}

func validRequest() Request {
	return Request{
		SchemaVersion: RequestSchemaV1,
		RequestID:     "req-1",
		InstanceID:    "inst-1",
		NodeID:        "node-1",
		AttemptID:     "att-1",
		Operation:     "change.open",
		Target:        Target{ForgeInstanceID: "forge-1", RepositoryID: "1001", ObjectKind: "change", ObjectID: "7"},
		Binding:       Binding{PolicyDigest: DigestOf([]byte("policy")), LeaseGeneration: 1},
		PayloadDigest: DigestOf([]byte("payload")),
	}
}

func TestRequestValidate(t *testing.T) {
	if err := validRequest().Validate(); err != nil {
		t.Fatalf("valid request refused: %v", err)
	}
	cases := map[string]func(*Request){
		"schema":      func(q *Request) { q.SchemaVersion = 2 },
		"request_id":  func(q *Request) { q.RequestID = "" },
		"target":      func(q *Request) { q.Target.RepositoryID = "" },
		"policy":      func(q *Request) { q.Binding.PolicyDigest = "" },
		"payload":     func(q *Request) { q.PayloadDigest = "sha256:zz" },
		"lease":       func(q *Request) { q.Binding.LeaseGeneration = 0 },
		"artifact":    func(q *Request) { q.Expected.ArtifactDigest = "nope" },
		"operation":   func(q *Request) { q.Operation = "" },
		"attempt_id":  func(q *Request) { q.AttemptID = "" },
		"pattern_bad": func(q *Request) { q.Binding.PatternDigest = "bad" },
	}
	for name, mutate := range cases {
		q := validRequest()
		mutate(&q)
		if err := q.Validate(); err == nil {
			t.Errorf("%s: mutated request validated", name)
		}
	}
}

func TestFactRoundTrip(t *testing.T) {
	f := Fact{ID: "f1", Source: "s", Kind: "k", Subject: "x", ObservedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Value: json.RawMessage(`{"a":1}`)}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var g Fact
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	if g.ID != f.ID || !g.ObservedAt.Equal(f.ObservedAt) || string(g.Value) != string(f.Value) {
		t.Fatalf("round trip changed the fact: %+v", g)
	}
}
