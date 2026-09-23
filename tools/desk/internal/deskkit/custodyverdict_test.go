package deskkit

import "testing"

// TestClassifyCustodyModel pins the three-state sort the fleet-provisioning write path
// relies on (the recorded custody ruling, option 2): an inconclusive read-back WARNS, a
// definite one REFUSES. It drives classifyCustodyModel with injected ACL data, so the Windows sort runs on
// every CI platform. The load-bearing negatives are the MIXED cases: an uninterpretable entry
// must never launder a determinable violation (a foreign owner, a foreign writer) into
// "inconclusive".
func TestClassifyCustodyModel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(m *rosterACLModel)
		want   CustodyState
	}{
		{"owner-only ACL is verified", func(*rosterACLModel) {}, CustodyVerified},
		{"unknown owner is inconclusive", func(m *rosterACLModel) { m.Owner = "" }, CustodyInconclusive},
		{"unknown invoking user is inconclusive", func(m *rosterACLModel) { m.CurrentUser = "" }, CustodyInconclusive},
		{"an uninterpretable entry alone is inconclusive", func(m *rosterACLModel) {
			m.Entries = append(m.Entries, rosterACE{Kind: rosterACEUnsupported})
		}, CustodyInconclusive},
		{"a foreign owner is refused", func(m *rosterACLModel) { m.Owner = sidOther }, CustodyRefused},
		{"a foreign write-capable entry is refused", func(m *rosterACLModel) {
			m.Entries = append(m.Entries, rosterACE{SID: sidOther, Kind: rosterACEAllow, GrantsWrite: true})
		}, CustodyRefused},
		{"a foreign writer BEHIND an uninterpretable entry is still refused", func(m *rosterACLModel) {
			m.Entries = append(m.Entries,
				rosterACE{Kind: rosterACEUnsupported},
				rosterACE{SID: sidWorld, Kind: rosterACEAllow, GrantsWrite: true})
		}, CustodyRefused},
		{"a foreign owner with an uninterpretable entry is still refused", func(m *rosterACLModel) {
			m.Owner = sidOther
			m.Entries = append(m.Entries, rosterACE{Kind: rosterACEUnsupported})
		}, CustodyRefused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := ownerOnlyModel()
			tc.mutate(&m)
			got := classifyCustodyModel(`C:\cfg\gitlab-worker.token`, m)
			if got.State != tc.want {
				t.Fatalf("state = %v, want %v (err=%v)", got.State, tc.want, got.Err)
			}
			if got.State != CustodyVerified && got.Err == nil {
				t.Fatalf("a %v verdict must carry its reason", got.State)
			}
		})
	}
}
