package main

// gitlab_concurrency_test.go — the two layers that stop one window's parallel mints from
// revoking their own role's live PAT:
//
//   1. concurrent mints for ONE role are SERIALISED, so neither presents a token the other
//      has already invalidated and the custody file always ends holding a live credential;
//   2. a mint that cannot take the lock REFUSES BEFORE contacting the rotate endpoint,
//      rather than starting a second overlapping rotation;
//   3. a --no-rotate lookup performs NO rotation at all, so a parallel sweep of read-only
//      verbs stops driving one rotation per call.
//
// Every request here is served by an in-process httptest fixture. Nothing in this file
// contacts a real GitLab deployment.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// concurrentRotateFixture models GitLab's self-rotation endpoint under CONCURRENT callers:
// every request is serialised on mu, the presented token is checked against the one live
// value, and a successful rotation atomically replaces it — so a caller presenting a token a
// peer already rotated away is rejected 401, exactly as the live endpoint does.
//
// makeRotateServer (gitlab_test.go) models the same endpoint but mutates its state without a
// mutex, which is correct for its sequential tests and a data race under concurrent ones.
// This fixture is that one guarded; it is not a weaker check.
type concurrentRotateFixture struct {
	mu       sync.Mutex
	valid    string // the single live token, as the server sees it
	issued   int    // successful rotations
	calls    int    // requests that reached the rotate endpoint
	rejected int    // requests presenting an already-invalidated token
}

func (f *concurrentRotateFixture) snapshot() (valid string, calls, rejected, issued int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.valid, f.calls, f.rejected, f.issued
}

func newConcurrentRotateServer(t *testing.T, initial string) (*httptest.Server, *concurrentRotateFixture) {
	t.Helper()
	f := &concurrentRotateFixture{valid: initial}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/personal_access_tokens/self/rotate") {
			http.Error(w, "not found", 404)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls++
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("PRIVATE-TOKEN") != f.valid {
			// This is the observed failure in the field: "Token was revoked".
			f.rejected++
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "401 Unauthorized - Token was revoked"})
			return
		}
		f.issued++
		// Low-entropy, obvious-placeholder shape, matching gitlab_test.go's fixtures.
		f.valid = fmt.Sprintf("glpat-example00000000000000000000-r%d", f.issued)
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token": f.valid, "expires_at": "2124-01-08T00:00:00Z", "active": true,
		})
	}))
	t.Cleanup(srv.Close)
	return srv, f
}

// TestGitLabConcurrentMintsForOneRoleDoNotRevokeTheLivePAT is the regression this file exists
// for. Two mints for ONE role are issued at the same moment — the shape a single window
// produces when it fans out several tool calls, each of which mints.
//
// UNSERIALISED, exactly one of them wins: the loser presents a token the winner has already
// invalidated, gets 401, and returns unverifiable. The custody file is then holding whatever
// the loser's failure left behind, with no live successor, and self-rotation cannot recover
// because reaching the endpoint at all needs a live token.
//
// SERIALISED, both succeed, each rotating from the value its predecessor persisted, and the
// file ends holding the credential the server still considers live. The final assertion is
// the one that matters operationally: the token on disk WORKS.
func TestGitLabConcurrentMintsForOneRoleDoNotRevokeTheLivePAT(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newConcurrentRotateServer(t, glOldWorker)
	pointHTTPClientAt(t, srv)

	const mints = 2
	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		errs  = make([]error, mints)
	)
	for i := 0; i < mints; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all mints together, so they genuinely overlap
			errs[i] = cmdGitLabRotate("worker", &auditCtx{argsDigest: "test"}, true)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("mint %d failed: %v", i, err)
		}
	}
	valid, calls, rejected, issued := fx.snapshot()
	if rejected != 0 {
		t.Errorf("%d rotation(s) presented an already-invalidated token — the mints were not serialised; "+
			"this is the race that revokes the role's live PAT", rejected)
	}
	if calls != mints || issued != mints {
		t.Errorf("rotate endpoint: calls=%d issued=%d, want %d of each", calls, issued, mints)
	}

	// The operational assertion: the credential left on disk is the one the server still
	// accepts. A file holding a revoked value is the lockout this fix exists to prevent.
	got, rerr := os.ReadFile(tokPath)
	if rerr != nil {
		t.Fatalf("read custody file: %v", rerr)
	}
	if string(got) != valid {
		t.Fatalf("custody file holds a token the server does not consider live — this is the lockout: "+
			"file and server-live value differ (file len=%d, live len=%d)", len(got), len(valid))
	}
	if fi, serr := os.Stat(tokPath); serr != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("custody file must remain 0600 after concurrent rotations (stat err %v)", serr)
	}
}

