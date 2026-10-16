package main

// tokencache_atomic_test.go — the GitHub installation-token cache is published atomically.
//
// Consumers read the cache path with `cat "$(desktoken <role>)"` while another process may be
// re-minting for the same role and installation. A reader must see the old token or the new
// one, whole — an empty or torn file becomes an empty token and the client then falls back to
// the host's stored login. Everything runs in a throwaway directory; no real token cache,
// HOME or network is touched.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// regression: #3129
func TestTokenCacheNeverTornUnderRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviewer-token-1")
	// Different lengths so a torn write is visible as a prefix of a longer token or as an
	// empty file, not only as a mix of two same-length values.
	tokens := []string{
		"ghs_" + strings.Repeat("a", 36),
		"ghs_" + strings.Repeat("b", 200),
		"ghs_" + strings.Repeat("c", 8),
	}
	valid := map[string]bool{}
	for _, tok := range tokens {
		valid[tok] = true
	}
	if err := writeTokenCacheAtomic(path, tokens[0]); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var done atomic.Bool
	var wg sync.WaitGroup
	var bad atomic.Value // first bad observation
	var reads atomic.Int64
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !done.Load() {
				b, err := os.ReadFile(path)
				if err != nil {
					bad.CompareAndSwap(nil, "read error: "+err.Error())
					return
				}
				reads.Add(1)
				if !valid[string(b)] {
					if len(b) > 40 {
						b = b[:40]
					}
					bad.CompareAndSwap(nil, "observed a partial or empty token, first bytes: "+string(b))
					return
				}
			}
		}()
	}

	const rewrites = 400
	var werr error
	for i := 0; i < rewrites && bad.Load() == nil; i++ {
		if werr = writeTokenCacheAtomic(path, tokens[i%len(tokens)]); werr != nil {
			break
		}
	}
	done.Store(true)
	wg.Wait()

	if werr != nil {
		t.Fatalf("rewrite: %v", werr)
	}
	if v := bad.Load(); v != nil {
		t.Fatalf("a concurrent reader saw a torn or missing token cache after %d reads: %v", reads.Load(), v)
	}
}

// regression: #3129
func TestTokenCacheOldTokenVisibleUntilRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviewer-token-1")
	oldTok := "ghs_old_" + strings.Repeat("o", 30)
	newTok := "ghs_new_" + strings.Repeat("n", 60)
	if err := writeTokenCacheAtomic(path, oldTok); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Forced interleaving: a reader runs at the instant the new token is fully written but not
	// yet published. It must still see the previous token, whole.
	var seen string
	var hookRan bool
	beforeTokenCacheRename = func() {
		hookRan = true
		b, err := os.ReadFile(path)
		if err != nil {
			seen = "read error: " + err.Error()
			return
		}
		seen = string(b)
	}
	t.Cleanup(func() { beforeTokenCacheRename = nil })

	if err := writeTokenCacheAtomic(path, newTok); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !hookRan {
		t.Fatal("beforeTokenCacheRename seam never ran")
	}
	if seen != oldTok {
		t.Fatalf("mid-write reader saw %q, want the previous token %q unchanged", seen, oldTok)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != newTok {
		t.Fatalf("after write: %q, %v; want the new token", got, err)
	}
}

// regression: #3129
func TestTokenCacheAtomicWriteMode0600NoLeftovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviewer-token-1")
	// A pre-existing, too-permissive file is replaced by a 0600 one, as the old Chmod did.
	writeFileMode(t, path, "stale", 0o644)
	if err := writeTokenCacheAtomic(path, "ghs_x"); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", fi.Mode().Perm())
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("directory holds %v, want only the cache (temp file left behind)", names)
	}
	// A failed publish leaves no temp behind either: a directory at path cannot be renamed over.
	bad := filepath.Join(dir, "adir")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeTokenCacheAtomic(bad, "ghs_y"); err == nil {
		t.Fatal("expected an error renaming over a non-empty directory")
	}
	ents, _ = os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temp file %s left behind after a failed publish", e.Name())
		}
	}
}
