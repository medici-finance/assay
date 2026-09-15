package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forge_test.go — a minimal deskkit.Forge double for the two READ verbs: `read`'s two calls
// (GetIssue, ListComments) and `sweep`'s one (ListOpenChanges). Embedding deskkit.Forge
// (nil) means any OTHER method panics if it is ever reached — which is the point: the read
// verbs must never touch anything but these three, and never shell out to `gh` at all
// (#984, #1123).
type stubForge struct {
	deskkit.Forge
	labels       []string
	comments     []string
	failIssue    error
	failComments error

	// openChanges is what ListOpenChanges serves; failChanges makes it fail instead.
	// listCalls counts the calls, so "one bounded read per repo" is asserted against the
	// backend rather than inferred from output.
	openChanges *deskkit.OpenChanges
	failChanges error
	listCalls   int
}

func (s *stubForge) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	if s.failIssue != nil {
		return nil, s.failIssue
	}
	return &deskkit.Issue{Number: n, Labels: s.labels}, nil
}

func (s *stubForge) ListComments(fr deskkit.ForgeRepo, n int) ([]deskkit.Comment, error) {
	if s.failComments != nil {
		return nil, s.failComments
	}
	out := make([]deskkit.Comment, 0, len(s.comments))
	for _, b := range s.comments {
		out = append(out, deskkit.Comment{Body: b})
	}
	return out, nil
}

func (s *stubForge) ListOpenChanges(fr deskkit.ForgeRepo) (*deskkit.OpenChanges, error) {
	s.listCalls++
	if s.failChanges != nil {
		return nil, s.failChanges
	}
	return s.openChanges, nil
}

// installStubForge points forgeForFn at sf for every repo, restored on test cleanup. No
// desktoken/mint path runs — the same shortcut deskclose's own tests take (stubRemote.install).
func installStubForge(t *testing.T, sf *stubForge) {
	t.Helper()
	installForge(t, func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return sf, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	})
}

// installForge substitutes the whole resolver, so a case can hand back a REAL backend
// (a deskkit.GitLabForge pointed at an httptest instance) rather than a double.
func installForge(t *testing.T, fn func(string) (deskkit.Forge, deskkit.ForgeRepo, error)) {
	t.Helper()
	old := forgeForFn
	forgeForFn = fn
	t.Cleanup(func() { forgeForFn = old })
}

// installForgeError makes every forge resolution fail — the shape a repo whose forge cannot
// be resolved (or whose token cannot be minted) presents to a verb.
func installForgeError(t *testing.T, err error) {
	t.Helper()
	installForge(t, func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return nil, deskkit.ForgeRepo{Owner: owner, Name: name}, err
	})
}
