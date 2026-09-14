package gitcore

// OpenWith's one job: route object reads through a cache the CALLER owns, so a pass that
// opens many worktrees of one repository decodes each object once rather than once per
// worktree. Open keeps building its own, so no existing caller changes behaviour.

import (
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// newCacheTestRepo builds a small multi-commit fixture repository and returns its root.
func newCacheTestRepo(t *testing.T) string {
	t.Helper()
	f := gittest.NewFixture(t)
	f.CommitFile(t, "a.txt", "one\n", "first")
	f.CommitFile(t, "b.txt", "two\n", "second")
	f.CommitFile(t, "c.txt", "three\n", "third")
	return f.Dir
}

// countingCache wraps an ObjectCache and records the traffic through it, so a test can
// assert the cache the caller handed over is the one that was actually used — rather than
// inferring it from a timing that would pass either way on a small fixture.
type countingCache struct {
	mu     sync.Mutex
	inner  ObjectCache
	puts   int
	gets   int
	hits   int
	misses int
}

func newCountingCache(inner ObjectCache) *countingCache { return &countingCache{inner: inner} }

func (c *countingCache) Put(o plumbing.EncodedObject) {
	c.mu.Lock()
	c.puts++
	c.mu.Unlock()
	c.inner.Put(o)
}

func (c *countingCache) Get(k plumbing.Hash) (plumbing.EncodedObject, bool) {
	o, ok := c.inner.Get(k)
	c.mu.Lock()
	c.gets++
	if ok {
		c.hits++
	} else {
		c.misses++
	}
	c.mu.Unlock()
	return o, ok
}

func (c *countingCache) Clear() { c.inner.Clear() }

func (c *countingCache) counts() (puts, gets, hits int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.puts, c.gets, c.hits
}

// TestOpenWithSharesCache: two handles on the SAME repository, given one cache, both read
// through it — and the second sees hits the first populated. Without sharing the second
// handle starts cold, which is the ~700 redundant inflations per sweep issue #1037 measured.
func TestOpenWithSharesCache(t *testing.T) {
	dir := newCacheTestRepo(t)

	shared := newCountingCache(NewObjectCache())

	first, err := OpenWith(dir, shared)
	if err != nil {
		t.Fatalf("OpenWith (first): %v", err)
	}
	if _, lerr := first.Log("HEAD"); lerr != nil {
		t.Fatalf("Log (first): %v", lerr)
	}
	putsAfterFirst, _, _ := shared.counts()
	if putsAfterFirst == 0 {
		t.Fatalf("the supplied cache was never written to — OpenWith is not routing through it")
	}

	second, err := OpenWith(dir, shared)
	if err != nil {
		t.Fatalf("OpenWith (second): %v", err)
	}
	if _, lerr := second.Log("HEAD"); lerr != nil {
		t.Fatalf("Log (second): %v", lerr)
	}
	_, gets, hits := shared.counts()
	if gets == 0 {
		t.Fatalf("the second handle never read the shared cache")
	}
	if hits == 0 {
		t.Fatalf("the second handle read the shared cache %d time(s) and hit nothing — the cache is not actually shared", gets)
	}
}

// TestOpenStillBuildsItsOwnCache: the negative control. Open must NOT have quietly become
// a shared-state call, or every existing caller in the tree changes behaviour at once.
func TestOpenStillBuildsItsOwnCache(t *testing.T) {
	dir := newCacheTestRepo(t)

	watched := newCountingCache(NewObjectCache())
	seeded, err := OpenWith(dir, watched)
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}
	if _, lerr := seeded.Log("HEAD"); lerr != nil {
		t.Fatalf("Log: %v", lerr)
	}
	_, getsBefore, _ := watched.counts()

	plain, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, lerr := plain.Log("HEAD"); lerr != nil {
		t.Fatalf("Log (plain): %v", lerr)
	}

	_, getsAfter, _ := watched.counts()
	if getsAfter != getsBefore {
		t.Fatalf("plain Open read a cache it was never given (%d -> %d reads) — Open must build its own",
			getsBefore, getsAfter)
	}
}

// A nil cache is not a panic and not a silently uncached repo: OpenWith substitutes a
// fresh one, so a caller that forgot gets Open's exact behaviour.
func TestOpenWithNilCacheFallsBackToItsOwn(t *testing.T) {
	dir := newCacheTestRepo(t)
	r, err := OpenWith(dir, nil)
	if err != nil {
		t.Fatalf("OpenWith(nil): %v", err)
	}
	if _, lerr := r.Log("HEAD"); lerr != nil {
		t.Fatalf("Log: %v", lerr)
	}
}
