package main

// flow_test.go — unit coverage for the parts flow_parity_test.go's real-jq comparison does
// not reach: cell resolution, the reader seam's could-not-check shapes, and end-to-end
// `deskinbox flow` CLI dispatch against a stubbed statusgen/deskboard (no real binary, no
// network) — mirroring query_test.go/table_test.go/main_test.go's style for table/walk.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func withLookPath(t *testing.T, found map[string]string) {
	t.Helper()
	prev := lookPathFn
	lookPathFn = func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New("exec: \"" + name + "\": executable file not found in $PATH")
	}
	t.Cleanup(func() { lookPathFn = prev })
}

func withRunReader(t *testing.T, fn func(bin string, args []string) ([]byte, []byte, error)) {
	t.Helper()
	prev := runReaderFn
	runReaderFn = fn
	t.Cleanup(func() { runReaderFn = prev })
}

func TestResolveCells_RootArgsWinOutright(t *testing.T) {
	cells, err := resolveCells([]string{"../a", "../b"}, "/somewhere")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cells) != 2 || cells[0].Path != "../a" || cells[1].Path != "../b" {
		t.Fatalf("want the two --root paths in order, got %+v", cells)
	}
}

func TestResolveCellsCellsTxt(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".assay"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# a comment\n\nrepo-a ../repo-a\nrepo-b\t../repo-b\n../repo-c\n"
	if err := os.WriteFile(filepath.Join(dir, ".assay", "cells.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cells, err := resolveCells(nil, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cells) != 3 {
		t.Fatalf("want 3 cells (comment/blank skipped), got %+v", cells)
	}
	if cells[0].Name != "repo-a" || cells[0].Path != "../repo-a" {
		t.Errorf("cell 0: got %+v", cells[0])
	}
	if cells[1].Name != "repo-b" || cells[1].Path != "../repo-b" {
		t.Errorf("cell 1: got %+v", cells[1])
	}
	if cells[2].Path != "../repo-c" || cells[2].Name != "repo-c" {
		t.Errorf("cell 2 (bare path, named by basename): got %+v", cells[2])
	}
}

func TestResolveCells_FallsBackToDot(t *testing.T) {
	dir := t.TempDir()
	cells, err := resolveCells(nil, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cells) != 1 || cells[0].Path != "." {
		t.Fatalf("want a single '.' cell, got %+v", cells)
	}
}

func TestCellNameFor_BasenameFallback(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "not-a-git-repo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := cellNameFor(sub); got != "not-a-git-repo" {
		t.Errorf("want basename fallback, got %q", got)
	}
}

func TestReadJSON_NonZeroExit_IsCouldNotCheck(t *testing.T) {
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		return nil, []byte("assay-config: routine banner\nflag provided but not defined: -bottleneck\n"),
			&fakeExitError{code: 2}
	})
	ok, errText, raw := readJSON("statusgen --bottleneck (x)", "statusgen", []string{"--bottleneck", "--json"})
	if ok {
		t.Fatalf("want !ok, got ok with raw=%s", raw)
	}
	if errText != "flag provided but not defined: -bottleneck" {
		t.Errorf("want the first non-assay-config stderr line, got %q", errText)
	}
}

func TestReadJSON_ExitZeroNonJSON_IsCouldNotCheck(t *testing.T) {
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		return []byte("not json at all"), nil, nil
	})
	ok, errText, _ := readJSON("deskboard throughput", "deskboard", []string{"throughput", "--json"})
	if ok {
		t.Fatal("want !ok for non-JSON stdout")
	}
	if !strings.Contains(errText, "not JSON") {
		t.Errorf("want a not-JSON diagnostic, got %q", errText)
	}
}

func TestReadJSON_ExitZeroValidJSON_IsOK(t *testing.T) {
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		return []byte(`{"state":"measured","untriaged":3}`), nil, nil
	})
	ok, errText, raw := readJSON("statusgen --intake-debt (x)", "statusgen", []string{"--intake-debt", "--json"})
	if !ok || errText != "" {
		t.Fatalf("want ok, got ok=%v errText=%q", ok, errText)
	}
	if string(raw) != `{"state":"measured","untriaged":3}` {
		t.Errorf("unexpected raw: %s", raw)
	}
}

// fakeExitError is a stand-in `error` for a stubbed runReaderFn's return — readJSON's tests
// here all supply non-empty stderr, so its firstDiagnosticLine finds a diagnostic before the
// *exec.ExitError-specific "exit N, no diagnostic" branch is ever reached; that fallback
// fires only on a real *exec.ExitError with empty stderr, which is exercised implicitly by
// the parity tests' real-jq runs never producing it (jq always writes a usage line on
// misuse) rather than a dedicated unit test here.
type fakeExitError struct{ code int }

func (e *fakeExitError) Error() string { return "exit status " + strconv.Itoa(e.code) }

