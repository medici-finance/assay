//go:build windows

package deskkit

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// checkFileOwner is the Windows variant of deskkit's roster owner check. Windows
// has no POSIX uid or mode bits — os.FileMode there is synthetic (a normal file
// reads 0666), so the group/world-writable mode test the unix variant runs would
// fire on EVERY file here. Ownership and write access on Windows are ACL
// questions, so this variant reads the file's owner SID and DACL and hands them to
// the neutral decision function evaluateRosterACL (rosteracl.go), which enforces
// the same intent as unix: the roster must be owned by the invoking user and
// writable by no one but the owner (plus the OS-trusted SYSTEM / Administrators
// principals).
//
// This is a PORT of the unix guarantee, not a weakening: unix refuses a roster
// another account can write; so does this. It never returns a silent nil — a
// descriptor it cannot read, or an ACE it cannot interpret, is a refusal. These
// are deskkit's OWN error strings, deliberately NOT converged with statusgen's.
func checkFileOwner(path string, _ os.FileInfo) error {
	model, err := windowsFileACLModel(path)
	if err != nil {
		return err
	}
	return evaluateRosterACL(path, model)
}

// windowsFileACLModel reads a file's owner SID and DACL and decodes them into the
// platform-independent rosterACLModel that the evaluateRosterACL (roster) and
// evaluateCustodyACL (GitLab token custody, #667) decisions consume. Both need the
// identical "who owns this file and who can write it" question answered from the
// real security descriptor, so the Win32 decode lives here once rather than being
// copied per caller. It never returns a silent nil model on a gap: a descriptor,
// owner, or ACE it cannot establish is a refusal, so the decision above it refuses
// rather than reading past an undetermined permission as clean.
func windowsFileACLModel(path string) (rosterACLModel, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return rosterACLModel{}, fmt.Errorf("cannot read the Windows owner/DACL of %s — refusing to act on a "+
			"file whose permissions cannot be established: %w", path, err)
	}

	model := rosterACLModel{}

	owner, _, err := sd.Owner()
	if err == nil && owner != nil {
		model.Owner = owner.String()
	}
	if user, uerr := windows.GetCurrentProcessToken().GetTokenUser(); uerr == nil &&
		user != nil && user.User.Sid != nil {
		model.CurrentUser = user.User.Sid.String()
	}
	// SYSTEM and the local Administrators group are OS-managed principals that
	// legitimately hold write on user files; they are trusted writers, mirroring
	// unix treating root as outside the "some other account" the check guards.
	if sys, e := windows.StringToSid("S-1-5-18"); e == nil {
		model.TrustedWriters = append(model.TrustedWriters, sys.String())
	}
	if admins, e := windows.StringToSid("S-1-5-32-544"); e == nil {
		model.TrustedWriters = append(model.TrustedWriters, admins.String())
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return rosterACLModel{}, fmt.Errorf("cannot establish a Windows DACL for %s — refusing to act on a "+
			"file whose write access cannot be bounded: %w", path, err)
	}
	// A nil DACL means "everyone, full control" in the Windows model — the widest
	// possible grant. Model it as one write-granting ACE for the World SID so the
	// decision refuses it rather than reading past a missing DACL as clean.
	if dacl == nil {
		world := "S-1-1-0"
		if w, e := windows.StringToSid(world); e == nil {
			world = w.String()
		}
		model.Entries = []rosterACE{{SID: world, Kind: rosterACEAllow, GrantsWrite: true}}
		return model, nil
	}

	for i := uint16(0); i < dacl.AceCount; i++ {
		var raw *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &raw); err != nil || raw == nil {
			return rosterACLModel{}, fmt.Errorf("cannot inspect Windows DACL entry %d for %s — refusing rather "+
				"than reading past an unreadable ACE: %w", i, path, err)
		}
		ace := rosterACE{
			InheritOnly: raw.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0,
			GrantsWrite: aceGrantsWrite(uint32(raw.Mask)),
		}
		switch raw.Header.AceType {
		case windows.ACCESS_ALLOWED_ACE_TYPE:
			ace.Kind = rosterACEAllow
		case windows.ACCESS_DENIED_ACE_TYPE:
			ace.Kind = rosterACEDeny
		default:
			ace.Kind = rosterACEUnsupported
		}
		sid := (*windows.SID)(unsafe.Pointer(&raw.SidStart))
		ace.SID = sid.String()
		model.Entries = append(model.Entries, ace)
	}

	return model, nil
}
