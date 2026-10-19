package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestAPIClientDoesNotFollowRedirects — an API answer that redirects is returned as its own
// status, and the credential header never reaches the server the redirect names. Server A
// answers every request with a 302 to server B; B records every request it gets.
func TestAPIClientDoesNotFollowRedirects(t *testing.T) {
	var mu sync.Mutex
	var bHits int
	var bTokens []string
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		bHits++
		if v := r.Header.Get("PRIVATE-TOKEN"); v != "" {
			bTokens = append(bTokens, v)
		}
		mu.Unlock()
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(b.Close)
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, b.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(a.Close)

	c := newAPIClient(a.URL, fakeOwnerToken)
	for _, call := range []func() (string, []byte){
		func() (string, []byte) { return c.call(http.MethodGet, "/groups/x", nil) },
		func() (string, []byte) { return c.call(http.MethodPost, "/groups/x/service_accounts", []byte(`{}`)) },
		func() (string, []byte) { return c.putFile("/user/avatar", "avatar", "a.png", []byte("png")) },
	} {
		if status, _ := call(); status != "302" {
			t.Errorf("status %q, want the redirect itself (302) returned unfollowed", status)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bTokens) > 0 {
		t.Errorf("the redirect target received the credential header %d time(s)", len(bTokens))
	}
	if bHits > 0 {
		t.Errorf("the redirect target received %d request(s), want none", bHits)
	}
}
