package main

// mutation_test.go — Verify row 11 / Task item 7. This does not re-run muhar itself (that
// is the reviewer's job, same as every other mutations.json in this tree); it proves the
// corpus carries the two required mutants VERBATIM against the real source (so the edit
// could actually land — the #34 "silent no-op" trap muhar's own tests guard against), and
// that the test named in the brief as the catch (row 6's TestCallbackBadState, row 5's
// TestNoSecretInLogs) already exercises the exact behaviour each mutant would break.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type mutationEntry struct{ Name, File, Old, New string }

func readMutationsJSON(t *testing.T) []mutationEntry {
	t.Helper()
	raw, err := os.ReadFile("mutations.json")
	if err != nil {
		t.Fatalf("read mutations.json: %v", err)
	}
	var spec struct {
		Mutations []mutationEntry `json:"mutations"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse mutations.json: %v", err)
	}
	return spec.Mutations
}

// TestMutationCorpusHasCallbackStateDrop — the first required mutant (Task 7a) is present,
// and its `old` text is found VERBATIM in server.go — a mutation whose anchor text does not
// exist in the real file cannot land at all (the exact "silent no-op" failure mode muhar's
// own harness refuses to read as a catch).
func TestMutationCorpusHasCallbackStateDrop(t *testing.T) {
	entries := readMutationsJSON(t)
	var m *mutationEntry
	for i := range entries {
		if strings.Contains(entries[i].File, "server.go") && strings.Contains(entries[i].Old, "if row == nil {") {
			m = &entries[i]
		}
	}
	if m == nil {
		t.Fatal("mutations.json lacks the state-check-drop mutant on cmd/deskapps/server.go")
	}
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), m.Old) {
		t.Fatalf("mutation's old text is not present verbatim in server.go — it cannot land:\n%s", m.Old)
	}

	// The guard this mutant reddens: TestCallbackBadState (Verify row 6) sends a callback
	// with a state matching no pending row and requires 403 + no conversion attempted. With
	// the state check dropped, `row` is nil past this point and every field access on it
	// panics — net/http recovers the panic into a 500, which is neither 403 nor "no
	// conversion attempted" (the mutated code reaches convertCodeFn before it would panic on
	// row.App), so TestCallbackBadState fails under the mutation. That is exercised directly
	// by TestCallbackBadState in this package; this test only proves the mutant is real.
}

// TestMutationCorpusHasResponseBodyLog — the second required mutant (Task 7b) is present,
// and its `old` text is found verbatim in convert.go.
func TestMutationCorpusHasResponseBodyLog(t *testing.T) {
	entries := readMutationsJSON(t)
	var m *mutationEntry
	for i := range entries {
		if strings.Contains(entries[i].File, "convert.go") && strings.Contains(entries[i].New, "fmt.Println") {
			m = &entries[i]
		}
	}
	if m == nil {
		t.Fatal("mutations.json lacks the response-body-log mutant on cmd/deskapps/convert.go")
	}
	src, err := os.ReadFile("convert.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), m.Old) {
		t.Fatalf("mutation's old text is not present verbatim in convert.go — it cannot land:\n%s", m.Old)
	}
	if !strings.Contains(m.New, m.Old) {
		t.Fatal("mutation's new text does not build on old (not a real edit)")
	}

	// The guard this mutant reddens: TestNoSecretInLogs (Verify row 5) drives a real
	// conversion carrying fake pem/client-secret/webhook-secret and asserts stdout AND the
	// audit log never carry that material. Logging the raw conversion response body would
	// print exactly those secrets to stdout, which TestNoSecretInLogs in this package
	// directly checks for.
}
