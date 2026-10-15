package deskkit

// forgetransport_test.go — the opt-in GitHub transport seam cannot bypass custody or the
// outbound check (issue #2421).
//
// SetGitHubTransport lets ONE tool swap the round-tripper beneath a resolved GitHub backend.
// These tests pin that the swap sits strictly BELOW both controls the resolver applies, with
// a wrapper installed that would happily answer any request it is handed:
//
//   - the resolver still returns the outbound-checked decorator, and its backend still
//     carries the minted token (not an ambient one);
//   - an unminted backend is refused before the wrapper sees a request, and an empty mint
//     yields no backend at all;
//   - a write the outbound check refuses never reaches the wrapper;
//   - the one request a read does send reaches the wrapper carrying the mint.

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// seamRecorder is an installed wrapper that never forwards: it records each request it is
// handed and answers it itself, so a request that reaches it would succeed.
type seamRecorder struct {
	mu   sync.Mutex
	seen []seenRequest
}

func (s *seamRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.seen = append(s.seen, seenRequest{path: r.URL.Path, auth: r.Header.Get("Authorization")})
	s.mu.Unlock()
	body := `{"number":7,"state":"open","head":{"sha":"abc123"},"id":1,"html_url":"https://example.invalid/c/1"}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

func (s *seamRecorder) requests() []seenRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]seenRequest(nil), s.seen...)
}

// installSeamRecorder installs a recording wrapper for the test and removes it afterwards.
func installSeamRecorder(t *testing.T) *seamRecorder {
	t.Helper()
	rec := &seamRecorder{}
	SetGitHubTransport(func(http.RoundTripper) http.RoundTripper { return rec })
	t.Cleanup(func() { SetGitHubTransport(nil) })
	return rec
}

// transportSeamRoster plants a roster that resolves obPrivate to GitHub and lists it as private, so
// the outbound check has a configured target to judge a write against.
func transportSeamRoster(t *testing.T) ForgeRepo {
	t.Helper()
	r := goldenRoster()
	r[EnvAllowedRepos] = obPublic + ":ci:public," + obPrivate + ":ci:private"
	r[EnvRepoForges] = obPrivate + "=github"
	r[EnvHumanLoginMap] = "alex:ada"
	withRoster(t, r)
	t.Setenv(EnvWithheldIdentifiers, obWithheld)
	t.Setenv(EnvOutboundCalloutRequired, "")
	return obRepo(obPrivate)
}

// regression: #2421
func TestGitHubTransportSeamCannotBypassCustody(t *testing.T) {
	t.Run("resolved_backend_stays_checked_and_minted", func(t *testing.T) {
		repo := transportSeamRoster(t)
		plantAmbientCredentials(t)
		rec := installSeamRecorder(t)
		tokFile := filepath.Join(t.TempDir(), "worker-token")
		if err := os.WriteFile(tokFile, []byte("minted-seam-stub\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stubMinter(t, tokFile, "", nil)

		f, err := ForgeFor(repo, "worker")
		if err != nil {
			t.Fatalf("ForgeFor with a seam installed: %v", err)
		}
		if !IsOutboundChecked(f) {
			t.Fatalf("with a seam installed ForgeFor returned %T — the wrapper must never replace the "+
				"outbound-checked decorator", f)
		}
		g, ok := backendOf(f).(*GitHubForge)
		if !ok {
			t.Fatalf("backend = %T, want *GitHubForge", backendOf(f))
		}
		if g.Token != "minted-seam-stub" {
			t.Fatalf("backend token = %q, want the mint — the seam must not change custody", g.Token)
		}
		if g.Client == nil {
			t.Fatal("backend has no client — the installed seam was not applied, so this test proves nothing")
		}

		// A read through the decorator reaches the wrapper exactly once, carrying the mint.
		if _, err := f.GetPullRequest(repo, 7); err != nil {
			t.Fatalf("read through the seam failed: %v", err)
		}
		got := rec.requests()
		if len(got) != 1 {
			t.Fatalf("read sent %d requests through the seam, want 1", len(got))
		}
		if !strings.Contains(got[0].auth, "minted-seam-stub") {
			t.Fatalf("request reached the seam with Authorization %q, want the mint", got[0].auth)
		}
		assertNoAmbient(t, got)

		// A write the outbound check refuses never reaches the wrapper.
		SetOutboundContext(OutboundContext{Tool: "conformance", Verb: "comment"})
		t.Cleanup(func() { SetOutboundContext(OutboundContext{}) })
		body := "the vendor contact is " + obEmailOutside + " per the thread"
		if _, err := f.PostComment(repo, 7, body); err == nil {
			t.Fatal("a write carrying personal data was not refused with a seam installed")
		}
		if n := len(rec.requests()); n != 1 {
			t.Fatalf("the seam saw %d requests after a refused write, want still 1 — the outbound "+
				"check must refuse ABOVE the transport", n)
		}
	})

	t.Run("unminted_backend_refused_before_the_seam", func(t *testing.T) {
		withRoster(t, goldenRoster())
		plantAmbientCredentials(t)
		rec := installSeamRecorder(t)
		g := &GitHubForge{Token: "", BaseURL: "https://api.example.invalid", Client: githubSeamClient()}
		if g.Client == nil {
			t.Fatal("seam client is nil with a wrapper installed — fixture broken")
		}
		if _, err := g.GetPullRequest(obRepo(obPrivate), 7); err == nil {
			t.Fatal("an unminted backend read succeeded through the seam")
		}
		if n := len(rec.requests()); n != 0 {
			t.Fatalf("%d request(s) reached the seam for an unminted backend — the refusal must "+
				"come before any transport", n)
		}
	})

	t.Run("empty_mint_yields_no_backend", func(t *testing.T) {
		repo := transportSeamRoster(t)
		plantAmbientCredentials(t)
		rec := installSeamRecorder(t)
		empty := filepath.Join(t.TempDir(), "worker-token-empty")
		if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stubMinter(t, empty, "", nil)
		if f, err := ForgeFor(repo, "worker"); err == nil || f != nil {
			t.Fatalf("an empty mint produced a backend with a seam installed: %#v, %v", f, err)
		}
		if n := len(rec.requests()); n != 0 {
			t.Fatalf("%d request(s) reached the seam after an empty mint", n)
		}
	})
}
