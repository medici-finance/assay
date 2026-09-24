package deskkit

// Class guard: no test in this package may resolve the operator's REAL state directory.
//
// The real state directory (~/.config/assay) holds the audit log that scan overrides and
// every outward write are reviewed from, plus the kill switch and the roster beacons. A test
// that appends there pollutes that trail with fixture rows carrying the local identity.
// Per-test discipline (call setup, or t.Setenv a temp HOME) is what a new test forgets, so
// the guard is package-wide instead: TestMain resolves the real directory BEFORE the fixture
// redirects the home, and installs the deskDir test hook (stateDirGuard) that REFUSES it.
//
// Refusing at resolution — not snapshotting the file afterwards — is deliberate on two
// counts. It stops the write before it happens, so the guard cannot itself be the
// pollution; and a stat/size comparison of a live audit log is racy on any machine where a
// real desk is appending to it while the suite runs. Every writer in this package reaches
// the state directory through deskDir (audit.go, auditrecover.go, killswitch.go, and the
// StateDir consumers), so one hook covers the whole class.
//
// A refused resolution is recorded with the test that caused it, and TestMain fails the run
// when any were recorded — so a test that swallows the refusal still turns the suite red.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// realStateDir is the operator's real state directory, resolved before the fixture home is
// installed. Empty when the home could not be resolved (the guard is then could-not-check,
// reported as such by TestMain and TestStateDirGuard).
var realStateDir string

type stateDirHit struct {
	test string
	dir  string
}

var (
	stateDirHitsMu sync.Mutex
	stateDirHits   []stateDirHit
)

// installStateDirGuard resolves the real state directory from the process's current home
// and installs the refusing hook. It must run before anything changes the home variables.
func installStateDirGuard() (uninstall func()) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Fprintf(os.Stderr, "statedirguard: could-not-check — cannot resolve the real home (%v); "+
			"the real-state-directory guard is NOT installed for this run\n", err)
		return func() {}
	}
	realStateDir = filepath.Join(home, ".config", "assay")
	prev := stateDirGuard
	stateDirGuard = refuseRealStateDir
	return func() { stateDirGuard = prev }
}

// refuseRealStateDir is the hook body: dir inside the real state directory is recorded and
// refused; anything else passes through untouched.
func refuseRealStateDir(dir string) error {
	if realStateDir == "" || !withinDir(dir, realStateDir) {
		return nil
	}
	stateDirHitsMu.Lock()
	stateDirHits = append(stateDirHits, stateDirHit{test: callingTest(), dir: dir})
	stateDirHitsMu.Unlock()
	return Unverifiable("test guard: refusing the REAL desk-tools state directory "+dir+
		" — point the test at t.TempDir() (setup(t), or t.Setenv HOME)", nil)
}

// withinDir reports whether p is base or lies beneath it, comparing both the cleaned
// spellings and, where they resolve, the symlink-evaluated ones.
func withinDir(p, base string) bool {
	in := func(p, base string) bool {
		return p == base || strings.HasPrefix(p, base+string(filepath.Separator))
	}
	if in(filepath.Clean(p), filepath.Clean(base)) {
		return true
	}
	rp, perr := filepath.EvalSymlinks(p)
	rb, berr := filepath.EvalSymlinks(base)
	return perr == nil && berr == nil && in(rp, rb)
}

// callingTest names the Test function on the current stack, so the failure report says
// which test to fix rather than only that one exists.
func callingTest() string {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		f, more := frames.Next()
		name := f.Function[strings.LastIndex(f.Function, "/")+1:]
		name = strings.TrimPrefix(name, "deskkit.")
		if strings.HasPrefix(name, "Test") && strings.HasSuffix(f.File, "_test.go") {
			return name
		}
		if !more {
			return "unknown (no Test frame on the stack)"
		}
	}
}

func stateDirHitCount() int {
	stateDirHitsMu.Lock()
	defer stateDirHitsMu.Unlock()
	return len(stateDirHits)
}

// reportStateDirHits prints every recorded hit and reports whether there were any.
func reportStateDirHits(w io.Writer) bool {
	stateDirHitsMu.Lock()
	defer stateDirHitsMu.Unlock()
	if len(stateDirHits) == 0 {
		return false
	}
	fmt.Fprintf(w, "FAIL: statedirguard: %d attempt(s) to resolve the REAL state directory %s:\n",
		len(stateDirHits), realStateDir)
	for _, h := range stateDirHits {
		fmt.Fprintf(w, "  %s -> %s\n", h.test, h.dir)
	}
	return true
}

// TestStateDirGuard proves the guard is live: with the home pointed back at the real one
// (and, separately, the override pointed at the real directory) deskDir refuses, Log writes
// nothing, and each refusal is recorded against this test. It then removes its own planted
// records so they do not fail the run. It is not parallel (t.Setenv), and top-level parallel
// tests do not start until the sequential ones finish, so nothing else records meanwhile.
func TestStateDirGuard(t *testing.T) {
	if realStateDir == "" {
		t.Skip("could-not-check: the real home was not resolvable, so the guard is not installed")
	}
	// Control: the fixture home TestMain installed resolves, and is not the real one.
	if dir, err := deskDir(); err != nil || withinDir(dir, realStateDir) {
		t.Fatalf("control: fixture state dir = %q, %v; want a non-real dir and no error", dir, err)
	}

	n0 := stateDirHitCount()
	t.Cleanup(func() {
		stateDirHitsMu.Lock()
		stateDirHits = stateDirHits[:n0]
		stateDirHitsMu.Unlock()
	})

	realHome := filepath.Dir(filepath.Dir(realStateDir))
	t.Setenv("HOME", realHome)
	t.Setenv("USERPROFILE", realHome)
	old := dirOverride
	dirOverride = ""
	t.Cleanup(func() { dirOverride = old })

	if dir, err := deskDir(); err == nil || dir != "" {
		t.Fatalf("deskDir under the real home = %q, %v; want a refusal", dir, err)
	}
	if err := Log(Entry{Tool: "deskpr", Verb: "guard-probe", Result: ResultOK}); err == nil {
		t.Fatal("Log under the real home succeeded; the guard did not stop the audit write")
	}
	dirOverride = filepath.Join(realStateDir, "sub")
	if _, err := deskDir(); err == nil {
		t.Fatal("deskDir with the override inside the real state dir was not refused")
	}

	stateDirHitsMu.Lock()
	got := append([]stateDirHit(nil), stateDirHits[n0:]...)
	stateDirHitsMu.Unlock()
	if len(got) != 3 {
		t.Fatalf("recorded hits = %d, want 3: %+v", len(got), got)
	}
	for _, h := range got {
		if h.test != "TestStateDirGuard" {
			t.Errorf("hit attributed to %q, want TestStateDirGuard", h.test)
		}
	}
}
