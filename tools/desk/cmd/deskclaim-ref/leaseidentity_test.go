package main

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestLeaseWritesRequireIdentity(t *testing.T) {
	t.Setenv("DESK_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	const id = "example--issue-91"
	for _, args := range [][]string{{"acquire", id}, {"steal", id, "--reason", "reclaim"}, {"progress", id, "--branch", "work"}} {
		t.Run(args[0], func(t *testing.T) {
			f := newStore()
			if args[0] != "acquire" {
				f.seedClaim(id, "unknown", "claimed", "", time.Minute)
			}
			before := f.claims[id]
			call, _, stderr := harness(t, f)
			rc := call(append(args, "--repo", aliasRepo)...)
			if rc != exitRefused || !strings.Contains(stderr.String(), "--owner") || f.claims[id] != before {
				t.Errorf("unidentified %s returned %d; claim changed=%t: %s", args[0], rc, f.claims[id] != before, stderr)
			}
		})
	}
	f := newStore()
	call, _, _ := harness(t, f)
	for _, owner := range []string{"unknown", ""} {
		f.seedClaim(id, owner, "claimed", "", time.Minute)
		before := f.claims[id]
		if rc := call("release", id, "--repo", aliasRepo); rc != exitRefused || f.claims[id] != before {
			t.Fatalf("anonymous release of legacy owner %q changed the claim or returned %d", owner, rc)
		}
	}
}

type successorAfterRead struct {
	*fakeStore
	key string
}

func (s *successorAfterRead) Read(id string) (deskkit.ClaimStoreRecord, deskkit.ClaimReadStatus) {
	ref, status := s.fakeStore.Read(id)
	if id == s.key && status == deskkit.ClaimReadHeld {
		s.seedClaim(id, "successor", "dispatched", "new", 0)
	}
	return ref, status
}

func TestReleaseAndAliasCleanupPreserveRacingSuccessor(t *testing.T) {
	const id = "a--pr-108"
	t.Run("release", func(t *testing.T) {
		f := newStore()
		f.seedClaim(id, "mine", "claimed", "", time.Minute)
		call := listFailHarness(t, &successorAfterRead{f, id})
		if rc := call("release", id, "--repo", aliasRepo, "--owner", "mine"); rc != exitRefused || fieldOf(f.claims[id].msg, "owner") != "successor" {
			t.Fatalf("release rc=%d; retained claim %q", rc, f.claims[id].msg)
		}
	})
	for _, when := range []string{"before cleanup read", "after cleanup read"} {
		t.Run(when, func(t *testing.T) {
			f := newStore()
			f.seedClaim(id, "mine", "claimed", "", 0)
			f.seedClaim("b--pr-108", "other", "dispatched", "", 0)
			written := f.claims[id].msg
			saved := store
			t.Cleanup(func() { store = saved })
			if when == "before cleanup read" {
				f.seedClaim(id, "successor", "dispatched", "new", 0)
				store = f
			} else {
				store = &successorAfterRead{f, id}
			}
			rc, stop := checkAliases(id, written)
			if !stop || rc != exitRefused || fieldOf(f.claims[id].msg, "owner") != "successor" {
				t.Fatalf("alias cleanup stop=%t rc=%d; retained claim %q", stop, rc, f.claims[id].msg)
			}
		})
	}
}
