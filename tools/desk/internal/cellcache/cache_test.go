//go:build darwin || linux

package cellcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture(t *testing.T) Policy {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := Resolve(dir, func(k string) string {
		if k == "CELL_GO_CACHE" {
			return "on"
		}
		if k == "CELL_GO_CACHE_MIN_FREE" {
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	return *p
}
func material(t *testing.T, p Policy, name string, n int) {
	t.Helper()
	path := filepath.Join(p.Root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), n), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestCachePolicy(t *testing.T) {
	p := fixture(t)
	if p.Budget != 8<<30 {
		t.Fatal(p)
	}
	for _, home := range []string{t.TempDir(), t.TempDir()} {
		t.Setenv("HOME", home)
		q, err := FromEnv(p.Env())
		if err != nil || *q != p {
			t.Fatalf("HOME changed policy: %v %v", q, err)
		}
		if strings.Contains(strings.Join(q.Env(), "\n"), home) {
			t.Fatal("per-home cache")
		}
	}
	q := fixture(t)
	if p.Root == q.Root || p.Domain == q.Domain {
		t.Fatal("domains shared")
	}
	for _, key := range []string{"CELL_GO_CACHE_BYTES", "CELL_GO_CACHE_MIN_FREE", "CELL_GO_CACHE"} {
		_, err := Resolve(filepath.Dir(p.Root), func(k string) string {
			if k == key {
				return "bad"
			}
			if k == "CELL_GO_CACHE" {
				return "on"
			}
			return ""
		})
		if err == nil {
			t.Fatalf("accepted invalid %s", key)
		}
	}
	override := filepath.Join(filepath.Dir(p.Root), "alternate")
	qptr, err := Resolve(filepath.Dir(p.Root), func(k string) string {
		if k == "CELL_GO_CACHE" {
			return "on"
		}
		if k == "CELL_GO_CACHE_ROOT" {
			return override
		}
		return ""
	})
	if err != nil || qptr.Root != override || qptr.Domain != p.Domain {
		t.Fatalf("override: %v %v", qptr, err)
	}
}
func TestCacheCustody(t *testing.T) {
	p := fixture(t)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	other := fixture(t)
	other.Root = p.Root
	if _, err := Check(other, false); err == nil {
		t.Fatal("foreign domain adopted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(p.Root, "build")); err != nil {
		t.Fatal(err)
	}
	r, err := Check(p, false)
	if err == nil || r.Before.LogicalBytes != nil || r.Outcome != "could-not-check" {
		t.Fatalf("symlink accepted: %+v %v", r, err)
	}
	q := fixture(t)
	q.Root = filepath.Join(p.Root, "build", "cache")
	if err := q.Validate(); err == nil {
		t.Fatal("escaping ancestor accepted")
	}
	q = fixture(t)
	if err := os.Mkdir(q.Root, 0700); err != nil {
		t.Fatal(err)
	}
	material(t, q, "precious", 12)
	if _, err := Check(q, false); err == nil {
		t.Fatal("unmarked data adopted")
	}
	q = fixture(t)
	if _, err := Check(q, false); err != nil {
		t.Fatal(err)
	}
	material(t, q, "build/a", 12)
	if err := os.Link(filepath.Join(q.Root, "build/a"), filepath.Join(q.Root, "build/b")); err != nil {
		t.Fatal(err)
	}
	if r, err := Check(q, false); err == nil || r.Before.LogicalBytes != nil {
		t.Fatal("hardlinks counted as owned data")
	}
}
func TestCacheWarmAndPressure(t *testing.T) {
	p := fixture(t)
	p.Budget = 32
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/result", 100)
	material(t, p, "mod/module/file", 100)
	r, err := Check(p, false)
	var hold *Deferred
	if !errors.As(err, &hold) || r.Outcome != "storage-deferred" || len(r.SkippedActive) != 1 || r.Reclaimed != 0 {
		t.Fatalf("active cache not protected: %+v %v", r, err)
	}
	if err = l.Finish(true); err != nil {
		t.Fatal(err)
	}
	r, err = Check(p, true)
	if err == nil || r.Reclaimed != 0 || *r.After.LogicalBytes != 200 {
		t.Fatal("dry run removed data")
	}
	if err := os.Chmod(filepath.Join(p.Root, "mod/module"), 0500); err != nil {
		t.Fatal(err)
	}
	r, err = Check(p, false)
	if err != nil || r.Reclaimed != 200 || *r.After.LogicalBytes != 0 {
		t.Fatalf("inactive pressure: %+v %v", r, err)
	}
	l, err = Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/warm", 10)
	if err = l.Finish(true); err != nil {
		t.Fatal(err)
	}
	r, err = Check(p, false)
	if err != nil || *r.After.LogicalBytes != 10 || r.Reclaimed != 0 {
		t.Fatal("ordinary completion lost warm cache")
	}
	b, err := os.ReadFile(filepath.Join(p.Root, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Report
	if err = json.Unmarshal(b, &saved); err != nil || saved.Before.At.IsZero() || saved.After.At.Before(saved.Before.At) || saved.Filesystem == "" {
		t.Fatalf("report: %s %v", b, err)
	}
	if _, err = os.Stat(filepath.Join(p.Root, "previous.json")); err != nil {
		t.Fatal("missing prior comparison", err)
	}
}
func TestCachePartial(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/result", 10)
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.close()
	m.free = func(string) (uint64, error) { return 0, fmt.Errorf("measurement unavailable") }
	r := m.check(false)
	if r.Outcome != "could-not-check" || r.Before.AvailableBytes != nil || r.Reclaimed != 0 || *r.After.LogicalBytes != 10 {
		t.Fatalf("unknown treated as zero/success: %+v", r)
	}
	m.free = func(string) (uint64, error) { return 100, nil }
	m.remove = func(string) error { return fmt.Errorf("removal denied") }
	r = m.check(false)
	if r.Outcome != "could-not-check" || len(r.Failures) == 0 || r.Reclaimed != 0 {
		t.Fatal("removal failure hidden")
	}
}

func TestCacheDryRunAndEnv(t *testing.T) {
	p := fixture(t)
	if _, err := Check(p, true); err == nil {
		t.Fatal("missing root reported measured")
	}
	if _, err := os.Stat(p.Root); !os.IsNotExist(err) {
		t.Fatal("dry run initialized root")
	}
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Check(p, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("dry run rewrote report")
	}
	wrong := p.Env()
	wrong[1] = "GOCACHE=/foreign"
	for _, env := range [][]string{wrong, append(p.Env(), "GOCACHE=/foreign"), p.Env()[:1]} {
		if _, err = Acquire(env); err == nil {
			t.Fatal("unmanaged cache environment admitted")
		}
	}
}

func TestCacheSymlinkRace(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep")
	if err := os.WriteFile(sentinel, []byte("evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	// Deterministic swap after measurement, immediately before deletion. The
	// concurrent repetitions below explore the remaining syscall boundaries.
	material(t, p, "build/data", 100)
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	m.remove = func(name string) error {
		if name == "build" {
			if err := os.Rename(filepath.Join(p.Root, name), filepath.Join(p.Root, "moved")); err != nil {
				return err
			}
			if err := os.Symlink(outside, filepath.Join(p.Root, name)); err != nil {
				return err
			}
		}
		return m.removeTree(name)
	}
	r := m.check(false)
	m.close()
	if r.Outcome != "could-not-check" {
		t.Fatal("racing replacement was not refused", r)
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "evidence" {
		t.Fatal("followed replacement out of root", err)
	}
	if err = os.RemoveAll(filepath.Join(p.Root, "build")); err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(filepath.Join(p.Root, "moved")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		material(t, p, "build/data", 100)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			old := filepath.Join(p.Root, "moved")
			build := filepath.Join(p.Root, "build")
			_ = os.Rename(build, old)
			_ = os.Symlink(outside, build)
		}()
		_, _ = Check(p, false)
		wg.Wait()
		b, err := os.ReadFile(sentinel)
		if err != nil || string(b) != "evidence" {
			t.Fatal("cleanup followed racing symlink", err)
		}
		if err = os.RemoveAll(filepath.Join(p.Root, "build")); err != nil {
			t.Fatal(err)
		}
		if err = os.RemoveAll(filepath.Join(p.Root, "moved")); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCacheRecovery(t *testing.T) {
	p := fixture(t)
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Finish(false); err != nil {
		t.Fatal(err)
	}
	p.Budget = 1
	material(t, p, "build/result", 10)
	if _, err = Check(p, false); err == nil {
		t.Fatal("uncertain consumer forgotten")
	}
	if err = Recover(p, false); err == nil {
		t.Fatal("recovery without confirmation")
	}
	if err = Recover(p, true); err != nil {
		t.Fatal(err)
	}
	if _, err = Check(p, false); err != nil {
		t.Fatal(err)
	}
	l, err = Acquire(p.Env())
	if err != nil {
		t.Fatal("did not recover", err)
	}
	if err = l.Finish(true); err != nil {
		t.Fatal(err)
	}
}
func TestCacheConcurrent(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/result", 10)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := Check(p, false)
			if err == nil || r.Reclaimed != 0 || len(r.SkippedActive) != 1 {
				t.Errorf("cleanup raced consumer: %+v %v", r, err)
			}
		}()
	}
	wg.Wait()
	if err = l.Finish(true); err != nil {
		t.Fatal(err)
	}
	if _, err = Check(p, false); err != nil {
		t.Fatal(err)
	}
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := Acquire(p.Env())
			if err != nil {
				t.Error(err)
				return
			}
			if err = l.Finish(true); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
func TestCacheBuildReuse(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go unavailable")
	}
	p := fixture(t)
	p.Budget = 1 << 30
	work := t.TempDir()
	if err = os.WriteFile(filepath.Join(work, "go.mod"), []byte("module cache.fixture\n\ngo 1.25\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(work, "cache.go"), []byte("package cache\nfunc Sum(a,b int) int{return a+b}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		home := t.TempDir()
		// No downloads, no toolchain acquisition, no standard library imports.
		env := append(p.Env(), "HOME="+home, "PATH="+os.Getenv("PATH"), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0")
		l, err := Acquire(env)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(goBin, "build", "-x", ".")
		cmd.Dir = work
		cmd.Env = env
		out, buildErr := cmd.CombinedOutput()
		finishErr := l.Finish(true)
		if buildErr != nil || finishErr != nil {
			t.Fatalf("build: %s %v %v", out, buildErr, finishErr)
		}
		compiled := strings.Contains(string(out), "/compile ") || strings.Contains(string(out), "/compile -")
		if i == 0 && !compiled {
			t.Fatalf("cold build never compiled: %s", out)
		}
		if i == 1 && compiled {
			t.Fatalf("second HOME recompiled: %s", out)
		}
		for _, name := range []string{".cache/go-build", "Library/Caches/go-build"} {
			if _, err = os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
				t.Fatal("left per-home cache", name, err)
			}
		}
	}
}
