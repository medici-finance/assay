package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// regression: #5371
func TestGCTargetFailureIsolated(t *testing.T) {
	ran := 0
	targets := []gcTarget{
		{name: "a", run: func(context.Context) []gcLine {
			ran++
			return []gcLine{{Target: "a", Outcome: "failed", Detail: "boom"}}
		}},
		{name: "b", run: func(context.Context) []gcLine { ran++; die("refused"); return nil }},
		{name: "c", run: func(context.Context) []gcLine { ran++; panic(errors.New("crash")) }},
		{name: "d", run: func(context.Context) []gcLine { ran++; return []gcLine{{Target: "d", Outcome: "planned"}} }},
	}
	lines := runGC(context.Background(), targets)
	if ran != 4 || len(lines) != 4 {
		t.Fatalf("a failing target stopped the rest: ran=%d lines=%+v", ran, lines)
	}
	for i, want := range []string{"failed", "failed", "failed", "planned"} {
		if lines[i].Outcome != want || lines[i].Target != targets[i].name {
			t.Fatalf("line %d = %+v, want %s", i, lines[i], want)
		}
	}
}

// Every action of a gc plan refuses at run time, so a confirmed apply reclaims nothing; its
// precondition re-measures unchanged, so the refusal is the held answer, not a stale plan.
//
// regression: #5371
func TestGCApplyHeld(t *testing.T) {
	c := &Cell{Name: "fixture", Dir: t.TempDir(), Env: &Env{vals: map[string]string{"CELL_ROOTS": "o/r=/abs/repo"}}}
	actions := c.gcActions()
	if len(actions) != 3 || actions[2].ID != "worktrees:o/r" {
		t.Fatalf("gc plan actions = %+v", actions)
	}
	for _, a := range actions {
		got, err := gcVerb.Measure(c, a)
		if err != nil || len(got) != 1 || got[0] != a.Preconditions[0] {
			t.Fatalf("%s re-measures %v %v, planned %v", a.ID, got, err, a.Preconditions)
		}
		if out, err := gcVerb.Run(c, a); err == nil || !strings.Contains(err.Error(), "held") || out != "" {
			t.Fatalf("%s ran under apply: %q %v", a.ID, out, err)
		}
	}
}

// Plain gc reclaims nothing and calls every target's dry form; the state-changing forms are
// refused off a terminal and reclaim nothing either.
//
// regression: #5371
func TestGCDryRunDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	w := newCLIWorld(t, "")
	root := filepath.Join(w.cellDir, "run", "scratch")
	must(t, os.MkdirAll(root, 0o700))
	build, record, kept := plantAbandoned(t, root)
	argvFile := filepath.Join(w.root, "deskwt-argv")
	must(t, os.WriteFile(filepath.Join(w.deskToolsDir(), "deskwt"), []byte("#!/bin/sh\nprintf '%s %s\\n' \"$DESK_LOOP\" \"$*\" >> '"+argvFile+"'\necho 'prune: fixture done'\n"), 0o755))

	run := w.run(t, nil, "gc", "example", "--role", "worker-desk")
	if run.Code != 0 {
		t.Fatalf("dry run exit %d\n%s\n%s", run.Code, run.Stdout, run.Stderr)
	}
	lines := gcParse(t, run.Stdout)
	if len(lines) != 3 || lines["scratch"].Outcome != "planned" || *lines["scratch"].Items != 1 ||
		lines["go-cache"].Outcome != "skipped" || lines["worktrees:example-org/example-repo"].Outcome != "planned" {
		t.Fatalf("dry-run report = %s", run.Stdout)
	}
	b, err := os.ReadFile(argvFile)
	must(t, err)
	if string(b) != "worker-desk prune --repo "+w.repoDir+" --no-cache --dry-run\n" {
		t.Fatalf("deskwt argv = %q", b)
	}
	// Off a terminal the state-changing forms are refused before anything runs.
	for _, args := range [][]string{{"gc", "example", "--run"}, {"gc", "example", "--apply", "some-plan"}} {
		r := w.run(t, nil, args...)
		if r.Code != 5 || !strings.Contains(r.Stdout+r.Stderr, cellop.ReasonNotInteractive) {
			t.Fatalf("%v off a terminal: exit %d\n%s%s", args, r.Code, r.Stdout, r.Stderr)
		}
	}
	if !scratchExists(t, build) || !scratchExists(t, record) || !scratchExists(t, kept) {
		t.Fatal("gc reclaimed scratch")
	}
	b, err = os.ReadFile(argvFile)
	must(t, err)
	if strings.Count(string(b), "\n") != 1 {
		t.Fatalf("a refused form ran deskwt: %q", b)
	}
	// A failing target is reported, the rest still run, and the exit says so.
	must(t, os.WriteFile(filepath.Join(w.deskToolsDir(), "deskwt"), []byte("#!/bin/sh\necho 'prune: refused'\nexit 4\n"), 0o755))
	run = w.run(t, nil, "gc", "example")
	lines = gcParse(t, run.Stdout)
	if run.Code != 1 || lines["scratch"].Outcome != "planned" || lines["worktrees:example-org/example-repo"].Outcome != "failed" {
		t.Fatalf("failing target: %d %s", run.Code, run.Stdout)
	}
}

func gcParse(t *testing.T, out string) map[string]gcLine {
	t.Helper()
	lines := map[string]gcLine{}
	for _, raw := range strings.Split(strings.TrimSpace(out), "\n") {
		var l gcLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("not a report line: %q (%v)", raw, err)
		}
		lines[l.Target] = l
	}
	return lines
}

// The go-cache line reports the recorded measurement and nothing it cannot know: no forecast
// figure, and an outcome that follows the record, so a partial or missing measurement exits 1.
//
// regression: #5371
func TestGCCacheLine(t *testing.T) {
	size := int64(4096)
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for outcome, want := range map[string]string{"checked-clean": "planned", "storage-deferred": "planned", "could-not-check": "could-not-check", "something-new": "could-not-check"} {
		l := gcCacheLine(cellcache.Report{Outcome: outcome, Reason: "why", Reclaimed: 999, After: cellcache.Measurement{At: at, LogicalBytes: &size}})
		if l.Outcome != want || l.Bytes == nil || *l.Bytes != size || !strings.Contains(l.Detail, "recorded "+outcome+" at 2026-11-20T12:00:00Z: why") {
			t.Fatalf("%s -> %+v", outcome, l)
		}
		b, _ := json.Marshal(l)
		if strings.Contains(string(b), "999") || strings.Contains(string(b), "reclaim") {
			t.Fatalf("%s line carries a past reclaim figure: %s", outcome, b)
		}
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("managed Go cache")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	c := &Cell{Name: "fixture", Dir: dir, Env: &Env{vals: map[string]string{"CELL_GO_CACHE": "on", "CELL_GO_CACHE_MIN_FREE": "0"}}}
	line := func() gcLine {
		lines := c.gcCacheTarget().run(context.Background())
		if len(lines) != 1 {
			t.Fatalf("lines = %+v", lines)
		}
		return lines[0]
	}
	if l := line(); l.Outcome != "could-not-check" || l.Bytes != nil || !strings.Contains(l.Detail, "not measured yet") {
		t.Fatalf("unmeasured cache = %+v", l)
	}
	p, err := c.cachePolicy()
	must(t, err)
	if _, err := cellcache.Check(*p, false); err != nil {
		t.Fatal(err)
	}
	if l := line(); l.Outcome != "planned" || l.Bytes == nil || !strings.Contains(l.Detail, "recorded checked-clean") {
		t.Fatalf("measured cache = %+v", l)
	}
}
