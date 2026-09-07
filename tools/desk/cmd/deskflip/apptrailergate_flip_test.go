package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// workerBot is the roster's worker-App login, read from the fixture roster (never a literal),
// so the test exercises the same resolution the gate uses in production.
func workerBot(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin("worker")
	if !ok {
		t.Fatal("the fixture roster does not bind the worker role")
	}
	return login
}

// TestFlipGate_TrailerAbsentAppFailsClosed is the #587 deskflip-gate proof. checkSecurityVerdict
// is the risk-class + Security-Review decision. On a repo that is NOT risk-classed by visibility
// (privateCIRepo) with NO risky paths, the ONLY thing that can risk-class the PR here is the
// trailer-absent-App term — so this isolates it. Prove each against the pre-fix behaviour: before
// #587 a trailer-less App PR was clean and would have flipped with no Security-Review.
func TestFlipGate_TrailerAbsentAppFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)

	o := flipOpts{pr: 7}
	reviewerLogin, _ := deskkit.RoleAppLogin(reviewerRole)
	const noTrailer = "## Context\nA change with no Brief: or Issue: link trailer.\n"
	// A quiet, non-risky, fully-read diff: a docs path in no compiled trigger set, with
	// ChangedFiles matching len(files) so neither the empty-diff nor the short-read fail-closed
	// term fires. That leaves the trailer-absent-App term as the ONLY thing that can risk-class.
	quiet := []fileInfo{{Path: "docs/streams/example/note.md"}}
	prFor := func(author string) prInfo {
		return prInfo{AuthorLogin: author, Body: noTrailer, ChangedFiles: len(quiet)}
	}

	secPassReview := func() []reviewInfo {
		r := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: pass", SubmittedAt: "2026-01-01T00:01:00Z"}
		r.User.Login = reviewerLogin
		return []reviewInfo{r}
	}

	t.Run("App author, no trailer, no Security-Review -> REFUSED (fails closed)", func(t *testing.T) {
		err := checkSecurityVerdict(o, privateCIRepo, prFor(workerBot(t)), quiet, nil, reviewerLogin, headSHA)
		if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("trailer-less App PR without a Security-Review must be REFUSED; got %v", err)
		}
	})

	t.Run("HUMAN author, no trailer, no Security-Review -> clean (keeps path terms)", func(t *testing.T) {
		if err := checkSecurityVerdict(o, privateCIRepo, prFor("ada"), quiet, nil, reviewerLogin, headSHA); err != nil {
			t.Fatalf("a trailer-less HUMAN-authored PR must not be risk-classed by this term; got %v", err)
		}
	})

	t.Run("App author, no trailer, Security-Review pass at head -> flips", func(t *testing.T) {
		if err := checkSecurityVerdict(o, privateCIRepo, prFor(workerBot(t)), quiet, secPassReview(), reviewerLogin, headSHA); err != nil {
			t.Fatalf("a trailer-less App PR WITH a Security-Review pass at head must clear the gate; got %v", err)
		}
	})
}
