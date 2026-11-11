package forgelabels

// http.go — the HTTP Transport for a caller that has no forge client of its own (cellctl's
// install and check paths). deskfleet keeps its own client and adapts it to Transport.
//
// CREDENTIAL CUSTODY. The credential is read from a FILE the operator names, under the
// read-side owner-only custody standard, and carried only as a request HEADER (GitHub
// `Authorization: Bearer`, GitLab `PRIVATE-TOKEN`). It is never a URL part, an argument or an
// environment variable, and no error built here formats it. Redirects are refused: net/http
// forwards a custom header such as PRIVATE-TOKEN to whatever host a Location names. The base
// must be https; plain http is accepted only for a loopback test stub.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// maxResponseBytes bounds one forge response read; no label answer is near it.
const maxResponseBytes = 4 << 20

// ValidateAPIBase refuses a base URL that would carry a credential in the clear.
func ValidateAPIBase(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s=%q is not an absolute URL (want e.g. https://api.github.com or https://gitlab.example.com/api/v4)", name, raw)
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

// ReadCredentialFile reads the credential in path, a file the operator named. A link is the
// operator's own layout and is followed; every check judges, and the read opens, the resolved
// file. It must be a regular, owner-only file (deskkit.VerifyCustodyOwnerOnly) and non-empty.
func ReadCredentialFile(name, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is not set (a FILE holding the credential — never a flag value or an environment variable)", name)
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%s %s: %v", name, path, err)
	}
	fi, err := os.Lstat(target)
	if err != nil {
		return "", fmt.Errorf("%s %s: %v", name, path, err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s %s is not a regular file", name, target)
	}
	if err := deskkit.VerifyCustodyOwnerOnly(target, fi); err != nil {
		return "", fmt.Errorf("%s: %v", name, err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("%s %s: %v", name, target, err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("%s %s is empty", name, path)
	}
	return tok, nil
}

// httpTransport is one forge API base plus the header that carries the credential.
type httpTransport struct {
	base   string // no trailing slash
	hc     *http.Client
	header string
	value  string // NEVER formatted into any output, error, URL or argv
	accept string
}

// NewHTTPTransport returns the Transport for forge ("github" or "gitlab") at base, carrying
// token as that forge's credential header over a copy of hc that refuses redirects.
func NewHTTPTransport(forge, base string, hc *http.Client, token string) (Transport, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if err := ValidateAPIBase("forge API base", base); err != nil {
		return nil, err
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	t := &httpTransport{base: base, hc: &c}
	switch forge {
	case "github":
		t.header, t.value, t.accept = "Authorization", "Bearer "+token, "application/vnd.github+json"
	case "gitlab":
		t.header, t.value = "PRIVATE-TOKEN", token
	default:
		return nil, errors.New("forge must be github or gitlab, got " + forge)
	}
	return t, nil
}

func (t *httpTransport) Do(method, path string, body any) (Response, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return Response{}, fmt.Errorf("%s %s: encode request: %v", method, path, err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, t.base+path, rdr)
	if err != nil {
		return Response{}, fmt.Errorf("%s %s: build request: %v", method, path, causeOf(err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if t.accept != "" {
		req.Header.Set("Accept", t.accept)
	}
	req.Header.Set(t.header, t.value)
	resp, err := t.hc.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("%s %s: %v", method, path, causeOf(err))
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Response{Status: resp.StatusCode}, fmt.Errorf("%s %s: read response: %v", method, path, err)
	}
	return Response{Status: resp.StatusCode, Body: b}, nil
}

// causeOf strips a *url.Error to its cause, so every error here reads "method path: cause".
func causeOf(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}
