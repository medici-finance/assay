package issueflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleHandoff() (Handoff, HandoffFacts) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	h := Handoff{Schema: HandoffSchema, Source: HandoffSource{Repo: "example-org/tracker", Brief: "alpha/01", SourceVersion: "1", SourceRevision: "source", ReceiptID: "receipt", CaseID: "repair/existing", Rows: []int{1, 2}, BlockerRef: "#12"}, Disposition: Repair, DecidingRole: "intake-desk", Successor: "alpha/02", ObservedAt: now, FreshUntil: now.Add(time.Hour)}
	return h, HandoffFacts{Source: h.Source, Complete: true, Now: now}
}

// regression: class verification-handoff
func TestHandoffPolicyParity(t *testing.T) {
	// Adopt the existing contract's closed vocabulary, not an untested third list.
	rawRoles, err := os.ReadFile(filepath.Join(repoRoot(t), "spec/intake-exit-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	roleSection := strings.SplitN(string(rawRoles), "## Closed role set", 2)
	if len(roleSection) != 2 {
		t.Fatal("missing role contract")
	}
	roleTable := strings.SplitN(roleSection[1], "## Typed refs", 2)[0]
	roles := 0
	for _, line := range strings.Split(roleTable, "\n") {
		if strings.HasPrefix(line, "| `") {
			parts := strings.Split(line, "`")
			if len(parts) > 1 {
				roles++
				if !knownRole(parts[1]) {
					t.Fatalf("inherited role rejected: %s", parts[1])
				}
			}
		}
	}
	if roles != 6 {
		t.Fatalf("closed role contract changed: got %d; reconcile the companion", roles)
	}

	if Valid(Scanner) {
		t.Fatal("library-only scanner became a CLI projection")
	}
	t.Cleanup(func() {
		if !t.Failed() {
			checkConsumers(t)
		}
	})
	for _, tc := range []struct {
		name                 string
		labels               []string
		scanner, work, inbox bool
	}{
		{"ordinary", []string{"bug", "raised-by:verifier"}, true, true, false},
		{"addressed", []string{"to:worker"}, false, false, true},
		{"duplicate-stamp", []string{" TO:worker ", "to:WORKER"}, false, false, true},
		{"conflicting", []string{"to:worker", "to:intake"}, false, false, false},
		{"malformed", []string{"to:"}, false, false, false},
	} {
		c := CandidateForLabels(Candidate{}, tc.labels)
		if Select(Scanner, c, "") != tc.scanner || Select(WorkReady, c, "") != tc.work || Select(Inbox, c, "worker") != tc.inbox || !Select(Intake, c, "") || !Select(Full, c, "") {
			t.Fatalf("population parity %s: %+v", tc.name, c)
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Handoff, *HandoffFacts)
		state  string
	}{
		{"repair", func(*Handoff, *HandoffFacts) {}, "observed"},
		{"legacy", func(h *Handoff, f *HandoffFacts) { h.Schema = "" }, "unknown"},
		{"future", func(h *Handoff, f *HandoffFacts) { h.Schema = "verification-handoff-v2" }, "unknown"},
		{"absent-disposition", func(h *Handoff, f *HandoffFacts) { h.Disposition = "" }, "unknown"},
		{"human-hold", func(h *Handoff, f *HandoffFacts) { f.HumanHold = true }, "held"},
		{"activation-hold", func(h *Handoff, f *HandoffFacts) { f.ActivationHold = true }, "held"},
		{"unreadable", func(h *Handoff, f *HandoffFacts) { f.Complete = false }, "unknown"},
		{"stale", func(h *Handoff, f *HandoffFacts) { f.Now = h.FreshUntil.Add(time.Second) }, "unknown"},
		{"wrong-source", func(h *Handoff, f *HandoffFacts) { f.Source.SourceRevision = "other" }, "unknown"},
		{"wrong-case", func(h *Handoff, f *HandoffFacts) { f.Source.CaseID = "other" }, "unknown"},
		{"wrong-rows", func(h *Handoff, f *HandoffFacts) { f.Source.Rows = []int{1, 3} }, "unknown"},
		{"expected-duplicate", func(h *Handoff, f *HandoffFacts) { f.Source.Rows = []int{1, 1} }, "unknown"},
		{"rows-reordered", func(h *Handoff, f *HandoffFacts) { f.Source.Rows = []int{2, 1} }, "observed"},
		{"bad-rows", func(h *Handoff, f *HandoffFacts) { h.Source.Rows = []int{0}; f.Source.Rows = h.Source.Rows }, "unknown"},
		{"missing-source", func(h *Handoff, f *HandoffFacts) { h.Source.Repo = ""; f.Source.Repo = "" }, "unknown"},
		{"mechanical", func(h *Handoff, f *HandoffFacts) { h.DecidingRole = "mechanical" }, "unknown"},
		{"no-successor", func(h *Handoff, f *HandoffFacts) { h.Successor = "" }, "unknown"},
		{"scoping", func(h *Handoff, f *HandoffFacts) { h.Disposition = NeedsScoping }, "observed"},
		{"duplicate", func(h *Handoff, f *HandoffFacts) { h.Disposition = Duplicate; h.Successor = "example-org/tracker#14" }, "observed"},
		{"external", func(h *Handoff, f *HandoffFacts) {
			h.Disposition = External
			h.Successor = ""
			h.ExternalRole = "operator"
			h.RequiredAction = "restore fixture"
		}, "observed"},
		{"external-missing", func(h *Handoff, f *HandoffFacts) { h.Disposition = External; h.Successor = "" }, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := sampleHandoff()
			tc.mutate(&h, &f)
			if got := AssessHandoff(h, f); got.State != tc.state {
				t.Fatalf("%s assessment = %+v, want %s", tc.name, got, tc.state)
			}
		})
	}
	h, _ := sampleHandoff()
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeHandoff(raw); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"schema":"intake-exit-v1"}`, `{"schema":"verification-handoff-v2"}`, `{"schema":"verification-handoff-v1","accepted":true}`, `{} {}`, `{"schema":"verification-handoff-v1","Schema":"verification-handoff-v1"}`, `{"schema":"verification-handoff-v1","schema":"verification-handoff-v1"}`} {
		if _, err := DecodeHandoff([]byte(raw)); err == nil {
			t.Fatal("unsupported/malformed contract accepted", raw)
		}
	}
}
