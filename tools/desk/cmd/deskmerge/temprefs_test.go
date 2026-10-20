package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
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
	for _, opts := range *w.fetches {
		for _, spec := range opts.RefSpecs {
			if strings.HasPrefix(spec, "+refs/pull/7/head:") {
				destinations = append(destinations, strings.TrimPrefix(spec, "+refs/pull/7/head:"))
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
	prev := fetchFn
	t.Cleanup(func() { fetchFn = prev })
	fetchFn = func(root string, opts gitcore.FetchOpts) error {
		if err := prev(root, opts); err != nil {
			return err
		}
		return fmt.Errorf("injected failure after private destination was written")
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

// regression: #2525
func TestFetchCompletionWarning(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	prev := fetchFn
	t.Cleanup(func() { fetchFn = prev })
	fetchFn = func(root string, opts gitcore.FetchOpts) error {
		err := prev(root, opts)
		if err == nil {
			// Stop only this fixture process after the fetch. Observe still succeeds,
			// but the real completion guard must refuse before declaring completion.
			t.Setenv("DESK_TOOLS_DISABLED", "1")
		}
		return err
	}
	var warn bytes.Buffer
	prevWarn := cleanupWarn
	cleanupWarn = &warn
	t.Cleanup(func() { cleanupWarn = prevWarn })
	base, head, err := fetchState(w.root, testRepo, prInfo{Number: testPR, HeadRefOid: w.headSHA, BaseRefName: "main"})
	if err != nil || base != w.baseSHA || head != w.headSHA {
		t.Fatalf("completion changed verdict: %s,%s,%v", base, head, err)
	}
	receipts, err := filepath.Glob(filepath.Join(w.root, ".git", "assay-maintenance", "temporary-refs", "*.json"))
	if err != nil || len(receipts) != 1 {
		t.Fatalf("receipt custody=%v,%v", receipts, err)
	}
	body, err := os.ReadFile(receipts[0])
	var receipt struct {
		Complete bool
		Ref      string
		Hash     string
	}
	if err != nil || json.Unmarshal(body, &receipt) != nil || receipt.Complete || receipt.Hash != w.headSHA {
		t.Fatalf("completion was not held: %+v,%v", receipt, err)
	}
	if strings.Contains(warn.String(), "completed receipt") || !strings.Contains(warn.String(), receipt.Ref) || !strings.Contains(warn.String(), "disabled") || !strings.Contains(warn.String(), "confirmed producer completion") {
		t.Fatalf("untruthful completion warning: %q", warn.String())
	}
}
