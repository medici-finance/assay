//go:build darwin || linux

package cellcache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// age backdates one fixture entry's mtime.
func age(t *testing.T, p Policy, name string, d time.Duration) {
	t.Helper()
	old := time.Now().Add(-d)
	if err := os.Chtimes(filepath.Join(p.Root, name), old, old); err != nil {
		t.Fatal(err)
	}
}

func exists(p Policy, name string) bool {
	_, err := os.Lstat(filepath.Join(p.Root, name))
	return err == nil
}

// A live consumer record no longer pins the compiler cache: under pressure,
// build entries past the policy age go; fresh ones, non-entry files, the
// module cache and installed programs stay.
//
// Register row: R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheTrimUnderLiveConsumer(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	material(t, p, "build/0a/old-a", 100)
	material(t, p, "build/0a/old-d", 50)
	material(t, p, "build/ff/fresh-d", 7)
	material(t, p, "build/0a/stale-x", 3)
	material(t, p, "build/trim.txt", 2)
	material(t, p, "mod/example/old.go", 11)
	material(t, p, "gopath/bin/tool", 5)
	for _, name := range []string{"build/0a/old-a", "build/0a/old-d", "build/0a/stale-x", "build/trim.txt", "mod/example/old.go", "gopath/bin/tool"} {
		age(t, p, name, 48*time.Hour)
	}
	age(t, p, "build/ff/fresh-d", 30*time.Minute)

	// A dry run (status) reports pressure and trims nothing.
	r, err := Check(p, true)
	if err == nil || r.TrimmedBuild != 0 || !exists(p, "build/0a/old-a") {
		t.Fatalf("dry run trimmed: %+v %v", r, err)
	}

	r, err = Check(p, false)
	var hold *Deferred
	if !errors.As(err, &hold) || r.Outcome != "storage-deferred" || len(r.SkippedActive) != 1 {
		t.Fatalf("still over budget with a live record: want storage-deferred, got %+v %v", r, err)
	}
	if r.TrimmedBuild != 2 || r.TrimmedBuildBytes != 150 || r.Reclaimed != 150 {
		t.Fatalf("trim report: entries %d bytes %d reclaimed %d, want 2/150/150", r.TrimmedBuild, r.TrimmedBuildBytes, r.Reclaimed)
	}
	for _, gone := range []string{"build/0a/old-a", "build/0a/old-d"} {
		if exists(p, gone) {
			t.Errorf("%s past the trim age survived", gone)
		}
	}
	for _, kept := range []string{"build/ff/fresh-d", "build/0a/stale-x", "build/trim.txt", "mod/example/old.go", "gopath/bin/tool"} {
		if !exists(p, kept) {
			t.Errorf("%s removed: only aged build entries may go under a live record", kept)
		}
	}
	b, err := os.ReadFile(p.ReportPath())
	if err != nil || !strings.Contains(string(b), `"trimmed_build_bytes": 150`) {
		t.Fatalf("persisted report lacks the trimmed bytes: %s %v", b, err)
	}
}

// When the trim alone clears pressure, admission passes with consumers live.
//
// Register row: R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheTrimClearsPressure(t *testing.T) {
	p := fixture(t)
	p.Budget = 64
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	material(t, p, "build/3c/old-d", 100)
	material(t, p, "build/3c/new-d", 10)
	age(t, p, "build/3c/old-d", 25*time.Hour)
	r, err := Check(p, false)
	if err != nil || r.Outcome != "checked-clean" || *r.After.LogicalBytes != 10 || r.TrimmedBuild != 1 {
		t.Fatalf("trim did not clear pressure: %+v %v", r, err)
	}
	// Without pressure nothing is trimmed, however old: warm caches stay.
	material(t, p, "build/3c/older-a", 5)
	age(t, p, "build/3c/older-a", 30*24*time.Hour)
	if r, err = Check(p, false); err != nil || r.TrimmedBuild != 0 || !exists(p, "build/3c/older-a") {
		t.Fatalf("trimmed without pressure: %+v %v", r, err)
	}
}

// The policy age is honoured, not a fixed one.
//
// Register row: R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheTrimPolicyAge(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	p.BuildMaxAge = int64((72 * time.Hour) / time.Second)
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	material(t, p, "build/00/two-days-a", 9)
	material(t, p, "build/00/four-days-a", 4)
	age(t, p, "build/00/two-days-a", 48*time.Hour)
	age(t, p, "build/00/four-days-a", 96*time.Hour)
	r, _ := Check(p, false)
	if r.TrimmedBuild != 1 || !exists(p, "build/00/two-days-a") || exists(p, "build/00/four-days-a") {
		t.Fatalf("72h policy: %+v", r)
	}
}

