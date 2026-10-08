package main

import (
	"errors"
	"strings"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/gitexec"
)

// stubFetch replaces both fetch seams: fetchFn records what it was asked for (and never
// reaches a network), fetchEndpointFn stands in for the minted-role endpoint.
func stubFetch(t *testing.T) *[]gitcore.FetchOpts {
	t.Helper()
	var got []gitcore.FetchOpts
	prevFetch, prevEP := fetchFn, fetchEndpointFn
	fetchFn = func(root string, opts gitcore.FetchOpts) error {
		got = append(got, opts)
		return nil
	}
	fetchEndpointFn = func(repo, originURL string) (deskkit.ForgeGitEndpoint, error) {
		return deskkit.ForgeGitEndpoint{Opts: gitcore.ListOpts{
			URL:  "https://github.com/" + repo + ".git",
			Auth: gitcore.BasicAuth("minted-token"),
		}}, nil
	}
	t.Cleanup(func() { fetchFn, fetchEndpointFn = prevFetch, prevEP })
	return &got
}

func TestFetchFromOrigin_HTTPSCanonicalURLTokenInMem(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	git(t, w.root, "remote", "set-url", "origin", "https://github.com/"+testRepo+".git")
	got := stubFetch(t)

	if err := fetchFromOrigin(w.root, testRepo, []string{"+refs/heads/main:refs/remotes/origin/main"}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("want one fetch, got %d", len(*got))
	}
	o := (*got)[0]
	if want := "https://github.com/" + testRepo + ".git"; o.URL != want {
		t.Fatalf("fetch URL = %q, want %q", o.URL, want)
	}
	if strings.Contains(o.URL, "minted-token") || strings.Contains(o.URL, "@") {
		t.Fatalf("the credential is in the fetch URL: %q", o.URL)
	}
	if ba, ok := o.Auth.(*githttp.BasicAuth); !ok || ba.Password != "minted-token" {
		t.Fatalf("credential not carried as the in-memory auth value: %#v", o.Auth)
	}
}

func TestFetchFromOrigin_LocalOriginGetsNoCredential(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	got := stubFetch(t)
	fetchEndpointFn = func(string, string) (deskkit.ForgeGitEndpoint, error) {
		t.Fatal("a local origin must not mint a credential")
		return deskkit.ForgeGitEndpoint{}, nil
	}
	if err := fetchFromOrigin(w.root, testRepo, []string{"+refs/heads/main:refs/remotes/origin/main"}); err != nil {
		t.Fatal(err)
	}
	if o := (*got)[0]; o.URL != w.remote || o.Auth != nil {
		t.Fatalf("local fetch = URL %q auth %v, want %q and no auth", o.URL, o.Auth, w.remote)
	}
}

func TestFetchFromOrigin_NonGitHubTransportRefused(t *testing.T) {
	for _, u := range []string{
		"github.com:" + testRepo + ".git",
		"ssh://github.com/" + testRepo + ".git",
		"http://github.com/" + testRepo + ".git",
	} {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		git(t, w.root, "remote", "set-url", "origin", u)
		got := stubFetch(t)
		err := fetchFromOrigin(w.root, testRepo, []string{"+refs/heads/main:refs/remotes/origin/main"})
		if err == nil || len(*got) != 0 {
			t.Fatalf("origin %q: want a refusal and no fetch, got err=%v fetches=%d", u, err, len(*got))
		}
	}
}

func TestFetchFromOrigin_OriginNotNamingRepoRefused(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	git(t, w.root, "remote", "set-url", "origin", "https://github.com/someone/else.git")
	got := stubFetch(t)
	if err := fetchFromOrigin(w.root, testRepo, nil); err == nil || len(*got) != 0 {
		t.Fatalf("want refusal and no fetch, got err=%v fetches=%d", err, len(*got))
	}
}

func TestFetchState_EndpointFailureIsCouldNotCheck(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	git(t, w.root, "remote", "set-url", "origin", "https://github.com/"+testRepo+".git")
	stubFetch(t)
	fetchEndpointFn = func(string, string) (deskkit.ForgeGitEndpoint, error) {
		return deskkit.ForgeGitEndpoint{}, errors.New("no credential")
	}
	_, _, err := fetchState(w.root, testRepo, prInfo{Number: 7, BaseRefName: "main"})
	if err == nil || !deskkit.IsUnverifiable(err) {
		t.Fatalf("want could-not-check, got %v", err)
	}
}

// fetch is no longer a binary verb for deskmerge: the audited git-binary seam refuses it.
func TestFetchIsNotABinaryVerb(t *testing.T) {
	if gitexecVerbs["fetch"] {
		t.Fatal("fetch must not be routed to the git binary")
	}
	if gitexec.Allowed(toolName, "fetch") {
		t.Fatal("deskmerge:fetch must not be allowlisted")
	}
}
