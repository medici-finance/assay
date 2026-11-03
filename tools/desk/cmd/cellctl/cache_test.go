//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2781
func TestFixturePolicyControl(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	policy := cellcache.Policy{Root: root, Domain: strings.Repeat("a", 64), Budget: 4096}
	got, err := cellcache.FromEnv(policy.Env())
	if err != nil || got == nil || *got != policy {
		t.Fatalf("synthetic policy must be accepted before testing fixture isolation: %v, %v", got, err)
	}
}

func TestCacheLaunchEnvironment(t *testing.T) {
	c := codexEnvironmentCell(t)
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	c.Env.Put("CELL_GO_CACHE", "on")
	c.Env.Put("CELL_GO_CACHE_MIN_FREE", "0")
	env := c.cacheEnv([]string{"GOCACHE=/foreign", "GOMODCACHE=/foreign-mod", "GH_TOKEN=secret", "GOFLAGS=-mod=mod"})
	values, err := c.codexCommandEnvironment(env)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := c.cachePolicy()
	if err != nil {
		t.Fatal(err)
	}
	// The launch carries the ambient GOFLAGS extended with the policy's, and the
	// policy it exports records the same value, so the consumer's exact-match
	// custody check accepts it.
	p := resolved.WithAmbientGoFlags("-mod=mod")
	if want := "-mod=mod -trimpath"; p.GoFlags != want {
		t.Fatalf("merged GOFLAGS %q, want %q", p.GoFlags, want)
	}
	exported, err := cellcache.FromEnv(env)
	if err != nil || exported == nil || exported.GoFlags != p.GoFlags {
		t.Fatalf("exported policy goflags %+v (err %v), want %q", exported, err, p.GoFlags)
	}
	for _, kv := range p.Env() {
		k, v, _ := strings.Cut(kv, "=")
		if values[k] != v {
			t.Fatalf("Codex loses %s across HOME change", k)
		}
	}
	if _, ok := values["GH_TOKEN"]; ok {
		t.Fatal("ambient secret copied")
	}
	// The scrubbed launch carries no ambient GOFLAGS, so it gets the policy alone.
	ce := c.scrubbedComposeEnv("worker-desk", "go", "fixture")
	for _, kv := range resolved.Env() {
		found := false
		for _, got := range ce.Pairs {
			if got == kv {
				found = true
			}
		}
		if !found {
			t.Fatal("scrubbed launch lost", kv)
		}
	}
	if _, err = os.Stat(filepath.Join(p.Root, "owner")); !os.IsNotExist(err) {
		t.Fatal("composition mutated cache")
	}
	c.Env.Put("CELL_GO_CACHE", "off")
	if strings.Contains(strings.Join(c.cacheEnv(env), "\n"), "ASSAY_GO_CACHE_POLICY=") {
		t.Fatal("foreign ambient policy survived opt-out")
	}
}

