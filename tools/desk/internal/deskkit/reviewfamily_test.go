package deskkit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestReviewFamilyKeys(t *testing.T) {
	for _, alias := range []string{"", "tracker=tracker:", "tracker=trk:", "tracker=wrong:,example-org/tracker=trk:"} {
		t.Run(alias, func(t *testing.T) {
			r := goldenRoster()
			r[EnvRepoAliases] = alias
			withRoster(t, r)
			families := ReviewClaimFamilies("example-org/tracker", 47)
			want := []string{"refs/dispatch/tracker--pr-47"}
			if strings.Contains(alias, "=trk:") {
				want = append([]string{"refs/dispatch/trk--pr-47"}, want...)
			}
			if !reflect.DeepEqual(families, want) {
				t.Fatalf("families=%v, want %v", families, want)
			}
			for _, prefix := range want {
				for _, lane := range []string{"", "--security", "--rr3-corr"} {
					key := strings.TrimPrefix(prefix, DispatchClaimActiveRefsPrefix) + lane
					if err := ValidateReviewClaimKey(key, "example-org/tracker", 47); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, key := range []string{"foreign--pr-47", "tracker--pr-479", "tracker--pr-4", "tracker--pr-47--", "tracker--pr-47--x/y", "tracker--issue-47"} {
				if err := ValidateReviewClaimKey(key, "example-org/tracker", 47); err == nil {
					t.Fatalf("accepted %s", key)
				}
			}
		})
	}
}

func TestReviewFamilyRead(t *testing.T) {
	r := goldenRoster()
	r[EnvRepoAliases] = "tracker=trk:"
	withRoster(t, r)
	for _, tc := range []struct {
		name   string
		states []ClaimLiveness
		want   ClaimLiveness
	}{
		{"empty", []ClaimLiveness{ClaimReleased, ClaimReleased}, ClaimReleased},
		{"basename", []ClaimLiveness{ClaimReleased, ClaimHeld}, ClaimHeld},
		{"alias", []ClaimLiveness{ClaimHeld, ClaimReleased}, ClaimHeld},
		{"first unreadable", []ClaimLiveness{ClaimLivenessUnknown, ClaimReleased}, ClaimLivenessUnknown},
		{"second unreadable", []ClaimLiveness{ClaimReleased, ClaimLivenessUnknown}, ClaimLivenessUnknown},
		{"positive after unreadable", []ClaimLiveness{ClaimLivenessUnknown, ClaimHeld}, ClaimHeld},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := ReadReviewClaims("example-org/tracker", 47, func(prefix string) ([]string, error) {
				state := tc.states[calls]
				calls++
				if state == ClaimLivenessUnknown {
					return nil, fmt.Errorf("offline unavailable")
				}
				if state == ClaimHeld {
					return []string{prefix + "--security"}, nil
				}
				return []string{prefix + "9--security", "refs/dispatch/foreign--pr-47"}, nil
			})
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if tc.states[0] != ClaimHeld && calls != 2 {
				t.Fatalf("read %d families, want 2", calls)
			}
		})
	}
	if got := ReadReviewClaims("example-org/tracker", 0, func(string) ([]string, error) { t.Fatal("invalid PR read"); return nil, nil }); got != ClaimLivenessUnknown {
		t.Fatal(got)
	}
}
