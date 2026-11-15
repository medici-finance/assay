package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func claimWorkFixture(t *testing.T) (deskkit.WorkCatalog, []readercaps.Observation) {
	t.Helper()
	id := workref.Identity{Kind: workref.Brief, Cell: "cell", Repo: "repo", Stream: "stream", Number: "08"}
	uuid := "11cf440e-4e8e-4e51-98d6-211210dbefab"
	registry, err := workref.NewRegistry([]workref.Root{{Cell: "cell", Repo: "repo", Aliases: []string{"old"}}}, []workref.Binding{{Identity: id, Key: uuid}})
	if err != nil {
		t.Fatal(err)
	}
	catalog := deskkit.WorkCatalog{Declaring: workref.Context{Cell: "cell", Repo: "repo"}, Registry: registry, Records: []deskkit.WorkRecord{{Identity: id, Label: "repo", Subject: deskkit.BriefSubject{UUID: uuid, Version: 2, Repository: "medici-finance/assay", Commit: strings.Repeat("a", 40), Path: "docs/streams/stream/brief-08-work.md"}}}}
	peers := []readercaps.Observation{{State: readercaps.Current, Advertisement: readercaps.Advertisement{Schema: readercaps.AdvertisementSchema, Provider: "fixture", Version: "1", Operations: []readercaps.Operation{{Operation: "claim", Schemas: []string{"brief-v4"}, Semantics: []string{"qualified-brief-subject-v1"}}}}}}
	return catalog, peers
}

// regression: #5704
func TestPrefixedClaimAliases(t *testing.T) {
	catalog, peers := claimWorkFixture(t)
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	loadClaimWork = func(string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		return catalog, peers, true, nil
	}
	f := newStore()
	run, so, se := harness(t, f)
	if rc := run("acquire", "stream:b-08@2", "--repo", "medici-finance/assay", "--owner", "worker"); rc != 0 {
		t.Fatalf("actual acquire failed %d %s", rc, se.String())
	}
	const key = "repo--stream--08"
	original := f.claims[key]
	if len(f.claims) != 1 {
		t.Fatalf("wrong physical claims %v", f.claims)
	}
	for _, alias := range []string{"stream/08", "repo:stream:08", "cell:repo:stream/08", "old:stream:b-08@2", "stream/b-08@2"} {
		if rc := run("acquire", alias, "--repo", "medici-finance/assay", "--owner", "other"); rc == 0 {
			t.Fatalf("second alias worker admitted %s %s", alias, so.String())
		}
		if len(f.claims) != 1 || f.claims[key] != original {
			t.Fatalf("alias changed holder/generation: %s", alias)
		}
	}
	if rc := run("progress", "old:stream:b-08@2", "--repo", "medici-finance/assay", "--owner", "worker", "--branch", "feat/work"); rc != 0 {
		t.Fatalf("progress %d %s", rc, se.String())
	}
	advanced := f.claims[key]
	if rc := run("progress", key, "--repo", "medici-finance/assay", "--owner", "worker", "--branch", "feat/work"); rc == 0 || f.claims[key] != advanced {
		t.Fatal("opaque writer erased selected subject")
	}

	subject, present, err := deskkit.DecodeClaimSubject(advanced.msg)
	if err != nil || !present || subject != catalog.Records[0].Subject || fieldOf(advanced.msg, "owner") != "worker" {
		t.Fatalf("subject/custody lost: %v %s", err, advanced.msg)
	}
	catalog.Records[0].Subject.Version = 3
	if rc := run("acquire", "stream:b-08@3", "--repo", "medici-finance/assay", "--owner", "other"); rc == 0 || f.claims[key] != advanced {
		t.Fatal("conflicting pinned subject overwrote claim")
	}
	catalog.Records[0].Subject.Version = 2
	for _, bad := range []string{"stream:m-08@2", "STREAM:08", "stream:b-08@0", "repo--stream--b-08@2"} {
		if rc := run("acquire", bad, "--repo", "medici-finance/assay"); rc == 0 || f.claims[key] != advanced {
			t.Fatalf("strict-refused input normalized: %s", bad)
		}
	}
	// An old payload remains held; a pinned request cannot silently upgrade it.
	f.seedClaim(key, "legacy", "claimed", "", 0)
	legacy := f.claims[key]
	if rc := run("acquire", "stream:b-08@2", "--repo", "medici-finance/assay"); rc == 0 || f.claims[key] != legacy {
		t.Fatal("old payload was upgraded or stolen")
	}
	// Incomplete/stale peer observations stop before even opening a new store.
	peers[0].State = readercaps.Stale
	if rc := run("acquire", "stream:b-08@2", "--repo", "medici-finance/assay"); rc == 0 {
		t.Fatal("stale peer admitted")
	}
}