// TestGitLabRotateRefusesWhenTheRoleLockIsHeld pins the fail-closed half: with the per-role
// lock held by a peer, a mint must REFUSE — could-not-check, exit 6 — and must not reach the
// rotate endpoint at all. Rotating anyway is the failure mode; a refusal costs a retry, a
// lost race costs a group owner's intervention.
func TestGitLabRotateRefusesWhenTheRoleLockIsHeld(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newConcurrentRotateServer(t, glOldWorker)
	pointHTTPClientAt(t, srv)

	// A peer holds the role's rotation lock for the whole test.
	holder, oerr := os.OpenFile(gitlabRotateLockPath(tokPath), os.O_CREATE|os.O_RDWR, 0o600)
	if oerr != nil {
		t.Fatalf("open lock file: %v", oerr)
	}
	if lerr := deskkit.TryLockExclusive(holder); lerr != nil {
		t.Fatalf("test could not take the peer lock: %v", lerr)
	}
	t.Cleanup(func() { _ = deskkit.UnlockFile(holder); _ = holder.Close() })

	// Shorten the wait so the REFUSAL branch is reached in milliseconds rather than a minute.
	oldWait := gitlabRotateLockWait
	gitlabRotateLockWait = 100 * time.Millisecond
	t.Cleanup(func() { gitlabRotateLockWait = oldWait })

	err := cmdGitLabRotate("worker", &auditCtx{argsDigest: "test"}, true)
	if err == nil {
		t.Fatal("a mint proceeded while a peer held the role's rotation lock — it must refuse, " +
			"never rotate unserialised")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("lock-contention refusal exit = %d, want %d (could-not-check)", got, deskkit.ExitUnverifiable)
	}
	if _, calls, _, _ := fx.snapshot(); calls != 0 {
		t.Fatalf("the rotate endpoint was contacted %d time(s) despite the held lock — the refusal must "+
			"come BEFORE a second rotation is in flight", calls)
	}
	// The refusal has to be actionable by the operator who is reading it, which is why it
	// names the owner re-issue path: a role whose PAT is already dead cannot self-rotate out.
	for _, want := range []string{"re-issue", "SEQUENTIALLY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("lock-contention refusal does not mention %q — it must name the recovery path:\n%s",
				want, err.Error())
		}
	}
	// The custody file must be untouched: nothing was rotated, so nothing was written.
	got, _ := os.ReadFile(tokPath)
	if string(got) != glOldWorker {
		t.Fatal("the custody file changed on a refused mint — a refusal must leave the credential alone")
	}
}

