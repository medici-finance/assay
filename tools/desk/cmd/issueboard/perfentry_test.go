package main

import (
	"encoding/json"
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"net/http"
	"os"
	"testing"
)

type perfNoNetwork struct{}

func (perfNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("fixture forbids network")
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
	os.Args = append([]string{"issueboard"}, args...)
	main()
}
