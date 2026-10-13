package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestEndpointClass(t *testing.T) {
	for path, want := range map[string]string{
		"/graphql":                          "graphql",
		"/api/graphql":                      "graphql",
		"/repos/o/r":                        "repo",
		"/repos/o/r/pulls/7":                "pulls",
		"/repos/o/r/pulls/7/files":          "pulls.files",
		"/repos/o/r/commits":                "commits",
		"/repos/o/r/commits/abc/check-runs": "commits.check-runs",
		"/api/v3/repos/o/r/issues/9/events": "issues.events",
		"/search/issues":                    "search",
	} {
		if got := endpointClass(path); got != want {
			t.Errorf("endpointClass(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestCacheEntryNeverTorn — concurrent writers and readers of one entry. A whole entry is
// seeded first, so once seeded EVERY read must return one writer's whole entry: a miss is a
// failure (a torn or truncated file parses as a miss), and so is a mix of two writers.
// Fails if writeFileAtomic is replaced by a direct os.WriteFile (truncate-then-write).
func TestCacheEntryNeverTorn(t *testing.T) {
	dir := t.TempDir()
	old := boardCacheDir
	boardCacheDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { boardCacheDir = old })
	req, _ := http.NewRequest(http.MethodGet, "https://forge.test/repos/o/r/pulls/1", nil)
	path := cachePath(req)
	if filepath.Dir(filepath.Dir(path)) != filepath.Join(dir, "http") {
		t.Fatalf("cache path %s is not under the cache root", path)
	}
	seedHdr := http.Header{}
	seedHdr.Set("ETag", `"w0"`)
	storeCacheEntry(path, req, seedHdr, []byte(fmt.Sprintf(`{"writer":%d,"pad":"%0512d"}`, 0, 0)))
	if loadCacheEntry(path, req) == nil {
		t.Fatal("the seeded whole entry did not load")
	}
	// Windows can refuse a rename over a file a reader holds open; there a transient miss is
	// not a torn read, so the miss assertion is skipped.
	missIsTorn := runtime.GOOS != "windows"
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			body := []byte(fmt.Sprintf(`{"writer":%d,"pad":"%0512d"}`, w, w))
			for i := 0; i < 50; i++ {
				h := http.Header{}
				h.Set("ETag", fmt.Sprintf(`"w%d"`, w))
				storeCacheEntry(path, req, h, body)
			}
		}(w)
	}
	var torn []string
	var mu sync.Mutex
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				e := loadCacheEntry(path, req)
				if e == nil {
					if missIsTorn {
						mu.Lock()
						torn = append(torn, "miss")
						mu.Unlock()
					}
					continue
				}
				var w int
				if _, err := fmt.Sscanf(e.ETag, `"w%d"`, &w); err != nil || string(e.Body) != fmt.Sprintf(`{"writer":%d,"pad":"%0512d"}`, w, w) {
					mu.Lock()
					torn = append(torn, e.ETag)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if len(torn) > 0 {
		t.Fatalf("%d reads saw a miss or an entry whose validator and body came from different writes: %v", len(torn), torn[:1])
	}
	if loadCacheEntry(path, req) == nil {
		t.Fatal("no whole entry survived the concurrent writes")
	}
}
