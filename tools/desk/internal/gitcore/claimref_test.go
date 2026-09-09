package gitcore

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// claimref_test.go — the dispatch-claim transport primitives, exercised OFFLINE against a
// local git-receive-pack / git-upload-pack server (a bare repo). go-git's client dispatches a
// local path to the file transport, which speaks the SAME plumbing protocol a real HTTPS remote
// does — so the server-side compare-and-swap these tests assert is the real thing git enforces,
// not a mock. (The production path is HTTPS, pure-Go, no git binary; these tests use a local
// git server purely as an offline stand-in.)

// bareServer creates an empty bare repo and returns its path, with partial-clone filters
// enabled so FetchTagPayload's tree:0 filter is honored (github/gitlab honor it in production).
func bareServer(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "--bare", "-b", "main", dir},
		{"-C", dir, "config", "uploadpack.allowFilter", "true"},
		{"-C", dir, "config", "uploadpack.allowAnySHA1InWant", "true"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

// TestPushRefUpdateCreateCASRejectsExisting is the fail-first CAS guard for ACQUIRE: a create
// (old=zero) succeeds once, and a SECOND create of the same ref is REJECTED by the server — the
// exact "someone already holds this claim" the acquire path maps to a held/dedup result.
func TestPushRefUpdateCreateCASRejectsExisting(t *testing.T) {
	server := bareServer(t)
	ctx := context.Background()
	id := "at--stream--07"
	ref := plumbing.ReferenceName("refs/dispatch/" + id)

	store, tagSHA, err := MintClaimTag(id, "dispatch-claim "+id+" owner=sess-A state=claimed branch=-", time.Now())
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	res, err := PushRefUpdate(ctx, RefUpdate{URL: server, Ref: ref, Old: plumbing.ZeroHash, New: tagSHA, Objects: store})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if res != RefUpdateApplied {
		t.Fatalf("first create result = %v, want Applied", res)
	}

	// A second create (old=zero) of the now-existing ref must be REJECTED server-side.
	store2, tag2, err := MintClaimTag(id, "dispatch-claim "+id+" owner=sess-B state=claimed branch=-", time.Now())
	if err != nil {
		t.Fatalf("mint 2: %v", err)
	}
	res, err = PushRefUpdate(ctx, RefUpdate{URL: server, Ref: ref, Old: plumbing.ZeroHash, New: tag2, Objects: store2})
	if err != nil {
		t.Fatalf("second create transport error (want a clean rejection, not an error): %v", err)
	}
	if res != RefUpdateRejected {
		t.Fatalf("second create result = %v, want Rejected (the CAS create must lose against an existing ref)", res)
	}
}

// TestPushRefUpdateStaleOldRejected is the fail-first CAS guard for PROGRESS/STEAL: an update
// whose Old no longer matches the ref's current value is REJECTED — the race the old
// PATCH force=true silently clobbered.
func TestPushRefUpdateStaleOldRejected(t *testing.T) {
	server := bareServer(t)
	ctx := context.Background()
	id := "at--stream--08"
	ref := plumbing.ReferenceName("refs/dispatch/" + id)

	store, first, err := MintClaimTag(id, "dispatch-claim "+id+" owner=a state=claimed branch=-", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res, err := PushRefUpdate(ctx, RefUpdate{URL: server, Ref: ref, Old: plumbing.ZeroHash, New: first, Objects: store}); err != nil || res != RefUpdateApplied {
		t.Fatalf("seed create: res=%v err=%v", res, err)
	}

	// Advance legitimately from `first` to `second` — the holder's own CAS, must apply.
	store2, second, err := MintClaimTag(id, "dispatch-claim "+id+" owner=a state=dispatched branch=feat/x", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res, err := PushRefUpdate(ctx, RefUpdate{URL: server, Ref: ref, Old: first, New: second, Objects: store2}); err != nil || res != RefUpdateApplied {
		t.Fatalf("legit advance: res=%v err=%v", res, err)
	}

	// Now try to advance from the STALE `first` again — the ref holds `second`, so the CAS
	// must be rejected server-side.
	store3, third, err := MintClaimTag(id, "dispatch-claim "+id+" owner=intruder state=claimed branch=-", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	res, err := PushRefUpdate(ctx, RefUpdate{URL: server, Ref: ref, Old: first, New: third, Objects: store3})
	if err != nil {
		t.Fatalf("stale-old update transport error (want a clean rejection): %v", err)
	}
	if res != RefUpdateRejected {
		t.Fatalf("stale-old update result = %v, want Rejected", res)
	}
}

// TestFetchTagPayloadRoundTripsGoMintedTag proves a Go-minted (empty-blob-target) claim tag is
// read back with its message and tagger date intact via the filtered upload-pack.
func TestFetchTagPayloadRoundTripsGoMintedTag(t *testing.T) {
	server := bareServer(t)
	ctx := context.Background()
	id := "at--stream--09"
	ref := plumbing.ReferenceName("refs/dispatch/" + id)
	when := time.Now().Add(-42 * time.Minute).Truncate(time.Second)
	msg := "dispatch-claim " + id + " owner=sess-Z state=dispatched branch=feat/y"

	store, tagSHA, err := MintClaimTag(id, msg, when)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := PushRefUpdate(ctx, RefUpdate{URL: server, Ref: ref, Old: plumbing.ZeroHash, New: tagSHA, Objects: store}); err != nil || res != RefUpdateApplied {
		t.Fatalf("create: res=%v err=%v", res, err)
	}

	got, err := FetchTagPayload(ctx, server, nil, tagSHA)
	if err != nil {
		t.Fatalf("FetchTagPayload: %v", err)
	}
	if strings.TrimRight(got.Message, "\n") != msg {
		t.Errorf("message round-trip: got %q want %q", got.Message, msg)
	}
	if !got.When.Equal(when.UTC()) {
		t.Errorf("tagger date round-trip: got %v want %v", got.When, when.UTC())
	}
}

// TestFetchTagPayloadReadsCommitTargetTag is the interop half: a bash/REST-minted claim tag
// targets a COMMIT (the script's `type=commit`), not the empty blob. The Go reader must parse
// its message and date regardless of the target type. This builds such a tag with the git
// binary (standing in for the REST tag-create the script uses) and reads it back with go-git.
func TestFetchTagPayloadReadsCommitTargetTag(t *testing.T) {
	server := bareServer(t)
	ctx := context.Background()
	id := "at--rest--01"
	msg := "dispatch-claim " + id + " owner=rest-sess state=claimed branch=-"

	// A working repo to author a commit-target annotated tag, then push it into the bare
	// server's dispatch namespace exactly as a REST tag-create + ref-create would leave it.
	work := gittest.NewFixture(t)
	if _, err := work.Git("tag", "-a", "dispatch/"+id, "-m", msg, "HEAD"); err != nil {
		t.Fatalf("git tag -a: %v", err)
	}
	if _, err := work.Git("push", server, "refs/tags/dispatch/"+id+":refs/dispatch/"+id); err != nil {
		t.Fatalf("push tag: %v", err)
	}

	refs, err := List(ListOpts{URL: server})
	if err != nil {
		t.Fatal(err)
	}
	var tagSHA plumbing.Hash
	for _, r := range refs {
		if r.Name().String() == "refs/dispatch/"+id {
			tagSHA = r.Hash()
		}
	}
	if tagSHA.IsZero() {
		t.Fatalf("refs/dispatch/%s not advertised; refs=%v", id, refs)
	}

	got, err := FetchTagPayload(ctx, server, nil, tagSHA)
	if err != nil {
		t.Fatalf("FetchTagPayload on a commit-target tag: %v", err)
	}
	if strings.TrimRight(got.Message, "\n") != msg {
		t.Errorf("commit-target message: got %q want %q", got.Message, msg)
	}
	if got.When.IsZero() {
		t.Error("commit-target tagger date was not read")
	}
}
