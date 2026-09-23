// convert.go — the code→conversion half of the manifest flow (design.md §3):
// `POST /app-manifests/{code}/conversions`, valid for the one hour GitHub gives a code. The
// response carries the private key and every other secret the App is born with, so this
// file's one job outside the HTTP call is to NEVER let that response reach a log line, an
// error string, or anything that could print it — mutations.json's "log the conversion
// response body" mutant targets exactly this file, and secrets_test.go's
// TestNoSecretInLogs is the guard that must catch it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// githubAPIBase is a test hook: production talks to the real GitHub API; tests point it at
// an httptest.Server that fakes the conversions endpoint. Never contacted for --dry-run.
var githubAPIBase = "https://api.github.com"

// conversionHTTPClient is a test hook, mirroring desktoken's httpClient pattern.
var conversionHTTPClient = &http.Client{Timeout: 20 * time.Second}

// errConversionExpired means the code exists but the conversion 404'd — the code is more
// than an hour old or was already converted (design.md §4: "posted → posted | conversion
// 404 after the hour: code expired, Create again"). No key was written on this path.
var errConversionExpired = errors.New("conversion code expired or already used")

// conversionResult is GitHub's POST /app-manifests/{code}/conversions response, trimmed to
// the fields this brief writes.
type conversionResult struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
	PEM           string `json:"pem"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// convertCodeFn is the indirection callers use (a test hook, like desktoken's httpClient):
// production always resolves to convertCode; tests substitute a fake conversion, including
// one that counts calls to prove a refused callback never reaches it.
var convertCodeFn = convertCode

// convertCode exchanges a one-hour manifest code for the App's credentials.
func convertCode(code string) (*conversionResult, error) {
	// url.PathEscape the code (S-3): it arrives straight off the /callback query string, so a
	// code carrying `/`, `..` or `?` would otherwise re-target the request path. The host is
	// pinned by githubAPIBase and the request carries no Authorization header, so the ceiling
	// is low, but escaping the one interpolated path segment costs nothing.
	reqURL := fmt.Sprintf("%s/app-manifests/%s/conversions", githubAPIBase, url.PathEscape(code))
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := conversionHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errConversionExpired
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// The body is DELIBERATELY not included in this error: a conversion response can
		// carry the pem/client_secret even on an unexpected status, and an error string is
		// exactly the kind of place a log line lands unexamined.
		return nil, fmt.Errorf("conversion failed: HTTP %d", resp.StatusCode)
	}

	var cr conversionResult
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("parse conversion response: %w", err)
	}
	return &cr, nil
}
