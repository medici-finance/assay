package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rowsAcrossADay writes `rows` entries, forcing a rotation at index splitAt by back-dating
// the live file's mtime to the previous UTC day — exactly the first-append-of-a-new-day
// condition rotateIfNeeded's one-stat check looks for. It returns the state dir.
func rowsAcrossADay(t *testing.T, rows []Entry, splitAt int) string {
	t.Helper()
	dir := setup(t)
	for i, e := range rows {
		if i == splitAt {
			// Make the live file look like it was last appended YESTERDAY, and put
			// "now" on the next day: exactly the first-append-of-a-new-day condition.
			p := filepath.Join(dir, "audit.jsonl")
			yesterday := time.Now().UTC().AddDate(0, 0, -1)
			if err := os.Chtimes(p, yesterday, yesterday); err != nil {
				t.Fatalf("chtimes: %v", err)
			}
		}
		if err := Log(e); err != nil {
			t.Fatalf("Log %d: %v", i, err)
		}
	}
	return dir
}

// TestRotationCarriesCounterAndIdempotencyForward is the rotation row. Rotation is the one
// operation auditrecover.go's package comment identifies as a full state RESET when done by
// a plain file move: the counter returns to full and the idempotency store forgets every
// prior write. This asserts that the daily rotation is not that — same rows, same meters,
// same answers — and that it touches nothing that is not a segment.
func TestRotationCarriesCounterAndIdempotencyForward(t *testing.T) {
	head := "f00dcafe"
	mk := func(i int, result string) Entry {
		pr := 7
		h := head
		return Entry{
			TS:      time.Now().UTC().Add(time.Duration(i-20) * time.Minute).Format(time.RFC3339),
			Tool:    "deskpost",
			Verb:    "comment",
			Repo:    "example-org/repo",
			PR:      &pr,
			HeadSHA: &h,
			Result:  result,
			Detail:  "row",
		}
	}
	rows := make([]Entry, 0, 12)
	for i := 0; i < 12; i++ {
		rows = append(rows, mk(i, ResultOK))
	}

	// (a) one unrotated file — the control.
	setup(t)
	for _, e := range rows {
		if err := Log(e); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
	flatEntries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries (flat): %v", err)
	}
	now := time.Now()
	flatPoints, err := pointsFor("deskpost", now)
	if err != nil {
		t.Fatalf("pointsFor (flat): %v", err)
	}
	flatBudget := AllowWriteAt("deskpost", "example-org/repo", 7, now)
	flatDone := AlreadyDoneIn(flatEntries, "example-org/repo", 7, head, "comment")

	// (b) the same rows across a rotation.
	dir := rowsAcrossADay(t, rows, 6)

	segs, err := segmentPaths()
	if err != nil {
		t.Fatalf("segmentPaths: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected one rotated segment plus the live file, got %v", segs)
	}
	segName := filepath.Base(segs[0])
	if !segmentPattern.MatchString(segName) {
		t.Fatalf("rotated segment %q does not match audit.jsonl.<YYYY-MM-DD>", segName)
	}

	rotEntries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries (rotated): %v", err)
	}
	if len(rotEntries) != len(flatEntries) {
		t.Fatalf("rotation lost rows: %d across segments vs %d flat", len(rotEntries), len(flatEntries))
	}
	for i := range rotEntries {
		if rotEntries[i].TS != flatEntries[i].TS || rotEntries[i].Detail != flatEntries[i].Detail {
			t.Fatalf("rotation changed row %d: %+v vs %+v", i, rotEntries[i], flatEntries[i])
		}
	}

	rotPoints, err := pointsFor("deskpost", now)
	if err != nil {
		t.Fatalf("pointsFor (rotated): %v", err)
	}
	if len(rotPoints) != len(flatPoints) {
		t.Fatalf("the counter did not carry forward: %d charged points across segments vs %d flat", len(rotPoints), len(flatPoints))
	}
	if got := AllowWriteAt("deskpost", "example-org/repo", 7, now); (got == nil) != (flatBudget == nil) {
		t.Fatalf("budget verdict changed across rotation: %v vs %v", got, flatBudget)
	}
	if got := AlreadyDoneIn(rotEntries, "example-org/repo", 7, head, "comment"); got != flatDone {
		t.Fatalf("the idempotency store did not carry forward: %v vs %v", got, flatDone)
	}
	if !flatDone {
		t.Fatal("fixture too weak: the control must be already-done, or the assertion above proves nothing")
	}

	// Rotation must touch nothing that is not a segment.
	for _, decoy := range []string{"audit.lock", "audit.jsonl.corrupt-20260101T000000Z", "DISABLED.bak"} {
		p := filepath.Join(dir, decoy)
		if err := os.WriteFile(p, []byte("decoy\n"), 0o600); err != nil {
			t.Fatalf("write decoy: %v", err)
		}
	}
	live := filepath.Join(dir, "audit.jsonl")
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	if err := os.Chtimes(live, yesterday, yesterday); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := Log(mk(20, ResultOK)); err != nil {
		t.Fatalf("Log after decoys: %v", err)
	}
	for _, decoy := range []string{"audit.lock", "audit.jsonl.corrupt-20260101T000000Z", "DISABLED.bak"} {
		if _, serr := os.Stat(filepath.Join(dir, decoy)); serr != nil {
			t.Fatalf("rotation removed or renamed %q, which is not a segment: %v", decoy, serr)
		}
	}
	segs2, err := segmentPaths()
	if err != nil {
		t.Fatalf("segmentPaths after decoys: %v", err)
	}
	for _, p := range segs2 {
		if base := filepath.Base(p); base != "audit.jsonl" && !segmentPattern.MatchString(base) {
			t.Fatalf("segmentPaths admitted a non-segment: %q", base)
		}
	}
}

