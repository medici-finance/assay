package main

// forgeread_test.go — the statusgen half of slice 1's Verify rows.
//
//	TestIssueDebtNoticeOptInOnly            the offline default, and the opt-in that restores the line
//	TestOfflineReaderNeverAnswersEmpty      the three-state property the whole offline half rests on
//	TestParseBriefFileMemoDistinctPaths     the memo, INCLUDING the mid-run change that must re-parse
//	TestAttributionOneWalkMatchesPerPathReads  the walk answers what the per-path pair answered

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- the offline default --------------------------------------------------------------------

// recordingReader is a forgeReader that counts its calls, so a test can assert a read did NOT
// happen rather than only that its output was absent.
type recordingReader struct {
	calls  int
	issues map[string][]forgeIssue
	unav   []forgeUnavailable
}

func (r *recordingReader) OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error) {
	r.calls++
	return r.issues, r.unav, nil
}

// TestIssueDebtNoticeOptInOnly is the negative-path row for the offline default.
//
// FAIL-FIRST. Before this change openIssueDebtNotice took no reader at all: it looked the forge
// CLI up on PATH and shelled one list per configured repo unconditionally under --lint. There was
// no argument to pass and no state to stub, so the "offline" case below could not even be
// expressed — which is why the first assertion is about a reader being wired at all.
func TestIssueDebtNoticeOptInOnly(t *testing.T) {
	// Offline (the DEFAULT): every repo could-not-check, so no line and no claim of zero debt.
	off := newOfflineReader()
	if got := openIssueDebtNotice(7, off); got != "" {
		t.Errorf("offline issue-debt notice = %q, want \"\" — an offline lint must not emit a debt line", got)
	}

	// A nil reader is also silent, and silent is the only safe direction: a missing reader is
	// not evidence of no debt.
	if got := openIssueDebtNotice(7, nil); got != "" {
		t.Errorf("nil-reader issue-debt notice = %q, want \"\"", got)
	}

	// Opted in, with a repo carrying a genuinely stale open issue: the line returns.
	repos := reposForIssues()
	if len(repos) == 0 {
		t.Fatal("reposForIssues returned nothing — the fixture roster is not loaded")
	}
	data := map[string][]forgeIssue{}
	for i, r := range repos {
		if i == 0 {
			data[r] = []forgeIssue{{
				Number:    4242,
				State:     "open",
				CreatedAt: nowFunc().Add(-90 * 24 * time.Hour),
			}}
			continue
		}
		data[r] = nil // read, and genuinely empty — an ANSWER, not an absence
	}
	rec := &recordingReader{issues: data}
	got := openIssueDebtNotice(7, rec)
	if rec.calls != 1 {
		t.Errorf("reader calls = %d, want exactly 1 — ONE read serves the whole repo set", rec.calls)
	}
	if !strings.Contains(got, "issue debt:") || !strings.Contains(got, "#4242") {
		t.Errorf("opted-in issue-debt notice = %q, want the debt line naming the stale issue", got)
	}

	// PARTIAL: one repo unread. The line is WITHHELD rather than computed from a subset — an
	// understated debt count reads as "we looked and it is fine", which is worse than silence.
	partial := &recordingReader{
		issues: data,
		unav:   []forgeUnavailable{{Repo: repos[0], Reason: "the App installation cannot read it"}},
	}
	if got := openIssueDebtNotice(7, partial); got != "" {
		t.Errorf("partial-read issue-debt notice = %q, want \"\" — a debt count assembled from a SUBSET understates it", got)
	}
}

// TestOfflineReaderNeverAnswersEmpty pins the three-state property every forge-backed check will
// depend on: the offline reader reports each repo as unavailable and returns NO data entry for
// it, so there is no shape in which "did not look" is indistinguishable from "found nothing".
func TestOfflineReaderNeverAnswersEmpty(t *testing.T) {
	repos := []string{"example-org/alpha", "example-org/beta"}
	data, unav, err := newOfflineReader().OpenIssues(repos)
	if err != nil {
		t.Fatalf("offline reader returned an error: %v", err)
	}
	if len(unav) != len(repos) {
		t.Fatalf("unavailable = %d, want %d — every repo is could-not-check offline", len(unav), len(repos))
	}
	for _, u := range unav {
		if u.Reason == "" {
			t.Errorf("%s is unavailable with no reason — an unexplained absence is not a could-not-check", u.Repo)
		}
	}
	for _, r := range repos {
		if _, present := data[r]; present {
			t.Errorf("%s carries a data entry offline — an unread repo must have NO entry, not an empty one", r)
		}
	}
}

// --- the memo ---------------------------------------------------------------------------------

