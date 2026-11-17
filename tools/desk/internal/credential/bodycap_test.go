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

// countingBody counts the bytes the caller pulled out of a response body.
type countingBody struct {
	io.ReadCloser
	n *int64
}

func (b countingBody) Read(p []byte) (int, error) {
	k, err := b.ReadCloser.Read(p)
	*b.n += int64(k)
	return k, err
}

// toServer sends every request to the offline test server in place of the forge host and
// counts the response-body bytes the caller reads.
type toServer struct {
	srv    *httptest.Server
	read   *int64
	method *string
	path   *string
}

func (t toServer) RoundTrip(r *http.Request) (*http.Response, error) {
	*t.method, *t.path = r.Method, r.URL.Path
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host, r.Host = "http", strings.TrimPrefix(t.srv.URL, "http://"), ""
	resp, err := t.srv.Client().Transport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	resp.Body = countingBody{ReadCloser: resp.Body, n: t.read}
	return resp, nil
}

// overCapClient answers every request with status and a body far longer than the lowered cap,
// and reports the method, path and response-body bytes the caller read.
func overCapClient(t *testing.T, status int, body string) (c *http.Client, method, path *string, read *int64) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body+strings.Repeat(" ", 64<<10))
	}))
	t.Cleanup(srv.Close)
	method, path, read = new(string), new(string), new(int64)
	return &http.Client{Transport: toServer{srv: srv, read: read, method: method, path: path}}, method, path, read
}

// assertCapRefusal: the call failed closed with the cap error, having read at most cap+1 bytes.
func assertCapRefusal(t *testing.T, err error, read, limit int64) {
	t.Helper()
	if err == nil {
		t.Fatal("an over-cap body read as success")
	}
	if !strings.Contains(err.Error(), "-byte cap") {
		t.Errorf("error = %q; want the response-body cap refusal", err)
	}
	if read > limit+1 {
		t.Errorf("read %d response-body bytes; want at most cap+1 (%d)", read, limit+1)
	}
}

// The POST path through githubDo (the read-only mint's access_tokens exchange, with a request
// body) goes through the cap.
//
// regression: #6780
func TestExchangeJWTBodyOverCapBody(t *testing.T) {
	const limit = 64
	lowCap(t, limit)
	c, method, path, read := overCapClient(t, http.StatusCreated,
		`{"token":"example-minted","expires_at":"2099-01-01T00:00:00Z"}`)
	res, err := exchangeJWTBody(c, "example-jwt", "42", []byte(`{"repositories":["example-repo"]}`))
	if *method != "POST" || *path != "/app/installations/42/access_tokens" {
		t.Fatalf("request = %s %s; want POST /app/installations/42/access_tokens", *method, *path)
	}
	if res != nil {
		t.Errorf("an over-cap mint answer returned a result: %+v", res)
	}
	assertCapRefusal(t, err, *read, limit)
}

// The DELETE path through githubDo (revoking an installation credential) goes through the cap.
//
// regression: #6780
func TestRevokeInstallationTokenOverCapBody(t *testing.T) {
	const limit = 64
	lowCap(t, limit)
	c, method, path, read := overCapClient(t, http.StatusOK, `{}`)
	err := RevokeInstallationToken(c, "example-installation-credential")
	if *method != "DELETE" || *path != "/installation/token" {
		t.Fatalf("request = %s %s; want DELETE /installation/token", *method, *path)
	}
	assertCapRefusal(t, err, *read, limit)
	if err != nil && strings.Contains(err.Error(), "example-installation-credential") {
		t.Errorf("the revoked credential appeared in the error")
	}
}
