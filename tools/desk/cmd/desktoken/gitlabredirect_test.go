package main

// gitlabredirect_test.go — review finding SEC-1 (class credential-forwarded-on-redirect), the
// desktoken sites. Rotation and the post-rotation self-check send PRIVATE-TOKEN, a custom header
// net/http forwards on a redirect to another host. Each test points the call at a loopback API
// server (127.0.0.1) that redirects to a second server spelled `localhost` — a different host as
// net/http compares them — and asserts the second server never sees a request.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func redirectPair(t *testing.T, status int) (api string, hits func() (int, []string)) {
	t.Helper()
	var mu sync.Mutex
	n, toks := 0, []string(nil)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		if v := r.Header.Get("PRIVATE-TOKEN"); v != "" {
			toks = append(toks, v)
		}
		mu.Unlock()
		_, _ = w.Write([]byte(`{"token":"example-placeholder-next-0000","active":true}`))
	}))
	t.Cleanup(other.Close)
	to := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to+r.URL.RequestURI(), status)
	}))
	t.Cleanup(srv.Close)
	old := httpClient
	httpClient = &http.Client{}
	t.Cleanup(func() { httpClient = old })
	return srv.URL + "/api/v4", func() (int, []string) {
		mu.Lock()
		defer mu.Unlock()
		return n, append([]string(nil), toks...)
	}
}

func TestGitLabRotateRefusesRedirect(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		api, hits := redirectPair(t, status)
		res, err := rotateGitLabToken(api, "example-placeholder-cur-0000")
		if n, toks := hits(); n > 0 {
			t.Fatalf("HTTP %d: rotation followed the redirect to another host (%d request(s), credentials %q)", status, n, toks)
		}
		if err == nil || res != nil {
			t.Fatalf("HTTP %d: a redirected rotation must fail, got result %v err %v", status, res, err)
		}
	}
}

func TestGitLabSelfCheckRefusesRedirect(t *testing.T) {
	api, hits := redirectPair(t, http.StatusFound)
	st, err := gitlabSelfCheck(api, "example-placeholder-cur-0000")
	if n, toks := hits(); n > 0 {
		t.Fatalf("self-check followed the redirect to another host (%d request(s), credentials %q)", n, toks)
	}
	if err != nil {
		t.Fatalf("a refused redirect is an answer, not a transport failure: %v", err)
	}
	if st == http.StatusOK {
		t.Fatal("a redirected self-check reported 200 — the new token would read as accepted")
	}
}
