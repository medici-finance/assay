package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAcquireConcurrentExactlyOneWinner is the MUTUAL-EXCLUSION test: N goroutines call
// Acquire on the SAME fresh item; exactly one may win. The directory-wide flock serialises
// the create-or-reclaim decision so only the first O_EXCL create succeeds and every other
// racer reads a live claim and backs off (false, nil). More than one winner is
// double-dispatch.
func TestAcquireConcurrentExactlyOneWinner(t *testing.T) {
	dir := t.TempDir()
	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: time.Hour}

	const racers = 16
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
		errs []error
		st   = make(chan struct{})
	)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-st
			ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "feature/01", Owner: id2owner(id)})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if ok {
				wins++
			}
		}(i)
	}
	close(st)
	wg.Wait()

	if len(errs) != 0 {
		t.Fatalf("concurrent Acquire returned errors: %v", errs)
	}
	if wins != 1 {
		t.Fatalf("winners = %d, want exactly 1 — the item would be dispatched %d times", wins, wins)
	}
	// The slash in the item ID must resolve to a single segment inside the claims dir.
	if _, err := os.Stat(filepath.Join(dir, "feature_01.claim")); err != nil {
		t.Fatalf("claim file not at sanitized path: %v", err)
	}
}

// TestAcquireContendedLockIsUnverifiableNotFree pins the fail-closed half (the
// double-dispatch lesson): when the claims flock is HELD from a second open file description, Acquire must
// REFUSE (exit 6 unverifiable) — it must NEVER return (false, nil) "assume free".
//
// flock(2) locks attach to the open file DESCRIPTION, not the process, so a second open()
// of the lock file in this test contends with Acquire exactly as a second desk session
// would.
func TestAcquireContendedLockIsUnverifiableNotFree(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	holder, err := os.OpenFile(filepath.Join(dir, ".claims.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	if err := TryLockExclusive(holder); err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	t.Cleanup(func() {
		_ = UnlockFile(holder)
		_ = holder.Close()
	})

	const shortWait = 200 * time.Millisecond
	old := claimLockWait
	claimLockWait = shortWait
	t.Cleanup(func() { claimLockWait = old })

	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: time.Hour}
	start := time.Now()
	ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "x", Owner: "runner"})
	if ok {
		t.Fatal("Acquire returned acquired=true while the lock was held elsewhere")
	}
	if err == nil {
		t.Fatal("Acquire returned (false, nil) on a lock it could not get — that is 'assume free', the double-dispatch failure")
	}
	if !IsUnverifiable(err) {
		t.Fatalf("Acquire error = %v (exit %d), want Unverifiable (exit %d)", err, ExitCodeOf(err), ExitUnverifiable)
	}
	if elapsed := time.Since(start); elapsed < shortWait {
		t.Errorf("returned after %s, want at least the %s deadline — it must WAIT for the lock, not give up on first contention", elapsed, shortWait)
	}
	// Nothing was written: no claim file for the contended item.
	if _, serr := os.Stat(claimPath(cfg, "x")); !os.IsNotExist(serr) {
		t.Fatalf("a claim file was written despite the lock being unavailable: %v", serr)
	}
}

// TestAcquireStaleReclaimDoesNotDoubleGrant seeds a claim, ages it past the stale window,
// then races N reclaimers for it. Exactly one may win the reclaim; the rest read the
// freshly-rewritten (now live) claim and back off. The reclaim is an in-place rewrite, so
// the file never vanishes and no fresh claimant can slip through a Remove/recreate gap.
func TestAcquireStaleReclaimDoesNotDoubleGrant(t *testing.T) {
	dir := t.TempDir()
	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: 30 * time.Minute}
	if ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "issue-841", Owner: "seeder"}); err != nil || !ok {
		t.Fatalf("seed: ok=%v err=%v", ok, err)
	}
	path := claimPath(cfg, "issue-841")
	aged := time.Now().Add(-90 * time.Minute)
	if err := os.Chtimes(path, aged, aged); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	const racers = 16
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins []string
		errs []error
		st   = make(chan struct{})
	)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			owner := id2owner(id)
			<-st
			ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "issue-841", Owner: owner})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if ok {
				wins = append(wins, owner)
			}
		}(i)
	}
	close(st)
	wg.Wait()

	if len(errs) != 0 {
		t.Fatalf("concurrent reclaim returned errors: %v", errs)
	}
	if len(wins) != 1 {
		t.Fatalf("reclaim winners = %d (%v), want exactly 1", len(wins), wins)
	}
	// The file records the winner, and no directory lock is left held.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	var c Claim
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse claim: %v", err)
	}
	if c.Owner != wins[0] {
		t.Fatalf("claim records owner %q but the winner was %q", c.Owner, wins[0])
	}
}

// TestAcquireLiveClaimNotStolen: a recent claim is a collision, not a reclaim.
func TestAcquireLiveClaimNotStolen(t *testing.T) {
	dir := t.TempDir()
	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: 30 * time.Minute}
	if ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "a", Owner: "first"}); err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "a", Owner: "second"})
	if err != nil {
		t.Fatalf("second errored: %v", err)
	}
	if ok {
		t.Fatal("a live claim was stolen — double-dispatch is possible")
	}
}

