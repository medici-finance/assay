package main

import (
	"errors"
	"os"
	"path/filepath"
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

// The credential goes to the forge's CANONICAL URL for the gated repo, never to whatever host
// the origin remote names. The case above cannot tell the two apart (its origin IS the
// canonical URL), so here origin names the right owner/repo on a DIFFERENT host: the fetch
// must connect to the canonical URL the endpoint resolver returned, carrying the token, and
// the origin host must appear nowhere in what the transport was handed. Binding the fetch URL
// to the origin string instead of the resolved endpoint goes red here.
func TestFetchFromOrigin_TokenNeverFollowsOriginHost(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	const elsewhere = "origin-host.invalid"
	git(t, w.root, "remote", "set-url", "origin", "https://"+elsewhere+"/"+testRepo+".git")
	got := stubFetch(t)

	if err := fetchFromOrigin(w.root, testRepo, []string{"+refs/heads/main:refs/remotes/origin/main"}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("want one fetch, got %d", len(*got))
	}
	o := (*got)[0]
	if want := "https://github.com/" + testRepo + ".git"; o.URL != want {
		t.Fatalf("fetch URL = %q, want the canonical %q (the origin host must not decide where the token goes)", o.URL, want)
	}
	if strings.Contains(strings.ToLower(o.URL), elsewhere) {
		t.Fatalf("the fetch was pointed at the origin host: %q", o.URL)
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

// A local-origin fetch runs the REAL fetch path in-process and starts no child: a recording
// stand-in for git-upload-pack (the program go-git's stock local transport looks up first)
// sits first on PATH, under environment-supplied git configuration a child would honour. The
// stand-in must never run, and the base branch must still land. Removing gitcore's in-process
// local transport turns this red on the stand-in.
func TestFetchFromOrigin_LocalOriginStartsNoChild(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "children.log")
	standIn := "#!/bin/sh\n{ echo \"STARTED $0 $*\"; env; } >>'" + logPath + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "git-upload-pack"), []byte(standIn), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "uploadpack.packObjectsHook")
	t.Setenv("GIT_CONFIG_VALUE_0", "/nonexistent/should-not-run")

	ferr := fetchFromOrigin(w.root, testRepo, []string{"+refs/heads/main:refs/remotes/origin/fetched-main"})
	if b, err := os.ReadFile(logPath); err == nil {
		first, _, _ := strings.Cut(string(b), "\n")
		t.Fatalf("a local-origin fetch started a child process (%s) — it must run in-process", first)
	}
	if ferr != nil {
		t.Fatal(ferr)
	}
	r, err := gitcore.Open(w.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("refs/remotes/origin/fetched-main"); err != nil {
		t.Fatalf("the fetch did not land: %v", err)
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
