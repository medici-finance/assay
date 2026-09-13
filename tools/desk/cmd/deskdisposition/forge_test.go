package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forge_test.go — a minimal deskkit.Forge double for `read`'s two calls (GetIssue,
// ListComments). Embedding deskkit.Forge (nil) means any OTHER method panics if it is
// ever reached — which is the point: `read` must never touch anything but these two, and
// never shell out to `gh` at all (#984).
type stubForge struct {
	deskkit.Forge
	labels       []string
	comments     []string
	failIssue    error
	failComments error
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

// installStubForge points forgeForFn at sf for every repo, restored on test cleanup. No
// desktoken/mint path runs — the same shortcut deskclose's own tests take (stubRemote.install).
func installStubForge(t *testing.T, sf *stubForge) {
	t.Helper()
	old := forgeForFn
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return sf, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	t.Cleanup(func() { forgeForFn = old })
}
