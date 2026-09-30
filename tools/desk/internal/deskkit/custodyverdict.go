package deskkit

// custodyverdict.go — a THREE-STATE answer to "is this freshly written custody file
// owner-only?", for the one caller that must treat "cannot tell" differently from "no":
// the GitLab fleet-provisioning verb (cmd/deskfleet) at the moment it WRITES a token.
//
// WHY A SECOND ENTRY POINT. VerifyCustodyOwnerOnly (custodyowner_{unix,windows}.go) is the
// READ-side guarantee and returns a plain error: every failure refuses, including "the
// access list cannot be read". That is the right posture for a reader and it is unchanged
// here. The write side is governed by a recorded human ruling (option 2 of the fleet-token custody decision):
// a read-back that DEFINITELY shows the file is not owner-only still refuses, but a read-back
// that is INCONCLUSIVE — the owner or invoking user cannot be established, the access list
// cannot be read, or one of its entries cannot be interpreted — WARNS and lets provisioning
// finish. To honour that the caller needs to know which of the two failures it got, and a
// string match on an error message is not a contract.
//
// WHAT IT DOES NOT DO. It does not re-derive an ACL opinion. The Windows decision is still
// evaluateCustodyACL's (custodyacl.go), called unchanged; the unix decision is still
// VerifyCustodyOwnerOnly's. This file only sorts their inputs into "the decision could be
// made" and "it could not", using evaluateCustodyACL's OWN documented could-not-determine
// cases (an empty owner, an empty current user, an entry it cannot interpret).

import "fmt"

// CustodyState is the three-state outcome of a custody read-back.
type CustodyState int

const (
	// CustodyVerified: the file was read back and is owner-only.
	CustodyVerified CustodyState = iota + 1
	// CustodyRefused: the file was read back and is DEFINITELY not owner-only (a foreign
	// owner, a foreign write-capable or read-capable entry, a POSIX mode other than 0600,
	// a symbolic link where the regular file was written).
	CustodyRefused
	// CustodyInconclusive: the read-back could not establish the answer either way.
	CustodyInconclusive
)

func (s CustodyState) String() string {
	switch s {
	case CustodyVerified:
		return "verified"
	case CustodyRefused:
		return "refused"
	case CustodyInconclusive:
		return "inconclusive"
	}
	return fmt.Sprintf("CustodyState(%d)", int(s))
}

// CustodyVerdict is a custody read-back's state plus, for any state but Verified, the
// underlying reason. Err never carries file CONTENTS — only the path and the access facts.
type CustodyVerdict struct {
	State CustodyState
	Err   error
}

// classifyCustodyModel sorts a Windows ACL model into the three states WITHOUT a second
// decision procedure: evaluateCustodyACL decides; this only asks whether a refusal rested
// on an input evaluateCustodyACL itself documents as could-not-determine.
//
//   - evaluateCustodyACL accepts the model → Verified.
//   - the owner or the invoking user is unknown → Inconclusive (ownership is the first
//     thing evaluateCustodyACL checks and it cannot be established).
//   - otherwise, re-run evaluateCustodyACL on the model with the entries it cannot
//     interpret set aside. If it STILL refuses, the refusal rests on facts it could read
//     (a foreign owner, a foreign write-capable or read-capable entry) → Refused. If it
//     now accepts, the only obstacle was the uninterpretable entries → Inconclusive.
//
// The second evaluation can only ever turn a refusal into Inconclusive when every
// determinable fact already passed; it can never turn a determinable violation into
// anything but Refused.
func classifyCustodyModel(path string, m rosterACLModel) CustodyVerdict {
	err := evaluateCustodyACL(path, m)
	if err == nil {
		return CustodyVerdict{State: CustodyVerified}
	}
	if m.Owner == "" || m.CurrentUser == "" {
		return CustodyVerdict{State: CustodyInconclusive, Err: err}
	}
	readable := m
	readable.Entries = nil
	for _, e := range m.Entries {
		if e.Kind != rosterACEUnsupported {
			readable.Entries = append(readable.Entries, e)
		}
	}
	if len(readable.Entries) == len(m.Entries) {
		return CustodyVerdict{State: CustodyRefused, Err: err}
	}
	if rerr := evaluateCustodyACL(path, readable); rerr != nil {
		return CustodyVerdict{State: CustodyRefused, Err: rerr}
	}
	return CustodyVerdict{State: CustodyInconclusive, Err: err}
}
