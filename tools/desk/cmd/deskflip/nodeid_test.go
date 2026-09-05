package main

// nodeid_test.go — the NEGATIVE path for the flip's opaque change id.
//
// THE PROPERTY. The id the ready mutation targets is READ from the change the backend
// returned, and is never composed locally. It matters because the id's ENCODING belongs to the
// backend: on GitHub it is a GraphQL global id, on GitLab it is a synthetic
// `gitlab:<owner>/<name>!<iid>` coordinate naming a PROJECT and an IID. A caller that
// string-built one would be addressing whatever project it guessed at — and would keep working
// on the happy path, where the guess matches, right until it did not.
//
// This test lives beside the suite it extends and exercises the NEW transport it was added
// with, rather than re-asserting the retired one.

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestNodeIDNotConstructed drives the whole verb and asserts the mutation carried the id the
// CHANGE READ produced — a value the verb has no other way to obtain.
func TestNodeIDNotConstructed(t *testing.T) {
	t.Run("mutation_targets_the_id_that_was_read", func(t *testing.T) {
		// A deliberately unguessable id: nothing in this package could compose it from the
		// repo, the number, or any other value it holds. If the mutation carries it, the
		// mutation got it from the change.
		const served = "PR_only_a_change_read_has_this"
		s := newStub()
		s.pr.NodeID = served
		s.install(t)
		s.reviews = approvalAtHead(t, headSHA)

		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
			t.Fatalf("flip rc = %d, want 0", rc)
		}
		var mutations int
		for _, r := range s.requests {
			if r.Method != http.MethodPost || r.Path != "/graphql" {
				continue
			}
			mutations++
			var body struct {
				Variables struct {
					ID string `json:"id"`
				} `json:"variables"`
			}
			if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
				t.Fatalf("the mutation body did not parse: %v (%s)", err, r.Body)
			}
			if body.Variables.ID != served {
				t.Fatalf("the mutation targeted id %q, but the change read returned %q — the id was "+
					"composed rather than read", body.Variables.ID, served)
			}
		}
		if mutations != 1 {
			t.Fatalf("recorded %d ready mutations, want exactly 1 — otherwise this assertion measured "+
				"nothing", mutations)
		}
	})

	// The refusal half: a backend that serves NO opaque id gets a could-not-check, not a
	// locally composed one. The recorder must show no write at all.
	t.Run("no_opaque_id_refuses_not_composes", func(t *testing.T) {
		s := newStub()
		s.pr.NodeID = "-" // rendered, then stripped below so the payload carries an EMPTY node_id
		s.install(t)
		s.pr.NodeID = ""
		s.reviews = approvalAtHead(t, headSHA)

		rc := run([]string{"7", "--repo", privateCIRepo})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc = %d, want %d (could-not-check) — a backend that serves no change id cannot be "+
				"asked to perform the flip", rc, deskkit.ExitUnverifiable)
		}
		if m := s.mutated(); len(m) != 0 {
			t.Fatalf("the refusal still wrote: %v — no id means no write, not a guessed id", m)
		}
	})

	// The rule is enforced by the SIGNATURE, not by convention: ReadyFlip takes the change the
	// backend returned, so there is no string parameter for a composed id to arrive through.
	// A GitLab-shaped id built by hand cannot reach the mutation at all.
	t.Run("no_composed_id_reaches_the_flip", func(t *testing.T) {
		src, err := readDeskflipSource()
		if err != nil {
			t.Fatalf("could-not-check: %v", err)
		}
		for _, forbidden := range []string{"gitlab:", "PR_kwDO"} {
			if strings.Contains(src, forbidden) {
				t.Errorf("non-test source under cmd/deskflip contains %q — an opaque change id spelled "+
					"in this package is an id this package composed", forbidden)
			}
		}
	})
}

// readDeskflipSource concatenates this package's NON-TEST Go source, so the assertion above
// reads what ships rather than what the tests happen to spell.
func readDeskflipSource() (string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	var read int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, rerr := os.ReadFile(name)
		if rerr != nil {
			return "", rerr
		}
		b.Write(raw)
		read++
	}
	if read == 0 {
		return "", errors.New("no non-test Go source was read — a scan that sees nothing certifies everything")
	}
	return b.String(), nil
}
