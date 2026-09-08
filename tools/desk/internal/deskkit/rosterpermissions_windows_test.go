//go:build windows

package deskkit

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// supportsPOSIXRosterModes is false on Windows: os.FileMode's permission bits are
// synthetic there, so os.Chmod cannot produce the group/world-writable state the
// POSIX permission loop asserts on. TestConfigHomePermissionsEnforced skips that
// loop here and relies on the ACL check plus the ACL cases below.
const supportsPOSIXRosterModes = false

// secureTestRosterPaths rewrites each path's DACL to grant the invoking user full
// control and nothing else (PROTECTED so no inherited ACE survives). A fresh
// t.TempDir() inherits ACEs from the profile that the roster ACL check would
// otherwise flag, so the fixture must establish the owner-only state the positive
// control expects — the same state a correctly-installed roster has on Windows.
func secureTestRosterPaths(paths ...string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
			nil, nil, acl, nil); err != nil {
			return err
		}
	}
	return nil
}

// TestWindowsRosterACLIntegration drives the real syscall adapter (checkFileOwner)
// end to end on Windows: an owner-only DACL is accepted, and adding a
// world-writable grant makes it refuse, naming the reason. It cannot run in CI
// (there is no Windows runner); it gives a Windows developer coverage of the
// adapter that the platform-independent TestEvaluateRosterACL cannot.
func TestWindowsRosterACLIntegration(t *testing.T) {
	dir := t.TempDir()
	if err := secureTestRosterPaths(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkFileOwner(dir, fi); err != nil {
		t.Fatalf("an owner-only DACL was refused: %v", err)
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
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err := checkFileOwner(dir, fi); err == nil ||
		!strings.Contains(err.Error(), "write-capable Windows access") {
		t.Fatalf("a world-writable DACL was not refused as write-capable: %v", err)
	}
}
