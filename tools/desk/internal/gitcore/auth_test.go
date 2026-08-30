package gitcore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// tokenFixture is a throwaway value standing in for an installation token — never a
// real credential, and deliberately NOT shaped like a real GitHub token prefix
// (ghp_/gho_/ghs_/github_pat_) so it can appear in test output and PR evidence without
// tripping a secret scanner on a fixture. Used only to assert it never leaves the
// process via any string this package returns.
const tokenFixture = "fixture-installation-token-never-leaves-0123456789abcdef"

func TestTokenNeverLeaves(t *testing.T) {
	auth := BasicAuth(tokenFixture)

	if auth.Username != appUsername {
		t.Fatalf("Username = %q, want %q", auth.Username, appUsername)
	}
	if auth.Password != tokenFixture {
		t.Fatalf("Password not set to the fixture token — the builder mutated it")
	}

	// Every string form the auth value itself can produce must mask the token: this
	// is what "in memory only" means operationally — a caller that accidentally logs
	// the auth value (fmt.Sprintf, %v/%+v/%s, .String()) must not leak it.
	forms := []string{
		auth.String(),
		fmt.Sprintf("%v", auth),
		fmt.Sprintf("%+v", auth),
		fmt.Sprintf("%s", auth),
	}
	for _, s := range forms {
		if strings.Contains(s, tokenFixture) {
			t.Fatalf("token leaked in string form: %q", s)
		}
	}

	// Exercise a real transport failure path (fetch against a nonexistent local
	// remote) and assert the token never surfaces in the returned error — the only
	// other place a caller might accidentally print it (an error log line).
	f := gittest.NewFixture(t)
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fetchErr := repo.Fetch(FetchOpts{
		URL:      f.Dir + "/does-not-exist",
		RefSpecs: []string{"refs/heads/main:refs/heads/main"},
		Auth:     auth,
	})
	if fetchErr == nil {
		t.Fatal("expected an error fetching from a nonexistent remote")
	}
	if strings.Contains(fetchErr.Error(), tokenFixture) {
		t.Fatalf("token leaked in Fetch error: %v", fetchErr)
	}

	// Same check on List's failure path.
	_, listErr := List(ListOpts{URL: f.Dir + "/does-not-exist", Auth: auth})
	if listErr == nil {
		t.Fatal("expected an error listing a nonexistent remote")
	}
	if strings.Contains(listErr.Error(), tokenFixture) {
		t.Fatalf("token leaked in List error: %v", listErr)
	}
}
