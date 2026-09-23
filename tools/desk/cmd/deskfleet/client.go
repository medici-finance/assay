package main

// client.go — the one HTTP path this verb uses to reach a forge.
//
// CREDENTIAL CUSTODY. The credential is carried as a request HEADER (GitLab `PRIVATE-TOKEN`,
// GitHub `Authorization: Bearer`) on an http.Request built in-process. It is never a
// subprocess argument (this package imports no os/exec), never an environment variable (it
// is read from a file and held in a struct field), and never interpolated into a URL. No
// error this file builds formats the token or a request header: a transport error names the
// method and path only, and a forge error names the status plus, where the forge sent one,
// its short `message` — and never that for the token-minting endpoint, whose success body IS
// a credential.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// maxResponseBytes bounds a forge response read; nothing this verb reads is near it.
const maxResponseBytes = 4 << 20

type apiClient struct {
	base   string // no trailing slash
	http   *http.Client
	header string // header name that carries the credential
	scheme string // value prefix ("" for GitLab, "Bearer " for GitHub)
	token  string // NEVER formatted into any output, error, URL or argv
	accept string // optional Accept header
}

func newGitLabClient(base string, hc *http.Client, token string) *apiClient {
	return &apiClient{base: strings.TrimRight(base, "/"), http: noRedirects(hc), header: "PRIVATE-TOKEN", token: token}
}

func newGitHubClient(base string, hc *http.Client, token string) *apiClient {
	return &apiClient{base: strings.TrimRight(base, "/"), http: noRedirects(hc), header: "Authorization",
		scheme: "Bearer ", token: token, accept: "application/vnd.github+json"}
}

// noRedirects returns a copy of hc that never follows a redirect. net/http strips the
// standard Authorization header on a cross-host redirect, but it FORWARDS a custom header —
// GitLab's PRIVATE-TOKEN among them — so a followed redirect could hand the credential to
// whatever host the Location names. A 3xx is returned to the caller as a non-2xx response
// (a failure), never followed.
func noRedirects(hc *http.Client) *http.Client {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

type apiResponse struct {
	Status int
	Body   []byte
}

// do sends one request. A transport failure is returned as an error naming method and path
// only; any HTTP status (including non-2xx) is returned as a response for the caller to judge.
func (c *apiClient) do(method, path string, body any) (apiResponse, error) {
	if body == nil {
		return c.send(method, path, "", nil)
	}
	b, err := json.Marshal(body)
	if err != nil {
		return apiResponse{}, fmt.Errorf("%s %s: encode request: %v", method, path, err)
	}
	return c.send(method, path, "application/json", bytes.NewReader(b))
}

// doFile sends one multipart/form-data request carrying a single file field — the shape of
// GitLab's PUT /user/avatar. Error and response handling are exactly do's.
func (c *apiClient) doFile(method, path, field, filename string, data []byte) (apiResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err == nil {
		_, err = fw.Write(data)
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		return apiResponse{}, fmt.Errorf("%s %s: encode request: %v", method, path, err)
	}
	return c.send(method, path, mw.FormDataContentType(), &buf)
}

func (c *apiClient) send(method, path, contentType string, rdr io.Reader) (apiResponse, error) {
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return apiResponse{}, fmt.Errorf("%s %s: build request: %v", method, path, redactURLError(err))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.accept != "" {
		req.Header.Set("Accept", c.accept)
	}
	req.Header.Set(c.header, c.scheme+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return apiResponse{}, fmt.Errorf("%s %s: %v", method, path, redactURLError(err))
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return apiResponse{Status: resp.StatusCode}, fmt.Errorf("%s %s: read response: %v", method, path, err)
	}
	return apiResponse{Status: resp.StatusCode, Body: b}, nil
}

// redactURLError strips a *url.Error down to its underlying cause: the URL it carries is the
// caller's own base+path (never a credential — the credential is a header), but the cause is
// all a reader needs and keeping the message to it keeps every error this file builds to
// "method path: cause".
func redactURLError(err error) error {
	if ue, ok := err.(*url.Error); ok {
		return ue.Err
	}
	return err
}

// forgeMessage extracts the forge's short human-readable reason from an error response, for
// diagnostics. It returns "" when the body carries none. Callers never pass it a
// token-minting response.
func forgeMessage(body []byte) string {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, k := range []string{"message", "error"} {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if len(s) > 300 {
				s = s[:300] + "…"
			}
			return s
		}
	}
	return ""
}

// statusText renders a non-2xx response for a failure line: the status and, when present,
// the forge's own message.
func statusText(r apiResponse) string {
	if msg := forgeMessage(r.Body); msg != "" {
		return fmt.Sprintf("HTTP %d: %s", r.Status, msg)
	}
	return fmt.Sprintf("HTTP %d", r.Status)
}

// validateAPIBase refuses a base URL that would carry a credential in the clear. HTTPS is
// required; plain HTTP is accepted only for a loopback host (a local stub).
func validateAPIBase(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s=%q is not an absolute URL (want e.g. https://gitlab.example.com/api/v4)", name, raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		h := u.Hostname()
		if h == "127.0.0.1" || h == "::1" || h == "localhost" {
			return nil
		}
	}
	return fmt.Errorf("%s=%q is not https — refusing to send a credential over an unencrypted "+
		"connection (plain http is accepted only for a loopback test stub)", name, raw)
}
