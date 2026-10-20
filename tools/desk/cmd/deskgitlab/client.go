package main

// client.go — the HTTP paths this command uses.
//
// CREDENTIAL CUSTODY. The bash provisioner kept every PAT off argv by handing curl a 0600
// config file (issue #786). Here there is no subprocess at all: the credential is a request
// HEADER (GitLab's PRIVATE-TOKEN) on an http.Request built in-process, held in a struct field,
// never in an environment variable this command sets, never in a URL, and never formatted
// into any output or error.
//
// TRANSPORT FAILURES are RECORDED, never fatal by themselves (issue #786): a request that
// gets no HTTP answer reports the status "000", exactly as the script's `|| echo "000"`
// sentinel did, and flows to the caller's status branch like any other non-2xx status.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxResponseBytes bounds a response read; nothing this command reads is near it.
const maxResponseBytes = 4 << 20

// transportFailed is the status a request that got no HTTP answer reports.
const transportFailed = "000"

// newHTTPClient is the transport every request uses: the environment's proxy settings, and a
// per-request deadline so a server that never answers cannot hang a provisioning run.
var newHTTPClient = func() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment}}
}

type apiClient struct {
	base  string // ends in /api/v4, no trailing slash
	http  *http.Client
	token string // NEVER formatted into any output, error, URL or argv
}

// newAPIClient returns a client for the instance at gitlabURL, authenticating as token.
// It never follows a redirect: net/http forwards a custom header such as PRIVATE-TOKEN to
// whatever host a Location names, so a 3xx is returned as a non-2xx status instead.
func newAPIClient(gitlabURL, token string) *apiClient {
	c := *newHTTPClient()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &apiClient{base: strings.TrimRight(gitlabURL, "/") + "/api/v4", http: &c, token: token}
}

// call sends one request with an optional JSON body and returns the status as the script
// printed it (three digits, or "000" for no answer) and the response body.
func (c *apiClient) call(method, path string, body []byte) (string, []byte) {
	if body == nil {
		return c.send(method, path, "", nil)
	}
	return c.send(method, path, "application/json", bytes.NewReader(body))
}

// putFile sends one multipart/form-data request carrying a single file field — the shape of
// GitLab's PUT /user/avatar.
func (c *apiClient) putFile(path, field, filename string, data []byte) (string, []byte) {
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
		return transportFailed, nil
	}
	return c.send(http.MethodPut, path, mw.FormDataContentType(), &buf)
}

func (c *apiClient) send(method, path, contentType string, rdr io.Reader) (string, []byte) {
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return transportFailed, nil
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return transportFailed, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	return strconv.Itoa(resp.StatusCode), b
}

// fetchIcon downloads one public role icon. Unlike the API client it follows redirects (the
// script used curl -L) and sends no credential at all. Any failure, a non-2xx status or an
// empty body returns nil.
func fetchIcon(rawURL string) []byte {
	resp, err := newHTTPClient().Get(rawURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

// validateGitLabURL refuses a base URL that would carry a credential in the clear. HTTPS is
// required; plain HTTP is accepted only for a loopback host (a local test stub).
func validateGitLabURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("error: --gitlab-url %q is not an absolute base URL (want e.g. https://gitlab.example.invalid)", raw)
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
	return fmt.Errorf("error: --gitlab-url %q is not https — refusing to send a credential over an unencrypted "+
		"connection (plain http is accepted only for a loopback test stub)", raw)
}

// uriEncode is jq's @uri: every byte outside the unreserved set A-Z a-z 0-9 - _ . ~ is
// percent-encoded with upper-case hex, so a group path such as a/b becomes a%2Fb.
func uriEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// --- JSON reading, with the script's jq semantics ---------------------------------------
//
// The script read every response with `jq -r`, and the comparisons it made are on that
// rendering: a number prints as written, a string raw, null as "null". `A // B` takes B when A
// is null or false (never 0). `has(k)` tests for the KEY, so a false value is not "absent".
// IDs went back out through `--argjson`, i.e. as the raw JSON text they were read as.

// decodeJSON parses a response body, keeping numbers as written. ok=false when it is not JSON.
func decodeJSON(b []byte) (any, bool) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, false
	}
	return v, true
}

// path walks objects by key and arrays by index (an int), returning nil for anything absent.
func path(v any, steps ...any) any {
	for _, s := range steps {
		switch k := s.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[k]
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				return nil
			}
			v = a[k]
		}
	}
	return v
}

// render is `jq -r` of one value.
func render(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// alt is `jq -r 'X // "fallback"'`.
func alt(v any, fallback string) string {
	if v == nil || v == false {
		return fallback
	}
	return render(v)
}

// hasString is `jq -r 'if has(k) then (.k|tostring) else "fallback" end'`.
func hasString(v any, key, fallback string) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fallback
	}
	val, ok := m[key]
	if !ok {
		return fallback
	}
	return render(val)
}

// firstMatch is `jq -r '.[] | select(.field==want) | .out' | head -1`: "" when nothing matches
// or the body is not a list.
func firstMatch(body []byte, field, want, out string) string {
	v, ok := decodeJSON(body)
	if !ok {
		return ""
	}
	list, ok := v.([]any)
	if !ok {
		return ""
	}
	for _, e := range list {
		if f, ok := path(e, field).(string); ok && f == want {
			return render(path(e, out))
		}
	}
	return ""
}

// --- request bodies -----------------------------------------------------------------------

// raw is a value sent as raw JSON text, the script's `--argjson`.
type raw string

// object builds a JSON object with its keys in the order given, as jq writes them.
// Values are strings, bools, raw, []string, or nested object/array results.
func object(kv ...any) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i := 0; i+1 < len(kv); i += 2 {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(kv[i].(string))
		b.Write(k)
		b.WriteByte(':')
		writeValue(&b, kv[i+1])
	}
	b.WriteByte('}')
	return b.Bytes()
}

// array builds a JSON array of objects (each already encoded by object).
func array(items ...[]byte) []byte {
	return append(append([]byte{'['}, bytes.Join(items, []byte{','})...), ']')
}

// writeValue encodes one value. A raw value that is not valid JSON (an id the server answered
// as something other than a number) is sent as a JSON string, so the request stays
// well-formed and the server's refusal is recorded like any other non-2xx answer — where the
// script's jq crashed mid-run.
func writeValue(b *bytes.Buffer, v any) {
	switch t := v.(type) {
	case raw:
		if json.Valid([]byte(t)) {
			b.WriteString(string(t))
			return
		}
		e, _ := json.Marshal(string(t))
		b.Write(e)
	case []byte:
		b.Write(t)
	default:
		e, _ := json.Marshal(t)
		b.Write(e)
	}
}
