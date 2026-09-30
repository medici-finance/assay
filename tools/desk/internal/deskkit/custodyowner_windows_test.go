//go:build windows

package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// TestWindowsCustodyACLIntegration drives the real syscall adapter
// (VerifyCustodyOwnerOnly → windowsFileACLModel → evaluateCustodyACL) end to end on
// Windows: a token file with an owner-only DACL is ACCEPTED — the #667 case the old
// POSIX 0600 test rejected as synthetic 0666 — and a world-writable or a
// world-READABLE grant makes it refuse, naming the reason. It cannot run in CI
// (there is no Windows runner); it gives a Windows developer coverage of the adapter that the
// platform-independent TestEvaluateCustodyACL cannot, mirroring
// TestWindowsRosterACLIntegration for the roster path.
func TestWindowsCustodyACLIntegration(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "gitlab-desk.token")
	if err := os.WriteFile(tokenPath, []byte("glpat-stub-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A fresh t.TempDir() file inherits profile ACEs the owner-only check would
	// flag; establish the owner-only state a correctly-provisioned PAT has.
	if err := secureTestRosterPaths(tokenPath); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyCustodyOwnerOnly(tokenPath, fi); err != nil {
		t.Fatalf("an owner-only token ACL was refused: %v", err)
	}

	// Each foreign grant the unix 0600 rule excludes is refused: world write, and
	// world READ alone (a credential file must be readable only by its owner).
	for _, tc := range []struct {
		access windows.ACCESS_MASK
		want   string
	}{
		{windows.GENERIC_WRITE, "write-capable Windows access"},
		{windows.GENERIC_READ, "read-capable Windows access"},
	} {
		if err := setWorldGrantDACL(tokenPath, tc.access); err != nil {
			t.Fatal(err)
		}
		if err := VerifyCustodyOwnerOnly(tokenPath, fi); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("a token ACL granting Everyone %#x was not refused as %q: %v", tc.access, tc.want, err)
		}
	}
}

// setWorldGrantDACL replaces path's DACL with a protected one holding a single
// Everyone grant of access.
func setWorldGrantDACL(path string, access windows.ACCESS_MASK) error {
	world, err := windows.StringToSid("S-1-1-0")
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: access,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(world),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil)
}
