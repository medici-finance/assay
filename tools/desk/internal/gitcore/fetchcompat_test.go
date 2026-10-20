package gitcore

import (
	"errors"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// regression: #3281
func TestFetchURLLegacyPush(t *testing.T) {
	f := gittest.NewFixture(t)
	const target = "https://github.com/example/project.git"
	for _, args := range [][]string{
		{"config", "remote.origin.pushurl", target},
		{"config", "url.https://push.invalid/.pushInsteadOf", "unmatched:"},
	} {
		if _, err := f.Git(args...); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// The legacy first-URL API retains push-only compatibility. Fetch gates use
	// the plural API, which must reject this same repository without a fetch URL.
	for name, read := range map[string]func(string) (string, error){
		"method": repo.RemoteURL,
		"shared": func(remote string) (string, error) { return RemoteURL(repo.repo, remote) },
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := read("origin"); err != nil || got != target {
				t.Fatalf("legacy URL = %q, %v; want %q", got, err, target)
			}
			if _, err := read("missing"); !errors.Is(err, git.ErrRemoteNotFound) {
				t.Fatalf("missing remote = %v, want wrapped remote-not-found", err)
			}
		})
	}
	if got, err := repo.RemoteURLs("origin"); err == nil {
		t.Fatalf("fetch URLs = %q, want no-fetch-URL error", got)
	}
}