// TestRecoverAcrossSegments — a malformed line in a ROTATED segment is exactly as fatal to
// every reader as one in the live file, so the recovery verb has to reach it. Before daily
// rotation existed there was only one file to repair; a recovery that still only looked
// there would leave the ledger permanently refusing with its own printed remedy already run.
func TestRecoverAcrossSegments(t *testing.T) {
	dir := setup(t)
	yday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// A rotated segment with two good rows and one malformed line between them, and a
	// live file with two good rows and one malformed line.
	segPath := filepath.Join(dir, "audit.jsonl."+yday)
	plantLedger(t, dir, "audit.jsonl."+yday, 2, "seg")
	appendRaw(t, segPath, "{this is not json")
	plantLedger(t, dir, "audit.jsonl", 2, "live")
	appendRaw(t, filepath.Join(dir, "audit.jsonl"), "]also not json")

	if _, err := LoadEntries(); !IsUnverifiable(err) {
		t.Fatalf("precondition: a malformed line must make LoadEntries refuse, got %v", err)
	}

	res, err := RecoverCorruptAudit()
	if err != nil {
		t.Fatalf("RecoverCorruptAudit: %v", err)
	}
	if res.Quarantined != 2 {
		t.Fatalf("quarantined %d lines, want 2 (one per file)", res.Quarantined)
	}
	if res.Carried != 4 {
		t.Fatalf("carried %d entries, want 4 (two per file)", res.Carried)
	}
	if !res.Rewrote {
		t.Fatal("Rewrote=false despite corruption")
	}

	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries after recovery: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("after recovery: %d entries, want 4 — every good row in BOTH files carries forward", len(entries))
	}
	want := []string{"seg 0", "seg 1", "live 0", "live 1"}
	for i, w := range want {
		if entries[i].Detail != w {
			t.Fatalf("entry %d = %q, want %q (segment order must survive recovery)", i, entries[i].Detail, w)
		}
	}
	side, err := os.ReadFile(res.QuarantinePath)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	for _, bad := range []string{"{this is not json", "]also not json"} {
		if !strings.Contains(string(side), bad) {
			t.Fatalf("sidecar is missing a quarantined line: %q", bad)
		}
	}
}

func appendRaw(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, werr := f.WriteString(line + "\n"); werr != nil {
		t.Fatalf("append: %v", werr)
	}
	if cerr := f.Close(); cerr != nil {
		t.Fatalf("close: %v", cerr)
	}
}

// TestRotationDueIsAUTCDayBoundary pins the one-stat due-check itself, since every other
// assertion here depends on it firing exactly at a UTC day boundary and not on an elapsed
// interval.
func TestRotationDueIsAUTCDayBoundary(t *testing.T) {
	base := time.Date(2026, 9, 14, 23, 59, 59, 0, time.UTC)
	cases := []struct {
		name       string
		mtime, now time.Time
		want       bool
	}{
		{"same day, 24h apart is impossible but same date is not due", base.Add(-23 * time.Hour), base, false},
		{"one second later, next UTC day", base, base.Add(time.Second), true},
		{"already rotated today", base.Add(time.Second), base.Add(time.Hour), false},
		{"days behind", base.AddDate(0, 0, -9), base, true},
	}
	for _, c := range cases {
		if got := rotationDue(c.mtime, c.now); got != c.want {
			t.Fatalf("%s: rotationDue = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSegmentPathsOrdersOldestFirst — the order LoadEntries concatenates in IS the ledger's
// order, so getting it wrong reorders history for every meter that reads it.
func TestSegmentPathsOrdersOldestFirst(t *testing.T) {
	dir := setup(t)
	for _, name := range []string{
		"audit.jsonl.2026-09-14", "audit.jsonl.2026-09-13.1", "audit.jsonl.2026-09-13",
		"audit.lock", "audit.jsonl.corrupt-20260913T101010Z", "notes.txt",
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	paths, err := segmentPaths()
	if err != nil {
		t.Fatalf("segmentPaths: %v", err)
	}
	var got []string
	for _, p := range paths {
		got = append(got, filepath.Base(p))
	}
	want := []string{"audit.jsonl.2026-09-13", "audit.jsonl.2026-09-13.1", "audit.jsonl.2026-09-14", "audit.jsonl"}
	if len(got) != len(want) {
		t.Fatalf("segmentPaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segmentPaths = %v, want %v", got, want)
		}
	}
}

// TestLogStillAppendsOnlyAcrossRotation — Log's append-only contract is untouched: the
// rotation hook must never truncate, rewrite or reorder, only rename.
func TestLogStillAppendsOnlyAcrossRotation(t *testing.T) {
	dir := setup(t)
	if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, Detail: "first"}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	if err := os.Chtimes(filepath.Join(dir, "audit.jsonl"), yesterday, yesterday); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultNoop, Detail: "second"}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if len(entries) != 2 || entries[0].Detail != "first" || entries[1].Detail != "second" {
		b, _ := json.Marshal(entries)
		t.Fatalf("rotation lost or reordered rows: %s", b)
	}
	live, err := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if strings.Contains(string(live), `"first"`) {
		t.Fatal("the pre-rotation row is still in the live file — nothing was rotated")
	}
}
