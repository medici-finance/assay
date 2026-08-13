package deskkit

import "testing"

// TestAlreadyDoneDuplicate — the positive: a prior ok entry at the same
// (repo, pr, head, verb) → AlreadyDone true.
func TestAlreadyDoneDuplicate(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{
		Tool: "deskpost", Verb: "ready", Result: ResultOK,
		Repo: "example-org/agents", PR: iptr(42), HeadSHA: sptr("deadbeef"),
	})
	if !AlreadyDone("example-org/agents", 42, "deadbeef", "ready") {
		t.Fatalf("AlreadyDone = false, want true for a duplicate")
	}
}

func TestAlreadyDoneNoopCounts(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{
		Tool: "deskpr", Verb: "pr-create", Result: ResultNoop,
		Repo: "example-org/agents", PR: iptr(7), HeadSHA: sptr("abc123"),
	})
	if !AlreadyDone("example-org/agents", 7, "abc123", "pr-create") {
		t.Fatalf("AlreadyDone = false, want true (noop counts as done)")
	}
}

// A prior REFUSAL does not count — a refusal followed by a state change is a
// legitimate retry.
func TestAlreadyDoneRefusalDoesNotCount(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{
		Tool: "deskpost", Verb: "ready", Result: ResultRefused,
		Repo: "example-org/agents", PR: iptr(42), HeadSHA: sptr("deadbeef"),
	})
	if AlreadyDone("example-org/agents", 42, "deadbeef", "ready") {
		t.Fatalf("AlreadyDone = true after only a refusal; a refusal must not count as done")
	}
}

// A different head, pr, repo, or verb is NOT the same idempotency key.
func TestAlreadyDoneKeyDiscrimination(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{
		Tool: "deskpost", Verb: "ready", Result: ResultOK,
		Repo: "example-org/agents", PR: iptr(42), HeadSHA: sptr("deadbeef"),
	})
	cases := []struct {
		name             string
		repo, head, verb string
		pr               int
	}{
		{"different head", "example-org/agents", "cafef00d", "ready", 42},
		{"different pr", "example-org/agents", "deadbeef", "ready", 99},
		{"different repo", "example-org/examples", "deadbeef", "ready", 42},
		{"different verb", "example-org/agents", "deadbeef", "comment", 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if AlreadyDone(c.repo, c.pr, c.head, c.verb) {
				t.Fatalf("AlreadyDone = true, want false for a different key")
			}
		})
	}
}

// TestAlreadyDoneCorruptNeverTrue — on a corrupt audit file AlreadyDone must never
// report true: a corrupt file must never masquerade as "already done" and silently
// skip a needed write. The corrupt-file REFUSAL itself is raised by
// AllowWrite / LoadEntries (exit 6), which the outward-write flow calls first.
func TestAlreadyDoneCorruptNeverTrue(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{
		Tool: "deskpost", Verb: "ready", Result: ResultOK,
		Repo: "example-org/agents", PR: iptr(42), HeadSHA: sptr("deadbeef"),
	})
	appendLine(t, dir, `{"corrupt`)
	if AlreadyDone("example-org/agents", 42, "deadbeef", "ready") {
		t.Fatalf("AlreadyDone = true on a corrupt file; must never falsely dedup")
	}
	// And the refusal is observable through the reader the flow gates on.
	if _, err := LoadEntries(); !IsUnverifiable(err) {
		t.Fatalf("LoadEntries on corrupt file = %v, want Unverifiable", err)
	}
}

// AlreadyDoneIn is a pure predicate with no error path (operates on already-verified
// entries under the flock).
func TestAlreadyDoneInPure(t *testing.T) {
	entries := []Entry{
		{Verb: "review", Result: ResultOK, Repo: "example-org/agents", PR: iptr(1), HeadSHA: sptr("h1")},
	}
	if !AlreadyDoneIn(entries, "example-org/agents", 1, "h1", "review") {
		t.Fatalf("AlreadyDoneIn = false, want true")
	}
	if AlreadyDoneIn(nil, "example-org/agents", 1, "h1", "review") {
		t.Fatalf("AlreadyDoneIn(nil) = true, want false")
	}
	// A nil PR/head entry must not match a concrete key.
	weird := []Entry{{Verb: "review", Result: ResultOK, Repo: "example-org/agents"}}
	if AlreadyDoneIn(weird, "example-org/agents", 1, "h1", "review") {
		t.Fatalf("AlreadyDoneIn matched an entry with nil pr/head")
	}
}