// TestTolerantReadOfLegacyShapes proves the schema-fork heal: List reads a legacy
// loopengine-shape file AND a legacy roster-shape file, mapping their disjoint field names
// onto the canonical Item/Owner/TS.
func TestTolerantReadOfLegacyShapes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Legacy loopengine dispatch claim.
	writeRaw(t, filepath.Join(dir, "feature_01.claim"),
		`{"itemID":"feature/01","runner":"worker-desk","branch":"feat/x","claimed":"2026-08-01T00:00:00Z"}`)
	// Legacy roster / bash claim.
	writeRaw(t, filepath.Join(dir, "legacy-roster-09.claim"),
		`{"brief":"feature/09","repo":"assay","session":"worker-a","ts":"2026-07-17T13:52:05Z"}`)

	claims, err := List(ClaimConfig{ClaimsDir: dir})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[string]Claim{}
	for _, c := range claims {
		got[c.Item] = c
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d claims, want 2: %+v", len(got), claims)
	}
	if c := got["feature/01"]; c.Owner != "worker-desk" || c.Branch != "feat/x" || c.TS != "2026-08-01T00:00:00Z" {
		t.Fatalf("loopengine-shape mapped wrong: %+v", c)
	}
	if c := got["feature/09"]; c.Owner != "worker-a" || c.TS != "2026-07-17T13:52:05Z" {
		t.Fatalf("roster-shape mapped wrong: %+v", c)
	}
}

// TestStaleReadOfLegacyBranchShape: isStale must read a legacy loopengine-shape file's
// branch and, with the branch provably live, decline to reclaim even an aged claim.
func TestStaleReadOfLegacyBranchShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "brief-x.claim")
	writeRaw(t, path, `{"itemID":"brief-x","runner":"other","branch":"feat/x","claimed":"2020-01-01T00:00:00Z"}`)
	aged := time.Now().Add(-90 * time.Minute)
	_ = os.Chtimes(path, aged, aged)

	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: 30 * time.Minute, BranchActive: func(string) bool { return true }}
	ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "brief-x", Owner: "taker"})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if ok {
		t.Fatal("an aged legacy claim whose branch is provably live was stolen — tolerant branch read failed")
	}
}

// TestAcquireUnreadableExistingFailsClosed: an unreadable existing claim is Unverifiable,
// never reclaimed.
func TestAcquireUnreadableExistingFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: 0000 perms do not block reads")
	}
	dir := t.TempDir()
	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: time.Minute}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := claimPath(cfg, "z")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	aged := time.Now().Add(-time.Hour)
	_ = os.Chtimes(path, aged, aged)

	ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "z", Owner: "r"})
	if ok {
		t.Fatal("unreadable existing claim reclaimed — must fail closed, not steal")
	}
	if err == nil || !IsUnverifiable(err) {
		t.Fatalf("want Unverifiable, got %v", err)
	}
}

// TestReleaseAndReacquire: Release removes the claim; a subsequent Acquire gets it fresh.
func TestReleaseAndReacquire(t *testing.T) {
	dir := t.TempDir()
	cfg := ClaimConfig{ClaimsDir: dir}
	if ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "a", Owner: "one"}); err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	if err := Release(cfg, "a"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := Release(cfg, "a"); err != nil {
		t.Fatalf("release of a missing claim must be a no-op, got: %v", err)
	}
	if ok, err := Acquire(cfg, Claim{Kind: KindDispatch, Item: "a", Owner: "two"}); err != nil || !ok {
		t.Fatalf("re-acquire after release: ok=%v err=%v", ok, err)
	}
}

// TestReleaseMatchingCompareAndDelete pins the compare-and-delete contract crash recovery
// relies on: ReleaseMatching removes a claim ONLY when the on-disk owner/branch/ts still match
// the claim that was classified. A claim reclaimed IN PLACE underneath the caller (a different
// ts, as Acquire's stale reclaim writes) is left untouched — otherwise recovery would delete a
// live claim, reopening the double-dispatch window. A matching claim is removed; a missing one
// is a no-op.
func TestReleaseMatchingCompareAndDelete(t *testing.T) {
	dir := t.TempDir()
	cfg := ClaimConfig{ClaimsDir: dir, StaleClaim: time.Hour}

	want := Claim{Kind: KindDispatch, Item: "feature/03", Owner: "worker-app", Branch: "brief/x-11", TS: "2026-08-16T00:00:00Z"}
	if ok, err := Acquire(cfg, want); err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}

	// A claim that no longer matches (someone reclaimed it in place → different owner+ts) must
	// NOT be removed.
	stale := want
	stale.Owner = "other-app"
	stale.TS = "2020-01-01T00:00:00Z"
	if removed, err := ReleaseMatching(cfg, stale); err != nil || removed {
		t.Fatalf("ReleaseMatching on a non-matching claim: removed=%v err=%v — must leave the live claim in place", removed, err)
	}
	if _, err := os.Stat(claimPath(cfg, "feature/03")); err != nil {
		t.Fatalf("a non-matching ReleaseMatching deleted the claim file: %v", err)
	}

	// The exact claim that was classified IS removed.
	if removed, err := ReleaseMatching(cfg, want); err != nil || !removed {
		t.Fatalf("ReleaseMatching on the matching claim: removed=%v err=%v — must remove it", removed, err)
	}
	if _, err := os.Stat(claimPath(cfg, "feature/03")); !os.IsNotExist(err) {
		t.Fatalf("matching ReleaseMatching did not remove the claim file (err=%v)", err)
	}

	// A missing claim is a no-op, never an error.
	if removed, err := ReleaseMatching(cfg, want); err != nil || removed {
		t.Fatalf("ReleaseMatching of a missing claim: removed=%v err=%v — want (false,nil)", removed, err)
	}
}

