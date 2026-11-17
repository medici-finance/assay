//go:build unix

package deskkit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #6768
// TestHotfixOwnerUserNS: the hotfix owner check trusts a root-owned file only in the initial
// user namespace. Under any other uid_map or gid_map the caller can read as uid 0, so the
// check refuses as could-not-check, before any owner is read, instead of passing.
func TestHotfixOwnerUserNS(t *testing.T) {
	sh, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Skipf("no /bin/sh to probe: %v", err)
	}
	id := "0 0 4294967295\n"
	if err := hotfixOwnedByNS(0, fixIDMaps(id, id), "linux")(sh); err != nil {
		t.Skipf("control: /bin/sh does not pass the uid-0 check in the initial namespace here: %v", err)
	}
	for name, read := range map[string]func(string) ([]byte, error){
		"caller mapped to uid 0":    fixIDMaps("0 1000 1\n", id),
		"caller and a subuid range": fixIDMaps("0 1000 1\n1 100000 65536\n", id),
		"identity short of one id":  fixIDMaps("0 0 4294967294\n", id),
		"gid_map remapped":          fixIDMaps(id, "0 1000 1\n"),
	} {
		err := hotfixOwnedByNS(0, read, "linux")(sh)
		if err == nil || !strings.Contains(err.Error(), "could-not-check") || strings.Contains(err.Error(), "owned by") {
			t.Errorf("%s: owner check on root-owned %s = %v, want could-not-check before any owner verdict", name, sh, err)
		}
	}
	// The production constructor in this test's own namespace keeps the existing verdict on a
	// file this test owns: refused as not root's.
	f := filepath.Join(t.TempDir(), "deskpr")
	if err := os.WriteFile(f, []byte("x\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() != 0 {
		if err := HotfixRootOwned()(f); err == nil || !strings.Contains(err.Error(), "not 0") {
			t.Fatalf("HotfixRootOwned on a file this test owns = %v, want owned-by refusal", err)
		}
	}
}

// regression: #6768
// TestHotfixOwnerReasonDistinct: when the namespace gate refuses, layer 2 and ValidateHotfix
// name could-not-check (HotfixReasonOwnerUnverified), never "not root-owned", which only a
// file owner actually read may earn.
func TestHotfixOwnerReasonDistinct(t *testing.T) {
	id := "0 0 4294967295\n"
	overlay := t.TempDir()
	gated := hotfixOwnedByNS(0, fixIDMaps("0 1000 1\n", id), "linux")
	in := hotfixSelfInput{Overlay: overlay, Exe: filepath.Join(overlay, "bin", "deskpr"), Owner: gated}
	if r := selfIntegrity(in, "deskpr"); !strings.HasPrefix(r, HotfixReasonOwnerUnverified+": ") {
		t.Errorf("layer 2, gate refusal: reason %q, want %q", r, HotfixReasonOwnerUnverified)
	}
	_, err := ValidateHotfix(overlay, "deskpr", "", time.Now(), gated)
	var he *HotfixError
	if !errors.As(err, &he) || he.Reason != HotfixReasonOwnerUnverified {
		t.Errorf("ValidateHotfix, gate refusal: %v, want reason %q", err, HotfixReasonOwnerUnverified)
	}
	if os.Getuid() == 0 {
		return
	}
	in.Owner = hotfixOwnedByNS(0, fixIDMaps(id, id), "linux")
	if r := selfIntegrity(in, "deskpr"); !strings.HasPrefix(r, HotfixReasonNotRootOwned+": ") {
		t.Errorf("control, a caller-owned overlay: reason %q, want %q", r, HotfixReasonNotRootOwned)
	}
}