// TestGitLabRotateRefusesBeforeRotatingWhenCustodyDirIsUnwritable pins a consequence of
// serialising that is worth having on its own terms.
//
// A custody directory this process cannot write to is fatal to the rotation either way: the
// new token can never be persisted there. What CHANGES is when that is discovered. Because
// the lock lives in that directory, the refusal now lands BEFORE the rotate endpoint is
// called — so the role's existing token is still live and the operator retries after fixing
// permissions. Previously the rotation went first: the live token was invalidated, the write
// then failed, and the role was locked out with no live successor, recoverable only by a
// group owner re-issuing the PAT.
//
// Discovering an impossible write before spending the credential, rather than after, is the
// whole shape of the fix in miniature.
func TestGitLabRotateRefusesBeforeRotatingWhenCustodyDirIsUnwritable(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newConcurrentRotateServer(t, glOldWorker)
	pointHTTPClientAt(t, srv)

	dir := filepath.Dir(tokPath)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := cmdGitLabRotate("worker", &auditCtx{argsDigest: "test"}, true)
	if err == nil {
		t.Fatal("rotation reported success with an unwritable custody directory")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("refusal exit = %d, want %d (could-not-check)", got, deskkit.ExitUnverifiable)
	}

	// The credential must be untouched: nothing was rotated, so the role is still usable.
	valid, calls, _, _ := fx.snapshot()
	if calls != 0 {
		t.Fatalf("the rotate endpoint was contacted %d time(s) — the role's live token was spent on a "+
			"rotation that could never have been persisted", calls)
	}
	if valid != glOldWorker {
		t.Fatal("the server's live token changed — a rotation happened despite the unwritable directory")
	}
	got, _ := os.ReadFile(tokPath)
	if string(got) != glOldWorker {
		t.Fatal("the custody file changed on a refused rotation")
	}
}

// TestGitLabNoRotateReadsCustodyWithoutRotating pins the second layer: a read-only or dry-run
// verb asks for the credential PATH and gets it, with NO rotation, NO network contact, and —
// because nothing is transmitted — no requirement that GITLAB_API_BASE be configured at all.
//
// GITLAB_API_BASE is deliberately left UNSET here. If the read-only path ever grows a request,
// this test fails on the base-required refusal rather than silently transmitting a credential.
func TestGitLabNoRotateReadsCustodyWithoutRotating(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newConcurrentRotateServer(t, glOldWorker)
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	t.Cleanup(func() { httpClient = oldClient })
	t.Setenv("GITLAB_API_BASE", "")

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "--no-rotate", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--no-rotate lookup rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stdout, tokPath) {
		t.Fatalf("--no-rotate must print the custody PATH %q; got: %s", tokPath, stdout)
	}
	if _, calls, _, _ := fx.snapshot(); calls != 0 {
		t.Fatalf("--no-rotate contacted the rotate endpoint %d time(s) — it must rotate nothing", calls)
	}
	got, _ := os.ReadFile(tokPath)
	if string(got) != glOldWorker {
		t.Fatal("--no-rotate changed the custody file — a read-only lookup must leave the credential alone")
	}
	// Neither token value may reach any stream, on this path as on the rotating one.
	assertNoTokenLeak(t, stdout+stderr)

	// The audit line must say what happened. Logging a rotation that never occurred would put
	// phantom rotations in the trail an incident reconstruction reads.
	entries := auditEntries(t)
	if len(entries) == 0 {
		t.Fatal("expected an audit entry")
	}
	last := entries[len(entries)-1]
	if last.Verb == "rotate" {
		t.Errorf("audit verb = %q for a --no-rotate lookup — the trail would record a rotation that "+
			"never happened", last.Verb)
	}
	if !strings.Contains(last.Detail, "no rotation performed") {
		t.Errorf("audit detail = %q, want it to record that no rotation was performed", last.Detail)
	}
}

// TestGitLabNoRotateStillRefusesBadCustody — the read-only path makes the SAME custody checks
// as the rotating one. A read verb that accepted custody the write verb refuses would report a
// usable credential for a role that has none, and the mismatch would only surface at the
// moment it mattered.
func TestGitLabNoRotateStillRefusesBadCustody(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")

	// Empty custody: present, correctly permissioned, and carrying no credential.
	writeTokenCache(t, tokPath, "")

	rc, _, stderr := runCap(t, []string{"--forge", "gitlab", "--no-rotate", "worker"})
	if rc == deskkit.ExitOK {
		t.Fatalf("--no-rotate accepted an EMPTY custody file — it must refuse like the rotating path does")
	}
	if !strings.Contains(stderr, "empty") {
		t.Errorf("refusal should name the empty custody file; got: %s", stderr)
	}
}
