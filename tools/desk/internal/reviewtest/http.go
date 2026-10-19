package reviewtest

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Attempt is recorded below the production provider adapter, once per HTTP attempt.
// Repetition is retained; without a client retry marker it is NOT called a retry.
// The fake provider supplies no billable quota data. Nil points means unknown.
type Attempt struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Query    string `json:"query"`
	BodyHash string `json:"bodyHash"`
	Status   int    `json:"status"`
	Points   *int64 `json:"points"`
	Retry    *bool  `json:"retry"`
}

// Transport answers HTTP locally, without opening any socket or resolving any host.
// The production GitHubForge builds and parses the requests around this boundary.
type Transport struct {
	mu       sync.Mutex
	Attempts []Attempt
	Answer   func(*http.Request) (int, string)
}

func (f *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(body)))
	}
	status, data := f.Answer(r)
	f.mu.Lock()
	f.Attempts = append(f.Attempts, Attempt{Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, BodyHash: fmt.Sprintf("%x", sha256.Sum256(body)), Status: status})
	f.mu.Unlock()
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(data)), Request: r}, nil
}

func (f *Transport) Snapshot() []Attempt {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Attempt(nil), f.Attempts...)
}
