package deskkit

// forgebodycap_test.go — the byte cap on raw forge response bodies: an over-cap body is
// could-not-check (Unverifiable, exit 6) with no bytes handed back, never a truncated success.

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
)

// overCapServer answers every request with HTTP 200 and a JSON array padded to n bytes. The
// array is valid JSON at every length, so a reader that truncated it would see a
// well-formed, complete-looking answer.
func overCapServer(t *testing.T, n int) *httptest.Server {
	t.Helper()
	body := paddedJSON(n)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// paddedJSON is `[]` followed by spaces, n bytes in all.
func paddedJSON(n int) []byte {
	return append([]byte("[]"), bytes.Repeat([]byte(" "), n-2)...)
}

// lowCap lowers the shared cap to limit for the test.
func lowCap(t *testing.T, limit int64) {
	t.Helper()
	t.Cleanup(SetForgeBodyCapForTest(limit))
}

// regression: #6730
func TestReadCappedForgeBodyOverCap(t *testing.T) {
	raw, err := readCapped(bytes.NewReader(paddedJSON(65)), "GET /example", 64)
	if err == nil {
		t.Fatalf("a body over the cap read as success (%d bytes)", len(raw))
	}
	if raw != nil {
		t.Errorf("an over-cap read handed back %d bytes; want none (a prefix is never the answer)", len(raw))
	}
	if !IsUnverifiable(err) || ExitCodeOf(err) != 6 {
		t.Errorf("over-cap must be Unverifiable (exit 6), got exit %d: %v", ExitCodeOf(err), err)
	}
	if !IsForgeBodyOverCap(err) {
		t.Errorf("over-cap error does not carry *ForgeBodyCapError: %v", err)
	}
}

// A body exactly at the cap is the whole answer.
//
// regression: #6730
func TestReadCappedForgeBodyAtCap(t *testing.T) {
	raw, err := readCapped(bytes.NewReader(paddedJSON(64)), "GET /example", 64)
	if err != nil || len(raw) != 64 {
		t.Fatalf("a body at the cap: %d bytes, err %v; want 64, nil", len(raw), err)
	}
}

// regression: #6730
func TestForgeBodyCapDefault(t *testing.T) {
	if MaxForgeBodyBytes != 32<<20 {
		t.Errorf("MaxForgeBodyBytes = %d, want 32 MiB", MaxForgeBodyBytes)
	}
	if forgeBodyCap != MaxForgeBodyBytes {
		t.Errorf("the cap ReadCappedForgeBody applies is %d, want MaxForgeBodyBytes (%d)", forgeBodyCap, MaxForgeBodyBytes)
	}
}

// The credential package carries its own copy of the cap; the two must not drift.
//
// regression: #6730
func TestForgeBodyCapMatches(t *testing.T) {
	if credential.MaxResponseBytes != MaxForgeBodyBytes {
		t.Errorf("credential.MaxResponseBytes = %d, deskkit.MaxForgeBodyBytes = %d — keep them equal",
			credential.MaxResponseBytes, MaxForgeBodyBytes)
	}
}

// ReadForgeBodyUpTo never widens the shared cap.
//
// regression: #6730
func TestReadForgeBodyUpToClamps(t *testing.T) {
	lowCap(t, 64)
	if _, err := ReadForgeBodyUpTo(bytes.NewReader(paddedJSON(80)), "GET /example", 70); !IsForgeBodyOverCap(err) {
		t.Errorf("an 80-byte body under a 70-byte limit: %v, want over-cap", err)
	}
	if raw, err := ReadForgeBodyUpTo(bytes.NewReader(paddedJSON(80)), "GET /example", 1<<40); err != nil || len(raw) != 80 {
		t.Errorf("a limit above MaxForgeBodyBytes must clamp to it, not refuse a small body: %d, %v", len(raw), err)
	}
}

// The forge library's JSON read: an over-cap answer is could-not-check, and it is not
// classed transient (the same request returns the same oversize body again).
//
// regression: #6730
func TestDoJSONOverCapBody(t *testing.T) {
	lowCap(t, 64)
	srv := overCapServer(t, 200)
	g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	var out []any
	_, err := g.doJSONHeader(http.MethodGet, "/repos/example-org/example-repo/issues", nil, &out)
	if err == nil {
		t.Fatalf("an over-cap answer decoded as success: %v", out)
	}
	if !IsForgeBodyOverCap(err) || ExitCodeOf(err) != 6 {
		t.Errorf("want over-cap Unverifiable (exit 6), got exit %d: %v", ExitCodeOf(err), err)
	}
	var de *DeskError
	if errors.As(err, &de) && de.transientRead {
		t.Errorf("an over-cap answer was classed transient (retryable): %v", err)
	}
}

// regression: #6730
func TestChangeDiffOverCapBody(t *testing.T) {
	lowCap(t, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "diff --git a/x b/x\n"+strings.Repeat("+line\n", 40))
	}))
	t.Cleanup(srv.Close)
	g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	diff, err := g.ChangeDiff(ForgeRepo{Owner: "example-org", Name: "example-repo"}, 1)
	if err == nil {
		t.Fatalf("an over-cap diff read as the whole diff (%d bytes)", len(diff))
	}
	if !IsForgeBodyOverCap(err) {
		t.Errorf("want over-cap refusal, got %v", err)
	}
}
