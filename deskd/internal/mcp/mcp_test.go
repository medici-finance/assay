package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// TestUnknownToolIsError is the package-level backstop for Verify row 2:
// dispatching a tool that does not exist (desk__answer) is a first-class error,
// never a fabricated empty success. deskcli mcp-call turns this error into a
// non-zero exit.
func TestUnknownToolIsError(t *testing.T) {
	cfg, _ := schema.Parse([]byte(testConfig))
	s := NewServer(Options{Config: cfg, Dir: index.NewDirectory(cfg)})
	for _, name := range []string{"desk__answer", "desk__adopt", "desk__nope"} {
		_, err := s.Dispatch(context.Background(), name, Call{})
		if err == nil {
			t.Fatalf("dispatching %q must error (the tool does not exist)", name)
		}
		var unknown *ErrUnknownTool
		if !isUnknownTool(err, &unknown) {
			t.Fatalf("%q should be ErrUnknownTool, got %T", name, err)
		}
	}
}

// TestJSONRPCToolsListAndCall exercises the MCP JSON-RPC surface: tools/list
// advertises the registry and an unknown tools/call returns a -32601 error.
func TestJSONRPCSurface(t *testing.T) {
	cfg, _ := schema.Parse([]byte(testConfig))
	s := NewServer(Options{Config: cfg, Dir: index.NewDirectory(cfg)})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// tools/list
	list := rpc(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if list.Error != nil {
		t.Fatalf("tools/list errored: %+v", list.Error)
	}
	raw, _ := json.Marshal(list.Result)
	for _, want := range []string{"desk__escalate", "desk__claim", "desk__status", "desk__metrics_query"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("tools/list missing %q:\n%s", want, raw)
		}
	}
	// The two absences must not be advertised.
	for _, absent := range []string{"desk__answer", "desk__adopt"} {
		if strings.Contains(string(raw), absent) {
			t.Fatalf("tools/list advertises the forbidden tool %q", absent)
		}
	}

	// tools/call on an unknown tool → JSON-RPC method-not-found.
	call := rpc(t, srv.URL, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"desk__answer"}}`)
	if call.Error == nil || call.Error.Code != -32601 {
		t.Fatalf("unknown tools/call should be -32601, got %+v", call.Error)
	}
}

func rpc(t *testing.T, url, body string) rpcResponse {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var out rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}
