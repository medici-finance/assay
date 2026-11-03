package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// twoCellConfig has two cells with disjoint loops and repositories, so a
// cross-cell or cross-repo call has something real to be refused against.
const twoCellConfig = `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.2]
        app: assay-review-app
  - name: other
    repos: [example-org/other-repo]
    streams_root: docs/streams
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
    loops:
      - role: dispatch
        schedule: "*/10 * * * *"
        models: [glm-5.2]
        app: other-worker-app
`

func twoCellServer(t *testing.T, filer Filer) *Server {
	t.Helper()
	cfg, err := schema.Parse([]byte(twoCellConfig))
	if err != nil {
		t.Fatalf("parse two-cell config: %v", err)
	}
	fixed := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	return NewServer(Options{Config: cfg, Dir: index.NewDirectory(cfg), Filer: filer, Now: func() time.Time { return fixed }})
}

func wantForbidden(t *testing.T, what string, err error) {
	t.Helper()
	var f *ErrForbidden
	if !errors.As(err, &f) {
		t.Fatalf("%s: got err=%v, want a forbidden refusal before the handler ran", what, err)
	}
}

func escalateArgs(t *testing.T, cell, repo string) Call {
	m := map[string]any{"cell": cell, "kind": "question", "title": "t", "situation": "s"}
	if repo != "" {
		m["repo"] = repo
	}
	return Call{Args: mustArgs(t, m)}
}

// regression: F-security-access — an authenticated login that is no loop of
// the cell filed an escalation into a repository of its choosing.
func TestEscalateRefusesCallerOutsideCell(t *testing.T) {
	f := &fakeFiler{}
	s := twoCellServer(t, f)

	call := escalateArgs(t, "assay", "")
	call.CallerLogin = "unrelated-user"
	_, err := s.Dispatch(context.Background(), "desk__escalate", call)
	wantForbidden(t, "an unrelated login escalating", err)

	// A loop of ANOTHER cell is not a loop of this one.
	call.CallerLogin = "other-worker-app[bot]"
	_, err = s.Dispatch(context.Background(), "desk__escalate", call)
	wantForbidden(t, "another cell's loop escalating", err)

	// An empty caller (loopback with no X-Desk-Caller) joins nothing.
	call.CallerLogin = ""
	_, err = s.Dispatch(context.Background(), "desk__escalate", call)
	wantForbidden(t, "an anonymous loopback caller escalating", err)

	if len(f.filed) != 0 {
		t.Fatalf("a refused caller still filed %d issue(s): %+v", len(f.filed), f.filed)
	}
}

// regression: F-security-access — the filing repository was taken from the
// arguments with no check that it belongs to the cell.
func TestEscalateRefusesRepoOutsideCell(t *testing.T) {
	f := &fakeFiler{}
	s := twoCellServer(t, f)
	for _, repo := range []string{"example-org/other-repo", "someone/anything"} {
		call := escalateArgs(t, "assay", repo)
		call.CallerLogin = reviewCaller
		_, err := s.Dispatch(context.Background(), "desk__escalate", call)
		wantForbidden(t, "escalating into "+repo, err)
		rep := Call{Args: mustArgs(t, map[string]any{"cell": "assay", "repo": repo, "classification": "bad", "summary": "x"}), CallerLogin: reviewCaller}
		_, err = s.Dispatch(context.Background(), "desk__report_file", rep)
		wantForbidden(t, "reporting into "+repo, err)
	}
	if len(f.filed) != 0 {
		t.Fatalf("an out-of-cell repository was still filed into: %+v", f.filed)
	}

	// Positive control: the cell's own loop, its own repository (any case).
	call := escalateArgs(t, "assay", "Example-Org/Toolkit")
	call.CallerLogin = reviewCaller
	if _, err := s.Dispatch(context.Background(), "desk__escalate", call); err != nil {
		t.Fatalf("the cell's own loop filing into its own repo was refused: %v", err)
	}
	if len(f.filed) != 1 {
		t.Fatalf("the authorized escalation did not file: %+v", f.filed)
	}
}

// regression: F-security-access — the read tools served any cell's items to
// any authenticated caller, and an empty cell read every cell.
func TestReadToolsRefuseCrossCell(t *testing.T) {
	s := twoCellServer(t, nil)
	reads := []string{"desk__inbox_list", "desk__item_get", "desk__status", "desk__metrics_query", "desk__overrides", "desk__desks", "desk__claim"}
	for _, tool := range reads {
		args := map[string]any{"cell": "other", "id": "x", "item": "x", "scope": "program"}
		_, err := s.Dispatch(context.Background(), tool, Call{Args: mustArgs(t, args), CallerLogin: reviewCaller})
		wantForbidden(t, tool+" on another cell", err)

		delete(args, "cell")
		_, err = s.Dispatch(context.Background(), tool, Call{Args: mustArgs(t, args), CallerLogin: reviewCaller})
		wantForbidden(t, tool+" with no cell (all cells)", err)
	}

	// Positive control: the caller's own cell is served.
	res, err := s.Dispatch(context.Background(), "desk__desks",
		Call{Args: mustArgs(t, map[string]any{"cell": "assay"}), CallerLogin: reviewCaller})
	if err != nil || res == nil {
		t.Fatalf("the caller's own cell was refused: %v", err)
	}
}

// TestEveryDataToolIsCellScoped is the planted-second-instance guard for the
// access defect class: a tool that reads the index or writes the platform can
// only be registered cell-scoped. ScopeNone is reserved for the delegated
// reads, which serve a fixed verdict and no cell data.
func TestEveryDataToolIsCellScoped(t *testing.T) {
	unscoped := map[string]bool{
		"desk__roadmap": true, "desk__portfolio": true, "desk__team_view": true, "desk__improve_list": true,
	}
	for _, tool := range registry() {
		if tool.Scope == ScopeNone && !unscoped[tool.Name] {
			t.Errorf("%s is registered ScopeNone but is not a delegated read — it would serve or write without cell authorization", tool.Name)
		}
		if tool.Scope == ScopeNone && tool.Mutates {
			t.Errorf("%s mutates yet is ScopeNone", tool.Name)
		}
	}
	// The planted instance: a new data tool that forgot to think about
	// authorization gets the zero value and is refused without a cell.
	s := twoCellServer(t, nil)
	s.register(append(s.Tools(), Tool{Name: "probe__data", Description: "planted",
		Handler: func(context.Context, *Server, Call) (any, error) { return "served", nil }}))
	_, err := s.Dispatch(context.Background(), "probe__data", Call{Args: mustArgs(t, map[string]any{"cell": "other"}), CallerLogin: reviewCaller})
	wantForbidden(t, "a planted tool registered with the zero Scope", err)
}
