package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCallbackBadState — Verify row 6. A callback carrying a state that matches no pending
// row is refused (403), no conversion is attempted, and the (unrelated, real) row is left
// exactly as it was.
func TestCallbackBadState(t *testing.T) {
	setupTest(t)

	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, realNonce := plantPendingRow(t, actSpec, "team")
	before := sf.rowByApp("example-act").State

	called := false
	prevConvert := convertCodeFn
	convertCodeFn = func(code string) (*conversionResult, error) {
		called = true
		return &conversionResult{PEM: "should-never-be-used"}, nil
	}
	t.Cleanup(func() { convertCodeFn = prevConvert })

	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/callback?code=abc123&state=this-is-a-foreign-state-nobody-issued")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body: %s)", resp.StatusCode, body)
	}
	if called {
		t.Fatal("conversion was attempted for a foreign/unknown state — the state nonce check did not gate it")
	}
	if got := sf.rowByApp("example-act").State; got != before {
		t.Fatalf("row state changed from %s to %s on a bad-state callback", before, got)
	}
	if got := sf.rowByApp("example-act").StateNonce; got != realNonce {
		t.Fatalf("row nonce changed on a bad-state callback: %s", got)
	}
}

// TestCallbackGoodStateConverts is the positive control for TestCallbackBadState: the SAME
// nonce the row was actually issued reaches conversion and keys the row. Without this, a
// bug that refused every callback (never just bad ones) would still pass the negative test.
func TestCallbackGoodStateConverts(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	fake := fakeConversionServer(t, conversionResult{ID: 1, ClientID: "c", WebhookSecret: "w", PEM: "PEMBYTES"})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// http.Get follows the callback's 302 redirect to /run.
		t.Fatalf("status = %d, want 200 (after following redirect to /run)", resp.StatusCode)
	}
	if got := sf.rowByApp("example-act").State; got != StateKeyed {
		t.Fatalf("row state = %s, want keyed", got)
	}
}

// TestCallbackReplayDoesNotOverwriteKey — S-2. Once a row is keyed, a replayed
// /callback?code=<fresh>&state=<same nonce> must NOT re-run the write path and overwrite the
// stored key. The nonce is consumed on keying (so rowByNonce no longer finds the row) and the
// state-machine guard refuses any row not pending/posted — together they close the replay
// window handleMarkPosted already guarded on its own transition.
func TestCallbackReplayDoesNotOverwriteKey(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	fake := fakeConversionServer(t, conversionResult{ID: 1, ClientID: "c", WebhookSecret: "w", PEM: "ORIGINAL-KEY"})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	// First callback keys the row.
	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := sf.rowByApp("example-act").State; got != StateKeyed {
		t.Fatalf("row state = %s, want keyed after the first callback", got)
	}
	orig, err := readPEMFor(t, "example-act")
	if err != nil {
		t.Fatalf("reading PEM after first callback: %v", err)
	}

	// A replay with the same nonce carrying a fresh code that WOULD write a different key.
	called := false
	prev := convertCodeFn
	convertCodeFn = func(code string) (*conversionResult, error) {
		called = true
		return &conversionResult{ID: 2, PEM: "ATTACKER-KEY"}, nil
	}
	t.Cleanup(func() { convertCodeFn = prev })

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp2, err := client.Get(ts.URL + "/callback?code=fresh&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	if called {
		t.Fatal("the replay reached conversion — a keyed row's nonce must be spent and the state guard must refuse it")
	}
	after, err := readPEMFor(t, "example-act")
	if err != nil {
		t.Fatalf("reading PEM after replay: %v", err)
	}
	if string(after) != string(orig) {
		t.Fatalf("the stored key was overwritten by a replay: %q -> %q", orig, after)
	}
}

// TestCallbackExpiredCodeReturnsToPosted — a 404 conversion (code expired) leaves the row
// at "posted" with no key written, per design.md §4.
func TestCallbackExpiredCodeReturnsToPosted(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	fake := fakeConversionServer(t, conversionResult{})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=expired&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := sf.rowByApp("example-act").State; got != StatePosted {
		t.Fatalf("row state = %s, want posted (code-expired path)", got)
	}
	if _, err := readPEMFor(t, "example-act"); err == nil {
		t.Fatal("a PEM was written despite an expired-code conversion")
	}
}
