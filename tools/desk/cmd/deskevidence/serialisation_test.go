package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// syncWriter makes the package-level stdout/stderr hooks safe for the two concurrent
// invocations these tests run. bytes.Buffer is not.
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// seedCharged writes n audit entries for deskevidence that CHARGE the rolling-hour write
// budget, so the next write is the last one under the cap. Written directly through
// deskkit.Log, the same path the tool uses.
func seedCharged(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool:   toolName,
			Verb:   "commit",
			Repo:   "example-org/tracker",
			Result: deskkit.ResultOK,
			Detail: "seed",
		}); err != nil {
			t.Fatalf("seed audit %d: %v", i, err)
		}
	}
}

// TestAuditLockIsHeldAcrossTheRemoteCall is the MECHANISM test: it proves the write window
// is locked for the whole of AllowWrite → commit → audit-append, by probing the lock from
// inside the PUT handler.
//
// flock(2) locks attach to the open file DESCRIPTION, not to the process, so a second
// open() of the same file inside this test contends with the tool's lock exactly as a
// second desk session would. The probe is non-blocking: it either gets the lock (the
// window is unserialised) or it gets EWOULDBLOCK (the window is held).
//
// On pre-#227 main this fails: there is no flock at all, so the probe acquires cleanly
// while the tool is mid-PUT.
func TestAuditLockIsHeldAcrossTheRemoteCall(t *testing.T) {
	f, _ := setupFake(t)

	var probedFree bool
	var probeErr error
	f.onPut = func() {
		dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
		// Create the dir if the tool has not: on a build with NO lock the desk-tools dir
		// may not exist yet at PUT time (deskkit.Log creates it later, at finalize). The
		// probe must then report "the lock was FREE", not "the file was missing" — those
		// are the same defect and the assertion below should name it as one.
		if err := os.MkdirAll(dir, 0o700); err != nil {
			probeErr = err
			return
		}
		fh, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			probeErr = err
			return
		}
		defer fh.Close()
		if err := deskkit.TryLockExclusive(fh); err == nil {
			probedFree = true
			_ = deskkit.UnlockFile(fh)
		}
	}

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old", "old-sha")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	if probeErr != nil {
		t.Fatalf("probe could not open the lock file: %v", probeErr)
	}
	if probedFree {
		t.Fatal("the audit lock was FREE during the Contents-API PUT — the write window is unserialised (#227)")
	}
}

// TestConcurrentInvocationsCannotBothSpendTheLastWrite is the CONSEQUENCE test: the
// observable defect #227 describes, which is that two invocations each read a budget
// below the cap and both write.
//
// Setup: (effective-cap − 1) charged entries are seeded, so exactly ONE write remains.
// Invocation A is held inside its PUT (i.e. after its AllowWrite, before its audit
// append) while invocation B runs to completion or blocks.
//
//	unserialised (pre-#227): B walks straight past AllowWrite — the ledger still shows
//	                         (effective-cap − 1) charged entries, because A has not
//	                         appended yet — and commits. putCalls == 2, B exits 0.
//	serialised   (fixed):    B blocks on the flock until A has appended, then reads a
//	                         spent budget. putCalls == 1, B exits 4 (rate-limited).
//
// Note what the fix is NOT: B is not retried and it does not lose a race. It waits, and
// then reads the ledger A actually wrote. A retry loop would only make the double-spend
// rarer; this makes it unreachable.
func TestConcurrentInvocationsCannotBothSpendTheLastWrite(t *testing.T) {
	f, _ := setupFake(t)

	// Concurrency-safe capture for the two goroutines.
	sw := &syncWriter{}
	oldOut, oldErr := stdout, stderr
	stdout, stderr = sw, sw
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })

	// Seed to ONE below the tool's EFFECTIVE unnumbered cap, not the base
	// RateLimitPerPRPerHour: deskevidence carries a per-tool override (30), so seeding to the
	// base would leave the bucket wide open and neither invocation would be the "last write".
	// UnnumberedCapFor is the same single source the gate consults, so this pin can never
	// drift from the override.
	effCap := deskkit.UnnumberedCapFor(toolName)
	seedCharged(t, effCap-1)

	putEntered := make(chan struct{})
	releaseA := make(chan struct{})
	var once sync.Once
	f.onPut = func() {
		once.Do(func() {
			close(putEntered)
			<-releaseA
		})
	}

	pathA := writeRepoFile(t, "docs/brief-a.md", "# A\n\n## Evidence\n| 1 | a | a |\n")
	pathB := writeRepoFile(t, "docs/brief-b.md", "# B\n\n## Evidence\n| 1 | b | b |\n")
	f.setFile(pathA, "old-a", "sha-a")
	f.setFile(pathB, "old-b", "sha-b")

	aDone := make(chan int, 1)
	go func() {
		aDone <- run([]string{"example-org/tracker", "main", "--evidence-file", pathA})
	}()

	select {
	case <-putEntered:
	case code := <-aDone:
		t.Fatalf("A finished (exit %d) without reaching the PUT", code)
	case <-time.After(10 * time.Second):
		t.Fatal("A never reached the PUT")
	}

	bDone := make(chan int, 1)
	go func() {
		bDone <- run([]string{"example-org/tracker", "main", "--evidence-file", pathB})
	}()

	// Ample time for B to run to completion if nothing is holding it back. An in-process
	// httptest round trip is microseconds; this is four orders of magnitude of slack, and
	// it only ever makes the UNFIXED behaviour easier to observe.
	time.Sleep(500 * time.Millisecond)
	close(releaseA)

	if code := <-aDone; code != deskkit.ExitOK {
		t.Fatalf("A exit = %d, want 0", code)
	}
	var bCode int
	select {
	case bCode = <-bDone:
	case <-time.After(10 * time.Second):
		t.Fatal("B never finished after A released the lock")
	}

	f.mu.Lock()
	puts := f.putCalls
	f.mu.Unlock()

	if bCode != deskkit.ExitRateLimited {
		t.Errorf("B exit = %d, want %d (rate-limited) — B spent a budget A had already taken",
			bCode, deskkit.ExitRateLimited)
	}
	if puts != 1 {
		t.Errorf("putCalls = %d, want 1 — two invocations both committed past a one-write budget (#227)", puts)
	}

	// And the ledger must agree: exactly the effective cap's worth of charged entries, never
	// more.
	entries := auditEntries(t)
	charged := 0
	for _, e := range entries {
		if e.Tool == toolName && (e.Result == deskkit.ResultOK || e.Result == deskkit.ResultUnverifiable) {
			charged++
		}
	}
	if charged > effCap {
		t.Errorf("audit shows %d charged writes, over the %d cap", charged, effCap)
	}
}

