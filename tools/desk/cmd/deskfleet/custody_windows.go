//go:build windows

package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// createRestricted creates a NEW file at path whose security descriptor is set IN the
// CreateFile call: owner = the invoking user, and a PROTECTED DACL (no inherited entries)
// granting full control to that user, SYSTEM and the local Administrators group only — the
// same trusted-writer set the read-side custody evaluation accepts. CREATE_NEW refuses an
// existing path. There is no moment at which the file exists with the directory's inherited,
// possibly wider, access list.
func createRestricted(path string) (*os.File, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return nil, fmt.Errorf("cannot determine the invoking Windows user to restrict %s: %v", path, err)
	}
	sid := user.User.Sid.String()
	sddl := "O:" + sid + "D:P(A;;FA;;;" + sid + ")(A;;FA;;;SY)(A;;FA;;;BA)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("build the owner-only security descriptor for %s: %v", path, err)
	}
	sa := windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(sa))
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode path %s: %v", path, err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, 0, &sa, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("create %s: %v", path, err)
	}
	return os.NewFile(uintptr(h), path), nil
}
