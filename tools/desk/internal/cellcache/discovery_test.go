//go:build darwin || linux

package cellcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	unmarked := filepath.Join(t.TempDir(), "go-cache")
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
