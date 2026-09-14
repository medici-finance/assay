package main

// Tests for the prune singleton (issue #1037, ask F7).
//
// The load-bearing property is the ASYMMETRY: the lock fails CLOSED and the TTL debounce
// fails OPEN. Most of what is asserted here is the second half — four kinds of bad stamp,
// each of which must let the sweep PROCEED. A singleton that failed closed on a corrupt
// stamp would wedge the boot step of every desk window on the machine, which is exactly
// the stale-lock class the design exists to avoid.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stampPathFor returns the singleton stamp path for the repo the fixture's sweep resolves.
func stampPathFor(t *testing.T, work string) string {
	t.Helper()
	dir, err := pruneStampDir()
	if err != nil {
		t.Fatalf("pruneStampDir: %v", err)
	}
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		t.Fatalf("mkdir %s: %v", dir, mkErr)
	}
	return filepath.Join(dir, pruneRepoKey(resolvePath(mustAbsOrRaw(work)))+".stamp")
}

// --- the LOCK fails closed ----------------------------------------------------------

func TestPruneSingletonHeldByALiveSweepSkips(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "held-candidate")
	advanceMain(t, work, "first")

	// Hold the lock on our own file descriptor — flock is per OPEN FILE DESCRIPTION, so a
	// second independent open in this process contends exactly as another process would.
	path := stampPathFor(t, work)
	holder, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("open stamp: %v", err)
	}
	if lerr := deskkit.TryLockExclusive(holder); lerr != nil {
		t.Fatalf("take the holding lock: %v", lerr)
	}
	b, _ := json.Marshal(pruneStamp{
		Schema:    pruneStampSchema,
		PID:       4242,
		StartedAt: time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339),
	})
	if _, werr := holder.Write(b); werr != nil {
		t.Fatalf("write stamp: %v", werr)
	}
	t.Cleanup(func() { _ = deskkit.UnlockFile(holder); _ = holder.Close() })

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("a held sweep must exit 0 (it is a clean no-op), got rc = %d; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "held by pid 4242") {
		t.Fatalf("expected the holder to be named; stderr:\n%s", errout)
	}
	assertExists(t, target) // the held sweep removed nothing
}

// --- the TTL debounce -----------------------------------------------------------------

func TestPruneSingletonRecentSweepSkips(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "recent-candidate")
	advanceMain(t, work, "first")

	writeStamp(t, stampPathFor(t, work), pruneStamp{
		Schema:     pruneStampSchema,
		PID:        4242, // NOT this process: a same-process re-run is deliberate, never debounced
		StartedAt:  time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
		FinishedAt: time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
	})

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "singleton TTL") {
		t.Fatalf("expected the recency skip; stderr:\n%s", errout)
	}
	assertExists(t, target)
}

// --singleton-ttl 0 and --no-singleton disable the DEBOUNCE. Neither disables the lock —
// there is no --force in this verb and these are not one.
func TestPruneSingletonTTLZeroAndNoSingletonStillSweep(t *testing.T) {
	for _, args := range [][]string{
		{"prune", "--singleton-ttl", "0"},
		{"prune", "--no-singleton"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			target := addWorktree(t, "ttl-off-candidate")
			advanceMain(t, work, "first")

			writeStamp(t, stampPathFor(t, work), pruneStamp{
				Schema:     pruneStampSchema,
				PID:        4242,
				StartedAt:  time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
				FinishedAt: time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
			})

			rc, errout := runCapErr(t, args)
			if rc != 0 {
				t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
			}
			if strings.Contains(errout, "singleton TTL") {
				t.Fatalf("the debounce fired with it disabled; stderr:\n%s", errout)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("the sweep did not run (err=%v); stderr:\n%s", err, errout)
			}
		})
	}
}

// --- the TTL half fails OPEN ----------------------------------------------------------
//
// Four kinds of unusable stamp. Every one must let the sweep PROCEED. If any of these
// held a sweep, a single bad write would wedge prune on the machine until somebody found
// and deleted the file by hand — the stale-lock class, recreated.

func TestPruneSingletonBadStampAlwaysSweeps(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	cases := []struct {
		name, body string
	}{
		{"truncated", `{"schema":"deskwt-prune-singleton-v1","pid":42,"finish`},
		{"not JSON", "this is not a stamp at all\n"},
		{"unknown schema", `{"schema":"something-else","pid":42,"finishedAt":"` + time.Now().UTC().Format(time.RFC3339) + `"}`},
		{"future finishedAt", `{"schema":"deskwt-prune-singleton-v1","pid":42,"finishedAt":"` + future + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			target := addWorktree(t, "failopen-candidate")
			advanceMain(t, work, "first")

			path := stampPathFor(t, work)
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatalf("plant stamp: %v", err)
			}

			rc, errout := runCapErr(t, []string{"prune"})
			if rc != 0 {
				t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
			}
			if strings.Contains(errout, "skipping this sweep") {
				t.Fatalf("a %s stamp HELD the sweep — the TTL half must fail open; stderr:\n%s", tc.name, errout)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("the sweep did not run with a %s stamp (err=%v); stderr:\n%s", tc.name, err, errout)
			}
		})
	}
}

// A missing stamp is the commonest case of all — the first sweep this machine ever ran.
func TestPruneSingletonMissingStampSweeps(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "firstever-candidate")
	advanceMain(t, work, "first")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("the first-ever sweep did not run (err=%v); stderr:\n%s", err, errout)
	}
	// And it left a stamp recording that it finished, so the NEXT window is debounced.
	st, ok := readPruneStamp(stampPathFor(t, work))
	if !ok {
		t.Fatalf("a completed sweep wrote no stamp")
	}
	if st.FinishedAt == "" {
		t.Fatalf("the stamp records no finish time: %+v", st)
	}
}

// --- the supervisor releases the lock per tick ------------------------------------------
//
// A --interval loop that held the singleton for its lifetime would lock out every
// boot-time and manual sweep on the machine — an efficiency mechanism turned into an
// outage. After a tick completes, the lock must be free.

func TestPruneIntervalReleasesLockPerTick(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	addWorktree(t, "tick-candidate")
	advanceMain(t, work, "first")

	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	cwd := resolvePath(mustAbsOrRaw(work))
	stop := make(chan struct{})
	ac := &auditCtx{verb: "prune"}

	out := runCapOut(t, func() {
		go func() { time.Sleep(300 * time.Millisecond); close(stop) }()
		if rerr := runPruneLoop(guard, work, cwd, 50*time.Millisecond, pruneOpts{singletonTTL: 0}, stop, ac); rerr != nil {
			t.Errorf("runPruneLoop: %v", rerr)
		}
	})
	if !strings.Contains(out, "tick 1") {
		t.Fatalf("the loop did not tick; stdout:\n%s", out)
	}

	// The supervisor has exited; the lock must be takeable immediately.
	f, oerr := os.OpenFile(stampPathFor(t, work), os.O_RDWR|os.O_CREATE, 0o600)
	if oerr != nil {
		t.Fatalf("open stamp: %v", oerr)
	}
	defer f.Close()
	if lerr := deskkit.TryLockExclusive(f); lerr != nil {
		t.Fatalf("the interval supervisor did not release the singleton: %v", lerr)
	}
	_ = deskkit.UnlockFile(f)
}

func writeStamp(t *testing.T, path string, st pruneStamp) {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal stamp: %v", err)
	}
	if werr := os.WriteFile(path, b, 0o600); werr != nil {
		t.Fatalf("write stamp: %v", werr)
	}
}