func TestRunFlow_NoDiagnostic_FallsBackToRealError(t *testing.T) {
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		return nil, nil, errors.New("boom")
	})
	ok, errText, _ := readJSON("statusgen --bottleneck (x)", "statusgen", nil)
	if ok {
		t.Fatal("want !ok")
	}
	if errText != "boom" {
		t.Errorf("want the runner error's own message as the fallback diagnostic, got %q", errText)
	}
}

func TestCountFlowFailures(t *testing.T) {
	doc := rawDoc{
		Cells: []cellRaw{
			{Bottleneck: &bottleneckJSON{}, Intake: nil, NetFlow: &netFlowJSON{}},
			{Bottleneck: nil, Intake: nil, NetFlow: nil},
		},
		Throughput: nil,
	}
	// cell 0: intake nil -> 1; cell 1: all three nil -> 3; throughput nil -> 1. total 5.
	if got := countFlowFailures(doc); got != 5 {
		t.Errorf("want 5 failures, got %d", got)
	}
}

// TestRunFlowEndToEnd drives `deskinbox flow` through run() with stubbed binary resolution
// and reader output — no real statusgen/deskboard, no network.
func TestRunFlowEndToEnd(t *testing.T) {
	withLookPath(t, map[string]string{"statusgen": "/fake/statusgen", "deskboard": "/fake/deskboard"})
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		switch {
		case contains(args, "--bottleneck"):
			return []byte(`{"constraint":"todo","stages":[{"stage":"todo","wip":1,"median_dwell":"1d","unknown_dwell":0}]}`), nil, nil
		case contains(args, "--intake-debt"):
			return []byte(`{"state":"measured","untriaged":0}`), nil, nil
		case contains(args, "--net-flow"):
			return []byte(`{"state":"ok","streams":[]}`), nil, nil
		case contains(args, "throughput"):
			return []byte(`{"bottleneck":"dispatch","stagesRead":4,"stagesTotal":4,"advice":"steady","stages":[]}`), nil, nil
		}
		return nil, nil, errors.New("unexpected reader invocation")
	})

	var stdout, stderr bytes.Buffer
	rc := run([]string{"flow", "--root", "."}, &stdout, &stderr, time.Now())
	if rc != 0 {
		t.Fatalf("want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"pipeline flow", "todo *", "bottleneck (largest queue/slots ratio): todo", "deskinbox: flow across 1 cell(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("flow output missing %q:\n%s", want, out)
		}
	}
}

func TestRunFlow_InvalidSince_Refused(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := run([]string{"flow", "--since", "not-a-date"}, &stdout, &stderr, time.Now())
	if rc != 5 {
		t.Fatalf("want refused (5), got %d", rc)
	}
	if !strings.Contains(stderr.String(), "--since must be YYYY-MM-DD") {
		t.Errorf("want a --since format refusal, got %q", stderr.String())
	}
}

func TestRunFlowHTML_WritesSelfContainedFile(t *testing.T) {
	withLookPath(t, map[string]string{"statusgen": "/fake/statusgen", "deskboard": "/fake/deskboard"})
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		switch {
		case contains(args, "--bottleneck"):
			return []byte(`{"constraint":"","stages":[]}`), nil, nil
		case contains(args, "--intake-debt"):
			return []byte(`{"state":"measured","untriaged":0}`), nil, nil
		case contains(args, "--net-flow"):
			return []byte(`{"state":"ok","streams":[]}`), nil, nil
		case contains(args, "throughput"):
			return []byte(`{"bottleneck":"","stagesRead":0,"stagesTotal":4,"advice":"nothing to widen","stages":[]}`), nil, nil
		}
		return nil, nil, errors.New("unexpected reader invocation")
	})

	dir := t.TempDir()
	out := filepath.Join(dir, "flow.html")
	var stdout, stderr bytes.Buffer
	rc := run([]string{"flow", "--html", out, "--root", "."}, &stdout, &stderr, time.Now())
	if rc != 0 {
		t.Fatalf("want exit 0, got %d; stderr=%s", rc, stderr.String())
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("expected the flow page to be written: %v", err)
	}
	for _, forbidden := range []string{"url(", "<script", " src="} {
		if strings.Contains(string(page), forbidden) {
			t.Errorf("flow page contains forbidden substring %q", forbidden)
		}
	}
	if !strings.Contains(string(page), "<title>Assay inbox — pipeline flow</title>") {
		t.Error("flow page missing its own title")
	}
}

func TestRunFlow_ReaderFailure_IsUnverifiable(t *testing.T) {
	withLookPath(t, map[string]string{"statusgen": "/fake/statusgen", "deskboard": "/fake/deskboard"})
	withRunReader(t, func(bin string, args []string) ([]byte, []byte, error) {
		return nil, []byte("flag provided but not defined: -bottleneck"), &fakeExitError{code: 2}
	})
	var stdout, stderr bytes.Buffer
	rc := run([]string{"flow", "--root", "."}, &stdout, &stderr, time.Now())
	if rc != 6 {
		t.Fatalf("want unverifiable (6), got %d", rc)
	}
	if !strings.Contains(stdout.String(), "FAILED") {
		t.Errorf("want the summary to say readers failed, got %q", stdout.String())
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
