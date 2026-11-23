package deskkit

// itemrepos_test.go — the shared tracking/deliverable resolver (#6274). Every repository below
// is a synthetic example-org fixture.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each role names the repository the class record assigns it: the decision issue, Evidence,
// outcome record and verifier claim go to the tracking repository; homes to the deliverable.
// regression: #6274
func TestItemReposRoles(t *testing.T) {
	r, err := NewItemRepos("example-org/tracking", "example-org/one")
	if err != nil {
		t.Fatal(err)
	}
	if !r.CrossRepo() {
		t.Fatal("a pair of different repositories is cross-repository")
	}
	for name, got := range map[string]string{
		"DecisionRepo": r.DecisionRepo(), "EvidenceRepo": r.EvidenceRepo(),
		"OutcomeRepo": r.OutcomeRepo(), "VerifierClaimRepo": r.VerifierClaimRepo(),
	} {
		if got != "example-org/tracking" {
			t.Errorf("%s = %q, want the tracking repository", name, got)
		}
	}
	if r.HomeRepo() != "example-org/one" {
		t.Errorf("HomeRepo = %q, want the deliverable repository", r.HomeRepo())
	}
	same, err := NewItemRepos("example-org/tracking", "")
	if err != nil || same.CrossRepo() || same.HomeRepo() != "example-org/tracking" {
		t.Fatalf("an undeclared deliverable is the tracking repository: %+v %v", same, err)
	}
	if folded, _ := NewItemRepos("example-org/tracking", "Example-Org/Tracking"); folded.CrossRepo() {
		t.Fatal("slugs that differ only in case are one repository")
	}
	for _, bad := range [][2]string{{"", "example-org/one"}, {"example-org/tracking", "not a slug"}, {"../x", ""}} {
		if _, err := NewItemRepos(bad[0], bad[1]); ExitCodeOf(err) != ExitRefused {
			t.Errorf("NewItemRepos(%q, %q) err = %v, want a refusal", bad[0], bad[1], err)
		}
	}
}

// ResolveItemRepos reads the declaration from the tracking commit with the verifier
// association's policy, so dispatch and verification can never disagree on the deliverable.
// regression: #6274
func TestResolveItemReposFromBrief(t *testing.T) {
	twoSiblingRoster(t)
	const brief = "docs/streams/sample/brief-01-plan.md"
	tracking := crossRepoGitFixture(t, "example-org/tracking", brief, "# Plan\n")
	head, _ := verifierGit(tracking, "rev-parse", "HEAD")
	r, err := ResolveItemRepos(tracking, "example-org/tracking", head, brief)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tracking != "example-org/tracking" || r.Deliverable != "example-org/one" {
		t.Fatalf("resolved %+v, want tracking example-org/tracking delivering into example-org/one", r)
	}
	if _, err := ResolveItemRepos(tracking, "example-org/tracking", head, "docs/streams/sample/missing.md"); ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("an unreadable brief must be could-not-check, got %v", err)
	}
}

