package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestQualifiedQueueRevalidatesLegacyCache(t *testing.T) {
	p := greenRollupPR(1, strings.Repeat("a", 40))
	p.UpdatedAt = "2026-10-23T12:00:00Z"
	f := factKey(p)
	f.HaveFiles = true
	f.Files = []string{"example.go"}
	s := &prFactStore{qualified: true, prior: map[int]prFacts{1: f}, next: map[int]prFacts{}}
	old := forgeFor
	calls := 0
	forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		calls++
		return nil, deskkit.ForgeRepo{}, errors.New("refresh required")
	}
	t.Cleanup(func() { forgeFor = old })
	if _, _, err := s.changedFiles("example-org/tracker", p); err == nil || calls != 1 {
		t.Fatal("unqualified legacy cache bypassed exact-head refresh", err, calls)
	}
	f.HeadValidated = true
	s.prior[1] = f
	files, complete, err := s.changedFiles("example-org/tracker", p)
	if err != nil || !complete || !files["example.go"] || calls != 1 {
		t.Fatal("qualified unchanged cache lost reuse", files, complete, err, calls)
	}
}
