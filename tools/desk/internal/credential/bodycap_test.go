package credential

// bodycap_test.go — the credential-path forge reads are capped: an over-cap body is refused
// with no bytes handed back, never read as a truncated answer.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// regression: #6730
func TestReadCappedResponseOverCap(t *testing.T) {
	raw, err := readCappedN(strings.NewReader(`{"token":"x"}`+strings.Repeat(" ", 100)), 64)
	if err == nil {
		t.Fatalf("an over-cap body read as success (%d bytes)", len(raw))
	}
	if raw != nil {
		t.Errorf("an over-cap read handed back %d bytes; want none", len(raw))
	}
	raw, err = readCappedN(strings.NewReader(strings.Repeat(" ", 64)), 64)
	if err != nil || len(raw) != 64 {
		t.Errorf("a body at the cap: %d bytes, %v; want 64, nil", len(raw), err)
	}
}

// regression: #6730
func TestMaxResponseBytesIsWired(t *testing.T) {
	if MaxResponseBytes != 32<<20 {
		t.Errorf("MaxResponseBytes = %d, want 32 MiB", MaxResponseBytes)
	}
	if responseCap != MaxResponseBytes {
		t.Errorf("responseCap = %d, want MaxResponseBytes (%d)", responseCap, MaxResponseBytes)
	}
	// readCappedResponse applies MaxResponseBytes: a body one byte over it is refused.
	big := strings.NewReader(strings.Repeat(" ", int(MaxResponseBytes)+1))
	if raw, err := readCappedResponse(big); err == nil {
		t.Errorf("a body one byte over MaxResponseBytes read as success (%d bytes)", len(raw))
	}
}

// lowCap lowers the cap readCappedResponse applies for one test.
func lowCap(t *testing.T, n int64) {
	t.Helper()
	prev := responseCap
	responseCap = n
	t.Cleanup(func() { responseCap = prev })
}

// overCapServer answers every request 200 with a JSON body longer than the lowered cap.
func overCapServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body+strings.Repeat(" ", 200))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The GitHub App read (installations, mint) goes through the cap.
//
// regression: #6730
func TestGithubGETOverCapBody(t *testing.T) {
	lowCap(t, 64)
	srv := overCapServer(t, `[]`)
	_, body, err := githubGET(srv.Client(), "GET", srv.URL+"/app/installations", "example-jwt")
	if err == nil {
		t.Fatalf("an over-cap body read as success (%d bytes)", len(body))
	}
	if body != nil {
		t.Errorf("an over-cap read handed back %d bytes; want none", len(body))
	}
}

// The GitLab self-rotation read goes through the cap.
//
// regression: #6730
func TestRotateGitLabOverCapBody(t *testing.T) {
	lowCap(t, 64)
	srv := overCapServer(t, `{"token":"example-rotated","expires_at":"2099-01-01"}`)
	res, err := RotateGitLabToken(srv.Client(), srv.URL, "example-current")
	if err == nil {
		t.Fatalf("an over-cap rotation answer read as success: %+v", res)
	}
	if strings.Contains(err.Error(), "example-current") {
		t.Errorf("the current credential appeared in the error")
	}
}
