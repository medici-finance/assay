package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type perfNoNetwork struct{}

func (perfNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("fixture forbids network")
}

type perfStampTransport struct {
	home  string
	forge ghStampServer
}

func (p *perfStampTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "example.invalid" {
		return nil, fmt.Errorf("fixture forbids host %s", r.URL.Host)
	}
	w := httptest.NewRecorder()
	p.forge.handle(w, r)
	// Observe actual requests and read-back state, never derive expected labels.
	data, err := json.Marshal(struct {
		Labels   []string     `json:"labels"`
		Requests []ghStampReq `json:"requests"`
	}{p.forge.labels, p.forge.requests})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(p.home, "work.jsonl"), data, 0600); err != nil {
		return nil, err
	}
	return w.Result(), nil
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
		http.DefaultTransport = &perfStampTransport{home: os.Getenv("ASSAY_PERF_TEST_HOME")}
		deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) {
			return ghStampToken, "https://example.invalid", nil
		})
	}
	os.Args = append([]string{"deskdispatch"}, args...)
	main()
}