// TestListMissingDirIsEmpty: a missing claims dir is empty history, not an error.
func TestListMissingDirIsEmpty(t *testing.T) {
	claims, err := List(ClaimConfig{ClaimsDir: filepath.Join(t.TempDir(), "nope")})
	if err != nil {
		t.Fatalf("List of a missing dir: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("List of a missing dir = %v, want empty", claims)
	}
}

// TestAcquireNeverAssumesFree_Source is the SOURCE GUARD for the fail-closed hard invariant: no
// code path in claim.go may grant a claim before the flock is held, and the lock helper
// must fail closed. This is a source guard rather than a behavioural test because "there
// exists no future edit that grants before locking" is a property of the code, not of any
// single run — a behavioural test passes for the code as written and says nothing about
// the edit that would reintroduce the defect.
func TestAcquireNeverAssumesFree_Source(t *testing.T) {
	src, err := os.ReadFile("claim.go")
	if err != nil {
		t.Fatalf("read claim.go: %v", err)
	}
	text := string(src)

	// 1. A real exclusive-lock call must exist. The lock primitive moved behind
	//    deskkit.TryLockExclusive (the unix/windows build-tag split); the fail-closed
	//    flock itself now lives in filelock_unix.go, and the windows variant's
	//    fail-closed-ness is pinned separately (ErrLockBusy in filelock_windows.go).
	//    This guard still binds the SAME invariant here: claim.go takes the lock, and
	//    nothing grants a claim before it.
	flockIdx := strings.Index(text, "TryLockExclusive(")
	if flockIdx < 0 {
		t.Fatal("claim.go has no TryLockExclusive — the claim is not lock-serialised (the lock-serialisation fix is gone)")
	}

	// 2. No claim may be GRANTED (`return true`) textually before the lock is taken. An
	//    early `return true` above the flock call would be a grant without the lock.
	for _, tok := range []string{"return true"} {
		for idx := 0; ; {
			at := strings.Index(text[idx:], tok)
			if at < 0 {
				break
			}
			abs := idx + at
			// Skip occurrences inside comments/doc prose is not needed here: `return true`
			// is code-only in this file. Any grant must sit below the flock call.
			if abs < flockIdx {
				t.Fatalf("`%s` appears at offset %d, BEFORE the flock at %d — a claim may not be granted before the lock is held", tok, abs, flockIdx)
			}
			idx = abs + len(tok)
		}
	}

	// 3. The lock helper must FAIL CLOSED: on the contention/timeout path it returns
	//    Unverifiable, never a nil error and never a granted claim. Assert the helper body
	//    carries Unverifiable and never returns (nil, nil).
	helper := sliceFunc(text, "func acquireClaimLock(")
	if helper == "" {
		t.Fatal("acquireClaimLock not found — the fail-closed lock helper was renamed; update this guard")
	}
	if !strings.Contains(helper, "Unverifiable(") {
		t.Fatal("acquireClaimLock does not return Unverifiable — a lock it cannot hold must fail closed (exit 6), not silently succeed")
	}
	if strings.Contains(helper, "return nil, nil") {
		t.Fatal("acquireClaimLock has a `return nil, nil` — a lock helper that reports success with no lock is 'assume free'")
	}

	// 4. Acquire must PROPAGATE the lock error (fail closed), never swallow it into a
	//    (false, nil) "assume free".
	acq := sliceFunc(text, "func Acquire(")
	if acq == "" {
		t.Fatal("Acquire not found — update this guard")
	}
	if !strings.Contains(acq, "return false, lerr") {
		t.Fatal("Acquire does not propagate the lock error (`return false, lerr`) — a lock it could not hold must surface as Unverifiable, never be assumed free")
	}
}

// --- test helpers ---

func id2owner(id int) string {
	return "runner-" + string(rune('a'+id%26)) + string(rune('0'+id/26))
}

func writeRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// sliceFunc returns the source of the function whose signature starts with prefix, from
// that signature to the first line that is a bare "}" at column 0 (the closing brace of a
// top-level func). Empty string if the prefix is not found.
func sliceFunc(text, prefix string) string {
	start := strings.Index(text, prefix)
	if start < 0 {
		return ""
	}
	rest := text[start:]
	if end := strings.Index(rest, "\n}\n"); end >= 0 {
		return rest[:end+3]
	}
	return rest
}
