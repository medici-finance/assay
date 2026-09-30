package deskkit

import (
	"strings"
	"testing"
)

// TestEvaluateCustodyACL is the platform-independent guard the #667 Windows-ACL
// port turns on for token custody. It drives evaluateCustodyACL with injected ACL
// data — the sidOwner/sidOther/... constants and ownerOnlyModel() are shared with
// rosteracl_test.go — so the security logic that decides whether a GitLab token
// file's owner-only ACL passes is exercised on EVERY CI platform even though there
// is no Windows runner. The positive case is the whole point of the issue: an
// owner-only ACL that the old POSIX 0600 test rejected as synthetic 0666 is now
// ACCEPTED; every negative case asserts a REFUSAL whose message names the reason.
func TestEvaluateCustodyACL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(m *rosterACLModel)
		wantErr    bool
		wantSubstr string
	}{
		{
			name:    "owner-only ACL is accepted (the #667 fix: not rejected as synthetic 0666)",
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
		// Read parity with the unix 0600 rule: no principal but the owner (and the
		// OS-trusted SYSTEM / Administrators) may read a credential file.
		{
			name: "a foreign read-only ACE is refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow, GrantsRead: true})
			},
			wantErr:    true,
			wantSubstr: "read-capable Windows access to SID " + sidOther,
		},
		{
			name: "an Everyone read ACE is refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidWorld, Kind: rosterACEAllow, GrantsRead: true})
			},
			wantErr:    true,
			wantSubstr: "read-capable Windows access to SID " + sidWorld,
		},
		{
			name: "an Authenticated Users read ACE is refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidAuthUsers, Kind: rosterACEAllow, GrantsRead: true})
			},
			wantErr:    true,
			wantSubstr: "read-capable Windows access to SID " + sidAuthUsers,
		},
		{
			name: "a Users group read ACE is refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidUsers, Kind: rosterACEAllow, GrantsRead: true})
			},
			wantErr:    true,
			wantSubstr: "read-capable Windows access to SID " + sidUsers,
		},
		{
			name: "an inherited group read ACE is refused and named as inherited",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidUsers, Kind: rosterACEAllow, GrantsRead: true, Inherited: true})
			},
			wantErr:    true,
			wantSubstr: "(inherited from a parent folder)",
		},
		{
			name: "an inherited foreign write ACE is refused and named as inherited",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow, GrantsWrite: true, Inherited: true})
			},
			wantErr:    true,
			wantSubstr: "write-capable Windows access to SID " + sidOther + " (inherited from a parent folder)",
		},
		{
			name: "a foreign read ACE after an owner-only prefix is still refused",
			mutate: func(m *rosterACLModel) {
				m.Entries = append([]rosterACE{{SID: sidWorld, Kind: rosterACEAllow, GrantsRead: true}}, m.Entries...)
			},
			wantErr:    true,
			wantSubstr: "read-capable Windows access",
		},
		{
			name: "owner and OS-trusted principals holding read and write are accepted",
			mutate: func(m *rosterACLModel) {
				for i := range m.Entries {
					m.Entries[i].GrantsRead = true
				}
			},
			wantErr: false,
		},
		{
			name: "a foreign metadata-only ACE is accepted (no read, no write)",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow})
			},
			wantErr: false,
		},
		{
			name: "a foreign DENY read ACE never widens access",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidWorld, Kind: rosterACEDeny, GrantsRead: true})
			},
			wantErr: false,
		},
		{
			name: "an inherit-only foreign read ACE does not apply to this object",
			mutate: func(m *rosterACLModel) {
				m.Entries = append(m.Entries, rosterACE{SID: sidUsers, Kind: rosterACEAllow, GrantsRead: true, InheritOnly: true, Inherited: true})
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
			err := evaluateCustodyACL(`C:\Users\ada\.config\assay\gitlab-desk.token`, m)
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

// Well-known group SIDs the custody read cases name; the decision compares SIDs as
// opaque strings, so these only need to be distinct from the owner and trusted set.
const (
	sidAuthUsers = "S-1-5-11"     // NT AUTHORITY\Authenticated Users
	sidUsers     = "S-1-5-32-545" // BUILTIN\Users
)

// TestEvaluateCustodyACLReadMask pins the access-mask decode the Win32 adapter
// feeds the custody decision: every right that exposes a credential file's
// contents (the counterpart of the group/other read and execute bits unix 0600
// excludes) sets GrantsRead, and the metadata-only rights do not. It is a pure
// function over the mask, so it runs on every CI platform.
func TestEvaluateCustodyACLReadMask(t *testing.T) {
	for _, tc := range []struct {
		name string
		mask uint32
		want bool
	}{
		{"FILE_READ_DATA", 0x00000001, true},
		{"FILE_READ_EA", 0x00000008, true},
		{"FILE_EXECUTE", 0x00000020, true},
		{"GENERIC_READ", 0x80000000, true},
		{"GENERIC_EXECUTE", 0x20000000, true},
		{"GENERIC_ALL", 0x10000000, true},
		{"FILE_GENERIC_READ", 0x00120089, true},
		{"FILE_ALL_ACCESS", 0x001F01FF, true},
		{"FILE_READ_ATTRIBUTES", 0x00000080, false},
		{"READ_CONTROL", 0x00020000, false},
		{"SYNCHRONIZE", 0x00100000, false},
		{"metadata only", 0x00120080, false},
		{"FILE_WRITE_DATA only", 0x00000002, false},
		{"no rights", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := aceGrantsRead(tc.mask); got != tc.want {
				t.Fatalf("aceGrantsRead(%#08x) = %v, want %v", tc.mask, got, tc.want)
			}
		})
	}
}
