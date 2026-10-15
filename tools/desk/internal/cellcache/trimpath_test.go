//go:build darwin || linux

package cellcache

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCacheTrimpathPolicy pins the managed cache's default GOFLAGS: on unless
// the cell opts out, carried in the serialized policy, and absent from a policy
// recorded before the field existed.
func TestCacheTrimpathPolicy(t *testing.T) {
	p := fixture(t)
	if p.GoFlags != TrimpathFlag || !slices.Contains(p.Env(), "GOFLAGS=-trimpath") {
		t.Fatalf("default policy: goflags %q, env %v", p.GoFlags, p.Env())
	}
	get := func(v string) func(string) string {
		return func(k string) string {
			switch k {
			case "CELL_GO_CACHE":
				return "on"
			case "CELL_GO_TRIMPATH":
				return v
			}
			return ""
		}
	}
	cell := filepath.Dir(p.Root)
	if q, err := Resolve(cell, get("on")); err != nil || q.GoFlags != TrimpathFlag {
		t.Fatalf("on: %v %v", q, err)
	}
	off, err := Resolve(cell, get("off"))
	if err != nil || off.GoFlags != "" {
		t.Fatalf("off: %v %v", off, err)
	}
	for _, kv := range off.Env() {
		if strings.HasPrefix(kv, "GOFLAGS=") {
			t.Fatalf("opted-out policy exports %s", kv)
		}
	}
	for _, bad := range []string{"yes", "true", "ON", "1"} {
		if _, err := Resolve(cell, get(bad)); err == nil {
			t.Errorf("accepted CELL_GO_TRIMPATH=%s", bad)
		}
	}
	// A policy serialized before the field existed round-trips byte-identically
	// and exports no GOFLAGS, so a consumer launched by an older tool keeps custody.
	legacy := p
	legacy.GoFlags = ""
	b, _ := json.Marshal(legacy)
	if strings.Contains(string(b), "goflags") {
		t.Fatalf("legacy policy serialization changed: %s", b)
	}
	l, err := Acquire(legacy.Env())
	if err != nil {
		t.Fatal("legacy consumer refused", err)
	}
	l.Finish(true)
	// The policy's GOFLAGS must match the environment exactly, like GOCACHE.
	env := append(p.Env()[:len(p.Env())-1:len(p.Env())-1], "GOFLAGS=-mod=mod")
	if _, err := Acquire(env); err == nil {
		t.Fatal("consumer whose GOFLAGS differs from its policy was admitted")
	}
}

// TestCacheAmbientGoFlags: an operator's own GOFLAGS is extended, never replaced
// or duplicated, and an opted-out policy leaves it alone.
func TestCacheAmbientGoFlags(t *testing.T) {
	p := fixture(t)
	for ambient, want := range map[string]string{
		"":                     "-trimpath",
		"   ":                  "-trimpath",
		"-mod=mod":             "-mod=mod -trimpath",
		"-trimpath":            "-trimpath",
		"-mod=mod  -trimpath ": "-mod=mod -trimpath",
	} {
		if got := p.WithAmbientGoFlags(ambient).GoFlags; got != want {
			t.Errorf("ambient %q: got %q, want %q", ambient, got, want)
		}
	}
	off := p
	off.GoFlags = ""
	if got := off.WithAmbientGoFlags("-mod=mod").GoFlags; got != "" {
		t.Errorf("opted-out policy took ambient flags: %q", got)
	}
	merged := p.WithAmbientGoFlags("-mod=mod")
	l, err := Acquire(merged.Env())
	if err != nil {
		t.Fatal("merged-flags consumer refused", err)
	}
	l.Finish(true)
}

// TestCacheBuildReuseAcrossWorktrees is the growth this default exists to stop:
// the same package built from a second directory must reuse the first build's
// cache entries. With the cell opted out, cmd/go keys the package on its
// absolute directory and recompiles it, which is the control.
func TestCacheBuildReuseAcrossWorktrees(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go unavailable")
	}
	for _, trimpath := range []bool{true, false} {
		p := fixture(t)
		p.Budget = 1 << 30
		if !trimpath {
			p.GoFlags = ""
		}
		for i := 0; i < 2; i++ {
			work := t.TempDir() // a new directory every build, as a dispatch worktree is
			if err = os.WriteFile(filepath.Join(work, "go.mod"), []byte("module cache.fixture\n\ngo 1.25\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(work, "cache.go"), []byte("package cache\nfunc Sum(a,b int) int{return a+b}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			env := append(p.Env(), "HOME="+t.TempDir(), "PATH="+os.Getenv("PATH"), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0")
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
				t.Fatalf("trimpath=%v: cold build never compiled: %s", trimpath, out)
			}
			if i == 1 && compiled == trimpath {
				t.Fatalf("trimpath=%v: second directory compiled=%v: %s", trimpath, compiled, out)
			}
		}
	}
}
