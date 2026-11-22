package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write refused") }

// regression: #7404
func TestTableReturnsWriteError(t *testing.T) {
	want := errors.New("write refused")
	withErr := &Report{renderTo: func(io.Writer) error { return want }}
	if err := withErr.table(&bytes.Buffer{}); !errors.Is(err, want) {
		t.Fatalf("table dropped the renderer's write error: %v", err)
	}
	called := false
	plain := &Report{render: func(io.Writer) { called = true }}
	if err := plain.table(&bytes.Buffer{}); err != nil || !called {
		t.Fatalf("render fallback: err=%v called=%t", err, called)
	}
}

// regression: #7404
func TestChangeTableWriteFailure(t *testing.T) {
	newEtagForge(t, 0)
	const repo = "example-org/tracker"
	t.Setenv("DESK_LOOP", "worker-desk")
	mintTokenFn = func(role, slug string) (string, string, error) {
		return "fixture-token", "", nil
	}
	head := strings.Repeat("a", 40)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch r.URL.Path {
		case "/repos/" + repo + "/pulls/7":
			result = map[string]any{"number": 7, "state": "open", "head": map[string]string{"sha": head}}
		case "/repos/" + repo + "/pulls/7/reviews":
			result = []any{}
		case "/repos/" + repo + "/commits/" + head + "/status":
			result = map[string]any{"statuses": []any{}}
		case "/repos/" + repo + "/commits/" + head + "/check-runs":
			result = map[string]any{"total_count": 0, "check_runs": []any{}}
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()
	forgeAPIBase = srv.URL
	var out, errout bytes.Buffer
	if code := run([]string{"change", repo, "7", head, "--table"}, &out, &errout); code != deskkit.ExitOK || !strings.Contains(out.String(), "head="+head) {
		t.Fatalf("table to a working writer: code=%d stdout=%s stderr=%s", code, &out, &errout)
	}
	errout.Reset()
	if code := run([]string{"change", repo, "7", head, "--table"}, failingWriter{}, &errout); code != deskkit.ExitUnverifiable || !strings.Contains(errout.String(), "table write failed") {
		t.Fatalf("a failed table write must exit unverifiable: code=%d stderr=%s", code, &errout)
	}
}