// The 2026-11-18 instance: a deliverable repository with no docs/streams cannot be a stream
// root, so DESK_ROOTS cannot name it. CheckoutForRepo answers it from DESK_CHECKOUTS (or the
// sibling convention) without asking for streams, and proves every answer by its origin.
// regression: #6274
func TestCheckoutForRepoStreamless(t *testing.T) {
	twoSiblingRoster(t)
	tracking := crossRepoGitFixture(t, "example-org/tracking", "docs/streams/sample/brief-01-plan.md", "# Plan\n")
	one := crossRepoGitFixture(t, "example-org/one", "a.go", "package one\n") // no docs/streams
	t.Setenv(RootsEnv, "example-org/tracking="+tracking)

	t.Run("unconfigured", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "")
		_, err := CheckoutForRepo("example-org/one", "")
		if ExitCodeOf(err) != ExitUnverifiable || !strings.Contains(err.Error(), CheckoutsEnv) {
			t.Fatalf("an unconfigured checkout must be could-not-check naming %s, got %v", CheckoutsEnv, err)
		}
	})
	t.Run("checkouts env", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "example-org/one="+one)
		c, err := CheckoutForRepo("Example-Org/One", "")
		if err != nil || c.Path != one || c.Source != CheckoutsEnv {
			t.Fatalf("CheckoutForRepo = %+v, %v; want %s from %s", c, err, one, CheckoutsEnv)
		}
	})
	t.Run("stream root wins", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "example-org/tracking="+one)
		c, err := CheckoutForRepo("example-org/tracking", "")
		if err != nil || c.Path != tracking || c.Source != RootsEnv {
			t.Fatalf("CheckoutForRepo = %+v, %v; want the DESK_ROOTS entry", c, err)
		}
	})
	t.Run("wrong origin refuses", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "example-org/one="+tracking)
		if _, err := CheckoutForRepo("example-org/one", ""); ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "wrong origin") {
			t.Fatalf("a mis-mapped checkout must refuse, got %v", err)
		}
	})
	t.Run("outside the repo set", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "example-org/nobody="+one)
		if _, err := CheckoutForRepo("example-org/one", ""); ExitCodeOf(err) != ExitRefused {
			t.Fatalf("a %s entry outside the repo set must refuse the whole value, got %v", CheckoutsEnv, err)
		}
		if _, err := CheckoutForRepo("example-org/nobody", ""); ExitCodeOf(err) != ExitRefused {
			t.Fatalf("a repository outside the set must refuse, got %v", err)
		}
	})
	t.Run("duplicate entry", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "example-org/one="+one+",Example-Org/One="+one)
		if _, err := CheckoutForRepo("example-org/one", ""); ExitCodeOf(err) != ExitRefused {
			t.Fatalf("a repeated %s entry must refuse, got %v", CheckoutsEnv, err)
		}
	})
	t.Run("sibling convention", func(t *testing.T) {
		t.Setenv(CheckoutsEnv, "")
		parent := t.TempDir()
		sib := filepath.Join(parent, "one")
		if err := os.Rename(one, sib); err != nil {
			t.Fatal(err)
		}
		near := filepath.Join(parent, "tracking")
		if err := os.Mkdir(near, 0o700); err != nil {
			t.Fatal(err)
		}
		c, err := CheckoutForRepo("example-org/one", near)
		if err != nil || c.Path != sib || c.Source != "sibling checkout" {
			t.Fatalf("CheckoutForRepo = %+v, %v; want the sibling %s", c, err, sib)
		}
		if err := os.Rename(sib, one); err != nil {
			t.Fatal(err)
		}
	})
}

// The planner's wake reader resolves a receipt's repo-source key through the same resolver,
// so a stream-less deliverable named only by DESK_CHECKOUTS is unchanged, not could-not-check
// (the re-offering the class record describes).
// regression: #6274
func TestWakeRepoSourceStreamlessCheckout(t *testing.T) {
	twoSiblingRoster(t)
	one := crossRepoGitFixture(t, "example-org/one", "a.go", "package one\n")
	const brief = "docs/streams/sample/brief-01-plan.md"
	tracking := crossRepoGitFixture(t, "example-org/tracking", brief, "# Plan\n")
	t.Setenv(RootsEnv, "example-org/tracking="+tracking)
	t.Setenv(CheckoutsEnv, "example-org/one="+one)
	oneSHA, _ := verifierGit(one, "rev-parse", "HEAD")
	rr := NewRootRevisionReader(tracking, "test-tool")
	if got, ok := rr.Revision(InputKeyRepoSourcePrefix + "example-org/one"); !ok || got != oneSHA {
		t.Fatalf("repo-source via %s = %q, %v; want %s", CheckoutsEnv, got, ok, oneSHA)
	}
	t.Setenv(CheckoutsEnv, "")
	if _, ok := rr.Revision(InputKeyRepoSourcePrefix + "example-org/one"); ok {
		t.Fatal("an unconfigured deliverable checkout must stay could-not-check")
	}
}
