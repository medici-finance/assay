package main

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"strings"
	"testing"
	"time"
)

// regression: #5704
func TestSubjectPayloadReader(t *testing.T) {
	const key = "repo--stream--08"
	subject := deskkit.BriefSubject{UUID: "11cf440e-4e8e-4e51-98d6-211210dbefab", Version: 2, Repository: "owner/repo", Commit: strings.Repeat("a", 40), Path: "docs/streams/stream/brief-08-work.md"}
	base := "dispatch-claim " + key + " owner=worker state=dispatched branch=feat/work"
	message, err := deskkit.EncodeClaimSubject(base, subject)
	if err != nil {
		t.Fatal(err)
	}
	hash := plumbing.NewHash(strings.Repeat("b", 40))
	now := time.Unix(1000, 0)
	refs := []*plumbing.Reference{plumbing.NewHashReference(plumbing.ReferenceName(deskkit.DispatchClaimActiveRefsPrefix+key), hash)}
	read := func(message string) []claimRecord {
		claims, _, err := claimPage(refs, "owner/repo", now, 10, "", func(plumbing.Hash) (gitcore.TagPayload, error) {
			return gitcore.TagPayload{Message: message, When: now}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return claims
	}
	old, new := read(base), read(message)
	if len(old) != 1 || len(new) != 1 || old[0] != new[0] {
		t.Fatalf("positional reader changed: %v %v", old, new)
	}
}
