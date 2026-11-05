//go:build darwin || linux

package cellcache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// recordedPolicy rewrites root's report.json with doctored — the tampering a
// sweep-side reader must refuse to act on. The report format is this
// package's own, so the fixture writes it through the same type.
func recordedPolicy(t *testing.T, p Policy, doctor func(*Report)) {
	t.Helper()
	b, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	var rep Report
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	doctor(&rep)
	out, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ReportPath(), out, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMarkedPolicy(t *testing.T) {
	// An unmarked directory is not a candidate: no marker, no policy, no error.
	// (The candidate path is canonical, exactly as the sweep's discovery
	// canonicalizes before probing — validatePath refuses symlink components.)
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	unmarked := filepath.Join(tmp, "go-cache")
	if p, err := LoadMarkedPolicy(unmarked); p != nil || err != nil {
		t.Fatalf("unmarked root: %v %v", p, err)
	}

	p := fixture(t)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	q, err := LoadMarkedPolicy(p.Root)
	if err != nil || q == nil || *q != p {
		t.Fatalf("marked policy did not round-trip: %v %v", q, err)
	}

	// A marked root with no readable report is could-not-check, never an
	// invented default policy.
	if err := os.Remove(p.ReportPath()); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadMarkedPolicy(p.Root); err == nil {
		t.Fatal("marked root without a report resolved a policy")
	}

	// A report whose policy names a DIFFERENT root must not redirect the
	// sweep there.
	if _, err = Check(p, false); err != nil {
		t.Fatal(err)
	}
	other := fixture(t)
	recordedPolicy(t, p, func(rep *Report) { rep.Policy.Root = other.Root })
	if _, err = LoadMarkedPolicy(p.Root); err == nil {
		t.Fatal("a policy carried over from another root was accepted")
	}

	// A report whose domain does not match the ownership marker is refused:
	// the marker is the root's identity.
	recordedPolicy(t, p, func(rep *Report) { rep.Policy.Root = p.Root; rep.Policy.Domain = other.Domain })
	if _, err = LoadMarkedPolicy(p.Root); err == nil {
		t.Fatal("a policy whose domain contradicts the marker was accepted")
	}

	// A marker in a foreign schema is not this package's root to touch.
	if err := os.WriteFile(filepath.Join(p.Root, "owner"), []byte("other-schema\n"+p.Domain+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadMarkedPolicy(p.Root); err == nil {
		t.Fatal("a foreign ownership marker was accepted")
	}
}

// The sweep matches a reaped directory to a cache root by trust domain, so the
// derivation exported for that match must be the one Resolve binds roots with.
func TestDomainForMatchesResolve(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := Resolve(dir, func(k string) string {
		if k == "CELL_GO_CACHE" {
			return "on"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Domain != DomainFor(dir) {
		t.Fatal("DomainFor disagrees with Resolve")
	}
	if DomainFor(dir) == DomainFor(filepath.Dir(dir)) {
		t.Fatal("domains not distinct per cell")
	}
}

func TestReap(t *testing.T) {
	p := fixture(t)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/result", 100)
	material(t, p, "mod/module/file", 100)
	// Launch uses fresh complete accounting after the timestamp safety margin;
	// only explicit maintenance refreshes payload measurements.
	time.Sleep(AdmitMtimeSlack)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}

	// A root with an active consumer record is refused, and left byte-identical.
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Reap(p); err == nil {
		t.Fatal("reaped a root with an active consumer")
	}
	if _, err = os.Stat(filepath.Join(p.Root, "build", "result")); err != nil {
		t.Fatal("refused reap still touched the root", err)
	}
	if err = l.Finish(true); err != nil {
		t.Fatal(err)
	}

	// Read-only module directories do not stop the reap.
	if err := os.Chmod(filepath.Join(p.Root, "mod", "module"), 0500); err != nil {
		t.Fatal(err)
	}
	removed, err := Reap(p)
	if err != nil || !removed {
		t.Fatalf("reap: %v %v", removed, err)
	}
	if _, err = os.Stat(p.Root); !os.IsNotExist(err) {
		t.Fatal("root survived its reap", err)
	}

	// A root already gone — the common case is one that lived inside the
	// removed cell directory — is a clean no-op, not an error.
	removed, err = Reap(p)
	if err != nil || removed {
		t.Fatalf("reap of a vanished root: %v %v", removed, err)
	}
}

// A consumer racing the reap re-enrolls under the root's lock; the reap must
// never proceed once a lease exists, however the interleave lands.
func TestReapHonoursLateLease(t *testing.T) {
	p := fixture(t)
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		l, err := Acquire(p.Env())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Reap(p); err == nil {
			t.Fatal("reap proceeded under a live lease")
		}
		if _, err = os.Stat(filepath.Join(p.Root, "owner")); err != nil {
			t.Fatal("refused reap removed the marker", err)
		}
		if err = l.Finish(true); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := Reap(p)
	if err != nil || !removed {
		t.Fatalf("final reap: %v %v", removed, err)
	}
	if _, err = os.Stat(p.Root); !os.IsNotExist(err) {
		t.Fatal("root survived its reap", err)
	}
}

// The boundary cases: a candidate in a shared or sticky cells root can belong
// to another principal, so every metadata read must verify before it reads and
// must return promptly. A FIFO at report.json, a linked report, an oversized
// report, a writable root, and an other-writable report are all refused —
// none of them may stall or redirect the read.
func TestLoadMarkedPolicyBoundary(t *testing.T) {
	marked := func(t *testing.T) Policy {
		p := fixture(t)
		if _, err := Check(p, false); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("fifo report returns promptly", func(t *testing.T) {
		p := marked(t)
		if err := os.Remove(p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(p.ReportPath(), 0600); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			q, err := LoadMarkedPolicy(p.Root)
			if q != nil {
				err = fmt.Errorf("a FIFO report resolved a policy")
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("a FIFO report resolved a policy")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the read stalled on a FIFO")
		}
	})

	t.Run("fifo swapped in between check and open", func(t *testing.T) {
		p := marked(t)
		old := betweenCheckAndOpen
		betweenCheckAndOpen = func(name string) {
			if name != "report.json" {
				return
			}
			// The regular report the checks just passed becomes a FIFO in the
			// check/open window — the exact swap O_NONBLOCK exists for.
			if err := os.Remove(p.ReportPath()); err != nil {
				t.Error(err)
			}
			if err := syscall.Mkfifo(p.ReportPath(), 0600); err != nil {
				t.Error(err)
			}
		}
		t.Cleanup(func() { betweenCheckAndOpen = old })
		done := make(chan error, 1)
		go func() {
			q, err := LoadMarkedPolicy(p.Root)
			if q != nil {
				err = fmt.Errorf("a swapped-in FIFO resolved a policy")
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("a swapped-in FIFO resolved a policy")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a FIFO swapped in after the checks stalled the open")
		}
	})

	t.Run("linked report refused", func(t *testing.T) {
		p := marked(t)
		if err := os.Remove(p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/dev/null", p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if q, err := LoadMarkedPolicy(p.Root); q != nil || err == nil {
			t.Fatal("a linked report resolved a policy")
		}
	})

	// The link's target being a VALID in-root report is exactly what the
	// descriptor checks (device, regular file) alone cannot refuse: same
	// device, same owner, regular, well-formed. Only the no-follow open
	// refuses it — that arm of the metadata discipline is what this kills.
	t.Run("report swapped for an in-root link refused", func(t *testing.T) {
		p := fixture(t)
		if _, err := Check(p, false); err != nil {
			t.Fatal(err)
		}
		if _, err := Check(p, false); err != nil {
			t.Fatal(err)
		}
		// Two live passes leave previous.json beside report.json.
		if _, err := os.Stat(filepath.Join(p.Root, "previous.json")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("previous.json", p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if q, err := LoadMarkedPolicy(p.Root); q != nil || err == nil {
			t.Fatal("a report swapped for an in-root link resolved a policy")
		}
	})

	t.Run("oversized report refused", func(t *testing.T) {
		p := marked(t)
		if err := os.WriteFile(p.ReportPath(), bytes.Repeat([]byte("x"), 128<<10), 0600); err != nil {
			t.Fatal(err)
		}
		if q, err := LoadMarkedPolicy(p.Root); q != nil || err == nil {
			t.Fatal("an oversized report resolved a policy")
		}
	})

	t.Run("writable root refused before any read", func(t *testing.T) {
		p := marked(t)
		// The candidate is BOTH outside the path discipline AND holding a
		// FIFO at report.json: which refusal fires proves the boundary runs
		// before any content read — a read-first order would report the FIFO.
		if err := os.Chmod(p.Root, 0777); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(p.ReportPath()); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(p.ReportPath(), 0600); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			q, err := LoadMarkedPolicy(p.Root)
			if q != nil {
				err = fmt.Errorf("a root writable by another principal resolved a policy")
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("a root writable by another principal resolved a policy")
			}
			if !strings.Contains(err.Error(), "writable cache ancestor") && !strings.Contains(err.Error(), "mode 0700") {
				t.Fatalf("the path boundary did not fire before the content read: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the read stalled on a FIFO the boundary should have pre-empted")
		}
	})

	t.Run("other-writable report refused", func(t *testing.T) {
		p := marked(t)
		if err := os.Chmod(p.ReportPath(), 0646); err != nil {
			t.Fatal(err)
		}
		if q, err := LoadMarkedPolicy(p.Root); q != nil || err == nil {
			t.Fatal("an other-writable report resolved a policy")
		}
	})
}

// The convergence regression: a sweep enforcing a policy it read BEFORE the
// root's lock would write that stale policy back over one a launch recorded in
// between. CheckRecorded re-reads under the lock, so the launch's policy is
// the one enforced AND the one the report keeps. The seam fires between the
// lock acquisition and the re-read — the racing launch's exact window.
func TestCheckRecordedInterleave(t *testing.T) {
	p := fixture(t) // 8 GiB budget
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	// Over the budget the launch is ABOUT to record, under the one the
	// advisory read still sees.
	material(t, p, "build/result", 100)

	old := betweenLockAndReread
	betweenLockAndReread = func() {
		// The racing launch: a newer, smaller budget lands between the
		// sweep's advisory read and its under-lock re-read. The write goes
		// through the root lock's own discipline — flock is advisory, so a
		// direct write is exactly what a racing record() does.
		q := p
		q.Budget = 32
		rep := Report{Schema: schema, Policy: q}
		b, err := json.Marshal(rep)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p.ReportPath(), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { betweenLockAndReread = old })

	rep, err := CheckRecorded(p.Root, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Policy.Budget != 32 || rep.Reclaimed != 100 {
		t.Fatalf("the stale advisory budget was enforced: %+v", rep)
	}
	b, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	var saved Report
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Policy.Budget != 32 {
		t.Fatalf("the stale budget was written back over the launch's: %+v", saved.Policy)
	}
	if _, err = os.Stat(filepath.Join(p.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("pressure under the recorded policy was not reclaimed", err)
	}
}

// CheckRecorded never initializes: a missing or unmarked root is
// could-not-check, and no root appears.
func TestCheckRecordedNeverInitializes(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "go-cache")
	r, err := CheckRecorded(root, false)
	if err == nil || r.Outcome != "could-not-check" {
		t.Fatalf("unmarked root: %+v %v", r, err)
	}
	if _, serr := os.Stat(root); !os.IsNotExist(serr) {
		t.Fatal("a sweep initialized a root", serr)
	}
}
