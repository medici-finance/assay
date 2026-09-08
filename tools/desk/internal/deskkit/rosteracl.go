package deskkit

import "fmt"

// This file holds the platform-INDEPENDENT core of the Windows roster-permission
// check, separated from the Win32 syscall layer in rosterowner_windows.go so the
// DECISION can be unit-tested on any OS with injected ACL data. There is no
// Windows CI runner in this house, so the syscall layer itself is exercised only
// on a Windows developer machine; this decision function — where the security
// logic actually lives — runs in every CI build.
//
// It carries no golang.org/x/sys/windows types on purpose: the Win32 adapter
// decodes the real security descriptor into the neutral model below and calls
// evaluateRosterACL, so nothing here needs to compile against the windows package.
//
// deskkit keeps its OWN copy of this decision, deliberately NOT converged with
// statusgen's, matching the existing rosterowner_unix.go split.

// rosterACEKind classifies a DACL entry the way the Windows access model does: an
// allow ACE grants the rights in its mask, a deny ACE removes them, and anything
// this tool does not recognise is refused rather than guessed.
type rosterACEKind int

const (
	rosterACEAllow rosterACEKind = iota
	rosterACEDeny
	rosterACEUnsupported
)

// rosterACE is one decoded access-control entry. GrantsWrite is computed by the
// Win32 adapter via aceGrantsWrite, so this model stays free of platform masks.
type rosterACE struct {
	SID         string // string form of the trustee SID, for comparison + messages
	Kind        rosterACEKind
	InheritOnly bool // applies only to children, not to this object itself
	GrantsWrite bool // the ACE's access mask intersects rosterWriteMask
}

// rosterACLModel is everything evaluateRosterACL needs, with no Win32 types in it.
// An empty Owner or CurrentUser means the adapter could not establish that value —
// a could-not-check, which the decision treats as a refusal, never a pass.
type rosterACLModel struct {
	Owner          string      // owner SID (string form)
	CurrentUser    string      // invoking user's SID (string form)
	Entries        []rosterACE // the object's DACL, in order
	TrustedWriters []string    // extra principals allowed to hold write: SYSTEM, Administrators
}

// rosterWriteMask is the set of Windows access rights that let a principal alter
// the roster file (or its directory): writing or appending data, changing
// attributes/EAs, deleting the object or a child, or rewriting the owner/DACL —
// the last two because a principal that can rewrite the DACL can grant itself
// write. GENERIC_WRITE and GENERIC_ALL are included because a generic right maps
// onto these specific ones. Values are the stable Win32 ABI constants (winnt.h);
// they are literals here so this file needs no golang.org/x/sys/windows import,
// and the Win32 adapter passes its raw ACCESS_MASK through aceGrantsWrite.
const rosterWriteMask uint32 = 0x00000002 | // FILE_WRITE_DATA
	0x00000004 | // FILE_APPEND_DATA
	0x00000010 | // FILE_WRITE_EA
	0x00000100 | // FILE_WRITE_ATTRIBUTES
	0x00000040 | // FILE_DELETE_CHILD
	0x00010000 | // DELETE
	0x00040000 | // WRITE_DAC
	0x00080000 | // WRITE_OWNER
	0x10000000 | // GENERIC_ALL
	0x40000000 //   GENERIC_WRITE

// aceGrantsWrite reports whether an access mask intersects the write-capable set.
func aceGrantsWrite(mask uint32) bool { return mask&rosterWriteMask != 0 }

// evaluateRosterACL is the sshd rule expressed for Windows ACLs — the Windows
// counterpart of the uid+mode check in rosterowner_unix.go. It refuses the roster
// unless BOTH hold:
//
//  1. the file is OWNED by the invoking user, and
//  2. no principal other than the owner or an OS-trusted writer (SYSTEM /
//     Administrators) holds write-capable access to it.
//
// A could-not-determine input (empty owner or current-user SID, or a DACL entry
// this tool cannot interpret) REFUSES rather than passing — the same
// "establish the permissions or don't read the roster" stance the unix variant
// takes, and the whole reason this check exists instead of a silent nil return.
func evaluateRosterACL(path string, m rosterACLModel) error {
	if m.Owner == "" {
		return fmt.Errorf("cannot determine the Windows owner of %s — refusing to read a roster "+
			"whose ownership cannot be established", path)
	}
	if m.CurrentUser == "" {
		return fmt.Errorf("cannot determine the invoking Windows user for %s — refusing to read a "+
			"roster whose ownership cannot be established", path)
	}
	if m.Owner != m.CurrentUser {
		return fmt.Errorf("roster config %s is owned by Windows SID %s, not by the invoking user (%s) — "+
			"refusing to take the trusted-identity list from a file this user does not own",
			path, m.Owner, m.CurrentUser)
	}
	trusted := map[string]bool{m.Owner: true}
	for _, s := range m.TrustedWriters {
		if s != "" {
			trusted[s] = true
		}
	}
	for i, ace := range m.Entries {
		switch ace.Kind {
		case rosterACEDeny:
			continue // a deny narrows access; it can never widen it
		case rosterACEUnsupported:
			return fmt.Errorf("roster config %s has a Windows DACL entry (#%d) this tool cannot "+
				"interpret — refusing rather than guessing its write scope", path, i)
		}
		if ace.InheritOnly || !ace.GrantsWrite {
			continue
		}
		if trusted[ace.SID] {
			continue
		}
		return fmt.Errorf("roster config %s grants write-capable Windows access to SID %s — "+
			"anything that can write it can name the accounts this tool trusts", path, ace.SID)
	}
	return nil
}