// A linked entry stops the pass: nothing is trimmed through it.
//
// Register row: R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheTrimRefusesLinks(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	outside := filepath.Join(t.TempDir(), "keep-a")
	if err := os.WriteFile(outside, []byte("evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.Root, "build/01"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(p.Root, "build/01/link-a")); err != nil {
		t.Fatal(err)
	}
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := m.trimBuild(time.Now().Add(time.Hour))
	m.close()
	if err == nil || n != 0 {
		t.Fatalf("symlinked entry accepted: %d %v", n, err)
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "evidence" {
		t.Fatal("trim followed a link out of the root", err)
	}
}

// Register row: R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheBuildAgePolicy(t *testing.T) {
	p := fixture(t)
	if p.BuildAge() != 24*time.Hour || p.BuildMaxAge != 86400 {
		t.Fatalf("default trim age %v (%d)", p.BuildAge(), p.BuildMaxAge)
	}
	get := func(v string) func(string) string {
		return func(k string) string {
			switch k {
			case "CELL_GO_CACHE":
				return "on"
			case "CELL_GO_CACHE_BUILD_MAX_AGE":
				return v
			}
			return ""
		}
	}
	cell := filepath.Dir(p.Root)
	for _, bad := range []string{"1h", "119m", "0", "-24h", "3h0.5s", "soon"} {
		if _, err := Resolve(cell, get(bad)); err == nil {
			t.Errorf("accepted CELL_GO_CACHE_BUILD_MAX_AGE=%s", bad)
		}
	}
	q, err := Resolve(cell, get("2h"))
	if err != nil || q.BuildAge() != 2*time.Hour {
		t.Fatalf("2h: %v %v", q, err)
	}
	// A serialized policy recorded before the field existed still round-trips
	// byte-identically, so a consumer launched by an older tool keeps custody.
	legacy := p
	legacy.BuildMaxAge = 0
	b, _ := json.Marshal(legacy)
	if strings.Contains(string(b), "build_max_age") {
		t.Fatalf("legacy policy serialization changed: %s", b)
	}
	if got, err := FromEnv(legacy.Env()); err != nil || got.BuildAge() != DefaultBuildMaxAge {
		t.Fatalf("legacy policy: %v %v", got, err)
	}
	l, err := Acquire(legacy.Env())
	if err != nil {
		t.Fatal("legacy consumer refused", err)
	}
	l.Finish(true)
	legacy.BuildMaxAge = 60
	if legacy.Validate() == nil {
		t.Fatal("recorded trim age under the floor accepted")
	}
}

// exeDir plants a directory-form "-d" output, the shape cmd/go caches a
// `go run` / `go tool` executable in, and backdates it and its file.
func exeDir(t *testing.T, p Policy, name string, d time.Duration) {
	t.Helper()
	material(t, p, name+"/prog", 40)
	age(t, p, name+"/prog", d)
	age(t, p, name, d)
}

// Directory-form entries are trimmed like files: an aged one goes whole, a
// fresh one stays, and neither stops the pass.
// Register row: R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheTrimExeDirs(t *testing.T) {
	p := fixture(t)
	p.Budget = 1
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	exeDir(t, p, "build/0c/oldexe-d", 48*time.Hour)
	exeDir(t, p, "build/0c/newexe-d", 10*time.Minute)
	material(t, p, "build/0d/old-a", 5)
	age(t, p, "build/0d/old-a", 48*time.Hour)
	r, err := Check(p, false)
	if r.Outcome != "storage-deferred" || len(r.Failures) != 0 {
		t.Fatalf("directory entry stopped the trim: %+v %v", r, err)
	}
	if r.TrimmedBuild != 2 || r.TrimmedBuildBytes != 45 {
		t.Fatalf("trim report: entries %d bytes %d, want 2/45", r.TrimmedBuild, r.TrimmedBuildBytes)
	}
	if exists(p, "build/0c/oldexe-d") || exists(p, "build/0d/old-a") {
		t.Fatal("aged entries survived")
	}
	if !exists(p, "build/0c/newexe-d/prog") {
		t.Fatal("fresh executable directory removed")
	}
}

// A directory entry holding a link is refused whole and never followed, and
// the trim still takes the entries after it. Register row:
// R-cache-consumer-custody (docs/contracts.md).
//
// regression: #2879
func TestCacheTrimSkipsBadDir(t *testing.T) {
	p := fixture(t)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "keep")
	if err := os.WriteFile(outside, []byte("evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	exeDir(t, p, "build/00/linked-d", 48*time.Hour)
	if err := os.Symlink(outside, filepath.Join(p.Root, "build/00/linked-d/link")); err != nil {
		t.Fatal(err)
	}
	age(t, p, "build/00/linked-d", 48*time.Hour)
	material(t, p, "build/00/zzz-a", 3)
	age(t, p, "build/00/zzz-a", 48*time.Hour)
	material(t, p, "build/ff/later-d", 6)
	age(t, p, "build/ff/later-d", 48*time.Hour)
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	n, bytes, err := m.trimBuild(time.Now().Add(-24 * time.Hour))
	m.close()
	if err == nil || !strings.Contains(err.Error(), "linked-d") {
		t.Fatalf("linked directory not reported: %v", err)
	}
	if n != 2 || bytes != 9 || exists(p, "build/00/zzz-a") || exists(p, "build/ff/later-d") {
		t.Fatalf("trim stopped at the refused entry: %d entries %d bytes", n, bytes)
	}
	if !exists(p, "build/00/linked-d/prog") || !exists(p, "build/00/linked-d/link") {
		t.Fatal("refused directory partly removed")
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "evidence" {
		t.Fatal("trim followed a link out of the root", err)
	}
}
