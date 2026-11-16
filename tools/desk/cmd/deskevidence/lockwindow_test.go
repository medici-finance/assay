package main

// lockwindow_test.go — #5023: the shared audit lock spans only the write window
// (budget → conditional forge write → audit append), never a landing's preparation.
// Preparation used to run inside it, so one landing's lint held the desk-wide lock and
// starved every other desk writer that takes it. Preparation keeps its own exclusion,
// scoped to the landing root it stages into.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// probeAuditLock reports whether a second writer could take the audit lock right now,
// the way another desk tool would: through the shared choke point, with a short wait.
func probeAuditLock(t *testing.T) bool {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	l, err := deskkit.AcquireAuditLock(dir, 100*time.Millisecond)
	if err != nil {
		return false
	}
	l.Release()
	return true
}

// stubPrep replaces the PROBLEM-diff seam — the slow, staging part of preparation —
// with fn, and restores it.
func stubPrep(t *testing.T, fn func(root string)) {
	t.Helper()
	old := lintDiffFn
	lintDiffFn = func(root, _ string, _ []byte, _ childOptions) ([]string, error) {
		fn(root)
		return nil, nil
	}
	t.Cleanup(func() { lintDiffFn = old })
}

// regression: #5023
func TestPrepFreesAuditLock(t *testing.T) {
	f, errBuf := setupFake(t)
	var prepFree, putFree bool
	stubPrep(t, func(string) { prepFree = probeAuditLock(t) })
	f.onPut = func() { putFree = probeAuditLock(t) }

	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if !prepFree {
		t.Error("the audit lock was HELD during preparation — another desk writer waiting on it would starve (#5023)")
	}
	if putFree {
		t.Error("the audit lock was FREE during the forge write — the write window is unserialised (#227)")
	}
}

// regression: #5023
func TestOutcomePrepFreesAuditLock(t *testing.T) {
	f, errBuf := setupFake(t)
	var prepFree, putFree bool
	stubPrep(t, func(string) { prepFree = probeAuditLock(t) })
	f.onPut = func() { putFree = probeAuditLock(t) }

	line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
	recFile := writeRepoFile(t, "record.json", line+"\n")
	if code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if !prepFree {
		t.Error("the audit lock was HELD during outcome-record preparation (#5023)")
	}
	if putFree {
		t.Error("the audit lock was FREE during the outcome-record forge write (#227)")
	}
}

// twoLandings runs landing A until it is inside preparation, then starts landing B and
// reports whether B reached preparation while A was still in it.
func twoLandings(t *testing.T, rootA, pathA, rootB, pathB string) (overlapped bool) {
	t.Helper()
	f, _ := setupFake(t)
	sw := &syncWriter{}
	oldOut, oldErr := stdout, stderr
	stdout, stderr = sw, sw
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })
	f.setFile(pathA, "old-a")
	f.setFile(pathB, "old-b")

	var mu sync.Mutex
	inPrep := 0
	aIn := make(chan struct{})
	releaseA := make(chan struct{})
	var once sync.Once
	stubPrep(t, func(string) {
		mu.Lock()
		inPrep++
		if inPrep > 1 {
			overlapped = true
		}
		mu.Unlock()
		first := false
		once.Do(func() { first = true })
		if first {
			close(aIn)
			<-releaseA
		}
		mu.Lock()
		inPrep--
		mu.Unlock()
	})

	aDone := make(chan int, 1)
	go func() {
		aDone <- run([]string{"example-org/tracker", "main", "--evidence-file", pathA, "--root", rootA})
	}()
	select {
	case <-aIn:
	case <-time.After(10 * time.Second):
		t.Fatal("A never reached preparation")
	}
	bDone := make(chan int, 1)
	go func() {
		bDone <- run([]string{"example-org/tracker", "main", "--evidence-file", pathB, "--root", rootB})
	}()
	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	got := overlapped
	mu.Unlock()
	close(releaseA)
	for name, ch := range map[string]chan int{"A": aDone, "B": bDone} {
		select {
		case code := <-ch:
			if code != deskkit.ExitOK {
				t.Fatalf("%s exit = %d, want 0 (output %q)", name, code, sw.buf.String())
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s never finished", name)
		}
	}
	return got
}

// regression: #5023
func TestOtherRootPrepConcurrent(t *testing.T) {
	pathA, pathB := "docs/streams/x/brief-a.md", "docs/streams/x/brief-b.md"
	rootA := rootWithFile(t, pathA, "# A\n\n## Evidence\n| 1 | a | a |\n")
	rootB := rootWithFile(t, pathB, "# B\n\n## Evidence\n| 1 | b | b |\n")
	if !twoLandings(t, rootA, pathA, rootB, pathB) {
		t.Fatal("landing B on another root could not prepare while A prepared — preparation is serialised desk-wide (#5023)")
	}
}

