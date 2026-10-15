//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHerdrOneWorkspacePerCell(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$HERDR_FIXTURE/calls"
case "$1 $2" in
 "workspace --help") echo 'create list close report-metadata';;
 "tab --help") echo 'create list close';;
 "pane --help") echo 'run';;
 "workspace create") shift 2; while test "$#" -gt 0; do if test "$1" = --label; then shift; label="$1"; fi; shift; done; touch "$HERDR_FIXTURE/$label.ws"; printf '{"result":{"workspace":{"workspace_id":"%s"},"tab":{"tab_id":"%s-default"}}}\n' "$label" "$label";;
 "workspace report-metadata") printf '%s' "${7#cellctl_owner=}" > "$HERDR_FIXTURE/$3.ws";;
 "workspace list") printf '{"result":{"workspaces":['; comma=''; for f in "$HERDR_FIXTURE/"*.ws; do test -f "$f" || continue; id="${f##*/}"; id="${id%.ws}"; printf '%s{"workspace_id":"%s","tokens":{"cellctl_owner":"%s"}}' "$comma" "$id" "$(cat "$f")"; comma=,; done; printf ']}}\n';;
 "workspace close") rm "$HERDR_FIXTURE/$3.ws";;
 "tab create") printf '{"result":{"tab":{"tab_id":"tab"},"root_pane":{"pane_id":"pane"}}}\n';;
 "tab list") printf '{"result":{"tabs":[]}}\n';;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_FIXTURE", dir)
	t.Setenv("PATH", dir+":/usr/bin:/bin")
	cell := func(name string) *Cell {
		p := filepath.Join(dir, name)
		os.Mkdir(p, 0700)
		return &Cell{Name: name, Dir: p, Env: envWith(map[string]string{})}
	}
	one, two := cell("one"), cell("two")
	a, err := one.ensureHerdrWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	b, err := two.ensureHerdrWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("cells share workspace")
	}
	again, err := one.ensureHerdrWorkspace()
	if err != nil || again.ID != a.ID {
		t.Fatal("cell workspace not reused", err)
	}
	one.herdrWindow("one-worker", "echo fixture", a.ID)
	s := &commsSurface{Cockpit: "herdr", Label: "two-comms-fixture"}
	if err := two.openCommsSurface(s); err != nil {
		t.Fatal(err)
	}
	if s.WorkspaceID != b.ID {
		t.Fatal("comms wrong workspace")
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if strings.Count(string(calls), "workspace create ") != 2 || !strings.Contains(string(calls), "tab create --workspace two --label two-comms-fixture") || !strings.Contains(string(calls), "tab create --workspace one --label one-worker") {
		t.Fatalf("unscoped creation: %s", calls)
	}
	os.WriteFile(filepath.Join(dir, "two.ws"), []byte("recycled-foreign-owner"), 0600)
	if err := two.closeHerdrWorkspace(); err == nil {
		t.Fatal("reused ID accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "two.ws")); err != nil {
		t.Fatal("foreign space closed")
	}
	if err := one.closeHerdrWorkspace(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "one.ws")); !os.IsNotExist(err) {
		t.Fatal("own workspace not closed")
	}
	// A create that never finished leaves an ID-less receipt; the next run recreates, never wedges.
	three := cell("three")
	os.MkdirAll(three.herdrDir(), 0700)
	if err := three.saveHerdrWorkspace(&herdrWorkspace{Owner: "unfinished"}); err != nil {
		t.Fatal(err)
	}
	c, err := three.ensureHerdrWorkspace()
	if err != nil || c.ID != "three" || c.Owner == "unfinished" {
		t.Fatal("unfinished receipt not healed", c, err)
	}
	if err := three.saveHerdrWorkspace(&herdrWorkspace{Owner: "unfinished"}); err != nil {
		t.Fatal(err)
	}
	if err := three.closeHerdrWorkspace(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(three.herdrDir(), "workspace.json")); !os.IsNotExist(err) {
		t.Fatal("unfinished receipt not dropped on close")
	}
}

// regression: assay#3027 — an unfinished create names the possible orphan; a finished one is silent.
func TestHerdrUnfinishedReceiptWarns(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	c := &Cell{Name: "orphan-cell"}
	c.warnUnfinishedHerdrReceipt(&herdrWorkspace{Owner: "o"}, "recreating it")
	c.warnUnfinishedHerdrReceipt(&herdrWorkspace{ID: "ws", Owner: "o"}, "recreating it")
	c.warnUnfinishedHerdrReceipt(nil, "recreating it")
	os.Stderr = old
	w.Close()
	b := make([]byte, 4096)
	n, _ := r.Read(b)
	got := string(b[:n])
	if strings.Count(got, "unfinished Herdr workspace create") != 1 || !strings.Contains(got, `"orphan-cell"`) {
		t.Fatalf("want exactly one orphan warning naming the cell, got %q", got)
	}
}
