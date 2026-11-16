package main

// prepwindow_test.go — preparation runs without the audit lock (#5023), so the write window
// must not trust what preparation read: each input the write relies on is re-read there and a
// change refuses (nothing written). The preparation lock is keyed on the root the landing
// actually uses, and a busy one is contention, not a could-not-check (#6227).

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const pwPath = "docs/streams/x/brief.md"

// pwLanding is one ordinary Evidence landing on a fresh root; it returns the exit code and
// the forge's write count.
func pwLanding(t *testing.T, f *fakeForge, args ...string) (int, int) {
	t.Helper()
	code := run(append([]string{"example-org/tracker", "main", "--evidence-file", pwPath}, args...))
	f.mu.Lock()
	defer f.mu.Unlock()
	return code, f.putCalls
}

func pwRoot(t *testing.T, f *fakeForge) string {
	t.Helper()
	f.setFile(pwPath, "old")
	return rootWithFile(t, pwPath, "# Brief\n\n## Evidence\n| 1 | x | y |\n")
}

// afterFirst returns true from the second call on: the first call is preparation's, the
// second the write window's re-check.
func afterFirst() func() bool {
	var n atomic.Int32
	return func() bool { return n.Add(1) > 1 }
}

func wantRefusedNoWrite(t *testing.T, code, puts int, errBuf interface{ String() string }, what string) {
	t.Helper()
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused): %s changed between preparation and the write was not caught (stderr %q)",
			code, deskkit.ExitRefused, what, errBuf.String())
	}
	if puts != 0 {
		t.Fatalf("putCalls = %d, want 0: the landing wrote after %s changed", puts, what)
	}
	if !strings.Contains(errBuf.String(), "changed between preparation and the write window") {
		t.Fatalf("stderr %q does not name the changed input", errBuf.String())
	}
}

// Invariant 1: what admitted the landing is re-checked in the write window.
func TestAdmissionChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	later := afterFirst()
	verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
		r := stubAdmittedReceipt(target)
		if later() {
			r.Binding.Model = "another-model"
		}
		return r, nil
	}
	code, puts := pwLanding(t, f, "--root", root)
	wantRefusedNoWrite(t, code, puts, errBuf, "the admitted binding")
}

// Invariant 1: an attestation that stops admitting (a label removed, a trust event, a home
// that moved) refuses at the write window.
func TestAdmissionRevokedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	later := afterFirst()
	verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
		if later() {
			return deskkit.VerifierReceipt{}, deskkit.Refused("attestation no longer trusted")
		}
		return stubAdmittedReceipt(target), nil
	}
	code, puts := pwLanding(t, f, "--root", root)
	wantRefusedNoWrite(t, code, puts, errBuf, "admission")
}

// Invariant 4: the local input is pinned by digest at preparation.
func TestLocalInputChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	stubPrep(t, func(string) {
		if err := os.WriteFile(filepath.Join(root, pwPath), []byte("# Brief\n\n## Evidence\n| 1 | x | CHANGED |\n"), 0o644); err != nil {
			t.Error(err)
		}
	})
	code, puts := pwLanding(t, f, "--root", root)
	wantRefusedNoWrite(t, code, puts, errBuf, "the local evidence file")
}

// Invariant 4: visibility is re-read, not taken from the publication snapshot.
func TestVisibilityChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	later := afterFirst()
	publicRepoGateFn = func(deskkit.RepoInfoFetcher, string, string) error {
		if later() {
			return deskkit.Refused("repository is now public")
		}
		return nil
	}
	code, puts := pwLanding(t, f, "--root", root)
	wantRefusedNoWrite(t, code, puts, errBuf, "visibility")
}

// Invariant 4: the board the gates judged is the board at the write.
func TestBoardChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	later := afterFirst()
	decisionGateBranchBoardFn = func(deskkit.Forge, deskkit.ForgeRepo, string, string, string) ([]boardEdit, error) {
		if later() {
			return []boardEdit{{Path: "docs/streams/x/README.md", Content: []byte("moved")}}, nil
		}
		return nil, nil
	}
	code, puts := pwLanding(t, f, "--root", root)
	wantRefusedNoWrite(t, code, puts, errBuf, "the board")
}

// Invariant 4: the decision-gate verdict (who wrote the ruling) is re-judged in the window.
func TestDecisionVerdictChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	later := afterFirst()
	decisionGateDiffFn = func(string, string, []byte, bool, []byte, []boardEdit, childOptions) ([]string, error) {
		if later() {
			return []string{"x/01 ruling comment no longer by a human"}, nil
		}
		return nil, nil
	}
	code, puts := pwLanding(t, f, "--root", root)
	wantRefusedNoWrite(t, code, puts, errBuf, "the decision-gate verdict")
}

func pwOutcome(t *testing.T) []string {
	t.Helper()
	line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
	return []string{"example-org/tracker", "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n")}
}

func pwRun(f *fakeForge, args []string) (int, int) {
	code := run(args)
	f.mu.Lock()
	defer f.mu.Unlock()
	return code, f.putCalls
}