// regression: #5023
func TestSameRootPrepSerialised(t *testing.T) {
	pathA, pathB := "docs/streams/x/brief-a.md", "docs/streams/x/brief-b.md"
	root := rootWithFile(t, pathA, "# A\n\n## Evidence\n| 1 | a | a |\n")
	if err := os.WriteFile(filepath.Join(root, pathB), []byte("# B\n\n## Evidence\n| 1 | b | b |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if twoLandings(t, root, pathA, root, pathB) {
		t.Fatal("two landings staged into the SAME root at once — preparation lost its per-root exclusion")
	}
}

// regression: #5023
//
// TestBudgetOnlyInWriteWindow is the defect-class guard: the write budget is spent ONLY
// through enterWriteWindow, which takes the audit lock first. A landing path that called
// evidenceWriteBudget directly would spend the budget outside the lock (#227); one that
// re-took the lock at its start would starve the other writers again (#5023).
func TestBudgetOnlyInWriteWindow(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`\bevidenceWriteBudget\(`)
	take := regexp.MustCompile(`\bacquireAuditLock\(`)
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		fn := ""
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(line, "func ") {
				fn = line
				continue
			}
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue
			}
			if call.MatchString(code) && !(name == "writeflow.go" && strings.Contains(fn, ") enterWriteWindow(")) {
				t.Errorf("%s:%d spends the write budget outside enterWriteWindow: %s", name, i+1, code)
			}
			if take.MatchString(code) && !(name == "writeflow.go" &&
				(strings.Contains(fn, ") enterWriteWindow(") || strings.Contains(fn, ") lockForAppend("))) {
				t.Errorf("%s:%d takes the audit lock outside the write window: %s", name, i+1, code)
			}
		}
	}
}

// holdLockFile takes an exclusive flock on dir/name from a second open file description, the
// way another desk session would hold it, and releases it at cleanup.
func holdLockFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	h, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := deskkit.TryLockExclusive(h); err != nil {
		t.Fatalf("hold %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = deskkit.UnlockFile(h)
		_ = h.Close()
	})
}

// refusedRunTime runs one landing on root against held locks with lockWait shortened to wait,
// asserts the fail-closed refusal (exit 6, one audit line, no write) and returns how long the
// run took.
func refusedRunTime(t *testing.T, f *fakeForge, root, evidencePath string, wait time.Duration) time.Duration {
	t.Helper()
	old := lockWait
	lockWait = wait
	t.Cleanup(func() { lockWait = old })
	before := len(auditEntries(t))
	start := time.Now()
	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	elapsed := time.Since(start)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
	}
	f.mu.Lock()
	puts := f.putCalls
	f.mu.Unlock()
	if puts != 0 {
		t.Fatalf("putCalls = %d, want 0", puts)
	}
	if n := len(auditEntries(t)) - before; n != 1 {
		t.Fatalf("wrote %d audit lines, want exactly 1", n)
	}
	return elapsed
}

// regression: #5023
//
// TestAuditWaitIsBounded: a run that cannot get the audit lock refuses after ONE lockWait. The
// deferred audit append must not wait a second full lockWait for the same busy lock.
func TestAuditWaitIsBounded(t *testing.T) {
	f, _ := setupFake(t)
	holdLockFile(t, filepath.Join(os.Getenv("HOME"), ".config", "assay"), "audit.lock")
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old")
	const wait = 400 * time.Millisecond
	if elapsed := refusedRunTime(t, f, root, evidencePath, wait); elapsed > wait+wait/2 {
		t.Fatalf("refused after %s, want at most %s: the run waited on the audit lock more than once "+
			"(the documented bound is one lockWait in total)", elapsed, wait+wait/2)
	}
}

// regression: #5023
//
// TestPrepRefusalWaitsOnce: a run whose preparation lock stays busy refuses after ONE lockWait
// in total, even when the audit lock its refusal row needs is busy too.
func TestPrepRefusalWaitsOnce(t *testing.T) {
	f, _ := setupFake(t)
	stateDir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if real, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = real
	}
	holdLockFile(t, stateDir, prepLockName(abs))
	holdLockFile(t, stateDir, "audit.lock")
	const wait = 400 * time.Millisecond
	if elapsed := refusedRunTime(t, f, root, evidencePath, wait); elapsed > wait+wait/2 {
		t.Fatalf("refused after %s, want at most %s: the preparation-lock wait and the audit-lock "+
			"wait were not one shared lockWait", elapsed, wait+wait/2)
	}
}
