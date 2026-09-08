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
// POSIX 0600 test rejected as synthetic 0666 — and adding a world-writable grant
// makes it refuse, naming the reason. It cannot run in CI (there is no Windows
// runner); it gives a Windows developer coverage of the adapter that the
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

	world, err := windows.StringToSid("S-1-1-0")
	if err != nil {
		t.Fatal(err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_WRITE,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(world),
		},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(tokenPath, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCustodyOwnerOnly(tokenPath, fi); err == nil ||
		!strings.Contains(err.Error(), "write-capable Windows access") {
		t.Fatalf("a world-writable token ACL was not refused as write-capable: %v", err)
	}
}