// regression: #5704
func TestSelectedClaimCAS(t *testing.T) {
	catalog, peers := claimWorkFixture(t)
	selected, err := catalog.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	f := newStore()
	s := &subjectClaimStore{ClaimStore: f, selected: selected, peers: peers, complete: true, repository: "medici-finance/assay"}
	msg := claimTestMessage(selected.ClaimKey, "worker", "claimed", "", "")
	if s.CreateIfAbsent(selected.ClaimKey, msg) != deskkit.ClaimWriteApplied {
		t.Fatal("create")
	}
	ref, status := s.Read(selected.ClaimKey)
	if status != deskkit.ClaimReadHeld {
		t.Fatal("read")
	}
	// A second actor wins after our read. Exact-old CAS must not restore worker.
	newer, _ := deskkit.EncodeClaimSubject(claimTestMessage(selected.ClaimKey, "other", "dispatched", "feat/other", ""), selected.Subject)
	f.UpdateFrom(selected.ClaimKey, ref.Version, newer)
	changed := f.claims[selected.ClaimKey]
	if s.UpdateFrom(selected.ClaimKey, ref.Version, msg) != deskkit.ClaimWriteRejected || f.claims[selected.ClaimKey] != changed {
		t.Fatal("stale subject CAS restored old owner")
	}
	// The lower effect boundary holds even with its upper preflight bypassed.
	s.selected.Reference.Kind = workref.Milestone
	if s.CreateIfAbsent(selected.ClaimKey, msg) == deskkit.ClaimWriteApplied {
		t.Fatal("milestone admitted below preflight")
	}
}

// regression: #5704
func TestClaimAliasHolds(t *testing.T) {
	catalog, peers := claimWorkFixture(t)
	catalog.Records[0].PhysicalAliases = []string{"old--stream--08"}
	work, err := catalog.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	f := newStore()
	s := &subjectClaimStore{ClaimStore: f, selected: work, peers: peers, complete: true, repository: "medici-finance/assay"}
	f.seedClaim("old--stream--08", "holder", "claimed", "", 0)
	before := f.claims["old--stream--08"]
	if s.CreateIfAbsent(work.ClaimKey, claimTestMessage(work.ClaimKey, "worker", "claimed", "", "")) == deskkit.ClaimWriteApplied || len(f.claims) != 1 || f.claims["old--stream--08"] != before {
		t.Fatal("historical live alias overwritten or second claim admitted")
	}
	if !strings.Contains(s.TransportCause(), "holder") {
		t.Fatal("alias holder not reported")
	}
	f.readFails = true
	if s.CreateIfAbsent(work.ClaimKey, claimTestMessage(work.ClaimKey, "worker", "claimed", "", "")) != deskkit.ClaimWriteUnverifiable {
		t.Fatal("missing enumeration granted create")
	}
	f.readFails = false
	delete(f.claims, "old--stream--08")
	race := &aliasRaceStore{fakeStore: f, alias: "old--stream--08"}
	s.ClaimStore = race
	if s.CreateIfAbsent(work.ClaimKey, claimTestMessage(work.ClaimKey, "worker", "claimed", "", "")) != deskkit.ClaimWriteUnverifiable {
		t.Fatal("post-create alias race admitted worker")
	}
	if race.removed {
		t.Fatal("post-create cleanup deleted a possibly reassigned ref")
	}
	// An incomplete writer census still holds with the upper preflight bypassed.
	s.complete = false
	if s.UpdateFrom(work.ClaimKey, f.claims[work.ClaimKey].sha, claimTestMessage(work.ClaimKey, "worker", "dispatched", "feat/work", "")) == deskkit.ClaimWriteApplied {
		t.Fatal("incomplete mixed writer set admitted write")
	}
}

type aliasRaceStore struct {
	*fakeStore
	alias   string
	removed bool
}

func (s *aliasRaceStore) CreateIfAbsent(id, msg string) deskkit.ClaimWriteOutcome {
	result := s.fakeStore.CreateIfAbsent(id, msg)
	s.seedClaim(s.alias, "racer", "claimed", "", 0)
	return result
}
func (s *aliasRaceStore) Remove(id string) (deskkit.ClaimWriteOutcome, bool) {
	s.removed = true
	return s.fakeStore.Remove(id)
}
