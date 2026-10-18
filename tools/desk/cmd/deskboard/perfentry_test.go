package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type perfNoNetwork struct{}

func (perfNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("fixture forbids network")
}

// Only the far side of the production Forge client is replaced. Every request
// is witnessed, and any route beyond this bounded fixture fails closed.
type perfFilesTransport struct{ home string }

func (p perfFilesTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "example.invalid" || r.Method != http.MethodGet {
		return nil, fmt.Errorf("unexpected fixture request %s %s", r.Method, r.URL)
	}
	var body string
	switch r.URL.Path {
	case "/repos/example-org/tracker/pulls/77":
		body = `{"number":77,"state":"open","changed_files":1,"head":{"sha":"abc123","ref":"feat/example"}}`
	case "/repos/example-org/tracker/pulls/77/files":
		body = `[{"filename":"example.go","status":"modified","additions":1,"deletions":0}]`
	default:
		return nil, fmt.Errorf("unexpected fixture route %s", r.URL.Path)
	}
	f, err := os.OpenFile(filepath.Join(p.home, "work.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	err = json.NewEncoder(f).Encode(r.Method + " " + r.URL.RequestURI())
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

// Runs main itself in a disposable test process: os.Exit is part of the contract.
func TestPerfEntrypoint(t *testing.T) {
	raw := os.Getenv("ASSAY_PERF_TEST_ARGS")
	if raw == "" {
		t.Skip("entrypoint subprocess only")
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", os.Getenv("ASSAY_PERF_TEST_HOME"))
	deskkit.ReloadConfig()
	http.DefaultTransport = perfNoNetwork{}
	if os.Getenv("ASSAY_PERF_TEST_WORK") == "1" {
		mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
		forgeAPIBase = "https://example.invalid"
		http.DefaultTransport = perfFilesTransport{os.Getenv("ASSAY_PERF_TEST_HOME")}
	}
	os.Args = append([]string{"deskboard"}, args...)
	main()
}
