package main

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/gitcore"
	"strings"
	"testing"
	"time"
)

func TestClaimPageBoundsAndImmutablePayload(t *testing.T) {
	now := time.Now().UTC()
	var refs []*plumbing.Reference
	for i := 0; i < 742; i++ {
		refs = append(refs, plumbing.NewHashReference(plumbing.ReferenceName(fmt.Sprintf("%sr--issue-%04d", deskkit.DispatchClaimActiveRefsPrefix, i)), plumbing.NewHash(fmt.Sprintf("%040x", i+1))))
	}
	calls := 0
	fetch := func(hash plumbing.Hash) (gitcore.TagPayload, error) {
		calls++
		var i int
		fmt.Sscanf(hash.String(), "%040x", &i)
		return gitcore.TagPayload{Message: fmt.Sprintf("dispatch-claim r--issue-%04d owner=session state=dispatched branch=topic", i-1), When: now.Add(-time.Hour)}, nil
	}
	claims, next, err := claimPage(refs, "org/repo", now, 50, "", fetch)
	if err != nil || calls != 50 || len(claims) != 50 || next != "r--issue-0049" {
		t.Fatalf("calls=%d len=%d next=%s err=%v", calls, len(claims), next, err)
	}
	if claims[0].Version != refs[0].Hash().String() || claims[0].Ref != refs[0].Name().String() {
		t.Fatal("immutable version not retained")
	}
	calls = 0
	claims, _, err = claimPage(refs, "org/repo", now, 50, next, fetch)
	if err != nil || calls != 50 || claims[0].Key != "r--issue-0050" {
		t.Fatalf("resume: %d %v", calls, err)
	}
	_, _, err = claimPage(refs, "org/repo", now, 50, "", func(plumbing.Hash) (gitcore.TagPayload, error) {
		return gitcore.TagPayload{Message: "ordinary commit", When: now}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "held for inspection") {
		t.Fatalf("malformed claim not held: %v", err)
	}
}
func TestReclaimVersionPreservesConcurrentNewOwner(t *testing.T) {
	old := plumbing.NewHash(strings.Repeat("1", 40))
	replacement := plumbing.NewHash(strings.Repeat("2", 40))
	claim := claimRecord{Key: "repo--issue-1", Ref: deskkit.DispatchClaimActiveRefsPrefix + "repo--issue-1", Version: old.String()}
	current := replacement
	push := func(_ context.Context, u gitcore.RefUpdate) (gitcore.RefUpdateResult, error) {
		if u.Old != old || u.New != plumbing.ZeroHash || u.Ref.String() != claim.Ref {
			t.Fatalf("wrong CAS: %+v", u)
		}
		if current != u.Old {
			return gitcore.RefUpdateRejected, nil
		}
		current = u.New
		return gitcore.RefUpdateApplied, nil
	}
	if err := reclaimVersion(claim, gitcore.ListOpts{}, push); err == nil {
		t.Fatal("replacement owner deleted")
	}
	if current != replacement {
		t.Fatal("replacement claim changed")
	}
	current = old
	if err := reclaimVersion(claim, gitcore.ListOpts{}, push); err != nil {
		t.Fatal(err)
	}
	if current != plumbing.ZeroHash {
		t.Fatal("assessed claim retained")
	}
	claim.Version = ""
	if err := reclaimVersion(claim, gitcore.ListOpts{}, func(context.Context, gitcore.RefUpdate) (gitcore.RefUpdateResult, error) {
		t.Fatal("unversioned claim sent")
		return 0, nil
	}); err == nil {
		t.Fatal("unversioned claim admitted")
	}
}

func TestClaimPageUnreadableAndMalformedPayloadsHold(t *testing.T) {
	now := time.Now().UTC()
	refs := []*plumbing.Reference{plumbing.NewHashReference(plumbing.ReferenceName(deskkit.DispatchClaimActiveRefsPrefix+"r--issue-1"), plumbing.NewHash(strings.Repeat("1", 40)))}
	for _, mode := range []string{"transport", "wrong-key"} {
		_, _, err := claimPage(refs, "org/repo", now, 50, "", func(plumbing.Hash) (gitcore.TagPayload, error) {
			if mode == "transport" {
				return gitcore.TagPayload{}, fmt.Errorf("unreadable")
			}
			msg := "dispatch-claim r--issue-1 owner=session state=dispatched branch=topic"
			at := now.Add(-time.Hour)
			if mode == "wrong-key" {
				msg = strings.Replace(msg, "r--issue-1", "r--issue-2", 1)
			}
			return gitcore.TagPayload{Message: msg, When: at}, nil
		})
		if err == nil {
			t.Fatalf("%s admitted", mode)
		}
	}
}
