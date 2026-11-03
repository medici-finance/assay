package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// desksTestConfig has two loops on one cell — enough to exercise the master
// default (dispatch) and a non-master row.
const desksTestConfig = `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.2]
        app: assay-review-app
      - role: dispatch
        schedule: "*/5 * * * *"
        models: [glm-5.2]
        app: assay-desk-app
`

func desksTestServer(t *testing.T) *Server {
	t.Helper()
	cfg, err := schema.Parse([]byte(desksTestConfig))
	if err != nil {
		t.Fatalf("parse test config: %v", err)
	}
	term := terminal.NewRegistry(cfg)
	return NewServer(Options{Config: cfg, Dir: index.NewDirectory(cfg), Terminal: term})
}

// TestDesksTool_RequiresCell — desk__desks with no cell argument is an error,
// never a fabricated cross-cell listing.
func TestDesksTool_RequiresCell(t *testing.T) {
	s := desksTestServer(t)
	if _, err := s.Dispatch(context.Background(), "desk__desks", Call{}); err == nil {
		t.Fatal("desk__desks with no cell must error")
	}
}

// TestDesksTool_UnknownCell — a cell this build does not configure is an
// error, not an empty result.
func TestDesksTool_UnknownCell(t *testing.T) {
	s := desksTestServer(t)
	args, _ := json.Marshal(map[string]string{"cell": "nope"})
	if _, err := s.Dispatch(context.Background(), "desk__desks", Call{Args: args}); err == nil {
		t.Fatal("desk__desks with an unconfigured cell must error")
	}
}

// TestDesksTool_ReturnsRowPerConfiguredLoop — the MCP twin returns the SAME
// shape as the HTTP read: one row per configured loop, master resolved by
// role (spec §5.2's "feature = tool", the desks view Verify row 8's cross-surface
// flow).
func TestDesksTool_ReturnsRowPerConfiguredLoop(t *testing.T) {
	s := desksTestServer(t)
	args, _ := json.Marshal(map[string]string{"cell": "assay"})
	res, err := s.Dispatch(context.Background(), "desk__desks", Call{Args: args, CallerLogin: reviewCaller})
	if err != nil {
		t.Fatalf("desk__desks: %v", err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var out struct {
		Cell  string `json:"cell"`
		Count int    `json:"count"`
		Desks []struct {
			ID    string `json:"id"`
			Role  string `json:"role"`
			Group string `json:"group"`
			State string `json:"state"`
		} `json:"desks"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("result does not decode as the desk registry shape: %v\n%s", err, raw)
	}
	if out.Cell != "assay" || out.Count != 2 || len(out.Desks) != 2 {
		t.Fatalf("desk__desks result = %+v, want cell=assay count=2 desks=2", out)
	}
	var masters int
	for _, d := range out.Desks {
		if d.Group == "master" {
			masters++
			if d.Role != "dispatch" {
				t.Errorf("master row role = %q, want dispatch (the default)", d.Role)
			}
		}
		if d.State != "could-not-check" {
			t.Errorf("row %s state = %q, want could-not-check (no terminal target configured)", d.ID, d.State)
		}
	}
	if masters != 1 {
		t.Fatalf("master rows = %d, want exactly 1", masters)
	}
}

// TestDesksTool_NilTerminalStillProducesRows — a server with no terminal
// registry wired still answers with one could-not-check row per configured
// loop, never a fabricated empty list (three-state, §10).
func TestDesksTool_NilTerminalStillProducesRows(t *testing.T) {
	cfg, err := schema.Parse([]byte(desksTestConfig))
	if err != nil {
		t.Fatalf("parse test config: %v", err)
	}
	s := NewServer(Options{Config: cfg, Dir: index.NewDirectory(cfg)}) // no Terminal
	args, _ := json.Marshal(map[string]string{"cell": "assay"})
	res, err := s.Dispatch(context.Background(), "desk__desks", Call{Args: args, CallerLogin: reviewCaller})
	if err != nil {
		t.Fatalf("desk__desks: %v", err)
	}
	raw, _ := json.Marshal(res)
	var out struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, raw)
	}
	if out.Count != 2 {
		t.Fatalf("count = %d, want 2 even with no terminal registry wired", out.Count)
	}
}