// TestLockFailureStillWritesOneAuditLine pins the fail-closed half: if the lock cannot be
// acquired the invocation is unverifiable (exit 6) and still leaves exactly one audit
// line, so the attempt is visible to the gates. A held lock is not "assume free".
func TestLockFailureStillWritesOneAuditLine(t *testing.T) {
	f, _ := setupFake(t)

	// Make the desk-tools dir a FILE so MkdirAll/OpenFile cannot produce a lock —
	// a deterministic acquisition failure that needs no 60s wait.
	deskPath := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	// deskkit.Log needs the dir to exist for its own audit file, so record the
	// pre-state, then break only the lock file's own path.
	if err := os.MkdirAll(deskPath, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(deskPath, "audit.lock"), 0o700); err != nil {
		t.Fatalf("mkdir audit.lock: %v", err) // a DIRECTORY named audit.lock -> OpenFile fails
	}

	before := len(auditEntries(t))
	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("putCalls = %d, want 0 — nothing may be written without the lock", f.putCalls)
	}
	after := auditEntries(t)
	if len(after)-before != 1 {
		t.Fatalf("wrote %d audit lines, want exactly 1", len(after)-before)
	}
	if got := after[len(after)-1].Result; got != deskkit.ResultUnverifiable {
		t.Fatalf("audit result = %q, want unverifiable", got)
	}
}

