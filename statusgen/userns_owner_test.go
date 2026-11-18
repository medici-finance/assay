//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sgFixIDMaps is a /proc reader serving the given uid_map and gid_map; every other path is
// absent.
func sgFixIDMaps(uidMap, gidMap string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		switch p {
		case "/proc/self/uid_map":
			return []byte(uidMap), nil
		case "/proc/self/gid_map":
			return []byte(gidMap), nil
		}
		return nil, os.ErrNotExist
	}
}

// sgFixRootOwnedFile is a file that, with every parent, passes the raw uid-0 owner check here:
// the shell. A host where it does not is skipped, since no test can create a root-owned file.
func sgFixRootOwnedFile(t *testing.T) string {
	t.Helper()
	sh, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Skipf("no /bin/sh to probe: %v", err)
	}
	if err := sgHotfixOwnedByIn(0, 0)(sh); err != nil {
		t.Skipf("/bin/sh does not pass the raw uid-0 check here: %v", err)
	}
	return sh
}

// regression: #6768
// TestStatusgenOwnerUserNS: a file that reads as root-owned is trusted only in the initial
// user namespace. Under any other uid_map or gid_map the caller can read as uid 0, so the
// verifier's ordinary owner check refuses as could-not-check instead of passing.
func TestStatusgenOwnerUserNS(t *testing.T) {
	sh := sgFixRootOwnedFile(t)
	ordinary := []string{"statusgen", "verifyrun"}
	identity := "         0          0 4294967295\n"
	if err := sgActiveOwnerFor(ordinary, sgFixIDMaps(identity, identity))(sh); err != nil {
		t.Fatalf("control: root-owned %s in the initial namespace refused: %v", sh, err)
	}
	for name, read := range map[string]func(string) ([]byte, error){
		"caller mapped to uid 0":      sgFixIDMaps("0 1000 1\n", identity),
		"caller and a subuid range":   sgFixIDMaps("0 1000 1\n1 100000 65536\n", identity),
		"identity short of one id":    sgFixIDMaps("0 0 4294967294\n", identity),
		"gid_map remapped":            sgFixIDMaps(identity, "0 1000 1\n"),
		"uid_map empty":               sgFixIDMaps("", identity),
		"identity extent plus a line": sgFixIDMaps(identity+"1 1 1\n", identity),
	} {
		err := sgActiveOwnerFor(ordinary, read)(sh)
		if err == nil || !strings.Contains(err.Error(), "could-not-check") {
			t.Errorf("%s: owner check on root-owned %s = %v, want a could-not-check refusal", name, sh, err)
		}
	}
}

// TestStatusgenOwnerGateFirst: the gate runs before any owner is read. A file this test owns,
// checked by its own uid, passes in the identity namespace and is could-not-check, with no
// owner verdict, outside it.
func TestStatusgenOwnerGateFirst(t *testing.T) {
	id := "0 0 4294967295\n"
	f := filepath.Join(t.TempDir(), "statusgen")
	sgFixWrite(t, f, "x\n", 0o755)
	me := uint32(os.Getuid())
	if err := sgHotfixOwnedByNS(me, sgFixIDMaps(id, id), "linux")(f); err != nil {
		t.Fatalf("control: own file under the identity mapping: %v", err)
	}
	err := sgHotfixOwnedByNS(me, sgFixIDMaps("0 1000 1\n", id), "linux")(f)
	if err == nil || !strings.Contains(err.Error(), "could-not-check") || strings.Contains(err.Error(), "owned by") {
		t.Fatalf("own file under a remapped namespace = %v, want could-not-check before any owner verdict", err)
	}
}

// regression: #6768
// TestStatusgenOwnerReasonDistinct: when the namespace gate refuses, the self-check names
// could-not-check (sgReasonOwnerUnverified), never "not root-owned", which only a file owner
// actually read may earn.
func TestStatusgenOwnerReasonDistinct(t *testing.T) {
	id := "0 0 4294967295\n"
	overlay := t.TempDir()
	in := sgHotfixInput{Overlay: overlay, Exe: filepath.Join(overlay, "bin", "statusgen")}
	in.Owner = sgHotfixOwnedByNS(0, sgFixIDMaps("0 1000 1\n", id), "linux")
	if r := sgSelfIntegrity(in); !strings.HasPrefix(r, sgReasonOwnerUnverified+": ") {
		t.Errorf("gate refusal: reason %q, want %q", r, sgReasonOwnerUnverified)
	}
	if os.Getuid() == 0 {
		return
	}
	in.Owner = sgHotfixOwnedByNS(0, sgFixIDMaps(id, id), "linux")
	if r := sgSelfIntegrity(in); !strings.HasPrefix(r, sgReasonNotRootOwned+": ") {
		t.Errorf("control, a caller-owned overlay: reason %q, want %q", r, sgReasonNotRootOwned)
	}
}
