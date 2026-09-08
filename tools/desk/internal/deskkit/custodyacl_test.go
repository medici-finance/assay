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