// TestContendedLockTimesOutUnverifiable pins the stale-lock deadline. Removing it (wait
// forever) survived every other test in this package — TestLockFailureStillWritesOneAuditLine
// covers the OpenFile-failure path, never the contended-wait path (#406).
//
// The deadline is injected rather than waited out: lockWait exists as a package var for
// exactly this, and shortening it here does not weaken the shipped value (60s), which the
// var's doc comment pins. Note what is being asserted — that a lock it cannot get makes
// the tool REFUSE (exit 6, one audit line, zero writes). It never proceeds on the
// assumption that the budget is free.
//
// The run is watched from a channel so that a build with no deadline fails in 30s with a
// named message instead of hanging the package until go test's global timeout.
func TestContendedLockTimesOutUnverifiable(t *testing.T) {
	f, _ := setupFake(t)

	// Hold the lock from a second open file DESCRIPTION. flock attaches to the description,
	// not the process, so this contends with the tool exactly as another desk session would.
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir desk-tools: %v", err)
	}
	holder, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	if err := deskkit.TryLockExclusive(holder); err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	t.Cleanup(func() {
		_ = deskkit.UnlockFile(holder)
		_ = holder.Close()
	})

	const shortWait = 200 * time.Millisecond
	old := lockWait
	lockWait = shortWait
	t.Cleanup(func() { lockWait = old })

	before := len(auditEntries(t))
	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old", "old-sha")

	done := make(chan int, 1)
	start := time.Now()
	go func() {
		done <- run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	}()

	var code int
	select {
	case code = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("deskevidence never returned while the lock was held — the stale-lock deadline is gone; " +
			"a tool that waits forever on a stuck holder is not fail-closed, it is hung")
	}

	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable) — a lock it could not get is not permission to write",
			code, deskkit.ExitUnverifiable)
	}
	if elapsed := time.Since(start); elapsed < shortWait {
		t.Errorf("returned after %s, want at least the %s deadline — it must WAIT for the lock, "+
			"not give up on first contention", elapsed, shortWait)
	}
	f.mu.Lock()
	puts := f.putCalls
	f.mu.Unlock()
	if puts != 0 {
		t.Fatalf("putCalls = %d, want 0 — nothing may be written without the lock", puts)
	}
	after := auditEntries(t)
	if len(after)-before != 1 {
		t.Fatalf("wrote %d audit lines, want exactly 1", len(after)-before)
	}
	if got := after[len(after)-1].Result; got != deskkit.ResultUnverifiable {
		t.Fatalf("audit result = %q, want unverifiable", got)
	}
}

// TestDeferOrderIsSourceGuarded guards the defer order in runOutward at SOURCE level, in
// the style of TestInstallForOwnerIsGone.
//
// runOutward relies on Go's LIFO defer order: `lock.release()` is registered FIRST so it
// runs LAST, i.e. AFTER the deferred audit append. Swapping the two registrations moves
// the append outside the critical section and re-opens the precise window #227 is about,
// and the file and the README both say "do not reorder" — but saying it is not enforcing it.
//
// This is a source guard rather than a behavioural test because the property is not
// observable from one: the gap between release() and the single O_APPEND write is
// microseconds, and the #406 review confirmed a purpose-built 50ms poller could not catch
// the swap even at -count=25. A source guard fails loudly on the refactor that would cause
// the defect; a behavioural test would pass either way and give false assurance.
func TestDeferOrderIsSourceGuarded(t *testing.T) {
	src, err := os.ReadFile("writeflow.go")
	if err != nil {
		t.Fatalf("read writeflow.go: %v", err)
	}
	relIdx, finIdx := -1, -1
	for i, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue // the doc comment discusses both; only code counts
		}
		if relIdx < 0 && strings.HasPrefix(trimmed, "defer lock.release()") {
			relIdx = i
		}
		if finIdx < 0 && strings.Contains(trimmed, "defer") && strings.Contains(trimmed, "ac.finalize(err)") {
			finIdx = i
		}
	}
	if relIdx < 0 {
		t.Fatal("no `defer lock.release()` in writeflow.go — the lock is not released on every exit path, " +
			"or it was renamed and this guard needs updating alongside it")
	}
	if finIdx < 0 {
		t.Fatal("no deferred `ac.finalize(err)` in writeflow.go — the single audit line is not deferred, " +
			"or it was renamed and this guard needs updating alongside it")
	}
	if relIdx > finIdx {
		t.Fatalf("`defer lock.release()` (line %d) is registered AFTER the deferred audit append (line %d). "+
			"Go runs defers LIFO, so the lock would now be released BEFORE the append — putting the append "+
			"outside the critical section and re-opening #227. Register release() first.",
			relIdx+1, finIdx+1)
	}
}

// TestAuditLockPathMatchesTheOtherDeskTools guards the property that makes the lock work
// at all: deskpost, deskrelease and deskevidence share ONE audit.jsonl, so they must
// share ONE lock file. A copy that drifted to its own path would serialise a tool against
// itself and against nothing else.
func TestAuditLockPathMatchesTheOtherDeskTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	dir, err := deskDir()
	if err != nil {
		t.Fatalf("deskDir: %v", err)
	}
	want := filepath.Join(home, ".config", "assay")
	if dir != want {
		t.Fatalf("deskDir = %q, want %q (cmd/deskpost/writeflow.go uses the same path)", dir, want)
	}
	l, err := acquireAuditLock()
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer l.release()
	if _, serr := os.Stat(filepath.Join(want, "audit.lock")); serr != nil {
		t.Fatalf("lock file not at %s/audit.lock: %v", want, serr)
	}
}
