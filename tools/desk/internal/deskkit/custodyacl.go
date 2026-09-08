package deskkit

import "fmt"

// evaluateCustodyACL is the Windows owner-only check for a token CUSTODY file —
// the sibling of evaluateRosterACL (rosteracl.go). It is the same decision over the
// same neutral rosterACLModel, using the same rosterWriteMask / aceGrantsWrite
// notion of "write-capable" (so the two can never disagree on WHICH access rights
// let a principal alter a protected file), and differs only in the subject its
// messages name: a credential file rather than the roster.
//
// It is the Windows half of the custody permission guarantee whose unix half is the
// POSIX 0600 test in custodyowner_unix.go, behind the same OS boundary the roster
// owner check uses (rosterowner_{unix,windows}.go). It exists because os.FileMode's
// permission bits are synthetic on Windows — a normal file reads 0666 — so the unix
// 0600 test rejects a token file that is correctly locked by an owner-only ACL
// (#667). The rule this enforces instead: a custody file must be OWNED by the
// invoking user and writable by no principal but the owner (plus the OS-trusted
// SYSTEM / Administrators writers the Win32 adapter supplies).
//
// A could-not-determine input (empty owner or current-user SID, or a DACL entry
// this tool cannot interpret) REFUSES rather than passing — establish the
// permissions or do not hand out the credential. This is a PORT of the unix
// guarantee, not a weakening: a `chmod 600` that makes Go report 0600 without
// tightening the DACL is exactly the fake green this check refuses to reward.
//
// deskkit keeps its OWN copy of the ACL model + decision, deliberately NOT converged
// with statusgen's, matching the existing rosterowner_unix.go split.
func evaluateCustodyACL(path string, m rosterACLModel) error {
	if m.Owner == "" {
		return fmt.Errorf("cannot determine the Windows owner of custody file %s — refusing to read a "+
			"credential whose ownership cannot be established", path)
	}
	if m.CurrentUser == "" {
		return fmt.Errorf("cannot determine the invoking Windows user for custody file %s — refusing to read "+
			"a credential whose ownership cannot be established", path)
	}
	if m.Owner != m.CurrentUser {
		return fmt.Errorf("custody file %s is owned by Windows SID %s, not by the invoking user (%s) — "+
			"refusing to read a credential from a file this user does not own", path, m.Owner, m.CurrentUser)
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
			return fmt.Errorf("custody file %s has a Windows DACL entry (#%d) this tool cannot "+
				"interpret — refusing rather than guessing its write scope", path, i)
		}
		if ace.InheritOnly || !ace.GrantsWrite {
			continue
		}
		if trusted[ace.SID] {
			continue
		}
		return fmt.Errorf("custody file %s grants write-capable Windows access to SID %s — "+
			"anything that can write it can substitute the credential this tool trusts", path, ace.SID)
	}
	return nil
}
