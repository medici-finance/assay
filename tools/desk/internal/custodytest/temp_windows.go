//go:build windows

// Package custodytest provides owner-only fixture directories for _test.go files only.
package custodytest

import (
	"golang.org/x/sys/windows"
	"testing"
)

// PrivateTempDir protects the directory DACL before any fixture credential is written.
func PrivateTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	return dir
}
