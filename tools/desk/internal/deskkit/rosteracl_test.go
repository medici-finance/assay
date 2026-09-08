package deskkit

import (
	"strings"
	"testing"
)

// These SIDs stand in for real principals; the decision compares them as opaque
// strings, so their exact values only need to be distinct and stable.
const (
	sidOwner  = "S-1-5-21-1-2-3-1001" // the invoking user
	sidOther  = "S-1-5-21-1-2-3-2002" // some other account
	sidWorld  = "S-1-1-0"             // Everyone
	sidSystem = "S-1-5-18"            // NT AUTHORITY\SYSTEM
	sidAdmins = "S-1-5-32-544"        // BUILTIN\Administrators
)

func ownerOnlyModel() rosterACLModel {
	return rosterACLModel{
		Owner:          sidOwner,
		CurrentUser:    sidOwner,
		TrustedWriters: []string{sidSystem, sidAdmins},
		Entries: []rosterACE{
			{SID: sidOwner, Kind: rosterACEAllow, GrantsWrite: true},
			{SID: sidSystem, Kind: rosterACEAllow, GrantsWrite: true},
			{SID: sidAdmins, Kind: rosterACEAllow, GrantsWrite: true},
		},
	}
}

// TestEvaluateRosterACL is the unit test the Windows-ACL port turns on: it drives
// the platform-independent decision function with injected ACL data, so the
// security logic is exercised on every CI platform even though there is no Windows
// runner. Each negative case asserts a REFUSAL, and the message names the reason.
func TestEvaluateRosterACL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(m *rosterACLModel)
		wantErr    bool
		wantSubstr string
	}{
		{
			name:    "owner-only DACL is accepted",
			mutate:  func(*rosterACLModel) {},
			wantErr: false,
		},
		{
			name: "a foreign write-granting ACE is refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow, GrantsWrite: true})
			},
			wantErr:    true,
			wantSubstr: "write-capable Windows access",
		},
		{
			name: "a world-writable ACE is refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidWorld, Kind: rosterACEAllow, GrantsWrite: true})
			},
			wantErr:    true,
			wantSubstr: "write-capable Windows access",
		},
		{
			name: "a foreign READ-only ACE is accepted",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow, GrantsWrite: false})
			},
			wantErr: false,
		},
		{
			name: "a foreign DENY ACE never widens access",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEDeny, GrantsWrite: true})
			},
			wantErr: false,
		},
		{
			name: "an inherit-only foreign write ACE does not apply to this object",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow, GrantsWrite: true, InheritOnly: true})
			},
			wantErr: false,
		},
		{
			name: "an uninterpretable ACE is refused rather than guessed",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEUnsupported})
			},
			wantErr:    true,
			wantSubstr: "cannot interpret",
		},
		{
			name:       "a file owned by another user is refused",
			mutate:     func(m *rosterACLModel) { m.Owner = sidOther },
			wantErr:    true,
			wantSubstr: "not by the invoking user",
		},
		{
			name:       "an undeterminable owner is refused, never passed",
			mutate:     func(m *rosterACLModel) { m.Owner = "" },
			wantErr:    true,
			wantSubstr: "cannot determine the Windows owner",
		},
		{
			name:       "an undeterminable current user is refused, never passed",
			mutate:     func(m *rosterACLModel) { m.CurrentUser = "" },
			wantErr:    true,
			wantSubstr: "cannot determine the invoking Windows user",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := ownerOnlyModel()
			tc.mutate(&m)
			err := evaluateRosterACL(`C:\Users\ada\.config\assay\roster.env`, m)
			if tc.wantErr && err == nil {
				t.Fatalf("expected a refusal, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected acceptance, got %v", err)
			}
			if tc.wantErr && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("refusal %q does not name the reason %q", err, tc.wantSubstr)
			}
		})
	}
}

// TestAceGrantsWrite pins the write-capable mask: every right that lets a
// principal alter the file (or rewrite its DACL to grant itself write) counts as
// write, and pure-read rights do not. A regression that drops WRITE_DAC/WRITE_OWNER
// from the mask — the subtle way this check silently weakens — reddens here.
func TestAceGrantsWrite(t *testing.T) {
	writeMasks := map[string]uint32{
		"FILE_WRITE_DATA":       0x00000002,
		"FILE_APPEND_DATA":      0x00000004,
		"FILE_WRITE_EA":         0x00000010,
		"FILE_WRITE_ATTRIBUTES": 0x00000100,
		"FILE_DELETE_CHILD":     0x00000040,
		"DELETE":                0x00010000,
		"WRITE_DAC":             0x00040000,
		"WRITE_OWNER":           0x00080000,
		"GENERIC_ALL":           0x10000000,
		"GENERIC_WRITE":         0x40000000,
	}
	for name, mask := range writeMasks {
		if !aceGrantsWrite(mask) {
			t.Errorf("%s (%#x) must count as write-capable", name, mask)
		}
	}
	readMasks := map[string]uint32{
		"FILE_READ_DATA": 0x00000001,
		"READ_CONTROL":   0x00020000,
		"SYNCHRONIZE":    0x00100000,
		"GENERIC_READ":   0x80000000,
		"none":           0x00000000,
	}
	for name, mask := range readMasks {
		if aceGrantsWrite(mask) {
			t.Errorf("%s (%#x) must NOT count as write-capable", name, mask)
		}
	}
}
