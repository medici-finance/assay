package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2513
func TestFetchRefsArePrivate(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	// A previous interrupted run's old ref is unproven ownership, not ours.
	git(t, w.root, "update-ref", "refs/deskmerge/pr-7", w.siblingSHA)
	p := prInfo{Number: testPR, HeadRefOid: w.headSHA, BaseRefName: "main"}
	var destinations []string
	for i := 0; i < 2; i++ {
		base, head, err := fetchState(w.root, testRepo, p)
		if err != nil || base != w.baseSHA || head != w.headSHA {
			t.Fatalf("fetch=%s,%s,%v", base, head, err)
		}
	}
	for _, call := range *w.gitAll {
		if len(call) > 1 && call[1] == "fetch" {
			for _, arg := range call {
				if strings.HasPrefix(arg, "+refs/pull/7/head:") {
					destinations = append(destinations, strings.TrimPrefix(arg, "+refs/pull/7/head:"))
				}
			}
		}
	}
	if len(destinations) != 2 || destinations[0] == destinations[1] {
		t.Fatalf("operation refs alias: %v", destinations)
	}
	if out := git(t, w.root, "for-each-ref", "--format=%(refname)", "refs/deskmerge/tmp/"); out != "" {
		t.Fatalf("operation refs leaked: %s", out)
	}
	if got := git(t, w.root, "rev-parse", "refs/deskmerge/pr-7"); got != w.siblingSHA {
		t.Fatal("legacy ref was deleted or overwritten")
	}
}

// regression: #2513
func TestFetchFailureDrainsRefs(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	prev := runGit
	runGit = func(dir string, args ...string) (string, error) {
		out, err := prev(dir, args...)
		if len(args) > 0 && args[0] == "fetch" && err == nil {
			return out, fmt.Errorf("injected failure after private destination was written")
		}
		return out, err
	}
	p := prInfo{Number: testPR, HeadRefOid: w.headSHA, BaseRefName: "main"}
	if _, _, err := fetchState(w.root, testRepo, p); err == nil {
		t.Fatal("failed fetch answered clean")
	}
	if out := git(t, w.root, "for-each-ref", "--format=%(refname)", "refs/deskmerge/tmp/"); out != "" {
		t.Fatalf("failed fetch leaked owned refs: %s", out)
	}
}

// regression: #2525
func TestFetchHookKeepsVerdict(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "reference-transaction"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, w.root, "config", "core.hooksPath", hooks)
	assertCleanupKeepsVerdict(t, w)
}

// assertCleanupKeepsVerdict: producer cleanup refused by the shared hook gate
// leaves the observed SHAs as the answer and the owned ref for later recovery.
func assertCleanupKeepsVerdict(t *testing.T, w *world) {
	t.Helper()
	var warn bytes.Buffer
	prev := cleanupWarn
	cleanupWarn = &warn
	t.Cleanup(func() { cleanupWarn = prev })
	p := prInfo{Number: testPR, HeadRefOid: w.headSHA, BaseRefName: "main"}
	base, head, err := fetchState(w.root, testRepo, p)
	if err != nil || base != w.baseSHA || head != w.headSHA {
		t.Fatalf("cleanup changed the verdict: %s,%s,%v", base, head, err)
	}
	out := git(t, w.root, "for-each-ref", "--format=%(refname) %(objectname)", "refs/deskmerge/tmp/")
	name, _, _ := strings.Cut(out, " ")
	if !strings.HasSuffix(out, " "+w.headSHA) || strings.Contains(out, "\n") {
		t.Fatalf("owned ref not retained for recovery: %q", out)
	}
	if !strings.Contains(warn.String(), name) || !strings.Contains(warn.String(), "receipt is retained") {
		t.Fatalf("cleanup failure was silent: %q", warn.String())
	}
}