// TestParseBriefFileMemoDistinctPaths is the memo's row. The assertion is on DISTINCT parses
// performed, not on calls avoided: the point of the memo is to decouple the two.
//
// FAIL-FIRST. Against the pre-change parser every call parsed, so distinct-parse count equalled
// CALL count — the first assertion below (3 calls, 1 parse) fails on the unfixed code. The
// mutation that re-reddens it is deleting the briefParseMemoGet lookup in parseBriefFile.
func TestParseBriefFileMemoDistinctPaths(t *testing.T) {
	resetBriefParseMemo()
	dir := t.TempDir()
	path := filepath.Join(dir, "brief-01-example.md")
	write := func(title string) {
		body := "---\n" +
			"brief: example-stream/01\n" +
			"title: " + title + "\n" +
			"wave: 1\ndepends: []\nunblocks: []\neffort: S\ngate: model\n" +
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
			"issues: []\nschema: brief-v1\nauthored: 2026-09-14 by test\nsources: []\nversion: 1\n" +
			"---\n\n# body\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("first")

	for i := 0; i < 3; i++ {
		bf, ok, err := parseBriefFile(path)
		if err != nil || !ok || bf == nil {
			t.Fatalf("parse %d: ok=%v err=%v", i, ok, err)
		}
		if bf.Title != "first" {
			t.Fatalf("parse %d: title=%q, want first", i, bf.Title)
		}
	}
	if n := briefParseCount.Load(); n != 1 {
		t.Errorf("distinct parses = %d after 3 calls on one unchanged file, want 1", n)
	}

	// The result must be a COPY: assigning to a field of one caller's result must not be
	// visible to the next caller.
	bf, _, _ := parseBriefFile(path)
	bf.Title = "mutated by a caller"
	again, _, _ := parseBriefFile(path)
	if again.Title != "first" {
		t.Errorf("a caller's field assignment reached the memo: title=%q, want first", again.Title)
	}

	// A file that CHANGES mid-run is re-parsed. This is the half that makes the memo safe: a
	// cache keyed on the path alone would keep serving content that is no longer on disk.
	time.Sleep(10 * time.Millisecond) // ensure a distinguishable mtime on a coarse-stamp filesystem
	write("second, and longer so the size differs too")
	changed, ok, err := parseBriefFile(path)
	if err != nil || !ok {
		t.Fatalf("re-parse after change: ok=%v err=%v", ok, err)
	}
	if changed.Title != "second, and longer so the size differs too" {
		t.Errorf("after a mid-run edit the memo served STALE content: title=%q", changed.Title)
	}
	if n := briefParseCount.Load(); n != 2 {
		t.Errorf("distinct parses = %d after the file changed, want 2", n)
	}
}

// --- the one-walk authorship index ------------------------------------------------------------

// TestAttributionOneWalkMatchesPerPathReads is the equality row. The walk is only allowed to be
// a COST change: for every path it accounts for, it must answer exactly what the per-path pair
// answers — including for a file with a multi-commit, multi-author history, which is the case a
// newest-first/oldest-last misreading gets backwards.
//
// FAIL-FIRST. Swapping the two assignments in buildPathAuthorIndex (taking the first sighting as
// `first` rather than `last`) reddens this test on the multi-author file while leaving the
// single-commit file passing — which is why the fixture carries both.
func TestAttributionOneWalkMatchesPerPathReads(t *testing.T) {
	root := t.TempDir()
	streams := filepath.Join(root, "docs", "streams", "example-stream")
	if err := os.MkdirAll(streams, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "commit.gpgsign", "false")

	commit := func(rel, content, name, email string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", rel)
		runGit(t, root, "-c", "user.name="+name, "-c", "user.email="+email,
			"commit", "-q", "-m", "touch "+rel)
	}

	// A file authored by one identity and last touched by another — the whole point of the
	// cross-check, and the case an inverted walk gets wrong.
	commit("docs/streams/example-stream/brief-01-a.md", "one\n", "Author One", "AUTHOR-one@example.test")
	commit("docs/streams/example-stream/brief-02-b.md", "two\n", "Author One", "author-one@example.test")
	commit("docs/streams/example-stream/brief-01-a.md", "one, edited\n", "Author Two", "author-two@example.test")
	// And a third commit touching a DIFFERENT file, so the newest commit in the walk is not the
	// one that touched brief-01 — a walk that ignored per-path sightings would read this one.
	commit("docs/streams/example-stream/brief-02-b.md", "two, edited\n", "Author Three", "author-three@example.test")

	idx, ok := buildPathAuthorIndex(root, filepath.FromSlash("docs/streams"))
	if !ok {
		t.Fatal("buildPathAuthorIndex failed on a real repo")
	}
	if len(idx) == 0 {
		t.Fatal("the index is empty — the walk parsed no paths")
	}

	for _, rel := range []string{
		"docs/streams/example-stream/brief-01-a.md",
		"docs/streams/example-stream/brief-02-b.md",
	} {
		wantFirst, fok := gitPathFirstAuthorIdentity(root, rel)
		wantLast, lok := gitPathLastAuthorIdentity(root, rel)
		if !fok || !lok {
			t.Fatalf("%s: the per-path pair could not read its own fixture", rel)
		}
		gotFirst, gotLast, gok := lookupPathAuthors(idx, root, rel)
		if !gok {
			t.Fatalf("%s: lookupPathAuthors reported unavailable where the pair succeeded", rel)
		}
		if gotFirst != wantFirst {
			t.Errorf("%s: first author = %q, per-path read says %q", rel, gotFirst, wantFirst)
		}
		if gotLast != wantLast {
			t.Errorf("%s: last author = %q, per-path read says %q", rel, gotLast, wantLast)
		}
	}

	// brief-01 must show DIFFERENT first and last identities, or the equality above would hold
	// trivially for a walk that returned the same value twice.
	f, l, _ := lookupPathAuthors(idx, root, "docs/streams/example-stream/brief-01-a.md")
	if f == l {
		t.Errorf("the fixture's multi-author file resolved first==last (%q) — the test would pass for a walk that never distinguishes them", f)
	}

	// A path the index does not account for falls back to the per-path reads rather than
	// reporting unavailable, so the walk can never LOSE a signal the pair would have found.
	commit("docs/streams/example-stream/brief-03-c.md", "three\n", "Author Four", "author-four@example.test")
	if _, _, fbok := lookupPathAuthors(idx, root, "docs/streams/example-stream/brief-03-c.md"); !fbok {
		t.Error("a path absent from a stale index did not fall back to the per-path read")
	}

	// And a path with no history at all still degrades LOUDLY — the property attribution's
	// per-brief NOTICE depends on.
	if _, _, uok := lookupPathAuthors(idx, root, "docs/streams/example-stream/never-committed.md"); uok {
		t.Error("an uncommitted path reported readable identities — the loud degradation is gone")
	}
}
