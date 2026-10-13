package main

import (
	"fmt"
	"net/http"
	"path/filepath"
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

// TestCacheEntryNeverTorn — concurrent writers and readers of one entry: every read is a
// miss or one writer's whole entry, never a mix.
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
		t.Fatalf("%d reads saw an entry whose validator and body came from different writes: %v", len(torn), torn[:1])
	}
	if loadCacheEntry(path, req) == nil {
		t.Fatal("no whole entry survived the concurrent writes")
	}
}