func TestCacheBootHeldBeforeFetch(t *testing.T) {
	c := codexEnvironmentCell(t)
	c.Repo = t.TempDir() // intentionally not a Git repository; never a real remote
	if err := os.Mkdir(filepath.Join(c.Repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	c.Env.Put("CELL_GO_CACHE", "on")
	c.Env.Put("CELL_GO_CACHE_MIN_FREE", "18446744073709551615")
	defer func() {
		r := recover()
		code, ok := r.(exitCode)
		if !ok || code.code != 6 {
			t.Fatalf("boot bypassed prefetch storage hold: %v", r)
		}
	}()
	// No repo or harness is configured: touching either first is a different
	// failure. The storage gate must exit 6 before those operations are possible.
	c.deskLaunch("worker-desk", "", "", "", "", "", "", "", Provider{}, "", false, nil, nil, cockpitResolution{})
	t.Fatal("boot continued despite unavailable storage floor")
}

// admissionCell is a cache-enabled cell with a 1 MiB budget and one locked pass
// already recorded, so the lock-free pre-check has a clean estimate to lean on.
func admissionCell(t *testing.T) (*Cell, cellcache.Policy) {
	t.Helper()
	c := codexEnvironmentCell(t)
	dir, err := filepath.EvalSymlinks(filepath.Dir(c.Dir))
	if err != nil {
		t.Fatal(err)
	}
	c.Dir = filepath.Join(dir, "cell")
	if err = os.MkdirAll(c.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	c.Env.Put("CELL_GO_CACHE", "on")
	c.Env.Put("CELL_GO_CACHE_BYTES", "1048576")
	c.Env.Put("CELL_GO_CACHE_MIN_FREE", "0")
	p, err := c.cachePolicy()
	if err != nil || p == nil {
		t.Fatalf("policy: %v %v", p, err)
	}
	if _, err = cellcache.Check(*p, false); err != nil {
		t.Fatal(err)
	}
	return c, *p
}

// holdAdmissionLock models a sibling launch inside its locked pass and makes a
// caller that queues on the lock come back deferred quickly.
func holdAdmissionLock(t *testing.T, p cellcache.Policy) {
	t.Helper()
	root, err := os.OpenRoot(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	f, err := root.OpenFile("lock", os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	held, err := deskkit.AcquireHeld(root, "lock", f, deskkit.LockSpec{Name: "cell Go cache", Wait: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	old := cellcache.LockWait
	cellcache.LockWait = 50 * time.Millisecond
	t.Cleanup(func() {
		cellcache.LockWait = old
		held.Release()
		root.Close()
	})
}

// A below-threshold launch admission takes no cache lock and does no walk: with
// a sibling holding the lock for the whole call, a path that takes it would wait
// out LockWait and come back deferred.
//
// regression: #3045
func TestCacheLaunchAdmissionNoLock(t *testing.T) {
	c, p := admissionCell(t)
	before, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	holdAdmissionLock(t, p)
	if err := c.cacheAdmission(); err != nil {
		t.Fatalf("clean launch admission queued on the cache lock: %v", err)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("lock-free launch admission rewrote the report (it walked under the lock)")
	}
}

// Near the budget or with missing accounting, launch admission refuses.
// Explicit maintenance owns full measurement; a free lock cannot authorize
// the launch when its accounting is unavailable.
//
// regression: #3045
func TestCacheLaunchAdmissionLocks(t *testing.T) {
	t.Run("near budget", func(t *testing.T) {
		c, p := admissionCell(t)
		// 900000 of 1048576 logical bytes is over three quarters of the budget.
		if err := os.MkdirAll(filepath.Join(p.Root, "build"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.Root, "build", "near"), make([]byte, 900000), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := cellcache.Check(p, false); err != nil {
			t.Fatal(err)
		}
		holdAdmissionLock(t, p)
		if err := c.cacheAdmission(); err == nil {
			t.Fatal("near-threshold cache admitted without taking the lock")
		}
	})
	t.Run("missing estimate", func(t *testing.T) {
		c, p := admissionCell(t)
		if err := os.Remove(p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		holdAdmissionLock(t, p)
		if err := c.cacheAdmission(); err == nil {
			t.Fatal("unreadable estimate admitted without taking the lock")
		}
	})
	t.Run("lock free: missing accounting refuses", func(t *testing.T) {
		c, p := admissionCell(t)
		if err := os.Remove(p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if err := c.cacheAdmission(); err == nil {
			t.Fatal("missing accounting admitted an idle cache")
		}
		if _, err := os.Stat(p.ReportPath()); !os.IsNotExist(err) {
			t.Fatalf("launch performed full accounting: %v", err)
		}
		if _, err := cellcache.Check(p, false); err != nil {
			t.Fatal(err)
		}
		if err := c.cacheAdmission(); err != nil {
			t.Fatalf("explicit maintenance did not recover admission: %v", err)
		}
	})
}
