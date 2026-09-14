package deskkit

// forge_openchange_test.go — the negative-path contract for OpenChangeForBranch (op 33): more
// than one OPEN change on a single source branch is a could-not-check REFUSAL naming the
// ambiguity, on BOTH backends, never a silent first-match. This is the guard the pre-mortem
// row keys on ("OpenChangeForBranch silently returns the first of several open changes") — a
// first-match would route deskpr's follow-up mergeable read or an edit at whichever change the
// forge happened to list first, which is exactly the wrong-actor/wrong-object failure this
// stream exists to close. The both-backend goldens pin the wire; this pins the SEMANTICS
// (refusal, nil result, exit 6) so the assertion is not buried in a golden diff.

import (
	"strings"
	"testing"
)

func TestOpenChangeForBranchAmbiguousRefuses(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		s := newGoldenServer(t)
		s.pullsList = []map[string]any{
			{"number": 21, "state": "open", "head": map[string]any{"ref": "feat/x"}},
			{"number": 22, "state": "open", "head": map[string]any{"ref": "feat/x"}},
		}
		pr, err := s.forge().OpenChangeForBranch(forgeTestRepo, "feat/x")
		assertAmbiguousRefusal(t, pr, err)
	})

	t.Run("gitlab", func(t *testing.T) {
		s := newGLServer(t)
		s.mrList = []map[string]any{glMR(map[string]any{"iid": 7}), glMR(map[string]any{"iid": 8})}
		pr, err := s.forge().OpenChangeForBranch(glRepo, "feat/x")
		assertAmbiguousRefusal(t, pr, err)
	})

	// The control: exactly one open change on the branch resolves, it does NOT refuse — so the
	// refusal above is provoked by the ambiguity, not by any two-or-more read failing outright.
	t.Run("github_single_resolves", func(t *testing.T) {
		s := newGoldenServer(t)
		s.pullsList = []map[string]any{
			{"number": 21, "state": "open", "head": map[string]any{"ref": "feat/x"}},
		}
		pr, err := s.forge().OpenChangeForBranch(forgeTestRepo, "feat/x")
		if err != nil {
			t.Fatalf("a single open change must resolve, got error: %v", err)
		}
		if pr == nil || pr.Number != 21 {
			t.Fatalf("expected the single open change #21, got %+v", pr)
		}
	})
}

func assertAmbiguousRefusal(t *testing.T, pr *PullRequest, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("two open changes on one source branch must REFUSE, got no error and pr=%+v", pr)
	}
	if pr != nil {
		t.Errorf("a refusal must return no change (a first-match is the very failure this guards), got %+v", pr)
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Errorf("ambiguity refusal exit code = %d, want %d (could-not-check)", got, ExitUnverifiable)
	}
	msg := strings.ToLower(err.Error())
	for _, want := range []string{"could-not-check", "source branch", "feat/x"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal message does not name %q: %s", want, err.Error())
		}
	}
}