// Invariant 1, outcome path.
func TestOutcomeAdmissionChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	later := afterFirst()
	verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
		if later() {
			return deskkit.VerifierReceipt{}, deskkit.Refused("attestation no longer trusted")
		}
		return stubAdmittedReceipt(target), nil
	}
	code, puts := pwRun(f, pwOutcome(t))
	wantRefusedNoWrite(t, code, puts, errBuf, "outcome admission")
}

// Invariant 4, outcome path: the record file is pinned by its bytes.
func TestOutcomeRecordChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	args := pwOutcome(t)
	recFile := args[len(args)-1]
	stubPrep(t, func(string) {
		if err := os.WriteFile(recFile, []byte("{}\n"), 0o644); err != nil {
			t.Error(err)
		}
	})
	code, puts := pwRun(f, args)
	wantRefusedNoWrite(t, code, puts, errBuf, "the outcome record file")
}

// Invariant 4, outcome path: visibility re-read.
func TestOutcomeVisibilityChangedRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	later := afterFirst()
	publicRepoGateFn = func(deskkit.RepoInfoFetcher, string, string) error {
		if later() {
			return deskkit.Refused("repository is now public")
		}
		return nil
	}
	code, puts := pwRun(f, pwOutcome(t))
	wantRefusedNoWrite(t, code, puts, errBuf, "visibility")
}

// holdPrepLock holds the preparation lock for root the way another landing would.
func holdPrepLock(t *testing.T, root string) {
	t.Helper()
	key, _, err := prepLockKey(root)
	if err != nil {
		t.Fatal(err)
	}
	holdLockFile(t, filepath.Join(os.Getenv("HOME"), ".config", "assay"), prepLockName(key))
}

// busyRun runs a landing with lockWait shortened and returns its exit code and write count.
func busyRun(t *testing.T, f *fakeForge, args ...string) (int, int) {
	t.Helper()
	old := lockWait
	lockWait = 200 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
	return pwLanding(t, f, args...)
}

// SR-2: a busy preparation lock is contention (#6227) — neither charged to the budget nor
// counted by the circuit breaker — never a could-not-check.
func TestPrepBusyIsContended(t *testing.T) {
	f, _ := setupFake(t)
	root := pwRoot(t, f)
	holdPrepLock(t, root)
	code, puts := busyRun(t, f, "--root", root)
	if code != deskkit.ExitUnverifiable || puts != 0 {
		t.Fatalf("exit = %d, puts = %d; want exit %d and no write", code, puts, deskkit.ExitUnverifiable)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultContended {
		t.Fatalf("audit result = %q, want %q: a busy preparation lock was charged as a failure", got, deskkit.ResultContended)
	}
}

// SR-1: the single-dash spelling the flag package accepts locks the same root.
func TestDashRootTakesPrepLock(t *testing.T) {
	f, _ := setupFake(t)
	root := pwRoot(t, f)
	holdPrepLock(t, root)
	code, puts := busyRun(t, f, "-root", root)
	if code != deskkit.ExitUnverifiable || puts != 0 {
		t.Fatalf("exit = %d, puts = %d: a landing on -root %s prepared while that root's preparation lock was held", code, puts, root)
	}
}

// SR-1: a re-cased spelling of the root on a case-insensitive filesystem is the same root.
func TestRecasedRootTakesPrepLock(t *testing.T) {
	f, _ := setupFake(t)
	f.setFile(pwPath, "old")
	base := t.TempDir()
	root := filepath.Join(base, "landing")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(pwPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, pwPath), []byte("# Brief\n\n## Evidence\n| 1 | x | y |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recased := filepath.Join(base, "LANDING")
	if _, err := os.Stat(recased); err != nil {
		t.Skip("case-sensitive filesystem: a re-cased path is another directory")
	}
	holdPrepLock(t, root)
	code, puts := busyRun(t, f, "--root", recased)
	if code != deskkit.ExitUnverifiable || puts != 0 {
		t.Fatalf("exit = %d, puts = %d: a landing on the re-cased root prepared while the root's preparation lock was held", code, puts)
	}
}

// SR-1: a repeated --root (any spelling) is refused before anything is read.
func TestRepeatedRootRefused(t *testing.T) {
	f, errBuf := setupFake(t)
	root := pwRoot(t, f)
	other := t.TempDir()
	for _, args := range [][]string{
		{"--root", other, "--root", root},
		{"-root", other, "--root=" + root},
	} {
		code, puts := pwLanding(t, f, args...)
		if code != deskkit.ExitRefused || puts != 0 {
			t.Fatalf("%v: exit = %d, puts = %d; want refused, no write (stderr %q)", args, code, puts, errBuf.String())
		}
	}
}

// Capability preflight reads the local tree at the landing root, so it runs under that root's
// preparation lock: with the lock held, a tree the preflight would refuse waits (contention)
// instead of being read while another landing stages into it.
func TestPreflightUnderPrepLock(t *testing.T) {
	f := setupStamp(t, true)
	root := rootWithFile(t, stampBrief, "---\nschema: brief-v1\n## Evidence\n| 1 | unresolved |\n")
	holdPrepLock(t, root)
	old := lockWait
	lockWait = 200 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
	code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root})
	if got := lastAudit(t).Result; f.putCalls != 0 || got != deskkit.ResultContended {
		t.Fatalf("exit = %d, result = %q: the capability preflight read the root before its preparation lock", code, got)
	}
}
